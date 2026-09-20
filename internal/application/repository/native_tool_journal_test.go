package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

type repositoryTestCallableTool struct{ calls int }

func (t *repositoryTestCallableTool) Declaration() *tool.Declaration {
	return &tool.Declaration{Name: "write"}
}
func (t *repositoryTestCallableTool) Call(context.Context, []byte) (any, error) {
	t.calls++
	return "ok", nil
}

type repositoryTestStreamableTool struct{ calls int }

func (t *repositoryTestStreamableTool) Declaration() *tool.Declaration {
	return &tool.Declaration{Name: "stream-write"}
}
func (t *repositoryTestStreamableTool) StreamableCall(context.Context, []byte) (*tool.StreamReader, error) {
	t.calls++
	return tool.NewStream(1).Reader, nil
}

type repositoryPreflight func(context.Context, nativecontract.ToolDispatchRequest) error

func (f repositoryPreflight) Authorize(ctx context.Context, request nativecontract.ToolDispatchRequest) error {
	return f(ctx, request)
}

func allowRepositoryDispatch(context.Context, nativecontract.ToolDispatchRequest) error { return nil }

type repositoryFailingPreparer struct{ calls int }

func (p *repositoryFailingPreparer) PrepareToolAttempt(context.Context, nativecontract.Fence, nativecontract.Attempt, nativecontract.ToolPlan) (nativecontract.Attempt, error) {
	p.calls++
	return nativecontract.Attempt{}, &nativecontract.Failure{Code: nativecontract.ErrStore}
}

