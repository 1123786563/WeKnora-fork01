package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
)

// NativeToolJournal is the durable, fenced boundary for a native logical tool
// call. It deliberately records an unknown effect without creating a result:
// an external success that cannot be persisted must remain recoverable rather
// than being represented as a local success.
type NativeToolJournal struct{ db *gorm.DB }

func NewNativeToolJournal(db *gorm.DB) *NativeToolJournal { return &NativeToolJournal{db: db} }

var _ nativecontract.AttemptJournal = (*NativeToolJournal)(nil)

func toolJournalFailure(code nativecontract.ErrorCode, message string, effect nativecontract.EffectState) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: effect}
}

func validToolFence(fence nativecontract.Fence) bool {
	return fence.Run.TenantID != 0 && fence.Run.RunID != "" && fence.Owner != "" && fence.Epoch > 0
}

func (j *NativeToolJournal) assertFence(tx *gorm.DB, fence nativecontract.Fence) error {
	if !validToolFence(fence) {
		return toolJournalFailure(nativecontract.ErrLeaseLost, "tool journal fence is incomplete", nativecontract.EffectNotDispatched)
	}
	updated := tx.Exec(`UPDATE native_agent_runs SET updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND run_id = ? AND lease_owner = ? AND lease_epoch = ? AND lease_expires_at > ?`,
		fence.Run.TenantID, fence.Run.RunID, fence.Owner, fence.Epoch, time.Now().UTC())
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return toolJournalFailure(nativecontract.ErrLeaseLost, "tool journal fence is stale", nativecontract.EffectNotDispatched)
	}
	return nil
}

func validToolPlan(plan nativecontract.ToolPlan, fence nativecontract.Fence) error {
	if plan.Version <= 0 || plan.CallID == "" || plan.ModelAttemptID == "" || plan.ArgsHash == "" || plan.Tool.Name == "" ||
		plan.Tool.SchemaHash == "" || plan.Tool.ConfigVersion == "" || !json.Valid(plan.Args) || plan.Run.TenantID != fence.Run.TenantID ||
		plan.Run.RunID != fence.Run.RunID || plan.Run.SessionID != fence.Run.SessionID {
		return toolJournalFailure(nativecontract.ErrInvalid, "tool plan is incomplete", nativecontract.EffectNotDispatched)
	}
	switch plan.Policy {
	case nativecontract.RecoveryHold, nativecontract.RecoveryQuery, nativecontract.RecoveryIdempotent, nativecontract.RecoveryReadOnly:
	default:
		return toolJournalFailure(nativecontract.ErrInvalid, "tool recovery policy is invalid", nativecontract.EffectNotDispatched)
	}
	if plan.Policy == nativecontract.RecoveryIdempotent && (plan.IdempotencyKey == "" || plan.IdempotencyExpiresAt.IsZero()) {
		return toolJournalFailure(nativecontract.ErrInvalid, "idempotent tool plan requires a bounded provider key", nativecontract.EffectNotDispatched)
	}
	return nil
}

