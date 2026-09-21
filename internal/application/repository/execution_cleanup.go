package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/execution"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CleanupClaim is the fenced lease for one session tombstone. The epoch is
// bumped for every claim so a delayed worker cannot complete a newer claim.
type CleanupClaim struct {
	TenantID         uint64
	OwnerID          string
	SessionID        string
	DeletionRevision int64
	BackupReconciled bool
	RetentionUntil   *time.Time
	Worker           string
	Epoch            int64
}

type executionCleanupRow struct {
	TenantID         uint64
	OwnerID          string
	SessionID        string
	DeletionRevision int64
	State            string
	Worker           string
	Epoch            int64
	LeaseUntil       *time.Time
	Stopped          bool
	Settled          bool
	RetentionElapsed bool
	BackupReconciled bool
	RetentionUntil   *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (executionCleanupRow) TableName() string { return "execution_cleanup" }

type cleanupLifecycleStore interface {
	TombstoneSession(context.Context, uint64, string, string) error
}

// TombstoneSession writes the deletion barrier before session/run rows may be
// reclaimed. The owner predicate is checked against the authenticated session
// owner; a different owner is indistinguishable from a missing session.
func (s *AgentRunStore) TombstoneSession(ctx context.Context, tenant uint64, owner, sessionID string) error {
	if tenant == 0 || owner == "" || sessionID == "" {
		return runtime.ErrNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing executionCleanupRow
		err := tx.Where("tenant_id=? AND session_id=?", tenant, sessionID).Take(&existing).Error
		if err == nil {
			if existing.OwnerID != owner {
				return runtime.ErrNotFound
			}
			// A new deletion revision invalidates all observations and claims
			// from the previous deletion. This is deliberately not idempotent:
			// replaying the delete must advance the barrier.
			return tx.Model(&executionCleanupRow{}).
				Where("tenant_id=? AND session_id=? AND owner_id=?", tenant, sessionID, owner).
				Updates(map[string]any{
					"state": "tombstoned", "deletion_revision": gorm.Expr("deletion_revision+1"),
					"worker": "", "lease_until": nil, "epoch": gorm.Expr("epoch+1"),
					"stopped": false, "settled": false, "retention_elapsed": false,
					"backup_reconciled": false, "retention_until": nil,
					"updated_at": s.cleanupNowExpr(),
				}).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var session struct {
			OwnerID string `gorm:"column:user_id"`
		}
		if err := tx.Table("sessions").Select("user_id").Where("tenant_id=? AND id=? AND user_id=?", tenant, sessionID, owner).Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runtime.ErrNotFound
			}
			return err
		}
		var revision struct{ Revision int64 }
		_ = tx.Table("agent_runs").Select("COALESCE(MAX(revision),0) AS revision").Where("tenant_id=? AND session_id=?", tenant, sessionID).Scan(&revision).Error
		row := executionCleanupRow{TenantID: tenant, OwnerID: owner, SessionID: sessionID, DeletionRevision: revision.Revision + 1, State: "tombstoned"}
		return tx.Create(&row).Error
	})
}

// ClaimCleanup leases one tombstone. Unknown cleanup evidence remains durable
// and is retried; callers must not interpret a claim as permission to purge.
func (s *AgentRunStore) ClaimCleanup(ctx context.Context, worker string, lease time.Duration) (CleanupClaim, error) {
	return s.claimCleanup(ctx, worker, lease, []string{"tombstoned", "cleanup_pending", "cleanup_claimed", "cleanup_ready"})
}

