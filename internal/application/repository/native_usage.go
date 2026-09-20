package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
)

// NativeUsageDelta is the newly observed portion of a cumulative provider
// observation. A replay returns its zero value, so callers cannot charge it.
type NativeUsageDelta struct {
	PromptTokens, CompletionTokens, TotalTokens, CachedTokens, CacheReadTokens, CacheCreateTokens int64
	IntentID                                                                                      string
	Pending                                                                                       bool
}

type nativeUsageSettlementIntent struct {
	Observation nativecontract.UsageObservation `json:"observation"`
	Delta       NativeUsageDelta                `json:"delta"`
}

// NativeUsageLedger is the durable UsageLedger implementation. The native
// table holds the current cumulative receipt; its revision CAS is the single
// linearization point for callbacks, stream replay, and checkpoint replay.
type NativeUsageLedger struct{ db *gorm.DB }

func NewNativeUsageLedger(db *gorm.DB) *NativeUsageLedger { return &NativeUsageLedger{db: db} }

var _ nativecontract.UsageLedger = (*NativeUsageLedger)(nil)

func (s *NativeUsageLedger) Observe(ctx context.Context, fence nativecontract.Fence, observation nativecontract.UsageObservation) error {
	_, err := s.ObserveDelta(ctx, fence, observation)
	return err
}

