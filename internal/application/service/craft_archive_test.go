package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// loadCraftArchiveFixture reads one committed archive fixture from the craft
// module's testdata directory.
func loadCraftArchiveFixture(t *testing.T, name string) []byte {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	data, err := os.ReadFile(filepath.Join(repoRoot, "internal", "modules", "craft", "testdata", "archive", name))
	require.NoError(t, err)
	return data
}

func sha256SumBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// buildZipEntries builds an in-memory regular-file zip.
func buildZipEntries(t *testing.T, names []string, contentFor func(string) []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(contentFor(name))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// buildTruncatedTarGz builds a valid gzipped tar with two members and cuts
// the tail so the second member's data dies mid-stream.
func buildTruncatedTarGz(t *testing.T) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	entries := []struct {
		name string
		data []byte
	}{
		{"first.txt", bytes.Repeat([]byte("A"), 4096)},
		{"second.txt", bytes.Repeat([]byte("B"), 1<<20)},
	}
	for _, e := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.data))}))
		_, err := tw.Write(e.data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	_, err := zw.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	full := gzBuf.Bytes()
	// Highly compressible members compress to a few hundred bytes; halving
	// the stream guarantees the damage lands mid-member.
	require.Greater(t, len(full), 128, "fixture must be long enough to truncate mid-member")
	return full[:len(full)/2]
}

// craftT02FailingFiles wraps the T01 fake files and fails SaveBytes on a
// chosen call (1-based across all saves) so publish-phase rollback can be
// exercised.
type craftT02FailingFiles struct {
	*craftT01Files
	failOnSave int
	saves      int
}

func (f *craftT02FailingFiles) SaveBytes(ctx context.Context, data []byte, tenantID uint64, name string, temp bool) (string, error) {
	f.saves++
	if f.saves == f.failOnSave {
		return "", errors.New("injected store failure")
	}
	return f.craftT01Files.SaveBytes(ctx, data, tenantID, name, temp)
}

