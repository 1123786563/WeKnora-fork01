// execution_gate_test.go pins the REAL gate's Begin->Finish contract: Begin
// marks the reservation dispatched (the outbound intent is persisted BEFORE
// the call), and Finish must settle that dispatched reservation — external
// spend may already exist, so the settlement path is the ONLY correct exit,
// never a release. (T12 fix-round QF-1: trySettleReservationHold used to
// accept only held/settled, making every production-gate Finish fail with
// reservation_key_conflict.)
package commercial

import (
	"context"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newExecutionGateTestDB builds the real gate over its own sqlite ledger.
func newExecutionGateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{}, &repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{}, &repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{}, &repocommercial.UsageCurrentRow{}, &repocommercial.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(time.Hour)
	expiry := end
	for _, row := range []any{
		&repocommercial.BudgetAccountRow{TenantID: settlementTenant, VerifiedMicro: 1_000_000, Watermark: "w1", Version: 1, VerifiedUntil: end},
		&repocommercial.TaskBudgetRow{TenantID: settlementTenant, RunID: "r1", LimitMicro: 2_000_000, Deadline: end, Version: 1},
		&repocommercial.BudgetLotRow{TenantID: settlementTenant, LotID: "lot1", RemainingMicro: 1_000_000, ExpiresAt: &expiry, IssuedAt: time.Now().UTC()},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func newTestExecutionGate(t *testing.T) (*ExecutionGateService, *gorm.DB) {
	t.Helper()
	db := newExecutionGateTestDB(t)
	gate, err := NewExecutionGateService(db, &settlementStubGateway{})
	if err != nil {
		t.Fatal(err)
	}
	gate, err = gate.WithRates(func(version string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: version, Rates: map[string]domain.DimensionRate{
			domain.DimensionConnector: {RateMicro: 1000, Units: 1},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return gate, db
}

func gateConnectorFact(call string, calls int64) domain.UsageFact {
	return domain.UsageFact{
		TenantID: settlementTenant, RunID: "r1", CallID: call, AttemptID: call,
		Funding: domain.FundingPlatform, Service: domain.ServiceConnector, PriceVersion: "v1",
		Revision: 1, OccurredAt: time.Now().UTC(),
		Dimensions: map[string]int64{domain.DimensionConnector: calls}, Status: domain.UsageStatusFinal,
	}
}

// TestExecutionGateSettlesDispatchedReservation is the QF-1 regression: Begin
// flips the reservation to dispatched, and Finish — the only sanctioned exit
// of a dispatched reservation — settles it: the usage fact lands, the hold is
// converted (consumed to unreflected, unused released), and the reservation
// ends settled. An identical duplicate Finish is an idempotent replay.
func TestExecutionGateSettlesDispatchedReservation(t *testing.T) {
	gate, db := newTestExecutionGate(t)
	ctx := context.Background()

	res, err := gate.Begin(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: "res_gate",
		Upper: 2000, Deadline: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if res.State != domain.ReservationStateDispatched {
		t.Fatalf("reservation state after Begin = %q, want dispatched", res.State)
	}

	if err := gate.Finish(ctx, res.ID, gateConnectorFact("act_gate", 1)); err != nil {
		t.Fatalf("Finish of a Begin-dispatched reservation failed: %v", err)
	}
	var usage int64
	db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act_gate").Count(&usage)
	if usage != 1 {
		t.Fatalf("usage rows = %d, want exactly 1", usage)
	}
	var row repocommercial.ReservationRow
	if err := db.Where("tenant_id = ? AND key = ?", settlementTenant, "res_gate").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != domain.ReservationStateSettled {
		t.Fatalf("reservation state after Finish = %q, want settled", row.State)
	}
	var acct repocommercial.BudgetAccountRow
	if err := db.Where("tenant_id = ?", settlementTenant).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	if acct.HeldMicro != 0 {
		t.Fatalf("hold not converted: held_micro = %d, want 0", acct.HeldMicro)
	}
	if acct.UnreflectedMicro != 1000 {
		t.Fatalf("unreflected = %d, want the consumed 1000", acct.UnreflectedMicro)
	}

	// Identical duplicate delivery is an idempotent replay, not a conflict.
	if err := gate.Finish(ctx, res.ID, gateConnectorFact("act_gate", 1)); err != nil {
		t.Fatalf("duplicate Finish not idempotent: %v", err)
	}
	db.Model(&repocommercial.UsageRow{}).Where("call_id = ?", "act_gate").Count(&usage)
	if usage != 1 {
		t.Fatalf("usage rows after replay = %d, want 1", usage)
	}
}

// TestExecutionGateZeroUsageFinishSettlesDispatchedReservation: the zero-usage
// release path (a provably unstarted dispatch's pre-allocation) also runs
// through Finish on a dispatched reservation — spend stays zero, the hold
// converts at zero, and the reservation still ends settled.
func TestExecutionGateZeroUsageFinishSettlesDispatchedReservation(t *testing.T) {
	gate, db := newTestExecutionGate(t)
	ctx := context.Background()

	res, err := gate.Begin(ctx, domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: "res_gate0",
		Upper: 2000, Deadline: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.Finish(ctx, res.ID, gateConnectorFact("act_gate0", 0)); err != nil {
		t.Fatalf("zero-usage Finish of a dispatched reservation failed: %v", err)
	}
	var row repocommercial.ReservationRow
	if err := db.Where("tenant_id = ? AND key = ?", settlementTenant, "res_gate0").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != domain.ReservationStateSettled {
		t.Fatalf("reservation state = %q, want settled", row.State)
	}
	var acct repocommercial.BudgetAccountRow
	if err := db.Where("tenant_id = ?", settlementTenant).First(&acct).Error; err != nil {
		t.Fatal(err)
	}
	if acct.HeldMicro != 0 || acct.UnreflectedMicro != 0 {
		t.Fatalf("zero-usage conversion wrong: held=%d unreflected=%d", acct.HeldMicro, acct.UnreflectedMicro)
	}
}
