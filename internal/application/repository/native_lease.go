package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NativeLeaseStore owns the native run lease linearization point. Every
// executing mutation must present the current owner and epoch; an expired
// worker can therefore never overwrite the worker that recovered its run.
type NativeLeaseStore struct{ db *gorm.DB }

func NewNativeLeaseStore(db *gorm.DB) *NativeLeaseStore { return &NativeLeaseStore{db: db} }

func nativeLeaseScope(db *gorm.DB, run nativecontract.RunIdentity) *gorm.DB {
	return db.Table("native_agent_runs").Where("tenant_id = ? AND run_id = ?", run.TenantID, run.RunID)
}

func (s *NativeLeaseStore) nowSQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday('now')"
	}
	return "clock_timestamp()"
}

func (s *NativeLeaseStore) expirySQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday(lease_expires_at)"
	}
	return "lease_expires_at"
}

func (s *NativeLeaseStore) expiry(ttl time.Duration) clause.Expr {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("strftime('%Y-%m-%d %H:%M:%f', 'now', ?)", fmt.Sprintf("+%.3f seconds", ttl.Seconds()))
	}
	return gorm.Expr("clock_timestamp() + (? * interval '1 second')", ttl.Seconds())
}

func nativeLeaseFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

func validNativeLeaseRun(run nativecontract.RunIdentity) bool {
	return run.TenantID != 0 && run.RunID != ""
}

// Claim takes either a newly queued run or a running run whose lease expired.
// The UPDATE predicate is the compare-and-swap: readers never make a claim
// decision before the write that establishes its epoch.
func (s *NativeLeaseStore) Claim(ctx context.Context, run nativecontract.RunIdentity, owner string, now time.Time, ttl time.Duration) (nativecontract.Fence, error) {
	if s == nil || s.db == nil {
		return nativecontract.Fence{}, nativeLeaseFailure(nativecontract.ErrStore, "native lease store is unavailable")
	}
	if !validNativeLeaseRun(run) || owner == "" || now.IsZero() || ttl < time.Millisecond {
		return nativecontract.Fence{}, nativeLeaseFailure(nativecontract.ErrInvalid, "run, owner, current time, and positive lease duration are required")
	}
	var fence nativecontract.Fence
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimable := "(status = 'queued' OR (status = 'running' AND lease_expires_at IS NOT NULL AND " + s.expirySQL() + " <= " + s.nowSQL() + "))"
		result := nativeLeaseScope(tx, run).Where(claimable).Updates(map[string]any{
			"lease_owner": owner, "lease_epoch": gorm.Expr("lease_epoch + 1"), "lease_expires_at": s.expiry(ttl),
			"revision": gorm.Expr("revision + 1"), "status": string(nativecontract.RunRunning), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nativeLeaseFailure(nativecontract.ErrLeaseLost, "run lease is not claimable")
		}
		var row struct {
			LeaseEpoch     int64
			LeaseExpiresAt time.Time
		}
		if err := nativeLeaseScope(tx, run).Select("lease_epoch, lease_expires_at").Take(&row).Error; err != nil {
			return err
		}
		fence = nativecontract.Fence{Run: run, Owner: owner, Epoch: row.LeaseEpoch, LeaseUntil: row.LeaseExpiresAt}
		return nil
	})
	return fence, err
}

func (s *NativeLeaseStore) fenced(db *gorm.DB, fence nativecontract.Fence) *gorm.DB {
	return nativeLeaseScope(db, fence.Run).Where(
		"lease_owner = ? AND lease_epoch = ? AND status = 'running' AND lease_expires_at IS NOT NULL AND "+s.expirySQL()+" > "+s.nowSQL(),
		fence.Owner, fence.Epoch,
	)
}

// Renew extends only a live lease held by exactly the presented fence. It
// cannot revive an expiry or make a stale epoch current. now is part of the
// frozen worker API; the database clock remains the comparison authority so
// every contender evaluates expiry against one shared clock.
func (s *NativeLeaseStore) Renew(ctx context.Context, fence nativecontract.Fence, now time.Time, ttl time.Duration) (nativecontract.Fence, error) {
	if s == nil || s.db == nil {
		return nativecontract.Fence{}, nativeLeaseFailure(nativecontract.ErrStore, "native lease store is unavailable")
	}
	if !validNativeLeaseRun(fence.Run) || fence.Owner == "" || fence.Epoch <= 0 || now.IsZero() || ttl < time.Millisecond {
		return nativecontract.Fence{}, nativeLeaseFailure(nativecontract.ErrInvalid, "live fence, current time, and positive lease duration are required")
	}
	var renewed nativecontract.Fence
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := s.fenced(tx, fence).Updates(map[string]any{
			"lease_expires_at": s.expiry(ttl), "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nativeLeaseFailure(nativecontract.ErrLeaseLost, "run fence is stale")
		}
		var row struct{ LeaseExpiresAt time.Time }
		if err := nativeLeaseScope(tx, fence.Run).Where("lease_owner = ? AND lease_epoch = ?", fence.Owner, fence.Epoch).Select("lease_expires_at").Take(&row).Error; err != nil {
			return err
		}
		renewed = fence
		renewed.LeaseUntil = row.LeaseExpiresAt
		return nil
	})
	return renewed, err
}

// Transition applies one worker-owned completion or wait transition. Clearing
// the lease atomically with the status prevents a worker that just parked or
// finalized the run from mutating it again.
func (s *NativeLeaseStore) Transition(ctx context.Context, fence nativecontract.Fence, status nativecontract.RunStatus) error {
	if s == nil || s.db == nil {
		return nativeLeaseFailure(nativecontract.ErrStore, "native lease store is unavailable")
	}
	if !validNativeLeaseRun(fence.Run) || fence.Owner == "" || fence.Epoch <= 0 {
		return nativeLeaseFailure(nativecontract.ErrInvalid, "live fence is required")
	}
	switch status {
	case nativecontract.RunWaiting, nativecontract.RunSucceeded, nativecontract.RunFailed, nativecontract.RunCancelled:
	default:
		return nativeLeaseFailure(nativecontract.ErrConflict, "native run transition is not allowed")
	}
	result := s.fenced(s.db.WithContext(ctx), fence).Updates(map[string]any{
		"status": string(status), "lease_owner": "", "lease_expires_at": nil,
		"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return nativeLeaseFailure(nativecontract.ErrLeaseLost, "run fence is stale")
	}
	return nil
}

// ScanRecoverable reports only expired in-flight runs. Queued work belongs to
// normal dispatch; waiting and terminal runs require their explicit decision
// or completion paths and are never revived by recovery scanning.
func (s *NativeLeaseStore) ScanRecoverable(ctx context.Context, limit int) ([]nativecontract.RunIdentity, error) {
	if s == nil || s.db == nil {
		return nil, nativeLeaseFailure(nativecontract.ErrStore, "native lease store is unavailable")
	}
	if limit <= 0 {
		return nil, nativeLeaseFailure(nativecontract.ErrInvalid, "recovery scan limit is required")
	}
	var runs []nativecontract.RunIdentity
	err := s.db.WithContext(ctx).Table("native_agent_runs").Select("tenant_id, run_id, session_id").
		Where("status = 'running' AND lease_expires_at IS NOT NULL AND " + s.expirySQL() + " <= " + s.nowSQL()).
		Order("updated_at ASC, tenant_id ASC, run_id ASC").Limit(limit).Scan(&runs).Error
	return runs, err
}
