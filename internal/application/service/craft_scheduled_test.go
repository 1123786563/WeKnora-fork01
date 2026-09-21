package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openCraftScheduledServiceDB applies the real SQLite migrations (000099_
// craft_scheduled included) into an isolated temp database, following
// openCraftUsageServiceDB.
func openCraftScheduledServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-scheduled.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// newCraftScheduledFixture wires the real repository behind the service with
// a pinned clock (Monday 2026-09-21 12:00 UTC), so ticket computation and
// the fires preview are asserted against exact times instead of sleeping.
func newCraftScheduledFixture(t *testing.T) (*CraftScheduledService, repository.CraftScheduledTaskRepository) {
	t.Helper()
	db := openCraftScheduledServiceDB(t)
	repo := repository.NewCraftScheduledTaskRepository(db)
	svc, err := NewCraftScheduledService(repo)
	require.NoError(t, err)
	pinned := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	svc.withClock(func() time.Time { return pinned })
	return svc, repo
}

// TestCompileCronSpecMatrix walks the three editor modes' compile contract
// plus every malformed shape: each must answer an error wrapping
// craft.ErrInvalidInput (the API's 400 semantics).
func TestCompileCronSpecMatrix(t *testing.T) {
	valid := []struct {
		name    string
		mode    string
		payload string
		want    string
	}{
		{"interval 30", types.CraftScheduledEditorModeInterval, `{"every_minutes":30}`, "*/30 * * * *"},
		{"interval 1", types.CraftScheduledEditorModeInterval, `{"every_minutes":1}`, "*/1 * * * *"},
		{"interval 45", types.CraftScheduledEditorModeInterval, `{"every_minutes":45}`, "*/45 * * * *"},
		{"interval 90", types.CraftScheduledEditorModeInterval, `{"every_minutes":90}`, "*/90 * * * *"},
		{"daily 09:30", types.CraftScheduledEditorModeDaily, `{"at":"09:30"}`, "30 9 * * *"},
		{"daily 9:30", types.CraftScheduledEditorModeDaily, `{"at":"9:30"}`, "30 9 * * *"},
		{"daily 00:00", types.CraftScheduledEditorModeDaily, `{"at":"00:00"}`, "0 0 * * *"},
		{"daily 23:59", types.CraftScheduledEditorModeDaily, `{"at":"23:59"}`, "59 23 * * *"},
		{"advanced passthrough", types.CraftScheduledEditorModeAdvanced, `{"cron":"*/5 * * * *"}`, "*/5 * * * *"},
		{"advanced trimmed", types.CraftScheduledEditorModeAdvanced, `{"cron":"  30 9 * * *  "}`, "30 9 * * *"},
		{"advanced dom", types.CraftScheduledEditorModeAdvanced, `{"cron":"30 9 30 * *"}`, "30 9 30 * *"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CompileCronSpec(tc.mode, json.RawMessage(tc.payload))
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}

	invalid := []struct {
		name    string
		mode    string
		payload string
	}{
		{"interval zero", types.CraftScheduledEditorModeInterval, `{"every_minutes":0}`},
		{"interval negative", types.CraftScheduledEditorModeInterval, `{"every_minutes":-5}`},
		{"interval missing field", types.CraftScheduledEditorModeInterval, `{}`},
		{"interval wrong field", types.CraftScheduledEditorModeInterval, `{"minutes":30}`},
		{"interval float", types.CraftScheduledEditorModeInterval, `{"every_minutes":1.5}`},
		{"interval string", types.CraftScheduledEditorModeInterval, `{"every_minutes":"30"}`},
		{"interval malformed json", types.CraftScheduledEditorModeInterval, `{"every_minutes":`},
		{"interval null payload", types.CraftScheduledEditorModeInterval, ``},
		{"daily hour 24", types.CraftScheduledEditorModeDaily, `{"at":"24:00"}`},
		{"daily minute 60", types.CraftScheduledEditorModeDaily, `{"at":"09:60"}`},
		{"daily minute -1", types.CraftScheduledEditorModeDaily, `{"at":"09:-1"}`},
		{"daily no colon", types.CraftScheduledEditorModeDaily, `{"at":"0930"}`},
		{"daily seconds", types.CraftScheduledEditorModeDaily, `{"at":"09:30:00"}`},
		{"daily empty", types.CraftScheduledEditorModeDaily, `{"at":""}`},
		{"daily non numeric", types.CraftScheduledEditorModeDaily, `{"at":"ab:30"}`},
		{"daily missing field", types.CraftScheduledEditorModeDaily, `{}`},
		{"daily malformed json", types.CraftScheduledEditorModeDaily, `{"at":09:30}`},
		{"advanced empty", types.CraftScheduledEditorModeAdvanced, `{"cron":""}`},
		{"advanced missing field", types.CraftScheduledEditorModeAdvanced, `{}`},
		{"advanced garbage", types.CraftScheduledEditorModeAdvanced, `{"cron":"not a cron"}`},
		{"advanced six fields", types.CraftScheduledEditorModeAdvanced, `{"cron":"0 9 * * * 1"}`},
		{"advanced descriptor", types.CraftScheduledEditorModeAdvanced, `{"cron":"@every 30s"}`},
		{"advanced tz prefix", types.CraftScheduledEditorModeAdvanced, `{"cron":"TZ=Asia/Tokyo 0 9 * * *"}`},
		{"unknown mode empty", "", `{"every_minutes":30}`},
		{"unknown mode weekly", "weekly", `{"every_minutes":30}`},
		{"unknown mode cron", "cron", `{"cron":"*/5 * * * *"}`},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CompileCronSpec(tc.mode, json.RawMessage(tc.payload))
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			require.Empty(t, got)
		})
	}
}

