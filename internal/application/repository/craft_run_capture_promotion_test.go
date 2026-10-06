package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCraftRunCapturePromotionMigration(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		db := openRunTestDB(t)
		require.True(t, db.Migrator().HasTable("craft_run_capture_promotion_cursor"))
		require.True(t, db.Migrator().HasTable("craft_run_capture_promotion_attempts"))
		sqlDB, err := db.DB()
		require.NoError(t, err)
		root := promotionRepoRoot(t)
		driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
		require.NoError(t, err)
		m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = m.Close() })
		// F08's receipt migration follows F06 in this integration chain. Roll
		// back to just below the promotion migration (000187) so it is the
		// final down step. Version-anchored instead of Steps(-N): migrations
		// renumbered past 000190 by the upstream-merge dedup changed the tail
		// count, and no-op versions must roll back harmlessly on the way.
		require.NoError(t, m.Migrate(187), "roll back to before the promotion migration")
		require.False(t, db.Migrator().HasTable("craft_run_capture_promotion_cursor"))
		require.False(t, db.Migrator().HasTable("craft_run_capture_promotion_attempts"))
		require.False(t, db.Migrator().HasTable("craft_web_build_receipts"))
		require.NoError(t, m.Up(), "replay through the promotion migration and the renumbered tail")
		require.True(t, db.Migrator().HasTable("craft_run_capture_promotion_cursor"))
		require.True(t, db.Migrator().HasTable("craft_run_capture_promotion_attempts"))
		require.True(t, db.Migrator().HasTable("craft_web_build_receipts"))
	})
}

func TestCraftRunCapturePromotionFreshScanUsesBoundedIndex(t *testing.T) {
	db := openRunTestDB(t)
	seedPromotionPlanReceipts(t, db)
	for _, tc := range []struct {
		name   string
		cursor craftRunCapturePromotionCursorRow
	}{
		{name: "reset cursor"},
		{name: "persisted cursor", cursor: craftRunCapturePromotionCursorRow{
			UpdatedAt: ptr(time.Date(2026, 1, 1, 0, 0, 47, 0, time.UTC)), TenantID: ptr(uint64(1)), WorkspaceID: ptr("plan-workspace"), RunID: ptr("plan-run-047"),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := explainPromotionFreshScanAtCursor(t, db, tc.cursor)
			require.Contains(t, plan, "idx_craft_run_captures_promotion_scan")
			require.NotContains(t, strings.ToUpper(plan), "TEMP B-TREE")
		})
	}
}

func TestCraftRunCapturePromotionCursorAdvancesAcrossAttemptedHistory(t *testing.T) {
	db := openRunTestDB(t)
	seedPromotionPlanReceipts(t, db)
	future := time.Now().Add(time.Hour)
	for i := 0; i < 96; i++ {
		require.NoError(t, db.Exec(`INSERT INTO craft_run_capture_promotion_attempts
			(tenant_id,workspace_id,run_id,retry_after,completed,updated_at)
			VALUES (1,'plan-workspace',?,?,FALSE,?)`, fmt.Sprintf("plan-run-%03d", i), future, future).Error)
	}
	store := NewCraftRunCaptureStore(db)
	rows, err := store.ClaimPromotionBatch(context.Background(), 8, time.Minute)
	require.NoError(t, err)
	require.Empty(t, rows, "the first raw keyset page contains only previously attempted receipts")
	var cursor craftRunCapturePromotionCursorRow
	require.NoError(t, db.Where("id = 1").Take(&cursor).Error)
	require.NotNil(t, cursor.RunID)
	require.Equal(t, "plan-run-007", *cursor.RunID, "raw cursor advances by at most the bounded scan limit, even with no fresh claims")

	created := time.Date(2026, 1, 1, 0, 2, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`INSERT INTO craft_run_captures
		(tenant_id,workspace_id,run_id,owner_id,session_id,generation,predecessor_revision,predecessor_state,
		 predecessor_run_id,predecessor_digest,state,draft_revision,updated_at)
		VALUES (1,'plan-workspace','new-work','u1','s1','g1',0,'empty','','','sealed',1,?)`, created).Error)
	for scans := 1; scans <= 50; scans++ {
		rows, err = store.ClaimPromotionBatch(context.Background(), 8, time.Minute)
		require.NoError(t, err)
		if len(rows) > 0 {
			require.Len(t, rows, 1)
			require.Equal(t, "new-work", rows[0].RunID)
			require.LessOrEqual(t, scans, 12, "new work is reached through bounded raw keyset pages")
			return
		}
	}
	t.Fatal("bounded cursor pages did not reach the newly eligible receipt")
}

