package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type captureSource struct {
	*fakeSandboxSource
	run, gen      string
	proof         error
	proofs, lists int
}

type captureResourceFileService struct {
	interfaces.FileService
	db      *gorm.DB
	inner   *dirBackedFileService
	refs    map[string]string
	nextRef int
}

func newCaptureResourceFileService(t *testing.T, db *gorm.DB) *captureResourceFileService {
	t.Helper()
	return &captureResourceFileService{db: db, inner: newDirBackedFileService(t), refs: map[string]string{}}
}

func (f *captureResourceFileService) SaveBytes(ctx context.Context, data []byte, tenantID uint64, name string, temp bool) (string, error) {
	physical, err := f.inner.SaveBytes(ctx, data, tenantID, name, temp)
	if err != nil {
		return "", err
	}
	f.nextRef++
	handle := fmt.Sprintf("%022d", f.nextRef)
	ref := types.BuildResourcePath(handle)
	sha := sha256.Sum256(data)
	location := sha256.Sum256([]byte(physical))
	if err := f.db.Exec(`INSERT INTO resources (id,handle,tenant_id,provider,physical_path,location_hash,size,content_hash,state)
		VALUES (?,?,?,?,?,?,?,?,?)`, fmt.Sprintf("capture-resource-%d", f.nextRef), handle, tenantID, "test", physical,
		hex.EncodeToString(location[:]), len(data), hex.EncodeToString(sha[:]), types.ResourceStateActive).Error; err != nil {
		return "", err
	}
	f.refs[ref] = physical
	return ref, nil
}

func (f *captureResourceFileService) GetFile(ctx context.Context, ref string) (io.ReadCloser, error) {
	physical, ok := f.refs[ref]
	if !ok {
		return nil, craft.ErrNotFound
	}
	return f.inner.GetFile(ctx, physical)
}

type failOnceDraftAdvance struct {
	craft.DraftHeadStore
	err error
}

func (s *failOnceDraftAdvance) Advance(ctx context.Context, scope craft.Scope, workspaceID string, expectedRevision int64, sourceRunID string, files []craft.File) (craft.DraftHead, error) {
	if s.err != nil {
		err := s.err
		s.err = nil
		return craft.DraftHead{}, err
	}
	return s.DraftHeadStore.Advance(ctx, scope, workspaceID, expectedRevision, sourceRunID, files)
}

func (s *captureSource) CraftArtifactRunID() string      { return s.run }
func (s *captureSource) CraftArtifactGeneration() string { return s.gen }
func (s *captureSource) VerifyCraftCaptureQuiescent(context.Context) error {
	s.proofs++
	return s.proof
}
func (s *captureSource) ListSessionFiles(ctx context.Context, session, dir string) ([]sandbox.RemoteDirEntry, error) {
	s.lists++
	return s.fakeSandboxSource.ListSessionFiles(ctx, session, dir)
}

type memoryCaptureStore struct {
	receipt   repository.CraftRunCapture
	seals     int
	marks     int
	markErr   error
	sealErr   error
	verifyErr error
}

func (s *memoryCaptureStore) EnsurePending(_ context.Context, scope craft.Scope, workspace, run, gen string) (repository.CraftRunCapture, error) {
	if s.receipt.RunID == "" {
		s.receipt = repository.CraftRunCapture{Scope: scope, WorkspaceID: workspace, RunID: run, Generation: gen, PredecessorState: craft.DraftHeadEmpty, State: "pending"}
	}
	return s.receipt, nil
}
func (s *memoryCaptureStore) RecoverPending(context.Context, int) ([]repository.CraftRunCapture, error) {
	if s.receipt.State == "" || s.receipt.State == "advanced" {
		return nil, nil
	}
	return []repository.CraftRunCapture{s.receipt}, nil
}
func (s *memoryCaptureStore) ClaimForDrain(_ context.Context, receipt repository.CraftRunCapture) (repository.CraftRunCapture, error) {
	if s.receipt.RunID == receipt.RunID {
		s.receipt.State = "capturing"
		return s.receipt, nil
	}
	return receipt, nil
}
func (s *memoryCaptureStore) RecoverPendingTick(ctx context.Context, limit int) ([]repository.CraftRunCapture, error) {
	return s.RecoverPending(ctx, limit)
}
func (s *memoryCaptureStore) RecoverPendingForRun(_ context.Context, tenantID uint64, runID string) ([]repository.CraftRunCapture, error) {
	if s.receipt.State == "" || s.receipt.State == "advanced" ||
		s.receipt.Scope.TenantID != tenantID || s.receipt.RunID != runID {
		return nil, nil
	}
	return []repository.CraftRunCapture{s.receipt}, nil
}
func (s *memoryCaptureStore) BeginCapture(_ context.Context, r repository.CraftRunCapture, digest string) (repository.CraftRunCapture, error) {
	if r.ManifestDigest != "" && r.ManifestDigest != digest {
		return repository.CraftRunCapture{}, craft.ErrConflict
	}
	r.ManifestDigest = digest
	r.State = "capturing"
	s.receipt = r
	return r, nil
}
func (s *memoryCaptureStore) Seal(_ context.Context, r repository.CraftRunCapture, files []craft.File, digest string) (repository.CraftRunCapture, error) {
	s.seals++
	if s.sealErr != nil {
		return repository.CraftRunCapture{}, s.sealErr
	}
	r.Files = append([]craft.File(nil), files...)
	r.ManifestDigest = digest
	r.State = "sealed"
	s.receipt = r
	return r, nil
}
func (s *memoryCaptureStore) MarkAdvanced(_ context.Context, r repository.CraftRunCapture, revision int64) error {
	s.marks++
	if s.markErr != nil {
		return s.markErr
	}
	r.State = "advanced"
	r.DraftRevision = &revision
	s.receipt = r
	return nil
}
func (s *memoryCaptureStore) MarkPendingError(context.Context, repository.CraftRunCapture, error) error {
	return nil
}
func (s *memoryCaptureStore) VerifySealedRefs(context.Context, repository.CraftRunCapture) error {
	return s.verifyErr
}

