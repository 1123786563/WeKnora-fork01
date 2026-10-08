package workbench

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	appdatabase "github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openApplicationTaskDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	require.NoError(t, err)
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	dbPath := filepath.Join(t.TempDir(), "application-task.db")
	require.NoError(t, appdatabase.RunMigrationsWithOptions(
		"sqlite3://unused",
		appdatabase.MigrationOptions{SQLiteDBPath: dbPath},
	))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func applicationTaskIntent() interfaces.CareerApplicationTaskIntent {
	return interfaces.CareerApplicationTaskIntent{
		ApplicationID: "0cd7ee38-03e5-45bf-a070-6a8675da7db3",
		RequestID:     "career-create-application",
		Title:         "\n  Backend   Engineer  Application \n",
	}
}

func requireApplicationProjection(t *testing.T, db *gorm.DB, link interfaces.CareerApplicationTaskLink) {
	t.Helper()
	ctx := context.Background()

	var session struct {
		ID               string
		TenantID         uint64
		UserID           string
		Title            string
		EngineType       string
		ActiveAgentRunID *string
	}
	require.NoError(t, db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND user_id = ?", uint64(701), "owner-1").
		Take(&session).Error)
	require.Equal(t, link.TaskID, session.ID)
	require.Equal(t, uint64(701), session.TenantID)
	require.Equal(t, "owner-1", session.UserID)
	require.Equal(t, "Backend Engineer Application", session.Title)
	require.Equal(t, "builtin", session.EngineType)
	require.Nil(t, session.ActiveAgentRunID)

	var run struct {
		TenantID           uint64
		RunID              string
		SessionID          string
		OwnerID            string
		RequestID          string
		AssistantMessageID string
		RequestHash        string
		EngineType         string
		Driver             string
		TargetID           string
		BudgetRef          string
		Status             string
		WaitReason         string
		Snapshot           string
		GraphVersion       string
		SchemaVersion      int
		LeaseOwner         string
		Epoch              int64
		Revision           int64
		TokenBudget        int64
		Deadline           time.Time
	}
	require.NoError(t, db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND owner_id = ?", uint64(701), "owner-1").
		Take(&run).Error)
	require.Equal(t, link.RunID, run.RunID)
	require.Equal(t, link.TaskID, run.SessionID)
	require.Equal(t, "career-create-application", run.RequestID)
	require.Empty(t, run.AssistantMessageID)
	require.Len(t, run.RequestHash, 64)
	require.Equal(t, "trpc", run.EngineType)
	require.Equal(t, "platform", run.Driver)
	require.Equal(t, "career_application", run.TargetID)
	require.Empty(t, run.BudgetRef)
	require.Equal(t, "waiting_user", run.Status)
	require.Equal(t, "career_application_linking", run.WaitReason)
	require.Equal(t, "1", run.GraphVersion)
	require.Equal(t, 1, run.SchemaVersion)
	require.Empty(t, run.LeaseOwner)
	require.Zero(t, run.Epoch)
	require.Equal(t, int64(1), run.Revision)
	require.Zero(t, run.TokenBudget)
	require.WithinDuration(t, time.Now().Add(time.Hour), run.Deadline, time.Minute)

	var snapshot map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Snapshot), &snapshot))
	require.Equal(t, applicationTaskIntent().ApplicationID, snapshot["application_id"])
	require.Equal(t, applicationTaskIntent().RequestID, snapshot["request_id"])
	require.Equal(t, "Backend Engineer Application", snapshot["title"])

	for table, expected := range map[string]int{
		"sessions":                    1,
		"agent_runs":                  1,
		"messages":                    0,
		"workbench_requests":          0,
		"workbench_application_tasks": 1,
	} {
		var count int64
		require.NoError(t, db.WithContext(ctx).Table(table).Count(&count).Error)
		require.Equal(t, int64(expected), count, table)
	}

	var mapping struct {
		TenantID        uint64
		OwnerID         string
		Origin          string
		OriginRequestID string
		ApplicationID   string
		TaskID          string
		RunID           string
		Title           string
	}
	require.NoError(t, db.WithContext(ctx).Table("workbench_application_tasks").
		Where("tenant_id = ? AND owner_id = ?", uint64(701), "owner-1").
		Take(&mapping).Error)
	require.Equal(t, uint64(701), mapping.TenantID)
	require.Equal(t, "owner-1", mapping.OwnerID)
	require.Equal(t, "career_application", mapping.Origin)
	require.Equal(t, applicationTaskIntent().RequestID, mapping.OriginRequestID)
	require.Equal(t, applicationTaskIntent().ApplicationID, mapping.ApplicationID)
	require.Equal(t, link.TaskID, mapping.TaskID)
	require.Equal(t, link.RunID, mapping.RunID)
	require.Equal(t, "Backend Engineer Application", mapping.Title)
}

