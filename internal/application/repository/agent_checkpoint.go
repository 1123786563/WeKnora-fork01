package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ agentruntime.CheckpointStore = (*AgentRunStore)(nil)

// UpdateCheckpoint atomically reads and changes a checkpoint under the run's
// fence. Serializing on the run row prevents concurrent pending-write updates
// from losing each other, even when they use distinct saver handles.
func (s *AgentRunStore) UpdateCheckpoint(ctx context.Context, fence agentruntime.Fence, namespace, id string,
	mutate func(*agentruntime.CheckpointRecord, int64) (*agentruntime.CheckpointRecord, error),
) error {
	if namespace == "" || id == "" || mutate == nil {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockCheckpointRun(tx, fence); err != nil {
			return err
		}
		var existing agentCheckpointRow
		err := tx.Where("tenant_id=? AND run_id=? AND namespace=? AND checkpoint_id=?",
			fence.TenantID, fence.RunID, namespace, id).Take(&existing).Error
		var current *agentruntime.CheckpointRecord
		if err == nil {
			record := checkpointRecord(existing)
			current = &record
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var next int64
		if err := tx.Table("agent_run_checkpoints").Select("COALESCE(MAX(seq),-1)+1").
			Where("tenant_id=? AND run_id=? AND namespace=?", fence.TenantID, fence.RunID, namespace).
			Scan(&next).Error; err != nil {
			return err
		}
		updated, err := mutate(current, next)
		if err != nil {
			return err
		}
		if updated == nil || updated.ID != id || updated.Namespace != namespace || updated.Seq < 0 ||
			!json.Valid(updated.State) || !json.Valid(updated.PendingWrites) {
			return agentruntime.ErrConflict
		}
		row := agentCheckpointRow{
			TenantID: fence.TenantID, RunID: fence.RunID, Namespace: namespace, CheckpointID: id,
			ParentID: updated.ParentID, Seq: updated.Seq,
			State: string(updated.State), PendingWrites: string(updated.PendingWrites),
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "run_id"}, {Name: "namespace"}, {Name: "checkpoint_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"parent_id", "seq", "state", "pending_writes", "updated_at"}),
		}).Create(&row).Error
	})
}

func (s *AgentRunStore) lockCheckpointRun(tx *gorm.DB, fence agentruntime.Fence) error {
	locked := s.fenced(tx, fence).Updates(map[string]any{
		"revision":   gorm.Expr("revision+1"),
		"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if locked.Error != nil {
		return locked.Error
	}
	if locked.RowsAffected != 1 {
		return agentruntime.ErrLeaseLost
	}
	return nil
}

// ListCheckpoints returns a tenant/run/namespace's records newest first.
func (s *AgentRunStore) ListCheckpoints(ctx context.Context, key agentruntime.RunKey, namespace string) (
	[]agentruntime.CheckpointRecord, error,
) {
	var rows []agentCheckpointRow
	err := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=? AND namespace=?", key.TenantID, key.RunID, namespace).
		Order("seq DESC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]agentruntime.CheckpointRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, checkpointRecord(row))
	}
	return result, nil
}

// DeleteCheckpoints removes only the fenced run's selected namespace.
func (s *AgentRunStore) DeleteCheckpoints(ctx context.Context, fence agentruntime.Fence, namespace string) error {
	if namespace == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockCheckpointRun(tx, fence); err != nil {
			return err
		}
		return tx.Where("tenant_id=? AND run_id=? AND namespace=?", fence.TenantID, fence.RunID, namespace).
			Delete(&agentCheckpointRow{}).Error
	})
}

// ValidateCheckpointCalls fails recovery when referenced journals are missing,
// or an applied result has no durable successful/failed result to reuse.
func (s *AgentRunStore) ValidateCheckpointCalls(ctx context.Context, key agentruntime.RunKey,
	pending []string, applied map[string]bool,
) error {
	ids := make(map[string]bool)
	for _, id := range pending {
		ids[id] = true
	}
	for id, done := range applied {
		if done {
			ids[id] = true
		}
	}
	for id := range ids {
		var row struct {
			Status    string
			Result    *string
			ResultRef string
		}
		err := s.db.WithContext(ctx).Table("agent_tool_calls").Select("status,result,result_ref").
			Where("tenant_id=? AND run_id=? AND call_id=?", key.TenantID, key.RunID, id).Take(&row).Error
		if err != nil {
			return fmt.Errorf("checkpoint tool journal %s: %w", id, err)
		}
		if applied[id] && ((row.Status != "succeeded" && row.Status != "failed") ||
			(row.Result == nil && row.ResultRef == "")) {
			return fmt.Errorf("checkpoint tool result %s is not durable", id)
		}
	}
	return nil
}

func checkpointRecord(row agentCheckpointRow) agentruntime.CheckpointRecord {
	return agentruntime.CheckpointRecord{
		Namespace: row.Namespace, ID: row.CheckpointID, ParentID: row.ParentID,
		Seq: row.Seq, State: json.RawMessage(row.State), PendingWrites: json.RawMessage(row.PendingWrites),
	}
}