// TestNextRunsAfter pins the preview iterator: strict-after semantics at an
// exact fire point, hour rollover, month-end and year-end crossings, and the
// degradation shapes (n <= 0, unparseable expression).
func TestNextRunsAfter(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 9, 21, h, m, 0, 0, time.UTC) }

	fires := NextRunsAfter("*/15 * * * *", at(10, 7), 3)
	require.Equal(t, []time.Time{at(10, 15), at(10, 30), at(10, 45)}, fires)

	// Strictly after: standing exactly on a fire point yields the NEXT one.
	fires = NextRunsAfter("*/15 * * * *", at(10, 15), 2)
	require.Equal(t, []time.Time{at(10, 30), at(10, 45)}, fires)

	// Hour rollover.
	fires = NextRunsAfter("*/15 * * * *", at(10, 50), 2)
	require.Equal(t, []time.Time{at(11, 0), at(11, 15)}, fires)

	// Day and month-end rollover: after Jan 31's 09:30 fire, daily-at-09:30
	// crosses into February.
	jan := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	fires = NextRunsAfter("30 9 * * *", jan, 3)
	require.Equal(t, []time.Time{
		time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 2, 2, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 2, 3, 9, 30, 0, 0, time.UTC),
	}, fires)

	// February owns no day 30: the month is skipped whole.
	fires = NextRunsAfter("30 9 30 * *", jan, 2)
	require.Equal(t, []time.Time{
		time.Date(2026, 3, 30, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 30, 9, 30, 0, 0, time.UTC),
	}, fires)

	// Year rollover.
	fires = NextRunsAfter("0 9 * * *", time.Date(2026, 12, 31, 10, 0, 0, 0, time.UTC), 2)
	require.Equal(t, []time.Time{
		time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC),
	}, fires)

	require.Empty(t, NextRunsAfter("*/5 * * * *", at(10, 0), 0), "n <= 0 answers nothing")
	require.Nil(t, NextRunsAfter("not a cron", at(10, 0), 3), "unparseable expressions preview nothing")
}

