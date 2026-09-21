package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

type nativeAdmissionControlsFake struct {
	mu       sync.Mutex
	values   []nativecontract.AdmissionControls
	errs     []error
	readCall int
}

func (f *nativeAdmissionControlsFake) Current(context.Context) (nativecontract.AdmissionControls, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.readCall
	f.readCall++
	if i < len(f.errs) && f.errs[i] != nil {
		return nativecontract.AdmissionControls{}, f.errs[i]
	}
	if i >= len(f.values) {
		i = len(f.values) - 1
	}
	return f.values[i], nil
}

type nativeAdmissionStoreKey struct {
	tenantID uint64
	ownerID  string
	appName  string
	userID   string
	session  string
	request  string
}

type nativeAdmissionStoreFake struct {
	mu      sync.Mutex
	records map[nativeAdmissionStoreKey]nativecontract.RunRecord
}

func (s *nativeAdmissionStoreFake) Lookup(_ context.Context, key NativeAdmissionKey) (nativecontract.RunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[nativeAdmissionStoreKey{key.TenantID, key.OwnerID, key.AppName, key.UserID, key.SessionID, key.RequestID}]
	return record, ok, nil
}

func (s *nativeAdmissionStoreFake) Admit(ctx context.Context, key NativeAdmissionKey, admission nativecontract.Admission, _ int64, reserve func(context.Context) error) (nativecontract.RunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fakeKey := nativeAdmissionStoreKey{key.TenantID, key.OwnerID, key.AppName, key.UserID, key.SessionID, key.RequestID}
	if prior, ok := s.records[fakeKey]; ok {
		if sameNativeAdmissionFingerprint(prior.Admission, admission) {
			return prior, true, nil
		}
		return nativecontract.RunRecord{}, false, nativecontract.Failure{Code: nativecontract.ErrConflict}
	}
	if err := reserve(ctx); err != nil {
		return nativecontract.RunRecord{}, false, err
	}
	record := nativecontract.RunRecord{Admission: admission, Status: nativecontract.RunQueued, Revision: 1}
	s.records[fakeKey] = record
	return record, false, nil
}

func (s *nativeAdmissionStoreFake) Get(_ context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity) (nativecontract.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.records {
		if key.tenantID == scope.TenantID && key.session == run.SessionID && record.Admission.Run.RunID == run.RunID {
			return record, nil
		}
	}
	return nativecontract.RunRecord{}, nativecontract.Failure{Code: nativecontract.ErrNotFound}
}

type nativeAdmissionAuthorityFake struct {
	mu         sync.Mutex
	scope      nativecontract.Scope
	config     nativecontract.ConfigBinding
	funding    nativecontract.FundingBinding
	grants     []nativecontract.ResourceGrant
	err        error
	rechecks   int
	revokeAt   int
	sessionErr error
}

func (f *nativeAdmissionAuthorityFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.scope, f.err
}

func (f *nativeAdmissionAuthorityFake) Recheck(_ context.Context, _ nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rechecks++
	if f.revokeAt == f.rechecks {
		return nativecontract.Scope{}, fmt.Errorf("grant revoked")
	}
	return f.scope, f.err
}

func (f *nativeAdmissionAuthorityFake) CurrentConfig(context.Context, nativecontract.Scope) (nativecontract.ConfigBinding, error) {
	return f.config, f.err
}

func (f *nativeAdmissionAuthorityFake) CurrentFunding(context.Context, nativecontract.Scope, nativecontract.RunIdentity) (nativecontract.FundingBinding, error) {
	return f.funding, f.err
}

func (f *nativeAdmissionAuthorityFake) RequiredGrants(context.Context, nativecontract.Scope, nativecontract.RunIdentity) ([]nativecontract.ResourceGrant, error) {
	return f.grants, f.err
}

func (f *nativeAdmissionAuthorityFake) ValidateSession(context.Context, nativecontract.Scope, session.Key) error {
	if f.sessionErr != nil {
		return f.sessionErr
	}
	return f.err
}

type nativeAdmissionBudgetFake struct {
	mu           sync.Mutex
	err          error
	reservations int
	last         nativecontract.Admission
}

func (b *nativeAdmissionBudgetFake) Reserve(_ context.Context, admission nativecontract.Admission) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reservations++
	b.last = admission
	return b.err
}

func (b *nativeAdmissionBudgetFake) Release(context.Context, nativecontract.Admission) error {
	return nil
}

