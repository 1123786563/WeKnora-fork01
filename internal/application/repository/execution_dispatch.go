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
	ErrDispatchUnknown   = errors.New("execution dispatch outcome is unknown")
)

// DispatchRecord is the durable intent/receipt for one provider command.
// A claimed record is written before the provider is called. A crash between
// that write and SaveReceipt therefore remains unknown and is never retried
// by guessing that the provider did not start.
type DispatchRecord struct {
	CommandID, RunID, AttemptID, PayloadHash, State, ExternalID string
	TenantID                                                    uint64
	Epoch                                                       int64
	Worker                                                      string
	LeaseUntil                                                  time.Time
	New                                                         bool
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
	result := DispatchRecord{TenantID: r.TenantID, CommandID: r.CommandID, RunID: r.RunID, AttemptID: r.AttemptID, PayloadHash: r.PayloadHash, State: r.State, ExternalID: r.ExternalID, Epoch: r.Epoch, Worker: r.Worker}
	if r.LeaseUntil != nil {
		result.LeaseUntil = r.LeaseUntil.UTC()
	}
	return result
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
	return s.ClaimDispatchWithPayloadHash(ctx, key, commandID, "", worker, lease)
}

// ClaimDispatchWithPayloadHash is the provider-facing form. A non-empty hash
// is immutable for a command id; changing it is a conflict rather than a new
// attempt under the same id.
func (s *ExecutionDispatchStore) ClaimDispatchWithPayloadHash(ctx context.Context, key agentruntime.RunKey, commandID, payloadHash, worker string, lease time.Duration) (DispatchRecord, error) {
	if key.TenantID == 0 || key.RunID == "" || strings.TrimSpace(commandID) == "" || strings.TrimSpace(worker) == "" || lease <= 0 {
		return DispatchRecord{}, agentruntime.ErrConflict
	}
	var result DispatchRecord
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			epoch, err := s.lockRun(ctx, tx, key)
			if err != nil {
				return err
			}
			row, err := s.getLocked(ctx, tx, key, commandID)
			now := time.Now().UTC()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				row = executionDispatchRow{TenantID: key.TenantID, CommandID: commandID, RunID: key.RunID, AttemptID: commandID, PayloadHash: payloadHash, State: "claimed", Worker: worker, Epoch: epoch, LeaseUntil: ptrTime(now.Add(lease))}
				if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
					return err
				}
				result = row.record()
				result.New = true
				return nil
			}
			if err != nil {
				return err
			}
			if payloadHash != "" && row.PayloadHash != "" && row.PayloadHash != payloadHash {
				return ErrDispatchConflict
			}
			if row.RunID != key.RunID || row.Epoch != epoch {
				return ErrDispatchLeaseLost
			}
			if row.PayloadHash == "" && payloadHash != "" {
				row.PayloadHash = payloadHash
			}
			if row.State == "completed" || row.State == "reconciled" {
				result = row.record()
				return nil
			}
			if row.State == "unknown" {
				return ErrDispatchUnknown
			}
			if row.LeaseUntil == nil || !row.LeaseUntil.After(now) {
				_ = tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", key.TenantID, commandID).Updates(map[string]any{"state": "unknown", "observed_state": "lease_expired", "lease_until": nil, "updated_at": now}).Error
				return ErrDispatchUnknown
			}
			if row.LeaseUntil != nil && row.LeaseUntil.After(now) && row.Worker != worker {
				return ErrDispatchBusy
			}
			row.State, row.Worker, row.LeaseUntil = "claimed", worker, ptrTime(now.Add(lease))
			updates := map[string]any{"state": row.State, "worker": worker, "lease_until": row.LeaseUntil, "updated_at": now}
			if payloadHash != "" {
				updates["payload_hash"] = payloadHash
			}
			if err := tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", key.TenantID, commandID).Updates(updates).Error; err != nil {
				return err
			}
			result = row.record()
			return nil
		})
		if err == nil || !isSQLiteBusy(s.db, err) {
			break
		}
		select {
		case <-ctx.Done():
			return DispatchRecord{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 10 * time.Millisecond):
		}
	}
	return result, err
}

func isSQLiteBusy(db *gorm.DB, err error) bool {
	return db.Dialector.Name() == "sqlite" && (strings.Contains(strings.ToLower(err.Error()), "database is locked") || strings.Contains(strings.ToLower(err.Error()), "database table is locked"))
}

// RecoverUnknown is an explicit recovery fence. It is the only operation
// that can turn an unknown/expired intent back into a claim, and only after a
// trusted observation proves that no provider process was started.
func (s *ExecutionDispatchStore) RecoverUnknown(ctx context.Context, record DispatchRecord, worker string, lease time.Duration) (DispatchRecord, error) {
	if record.TenantID == 0 || record.RunID == "" || record.CommandID == "" || strings.TrimSpace(worker) == "" || lease <= 0 {
		return DispatchRecord{}, agentruntime.ErrConflict
	}
	var result DispatchRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if row.State != "unknown" || row.ObservedState != "not_started" {
			return ErrDispatchUnknown
		}
		if record.PayloadHash != "" && row.PayloadHash != record.PayloadHash {
			return ErrDispatchConflict
		}
		now := time.Now().UTC()
		until := now.Add(lease)
		if err := tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", record.TenantID, record.CommandID).Updates(map[string]any{"state": "claimed", "worker": worker, "lease_until": until, "updated_at": now}).Error; err != nil {
			return err
		}
		row.State, row.Worker, row.LeaseUntil = "claimed", worker, &until
		result = row.record()
		result.New = true
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
		if record.PayloadHash != "" && row.PayloadHash != record.PayloadHash {
			return ErrDispatchConflict
		}
		if row.ExternalID != "" && row.ExternalID != externalID {
			return ErrDispatchConflict
		}
		if row.State == "completed" && row.ExternalID == externalID {
			return nil
		}
		if row.Worker != record.Worker || row.State != "claimed" || row.LeaseUntil == nil || !row.LeaseUntil.After(time.Now().UTC()) {
			return ErrDispatchLeaseLost
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
		// Reconciliation is a fenced write just like receipt persistence. A
		// claimant that has lost its lease must never clear or rewrite a newer
		// claimant's lease after reclaim. Epoch alone is insufficient because
		// lease turnover intentionally does not advance the run epoch.
		if row.Worker != record.Worker || (row.State != "claimed" && row.State != "unknown") || row.LeaseUntil == nil || !row.LeaseUntil.After(time.Now().UTC()) {
			return ErrDispatchLeaseLost
		}
		if row.ExternalID != "" && externalID != "" && row.ExternalID != externalID {
			return ErrDispatchConflict
		}
		state := "unknown"
		if strings.TrimSpace(externalID) != "" {
			state = "reconciled"
		}
		now := time.Now().UTC()
		updates := map[string]any{"state": state, "external_id": externalID, "observed_state": observedState, "updated_at": now}
		// Keep the current claimant's lease while the provider observation is
		// still inconclusive. This permits the same claimant to complete a
		// lookup, while an identified receipt or an expired lease closes the
		// fence and forces a deliberate recovery path.
		if strings.TrimSpace(externalID) != "" {
			updates["lease_until"] = nil
		}
		return tx.WithContext(ctx).Model(&executionDispatchRow{}).Where("tenant_id = ? AND command_id = ?", record.TenantID, record.CommandID).Updates(updates).Error
	})
}

func ptrTime(t time.Time) *time.Time { return &t }