// TestCraftT02Journey is the T02 acceptance journey: a member uploads a
// bounded archive, the server expands it into content-addressed read-only
// inputs under the hard ceilings, the expanded material rides the same
// admission fence as ordinary uploads, and replay is idempotent.
func TestCraftT02Journey(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-journey", "Archive input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	// The archive enters as an ordinary opaque input within the T01 quotas.
	archiveBytes := loadCraftArchiveFixture(t, "valid.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "sales-pack.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)
	require.Len(t, uploaded, 1)
	require.NotNil(t, uploaded[0].Recognition)
	require.True(t, uploaded[0].Recognition.Accepted)
	require.False(t, uploaded[0].Recognition.Understood, "the archive itself stays opaque until expanded")

	// Expansion produces content-addressed member inputs with real content
	// recognition: the valid JSON member is understood, the others are
	// accepted but explicitly not understood.
	members, err := env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.NoError(t, err)
	require.Len(t, members, 3)
	byPath := make(map[string]craft.Input, len(members))
	for _, member := range members {
		byPath[member.Name] = member
		require.NotEmpty(t, member.Ref, "each member becomes a referenced immutable object")
		require.NotEmpty(t, member.CitationID)
		require.NotNil(t, member.Recognition)
		require.True(t, member.Recognition.Accepted)
	}
	// Members carry the base name of their canonical archive path: the
	// frozen input contract names exactly one canonical path element, and
	// the full path was already validated and deduplicated at extraction.
	require.Equal(t, "meta.json", members[0].Name)
	require.Equal(t, "sales.csv", members[1].Name)
	require.Equal(t, "readme.md", members[2].Name)
	csvMember := byPath["sales.csv"]
	csvSum := sha256.Sum256([]byte("region,sales\nnorth,120\nsouth,80\n"))
	require.Equal(t, hex.EncodeToString(csvSum[:]), csvMember.SHA256, "members are content-addressed by their own bytes")
	require.EqualValues(t, len("region,sales\nnorth,120\nsouth,80\n"), csvMember.Bytes)
	jsonMember := byPath["meta.json"]
	require.True(t, jsonMember.Recognition.Understood, "valid bounded JSON is parsed by the supported consumer")
	require.False(t, csvMember.Recognition.Understood)

	// The members are persisted inputs of the workspace.
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 4, "archive plus three expanded members")

	// Expanded material rides the same admission fence as uploads: the
	// unrecognized member cannot join a Run without an explicit continue.
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{
		RequestID: "r1", Prompt: "Make a sales page", InputRefs: []string{csvMember.Ref},
	})
	require.ErrorIs(t, err, craft.ErrConflict, "expanded unrecognized material must not silently join a Run")
	require.NoError(t, env.svc.DecideInput(ctx, scope, csvMember.Ref, "continue"))
	run, _, err := env.svc.StartRun(ctx, scope, CraftRunRequest{
		RequestID: "r1", Prompt: "Make a sales page", InputRefs: []string{csvMember.Ref},
	})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
	var runs int64
	require.NoError(t, env.db.Table("agent_runs").Where("session_id = ?", ws.SessionID).Count(&runs).Error)
	require.EqualValues(t, 1, runs)

	// Idempotent replay: expanding the same archive again returns the same
	// member manifest without duplicating rows.
	replayed, err := env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.NoError(t, err)
	require.Equal(t, members, replayed)
	persisted, err = env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 4, "replay must not duplicate expanded inputs")

	// Authorization: another member cannot expand this workspace's archive.
	strangerCtx := craftCtx(1, "u2", ws.SessionID)
	strangerScope := ownerScope(1, "u2", ws.SessionID)
	_, err = env.svc.ExpandArchive(strangerCtx, strangerScope, uploaded[0].Ref)
	require.True(t, errors.Is(err, craft.ErrForbidden) || errors.Is(err, craft.ErrNotFound),
		"cross-user expansion must fail closed, got %v", err)

	// An unknown ref resolves to nothing usable.
	_, err = env.svc.ExpandArchive(ctx, scope, "craft-test://missing.zip")
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// TestCraftArchiveRejectsHostileArchive proves the path gauntlet end to end:
// the hostile member is refused and no member object or row is ever written.
func TestCraftArchiveRejectsHostileArchive(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-hostile", "Hostile archive", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	hostile := buildZipEntries(t, []string{"ok.txt", "../escape.txt"}, func(name string) []byte {
		if name == "ok.txt" {
			return []byte("fine")
		}
		return []byte("evil")
	})
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "hostile.zip", Content: hostile, SHA256: sha256SumBytes(hostile),
	}})
	require.NoError(t, err)

	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1, "only the archive itself remains associated")
	require.Equal(t, uploaded[0].Ref, persisted[0].Ref)
	require.Len(t, files.blobs, 1, "no member object may be stored from a rejected archive")
	require.Contains(t, files.blobs, uploaded[0].Ref)
}

// TestCraftArchiveBombFailsWithNoMaterial proves a compression bomb is
// refused deterministically and leaves no expanded material behind.
func TestCraftArchiveBombFailsWithNoMaterial(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-bomb", "Bomb archive", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	bomb := loadCraftArchiveFixture(t, "bomb.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "bomb.zip", Content: bomb, SHA256: sha256SumBytes(bomb),
	}})
	require.NoError(t, err)

	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Len(t, files.blobs, 1, "the bomb must not expand into stored objects")
}

// TestCraftArchiveMidExtractionFailureLeavesNoMaterial proves the recovery
// path: a stream that dies mid-member leaves no partially expanded material
// usable by a Run.
func TestCraftArchiveMidExtractionFailureLeavesNoMaterial(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-truncated", "Truncated archive", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	truncated := buildTruncatedTarGz(t)
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "partial.tgz", Content: truncated, SHA256: sha256SumBytes(truncated),
	}})
	require.NoError(t, err)

	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.Error(t, err, "a truncated stream must fail, got nil")
	require.NotErrorIs(t, err, craft.ErrUnsupported)

	// No member row, no member object: nothing a Run could resolve.
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Len(t, files.blobs, 1)
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{
		RequestID: "r1", Prompt: "build", InputRefs: []string{"craft-test://craft_input_phantom_member.txt"},
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput, "no phantom expanded material may be usable by a Run")
}