type nativeAdmissionDispatchSpy struct {
	mu    sync.Mutex
	calls int
}

func (s *nativeAdmissionDispatchSpy) Dispatch(context.Context, nativecontract.RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return nil
}

func nativeAdmissionOpenControls() nativecontract.AdmissionControls {
	return nativecontract.AdmissionControls{Revision: 1, RecoveryEnabled: true, AdmissionEnabled: true, NativeExecutionApproved: true, DependenciesReady: true}
}

func nativeAdmissionFixture(inputHash, configHash string) nativecontract.Admission {
	return nativecontract.Admission{
		Scope:   nativecontract.Scope{TenantID: 7, ActorUserID: "actor-1", SessionOwnerID: "owner-1"},
		Run:     nativecontract.RunIdentity{TenantID: 7, SessionID: "session-1", RunID: "run-1", RequestID: "request-1"},
		Session: session.Key{AppName: "native", UserID: "owner-1", SessionID: "session-1"},
		Input:   model.NewUserMessage("hello"), InputHash: inputHash,
		Config:  nativecontract.ConfigBinding{ConfigHash: configHash, ModelConfigVersion: "model-v1"},
		Funding: nativecontract.FundingBinding{BudgetRef: "requested-budget"},
	}
}

func nativeAdmissionService(controls *nativeAdmissionControlsFake, store *nativeAdmissionStoreFake, budget *nativeAdmissionBudgetFake, dispatch *nativeAdmissionDispatchSpy) *NativeAdmissionService {
	admission := nativeAdmissionFixture("input-1", "config-1")
	authority := &nativeAdmissionAuthorityFake{
		scope: admission.Scope, config: admission.Config, funding: nativecontract.FundingBinding{BudgetRef: "authoritative-budget"},
		grants: []nativecontract.ResourceGrant{{ResourceType: "agent", ResourceID: "a-1", Action: "run"}},
	}
	return NewNativeAdmissionService(controls, authority, authority, store, budget, dispatch)
}

func nativeAdmissionCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure nativecontract.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error %v is not a native failure", err)
	}
	return failure.Code
}

func TestNativeAdmissionValidateControlsSeparatesExecutionApprovalAndDependencies(t *testing.T) {
	approvalClosed := nativeAdmissionOpenControls()
	approvalClosed.NativeExecutionApproved = false
	if got := nativeAdmissionCode(t, ValidateAdmissionControls(approvalClosed)); got != nativecontract.ErrExecutionGate {
		t.Fatalf("approval code = %q", got)
	}
	dependenciesMissing := nativeAdmissionOpenControls()
	dependenciesMissing.DependenciesReady = false
	if got := nativeAdmissionCode(t, ValidateAdmissionControls(dependenciesMissing)); got != nativecontract.ErrStore {
		t.Fatalf("dependencies code = %q", got)
	}
}

func TestNativeAdmissionReplaysAuthorizedRecordDuringDrainAndProtectsGetScope(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	store := &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}
	budget := &nativeAdmissionBudgetFake{}
	svc := nativeAdmissionService(controls, store, budget, &nativeAdmissionDispatchSpy{})
	admission := nativeAdmissionFixture("input-1", "config-1")
	first, err := svc.Admit(context.Background(), admission)
	if err != nil {
		t.Fatal(err)
	}
	closed := nativeAdmissionOpenControls()
	closed.WorkerDrain = true
	controls.values = []nativecontract.AdmissionControls{closed}
	controls.readCall = 0

	replay, err := svc.Admit(context.Background(), admission)
	if err != nil || replay.Admission.Run.RunID != first.Admission.Run.RunID || budget.reservations != 1 {
		t.Fatalf("replay=%+v err=%v reservations=%d", replay, err, budget.reservations)
	}
	got, err := svc.Get(context.Background(), admission.Scope, admission.Run)
	if err != nil || got.Admission.Run.RunID != first.Admission.Run.RunID {
		t.Fatalf("drain Get() = %+v, %v", got, err)
	}
	closed.WorkerDrain = false
	closed.AdmissionEnabled = false
	controls.values = []nativecontract.AdmissionControls{closed}
	if replay, err = svc.Admit(context.Background(), admission); err != nil || replay.Admission.Run.RunID != first.Admission.Run.RunID || budget.reservations != 1 {
		t.Fatalf("closed-gate replay=%+v err=%v reservations=%d", replay, err, budget.reservations)
	}
	conflict := admission
	conflict.InputHash = "input-2"
	if got := nativeAdmissionCode(t, mustAdmissionError(t, svc, conflict)); got != nativecontract.ErrConflict {
		t.Fatalf("conflict code=%q", got)
	}
	wrong := admission.Scope
	wrong.SessionOwnerID = "other-owner"
	if got := nativeAdmissionCode(t, mustGetError(t, svc, wrong, admission.Run)); got != nativecontract.ErrForbidden {
		t.Fatalf("wrong scope code=%q", got)
	}
}

