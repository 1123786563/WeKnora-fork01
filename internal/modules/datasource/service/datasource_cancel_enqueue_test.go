package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────────────
// Fakes (SP2-a Task 5): a recording TaskInspector plus a recording
// sync-log repo wrapper so tests can assert both the inspector call
// arguments and the ordering against the durable sync-log sweep.
// ──────────────────────────────────────────────────────────────────────

// cancelOrderRecorder captures cross-component call order.
type cancelOrderRecorder struct {
	events []string
}

func (r *cancelOrderRecorder) record(event string) {
	r.events = append(r.events, event)
}

func (r *cancelOrderRecorder) indexOf(event string) int {
	for i, e := range r.events {
		if e == event {
			return i
		}
	}
	return -1
}

// fakeCancelTaskInspector implements TaskInspector + KnowledgeBaseTaskCanceller
// (the same surface the asynq production inspector exposes) and records every
// CancelTasksForKnowledgeBase call.
type fakeCancelTaskInspector struct {
	recorder *cancelOrderRecorder
	calls    []fakeCancelCall
}

type fakeCancelCall struct {
	kbID          string
	knowledgeIDs  []string
	dataSourceIDs []string
}

func (f *fakeCancelTaskInspector) CancelTasksForKnowledge(context.Context, string) (int, int, error) {
	return 0, 0, nil
}

func (f *fakeCancelTaskInspector) HasQueuedTasksForKnowledge(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeCancelTaskInspector) QueueStats(context.Context) ([]types.QueueStat, bool, error) {
	return nil, false, nil
}

func (f *fakeCancelTaskInspector) WorkerServerStats(context.Context) ([]types.WorkerServerStat, bool, error) {
	return nil, false, nil
}

func (f *fakeCancelTaskInspector) CancelTasksForKnowledgeBase(
	_ context.Context, kbID string, knowledgeIDs, dataSourceIDs []string,
) (int, int, error) {
	f.calls = append(f.calls, fakeCancelCall{
		kbID:          kbID,
		knowledgeIDs:  knowledgeIDs,
		dataSourceIDs: dataSourceIDs,
	})
	if f.recorder != nil {
		f.recorder.record("inspector_cancel:" + strings.Join(dataSourceIDs, ","))
	}
	return 2, 1, nil
}

var _ interfaces.TaskInspector = (*fakeCancelTaskInspector)(nil)
var _ interfaces.KnowledgeBaseTaskCanceller = (*fakeCancelTaskInspector)(nil)

// recordingSyncLogRepo wraps the sqlite repo and records the sweep the
// hard-cancel must be ordered against.
type recordingSyncLogRepo struct {
	interfaces.SyncLogRepository
	recorder *cancelOrderRecorder
}

func (r *recordingSyncLogRepo) CancelPendingByDataSource(ctx context.Context, dsID string) error {
	r.recorder.record("cancel_pending:" + dsID)
	return r.SyncLogRepository.CancelPendingByDataSource(ctx, dsID)
}

// reloadDS re-reads a data source row straight from its repository.
func reloadDS(t *testing.T, repo interfaces.DataSourceRepository, id string) *types.DataSource {
	t.Helper()
	ds, err := repo.FindByID(context.Background(), id)
	require.NoError(t, err)
	return ds
}

// ──────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────

// DeleteDataSource must hard-cancel queued (and running) datasource:sync
// tasks through the inspector BEFORE the durable sync-log sweep, scoped to
// exactly this data source.
func TestDeleteDataSourceHardCancelsQueuedSyncTasksBeforeSweep(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	recorder := &cancelOrderRecorder{}
	inspector := &fakeCancelTaskInspector{recorder: recorder}
	repo := &recordingSyncLogRepo{SyncLogRepository: f.syncLogRepo, recorder: recorder}
	svc := &DataSourceService{
		dsRepo:        f.dsRepo,
		syncLogRepo:   repo,
		scheduler:     datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil),
		taskInspector: inspector,
	}

	require.NoError(t, svc.DeleteDataSource(context.Background(), f.ds.ID, false))

	require.Len(t, inspector.calls, 1, "inspector must be consulted exactly once")
	call := inspector.calls[0]
	assert.Empty(t, call.kbID,
		"kbID must stay empty: dssync payloads carry no knowledge_base_id and a non-empty kbID would over-cancel the whole knowledge base")
	assert.Empty(t, call.knowledgeIDs)
	assert.Equal(t, []string{f.ds.ID}, call.dataSourceIDs)

	cancelIdx := recorder.indexOf("inspector_cancel:" + f.ds.ID)
	sweepIdx := recorder.indexOf("cancel_pending:" + f.ds.ID)
	require.GreaterOrEqual(t, cancelIdx, 0, "inspector cancel was not called")
	require.GreaterOrEqual(t, sweepIdx, 0, "sync-log sweep was not called")
	assert.Less(t, cancelIdx, sweepIdx,
		"inspector hard-cancel must run before CancelPendingByDataSource so retried queue records are swept too")

	assert.Equal(t, types.SyncLogStatusCanceled, f.reloadLog(t, f.syncLog.ID).Status,
		"the running log must reach a terminal state when its task is hard-cancelled")
}

// SetTaskInspector is the production injection path: a service built through
// NewDataSourceService (nil inspector) must start hard-cancelling once the
// setter installs one.
func TestSetTaskInspectorInstallsHardCancel(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	inspector := &fakeCancelTaskInspector{}
	svc := NewDataSourceService(f.dsRepo, f.syncLogRepo, nil, nil, nil, nil,
		datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil), nil, nil, nil)
	svc.(*DataSourceService).SetTaskInspector(inspector)

	require.NoError(t, svc.DeleteDataSource(context.Background(), f.ds.ID, false))
	require.Len(t, inspector.calls, 1)
	assert.Equal(t, []string{f.ds.ID}, inspector.calls[0].dataSourceIDs)
}

