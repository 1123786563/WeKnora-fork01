package repository_test

// Repository-layer old-vs-new differential gate (Wave 1, Task 9, brief
// Step 6). Two INDEPENDENT SQLite databases (isolated temp files, same
// migration chain, identical fixtures) each serve one stack: the LEGACY
// QueryHistoryExportJobRepository on one, the NEW adapters.GormExportJobStore
// + adapters.LegacyAudit (over the SessionAuditRepository seam) on the other.
// Job CRUD states compare through testkit.JobObservation; export rows compare
// value-for-value (types.QueryHistoryExportRow is the Wave 1 alias of
// domain.ExportRow).
//
// Scenario list (brief Step 6, verbatim):
//
//	job CRUD, tenant-scoped missing/foreign reads, source classification,
//	user/time/feedback filters, chronological tie-breaking, counts,
//	soft-delete exclusion, skill-maintenance exclusion, and the 10,000-row cap.

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tencent/WeKnora/internal/application/repository"
	qhadapters "github.com/Tencent/WeKnora/internal/conversation/queryhistory/adapters"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/testkit"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// diffRepoClock is the fixture clock every session timestamp comes from, so
// both databases hold byte-identical rows and the row comparisons (including
// rendered times) are deterministic.
var diffRepoClock = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// openDiffRepoDB opens one independent migrated SQLite database.
func openDiffRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "query-history-differential.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, closeErr := db.DB()
		if closeErr == nil {
			_ = conn.Close()
		}
	})
	return db
}

