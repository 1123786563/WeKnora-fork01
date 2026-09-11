package service

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"testing"
)

type resourceRecoveryFake struct {
	obs   sandbox.ExecutionObservation
	calls int
}

func (f *resourceRecoveryFake) Observe(context.Context, sandbox.ExecutionRef) (sandbox.ExecutionObservation, error) {
	f.calls++
	return f.obs, nil
}
func (f *resourceRecoveryFake) CancelExecution(context.Context, sandbox.ExecutionRef) error {
	return nil
}
func testResource() AgentRunResource {
	return AgentRunResource{TenantID: 1, RunID: "r1", SessionID: "s1", Ref: sandbox.ExecutionRef{TenantID: 1, SessionID: "s1", Provider: "docker", ConfigID: "cfg", InstanceID: "i1", Generation: "g1", TaskID: "t1", WorkspaceID: "w1"}}
}
func TestAgentRunResourceRecoveryStates(t *testing.T) {
	for _, tc := range []struct {
		name, state string
		result      []byte
		wantErr     error
	}{
		{"running", "running", nil, nil}, {"complete", "succeeded", []byte(`{"ok":true}`), nil}, {"unknown", "unknown", nil, ErrSandboxUnavailable}, {"missing", "missing", nil, ErrSandboxUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMemoryAgentRunResourceRepository()
			f := &resourceRecoveryFake{obs: sandbox.ExecutionObservation{State: tc.state, Result: tc.result}}
			_, err := ReconcileExecution(context.Background(), repo, f, testResource())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if f.calls != 1 {
				t.Fatal("observe count")
			}
		})
	}
}
func TestAgentRunResourceRejectsCrossTenantAndGeneration(t *testing.T) {
	repo := NewMemoryAgentRunResourceRepository()
	f := &resourceRecoveryFake{obs: sandbox.ExecutionObservation{State: "succeeded", Result: []byte(`{"ok":true}`)}}
	r := testResource()
	r.Ref.TenantID = 2
	if _, err := ReconcileExecution(context.Background(), repo, f, r); !errors.Is(err, ErrSandboxResourceConflict) {
		t.Fatalf("err=%v", err)
	}
}

func TestAgentRunResourcePersistsUnavailableAndRejectsGenerationChange(t *testing.T) {
	repo := NewMemoryAgentRunResourceRepository()
	r := testResource()
	f := &resourceRecoveryFake{obs: sandbox.ExecutionObservation{State: "missing"}}
	_, err := ReconcileExecution(context.Background(), repo, f, r)
	if !errors.Is(err, ErrSandboxUnavailable) {
		t.Fatalf("err=%v", err)
	}
	saved, err := repo.Get(context.Background(), 1, "r1")
	if err != nil || saved.State != "sandbox_unavailable" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	changed := r
	changed.Ref.Generation = "old-generation"
	_, err = ReconcileExecution(context.Background(), repo, f, changed)
	if !errors.Is(err, ErrSandboxResourceConflict) {
		t.Fatalf("generation err=%v", err)
	}
	protected, err := repo.ProtectsSandbox(context.Background(), sandbox.RemoteSandboxSummary{ID: "i1"})
	if err != nil || !protected {
		t.Fatalf("protected=%v err=%v", protected, err)
	}
}
func TestAgentRunResourceProtectsNonterminal(t *testing.T) {
	repo := NewMemoryAgentRunResourceRepository()
	r := testResource()
	r.State = "running"
	if err := repo.Put(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	ok, err := repo.Protects(context.Background(), 1, "r1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	r.State = "succeeded"
	if err := repo.Put(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	ok, err = repo.Protects(context.Background(), 1, "r1")
	if err != nil || ok {
		t.Fatalf("terminal protected=%v err=%v", ok, err)
	}
}