func (s *NativeUsageLedger) ObserveDelta(ctx context.Context, fence nativecontract.Fence, o nativecontract.UsageObservation) (NativeUsageDelta, error) {
	if s == nil || s.db == nil {
		return NativeUsageDelta{}, nativeUsageFailure(nativecontract.ErrStore, "usage ledger is unavailable")
	}
	if err := validateNativeUsage(fence, o); err != nil {
		return NativeUsageDelta{}, err
	}
	var delta NativeUsageDelta
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := nativeUsageFence(tx, fence); err != nil {
			return err
		}
		var attempts int64
		if err := tx.Table("native_agent_attempts").Where("tenant_id=? AND run_id=? AND attempt_id=?", fence.Run.TenantID, fence.Run.RunID, o.AttemptID).Count(&attempts).Error; err != nil {
			return err
		}
		if attempts != 1 {
			return nativeUsageFailure(nativecontract.ErrNotFound, "usage observation attempt was not found")
		}
		payload, err := json.Marshal(o)
		if err != nil {
			return nativeUsageFailure(nativecontract.ErrInvalid, "usage observation is not durable JSON")
		}
		sum := sha256.Sum256(payload)
		hash := hex.EncodeToString(sum[:])
		dimensions, err := json.Marshal(o.Dimensions)
		if err != nil {
			return nativeUsageFailure(nativecontract.ErrInvalid, "usage dimensions are not durable JSON")
		}
		var previous struct {
			Revision, PromptTokens, CompletionTokens, CachedTokens, CacheReadTokens, CacheCreateTokens int64
			PayloadHash                                                                                string
		}
		err = tx.Table("native_agent_usage_observations").Select("revision, input_tokens AS prompt_tokens, output_tokens AS completion_tokens, cached_tokens, cache_read_tokens, cache_create_tokens, payload_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", fence.Run.TenantID, fence.Run.RunID, o.AttemptID, o.ObservationID).Take(&previous).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			created := tx.Exec(`INSERT INTO native_agent_usage_observations (tenant_id, run_id, attempt_id, observation_id, revision, provider, model, provider_request_id, funding_ref, budget_root_run_id, input_tokens, output_tokens, cached_tokens, cache_read_tokens, cache_create_tokens, accounting_status, dimensions, occurred_at, payload_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, fence.Run.TenantID, fence.Run.RunID, o.AttemptID, o.ObservationID, o.Revision, o.Funding.Service, o.Funding.PriceVersion, o.ProviderRequestID, o.Funding.BudgetRef, o.Funding.BudgetRootRunID, o.PromptTokens, o.CompletionTokens, o.CachedTokens, o.CacheReadTokens, o.CacheCreateTokens, o.AccountingStatus, string(dimensions), o.OccurredAt, hash)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected != 1 {
				// A concurrent first callback won. Re-read below so an identical
				// receipt converges on its durable settlement intent.
				if err := tx.Table("native_agent_usage_observations").Select("revision, input_tokens AS prompt_tokens, output_tokens AS completion_tokens, cached_tokens, cache_read_tokens, cache_create_tokens, payload_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", fence.Run.TenantID, fence.Run.RunID, o.AttemptID, o.ObservationID).Take(&previous).Error; err != nil {
					return err
				}
				if previous.Revision != o.Revision || previous.PayloadHash != hash {
					return nativeUsageFailure(nativecontract.ErrConflict, "usage observation identity conflict")
				}
				return s.usageSettlementIntent(tx, fence, o, hash, &delta)
			}
			delta = nativeUsageDelta(o, NativeUsageDelta{})
			return s.usageSettlementIntent(tx, fence, o, hash, &delta)
		}
		if previous.Revision == o.Revision {
			if previous.PayloadHash == hash {
				return s.usageSettlementIntent(tx, fence, o, hash, &delta)
			}
			return nativeUsageFailure(nativecontract.ErrConflict, "usage observation payload changed")
		}
		if previous.Revision > o.Revision {
			return nativeUsageFailure(nativecontract.ErrConflict, "usage observation revision is stale")
		}
		before := NativeUsageDelta{PromptTokens: previous.PromptTokens, CompletionTokens: previous.CompletionTokens, TotalTokens: previous.PromptTokens + previous.CompletionTokens, CachedTokens: previous.CachedTokens, CacheReadTokens: previous.CacheReadTokens, CacheCreateTokens: previous.CacheCreateTokens}
		delta = nativeUsageDelta(o, before)
		if delta.PromptTokens < 0 || delta.CompletionTokens < 0 || delta.TotalTokens < 0 || delta.CachedTokens < 0 || delta.CacheReadTokens < 0 || delta.CacheCreateTokens < 0 {
			return nativeUsageFailure(nativecontract.ErrConflict, "cumulative usage regressed")
		}
		updated := tx.Exec(`UPDATE native_agent_usage_observations SET revision=?, provider=?, model=?, provider_request_id=?, funding_ref=?, budget_root_run_id=?, input_tokens=?, output_tokens=?, cached_tokens=?, cache_read_tokens=?, cache_create_tokens=?, accounting_status=?, dimensions=?, occurred_at=?, payload_hash=? WHERE tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=? AND revision=?`, o.Revision, o.Funding.Service, o.Funding.PriceVersion, o.ProviderRequestID, o.Funding.BudgetRef, o.Funding.BudgetRootRunID, o.PromptTokens, o.CompletionTokens, o.CachedTokens, o.CacheReadTokens, o.CacheCreateTokens, o.AccountingStatus, string(dimensions), o.OccurredAt, hash, fence.Run.TenantID, fence.Run.RunID, o.AttemptID, o.ObservationID, previous.Revision)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			if err := tx.Table("native_agent_usage_observations").Select("revision, payload_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", fence.Run.TenantID, fence.Run.RunID, o.AttemptID, o.ObservationID).Take(&previous).Error; err != nil {
				return err
			}
			if previous.Revision != o.Revision || previous.PayloadHash != hash {
				return nativeUsageFailure(nativecontract.ErrConflict, "usage observation revision raced")
			}
			return s.usageSettlementIntent(tx, fence, o, hash, &delta)
		}
		return s.usageSettlementIntent(tx, fence, o, hash, &delta)
	})
	return delta, err
}

// usageSettlementIntent shares the native CommitIntent authority: the usage
// receipt and its pending settlement instruction are durable in one database
// transaction. Replays recover the original delta until a budget transition
// confirms the intent; they never reinterpret a failed settlement as free.
func (s *NativeUsageLedger) usageSettlementIntent(tx *gorm.DB, f nativecontract.Fence, o nativecontract.UsageObservation, observationHash string, delta *NativeUsageDelta) error {
	id := "usage-settlement:" + o.AttemptID + ":" + o.ObservationID + ":" + fmt.Sprint(o.Revision)
	var prior int64
	if err := tx.Table("native_agent_commit_intents").Where("tenant_id=? AND run_id=? AND intent_id LIKE ? AND intent_id <> ? AND state IN ?", f.Run.TenantID, f.Run.RunID, "usage-settlement:"+o.AttemptID+":"+o.ObservationID+":%", id, []string{"pending", "applying"}).Count(&prior).Error; err != nil {
		return err
	}
	if prior != 0 {
		return nativeUsageFailure(nativecontract.ErrConflict, "prior usage settlement is pending")
	}
	var row struct{ State, PayloadHash, Payload string }
	err := tx.Table("native_agent_commit_intents").Select("state, payload_hash, payload").Where("tenant_id=? AND run_id=? AND intent_id=?", f.Run.TenantID, f.Run.RunID, id).Take(&row).Error
	if err == nil {
		if row.PayloadHash != observationHash {
			return nativeUsageFailure(nativecontract.ErrConflict, "usage settlement payload changed")
		}
		if row.State == "applied" {
			*delta = NativeUsageDelta{}
		} else if row.Payload != "" {
			var stored nativeUsageSettlementIntent
			if err := json.Unmarshal([]byte(row.Payload), &stored); err != nil {
				return nativeUsageFailure(nativecontract.ErrStore, "usage settlement intent is corrupt")
			}
			*delta = stored.Delta
		}
		delta.IntentID, delta.Pending = id, row.State != "applied"
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	delta.IntentID, delta.Pending = id, true
	payload, err := json.Marshal(nativeUsageSettlementIntent{Observation: o, Delta: *delta})
	if err != nil {
		return err
	}
	created := tx.Exec(`INSERT INTO native_agent_commit_intents (tenant_id, run_id, intent_id, payload_hash, lease_epoch, state, version, payload, terminal_status) VALUES (?, ?, ?, ?, ?, 'pending', 1, ?, '') ON CONFLICT DO NOTHING`, f.Run.TenantID, f.Run.RunID, id, observationHash, f.Epoch, string(payload))
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected == 1 {
		return nil
	}
	return s.usageSettlementIntent(tx, f, o, observationHash, delta)
}

// ConfirmSettlement atomically marks the existing native commit intent
// applied. A failed budget call deliberately leaves it pending for replay.
func (s *NativeUsageLedger) ConfirmSettlement(ctx context.Context, fence nativecontract.Fence, intentID string) error {
	if intentID == "" {
		return nativeUsageFailure(nativecontract.ErrInvalid, "usage settlement intent is required")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := nativeUsageFence(tx, fence); err != nil {
			return err
		}
		updated := tx.Exec(`UPDATE native_agent_commit_intents SET state='applied', applied_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND run_id=? AND intent_id=? AND state='applying'`, fence.Run.TenantID, fence.Run.RunID, intentID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			var state string
			if err := tx.Table("native_agent_commit_intents").Select("state").Where("tenant_id=? AND run_id=? AND intent_id=?", fence.Run.TenantID, fence.Run.RunID, intentID).Take(&state).Error; err != nil {
				return nativeUsageFailure(nativecontract.ErrNotFound, "usage settlement intent was not found")
			}
			if state != "applied" {
				return nativeUsageFailure(nativecontract.ErrConflict, "usage settlement intent is not claimed")
			}
		}
		return nil
	})
}

func (s *NativeUsageLedger) ClaimSettlement(ctx context.Context, fence nativecontract.Fence, intentID string) (bool, error) {
	if intentID == "" {
		return false, nativeUsageFailure(nativecontract.ErrInvalid, "usage settlement intent is required")
	}
	claimed := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := nativeUsageFence(tx, fence); err != nil {
			return err
		}
		r := tx.Exec(`UPDATE native_agent_commit_intents SET state='applying' WHERE tenant_id=? AND run_id=? AND intent_id=? AND state='pending'`, fence.Run.TenantID, fence.Run.RunID, intentID)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 1 {
			claimed = true
			return nil
		}
		var state string
		if err := tx.Table("native_agent_commit_intents").Select("state").Where("tenant_id=? AND run_id=? AND intent_id=?", fence.Run.TenantID, fence.Run.RunID, intentID).Take(&state).Error; err != nil {
			return nativeUsageFailure(nativecontract.ErrNotFound, "usage settlement intent was not found")
		}
		if state == "applying" || state == "applied" {
			return nil
		}
		return nativeUsageFailure(nativecontract.ErrConflict, "usage settlement intent is invalid")
	})
	return claimed, err
}
func (s *NativeUsageLedger) ReleaseSettlement(ctx context.Context, fence nativecontract.Fence, intentID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := nativeUsageFence(tx, fence); err != nil {
			return err
		}
		r := tx.Exec(`UPDATE native_agent_commit_intents SET state='pending' WHERE tenant_id=? AND run_id=? AND intent_id=? AND state='applying'`, fence.Run.TenantID, fence.Run.RunID, intentID)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return nativeUsageFailure(nativecontract.ErrConflict, "usage settlement intent is not claimed")
		}
		return nil
	})
}
func nativeUsageDelta(o nativecontract.UsageObservation, before NativeUsageDelta) NativeUsageDelta {
	return NativeUsageDelta{PromptTokens: o.PromptTokens - before.PromptTokens, CompletionTokens: o.CompletionTokens - before.CompletionTokens, TotalTokens: o.TotalTokens - before.TotalTokens, CachedTokens: o.CachedTokens - before.CachedTokens, CacheReadTokens: o.CacheReadTokens - before.CacheReadTokens, CacheCreateTokens: o.CacheCreateTokens - before.CacheCreateTokens}
}
func validateNativeUsage(f nativecontract.Fence, o nativecontract.UsageObservation) error {
	if f.Run.TenantID == 0 || f.Run.RunID == "" || f.Owner == "" || f.Epoch < 0 || o.Version <= 0 || o.AttemptID == "" || o.ObservationID == "" || o.Revision < 0 || o.OccurredAt.IsZero() || o.Run.TenantID != f.Run.TenantID || o.Run.RunID != f.Run.RunID || o.Run.SessionID != f.Run.SessionID || o.PromptTokens < 0 || o.CompletionTokens < 0 || o.TotalTokens < 0 || o.CachedTokens < 0 || o.CacheReadTokens < 0 || o.CacheCreateTokens < 0 || o.TotalTokens != o.PromptTokens+o.CompletionTokens {
		return nativeUsageFailure(nativecontract.ErrInvalid, "usage observation is incomplete")
	}
	switch o.AccountingStatus {
	case "known", "partial", "unknown":
	default:
		return nativeUsageFailure(nativecontract.ErrInvalid, "usage accounting status is invalid")
	}
	return nil
}
func nativeUsageFence(tx *gorm.DB, f nativecontract.Fence) error {
	updated := tx.Exec(`UPDATE native_agent_runs SET updated_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND run_id=? AND lease_owner=? AND lease_epoch=? AND lease_expires_at>?`, f.Run.TenantID, f.Run.RunID, f.Owner, f.Epoch, time.Now().UTC())
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return nativeUsageFailure(nativecontract.ErrLeaseLost, "run fence is stale")
	}
	return nil
}
func nativeUsageFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}
