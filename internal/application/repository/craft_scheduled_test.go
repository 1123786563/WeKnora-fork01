package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openCraftScheduledDB applies the real SQLite migrations (000099_craft_
// scheduled included); a subtest named "postgres" runs the same assertions
// against an isolated PostgreSQL schema when TRPC_TEST_POSTGRES_DSN is set.
func openCraftScheduledDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openRunTestDB(t)
}

// scheduledTask builds one valid active recipe whose next fire is already
// due (a minute in the past) unless the caller overrides NextRunAt.
func scheduledTask(tenant uint64, owner, cron string) types.CraftScheduledTask {
	due := time.Now().UTC().Add(-time.Minute)
	return types.CraftScheduledTask{
		TenantID:       tenant,
		OwnerID:        owner,
		Name:           "nightly brief",
		Prompt:         "summarize today",
		CronExpression: cron,
		EditorMode:     types.CraftScheduledEditorModeAdvanced,
		Status:         types.CraftScheduledTaskStatusActive,
		NextRunAt:      &due,
	}
}

// TestCraftScheduledTaskCRUDMatrix walks the full owner-scoped CRUD matrix:
// create/read round-trip, newest-first owner listing, update of the mutable
// fields, wrong-owner/tenant 404 sentinels, invalid-cron rejection and the
// idempotent soft delete that hides the row from every owner-scoped read.
func TestCraftScheduledTaskCRUDMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftScheduledDB(t)
			repo := NewCraftScheduledTaskRepository(db)
			ctx := context.Background()

			older := scheduledTask(1, "owner-a", "*/5 * * * *")
			older.ID = "sched-old"
			older.CreatedAt = time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
			newer := scheduledTask(1, "owner-a", "0 9 * * *")
			newer.ID = "sched-new"
			newer.CreatedAt = older.CreatedAt.Add(time.Hour)
			require.NoError(t, repo.Create(ctx, &older))
			require.NoError(t, repo.Create(ctx, &newer))

			// Read: owner-scoped get returns the stored row verbatim.
			got, err := repo.GetByID(ctx, 1, "owner-a", "sched-new")
			require.NoError(t, err)
			require.Equal(t, "nightly brief", got.Name)
			require.Equal(t, "0 9 * * *", got.CronExpression)
			require.NotNil(t, got.NextRunAt, "active task keeps a due ticket")

			// Owner scoping: another owner's or tenant's task does not
			// exist for the caller — the 404 sentinel, no leakage.
			_, err = repo.GetByID(ctx, 1, "owner-b", "sched-new")
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = repo.GetByID(ctx, 2, "owner-a", "sched-new")
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = repo.GetByID(ctx, 1, "owner-a", "sched-missing")
			require.ErrorIs(t, err, craft.ErrNotFound)

			// List: newest-first, only the owner's live rows.
			other := scheduledTask(1, "owner-b", "*/10 * * * *")
			other.ID = "sched-other"
			other.CreatedAt = older.CreatedAt.Add(2 * time.Hour)
			require.NoError(t, repo.Create(ctx, &other))
			list, err := repo.ListByOwner(ctx, 1, "owner-a")
			require.NoError(t, err)
			require.Len(t, list, 2)
			require.Equal(t, "sched-new", list[0].ID, "newest first")
			require.Equal(t, "sched-old", list[1].ID)

			// Update: mutable fields change, scoped to owner; a wrong owner
			// update is a 404 and touches nothing.
			pausedNext := (*time.Time)(nil)
			updated := newer
			updated.Name = "morning brief"
			updated.Prompt = "summarize the night"
			updated.Status = types.CraftScheduledTaskStatusPaused
			updated.NextRunAt = pausedNext
			require.NoError(t, repo.Update(ctx, &updated))
			reread, err := repo.GetByID(ctx, 1, "owner-a", "sched-new")
			require.NoError(t, err)
			require.Equal(t, "morning brief", reread.Name)
			require.Equal(t, "summarize the night", reread.Prompt)
			require.Equal(t, types.CraftScheduledTaskStatusPaused, reread.Status)
			require.Nil(t, reread.NextRunAt, "paused task parks its ticket")

			wrongOwner := updated
			wrongOwner.OwnerID = "owner-b"
			require.ErrorIs(t, repo.Update(ctx, &wrongOwner), craft.ErrNotFound)

			// Cron validation guards both write entrances — including the
			// never-firing class (Feb 30 parses but holds no occurrence, which
			// would store a permanently-due year-0001 ticket).
			bad := scheduledTask(1, "owner-a", "not a cron")
			require.Error(t, repo.Create(ctx, &bad), "invalid cron must not be stored")
			feb30 := scheduledTask(1, "owner-a", "0 0 30 2 *")
			require.Error(t, repo.Create(ctx, &feb30), "a never-firing schedule must not be stored")
			worse := *reread
			worse.CronExpression = "* * *"
			require.Error(t, repo.Update(ctx, &worse), "invalid cron must not be stored")
			worse.CronExpression = "0 0 31 4 *"
			require.Error(t, repo.Update(ctx, &worse), "a never-firing schedule must not be stored")

			// Soft delete: idempotent, hides the row from get and list.
			require.NoError(t, repo.SoftDelete(ctx, 1, "owner-a", "sched-new"))
			require.NoError(t, repo.SoftDelete(ctx, 1, "owner-a", "sched-new"),
				"second delete of the same task is a no-op success")
			_, err = repo.GetByID(ctx, 1, "owner-a", "sched-new")
			require.ErrorIs(t, err, craft.ErrNotFound)
			list, err = repo.ListByOwner(ctx, 1, "owner-a")
			require.NoError(t, err)
			require.Len(t, list, 1)
			require.Equal(t, "sched-old", list[0].ID)
			// Deleting another owner's task is also a silent no-op — the
			// desired end state already holds, nothing is leaked.
			require.NoError(t, repo.SoftDelete(ctx, 1, "owner-b", "sched-old"))
			_, err = repo.GetByID(ctx, 1, "owner-a", "sched-old")
			require.NoError(t, err)
		})
	}
}

