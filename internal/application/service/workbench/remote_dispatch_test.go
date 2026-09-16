package workbench

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type integrationUsageGate struct {
	begins, finishes int
	attached         []string
}

func (g *integrationUsageGate) Begin(_ context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	g.begins++
	return commercial.Reservation{ID: req.Key}, nil
}
func (g *integrationUsageGate) Finish(_ context.Context, _ string, _ commercial.UsageFact) error {
	g.finishes++
	return nil
}
func (g *integrationUsageGate) AttachChildRun(_ context.Context, _ uint64, child, parent string) error {
	g.attached = append(g.attached, child+":"+parent)
	return nil
}

type integrationRemoteProvider struct{ starts int }

type missingUsageProvider struct{ starts int }

func (p *missingUsageProvider) Start(context.Context, agentruntime.RunKey, string) (string, error) {
	return "legacy", nil
}
func (p *missingUsageProvider) StartCommand(context.Context, agentruntime.RemoteStartRequest) (string, error) {
	p.starts++
	return "external-missing-usage", nil
}

func (p *integrationRemoteProvider) Start(context.Context, agentruntime.RunKey, string) (string, error) {
	return "legacy", nil
}
func (p *integrationRemoteProvider) StartCommand(context.Context, agentruntime.RemoteStartRequest) (string, error) {
	return "legacy", nil
}
func (p *integrationRemoteProvider) StartCommandWithUsage(_ context.Context, request agentruntime.RemoteStartRequest) (agentruntime.RemoteStartResult, error) {
	p.starts++
	service := request.Fence.UsageService
	if service == "" {
		service = commercial.ServiceConnector
	}
	dimension := commercial.DimensionConnector
	if service == commercial.ServiceModel {
		dimension = commercial.DimensionModel
	}
	return agentruntime.RemoteStartResult{ExternalID: "external-" + request.CommandID, Usage: &agentruntime.RemoteUsageObservation{
		Service: service, Status: commercial.UsageStatusFinal, PriceVersion: "remote-v1", Revision: 1,
		OccurredAt: time.Now().UTC(), Dimensions: map[string]int64{dimension: 1},
	}}, nil
}

func newDispatchIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	for _, ddl := range []string{
		`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, epoch INTEGER NOT NULL, PRIMARY KEY (tenant_id, run_id))`,
		`CREATE TABLE execution_dispatches (tenant_id INTEGER NOT NULL, command_id TEXT NOT NULL, run_id TEXT NOT NULL, attempt_id TEXT NOT NULL, payload_hash TEXT NOT NULL, state TEXT NOT NULL, external_id TEXT NOT NULL DEFAULT '', worker TEXT NOT NULL, epoch INTEGER NOT NULL, lease_until DATETIME, observed_state TEXT NOT NULL DEFAULT '', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (tenant_id, command_id))`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, epoch) VALUES (1, 'run-1', 3)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func integrationFence(funding, service string) agentruntime.Fence {
	return agentruntime.Fence{
		RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-1"}, Owner: "worker-1", Epoch: 3,
		TargetID: "target", WorkspaceRef: "workspace", Prompt: "hello", Provider: "paseo",
		ParentRunID: "root-1", UsageFunding: funding, UsageService: service,
	}
}

func TestRemoteDispatcherUsesProviderUsageAndChildBudgetBinding(t *testing.T) {
	db := newDispatchIntegrationDB(t)
	dispatch := repository.NewExecutionDispatchStore(db)
	gate := &integrationUsageGate{}
	usage, err := NewRemoteUsageService(gate)
	if err != nil {
		t.Fatal(err)
	}
	d := NewRemoteDispatcherWithUsage(dispatch, usage)
	provider := &integrationRemoteProvider{}
	id, err := d.DispatchFence(context.Background(), integrationFence(commercial.FundingPlatform, commercial.ServiceConnector), "cmd-1", "hash-1", time.Minute, provider)
	if err != nil || id != "external-cmd-1" {
		t.Fatalf("dispatch id=%q err=%v", id, err)
	}
	if gate.begins != 1 || gate.finishes != 1 || len(gate.attached) != 1 || gate.attached[0] != "run-1:root-1" {
		t.Fatalf("usage wiring begins=%d finishes=%d attached=%v", gate.begins, gate.finishes, gate.attached)
	}
	if _, err := d.DispatchFence(context.Background(), integrationFence(commercial.FundingPlatform, commercial.ServiceConnector), "cmd-1", "hash-1", time.Minute, provider); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if provider.starts != 1 || gate.finishes != 1 {
		t.Fatalf("replay started=%d finishes=%d", provider.starts, gate.finishes)
	}
}

func TestRemoteDispatcherBYOKModelDoesNotReservePlatformGate(t *testing.T) {
	db := newDispatchIntegrationDB(t)
	dispatch := repository.NewExecutionDispatchStore(db)
	gate := &integrationUsageGate{}
	usage, _ := NewRemoteUsageService(gate)
	d := NewRemoteDispatcherWithUsage(dispatch, usage)
	provider := &integrationRemoteProvider{}
	fence := integrationFence(commercial.FundingBYOK, commercial.ServiceModel)
	fence.UsageDimensions = map[string]int64{commercial.DimensionModel: 10}
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-byok", "hash-byok", time.Minute, provider); err != nil {
		t.Fatal(err)
	}
	if gate.begins != 0 || gate.finishes != 0 {
		t.Fatalf("BYOK model reached platform gate: begins=%d finishes=%d", gate.begins, gate.finishes)
	}
}

func TestRemoteDispatcherReconcilesMissingUsageAfterProviderStart(t *testing.T) {
	db := newDispatchIntegrationDB(t)
	dispatch := repository.NewExecutionDispatchStore(db)
	gate := &integrationUsageGate{}
	usage, _ := NewRemoteUsageService(gate)
	d := NewRemoteDispatcherWithUsage(dispatch, usage)
	provider := &missingUsageProvider{}
	// A provider that returns no trusted usage may have started the process.
	// The claimed record must become reconciled, never remain dangling.
	fence := integrationFence(commercial.FundingPlatform, commercial.ServiceConnector)
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-missing", "hash-missing", time.Minute, provider); err == nil {
		t.Fatal("missing usage unexpectedly completed")
	}
	var state, observed string
	if err := db.Raw(`SELECT state, observed_state FROM execution_dispatches WHERE tenant_id=1 AND command_id='cmd-missing'`).Row().Scan(&state, &observed); err != nil {
		t.Fatal(err)
	}
	if state != "reconciled" || observed != "usage_missing" || provider.starts != 1 {
		t.Fatalf("state=%q observed=%q", state, observed)
	}
}
