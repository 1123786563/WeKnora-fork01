package service

import (
	"context"
	stderrors "errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Test doubles
// -----------------------------------------------------------------------------

// dirBackedFileService is the real-storage stand-in: unlike a map-only fake it
// performs actual disk I/O for every SaveBytes/GetFile, so a test that reads a
// version's files back through its refs compares real persisted bytes, not
// recorded calls. failFrom > 0 makes the Nth save onwards fail so upload
// failures are exercisable; the boundary against the production object store
// is interfaces.FileService, exercised for real in the repository layer.
type dirBackedFileService struct {
	root     string
	failFrom int
	saves    int
}

func newDirBackedFileService(t *testing.T) *dirBackedFileService {
	t.Helper()
	return &dirBackedFileService{root: t.TempDir()}
}

func (f *dirBackedFileService) CheckConnectivity(context.Context) error { return nil }

func (f *dirBackedFileService) SaveFile(context.Context, *multipart.FileHeader, uint64, string) (string, error) {
	panic("SaveFile should not be called by CraftArtifactService")
}

func (f *dirBackedFileService) SaveBytes(_ context.Context, data []byte, _ uint64, fileName string, _ bool) (string, error) {
	f.saves++
	if f.failFrom > 0 && f.saves >= f.failFrom {
		return "", stderrors.New("storage unavailable")
	}
	p := filepath.Join(f.root, fileName)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return "", err
	}
	return "file://" + p, nil
}

func (f *dirBackedFileService) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	return os.Open(strings.TrimPrefix(ref, "file://"))
}

func (f *dirBackedFileService) GetFileURL(context.Context, string) (string, error) {
	panic("GetFileURL should not be called by CraftArtifactService")
}

func (f *dirBackedFileService) DeleteFile(context.Context, string) error {
	panic("DeleteFile should not be called by CraftArtifactService")
}

func (f *dirBackedFileService) CopyFile(context.Context, string, uint64, string) (string, error) {
	panic("CopyFile should not be called by CraftArtifactService")
}

// readRef loads one stored object back through the storage boundary.
func (f *dirBackedFileService) readRef(t *testing.T, ref string) []byte {
	t.Helper()
	rc, err := f.GetFile(context.Background(), ref)
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return data
}

// memVersionStore is the in-memory craft.VersionStore stand-in. It mirrors
// the real store's identity semantics (derived ids, idempotent identical
// publishes); the real GORM store and its scope ACL are exercised against
// real migrations in the repository package tests.
type memVersionStore struct {
	mu        sync.Mutex
	byID      map[string]craft.Version
	order     []string
	publishes int
	// failPublish injects a publish-transaction failure (CFT-S03-T019):
	// uploads already succeeded, so staging exists but no version lands.
	failPublish bool
}

func newMemVersionStore() *memVersionStore {
	return &memVersionStore{byID: map[string]craft.Version{}}
}

func (m *memVersionStore) Publish(_ context.Context, _ craft.Scope, v craft.Version) (craft.Version, error) {
	if m.failPublish {
		return craft.Version{}, stderrors.New("publish transaction failed (injected)")
	}
	digest, err := craft.ManifestDigest(v.Files)
	if err != nil {
		return craft.Version{}, err
	}
	id := craft.VersionID(v.WorkspaceID, v.RunID, digest)
	if v.ID != "" && v.ID != id {
		return craft.Version{}, stderrors.New("id mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if stored, ok := m.byID[id]; ok {
		return stored, nil
	}
	v.ID = id
	m.byID[id] = v
	m.order = append(m.order, id)
	m.publishes++
	return v, nil
}

func (m *memVersionStore) List(_ context.Context, _ craft.Scope) ([]craft.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]craft.Version, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		out = append(out, m.byID[m.order[i]])
	}
	return out, nil
}

func (m *memVersionStore) Get(_ context.Context, _ craft.Scope, id string) (craft.Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.byID[id]; ok {
		return v, nil
	}
	return craft.Version{}, craft.ErrNotFound
}

// -----------------------------------------------------------------------------
// Fixtures
// -----------------------------------------------------------------------------

const craftTestOutputDir = "/workspace/output"

// sameModTime pins one mtime for every entry: identity must come from
// content, never from (path, mtime) pairs.
var sameModTime = time.Unix(1700000000, 0).UTC()

func craftArtifactTask(sessionID, workspaceID, runID string) craft.Task {
	return craft.Task{
		ID: "dlg_" + runID, ToolCallID: "call_" + runID,
		Prompt: "make a site", RequestHash: "rh-" + runID,
		Scope:       craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID},
		Fence:       agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: runID}, Owner: "worker", Epoch: 1},
		WorkspaceID: workspaceID,
	}
}

