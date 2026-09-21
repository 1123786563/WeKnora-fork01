package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/execution"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type noopCleanupFilePurger struct{}

func (noopCleanupFilePurger) PurgeSessionFiles(context.Context, CleanupClaim) error { return nil }

type recordingCleanupFiles struct {
	interfaces.FileService
	deleted []string
}

// idempotentRecordingCleanupFiles implements the idempotent deleter protocol
// and records the provider idempotency key each delete was fenced with.
// recordingCleanupFiles above deliberately stays legacy-only (bare DeleteFile),
// matching the current production FileService backends.
type idempotentRecordingCleanupFiles struct {
	interfaces.FileService
	deleted []string
	keys    []string
}

func (f *idempotentRecordingCleanupFiles) DeleteFileIdempotent(_ context.Context, ref, key string) error {
	f.deleted = append(f.deleted, ref)
	f.keys = append(f.keys, key)
	return nil
}

type crashCleanupFiles struct {
	interfaces.FileService
	attempts  int
	failFirst bool
}

func (f *crashCleanupFiles) DeleteFileIdempotent(ctx context.Context, ref, _ string) error {
	return f.DeleteFile(ctx, ref)
}

func (f *crashCleanupFiles) DeleteFile(_ context.Context, _ string) error {
	f.attempts++
	if f.failFirst {
		f.failFirst = false
		return assertCleanupCrash{}
	}
	return nil
}

type assertCleanupCrash struct{}

func (assertCleanupCrash) Error() string { return "simulated crash after intent" }

func (f *recordingCleanupFiles) DeleteFile(_ context.Context, ref string) error {
	f.deleted = append(f.deleted, ref)
	return nil
}

func TestExecutionCleanupTombstoneClaimAndSettlement(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	s.SetCleanupFilePurger(noopCleanupFilePurger{})
	ctx := context.Background()
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	require.ErrorIs(t, s.TombstoneSession(ctx, 1, "other", "s1"), runtime.ErrNotFound)
	claim, err := s.ClaimCleanup(ctx, "cleanup-worker", time.Minute)
	require.NoError(t, err)
	require.Equal(t, uint64(1), claim.TenantID)
	require.Equal(t, "s1", claim.SessionID)
	require.Equal(t, "u1", claim.OwnerID)
	require.Positive(t, claim.DeletionRevision)
	require.NoError(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Stopped: true, Settled: false, RetentionElapsed: true}))
	var state string
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "cleanup_pending", state)
	claim, err = s.ClaimCleanup(ctx, "cleanup-worker-2", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.RecordCleanupBackupRestore(ctx, claim, true))
	require.NoError(t, s.RecordCleanupRetention(ctx, claim, time.Now().Add(-time.Second)))
	require.NoError(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Stopped: true, Settled: true, RetentionElapsed: true}))
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "cleanup_ready", state)
	require.NoError(t, s.RunCleanupPurgeOnce(ctx, "cleanup-purger", time.Minute))
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Pluck("state", &state).Error)
	require.Equal(t, "purged", state)
	require.ErrorIs(t, s.PurgeCleanupClaim(ctx, claim), runtime.ErrLeaseLost)
}

func TestExecutionCleanupFactsAreMonotonicAndRevisionFenced(t *testing.T) {
	db := openRunTestDB(t)
	s := NewAgentRunStore(db)
	ctx := context.Background()
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	oldClaim, err := s.ClaimCleanup(ctx, "cleanup-worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.CompleteCleanup(ctx, oldClaim, execution.CleanupFacts{Stopped: true}))
	claim, err := s.ClaimCleanup(ctx, "cleanup-worker-2", time.Minute)
	require.NoError(t, err)
	require.NoError(t, s.TombstoneSession(ctx, 1, "u1", "s1"))
	require.ErrorIs(t, s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Settled: true, RetentionElapsed: true}), runtime.ErrLeaseLost)
	newClaim, err := s.ClaimCleanup(ctx, "cleanup-worker-3", time.Minute)
	require.NoError(t, err)
	require.Greater(t, newClaim.DeletionRevision, claim.DeletionRevision)
	require.NoError(t, s.CompleteCleanup(ctx, newClaim, execution.CleanupFacts{Stopped: true, Settled: false, RetentionElapsed: false}))
	var stopped, settled, retained bool
	require.NoError(t, db.Table("execution_cleanup").Where("tenant_id=? AND session_id=?", 1, "s1").Select("stopped, settled, retention_elapsed").Row().Scan(&stopped, &settled, &retained))
	require.True(t, stopped)
	require.False(t, settled)
	require.False(t, retained)
}

