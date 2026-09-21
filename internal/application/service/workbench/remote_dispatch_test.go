package workbench

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
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
	dimensions := request.Fence.UsageDimensions
	if dimensions == nil {
		dimension := commercial.DimensionConnector
		if service == commercial.ServiceModel {
			dimension = commercial.DimensionModel
		}
		dimensions = map[string]int64{dimension: 1}
	}
	revision := request.Fence.UsageRevision
	if revision <= 0 {
		revision = 1
	}
	price := request.Fence.UsagePriceVersion
	if price == "" {
		price = "remote-v1"
	}
	return agentruntime.RemoteStartResult{ExternalID: "external-" + request.CommandID, Usage: &agentruntime.RemoteUsageObservation{
		Service: service, Status: commercial.UsageStatusFinal, PriceVersion: price, Revision: revision,
		OccurredAt: time.Now().UTC(), Dimensions: dimensions,
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

type remoteUsageGateway struct{}

func (remoteUsageGateway) ApplyBenefit(context.Context, commercial.BenefitRequest) (commercial.BenefitReceipt, error) {
	return commercial.BenefitReceipt{ExternalID: "benefit"}, nil
}
func (remoteUsageGateway) FindBenefit(context.Context, string) (commercial.BenefitReceipt, error) {
	return commercial.BenefitReceipt{ExternalID: "benefit"}, nil
}
func (remoteUsageGateway) RevokeBenefit(context.Context, string, commercial.Credits) error {
	return nil
}
func (remoteUsageGateway) Settle(context.Context, commercial.Settlement) (commercial.SettlementReceipt, error) {
	return commercial.SettlementReceipt{ExternalID: "settlement"}, nil
}
func (remoteUsageGateway) ConfirmSettlement(context.Context, string) (commercial.SettlementReceipt, error) {
	return commercial.SettlementReceipt{ExternalID: "settlement", Watermark: "w2"}, nil
}

func newRealUsageDispatch(t *testing.T) (*gorm.DB, *commercialsvc.ExecutionGateService) {
	t.Helper()
	db := newDispatchIntegrationDB(t)
	if err := db.AutoMigrate(&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{}, &repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{}, &repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	if err := db.Create(&repocommercial.BudgetAccountRow{TenantID: 1, VerifiedMicro: 100000, Watermark: "w1", Version: 1, VerifiedUntil: end}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.TaskBudgetRow{TenantID: 1, RunID: "root-1", LimitMicro: 100000, Deadline: end, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	expiry := end
	if err := db.Create(&repocommercial.BudgetLotRow{TenantID: 1, LotID: "lot-1", RemainingMicro: 100000, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	gate, err := commercialsvc.NewExecutionGateService(db, remoteUsageGateway{})
	if err != nil {
		t.Fatal(err)
	}
	gate, err = gate.WithRates(func(version string) (commercial.PriceVersionRates, error) {
		return commercial.PriceVersionRates{Version: version, Rates: map[string]commercial.DimensionRate{commercial.DimensionConnector: {RateMicro: 100, Units: 1}, commercial.DimensionModel: {RateMicro: 100, Units: 1}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, gate
}

func TestRemoteDispatcherUsesRealExecutionGateAndLateFinalReplayIsIdempotent(t *testing.T) {
	db, gate := newRealUsageDispatch(t)
	dispatch := repository.NewExecutionDispatchStore(db)
	usage, err := NewRemoteUsageServiceWithDB(gate, db)
	if err != nil {
		t.Fatal(err)
	}
	d := NewRemoteDispatcherWithUsage(dispatch, usage)
	provider := &integrationRemoteProvider{}
	fence := integrationFence(commercial.FundingPlatform, commercial.ServiceConnector)
	fence.UsageUpper = 1000
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-real", "hash-real", time.Minute, provider); err != nil {
		t.Fatal(err)
	}
	// The same dispatch receipt is an idempotent replay and never starts or settles twice.
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-real", "hash-real", time.Minute, provider); err != nil {
		t.Fatal(err)
	}
	var usages int64
	if err := db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "cmd-real").Count(&usages).Error; err != nil {
		t.Fatal(err)
	}
	if usages != 1 || provider.starts != 1 {
		t.Fatalf("usage rows=%d provider starts=%d", usages, provider.starts)
	}
}

type unsupportedRemoteProvider struct{}

func (unsupportedRemoteProvider) Start(context.Context, agentruntime.RunKey, string) (string, error) {
	return "", nil
}

func TestRemoteDispatcherReconcilesUnsupportedProviderAfterClaim(t *testing.T) {
	db := newDispatchIntegrationDB(t)
	dispatch := repository.NewExecutionDispatchStore(db)
	usage, err := NewRemoteUsageService(&integrationUsageGate{})
	if err != nil {
		t.Fatal(err)
	}
	d := NewRemoteDispatcherWithUsage(dispatch, usage)
	fence := integrationFence(commercial.FundingPlatform, commercial.ServiceConnector)
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-unsupported", "hash-unsupported", time.Minute, unsupportedRemoteProvider{}); err == nil {
		t.Fatal("unsupported provider unexpectedly succeeded")
	}
	var state, observed string
	if err := db.Raw(`SELECT state, observed_state FROM execution_dispatches WHERE tenant_id=1 AND command_id='cmd-unsupported'`).Row().Scan(&state, &observed); err != nil {
		t.Fatal(err)
	}
	if state != "unknown" || observed != "provider_capability_missing" {
		t.Fatalf("state=%q observed=%q", state, observed)
	}
}

func TestRemoteUsageRealGateLateFinalAfterNonBillableObservation(t *testing.T) {
	db, gate := newRealUsageDispatch(t)
	usage, err := NewRemoteUsageServiceWithDB(gate, db)
	if err != nil {
		t.Fatal(err)
	}
	fence := integrationFence(commercial.FundingPlatform, commercial.ServiceConnector)
	fence.UsageUpper = 1000
	fence.UsageDimensions = map[string]int64{commercial.DimensionConnector: 1}
	handle, err := usage.BeginRemote(context.Background(), fence, "late-real")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{commercial.UsageStatusUnknown, commercial.UsageStatusPartial, commercial.UsageStatusDisplayOnly} {
		if err := usage.FinishRemoteObservation(context.Background(), handle, &agentruntime.RemoteUsageObservation{Status: status}); err == nil {
			t.Fatalf("status %q settled", status)
		}
	}
	final := &agentruntime.RemoteUsageObservation{Service: commercial.ServiceConnector, PriceVersion: "remote-v1", Revision: 1, Status: commercial.UsageStatusFinal, Dimensions: map[string]int64{commercial.DimensionConnector: 1}, OccurredAt: time.Now().UTC()}
	if err := usage.ReconcileRemoteObservation(context.Background(), fence, "late-real", final); err != nil {
		t.Fatal(err)
	}
	if err := usage.ReconcileRemoteObservation(context.Background(), fence, "late-real", final); err != nil {
		t.Fatal(err)
	}
	var usages, reservations int64
	if err := db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "late-real").Count(&usages).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&repocommercial.ReservationRow{}).Where("tenant_id = ? AND key = ?", 1, "late-real:late-real:1").Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if usages != 1 || reservations != 1 {
		t.Fatalf("usage facts=%d reservations=%d", usages, reservations)
	}
}

type unknownUsageProvider struct{}

func (unknownUsageProvider) Start(context.Context, agentruntime.RunKey, string) (string, error) {
	return "", nil
}
func (unknownUsageProvider) StartCommand(context.Context, agentruntime.RemoteStartRequest) (string, error) {
	return "external-unknown", nil
}
func (unknownUsageProvider) StartCommandWithUsage(_ context.Context, request agentruntime.RemoteStartRequest) (agentruntime.RemoteStartResult, error) {
	return agentruntime.RemoteStartResult{ExternalID: "external-unknown", Usage: &agentruntime.RemoteUsageObservation{Service: commercial.ServiceConnector, PriceVersion: "remote-v1", Revision: 1, Status: commercial.UsageStatusUnknown}}, nil
}

func TestRemoteDispatcherUnknownThenFreshWorkerLateFinalIsSingleCharge(t *testing.T) {
	db, gate := newRealUsageDispatch(t)
	usage, err := NewRemoteUsageServiceWithDB(gate, db)
	if err != nil {
		t.Fatal(err)
	}
	d := NewRemoteDispatcherWithUsage(repository.NewExecutionDispatchStore(db), usage)
	fence := integrationFence(commercial.FundingPlatform, commercial.ServiceConnector)
	fence.UsageUpper = 1000
	fence.UsageDimensions = map[string]int64{commercial.DimensionConnector: 1}
	if _, err := d.DispatchFence(context.Background(), fence, "cmd-unknown", "hash-unknown", time.Minute, unknownUsageProvider{}); err == nil {
		t.Fatal("unknown usage unexpectedly succeeded")
	}
	var observed string
	if err := db.Raw(`SELECT observed_state FROM execution_dispatches WHERE tenant_id=1 AND command_id='cmd-unknown'`).Row().Scan(&observed); err != nil {
		t.Fatal(err)
	}
	if observed != "usage_unknown" {
		t.Fatalf("observed=%q", observed)
	}
	// A fresh dispatcher/service boundary reconciles the retained durable key.
	freshUsage, err := NewRemoteUsageServiceWithDB(gate, db)
	if err != nil {
		t.Fatal(err)
	}
	fresh := NewRemoteDispatcherWithUsage(repository.NewExecutionDispatchStore(db), freshUsage)
	final := &agentruntime.RemoteUsageObservation{Service: commercial.ServiceConnector, PriceVersion: "remote-v1", Revision: 1, Status: commercial.UsageStatusFinal, Dimensions: map[string]int64{commercial.DimensionConnector: 1}, OccurredAt: time.Now().UTC()}
	if err := fresh.ReconcileLateUsage(context.Background(), fence, "cmd-unknown", final); err != nil {
		t.Fatal(err)
	}
	if err := fresh.ReconcileLateUsage(context.Background(), fence, "cmd-unknown", final); err != nil {
		t.Fatal(err)
	}
	var usages int64
	if err := db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "cmd-unknown").Count(&usages).Error; err != nil {
		t.Fatal(err)
	}
	if usages != 1 {
		t.Fatalf("usage facts=%d", usages)
	}
}
