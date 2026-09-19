package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

type nativeAdmissionControlsFake struct {
	controls nativecontract.AdmissionControls
}

func (f *nativeAdmissionControlsFake) Current(context.Context) (nativecontract.AdmissionControls, error) {
	return f.controls, nil
}

type nativeAdmissionStoreFake struct {
	mu      sync.Mutex
	records map[string]nativecontract.RunRecord
}

func (s *nativeAdmissionStoreFake) FindAdmission(_ context.Context, tenantID uint64, ownerID, requestID string) (nativecontract.RunRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[nativeAdmissionKey(tenantID, ownerID, requestID)]
	return r, ok, nil
}

func (s *nativeAdmissionStoreFake) CreateAdmission(_ context.Context, admission nativecontract.Admission, _ int64) (nativecontract.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := nativecontract.RunRecord{Admission: admission, Status: nativecontract.RunQueued, Revision: 1}
	s.records[nativeAdmissionKey(admission.Scope.TenantID, admission.Scope.ActorUserID, admission.Run.RequestID)] = r
	return r, nil
}

func (s *nativeAdmissionStoreFake) Get(_ context.Context, _ nativecontract.Scope, run nativecontract.RunIdentity) (nativecontract.RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.Admission.Run.RunID == run.RunID {
			return r, nil
		}
	}
	return nativecontract.RunRecord{}, nativecontract.Failure{Code: nativecontract.ErrNotFound}
}

type nativeAdmissionBudgetFake struct {
	err          error
	reservations int
	releases     int
}

func (b *nativeAdmissionBudgetFake) Reserve(context.Context, nativecontract.Admission) error {
	b.reservations++
	return b.err
}
func (b *nativeAdmissionBudgetFake) Release(context.Context, nativecontract.Admission) error {
	b.releases++
	return nil
}

func nativeAdmissionKey(tenantID uint64, ownerID, requestID string) string {
	return string(rune(tenantID)) + "|" + ownerID + "|" + requestID
}

func nativeAdmissionOpenControls() nativecontract.AdmissionControls {
	return nativecontract.AdmissionControls{
		Revision: 1, RecoveryEnabled: true, AdmissionEnabled: true,
		NativeExecutionApproved: true, DependenciesReady: true,
	}
}

func nativeAdmissionFixture(inputHash, configHash string) nativecontract.Admission {
	return nativecontract.Admission{
		Scope: nativecontract.Scope{TenantID: 7, ActorUserID: "owner-1"},
		Run:   nativecontract.RunIdentity{TenantID: 7, SessionID: "session-1", RunID: "run-1", RequestID: "request-1"},
		Input: model.NewUserMessage("hello"), InputHash: inputHash,
		Config: nativecontract.ConfigBinding{ConfigHash: configHash},
	}
}

func nativeAdmissionCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure nativecontract.Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error %v is not a native failure", err)
	}
	return failure.Code
}

func TestNativeAdmissionRejectsClosedAdmission(t *testing.T) {
	controls := &nativeAdmissionControlsFake{controls: nativeAdmissionOpenControls()}
	controls.controls.AdmissionEnabled = false
	budget := &nativeAdmissionBudgetFake{}
	store := &nativeAdmissionStoreFake{records: map[string]nativecontract.RunRecord{}}
	svc := NewNativeAdmissionService(controls, store, budget)

	_, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrAdmissionClosed {
		t.Fatalf("code = %q, want %q", got, nativecontract.ErrAdmissionClosed)
	}
	if budget.reservations != 0 {
		t.Fatalf("closed admission reserved budget %d times", budget.reservations)
	}
}

func TestValidateAdmissionControlsRejectsExecutionGateAndUnavailableDependencies(t *testing.T) {
	for _, controls := range []nativecontract.AdmissionControls{
		{Revision: 1, RecoveryEnabled: true, AdmissionEnabled: true, DependenciesReady: true},
		{Revision: 1, RecoveryEnabled: true, AdmissionEnabled: true, NativeExecutionApproved: true},
	} {
		err := ValidateAdmissionControls(controls)
		if got := nativeAdmissionCode(t, err); got != nativecontract.ErrExecutionGate {
			t.Fatalf("code = %q, want %q", got, nativecontract.ErrExecutionGate)
		}
	}
}

func TestNativeAdmissionReplaysSameRequestAndRejectsHashConflict(t *testing.T) {
	controls := &nativeAdmissionControlsFake{controls: nativeAdmissionOpenControls()}
	budget := &nativeAdmissionBudgetFake{}
	store := &nativeAdmissionStoreFake{records: map[string]nativecontract.RunRecord{}}
	svc := NewNativeAdmissionService(controls, store, budget)

	first, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if err != nil {
		t.Fatal(err)
	}
	controls.controls.WorkerDrain = true
	replay, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if err != nil {
		t.Fatalf("same request replay = %v", err)
	}
	if replay.Admission.Run.RunID != first.Admission.Run.RunID || budget.reservations != 1 {
		t.Fatalf("replay=%+v reservations=%d", replay, budget.reservations)
	}
	_, err = svc.Admit(context.Background(), nativeAdmissionFixture("input-2", "config-1"))
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrConflict {
		t.Fatalf("code = %q, want %q", got, nativecontract.ErrConflict)
	}
}

func TestNativeAdmissionWorkerDrainBlocksNewClaimsButGetRemainsAvailable(t *testing.T) {
	controls := &nativeAdmissionControlsFake{controls: nativeAdmissionOpenControls()}
	store := &nativeAdmissionStoreFake{records: map[string]nativecontract.RunRecord{}}
	svc := NewNativeAdmissionService(controls, store, &nativeAdmissionBudgetFake{})
	admission := nativeAdmissionFixture("input-1", "config-1")
	if _, err := svc.Admit(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	controls.controls.WorkerDrain = true
	next := nativeAdmissionFixture("input-2", "config-1")
	next.Run.RequestID = "request-2"
	next.Run.RunID = "run-2"
	_, err := svc.Admit(context.Background(), next)
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrAdmissionClosed {
		t.Fatalf("code = %q, want %q", got, nativecontract.ErrAdmissionClosed)
	}
	got, err := svc.Get(context.Background(), admission.Scope, admission.Run)
	if err != nil || got.Admission.Run.RunID != "run-1" {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
}

func TestNativeAdmissionBudgetFailureDoesNotDispatchModel(t *testing.T) {
	controls := &nativeAdmissionControlsFake{controls: nativeAdmissionOpenControls()}
	budget := &nativeAdmissionBudgetFake{err: nativecontract.Failure{Code: nativecontract.ErrBudget}}
	store := &nativeAdmissionStoreFake{records: map[string]nativecontract.RunRecord{}}
	svc := NewNativeAdmissionService(controls, store, budget)

	_, err := svc.Admit(context.Background(), nativeAdmissionFixture("input-1", "config-1"))
	if got := nativeAdmissionCode(t, err); got != nativecontract.ErrBudget {
		t.Fatalf("code = %q, want %q", got, nativecontract.ErrBudget)
	}
	if len(store.records) != 0 {
		t.Fatalf("budget denial created a run: %+v", store.records)
	}
}
