package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type craftBudgetPauseIntentView struct {
	ExtensionAction *struct {
		Key          string `json:"key"`
		ExtraCalls   int    `json:"extra_calls"`
		ExtraCredits int64  `json:"extra_credits"`
	} `json:"extension_action"`
}

func craftBudgetPauseIntent(t *testing.T, pause craft.BudgetPause) *struct {
	Key          string `json:"key"`
	ExtraCalls   int    `json:"extra_calls"`
	ExtraCredits int64  `json:"extra_credits"`
} {
	t.Helper()
	encoded, err := json.Marshal(pause)
	require.NoError(t, err)
	var view craftBudgetPauseIntentView
	require.NoError(t, json.Unmarshal(encoded, &view))
	require.NotNil(t, view.ExtensionAction, "paused Run exposes its durable server-owned extension action")
	return view.ExtensionAction
}

func newPausedCraftBudgetIntentFixture(t *testing.T, tenant uint64, sessionID, runID string) (*gorm.DB, *CraftBudgetService, craft.Scope) {
	t.Helper()
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, tenant, 100000)
	seedCraftBudgetRun(t, db, tenant, runID, sessionID)
	scope := craft.Scope{TenantID: tenant, UserID: "owner", SessionID: sessionID}
	grant, err := svc.Admit(context.Background(), scope, runID)
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(context.Background(), grant.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(context.Background(), grant.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted)
	return db, svc, scope
}

