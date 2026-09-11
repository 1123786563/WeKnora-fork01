package appconnector

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/appconnector"
	service "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/hibiken/asynq"
)

// --- fakes ---------------------------------------------------------------

type fakeDSRepo struct{ ds *types.DataSource }

func (f *fakeDSRepo) Create(ctx context.Context, ds *types.DataSource) error { return nil }
func (f *fakeDSRepo) FindByID(ctx context.Context, id string) (*types.DataSource, error) {
	if f.ds != nil && f.ds.ID == id {
		return f.ds, nil
	}
	return nil, errors.New("data source not found")
}
func (f *fakeDSRepo) FindByKnowledgeBase(ctx context.Context, kbID string) ([]*types.DataSource, error) {
	return nil, nil
}
func (f *fakeDSRepo) Update(ctx context.Context, ds *types.DataSource) error          { return nil }
func (f *fakeDSRepo) UpdateSyncState(ctx context.Context, ds *types.DataSource) error { return nil }
func (f *fakeDSRepo) Delete(ctx context.Context, id string) error                     { return nil }
func (f *fakeDSRepo) FindActive(ctx context.Context) ([]*types.DataSource, error)     { return nil, nil }

type fakeSyncLogRepo struct{ created int }

func (f *fakeSyncLogRepo) Create(ctx context.Context, log *types.SyncLog) error {
	f.created++
	if log.ID == "" {
		log.ID = "synclog-1"
	}
	return nil
}
func (f *fakeSyncLogRepo) FindByID(ctx context.Context, id string) (*types.SyncLog, error) {
	return nil, errors.New("sync log not found")
}
func (f *fakeSyncLogRepo) FindByDataSource(ctx context.Context, dsID string, limit, offset int) ([]*types.SyncLog, error) {
	return nil, nil
}
func (f *fakeSyncLogRepo) FindLatest(ctx context.Context, dsID string) (*types.SyncLog, error) {
	return nil, nil
}
func (f *fakeSyncLogRepo) HasRunningSync(ctx context.Context, dsID string) (bool, error) {
	return false, nil
}
func (f *fakeSyncLogRepo) Update(ctx context.Context, log *types.SyncLog) error       { return nil }
func (f *fakeSyncLogRepo) UpdateResult(ctx context.Context, log *types.SyncLog) error { return nil }
func (f *fakeSyncLogRepo) CancelPendingByDataSource(ctx context.Context, dsID string) error {
	return nil
}
func (f *fakeSyncLogRepo) CleanupOldLogs(ctx context.Context, retentionDays int) error { return nil }

type fakeEnqueuer struct{ enqueued int }

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.enqueued++
	return &asynq.TaskInfo{ID: "t1"}, nil
}

type fakeBindingStore struct {
	row *appconnector.StoredSyncBinding
	err error
}

func (f *fakeBindingStore) FindSyncBinding(ctx context.Context, tenantID uint64, dataSourceID string) (*appconnector.StoredSyncBinding, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.row == nil {
		return nil, appconnector.ErrSyncBindingNotFound
	}
	return f.row, nil
}

type fakeSpaceState struct{ state *appconnector.BindingState }

func (f *fakeSpaceState) SpaceBindingState(ctx context.Context, tenantID uint64, connectionID string) *appconnector.BindingState {
	return f.state
}

func newTestService(ds *types.DataSource, logs *fakeSyncLogRepo, enq *fakeEnqueuer) *service.DataSourceService {
	return service.NewDataSourceService(
		&fakeDSRepo{ds: ds}, logs, nil, nil, enq, nil, nil, nil, nil, nil,
	).(*service.DataSourceService)
}

func activeDS() *types.DataSource {
	return &types.DataSource{
		ID: "ds-1", TenantID: 7, KnowledgeBaseID: "kb-1",
		Status: types.DataSourceStatusActive, Name: "Feishu wiki", Type: "feishu",
	}
}

// --- behavior ------------------------------------------------------------

// Plan expiry must pause NEW sync dispatch before any sync log or task exists,
// while persisted settlements still complete.
func TestManualSyncPausedByPlanExpiryBeforeScheduling(t *testing.T) {
	logs := &fakeSyncLogRepo{}
	enq := &fakeEnqueuer{}
	svc := newTestService(activeDS(), logs, enq)
	svc.SetSyncExecution(nil, nil, func(ctx context.Context, tenant uint64) bool { return false })

	_, err := svc.ManualSync(context.Background(), "ds-1")
	if err == nil {
		t.Fatal("expired plan must pause manual sync")
	}
	var paused *appconnector.SyncPausedError
	if !errors.As(err, &paused) || paused.Reason != appconnector.PauseReasonPlan {
		t.Fatalf("expected plan pause reason, got %v", err)
	}
	if logs.created != 0 || enq.enqueued != 0 {
		t.Fatalf("paused sync must not create sync logs or tasks (logs=%d tasks=%d)", logs.created, enq.enqueued)
	}
	if !appconnector.MaySettlePersistedSync(false) {
		t.Fatal("persisted settlement must still be allowed during plan pause")
	}
}