func TestEnsureCareerApplicationTaskCreatesOwnedVisibleProjection(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)

	link, err := coordinator.EnsureCareerApplicationTask(
		context.Background(), 701, "owner-1", applicationTaskIntent(),
	)
	require.NoError(t, err)
	require.NotEmpty(t, link.TaskID)
	require.NotEmpty(t, link.RunID)
	requireApplicationProjection(t, db, link)

	page, err := repository.NewWorkbenchListStore(db).ListOwnedExecutions(
		context.Background(), 701, "owner-1", repository.WorkbenchExecutionFilter{Status: "waiting_user"},
	)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, link.RunID, page.Items[0].RunID)
	require.Equal(t, link.TaskID, page.Items[0].SessionID)
	require.Equal(t, "Backend Engineer Application", page.Items[0].Title)
	require.Equal(t, "career_application_linking", page.Items[0].WaitReason)
	require.Equal(t, "required", page.Items[0].Attention)
}

func TestEnsureCareerApplicationTaskReplayReturnsSameTaskAndRun(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	first, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)
	replayIntent := applicationTaskIntent()
	replayIntent.Title = " Backend   Engineer Application "
	second, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", replayIntent)
	require.NoError(t, err)
	require.Equal(t, first, second)

	var sessions, runs, mappings int64
	require.NoError(t, db.Table("sessions").Count(&sessions).Error)
	require.NoError(t, db.Table("agent_runs").Count(&runs).Error)
	require.NoError(t, db.Table("workbench_application_tasks").Count(&mappings).Error)
	require.Equal(t, int64(1), sessions)
	require.Equal(t, int64(1), runs)
	require.Equal(t, int64(1), mappings)
}

type applicationTaskCallResult struct {
	link interfaces.CareerApplicationTaskLink
	err  error
}

func ensureCareerApplicationTasksConcurrently(
	t *testing.T,
	coordinator *ApplicationTaskCoordinator,
	intents []interfaces.CareerApplicationTaskIntent,
) []applicationTaskCallResult {
	t.Helper()

	start := make(chan struct{})
	ready := make(chan struct{}, len(intents))
	sessionCreates := make(chan struct{}, len(intents))
	resumeSessionCreates := make(chan struct{})
	results := make(chan applicationTaskCallResult, len(intents))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	require.NoError(t, coordinator.db.Callback().Create().Before("gorm:create").Register(
		"test/pause_application_task_session_creates",
		func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Table != "sessions" {
				return
			}
			sessionCreates <- struct{}{}
			<-resumeSessionCreates
		},
	))
	defer func() {
		coordinator.db.Callback().Create().Remove("test/pause_application_task_session_creates")
	}()
	for _, intent := range intents {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			<-start
			link, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", intent)
			results <- applicationTaskCallResult{link: link, err: err}
		}()
	}
	for range intents {
		<-ready
	}
	close(start)
	for range intents {
		<-sessionCreates
	}
	close(resumeSessionCreates)
	workers.Wait()
	close(results)

	output := make([]applicationTaskCallResult, 0, len(intents))
	for result := range results {
		output = append(output, result)
	}
	return output
}

func TestEnsureCareerApplicationTaskConcurrentExactReplayReturnsOriginal(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)

	intents := make([]interfaces.CareerApplicationTaskIntent, 2)
	for i := range intents {
		intents[i] = applicationTaskIntent()
	}
	results := ensureCareerApplicationTasksConcurrently(t, coordinator, intents)
	for _, result := range results {
		require.NoError(t, result.err)
		require.Equal(t, results[0].link, result.link)
	}
	requireApplicationProjection(t, db, results[0].link)
}

func TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)

	intents := make([]interfaces.CareerApplicationTaskIntent, 2)
	for i := range intents {
		intents[i] = applicationTaskIntent()
		intents[i].ApplicationID = uuid.NewString()
	}
	results := ensureCareerApplicationTasksConcurrently(t, coordinator, intents)
	winners := 0
	for _, result := range results {
		if result.err == nil {
			winners++
			continue
		}
		require.ErrorIs(t, result.err, ErrApplicationTaskConflict)
	}
	require.Equal(t, 1, winners)
	var sessions, runs, mappings int64
	require.NoError(t, db.Table("sessions").Count(&sessions).Error)
	require.NoError(t, db.Table("agent_runs").Count(&runs).Error)
	require.NoError(t, db.Table("workbench_application_tasks").Count(&mappings).Error)
	require.Equal(t, int64(1), sessions)
	require.Equal(t, int64(1), runs)
	require.Equal(t, int64(1), mappings)
}

