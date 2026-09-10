package service

import (
	"context"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"testing"
	"time"
)

type lifecycleStoreFake struct {
	cancelled []agentruntime.RunKey
	deleted   []string
}

func (f *lifecycleStoreFake) Admit(context.Context, agentruntime.Admission) (agentruntime.Run, error) {
	return agentruntime.Run{}, nil
}
func (f *lifecycleStoreFake) Get(context.Context, agentruntime.RunKey) (agentruntime.Run, error) {
	return agentruntime.Run{}, nil
}
func (f *lifecycleStoreFake) Claim(context.Context, agentruntime.RunKey, string, time.Duration) (agentruntime.Fence, error) {
	return agentruntime.Fence{}, nil
}
func (f *lifecycleStoreFake) Renew(context.Context, agentruntime.Fence, time.Duration) error {
	return nil
}
func (f *lifecycleStoreFake) Scan(context.Context, int) ([]agentruntime.RunKey, error) {
	return nil, nil
}
func (f *lifecycleStoreFake) SaveCheckpoint(context.Context, agentruntime.Fence, agentruntime.CheckpointRecord) error {
	return nil
}
func (f *lifecycleStoreFake) SetStatus(context.Context, agentruntime.Fence, string, string) error {
	return nil
}
func (f *lifecycleStoreFake) LoadCheckpoint(context.Context, agentruntime.RunKey) (agentruntime.CheckpointRecord, error) {
	return agentruntime.CheckpointRecord{}, nil
}
func (f *lifecycleStoreFake) CancelRun(_ context.Context, key agentruntime.RunKey, _ string) error {
	f.cancelled = append(f.cancelled, key)
	return nil
}
func (f *lifecycleStoreFake) DeleteSessionRuns(_ context.Context, tenant uint64, session string) error {
	f.deleted = append(f.deleted, session)
	return nil
}
func TestAgentRunLifecycleCancelIsDurable(t *testing.T) {
	f := &lifecycleStoreFake{}
	s := NewAgentRunService(f)
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	if err := s.Cancel(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if len(f.cancelled) != 1 {
		t.Fatalf("cancel calls=%d", len(f.cancelled))
	}
}
func TestAgentRunLifecycleDeleteSessionRuns(t *testing.T) {
	f := &lifecycleStoreFake{}
	s := NewAgentRunService(f)
	if err := s.DeleteSessionRuns(context.Background(), 7, "s1"); err != nil {
		t.Fatal(err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "s1" {
		t.Fatalf("deleted=%v", f.deleted)
	}
}