func TestCraftBudgetExtensionIntentPersistsAcrossResumeFailureAndRenews(t *testing.T) {
	db, svc, scope := newPausedCraftBudgetIntentFixture(t, 119, "task-intent-journey", "run-intent-journey")
	ctx := context.Background()
	pause, err := svc.BudgetPause(ctx, scope, "run-intent-journey")
	require.NoError(t, err)
	first := craftBudgetPauseIntent(t, pause)
	require.NotEmpty(t, first.Key)
	require.Equal(t, 10, first.ExtraCalls)
	require.Equal(t, int64(svc.policy.TaskLimit), first.ExtraCredits)

	wrongAmount := commercial.Credits(first.ExtraCredits + 1)
	require.ErrorIs(t, svc.ExtendAndResume(ctx, scope, "run-intent-journey", first.Key,
		first.ExtraCalls, wrongAmount), craft.ErrConflict, "a caller cannot change the server-owned credit quantum")
	require.ErrorIs(t, svc.ExtendAndResume(ctx, scope, "run-intent-journey", first.Key,
		first.ExtraCalls+1, commercial.Credits(first.ExtraCredits)), craft.ErrConflict, "a caller cannot change the server-owned call quantum")

	// Reconciliation is settled, then an injected resume write failure proves
	// the already-applied extension remains attached to the same pending action.
	require.NoError(t, db.Model(&repocommercial.ReservationRow{}).
		Where("tenant_id = ? AND run_id = ?", scope.TenantID, "run-intent-journey").
		Update("state", commercial.ReservationStateSettled).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_budget_extension_resume BEFORE UPDATE OF status ON agent_runs
		WHEN NEW.status = 'recovering' BEGIN SELECT RAISE(ABORT, 'injected budget resume failure'); END`).Error)
	resumeErr := svc.ExtendAndResume(ctx, scope, "run-intent-journey", first.Key,
		first.ExtraCalls, commercial.Credits(first.ExtraCredits))
	require.ErrorContains(t, resumeErr, "injected budget resume failure")
	// Model process/service restart after the commercial extension committed
	// but the Run resume transaction failed. The next GET and POST must recover
	// the pending action from the database, with no in-memory cache involved.
	svc, err = NewCraftBudgetService(db, nil, svc.policy)
	require.NoError(t, err)
	refetched, err := svc.BudgetPause(ctx, scope, "run-intent-journey")
	require.NoError(t, err)
	second := craftBudgetPauseIntent(t, refetched)
	require.Equal(t, first.Key, second.Key)
	require.Equal(t, *first, *second)
	var intentStatus string
	require.NoError(t, db.Table("craft_budget_extension_intents").Select("status").
		Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, "run-intent-journey").Take(&intentStatus).Error)
	require.Equal(t, "pending", intentStatus, "failed resume leaves the same action pending")
	var taskBudget repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", scope.TenantID, scope.SessionID).Take(&taskBudget).Error)
	require.Equal(t, int64(5000)+first.ExtraCredits, taskBudget.LimitMicro)
	require.NoError(t, db.Exec("DROP TRIGGER fail_budget_extension_resume").Error)

	var grant CraftBudgetGrantRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", scope.TenantID, "run-intent-journey").Take(&grant).Error)
	require.NoError(t, svc.ExtendAndResume(ctx, scope, "run-intent-journey", first.Key,
		first.ExtraCalls, commercial.Credits(first.ExtraCredits)))
	var run struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", scope.TenantID, "run-intent-journey").Take(&run).Error)
	require.Equal(t, "recovering", run.Status)
	require.Empty(t, run.WaitReason)
	require.NoError(t, db.Table("craft_budget_extension_intents").Select("status").
		Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, "run-intent-journey").Take(&intentStatus).Error)
	require.Equal(t, "completed", intentStatus, "action completes with the committed resume")
	require.NoError(t, svc.PauseRunForBudget(ctx, grant.GrantID))
	freshPause, err := svc.BudgetPause(ctx, scope, "run-intent-journey")
	require.NoError(t, err)
	fresh := craftBudgetPauseIntent(t, freshPause)
	require.NotEqual(t, first.Key, fresh.Key, "a later pause gets a new server-owned action identity")
}

func TestCraftBudgetPauseConcurrentIntentCreationHasSingleWinner(t *testing.T) {
	db, svc, scope := newPausedCraftBudgetIntentFixture(t, 120, "task-intent-race", "run-intent-race")
	const callers = 8
	start := make(chan struct{})
	type result struct {
		pause craft.BudgetPause
		err   error
	}
	results := make(chan result, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			pause, err := svc.BudgetPause(context.Background(), scope, "run-intent-race")
			results <- result{pause: pause, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var winner string
	for result := range results {
		require.NoError(t, result.err)
		intent := craftBudgetPauseIntent(t, result.pause)
		if winner == "" {
			winner = intent.Key
		}
		require.Equal(t, winner, intent.Key)
	}
	require.NotEmpty(t, winner)
	var intents int64
	require.NoError(t, db.Table("craft_budget_extension_intents").
		Where("tenant_id = ? AND session_id = ? AND run_id = ? AND status = ?", scope.TenantID, scope.SessionID, "run-intent-race", "pending").
		Count(&intents).Error)
	require.EqualValues(t, 1, intents)
}

// TestCraftT19Journey exercises the durable budget boundary over the real
// commercial reservation tables and the persisted main Run, including reload.
func TestCraftT19Journey(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 91, 10000)
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('craft107-t19', 91, 'budget', 'owner', 'trpc')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at) VALUES (91, 'run-t19', 'craft107-t19', 'owner', 'req-t19', 'am-t19', 'rh', '{}', 'running', '', ?, ?, ?)", time.Now().Add(time.Hour), time.Now(), time.Now()).Error)
	scope := craft.Scope{TenantID: 91, UserID: "owner", SessionID: "craft107-t19"}
	grant, err := svc.Admit(ctx, scope, "run-t19")
	require.NoError(t, err)
	first, err := svc.AuthorizeBinding(ctx, grant.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NotEmpty(t, first)
	_, err = svc.AuthorizeBinding(ctx, grant.ID, CraftCallBinding{DelegationID: "delegate", ModelID: "oc", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted)
	require.Equal(t, int64(1), craftGrantCalls(t, db, 91, grant.ID))
	require.Len(t, craftReservations(t, db, 91), 1)

	// Reload from durable storage: the Run stays paused and the typed fact is
	// stable, with call counts only (no Credits or credential material).
	reloaded, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	pause, err := reloaded.BudgetPause(ctx, scope, "run-t19")
	require.NoError(t, err)
	require.Equal(t, "run-t19", pause.RunID)
	require.Equal(t, "exhausted", pause.Reason)
	require.Equal(t, int64(1), pause.Limit)
	require.Equal(t, int64(1), pause.Used)
	action := craftBudgetPauseIntent(t, pause)
	// TaskRead collaborators can learn why the Run paused; the HTTP handler
	// separately projects whether this reader may extend the budget.
	readerScope := craft.Scope{TenantID: 91, UserID: "member", SessionID: "craft107-t19"}
	readerPause, err := reloaded.BudgetPause(ctx, readerScope, "run-t19")
	require.NoError(t, err)
	require.Equal(t, "run-t19", readerPause.RunID)
	require.Equal(t, "exhausted", readerPause.Reason)
	require.Equal(t, int64(1), readerPause.Limit)
	require.Equal(t, int64(1), readerPause.Used)
	require.Equal(t, action.Key, craftBudgetPauseIntent(t, readerPause).Key)
	var run struct {
		Status     string
		WaitReason string
	}
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id = ? AND run_id = ?", 91, "run-t19").Take(&run).Error)
	require.Equal(t, "waiting_user", run.Status)
	require.Equal(t, "budget_exhausted", run.WaitReason)
	beforePausedCall := len(craftReservations(t, db, 91))
	_, err = reloaded.AuthorizeBinding(ctx, grant.ID, CraftCallBinding{ModelID: "after-pause", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)
	require.Len(t, craftReservations(t, db, 91), beforePausedCall, "no new reservation after the durable pause")

	// A member cannot purchase an extension. The owner can extend, but an
	// unconfirmed dispatched call is never blindly replayed on resume.
	require.ErrorIs(t, reloaded.ExtendAndResume(ctx, readerScope, "run-t19", action.Key, action.ExtraCalls, commercial.Credits(action.ExtraCredits)), craft.ErrForbidden)
	require.ErrorIs(t, reloaded.ExtendAndResume(ctx, scope, "run-t19", action.Key, action.ExtraCalls, commercial.Credits(action.ExtraCredits)), craft.ErrReconcilePending)
	pause, err = reloaded.BudgetPause(ctx, scope, "run-t19")
	require.NoError(t, err)
	require.Equal(t, "exhausted", pause.Reason)
	require.Equal(t, action.Key, craftBudgetPauseIntent(t, pause).Key)
	require.NoError(t, db.Model(&repocommercial.ReservationRow{}).Where("tenant_id = ? AND run_id = ?", 91, "run-t19").Update("state", commercial.ReservationStateSettled).Error)
	require.NoError(t, reloaded.ExtendAndResume(ctx, scope, "run-t19", action.Key, action.ExtraCalls, commercial.Credits(action.ExtraCredits)))
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id = ? AND run_id = ?", 91, "run-t19").Take(&run).Error)
	require.Equal(t, "recovering", run.Status)
	require.Empty(t, run.WaitReason)
}

func TestCraftBudgetTaskIsSessionAcrossRuns(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.TaskLimit = commercial.Credits(500)
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 92, 10000)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 92, UserID: "owner", SessionID: "craft107-t19"}
	seedCraftBudgetRun(t, db, 92, "run-first", scope.SessionID)
	seedCraftBudgetRun(t, db, 92, "run-second", scope.SessionID)
	first, err := svc.Admit(ctx, scope, "run-first")
	require.NoError(t, err)
	second, err := svc.Admit(ctx, scope, "run-second")
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(ctx, first.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(ctx, second.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)
	var owner repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 92, scope.SessionID).Take(&owner).Error)
	require.Equal(t, int64(500), owner.LimitMicro)
	var child repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 92, "run-second").Take(&child).Error)
	require.Equal(t, scope.SessionID, child.RootRunID)
}

func TestCraftBudgetSandboxActivityIsStableAndSharesTask(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 93, 10000)
	ctx := context.Background()
	seedCraftBudgetRun(t, db, 93, "run-sandbox", "craft107-t19")
	grant, err := svc.Admit(ctx, craft.Scope{TenantID: 93, UserID: "owner", SessionID: "craft107-t19"}, "run-sandbox")
	require.NoError(t, err)
	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-1"))
	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-1"))
	require.Equal(t, int64(1), craftGrantCalls(t, db, 93, grant.ID))
	reservations := craftReservations(t, db, 93)
	require.Len(t, reservations, 1)
	require.Equal(t, "run-sandbox", reservations[0].RunID)
	var child repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 93, "run-sandbox").Take(&child).Error)
	require.Equal(t, "craft107-t19", child.RootRunID)
}

func TestCraftBudgetAdmitDerivesTaskFromDurableRun(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('craft107-t19', 94, 'budget', 'owner', 'trpc')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at) VALUES (94, 'run-live', 'craft107-t19', 'owner', 'req-live', 'am-live', 'rh', '{}', 'running', '', ?, ?, ?)", time.Now().Add(time.Hour), time.Now(), time.Now()).Error)
	grant, err := svc.Admit(ctx, craft.Scope{TenantID: 94, UserID: "owner"}, "run-live")
	require.NoError(t, err)
	require.NotEmpty(t, grant.ID)
	var child repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 94, "run-live").Take(&child).Error)
	require.Equal(t, "craft107-t19", child.RootRunID)
}

func TestCraftBudgetExtensionReplayRepairsCallCap(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 95, 10000)
	seedCraftBudgetRun(t, db, 95, "run-extension", "task-extension")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 95, UserID: "owner", SessionID: "task-extension"}, "run-extension")
	require.NoError(t, err)

	// Simulate process loss after the independent G4 extension transaction
	// committed but before the Craft call-count fence was updated.
	require.NoError(t, svc.budget.ExtendTaskLimit(context.Background(), 95, "run-extension", "retry-extension", commercial.Credits(500)))
	const retries = 8
	var wg sync.WaitGroup
	errCh := make(chan error, retries)
	for i := 0; i < retries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- svc.Extend(context.Background(), grant.ID, "retry-extension", 2, commercial.Credits(500))
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	snapshot, err := svc.GrantSnapshot(context.Background(), grant.ID)
	require.NoError(t, err)
	require.Equal(t, 3, snapshot.MaxCalls)
	var task repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 95, "task-extension").Take(&task).Error)
	require.Equal(t, int64(5500), task.LimitMicro, "G4 extension applies once despite concurrent replay")
	var extensions int64
	require.NoError(t, db.Model(&repocommercial.TaskBudgetExtensionRow{}).
		Where("tenant_id = ? AND run_id = ? AND key = ?", 95, "run-extension", "retry-extension").Count(&extensions).Error)
	require.EqualValues(t, 1, extensions)
}

func seedCraftBudgetRun(t *testing.T, db *gorm.DB, tenant uint64, runID, sessionID string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Exec("INSERT OR IGNORE INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, 'budget test', 'owner', 'trpc')", sessionID, tenant).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (?, ?, ?, 'owner', ?, ?, 'hash', '{}', 'running', '', ?, ?, ?)`,
		tenant, runID, sessionID, "req-"+runID, "msg-"+runID, now.Add(time.Hour), now, now).Error)
}

