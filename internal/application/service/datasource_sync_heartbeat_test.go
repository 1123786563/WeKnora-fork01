package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sqliteSyncHeartbeatFixture mirrors datasource_delete_sqlite_test.go: an
// in-file SQLite database holding the two rows the heartbeat path touches, so
// every heartbeat write is observable straight from the persisted sync log.
type sqliteSyncHeartbeatFixture struct {
	db          *gorm.DB
	dsRepo      interfaces.DataSourceRepository
	syncLogRepo interfaces.SyncLogRepository
	svc         *DataSourceService
	ds          *types.DataSource
	syncLog     *types.SyncLog
}

func newSQLiteSyncHeartbeatFixture(t *testing.T) *sqliteSyncHeartbeatFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "weknora.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.DataSource{}, &types.SyncLog{}))

	dsRepo := repository.NewDataSourceRepository(db)
	syncLogRepo := repository.NewSyncLogRepository(db)
	ds := &types.DataSource{
		ID:              "ds-sync-heartbeat",
		TenantID:        1,
		KnowledgeBaseID: "kb-sync-heartbeat",
		Name:            "SQLite heartbeat",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
	}
	syncLog := &types.SyncLog{
		ID:           "log-sync-heartbeat",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
	}
	require.NoError(t, dsRepo.Create(context.Background(), ds))
	require.NoError(t, syncLogRepo.Create(context.Background(), syncLog))

	return &sqliteSyncHeartbeatFixture{
		db:          db,
		dsRepo:      dsRepo,
		syncLogRepo: syncLogRepo,
		svc:         &DataSourceService{dsRepo: dsRepo, syncLogRepo: syncLogRepo},
		ds:          ds,
		syncLog:     syncLog,
	}
}

// heartbeatSentinel overwrites the stored heartbeat_at with a fixed past
// timestamp so a later (unwanted) write is detectable: any value other than the
// sentinel means maybeHeartbeat hit the database again.
func (f *sqliteSyncHeartbeatFixture) heartbeatSentinel(t *testing.T, at time.Time) {
	t.Helper()
	require.NoError(t, f.db.Exec("UPDATE sync_logs SET heartbeat_at = ?", at).Error)
}

func (f *sqliteSyncHeartbeatFixture) heartbeatFromDB(t *testing.T) *time.Time {
	t.Helper()
	log, err := f.syncLogRepo.FindByID(context.Background(), f.syncLog.ID)
	require.NoError(t, err)
	return log.HeartbeatAt
}

// TestSyncHeartbeatMaybeHeartbeatThrottleWindow covers the three states of the
// in-memory throttle: the first call writes immediately, an in-window call is
// skipped, and a call past the window writes again.
func TestSyncHeartbeatMaybeHeartbeatThrottleWindow(t *testing.T) {
	fixture := newSQLiteSyncHeartbeatFixture(t)
	ctx := context.Background()

	// State 1: first call writes (zero lastBeat means "never beaten").
	var lastBeat time.Time
	maybeHeartbeat(ctx, fixture.syncLogRepo, fixture.syncLog.ID, &lastBeat)
	assert.False(t, lastBeat.IsZero(), "first call must set lastBeat")
	beat := fixture.heartbeatFromDB(t)
	require.NotNil(t, beat, "first call must persist heartbeat_at")

	// State 2: within the throttle window nothing is written.
	sentinel := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	fixture.heartbeatSentinel(t, sentinel)
	maybeHeartbeat(ctx, fixture.syncLogRepo, fixture.syncLog.ID, &lastBeat)
	beat = fixture.heartbeatFromDB(t)
	require.NotNil(t, beat)
	assert.True(t, sentinel.Equal(*beat), "in-window call must not touch heartbeat_at")

	// State 3: past the window the next call writes again.
	rewound := lastBeat.Add(-syncHeartbeatInterval - time.Second)
	lastBeat = rewound
	maybeHeartbeat(ctx, fixture.syncLogRepo, fixture.syncLog.ID, &lastBeat)
	beat = fixture.heartbeatFromDB(t)
	require.NotNil(t, beat)
	assert.True(t, beat.After(sentinel), "out-of-window call must refresh heartbeat_at")
	assert.True(t, lastBeat.After(rewound), "out-of-window beat must advance lastBeat")
}

// TestSyncHeartbeatCheckpointPersistsHeartbeat verifies the streaming-path
// integration point: Checkpoint stamps heartbeat_at alongside its progress
// UpdateResult so stall detection sees page-boundary liveness.
func TestSyncHeartbeatCheckpointPersistsHeartbeat(t *testing.T) {
	fixture := newSQLiteSyncHeartbeatFixture(t)
	ctx := context.Background()

	handler := &streamSyncHandler{
		svc:     fixture.svc,
		ds:      fixture.ds,
		result:  &types.SyncResult{Total: 3, Created: 3},
		syncLog: fixture.syncLog,
	}
	cursor := &types.SyncCursor{LastSyncTime: time.Now().UTC()}
	require.NoError(t, handler.Checkpoint(ctx, cursor))
	require.NoError(t, handler.Checkpoint(ctx, cursor))

	beat := fixture.heartbeatFromDB(t)
	require.NotNil(t, beat, "Checkpoint must persist heartbeat_at")
	assert.False(t, beat.After(time.Now().UTC().Add(time.Minute)), "heartbeat must not be in the future")

	// The progress mirror rides along on the same checkpoint write.
	log, err := fixture.syncLogRepo.FindByID(ctx, fixture.syncLog.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, log.ItemsTotal)
	assert.Equal(t, 3, log.ItemsCreated)
}
