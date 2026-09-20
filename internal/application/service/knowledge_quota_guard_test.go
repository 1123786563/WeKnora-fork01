package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"github.com/stretchr/testify/require"
)

// recordingGuard records every growth reservation and can be switched to
// refuse growth (delta > 0) — the counting-guard contract the plan pins
// read/export paths with.
type recordingGuard struct {
	mu     sync.Mutex
	calls  []string
	refuse bool
}

func (g *recordingGuard) ReserveGrowth(_ context.Context, _ uint64, dimension string, delta int64) (func(), error) {
	g.mu.Lock()
	g.calls = append(g.calls, dimension+"|"+strconv.FormatInt(delta, 10))
	refuse := g.refuse
	g.mu.Unlock()
	if refuse && delta > 0 {
		return nil, commercial.ErrQuotaGrowthRefused
	}
	return func() {}, nil
}

func (g *recordingGuard) recorded() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.calls...)
}

func (g *recordingGuard) setRefuse(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refuse = v
}

var _ commercial.ResourceQuotaGuard = (*recordingGuard)(nil)

// quotaReadRepoStub extends the create stub with the read-path method the
// read test exercises (the embedded interface would nil-panic).
type quotaReadRepoStub struct {
	createKnowledgeFileRepoStub
}

func (r *quotaReadRepoStub) GetKnowledgeByID(_ context.Context, _ uint64, _ string) (*types.Knowledge, error) {
	return nil, errors.New("not found")
}

// newQuotaGuardedKnowledgeService builds the minimal knowledge service with
// the guard wired (the knowledge_create_test.go stub convention).
func newQuotaGuardedKnowledgeService(t *testing.T, guard commercial.ResourceQuotaGuard) (*knowledgeService, *quotaReadRepoStub) {
	t.Helper()
	repo := &quotaReadRepoStub{}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	svc.SetResourceQuotaGuard(guard)
	return svc, repo
}

// TestStorageQuotaBlocksCreateAtLimit: with the guard refusing growth, all
// three knowledge-create entry points answer the storage-quota error class
// and persist nothing.
func TestStorageQuotaBlocksCreateAtLimit(t *testing.T) {
	t.Parallel()
	guard := &recordingGuard{refuse: true}
	svc, repo := newQuotaGuardedKnowledgeService(t, guard)
	ctx := newCreateKnowledgeFileContext()

	_, err := svc.CreateKnowledgeFromFile(ctx, "kb-1", newMultipartFileHeader(t, "doc.txt", "hello"), nil, nil, "", nil, "", nil)
	require.Error(t, err)
	var quotaErr *types.StorageQuotaExceededError
	require.True(t, errors.As(err, &quotaErr) || strings.Contains(err.Error(), "quota"),
		"the refusal must surface the storage-quota error class, got %v", err)
	require.Zero(t, repo.createCalls)

	_, err = svc.CreateKnowledgeFromURL(ctx, "kb-1", "https://example.com/page", "page", "html", nil, "", nil, "", nil)
	require.Error(t, err)

	_, err = svc.CreateKnowledgeFromURL(ctx, "kb-1", "https://example.com/file.pdf", "file.pdf", "pdf", nil, "", nil, "", nil)
	require.Error(t, err)
	require.Zero(t, repo.createCalls)
	// Every create site consulted the guard under the storage dimension.
	calls := guard.recorded()
	require.NotEmpty(t, calls)
	for _, c := range calls {
		require.True(t, strings.HasPrefix(c, "storage_gb|"), "guard calls must target storage_gb, got %q", c)
	}
}

// TestStorageQuotaReservesKnownBytes: the file-upload site reserves the
// ACTUAL byte size (not a placeholder).
func TestStorageQuotaReservesKnownBytes(t *testing.T) {
	t.Parallel()
	guard := &recordingGuard{}
	repo := &createKnowledgeFileRepoStub{createErr: errors.New("insert failed")}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		fileSvc:   &createKnowledgeFileServiceStub{},
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	svc.SetResourceQuotaGuard(guard)

	_, err := svc.CreateKnowledgeFromFile(newCreateKnowledgeFileContext(), "kb-1",
		newMultipartFileHeader(t, "doc.txt", "hello"), nil, nil, "", nil, "", nil)
	require.Error(t, err)
	calls := guard.recorded()
	require.NotEmpty(t, calls, "the create site must consult the guard")
	// "hello" = 5 bytes: the reserve carries the file size in bytes.
	joined := strings.Join(calls, ",")
	require.Contains(t, joined, "storage_gb|5", "the reserve must carry the file size in bytes, got %s", joined)
}

// TestStorageQuotaReadPathsNeverCallGuard: read paths must not touch the
// growth gate — pinned with a guard that REFUSES everything: reads still
// complete without the gate and the call log stays empty.
func TestStorageQuotaReadPathsNeverCallGuard(t *testing.T) {
	t.Parallel()
	guard := &recordingGuard{refuse: true}
	svc, _ := newQuotaGuardedKnowledgeService(t, guard)
	ctx := newCreateKnowledgeFileContext()

	// GetKnowledgeByID is the canonical read path (view/export derive from
	// it); a missing row answering an error is fine — the assertion is that
	// the growth gate was NEVER consulted.
	_, _ = svc.GetKnowledgeByID(ctx, "missing-id")
	require.Empty(t, guard.recorded(), "read paths must never consult the growth gate")
}

// TestStorageQuotaDeleteDecrementsBestEffort: the production decrement hook
// (wired into the delete flow) releases bytes with a NEGATIVE delta — delta
// <= 0 always passes, refusal is ignored (cleanup never blocks).
func TestStorageQuotaDeleteDecrementsBestEffort(t *testing.T) {
	t.Parallel()
	guard := &recordingGuard{refuse: true} // even a refusing guard must not block cleanup
	svc, _ := newQuotaGuardedKnowledgeService(t, guard)

	svc.decreaseStorageUsage(newCreateKnowledgeFileContext(), 1, 128)
	calls := guard.recorded()
	require.Len(t, calls, 1)
	require.Equal(t, "storage_gb|-128", calls[0])
}