func TestCraftBudgetLaterRunRenewsExpiredTaskDeadline(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 96, 10000)
	seedCraftBudgetRun(t, db, 96, "run-first", "task-deadline")
	seedCraftBudgetRun(t, db, 96, "run-later", "task-deadline")
	ctx := context.Background()
	first, err := svc.Admit(ctx, craft.Scope{TenantID: 96, UserID: "owner", SessionID: "task-deadline"}, "run-first")
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(ctx, first.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NoError(t, db.Model(&repocommercial.TaskBudgetRow{}).
		Where("tenant_id = ? AND run_id = ?", 96, "task-deadline").
		Update("deadline", time.Now().UTC().Add(-time.Minute)).Error)

	later, err := svc.Admit(ctx, craft.Scope{TenantID: 96, UserID: "owner", SessionID: "task-deadline"}, "run-later")
	require.NoError(t, err)
	require.True(t, later.Deadline.After(first.Deadline))
	_, err = svc.AuthorizeBinding(ctx, later.ID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	var root repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 96, "task-deadline").Take(&root).Error)
	require.True(t, root.Deadline.After(time.Now().UTC()))
	require.Zero(t, root.SpentMicro)
	require.Equal(t, int64(1000), root.HeldMicro, "deadline renewal preserves the first Run's hold")
}