// TestCraftScheduledClaimDueTasksCASSingleWinner is the Ruling P-1 contract:
// two concurrent sweeps that both read the same due row, only the one whose
// compare-and-swap UPDATE still finds the OLD next_run_at wins; the loser
// observes nothing and moves on. Exactly one fire leaves the claim.
func TestCraftScheduledClaimDueTasksCASSingleWinner(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftScheduledDB(t)
			repo := NewCraftScheduledTaskRepository(db)
			ctx := context.Background()

			due := time.Now().UTC().Add(-2 * time.Minute)
			task := types.CraftScheduledTask{
				TenantID: 1, OwnerID: "owner-a", Name: "every five",
				Prompt: "tick", CronExpression: "*/5 * * * *",
				EditorMode: types.CraftScheduledEditorModeAdvanced,
				Status:     types.CraftScheduledTaskStatusActive,
				NextRunAt:  &due,
			}
			require.NoError(t, repo.Create(ctx, &task))

			now := time.Now().UTC().Truncate(time.Second)
			var mu sync.Mutex
			total := 0
			var won ClaimedTask
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					claims, err := repo.ClaimDueTasks(ctx, now, 10)
					if err != nil {
						return
					}
					mu.Lock()
					defer mu.Unlock()
					total += len(claims)
					for _, c := range claims {
						won = c
					}
				}()
			}
			wg.Wait()

			require.Equal(t, 1, total, "exactly one of the racing sweeps claims the fire")
			require.Equal(t, task.ID, won.Task.ID)
			require.True(t, won.PrevNextRunAt.Equal(due), "the claim reports the old ticket")
			require.True(t, won.Task.NextRunAt.After(now),
				"the stored ticket advances past the sweep time")

			// The durable row: next_run_at advanced, last_run_at pinned to
			// the fired due time, and a re-sweep at the same instant is a
			// no-op — the ticket moved.
			row, err := repo.GetByID(ctx, 1, "owner-a", task.ID)
			require.NoError(t, err)
			require.True(t, row.NextRunAt.After(now))
			require.NotNil(t, row.LastRunAt)
			require.True(t, row.LastRunAt.Equal(due), "the fire time lands in last_run_at")
			again, err := repo.ClaimDueTasks(ctx, now, 10)
			require.NoError(t, err)
			require.Empty(t, again, "an advanced ticket is not due again")
		})
	}
}

