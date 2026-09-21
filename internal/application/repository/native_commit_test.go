package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func nativeCommitFixture(t *testing.T) (*NativeCommitCoordinator, nativecontract.Fence) {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", 1, "owner/dTE", "session/czE").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, status, lease_owner, lease_epoch, lease_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "owner/dTE", "session/czE", "running", "worker-1", 7, time.Now().Add(time.Hour)).Error)
	return NewNativeCommitCoordinator(db), nativecontract.Fence{
		Run:   nativecontract.RunIdentity{TenantID: 1, SessionID: "session/czE", RunID: "run-1"},
		Owner: "worker-1", Epoch: 7, LeaseUntil: time.Now().Add(time.Hour),
	}
}

func nativeCommitIntent(fence nativecontract.Fence, id, hash string) nativecontract.CommitIntent {
	return nativecontract.CommitIntent{Version: 1, ID: id, PayloadHash: hash, Fence: fence}
}

func nativeCommitToolOutcome(t *testing.T, coordinator *NativeCommitCoordinator, fence nativecontract.Fence, modelAttemptID, toolAttemptID, callID, argsHash, resultHash string) nativecontract.ToolOutcome {
	t.Helper()
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch, kind, attempt_number) VALUES (?, ?, ?, ?, 'model', ?)`, 1, "run-1", modelAttemptID, fence.Epoch, 1).Error)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_tool_calls (tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", modelAttemptID, callID, 1, argsHash, fence.Epoch).Error)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch, kind, logical_call_id, attempt_number) VALUES (?, ?, ?, ?, 'tool', ?, ?)`, 1, "run-1", toolAttemptID, fence.Epoch, callID, 2).Error)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_tool_calls (tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", toolAttemptID, callID, 1, argsHash, fence.Epoch).Error)
	return nativecontract.ToolOutcome{AttemptID: toolAttemptID, CallID: callID, SourceModelAttemptID: modelAttemptID, ResultHash: resultHash, Effect: nativecontract.EffectConfirmed, Content: []byte(`{}`)}
}

func failureCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure), "expected typed native failure, got %v", err)
	return failure.Code
}

func nativeCommitSessionHash(t *testing.T, e *event.Event) string {
	t.Helper()
	hash, err := CanonicalNativeSessionEventHash(e)
	require.NoError(t, err)
	return hash
}

func TestNativeCommitSameIntentIsIdempotentAndChangedHashConflicts(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()

	first, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.Equal(t, first, second)

	_, err = coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "changed"))
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeBarrierRequiresAppliedIntent(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()

	require.Equal(t, nativecontract.ErrConflict, failureCode(t, coordinator.Barrier(ctx, fence, "missing")))
	_, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)
	require.NoError(t, coordinator.Barrier(ctx, fence, "intent-1"))
}

func TestNativeCommitRejectsOutcomeWithoutBoundSourceModelAttempt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*nativecontract.ToolOutcome)
		code   nativecontract.ErrorCode
	}{
		{"missing", func(outcome *nativecontract.ToolOutcome) { outcome.SourceModelAttemptID = "" }, nativecontract.ErrInvalid},
		{"mismatched", func(outcome *nativecontract.ToolOutcome) { outcome.SourceModelAttemptID = "model-other" }, nativecontract.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coordinator, fence := nativeCommitFixture(t)
			outcome := nativeCommitToolOutcome(t, coordinator, fence, "model-result", "attempt-result", "call-result", "args", "result")
			tc.mutate(&outcome)
			intent := nativeCommitIntent(fence, "intent-"+tc.name, "hash-"+tc.name)
			intent.Results = []nativecontract.ToolOutcome{outcome}
			_, err := coordinator.Commit(context.Background(), intent)
			require.Equal(t, tc.code, failureCode(t, err))
		})
	}
}

