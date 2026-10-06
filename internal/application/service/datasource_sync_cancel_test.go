package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sqliteSyncCancelFixture mirrors datasource_sync_heartbeat_test.go: a real
// SQLite database behind the real repositories, so CancelSyncLog's ownership
// checks and RequestCancel's running-only predicate run against actual SQL.
type sqliteSyncCancelFixture struct {
	db          *gorm.DB
	dsRepo      interfaces.DataSourceRepository
	syncLogRepo interfaces.SyncLogRepository
	svc         *DataSourceService
	ds          *types.DataSource
	otherDS     *types.DataSource
	syncLog     *types.SyncLog
	doneLog     *types.SyncLog
}

func newSQLiteSyncCancelFixture(t *testing.T) *sqliteSyncCancelFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "weknora.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.DataSource{}, &types.SyncLog{}))

	dsRepo := repository.NewDataSourceRepository(db)
	syncLogRepo := repository.NewSyncLogRepository(db)
	ds := &types.DataSource{
		ID:              "ds-cancel",
		TenantID:        7,
		KnowledgeBaseID: "kb-cancel",
		Name:            "Cancel target",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
	}
	otherDS := &types.DataSource{
		ID:              "ds-other",
		TenantID:        7,
		KnowledgeBaseID: "kb-cancel",
		Name:            "Other source",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
	}
	syncLog := &types.SyncLog{
		ID:           "log-cancel-running",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	doneLog := &types.SyncLog{
		ID:           "log-cancel-done",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusSuccess,
		StartedAt:    time.Now().UTC(),
	}
	require.NoError(t, dsRepo.Create(context.Background(), ds))
	require.NoError(t, dsRepo.Create(context.Background(), otherDS))
	require.NoError(t, syncLogRepo.Create(context.Background(), syncLog))
	require.NoError(t, syncLogRepo.Create(context.Background(), doneLog))

	return &sqliteSyncCancelFixture{
		db:          db,
		dsRepo:      dsRepo,
		syncLogRepo: syncLogRepo,
		svc:         &DataSourceService{dsRepo: dsRepo, syncLogRepo: syncLogRepo},
		ds:          ds,
		otherDS:     otherDS,
		syncLog:     syncLog,
		doneLog:     doneLog,
	}
}

func (f *sqliteSyncCancelFixture) reloadLog(t *testing.T, id string) *types.SyncLog {
	t.Helper()
	log, err := f.syncLogRepo.FindByID(context.Background(), id)
	require.NoError(t, err)
	return log
}

// The API entry point: flag a running log owned by (tenant, data source).
func TestCancelSyncLog_FlagsRunningLogWithinTenantAndDataSource(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)

	require.NoError(t, f.svc.CancelSyncLog(context.Background(), 7, f.ds.ID, f.syncLog.ID))

	reloaded := f.reloadLog(t, f.syncLog.ID)
	assert.True(t, reloaded.CancelRequested, "CancelSyncLog must set cancel_requested")
	assert.Equal(t, types.SyncLogStatusRunning, reloaded.Status,
		"the flag alone must not change status; the sync loop owns the terminal transition")
}

// Ownership and state: a log foreign to the URL's data source, a foreign
// tenant, a missing id and an already-terminal log all return
// ErrSyncLogNotFound uniformly (no information leak about other tenants' logs)
// and none of them ever sets the flag.
func TestCancelSyncLog_RejectsForeignOrTerminalLogs(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	ctx := context.Background()

	assert.ErrorIs(t, f.svc.CancelSyncLog(ctx, 7, f.otherDS.ID, f.syncLog.ID), ErrSyncLogNotFound,
		"a log of another data source must not be cancelable through this URL")
	assert.ErrorIs(t, f.svc.CancelSyncLog(ctx, 8, f.ds.ID, f.syncLog.ID), ErrSyncLogNotFound,
		"a foreign tenant must not cancel the log")
	assert.ErrorIs(t, f.svc.CancelSyncLog(ctx, 7, f.ds.ID, "log-missing"), ErrSyncLogNotFound)
	assert.ErrorIs(t, f.svc.CancelSyncLog(ctx, 7, f.ds.ID, f.doneLog.ID), ErrSyncLogNotFound,
		"an already-terminal log cannot be canceled")

	assert.False(t, f.reloadLog(t, f.syncLog.ID).CancelRequested)
	assert.False(t, f.reloadLog(t, f.doneLog.ID).CancelRequested,
		"RequestCancel must remain a no-op on non-running rows")
}

