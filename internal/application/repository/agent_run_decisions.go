package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	for attempt := 0; attempt < 3; attempt++ {
		run, err := s.applyDecisionOnce(ctx, key, actor, d)
		if err == nil || !isSQLiteLock(err) {
			return run, err
		}
		select {
		case <-ctx.Done():
			return agentruntime.Run{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 15 * time.Millisecond):
		}
	}
	return agentruntime.Run{}, agentruntime.ErrConflict
}

func isSQLiteLock(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") || strings.Contains(s, "database table is locked")
}

func (s *AgentRunStore) applyDecisionOnce(
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
		var pending struct{ CallID, ArgsHash string }
		pendingErr := tx.Table("agent_tool_calls").Select("call_id, args_hash").Where(
			"tenant_id = ? AND run_id = ? AND status = 'unknown' AND (unknown_reason = ? OR call_id = ?)",
			key.TenantID, key.RunID, d.PendingID, d.PendingID,
		).Take(&pending).Error
		if pendingErr == nil {
			if d.ArgsHash == "" || d.ArgsHash != pending.ArgsHash {
				return agentruntime.ErrConflict
			}
		} else if !errors.Is(pendingErr, gorm.ErrRecordNotFound) {
			return pendingErr
		}
		r := agentRunDecisionRow{
			TenantID: key.TenantID, RunID: key.RunID, DecisionID: d.DecisionID,
			PendingID: d.PendingID, ExpectedRevision: d.ExpectedRevision,
			ActorID: actor, Action: d.Action, Result: string(d.Result), Reason: d.Reason, Applied: true,
		}
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		if pendingErr == nil {
			if d.Action == "provide_result" {
				updated := tx.Table("agent_tool_calls").Where(
					"tenant_id = ? AND run_id = ? AND call_id = ? AND status = 'unknown'",
					key.TenantID, key.RunID, pending.CallID,
				).Updates(map[string]any{"result": string(d.Result), "source": "user", "status": "succeeded", "unknown_reason": ""})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return agentruntime.ErrConflict
				}
			} else if d.Action == "retry" {
				var last struct{ Attempt int }
				if err := tx.Table("agent_tool_attempts").Select("attempt").Where(
					"tenant_id = ? AND run_id = ? AND call_id = ?", key.TenantID, key.RunID, pending.CallID,
				).Order("attempt DESC").Take(&last).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				attempt := last.Attempt + 1
				row := agentToolAttemptRow{TenantID: key.TenantID, RunID: key.RunID, CallID: pending.CallID, Attempt: attempt, Epoch: run.Epoch, Status: "planned"}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			}
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
