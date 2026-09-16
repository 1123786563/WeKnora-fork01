package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSourceConflict = errors.New("execution source event payload conflict")
	ErrSourceBinding  = errors.New("execution source binding not found")
)

type SourceObservation struct {
	BindingID, Generation, EventID, AttemptID, Type, PayloadHash string
	Payload                                                      json.RawMessage
	SourceSeq                                                    int64
}

type executionObservationRow struct {
	TenantID                              uint64 `gorm:"primaryKey;column:tenant_id"`
	RunID, BindingID, Generation, EventID string
	AttemptID, EventType, PayloadHash     string
	Payload                               []byte
	ProductSeq                            int64
	SourceSeq                             int64
	HistoryIncomplete                     bool
	ConfirmedSnapshot                     []byte
	CreatedAt                             time.Time
}

func (executionObservationRow) TableName() string { return "execution_observations" }

type executionBindingRow struct {
	TenantID  uint64
	RunID     string
	CommandID string
}

// ExecutionObservationStore turns provider observations into the product event log.
// Binding resolution and source de-duplication happen inside the same transaction as
// the product seq allocation, so reconnects cannot advance the cursor twice.
type ExecutionObservationStore struct{ db *gorm.DB }

func NewExecutionObservationStore(db *gorm.DB) *ExecutionObservationStore {
	return &ExecutionObservationStore{db: db}
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
func observationType(t string) string {
	if strings.HasPrefix(t, "text.") || strings.HasPrefix(t, "thought.") || strings.HasPrefix(t, "tool.") || strings.HasPrefix(t, "run.") || strings.HasPrefix(t, "execution.") || strings.HasPrefix(t, "approval.") || strings.HasPrefix(t, "usage.") || strings.HasPrefix(t, "artifact.") || t == "error" {
		return t
	}
	return "unknown"
}

func (s *ExecutionObservationStore) resolveBinding(ctx context.Context, tx *gorm.DB, bindingID string) (agentruntime.RunKey, error) {
	var dispatch executionBindingRow
	err := tx.WithContext(ctx).Table("execution_dispatches").Select("tenant_id, run_id, command_id").Where("command_id = ?", bindingID).Take(&dispatch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// A provider may use the durable run id as its binding after dispatch recovery.
		err = tx.WithContext(ctx).Table("agent_runs").Select("tenant_id, run_id").Where("run_id = ?", bindingID).Take(&dispatch).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.RunKey{}, ErrSourceBinding
	}
	if err != nil {
		return agentruntime.RunKey{}, err
	}
	return agentruntime.RunKey{TenantID: dispatch.TenantID, RunID: dispatch.RunID}, nil
}

func (s *ExecutionObservationStore) IngestSourceEvent(ctx context.Context, bindingID string, source SourceObservation) (workbench.ExecutionEvent, error) {
	if s == nil || s.db == nil || strings.TrimSpace(bindingID) == "" || strings.TrimSpace(source.Generation) == "" || strings.TrimSpace(source.EventID) == "" || strings.TrimSpace(source.Type) == "" || len(source.Payload) == 0 || !json.Valid(source.Payload) {
		return workbench.ExecutionEvent{}, agentruntime.ErrConflict
	}
	if source.PayloadHash == "" {
		source.PayloadHash = hashPayload(source.Payload)
	}
	if source.PayloadHash != hashPayload(source.Payload) {
		return workbench.ExecutionEvent{}, ErrSourceConflict
	}
	var result workbench.ExecutionEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		key, err := s.resolveBinding(ctx, tx, bindingID)
		if err != nil {
			return err
		}
		lock := tx.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID)
		if tx.Dialector.Name() == "postgres" {
			lock = lock.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var run struct{ RunID string }
		if err := lock.Select("run_id").Take(&run).Error; err != nil {
			return err
		}
		var existing executionObservationRow
		err = tx.WithContext(ctx).Where("tenant_id = ? AND binding_id = ? AND generation = ? AND event_id = ?", key.TenantID, bindingID, source.Generation, source.EventID).Take(&existing).Error
		if err == nil {
			if existing.PayloadHash != source.PayloadHash {
				return ErrSourceConflict
			}
			result = toExecutionEvent(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var max struct{ Seq int64 }
		if err := tx.WithContext(ctx).Table("agent_run_events").Select("COALESCE(MAX(seq), 0) AS seq").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Scan(&max).Error; err != nil {
			return err
		}
		seq := max.Seq + 1
		if seq < 1 || seq > workbench.MaxSafeInteger {
			return workbench.ErrSequenceOverflow
		}
		typ := observationType(source.Type)
		payload := source.Payload
		if typ == "unknown" {
			payload, _ = json.Marshal(map[string]any{"source_type": source.Type, "payload": json.RawMessage(source.Payload)})
		}
		now := time.Now().UTC()
		incomplete := source.SourceSeq > 0 && source.SourceSeq > seq
		row := executionObservationRow{TenantID: key.TenantID, RunID: key.RunID, BindingID: bindingID, Generation: source.Generation, EventID: source.EventID, AttemptID: source.AttemptID, EventType: typ, PayloadHash: source.PayloadHash, Payload: payload, ProductSeq: seq, SourceSeq: source.SourceSeq, HistoryIncomplete: incomplete, ConfirmedSnapshot: append([]byte(nil), payload...), CreatedAt: now}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		event := executionEventFor(key.RunID, source.AttemptID, seq, typ, payload, now)
		if err := tx.WithContext(ctx).Create(&snapshotEventRow{TenantID: key.TenantID, RunID: key.RunID, Seq: seq, AttemptID: source.AttemptID, EventType: typ, Payload: payload, CreatedAt: now}).Error; err != nil {
			return err
		}
		result = event
		return nil
	})
	return result, err
}

func toExecutionEvent(row executionObservationRow) workbench.ExecutionEvent {
	return executionEventFor(row.RunID, row.AttemptID, row.ProductSeq, row.EventType, row.Payload, row.CreatedAt)
}
func executionEventFor(runID, attemptID string, seq int64, typ string, payload []byte, at time.Time) workbench.ExecutionEvent {
	return workbench.ExecutionEvent{SchemaVersion: 1, RunID: runID, AttemptID: attemptID, Seq: seq, Type: typ, OccurredAt: at.UTC().Format(time.RFC3339Nano), Payload: append([]byte(nil), payload...)}
}