// The consumption point on the streaming path: a checkpoint after the flag was
// set ends the run — the sentinel is returned, the log goes terminal with
// "canceled by user", the live progress counts are kept, and the cursor stays
// persisted at exactly this checkpoint so a later run resumes instead of
// restarting.
func TestCheckpoint_CancelFlagEndsRunWithCursorPreserved(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	ctx := context.Background()
	require.NoError(t, f.svc.CancelSyncLog(ctx, 7, f.ds.ID, f.syncLog.ID))

	handler := &streamSyncHandler{
		svc:     f.svc,
		ds:      f.ds,
		syncLog: f.syncLog,
		result:  &types.SyncResult{Total: 4, Created: 3, Skipped: 1},
	}
	cursor := &types.SyncCursor{
		LastSyncTime:    time.Now().UTC(),
		ConnectorCursor: map[string]interface{}{"space_node_times": map[string]string{"nt1": "100"}},
	}
	err := handler.Checkpoint(ctx, cursor)
	require.ErrorIs(t, err, errSyncCanceled)

	reloaded := f.reloadLog(t, f.syncLog.ID)
	assert.Equal(t, types.SyncLogStatusCanceled, reloaded.Status)
	assert.Equal(t, "canceled by user", reloaded.ErrorMessage)
	require.NotNil(t, reloaded.FinishedAt)
	assert.Equal(t, 4, reloaded.ItemsTotal, "progress counts must survive into the canceled log")
	assert.Equal(t, 3, reloaded.ItemsCreated)
	require.NotNil(t, reloaded.HeartbeatAt, "the cancel-detecting pulse must also stamp the heartbeat")

	ds, err := f.dsRepo.FindByID(ctx, f.ds.ID)
	require.NoError(t, err)
	cursorJSON, cerr := cursor.ToJSON() // after Checkpoint mutated in the fence stamp
	require.NoError(t, cerr)
	assert.Equal(t, string(cursorJSON), string(ds.LastSyncCursor),
		"the checkpoint cursor must stay persisted for resume")
}

// The cancel check rides the heartbeat throttle: within the 30s window a
// checkpoint does not re-query, so a flag raised mid-run lands at most one
// window plus one checkpoint later ("~30s + 1 checkpoint" response contract).
func TestCheckpoint_CancelCheckRidesHeartbeatThrottle(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	ctx := context.Background()

	handler := &streamSyncHandler{svc: f.svc, ds: f.ds, syncLog: f.syncLog, result: &types.SyncResult{}}
	cursor := &types.SyncCursor{ConnectorCursor: map[string]interface{}{"phase": "one"}}
	require.NoError(t, handler.Checkpoint(ctx, cursor), "a clean first checkpoint must pulse and pass")

	// Flag raised right after: the very next checkpoint is inside the throttle
	// window and must not observe it yet.
	require.NoError(t, f.svc.CancelSyncLog(ctx, 7, f.ds.ID, f.syncLog.ID))
	require.NoError(t, handler.Checkpoint(ctx, cursor))
	assert.Equal(t, types.SyncLogStatusRunning, f.reloadLog(t, f.syncLog.ID).Status,
		"in-window checkpoint must not act on the cancel flag")

	// Past the window the next checkpoint observes the flag and ends the run.
	handler.lastBeat = handler.lastBeat.Add(-syncHeartbeatInterval - time.Second)
	require.ErrorIs(t, handler.Checkpoint(ctx, cursor), errSyncCanceled)
	assert.Equal(t, types.SyncLogStatusCanceled, f.reloadLog(t, f.syncLog.ID).Status)
}