type memoryCaptureDrafts struct {
	head       craft.DraftHead
	revisions  map[int64]craft.DraftHead
	advanceErr error
	advances   int
}

func (s *memoryCaptureDrafts) Read(context.Context, craft.Scope, string) (craft.DraftHead, error) {
	return s.head, nil
}
func (s *memoryCaptureDrafts) ReadRevision(_ context.Context, _ craft.Scope, workspace string, revision int64) (craft.DraftHead, error) {
	if h, ok := s.revisions[revision]; ok {
		return h, nil
	}
	if revision == 0 {
		return craft.DraftHead{WorkspaceID: workspace, Revision: 0, State: craft.DraftHeadEmpty}, nil
	}
	return craft.DraftHead{}, craft.ErrNotFound
}
func (s *memoryCaptureDrafts) Advance(_ context.Context, _ craft.Scope, workspace string, revision int64, run string, files []craft.File) (craft.DraftHead, error) {
	s.advances++
	if s.advanceErr != nil {
		return craft.DraftHead{}, s.advanceErr
	}
	digest, _ := craft.ManifestDigest(files)
	head := craft.DraftHead{WorkspaceID: workspace, Revision: revision + 1, State: craft.DraftHeadSelected, SourceRunID: run, ManifestDigest: digest, Files: files}
	s.head = head
	if s.revisions == nil {
		s.revisions = map[int64]craft.DraftHead{}
	}
	s.revisions[head.Revision] = head
	return head, nil
}

func TestCaptureQuiescentDraftFailsClosedWithoutQuiescenceProof(t *testing.T) {
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: "run-b", gen: "gen-b", proof: errors.New("writer may remain")}
	files := newDirBackedFileService(t)
	captures := &memoryCaptureStore{}
	drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}}
	artifacts := NewCraftArtifactService(source, files, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	_, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-b", craft.KindWeb, source)
	require.ErrorIs(t, err, craft.ErrBusy)
	require.Equal(t, 1, source.proofs)
	require.Zero(t, source.lists)
	require.Zero(t, files.saves)
	require.Equal(t, int64(0), drafts.head.Revision)
	require.Zero(t, drafts.advances)
	// The drain CLAIMS the receipt (pending→capturing) before quiescence so
	// the freshness gate covers the pre-staging phase; a failed proof leaves
	// it claimed for the periodic scan to retry.
	require.Equal(t, "capturing", captures.receipt.State)
}

func TestCaptureQuiescentTerminalDraftAdvancesWithoutPublishingVersion(t *testing.T) {
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 3}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("D2!")}), run: "run-failed", gen: "gen-failed"}
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	captures := &memoryCaptureStore{}
	drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}}
	artifacts := NewCraftArtifactService(source, files, versions, nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	head, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-failed", craft.KindWeb, source)
	require.NoError(t, err)
	require.EqualValues(t, 1, head.Revision)
	require.Equal(t, "run-failed", head.SourceRunID)
	require.Equal(t, "advanced", captures.receipt.State)
	require.Zero(t, versions.publishes, "draft capture cannot publish or replace a Version")
}

