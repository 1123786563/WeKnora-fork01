package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
)

// NativeUsageDelta is the newly observed portion of a cumulative provider
// observation. A replay returns its zero value, so callers cannot charge it.
type NativeUsageDelta struct{ PromptTokens, CompletionTokens, TotalTokens, CachedTokens, CacheReadTokens, CacheCreateTokens int64 }

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
				return nativeUsageFailure(nativecontract.ErrConflict, "usage observation identity conflict")
			}
			delta = nativeUsageDelta(o, NativeUsageDelta{})
			return nil
		}
		if previous.Revision == o.Revision {
			if previous.PayloadHash == hash {
				return nil
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
			return nativeUsageFailure(nativecontract.ErrConflict, "usage observation revision raced")
		}
		return nil
	})
	return delta, err
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
