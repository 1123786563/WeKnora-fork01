package repository

import (
	"context"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
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
		payload := `{"reason":"` + reason + `"}`
		if err := tx.Create(&agentRunEventRow{TenantID: key.TenantID, RunID: key.RunID, Seq: nextEventSeq(tx, agentruntime.Fence{RunKey: key}), EventType: "cancellation_requested", Payload: payload}).Error; err != nil {
			return err
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", key.TenantID, run.SessionID, key.RunID).Update("active_agent_run_id", nil).Error
	})
}

// DeleteSessionRuns cancels first, then removes dependent durable records in a
// single transaction. Deleting the rows makes stale fences fail closed.
func (s *AgentRunStore) DeleteSessionRuns(ctx context.Context, tenantID uint64, sessionID string) error {
	if tenantID == 0 || sessionID == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Table("agent_runs").Where("tenant_id=? AND session_id=?", tenantID, sessionID).Pluck("run_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			// Mark before deleting so concurrent workers cannot commit a terminal state.
			if err := tx.Table("agent_runs").Where("tenant_id=? AND run_id=?", tenantID, id).Updates(map[string]any{"status": "canceled", "wait_reason": "session_deleted", "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision+1")}).Error; err != nil {
				return err
			}
			for _, table := range []string{"agent_run_inputs", "agent_run_decisions", "agent_run_events", "agent_tool_attempts", "agent_tool_calls", "agent_run_checkpoints"} {
				if !tx.Migrator().HasTable(table) {
					continue
				}
				if err := tx.Exec("DELETE FROM "+table+" WHERE tenant_id=? AND run_id=?", tenantID, id).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec("DELETE FROM agent_runs WHERE tenant_id=? AND run_id=?", tenantID, id).Error; err != nil {
				return err
			}
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=?", tenantID, sessionID).Update("active_agent_run_id", nil).Error
	})
}
