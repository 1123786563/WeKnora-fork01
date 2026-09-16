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
	target           execution.Target
	workspace        execution.Workspace
	lookups          int
	afterFirstLookup func(*admissionTargetStub)
}

func (s *admissionTargetStub) CreateTarget(context.Context, execution.Target, string) error {
	return nil
}
func (s *admissionTargetStub) CreateTargetIfTrusted(context.Context, execution.Target, string) error {
	return nil
}
func (s *admissionTargetStub) GetOwnedTarget(_ context.Context, tenant uint64, owner, id string) (execution.Target, error) {
	s.lookups++
	if s.target.TenantID != tenant || s.target.OwnerID != owner || s.target.ID != id || s.target.State != "active" {
		return execution.Target{}, repository.ErrExecutionTargetNotFound
	}
	result := s.target
	if s.lookups == 1 && s.afterFirstLookup != nil {
		s.afterFirstLookup(s)
	}
	return result, nil
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

func TestPersonalTargetAdmissionFencesTenantOwnerWorkspaceAndCredential(t *testing.T) {
	base := execution.Target{ID: "node-1", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 4}
	cases := []struct {
		name      string
		target    execution.Target
		workspace execution.Workspace
		ctxTenant uint64
		ctxOwner  string
		want      error
	}{
		{"wrong tenant", base, execution.Workspace{ID: "ws-1", TenantID: 1, TargetID: base.ID}, 2, "u1", execution.ErrTargetForbidden},
		{"wrong owner", base, execution.Workspace{ID: "ws-1", TenantID: 1, TargetID: base.ID}, 1, "u2", execution.ErrTargetForbidden},
		{"wrong workspace target", base, execution.Workspace{ID: "ws-1", TenantID: 1, TargetID: "other"}, 1, "u1", execution.ErrTargetForbidden},
		{"zero credential", execution.Target{ID: base.ID, TenantID: 1, OwnerID: "u1", Kind: base.Kind, State: "active"}, execution.Workspace{ID: "ws-1", TenantID: 1, TargetID: base.ID}, 1, "u1", execution.ErrTargetForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openAdmissionConcurrencyDB(t)
			stub := &admissionTargetStub{target: tc.target, workspace: tc.workspace}
			coordinator := NewAdmissionCoordinatorWithTargets(db, repository.NewAgentRunStore(db), stub, NoopTaskBudget{}, nil)
			ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, tc.ctxTenant), types.UserIDContextKey, tc.ctxOwner)
			_, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: base.ID, WorkspaceRef: "ws-1", RequestID: "request-" + tc.name, Text: "hello", BudgetUpper: 10})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPersonalTargetAdmissionReResolvesAfterPendingRequest(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	stub := &admissionTargetStub{target: execution.Target{ID: "node-1", TenantID: 1, OwnerID: "u1", Kind: "personal_node", State: "active", CredentialVersion: 2}, workspace: execution.Workspace{ID: "ws-1", TenantID: 1, TargetID: "node-1"}}
	stub.afterFirstLookup = func(s *admissionTargetStub) { s.target.State = "revoked" }
	coordinator := NewAdmissionCoordinatorWithTargets(db, repository.NewAgentRunStore(db), stub, NoopTaskBudget{}, nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	_, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "node-1", WorkspaceRef: "ws-1", RequestID: "race-request", Text: "hello", BudgetUpper: 10})
	if !errors.Is(err, execution.ErrTargetForbidden) {
		t.Fatalf("revoked target admitted after final re-resolution: %v", err)
	}
	if stub.lookups != 2 {
		t.Fatalf("expected initial and final target lookups, got %d", stub.lookups)
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