func TestCheckCancelRequested_ReadsFlagFromRepository(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	ctx := context.Background()

	assert.False(t, checkCancelRequested(ctx, f.syncLogRepo, f.syncLog.ID))
	require.NoError(t, f.svc.CancelSyncLog(ctx, 7, f.ds.ID, f.syncLog.ID))
	assert.True(t, checkCancelRequested(ctx, f.syncLogRepo, f.syncLog.ID))
	assert.False(t, checkCancelRequested(ctx, f.syncLogRepo, "log-missing"),
		"a missing row must read as not canceled, never abort the sync")
}

const cancelStreamConnectorType = "test-sync-cancel-stream"

// cancelStreamConnector checkpoints immediately (like a one-document page) and
// propagates the handler's error verbatim, matching Confluence/GitLab.
type cancelStreamConnector struct{}

func (cancelStreamConnector) Type() string { return cancelStreamConnectorType }

func (cancelStreamConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}

func (cancelStreamConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}

func (cancelStreamConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}

func (cancelStreamConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	return nil, nil
}

func (cancelStreamConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return nil, nil, nil
}

func (cancelStreamConnector) FetchStream(
	ctx context.Context, _ *types.DataSourceConfig, _ *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	cursor := &types.SyncCursor{ConnectorCursor: map[string]interface{}{"phase": "cancel-boundary"}}
	if err := h.Checkpoint(ctx, cursor); err != nil {
		return nil, err
	}
	return cursor, nil
}

var _ datasource.StreamingConnector = cancelStreamConnector{}

// End to end on the streaming path: a user cancel observed at a checkpoint
// makes ProcessSync return nil (asynq must not retry a run the user stopped)
// and leaves the log canceled with the checkpoint cursor kept.
func TestProcessSyncStreaming_UserCancelIsGracefulSuccess(t *testing.T) {
	ds := &types.DataSource{
		ID:              "ds-stream-cancel",
		TenantID:        1,
		KnowledgeBaseID: "kb-1",
		Name:            "Stream cancel",
		Type:            cancelStreamConnectorType,
		Status:          types.DataSourceStatusActive,
	}
	configJSON, err := (&types.DataSourceConfig{Type: cancelStreamConnectorType}).ToJSON()
	require.NoError(t, err)
	ds.Config = configJSON
	syncLog := &types.SyncLog{
		ID: "log-stream-cancel", DataSourceID: ds.ID, TenantID: ds.TenantID,
		Status: types.SyncLogStatusRunning, StartedAt: time.Now().UTC(),
	}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}
	dsRepo := &recordingDSRepo{kbDeleteDSRepo: *newKBDeleteDSRepo(ds.KnowledgeBaseID, ds)}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(cancelStreamConnector{}))

	svc := &DataSourceService{
		dsRepo:            dsRepo,
		syncLogRepo:       syncLogRepo,
		kbService:         &processSyncKBService{kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}},
		connectorRegistry: registry,
		tenantRepo:        &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
		tagService:        &processSyncTagService{},
	}

	require.NoError(t, svc.CancelSyncLog(context.Background(), ds.TenantID, ds.ID, syncLog.ID))

	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: ds.ID, TenantID: ds.TenantID, SyncLogID: syncLog.ID,
	})
	require.NoError(t, err)
	require.NoError(t, svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload)),
		"a user cancel must not surface as an asynq failure")

	updated := syncLogRepo.logs[syncLog.ID]
	assert.Equal(t, types.SyncLogStatusCanceled, updated.Status)
	assert.Equal(t, "canceled by user", updated.ErrorMessage)
	require.NotNil(t, updated.FinishedAt)
	require.Len(t, dsRepo.updated, 1, "the cancel-boundary checkpoint must persist the cursor exactly once")
	assert.NotEmpty(t, dsRepo.updated[0].LastSyncCursor)
}

