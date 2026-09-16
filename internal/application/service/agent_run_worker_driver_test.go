package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestAgentRunWorkerTickSkipsPaseo uses the real AgentRunStore and worker
// entry point. The legacy worker must receive only platform rows when both
// drivers have queued work.
func TestAgentRunWorkerTickSkipsPaseo(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	dbPath := filepath.Join(t.TempDir(), "worker-driver.db")
	require.NoError(t, database.RunMigrationsWithOptions("sqlite3://"+dbPath, database.MigrationOptions{
		SQLiteDBPath: dbPath,
	}))
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	seedWorkerDriverFixtures(t, db)

	store := repository.NewAgentRunStore(db)
	ctx := context.Background()
	platform := workerDriverAdmission("platform-run", "s1", "platform-request", "platform-assistant", "")
	paseo := workerDriverAdmission("paseo-run", "s2", "paseo-request", "paseo-assistant", "paseo")
	_, err = store.Admit(ctx, platform)
	require.NoError(t, err)
	_, err = store.Admit(ctx, paseo)
	require.NoError(t, err)

	type execution struct {
		fence agentruntime.Fence
		runID string
	}
	executed := make(chan execution, 1)
	worker, err := NewAgentRunWorker(store, func(ctx context.Context, fence agentruntime.Fence) error {
		runID, ok := types.RunIDFromContext(ctx)
		if !ok {
			return errors.New("worker did not install durable run id")
		}
		executed <- execution{fence: fence, runID: runID}
		return nil
	}, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: time.Second, ScanInterval: time.Second, MaxWorkers: 2})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(ctx))

	select {
	case got := <-executed:
		require.Equal(t, platform.Key, got.fence.RunKey)
		require.Equal(t, platform.Key.RunID, got.runID)
		require.NotEqual(t, platform.RequestID, got.runID)
	case <-time.After(time.Second):
		t.Fatal("platform worker did not execute its platform run")
	}
	remote, err := store.Get(ctx, paseo.Key)
	require.NoError(t, err)
	require.Equal(t, "queued", remote.Status)
}

func seedWorkerDriverFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (1, 'worker-driver', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id)
		VALUES ('u1', 'u1', 'u1@example.test', 'x', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		('s1', 1, 'platform', 'u1', 'trpc'),
		('s2', 1, 'paseo', 'u1', 'builtin')`).Error)
}

func workerDriverAdmission(runID, sessionID, requestID, assistantID, driver string) agentruntime.Admission {
	return agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          sessionID,
		UserID:             "u1",
		RequestID:          requestID,
		AssistantMessageID: assistantID,
		Driver:             driver,
		RequestHash:        "hash-" + runID,
		Snapshot:           json.RawMessage(`{"version":1}`),
		UserMessage:        json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(time.Hour),
	}
}