func TestCraftBudgetAdmissionAndChargeRequireDurableMatchingRun(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 97, 10000)
	ctx := context.Background()

	_, err = svc.Admit(ctx, craft.Scope{TenantID: 97, UserID: "owner", SessionID: "task-missing"}, "run-missing")
	require.ErrorIs(t, err, craft.ErrNotFound)

	seedCraftBudgetRun(t, db, 97, "run-mismatch", "actual-task")
	_, err = svc.Admit(ctx, craft.Scope{TenantID: 97, UserID: "owner", SessionID: "client-task"}, "run-mismatch")
	require.ErrorIs(t, err, craft.ErrForbidden)

	grant := CraftBudgetGrantRow{TenantID: 97, RunID: "run-absent-charge", GrantID: "grant-absent-charge",
		Deadline: time.Now().UTC().Add(time.Hour), MaxCalls: 1, Allowed: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	require.NoError(t, db.Create(&grant).Error)
	_, err = svc.AuthorizeBinding(ctx, grant.GrantID, CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrNotFound)
}

func TestCraftChargeFenceAFirstPausePreventsStart(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	if pool, err := db.DB(); err == nil {
		pool.SetMaxOpenConns(4)
	}
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 98, 10000)
	seedCraftBudgetRun(t, db, 98, "run-fence-a", "task-fence-a")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 98, UserID: "owner", SessionID: "task-fence-a"}, "run-fence-a")
	require.NoError(t, err)
	require.NoError(t, svc.PauseRunForBudget(context.Background(), grant.ID))

	starts := 0
	_, err = svc.StartBinding(context.Background(), grant.ID, "activity-a", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)
	require.Zero(t, starts, "A's durable pause is ordered before any B start callback")
	require.Empty(t, craftReservations(t, db, 98))
}

