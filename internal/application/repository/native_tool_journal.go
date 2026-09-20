package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
	agenttool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// NativeToolJournal is the durable, fenced boundary for a native logical tool
// call. It deliberately records an unknown effect without creating a result:
// an external success that cannot be persisted must remain recoverable rather
// than being represented as a local success.
type NativeToolJournal struct {
	db        *gorm.DB
	committer nativecontract.CommitCoordinator
}

func NewNativeToolJournal(db *gorm.DB) *NativeToolJournal { return &NativeToolJournal{db: db} }

// NewNativeToolBoundary supplies the P1.5 commit barrier required before a
// callable wrapper may report a confirmed external result to its caller.
func NewNativeToolBoundary(db *gorm.DB, committer nativecontract.CommitCoordinator) *NativeToolJournal {
	return &NativeToolJournal{db: db, committer: committer}
}

var _ nativecontract.AttemptJournal = (*NativeToolJournal)(nil)
var _ nativecontract.ToolBoundary = (*NativeToolJournal)(nil)

type nativeToolDispatchKey struct{}

// NativeToolDispatch is server-owned dispatch state supplied by P3. It is not
// derived from SDK tool metadata or model input.
type NativeToolDispatch struct {
	Fence          nativecontract.Fence
	Plan           nativecontract.ToolPlan
	Attempt        nativecontract.Attempt
	CommitIntentID string
}

