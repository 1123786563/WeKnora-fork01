package repository

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func installRunViewEffectBarrier(t *testing.T, register func(func(*gorm.DB)) error, table string) (<-chan struct{}, chan struct{}) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	callback := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			once.Do(func() {
				close(entered)
				select {
				case <-release:
				case <-time.After(10 * time.Second):
					t.Error("timed out waiting to release the Run transition barrier")
				}
			})
		}
	}
	require.NoError(t, register(callback))
	return entered, release
}

func waitRunViewEffectBarrier(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the transactional Run barrier")
	}
}

func TestCraftRunViewEffectClaimFirstBarrierSerializesCancellation(t *testing.T) {
	db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
	entered, release := installRunViewEffectBarrier(t, func(callback func(*gorm.DB)) error {
		return db.Callback().Create().Before("gorm:create").Register("test:craft_run_view_effect_claim_barrier", callback)
	}, "craft_run_view_effect_intents")
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	type claimResult struct {
		claim   craft.RunViewEffectClaim
		maySend bool
		err     error
	}
	claimDone := make(chan claimResult, 1)
	go func() {
		claim, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
		claimDone <- claimResult{claim: claim, maySend: maySend, err: err}
	}()
	waitRunViewEffectBarrier(t, entered)

	transitionStarted, transitionDone := make(chan struct{}), make(chan error, 1)
	go func() {
		close(transitionStarted)
		transitionDone <- NewAgentRunStore(db).CancelRun(ctx, task.Fence.RunKey, "member_stop")
	}()
	<-transitionStarted
	close(release)
	claim := <-claimDone
	require.NoError(t, claim.err)
	require.True(t, claim.maySend)
	require.ErrorIs(t, <-transitionDone, ErrCraftRunViewEffectUnresolved)

	var run agentRunRow
	require.NoError(t, db.Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Take(&run).Error)
	require.Equal(t, "running", run.Status)
	var slot *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Scan(&slot).Error)
	require.NotNil(t, slot)
	require.Equal(t, task.Fence.RunID, *slot)
}

func TestCraftRunViewEffectTransitionFirstBarrierDeniesClaim(t *testing.T) {
	db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
	entered, release := installRunViewEffectBarrier(t, func(callback func(*gorm.DB)) error {
		return db.Callback().Update().After("gorm:update").Register("test:craft_run_view_effect_transition_barrier", callback)
	}, "agent_runs")
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()

	transitionDone := make(chan error, 1)
	go func() { transitionDone <- NewAgentRunStore(db).CancelRun(ctx, task.Fence.RunKey, "member_stop") }()
	waitRunViewEffectBarrier(t, entered)
	claimDone := make(chan error, 1)
	go func() {
		_, _, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
		claimDone <- err
	}()
	close(release)
	require.NoError(t, <-transitionDone)
	require.ErrorIs(t, <-claimDone, agentruntime.ErrLeaseLost)
	var count int64
	require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id=? AND run_id=? AND effect_kind=?", task.Fence.TenantID, task.Fence.RunID, craft.RunViewEffectDockerCreate).Count(&count).Error)
	require.Zero(t, count, "the committed transition wins before any provider send intent")
}

