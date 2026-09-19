package native

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
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

func TestNativeMemoryToolsUseControlledFacadeSurface(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, _ := newNativeMemoryFacade(t, scope)
	tools := svc.Tools()
	require.Len(t, tools, 6)
	for _, candidate := range tools {
		require.NotNil(t, candidate)
	}
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

func TestNativeMemoryFacadePreservesMetadataAndAtomicallyUpdates(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	key, err := nativecontract.MemoryKey(scope)
	require.NoError(t, err)
	require.NoError(t, svc.AddMemory(ctx, key, "old preference", []string{"profile"}, memory.WithMetadata(&memory.Metadata{Kind: memory.KindFact, Location: "Shanghai", Participants: []string{"Ada"}})))
	entries, err := svc.ReadMemories(ctx, key, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, []string{"profile"}, entries[0].Memory.Topics)
	require.Equal(t, memory.KindFact, entries[0].Memory.Kind)
	result := &memory.UpdateResult{}
	require.NoError(t, svc.UpdateMemory(ctx, memory.Key{AppName: key.AppName, UserID: key.UserID, MemoryID: entries[0].ID}, "new preference", []string{"profile"}, memory.WithUpdateResult(result)))
	require.NotEmpty(t, result.MemoryID)
	entries, err = svc.ReadMemories(ctx, key, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "new preference", entries[0].Memory.Memory)
	// An SDK update can retain the same canonical key. It must update in place
	// instead of tombstoning the row it has just upserted.
	require.NoError(t, svc.UpdateMemory(ctx, memory.Key{AppName: key.AppName, UserID: key.UserID, MemoryID: entries[0].ID}, "new preference", []string{"updated"}))
	entries, err = svc.ReadMemories(ctx, key, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, []string{"updated"}, entries[0].Memory.Topics)
}

func TestNativeMemoryNilExtractorPersistsFailedJob(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: "nil-extractor", Scope: scope, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: "event"}
	require.NoError(t, svc.Enqueue(ctx, job))
	require.ErrorIs(t, svc.Execute(ctx, job), ErrMemoryExtractorUnavailable)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobFailed, status)
}

func TestNativeMemoryAutoJobUsesAuthorizedSessionAndLastEvent(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	key, err := nativecontract.SessionKey(scope, "session-1")
	require.NoError(t, err)
	sess := &session.Session{ID: "session-1", AppName: key.AppName, UserID: key.UserID, Events: []event.Event{{ID: "e1"}, {ID: "e2"}}}
	require.NoError(t, svc.EnqueueAutoMemoryJob(ctx, sess))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: memoryID(sess.AppName + "\x00" + sess.UserID + "\x00" + sess.ID + "\x00e2"), Scope: scope, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: "e2"}
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobQueued, status)
	sess.UserID = "forged"
	require.ErrorIs(t, svc.EnqueueAutoMemoryJob(ctx, sess), ErrMemorySessionScopeDenied)
}