func TestEnsureCareerApplicationTaskCanonicalizesEquivalentUUIDForms(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	first, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)
	for _, equivalentForm := range []string{
		"{0CD7EE38-03E5-45BF-A070-6A8675DA7DB3}",
		"0CD7EE3803E545BFA0706A8675DA7DB3",
		"urn:uuid:0cd7ee38-03e5-45bf-a070-6a8675da7db3",
	} {
		equivalent := applicationTaskIntent()
		equivalent.ApplicationID = equivalentForm
		replayed, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", equivalent)
		require.NoError(t, err)
		require.Equal(t, first, replayed)
	}

	var applications []string
	require.NoError(t, db.Table("workbench_application_tasks").
		Pluck("application_id", &applications).Error)
	require.Equal(t, []string{applicationTaskIntent().ApplicationID}, applications)
}

func TestEnsureCareerApplicationTaskChangedIntentConflicts(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	first, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)

	changedApplication := applicationTaskIntent()
	changedApplication.ApplicationID = "9e0af5ee-6ff2-45ab-b653-16eae47dcd8b"
	_, err = coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", changedApplication)
	require.ErrorIs(t, err, ErrApplicationTaskConflict)

	changedTitle := applicationTaskIntent()
	changedTitle.Title = "Staff Backend Engineer Application"
	_, err = coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", changedTitle)
	require.ErrorIs(t, err, ErrApplicationTaskConflict)

	differentRequest := applicationTaskIntent()
	differentRequest.RequestID = "career-create-application-2"
	_, err = coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", differentRequest)
	require.ErrorIs(t, err, ErrApplicationTaskConflict)

	requireApplicationProjection(t, db, first)
}

func TestFindCareerApplicationTaskIsTenantAndOwnerScoped(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	created, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)
	found, err := coordinator.FindCareerApplicationTask(ctx, 701, "owner-1", "career-create-application")
	require.NoError(t, err)
	require.Equal(t, created.TaskID, found.TaskID)
	require.Equal(t, created.RunID, found.RunID)
	// The finder discloses which application owns the durable task so the
	// caller can reject a request ID that resolved to foreign content.
	require.Equal(t, "0cd7ee38-03e5-45bf-a070-6a8675da7db3", found.ApplicationID)

	_, err = coordinator.FindCareerApplicationTask(ctx, 702, "owner-1", "career-create-application")
	require.ErrorIs(t, err, ErrApplicationTaskNotFound)
	_, err = coordinator.FindCareerApplicationTask(ctx, 701, "owner-2", "career-create-application")
	require.ErrorIs(t, err, ErrApplicationTaskNotFound)
}

func TestApplicationTaskProjectionIsListReadableArchivableAndRestorable(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	runs := repository.NewAgentRunStore(db)
	lists := repository.NewWorkbenchListStore(db)
	states := repository.NewWorkbenchTaskStateStore(db)
	ctx := context.Background()

	link, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)
	run, err := runs.GetOwnedRun(ctx, 701, "owner-1", link.RunID)
	require.NoError(t, err)
	require.Equal(t, link.RunID, run.Key.RunID)
	require.Equal(t, link.TaskID, run.SessionID)
	require.Equal(t, "career-create-application", run.RequestID)

	facts, err := lists.ReadTaskFactsForRun(ctx, 701, "owner-1", link.RunID)
	require.NoError(t, err)
	require.Equal(t, link.TaskID, facts.TaskID)
	require.Equal(t, "Backend Engineer Application", facts.Title)
	require.Equal(t, "required", facts.Attention)
	require.Empty(t, facts.ArchivedAt)

	_, err = runs.GetOwnedRun(ctx, 702, "owner-1", link.RunID)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = runs.GetOwnedRun(ctx, 701, "owner-2", link.RunID)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = lists.ReadTaskFactsForRun(ctx, 702, "owner-1", link.RunID)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = lists.ReadTaskFactsForRun(ctx, 701, "owner-2", link.RunID)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	archivedAt := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	err = states.SetTaskArchived(ctx, 702, "owner-1", link.TaskID, true, archivedAt)
	require.ErrorIs(t, err, repository.ErrWorkbenchTaskNotFound)
	err = states.SetTaskArchived(ctx, 701, "owner-2", link.TaskID, true, archivedAt)
	require.ErrorIs(t, err, repository.ErrWorkbenchTaskNotFound)
	require.NoError(t, states.SetTaskArchived(ctx, 701, "owner-1", link.TaskID, true, archivedAt))

	active, err := lists.ListOwnedExecutions(ctx, 701, "owner-1", repository.WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Empty(t, active.Items)
	archived, err := lists.ListOwnedExecutions(ctx, 701, "owner-1", repository.WorkbenchExecutionFilter{ArchivedOnly: true})
	require.NoError(t, err)
	require.Len(t, archived.Items, 1)
	require.Equal(t, link.RunID, archived.Items[0].RunID)
	require.Equal(t, archivedAt.UTC().Format(time.RFC3339Nano), archived.Items[0].ArchivedAt)

	require.NoError(t, states.SetTaskArchived(ctx, 701, "owner-1", link.TaskID, false, archivedAt.Add(time.Minute)))
	restored, err := lists.ListOwnedExecutions(ctx, 701, "owner-1", repository.WorkbenchExecutionFilter{})
	require.NoError(t, err)
	require.Len(t, restored.Items, 1)
	require.Equal(t, link.RunID, restored.Items[0].RunID)
}

