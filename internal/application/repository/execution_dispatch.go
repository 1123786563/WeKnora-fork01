package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrDispatchBusy      = errors.New("execution dispatch lease is held")
	ErrDispatchConflict  = errors.New("execution dispatch conflicts with the durable record")
	ErrDispatchLeaseLost = errors.New("execution dispatch lease lost")
)

// DispatchRecord is the durable intent/receipt for one provider command.
// A claimed record is written before the provider is called. A crash between
// that write and SaveReceipt therefore remains unknown and is never retried
// by guessing that the provider did not start.
type DispatchRecord struct {
	CommandID, RunID, AttemptID, PayloadHash, State, ExternalID string
	TenantID                                                    uint64
	Epoch                                                       int64
}

type executionDispatchRow struct {
	TenantID      uint64     `gorm:"primaryKey;column:tenant_id"`
	CommandID     string     `gorm:"primaryKey;column:command_id"`
	RunID         string     `gorm:"column:run_id"`
	AttemptID     string     `gorm:"column:attempt_id"`
	PayloadHash   string     `gorm:"column:payload_hash"`
	State         string     `gorm:"column:state"`
	ExternalID    string     `gorm:"column:external_id"`
	Worker        string     `gorm:"column:worker"`
	Epoch         int64      `gorm:"column:epoch"`
	LeaseUntil    *time.Time `gorm:"column:lease_until"`
	ObservedState string     `gorm:"column:observed_state"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (executionDispatchRow) TableName() string { return "execution_dispatches" }

func (r executionDispatchRow) record() DispatchRecord {
	return DispatchRecord{TenantID: r.TenantID, CommandID: r.CommandID, RunID: r.RunID, AttemptID: r.AttemptID, PayloadHash: r.PayloadHash, State: r.State, ExternalID: r.ExternalID, Epoch: r.Epoch}
}

// ExecutionDispatchStore persists provider intents and receipts. It is kept
// separate from AgentRunStore so recovery can scan commands without invoking
// the provider or changing the run state speculatively.
type ExecutionDispatchStore struct{ db *gorm.DB }

func NewExecutionDispatchStore(db *gorm.DB) *ExecutionDispatchStore {
	return &ExecutionDispatchStore{db: db}
}

func (s *ExecutionDispatchStore) lockRun(ctx context.Context, tx *gorm.DB, key agentruntime.RunKey) (int64, error) {
	var run struct{ Epoch int64 }
	q := tx.WithContext(ctx).Table("agent_runs").Select("epoch").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID)
	if tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Take(&run).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, agentruntime.ErrNotFound
	} else if err != nil {
		return 0, err
	}
	return run.Epoch, nil
}

func (s *ExecutionDispatchStore) getLocked(ctx context.Context, tx *gorm.DB, key agentruntime.RunKey, commandID string) (executionDispatchRow, error) {
	var row executionDispatchRow
	q := tx.WithContext(ctx).Where("tenant_id = ? AND command_id = ?", key.TenantID, commandID)
	if tx.Dialector.Name() == "postgres" {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return executionDispatchRow{}, gorm.ErrRecordNotFound
	}
	return row, err
}

// ClaimDispatch writes the started intent before any provider call. Repeating
// a live claim by the same worker is idempotent; another worker must wait for
// the lease to expire, and a changed run epoch can never reuse the old fence.
func (s *ExecutionDispatchStore) ClaimDispatch(ctx context.Context, key agentruntime.RunKey, commandID, worker string, lease time.Duration) (DispatchRecord, error) {
	if key.TenantID == 0 || key.RunID == "" || strings.TrimSpace(commandID) == "" || strings.TrimSpace(worker) == "" || lease <= 0 {
		return DispatchRecord{}, agentruntime.ErrConflict
	}
	var result DispatchRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		epoch, err := s.lockRun(ctx, tx, key)
		if err != nil {
			return err
		}
		row, err := s.getLocked(ctx, tx, key, commandID)
		now := time.Now().UTC()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = executionDispatchRow{TenantID: key.TenantID, CommandID: commandID, RunID: key.RunID, AttemptID: commandID, State: "claimed", Worker: worker, Epoch: epoch, LeaseUntil: ptrTime(now.Add(lease))}
			if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
			result = row.record()
			return nil
		}
		if err != nil {
			return err
		}
		if row.RunID != key.RunID || row.Epoch != epoch {
			return ErrDispatchLeaseLost
		}
		if row.State == "completed" || row.State == "reconciled" {
			result = row.record()
			return nil
		}
		if row.LeaseUntil != nil && row.LeaseUntil.After(now) && row.Worker != worker {
			return ErrDispatchBusy
		}
		row.State, row.Worker, row.LeaseUntil = "claimed", worker, ptrTime(now.Add(lease))
		if err := tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", key.TenantID, commandID).Updates(map[string]any{"state": row.State, "worker": worker, "lease_until": row.LeaseUntil, "updated_at": now}).Error; err != nil {
			return err
		}
		result = row.record()
		return nil
	})
	return result, err
}

// SaveReceipt is the only transition that proves a provider external id was
// returned. It rechecks tenant, run and epoch inside the same transaction.
func (s *ExecutionDispatchStore) SaveReceipt(ctx context.Context, record DispatchRecord, externalID string) error {
	if record.TenantID == 0 || record.CommandID == "" || record.RunID == "" || strings.TrimSpace(externalID) == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		epoch, err := s.lockRun(ctx, tx, agentruntime.RunKey{TenantID: record.TenantID, RunID: record.RunID})
		if err != nil {
			return err
		}
		if epoch != record.Epoch {
			return ErrDispatchLeaseLost
		}
		row, err := s.getLocked(ctx, tx, agentruntime.RunKey{TenantID: record.TenantID, RunID: record.RunID}, record.CommandID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if row.RunID != record.RunID || row.AttemptID != record.AttemptID || row.Epoch != record.Epoch {
			return ErrDispatchConflict
		}
		if row.ExternalID != "" && row.ExternalID != externalID {
			return ErrDispatchConflict
		}
		if row.State == "completed" && row.ExternalID == externalID {
			return nil
		}
		now := time.Now().UTC()
		return tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", record.TenantID, record.CommandID).Updates(map[string]any{"state": "completed", "external_id": externalID, "lease_until": nil, "updated_at": now}).Error
	})
}

// ReconcileUnknown records a provider observation after a lost response. An
// observation with no external id remains unknown; it is never treated as a
// safe-to-retry start. An identified provider process is marked reconciled.
func (s *ExecutionDispatchStore) ReconcileUnknown(ctx context.Context, record DispatchRecord, observedState, externalID string) error {
	if record.TenantID == 0 || record.CommandID == "" || record.RunID == "" || strings.TrimSpace(observedState) == "" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		epoch, err := s.lockRun(ctx, tx, agentruntime.RunKey{TenantID: record.TenantID, RunID: record.RunID})
		if err != nil {
			return err
		}
		if epoch != record.Epoch {
			return ErrDispatchLeaseLost
		}
		row, err := s.getLocked(ctx, tx, agentruntime.RunKey{TenantID: record.TenantID, RunID: record.RunID}, record.CommandID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return agentruntime.ErrNotFound
			}
			return err
		}
		if row.RunID != record.RunID || row.AttemptID != record.AttemptID || row.Epoch != record.Epoch {
			return ErrDispatchConflict
		}
		if row.ExternalID != "" && externalID != "" && row.ExternalID != externalID {
			return ErrDispatchConflict
		}
		state := "unknown"
		if strings.TrimSpace(externalID) != "" {
			state = "reconciled"
		}
		now := time.Now().UTC()
		return tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", record.TenantID, record.CommandID).Updates(map[string]any{"state": state, "external_id": externalID, "observed_state": observedState, "lease_until": nil, "updated_at": now}).Error
	})
}

func ptrTime(t time.Time) *time.Time { return &t }
