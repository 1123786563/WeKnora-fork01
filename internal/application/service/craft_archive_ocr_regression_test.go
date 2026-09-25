package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// craftT02NondeterministicFiles mimics the production object backends: every
// SaveBytes mints a brand-new object key (timestamp/uuid style), so refs are
// NOT content-addressed. It can fail a chosen save while canceling its own
// context (the production failure mode behind the extract budget), and it
// refuses DeleteFile on an already-canceled context exactly like real
// storage backends do.
type craftT02NondeterministicFiles struct {
	interfaces.FileService
	blobs   map[string][]byte
	deleted []string
	saves   int

	failOnSave      int
	cancelOnFailure context.CancelFunc

	refuseDeleteOnCanceledContext bool
	deleteRefusals                int
}

func (f *craftT02NondeterministicFiles) SaveBytes(ctx context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	f.saves++
	if f.failOnSave != 0 && f.saves == f.failOnSave {
		if f.cancelOnFailure != nil {
			f.cancelOnFailure()
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("injected store failure")
	}
	ref := fmt.Sprintf("craft-nondet://%d/%s", f.saves, name)
	f.blobs[ref] = append([]byte(nil), data...)
	return ref, nil
}

func (f *craftT02NondeterministicFiles) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.blobs[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *craftT02NondeterministicFiles) DeleteFile(ctx context.Context, ref string) error {
	if f.refuseDeleteOnCanceledContext && ctx.Err() != nil {
		f.deleteRefusals++
		return ctx.Err()
	}
	delete(f.blobs, ref)
	f.deleted = append(f.deleted, ref)
	return nil
}

// TestCraftArchiveReplayIsIdempotentWithNondeterministicObjectRefs is the OCR
// high-finding regression: with production-style non-content-addressed
// object refs, replaying an expansion must return the identical manifest
// (same original refs), upload nothing new and add no rows.
func TestCraftArchiveReplayIsIdempotentWithNondeterministicObjectRefs(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT02NondeterministicFiles{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-ocr-replay", "Archive input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	archiveBytes := loadCraftArchiveFixture(t, "valid.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "pack.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)

	first, err := env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.NoError(t, err)
	require.Len(t, first, 3)
	savesAfterFirst := files.saves // 1 archive upload + 3 member objects

	replayed, err := env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.NoError(t, err)
	require.Equal(t, first, replayed, "replay must return the identical manifest including the original refs")
	require.Equal(t, savesAfterFirst, files.saves, "replay must not mint new objects")
	require.Empty(t, files.deleted, "replay must not delete anything either")

	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, persisted, 4, "archive plus three members; replay adds no rows")
}

// TestCraftArchiveRollbackSurvivesCanceledRequestContext is the OCR
// medium-finding regression: when the publish phase fails because the
// request context was canceled (the 30s extract budget firing mid-upload),
// the rollback must still delete the already-stored objects on a detached
// context instead of failing every cleanup call and leaking them.
func TestCraftArchiveRollbackSurvivesCanceledRequestContext(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT02NondeterministicFiles{
		blobs:                        map[string][]byte{},
		refuseDeleteOnCanceledContext: true,
	}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-ocr-rollback", "Archive input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	craftBase := context.WithValue(context.WithValue(context.Background(),
		types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	ctx, cancel := context.WithCancel(craftBase)
	files.failOnSave = 3 // save 1 = archive upload; member objects start at 2
	files.cancelOnFailure = cancel

	archiveBytes := loadCraftArchiveFixture(t, "valid.zip")
	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "pack.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)

	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.Error(t, err)
	cancel()

	require.Zero(t, files.deleteRefusals, "cleanup must not run on the canceled request context")
	require.Len(t, files.deleted, 1, "the one stored member object must be rolled back")
	for ref := range files.blobs {
		require.Equal(t, uploaded[0].Ref, ref, "only the still-associated archive object may remain")
	}
	persisted, err := env.svc.WorkspaceInputs(craftCtx(1, "u1", ws.SessionID), scope)
	require.NoError(t, err)
	require.Len(t, persisted, 1, "only the archive input remains; no partial manifest")
}

// TestCraftArchiveRejectsEmptyMemberWithRootCauseError is the OCR low-finding
// regression: an archive holding an empty member (a bare __init__.py) is
// rejected up front with the root cause, instead of an opaque per-name
// "no declared size" from the shared manifest gate.
func TestCraftArchiveRejectsEmptyMemberWithRootCauseError(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT02NondeterministicFiles{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t02-ocr-empty", "Archive input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	empty, err := zw.Create("__init__.py")
	require.NoError(t, err)
	_, err = empty.Write(nil)
	require.NoError(t, err)
	main, err := zw.Create("main.py")
	require.NoError(t, err)
	_, err = main.Write([]byte("print('hi')"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	archiveBytes := zipBuf.Bytes()

	uploaded, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "pkg.zip", Content: archiveBytes, SHA256: sha256SumBytes(archiveBytes),
	}})
	require.NoError(t, err)

	_, err = env.svc.ExpandArchive(ctx, scope, uploaded[0].Ref)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Contains(t, err.Error(), "empty member", "the error must name the root cause, not a per-name size complaint")
	require.Empty(t, files.deleted, "nothing was uploaded before the rejection")
}
