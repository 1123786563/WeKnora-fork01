package sandbox

import (
	"context"
	"testing"
)

type recoveryFake struct {
	observations map[string]ExecutionObservation
	observed     int
	created      int
}

func (f *recoveryFake) Observe(_ context.Context, ref ExecutionRef) (ExecutionObservation, error) {
	f.observed++
	return f.observations[ref.TaskID], nil
}
func (f *recoveryFake) CancelExecution(context.Context, ExecutionRef) error { return nil }
func validExecutionRef() ExecutionRef {
	return ExecutionRef{TenantID: 1, SessionID: "s1", Provider: "docker", ConfigID: "cfg", InstanceID: "i1", Generation: "g1", TaskID: "t1", WorkspaceID: "w1"}
}
func TestMissingSandboxCannotResumeAsSuccess(t *testing.T) {
	if CanImportObservation(ExecutionObservation{State: "missing"}) {
		t.Fatal("missing observation imported")
	}
	if CanImportObservation(ExecutionObservation{State: "succeeded"}) {
		t.Fatal("empty success imported")
	}
}
func TestObserveOnlyRecoveryDoesNotCreate(t *testing.T) {
	f := &recoveryFake{observations: map[string]ExecutionObservation{"t1": {State: "running"}}}
	r := ObserveOnlyRecovery{Recovery: f}
	got, err := r.Observe(context.Background(), validExecutionRef())
	if err != nil || got.State != "running" {
		t.Fatalf("got %#v err %v", got, err)
	}
	if f.created != 0 || f.observed != 1 {
		t.Fatalf("created=%d observed=%d", f.created, f.observed)
	}
}
func TestExecutionRefRejectsIncomplete(t *testing.T) {
	r := validExecutionRef()
	r.TenantID = 0
	if r.Validate() == nil {
		t.Fatal("accepted cross-boundary ref")
	}
}
