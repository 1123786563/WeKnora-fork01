package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"gorm.io/gorm"
)

// AgentRunSnapshotRepository reads the durable run and event projection. It
// deliberately does not reconstruct a snapshot from chat stream buffers:
// those buffers are mutable and may already have been trimmed.
type AgentRunSnapshotRepository struct {
	db           *gorm.DB
	capabilities CapabilityResolver
}

// CapabilityResolver is the runtime/provider capability boundary. A
// persisted driver is an intent and cannot by itself advertise availability.
type CapabilityResolver interface {
	ResolveExecutionCapabilities(ctx context.Context, selectedDriver string) map[string]workbench.Capability
}

type environmentCapabilityResolver struct{}

func (environmentCapabilityResolver) ResolveExecutionCapabilities(_ context.Context, _ string) map[string]workbench.Capability {
	capabilities := map[string]workbench.Capability{
		"platform": {State: workbench.CapabilityUnavailable, Reason: "platform_not_configured"},
		"paseo":    {State: workbench.CapabilityUnavailable, Reason: "PASEO_URL_not_configured"},
	}
	engine := strings.ToLower(strings.TrimSpace(os.Getenv("WEKNORA_AGENT_ENGINE")))
	if engine == "" || engine == "trpc" {
		capabilities["platform"] = workbench.Capability{State: workbench.CapabilitySupported}
	} else {
		capabilities["platform"] = workbench.Capability{State: workbench.CapabilityUnavailable, Reason: "unsupported_agent_engine"}
	}
	if strings.TrimSpace(os.Getenv("PASEO_URL")) != "" {
		capabilities["paseo"] = workbench.Capability{State: workbench.CapabilitySupported}
	}
	return capabilities
}

func NewAgentRunSnapshotRepository(db *gorm.DB) *AgentRunSnapshotRepository {
	return NewAgentRunSnapshotRepositoryWithResolver(db, environmentCapabilityResolver{})
}

func NewAgentRunSnapshotRepositoryWithResolver(db *gorm.DB, resolver CapabilityResolver) *AgentRunSnapshotRepository {
	if resolver == nil {
		resolver = environmentCapabilityResolver{}
	}
	return &AgentRunSnapshotRepository{db: db, capabilities: resolver}
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

func executionFromRun(ctx context.Context, resolver CapabilityResolver, row snapshotRunRow, seq int64) workbench.ExecutionDTO {
	settlement := "pending"
	switch row.Status {
	case "succeeded", "failed", "canceled":
		settlement = "settled"
	}
	driver := row.Driver
	if driver == "" {
		driver = "platform"
	}
	capabilities := resolver.ResolveExecutionCapabilities(ctx, driver)
	if capabilities == nil {
		capabilities = map[string]workbench.Capability{driver: {State: workbench.CapabilityUnavailable, Reason: "capability_probe_unavailable"}}
	}
	return workbench.ExecutionDTO{
		SchemaVersion: 1, RunID: row.RunID, SessionID: row.SessionID,
		Revision: row.Revision, Driver: driver, RunStatus: row.Status,
		ExecutionStatus: row.Status, SettlementStatus: settlement, Seq: seq,
		Capabilities: capabilities,
	}
}

func projectExecutionEvents(execution workbench.ExecutionDTO, events []workbench.ExecutionEvent) workbench.ExecutionDTO {
	// The durable run row remains authoritative for a terminal outcome. While a
	// run is active, however, provider observations can advance the product
	// projection before the worker updates agent_runs; expose that progress in
	// the same DTO consumed by the mobile client.
	if execution.RunStatus == "queued" || execution.RunStatus == "running" || execution.RunStatus == "reconciling" {
		succeeded, failed, canceled := false, false, false
		for _, event := range events {
			switch event.Type {
			case "run.completed", "execution.succeeded", "status.succeeded":
				succeeded = true
			case "run.failed", "execution.failed", "status.failed":
				failed = true
			case "run.canceled", "execution.canceled", "status.canceled":
				canceled = true
			}
		}
		switch {
		case canceled:
			execution.RunStatus, execution.ExecutionStatus = "canceled", "canceled"
		case failed:
			execution.RunStatus, execution.ExecutionStatus = "failed", "failed"
		case succeeded:
			execution.RunStatus, execution.ExecutionStatus = "succeeded", "succeeded"
		}
		if canceled || failed || succeeded {
			execution.SettlementStatus = "settled"
		}
	}
	return execution
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
	// PostgreSQL needs an explicit repeatable-read snapshot: READ COMMITTED
	// could observe the run row and event projection at different commits. A
	// SQLite transaction is already a consistent read snapshot; passing a
	// PostgreSQL isolation level to SQLite is driver-dependent, so begin with
	// the default there.
	var txOptions *sql.TxOptions
	if s.db.Name() != "sqlite" {
		txOptions = &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}
	}
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
		var observation struct {
			Incomplete        int
			ConfirmedSeq      int64
			ConfirmedSnapshot []byte
		}
		var sourceRows []executionObservationRow
		_ = tx.Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Find(&sourceRows).Error
		for _, sourceRow := range sourceRows {
			if sourceRow.SourceSeq <= 0 {
				continue
			}
			var cursor executionSourceCursorRow
			if tx.Where("tenant_id = ? AND binding_id = ? AND generation = ?", key.TenantID, sourceRow.BindingID, sourceRow.Generation).Take(&cursor).Error == nil && sourceRow.SourceSeq > cursor.LastConfirmedSeq {
				observation.Incomplete = 1
				if cursor.LastConfirmedSeq > observation.ConfirmedSeq {
					observation.ConfirmedSeq = cursor.LastConfirmedSeq
					observation.ConfirmedSnapshot = cursor.ConfirmedSnapshot
				}
			}
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
		resolver := s.capabilities
		if resolver == nil {
			resolver = environmentCapabilityResolver{}
		}
		execution := projectExecutionEvents(executionFromRun(ctx, resolver, run, watermark), events)
		incomplete := observation.Incomplete > 0 || (len(events) > 0 && events[0].Seq > 1)
		confirmed := observation.ConfirmedSeq
		if confirmed < 0 {
			confirmed = 0
		}
		if confirmed == 0 && !incomplete {
			confirmed = watermark
		}
		if incomplete && len(observation.ConfirmedSnapshot) > 0 {
			var fallback []workbench.ExecutionEvent
			if json.Unmarshal(observation.ConfirmedSnapshot, &fallback) == nil && len(fallback) > 0 {
				events = fallback
			}
		}
		result = workbench.ExecutionSnapshot{Execution: execution, Watermark: watermark, Incomplete: incomplete, ConfirmedWatermark: confirmed, Events: events}
		return result.Validate()
	}, txOptions)
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
	expectedSeq := cursor + 1
	for _, row := range rows {
		if row.Seq != expectedSeq {
			return nil, watermark, agentruntime.ErrCursorExpired
		}
		expectedSeq++
		event := workbench.ExecutionEvent{SchemaVersion: 1, RunID: row.RunID, AttemptID: row.AttemptID, Seq: row.Seq, Type: row.EventType, OccurredAt: row.CreatedAt.UTC().Format(time.RFC3339Nano), Payload: append([]byte(nil), row.Payload...)}
		if err := event.Validate(); err != nil {
			return nil, watermark, err
		}
		events = append(events, event)
	}
	return events, watermark, nil
}
