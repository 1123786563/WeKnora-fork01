package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"gorm.io/gorm"
)

// NativeCommitCoordinator persists the authoritative commit intent before it
// exposes a checkpoint or an event. Every mutating operation checks the live
// run fence in the same transaction as its writes.
type NativeCommitCoordinator struct{ db *gorm.DB }

var _ nativecontract.CommitCoordinator = (*NativeCommitCoordinator)(nil)
var _ nativecontract.EventReader = (*NativeCommitCoordinator)(nil)

func NewNativeCommitCoordinator(db *gorm.DB) *NativeCommitCoordinator {
	return &NativeCommitCoordinator{db: db}
}

type nativeCommitIntentRow struct {
	PayloadHash, State, Payload string
	LeaseEpoch                  int64
}

func typedFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

func (c *NativeCommitCoordinator) Commit(ctx context.Context, intent nativecontract.CommitIntent) (nativecontract.CommitReceipt, error) {
	if err := validCommitIntent(intent); err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	var receipt nativecontract.CommitReceipt
	applied := false
	persisted := intent
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := c.assertFence(tx, intent.Fence); err != nil {
			return err
		}
		row, found, err := c.intent(tx, intent.Fence.Run, intent.ID)
		if err != nil {
			return err
		}
		if found {
			if row.PayloadHash != intent.PayloadHash {
				return typedFailure(nativecontract.ErrConflict, "commit intent payload changed")
			}
			if row.State == "applied" {
				receipt, err = c.receipt(tx, intent.Fence.Run, intent.ID)
				applied = err == nil
				return err
			}
			if err := json.Unmarshal([]byte(row.Payload), &persisted); err != nil {
				return typedFailure(nativecontract.ErrStore, "stored commit intent is corrupt")
			}
			if persisted.ID != intent.ID || persisted.PayloadHash != row.PayloadHash {
				return typedFailure(nativecontract.ErrStore, "stored commit intent identity is corrupt")
			}
			// The durable payload is the recovery instruction, while the live
			// caller fence is the authority to replay it.
			persisted.Fence = intent.Fence
		} else {
			payload, err := json.Marshal(intent)
			if err != nil {
				return typedFailure(nativecontract.ErrInvalid, "commit intent is not durable JSON")
			}
			created := tx.Exec(`INSERT INTO native_agent_commit_intents
				(tenant_id, run_id, intent_id, payload_hash, lease_epoch, state, version, payload, terminal_status)
				VALUES (?, ?, ?, ?, ?, 'pending', ?, ?, ?) ON CONFLICT DO NOTHING`,
				intent.Fence.Run.TenantID, intent.Fence.Run.RunID, intent.ID, intent.PayloadHash, intent.Fence.Epoch,
				intent.Version, string(payload), string(intent.TerminalStatus))
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected != 1 {
				return typedFailure(nativecontract.ErrConflict, "commit intent identity conflict")
			}
		}
		return nil
	})
	if err != nil || applied {
		return receipt, err
	}
	// The pending record is an explicit durable barrier. Downstream writes use
	// their own transaction so a failed Session/checkpoint/event projection
	// cannot roll back the recovery instruction.
	err = c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := c.assertFence(tx, intent.Fence); err != nil {
			return err
		}
		receipt, err = c.apply(tx, persisted)
		return err
	})
	return receipt, err
}

func (c *NativeCommitCoordinator) Reconcile(ctx context.Context, fence nativecontract.Fence, intentID string) (nativecontract.CommitReceipt, error) {
	if intentID == "" {
		return nativecontract.CommitReceipt{}, typedFailure(nativecontract.ErrInvalid, "intent ID is required")
	}
	var receipt nativecontract.CommitReceipt
	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := c.assertFence(tx, fence); err != nil {
			return err
		}
		row, found, err := c.intent(tx, fence.Run, intentID)
		if err != nil {
			return err
		}
		if !found {
			return typedFailure(nativecontract.ErrNotFound, "commit intent was not found")
		}
		if row.State == "applied" {
			receipt, err = c.receipt(tx, fence.Run, intentID)
			return err
		}
		var intent nativecontract.CommitIntent
		if err := json.Unmarshal([]byte(row.Payload), &intent); err != nil {
			return typedFailure(nativecontract.ErrStore, "stored commit intent is corrupt")
		}
		if intent.ID != intentID || intent.PayloadHash != row.PayloadHash {
			return typedFailure(nativecontract.ErrStore, "stored commit intent identity is corrupt")
		}
		intent.Fence = fence // a current fence owns the replay, never the old lease.
		receipt, err = c.apply(tx, intent)
		return err
	})
	return receipt, err
}