func TestCraftRunCapturePromotionDuePageUsesBoundedOrderIndex(t *testing.T) {
	db := openRunTestDB(t)
	seedPromotionPlanReceipts(t, db)
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 96; i++ {
		require.NoError(t, db.Exec(`INSERT INTO craft_run_capture_promotion_attempts
			(tenant_id,workspace_id,run_id,retry_after,completed,updated_at)
			VALUES (1,'plan-workspace',?,?,FALSE,?)`, fmt.Sprintf("plan-run-%03d", i), due, due).Error)
	}
	query := db.Session(&gorm.Session{DryRun: true}).Table("craft_run_captures AS c").
		Select("c.tenant_id,c.workspace_id,c.run_id,c.owner_id,c.session_id,c.state,c.draft_revision,c.updated_at").
		Where("c.state IN ('sealed','advanced') AND c.draft_revision IS NOT NULL")
	query = duePromotionQuery(query, due.Add(time.Hour), 24)
	var projection []craftPromotionReceiptRow
	generated := query.Find(&projection).Statement
	var planRows []struct{ Detail string }
	require.NoError(t, db.Raw("EXPLAIN QUERY PLAN "+generated.SQL.String(), generated.Vars...).Scan(&planRows).Error)
	var plan []string
	for _, row := range planRows {
		plan = append(plan, row.Detail)
	}
	planText := strings.Join(plan, "\n")
	require.Contains(t, planText, "idx_craft_capture_promotion_due")
	require.NotContains(t, strings.ToUpper(planText), "TEMP B-TREE")
}

func TestCraftRunCapturePromotionCursorWrapsAfterRawTailExhaustion(t *testing.T) {
	db := openRunTestDB(t)
	seedPromotionPlanReceipts(t, db)
	future := time.Now().Add(time.Hour)
	for i := 0; i < 96; i++ {
		require.NoError(t, db.Exec(`INSERT INTO craft_run_capture_promotion_attempts
			(tenant_id,workspace_id,run_id,retry_after,completed,updated_at)
			VALUES (1,'plan-workspace',?,?,FALSE,?)`, fmt.Sprintf("plan-run-%03d", i), future, future).Error)
	}
	store := NewCraftRunCaptureStore(db)
	for i := 0; i < 3; i++ {
		rows, err := store.ClaimPromotionBatch(context.Background(), 32, time.Minute)
		require.NoError(t, err)
		require.Empty(t, rows)
	}
	var cursor craftRunCapturePromotionCursorRow
	require.NoError(t, db.Where("id = 1").Take(&cursor).Error)
	require.NotNil(t, cursor.RunID)
	require.Equal(t, "plan-run-095", *cursor.RunID)
	rows, err := store.ClaimPromotionBatch(context.Background(), 32, time.Minute)
	require.NoError(t, err)
	require.Empty(t, rows)
	cursor = craftRunCapturePromotionCursorRow{}
	require.NoError(t, db.Where("id = 1").Take(&cursor).Error)
	require.Nil(t, cursor.UpdatedAt, "an empty raw page at the tail resets the keyset cursor")
	require.NoError(t, db.Exec(`DELETE FROM craft_run_capture_promotion_attempts WHERE tenant_id=1 AND workspace_id='plan-workspace' AND run_id='plan-run-000'`).Error)
	rows, err = store.ClaimPromotionBatch(context.Background(), 32, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "plan-run-000", rows[0].RunID, "older newly eligible work is claimable after wrap")
}

func seedPromotionPlanReceipts(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA foreign_keys=OFF").Error)
	for i := 0; i < 96; i++ {
		state := "sealed"
		if i%2 == 1 {
			state = "advanced"
		}
		stamp := time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC)
		require.NoError(t, db.Exec(`INSERT INTO craft_run_captures
			(tenant_id,workspace_id,run_id,owner_id,session_id,generation,predecessor_revision,predecessor_state,
			 predecessor_run_id,predecessor_digest,state,draft_revision,updated_at)
			VALUES (1,'plan-workspace',?,'u1','s1','g1',0,'empty','','',?,?,?)`,
			fmt.Sprintf("plan-run-%03d", i), state, i+1, stamp).Error)
	}
}