func TestCraftChargeFenceBFirstStartOrdersBeforePause(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(4)
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	workerA, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	workerB, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 99, 10000)
	seedCraftBudgetRun(t, db, 99, "run-fence-b", "task-fence-b")
	grant, err := workerA.Admit(context.Background(), craft.Scope{TenantID: 99, UserID: "owner", SessionID: "task-fence-b"}, "run-fence-b")
	require.NoError(t, err)

	startEntered := make(chan struct{})
	finishStart := make(chan struct{})
	chargeDone := make(chan error, 1)
	starts := 0
	go func() {
		_, err := workerB.StartBinding(context.Background(), grant.ID, "activity-b", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
			func(context.Context) (CraftChargeStartOutcome, error) {
				starts++
				close(startEntered)
				<-finishStart
				return CraftChargeStartStarted, nil
			})
		chargeDone <- err
	}()
	<-startEntered // B is in transport initiation after committing its intent and hold.
	var journal CraftChargeStartJournalRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", 99, "run-fence-b", "activity-b").Take(&journal).Error)
	require.Equal(t, "intent", journal.State, "the callback may run only after intent commit")
	reservations := craftReservations(t, db, 99)
	require.Len(t, reservations, 1)
	require.Equal(t, commercial.ReservationStateDispatched, reservations[0].State)

	pauseDone := make(chan error, 1)
	go func() { pauseDone <- workerA.PauseRunForBudget(context.Background(), grant.ID) }()
	select {
	case err := <-pauseDone:
		require.NoError(t, err, "pause may record after the committed start intent while transport is in flight")
	case <-time.After(5 * time.Second):
		t.Fatal("pause blocked on a callback that must be outside SQL")
	}
	var before struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", 99, "run-fence-b").Take(&before).Error)
	require.Equal(t, "reconciling", before.Status, "the start is unresolved while the callback is still in flight")
	require.Equal(t, "craft_charge_start_pending", before.WaitReason)
	reservations = craftReservations(t, db, 99)
	require.Len(t, reservations, 1, "pause does not release an unresolved start hold")
	require.Equal(t, commercial.ReservationStateDispatched, reservations[0].State)

	close(finishStart)
	require.NoError(t, <-chargeDone)
	var pending struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", 99, "run-fence-b").Take(&pending).Error)
	require.Equal(t, "reconciling", pending.Status, "recording a started outcome does not itself confirm the budget pause")
	runs := repository.NewAgentRunStore(db)
	keys, err := runs.Scan(context.Background(), 20)
	require.NoError(t, err)
	require.Contains(t, keys, agentruntime.RunKey{TenantID: 99, RunID: "run-fence-b"})
	_, err = runs.Claim(context.Background(), agentruntime.RunKey{TenantID: 99, RunID: "run-fence-b"}, "pause-reconciler", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "reconciliation confirms the pause without a new execution claim")
	var after struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", 99, "run-fence-b").Take(&after).Error)
	require.Equal(t, "waiting_user", after.Status)
	require.Equal(t, craftBudgetWaitReason, after.WaitReason)
	require.Equal(t, int64(1), craftGrantCalls(t, db, 99, grant.ID))
	reservations = craftReservations(t, db, 99)
	require.Len(t, reservations, 1)
	require.Equal(t, commercial.ReservationStateDispatched, reservations[0].State)
	_, err = workerB.StartBinding(context.Background(), grant.ID, "activity-b", CraftCallBinding{ModelID: "lead", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)
	require.Equal(t, 1, starts, "same-key retry after pause never starts a second external action")
}

