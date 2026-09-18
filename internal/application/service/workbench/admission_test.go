package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
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

// W11 follow-up accepted in W12: the run snapshot persists space_id so the
// owned execution list can navigate without a second lookup, while the
// request hash stays stable when a retry omits the navigation hint.
func TestAdmissionSnapshotPersistsSpaceIDWithoutHashImpact(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, nil, nil)
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "agent-1", TargetID: "platform", SpaceID: "space-7", RequestID: "space-snapshot", Text: "hello", BudgetUpper: 100}
	_, err := coordinator.Start(ctx, in)
	require.NoError(t, err)

	var raw string
	require.NoError(t, db.Table("agent_runs").Where("session_id = ?", "s1").Select("snapshot").Scan(&raw).Error)
	var snapshot map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &snapshot))
	require.Equal(t, "space-7", snapshot["space_id"])

	// A retry without the space hint still reconciles onto the same run.
	retry := in
	retry.SpaceID = ""
	run, err := coordinator.Start(ctx, retry)
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
}

func TestServerAdmissionBindingResolverRejectsRequestScopedBinding(t *testing.T) {
	resolver := NewDatabaseAdmissionBindingResolver(nil)
	platform, err := resolver.Resolve(context.Background(), 1, "owner", StartInput{TargetID: "platform", BudgetUpper: 10})
	if err != nil {
		t.Fatal(err)
	}
	if platform.Source != "platform_gateway" || platform.Funding != "platform" || platform.Service != "connector" || platform.Revision != 1 {
		t.Fatalf("platform binding=%+v", platform)
	}
	if _, err := resolver.Resolve(context.Background(), 1, "owner", StartInput{TargetID: "paseo", Binding: &TrustedAdmissionBinding{Funding: "byok"}}); !errors.Is(err, execution.ErrTargetUntrusted) {
		t.Fatalf("request binding err=%v", err)
	}
}

func TestProductionAdmissionPersistsPlatformBYOKParentBinding(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	if err := db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc')").Error; err != nil {
		t.Fatal(err)
	}
	targetStore := repository.NewExecutionTargetStore(db)
	if err := targetStore.CreateTarget(context.Background(), execution.Target{ID: "paseo", TenantID: 1, OwnerID: "u1", Kind: "managed_node", State: "active", CredentialVersion: 7, RuntimeID: "runtime-1", ExternalTargetID: "external-1", UsageBinding: execution.UsageBinding{ParentRunID: "root-run", Source: "platform_gateway", Funding: "byok", Service: "model", PriceVersion: "pv-byok", Revision: 2, Status: "final", Dimensions: map[string]int64{"model": 8}}}, "root"); err != nil {
		t.Fatal(err)
	}
	resolver := NewDatabaseAdmissionBindingResolver(targetStore)
	coordinator, err := NewAdmissionCoordinatorWithBinding(db, repository.NewAgentRunStore(db), &retrySafeBudget{}, nil, resolver)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	if _, err := coordinator.Start(ctx, StartInput{SessionID: "s1", TargetID: "platform", RequestID: "prod-platform", Text: "platform", BudgetUpper: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Start(ctx, StartInput{SessionID: "s2", TargetID: "paseo", RequestID: "prod-byok", Text: "byok", BudgetUpper: 1}); err != nil {
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