func (s *AgentRunStore) claimCleanup(ctx context.Context, worker string, lease time.Duration, states []string) (CleanupClaim, error) {
	if worker == "" || lease <= 0 {
		return CleanupClaim{}, runtime.ErrConflict
	}
	var claim CleanupClaim
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row executionCleanupRow
		now := time.Now()
		if err := tx.Where("state IN ? AND (lease_until IS NULL OR lease_until <= ?)", states, now).
			Order("updated_at, tenant_id, session_id").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runtime.ErrNotFound
			}
			return err
		}
		until := now.Add(lease)
		updated := tx.Model(&executionCleanupRow{}).Where("tenant_id=? AND session_id=? AND epoch=? AND state IN ? AND (lease_until IS NULL OR lease_until <= ?)", row.TenantID, row.SessionID, row.Epoch, states, now).
			Updates(map[string]any{"state": "cleanup_claimed", "worker": worker, "epoch": gorm.Expr("epoch+1"), "lease_until": until, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return runtime.ErrConflict
		}
		claim = CleanupClaim{TenantID: row.TenantID, OwnerID: row.OwnerID, SessionID: row.SessionID, DeletionRevision: row.DeletionRevision, BackupReconciled: row.BackupReconciled, RetentionUntil: row.RetentionUntil, Worker: worker, Epoch: row.Epoch + 1}
		return nil
	})
	return claim, err
}

// CompleteCleanup records observed stop/settlement/retention facts under the
// claim fence. Purging is only marked after all three facts are true.
func (s *AgentRunStore) CompleteCleanup(ctx context.Context, claim CleanupClaim, facts execution.CleanupFacts) error {
	if claim.TenantID == 0 || claim.OwnerID == "" || claim.SessionID == "" || claim.DeletionRevision <= 0 || claim.Worker == "" || claim.Epoch <= 0 {
		return runtime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row executionCleanupRow
		if err := tx.Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_claimed'", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runtime.ErrLeaseLost
			}
			return err
		}
		stopped, settled, retained := row.Stopped || facts.Stopped, row.Settled || facts.Settled, row.RetentionElapsed || facts.RetentionElapsed
		state := "cleanup_pending"
		canPurge := execution.CanPurge(execution.CleanupFacts{Stopped: stopped, Settled: settled, RetentionElapsed: retained}) && row.BackupReconciled && row.RetentionUntil != nil && !row.RetentionUntil.After(time.Now())
		if canPurge {
			// File/blob/backup deletion is an external side effect. Persist a
			// durable ready intent and let RunCleanupPurgeOnce execute it outside
			// this SQL transaction.
			state = "cleanup_ready"
		}
		worker := ""
		if canPurge {
			worker = claim.Worker // exact same claim continues into purge
		}
		updated := tx.Model(&executionCleanupRow{}).
			Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_claimed'", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).
			Updates(map[string]any{"state": state, "stopped": stopped, "settled": settled, "retention_elapsed": retained, "worker": worker, "lease_until": nil, "updated_at": s.cleanupNowExpr()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return runtime.ErrLeaseLost
		}
		return nil
	})
}

// RunCleanupPurgeOnce claims a durable cleanup_ready intent, performs the
// external idempotent file/blob/backup deletion outside the DB transaction,
// then atomically removes run-owned rows and marks the tombstone purged.
func (s *AgentRunStore) RunCleanupPurgeOnce(ctx context.Context, worker string, lease time.Duration) error {
	if s == nil || s.cleanupFiles == nil {
		return ErrCleanupFilesUnavailable
	}
	claim, err := s.claimCleanup(ctx, worker, lease, []string{"cleanup_ready"})
	if err != nil {
		return err
	}
	return s.PurgeCleanupClaim(ctx, claim)
}

// PurgeCleanupClaim continues the exact claim that produced cleanup_ready.
// It rechecks every durable fact before and after the external delete.
func (s *AgentRunStore) claimReadyForPurge(ctx context.Context, claim CleanupClaim, lease time.Duration) error {
	if !claim.BackupReconciled || claim.RetentionUntil == nil || claim.RetentionUntil.After(time.Now()) {
		return runtime.ErrLeaseLost
	}
	updated := s.db.WithContext(ctx).Model(&executionCleanupRow{}).
		Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_ready' AND stopped=? AND settled=? AND retention_elapsed=? AND backup_reconciled=? AND retention_until IS NOT NULL AND retention_until <= ?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch, true, true, true, true, time.Now()).
		Updates(map[string]any{"state": "cleanup_claimed", "lease_until": time.Now().Add(lease), "updated_at": s.cleanupNowExpr()})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return runtime.ErrLeaseLost
	}
	return nil
}

