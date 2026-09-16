package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"gorm.io/gorm"
)

// AgentRunSnapshotRepository reads the durable run and event projection. It
// deliberately does not reconstruct a snapshot from chat stream buffers:
// those buffers are mutable and may already have been trimmed.
type AgentRunSnapshotRepository struct{ db *gorm.DB }

func NewAgentRunSnapshotRepository(db *gorm.DB) *AgentRunSnapshotRepository {
	return &AgentRunSnapshotRepository{db: db}
}

type snapshotRunRow struct {
	TenantID                         uint64
	RunID, SessionID, Driver, Status string
	Revision                         int64
}

func (snapshotRunRow) TableName() string { return "agent_runs" }

type snapshotEventRow struct {
	TenantID  uint64
	RunID     string
	Seq       int64
	AttemptID string
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

func (snapshotEventRow) TableName() string { return "agent_run_events" }

func executionFromRun(row snapshotRunRow, seq int64) workbench.ExecutionDTO {
	settlement := "pending"
	switch row.Status {
	case "succeeded", "failed", "canceled":
		settlement = "settled"
	}
	driver := row.Driver
	if driver == "" {
		driver = "platform"
	}
	capabilities := map[string]workbench.Capability{
		"platform": {State: workbench.CapabilityUnavailable, Reason: "not_selected"},
		"paseo":    {State: workbench.CapabilityUnavailable, Reason: "not_selected"},
	}
	capabilities[driver] = workbench.Capability{State: workbench.CapabilitySupported}
	return workbench.ExecutionDTO{
		SchemaVersion: 1, RunID: row.RunID, SessionID: row.SessionID,
		Revision: row.Revision, Driver: driver, RunStatus: row.Status,
		ExecutionStatus: row.Status, SettlementStatus: settlement, Seq: seq,
		Capabilities: capabilities,
	}
}

// ReadRunSnapshot reads the run row and all retained events in one database
// transaction. The transaction gives callers a single watermark and prevents
// a concurrent event append from producing a snapshot whose execution seq is
// newer than the event projection it contains.
func (s *AgentRunSnapshotRepository) ReadRunSnapshot(ctx context.Context, key agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	if s == nil || s.db == nil || key.TenantID == 0 || key.RunID == "" {
		return workbench.ExecutionSnapshot{}, agentruntime.ErrNotFound
	}
	var result workbench.ExecutionSnapshot
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run snapshotRunRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		var rows []snapshotEventRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
			Order("seq ASC").Find(&rows).Error; err != nil {
			return err
		}
		events := make([]workbench.ExecutionEvent, 0, len(rows))
		var watermark int64
		for _, row := range rows {
			if row.Seq > watermark {
				watermark = row.Seq
			}
			event := workbench.ExecutionEvent{
				SchemaVersion: 1, RunID: row.RunID, AttemptID: row.AttemptID,
				Seq: row.Seq, Type: row.EventType, OccurredAt: row.CreatedAt.UTC().Format(time.RFC3339Nano),
				Payload: append([]byte(nil), row.Payload...),
			}
			if err := event.Validate(); err != nil {
				return fmt.Errorf("agent_run_events seq %d: %w", row.Seq, err)
			}
			events = append(events, event)
		}
		result = workbench.ExecutionSnapshot{Execution: executionFromRun(run, watermark), Watermark: watermark, Events: events}
		return result.Validate()
	})
	return result, err
}

// ReadRunEvents returns retained events after cursor and the current
// watermark. A cursor before the retained lower bound is explicit rather than
// silently replaying an incomplete history.
func (s *AgentRunSnapshotRepository) ReadRunEvents(ctx context.Context, key agentruntime.RunKey, cursor int64, limit int) ([]workbench.ExecutionEvent, int64, error) {
	if s == nil || s.db == nil || key.TenantID == 0 || key.RunID == "" || cursor < 0 {
		return nil, 0, agentruntime.ErrNotFound
	}
	if limit <= 0 || limit > 256 {
		limit = 256
	}
	var rows []snapshotEventRow
	query := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND seq > ?", key.TenantID, key.RunID, cursor).Order("seq ASC").Limit(limit)
	if err := query.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	var watermark int64
	if err := s.db.WithContext(ctx).Model(&snapshotEventRow{}).Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Select("COALESCE(MAX(seq), 0)").Scan(&watermark).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) > 0 && rows[0].Seq > cursor+1 {
		return nil, watermark, agentruntime.ErrCursorExpired
	}
	events := make([]workbench.ExecutionEvent, 0, len(rows))
	for _, row := range rows {
		event := workbench.ExecutionEvent{SchemaVersion: 1, RunID: row.RunID, AttemptID: row.AttemptID, Seq: row.Seq, Type: row.EventType, OccurredAt: row.CreatedAt.UTC().Format(time.RFC3339Nano), Payload: append([]byte(nil), row.Payload...)}
		if err := event.Validate(); err != nil {
			return nil, watermark, err
		}
		events = append(events, event)
	}
	return events, watermark, nil
}
