package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type nativePendingReservedFake struct {
	nativePendingStoreFake
	consumed      int
	mutateReceipt func(*nativecontract.PendingDecisionDetail)
}

func (s *nativePendingReservedFake) ResolveReserved(_ context.Context, _ nativecontract.Scope, _ nativecontract.PendingKey, req nativecontract.ResolvePendingRequest, _ nativecontract.Fence, reserve func() error) (nativecontract.PendingResolution, error) {
	if err := reserve(); err != nil {
		return nativecontract.PendingResolution{}, err
	}
	if s.detail.Status == nativecontract.PendingOpen {
		s.consumed++
		s.detail.Status = nativecontract.PendingResolved
		s.detail.ResolvedDecisionID = req.DecisionID
		s.detail.ResolvedAction = req.Action
		s.detail.Ref.Revision = "2"
	}
	receipt := s.detail
	if s.mutateReceipt != nil {
		s.mutateReceipt(&receipt)
	}
	return nativecontract.PendingResolution{Detail: receipt}, nil
}

func nativePendingDispatch(key nativecontract.PendingKey, scope nativecontract.Scope) nativecontract.ToolDispatchRequest {
	fence := nativecontract.Fence{Run: key.Run, Owner: "worker", Epoch: 1}
	return nativecontract.ToolDispatchRequest{Scope: scope, Fence: fence,
		Plan:    nativecontract.ToolPlan{Run: key.Run, Version: 2, CallID: "call-1", ArgsHash: "args-v1", Tool: nativecontract.ToolIdentity{Name: "create", ServiceID: "svc-1", InstallationID: "install-1", SchemaHash: "schema-v1"}},
		Attempt: nativecontract.Attempt{ID: "attempt", Run: key.Run, Kind: nativecontract.ToolAttempt, Epoch: 1, LogicalCallID: "call-1"}, Funding: nativecontract.FundingBinding{BudgetRootRunID: key.Run.RunID}, ReservationUnits: 7}
}

// Corrupt store receipts must never mint a typed result for another call,
// another immutable plan, a termination, or an uncommitted revision.
func TestNativePendingCoordinatorRejectsMutatedReceipt(t *testing.T) {
	for name, mutate := range map[string]func(*nativecontract.PendingDecisionDetail){
		"call":               func(d *nativecontract.PendingDecisionDetail) { d.CallID = "another-call" },
		"plan":               func(d *nativecontract.PendingDecisionDetail) { d.PlanVersion = 3 },
		"args":               func(d *nativecontract.PendingDecisionDetail) { d.ArgsHash = "other-args" },
		"action":             func(d *nativecontract.PendingDecisionDetail) { d.ResolvedAction = nativecontract.DecisionTerminate },
		"revision":           func(d *nativecontract.PendingDecisionDetail) { d.Ref.Revision = "1" },
		"empty revision":     func(d *nativecontract.PendingDecisionDetail) { d.Ref.Revision = "" },
		"malformed revision": func(d *nativecontract.PendingDecisionDetail) { d.Ref.Revision = "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			base, _, scope, key := nativePendingServiceFixture(t, nil)
			base.store = &nativePendingReservedFake{nativePendingStoreFake: nativePendingStoreFake{detail: nativePendingServiceDetail(key)}, mutateReceipt: mutate}
			dispatch := nativePendingDispatch(key, scope)
			reservation := repository.NewInMemoryNativeToolDispatchReservation(repository.NativeToolDispatchBudget{Root: key.Run, Available: 10})
			reservation.SetLiveFence(dispatch.Fence)
			got, err := NewNativePendingConsumptionCoordinator(base, reservation).ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
			var failure *nativecontract.Failure
			require.ErrorAs(t, err, &failure)
			require.Equal(t, nativecontract.ErrStore, failure.Code)
			require.Equal(t, nativecontract.EffectNotDispatched, failure.Effect)
			require.Equal(t, NativeResolvedPendingIdentity{}, got)
		})
	}
}

