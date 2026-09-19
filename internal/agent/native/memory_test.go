package native

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type memoryScopeResolver struct{ scope nativecontract.Scope }

func (r memoryScopeResolver) Resolve(context.Context) (nativecontract.Scope, error) {
	return r.scope, nil
}
func (r memoryScopeResolver) Recheck(_ context.Context, scope nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	if scope.TenantID != r.scope.TenantID || scope.MemorySubjectID != r.scope.MemorySubjectID {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	return r.scope, nil
}

func newNativeMemoryFacade(t *testing.T, scope nativecontract.Scope) (*MemoryService, *repository.NativeMemoryRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_tenants (tenant_id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_memory_scopes (tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, generation INTEGER NOT NULL DEFAULT 0, tombstone_generation INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1, policy_revision INTEGER NOT NULL DEFAULT 0, updated_at DATETIME, PRIMARY KEY (tenant_id, user_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_memory_entries (tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, memory_id TEXT NOT NULL, generation INTEGER NOT NULL, tombstoned INTEGER NOT NULL DEFAULT 0, content TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME, PRIMARY KEY (tenant_id, user_id, memory_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_memory_jobs (tenant_id INTEGER NOT NULL, subject_id TEXT NOT NULL, job_id TEXT NOT NULL, generation INTEGER NOT NULL, through_event_id TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'queued', policy_revision INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME, PRIMARY KEY (tenant_id, subject_id, job_id), UNIQUE (tenant_id, subject_id, through_event_id, generation))").Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", scope.TenantID).Error)
	repo := repository.NewNativeMemoryRepository(db)
	return NewMemoryService(memoryScopeResolver{scope: scope}, repo, nil), repo
}

func TestMemoryWriteRejectsOldGeneration(t *testing.T) {
	require.False(t, AcceptMemoryWrite(true, 2, 1))
	require.False(t, AcceptMemoryWrite(false, 2, 2))
	require.True(t, AcceptMemoryWrite(true, 2, 2))
}

func TestNativeMemoryWorkerChecksGenerationAtCommitAndDoesNotRetry(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: "j1", Scope: scope, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: "e1"}
	require.NoError(t, svc.Enqueue(ctx, job))
	svc.SetExtractor(func(context.Context, nativecontract.MemoryJob) ([]MemoryWrite, error) {
		require.NoError(t, svc.Clear(ctx, scope))
		return []MemoryWrite{{ID: "m1", Content: "late"}}, nil
	})
	require.NoError(t, svc.Execute(ctx, job))
	entries, err := repo.Read(ctx, scope, 10)
	require.NoError(t, err)
	require.Empty(t, entries)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobDiscarded, status)
}

func TestNativeMemoryFacadeRejectsCrossScopeReadAndDisabledWrite(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	key, err := nativecontract.MemoryKey(scope)
	require.NoError(t, err)
	require.NoError(t, svc.SetEnabled(ctx, scope, false))
	entries, err := svc.ReadMemories(ctx, key, 10)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.ErrorIs(t, svc.AddMemory(ctx, key, "must not write", nil), ErrMemoryWriteDenied)
	key.UserID = "subject/other"
	_, err = svc.ReadMemories(ctx, key, 10)
	require.ErrorIs(t, err, ErrMemoryScopeDenied)
}