func craftEntry(t *testing.T, rel, content string) sandbox.RemoteDirEntry {
	t.Helper()
	return sandbox.RemoteDirEntry{
		Name: filepath.Base(rel), Path: craftTestOutputDir + "/" + rel,
		Type: sandbox.RemoteEntryFile, Size: int64(len(content)), ModTime: sameModTime,
	}
}

func craftSourceWith(entries []sandbox.RemoteDirEntry, contents map[string][]byte) *fakeSandboxSource {
	return &fakeSandboxSource{
		entries:  map[string][]sandbox.RemoteDirEntry{"s1": entries},
		contents: contents,
	}
}

func newCraftArtifactService(source *fakeSandboxSource, files *dirBackedFileService, versions *memVersionStore) *CraftArtifactService {
	return NewCraftArtifactService(source, files, versions, nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

// TestCraftArtifactCollectPinsImmutableVersions pins the user result: a
// version, once published, keeps serving its original bytes. The second
// round rewrites content under the same path and the SAME mtime — the new
// content still becomes a new version (never hidden inside the old one), and
// reading the old version's refs through the storage boundary returns the
// original bytes.
func TestCraftArtifactCollectPinsImmutableVersions(t *testing.T) {
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	v1HTML := []byte("<h1>round one</h1>")
	v1JS := []byte("console.log(1)")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "index.html", string(v1HTML)), craftEntry(t, "assets/app.js", string(v1JS))},
		map[string][]byte{
			craftTestOutputDir + "/index.html":    v1HTML,
			craftTestOutputDir + "/assets/app.js": v1JS,
		},
	)
	svc := newCraftArtifactService(source, files, versions)
	task := craftArtifactTask("s1", "ws-1", "run-1")

	v1, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	require.Len(t, v1.Files, 2)
	require.Equal(t, "assets/app.js", v1.Files[0].Path, "manifest sorted by path")
	require.Equal(t, "index.html", v1.Files[1].Path)
	require.Equal(t, "text/html; charset=utf-8", v1.Files[1].MIME)
	digest, err := craft.ManifestDigest(v1.Files)
	require.NoError(t, err)
	require.Equal(t, craft.VersionID("ws-1", "run-1", digest), v1.ID)

	// Second round: same paths, same mtime, different content.
	v2HTML := []byte("<h1>round two with more words</h1>")
	source.contents[craftTestOutputDir+"/index.html"] = v2HTML
	v2, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	require.NotEqual(t, v1.ID, v2.ID, "same path+mtime with new content must not hide inside the old version")

	// The old version still serves its original bytes through its own refs.
	got, err := versions.Get(context.Background(), task.Scope, v1.ID)
	require.NoError(t, err)
	for _, f := range got.Files {
		switch f.Path {
		case "index.html":
			require.Equal(t, v1HTML, files.readRef(t, f.Ref), "old download changed")
		case "assets/app.js":
			require.Equal(t, v1JS, files.readRef(t, f.Ref), "old download changed")
		default:
			t.Fatalf("unexpected file %q", f.Path)
		}
		require.NotEmpty(t, f.SHA256)
	}
}

// TestCraftArtifactCollectIsIdempotentForSameContent pins the retry rule:
// replaying the identical collection answers the same version id and does
// not grow the version list.
func TestCraftArtifactCollectIsIdempotentForSameContent(t *testing.T) {
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	content := []byte("<h1>stable</h1>")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "index.html", string(content))},
		map[string][]byte{craftTestOutputDir + "/index.html": content},
	)
	svc := newCraftArtifactService(source, files, versions)
	task := craftArtifactTask("s1", "ws-1", "run-1")

	first, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	second, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	list, err := versions.List(context.Background(), task.Scope)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

// TestCraftArtifactCollectUploadFailureLeavesNoVersion pins the publish
// order: objects upload first, the version row publishes last, so a failed
// upload leaves no visible version behind.
func TestCraftArtifactCollectUploadFailureLeavesNoVersion(t *testing.T) {
	files := newDirBackedFileService(t)
	files.failFrom = 1
	versions := newMemVersionStore()
	content := []byte("<h1>broken round</h1>")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "index.html", string(content))},
		map[string][]byte{craftTestOutputDir + "/index.html": content},
	)
	svc := newCraftArtifactService(source, files, versions)

	_, err := svc.Collect(context.Background(), craftArtifactTask("s1", "ws-1", "run-1"))
	require.Error(t, err)
	require.Equal(t, 0, versions.publishes, "no version may become visible on upload failure")
	list, err := versions.List(context.Background(), craftArtifactTask("s1", "ws-1", "run-1").Scope)
	require.NoError(t, err)
	require.Empty(t, list)
}

