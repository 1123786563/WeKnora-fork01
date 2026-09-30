package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	require.Equal(t, craft.BudgetPause{RunID: "run-t19", Reason: "exhausted", Limit: 1, Used: 1}, pause)
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
	require.ErrorIs(t, reloaded.ExtendAndResume(ctx, craft.Scope{TenantID: 91, UserID: "member", SessionID: "craft107-t19"}, "run-t19", "ext-t19", 1, commercial.Credits(500)), craft.ErrForbidden)
	require.ErrorIs(t, reloaded.ExtendAndResume(ctx, scope, "run-t19", "ext-t19", 1, commercial.Credits(500)), craft.ErrReconcilePending)
	pause, err = reloaded.BudgetPause(ctx, scope, "run-t19")
	require.NoError(t, err)
	require.Equal(t, "exhausted", pause.Reason)
	require.NoError(t, db.Model(&repocommercial.ReservationRow{}).Where("tenant_id = ? AND run_id = ?", 91, "run-t19").Update("state", commercial.ReservationStateSettled).Error)
	require.NoError(t, reloaded.ExtendAndResume(ctx, scope, "run-t19", "ext-t19", 1, commercial.Credits(500)))
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
	require.Equal(t, "waiting_user", before.Status)
	require.Equal(t, craftBudgetWaitReason, before.WaitReason)
	reservations = craftReservations(t, db, 99)
	require.Len(t, reservations, 1, "pause does not release an unresolved start hold")
	require.Equal(t, commercial.ReservationStateDispatched, reservations[0].State)

	close(finishStart)
	require.NoError(t, <-chargeDone)
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