func TestApplicationTaskMigrationUpAndDownShapes(t *testing.T) {
	db := openApplicationTaskDB(t)

	var columns []string
	require.NoError(t, db.Raw(
		"SELECT name FROM pragma_table_info('workbench_application_tasks') ORDER BY cid",
	).Scan(&columns).Error)
	require.Equal(t, []string{
		"tenant_id", "owner_id", "origin", "origin_request_id", "application_id",
		"task_id", "run_id", "title", "created_at", "updated_at",
	}, columns)

	type indexDefinition struct {
		Name string
		SQL  string
	}
	var indexes []indexDefinition
	require.NoError(t, db.Table("sqlite_master").
		Select("name, sql").
		Where("type = ? AND name IN ?", "index", []string{
			"uq_workbench_application_tasks_request",
			"uq_workbench_application_tasks_application",
		}).
		Find(&indexes).Error)
	require.Len(t, indexes, 2)
	for _, index := range indexes {
		require.Contains(t, index.SQL, "CREATE UNIQUE INDEX")
	}

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, execMigrationFile(sqlDB, "migrations/sqlite/000145_workbench_application_tasks.down.sql"))
	var count int
	require.NoError(t, sqlDB.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'workbench_application_tasks'",
	).Scan(&count))
	require.Zero(t, count)
}

func execMigrationFile(db *sql.DB, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := db.Exec(string(contents)); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}

// Step-0 debt from the T14 subtask-1 review, revised by the OCR r1 fix: an
// exhausted race budget is NOT a definite conflict — a twin request may have
// committed after the last attempt. The recovery Find resolves whatever
// landed; with nothing durable the caller receives the typed undecided
// sentinel (never the raw lock error) so Career keeps its linking state.
func TestEnsureApplicationTaskRetryExhaustionIsUndecidedWithoutDurableTask(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	calls := 0
	once := func(context.Context, uint64, string, interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
		calls++
		return interfaces.CareerApplicationTaskLink{}, errors.New("database is locked")
	}
	link, err := coordinator.ensureWithRetry(
		context.Background(), 701, "owner-1", applicationTaskIntent(), once,
	)
	require.Empty(t, link.TaskID)
	require.Empty(t, link.RunID)
	require.ErrorIs(t, err, ErrApplicationTaskUndecided)
	require.NotErrorIs(t, err, ErrApplicationTaskConflict, "an exhausted race is undecided, not a definite conflict")
	require.NotEqual(t, "database is locked", err.Error(), "raw race error must not leak to the caller")
	require.Equal(t, 3, calls, "the bounded retry budget must stay at three attempts")
}

// When the racing twin's commit lands before the recovery Find, the
// exhausted retry resolves to the durable task instead of reporting
// undecided.
func TestEnsureApplicationTaskRetryExhaustionRecoversDurableTwin(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()
	ensured, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)

	once := func(context.Context, uint64, string, interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
		return interfaces.CareerApplicationTaskLink{}, errors.New("database is locked")
	}
	link, err := coordinator.ensureWithRetry(ctx, 701, "owner-1", applicationTaskIntent(), once)
	require.NoError(t, err)
	require.Equal(t, ensured.TaskID, link.TaskID)
	require.Equal(t, ensured.RunID, link.RunID)
}