func nativeToolJournalFixture(t *testing.T) (*NativeToolJournal, nativecontract.Fence) {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", 1, "owner", "session").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, status, lease_owner, lease_epoch, lease_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "owner", "session", "running", "worker-1", 7, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch, kind, attempt_number)
		VALUES (?, ?, ?, ?, ?, ?)`, 1, "run-1", "model-1", 7, "model", 1).Error)
	return NewNativeToolJournal(db), nativecontract.Fence{
		Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, Owner: "worker-1", Epoch: 7,
	}
}

func nativeToolPlan() nativecontract.ToolPlan {
	return nativecontract.ToolPlan{
		Version: 1, Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"},
		CallID: "call-1", ProviderToolCallID: "provider-call-1", ModelAttemptID: "model-1",
		Tool: nativecontract.ToolIdentity{Kind: "connector", ServiceID: "svc", InstallationID: "install", Name: "write", SchemaHash: "schema-v1", ConfigVersion: "config-v1"},
		Args: json.RawMessage(`{"record":"one"}`), ArgsHash: "args-v1", Policy: nativecontract.RecoveryHold,
	}
}

func TestNativeToolPlanIsFencedAndChangedArgumentsConflict(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	plan := nativeToolPlan()

	stored, err := journal.Plan(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, plan, stored)
	_, err = journal.Plan(ctx, fence, plan)
	require.NoError(t, err, "the same call plan is idempotent")
	changed := plan
	changed.Args = json.RawMessage(`{"record":"two"}`)
	changed.ArgsHash = "args-v2"
	_, err = journal.Plan(ctx, fence, changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))

	stale := fence
	stale.Epoch--
	_, err = journal.Plan(ctx, stale, nativeToolPlan())
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
}

func TestNativeToolAttemptPersistsUnknownEffectWithoutForgingResult(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	require.NoError(t, func() error { _, err := journal.Plan(ctx, fence, nativeToolPlan()); return err }())

	attempt, err := journal.Begin(ctx, fence, nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 2, Epoch: fence.Epoch, StartedAt: time.Now()})
	require.NoError(t, err)
	require.Equal(t, nativecontract.ToolAttempt, attempt.Kind)
	require.NoError(t, journal.Finish(ctx, fence, attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrUnknownEffect, Effect: nativecontract.EffectUnknown}))

	got, err := journal.Get(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, attempt.ID)
	require.NoError(t, err)
	require.Equal(t, attempt.ID, got.ID)
	var effect string
	require.NoError(t, journal.db.Table("native_agent_attempts").Select("effect_state").Where("tenant_id=? AND run_id=? AND attempt_id=?", 1, "run-1", attempt.ID).Scan(&effect).Error)
	require.Equal(t, string(nativecontract.EffectUnknown), effect)
	var failure string
	require.NoError(t, journal.db.Table("native_agent_tool_results").Select("failure").Where("tenant_id=? AND run_id=? AND attempt_id=? AND call_id=?", 1, "run-1", attempt.ID, "call-1").Scan(&failure).Error)
	require.Contains(t, failure, string(nativecontract.ErrUnknownEffect))
	_, err = journal.LookupResult(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "model-1", "call-1")
	require.Equal(t, nativecontract.ErrNotFound, failureCode(t, err), "unknown effect is not a confirmed result")
}

func TestNativeToolBoundaryRejectsStreamableDelegateUntilCompletionAdapterExists(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	boundary := NewNativeToolBoundary(journal.db, NewNativeCommitCoordinator(journal.db))
	plan := nativeToolPlan()
	dispatch := NativeToolDispatch{
		Fence: fence, Plan: plan,
		Attempt:        nativecontract.Attempt{ID: "tool-stream", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: plan.CallID, Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
		CommitIntentID: "intent-stream", DecisionReference: "decision-stream", Preflight: repositoryPreflight(allowRepositoryDispatch),
	}
	delegate := &repositoryTestStreamableTool{}
	_, err := boundary.Wrap(WithNativeToolDispatch(context.Background(), dispatch), nativecontract.Scope{TenantID: 1}, plan.Tool, delegate)
	require.Equal(t, nativecontract.ErrForbidden, failureCode(t, err))
	require.Zero(t, delegate.calls)
}

func TestNativeToolBoundaryPreparesAttemptBeforePreflightConsumption(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	boundary := NewNativeToolBoundary(journal.db, NewNativeCommitCoordinator(journal.db))
	plan := nativeToolPlan()
	preflightCalls := 0
	preparer := &repositoryFailingPreparer{}
	dispatch := NativeToolDispatch{
		Fence: fence, Plan: plan,
		Attempt:        nativecontract.Attempt{ID: "tool-prepare-fail", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: plan.CallID, Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
		CommitIntentID: "intent-prepare-fail", DecisionReference: "decision-prepare-fail", Preparer: preparer,
		Preflight: repositoryPreflight(func(context.Context, nativecontract.ToolDispatchRequest) error {
			preflightCalls++
			return nil
		}),
	}
	delegate := &repositoryTestCallableTool{}
	wrapped, err := boundary.Wrap(WithNativeToolDispatch(context.Background(), dispatch), nativecontract.Scope{TenantID: 1}, plan.Tool, delegate)
	require.NoError(t, err)
	_, err = wrapped.(tool.CallableTool).Call(context.Background(), plan.Args)
	require.Equal(t, nativecontract.ErrStore, failureCode(t, err))
	require.Equal(t, 1, preparer.calls)
	require.Zero(t, preflightCalls, "a failed durable attempt must not consume a decision or budget reservation")
	require.Zero(t, delegate.calls)
}

func TestNativeToolAttemptRejectsChangedImmutableIdentity(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	_, err := journal.Plan(ctx, fence, nativeToolPlan())
	require.NoError(t, err)
	attempt := nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", InvocationID: "invoke-1", ProviderRequestID: "provider-1", Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()}
	_, err = journal.Begin(ctx, fence, attempt)
	require.NoError(t, err)
	changed := attempt
	changed.ProviderRequestID = "provider-2"
	_, err = journal.Begin(ctx, fence, changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeToolBoundaryFailsClosedWithoutDurableDispatch(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	delegate := &repositoryTestCallableTool{}
	_, err := journal.Wrap(context.Background(), nativecontract.Scope{TenantID: fence.Run.TenantID}, nativeToolPlan().Tool, delegate)
	require.Error(t, err)
	require.True(t, errors.As(err, new(*nativecontract.Failure)))
	require.Zero(t, delegate.calls)
}

func TestNativeToolBoundaryCommitsConfirmedCallableResultBehindBarrier(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	boundary := NewNativeToolBoundary(journal.db, NewNativeCommitCoordinator(journal.db))
	plan := nativeToolPlan()
	dispatch := NativeToolDispatch{
		Fence: fence, Plan: plan,
		Attempt:           nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: plan.CallID, Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
		CommitIntentID:    "tool-result-1",
		DecisionReference: "decision-1",
		Preflight:         repositoryPreflight(allowRepositoryDispatch),
	}
	delegate := &repositoryTestCallableTool{}
	wrapped, err := boundary.Wrap(WithNativeToolDispatch(context.Background(), dispatch), nativecontract.Scope{TenantID: 1}, plan.Tool, delegate)
	require.NoError(t, err)
	callable, ok := wrapped.(tool.CallableTool)
	require.True(t, ok)
	result, err := callable.Call(context.Background(), plan.Args)
	require.NoError(t, err)
	require.Equal(t, "ok", result)
	require.Equal(t, 1, delegate.calls)
	stored, err := boundary.LookupResult(context.Background(), nativecontract.Scope{TenantID: 1}, fence.Run, plan.ModelAttemptID, plan.CallID)
	require.NoError(t, err)
	require.Equal(t, nativecontract.EffectConfirmed, stored.Effect)
	require.NoError(t, NewNativeCommitCoordinator(journal.db).Barrier(context.Background(), fence, dispatch.CommitIntentID))
}

func TestNativeToolBoundaryPreflightFailureMakesZeroDelegateCalls(t *testing.T) {
	for _, code := range []nativecontract.ErrorCode{nativecontract.ErrForbidden, nativecontract.ErrBudget, nativecontract.ErrLeaseLost} {
		t.Run(string(code), func(t *testing.T) {
			journal, fence := nativeToolJournalFixture(t)
			boundary := NewNativeToolBoundary(journal.db, NewNativeCommitCoordinator(journal.db))
			plan := nativeToolPlan()
			delegate := &repositoryTestCallableTool{}
			dispatch := NativeToolDispatch{
				Fence: fence, Plan: plan,
				Attempt:        nativecontract.Attempt{ID: "tool-" + string(code), Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: plan.CallID, Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
				CommitIntentID: "intent-" + string(code), DecisionReference: "decision-" + string(code),
				Preflight: repositoryPreflight(func(context.Context, nativecontract.ToolDispatchRequest) error {
					return &nativecontract.Failure{Code: code}
				}),
			}
			wrapped, err := boundary.Wrap(WithNativeToolDispatch(context.Background(), dispatch), nativecontract.Scope{TenantID: 1}, plan.Tool, delegate)
			require.NoError(t, err)
			_, err = wrapped.(tool.CallableTool).Call(context.Background(), plan.Args)
			require.Equal(t, code, failureCode(t, err))
			require.Zero(t, delegate.calls)
		})
	}
}

func TestNativeToolBoundaryNeverRedispatchesWhenAtomicOutcomeIntentWriteFails(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	boundary := NewNativeToolBoundary(journal.db, NewNativeCommitCoordinator(journal.db))
	plan := nativeToolPlan()
	preflightCalls := 0
	dispatch := NativeToolDispatch{
		Fence: fence, Plan: plan,
		Attempt:        nativecontract.Attempt{ID: "tool-atomic", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: plan.CallID, Number: 2, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
		CommitIntentID: "intent-atomic", DecisionReference: "decision-atomic", Preflight: repositoryPreflight(func(context.Context, nativecontract.ToolDispatchRequest) error {
			preflightCalls++
			return nil
		}),
	}
	require.NoError(t, journal.db.Exec(`CREATE TRIGGER abort_tool_intent BEFORE INSERT ON native_agent_commit_intents BEGIN SELECT RAISE(ABORT, 'intent unavailable'); END`).Error)
	delegate := &repositoryTestCallableTool{}
	wrapped, err := boundary.Wrap(WithNativeToolDispatch(context.Background(), dispatch), nativecontract.Scope{TenantID: 1}, plan.Tool, delegate)
	require.NoError(t, err)
	_, err = wrapped.(tool.CallableTool).Call(context.Background(), plan.Args)
	require.Error(t, err)
	require.Equal(t, 1, delegate.calls)
	var repair struct{ Effect string }
	require.NoError(t, journal.db.Table("native_agent_tool_results").Select("effect_state AS effect").Where("tenant_id=? AND run_id=? AND attempt_id=?", 1, "run-1", dispatch.Attempt.ID).Take(&repair).Error)
	require.Equal(t, string(nativecontract.EffectUnknown), repair.Effect, "the repair record is unknown, never a forged success")
	require.NoError(t, journal.db.Exec("DROP TRIGGER abort_tool_intent").Error)
	_, err = wrapped.(tool.CallableTool).Call(context.Background(), plan.Args)
	require.Equal(t, nativecontract.ErrUnknownEffect, failureCode(t, err))
	require.Equal(t, 1, delegate.calls, "an unknown non-idempotent external effect must not be redispatched")
	require.Equal(t, 1, preflightCalls, "a held attempt must not consume another decision or budget reservation")
}

func TestNativeToolPlanRejectsDuplicateCallIDAcrossModelAttempts(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	plan := nativeToolPlan()
	require.NoError(t, func() error { _, err := journal.Plan(context.Background(), fence, plan); return err }())
	require.NoError(t, journal.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch, kind, attempt_number) VALUES (?, ?, ?, ?, ?, ?)`, 1, "run-1", "model-2", 7, "model", 2).Error)
	other := plan
	other.ModelAttemptID = "model-2"
	_, err := journal.Plan(context.Background(), fence, other)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativeToolConfirmedOutcomeIsImmutableAndVisibleOnlyInScope(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	require.NoError(t, func() error { _, err := journal.Plan(ctx, fence, nativeToolPlan()); return err }())
	attempt, err := journal.Begin(ctx, fence, nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 2, Epoch: fence.Epoch, StartedAt: time.Now()})
	require.NoError(t, err)
	outcome := nativecontract.ToolOutcome{AttemptID: attempt.ID, CallID: "call-1", ProviderReceipt: "receipt-1", ResultHash: "result-1", Effect: nativecontract.EffectConfirmed, Content: json.RawMessage(`{"ok":true}`)}
	require.NoError(t, journal.RecordOutcome(ctx, fence, outcome))
	require.NoError(t, journal.RecordOutcome(ctx, fence, outcome))
	changed := outcome
	changed.ResultHash = "result-2"
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, journal.RecordOutcome(ctx, fence, changed)))

	got, err := journal.LookupResult(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "model-1", "call-1")
	require.NoError(t, err)
	require.Equal(t, outcome, got)
	_, err = journal.LookupResult(ctx, nativecontract.Scope{TenantID: 2}, fence.Run, "model-1", "call-1")
	require.Equal(t, nativecontract.ErrForbidden, failureCode(t, err))
}