// A bound data source whose credentials cannot prove space ownership (legacy
// pre-appconnector credentials) must pause with reason permission, not fail.
func TestManualSyncPausedForUnprovableLegacyCredentials(t *testing.T) {
	logs := &fakeSyncLogRepo{}
	svc := newTestService(activeDS(), logs, &fakeEnqueuer{})
	store := &fakeBindingStore{row: &appconnector.StoredSyncBinding{
		TenantID: 7, DataSourceID: "ds-1", InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 1,
	}}
	svc.SetSyncExecution(store, &fakeSpaceState{state: nil}, func(ctx context.Context, tenant uint64) bool { return true })

	_, err := svc.ManualSync(context.Background(), "ds-1")
	var paused *appconnector.SyncPausedError
	if !errors.As(err, &paused) || paused.Reason != appconnector.PauseReasonPermission {
		t.Fatalf("expected permission pause for legacy credentials, got %v", err)
	}
	if logs.created != 0 {
		t.Fatalf("paused sync must not create sync logs (got %d)", logs.created)
	}
}

// A revoked space connection pauses team sync; the local copy is untouched.
func TestManualSyncPausedForRevokedSpaceConnection(t *testing.T) {
	logs := &fakeSyncLogRepo{}
	svc := newTestService(activeDS(), logs, &fakeEnqueuer{})
	store := &fakeBindingStore{row: &appconnector.StoredSyncBinding{
		TenantID: 7, DataSourceID: "ds-1", InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 2,
	}}
	svc.SetSyncExecution(store, &fakeSpaceState{state: &appconnector.BindingState{
		ConnectionState:   appconnector.ConnectionRevoked,
		ConnectionKind:    appconnector.ConnectionKindSpace,
		InstallationState: appconnector.InstallationActive,
	}}, func(ctx context.Context, tenant uint64) bool { return true })

	if _, err := svc.ManualSync(context.Background(), "ds-1"); err == nil {
		t.Fatal("revoked connection must pause manual sync")
	}
}

// An active space connection on an active installation dispatches normally.
func TestManualSyncDispatchesWithActiveSpaceConnection(t *testing.T) {
	logs := &fakeSyncLogRepo{}
	enq := &fakeEnqueuer{}
	svc := newTestService(activeDS(), logs, enq)
	store := &fakeBindingStore{row: &appconnector.StoredSyncBinding{
		TenantID: 7, DataSourceID: "ds-1", InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 2,
	}}
	svc.SetSyncExecution(store, &fakeSpaceState{state: &appconnector.BindingState{
		ConnectionState:   appconnector.ConnectionActive,
		ConnectionKind:    appconnector.ConnectionKindSpace,
		InstallationState: appconnector.InstallationActive,
	}}, func(ctx context.Context, tenant uint64) bool { return true })

	if _, err := svc.ManualSync(context.Background(), "ds-1"); err != nil {
		t.Fatalf("active space connection must dispatch: %v", err)
	}
	if logs.created != 1 || enq.enqueued != 1 {
		t.Fatalf("expected one sync log and one task (logs=%d tasks=%d)", logs.created, enq.enqueued)
	}
}

// Other (unbound) data sources keep the legacy path, but they cannot bypass
// the budget/plan gate of an enabled space.
func TestLegacyUnboundDataSourceStillGatedByPlan(t *testing.T) {
	logs := &fakeSyncLogRepo{}
	svc := newTestService(activeDS(), logs, &fakeEnqueuer{})
	svc.SetSyncExecution(&fakeBindingStore{}, nil, func(ctx context.Context, tenant uint64) bool { return false })

	if _, err := svc.ManualSync(context.Background(), "ds-1"); err == nil {
		t.Fatal("unbound legacy data source must not bypass the plan gate")
	}
	// With the plan active, the same unbound data source runs the legacy path.
	svc.SetSyncExecution(&fakeBindingStore{}, nil, func(ctx context.Context, tenant uint64) bool { return true })
	if err := svc.AuthorizeSyncExecution(context.Background(), activeDS()); err != nil {
		t.Fatalf("unbound data source with active plan must use the legacy path: %v", err)
	}
}

// A stale worker that lost its lease fence must not advance the checkpoint.
func TestCheckpointFenceRefusesStaleWorker(t *testing.T) {
	svc := newTestService(activeDS(), &fakeSyncLogRepo{}, &fakeEnqueuer{})
	worker := svc.TakeSyncFence("ds-1")
	if worker != 1 {
		t.Fatalf("first fence must be 1, got %d", worker)
	}
	current := svc.TakeSyncFence("ds-1") // a newer worker took over
	if appconnector.CanAdvanceCheckpoint(true, current, worker) {
		t.Fatal("stale worker must not advance the checkpoint cursor")
	}
}