// TestCraftScheduledClaimDueTasksScoping proves the due scan only picks
// rows the dispatcher may fire: live, active, ticket due, oldest first and
// capped by the batch.
func TestCraftScheduledClaimDueTasksScoping(t *testing.T) {
	db := openCraftScheduledDB(t)
	repo := NewCraftScheduledTaskRepository(db)
	ctx := context.Background()

	mk := func(id string, age time.Duration) types.CraftScheduledTask {
		t := scheduledTask(1, "owner-a", "*/5 * * * *")
		t.ID = id
		at := time.Now().UTC().Add(-age)
		t.NextRunAt = &at
		t.CreatedAt = at
		return t
	}
	dueOld := mk("due-old", 3*time.Hour)
	dueNew := mk("due-new", time.Hour)
	require.NoError(t, repo.Create(ctx, &dueOld))
	require.NoError(t, repo.Create(ctx, &dueNew))

	paused := mk("paused", 2*time.Hour)
	paused.Status = types.CraftScheduledTaskStatusPaused
	require.NoError(t, repo.Create(ctx, &paused))

	deleted := mk("deleted", 30*time.Minute)
	require.NoError(t, repo.Create(ctx, &deleted))
	require.NoError(t, repo.SoftDelete(ctx, 1, "owner-a", "deleted"))

	future := mk("future", -time.Hour)
	require.NoError(t, repo.Create(ctx, &future))

	parked := mk("parked", 2*time.Hour)
	parked.NextRunAt = nil
	require.NoError(t, repo.Create(ctx, &parked))

	now := time.Now().UTC().Truncate(time.Second)
	claims, err := repo.ClaimDueTasks(ctx, now, 1)
	require.NoError(t, err)
	require.Len(t, claims, 1, "batch caps the sweep")
	require.Equal(t, "due-old", claims[0].Task.ID, "oldest due ticket first")

	claims, err = repo.ClaimDueTasks(ctx, now, 50)
	require.NoError(t, err)
	require.Len(t, claims, 1, "only due-old and due-new were claimable")
	require.Equal(t, "due-new", claims[0].Task.ID)
}

// TestCraftScheduledRunsKeysetPagination pages the run history by
// started_at descending with the strict-before cursor and a clamped limit.
func TestCraftScheduledRunsKeysetPagination(t *testing.T) {
	db := openCraftScheduledDB(t)
	repo := NewCraftScheduledTaskRepository(db)
	ctx := context.Background()

	task := scheduledTask(1, "owner-a", "0 9 * * *")
	require.NoError(t, repo.Create(ctx, &task))

	base := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	ids := []string{"r1", "r2", "r3", "r4", "r5"}
	for i, id := range ids {
		at := base.Add(time.Duration(i) * time.Minute)
		run := types.CraftScheduledTaskRun{
			ID: id, TaskID: task.ID, Status: types.CraftScheduledRunStatusSucceeded,
			TriggerSource: types.CraftScheduledTriggerScheduled, StartedAt: &at,
		}
		require.NoError(t, repo.InsertRun(ctx, &run))
	}

	page, err := repo.ListRunsByTask(ctx, task.ID, nil, 2)
	require.NoError(t, err)
	require.Equal(t, []string{"r5", "r4"}, scheduledRunIDs(page))

	cursor := page[len(page)-1].StartedAt
	page, err = repo.ListRunsByTask(ctx, task.ID, cursor, 2)
	require.NoError(t, err)
	require.Equal(t, []string{"r3", "r2"}, scheduledRunIDs(page))

	cursor = page[len(page)-1].StartedAt
	page, err = repo.ListRunsByTask(ctx, task.ID, cursor, 100)
	require.NoError(t, err)
	require.Equal(t, []string{"r1"}, scheduledRunIDs(page), "the last page drains the tail")

	cursor = page[0].StartedAt
	page, err = repo.ListRunsByTask(ctx, task.ID, cursor, 100)
	require.NoError(t, err)
	require.Empty(t, page, "before the oldest run there is nothing")

	page, err = repo.ListRunsByTask(ctx, task.ID, nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 5, "a non-positive limit falls back to the default")
}

func scheduledRunIDs(runs []types.CraftScheduledTaskRun) []string {
	ids := make([]string, 0, len(runs))
	for _, r := range runs {
		ids = append(ids, r.ID)
	}
	return ids
}