func (s *AgentRunStore) refreshCleanupClaim(ctx context.Context, claim CleanupClaim) (CleanupClaim, error) {
	var row executionCleanupRow
	err := s.db.WithContext(ctx).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CleanupClaim{}, runtime.ErrLeaseLost
	}
	if err != nil {
		return CleanupClaim{}, err
	}
	claim.BackupReconciled, claim.RetentionUntil = row.BackupReconciled, row.RetentionUntil
	return claim, nil
}

func (s *AgentRunStore) PurgeCleanupClaim(ctx context.Context, claim CleanupClaim) error {
	if s == nil || s.cleanupFiles == nil {
		return ErrCleanupFilesUnavailable
	}
	if claim.TenantID == 0 || claim.OwnerID == "" || claim.SessionID == "" || claim.DeletionRevision <= 0 || claim.Worker == "" || claim.Epoch <= 0 {
		return runtime.ErrConflict
	}
	var row executionCleanupRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_claimed' AND backup_reconciled=? AND retention_until IS NOT NULL AND retention_until <= ?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch, true, time.Now()).Take(&row).Error; err != nil {
		return runtime.ErrLeaseLost
	}
	if !row.Stopped || !row.Settled || !row.RetentionElapsed || !row.BackupReconciled || row.RetentionUntil == nil || row.RetentionUntil.After(time.Now()) {
		return runtime.ErrLeaseLost
	}
	if err := s.cleanupFiles.PurgeSessionFiles(ctx, claim); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row executionCleanupRow
		if err := tx.Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_claimed' AND backup_reconciled=? AND retention_until IS NOT NULL AND retention_until <= ?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch, true, time.Now()).Take(&row).Error; err != nil {
			return runtime.ErrLeaseLost
		}
		if !row.Stopped || !row.Settled || !row.RetentionElapsed || !row.BackupReconciled || row.RetentionUntil == nil || row.RetentionUntil.After(time.Now()) {
			return runtime.ErrLeaseLost
		}
		if err := purgeExecutionRecords(tx, claim); err != nil {
			return err
		}
		return tx.Model(&executionCleanupRow{}).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).Updates(map[string]any{"state": "purged", "worker": "", "lease_until": nil, "updated_at": s.cleanupNowExpr()}).Error
	})
}

func purgeExecutionRecords(tx *gorm.DB, claim CleanupClaim) error {
	var runIDs []string
	if err := tx.Table("agent_runs").Where("tenant_id=? AND session_id=?", claim.TenantID, claim.SessionID).Pluck("run_id", &runIDs).Error; err != nil {
		return err
	}
	for _, runID := range runIDs {
		for _, table := range []string{"agent_run_inputs", "agent_run_decisions", "agent_run_events", "agent_tool_attempts", "agent_tool_calls", "agent_run_checkpoints"} {
			if !tx.Migrator().HasTable(table) {
				continue
			}
			if err := tx.Exec("DELETE FROM "+table+" WHERE tenant_id=? AND run_id=?", claim.TenantID, runID).Error; err != nil {
				return err
			}
		}
	}
	// Keep execution_cleanup as the tombstone/audit reference. All run-owned
	// control/event rows are disposable only after the evidence fence passes.
	return tx.Exec("DELETE FROM agent_runs WHERE tenant_id=? AND session_id=?", claim.TenantID, claim.SessionID).Error
}

// CleanupObservationSource is the production adapter boundary. Each method
// must query the authoritative stop, usage, replay and restore systems for
// this deletion revision; absence or uncertainty is false.
type CleanupObservationSource interface {
	ObserveStop(context.Context, CleanupClaim) (bool, error)
	ObserveUsage(context.Context, CleanupClaim) (bool, error)
	ObserveReplay(context.Context, CleanupClaim) (bool, error)
	ObserveBackupRestore(context.Context, CleanupClaim) (bool, error)
	ObserveRetention(context.Context, CleanupClaim) (bool, error)
}

// CleanupFilePurger is the W26 adapter boundary. Production must provide the
// tenant-scoped object/blob/backup deleter before a tombstone can be purged.
// A nil adapter is intentionally fail-closed.
type CleanupFilePurger interface {
	PurgeSessionFiles(context.Context, CleanupClaim) error
}

