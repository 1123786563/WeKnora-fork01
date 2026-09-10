package service

import (
	"context"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
	"time"
)

type lifecycleStoreFake struct {
	cancelled []agentruntime.RunKey
	deleted   []string
	actor     string
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

func (f *lifecycleStoreFake) ApplyDecision(_ context.Context, _ agentruntime.RunKey, actor string, _ agentruntime.Decision) (agentruntime.Run, error) {
	f.actor = actor
	return agentruntime.Run{}, nil
}

func TestResolveUsesTenantAPIKeySessionOwner(t *testing.T) {
	f := &lifecycleStoreFake{}
	s := NewAgentRunService(f)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalAPITenant, ID: "99"})
	ctx = types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KeyID: 99})
	_, err := s.Resolve(ctx, agentruntime.RunKey{TenantID: 7, RunID: "r"}, agentruntime.Decision{PendingID: "p", DecisionID: "d", Action: "terminate", Reason: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	if f.actor != "api_tenant_key:7:99" {
		t.Fatalf("actor=%q", f.actor)
	}
}