func (c *NativeCommitCoordinator) Barrier(ctx context.Context, fence nativecontract.Fence, intentID string) error {
	if intentID == "" {
		return typedFailure(nativecontract.ErrInvalid, "intent ID is required")
	}
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := c.assertFence(tx, fence); err != nil {
			return err
		}
		row, found, err := c.intent(tx, fence.Run, intentID)
		if err != nil {
			return err
		}
		if !found || row.State != "applied" {
			return typedFailure(nativecontract.ErrConflict, "commit intent has not been applied")
		}
		return nil
	})
}

func validCommitIntent(intent nativecontract.CommitIntent) error {
	if intent.Version <= 0 || intent.ID == "" || intent.PayloadHash == "" || intent.Fence.Run.TenantID == 0 ||
		intent.Fence.Run.RunID == "" || intent.Fence.Owner == "" || intent.Fence.Epoch < 0 {
		return typedFailure(nativecontract.ErrInvalid, "commit intent is incomplete")
	}
	return nil
}

func (c *NativeCommitCoordinator) assertFence(tx *gorm.DB, fence nativecontract.Fence) error {
	if fence.Run.TenantID == 0 || fence.Run.RunID == "" || fence.Owner == "" || fence.Epoch < 0 {
		return typedFailure(nativecontract.ErrLeaseLost, "fence is incomplete")
	}
	updated := tx.Exec(`UPDATE native_agent_runs SET updated_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND run_id = ? AND lease_owner = ? AND lease_epoch = ? AND lease_expires_at > ?`,
		fence.Run.TenantID, fence.Run.RunID, fence.Owner, fence.Epoch, time.Now().UTC())
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return typedFailure(nativecontract.ErrLeaseLost, "run fence is stale")
	}
	return nil
}

func (c *NativeCommitCoordinator) intent(tx *gorm.DB, run nativecontract.RunIdentity, id string) (nativeCommitIntentRow, bool, error) {
	var row nativeCommitIntentRow
	err := tx.Table("native_agent_commit_intents").Select("payload_hash, state, payload, lease_epoch").
		Where("tenant_id = ? AND run_id = ? AND intent_id = ?", run.TenantID, run.RunID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nativeCommitIntentRow{}, false, nil
	}
	return row, err == nil, err
}

func (c *NativeCommitCoordinator) apply(tx *gorm.DB, intent nativecontract.CommitIntent) (nativecontract.CommitReceipt, error) {
	for _, append := range intent.SessionAppends {
		if err := appendStableInCommit(tx, intent.Fence.Run.TenantID, append); err != nil {
			return nativecontract.CommitReceipt{}, err
		}
	}
	if err := persistOutcomes(tx, intent); err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	if err := persistCheckpoint(tx, intent); err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	if err := persistUsage(tx, intent); err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	last, err := persistEvents(tx, intent)
	if err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	if intent.TerminalStatus != "" {
		if err := tx.Exec("UPDATE native_agent_runs SET status = ? WHERE tenant_id = ? AND run_id = ?", string(intent.TerminalStatus), intent.Fence.Run.TenantID, intent.Fence.Run.RunID).Error; err != nil {
			return nativecontract.CommitReceipt{}, err
		}
	}
	updated := tx.Exec(`UPDATE native_agent_commit_intents SET state = 'applied', applied_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND run_id = ? AND intent_id = ? AND state = 'pending'`, intent.Fence.Run.TenantID, intent.Fence.Run.RunID, intent.ID)
	if updated.Error != nil {
		return nativecontract.CommitReceipt{}, updated.Error
	}
	if updated.RowsAffected != 1 {
		return nativecontract.CommitReceipt{}, typedFailure(nativecontract.ErrConflict, "commit intent state changed")
	}
	return nativecontract.CommitReceipt{IntentID: intent.ID, Applied: true, LastSequence: last}, nil
}

