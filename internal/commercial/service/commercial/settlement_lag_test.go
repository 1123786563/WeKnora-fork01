// #90 / Lago 18 — service face of the pricing-lag protections: Finish above
// the reserved upper bound must (a) keep the actual usage recorded, (b)
// durably suspend the run so later Begins are refused, and (c) leave other
// runs alone; lag alerts surface settlements older than the 15-minute ops
// threshold or carrying an ingest error (events.errors).
package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
)

// TestFinishAbnormalCostSuspendsTaskAndKeepsUsage pins AC4 end-to-end: the
// settle stays recorded (actual usage is never discarded), the dispatcher
// sees ErrAbnormalCost, and the run's next Begin is refused by the durable
// task suspension while a sibling run keeps reserving.
func TestFinishAbnormalCostSuspendsTaskAndKeepsUsage(t *testing.T) {
	gate, db := newTestExecutionGate(t)
	ctx := context.Background()
	// The over-ceiling settle drives unreflected far past the seeded
	// account face; widen it so the sibling-run check below exercises the
	// SUSPENSION scope, not account exhaustion.
	if err := db.Model(&repocommercial.BudgetAccountRow{}).
		Where("tenant_id = ?", settlementTenant).
		Updates(map[string]any{"verified_micro": 20_000_000}).Error; err != nil {
		t.Fatal(err)
	}

	// Upper 1000 micro; the fact settles connector_calls=5000 at 1000/1.
	res, err := gate.Begin(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: "res_abn", Upper: domain.Credits(1000),
		Deadline: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.Finish(ctx, res.ID, gateConnectorFact("call_abn", 5000)); !errors.Is(err, domain.ErrAbnormalCost) {
		t.Fatalf("over-ceiling finish: want ErrAbnormalCost, got %v", err)
	}

	var usage repocommercial.UsageRow
	if err := db.Where("tenant_id = ? AND call_id = ?", settlementTenant, "call_abn").First(&usage).Error; err != nil {
		t.Fatalf("actual usage must stay recorded: %v", err)
	}

	if _, err := gate.Begin(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: "res_abn_next", Upper: domain.Credits(1000),
		Deadline: time.Now().UTC().Add(time.Hour),
	}); !errors.Is(err, repocommercial.ErrTaskSuspended) && !errors.Is(err, domain.ErrInsufficientBudgetGate) {
		t.Fatalf("suspended run Begin must be refused, got %v", err)
	}
	if err := db.Create(&repocommercial.TaskBudgetRow{
		TenantID: settlementTenant, RunID: "r2", LimitMicro: 2_000_000,
		Deadline: time.Now().UTC().Add(time.Hour), Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Begin(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r2", Key: "res_abn_other", Upper: domain.Credits(1000),
		Deadline: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("other run must keep reserving: %v", err)
	}

	var task repocommercial.TaskBudgetRow
	if err := db.Where("tenant_id = ? AND run_id = ?", settlementTenant, "r1").First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.SuspendedReason != "abnormal_cost" {
		t.Fatalf("suspension must be durable, got %+v", task)
	}
}

// TestScanLagAlerts pins AC2: unconfirmed settlements older than 15 minutes
// alert; so does any settlement carrying an ingest error (events.errors)
// regardless of age; fresh and confirmed settlements never do.
func TestScanLagAlerts(t *testing.T) {
	svc, _, db, _ := testSettlementService(t)
	ctx := context.Background()
	now := time.Now().UTC()

	seed := func(key, state string, age time.Duration, lastErr string) {
		t.Helper()
		if err := db.Create(&SettlementRecord{
			Key: key, TenantID: settlementTenant, CallID: "c-" + key, AttemptID: "a",
			ReservationKey: "res-" + key, RunID: "r1", AmountMicro: 1, Revision: 1,
			State: state, LastError: lastErr, OccurredAt: now.Add(-age), UpdatedAt: now.Add(-age),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed("settle:101:slow", domain.SettlementStateDispatched, 16*time.Minute, "")
	seed("settle:101:fresh", domain.SettlementStateDispatched, 6*time.Minute, "")
	seed("settle:101:ingesterr", domain.SettlementStateDispatched, time.Minute, "422 validation")
	seed("settle:101:done", domain.SettlementStateConfirmed, time.Hour, "")

	alerts, err := svc.ScanLagAlerts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, a := range alerts {
		got[a.SettlementKey] = true
	}
	if !got["settle:101:slow"] || !got["settle:101:ingesterr"] {
		t.Fatalf("alerts must cover the 15-minute lag and the ingest error, got %v", got)
	}
	if got["settle:101:fresh"] || got["settle:101:done"] {
		t.Fatalf("fresh/confirmed settlements must not alert, got %v", got)
	}
}

// TestDispatchRecordsLastError pins the events.errors face: a failed
// settlement dispatch records its last error durably so the alert scan can
// surface provider rejections, not just age.
func TestDispatchRecordsLastError(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	ctx := context.Background()
	seedSettlementBudget(t, db)

	if _, err := budget.Reserve(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: "res_err", Upper: domain.Credits(settleHold),
		Deadline: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Finalize(ctx, settlementFinalFact("call_err", 1, 80), "res_err"); err != nil {
		t.Fatal(err)
	}

	gw.setSettleErr(errors.New("422 events batch rejected"))
	if err := svc.Dispatch(ctx); err == nil {
		t.Fatal("dispatch must surface the gateway error")
	}
	var rec SettlementRecord
	if err := db.Where("call_id = ?", "call_err").First(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if rec.LastError == "" {
		t.Fatal("failed dispatch must record its last error durably")
	}
}
