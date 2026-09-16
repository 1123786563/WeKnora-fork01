package workbench

import (
	"context"
	"errors"
	"testing"
)

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

func TestServerAdmissionBindingResolverPersistsTrustedPlatformAndBYOKParentBindings(t *testing.T) {
	resolver := NewServerAdmissionBindingResolver()
	platform, err := resolver.Resolve(context.Background(), 1, "owner", StartInput{TargetID: "platform", BudgetUpper: 10})
	if err != nil {
		t.Fatal(err)
	}
	if platform.Source != "platform_gateway" || platform.Funding != "platform" || platform.Service != "connector" || platform.Revision != 1 {
		t.Fatalf("platform binding=%+v", platform)
	}
	binding := &TrustedAdmissionBinding{ParentRunID: "root", Source: "platform_gateway", Funding: "byok", Service: "model", PriceVersion: "pv-byok", CredentialVersion: 4, Upper: 100, Revision: 2, Status: "final", Dimensions: map[string]int64{"model": 8}}
	byok, err := resolver.Resolve(context.Background(), 1, "owner", StartInput{TargetID: "paseo", Binding: binding})
	if err != nil {
		t.Fatal(err)
	}
	if byok.ParentRunID != "root" || byok.Funding != "byok" || byok.CredentialVersion != 4 || byok.Dimensions["model"] != 8 {
		t.Fatalf("trusted binding=%+v", byok)
	}
}