func TestCraftRunCapturePromotionPostgresMigrationAndClaimSerialization(t *testing.T) {
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL promotion migration/claim serialization NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "craft_promotion_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error) })
	testDSN, err := postgresDSNWithSchema(dsn, schema)
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(testDSN), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE craft_run_captures (
	 tenant_id BIGINT NOT NULL, workspace_id VARCHAR(64) NOT NULL, run_id VARCHAR(64) NOT NULL,
	 owner_id VARCHAR(512) NOT NULL, session_id VARCHAR(36) NOT NULL, state VARCHAR(16) NOT NULL,
	 draft_revision BIGINT NULL, updated_at TIMESTAMPTZ NOT NULL, PRIMARY KEY(tenant_id,workspace_id,run_id))`).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	root := promotionRepoRoot(t)
	require.NoError(t, migratePromotionPostgres(t, sqlDB, root, schema))
	require.True(t, db.Migrator().HasIndex("craft_run_captures", "idx_craft_run_captures_promotion_scan"))

	// The production cursor lock must serialize simultaneous global and targeted claims.
	require.NoError(t, db.Exec(`INSERT INTO craft_run_captures VALUES (1,'w1','r1','u1','s1','sealed',1,CURRENT_TIMESTAMP)`).Error)
	store1, store2 := NewCraftRunCaptureStore(db), NewCraftRunCaptureStore(reopenPromotionPostgresDB(t, testDSN))
	start := make(chan struct{})
	results := make(chan int, 2)
	errs := make(chan error, 2)
	go func() {
		<-start
		rows, e := store1.ClaimPromotionBatch(context.Background(), 8, time.Minute)
		results <- len(rows)
		errs <- e
	}()
	go func() {
		<-start
		rows, e := store2.ClaimPromotionForRun(context.Background(), 1, "r1", time.Minute)
		results <- len(rows)
		errs <- e
	}()
	close(start)
	first, second := <-results, <-results
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, 1, first+second)
}

func TestPostgresDSNWithSchemaSupportsURLAndKeywordFormats(t *testing.T) {
	for _, tc := range []struct {
		name, dsn string
	}{
		{name: "url", dsn: "postgres://tester:secret@db.example/test?sslmode=disable"},
		{name: "keyword", dsn: "host=db.example user=tester password=secret dbname=test sslmode=disable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := postgresDSNWithSchema(tc.dsn, "isolated")
			require.NoError(t, err)
			if tc.name == "url" {
				parsed, parseErr := url.Parse(got)
				require.NoError(t, parseErr)
				require.Equal(t, "isolated,public", parsed.Query().Get("search_path"))
			} else {
				require.Contains(t, got, "search_path=isolated,public")
			}
		})
	}
}

func promotionRepoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
}

func explainPromotionFreshScanAtCursor(t *testing.T, db *gorm.DB, cursor craftRunCapturePromotionCursorRow) string {
	t.Helper()
	query := db.Session(&gorm.Session{DryRun: true}).Table("craft_run_captures AS c").
		Select("c.tenant_id, c.workspace_id, c.run_id, c.owner_id, c.session_id, c.state, c.draft_revision, c.updated_at").
		Where("c.state IN ('sealed','advanced') AND c.draft_revision IS NOT NULL")
	query = freshPromotionPageQuery(query, cursor, 8)
	var projection []craftPromotionReceiptRow
	generated := query.Find(&projection).Statement
	var rows []struct{ Detail string }
	generatedSQL := generated.SQL.String()
	require.NotEmpty(t, generatedSQL)
	require.NoError(t, db.Raw("EXPLAIN QUERY PLAN "+generatedSQL, generated.Vars...).Scan(&rows).Error)
	var details []string
	for _, row := range rows {
		details = append(details, row.Detail)
	}
	return strings.Join(details, "\n")
}

func postgresDSNWithSchema(dsn, schema string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		query := parsed.Query()
		query.Set("search_path", schema+",public")
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
	return dsn + " search_path=" + schema + ",public", nil
}

func ptr[T any](value T) *T { return &value }

func migratePromotionPostgres(t *testing.T, sqlDB *sql.DB, root, schema string) error {
	t.Helper()
	dir := t.TempDir()
	for _, direction := range []string{"up", "down"} {
		contents, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000216_craft_run_capture_promotion_scan."+direction+".sql"))
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, "000001_promotion."+direction+".sql"), contents, 0o600); err != nil {
			return err
		}
	}
	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{SchemaName: schema})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+dir, "postgres", driver)
	if err != nil {
		return err
	}
	t.Cleanup(func() { _, _ = m.Close() })
	if err = m.Up(); err != nil {
		return err
	}
	return nil
}

func reopenPromotionPostgresDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return db
}