type idempotentFileDeleter interface {
	DeleteFileIdempotent(context.Context, string, string) error
}

var (
	ErrCleanupFilesUnavailable         = errors.New("execution cleanup file purger unavailable")
	ErrCleanupObservationUnavailable   = errors.New("execution cleanup observation unavailable")
	ErrCleanupSharedReference          = errors.New("execution cleanup shared reference still live")
	ErrCleanupFileDeleterNotIdempotent = errors.New("execution cleanup file deleter does not support idempotent deletion")
)

// FileCleanupPurger is the production object-store adapter. It removes only
// refs with no remaining live owner and keeps artifact rows as audit receipts.
// The wrapped FileService must implement the idempotent deleter protocol
// (DeleteFileIdempotent with the persisted provider idempotency key); a
// legacy FileService fails closed with ErrCleanupFileDeleterNotIdempotent and
// no external delete is attempted.
type FileCleanupPurger struct {
	db    *gorm.DB
	files interfaces.FileService
}

func NewFileCleanupPurger(db *gorm.DB, files interfaces.FileService) *FileCleanupPurger {
	return &FileCleanupPurger{db: db, files: files}
}

func (p *FileCleanupPurger) PurgeSessionFiles(ctx context.Context, claim CleanupClaim) error {
	if p == nil || p.db == nil || p.files == nil {
		return ErrCleanupFilesUnavailable
	}
	deleter, idempotent := p.files.(idempotentFileDeleter)
	if !idempotent {
		// Fail closed: a legacy DeleteFile call cannot carry the persisted
		// provider idempotency key or honor the delete lease, so an external
		// delete must not be attempted. The claim keeps its lease and every
		// artifact row stays pending with its previous receipt, so the purge
		// retries once a W26-capable deleter is installed instead of silently
		// succeeding through the non-idempotent path.
		return ErrCleanupFileDeleterNotIdempotent
	}
	var refs []struct {
		Ref  string
		Kind string
	}
	// Reserve the durable intent rows in a short transaction. The external
	// delete happens after commit; a crash leaves a token that can be retried.
	if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("execution_cleanup_artifacts").Clauses(clause.Locking{Strength: "UPDATE"}).Select("ref, kind").Where("tenant_id=? AND session_id=? AND deletion_revision=? AND state='pending'", claim.TenantID, claim.SessionID, claim.DeletionRevision).Find(&refs).Error; err != nil {
			return err
		}
		for _, item := range refs {
			var live int64
			if err := tx.Table("execution_cleanup_artifacts").Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND ref=? AND state='pending'", claim.TenantID, item.Ref).Count(&live).Error; err != nil {
				return err
			}
			if live > 1 {
				return ErrCleanupSharedReference
			}
			token := fmt.Sprintf("%d:%s:%d:%d:%s", claim.TenantID, claim.SessionID, claim.DeletionRevision, claim.Epoch, item.Ref)
			key := fmt.Sprintf("cleanup:%d:%s:%d:%s", claim.TenantID, claim.SessionID, claim.DeletionRevision, item.Ref)
			if err := tx.Table("execution_cleanup_artifacts").Where("tenant_id=? AND session_id=? AND deletion_revision=? AND ref=? AND state='pending'", claim.TenantID, claim.SessionID, claim.DeletionRevision, item.Ref).Updates(map[string]any{"delete_token": token, "provider_idempotency_key": key, "attempt": gorm.Expr("attempt+1"), "receipt_state": "uncertain", "delete_lease_until": time.Now().Add(10 * time.Minute)}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range refs {
		key := fmt.Sprintf("cleanup:%d:%s:%d:%s", claim.TenantID, claim.SessionID, claim.DeletionRevision, item.Ref)
		deleteErr := deleter.DeleteFileIdempotent(ctx, item.Ref, key)
		if deleteErr != nil {
			return deleteErr
		}
		token := fmt.Sprintf("%d:%s:%d:%d:%s", claim.TenantID, claim.SessionID, claim.DeletionRevision, claim.Epoch, item.Ref)
		if err := p.db.WithContext(ctx).Table("execution_cleanup_artifacts").Where("tenant_id=? AND session_id=? AND deletion_revision=? AND ref=? AND state='pending' AND delete_token=?", claim.TenantID, claim.SessionID, claim.DeletionRevision, item.Ref, token).Updates(map[string]any{"state": "deleted", "deleted_at": time.Now(), "receipt_state": "confirmed", "delete_token": "", "delete_lease_until": nil}).Error; err != nil {
			return err
		}
	}
	return nil
}

// SetCleanupFilePurger installs the W26-owned file/blob/backup adapter. The
// default container leaves it unset while that dependency is blocked.
func (s *AgentRunStore) SetCleanupFilePurger(purger CleanupFilePurger) {
	if s != nil {
		s.cleanupFiles = purger
	}
}

// RecordCleanupBackupRestore persists the authoritative W26 restore receipt
// under the deletion fence. A false receipt never opens cleanup.
func (s *AgentRunStore) RecordCleanupBackupRestore(ctx context.Context, claim CleanupClaim, reconciled bool) error {
	return s.updateCleanupObservation(ctx, claim, map[string]any{"backup_reconciled": reconciled})
}

// RecordCleanupRetention persists the retention deadline selected by policy.
func (s *AgentRunStore) RecordCleanupRetention(ctx context.Context, claim CleanupClaim, until time.Time) error {
	if until.IsZero() {
		return runtime.ErrConflict
	}
	return s.updateCleanupObservation(ctx, claim, map[string]any{"retention_until": until})
}

// RegisterCleanupArtifact is the late-upload guard. A producer carrying an
// old deletion revision cannot attach a new ref to a tombstoned session.
func (s *AgentRunStore) RegisterCleanupArtifact(ctx context.Context, claim CleanupClaim, ref, kind string) error {
	if claim.TenantID == 0 || claim.OwnerID == "" || claim.SessionID == "" || claim.DeletionRevision <= 0 || ref == "" || kind == "" {
		return runtime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row executionCleanupRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND state <> 'purged'", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision).Take(&row).Error; err != nil {
			return runtime.ErrLeaseLost
		}
		// Serialize all references, including rows owned by other sessions. This
		// closes the late-upload race with the purge worker at the ref level.
		var refs []struct{ Ref string }
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("execution_cleanup_artifacts").Select("ref").Where("tenant_id=? AND ref=?", claim.TenantID, ref).Find(&refs).Error; err != nil {
			return err
		}
		return tx.Table("execution_cleanup_artifacts").Create(map[string]any{"tenant_id": claim.TenantID, "session_id": claim.SessionID, "deletion_revision": claim.DeletionRevision, "ref": ref, "kind": kind, "state": "pending", "receipt_state": "none"}).Error
	})
}

