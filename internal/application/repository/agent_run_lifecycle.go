package repository

import (
	"context"
	"encoding/json"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
)

// CancelRun atomically marks a run terminal and releases its session slot.
// A canceled row remains durable so a stale worker's fenced writes fail.
func (s *AgentRunStore) CancelRun(ctx context.Context, key agentruntime.RunKey, reason string) error {
	if key.TenantID == 0 || key.RunID == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run agentRunRow
		if err := runScope(tx, key).Take(&run).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if run.Status == "canceled" {
			return nil
		}
		if run.Status == "succeeded" || run.Status == "failed" {
			return agentruntime.ErrConflict
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
		if err := tx.Table("agent_runs").Where("tenant_id=? AND session_id=?", tenantID, sessionID).Pluck("run_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := tx.Table("agent_runs").Where("tenant_id=? AND run_id=?", tenantID, id).
				Updates(map[string]any{"status": "canceled", "wait_reason": "session_deleted", "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision+1")}).Error; err != nil {
				return err
			}
			payload, err := json.Marshal(map[string]string{"reason": "session_deleted"})
			if err != nil {
				return err
			}
			if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: tenantID, RunID: id}}, "cancellation_requested", string(payload)); err != nil {
				return err
			}
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=?", tenantID, sessionID).Update("active_agent_run_id", nil).Error
	})
}
