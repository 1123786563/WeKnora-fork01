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
	TenantID         uint64 `gorm:"column:tenant_id"`
	RunID            string `gorm:"column:run_id"`
	DecisionID       string `gorm:"column:decision_id"`
	PendingID        string `gorm:"column:pending_id"`
	ToolCallID       string `gorm:"column:tool_call_id"`
	ExpectedRevision int64  `gorm:"column:expected_revision"`
	ActorID          string `gorm:"column:actor_id"`
	Action           string `gorm:"column:action"`
	Result           string `gorm:"column:result"`
	Reason           string `gorm:"column:reason"`
	ArgsHash         string `gorm:"column:args_hash"`
	ResourceRef      string `gorm:"column:resource_ref"`
	Applied          bool   `gorm:"column:applied"`
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
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
			if replay, ok := s.replayDecision(ctx, key, actor, d); ok {
				return replay, nil
			}
		}
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

func (s *AgentRunStore) replayDecision(ctx context.Context, key agentruntime.RunKey, actor string, d agentruntime.Decision) (agentruntime.Run, bool) {
	var row agentRunDecisionRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=? AND decision_id=?", key.TenantID, key.RunID, d.DecisionID).Take(&row).Error; err != nil {
		return agentruntime.Run{}, false
	}
	if row.PendingID != d.PendingID || row.ToolCallID != d.ToolCallID || row.ActorID != actor || row.Action != d.Action || row.Reason != d.Reason || row.ArgsHash != d.ArgsHash || row.ResourceRef != d.ResourceRef || row.Result != string(d.Result) || row.ExpectedRevision != d.ExpectedRevision {
		return agentruntime.Run{}, false
	}
	run, err := s.Get(ctx, key)
	return run, err == nil
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
			var mismatch int64
			if err := tx.Table("agent_run_decisions").Where(
				"tenant_id = ? AND run_id = ? AND decision_id = ? AND (reason <> ? OR action <> ? OR pending_id <> ? OR actor_id <> ? OR expected_revision <> ?)",
				key.TenantID, key.RunID, d.DecisionID, d.Reason, d.Action, d.PendingID, actor, d.ExpectedRevision,
			).Count(&mismatch).Error; err != nil {
				return err
			}
			if mismatch != 0 {
				return agentruntime.ErrConflict
			}
			var stored struct {
				Reason, ToolCallID, ArgsHash, ResourceRef, Result, Action, PendingID, ActorID string
				ExpectedRevision                                                              int64
			}
			if err := tx.Table("agent_run_decisions").Select("reason,tool_call_id,args_hash,resource_ref,result,action,pending_id,actor_id,expected_revision").Where("tenant_id = ? AND run_id = ? AND decision_id = ?", key.TenantID, key.RunID, d.DecisionID).Scan(&stored).Error; err != nil {
				return err
			}
			if prior.PendingID != d.PendingID || prior.ToolCallID != d.ToolCallID || prior.ExpectedRevision != d.ExpectedRevision ||
				prior.ActorID != actor || prior.Action != d.Action || prior.Reason != d.Reason ||
				prior.Result != string(d.Result) || prior.ArgsHash != d.ArgsHash || prior.ResourceRef != d.ResourceRef {
				return agentruntime.ErrConflict
			}
			if stored.Reason != d.Reason || stored.ToolCallID != d.ToolCallID || stored.ArgsHash != d.ArgsHash || stored.ResourceRef != d.ResourceRef || stored.Result != string(d.Result) || stored.Action != d.Action || stored.PendingID != d.PendingID || stored.ActorID != actor || stored.ExpectedRevision != d.ExpectedRevision {
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
		pendingQuery := tx.Table("agent_tool_calls").Select("call_id, args_hash").Where("tenant_id = ? AND run_id = ? AND status = 'unknown'", key.TenantID, key.RunID)
		if d.ToolCallID != "" {
			pendingQuery = pendingQuery.Where("call_id = ?", d.ToolCallID)
		} else {
			pendingQuery = pendingQuery.Where("unknown_reason = ? OR unknown_reason LIKE ? OR call_id = ?", d.PendingID, d.PendingID+"|%", d.PendingID)
		}
		var pendingRows []struct{ CallID, ArgsHash string }
		pendingErr := pendingQuery.Find(&pendingRows).Error
		if pendingErr == nil {
			if len(pendingRows) > 1 {
				return agentruntime.ErrConflict
			}
			if len(pendingRows) == 1 {
				pending = pendingRows[0]
			}
			if len(pendingRows) == 1 && d.ToolCallID == "" {
				return agentruntime.ErrConflict
			}
			if len(pendingRows) == 0 && d.ToolCallID != "" {
				return agentruntime.ErrConflict
			}
		}
		if pendingErr == nil && len(pendingRows) == 1 {
			if d.ArgsHash == "" || d.ArgsHash != pending.ArgsHash {
				return agentruntime.ErrConflict
			}
		} else if d.ToolCallID != "" || !errors.Is(pendingErr, gorm.ErrRecordNotFound) {
			if errors.Is(pendingErr, gorm.ErrRecordNotFound) {
				return agentruntime.ErrConflict
			}
			return pendingErr
		}
		if err := tx.Exec("INSERT INTO agent_run_decisions (tenant_id,run_id,decision_id,pending_id,tool_call_id,expected_revision,actor_id,action,result,reason,args_hash,resource_ref,applied) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", key.TenantID, key.RunID, d.DecisionID, d.PendingID, pending.CallID, d.ExpectedRevision, actor, d.Action, string(d.Result), d.Reason, d.ArgsHash, d.ResourceRef, true).Error; err != nil {
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
				planned := tx.Table("agent_tool_calls").Where("tenant_id = ? AND run_id = ? AND call_id = ? AND status = 'unknown'", key.TenantID, key.RunID, pending.CallID).Updates(map[string]any{"status": "planned", "unknown_reason": d.Reason})
				if planned.Error != nil {
					return planned.Error
				}
				if planned.RowsAffected != 1 {
					return agentruntime.ErrConflict
				}
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
