package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCanceledRunKeepsWorkspaceWhileStopUnknown(t *testing.T) {
	if MayReleaseWorkspace("canceled", ExecutionObservation{ProcessState: "unknown", Fresh: true}) {
		t.Fatal("released unconfirmed workspace")
	}
	if !MayReleaseWorkspace("canceled", ExecutionObservation{ProcessState: "exited", Fresh: true}) {
		t.Fatal("confirmed exit did not release workspace")
	}
	if !MayReleaseWorkspace("failed", ExecutionObservation{ProcessState: "destroyed", Fresh: true}) {
		t.Fatal("destroyed failed run did not release workspace")
	}
	if MayReleaseWorkspace("canceled", ExecutionObservation{ProcessState: "exited", Fresh: false}) {
		t.Fatal("stale observation released workspace")
	}
}

type fakeRemoteControl struct {
	mu        sync.Mutex
	cancels   int
	state     string
	cancelErr error
}

func (f *fakeRemoteControl) Cancel(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels++
	return f.cancelErr
}
func (f *fakeRemoteControl) Observe(_ context.Context, _ string) (ExecutionObservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return ExecutionObservation{ProcessState: f.state, Fresh: true}, nil
}

func TestStopRemoteExecutionSeparatesAcceptedStopFromConfirmedExit(t *testing.T) {
	remote := &fakeRemoteControl{state: "running"}
	result, err := StopRemoteExecution(context.Background(), remote, "ext-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StopUnconfirmed || result.Confirmed {
		t.Fatalf("result=%+v", result)
	}
	remote.state = "exited"
	result, err = StopRemoteExecution(context.Background(), remote, "ext-1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StopConfirmed || !result.Confirmed {
		t.Fatalf("result=%+v", result)
	}
	if remote.cancels != 2 {
		t.Fatalf("cancel calls=%d", remote.cancels)
	}
}

func TestStopRemoteExecutionDoesNotRetryAfterUnknownCancel(t *testing.T) {
	remote := &fakeRemoteControl{state: "unknown", cancelErr: errors.New("offline")}
	result, err := StopRemoteExecution(context.Background(), remote, "ext-1", 10*time.Millisecond)
	if err == nil || result.State != StopUnknown {
		t.Fatalf("err=%v result=%+v", err, result)
	}
	if remote.cancels != 1 {
		t.Fatalf("cancel calls=%d", remote.cancels)
	}
}

func TestWorkspaceLeaseFencesOwnerAndEpoch(t *testing.T) {
	locks := NewWorkspaceLocks()
	first, err := locks.Acquire(7, "ws-1", "run-1", "worker-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locks.Acquire(7, "ws-1", "run-2", "worker-b", time.Minute); !errors.Is(err, ErrWorkspaceLocked) {
		t.Fatalf("err=%v", err)
	}
	if err := locks.Renew(first, "worker-b", time.Minute); !errors.Is(err, ErrWorkspaceLeaseLost) {
		t.Fatalf("err=%v", err)
	}
	if err := locks.Release(first, "worker-b"); !errors.Is(err, ErrWorkspaceLeaseLost) {
		t.Fatalf("err=%v", err)
	}
	if err := locks.Release(first, "worker-a"); err != nil {
		t.Fatal(err)
	}
	second, err := locks.Acquire(7, "ws-1", "run-2", "worker-b", time.Minute)
	if err != nil || second.Epoch <= first.Epoch {
		t.Fatalf("first=%+v second=%+v err=%v", first, second, err)
	}
}
