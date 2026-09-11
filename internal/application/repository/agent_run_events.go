package repository

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentRunEventRow struct {
	TenantID                      uint64
	RunID                         string
	Seq                           int64
	AttemptID, EventType, Payload string
}

func (agentRunEventRow) TableName() string { return "agent_run_events" }

// AppendEvent allocates the next sequence while holding the run's fenced row.
func (s *AgentRunStore) AppendEvent(ctx context.Context, fence agentruntime.Fence, evt agentruntime.RunEvent) (agentruntime.RunEvent, error) {
	if evt.Type == "" || len(evt.Type) > 128 || len(evt.Payload) == 0 || !json.Valid(evt.Payload) {
		return agentruntime.RunEvent{}, agentruntime.ErrConflict
	}
	var out agentruntime.RunEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			return err
		}
		var last agentRunEventRow
		e := tx.Where("tenant_id=? AND run_id=?", fence.TenantID, fence.RunID).Order("seq DESC").Take(&last).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		seq := last.Seq + 1
		row := agentRunEventRow{TenantID: fence.TenantID, RunID: fence.RunID, Seq: seq, AttemptID: evt.AttemptID, EventType: evt.Type, Payload: string(evt.Payload)}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		out = agentruntime.RunEvent{Seq: seq, AttemptID: evt.AttemptID, Type: evt.Type, Payload: append(json.RawMessage(nil), evt.Payload...)}
		return nil
	})
	return out, err
}

func (s *AgentRunStore) ReadEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]agentruntime.RunEvent, error) {
	if limit <= 0 {
		return nil, agentruntime.ErrConflict
	}
	if after < 0 {
		return nil, agentruntime.ErrCursorExpired
	}
	var total int64
	if err := s.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND run_id=?", key.TenantID, key.RunID).Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, agentruntime.ErrNotFound
	}
	var first agentRunEventRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=?", key.TenantID, key.RunID).Order("seq ASC").Take(&first).Error; err == nil && first.Seq > after+1 {
		return nil, agentruntime.ErrCursorExpired
	}
	var rows []agentRunEventRow
	q := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=? AND seq>?", key.TenantID, key.RunID, after).Order("seq ASC").Limit(limit).Find(&rows)
	if q.Error != nil {
		return nil, q.Error
	}
	out := make([]agentruntime.RunEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, agentruntime.RunEvent{Seq: r.Seq, AttemptID: r.AttemptID, Type: r.EventType, Payload: json.RawMessage(r.Payload)})
	}
	return out, nil
}

// Finalize atomically completes the assistant message, run, completion event, and slot.
func (s *AgentRunStore) Finalize(ctx context.Context, fence agentruntime.Fence, answer json.RawMessage) error {
	if len(answer) == 0 || !json.Valid(answer) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			var terminal agentRunRow
			if e := runScope(tx, fence.RunKey).Where("status = ?", "succeeded").Take(&terminal).Error; e == nil {
				return nil
			}
			return agentruntime.ErrLeaseLost
		}
		var run agentRunRow
		if err := runScope(tx, fence.RunKey).Take(&run).Error; err != nil {
			return err
		}
		var obj struct {
			Content string `json:"content"`
			Answer  string `json:"answer"`
		}
		if err := json.Unmarshal(answer, &obj); err != nil {
			return err
		}
		if obj.Content == "" {
			obj.Content = obj.Answer
		}
		if obj.Content == "" {
			obj.Content = string(answer)
		}
		// Idempotent finalize: a terminal run is already committed.
		if run.Status == "succeeded" {
			return nil
		}
		if run.AssistantMessageID != "" {
			if err := tx.Table("messages").Where("id=? AND session_id=?", run.AssistantMessageID, run.SessionID).Updates(map[string]any{"content": obj.Content, "is_completed": true}).Error; err != nil {
				return err
			}
		}
		if err := s.fenced(tx, fence).Updates(map[string]any{"status": "succeeded", "wait_reason": "", "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision+1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type='run_completed'", fence.TenantID, fence.RunID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			payload := string(answer)
			if err := tx.Create(&agentRunEventRow{TenantID: fence.TenantID, RunID: fence.RunID, Seq: nextEventSeq(tx, fence), EventType: "run_completed", Payload: payload}).Error; err != nil {
				return err
			}
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", fence.TenantID, run.SessionID, fence.RunID).Update("active_agent_run_id", nil).Error
	})
}
func nextEventSeq(tx *gorm.DB, f agentruntime.Fence) int64 {
	var r agentRunEventRow
	if tx.Where("tenant_id=? AND run_id=?", f.TenantID, f.RunID).Order("seq DESC").Take(&r).Error != nil {
		return 1
	}
	return r.Seq + 1
}