// TestCraftScheduledServiceCreateSemantics walks the POST entrance: the three
// editor modes compile into the stored cron, the first claim ticket lands on
// the next fire after the pinned now, run_immediately appends a manual
// queued run without touching the ticket, a paused task parks the ticket
// NULL, and every malformed shape answers 400 semantics storing nothing.
func TestCraftScheduledServiceCreateSemantics(t *testing.T) {
	svc, repo := newCraftScheduledFixture(t)
	ctx := context.Background()

	// Interval: every 30 minutes from Monday 12:00 → first ticket 12:30.
	interval, runID, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "  half-hour tick  ", Prompt: " brief me ",
		EditorMode: types.CraftScheduledEditorModeInterval,
		Payload:    json.RawMessage(`{"every_minutes":30}`),
	})
	require.NoError(t, err)
	require.Empty(t, runID, "no run_immediately, no run row")
	require.NotEmpty(t, interval.ID)
	require.Equal(t, "*/30 * * * *", interval.CronExpression)
	require.Equal(t, types.CraftScheduledEditorModeInterval, interval.EditorMode)
	require.Equal(t, types.CraftScheduledTaskStatusActive, interval.Status, "status defaults to active")
	require.Equal(t, "half-hour tick", interval.Name, "name is stored trimmed")
	require.Equal(t, "brief me", interval.Prompt, "prompt is stored trimmed")
	require.NotNil(t, interval.NextRunAt)
	require.True(t, interval.NextRunAt.Equal(time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC)))

	// Daily with run_immediately: ticket is tomorrow's 09:30 (Monday noon
	// already passed 09:30); one manual queued run exists for it.
	daily, runID, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "morning brief", Prompt: "summarize",
		EditorMode:     types.CraftScheduledEditorModeDaily,
		Payload:        json.RawMessage(`{"at":"09:30"}`),
		RunImmediately: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, runID)
	require.True(t, daily.NextRunAt.Equal(time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC)))
	runs, err := repo.ListRunsByTask(ctx, daily.ID, nil, 50)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, types.CraftScheduledRunStatusQueued, runs[0].Status)
	require.Equal(t, types.CraftScheduledTriggerManualRunNow, runs[0].TriggerSource)
	require.Equal(t, runID, runs[0].ID)

	// A paused creation parks the ticket NULL and is never dispatched.
	paused, _, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "parked", Prompt: "hold",
		EditorMode: types.CraftScheduledEditorModeAdvanced,
		Payload:    json.RawMessage(`{"cron":"*/5 * * * *"}`),
		Status:     types.CraftScheduledTaskStatusPaused,
	})
	require.NoError(t, err)
	require.Equal(t, types.CraftScheduledTaskStatusPaused, paused.Status)
	require.Nil(t, paused.NextRunAt)

	// Every malformed shape: 400 semantics, nothing stored.
	bad := []CraftScheduledTaskCreate{
		{Name: "", Prompt: "p", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"*/5 * * * *"}`)},
		{Name: strings.Repeat("x", 129), Prompt: "p", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"*/5 * * * *"}`)},
		{Name: "n", Prompt: "  ", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"*/5 * * * *"}`)},
		{Name: "n", Prompt: "p", EditorMode: "weekly",
			Payload: json.RawMessage(`{"every_minutes":30}`)},
		{Name: "n", Prompt: "p", EditorMode: types.CraftScheduledEditorModeInterval,
			Payload: json.RawMessage(`{"every_minutes":0}`)},
		{Name: "n", Prompt: "p", EditorMode: types.CraftScheduledEditorModeInterval,
			Payload: json.RawMessage(`{"every_minutes":`)},
		{Name: "n", Prompt: "p", EditorMode: types.CraftScheduledEditorModeDaily,
			Payload: json.RawMessage(`{"at":"24:00"}`)},
		{Name: "n", Prompt: "p", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"@every 30s"}`)},
		{Name: "n", Prompt: "p", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"*/5 * * * *"}`), Status: "enabled"},
	}
	for i, in := range bad {
		task, runID, err := svc.CreateScheduledTask(ctx, 1, "owner-a", in)
		require.ErrorIs(t, err, craft.ErrInvalidInput, "case %d", i)
		require.Nil(t, task, "case %d", i)
		require.Empty(t, runID, "case %d", i)
	}
	list, err := svc.ListScheduledTasks(ctx, 1, "owner-a")
	require.NoError(t, err)
	require.Len(t, list, 3, "only the three valid creations stored")

	_, _, err = svc.CreateScheduledTask(ctx, 0, "owner-a", bad[0])
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, _, err = svc.CreateScheduledTask(ctx, 1, "  ", bad[0])
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

// TestCraftScheduledServiceUpdateSemantics walks the PATCH entrance: partial
// merge, the editor-mode/payload pair rule, ticket recomputation (pause
// parks NULL, resume re-arms from now, an unchanged schedule recomputes to
// the same fire), validation and owner scoping.
func TestCraftScheduledServiceUpdateSemantics(t *testing.T) {
	svc, _ := newCraftScheduledFixture(t)
	ctx := context.Background()
	task, _, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "tick", Prompt: "go",
		EditorMode: types.CraftScheduledEditorModeInterval,
		Payload:    json.RawMessage(`{"every_minutes":30}`),
	})
	require.NoError(t, err)
	firstFire := time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC)

	strPtr := func(s string) *string { return &s }

	// Name-only edit: the recomputed ticket is the SAME fire — the schedule
	// did not move (idempotent recomputation).
	renamed := "renamed"
	updated, err := svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Name: &renamed})
	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.True(t, updated.NextRunAt.Equal(firstFire), "an unchanged schedule keeps its ticket")

	// The pair rule: mode without payload, payload without mode — both 400.
	mode := types.CraftScheduledEditorModeDaily
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{EditorMode: &mode})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID,
		CraftScheduledTaskUpdate{Payload: json.RawMessage(`{"at":"09:30"}`)})
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Schedule change: daily 09:30 recompiles the stored cron and re-arms
	// the ticket to tomorrow's fire.
	updated, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{
		EditorMode: &mode, Payload: json.RawMessage(`{"at":"09:30"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "30 9 * * *", updated.CronExpression)
	require.Equal(t, types.CraftScheduledEditorModeDaily, updated.EditorMode)
	require.True(t, updated.NextRunAt.Equal(time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC)))

	// Pause parks the ticket; resume re-arms from the pinned now.
	paused := types.CraftScheduledTaskStatusPaused
	updated, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Status: &paused})
	require.NoError(t, err)
	require.Equal(t, types.CraftScheduledTaskStatusPaused, updated.Status)
	require.Nil(t, updated.NextRunAt)
	active := types.CraftScheduledTaskStatusActive
	updated, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Status: &active})
	require.NoError(t, err)
	require.Equal(t, types.CraftScheduledTaskStatusActive, updated.Status)
	require.True(t, updated.NextRunAt.Equal(time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC)),
		"resume re-arms the next fire after now")

	// Invalid partials and foreign scope.
	empty := ""
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Name: &empty})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	badMode := "weekly"
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID,
		CraftScheduledTaskUpdate{EditorMode: &badMode, Payload: json.RawMessage(`{"every_minutes":5}`)})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	badStatus := "enabled"
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Status: &badStatus})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	badCronMode := types.CraftScheduledEditorModeAdvanced
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID,
		CraftScheduledTaskUpdate{EditorMode: &badCronMode, Payload: json.RawMessage(`{"cron":"nope"}`)})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-b", task.ID, CraftScheduledTaskUpdate{Name: strPtr("steal")})
	require.ErrorIs(t, err, craft.ErrNotFound, "a foreign recipe is an indistinguishable 404")
	_, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", "missing", CraftScheduledTaskUpdate{Name: strPtr("x")})
	require.ErrorIs(t, err, craft.ErrNotFound)

	// Prompt edit stores trimmed.
	prompt := "  fresh prompt  "
	updated, err = svc.UpdateScheduledTask(ctx, 1, "owner-a", task.ID, CraftScheduledTaskUpdate{Prompt: &prompt})
	require.NoError(t, err)
	require.Equal(t, "fresh prompt", updated.Prompt)
}

