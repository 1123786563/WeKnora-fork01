package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
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

func TestProductionAdmissionPersistsPlatformBYOKParentBinding(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	if err := db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc')").Error; err != nil {
		t.Fatal(err)
	}
	resolver := AdmissionBindingResolverFunc(func(_ context.Context, _ uint64, _ string, in StartInput) (TrustedAdmissionBinding, error) {
		if in.Binding == nil {
			return TrustedAdmissionBinding{}, errors.New("trusted binding missing")
		}
		return *in.Binding, nil
	})
	coordinator, err := NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), &retrySafeBudget{}, nil, resolver)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	platform := &TrustedAdmissionBinding{Source: "platform_gateway", Funding: "platform", Service: "connector", PriceVersion: "pv-platform", CredentialVersion: 1, Upper: 10, Revision: 1, Status: "final", Dimensions: map[string]int64{"connector": 1}}
	byok := &TrustedAdmissionBinding{ParentRunID: "root-run", Source: "platform_gateway", Funding: "byok", Service: "model", PriceVersion: "pv-byok", CredentialVersion: 7, Upper: 20, Revision: 2, Status: "final", Dimensions: map[string]int64{"model": 8}}
	if _, err := coordinator.Start(ctx, StartInput{SessionID: "s1", TargetID: "platform", RequestID: "prod-platform", Text: "platform", BudgetUpper: 1, Binding: platform}); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Start(ctx, StartInput{SessionID: "s2", TargetID: "paseo", RequestID: "prod-byok", Text: "byok", BudgetUpper: 1, Binding: byok}); err != nil {
		t.Fatal(err)
	}
	var snapshots []string
	if err := db.Table("agent_runs").Where("request_id IN ?", []string{"prod-platform", "prod-byok"}).Pluck("snapshot", &snapshots).Error; err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots=%d", len(snapshots))
	}
	for _, raw := range snapshots {
		var got map[string]any
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatal(err)
		}
		if got["usage_source"] == "client" || got["usage_funding"] == "client" {
			t.Fatalf("client binding survived: %#v", got)
		}
	}
	var got map[string]any
	for _, raw := range snapshots {
		var candidate map[string]any
		_ = json.Unmarshal([]byte(raw), &candidate)
		if candidate["usage_funding"] == "byok" {
			got = candidate
		}
	}
	if got["parent_run_id"] != "root-run" || got["credential_version"] != float64(7) || got["price_version"] != "pv-byok" {
		t.Fatalf("BYOK binding not persisted: %#v", got)
	}
}