func TestCraftRunViewEffectClaimFirstBlocksRunTransitions(t *testing.T) {
	transitions := []struct {
		name string
		run  func(*testing.T, *AgentRunStore, context.Context, craft.Task, *CraftRunViewEffectStore) error
	}{
		{name: "cancel", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.CancelRun(ctx, task.Fence.RunKey, "member_stop")
		}},
		{name: "owned cancel", run: func(t *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			run, err := runs.Get(ctx, task.Fence.RunKey)
			require.NoError(t, err)
			return runs.CancelRunOwnedAtRevision(ctx, task.Scope.TenantID, task.Scope.UserID, task.Fence.RunID, run.Revision, "member_stop")
		}},
		{name: "pause", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.SetStatus(ctx, task.Fence, "waiting_user", "needs_input")
		}},
		{name: "success status", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.SetStatus(ctx, task.Fence, "succeeded", "done")
		}},
		{name: "failure status", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.SetStatus(ctx, task.Fence, "failed", "failed")
		}},
		{name: "event driven finalize", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`))
		}},
		{name: "lease recovery", run: func(t *testing.T, runs *AgentRunStore, _ context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			require.NoError(t, runs.db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", task.Fence.TenantID, task.Fence.RunID).
				Update("lease_until", time.Now().Add(-time.Minute)).Error)
			_, err := runs.ClaimDriver(context.Background(), task.Fence.RunKey, "platform", "replacement-worker", time.Hour)
			return err
		}},
		{name: "decision cancel", run: func(t *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			require.NoError(t, runs.db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='effect-pending', lease_owner='', lease_until=NULL, revision=revision+1 WHERE tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Error)
			run, err := runs.Get(ctx, task.Fence.RunKey)
			require.NoError(t, err)
			_, err = runs.ApplyDecision(ctx, task.Fence.RunKey, task.Scope.UserID, agentruntime.Decision{
				PendingID: "effect-pending", DecisionID: "effect-decision", Action: "terminate", ExpectedRevision: run.Revision,
			})
			return err
		}},
		{name: "session deletion", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			return runs.DeleteSessionRuns(ctx, task.Scope.TenantID, task.Scope.SessionID)
		}},
		{name: "admission transfer", run: func(t *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task, _ *CraftRunViewEffectStore) error {
			require.NoError(t, runs.db.Table("sessions").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Update("active_agent_run_id", nil).Error)
			next := craftSeedAdmission(t, "run-effect-next", "request-effect-next", task.Scope.UserID, "Next", "")
			_, err := runs.Admit(ctx, next)
			return err
		}},
	}
	for _, state := range []string{"pending", "unknown"} {
		for _, transition := range transitions {
			t.Run(state+"/"+transition.name, func(t *testing.T) {
				db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
				claim, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerStart)
				require.NoError(t, err)
				require.True(t, maySend)
				if state == "unknown" {
					require.NoError(t, effects.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: "runtime-id"}))
				}
				runs := NewAgentRunStore(db)
				var before agentRunRow
				require.NoError(t, db.Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Take(&before).Error)
				err = transition.run(t, runs, ctx, task, effects)
				require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved)
				var run agentRunRow
				require.NoError(t, db.Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Take(&run).Error)
				require.Equal(t, before.Epoch, run.Epoch, "an unresolved effect cannot transfer the Run writer epoch")
				if transition.name != "decision cancel" {
					require.Equal(t, before.Status, run.Status)
					require.Equal(t, before.LeaseOwner, run.LeaseOwner)
				}
				require.NotEqual(t, "canceled", run.Status)
				require.NotEqual(t, "succeeded", run.Status)
				require.NotEqual(t, "failed", run.Status)
				var active *string
				require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Scan(&active).Error)
				if transition.name != "admission transfer" {
					require.NotNil(t, active, "unresolved provider effects retain the existing writer slot")
					require.Equal(t, task.Fence.RunID, *active)
				}
				if transition.name == "session deletion" {
					var count int64
					require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type='cancellation_requested'", task.Fence.TenantID, task.Fence.RunID).Count(&count).Error)
					require.Zero(t, count, "deletion cannot record cancellation before effect reconciliation")
				}
				if transition.name == "admission transfer" {
					var count int64
					require.NoError(t, db.Table("agent_runs").Where("tenant_id=? AND run_id=?", task.Scope.TenantID, "run-effect-next").Count(&count).Error)
					require.Zero(t, count)
				}
			})
		}
	}
}

func TestCraftRunViewEffectTransitionFirstDeniesClaim(t *testing.T) {
	transitions := []struct {
		name string
		run  func(*testing.T, *AgentRunStore, context.Context, craft.Task) error
	}{
		{name: "cancel", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.CancelRun(ctx, task.Fence.RunKey, "member_stop")
		}},
		{name: "failed status", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.SetStatus(ctx, task.Fence, "failed", "safe test failure")
		}},
		{name: "pause", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.SetStatus(ctx, task.Fence, "waiting_user", "needs_input")
		}},
		{name: "success status", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.SetStatus(ctx, task.Fence, "succeeded", "done")
		}},
		{name: "finalize", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`))
		}},
		{name: "lease recovery", run: func(t *testing.T, runs *AgentRunStore, _ context.Context, task craft.Task) error {
			require.NoError(t, runs.db.Table("agent_runs").Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Update("lease_until", time.Now().Add(-time.Minute)).Error)
			_, err := runs.ClaimDriver(context.Background(), task.Fence.RunKey, "platform", "new-writer", time.Hour)
			return err
		}},
		{name: "owned cancel", run: func(t *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			run, err := runs.Get(ctx, task.Fence.RunKey)
			require.NoError(t, err)
			return runs.CancelRunOwnedAtRevision(ctx, task.Scope.TenantID, task.Scope.UserID, task.Fence.RunID, run.Revision, "member_stop")
		}},
		{name: "session deletion", run: func(_ *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			return runs.DeleteSessionRuns(ctx, task.Scope.TenantID, task.Scope.SessionID)
		}},
		{name: "decision cancel", run: func(t *testing.T, runs *AgentRunStore, ctx context.Context, task craft.Task) error {
			require.NoError(t, runs.db.Exec("UPDATE agent_runs SET status='waiting_user', wait_reason='effect-pending', lease_owner='', lease_until=NULL, revision=revision+1 WHERE tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Error)
			run, err := runs.Get(ctx, task.Fence.RunKey)
			require.NoError(t, err)
			_, err = runs.ApplyDecision(ctx, task.Fence.RunKey, task.Scope.UserID, agentruntime.Decision{
				PendingID: "effect-pending", DecisionID: "effect-decision", Action: "terminate", ExpectedRevision: run.Revision,
			})
			return err
		}},
	}
	for _, transition := range transitions {
		t.Run(transition.name, func(t *testing.T) {
			db, ctx, task, effects := newCraftRunViewEffectUnallocatedFixture(t)
			runs := NewAgentRunStore(db)
			require.NoError(t, transition.run(t, runs, ctx, task))
			_, maySend, err := effects.BeginEffect(ctx, task, "no-view-generation", craft.RunViewEffectDockerStart)
			require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
			require.False(t, maySend)
			var count int64
			require.NoError(t, db.Table("craft_run_view_effect_intents").Where("tenant_id=? AND run_id=? AND effect_kind=?", task.Fence.TenantID, task.Fence.RunID, craft.RunViewEffectDockerStart).Count(&count).Error)
			require.Zero(t, count, "transition-first must not create a provider intent")
		})
	}
}

