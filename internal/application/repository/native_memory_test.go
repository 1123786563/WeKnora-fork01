package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func nativeMemoryScope(tenant uint64, subject string) nativecontract.Scope {
	return nativecontract.Scope{TenantID: tenant, MemorySubjectID: subject, PolicyRevision: 1}
}

func newNativeMemoryTestRepository(t *testing.T) *NativeMemoryRepository {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	return NewNativeMemoryRepository(db)
}

func TestNativeMemoryOldExtractionAfterClearCannotResurrect(t *testing.T) {
	repo := newNativeMemoryTestRepository(t)
	ctx, scope := context.Background(), nativeMemoryScope(1, "subject-1")
	require.NoError(t, repo.EnsureScope(ctx, scope))
	before, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: "old-job", Scope: scope, Generation: before.Generation, PolicyRevision: before.PolicyRevision, ThroughEventID: "event-1"}
	require.NoError(t, repo.Enqueue(ctx, job))
	require.NoError(t, repo.Clear(ctx, scope))

	written, err := repo.Commit(ctx, job, "memory-1", "must stay forgotten", nil)
	require.NoError(t, err)
	require.False(t, written)
	entries, err := repo.Read(ctx, scope, 10)
	require.NoError(t, err)
	require.Empty(t, entries)
	status, err := repo.JobStatus(ctx, job)
	require.NoError(t, err)
	require.Equal(t, NativeMemoryJobDiscarded, status)
}

func TestNativeMemoryDeleteWritesTombstoneAndInvalidatesOldJob(t *testing.T) {
	repo := newNativeMemoryTestRepository(t)
	ctx, scope := context.Background(), nativeMemoryScope(1, "subject-1")
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: "extract", Scope: scope, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: "event-1"}
	require.NoError(t, repo.Enqueue(ctx, job))
	require.NoError(t, repo.Write(ctx, scope, state.Generation, "m-1", "present", nil))
	require.NoError(t, repo.Delete(ctx, scope, "m-1"))

	written, err := repo.Commit(ctx, job, "m-1", "resurrected", nil)
	require.NoError(t, err)
	require.False(t, written)
	entry, err := repo.Entry(ctx, scope, "m-1")
	require.NoError(t, err)
	require.True(t, entry.Tombstoned)
}

func TestNativeMemoryDisableAndReenableRejectOldGenerationAfterRestart(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	repo := NewNativeMemoryRepository(db)
	ctx, scope := context.Background(), nativeMemoryScope(1, "subject-1")
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	job := nativecontract.MemoryJob{ID: "old", Scope: scope, Generation: state.Generation, PolicyRevision: state.PolicyRevision, ThroughEventID: "event-1"}
	require.NoError(t, repo.Enqueue(ctx, job))
	require.NoError(t, repo.SetEnabled(ctx, scope, false))
	require.NoError(t, repo.SetEnabled(ctx, scope, true))

	repo = NewNativeMemoryRepository(reopenRunDB(t, db))
	written, err := repo.Commit(ctx, job, "m-1", "old generation", nil)
	require.NoError(t, err)
	require.False(t, written)
	state, err = repo.State(ctx, scope)
	require.NoError(t, err)
	require.True(t, state.Enabled)
	require.Greater(t, state.Generation, job.Generation)
}

func TestNativeMemoryScopeIsolationAndDisabledReads(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO tenants (id, name, business) VALUES (2, 'tenant-2', 'test')").Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 2).Error)
	repo := NewNativeMemoryRepository(db)
	ctx := context.Background()
	first, second := nativeMemoryScope(1, "same-subject"), nativeMemoryScope(2, "same-subject")
	require.NoError(t, repo.EnsureScope(ctx, first))
	require.NoError(t, repo.EnsureScope(ctx, second))
	state, err := repo.State(ctx, first)
	require.NoError(t, err)
	require.NoError(t, repo.Write(ctx, first, state.Generation, "m-1", "tenant one", nil))
	other, err := repo.Read(ctx, second, 10)
	require.NoError(t, err)
	require.Empty(t, other, "same subject ID in another tenant must not recall tenant one data")
	require.NoError(t, repo.SetEnabled(ctx, first, false))
	entries, err := repo.Read(ctx, first, 10)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.ErrorIs(t, repo.Write(ctx, first, state.Generation, "m-2", "disabled", nil), ErrNativeMemoryWriteRejected)
	require.False(t, errors.Is(ErrNativeMemoryWriteRejected, ErrNativeMemoryScope))
}

func TestNativeMemoryToggleAndPolicyDriftRetainUntombstonedEntries(t *testing.T) {
	repo := newNativeMemoryTestRepository(t)
	ctx, scope := context.Background(), nativeMemoryScope(1, "subject-1")
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	require.NoError(t, repo.Write(ctx, scope, state.Generation, "one", "first", nil))
	require.NoError(t, repo.Write(ctx, scope, state.Generation, "two", "second", nil))
	require.NoError(t, repo.Delete(ctx, scope, "one"))
	require.NoError(t, repo.SetEnabled(ctx, scope, false))
	require.NoError(t, repo.SetEnabled(ctx, scope, true))
	changed := scope
	changed.PolicyRevision = 2
	require.NoError(t, repo.EnsureScope(ctx, changed))
	entries, err := repo.Read(ctx, changed, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "two", entries[0].ID)
	state, err = repo.State(ctx, changed)
	require.NoError(t, err)
	require.NoError(t, repo.Replace(ctx, changed, state.Generation, "two", NativeMemoryEntry{ID: "three", Content: "updated"}))
	entries, err = repo.Read(ctx, changed, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "three", entries[0].ID)
}

func TestNativeMemoryReplaceRejectsPolicyDriftAndActiveTargetCollision(t *testing.T) {
	repo := newNativeMemoryTestRepository(t)
	ctx, scope := context.Background(), nativeMemoryScope(1, "subject-1")
	require.NoError(t, repo.EnsureScope(ctx, scope))
	state, err := repo.State(ctx, scope)
	require.NoError(t, err)
	require.NoError(t, repo.Write(ctx, scope, state.Generation, "source", "source", nil))
	require.NoError(t, repo.Write(ctx, scope, state.Generation, "target", "target", nil))
	require.ErrorIs(t, repo.Replace(ctx, scope, state.Generation, "source", NativeMemoryEntry{ID: "target", Content: "replacement"}), ErrNativeMemoryWriteRejected)
	source, err := repo.Entry(ctx, scope, "source")
	require.NoError(t, err)
	require.False(t, source.Tombstoned)
	drifted := scope
	drifted.PolicyRevision++
	require.NoError(t, repo.EnsureScope(ctx, drifted))
	require.ErrorIs(t, repo.Replace(ctx, scope, state.Generation, "source", NativeMemoryEntry{ID: "new", Content: "replacement"}), ErrNativeMemoryWriteRejected)
	require.ErrorIs(t, repo.Replace(ctx, drifted, state.Generation+1, "missing", NativeMemoryEntry{ID: "missing", Content: "replacement"}), ErrNativeMemoryWriteRejected)
	require.NoError(t, repo.Delete(ctx, drifted, "source"))
	state, err = repo.State(ctx, drifted)
	require.NoError(t, err)
	require.ErrorIs(t, repo.Replace(ctx, drifted, state.Generation, "source", NativeMemoryEntry{ID: "source", Content: "revive"}), ErrNativeMemoryWriteRejected)
}