func appendStableInCommit(tx *gorm.DB, tenant uint64, append nativecontract.SessionAppend) error {
	if append.Event == nil || append.StableEventID == "" || append.PayloadHash == "" {
		return typedFailure(nativecontract.ErrInvalid, "session append is incomplete")
	}
	if append.StableEventID != append.Event.ID {
		return typedFailure(nativecontract.ErrInvalid, "stable session event ID does not match event")
	}
	keyTenant, err := nativeSessionTenant(append.Key)
	if err != nil || keyTenant != tenant {
		return typedFailure(nativecontract.ErrInvalid, "session append has another tenant")
	}
	canonicalHash, err := CanonicalNativeSessionEventHash(append.Event)
	if err != nil {
		return err
	}
	append.PayloadHash = canonicalHash
	eventPayload, err := json.Marshal(append.Event)
	if err != nil {
		return err
	}
	var ordinal int64
	if err := tx.Table("native_agent_session_events").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ?", tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID).Count(&ordinal).Error; err != nil {
		return err
	}
	inserted := tx.Exec(`INSERT INTO native_agent_session_events
		(tenant_id, app_name, user_id, session_id, stable_event_id, payload_hash, payload, ordinal)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`, tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID, append.StableEventID, append.PayloadHash, string(eventPayload), ordinal)
	if inserted.Error != nil || inserted.RowsAffected == 1 {
		return inserted.Error
	}
	var stored string
	if err := tx.Table("native_agent_session_events").Select("payload_hash").Where("tenant_id = ? AND app_name = ? AND user_id = ? AND session_id = ? AND stable_event_id = ?", tenant, append.Key.AppName, append.Key.UserID, append.Key.SessionID, append.StableEventID).Row().Scan(&stored); err != nil {
		return err
	}
	if stored != append.PayloadHash {
		return typedFailure(nativecontract.ErrConflict, "stable session event payload changed")
	}
	return nil
}