// TestCraftScheduledRunLifecycleAndInFlight covers the run writer pair and
// the SKIP_IF_RUNNING probe: queued/running are in flight; every terminal
// state — including skipped — is not; a terminal write lands status,
// summary, error fields and finished_at; an empty string clears to NULL.
func TestCraftScheduledRunLifecycleAndInFlight(t *testing.T) {
	db := openCraftScheduledDB(t)
	repo := NewCraftScheduledTaskRepository(db)
	ctx := context.Background()

	task := scheduledTask(1, "owner-a", "0 9 * * *")
	require.NoError(t, repo.Create(ctx, &task))

	run := types.CraftScheduledTaskRun{
		TaskID: task.ID, Status: types.CraftScheduledRunStatusQueued,
		TriggerSource: types.CraftScheduledTriggerScheduled,
	}
	require.NoError(t, repo.InsertRun(ctx, &run))
	require.NotEmpty(t, run.ID, "insert mints the row key")
	require.NotNil(t, run.StartedAt, "insert stamps the fire time")

	inFlight, err := repo.HasInFlightRun(ctx, task.ID)
	require.NoError(t, err)
	require.True(t, inFlight, "queued is in flight")

	require.NoError(t, repo.UpdateRunStatus(ctx, run.ID,
		types.CraftScheduledRunStatusRunning, "", "", "", nil))
	inFlight, err = repo.HasInFlightRun(ctx, task.ID)
	require.NoError(t, err)
	require.True(t, inFlight, "running is in flight")

	fin := time.Now().UTC()
	require.NoError(t, repo.UpdateRunStatus(ctx, run.ID,
		types.CraftScheduledRunStatusSucceeded, "", "", "all good", &fin))
	inFlight, err = repo.HasInFlightRun(ctx, task.ID)
	require.NoError(t, err)
	require.False(t, inFlight, "succeeded is terminal")

	var stored types.CraftScheduledTaskRun
	require.NoError(t, db.Where("id = ?", run.ID).First(&stored).Error)
	require.Equal(t, types.CraftScheduledRunStatusSucceeded, stored.Status)
	require.NotNil(t, stored.Summary)
	require.Equal(t, "all good", *stored.Summary)
	require.Nil(t, stored.ErrorClass, "empty error class clears to NULL")
	require.NotNil(t, stored.FinishedAt)
	require.True(t, stored.FinishedAt.Equal(fin))

	// A failed sibling and a skipped sibling on another task.
	fail := types.CraftScheduledTaskRun{
		TaskID: task.ID, Status: types.CraftScheduledRunStatusFailed,
		TriggerSource: types.CraftScheduledTriggerManualRunNow,
	}
	require.NoError(t, repo.InsertRun(ctx, &fail))
	require.NoError(t, repo.UpdateRunStatus(ctx, fail.ID,
		types.CraftScheduledRunStatusFailed, "model_error", "boom", "", &fin))
	// A fresh destination: gorm's First folds a pre-populated primary key
	// into the WHERE clause, so a reused struct would query the wrong id.
	var storedFail types.CraftScheduledTaskRun
	require.NoError(t, db.Where("id = ?", fail.ID).First(&storedFail).Error)
	stored = storedFail
	require.NotNil(t, stored.ErrorClass)
	require.Equal(t, "model_error", *stored.ErrorClass)
	require.NotNil(t, stored.ErrorDetail)
	require.Equal(t, "boom", *stored.ErrorDetail)

	skip := types.CraftScheduledTaskRun{
		TaskID: task.ID, Status: types.CraftScheduledRunStatusSkipped,
		TriggerSource: types.CraftScheduledTriggerScheduled,
		SkipReason:    strPtr(types.CraftScheduledSkipPriorInFlight),
	}
	require.NoError(t, repo.InsertRun(ctx, &skip))
	inFlight, err = repo.HasInFlightRun(ctx, task.ID)
	require.NoError(t, err)
	require.False(t, inFlight, "failed and skipped are not in flight")

	require.ErrorIs(t, repo.UpdateRunStatus(ctx, "no-such-run",
		types.CraftScheduledRunStatusFailed, "", "", "", nil), craft.ErrNotFound)

	// Run rows of a soft-deleted task stay readable (run history follows the
	// recipe, the spec's retention shape).
	require.NoError(t, repo.SoftDelete(ctx, 1, "owner-a", task.ID))
	history, err := repo.ListRunsByTask(ctx, task.ID, nil, 50)
	require.NoError(t, err)
	require.Len(t, history, 3)
}

