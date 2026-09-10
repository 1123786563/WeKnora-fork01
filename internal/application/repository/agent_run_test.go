package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	if strings.Contains(t.Name(), "/postgres") {
		return openPostgresRunTestDB(t, repoRoot)
	}
	dbPath := filepath.Join(t.TempDir(), "agent-runs.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	seedRunFixtures(t, db)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

func seedRunFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, username, email, password_hash, tenant_id)
		 VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('s1', 1, 'session-1', 'u1', 'trpc'),
		 ('s2', 1, 'session-2', 'u1', 'trpc')`,
	).Error)
}

func openPostgresRunTestDB(t *testing.T, root string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL recovery acceptance NOT VERIFIED")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "trpc_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		conn, e := admin.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, e := url.Parse(dsn)
		require.NoError(t, e)
		q := u.Query()
		q.Set("search_path", schema+",public")
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema + ",public"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	driver, err := pgmigrate.WithInstance(conn, &pgmigrate.Config{SchemaName: schema})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/versioned"), "postgres", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	t.Cleanup(func() { _, _ = m.Close() })
	seedRunFixtures(t, db)
	return db
}

func reopenRunDB(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	var dialector gorm.Dialector
	if db.Name() == "sqlite" {
		dialector = sqlite.Open(db.Dialector.(*sqlite.Dialector).DSN)
	} else {
		dialector = postgres.Open(db.Dialector.(*postgres.Dialector).DSN)
	}
	other, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := other.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return other
}

func testAdmission() agentruntime.Admission {
	return agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: "r1"},
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "q1",
		AssistantMessageID: "a1",
		RequestHash:        "h1",
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(time.Hour),
	}
}

func TestAgentRunAdmissionIdempotent(t *testing.T) {
	store := NewAgentRunStore(openRunTestDB(t))
	in := testAdmission()

	first, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	in.Key.RunID = "r2"
	again, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, first.Key, again.Key)
	var assistantID string
	require.NoError(t, store.db.Table("messages").Where("role = 'assistant'").Pluck("id", &assistantID).Error)
	require.Equal(t, "a1", assistantID)
}

func TestAgentRunAdmissionGuards(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	in := testAdmission()
	_, err := store.Admit(ctx, in)
	require.NoError(t, err)
	in.RequestHash = "different"
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	in = testAdmission()
	in.RequestID, in.Key.RunID = "q2", "r2"
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrRunActive)
	require.NoError(t, db.Exec("UPDATE agent_runs SET status = 'waiting_user'").Error)
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrRunActive)
	in = testAdmission()
	in.UserID = "other"
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	in = testAdmission()
	in.Key.TenantID = 2
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	in = testAdmission()
	in.SessionID = "s2"
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	in.RequestID = "q2"
	require.NoError(t, db.Exec("UPDATE sessions SET engine_type = 'builtin' WHERE id = 's2'").Error)
	_, err = store.Admit(ctx, in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var count int64
	require.NoError(t, db.Table("messages").Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestAgentRunAdmissionRollback(t *testing.T) {
	db := openRunTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO messages (id, request_id, session_id, role, content)
		VALUES ('a1', 'old', 's2', 'assistant', '')`).Error)
	_, err := NewAgentRunStore(db).Admit(context.Background(), testAdmission())
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Table("agent_runs").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("messages").Count(&count).Error)
	require.EqualValues(t, 1, count)
	var slot *string
	require.NoError(t, db.Raw("SELECT active_agent_run_id FROM sessions WHERE id = 's1'").Scan(&slot).Error)
	require.Nil(t, slot)
}