func TestNativeAdmissionPersistsOnlyAuthoritativeFrozenAdmission(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	store := &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}
	budget := &nativeAdmissionBudgetFake{}
	dispatch := &nativeAdmissionDispatchSpy{}
	requested := nativeAdmissionFixture("input-1", "config-1")
	requested.Scope.ActorUserID = "untrusted-actor"
	requested.Config.ModelID = "untrusted-model"
	requested.Config.CredentialRef = "untrusted-credential"
	requested.Config.ToolSetHash = "untrusted-tools"
	requested.Funding = nativecontract.FundingBinding{BudgetRef: "untrusted-budget"}
	authoritativeScope := requested.Scope
	authoritativeScope.ActorUserID = "authoritative-actor"
	authoritativeScope.PolicyRevision = 9
	authoritativeConfig := requested.Config
	authoritativeConfig.ModelID = "authoritative-model"
	authoritativeConfig.CredentialRef = "authoritative-credential"
	authoritativeConfig.ToolSetHash = "authoritative-tools"
	authority := &nativeAdmissionAuthorityFake{
		scope: authoritativeScope, config: authoritativeConfig,
		funding: nativecontract.FundingBinding{BudgetRef: "authoritative-budget", BudgetRootRunID: "root-1"},
		grants:  []nativecontract.ResourceGrant{{ResourceType: "agent", ResourceID: "a-1", Action: "run"}},
	}
	svc := NewNativeAdmissionService(controls, authority, authority, store, budget, dispatch)
	if _, err := svc.Admit(context.Background(), requested); err != nil {
		t.Fatal(err)
	}
	if budget.last.Scope.ActorUserID != "authoritative-actor" || budget.last.Config.ModelID != "authoritative-model" ||
		budget.last.Config.CredentialRef != "authoritative-credential" || budget.last.Config.ToolSetHash != "authoritative-tools" ||
		budget.last.Funding.BudgetRef != "authoritative-budget" {
		t.Fatalf("budget saw unfrozen admission: %+v", budget.last)
	}
	for _, record := range store.records {
		if record.Admission.Scope.ActorUserID != "authoritative-actor" || record.Admission.Config.ModelID != "authoritative-model" || record.Admission.Funding.BudgetRootRunID != "root-1" {
			t.Fatalf("store persisted unfrozen admission: %+v", record.Admission)
		}
	}
}

func TestNativeAdmissionRejectsClosedAdmissionBeforeBudget(t *testing.T) {
	closed := nativeAdmissionOpenControls()
	closed.AdmissionEnabled = false
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{closed}}
	budget := &nativeAdmissionBudgetFake{}
	svc := nativeAdmissionService(controls, &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}, budget, &nativeAdmissionDispatchSpy{})

	_, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrAdmissionClosed || budget.reservations != 0 {
		t.Fatalf("code=%q reservations=%d", got, budget.reservations)
	}
}

func TestNativeAdmissionUsesOwnerAndSessionRequestKeyForConcurrentReplayAndConflict(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	store := &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}
	budget := &nativeAdmissionBudgetFake{}
	svc := nativeAdmissionService(controls, store, budget, &nativeAdmissionDispatchSpy{})

	base := nativeAdmissionFixture("input-1", "config-1")
	same := base
	same.Scope.ActorUserID = "actor-2"
	conflict := base
	conflict.InputHash = "input-2"
	requests := []nativecontract.Admission{base, same, conflict}
	var wg sync.WaitGroup
	errs := make([]error, len(requests))
	for i := range requests {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = svc.Admit(context.Background(), requests[i]) }(i)
	}
	wg.Wait()

	var success, conflicts int
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if nativeAdmissionCode(t, err) == nativecontract.ErrConflict {
			conflicts++
		}
	}
	if success+conflicts != len(requests) || success < 1 || len(store.records) != 1 || budget.reservations != 1 {
		t.Fatalf("success=%d conflicts=%d records=%d reservations=%d", success, conflicts, len(store.records), budget.reservations)
	}
}