func WithNativeToolDispatch(ctx context.Context, dispatch NativeToolDispatch) context.Context {
	return context.WithValue(ctx, nativeToolDispatchKey{}, dispatch)
}

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
			if row.Kind != attempt.Kind || row.LogicalCallID != attempt.LogicalCallID || row.InvocationID != attempt.InvocationID || row.ReplacesAttemptID != attempt.ReplacesAttemptID || row.ProviderRequestID != attempt.ProviderRequestID || row.Number != attempt.Number || row.Epoch != attempt.Epoch || !row.StartedAt.Equal(attempt.StartedAt) {
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
		var callID string
		if err := tx.Table("native_agent_attempts").Select("logical_call_id").Where("tenant_id=? AND run_id=? AND attempt_id=? AND kind='tool'", fence.Run.TenantID, fence.Run.RunID, attemptID).Row().Scan(&callID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return toolJournalFailure(nativecontract.ErrNotFound, "tool attempt was not found", nativecontract.EffectNotDispatched)
			}
			return err
		}
		if failure != nil {
			failure.Effect = effect
			failure.AttemptID = attemptID
			content := json.RawMessage(`{}`)
			outcome := nativecontract.ToolOutcome{AttemptID: attemptID, CallID: callID, ResultHash: failureHash(attemptID, failure), Effect: effect, IsError: true, Content: content, Failure: failure}
			payload, err := json.Marshal(outcome)
			if err != nil {
				return toolJournalFailure(nativecontract.ErrStore, "tool failure is not durable JSON", nativecontract.EffectNotDispatched)
			}
			inserted := tx.Exec(`INSERT INTO native_agent_tool_results (tenant_id, run_id, attempt_id, call_id, result_hash, outcome, effect_state, is_error, content, failure)
				VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?) ON CONFLICT DO NOTHING`, fence.Run.TenantID, fence.Run.RunID, attemptID, callID, outcome.ResultHash, string(payload), string(effect), string(content), nullableFailure(failure))
			if inserted.Error != nil {
				return inserted.Error
			}
			if inserted.RowsAffected == 0 {
				var stored string
				if err := tx.Table("native_agent_tool_results").Select("result_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", fence.Run.TenantID, fence.Run.RunID, attemptID, callID).Row().Scan(&stored); err != nil {
					return err
				}
				if stored != outcome.ResultHash {
					return toolJournalFailure(nativecontract.ErrConflict, "tool failure changed", nativecontract.EffectNotDispatched)
				}
			}
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

func failureHash(attemptID string, failure *nativecontract.Failure) string {
	payload, _ := json.Marshal(struct {
		AttemptID string
		Failure   *nativecontract.Failure
	}{attemptID, failure})
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Decide intentionally fails closed until P2.4 provides the durable pending
// decision CAS store. It prevents a caller from treating an in-memory approval
// as authority to dispatch a tool.
func (j *NativeToolJournal) Decide(_ context.Context, _ nativecontract.Scope, _ nativecontract.UserDecision) error {
	return toolJournalFailure(nativecontract.ErrConflict, "durable pending decision handling is owned by P2.4", nativecontract.EffectNotDispatched)
}

func (j *NativeToolJournal) Wrap(ctx context.Context, scope nativecontract.Scope, identity nativecontract.ToolIdentity, delegate agenttool.Tool) (agenttool.Tool, error) {
	dispatch, ok := ctx.Value(nativeToolDispatchKey{}).(NativeToolDispatch)
	if !ok || dispatch.CommitIntentID == "" || dispatch.Attempt.ID == "" || dispatch.Plan.CallID == "" {
		return nil, toolJournalFailure(nativecontract.ErrStore, "durable tool dispatch is required", nativecontract.EffectNotDispatched)
	}
	if j == nil || j.committer == nil || scope.TenantID == 0 || scope.TenantID != dispatch.Fence.Run.TenantID || identity != dispatch.Plan.Tool || delegate == nil {
		return nil, toolJournalFailure(nativecontract.ErrForbidden, "tool boundary dispatch is not authorized", nativecontract.EffectNotDispatched)
	}
	if _, err := j.Plan(ctx, dispatch.Fence, dispatch.Plan); err != nil {
		return nil, err
	}
	if stream, ok := delegate.(agenttool.StreamableTool); ok {
		return &journalStreamableTool{delegate: stream, journal: j, dispatch: dispatch}, nil
	}
	if callable, ok := delegate.(agenttool.CallableTool); ok {
		return &journalCallableTool{delegate: callable, journal: j, dispatch: dispatch}, nil
	}
	return nil, toolJournalFailure(nativecontract.ErrInvalid, "tool is neither callable nor streamable", nativecontract.EffectNotDispatched)
}

type journalCallableTool struct {
	delegate agenttool.CallableTool
	journal  *NativeToolJournal
	dispatch NativeToolDispatch
}

func (t *journalCallableTool) Declaration() *agenttool.Declaration { return t.delegate.Declaration() }
func (t *journalCallableTool) Call(ctx context.Context, args []byte) (any, error) {
	if string(args) != string(t.dispatch.Plan.Args) {
		return nil, toolJournalFailure(nativecontract.ErrConflict, "tool arguments differ from durable plan", nativecontract.EffectNotDispatched)
	}
	attempt, err := t.journal.Begin(ctx, t.dispatch.Fence, t.dispatch.Attempt)
	if err != nil {
		return nil, err
	}
	result, err := t.delegate.Call(ctx, args)
	if err != nil {
		_ = t.journal.Finish(ctx, t.dispatch.Fence, attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrProvider, Message: "tool call returned without a durable outcome", Effect: nativecontract.EffectUnknown})
		return nil, err
	}
	content, err := json.Marshal(result)
	if err != nil {
		_ = t.journal.Finish(ctx, t.dispatch.Fence, attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrStore, Message: "tool result cannot be persisted", Effect: nativecontract.EffectUnknown})
		return nil, err
	}
	sum := sha256.Sum256(content)
	outcome := nativecontract.ToolOutcome{AttemptID: attempt.ID, CallID: t.dispatch.Plan.CallID, ResultHash: "sha256:" + hex.EncodeToString(sum[:]), Effect: nativecontract.EffectConfirmed, Content: content}
	if err := t.journal.RecordOutcome(ctx, t.dispatch.Fence, outcome); err != nil {
		_ = t.journal.Finish(ctx, t.dispatch.Fence, attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrStore, Message: "tool result persistence failed", Effect: nativecontract.EffectUnknown})
		return nil, err
	}
	intent := nativecontract.CommitIntent{Version: 1, ID: t.dispatch.CommitIntentID, PayloadHash: outcome.ResultHash, Fence: t.dispatch.Fence, Results: []nativecontract.ToolOutcome{outcome}}
	if _, err := t.journal.committer.Commit(ctx, intent); err != nil {
		return nil, err
	}
	if err := t.journal.committer.Barrier(ctx, t.dispatch.Fence, intent.ID); err != nil {
		return nil, err
	}
	return result, nil
}

type journalStreamableTool struct {
	delegate agenttool.StreamableTool
	journal  *NativeToolJournal
	dispatch NativeToolDispatch
}

func (t *journalStreamableTool) Declaration() *agenttool.Declaration { return t.delegate.Declaration() }
func (t *journalStreamableTool) StreamableCall(ctx context.Context, args []byte) (*agenttool.StreamReader, error) {
	if string(args) != string(t.dispatch.Plan.Args) {
		return nil, toolJournalFailure(nativecontract.ErrConflict, "tool arguments differ from durable plan", nativecontract.EffectNotDispatched)
	}
	if _, err := t.journal.Begin(ctx, t.dispatch.Fence, t.dispatch.Attempt); err != nil {
		return nil, err
	}
	reader, err := t.delegate.StreamableCall(ctx, args)
	if err != nil {
		_ = t.journal.Finish(ctx, t.dispatch.Fence, t.dispatch.Attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrProvider, Message: "stream tool returned without a durable outcome", Effect: nativecontract.EffectUnknown})
		return nil, err
	}
	// A StreamReader reports progress only. Its completion cannot prove an
	// external side effect, so P3/P7 must query or explicitly confirm it.
	return reader, nil
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
