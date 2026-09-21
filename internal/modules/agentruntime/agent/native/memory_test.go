package native

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/memory"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

type memoryScopeResolver struct {
	scope  nativecontract.Scope
	denied bool
}

var memoryFacadeDBID uint64

func (r memoryScopeResolver) Resolve(context.Context) (nativecontract.Scope, error) {
	return r.scope, nil
}

func (r memoryScopeResolver) Recheck(_ context.Context, scope nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	if r.denied {
		return nativecontract.Scope{}, errors.New("memory access revoked")
	}
	if scope.TenantID != r.scope.TenantID || scope.MemorySubjectID != r.scope.MemorySubjectID {
		return nativecontract.Scope{}, ErrMemoryScopeDenied
	}
	return r.scope, nil
}

func newNativeMemoryFacade(t *testing.T, scope nativecontract.Scope) (*MemoryService, *repository.NativeMemoryRepository) {
	t.Helper()
	return newNativeMemoryFacadeWithResolver(t, &memoryScopeResolver{scope: scope})
}

func newNativeMemoryFacadeWithResolver(t *testing.T, resolver *memoryScopeResolver) (*MemoryService, *repository.NativeMemoryRepository) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared&_foreign_keys=on", t.Name(), atomic.AddUint64(&memoryFacadeDBID, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_tenants (tenant_id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_sessions (tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, session_id TEXT NOT NULL, PRIMARY KEY (tenant_id, owner_id, session_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_memory_scopes (tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, generation INTEGER NOT NULL DEFAULT 0, tombstone_generation INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1, policy_revision INTEGER NOT NULL DEFAULT 0, updated_at DATETIME, PRIMARY KEY (tenant_id, user_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_memory_entries (tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL, memory_id TEXT NOT NULL, generation INTEGER NOT NULL, tombstoned INTEGER NOT NULL DEFAULT 0, content TEXT NOT NULL, metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME, PRIMARY KEY (tenant_id, user_id, memory_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_memory_jobs (tenant_id INTEGER NOT NULL, subject_id TEXT NOT NULL, job_id TEXT NOT NULL, generation INTEGER NOT NULL, through_event_id TEXT NOT NULL, session_app_name TEXT NOT NULL DEFAULT '', session_user_id TEXT NOT NULL DEFAULT '', session_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'queued', policy_revision INTEGER NOT NULL DEFAULT 0, retry_attempt INTEGER NOT NULL DEFAULT 0, max_attempts INTEGER NOT NULL DEFAULT 3, retry_status TEXT NOT NULL DEFAULT 'none', next_attempt_at DATETIME, last_error TEXT, created_at DATETIME, updated_at DATETIME, PRIMARY KEY (tenant_id, subject_id, job_id), UNIQUE (tenant_id, subject_id, through_event_id, generation))").Error)
	require.NoError(t, db.Exec("CREATE TABLE native_agent_session_events (tenant_id INTEGER NOT NULL, app_name TEXT NOT NULL, user_id TEXT NOT NULL, session_id TEXT NOT NULL, stable_event_id TEXT NOT NULL, PRIMARY KEY (tenant_id, app_name, user_id, session_id, stable_event_id), FOREIGN KEY (tenant_id, user_id, session_id) REFERENCES native_agent_sessions(tenant_id, owner_id, session_id))").Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", resolver.scope.TenantID).Error)
	repo := repository.NewNativeMemoryRepository(db)
	return NewMemoryService(resolver, repo, nil), repo
}

func nativeMemoryJob(t *testing.T, repo *repository.NativeMemoryRepository, scope nativecontract.Scope, id, eventID string) nativecontract.MemoryJob {
	t.Helper()
	key := session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/czE"}
	require.NoError(t, repo.DB().Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", scope.TenantID, key.UserID, key.SessionID).Error)
	require.NoError(t, repo.DB().Exec("INSERT INTO native_agent_session_events (tenant_id, app_name, user_id, session_id, stable_event_id) VALUES (?, ?, ?, ?, ?)", scope.TenantID, key.AppName, key.UserID, key.SessionID, eventID).Error)
	state, err := repo.State(context.Background(), scope)
	require.NoError(t, err)
	return nativecontract.MemoryJob{ID: id, Scope: scope, SessionKey: key, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: eventID}
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
	expectedNames := map[string]struct{}{
		memory.AddToolName: {}, memory.UpdateToolName: {}, memory.DeleteToolName: {},
		memory.ClearToolName: {}, memory.SearchToolName: {}, memory.LoadToolName: {},
	}
	for _, candidate := range tools {
		declaration := candidate.Declaration()
		require.NotNil(t, declaration)
		require.NotEmpty(t, declaration.Description)
		require.NotNil(t, declaration.InputSchema)
		_, found := expectedNames[declaration.Name]
		require.True(t, found, declaration.Name)
		delete(expectedNames, declaration.Name)
	}
	require.Empty(t, expectedNames)
}

func TestNativeMemoryWorkerChecksGenerationAtCommitAndDoesNotRetry(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "j1", "e1")
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

func TestNativeMemoryWorkerRejectsForgedOrUnqueuedJobBeforeExtraction(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "queued", "event-1")
	require.NoError(t, svc.Enqueue(ctx, job))
	calls := 0
	svc.SetExtractor(func(context.Context, nativecontract.MemoryJob) ([]MemoryWrite, error) {
		calls++
		return []MemoryWrite{{ID: "m1", Content: "must not extract"}}, nil
	})

	forged := job
	forged.ThroughEventID = "other-event"
	require.ErrorIs(t, svc.Execute(ctx, forged), ErrMemoryWriteDenied)
	unqueued := job
	unqueued.ID = "never-enqueued"
	require.ErrorIs(t, svc.Execute(ctx, unqueued), ErrMemoryWriteDenied)
	require.Zero(t, calls)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobQueued, status)
}

