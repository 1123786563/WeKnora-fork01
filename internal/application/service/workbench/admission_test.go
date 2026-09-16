package workbench

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
)

type admissionTargetStub struct {
	target    execution.Target
	workspace execution.Workspace
}

func (s *admissionTargetStub) CreateTarget(context.Context, execution.Target, string) error {
	return nil
}
func (s *admissionTargetStub) CreateTargetIfTrusted(context.Context, execution.Target, string) error {
	return nil
}
func (s *admissionTargetStub) GetOwnedTarget(_ context.Context, tenant uint64, owner, id string) (execution.Target, error) {
	if s.target.TenantID != tenant || s.target.OwnerID != owner || s.target.ID != id || s.target.State != "active" {
		return execution.Target{}, repository.ErrExecutionTargetNotFound
	}
	return s.target, nil
}
func (*admissionTargetStub) ListOwnedTargets(context.Context, uint64, string) ([]execution.Target, error) {
	return nil, nil
}
func (*admissionTargetStub) RevokeTarget(context.Context, uint64, string, string) error { return nil }
func (*admissionTargetStub) CreateWorkspace(context.Context, execution.Workspace) error { return nil }
func (s *admissionTargetStub) GetOwnedWorkspace(_ context.Context, tenant uint64, owner, id string) (execution.Workspace, error) {
	if s.workspace.TenantID != tenant || s.target.OwnerID != owner || s.workspace.ID != id || s.workspace.TargetID != s.target.ID {
		return execution.Workspace{}, repository.ErrExecutionTargetNotFound
	}
	return s.workspace, nil
}

func TestPersonalTargetAdmissionUsesTrustedTargetAndRevocation(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	stub := &admissionTargetStub{target: execution.Target{ID: "node_1", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 3}, workspace: execution.Workspace{ID: "ws_1", TenantID: 1, TargetID: "node_1", RootRef: "opaque"}}
	coordinator := NewAdmissionCoordinatorWithTargets(db, repository.NewAgentRunStore(db), stub, NoopTaskBudget{}, nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "node_1", WorkspaceRef: "ws_1", RequestID: "node-request", Text: "hello", BudgetUpper: 10})
	if err != nil {
		t.Fatalf("personal target admission: %v", err)
	}
	if run.Driver != "paseo" || run.TargetID != "node_1" {
		t.Fatalf("unexpected personal run: driver=%q target=%q", run.Driver, run.TargetID)
	}
	stub.target.State = "revoked"
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "node_1", WorkspaceRef: "ws_1", RequestID: "revoked-request", Text: "hello", BudgetUpper: 10})
	if !errors.Is(err, execution.ErrTargetForbidden) {
		t.Fatalf("revoked target admitted: %v", err)
	}
}

func TestAdmissionNeverDispatchesWithoutBudget(t *testing.T) {
	called := false
	deny := errors.New("budget_denied")
	err := admitThenPublish(func() error { return deny }, func() error { called = true; return nil })
	if !errors.Is(err, deny) || called {
		t.Fatal("unfunded execution was published")
	}
}

func TestRequestHashChangesWithImmutableInput(t *testing.T) {
	a := StartInput{SessionID: "s", AgentID: "a", TargetID: "platform", Text: "hello", BudgetUpper: 10}
	b := a
	b.Text = "changed"
	if requestHash(a) == requestHash(b) {
		t.Fatal("request hash ignored immutable input")
	}
}