func TestCaptureReplaysSealedRefsAfterAdvanceReceiptFailure(t *testing.T) {
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: "run-b", gen: "gen-b"}
	files := newDirBackedFileService(t)
	captures := &memoryCaptureStore{markErr: errors.New("receipt write failed")}
	drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}}
	artifacts := NewCraftArtifactService(source, files, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	_, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-b", craft.KindWeb, source)
	require.Error(t, err)
	require.Equal(t, int64(1), drafts.head.Revision)
	require.Equal(t, "sealed", captures.receipt.State)
	// Filesystem bytes mutate after the first Advance. Recovery must use the
	// sealed object refs and recognize the already committed revision exactly.
	source.contents[entry.Path] = []byte("y")
	drafts.advanceErr = craft.ErrConflict
	captures.markErr = nil
	_, err = svc.capture(context.Background(), captures.receipt, craft.KindWeb, source)
	require.NoError(t, err)
	require.Equal(t, "advanced", captures.receipt.State)
	require.Equal(t, int64(1), drafts.head.Revision)
	require.Equal(t, 1, captures.seals)
}

func TestCaptureDirectTerminalRetryUsesSealedRefsWithoutReadingChangedSource(t *testing.T) {
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: "run-direct-retry", gen: "gen-direct-retry"}
	files := newDirBackedFileService(t)
	captures := &memoryCaptureStore{}
	drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}, advanceErr: errors.New("stop after seal before draft CAS")}
	artifacts := NewCraftArtifactService(source, files, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	_, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-direct-retry", craft.KindWeb, source)
	require.Error(t, err)
	require.Equal(t, "sealed", captures.receipt.State)
	sealedFiles := append([]craft.File(nil), captures.receipt.Files...)
	require.Len(t, sealedFiles, 1)

	// The next invocation enters through CaptureTerminal/EnsurePending, as a
	// direct terminal retry would, after the source's mutable bytes have changed.
	source.contents[entry.Path] = []byte("y")
	drafts.advanceErr = nil
	head, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-direct-retry", craft.KindWeb, source)
	require.NoError(t, err)
	require.EqualValues(t, 1, head.Revision)
	require.Equal(t, sealedFiles, head.Files)
	require.EqualValues(t, 1, source.lists, "sealed retry must not re-list or read the mutable source")
	require.EqualValues(t, 1, files.saves, "sealed retry must not upload a replacement object")
	require.Equal(t, "advanced", captures.receipt.State)
}

func TestCaptureSQLiteSealedRetry(t *testing.T) {
	ctx := durableRunCtx()
	db := openDurableRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s1',1,'web')`).Error)
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	_, err := repository.NewCraftStore(db).PutWorkspace(ctx, craft.Workspace{Scope: scope, SandboxID: "capture-replay-sandbox"}, 0)
	require.NoError(t, err)
	workspace, err := repository.NewCraftStore(db).GetWorkspace(ctx, scope)
	require.NoError(t, err)
	snapshot, err := BuildDurableCraftRunSnapshot("capture terminal output", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{})
	require.NoError(t, err)
	runs := repository.NewAgentRunStore(db)
	runKey := admitDurableCraftRunAsActor(t, runs, snapshot, "u1")
	views := repository.NewCraftRunViewStore(db)
	viewKey := craft.RunViewKey{TenantID: scope.TenantID, OwnerID: scope.UserID, SessionID: scope.SessionID, RunID: runKey.RunID}
	view, err := views.Allocate(ctx, viewKey)
	require.NoError(t, err)
	view, _, err = views.BeginSessionCreate(ctx, viewKey, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, viewKey, view.Generation, craft.RunViewRuntime{RuntimeID: "capture-replay-runtime", ContainerID: "capture-replay-container", OpenCodeSessionID: "capture-replay-oc"})
	require.NoError(t, err)
	require.NoError(t, runs.CancelRun(ctx, runKey, "terminal"))

	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: runKey.RunID, gen: view.Generation}
	objectFiles := newCaptureResourceFileService(t, db)
	artifacts := NewCraftArtifactService(source, objectFiles, repository.NewCraftVersionStore(db), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	realDrafts := repository.NewCraftDraftHeadStore(db)
	drafts := &failOnceDraftAdvance{DraftHeadStore: realDrafts, err: errors.New("crash after seal before draft CAS")}
	captures := repository.NewCraftRunCaptureStore(db)
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	_, err = svc.CaptureTerminal(ctx, scope, workspace.ID, runKey.RunID, craft.KindWeb, source)
	require.Error(t, err)
	var persisted repository.CraftRunCapture
	rows, err := captures.RecoverPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	persisted = rows[0]
	require.Equal(t, "sealed", persisted.State)
	require.Len(t, persisted.Files, 1)

	source.contents[entry.Path] = []byte("y")
	head, err := svc.CaptureTerminal(ctx, scope, workspace.ID, runKey.RunID, craft.KindWeb, source)
	require.NoError(t, err)
	require.EqualValues(t, 1, head.Revision)
	stored, err := objectFiles.GetFile(ctx, head.Files[0].Ref)
	require.NoError(t, err)
	storedBytes, err := io.ReadAll(stored)
	require.NoError(t, err)
	require.NoError(t, stored.Close())
	require.Equal(t, "x", string(storedBytes))
	require.Equal(t, persisted.Files, head.Files, "the durable sealed refs, not changed source bytes, become the draft")
	require.EqualValues(t, 1, source.lists)
	require.EqualValues(t, 1, objectFiles.inner.saves)
	final, err := captures.EnsurePending(ctx, scope, workspace.ID, runKey.RunID, view.Generation)
	require.NoError(t, err)
	require.Equal(t, "advanced", final.State)
}

