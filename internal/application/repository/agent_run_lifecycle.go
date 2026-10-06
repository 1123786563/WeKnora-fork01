package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CancelRun atomically marks a run terminal and releases its session slot.
// A canceled row remains durable so a stale worker's fenced writes fail.
func (s *AgentRunStore) CancelRun(ctx context.Context, key agentruntime.RunKey, reason string) error {
	if key.TenantID == 0 || key.RunID == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.cancelRunTx(tx, key, reason)
	})
}

func (s *AgentRunStore) cancelRunTx(tx *gorm.DB, key agentruntime.RunKey, reason string) error {
	if err := lockRunTransitionRow(tx, key); err != nil {
		if errors.Is(err, agentruntime.ErrNotFound) {
			return agentruntime.ErrNotFound
		}
		return err
	}
	var run agentRunRow
	if err := runScope(tx, key).Take(&run).Error; err != nil {
		return err
	}
	if err := rejectUnresolvedCraftRunViewEffects(tx, key); err != nil {
		return err
	}
	if run.Status == "canceled" {
		return nil
	}
	if run.Status == "succeeded" || run.Status == "failed" {
		return agentruntime.ErrConflict
	}
	unresolved, err := hasUnresolvedCraftChargeStart(tx, key)
	if err != nil {
		return err
	}
	if unresolved {
		if run.Status == "reconciling" && run.WaitReason == craftChargeStartPendingWaitReason {
			var cancelRequests int64
			if err := tx.Table("agent_run_events").Where(
				"tenant_id = ? AND run_id = ? AND event_type = ?", key.TenantID, key.RunID, "craft_charge_cancel_requested",
			).Count(&cancelRequests).Error; err != nil {
				return err
			}
			if cancelRequests != 0 {
				return nil
			}
		}
		if err := requestCraftChargeCancel(tx, key, reason); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]string{"reason": reason})
		return appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "cancellation_requested", string(payload))
	}
	if err := runScope(tx, key).Updates(map[string]any{"status": "canceled", "wait_reason": reason, "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision+1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
		return err
	}
	payloadBytes, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "cancellation_requested", string(payloadBytes)); err != nil {
		return err
	}
	return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", key.TenantID, run.SessionID, key.RunID).Update("active_agent_run_id", nil).Error
}

// CancelRunOwnedAtRevision applies the Workbench's authenticated revision
// fence through the same journal-aware cancellation transition as RunStore.
func (s *AgentRunStore) CancelRunOwnedAtRevision(ctx context.Context, tenantID uint64, ownerID, runID string, revision int64, reason string) error {
	if tenantID == 0 || ownerID == "" || runID == "" || revision < 0 {
		return agentruntime.ErrConflict
	}
	key := agentruntime.RunKey{TenantID: tenantID, RunID: runID}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := runScope(tx, key).Where("owner_id = ? AND revision = ? AND status IN ('queued','running','waiting_user','reconciling','recovering')", ownerID, revision).
			UpdateColumn("revision", gorm.Expr("revision"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		return s.cancelRunTx(tx, key, reason)
	})
}

// DeleteSessionRuns writes the session deletion barrier and fences its runs.
// Durable run/control/usage rows are retained until the cleanup worker proves
// stop, settlement, and retention; deleting them here would lose late facts.
func (s *AgentRunStore) DeleteSessionRuns(ctx context.Context, tenantID uint64, sessionID string) error {
	if tenantID == 0 || sessionID == "" {
		return agentruntime.ErrConflict
	}
	var owner string
	if err := s.db.WithContext(ctx).Table("sessions").Select("user_id").Where("tenant_id=? AND id=?", tenantID, sessionID).Scan(&owner).Error; err != nil {
		return err
	}
	if owner == "" {
		return agentruntime.ErrNotFound
	}
	if err := s.TombstoneSession(ctx, tenantID, owner, sessionID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		// Only non-terminal runs are cancelled here: cancelRunTx refuses
		// succeeded/failed runs (their settlement and retention facts must
		// survive), and a session that merely completed work must stay
		// deletable. Canceled runs are the idempotent no-op below.
		if err := tx.Table("agent_runs").
			Where("tenant_id=? AND session_id=? AND status NOT IN ('succeeded','failed')", tenantID, sessionID).
			Pluck("run_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := s.cancelRunTx(tx, agentruntime.RunKey{TenantID: tenantID, RunID: id}, "session_deleted"); err != nil {
				if errors.Is(err, agentruntime.ErrConflict) {
					// The run settled BETWEEN the pluck and this locked
					// cancel — exactly the race the comment above names.
					// Its settlement facts survive by design; skipping (not
					// aborting the whole delete) keeps a session that merely
					// completed work deletable.
					continue
				}
				return err
			}
		}
		var pending int64
		if err := tx.Table("agent_runs").Where("tenant_id = ? AND session_id = ? AND status = 'reconciling' AND wait_reason = ?",
			tenantID, sessionID, craftChargeStartPendingWaitReason).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return nil
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=?", tenantID, sessionID).Update("active_agent_run_id", nil).Error
	})
}

// CancelOwnedRun is the workbench command-surface cancellation: a revision-CAS
// transition to canceled that also writes the durable cancellation_requested
// run event and releases the session's active-run slot in the same
// transaction. The slot release is what makes a later restart (queue_next on
// the terminal run) admissible; the run event is what makes the stop request a
// timeline fact clients can present. A foreign owner is indistinguishable from
// a missing run: uniform not-found, zero side effects.
func (s *AgentRunStore) CancelOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64, reason string) error {
	if tenantID == 0 || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(runID) == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run agentRunRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND run_id = ?", tenantID, strings.TrimSpace(runID)).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if run.OwnerID != strings.TrimSpace(ownerID) {
			return agentruntime.ErrNotFound
		}
		if run.Status == "canceled" {
			return nil // idempotent, same as CancelRun
		}
		if run.Status == "succeeded" || run.Status == "failed" || run.Revision != expectedRevision {
			return agentruntime.ErrConflict
		}
		if err := tx.Model(&agentRunRow{}).
			Where("tenant_id = ? AND run_id = ? AND revision = ?", tenantID, run.RunID, run.Revision).
			Updates(map[string]any{"status": "canceled", "wait_reason": reason, "lease_owner": "", "lease_until": nil,
				"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
			return err
		}
		payloadBytes, err := json.Marshal(map[string]string{"reason": reason})
		if err != nil {
			return err
		}
		if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: tenantID, RunID: run.RunID}}, "cancellation_requested", string(payloadBytes)); err != nil {
			return err
		}
		return tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id = ?",
			tenantID, run.SessionID, run.RunID).Update("active_agent_run_id", nil).Error
	})
}