func TestFileCleanupPurgerPreservesSharedRefsAndIsIdempotent(t *testing.T) {
	db := openRunTestDB(t)
	files := &idempotentRecordingCleanupFiles{}
	purger := NewFileCleanupPurger(db, files)
	require.NoError(t, db.Exec("INSERT INTO execution_cleanup (tenant_id, owner_id, session_id, deletion_revision, state) VALUES (1,'u1','s1',1,'cleanup_ready'),(1,'u1','s2',1,'tombstoned')").Error)
	require.NoError(t, db.Exec("INSERT INTO execution_cleanup_artifacts (tenant_id,session_id,deletion_revision,ref,kind) VALUES (1,'s1',1,'blob://shared','file'),(1,'s2',1,'blob://shared','file')").Error)
	claim := CleanupClaim{TenantID: 1, OwnerID: "u1", SessionID: "s1", DeletionRevision: 1, Worker: "w", Epoch: 1}
	require.ErrorIs(t, purger.PurgeSessionFiles(context.Background(), claim), ErrCleanupSharedReference)
	require.Empty(t, files.deleted)
	require.NoError(t, db.Exec("DELETE FROM execution_cleanup_artifacts WHERE session_id='s2'").Error)
	require.NoError(t, purger.PurgeSessionFiles(context.Background(), claim))
	require.Equal(t, []string{"blob://shared"}, files.deleted)
	require.Equal(t, []string{"cleanup:1:s1:1:blob://shared"}, files.keys)
	require.NoError(t, purger.PurgeSessionFiles(context.Background(), claim))
	require.Equal(t, []string{"blob://shared"}, files.deleted)
}

func TestFileCleanupPurgerFailsClosedOnLegacyFileService(t *testing.T) {
	db := openRunTestDB(t)
	files := &recordingCleanupFiles{}
	purger := NewFileCleanupPurger(db, files)
	require.NoError(t, db.Exec("INSERT INTO execution_cleanup_artifacts (tenant_id,session_id,deletion_revision,ref,kind) VALUES (1,'s1',1,'blob://legacy','file')").Error)
	claim := CleanupClaim{TenantID: 1, OwnerID: "u1", SessionID: "s1", DeletionRevision: 1, Worker: "w", Epoch: 1}
	err := purger.PurgeSessionFiles(context.Background(), claim)
	require.ErrorIs(t, err, ErrCleanupFileDeleterNotIdempotent)
	require.Empty(t, files.deleted)
	var artifact struct {
		State        string
		ReceiptState string
		Attempt      int
	}
	require.NoError(t, db.Table("execution_cleanup_artifacts").Select("state, receipt_state, attempt").Where("tenant_id=1 AND session_id='s1'").Scan(&artifact).Error)
	require.Equal(t, "pending", artifact.State)
	require.Equal(t, "none", artifact.ReceiptState)
	require.Zero(t, artifact.Attempt)
}

func TestFileCleanupPurgerRecoversUncertainReceipt(t *testing.T) {
	db := openRunTestDB(t)
	files := &crashCleanupFiles{failFirst: true}
	purger := NewFileCleanupPurger(db, files)
	require.NoError(t, db.Exec("INSERT INTO execution_cleanup_artifacts (tenant_id,session_id,deletion_revision,ref,kind) VALUES (1,'s1',1,'blob://crash','file')").Error)
	claim := CleanupClaim{TenantID: 1, OwnerID: "u1", SessionID: "s1", DeletionRevision: 1, Worker: "w", Epoch: 1}
	require.Error(t, purger.PurgeSessionFiles(context.Background(), claim))
	var receipt, token string
	var attempts int
	var artifact struct {
		ReceiptState           string
		DeleteToken            string
		ProviderIdempotencyKey string
		Attempt                int
	}
	require.NoError(t, db.Table("execution_cleanup_artifacts").Select("receipt_state, delete_token, provider_idempotency_key, attempt").Where("tenant_id=1 AND session_id='s1'").Scan(&artifact).Error)
	receipt, token, attempts = artifact.ReceiptState, artifact.DeleteToken, artifact.Attempt
	require.Equal(t, "uncertain", receipt)
	require.NotEmpty(t, token)
	require.NotEmpty(t, artifact.ProviderIdempotencyKey)
	require.Equal(t, 1, attempts)
	require.NoError(t, purger.PurgeSessionFiles(context.Background(), claim))
	require.Equal(t, 2, files.attempts)
	require.NoError(t, db.Table("execution_cleanup_artifacts").Where("tenant_id=1 AND session_id='s1'").Pluck("receipt_state", &receipt).Error)
	require.Equal(t, "confirmed", receipt)
}