// Plan persists the immutable model decision before an external dispatch is
// allowed. A replay with exactly the same decision succeeds; any changed
// arguments, schema, configuration, or recovery capability is a conflict.
func (j *NativeToolJournal) Plan(ctx context.Context, fence nativecontract.Fence, plan nativecontract.ToolPlan) (nativecontract.ToolPlan, error) {
	if j == nil || j.db == nil {
		return nativecontract.ToolPlan{}, toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if err := validToolPlan(plan, fence); err != nil {
		return nativecontract.ToolPlan{}, err
	}
	grants, err := json.Marshal(plan.RequiredGrants)
	if err != nil {
		return nativecontract.ToolPlan{}, toolJournalFailure(nativecontract.ErrInvalid, "tool grants are not durable JSON", nativecontract.EffectNotDispatched)
	}
	err = j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := j.assertFence(tx, fence); err != nil {
			return err
		}
		var count int64
		if err := tx.Table("native_agent_attempts").Where("tenant_id=? AND run_id=? AND attempt_id=?", fence.Run.TenantID, fence.Run.RunID, plan.ModelAttemptID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return toolJournalFailure(nativecontract.ErrNotFound, "model attempt for tool plan was not found", nativecontract.EffectNotDispatched)
		}
		call := tx.Exec(`INSERT INTO native_agent_tool_calls
			(tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch)
			VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, fence.Run.TenantID, fence.Run.RunID, plan.ModelAttemptID, plan.CallID, plan.Version, plan.ArgsHash, fence.Epoch)
		if call.Error != nil {
			return call.Error
		}
		if call.RowsAffected == 0 {
			var storedVersion int
			var storedHash string
			if err := tx.Table("native_agent_tool_calls").Select("plan_version, args_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", fence.Run.TenantID, fence.Run.RunID, plan.ModelAttemptID, plan.CallID).Row().Scan(&storedVersion, &storedHash); err != nil {
				return err
			}
			if storedVersion != plan.Version || storedHash != plan.ArgsHash {
				return toolJournalFailure(nativecontract.ErrConflict, "tool call plan identity changed", nativecontract.EffectNotDispatched)
			}
		}
		inserted := tx.Exec(`INSERT INTO native_agent_tool_plans
			(tenant_id, run_id, attempt_id, call_id, plan_version, provider_tool_call_id, kind, service_id, installation_id, name, schema_hash, config_version, args, args_hash, policy, idempotency_key, idempotency_expires_at, required_grants)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			fence.Run.TenantID, fence.Run.RunID, plan.ModelAttemptID, plan.CallID, plan.Version, plan.ProviderToolCallID, plan.Tool.Kind, plan.Tool.ServiceID, plan.Tool.InstallationID, plan.Tool.Name, plan.Tool.SchemaHash, plan.Tool.ConfigVersion, string(plan.Args), plan.ArgsHash, string(plan.Policy), plan.IdempotencyKey, nullableTime(plan.IdempotencyExpiresAt), string(grants))
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			return nil
		}
		var row struct {
			ProviderToolCallID, Kind, ServiceID, InstallationID, Name, SchemaHash, ConfigVersion, Args, ArgsHash, Policy, IdempotencyKey, RequiredGrants string
			IdempotencyExpiresAt                                                                                                                         *time.Time
		}
		if err := tx.Table("native_agent_tool_plans").Select("provider_tool_call_id, kind, service_id, installation_id, name, schema_hash, config_version, args, args_hash, policy, idempotency_key, idempotency_expires_at, required_grants").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=? AND plan_version=?", fence.Run.TenantID, fence.Run.RunID, plan.ModelAttemptID, plan.CallID, plan.Version).Take(&row).Error; err != nil {
			return err
		}
		if row.ProviderToolCallID != plan.ProviderToolCallID || row.Kind != plan.Tool.Kind || row.ServiceID != plan.Tool.ServiceID || row.InstallationID != plan.Tool.InstallationID || row.Name != plan.Tool.Name || row.SchemaHash != plan.Tool.SchemaHash || row.ConfigVersion != plan.Tool.ConfigVersion || row.Args != string(plan.Args) || row.ArgsHash != plan.ArgsHash || row.Policy != string(plan.Policy) || row.IdempotencyKey != plan.IdempotencyKey || !sameTime(row.IdempotencyExpiresAt, plan.IdempotencyExpiresAt) || row.RequiredGrants != string(grants) {
			return toolJournalFailure(nativecontract.ErrConflict, "tool plan payload changed", nativecontract.EffectNotDispatched)
		}
		return nil
	})
	return plan, err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
func sameTime(stored *time.Time, supplied time.Time) bool {
	return (stored == nil && supplied.IsZero()) || (stored != nil && stored.Equal(supplied))
}

