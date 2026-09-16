package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/execution"
	"gorm.io/gorm"
)

// CleanupClaim is the fenced lease for one session tombstone. The epoch is
// bumped for every claim so a delayed worker cannot complete a newer claim.
type CleanupClaim struct {
	TenantID         uint64
	OwnerID          string
	SessionID        string
	DeletionRevision int64
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
	if worker == "" || lease <= 0 {
		return CleanupClaim{}, runtime.ErrConflict
	}
	var claim CleanupClaim
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row executionCleanupRow
		now := time.Now()
		if err := tx.Where("state IN ? AND (lease_until IS NULL OR lease_until <= ?)", []string{"tombstoned", "cleanup_pending", "cleanup_claimed"}, now).
			Order("updated_at, tenant_id, session_id").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runtime.ErrNotFound
			}
			return err
		}
		until := now.Add(lease)
		updated := tx.Model(&executionCleanupRow{}).Where("tenant_id=? AND session_id=? AND epoch=? AND state IN ? AND (lease_until IS NULL OR lease_until <= ?)", row.TenantID, row.SessionID, row.Epoch, []string{"tombstoned", "cleanup_pending", "cleanup_claimed"}, now).
			Updates(map[string]any{"state": "cleanup_claimed", "worker": worker, "epoch": gorm.Expr("epoch+1"), "lease_until": until, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return runtime.ErrConflict
		}
		claim = CleanupClaim{TenantID: row.TenantID, OwnerID: row.OwnerID, SessionID: row.SessionID, DeletionRevision: row.DeletionRevision, Worker: worker, Epoch: row.Epoch + 1}
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
		canPurge := execution.CanPurge(execution.CleanupFacts{Stopped: stopped, Settled: settled, RetentionElapsed: retained})
		if canPurge {
			if s.cleanupFiles == nil {
				return ErrCleanupFilesUnavailable
			}
			// The W26 adapter is idempotent and tenant-scoped. It runs before
			// durable row deletion so a file failure leaves the claim retryable.
			if err := s.cleanupFiles.PurgeSessionFiles(ctx, claim); err != nil {
				return err
			}
			if err := purgeExecutionRecords(tx, claim); err != nil {
				return err
			}
			state = "purged"
		}
		updated := tx.Model(&executionCleanupRow{}).
			Where("tenant_id=? AND session_id=? AND owner_id=? AND deletion_revision=? AND worker=? AND epoch=? AND state='cleanup_claimed'", claim.TenantID, claim.SessionID, claim.OwnerID, claim.DeletionRevision, claim.Worker, claim.Epoch).
			Updates(map[string]any{"state": state, "stopped": stopped, "settled": settled, "retention_elapsed": retained, "worker": "", "lease_until": nil, "updated_at": s.cleanupNowExpr()})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return runtime.ErrLeaseLost
		}
		return nil
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

var ErrCleanupFilesUnavailable = errors.New("execution cleanup file purger unavailable")

// SetCleanupFilePurger installs the W26-owned file/blob/backup adapter. The
// default container leaves it unset while that dependency is blocked.
func (s *AgentRunStore) SetCleanupFilePurger(purger CleanupFilePurger) {
	if s != nil {
		s.cleanupFiles = purger
	}
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
	return s.CompleteCleanup(ctx, claim, execution.CleanupFacts{Stopped: stop && replay, Settled: usage, RetentionElapsed: retention && restore})
}

// NewDurableCleanupObservationSource builds the default fail-closed source.
// It reads product run/event/observation state only; remote provider, backup,
// and object-store adapters must be added through their owning tasks.
func NewDurableCleanupObservationSource(db *gorm.DB) CleanupObservationSource {
	return &durableCleanupObservationSource{db: db}
}

type durableCleanupObservationSource struct{ db *gorm.DB }

func (o *durableCleanupObservationSource) ObserveStop(ctx context.Context, claim CleanupClaim) (bool, error) {
	var active int64
	err := o.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND session_id=? AND status NOT IN ?", claim.TenantID, claim.SessionID, []string{"canceled", "succeeded", "failed"}).Count(&active).Error
	return active == 0, err
}

func (o *durableCleanupObservationSource) ObserveUsage(ctx context.Context, claim CleanupClaim) (bool, error) {
	var runs int64
	if err := o.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND session_id=?", claim.TenantID, claim.SessionID).Count(&runs).Error; err != nil {
		return false, err
	}
	if runs == 0 {
		return false, nil
	}
	var usage int64
	err := o.db.WithContext(ctx).Table("agent_run_events e").Joins("JOIN agent_runs r ON r.tenant_id=e.tenant_id AND r.run_id=e.run_id").Where("e.tenant_id=? AND r.session_id=? AND e.event_type LIKE 'usage.%'", claim.TenantID, claim.SessionID).Count(&usage).Error
	return usage > 0, err
}

func (o *durableCleanupObservationSource) ObserveReplay(ctx context.Context, claim CleanupClaim) (bool, error) {
	var incomplete int64
	err := o.db.WithContext(ctx).Table("execution_observations o").Joins("JOIN agent_runs r ON r.tenant_id=o.tenant_id AND r.run_id=o.run_id").Where("o.tenant_id=? AND r.session_id=? AND o.history_incomplete = ?", claim.TenantID, claim.SessionID, true).Count(&incomplete).Error
	if err != nil && !strings.Contains(err.Error(), "no such table") {
		return false, err
	}
	return incomplete == 0, nil
}

func (o *durableCleanupObservationSource) ObserveBackupRestore(context.Context, CleanupClaim) (bool, error) {
	// W26 backup/object receipt is not available in this checkout.
	return false, nil
}

func (o *durableCleanupObservationSource) ObserveRetention(context.Context, CleanupClaim) (bool, error) {
	return false, nil
}

func (s *AgentRunStore) cleanupNowExpr() any {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("CURRENT_TIMESTAMP")
	}
	return gorm.Expr("CURRENT_TIMESTAMP")
}