func nativePendingCoordinatorDB(t *testing.T) (*gorm.DB, *repository.NativePendingDecisionRepository, *NativePendingConsumptionCoordinator, *repository.InMemoryNativeToolDispatchReservation, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pending.db")+"?_foreign_keys=on&_busy_timeout=5000&_txlock=immediate"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE tenants (id INTEGER PRIMARY KEY)").Error)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	schema, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "../../../migrations/sqlite/000083_native_agent_schema.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
	for _, sql := range []string{
		"INSERT INTO tenants (id) VALUES (1)",
		"INSERT INTO native_agent_tenants (tenant_id) VALUES (1)",
		"INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (1, 'owner', 'session')",
	} {
		require.NoError(t, db.Exec(sql).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO native_agent_runs (tenant_id, run_id, session_id, owner_id, status, revision, lease_owner, lease_epoch, lease_expires_at) VALUES (1, 'run-1', 'session', 'owner', 'waiting_user', 4, 'worker', 1, ?)", time.Now().Add(time.Hour)).Error)
	base, _, scope, key := nativePendingServiceFixture(t, nil)
	repo := repository.NewNativePendingDecisionRepository(db)
	require.NoError(t, repo.Create(context.Background(), key.Run, nativePendingServiceDetail(key)))
	base.store = repo
	reservation := repository.NewInMemoryNativeToolDispatchReservation(repository.NativeToolDispatchBudget{Root: key.Run, Available: 10})
	reservation.SetLiveFence(nativePendingDispatch(key, scope).Fence)
	return db, repo, NewNativePendingConsumptionCoordinator(base, reservation), reservation, scope, key
}

func TestNativePendingCoordinatorConcurrentRealRepository(t *testing.T) {
	_, repo, coordinator, reservation, scope, key := nativePendingCoordinatorDB(t)
	ctx := context.Background()
	dispatch := nativePendingDispatch(key, scope)
	start := make(chan struct{})
	type result struct {
		identity NativeResolvedPendingIdentity
		err      error
	}
	results := make(chan result, 8)
	for i := 0; i < cap(results); i++ {
		go func() {
			<-start
			got, err := coordinator.ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
			results <- result{got, err}
		}()
	}
	close(start)
	var first NativeResolvedPendingIdentity
	for i := 0; i < cap(results); i++ {
		got := <-results
		require.NoError(t, got.err)
		if i == 0 {
			first = got.identity
		} else {
			require.Equal(t, first, got.identity)
		}
	}
	replay, err := coordinator.ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	remaining, ok := reservation.Remaining(key.Run)
	require.True(t, ok)
	require.EqualValues(t, 3, remaining)
	detail, err := repo.Get(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, nativecontract.PendingResolved, detail.Status)
	require.Equal(t, nativecontract.RunQueued, detail.RunStatus)
	require.Equal(t, "2", detail.Ref.Revision)
	require.Equal(t, "5", detail.RunRevision)
}

// A SQL failure after the in-memory reservation must return no typed receipt,
// roll back both durable transitions, and preserve exactly one hold on replay.
func TestNativePendingCoordinatorPostReservationFailureRetainsHold(t *testing.T) {
	db, repo, coordinator, reservation, scope, key := nativePendingCoordinatorDB(t)
	ctx := context.Background()
	dispatch := nativePendingDispatch(key, scope)
	require.NoError(t, db.Exec(`CREATE TRIGGER pending_commit_fault BEFORE UPDATE OF status ON native_agent_runs WHEN NEW.status='queued' BEGIN SELECT RAISE(ABORT, 'test post-reservation fault'); END`).Error)
	for attempt := 0; attempt < 2; attempt++ {
		got, err := coordinator.ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
		require.ErrorContains(t, err, "test post-reservation fault")
		require.Equal(t, NativeResolvedPendingIdentity{}, got)
		remaining, ok := reservation.Remaining(key.Run)
		require.True(t, ok)
		require.EqualValues(t, 3, remaining)
		detail, err := repo.Get(ctx, scope, key)
		require.NoError(t, err)
		require.Equal(t, nativecontract.PendingOpen, detail.Status)
		require.Equal(t, nativecontract.RunWaiting, detail.RunStatus)
		require.Equal(t, "1", detail.Ref.Revision)
		require.Equal(t, "4", detail.RunRevision)
		require.Empty(t, detail.ResolvedDecisionID)
	}
	// Once storage is repaired the exact replay reconciles the existing hold;
	// it does not reserve again or invoke any execution component.
	require.NoError(t, db.Exec("DROP TRIGGER pending_commit_fault").Error)
	got, err := coordinator.ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
	require.NoError(t, err)
	require.Equal(t, "2", got.PendingRevision)
	remaining, _ := reservation.Remaining(key.Run)
	require.EqualValues(t, 3, remaining)
}

func TestNativePendingCoordinatorReservesBeforeConsumption(t *testing.T) {
	base, resolver, scope, key := nativePendingServiceFixture(t, nil)
	store := &nativePendingReservedFake{nativePendingStoreFake: nativePendingStoreFake{detail: nativePendingServiceDetail(key)}}
	base.store = store
	reservation := repository.NewInMemoryNativeToolDispatchReservation(repository.NativeToolDispatchBudget{Root: key.Run, Available: 6})
	fence := nativecontract.Fence{Run: key.Run, Owner: "worker", Epoch: 1}
	reservation.SetLiveFence(fence)
	coordinator := NewNativePendingConsumptionCoordinator(base, reservation)
	dispatch := nativecontract.ToolDispatchRequest{Scope: scope, Fence: fence,
		Plan:    nativecontract.ToolPlan{Run: key.Run, Version: 2, CallID: "call-1", ArgsHash: "args-v1", Tool: nativecontract.ToolIdentity{Name: "create", ServiceID: "svc-1", InstallationID: "install-1", SchemaHash: "schema-v1"}},
		Attempt: nativecontract.Attempt{ID: "attempt", Run: key.Run, Kind: nativecontract.ToolAttempt, Epoch: 1, LogicalCallID: "call-1"}, Funding: nativecontract.FundingBinding{BudgetRootRunID: key.Run.RunID}, ReservationUnits: 7}
	_, err := coordinator.ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
	require.Error(t, err)
	require.Zero(t, store.consumed)
	reservation.SetBudget(key.Run, 10)
	first, err := coordinator.ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
	require.NoError(t, err)
	require.Equal(t, key, first.Key)
	require.Equal(t, "decision-1", first.DecisionID)
	_, err = coordinator.ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
	require.NoError(t, err)
	require.Equal(t, 1, store.consumed)
	remaining, ok := reservation.Remaining(key.Run)
	require.True(t, ok)
	require.EqualValues(t, 3, remaining)
	dispatch.ReservationUnits = 8
	_, err = coordinator.ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
	require.Error(t, err)
	resolver.err = errors.New("revoked")
	resolver.errAt = 0
	_, err = coordinator.ResolveAndReserve(context.Background(), scope, key, nativePendingServiceRequest(), dispatch)
	require.Error(t, err)
	require.Equal(t, 1, store.consumed)
}

type nativePendingScopeResolverFake struct {
	mu     sync.Mutex
	result nativecontract.Scope
	err    error
	errAt  int
	grants [][]nativecontract.ResourceGrant
}

func (f *nativePendingScopeResolverFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.result, f.err
}
func (f *nativePendingScopeResolverFake) Recheck(_ context.Context, _ nativecontract.Scope, grants []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants = append(f.grants, grants)
	if f.err != nil && (f.errAt == 0 || len(f.grants) == f.errAt) {
		return nativecontract.Scope{}, f.err
	}
	return f.result, nil
}

type nativePendingStoreFake struct {
	detail nativecontract.PendingDecisionDetail
}

func (s nativePendingStoreFake) List(_ context.Context, _ nativecontract.Scope, _ nativecontract.RunIdentity, _ string, _ int) (nativecontract.PendingDecisionPage, error) {
	return nativecontract.PendingDecisionPage{Items: []nativecontract.PendingDecisionDetail{s.detail}}, nil
}
func (s nativePendingStoreFake) Get(_ context.Context, _ nativecontract.Scope, _ nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error) {
	return s.detail, nil
}
func (s nativePendingStoreFake) Resolve(_ context.Context, _ nativecontract.Scope, _ nativecontract.PendingKey, _ nativecontract.ResolvePendingRequest) (nativecontract.PendingResolution, error) {
	return nativecontract.PendingResolution{Detail: s.detail}, nil
}

func nativePendingServiceFixture(t *testing.T, recheckErr error) (*NativePendingDecisionService, *nativePendingScopeResolverFake, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	key := nativecontract.PendingKey{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, PendingID: "pending-1"}
	detail := nativePendingServiceDetail(key)
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", Grants: []nativecontract.ResourceGrant{{ResourceType: "connector", ResourceID: "calendar-1", Action: "write"}}}
	resolver := &nativePendingScopeResolverFake{result: scope, err: recheckErr, errAt: 2}
	return NewNativePendingDecisionService(nativePendingStoreFake{detail: detail}, resolver), resolver, scope, key
}

func nativePendingServiceDetail(key nativecontract.PendingKey) nativecontract.PendingDecisionDetail {
	return nativecontract.PendingDecisionDetail{Version: 1, Ref: nativecontract.PendingReference{PendingID: key.PendingID, Revision: "1"}, SessionID: key.Run.SessionID, RunID: key.Run.RunID, CallID: "call-1", WaitKind: nativecontract.WaitApproval, Status: nativecontract.PendingOpen, RunStatus: nativecontract.RunWaiting, RunRevision: "4", PlanVersion: 2, ArgsHash: "args-v1", Service: nativecontract.PendingServiceIdentity{Kind: "connector", ServiceID: "svc-1", InstallationID: "install-1", ResourceRef: "calendar-1", ToolName: "create", SchemaHash: "schema-v1"}, OperationDescription: "create", RedactedArgs: []byte(`{}`), ExpiresAt: nativePendingFuture(), AllowedActions: []nativecontract.DecisionAction{nativecontract.DecisionRetry}}
}

func nativePendingFuture() time.Time { return time.Now().Add(time.Hour).UTC() }

func nativePendingServiceRequest() nativecontract.ResolvePendingRequest {
	return nativecontract.ResolvePendingRequest{DecisionID: "decision-1", CallID: "call-1", ExpectedRevision: "4", PendingRevision: "1", PlanVersion: 2, ArgsHash: "args-v1", ResourceRef: "calendar-1", Action: nativecontract.DecisionRetry, Reason: "approved"}
}

func TestNativePendingDecisionServiceFailsClosedWhenScopeWasRevoked(t *testing.T) {
	svc, _, scope, key := nativePendingServiceFixture(t, errors.New("grant revoked"))
	_, err := svc.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrForbidden, failure.Code)
}