const cancelBatchConnectorType = "test-sync-cancel-batch"

// cancelBatchConnector serves three deleted items so the batch loop's ingest
// path stays on the proven deletion harness wiring.
type cancelBatchConnector struct{}

func (cancelBatchConnector) Type() string { return cancelBatchConnectorType }

func (cancelBatchConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}

func (cancelBatchConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}

func (cancelBatchConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}

func (cancelBatchConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	return []types.FetchedItem{
		{ExternalID: "file:one", SourceResourceID: "folder:1", IsDeleted: true},
		{ExternalID: "file:two", SourceResourceID: "folder:1", IsDeleted: true},
		{ExternalID: "file:three", SourceResourceID: "folder:1", IsDeleted: true},
	}, nil
}

func (cancelBatchConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	items, err := (cancelBatchConnector{}).FetchAll(context.Background(), nil, nil)
	return items, nil, err
}

// End to end on the batch (non-streaming) path: the cancel pulse at the first
// item boundary ends the run gracefully — ProcessSync returns nil, the log goes
// canceled with "canceled by user" — and the data source's sync state (cursor,
// last_sync_at) is never touched because the batch path only persists those on
// completion.
func TestProcessSyncBatch_UserCancelAtItemBoundary(t *testing.T) {
	ds := &types.DataSource{
		ID:              "ds-batch-cancel",
		TenantID:        1,
		KnowledgeBaseID: "kb-1",
		Name:            "Batch cancel",
		Type:            cancelBatchConnectorType,
		SyncMode:        types.SyncModeFull,
		Status:          types.DataSourceStatusActive,
		SyncDeletions:   true,
	}
	configJSON, err := (&types.DataSourceConfig{Type: cancelBatchConnectorType}).ToJSON()
	require.NoError(t, err)
	ds.Config = configJSON
	syncLog := &types.SyncLog{
		ID: "log-batch-cancel", DataSourceID: ds.ID, TenantID: ds.TenantID,
		Status: types.SyncLogStatusRunning, StartedAt: time.Now().UTC(),
	}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}
	dsRepo := &recordingDSRepo{kbDeleteDSRepo: *newKBDeleteDSRepo(ds.KnowledgeBaseID, ds)}
	knowledgeRepo := &deletionLookupKnowledgeRepo{knowledge: &types.Knowledge{ID: "knowledge-gone"}}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(cancelBatchConnector{}))

	svc := &DataSourceService{
		dsRepo:            dsRepo,
		syncLogRepo:       syncLogRepo,
		knowledgeService:  &sweepFakeKS{repo: knowledgeRepo},
		kbService:         &processSyncKBService{kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}},
		connectorRegistry: registry,
		tenantRepo:        &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
		tagService:        &processSyncTagService{},
	}

	require.NoError(t, svc.CancelSyncLog(context.Background(), ds.TenantID, ds.ID, syncLog.ID))

	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: ds.ID, TenantID: ds.TenantID, SyncLogID: syncLog.ID, ForceFull: true,
	})
	require.NoError(t, err)
	require.NoError(t, svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload)),
		"a user cancel must not surface as an asynq failure")

	updated := syncLogRepo.logs[syncLog.ID]
	assert.Equal(t, types.SyncLogStatusCanceled, updated.Status)
	assert.Equal(t, "canceled by user", updated.ErrorMessage)
	require.NotNil(t, updated.FinishedAt)
	assert.Equal(t, 1, updated.ItemsDeleted,
		"the item processed before the pulse must be counted; the rest never run")
	assert.Empty(t, dsRepo.updated,
		"a canceled batch run must not persist sync state: the cursor stays for the next run")
}
