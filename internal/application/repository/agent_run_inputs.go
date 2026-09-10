package repository

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type agentRunInputRow struct {
	TenantID                              uint64
	RunID, SteerID, Mode, Message, Status string
	Cursor                                int64
	CreatedAt                             time.Time
	ProcessedAt                           *time.Time
}

func (agentRunInputRow) TableName() string { return "agent_run_inputs" }
func (s *AgentRunStore) AppendInput(ctx context.Context, key agentruntime.RunKey, in agentruntime.RunInput) error {
	if in.SteerID == "" || in.Mode != "inject" && in.Mode != "after" || len(in.Message) == 0 || !json.Valid(in.Message) {
		return agentruntime.ErrConflict
	}
	row := agentRunInputRow{TenantID: key.TenantID, RunID: key.RunID, SteerID: in.SteerID, Mode: in.Mode, Message: string(in.Message)}
	e := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if e.Error != nil {
		return e.Error
	}
	if e.RowsAffected == 0 {
		var old agentRunInputRow
		if er := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=? AND steer_id=?", key.TenantID, key.RunID, in.SteerID).Take(&old).Error; er != nil {
			return er
		}
		if old.Mode != in.Mode || old.Message != string(in.Message) {
			return agentruntime.ErrConflict
		}
	}
	return nil
}
func (s *AgentRunStore) ApplyInput(ctx context.Context, fence agentruntime.Fence, steerID string, cp agentruntime.CheckpointRecord) error {
	if steerID == "" || cp.ID == "" || !json.Valid(cp.State) || !json.Valid(cp.PendingWrites) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			return err
		}
		var in agentRunInputRow
		if err := tx.Where("tenant_id=? AND run_id=? AND steer_id=?", fence.TenantID, fence.RunID, steerID).Take(&in).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if in.Status == "processed" {
			return nil
		}
		now := time.Now()
		if err := tx.Table("agent_run_inputs").Where("tenant_id=? AND run_id=? AND steer_id=?", fence.TenantID, fence.RunID, steerID).Updates(map[string]any{"status": "processed", "processed_at": now, "cursor": gorm.Expr("cursor+1")}).Error; err != nil {
			return err
		}
		row := agentCheckpointRow{TenantID: fence.TenantID, RunID: fence.RunID, Namespace: cp.Namespace, CheckpointID: cp.ID, ParentID: cp.ParentID, Seq: cp.Seq, State: string(cp.State), PendingWrites: string(cp.PendingWrites)}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "run_id"}, {Name: "namespace"}, {Name: "checkpoint_id"}}, DoUpdates: clause.AssignmentColumns([]string{"parent_id", "seq", "state", "pending_writes", "updated_at"})}).Create(&row).Error
	})
}