func seedUnresolvedBudgetPause(t *testing.T, db *gorm.DB, svc *CraftBudgetService, grant CraftBudgetGrantRow, state, activity string) string {
	t.Helper()
	ctx := context.Background()
	reservationKey := CraftCallKey("pending/" + activity)
	_, err := svc.budget.Reserve(ctx, commercial.BudgetRequest{
		TenantID: grant.TenantID, RunID: grant.RunID, Key: reservationKey,
		Upper: svc.policy.CallUpper, Deadline: grant.Deadline,
	})
	require.NoError(t, err)
	require.NoError(t, svc.budget.MarkReservationDispatched(ctx, grant.TenantID, reservationKey))
	now := time.Now().UTC()
	require.NoError(t, db.Create(&CraftChargeStartJournalRow{
		TenantID: grant.TenantID, RunID: grant.RunID, ActivityKey: activity,
		GrantID: grant.GrantID, CallID: "call/" + activity, ReservationKey: reservationKey,
		State: state, RunRevision: 0, CreatedAt: now, UpdatedAt: now,
	}).Error)
	return reservationKey
}

func TestCraftBudgetPauseWritersFenceIntentAndUnknown(t *testing.T) {
	paths := []string{"start max calls", "start reservation denial", "pause run", "pause on denial"}
	states := []string{"intent", "unknown"}
	for pathIndex, path := range paths {
		for stateIndex, state := range states {
			t.Run(path+"/"+state, func(t *testing.T) {
				tenant := uint64(140 + pathIndex*2 + stateIndex)
				policy := craftBudgetPolicy()
				funds := int64(10000)
				if path == "start max calls" {
					policy.MaxCalls = 1
				}
				if path == "start reservation denial" {
					funds = int64(policy.CallUpper) + int64(policy.CallUpper)/2
				}
				db := openCraftBudgetTestDB(t)
				svc, err := NewCraftBudgetService(db, nil, policy)
				require.NoError(t, err)
				seedCraftFundedTenant(t, db, tenant, funds)
				runID, sessionID := fmt.Sprintf("run-budget-pause-%d", tenant), fmt.Sprintf("task-budget-pause-%d", tenant)
				seedCraftBudgetRun(t, db, tenant, runID, sessionID)
				grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: tenant, UserID: "owner", SessionID: sessionID}, runID)
				require.NoError(t, err)
				grantRow, err := svc.loadGrant(context.Background(), grant.ID)
				require.NoError(t, err)
				require.NoError(t, db.Table("sessions").Where("tenant_id = ? AND id = ?", tenant, sessionID).
					Update("active_agent_run_id", runID).Error)
				require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", tenant, runID).
					Updates(map[string]any{"lease_owner": "budget-worker", "lease_until": time.Now().UTC().Add(time.Minute)}).Error)
				reservationKey := seedUnresolvedBudgetPause(t, db, svc, grantRow, state, "existing-start")
				if path == "start max calls" {
					require.NoError(t, db.Create(&CraftBudgetCallRow{TenantID: tenant, CallKey: CraftCallKey("prior-call"), GrantID: grant.ID,
						RunID: runID, ModelID: "prior", Funding: commercial.FundingPlatform, CallSeq: 1, CallID: "prior-call", CreatedAt: time.Now().UTC()}).Error)
				}

				var gotErr error
				starts := 0
				switch path {
				case "start max calls", "start reservation denial":
					_, gotErr = svc.StartBinding(context.Background(), grant.ID, "denied-start", CraftCallBinding{ModelID: "next", Funding: commercial.FundingPlatform},
						func(context.Context) (CraftChargeStartOutcome, error) {
							starts++
							return CraftChargeStartStarted, nil
						})
				case "pause run":
					gotErr = svc.PauseRunForBudget(context.Background(), grant.ID)
				case "pause on denial":
					gotErr = svc.pauseOnDenial(context.Background(), grantRow, craft.ErrBudgetDenied)
				}
				if path == "start max calls" {
					require.ErrorIs(t, gotErr, craft.ErrGrantExhausted)
				} else if path == "start reservation denial" || path == "pause on denial" {
					require.ErrorIs(t, gotErr, craft.ErrBudgetDenied)
				} else {
					require.NoError(t, gotErr)
				}
				require.Zero(t, starts, "budget denial cannot invoke the external start callback")
				var run struct{ Status, WaitReason, LeaseOwner string }
				require.NoError(t, db.Table("agent_runs").Select("status, wait_reason, lease_owner").Where("tenant_id = ? AND run_id = ?", tenant, runID).Take(&run).Error)
				require.Equal(t, "reconciling", run.Status, "pause is pending while the external-start journal is unresolved")
				require.Equal(t, "craft_charge_start_pending", run.WaitReason)
				require.Equal(t, "budget-worker", run.LeaseOwner, "an unresolved start retains its Run lease")
				var activeRunID *string
				require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", tenant, sessionID).Scan(&activeRunID).Error)
				require.NotNil(t, activeRunID)
				require.Equal(t, runID, *activeRunID, "pending pause retains its active session slot")
				var pauseEvents int64
				require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", tenant, runID, "craft_charge_pause_requested").Count(&pauseEvents).Error)
				require.EqualValues(t, 1, pauseEvents, "the committed pause request must be durable before returning")
				var journal CraftChargeStartJournalRow
				require.NoError(t, db.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, runID, "existing-start").Take(&journal).Error)
				require.Equal(t, state, journal.State)
				var reservations []repocommercial.ReservationRow
				require.NoError(t, db.Where("tenant_id = ?", tenant).Find(&reservations).Error)
				require.Len(t, reservations, 1, "budget pause must retain the pre-existing G4 hold")
				require.Equal(t, reservationKey, reservations[0].Key)
				require.Equal(t, commercial.ReservationStateDispatched, reservations[0].State)
				runs := repository.NewAgentRunStore(db)
				keys, err := runs.Scan(context.Background(), 20)
				require.NoError(t, err)
				require.NotContains(t, keys, agentruntime.RunKey{TenantID: tenant, RunID: runID})
				_, err = runs.Claim(context.Background(), agentruntime.RunKey{TenantID: tenant, RunID: runID}, "pause-reconciler", time.Minute)
				require.ErrorIs(t, err, agentruntime.ErrLeaseLost)

				require.NoError(t, db.Model(&CraftChargeStartJournalRow{}).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", tenant, runID, "existing-start").Update("state", "started").Error)
				keys, err = runs.Scan(context.Background(), 20)
				require.NoError(t, err)
				require.Contains(t, keys, agentruntime.RunKey{TenantID: tenant, RunID: runID})
				_, err = runs.Claim(context.Background(), agentruntime.RunKey{TenantID: tenant, RunID: runID}, "pause-reconciler", time.Minute)
				require.ErrorIs(t, err, agentruntime.ErrLeaseLost, "reconciliation finalizes the pause without starting another worker")
				require.NoError(t, db.Table("agent_runs").Select("status, wait_reason, lease_owner").Where("tenant_id = ? AND run_id = ?", tenant, runID).Take(&run).Error)
				require.Equal(t, "waiting_user", run.Status)
				require.Equal(t, craftBudgetWaitReason, run.WaitReason)
				require.Empty(t, run.LeaseOwner)
				require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", tenant, sessionID).Scan(&activeRunID).Error)
				require.NotNil(t, activeRunID)
				require.Equal(t, runID, *activeRunID, "confirmed budget wait continues to own its session slot")
				var afterReservations int64
				require.NoError(t, db.Model(&repocommercial.ReservationRow{}).Where("tenant_id = ? AND key = ? AND state = ?", tenant, reservationKey, commercial.ReservationStateDispatched).Count(&afterReservations).Error)
				require.EqualValues(t, 1, afterReservations, "resolving the pause does not release an unresolved provider hold")
			})
		}
	}
}