func TestNativeMemoryWorkerDiscardsJobWhenAccessIsRevokedDuringExtraction(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	resolver := &memoryScopeResolver{scope: scope}
	svc, repo := newNativeMemoryFacadeWithResolver(t, resolver)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "revoked", "event-1")
	require.NoError(t, svc.Enqueue(ctx, job))
	calls := 0
	svc.SetExtractor(func(context.Context, nativecontract.MemoryJob) ([]MemoryWrite, error) {
		calls++
		resolver.denied = true
		return []MemoryWrite{{ID: "m1", Content: "must not persist"}}, nil
	})

	require.ErrorIs(t, svc.Execute(ctx, job), ErrMemoryScopeDenied)
	require.Equal(t, 1, calls)
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

func TestNativeMemoryNilExtractorSchedulesRetry(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "nil-extractor", "event")
	require.NoError(t, svc.Enqueue(ctx, job))
	require.ErrorIs(t, svc.Execute(ctx, job), ErrMemoryExtractorUnavailable)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobQueued, status)
}

func TestNativeMemoryExecuteRenewsLiveAttemptAcrossLeaseBoundary(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "long-running", "event")
	require.NoError(t, svc.Enqueue(ctx, job))
	// The production interval remains below the one-minute claim lease. A
	// short test interval lets this deterministic blocked extractor cross the
	// lease boundary without sleeping for a minute.
	svc.claimRenewInterval = 10 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	svc.SetExtractor(func(ctx context.Context, _ nativecontract.MemoryJob) ([]MemoryWrite, error) {
		close(started)
		select {
		case <-release:
			return []MemoryWrite{{ID: "memory-1", Content: "completed once"}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	done := make(chan error, 1)
	go func() { done <- svc.Execute(ctx, job) }()
	<-started
	// Model the original one-minute expiry while extraction remains live. The
	// service heartbeat must replace it with a future durable deadline.
	require.NoError(t, repo.DB().Exec("UPDATE native_memory_jobs SET next_attempt_at=? WHERE tenant_id=? AND job_id=?", time.Now().UTC().Add(-time.Second), scope.TenantID, job.ID).Error)
	require.Eventually(t, func() bool {
		var row struct{ NextAttemptAt *time.Time }
		if err := repo.DB().Table("native_memory_jobs").Select("next_attempt_at").Where("tenant_id=? AND job_id=?", scope.TenantID, job.ID).Take(&row).Error; err != nil {
			return false
		}
		return row.NextAttemptAt != nil && row.NextAttemptAt.After(time.Now().UTC())
	}, time.Second, time.Millisecond)
	// A recovery worker using a fresh repository must still see the renewed
	// durable claim. Without this check, a test that only inspects the deadline
	// can pass even though a healthy extractor is reclaimed and eventually
	// consumes all retry attempts.
	_, reclaimed, err := repository.NewNativeMemoryRepository(repo.DB()).Claim(ctx, job)
	require.NoError(t, err)
	require.False(t, reclaimed, "a healthy extractor must retain its renewed claim")

	close(release)
	require.NoError(t, <-done)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobSucceeded, status)
}

func TestNativeMemoryWorkerRejectsStaleGenerationWithoutBackendDispatch(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	job := nativeMemoryJob(t, repo, scope, "stale", "event")
	require.NoError(t, svc.Enqueue(ctx, job))
	require.NoError(t, svc.Clear(ctx, scope))
	calls := 0
	svc.SetExtractor(func(context.Context, nativecontract.MemoryJob) ([]MemoryWrite, error) {
		calls++
		return []MemoryWrite{{ID: "m1", Content: "must not extract"}}, nil
	})

	require.ErrorIs(t, svc.Execute(ctx, job), ErrMemoryWriteDenied)
	require.Zero(t, calls)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, repository.NativeMemoryJobDiscarded, status)
}

func TestNativeMemoryAutoJobUsesAuthorizedSessionAndLastEvent(t *testing.T) {
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", MemorySubjectID: "u1", PolicyRevision: 1}
	svc, repo := newNativeMemoryFacade(t, scope)
	ctx := context.Background()
	require.NoError(t, repo.EnsureScope(ctx, scope))
	key, err := nativecontract.SessionKey(scope, "session-1")
	require.NoError(t, err)
	sess := &session.Session{ID: "session-1", AppName: key.AppName, UserID: key.UserID, Events: []event.Event{{ID: "e1"}, {ID: "e2"}}}
	require.NoError(t, repo.DB().Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", scope.TenantID, key.UserID, key.SessionID).Error)
	require.NoError(t, repo.DB().Exec("INSERT INTO native_agent_session_events (tenant_id, app_name, user_id, session_id, stable_event_id) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)", scope.TenantID, key.AppName, key.UserID, key.SessionID, "e1", scope.TenantID, key.AppName, key.UserID, key.SessionID, "e2").Error)
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