// Begin creates one immutable dispatch attempt. It copies the call identity to
// the attempt so a result can be keyed by (run, call, attempt) without
// loosening the model plan's identity.
func (j *NativeToolJournal) Begin(ctx context.Context, fence nativecontract.Fence, attempt nativecontract.Attempt) (nativecontract.Attempt, error) {
	if j == nil || j.db == nil {
		return nativecontract.Attempt{}, toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if attempt.ID == "" || attempt.Kind != nativecontract.ToolAttempt || attempt.LogicalCallID == "" || attempt.Number <= 0 || attempt.Run != fence.Run || attempt.Epoch != fence.Epoch {
		return nativecontract.Attempt{}, toolJournalFailure(nativecontract.ErrInvalid, "tool attempt is incomplete", nativecontract.EffectNotDispatched)
	}
	err := j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := j.assertFence(tx, fence); err != nil {
			return err
		}
		created := tx.Exec(`INSERT INTO native_agent_attempts
			(tenant_id, run_id, attempt_id, lease_epoch, kind, logical_call_id, invocation_id, replaces_attempt_id, attempt_number, provider_request_id, effect_state, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'started', ?) ON CONFLICT DO NOTHING`,
			fence.Run.TenantID, fence.Run.RunID, attempt.ID, fence.Epoch, string(attempt.Kind), attempt.LogicalCallID, attempt.InvocationID, attempt.ReplacesAttemptID, attempt.Number, attempt.ProviderRequestID, string(nativecontract.EffectNotDispatched), attempt.StartedAt)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var row nativecontract.Attempt
			if err := j.loadAttempt(tx, fence.Run, attempt.ID, &row); err != nil {
				return err
			}
			if row.Kind != attempt.Kind || row.LogicalCallID != attempt.LogicalCallID || row.Number != attempt.Number || row.Epoch != attempt.Epoch {
				return toolJournalFailure(nativecontract.ErrConflict, "tool attempt identity changed", nativecontract.EffectNotDispatched)
			}
			return nil
		}
		copied := tx.Exec(`INSERT INTO native_agent_tool_calls (tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch)
			SELECT tenant_id, run_id, ?, call_id, plan_version, args_hash, ? FROM native_agent_tool_calls
			WHERE tenant_id=? AND run_id=? AND call_id=? ORDER BY plan_version DESC LIMIT 1`, attempt.ID, fence.Epoch, fence.Run.TenantID, fence.Run.RunID, attempt.LogicalCallID)
		if copied.Error != nil {
			return copied.Error
		}
		if copied.RowsAffected != 1 {
			return toolJournalFailure(nativecontract.ErrNotFound, "tool plan was not found", nativecontract.EffectNotDispatched)
		}
		return nil
	})
	return attempt, err
}

func (j *NativeToolJournal) Finish(ctx context.Context, fence nativecontract.Fence, attemptID string, effect nativecontract.EffectState, failure *nativecontract.Failure) error {
	if j == nil || j.db == nil {
		return toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if attemptID == "" || (effect != nativecontract.EffectUnknown && effect != nativecontract.EffectNotDispatched) {
		return toolJournalFailure(nativecontract.ErrInvalid, "tool finish effect is invalid", nativecontract.EffectNotDispatched)
	}
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := j.assertFence(tx, fence); err != nil {
			return err
		}
		status := "failed"
		if effect == nativecontract.EffectUnknown {
			status = "unknown"
		}
		updated := tx.Exec(`UPDATE native_agent_attempts SET effect_state=?, status=?, finished_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND run_id=? AND attempt_id=? AND kind='tool'`, string(effect), status, fence.Run.TenantID, fence.Run.RunID, attemptID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return toolJournalFailure(nativecontract.ErrNotFound, "tool attempt was not found", nativecontract.EffectNotDispatched)
		}
		return nil
	})
}

