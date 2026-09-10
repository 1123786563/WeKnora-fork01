package repository

import (
	"context"
	"encoding/json"
	"errors"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
)

type agentRunDecisionRow struct {
	TenantID                                 uint64
	RunID, DecisionID, PendingID, ToolCallID string
	ExpectedRevision                         int64
	ActorID, Action, Result, Reason          string
	Applied                                  bool
}

func (agentRunDecisionRow) TableName() string { return "agent_run_decisions" }

// ApplyDecision atomically consumes one pending run decision. The revision CAS
// makes concurrent users mutually exclusive; replaying the same decision ID
// with the same payload returns the already committed run.
func (s *AgentRunStore) ApplyDecision(
	ctx context.Context, key agentruntime.RunKey, actor string, d agentruntime.Decision,
) (agentruntime.Run, error) {
	if key.TenantID == 0 || key.RunID == "" || actor == "" || d.PendingID == "" || d.DecisionID == "" {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	if d.Action != "retry" && d.Action != "provide_result" && d.Action != "terminate" {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	if d.Action == "provide_result" && (len(d.Result) == 0 || !json.Valid(d.Result)) {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	var out agentruntime.Run
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run agentRunRow
		if err := runScope(tx, key).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if run.OwnerID != actor {
			return agentruntime.ErrNotFound
		}
		var prior agentRunDecisionRow
		priorErr := tx.Where(
			"tenant_id = ? AND run_id = ? AND decision_id = ?",
			key.TenantID, key.RunID, d.DecisionID,
		).Take(&prior).Error
		if priorErr == nil {
			if prior.PendingID != d.PendingID || prior.ExpectedRevision != d.ExpectedRevision ||
				prior.ActorID != actor || prior.Action != d.Action || prior.Reason != d.Reason ||
				prior.Result != string(d.Result) {
				return agentruntime.ErrConflict
			}
			out = run.view()
			return nil
		}
		if !errors.Is(priorErr, gorm.ErrRecordNotFound) {
			return priorErr
		}
		if run.Status != "waiting_user" || run.WaitReason != d.PendingID || run.Revision != d.ExpectedRevision {
			return agentruntime.ErrConflict
		}
		r := agentRunDecisionRow{
			TenantID: key.TenantID, RunID: key.RunID, DecisionID: d.DecisionID,
			PendingID: d.PendingID, ExpectedRevision: d.ExpectedRevision,
			ActorID: actor, Action: d.Action, Result: string(d.Result), Reason: d.Reason, Applied: true,
		}
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		status := "queued"
		if d.Action == "terminate" {
			status = "canceled"
		}
		upd := runScope(tx, key).Where(
			"revision = ? AND status = 'waiting_user' AND wait_reason = ?",
			d.ExpectedRevision, d.PendingID,
		).Updates(map[string]any{
			"status": status, "wait_reason": "", "lease_owner": "", "lease_until": nil,
			"revision": gorm.Expr("revision + 1"),
		})
		if upd.Error != nil {
			return upd.Error
		}
		if upd.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		if err := runScope(tx, key).Take(&run).Error; err != nil {
			return err
		}
		out = run.view()
		return nil
	})
	return out, err
}