func TestCraftBudgetPauseDoesNotDowngradePendingCancellation(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 160, 10000)
	seedCraftBudgetRun(t, db, 160, "run-pause-after-cancel", "task-pause-after-cancel")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 160, UserID: "owner", SessionID: "task-pause-after-cancel"}, "run-pause-after-cancel")
	require.NoError(t, err)
	grantRow, err := svc.loadGrant(context.Background(), grant.ID)
	require.NoError(t, err)
	seedUnresolvedBudgetPause(t, db, svc, grantRow, "unknown", "cancel-first")
	runs := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 160, RunID: "run-pause-after-cancel"}
	require.NoError(t, runs.CancelRun(context.Background(), key, "member_stop"))

	err = svc.PauseRunForBudget(context.Background(), grant.ID)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "later budget pause cannot replace pending cancellation")
	var run struct{ Status, WaitReason string }
	require.NoError(t, db.Table("agent_runs").Select("status, wait_reason").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error)
	require.Equal(t, "reconciling", run.Status)
	require.Equal(t, "craft_charge_start_pending", run.WaitReason)
	var cancels, pauses int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", key.TenantID, key.RunID, "craft_charge_cancel_requested").Count(&cancels).Error)
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", key.TenantID, key.RunID, "craft_charge_pause_requested").Count(&pauses).Error)
	require.EqualValues(t, 1, cancels)
	require.Zero(t, pauses)
}

