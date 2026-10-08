package commercial

import (
	"context"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
)

// TestSettlementDispatchWorkerDrainsPendingWithoutManualDispatch (#99 /
// Lago 27): a usage settlement outbox event enqueued by Finalize must be
// drained to the gateway by the BACKGROUND dispatch loop alone — no manual
// Dispatch call, no operator replay. After the loop's first pass the
// record is accepted (receipt identity, protection kept) and the event is
// marked sent.
func TestSettlementDispatchWorkerDrainsPendingWithoutManualDispatch(t *testing.T) {
	svc, gw, db, budget := testSettlementService(t)
	seedSettlementBudget(t, db)

	resKey := "res_worker_drain"
	if _, err := budget.Reserve(context.Background(), domain.BudgetRequest{
		TenantID: settlementTenant, RunID: "r1", Key: resKey,
		Upper: domain.Credits(settleHold), Deadline: time.Now().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Finalize(context.Background(), settlementFinalFact("call_worker", 1, 80_000), resKey)
	if err != nil {
		t.Fatal(err)
	}

	svc.dispatchInterval = 5 * time.Millisecond
	svc.StartBackground(context.Background())
	defer svc.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		rec := settlementRecord(t, db, st.ID)
		if rec.State == domain.SettlementStateAccepted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker never drained the settlement: state=%s", rec.State)
		}
		time.Sleep(2 * time.Millisecond)
	}

	if gw.settles == 0 {
		t.Fatal("worker drained the record without a gateway settle")
	}
	var sent int64
	if err := db.Model(&repocommercial.OutboxEvent{}).
		Where("kind = ?", repocommercial.OutboxKindUsageSettlement).
		Where("state = ?", repocommercial.OutboxStateSent).Count(&sent).Error; err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatalf("sent settlement events=%d want 1", sent)
	}
}

// TestSettlementDispatchWorkerStopIsIdempotent: Stop on a never-started
// service is a no-op and a second Stop after Start is safe.
func TestSettlementDispatchWorkerStopIsIdempotent(t *testing.T) {
	svc, _, _, _ := testSettlementService(t)
	svc.Stop()
	svc.StartBackground(context.Background())
	svc.Stop()
	svc.Stop()
}