// TestCraftScheduledServiceReadAndDeleteSemantics covers the read side and
// the tombstone: the next-3-fires preview only for active recipes, the
// newest-first owner list, soft-delete idempotence and the 404 scoping.
func TestCraftScheduledServiceReadAndDeleteSemantics(t *testing.T) {
	svc, _ := newCraftScheduledFixture(t)
	ctx := context.Background()
	created := make([]string, 0, 2)
	for _, in := range []CraftScheduledTaskCreate{
		{Name: "daily", Prompt: "p", EditorMode: types.CraftScheduledEditorModeDaily,
			Payload: json.RawMessage(`{"at":"09:30"}`)},
		{Name: "paused", Prompt: "p", EditorMode: types.CraftScheduledEditorModeAdvanced,
			Payload: json.RawMessage(`{"cron":"*/5 * * * *"}`),
			Status:  types.CraftScheduledTaskStatusPaused},
	} {
		task, _, err := svc.CreateScheduledTask(ctx, 1, "owner-a", in)
		require.NoError(t, err)
		created = append(created, task.ID)
	}
	foreign, _, err := svc.CreateScheduledTask(ctx, 1, "owner-b", CraftScheduledTaskCreate{
		Name: "not yours", Prompt: "p",
		EditorMode: types.CraftScheduledEditorModeAdvanced,
		Payload:    json.RawMessage(`{"cron":"*/5 * * * *"}`),
	})
	require.NoError(t, err)

	// Preview: active daily answers the next three 09:30 fires (Tue-Thu).
	detail, err := svc.GetScheduledTask(ctx, 1, "owner-a", created[0])
	require.NoError(t, err)
	require.Equal(t, []time.Time{
		time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 9, 23, 9, 30, 0, 0, time.UTC),
		time.Date(2026, 9, 24, 9, 30, 0, 0, time.UTC),
	}, detail.NextFires)

	// A paused recipe previews nothing: nothing is scheduled.
	detail, err = svc.GetScheduledTask(ctx, 1, "owner-a", created[1])
	require.NoError(t, err)
	require.Empty(t, detail.NextFires)
	require.Equal(t, types.CraftScheduledTaskStatusPaused, detail.Task.Status)

	// Foreign and missing recipes are the same 404.
	_, err = svc.GetScheduledTask(ctx, 1, "owner-a", foreign.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = svc.GetScheduledTask(ctx, 1, "owner-b", created[0])
	require.ErrorIs(t, err, craft.ErrNotFound)

	// The owner list carries only the owner's live recipes.
	list, err := svc.ListScheduledTasks(ctx, 1, "owner-a")
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Soft delete: idempotent, hides the recipe from get and list; deleting
	// another owner's recipe is the same silent success.
	require.NoError(t, svc.DeleteScheduledTask(ctx, 1, "owner-a", created[0]))
	require.NoError(t, svc.DeleteScheduledTask(ctx, 1, "owner-a", created[0]),
		"the second delete is a no-op success")
	_, err = svc.GetScheduledTask(ctx, 1, "owner-a", created[0])
	require.ErrorIs(t, err, craft.ErrNotFound)
	list, err = svc.ListScheduledTasks(ctx, 1, "owner-a")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NoError(t, svc.DeleteScheduledTask(ctx, 1, "owner-b", created[1]),
		"a foreign delete is a silent no-op")
	detail, err = svc.GetScheduledTask(ctx, 1, "owner-a", created[1])
	require.NoError(t, err, "the foreign delete touched nothing")
	require.Equal(t, created[1], detail.Task.ID)
}