func TestCraftRunViewEffectResolutionAllowsFinalizeOnce(t *testing.T) {
	db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
	claim, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerStart)
	require.NoError(t, err)
	require.True(t, maySend)
	runs := NewAgentRunStore(db)
	require.ErrorIs(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)), ErrCraftRunViewEffectUnresolved)
	require.NoError(t, effects.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "runtime-id"}))
	views := NewCraftRunViewStore(db)
	_, _, err = views.BeginSessionCreate(ctx, view.Key, view.Generation)
	require.NoError(t, err)
	_, err = views.BindRuntime(ctx, view.Key, view.Generation, craft.RunViewRuntime{RuntimeID: "runtime-id", ContainerID: "container-id", OpenCodeSessionID: "session-id"})
	require.NoError(t, err)
	require.NoError(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)))
	require.NoError(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)), "terminal enqueue replay remains idempotent")
	var completed, active int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type='run_completed'", task.Fence.TenantID, task.Fence.RunID).Count(&completed).Error)
	require.EqualValues(t, 1, completed)
	require.NoError(t, db.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", task.Scope.TenantID, task.Scope.SessionID, task.Fence.RunID).Count(&active).Error)
	require.Zero(t, active)
	var captures int64
	require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND run_id=? AND state='pending'", task.Scope.TenantID, task.Fence.RunID).Count(&captures).Error)
	require.EqualValues(t, 1, captures, "delayed terminal enqueue still occurs once after resolution")
}

func TestCraftRunViewEffectUnresolvedSessionRunBlocksAdmissionSlotTransfer(t *testing.T) {
	db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
	_, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectOpenCodeCreate)
	require.NoError(t, err)
	require.True(t, maySend)
	require.NoError(t, db.Table("sessions").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Update("active_agent_run_id", nil).Error)

	next := craftSeedAdmission(t, "run-effect-next", "request-effect-next", task.Scope.UserID, "Next", "")
	_, err = NewAgentRunStore(db).Admit(ctx, next)
	require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved)
	var active *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Scan(&active).Error)
	require.Nil(t, active, "failed slot transfer leaves the artificially empty slot unchanged")
	var count int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id=? AND run_id=?", task.Scope.TenantID, next.Key.RunID).Count(&count).Error)
	require.Zero(t, count, "new generation must not be admitted while an older session effect is unresolved")
}