func TestCraftBudgetStartDenialReturnsRepositoryFailureInsteadOfDenial(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.MaxCalls = 1
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 161, 10000)
	seedCraftBudgetRun(t, db, 161, "run-pause-event-failure", "task-pause-event-failure")
	grant, err := svc.Admit(context.Background(), craft.Scope{TenantID: 161, UserID: "owner", SessionID: "task-pause-event-failure"}, "run-pause-event-failure")
	require.NoError(t, err)
	grantRow, err := svc.loadGrant(context.Background(), grant.ID)
	require.NoError(t, err)
	reservationKey := seedUnresolvedBudgetPause(t, db, svc, grantRow, "unknown", "existing-start")
	require.NoError(t, db.Create(&CraftBudgetCallRow{TenantID: 161, CallKey: CraftCallKey("prior-call"), GrantID: grant.ID,
		RunID: grantRow.RunID, ModelID: "prior", Funding: commercial.FundingPlatform, CallSeq: 1, CallID: "prior-call", CreatedAt: time.Now().UTC()}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_service_budget_pause BEFORE INSERT ON agent_run_events
		WHEN NEW.event_type = 'craft_charge_pause_requested'
		BEGIN SELECT RAISE(ABORT, 'reject service pause event'); END`).Error)
	starts := 0
	_, err = svc.StartBinding(context.Background(), grant.ID, "denied-start", CraftCallBinding{ModelID: "next", Funding: commercial.FundingPlatform},
		func(context.Context) (CraftChargeStartOutcome, error) {
			starts++
			return CraftChargeStartStarted, nil
		})
	require.Error(t, err)
	require.NotErrorIs(t, err, craft.ErrGrantExhausted, "a failed durable pause must not be presented as a committed budget denial")
	require.Zero(t, starts)
	var run struct{ Status string }
	require.NoError(t, db.Table("agent_runs").Select("status").Where("tenant_id = ? AND run_id = ?", 161, grantRow.RunID).Take(&run).Error)
	require.Equal(t, "running", run.Status, "failed event append rolls back the status transition")
	var pauses int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", 161, grantRow.RunID, "craft_charge_pause_requested").Count(&pauses).Error)
	require.Zero(t, pauses)
	var hold repocommercial.ReservationRow
	require.NoError(t, db.Where("tenant_id = ? AND key = ?", 161, reservationKey).Take(&hold).Error)
	require.Equal(t, commercial.ReservationStateDispatched, hold.State)
}