// TestCraftArchiveQuotaBound proves the round caps hold at the service seam:
// expanded content cannot bypass the per-file, count or 100 MiB round limits.
func TestCraftArchiveQuotaBound(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-quota", "Quota archive", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	uploadArchive := func(name string, data []byte) craft.Input {
		uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
			Name: name, Content: data, SHA256: sha256SumBytes(data),
		}})
		require.NoError(t, err)
		return uploaded[0]
	}

	chunk := make([]byte, craft.MaxInputBytes+1)
	oversize := buildZipEntries(t, []string{"huge.bin"}, func(string) []byte { return chunk })
	input := uploadArchive("oversize.zip", oversize)
	_, err := env.svc.ExpandArchive(ctx, scope, input.Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput, "a member over the per-input cap must refuse the expansion")

	names := make([]string, 0, craft.MaxInputsPerRound+1)
	for i := 0; i < craft.MaxInputsPerRound+1; i++ {
		names = append(names, "f"+hex.EncodeToString([]byte{byte(i)})+".txt")
	}
	tooMany := buildZipEntries(t, names, func(string) []byte { return []byte("x") })
	input = uploadArchive("many.zip", tooMany)
	_, err = env.svc.ExpandArchive(ctx, scope, input.Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput, "more members than the per-round count cap must be refused")

	part := make([]byte, 18<<20)
	cumulativeNames := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		cumulativeNames = append(cumulativeNames, "part"+string(rune('a'+i))+".bin")
	}
	cumulative := buildZipEntries(t, cumulativeNames, func(string) []byte { return part })
	require.Less(t, int64(len(cumulative)), int64(craft.MaxInputBytes), "the archive itself stays under the input cap")
	input = uploadArchive("cumulative.zip", cumulative)
	_, err = env.svc.ExpandArchive(ctx, scope, input.Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput, "expanded content cannot bypass the 100 MiB per-round limit")

	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 3, "only the three refused archives remain; nothing expanded")
	require.Len(t, files.blobs, 3, "no member object from any refused expansion")
}

// TestCraftArchiveRejectsNonArchiveInput refuses bytes that are not a
// supported archive instead of guessing.
func TestCraftArchiveRejectsNonArchiveInput(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t02-format", "Format", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	plain := []byte("just text wearing a zip extension")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "fake.zip", Content: plain, SHA256: sha256SumBytes(plain),
	}})
	require.NoError(t, err)
	// Non-archive bytes are a deterministic rejection of the caller's own
	// content: request-scoped 400 (ErrInvalidInput), never the 503
	// (ErrUnsupported) that would promise a retryable server dependency.
	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.NotErrorIs(t, err, craft.ErrUnsupported)
}

// TestCraftArchivePublishRollbackOnStoreFailure proves the publish phase is
// compensated: when the store dies after one member object was saved, no
// partial manifest row survives and the orphaned object is removed.
func TestCraftArchivePublishRollbackOnStoreFailure(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	base := &craftT01Files{blobs: map[string][]byte{}}
	files := &craftT02FailingFiles{craftT01Files: base, failOnSave: 3}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-rollback", "Rollback", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	archiveBytes := loadCraftArchiveFixture(t, "valid.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "sales-pack.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)

	// Save order: upload archive (1), member data/meta.json (2, saved),
	// member data/sales.csv (3, fails) — the saved member object must be
	// rolled back with no row surviving.
	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.ErrorContains(t, err, "injected store failure")

	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1, "no member row may survive a failed publish")
	require.Equal(t, uploaded[0].Ref, persisted[0].Ref)
	require.Contains(t, files.blobs, uploaded[0].Ref)
	require.Len(t, files.blobs, 1, "orphaned member objects must be cleaned up")
}

// TestCraftArchiveHonorsCancelledContext proves the extraction honors the
// caller's deadline and fails without materializing anything.
func TestCraftArchiveHonorsCancelledContext(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-cancel", "Cancel", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	archiveBytes := loadCraftArchiveFixture(t, "valid.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "sales-pack.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = env.svc.ExpandArchive(cancelled, scope, uploaded[0].Ref)
	require.Error(t, err, "a cancelled context must abort the expansion")
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Len(t, files.blobs, 1)
}