// seedDiffRepoFixtures inserts the identical audit fixture set into one
// database: web/api/embed/im origins, tallies, a soft-deleted row, a
// skill-maintenance row, a foreign-tenant row, and two same-timestamp
// sessions for tie-breaking.
func seedDiffRepoFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test'), (2, 'tenant-2', 'test')`,
	).Error)

	insertSession := func(id string, tenantID uint64, userID, description string, day int) {
		t.Helper()
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
			 VALUES (?, ?, ?, ?, ?, 'builtin')`,
			id, tenantID, "title-"+id, userID, description,
		).Error)
		// Bind timestamps through bound values so the driver serializes them
		// exactly as it serializes the query bounds.
		created := diffRepoClock.AddDate(0, 0, day)
		require.NoError(t, db.Exec(
			`UPDATE sessions SET created_at = ?, updated_at = ? WHERE id = ?`,
			created, created, id,
		).Error)
	}
	insertSession("rep-web", 1, "u1", "", 1)
	insertSession("rep-api", 1, types.SessionOwnerAPITenantKeyPrefix+"k1", "", 2)
	insertSession("rep-embed", 1, "", types.EmbedSessionMarkerPrefix+"ch1", 3)
	insertSession("rep-im", 1, "", "", 4)
	insertSession("rep-deleted", 1, "u1", "", 5)
	require.NoError(t, db.Exec(
		`UPDATE sessions SET deleted_at = ? WHERE id = 'rep-deleted'`, diffRepoClock,
	).Error)
	insertSession("rep-skill", 1, "u1", types.SkillMaintenanceSessionMarker+"job1", 6)
	insertSession("rep-other", 2, "u2", "", 1)
	// Same created_at on both: the tie-break is the session id, ascending.
	tie := diffRepoClock.AddDate(0, 0, 7)
	for _, id := range []string{"rep-tie-b", "rep-tie-a"} {
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, created_at, updated_at)
			 VALUES (?, 1, ?, 'u1', 'builtin', ?, ?)`, id, "title-"+id, tie, tie,
		).Error)
	}

	require.NoError(t, db.Exec(
		`INSERT INTO im_channel_sessions (id, platform, user_id, chat_id, session_id, tenant_id)
		 VALUES ('rep-ics-1', 'feishu', 'fu1', 'chat1', 'rep-im', 1)`,
	).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO messages (id, request_id, session_id, role, content) VALUES
		 ('rm-1', 'rr-1', 'rep-web', 'user', 'q'),
		 ('rm-2', 'rr-2', 'rep-web', 'assistant', 'a'),
		 ('rm-3', 'rr-3', 'rep-api', 'user', 'q')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating) VALUES
		 (101, 1, 'u1', 'rm-1', 'rep-web', 'like'),
		 (102, 1, 'u1', 'rm-2', 'rep-web', 'dislike'),
		 (103, 1, 'u1', 'rm-3', 'rep-api', 'like'),
		 (104, 2, 'foreign', 'rm-1', 'rep-web', 'like')`,
	).Error)
}

// newDiffRepoStacks opens and seeds the two databases and wires both stacks:
// the legacy repository over one, the new adapters over the other.
func newDiffRepoStacks(t *testing.T) (
	legacy interfaces.QueryHistoryExportJobRepository,
	modernJobs *qhadapters.GormExportJobStore,
	modernRows *qhadapters.LegacyAudit,
) {
	t.Helper()
	legacyDB := openDiffRepoDB(t)
	newDB := openDiffRepoDB(t)
	seedDiffRepoFixtures(t, legacyDB)
	seedDiffRepoFixtures(t, newDB)
	return repository.NewQueryHistoryExportJobRepository(legacyDB),
		qhadapters.NewGormExportJobStore(newDB),
		qhadapters.NewLegacyAudit(repository.NewSessionAuditRepository(newDB))
}

// legacyJobObs projects a job row onto its comparison observation
// (time-independent fields — the legacy stack has no injectable clock for
// row writes).
func legacyJobObs(job *types.QueryHistoryExportJob) testkit.JobObservation {
	return testkit.JobObservation{
		ID: job.ID, TenantID: job.TenantID, RequestedBy: job.RequestedBy,
		Status: job.Status, FilePath: job.FilePath, ErrorMessage: job.ErrorMessage,
	}
}

func newJobObs(job *domain.ExportJob) testkit.JobObservation {
	return testkit.JobObservation{
		ID: job.ID, TenantID: job.TenantID, RequestedBy: job.RequestedBy,
		Status: job.Status, FilePath: job.FilePath, ErrorMessage: job.ErrorMessage,
	}
}

// TestQueryHistoryDifferentialJobCRUD pins the job-lifecycle store contract:
// creation defaulting to pending, tenant-scoped reads, the failed transition
// with its message, and the done transition rewriting both columns.
func TestQueryHistoryDifferentialJobCRUD(t *testing.T) {
	legacy, modernJobs, _ := newDiffRepoStacks(t)
	ctx := context.Background()

	legacyJob := &types.QueryHistoryExportJob{TenantID: 1, RequestedBy: "admin-1"}
	require.NoError(t, legacy.Create(ctx, legacyJob))
	newJob := &domain.ExportJob{TenantID: 1, RequestedBy: "admin-1"}
	require.NoError(t, modernJobs.Create(ctx, newJob))
	require.NotZero(t, legacyJob.ID)
	require.Equal(t, legacyJob.ID, newJob.ID, "both empty tables admit the first job as id 1")

	// Created pending.
	legacyGot, err := legacy.GetByID(ctx, 1, legacyJob.ID)
	require.NoError(t, err)
	newGot, err := modernJobs.Get(ctx, 1, newJob.ID)
	require.NoError(t, err)
	require.NoError(t, testkit.Compare(
		testkit.Observation{Jobs: []testkit.JobObservation{legacyJobObs(legacyGot)}},
		testkit.Observation{Jobs: []testkit.JobObservation{newJobObs(newGot)}}))

	// Failed transition records the message.
	require.NoError(t, legacy.UpdateStatus(ctx, legacyJob.ID, types.QueryHistoryExportFailed, "", "boom"))
	require.NoError(t, modernJobs.UpdateStatus(ctx, 1, newJob.ID, domain.ExportFailed, "", "boom"))
	legacyGot, err = legacy.GetByID(ctx, 1, legacyJob.ID)
	require.NoError(t, err)
	newGot, err = modernJobs.Get(ctx, 1, newJob.ID)
	require.NoError(t, err)
	require.NoError(t, testkit.Compare(
		testkit.Observation{Jobs: []testkit.JobObservation{legacyJobObs(legacyGot)}},
		testkit.Observation{Jobs: []testkit.JobObservation{newJobObs(newGot)}}))
	require.Equal(t, "boom", newGot.ErrorMessage)

	// Done transition rewrites both columns: no stale error survives.
	require.NoError(t, legacy.UpdateStatus(ctx, legacyJob.ID, types.QueryHistoryExportDone, "local://exports/1.csv", ""))
	require.NoError(t, modernJobs.UpdateStatus(ctx, 1, newJob.ID, domain.ExportDone, "local://exports/1.csv", ""))
	legacyGot, err = legacy.GetByID(ctx, 1, legacyJob.ID)
	require.NoError(t, err)
	newGot, err = modernJobs.Get(ctx, 1, newJob.ID)
	require.NoError(t, err)
	require.NoError(t, testkit.Compare(
		testkit.Observation{Jobs: []testkit.JobObservation{legacyJobObs(legacyGot)}},
		testkit.Observation{Jobs: []testkit.JobObservation{newJobObs(newGot)}}))
	require.Empty(t, legacyGot.ErrorMessage)
	require.Empty(t, newGot.ErrorMessage)

	// Tenant-scoped missing/foreign reads answer the SAME sentinel so ids
	// cannot be probed across workspaces.
	_, legacyForeignErr := legacy.GetByID(ctx, 2, legacyJob.ID)
	_, newForeignErr := modernJobs.Get(ctx, 2, newJob.ID)
	require.ErrorIs(t, legacyForeignErr, repository.ErrQueryHistoryExportJobNotFound)
	require.ErrorIs(t, newForeignErr, repository.ErrQueryHistoryExportJobNotFound)
	require.Equal(t, legacyForeignErr.Error(), newForeignErr.Error())

	_, legacyMissingErr := legacy.GetByID(ctx, 1, 999999)
	_, newMissingErr := modernJobs.Get(ctx, 1, 999999)
	require.ErrorIs(t, legacyMissingErr, repository.ErrQueryHistoryExportJobNotFound)
	require.ErrorIs(t, newMissingErr, repository.ErrQueryHistoryExportJobNotFound)
	require.Equal(t, legacyMissingErr.Error(), newMissingErr.Error())
}

// diffRowsCase runs one export-rows query against both stacks and compares
// the rows value-for-value.
func diffRowsCase(t *testing.T, name string,
	legacyQuery *types.SessionListQuery, newFilter domain.ExportFilter,
) {
	t.Helper()
	legacyRepo, _, modernRows := newDiffRepoStacks(t)
	ctx := context.Background()

	legacyRows, err := legacyRepo.ExportSessionRows(ctx, legacyQuery)
	require.NoError(t, err)
	newRows, err := modernRows.ExportRows(ctx, legacyQuery.TenantID, newFilter)
	require.NoError(t, err)
	require.Equal(t, legacyRows, newRows,
		"differential %s: legacy and new rows must be identical", name)
}

// TestQueryHistoryDifferentialExportRows pins the shared export-row contract:
// origin classification, tallies (counts), the user/time/feedback filters,
// chronological ordering with id tie-breaking, the soft-delete exclusion, and
// the skill-maintenance exclusion.
func TestQueryHistoryDifferentialExportRows(t *testing.T) {
	legacyRepo, _, modernRows := newDiffRepoStacks(t)
	ctx := context.Background()

	// Whole-tenant view: classification, counts, ordering, exclusions.
	legacyRows, err := legacyRepo.ExportSessionRows(ctx, &types.SessionListQuery{
		TenantID: 1, Source: types.SessionListSourceAll,
	})
	require.NoError(t, err)
	newRows, err := modernRows.ExportRows(ctx, 1, domain.ExportFilter{})
	require.NoError(t, err)
	require.Equal(t, legacyRows, newRows)

	byID := make(map[string]types.QueryHistoryExportRow, len(newRows))
	for _, row := range newRows {
		byID[row.SessionID] = row
	}
	require.Equal(t, "web", byID["rep-web"].Source)
	require.Equal(t, "api", byID["rep-api"].Source)
	require.Equal(t, "embed", byID["rep-embed"].Source)
	require.Equal(t, "feishu", byID["rep-im"].Source, "IM origin classification")
	require.Equal(t, int64(2), byID["rep-web"].MessageCount)
	require.Equal(t, int64(1), byID["rep-web"].LikeCount)
	require.Equal(t, int64(1), byID["rep-web"].DislikeCount)
	require.Equal(t, int64(1), byID["rep-api"].LikeCount, "tenant-scoped feedback count")
	require.NotContains(t, byID, "rep-deleted", "soft-deleted sessions are excluded")
	require.NotContains(t, byID, "rep-skill", "skill-maintenance sessions are excluded")
	require.NotContains(t, byID, "rep-other", "foreign-tenant sessions are excluded")

	// Chronological order with the id tie-break: rep-tie-a sorts before
	// rep-tie-b at the same created_at.
	ids := make([]string, 0, len(newRows))
	for _, row := range newRows {
		ids = append(ids, row.SessionID)
	}
	require.Equal(t, []string{
		"rep-web", "rep-api", "rep-embed", "rep-im", "rep-tie-a", "rep-tie-b",
	}, ids)

	// Per-user drill-down.
	diffRowsCase(t, "user filter",
		&types.SessionListQuery{TenantID: 1, Source: types.SessionListSourceAll, UserID: "u1"},
		domain.ExportFilter{UserID: "u1"})

	// Half-open time window [start, end).
	diffRowsCase(t, "time window",
		&types.SessionListQuery{
			TenantID: 1, Source: types.SessionListSourceAll,
			StartTime: diffRepoClock.AddDate(0, 0, 2), EndTime: diffRepoClock.AddDate(0, 0, 4),
		},
		domain.ExportFilter{
			StartTime: diffRepoClock.AddDate(0, 0, 2), EndTime: diffRepoClock.AddDate(0, 0, 4),
		})

	// Feedback drill-down.
	diffRowsCase(t, "feedback filter",
		&types.SessionListQuery{
			TenantID: 1, Source: types.SessionListSourceAll,
			FeedbackRating: types.FeedbackRatingLike,
		},
		domain.ExportFilter{FeedbackRating: types.FeedbackRatingLike})

	// Foreign tenant scope on the new entry lands on the same rows the
	// legacy entry serves for that tenant.
	diffRowsCase(t, "foreign tenant scope",
		&types.SessionListQuery{TenantID: 2, Source: types.SessionListSourceAll},
		domain.ExportFilter{})
}

// TestQueryHistoryDifferentialExportRowsCap pins the 10,000-row anti-explosion
// cap on fresh databases (cap+1 sessions each): both stacks answer exactly
// the cap's worth of rows, keeping the OLDEST.
func TestQueryHistoryDifferentialExportRowsCap(t *testing.T) {
	const exportRowCap = 10000

	openAndFill := func(t *testing.T) *gorm.DB {
		t.Helper()
		db := openDiffRepoDB(t)
		require.NoError(t, db.Exec(
			`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`).Error)
		const total = exportRowCap + 1
		var sb strings.Builder
		sb.WriteString("INSERT INTO sessions (id, tenant_id, title, engine_type, created_at, updated_at) VALUES ")
		for i := 0; i < total; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			created := diffRepoClock.Add(time.Duration(i) * time.Minute)
			fmt.Fprintf(&sb, "('cap-%06d', 1, 't', 'builtin', '%s', '%s')", i,
				created.Format("2006-01-02 15:04:05"), created.Format("2006-01-02 15:04:05"))
		}
		require.NoError(t, db.Exec(sb.String()).Error)
		return db
	}

	legacyDB := openAndFill(t)
	newDB := openAndFill(t)
	ctx := context.Background()

	legacyRepo := repository.NewQueryHistoryExportJobRepository(legacyDB)
	modernRows := qhadapters.NewLegacyAudit(repository.NewSessionAuditRepository(newDB))

	legacyRows, err := legacyRepo.ExportSessionRows(ctx, &types.SessionListQuery{
		TenantID: 1, Source: types.SessionListSourceAll,
	})
	require.NoError(t, err)
	newRows, err := modernRows.ExportRows(ctx, 1, domain.ExportFilter{})
	require.NoError(t, err)

	require.Len(t, legacyRows, exportRowCap, "legacy keeps exactly the cap")
	require.Len(t, newRows, exportRowCap, "new keeps exactly the cap")
	require.Equal(t, legacyRows, newRows)
	require.Equal(t, "cap-000000", newRows[0].SessionID)
	require.Equal(t, "cap-009999", newRows[len(newRows)-1].SessionID,
		"the cap keeps the OLDEST rows")
}