func (j *NativeToolJournal) Get(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity, id string) (nativecontract.Attempt, error) {
	if j == nil || j.db == nil {
		return nativecontract.Attempt{}, toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if scope.TenantID == 0 || scope.TenantID != run.TenantID {
		return nativecontract.Attempt{}, toolJournalFailure(nativecontract.ErrForbidden, "tool attempt is outside scope", nativecontract.EffectNotDispatched)
	}
	var attempt nativecontract.Attempt
	err := j.loadAttempt(j.db.WithContext(ctx), run, id, &attempt)
	return attempt, err
}

func (j *NativeToolJournal) loadAttempt(db *gorm.DB, run nativecontract.RunIdentity, id string, attempt *nativecontract.Attempt) error {
	var row struct {
		ID, Kind, LogicalCallID, InvocationID, ReplacesAttemptID, ProviderRequestID string
		Number, Epoch                                                               int64
		StartedAt                                                                   time.Time
	}
	err := db.Table("native_agent_attempts").Select("attempt_id AS id, kind, logical_call_id, invocation_id, replaces_attempt_id, provider_request_id, attempt_number AS number, lease_epoch AS epoch, created_at AS started_at").Where("tenant_id=? AND run_id=? AND attempt_id=?", run.TenantID, run.RunID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
		return toolJournalFailure(nativecontract.ErrNotFound, "tool attempt was not found", nativecontract.EffectNotDispatched)
	}
	if err != nil {
		return err
	}
	*attempt = nativecontract.Attempt{ID: row.ID, Run: run, Kind: nativecontract.AttemptKind(row.Kind), LogicalCallID: row.LogicalCallID, InvocationID: row.InvocationID, ReplacesAttemptID: row.ReplacesAttemptID, Number: int(row.Number), Epoch: row.Epoch, ProviderRequestID: row.ProviderRequestID, StartedAt: row.StartedAt}
	return nil
}

func (j *NativeToolJournal) RecordOutcome(ctx context.Context, fence nativecontract.Fence, outcome nativecontract.ToolOutcome) error {
	if j == nil || j.db == nil {
		return toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if outcome.AttemptID == "" || outcome.CallID == "" || outcome.ResultHash == "" || outcome.Effect != nativecontract.EffectConfirmed || !json.Valid(outcome.Content) {
		return toolJournalFailure(nativecontract.ErrInvalid, "confirmed tool outcome is incomplete", nativecontract.EffectNotDispatched)
	}
	payload, err := json.Marshal(outcome)
	if err != nil {
		return toolJournalFailure(nativecontract.ErrInvalid, "tool outcome is not durable JSON", nativecontract.EffectNotDispatched)
	}
	return j.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := j.assertFence(tx, fence); err != nil {
			return err
		}
		var calls int64
		if err := tx.Table("native_agent_tool_calls").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", fence.Run.TenantID, fence.Run.RunID, outcome.AttemptID, outcome.CallID).Count(&calls).Error; err != nil {
			return err
		}
		if calls != 1 {
			return toolJournalFailure(nativecontract.ErrNotFound, "tool attempt call was not found", nativecontract.EffectNotDispatched)
		}
		failure := nullableFailure(outcome.Failure)
		inserted := tx.Exec(`INSERT INTO native_agent_tool_results (tenant_id, run_id, attempt_id, call_id, result_hash, outcome, provider_receipt, query_anchor, effect_state, is_error, truncated, content, failure)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, fence.Run.TenantID, fence.Run.RunID, outcome.AttemptID, outcome.CallID, outcome.ResultHash, string(payload), outcome.ProviderReceipt, outcome.QueryAnchor, string(outcome.Effect), outcome.IsError, outcome.Truncated, string(outcome.Content), failure)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 0 {
			var hash, stored string
			if err := tx.Table("native_agent_tool_results").Select("result_hash, outcome").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", fence.Run.TenantID, fence.Run.RunID, outcome.AttemptID, outcome.CallID).Row().Scan(&hash, &stored); err != nil {
				return err
			}
			if hash != outcome.ResultHash || stored != string(payload) {
				return toolJournalFailure(nativecontract.ErrConflict, "tool outcome changed", nativecontract.EffectNotDispatched)
			}
			return nil
		}
		updated := tx.Exec("UPDATE native_agent_attempts SET effect_state=?, status='finished', finished_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND run_id=? AND attempt_id=? AND kind='tool'", string(nativecontract.EffectConfirmed), fence.Run.TenantID, fence.Run.RunID, outcome.AttemptID)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return toolJournalFailure(nativecontract.ErrNotFound, "tool attempt was not found", nativecontract.EffectNotDispatched)
		}
		return nil
	})
}

func (j *NativeToolJournal) LookupResult(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity, callID string) (nativecontract.ToolOutcome, error) {
	if j == nil || j.db == nil {
		return nativecontract.ToolOutcome{}, toolJournalFailure(nativecontract.ErrStore, "tool journal is unavailable", nativecontract.EffectNotDispatched)
	}
	if scope.TenantID == 0 || scope.TenantID != run.TenantID {
		return nativecontract.ToolOutcome{}, toolJournalFailure(nativecontract.ErrForbidden, "tool result is outside scope", nativecontract.EffectNotDispatched)
	}
	var payload string
	err := j.db.WithContext(ctx).Table("native_agent_tool_results").Select("outcome").Where("tenant_id=? AND run_id=? AND call_id=? AND effect_state=?", run.TenantID, run.RunID, callID, string(nativecontract.EffectConfirmed)).Order("created_at DESC").Limit(1).Row().Scan(&payload)
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
		return nativecontract.ToolOutcome{}, toolJournalFailure(nativecontract.ErrNotFound, "confirmed tool result was not found", nativecontract.EffectNotDispatched)
	}
	if err != nil {
		return nativecontract.ToolOutcome{}, err
	}
	var outcome nativecontract.ToolOutcome
	if err := json.Unmarshal([]byte(payload), &outcome); err != nil {
		return nativecontract.ToolOutcome{}, toolJournalFailure(nativecontract.ErrStore, "stored tool outcome is corrupt", nativecontract.EffectNotDispatched)
	}
	return outcome, nil
}