func (s *AgentRunStore) updateCleanupObservation(ctx context.Context, claim CleanupClaim, updates map[string]any) error {
	updated := s.db.WithContext(ctx).Model(&executionCleanupRow{}).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).Updates(updates)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return runtime.ErrLeaseLost
	}
	return nil
}

// RunCleanupOnce executes one leased observation cycle. It intentionally does
// not fake provider/file evidence; callers must supply real adapters.
func (s *AgentRunStore) RunCleanupOnce(ctx context.Context, worker string, lease time.Duration) error {
	if s == nil || s.cleanupSource == nil {
		return runtime.ErrConflict
	}
	source := s.cleanupSource
	claim, err := s.ClaimCleanup(ctx, worker, lease)
	if err != nil {
		return err
	}
	stop, err := source.ObserveStop(ctx, claim)
	if err != nil {
		return err
	}
	usage, err := source.ObserveUsage(ctx, claim)
	if err != nil {
		return err
	}
	replay, err := source.ObserveReplay(ctx, claim)
	if err != nil {
		return err
	}
	restore, err := source.ObserveBackupRestore(ctx, claim)
	if err != nil {
		return err
	}
	retention, err := source.ObserveRetention(ctx, claim)
	if err != nil {
		return err
	}
	facts := execution.CleanupFacts{Stopped: stop && replay, Settled: usage, RetentionElapsed: retention && restore}
	if err := s.CompleteCleanup(ctx, claim, facts); err != nil {
		return err
	}
	if execution.CanPurge(facts) {
		claim, err = s.refreshCleanupClaim(ctx, claim)
		if err != nil {
			return err
		}
		if err := s.claimReadyForPurge(ctx, claim, lease); err != nil {
			return err
		}
		return s.PurgeCleanupClaim(ctx, claim)
	}
	return nil
}