func TestCaptureSealedRetryFailsClosedForUnavailableOrInvalidRefs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(t *testing.T, receipt repository.CraftRunCapture)
		verifyErr error
	}{
		{name: "missing object", mutate: func(t *testing.T, receipt repository.CraftRunCapture) {
			require.NoError(t, os.Remove(strings.TrimPrefix(receipt.Files[0].Ref, "file://")))
		}},
		{name: "corrupt bytes", mutate: func(t *testing.T, receipt repository.CraftRunCapture) {
			require.NoError(t, os.WriteFile(strings.TrimPrefix(receipt.Files[0].Ref, "file://"), []byte("y"), 0o600))
		}},
		{name: "wrong size", mutate: func(t *testing.T, receipt repository.CraftRunCapture) {
			require.NoError(t, os.WriteFile(strings.TrimPrefix(receipt.Files[0].Ref, "file://"), []byte("xy"), 0o600))
		}},
		{name: "wrong tenant object ref", verifyErr: craft.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
			entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
			source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: "run-invalid-ref", gen: "gen-invalid-ref"}
			files := newDirBackedFileService(t)
			captures := &memoryCaptureStore{}
			drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}, advanceErr: errors.New("stop after seal before draft CAS")}
			artifacts := NewCraftArtifactService(source, files, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
			svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
			_, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-invalid-ref", craft.KindWeb, source)
			require.Error(t, err)
			require.Equal(t, "sealed", captures.receipt.State)
			captures.verifyErr = tc.verifyErr
			if tc.mutate != nil {
				tc.mutate(t, captures.receipt)
			}
			drafts.advanceErr = nil
			_, err = svc.CaptureTerminal(context.Background(), scope, "ws", "run-invalid-ref", craft.KindWeb, source)
			require.Error(t, err, "unavailable, corrupt or cross-tenant objects cannot advance the draft")
			require.EqualValues(t, 0, drafts.head.Revision)
			require.EqualValues(t, 1, drafts.advances, "verification must reject before a second Advance")
			require.Equal(t, "sealed", captures.receipt.State, "failed verification leaves the receipt unresolved")
			require.EqualValues(t, 1, files.saves, "sealed retry must not upload replacement bytes")
		})
	}
}

func TestCaptureConflictsOnChangedBytesAfterUploadBeforeSealCrash(t *testing.T) {
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "s1"}
	entry := sandbox.RemoteDirEntry{Name: "index.html", Path: craftTestOutputDir + "/index.html", Type: sandbox.RemoteEntryFile, Size: 1}
	source := &captureSource{fakeSandboxSource: craftSourceWith([]sandbox.RemoteDirEntry{entry}, map[string][]byte{entry.Path: []byte("x")}), run: "run-crash", gen: "gen-crash"}
	files := newDirBackedFileService(t)
	captures := &memoryCaptureStore{sealErr: errors.New("crash before seal")}
	drafts := &memoryCaptureDrafts{head: craft.DraftHead{WorkspaceID: "ws", Revision: 0, State: craft.DraftHeadEmpty}}
	artifacts := NewCraftArtifactService(source, files, newMemVersionStore(), nil, CraftArtifactConfig{OutputDir: craftTestOutputDir})
	svc := NewCraftRunCaptureService(artifacts, captures, drafts, nil)
	_, err := svc.CaptureTerminal(context.Background(), scope, "ws", "run-crash", craft.KindWeb, source)
	require.Error(t, err)
	require.Equal(t, "capturing", captures.receipt.State)
	require.EqualValues(t, 1, files.saves)
	require.Zero(t, drafts.advances)
	source.contents[entry.Path] = []byte("y")
	_, err = svc.capture(context.Background(), captures.receipt, craft.KindWeb, source)
	require.ErrorIs(t, err, craft.ErrConflict, "a changed same-Run manifest cannot replace the attempted bytes")
	require.EqualValues(t, 1, files.saves, "conflict is detected before uploading replacement bytes")
	require.Zero(t, drafts.advances)
}