// TestCraftScheduledServiceRunNow covers the manual entrance: a queued run
// with the manual trigger source is appended, the claim ticket is untouched
// (active keeps its fire, paused stays parked), and only live own recipes
// can be fired.
func TestCraftScheduledServiceRunNow(t *testing.T) {
	svc, repo := newCraftScheduledFixture(t)
	ctx := context.Background()
	active, _, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "tick", Prompt: "p",
		EditorMode: types.CraftScheduledEditorModeInterval,
		Payload:    json.RawMessage(`{"every_minutes":30}`),
	})
	require.NoError(t, err)
	paused, _, err := svc.CreateScheduledTask(ctx, 1, "owner-a", CraftScheduledTaskCreate{
		Name: "parked", Prompt: "p",
		EditorMode: types.CraftScheduledEditorModeAdvanced,
		Payload:    json.RawMessage(`{"cron":"*/5 * * * *"}`),
		Status:     types.CraftScheduledTaskStatusPaused,
	})
	require.NoError(t, err)

	// Active: a manual fire never consumes or shifts the ticket.
	runID, err := svc.RunScheduledTaskNow(ctx, 1, "owner-a", active.ID)
	require.NoError(t, err)
	require.NotEmpty(t, runID)
	runs, err := repo.ListRunsByTask(ctx, active.ID, nil, 50)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, runID, runs[0].ID)
	require.Equal(t, types.CraftScheduledRunStatusQueued, runs[0].Status)
	require.Equal(t, types.CraftScheduledTriggerManualRunNow, runs[0].TriggerSource)
	detail, err := svc.GetScheduledTask(ctx, 1, "owner-a", active.ID)
	require.NoError(t, err)
	require.True(t, detail.Task.NextRunAt.Equal(time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC)),
		"run-now does not move the ticket")

	// Paused recipes may be fired manually; the ticket stays parked.
	runID, err = svc.RunScheduledTaskNow(ctx, 1, "owner-a", paused.ID)
	require.NoError(t, err)
	require.NotEmpty(t, runID)
	detail, err = svc.GetScheduledTask(ctx, 1, "owner-a", paused.ID)
	require.NoError(t, err)
	require.Nil(t, detail.Task.NextRunAt, "a manual fire does not arm a paused recipe")

	// A deleted or foreign recipe is the same 404.
	require.NoError(t, svc.DeleteScheduledTask(ctx, 1, "owner-a", paused.ID))
	_, err = svc.RunScheduledTaskNow(ctx, 1, "owner-a", paused.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = svc.RunScheduledTaskNow(ctx, 1, "owner-b", active.ID)
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = svc.RunScheduledTaskNow(ctx, 1, "owner-a", "missing")
	require.ErrorIs(t, err, craft.ErrNotFound)
}
