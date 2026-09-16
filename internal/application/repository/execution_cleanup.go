package repository

import (
	"context"
	"errors"
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
		if execution.CanPurge(execution.CleanupFacts{Stopped: stopped, Settled: settled, RetentionElapsed: retained}) {
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

// RunCleanupOnce executes one leased observation cycle. It intentionally does
// not fake provider/file evidence; callers must supply real adapters.
func (s *AgentRunStore) RunCleanupOnce(ctx context.Context, worker string, lease time.Duration, source CleanupObservationSource) error {
	if source == nil {
		return runtime.ErrConflict
	}
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

func (s *AgentRunStore) cleanupNowExpr() any {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("CURRENT_TIMESTAMP")
	}
	return gorm.Expr("CURRENT_TIMESTAMP")
}