func TestCraftRunViewEffectCannotBeBypassedByChargeReconciliationOrDeleteCleanup(t *testing.T) {
	db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
	_, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
	require.NoError(t, err)
	require.True(t, maySend)
	insertUnresolvedCraftStart(t, db, task.Fence.RunKey, "effect-charge-interaction")
	require.NoError(t, db.Table("agent_runs").Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).
		Updates(map[string]any{"status": "reconciling", "wait_reason": craftChargeStartPendingWaitReason}).Error)

	runs := NewAgentRunStore(db)
	_, err = runs.ClaimDriver(ctx, task.Fence.RunKey, "platform", "charge-recovery-worker", time.Hour)
	require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved, "charge-start recovery cannot bypass an unresolved RunView operation")
	err = runs.DeleteSessionRuns(ctx, task.Scope.TenantID, task.Scope.SessionID)
	require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved, "session cleanup cannot turn a pending effect into a charge-only pause/cancel")

	var run agentRunRow
	require.NoError(t, db.Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, craftChargeStartPendingWaitReason, run.WaitReason)
	var slot *string
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Scan(&slot).Error)
	require.NotNil(t, slot)
	require.Equal(t, task.Fence.RunID, *slot)
	var cancelEvents int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type IN ?", task.Fence.TenantID, task.Fence.RunID, []string{"craft_charge_cancel_requested", "cancellation_requested", "cancellation_confirmed"}).Count(&cancelEvents).Error)
	require.Zero(t, cancelEvents, "unresolved RunView operation remains separate from budget settlement")
}

func TestCraftRunViewEffectTransitionFencePostgres(t *testing.T) {
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL transition fence NOT VERIFIED")
	}
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("options", "-c app.skip_embedding=true")
	parsed.RawQuery = query.Encode()
	t.Setenv("TRPC_TEST_POSTGRES_DSN", parsed.String())
	t.Run("postgres", func(t *testing.T) {
		t.Run("pending-effect-delays-terminal-and-capture", func(t *testing.T) {
			db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
			var skipEmbedding string
			require.NoError(t, db.Raw("SELECT current_setting('app.skip_embedding', true)").Scan(&skipEmbedding).Error)
			require.Equal(t, "true", skipEmbedding)
			claim, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerCreate)
			require.NoError(t, err)
			require.True(t, maySend)
			runs := NewAgentRunStore(db)
			require.ErrorIs(t, runs.CancelRun(ctx, task.Fence.RunKey, "stop"), ErrCraftRunViewEffectUnresolved)
			require.ErrorIs(t, runs.SetStatus(ctx, task.Fence, "failed", "failed"), ErrCraftRunViewEffectUnresolved)
			require.ErrorIs(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)), ErrCraftRunViewEffectUnresolved)
			_, err = runs.ClaimDriver(ctx, task.Fence.RunKey, "platform", "replacement-worker", time.Hour)
			require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved)
			require.NoError(t, effects.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: "runtime-id"}))
			views := NewCraftRunViewStore(db)
			_, _, err = views.BeginSessionCreate(ctx, view.Key, view.Generation)
			require.NoError(t, err)
			_, err = views.BindRuntime(ctx, view.Key, view.Generation, craft.RunViewRuntime{RuntimeID: "runtime-id", ContainerID: "container-id", OpenCodeSessionID: "session-id"})
			require.NoError(t, err)
			require.NoError(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)))
			require.NoError(t, runs.Finalize(ctx, task.Fence, json.RawMessage(`{"content":"answer"}`)))
			var captures int64
			require.NoError(t, db.Table("craft_run_captures").Where("tenant_id=? AND run_id=? AND state='pending'", task.Scope.TenantID, task.Fence.RunID).Count(&captures).Error)
			require.EqualValues(t, 1, captures, "terminal capture trigger runs once after the intent resolves")
		})

		t.Run("unknown-effect-retains-slot-through-session-delete", func(t *testing.T) {
			db, ctx, task, effects, view := newCraftRunViewEffectFixture(t)
			claim, maySend, err := effects.BeginEffect(ctx, task, view.Generation, craft.RunViewEffectDockerStart)
			require.NoError(t, err)
			require.True(t, maySend)
			require.NoError(t, effects.FinishEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: "runtime-unknown"}))
			require.NoError(t, db.Table("agent_runs").Where("tenant_id=? AND run_id=?", task.Fence.TenantID, task.Fence.RunID).Update("lease_until", time.Now().Add(-time.Minute)).Error)
			_, err = NewAgentRunStore(db).ClaimDriver(ctx, task.Fence.RunKey, "platform", "replacement-worker", time.Hour)
			require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved)
			err = NewAgentRunStore(db).DeleteSessionRuns(ctx, task.Scope.TenantID, task.Scope.SessionID)
			require.ErrorIs(t, err, ErrCraftRunViewEffectUnresolved)
			var slot *string
			require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id=? AND id=?", task.Scope.TenantID, task.Scope.SessionID).Scan(&slot).Error)
			require.NotNil(t, slot)
			require.Equal(t, task.Fence.RunID, *slot)
		})
	})
}