// Without an inspector the delete must degrade to the previous behavior
// (soft delete + cron removal + sync-log sweep) instead of failing.
func TestDeleteDataSourceWithoutInspectorDegradesToSweep(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	svc := &DataSourceService{
		dsRepo:      f.dsRepo,
		syncLogRepo: f.syncLogRepo,
		scheduler:   datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil),
	}

	require.NoError(t, svc.DeleteDataSource(context.Background(), f.ds.ID, false))
	assert.Equal(t, types.SyncLogStatusCanceled, f.reloadLog(t, f.syncLog.ID).Status)
}

// PauseDataSource must also cancel running + queued syncs for the paused
// data source only, with the inspector running before the sync-log sweep.
func TestPauseDataSourceCancelsRunningAndQueuedSyncs(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	// A running sync of another data source must stay untouched.
	otherLog := &types.SyncLog{
		ID:           "log-pause-other",
		DataSourceID: f.otherDS.ID,
		TenantID:     f.otherDS.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	require.NoError(t, f.syncLogRepo.Create(context.Background(), otherLog))

	recorder := &cancelOrderRecorder{}
	inspector := &fakeCancelTaskInspector{recorder: recorder}
	repo := &recordingSyncLogRepo{SyncLogRepository: f.syncLogRepo, recorder: recorder}
	svc := &DataSourceService{
		dsRepo:        f.dsRepo,
		syncLogRepo:   repo,
		scheduler:     datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil),
		taskInspector: inspector,
	}

	require.NoError(t, svc.PauseDataSource(context.Background(), f.ds.ID))

	assert.Equal(t, types.DataSourceStatusPaused, reloadDS(t, f.dsRepo, f.ds.ID).Status)
	require.Len(t, inspector.calls, 1)
	call := inspector.calls[0]
	assert.Empty(t, call.kbID)
	assert.Equal(t, []string{f.ds.ID}, call.dataSourceIDs)
	assert.Less(t,
		recorder.indexOf("inspector_cancel:"+f.ds.ID),
		recorder.indexOf("cancel_pending:"+f.ds.ID),
		"inspector hard-cancel must run before the sync-log sweep on pause")

	assert.Equal(t, types.SyncLogStatusCanceled, f.reloadLog(t, f.syncLog.ID).Status,
		"the running log must reach a terminal state when its task is hard-cancelled")
	assert.Equal(t, types.SyncLogStatusRunning, f.reloadLog(t, otherLog.ID).Status,
		"pause must not touch other data sources")
}

// manualSyncEnqueuer hands out a fixed asynq task id like the real client.
type manualSyncEnqueuer struct{ taskID string }

func (e *manualSyncEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	return &asynq.TaskInfo{ID: e.taskID, Payload: task.Payload()}, nil
}

// ManualSync must persist the enqueued task's asynq id onto the sync log so
// cancel flows and the runtime dashboard can correlate row ↔ queue record.
func TestManualSyncRecordsAsynqTaskID(t *testing.T) {
	f := newSQLiteSyncCancelFixture(t)
	svc := &DataSourceService{
		dsRepo:       f.dsRepo,
		syncLogRepo:  f.syncLogRepo,
		taskEnqueuer: &manualSyncEnqueuer{taskID: "asynq-manual-1"},
		scheduler:    datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil),
	}

	log, err := svc.ManualSync(context.Background(), f.ds.ID, false)
	require.NoError(t, err)
	require.NotNil(t, log)
	assert.NotEmpty(t, log.AsynqTaskID, "returned log must carry the asynq task id")
	assert.Equal(t, "asynq-manual-1", log.AsynqTaskID)

	stored := f.reloadLog(t, log.ID)
	assert.Equal(t, "asynq-manual-1", stored.AsynqTaskID, "task id must be persisted")
}

// payloadCaptureEnqueuer records every enqueued task payload so tests can
// assert what actually reaches the asynq queue.
type payloadCaptureEnqueuer struct{ payloads [][]byte }

func (e *payloadCaptureEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.payloads = append(e.payloads, task.Payload())
	return &asynq.TaskInfo{ID: "asynq-manual-x", Payload: task.Payload()}, nil
}

// ManualSync's forceFull argument must reach the task payload verbatim so the
// handler's force_full flag drives the full/incremental choice inside
// ProcessSync (the payload field already existed; the API now opens it up).
func TestManualSyncPassesForceFullToPayload(t *testing.T) {
	for _, force := range []bool{true, false} {
		f := newSQLiteSyncCancelFixture(t)
		enq := &payloadCaptureEnqueuer{}
		svc := &DataSourceService{
			dsRepo:       f.dsRepo,
			syncLogRepo:  f.syncLogRepo,
			taskEnqueuer: enq,
			scheduler:    datasource.NewScheduler(f.dsRepo, f.syncLogRepo, nil),
		}

		log, err := svc.ManualSync(context.Background(), f.ds.ID, force)
		require.NoError(t, err)
		require.NotNil(t, log)
		require.Len(t, enq.payloads, 1, "exactly one sync task must be enqueued")

		var payload types.DataSourceSyncPayload
		require.NoError(t, json.Unmarshal(enq.payloads[0], &payload))
		assert.Equal(t, f.ds.ID, payload.DataSourceID)
		assert.Equal(t, force, payload.ForceFull,
			"payload ForceFull must mirror the ManualSync forceFull argument")
	}
}