// Pure input-validation failures carry their own sentinel so Career can
// answer 400 instead of a 409 conflict for an intent that was never written.
func TestApplicationTaskValidationFailuresAreInvalidNotConflict(t *testing.T) {
	_, _, _, err := normalizeApplicationTaskScope(1, "owner", strings.Repeat("r", 65))
	require.ErrorIs(t, err, ErrApplicationTaskInvalid)
	require.NotErrorIs(t, err, ErrApplicationTaskConflict)

	intent := applicationTaskIntent()
	intent.ApplicationID = "not-a-uuid"
	_, err = normalizeApplicationTaskIntent(1, "owner", intent)
	require.ErrorIs(t, err, ErrApplicationTaskInvalid)
	require.NotErrorIs(t, err, ErrApplicationTaskConflict)

	intent = applicationTaskIntent()
	intent.Title = ""
	_, err = normalizeApplicationTaskIntent(1, "owner", intent)
	require.ErrorIs(t, err, ErrApplicationTaskInvalid)
}

func TestEnsureApplicationTaskNonRaceErrorIsReturnedAsIs(t *testing.T) {
	coordinator := NewApplicationTaskCoordinator(nil)
	calls := 0
	cause := errors.New("connection refused")
	once := func(context.Context, uint64, string, interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
		calls++
		return interfaces.CareerApplicationTaskLink{}, cause
	}
	link, err := coordinator.ensureWithRetry(
		context.Background(), 701, "owner-1", applicationTaskIntent(), once,
	)
	require.Empty(t, link.TaskID)
	require.ErrorIs(t, err, cause)
	require.Equal(t, 1, calls, "non-race errors must not be retried")
}

func TestIsApplicationTaskCreationRaceMatchesOnlyPlannedMarkers(t *testing.T) {
	for _, err := range []error{
		gorm.ErrDuplicatedKey,
		fmt.Errorf("insert session: %w", gorm.ErrDuplicatedKey),
		errors.New(`duplicate key value violates unique constraint "uq_agent_runs_request"`),
		errors.New(`duplicate key value violates unique constraint "uq_workbench_application_tasks_request"`),
		errors.New("database is locked"),
		errors.New("database table is locked"),
		errors.New("sqlite_busy: will retry preparable statement"),
	} {
		require.Truef(t, isApplicationTaskCreationRace(err), "planned race marker rejected: %v", err)
	}
	for _, err := range []error{
		errors.New("unique constraint failed: career_facts.tenant_id"),
		errors.New(`duplicate key value violates unique constraint "uq_career_applications_scope"`),
		errors.New("sqlstate 23505 without a request constraint name"),
		errors.New("deadlock detected"),
		errors.New("serialization failure: sqlstate 40001"),
		errors.New("sqlstate 40P01"),
		errors.New("connection refused"),
	} {
		require.Falsef(t, isApplicationTaskCreationRace(err), "marker outside the fix-r1 plan accepted: %v", err)
	}
}

// OCR round 2 fix ocr2-150: when the exhausted race budget recovers a durable
// row, that row must still match THIS intent — a twin that committed the same
// request ID for a different application is a typed conflict, never a
// successful link handed to the losing application.
func TestEnsureApplicationTaskRetryExhaustionRejectsForeignTwin(t *testing.T) {
	db := openApplicationTaskDB(t)
	coordinator := NewApplicationTaskCoordinator(db)
	ctx := context.Background()

	_, err := coordinator.EnsureCareerApplicationTask(ctx, 701, "owner-1", applicationTaskIntent())
	require.NoError(t, err)

	loser := applicationTaskIntent()
	loser.ApplicationID = uuid.NewString()
	once := func(context.Context, uint64, string, interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
		return interfaces.CareerApplicationTaskLink{}, errors.New("database is locked")
	}
	link, err := coordinator.ensureWithRetry(ctx, 701, "owner-1", loser, once)
	require.Empty(t, link.TaskID)
	require.Empty(t, link.RunID)
	require.ErrorIs(t, err, ErrApplicationTaskConflict, "the foreign twin's link must not be handed out")
	require.NotErrorIs(t, err, ErrApplicationTaskUndecided, "a durable foreign row is a definite conflict, not undecided")

	// The durable row still belongs to the winner, untouched.
	var mappings int64
	require.NoError(t, db.Table("workbench_application_tasks").
		Where("origin_request_id = ?", "career-create-application").Count(&mappings).Error)
	require.Equal(t, int64(1), mappings)
}