func TestNativeAdmissionRejectsCrossSessionAndCurrentAuthorityChanges(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	store := &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}
	budget := &nativeAdmissionBudgetFake{}
	dispatch := &nativeAdmissionDispatchSpy{}
	svc := nativeAdmissionService(controls, store, budget, dispatch)

	crossSession := nativeAdmissionFixture("input-1", "config-1")
	crossSession.Session.SessionID = "other-session"
	if got := nativeAdmissionCode(t, mustAdmissionError(t, svc, crossSession)); got != nativecontract.ErrInvalid {
		t.Fatalf("cross session code=%q", got)
	}

	authority := &nativeAdmissionAuthorityFake{
		scope:  nativecontract.Scope{TenantID: 7, ActorUserID: "actor-9", SessionOwnerID: "owner-1"},
		config: nativecontract.ConfigBinding{ConfigHash: "config-2", ModelConfigVersion: "model-v2"},
	}
	svc = NewNativeAdmissionService(controls, authority, authority, store, budget, dispatch)
	if got := nativeAdmissionCode(t, mustAdmissionError(t, svc, nativeAdmissionFixture("input-1", "config-1"))); got != nativecontract.ErrForbidden {
		t.Fatalf("current authority code=%q", got)
	}
}

func TestNativeAdmissionBudgetFailureDoesNotDispatch(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	budget := &nativeAdmissionBudgetFake{err: nativecontract.Failure{Code: nativecontract.ErrBudget}}
	dispatch := &nativeAdmissionDispatchSpy{}
	store := &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}
	svc := nativeAdmissionService(controls, store, budget, dispatch)

	_, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrBudget {
		t.Fatalf("code=%q", got)
	}
	if len(store.records) != 0 || dispatch.calls != 0 {
		t.Fatalf("records=%d dispatches=%d", len(store.records), dispatch.calls)
	}
}

func TestNativeAdmissionControlSourceErrorsFailClosedOnEitherRead(t *testing.T) {
	for _, errs := range [][]error{{fmt.Errorf("first control read failed")}, {nil, fmt.Errorf("second control read failed")}} {
		controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}, errs: errs}
		svc := nativeAdmissionService(controls, &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}, &nativeAdmissionBudgetFake{}, &nativeAdmissionDispatchSpy{})
		if got := nativeAdmissionCode(t, mustAdmissionError(t, svc, nativeAdmissionFixture("input-1", "config-1"))); got != nativecontract.ErrStore {
			t.Fatalf("code=%q", got)
		}
	}
}

func TestNativeAdmissionRejectsRevokedGrantAndSessionSlot(t *testing.T) {
	controls := &nativeAdmissionControlsFake{values: []nativecontract.AdmissionControls{nativeAdmissionOpenControls()}}
	admission := nativeAdmissionFixture("input-1", "config-1")
	for _, authority := range []*nativeAdmissionAuthorityFake{
		{scope: admission.Scope, config: admission.Config, grants: []nativecontract.ResourceGrant{{ResourceType: "agent", ResourceID: "a-1", Action: "run"}}, revokeAt: 2},
		{scope: admission.Scope, config: admission.Config, grants: []nativecontract.ResourceGrant{{ResourceType: "agent", ResourceID: "a-1", Action: "run"}}, sessionErr: fmt.Errorf("slot unavailable")},
	} {
		budget := &nativeAdmissionBudgetFake{}
		svc := NewNativeAdmissionService(controls, authority, authority, &nativeAdmissionStoreFake{records: map[nativeAdmissionStoreKey]nativecontract.RunRecord{}}, budget, &nativeAdmissionDispatchSpy{})
		if got := nativeAdmissionCode(t, mustAdmissionError(t, svc, admission)); got != nativecontract.ErrForbidden || budget.reservations != 0 {
			t.Fatalf("code=%q reservations=%d", got, budget.reservations)
		}
	}
}

func mustAdmissionError(t *testing.T, svc *NativeAdmissionService, admission nativecontract.Admission) error {
	t.Helper()
	_, err := svc.Admit(context.Background(), admission)
	if err == nil {
		t.Fatal("expected admission error")
	}
	return err
}

func mustGetError(t *testing.T, svc *NativeAdmissionService, scope nativecontract.Scope, run nativecontract.RunIdentity) error {
	t.Helper()
	_, err := svc.Get(context.Background(), scope, run)
	if err == nil {
		t.Fatal("expected get error")
	}
	return err
}
