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
	TenantID  uint64
	SessionID string
	Worker    string
	Epoch     int64
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
		var session struct {
			OwnerID string `gorm:"column:user_id"`
		}
		if err := tx.Table("sessions").Select("user_id").Where("tenant_id=? AND id=? AND user_id=?", tenant, sessionID, owner).Take(&session).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return runtime.ErrNotFound
			}
			return err
		}
		var existing executionCleanupRow
		err := tx.Where("tenant_id=? AND session_id=?", tenant, sessionID).Take(&existing).Error
		if err == nil {
			if existing.OwnerID != owner {
				return runtime.ErrNotFound
			}
			if existing.State == "purged" {
				return nil
			}
			return tx.Model(&executionCleanupRow{}).Where("tenant_id=? AND session_id=? AND owner_id=?", tenant, sessionID, owner).
				Updates(map[string]any{"state": "tombstoned", "worker": "", "lease_until": nil, "updated_at": s.cleanupNowExpr()}).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
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
		claim = CleanupClaim{TenantID: row.TenantID, SessionID: row.SessionID, Worker: worker, Epoch: row.Epoch + 1}
		return nil
	})
	return claim, err
}

// CompleteCleanup records observed stop/settlement/retention facts under the
// claim fence. Purging is only marked after all three facts are true.
func (s *AgentRunStore) CompleteCleanup(ctx context.Context, claim CleanupClaim, facts execution.CleanupFacts) error {
	if claim.TenantID == 0 || claim.SessionID == "" || claim.Worker == "" || claim.Epoch <= 0 {
		return runtime.ErrConflict
	}
	state := "cleanup_pending"
	if execution.CanPurge(facts) {
		state = "purged"
	}
	updated := s.db.WithContext(ctx).Model(&executionCleanupRow{}).
		Where("tenant_id=? AND session_id=? AND worker=? AND epoch=? AND state='cleanup_claimed'", claim.TenantID, claim.SessionID, claim.Worker, claim.Epoch).
		Updates(map[string]any{"state": state, "stopped": facts.Stopped, "settled": facts.Settled, "retention_elapsed": facts.RetentionElapsed, "worker": "", "lease_until": nil, "updated_at": s.cleanupNowExpr()})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
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