func TestAgentRunLeaseAndCheckpoint(t *testing.T) {
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	ctx := context.Background()
	in := testAdmission()
	_, err := store.Admit(ctx, in)
	require.NoError(t, err)
	loaded, err := store.Get(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, "queued", loaded.Status)
	_, err = store.Get(ctx, agentruntime.RunKey{TenantID: 2, RunID: in.Key.RunID})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	keys, err := store.Scan(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []agentruntime.RunKey{in.Key}, keys)
	fence, err := store.Claim(ctx, in.Key, "worker-1", time.Minute)
	require.NoError(t, err)
	require.EqualValues(t, 1, fence.Epoch)
	_, err = store.Claim(ctx, in.Key, "worker-2", time.Minute)
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.NoError(t, store.Renew(ctx, fence, time.Minute))
	keys, err = store.Scan(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, keys)
	cp := agentruntime.CheckpointRecord{
		Namespace: "1/r1/1", ID: "cp1", Seq: 1,
		State:         json.RawMessage(`{"version":1,"messages":["hello"]}`),
		PendingWrites: json.RawMessage(`[{"node":"tool"}]`),
	}
	require.NoError(t, store.SaveCheckpoint(ctx, fence, cp))
	got, err := store.LoadCheckpoint(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, cp, got)
	_, err = store.LoadCheckpoint(ctx, agentruntime.RunKey{TenantID: 2, RunID: "r1"})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	require.NoError(t, db.Exec("UPDATE agent_runs SET lease_until = ?", time.Now().Add(-time.Hour)).Error)
	require.ErrorIs(t, store.Renew(ctx, fence, time.Minute), agentruntime.ErrLeaseLost)
	require.ErrorIs(t, store.SaveCheckpoint(ctx, fence, cp), agentruntime.ErrLeaseLost)
	newFence, err := store.Claim(ctx, in.Key, "worker-2", time.Minute)
	require.NoError(t, err)
	require.EqualValues(t, 2, newFence.Epoch)
	require.ErrorIs(t, store.SaveCheckpoint(ctx, fence, cp), agentruntime.ErrLeaseLost)
	cp.ID, cp.ParentID, cp.Seq = "cp2", "cp1", 2
	require.NoError(t, store.SaveCheckpoint(ctx, newFence, cp))
	got, err = store.LoadCheckpoint(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, cp, got)
}

func TestAgentRunConcurrentClaim(t *testing.T) {
	db := openRunTestDB(t)
	other := reopenRunDB(t, db)
	store := NewAgentRunStore(db)
	_, err := store.Admit(context.Background(), testAdmission())
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	start := make(chan struct{})
	for i, instance := range []*AgentRunStore{store, NewAgentRunStore(other)} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = instance.Claim(context.Background(), testAdmission().Key, "worker", time.Minute)
		}()
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, e := range errs {
		if e == nil {
			wins++
		} else {
			require.ErrorIs(t, e, agentruntime.ErrLeaseLost)
		}
	}
	require.Equal(t, 1, wins)
}

func TestAgentRunReopenAndMigrations(t *testing.T) {
	db := openRunTestDB(t)
	ctx := context.Background()
	store := NewAgentRunStore(db)
	in := testAdmission()
	_, err := store.Admit(ctx, in)
	require.NoError(t, err)
	fence, err := store.Claim(ctx, in.Key, "owner", time.Minute)
	require.NoError(t, err)
	cp := agentruntime.CheckpointRecord{
		Namespace: "graph", ID: "cp", Seq: 1,
		State: json.RawMessage(`{"version":1}`), PendingWrites: json.RawMessage(`[]`),
	}
	require.NoError(t, store.SaveCheckpoint(ctx, fence, cp))
	other := reopenRunDB(t, db)
	got, err := NewAgentRunStore(other).LoadCheckpoint(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, cp, got)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dir, version, suffix := "sqlite", "000014", "agent_runs"
	if db.Name() == "postgres" {
		dir, version, suffix = "versioned", "000093", "agent_runs"
	}
	for _, direction := range []string{"down", "up"} {
		script, e := os.ReadFile(filepath.Join(root, "migrations", dir, version+"_"+suffix+"."+direction+".sql"))
		require.NoError(t, e)
		require.NoError(t, db.Exec(string(script)).Error)
	}
	var engine string
	require.NoError(t, db.Table("sessions").Where("id = 's1'").Pluck("engine_type", &engine).Error)
	require.Equal(t, "builtin", engine)
	for _, index := range []struct{ table, name string }{
		{"agent_runs", "uq_agent_runs_request"},
		{"agent_runs", "idx_agent_runs_recovery_scan"},
		{"sessions", "uq_sessions_tenant_id_id"},
		{"agent_run_decisions", "uq_agent_run_decisions_applied_pending"},
	} {
		require.True(t, db.Migrator().HasIndex(index.table, index.name), index.name)
	}
}

func TestAgentRunPostgres(t *testing.T) {
	if os.Getenv("TRPC_TEST_POSTGRES_DSN") == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL recovery acceptance NOT VERIFIED")
	}
	for _, scenario := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"idempotent", TestAgentRunAdmissionIdempotent},
		{"guards", TestAgentRunAdmissionGuards},
		{"rollback", TestAgentRunAdmissionRollback},
		{"lease_checkpoint", TestAgentRunLeaseAndCheckpoint},
		{"claim_race", TestAgentRunConcurrentClaim},
		{"reopen_migrations", TestAgentRunReopenAndMigrations},
	} {
		t.Run("postgres/"+scenario.name, scenario.run)
	}
}