func TestNativePendingDecisionServiceDoesNotForgeOAuthCompletion(t *testing.T) {
	svc, _, scope, key := nativePendingServiceFixture(t, nil)
	_, err := svc.BeginOAuth(context.Background(), scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://example.test/callback"})
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrStore, failure.Code)
}

func TestNativePendingDecisionServiceRechecksDetailGrantsForReads(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) error
	}{
		{"list", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.List(context.Background(), scope, key.Run, "", 10)
			return err
		}},
		{"get", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Get(context.Background(), scope, key)
			return err
		}},
		{"resolve", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			svc, resolver, scope, key := nativePendingServiceFixture(t, errors.New("resource revoked"))
			var failure *nativecontract.Failure
			require.True(t, errors.As(operation.call(svc, scope, key), &failure))
			require.Equal(t, nativecontract.ErrForbidden, failure.Code)
			require.Len(t, resolver.grants, 2)
			require.Equal(t, []nativecontract.ResourceGrant{
				{ResourceType: "native_pending_resource", ResourceID: "calendar-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_service", ResourceID: "svc-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_installation", ResourceID: "install-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_tool", ResourceID: "create", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
			}, resolver.grants[1], "detail identity must be rechecked, not just the caller scope")
		})
	}
}

func TestNativePendingDecisionServiceHidesForeignTenantAndSession(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) error
	}{
		{"list", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.List(context.Background(), scope, key.Run, "", 10)
			return err
		}},
		{"get", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Get(context.Background(), scope, key)
			return err
		}},
		{"resolve", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
			return err
		}},
	} {
		for _, foreign := range []struct {
			name  string
			scope nativecontract.Scope
			key   func(nativecontract.PendingKey) nativecontract.PendingKey
		}{
			{"tenant", nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner"}, func(key nativecontract.PendingKey) nativecontract.PendingKey { key.Run.TenantID = 2; return key }},
			{"session", nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner"}, func(key nativecontract.PendingKey) nativecontract.PendingKey {
				key.Run.SessionID = "other-session"
				return key
			}},
		} {
			t.Run(operation.name+"/"+foreign.name, func(t *testing.T) {
				svc, _, _, key := nativePendingServiceFixture(t, nil)
				var failure *nativecontract.Failure
				require.True(t, errors.As(operation.call(svc, foreign.scope, foreign.key(key)), &failure))
				require.Equal(t, nativecontract.ErrNotFound, failure.Code)
			})
		}
	}
}