func TestNativeCommitRejectsModelAttemptAsExecutableOutcomeAttempt(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	outcome := nativeCommitToolOutcome(t, coordinator, fence, "model-result", "attempt-result", "call-result", "args", "result")
	outcome.AttemptID = "model-result"
	intent := nativeCommitIntent(fence, "intent-model-as-tool", "hash-model-as-tool")
	intent.Results = []nativecontract.ToolOutcome{outcome}
	_, err := coordinator.Commit(context.Background(), intent)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeCommitRejectsAmbiguousOutcomeSourceModelAttempt(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	outcome := nativeCommitToolOutcome(t, coordinator, fence, "model-result", "attempt-result", "call-result", "args", "result")
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch, kind, attempt_number) VALUES (?, ?, ?, ?, 'model', ?)`, 1, "run-1", "model-duplicate", fence.Epoch, 3).Error)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_tool_calls (tenant_id, run_id, attempt_id, call_id, plan_version, args_hash, lease_epoch) VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "model-duplicate", "call-result", 1, "args", fence.Epoch).Error)
	intent := nativeCommitIntent(fence, "intent-ambiguous-source", "hash-ambiguous-source")
	intent.Results = []nativecontract.ToolOutcome{outcome}
	_, err := coordinator.Commit(context.Background(), intent)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeCommitReconcileRejectsStaleFence(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	_, err := coordinator.Commit(ctx, nativeCommitIntent(fence, "intent-1", "hash-1"))
	require.NoError(t, err)

	stale := fence
	stale.Epoch--
	_, err = coordinator.Reconcile(ctx, stale, "intent-1")
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
}

func TestNativeCommitStaleFenceWritesNoDurableRecords(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	stale := fence
	stale.Epoch--
	intent := nativeCommitIntent(stale, "intent-stale", "hash-stale")
	intent.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-stale", "stale")}

	_, err := coordinator.Commit(context.Background(), intent)
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
	for _, table := range []string{
		"native_agent_commit_intents",
		"native_agent_session_events",
		"native_agent_tool_results",
		"native_agent_checkpoints",
		"native_agent_events",
	} {
		var count int64
		require.NoError(t, coordinator.db.Table(table).Count(&count).Error)
		require.Zero(t, count, "stale fence wrote %s", table)
	}
}

func TestNativeCommitPersistsPendingIntentAcrossDownstreamBarrierFailures(t *testing.T) {
	for _, gap := range []struct {
		name, table string
		configure   func(*NativeCommitCoordinator, nativecontract.Fence) nativecontract.CommitIntent
	}{
		{
			name: "session append", table: "native_agent_session_events",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-session", "hash-session")
				e := &event.Event{ID: "session-event", Author: "agent"}
				intent.SessionAppends = []nativecontract.SessionAppend{{
					Key:           session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/czE"},
					StableEventID: e.ID, PayloadHash: nativeCommitSessionHash(t, e), Event: e,
				}}
				return intent
			},
		},
		{
			name: "tool result", table: "native_agent_tool_results",
			configure: func(coordinator *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-result", "hash-result")
				intent.Results = []nativecontract.ToolOutcome{nativeCommitToolOutcome(t, coordinator, fence, "model-result", "attempt-result", "call-result", "args", "result")}
				return intent
			},
		},
		{
			name: "checkpoint", table: "native_agent_checkpoints",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-checkpoint", "hash-checkpoint")
				intent.Checkpoint = &nativecontract.CheckpointWrite{SchemaVersion: 1, SDKVersion: "sdk", GraphVersion: "graph", Namespace: "native/run", LineageID: "lineage", Request: graph.PutFullRequest{Checkpoint: &graph.Checkpoint{ID: "checkpoint-gap"}}}
				return intent
			},
		},
		{
			name: "event", table: "native_agent_events",
			configure: func(_ *NativeCommitCoordinator, fence nativecontract.Fence) nativecontract.CommitIntent {
				intent := nativeCommitIntent(fence, "intent-event", "hash-event")
				intent.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("event-gap", "gap")}
				return intent
			},
		},
	} {
		t.Run(gap.name, func(t *testing.T) {
			coordinator, fence := nativeCommitFixture(t)
			intent := gap.configure(coordinator, fence)
			trigger := "abort_" + strings.ReplaceAll(intent.ID, "-", "_")
			require.NoError(t, coordinator.db.Exec("CREATE TRIGGER "+trigger+" BEFORE INSERT ON "+gap.table+" BEGIN SELECT RAISE(ABORT, 'injected "+gap.name+" failure'); END").Error)

			_, err := coordinator.Commit(context.Background(), intent)
			require.Error(t, err)
			var state string
			require.NoError(t, coordinator.db.Table("native_agent_commit_intents").Select("state").Where("tenant_id=? AND run_id=? AND intent_id=?", 1, "run-1", intent.ID).Row().Scan(&state))
			require.Equal(t, "pending", state)

			require.NoError(t, coordinator.db.Exec("DROP TRIGGER "+trigger).Error)
			require.NoError(t, coordinator.db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_epoch=?, lease_expires_at=? WHERE tenant_id=? AND run_id=?", "worker-2", 8, time.Now().Add(time.Hour), 1, "run-1").Error)
			freshFence := fence
			freshFence.Owner, freshFence.Epoch, freshFence.LeaseUntil = "worker-2", 8, time.Now().Add(time.Hour)
			receipt, err := NewNativeCommitCoordinator(reopenRunDB(t, coordinator.db)).Reconcile(context.Background(), freshFence, intent.ID)
			require.NoError(t, err)
			require.True(t, receipt.Applied)
		})
	}
}

func TestNativeCommitExistingPendingIntentUsesPersistedPayload(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	stored := nativeCommitIntent(fence, "intent-authority", "hash-authority")
	stored.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("stored-event", "stored")}
	require.NoError(t, coordinator.db.Exec("CREATE TRIGGER abort_authority BEFORE INSERT ON native_agent_events BEGIN SELECT RAISE(ABORT, 'inject pending intent'); END").Error)
	_, err := coordinator.Commit(context.Background(), stored)
	require.Error(t, err)
	require.NoError(t, coordinator.db.Exec("DROP TRIGGER abort_authority").Error)

	caller := stored
	caller.Events = []nativecontract.BusinessEvent{nativeBusinessEvent("caller-event", "mutable caller payload")}
	receipt, err := coordinator.Commit(context.Background(), caller)
	require.NoError(t, err)
	require.True(t, receipt.Applied)
	var eventIDs []string
	require.NoError(t, coordinator.db.Table("native_agent_events").Order("sequence").Pluck("event_id", &eventIDs).Error)
	require.Equal(t, []string{"stored-event"}, eventIDs)
}

func TestNativeCommitReconcilePersistsUsageFromPendingIntent(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-usage", 7).Error)
	intent := nativeCommitIntent(fence, "intent-usage-replay", "hash-usage-replay")
	intent.Usage = []nativecontract.UsageObservation{{
		Version: 1, Run: fence.Run, AttemptID: "attempt-usage", ObservationID: "usage-1", Revision: 2,
		ProviderRequestID: "provider-request-1",
		Funding:           nativecontract.FundingBinding{BudgetRef: "budget-1", BudgetRootRunID: "run-1", Service: "openai", PriceVersion: "gpt-5"},
		PromptTokens:      11, CompletionTokens: 7, TotalTokens: 18, CachedTokens: 3, CacheReadTokens: 2, CacheCreateTokens: 1,
		AccountingStatus: "reported", Dimensions: map[string]int64{"reasoning": 4}, OccurredAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}}
	require.NoError(t, coordinator.db.Exec("CREATE TRIGGER abort_usage_replay BEFORE INSERT ON native_agent_usage_observations BEGIN SELECT RAISE(ABORT, 'injected usage failure'); END").Error)

	_, err := coordinator.Commit(context.Background(), intent)
	require.Error(t, err)
	var state string
	require.NoError(t, coordinator.db.Table("native_agent_commit_intents").Select("state").Where("tenant_id=? AND run_id=? AND intent_id=?", 1, "run-1", intent.ID).Row().Scan(&state))
	require.Equal(t, "pending", state)
	require.NoError(t, coordinator.db.Exec("DROP TRIGGER abort_usage_replay").Error)
	require.NoError(t, coordinator.db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_epoch=?, lease_expires_at=? WHERE tenant_id=? AND run_id=?", "worker-2", 8, time.Now().Add(time.Hour), 1, "run-1").Error)
	fresh := fence
	fresh.Owner, fresh.Epoch, fresh.LeaseUntil = "worker-2", 8, time.Now().Add(time.Hour)

	receipt, err := NewNativeCommitCoordinator(reopenRunDB(t, coordinator.db)).Reconcile(context.Background(), fresh, intent.ID)
	require.NoError(t, err)
	require.True(t, receipt.Applied)
	var stored struct {
		Revision, InputTokens, OutputTokens, CachedTokens, CacheReadTokens, CacheCreateTokens         int64
		Provider, Model, ProviderRequestID, FundingRef, BudgetRootRunID, AccountingStatus, Dimensions string
	}
	require.NoError(t, coordinator.db.Table("native_agent_usage_observations").
		Select("revision, input_tokens, output_tokens, cached_tokens, cache_read_tokens, cache_create_tokens, provider, model, provider_request_id, funding_ref, budget_root_run_id, accounting_status, dimensions").
		Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", 1, "run-1", "attempt-usage", "usage-1").Take(&stored).Error)
	require.Equal(t, int64(2), stored.Revision)
	require.Equal(t, int64(11), stored.InputTokens)
	require.Equal(t, int64(7), stored.OutputTokens)
	require.Equal(t, int64(3), stored.CachedTokens)
	require.Equal(t, int64(2), stored.CacheReadTokens)
	require.Equal(t, int64(1), stored.CacheCreateTokens)
	require.Equal(t, "openai", stored.Provider)
	require.Equal(t, "gpt-5", stored.Model)
	require.Equal(t, "provider-request-1", stored.ProviderRequestID)
	require.Equal(t, "budget-1", stored.FundingRef)
	require.Equal(t, "run-1", stored.BudgetRootRunID)
	require.Equal(t, "reported", stored.AccountingStatus)
	require.JSONEq(t, `{"reasoning":4}`, stored.Dimensions)
}

func TestNativeCommitUsageHigherRevisionReplacesCumulativeObservation(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-usage", 7).Error)
	first := nativeCommitIntent(fence, "intent-usage-revision-0", "hash-usage-revision-0")
	first.Usage = []nativecontract.UsageObservation{{
		Version: 1, Run: fence.Run, AttemptID: "attempt-usage", ObservationID: "usage-1", Revision: 0,
		Funding: nativecontract.FundingBinding{Service: "openai", PriceVersion: "gpt-5"}, PromptTokens: 11, CompletionTokens: 7, TotalTokens: 18,
		AccountingStatus: "reported", Dimensions: map[string]int64{"reasoning": 4}, OccurredAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}}
	_, err := coordinator.Commit(context.Background(), first)
	require.NoError(t, err)

	higher := nativeCommitIntent(fence, "intent-usage-revision-1", "hash-usage-revision-1")
	higher.Usage = append([]nativecontract.UsageObservation(nil), first.Usage...)
	higher.Usage[0].Revision = 1
	higher.Usage[0].PromptTokens = 19
	higher.Usage[0].CompletionTokens = 13
	higher.Usage[0].TotalTokens = 32
	higher.Usage[0].Dimensions = map[string]int64{"reasoning": 9}
	_, err = coordinator.Commit(context.Background(), higher)
	require.NoError(t, err)

	var stored struct {
		Revision, InputTokens, OutputTokens int64
		Dimensions                          string
	}
	require.NoError(t, coordinator.db.Table("native_agent_usage_observations").
		Select("revision, input_tokens, output_tokens, dimensions").
		Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", 1, "run-1", "attempt-usage", "usage-1").Take(&stored).Error)
	require.Equal(t, int64(1), stored.Revision)
	require.Equal(t, int64(19), stored.InputTokens)
	require.Equal(t, int64(13), stored.OutputTokens)
	require.JSONEq(t, `{"reasoning":9}`, stored.Dimensions)
}

func TestNativeCommitUsageStaleRevisionConflicts(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-usage", 7).Error)
	current := nativeCommitIntent(fence, "intent-usage-current", "hash-usage-current")
	current.Usage = []nativecontract.UsageObservation{{
		Version: 1, Run: fence.Run, AttemptID: "attempt-usage", ObservationID: "usage-1", Revision: 2,
		Funding: nativecontract.FundingBinding{Service: "openai", PriceVersion: "gpt-5"}, PromptTokens: 19, CompletionTokens: 13, TotalTokens: 32,
		AccountingStatus: "reported", OccurredAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}}
	_, err := coordinator.Commit(context.Background(), current)
	require.NoError(t, err)

	stale := nativeCommitIntent(fence, "intent-usage-stale", "hash-usage-stale")
	stale.Usage = append([]nativecontract.UsageObservation(nil), current.Usage...)
	stale.Usage[0].Revision = 1
	stale.Usage[0].PromptTokens = 11
	stale.Usage[0].CompletionTokens = 7
	stale.Usage[0].TotalTokens = 18
	_, err = coordinator.Commit(context.Background(), stale)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeCommitUsageSameRevisionIsIdempotentAndChangedPayloadConflicts(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-usage", 7).Error)
	first := nativeCommitIntent(fence, "intent-usage-first", "hash-usage-first")
	first.Usage = []nativecontract.UsageObservation{{
		Version: 1, Run: fence.Run, AttemptID: "attempt-usage", ObservationID: "usage-1", Revision: 1,
		Funding: nativecontract.FundingBinding{Service: "openai", PriceVersion: "gpt-5"}, PromptTokens: 11,
		AccountingStatus: "reported", OccurredAt: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC),
	}}
	_, err := coordinator.Commit(context.Background(), first)
	require.NoError(t, err)

	replay := nativeCommitIntent(fence, "intent-usage-replay", "hash-usage-replay")
	replay.Usage = append([]nativecontract.UsageObservation(nil), first.Usage...)
	_, err = coordinator.Commit(context.Background(), replay)
	require.NoError(t, err)

	changed := nativeCommitIntent(fence, "intent-usage-changed", "hash-usage-changed")
	changed.Usage = append([]nativecontract.UsageObservation(nil), first.Usage...)
	changed.Usage[0].CompletionTokens = 1
	_, err = coordinator.Commit(context.Background(), changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeCommitSessionAppendCanonicalizesHashAndRequiresEventID(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	e := &event.Event{ID: "event-1", Author: "agent"}
	valid := nativeCommitIntent(fence, "intent-valid", "hash-valid")
	valid.SessionAppends = []nativecontract.SessionAppend{{
		Key:           session.Key{AppName: "weknora/native-v1/tenant/1", UserID: "owner/dTE", SessionID: "session/czE"},
		StableEventID: e.ID, PayloadHash: "caller-supplied-hash", Event: e,
	}}
	_, err := coordinator.Commit(context.Background(), valid)
	require.NoError(t, err)
	var stored string
	require.NoError(t, coordinator.db.Table("native_agent_session_events").Select("payload_hash").Where("tenant_id=? AND stable_event_id=?", 1, e.ID).Row().Scan(&stored))
	require.Equal(t, nativeCommitSessionHash(t, e), stored)

	wrongID := valid
	wrongID.ID, wrongID.PayloadHash = "intent-wrong-id", "hash-wrong-id"
	wrongID.SessionAppends = append([]nativecontract.SessionAppend(nil), valid.SessionAppends...)
	wrongID.SessionAppends[0].StableEventID = "another-id"
	_, err = coordinator.Commit(context.Background(), wrongID)
	require.Equal(t, nativecontract.ErrInvalid, failureCode(t, err))
}

func TestNativeCommitCheckpointRequiresAndRecordsDurableToolReceipt(t *testing.T) {
	coordinator, fence := nativeCommitFixture(t)
	ctx := context.Background()
	checkpoint := &nativecontract.CheckpointWrite{
		SchemaVersion: 1, SDKVersion: "sdk-1", GraphVersion: "graph-1", Namespace: "native/run", LineageID: "lineage-1",
		Request: graph.PutFullRequest{Checkpoint: &graph.Checkpoint{ID: "checkpoint-1"}}, ResultCallIDs: []string{"call-1"},
	}
	missing := nativeCommitIntent(fence, "intent-missing", "hash-missing")
	missing.Checkpoint = checkpoint
	_, err := coordinator.Commit(ctx, missing)
	require.Equal(t, nativecontract.ErrCheckpoint, failureCode(t, err))

	// Tool-call identity is owned by the already durable P1.4 journal. Commit
	// writes its actual result into that identity before it makes the checkpoint runnable.
	applied := nativeCommitIntent(fence, "intent-applied", "hash-applied")
	applied.Checkpoint = checkpoint
	applied.Results = []nativecontract.ToolOutcome{nativeCommitToolOutcome(t, coordinator, fence, "model-1", "attempt-1", "call-1", "args-1", "result-1")}
	_, err = coordinator.Commit(ctx, applied)
	require.NoError(t, err)
	var resultCalls string
	require.NoError(t, coordinator.db.Table("native_agent_checkpoints").Select("result_call_ids").Where("tenant_id=? AND run_id=? AND checkpoint_id=?", 1, "run-1", "checkpoint-1").Row().Scan(&resultCalls))
	require.JSONEq(t, `["call-1"]`, resultCalls)
}