// NewDurableCleanupObservationSource builds the default fail-closed source.
// It reads product run/event/observation state only; remote provider, backup,
// and object-store adapters must be added through their owning tasks.
func NewDurableCleanupObservationSource(db *gorm.DB) CleanupObservationSource {
	return &durableCleanupObservationSource{db: db}
}

type durableCleanupObservationSource struct{ db *gorm.DB }

func (o *durableCleanupObservationSource) ObserveStop(ctx context.Context, claim CleanupClaim) (bool, error) {
	if err := o.validateClaim(ctx, claim); err != nil {
		return false, err
	}
	var active int64
	err := o.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND owner_id=? AND session_id=? AND status NOT IN ?", claim.TenantID, claim.OwnerID, claim.SessionID, []string{"canceled", "succeeded", "failed"}).Count(&active).Error
	return active == 0, err
}

func (o *durableCleanupObservationSource) ObserveUsage(ctx context.Context, claim CleanupClaim) (bool, error) {
	if err := o.validateClaim(ctx, claim); err != nil {
		return false, err
	}
	var runs int64
	if err := o.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND owner_id=? AND session_id=?", claim.TenantID, claim.OwnerID, claim.SessionID).Count(&runs).Error; err != nil {
		return false, err
	}
	if runs == 0 {
		return false, nil
	}
	var usage int64
	err := o.db.WithContext(ctx).Table("agent_run_events e").Joins("JOIN agent_runs r ON r.tenant_id=e.tenant_id AND r.run_id=e.run_id").Where("e.tenant_id=? AND r.owner_id=? AND r.session_id=? AND e.event_type LIKE 'usage.%'", claim.TenantID, claim.OwnerID, claim.SessionID).Count(&usage).Error
	return usage > 0, err
}

func (o *durableCleanupObservationSource) ObserveReplay(ctx context.Context, claim CleanupClaim) (bool, error) {
	if err := o.validateClaim(ctx, claim); err != nil {
		return false, err
	}
	var incomplete int64
	err := o.db.WithContext(ctx).Table("execution_observations o").Joins("JOIN agent_runs r ON r.tenant_id=o.tenant_id AND r.run_id=o.run_id").Where("o.tenant_id=? AND r.owner_id=? AND r.session_id=? AND o.history_incomplete = ?", claim.TenantID, claim.OwnerID, claim.SessionID, true).Count(&incomplete).Error
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return false, ErrCleanupObservationUnavailable
		}
		return false, err
	}
	return incomplete == 0, nil
}

func (o *durableCleanupObservationSource) ObserveBackupRestore(ctx context.Context, claim CleanupClaim) (bool, error) {
	if err := o.validateClaim(ctx, claim); err != nil {
		return false, err
	}
	var row executionCleanupRow
	if err := o.db.WithContext(ctx).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision).Take(&row).Error; err != nil {
		return false, err
	}
	return row.BackupReconciled, nil
}

func (o *durableCleanupObservationSource) ObserveRetention(ctx context.Context, claim CleanupClaim) (bool, error) {
	if err := o.validateClaim(ctx, claim); err != nil {
		return false, err
	}
	var row executionCleanupRow
	if err := o.db.WithContext(ctx).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision).Take(&row).Error; err != nil {
		return false, err
	}
	return row.RetentionUntil != nil && !row.RetentionUntil.After(time.Now()), nil
}

func (o *durableCleanupObservationSource) validateClaim(ctx context.Context, claim CleanupClaim) error {
	var row executionCleanupRow
	if err := o.db.WithContext(ctx).Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=?", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).Take(&row).Error; err != nil {
		return runtime.ErrLeaseLost
	}
	return nil
}

func (s *AgentRunStore) cleanupNowExpr() any {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("CURRENT_TIMESTAMP")
	}
	return gorm.Expr("CURRENT_TIMESTAMP")
}