// TestCraftScheduledClaimDueTasksPoisonRowQuarantined is the Task-1 review
// Important-1 contract: a poison row — a stored cron that no longer parses —
// is quarantined and skipped, never a sweep abort. Every OTHER due fire in
// the same batch is still claimed; the quarantined row keeps its due ticket
// untouched for repair or deletion.
func TestCraftScheduledClaimDueTasksPoisonRowQuarantined(t *testing.T) {
	db := openCraftScheduledDB(t)
	repo := NewCraftScheduledTaskRepository(db)
	ctx := context.Background()

	// The poison row: oldest due ticket in the batch (so the sweep meets it
	// first) with a cron that bypassed validation via a raw insert — only
	// corrupted state can hold one.
	poisonDue := time.Now().UTC().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO craft_scheduled_tasks
		   (id, tenant_id, owner_id, name, prompt, cron_expression, editor_mode, status, next_run_at)
		 VALUES ('sched-poison', 1, 'owner-a', 'poisoned', 'tick', 'not a cron', 'advanced', 'active', ?)`,
		poisonDue,
	).Error)
	// The never-fires sibling: parses, but no occurrence exists — the
	// zero-time guard in NextCronFire routes it into the same quarantine
	// instead of minting a year-0001 ticket the CAS would rewrite forever.
	require.NoError(t, db.Exec(
		`INSERT INTO craft_scheduled_tasks
		   (id, tenant_id, owner_id, name, prompt, cron_expression, editor_mode, status, next_run_at)
		 VALUES ('sched-never', 1, 'owner-a', 'never', 'tick', '0 0 30 2 *', 'advanced', 'active', ?)`,
		poisonDue,
	).Error)

	good := scheduledTask(1, "owner-a", "*/5 * * * *")
	good.ID = "sched-good"
	require.NoError(t, repo.Create(ctx, &good))

	now := time.Now().UTC().Truncate(time.Second)
	claims, err := repo.ClaimDueTasks(ctx, now, 50)
	require.NoError(t, err, "a poison row must not abort the sweep")
	require.Len(t, claims, 1, "the healthy row is still claimed")
	require.Equal(t, "sched-good", claims[0].Task.ID)

	// The quarantined rows keep their due tickets exactly: they stay visibly
	// due (re-examined by every sweep) instead of being advanced or deleted.
	var poison types.CraftScheduledTask
	require.NoError(t, db.Where("id = ?", "sched-poison").First(&poison).Error)
	require.NotNil(t, poison.NextRunAt)
	require.True(t, poison.NextRunAt.Equal(poisonDue), "the poison row's ticket is untouched")
	var never types.CraftScheduledTask
	require.NoError(t, db.Where("id = ?", "sched-never").First(&never).Error)
	require.NotNil(t, never.NextRunAt)
	require.True(t, never.NextRunAt.Equal(poisonDue), "the never-firing row's ticket is untouched")
}

// TestCraftScheduledClaimDueTasksPartialClaimsOnError is the other half of
// the Important-1 contract: when a row's claim UPDATE hits a database error
// mid-sweep, the sweep returns the claims it already won TOGETHER with the
// error — won tickets are never dropped, so the dispatcher can still run the
// fires it owns instead of losing them tracelessly.
func TestCraftScheduledClaimDueTasksPartialClaimsOnError(t *testing.T) {
	db := openCraftScheduledDB(t)
	repo := NewCraftScheduledTaskRepository(db)
	ctx := context.Background()

	older := time.Now().UTC().Add(-2 * time.Hour)
	newer := older.Add(time.Hour)
	dueOld := scheduledTask(1, "owner-a", "*/5 * * * *")
	dueOld.ID = "due-old"
	dueOld.NextRunAt = &older
	require.NoError(t, repo.Create(ctx, &dueOld))
	dueNew := scheduledTask(1, "owner-a", "*/10 * * * *")
	dueNew.ID = "due-new"
	dueNew.NextRunAt = &newer
	require.NoError(t, repo.Create(ctx, &dueNew))

	// Fail the SECOND craft_scheduled_tasks UPDATE (the CAS of due-new, the
	// newer ticket): due-old's claim commits first, then the sweep hits the
	// injected error.
	var casCount int32
	const failSecond = "test:fail_second_cas"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(failSecond, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Model.(*types.CraftScheduledTask); !ok {
			return
		}
		if atomic.AddInt32(&casCount, 1) == 2 {
			_ = tx.AddError(errors.New("injected cas failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(failSecond) })

	now := time.Now().UTC().Truncate(time.Second)
	claims, err := repo.ClaimDueTasks(ctx, now, 50)
	require.Error(t, err, "the sweep reports the row's failure")
	require.Len(t, claims, 1, "the already-won claim survives the error")
	require.Equal(t, "due-old", claims[0].Task.ID)
	require.True(t, claims[0].PrevNextRunAt.Equal(*dueOld.NextRunAt))

	// due-old's CAS committed: its ticket advanced past the sweep time;
	// due-new's did not — still carrying the ticket the sweep read.
	oldRow, err := repo.GetByID(ctx, 1, "owner-a", "due-old")
	require.NoError(t, err)
	require.True(t, oldRow.NextRunAt.After(now), "the won claim's ticket advanced")
	newRow, err := repo.GetByID(ctx, 1, "owner-a", "due-new")
	require.NoError(t, err)
	require.True(t, newRow.NextRunAt.Equal(newer), "the failed row keeps its ticket")
}

func strPtr(s string) *string { return &s }