func persistOutcomes(tx *gorm.DB, intent nativecontract.CommitIntent) error {
	for _, outcome := range intent.Results {
		if outcome.AttemptID == "" || outcome.CallID == "" || outcome.SourceModelAttemptID == "" || outcome.ResultHash == "" || !json.Valid(outcome.Content) {
			return typedFailure(nativecontract.ErrInvalid, "tool result is incomplete")
		}
		if err := assertCommittedOutcomeSource(tx, intent.Fence.Run, outcome); err != nil {
			return err
		}
		outcomeJSON, err := json.Marshal(outcome)
		if err != nil {
			return err
		}
		inserted := tx.Exec(`INSERT INTO native_agent_tool_results
			(tenant_id, run_id, attempt_id, call_id, result_hash, outcome, provider_receipt, query_anchor, effect_state, is_error, truncated, content, failure)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			intent.Fence.Run.TenantID, intent.Fence.Run.RunID, outcome.AttemptID, outcome.CallID, outcome.ResultHash, string(outcomeJSON), outcome.ProviderReceipt, outcome.QueryAnchor, string(outcome.Effect), outcome.IsError, outcome.Truncated, string(outcome.Content), nullableFailure(outcome.Failure))
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			continue
		}
		var stored string
		if err := tx.Table("native_agent_tool_results").Select("result_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID, outcome.AttemptID, outcome.CallID).Row().Scan(&stored); err != nil {
			return err
		}
		if stored != outcome.ResultHash {
			return typedFailure(nativecontract.ErrConflict, "tool result payload changed")
		}
	}
	return nil
}

// assertCommittedOutcomeSource keeps CommitIntent replay on the same durable
// provenance rule as the journal: the executable tool call must resolve to
// exactly one model call with matching immutable plan version and args hash.
func assertCommittedOutcomeSource(tx *gorm.DB, run nativecontract.RunIdentity, outcome nativecontract.ToolOutcome) error {
	var toolCall struct {
		Version  int
		ArgsHash string
	}
	if err := tx.Table("native_agent_tool_calls").Select("plan_version AS version, args_hash").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", run.TenantID, run.RunID, outcome.AttemptID, outcome.CallID).Take(&toolCall).Error; err != nil {
		return typedFailure(nativecontract.ErrNotFound, "tool result source was not found")
	}
	modelCalls := tx.Table("native_agent_tool_calls AS calls").Joins("JOIN native_agent_attempts AS source ON source.tenant_id=calls.tenant_id AND source.run_id=calls.run_id AND source.attempt_id=calls.attempt_id AND source.kind='model'").Where("calls.tenant_id=? AND calls.run_id=? AND calls.call_id=? AND calls.plan_version=? AND calls.args_hash=?", run.TenantID, run.RunID, outcome.CallID, toolCall.Version, toolCall.ArgsHash)
	var all, requested int64
	if err := modelCalls.Count(&all).Error; err != nil {
		return err
	}
	if err := modelCalls.Where("calls.attempt_id=?", outcome.SourceModelAttemptID).Count(&requested).Error; err != nil {
		return err
	}
	if all != 1 || requested != 1 {
		return typedFailure(nativecontract.ErrConflict, "tool result source model attempt is ambiguous")
	}
	return nil
}

func nullableFailure(failure *nativecontract.Failure) any {
	if failure == nil {
		return nil
	}
	b, err := json.Marshal(failure)
	if err != nil {
		return nil
	}
	return string(b)
}

func persistCheckpoint(tx *gorm.DB, intent nativecontract.CommitIntent) error {
	if intent.Checkpoint == nil {
		return nil
	}
	cp := intent.Checkpoint
	if cp.SchemaVersion <= 0 || cp.SDKVersion == "" || cp.GraphVersion == "" || cp.Namespace == "" || cp.LineageID == "" || cp.Request.Checkpoint == nil || cp.Request.Checkpoint.ID == "" {
		return typedFailure(nativecontract.ErrInvalid, "checkpoint is incomplete")
	}
	for _, callID := range cp.ResultCallIDs {
		var count int64
		if err := tx.Table("native_agent_tool_results").Where("tenant_id=? AND run_id=? AND call_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID, callID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return typedFailure(nativecontract.ErrCheckpoint, "checkpoint references an undurable tool result")
		}
	}
	request, err := json.Marshal(cp.Request)
	if err != nil {
		return typedFailure(nativecontract.ErrInvalid, "checkpoint request is not durable JSON")
	}
	ids, err := json.Marshal(cp.ResultCallIDs)
	if err != nil {
		return err
	}
	created := tx.Exec(`INSERT INTO native_agent_checkpoints
		(tenant_id, run_id, checkpoint_id, intent_id, lease_epoch, runnable, payload, schema_version, sdk_version, graph_version, namespace, lineage_id, request_payload, result_call_ids)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		intent.Fence.Run.TenantID, intent.Fence.Run.RunID, cp.Request.Checkpoint.ID, intent.ID, intent.Fence.Epoch, string(request), cp.SchemaVersion, cp.SDKVersion, cp.GraphVersion, cp.Namespace, cp.LineageID, string(request), string(ids))
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected == 0 {
		return typedFailure(nativecontract.ErrConflict, "checkpoint identity conflict")
	}
	return nil
}

func persistUsage(tx *gorm.DB, intent nativecontract.CommitIntent) error {
	for _, observation := range intent.Usage {
		if err := validUsageObservation(intent.Fence.Run, observation); err != nil {
			return err
		}
		var attempts int64
		if err := tx.Table("native_agent_attempts").Where("tenant_id=? AND run_id=? AND attempt_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID, observation.AttemptID).Count(&attempts).Error; err != nil {
			return err
		}
		if attempts != 1 {
			return typedFailure(nativecontract.ErrNotFound, "usage observation attempt was not found")
		}
		payload, err := json.Marshal(observation)
		if err != nil {
			return typedFailure(nativecontract.ErrInvalid, "usage observation is not durable JSON")
		}
		sum := sha256.Sum256(payload)
		hash := hex.EncodeToString(sum[:])
		dimensions, err := json.Marshal(observation.Dimensions)
		if err != nil {
			return typedFailure(nativecontract.ErrInvalid, "usage observation dimensions are not durable JSON")
		}
		inserted := tx.Exec(`INSERT INTO native_agent_usage_observations
			(tenant_id, run_id, attempt_id, observation_id, revision, provider, model, provider_request_id, funding_ref, budget_root_run_id,
			 input_tokens, output_tokens, cached_tokens, cache_read_tokens, cache_create_tokens, accounting_status, dimensions, occurred_at, payload_hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			intent.Fence.Run.TenantID, intent.Fence.Run.RunID, observation.AttemptID, observation.ObservationID, observation.Revision,
			observation.Funding.Service, observation.Funding.PriceVersion, observation.ProviderRequestID, observation.Funding.BudgetRef, observation.Funding.BudgetRootRunID,
			observation.PromptTokens, observation.CompletionTokens, observation.CachedTokens, observation.CacheReadTokens, observation.CacheCreateTokens,
			observation.AccountingStatus, string(dimensions), observation.OccurredAt, hash)
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected == 1 {
			continue
		}
		advanced := tx.Exec(`UPDATE native_agent_usage_observations SET
			revision = ?, provider = ?, model = ?, provider_request_id = ?, funding_ref = ?, budget_root_run_id = ?,
			input_tokens = ?, output_tokens = ?, cached_tokens = ?, cache_read_tokens = ?, cache_create_tokens = ?,
			accounting_status = ?, dimensions = ?, occurred_at = ?, payload_hash = ?
			WHERE tenant_id = ? AND run_id = ? AND attempt_id = ? AND observation_id = ? AND revision < ?`,
			observation.Revision, observation.Funding.Service, observation.Funding.PriceVersion, observation.ProviderRequestID,
			observation.Funding.BudgetRef, observation.Funding.BudgetRootRunID, observation.PromptTokens, observation.CompletionTokens,
			observation.CachedTokens, observation.CacheReadTokens, observation.CacheCreateTokens, observation.AccountingStatus,
			string(dimensions), observation.OccurredAt, hash, intent.Fence.Run.TenantID, intent.Fence.Run.RunID,
			observation.AttemptID, observation.ObservationID, observation.Revision)
		if advanced.Error != nil {
			return advanced.Error
		}
		if advanced.RowsAffected == 1 {
			continue
		}
		var storedRevision int64
		var storedHash string
		if err := tx.Table("native_agent_usage_observations").Select("revision, payload_hash").
			Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID, observation.AttemptID, observation.ObservationID).
			Row().Scan(&storedRevision, &storedHash); err != nil {
			return err
		}
		if storedRevision == observation.Revision && storedHash == hash {
			continue
		}
		if storedRevision == observation.Revision {
			return typedFailure(nativecontract.ErrConflict, "usage observation payload changed")
		}
		return typedFailure(nativecontract.ErrConflict, "usage observation revision is stale")
	}
	return nil
}

func validUsageObservation(run nativecontract.RunIdentity, observation nativecontract.UsageObservation) error {
	if observation.Version <= 0 || observation.AttemptID == "" || observation.ObservationID == "" || observation.Revision < 0 || observation.OccurredAt.IsZero() ||
		observation.Run.TenantID != run.TenantID || observation.Run.RunID != run.RunID || observation.Run.SessionID != run.SessionID ||
		observation.PromptTokens < 0 || observation.CompletionTokens < 0 || observation.TotalTokens < 0 || observation.CachedTokens < 0 || observation.CacheReadTokens < 0 || observation.CacheCreateTokens < 0 {
		return typedFailure(nativecontract.ErrInvalid, "usage observation is incomplete")
	}
	return nil
}

func persistEvents(tx *gorm.DB, intent nativecontract.CommitIntent) (int64, error) {
	var last int64
	if err := tx.Table("native_agent_events").Select("COALESCE(MAX(sequence), 0)").Where("tenant_id=? AND run_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID).Scan(&last).Error; err != nil {
		return 0, err
	}
	for _, event := range intent.Events {
		if event.EventID == "" || event.TenantID != fmt.Sprint(intent.Fence.Run.TenantID) || event.RunID != intent.Fence.Run.RunID || event.SessionID != intent.Fence.Run.SessionID {
			return 0, typedFailure(nativecontract.ErrInvalid, "business event is outside the commit run")
		}
		event.Sequence = ""
		hash, payload, err := nativeEventPayload(event)
		if err != nil {
			return 0, err
		}
		var stored string
		err = tx.Table("native_agent_events").Select("payload_hash").Where("tenant_id=? AND run_id=? AND event_id=?", intent.Fence.Run.TenantID, intent.Fence.Run.RunID, event.EventID).Row().Scan(&stored)
		if err == nil {
			if stored != hash {
				return 0, typedFailure(nativecontract.ErrConflict, "business event payload changed")
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		last++
		event.Sequence = fmt.Sprint(last)
		storedPayload, err := json.Marshal(event)
		if err != nil {
			return 0, err
		}
		created := tx.Exec(`INSERT INTO native_agent_events
			(tenant_id, run_id, sequence, event_id, intent_id, attempt_id, protocol, schema_version, kind, payload_hash, payload)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, intent.Fence.Run.TenantID, intent.Fence.Run.RunID, last, event.EventID, intent.ID, event.AttemptID, event.Protocol, event.SchemaVersion, string(event.Kind), hash, string(storedPayload))
		if created.Error != nil {
			return 0, created.Error
		}
		_ = payload
	}
	return last, nil
}

func nativeEventPayload(event nativecontract.BusinessEvent) (string, []byte, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), payload, nil
}

func (c *NativeCommitCoordinator) receipt(tx *gorm.DB, run nativecontract.RunIdentity, intentID string) (nativecontract.CommitReceipt, error) {
	var last int64
	if err := tx.Table("native_agent_events").Select("COALESCE(MAX(sequence), 0)").Where("tenant_id=? AND run_id=?", run.TenantID, run.RunID).Scan(&last).Error; err != nil {
		return nativecontract.CommitReceipt{}, err
	}
	return nativecontract.CommitReceipt{IntentID: intentID, Applied: true, LastSequence: last}, nil
}