// TestCraftArtifactCollectRejectsHostileOutput pins the collection rules:
// traversal, symlinks, oversize and credential-looking files abort the whole
// collection before any version exists — a version manifest is complete or
// nothing.
func TestCraftArtifactCollectRejectsHostileOutput(t *testing.T) {
	task := craftArtifactTask("s1", "ws-1", "run-1")
	cases := map[string]struct {
		entries  []sandbox.RemoteDirEntry
		contents map[string][]byte
		config   CraftArtifactConfig
	}{
		"traversal": {
			entries: []sandbox.RemoteDirEntry{{
				Name: "secret.txt", Path: "/workspace/output/../secret.txt",
				Type: sandbox.RemoteEntryFile, Size: 2, ModTime: sameModTime,
			}},
			contents: map[string][]byte{"/workspace/output/../secret.txt": []byte("x")},
		},
		"credential file": {
			entries:  []sandbox.RemoteDirEntry{craftEntry(t, ".env", "KEY=1")},
			contents: map[string][]byte{craftTestOutputDir + "/.env": []byte("KEY=1")},
		},
		"symlink entry": {
			entries: []sandbox.RemoteDirEntry{{
				Name: "link", Path: craftTestOutputDir + "/link",
				Type: sandbox.RemoteEntryOther, Size: 1, ModTime: sameModTime,
			}},
			contents: map[string][]byte{craftTestOutputDir + "/link": []byte("x")},
		},
		"oversize file": {
			entries:  []sandbox.RemoteDirEntry{craftEntry(t, "index.html", "<h1>big</h1>")},
			contents: map[string][]byte{craftTestOutputDir + "/index.html": []byte("<h1>big</h1>")},
			config:   CraftArtifactConfig{OutputDir: craftTestOutputDir, MaxFileBytes: 3},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			files := newDirBackedFileService(t)
			versions := newMemVersionStore()
			cfg := tc.config
			if cfg.OutputDir == "" {
				cfg = CraftArtifactConfig{OutputDir: craftTestOutputDir}
			}
			svc := NewCraftArtifactService(craftSourceWith(tc.entries, tc.contents), files, versions, nil, cfg)
			_, err := svc.Collect(context.Background(), task)
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			require.Zero(t, versions.publishes)
		})
	}
}

// TestCraftArtifactCollectChecksAreHonestFacts pins the verification-fact
// rules on the published version: without evidence build and preview stay
// not_run (W02 owns the preview fact), the entry check passes on the real
// manifest, and a claimed success without the entry file publishes a failed
// entry check.
func TestCraftArtifactCollectChecksAreHonestFacts(t *testing.T) {
	task := craftArtifactTask("s1", "ws-1", "run-1")

	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	content := []byte("<h1>v</h1>")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "index.html", string(content))},
		map[string][]byte{craftTestOutputDir + "/index.html": content},
	)
	svc := NewCraftArtifactService(source, files, versions, nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	v, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	byName := map[string]craft.Check{}
	for _, c := range v.Checks {
		byName[c.Name] = c
	}
	require.Equal(t, craft.CheckPassed, byName[craft.CheckEntry].Status)
	require.Equal(t, craft.CheckNotRun, byName[craft.CheckBuild].Status)
	require.Equal(t, craft.CheckNotRun, byName[craft.CheckPreview].Status, "W01 never records preview passed")

	// Claimed success, nothing produced: the entry check fails.
	files2 := newDirBackedFileService(t)
	versions2 := newMemVersionStore()
	svc2 := NewCraftArtifactService(
		craftSourceWith(nil, nil), files2, versions2,
		func(context.Context, craft.Task) craft.ArtifactEvidence {
			return craft.ArtifactEvidence{ClaimedSuccess: true}
		},
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	)
	v2, err := svc2.Collect(context.Background(), task)
	require.NoError(t, err)
	byName2 := map[string]craft.Check{}
	for _, c := range v2.Checks {
		byName2[c.Name] = c
	}
	require.Equal(t, craft.CheckFailed, byName2[craft.CheckEntry].Status)
	require.Equal(t, craft.CheckNotRun, byName2[craft.CheckPreview].Status)
}

// TestCraftArtifactCollectValidatesTask pins the input gate: an incomplete
// delegation request never reaches the sandbox source or the store.
func TestCraftArtifactCollectValidatesTask(t *testing.T) {
	svc := newCraftArtifactService(
		craftSourceWith(nil, nil), newDirBackedFileService(t), newMemVersionStore())
	for _, task := range []craft.Task{
		{},
		{Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}},
		{Scope: craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}, WorkspaceID: "ws-1"},
	} {
		_, err := svc.Collect(context.Background(), task)
		require.ErrorIs(t, err, craft.ErrInvalidInput, "task %+v", task)
	}
}
