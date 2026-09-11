package commercial

import (
	"context"
	"sync"
	"testing"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubGateway is the external-boundary fake for these worker tests. This
// file proves worker STATE transitions against the real SQLite stores
// (orders, outbox, fulfillment records); channel acceptance for the real
// provider is proven separately by the httptest evidence in
// internal/infrastructure/openmeter/commercial_test.go.
type stubGateway struct {
	mu        sync.Mutex
	saved     map[string]domain.BenefitReceipt // benefits the "remote" persisted
	applies   int                              // grants that actually stored a benefit
	applyErr  error                            // returned AFTER saving (dropped response)
	findable  bool                             // FindBenefit reports saved benefits
	revokeErr error                            // returned by RevokeBenefit (revocation failure)
	revokes   int                              // precise-credits revocation attempts (incl. failures)
}

func (g *stubGateway) ApplyBenefit(_ context.Context, req domain.BenefitRequest) (domain.BenefitReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.saved == nil {
		g.saved = map[string]domain.BenefitReceipt{}
	}
	if r, ok := g.saved[req.Key]; ok {
		return r, nil // provider-side idempotency on the fulfillment key
	}
	g.applies++
	r := domain.BenefitReceipt{ExternalID: "om-evt-" + req.Key, EffectiveAt: req.EffectiveAt}
	g.saved[req.Key] = r
	// The benefit is durably saved remotely; the response may still be lost.
	return r, g.applyErr
}

func (g *stubGateway) FindBenefit(_ context.Context, key string) (domain.BenefitReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.findable {
		if r, ok := g.saved[key]; ok {
			return r, nil
		}
	}
	return domain.BenefitReceipt{}, domain.ErrBenefitNotFound
}

func (g *stubGateway) appliedCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.applies
}

func (g *stubGateway) setApplyErr(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.applyErr = err
}

func (g *stubGateway) setFindable(v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.findable = v
}

// RevokeBenefit is the C05 precise-credits revocation boundary of the same
// stub: it counts revocation ATTEMPTS and can be made to fail to prove
// that refund-success/revoke-failure recovery retries the revocation only
// (a failed attempt followed by a successful retry counts >= 2).
func (g *stubGateway) RevokeBenefit(_ context.Context, key string, credits domain.Credits) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revokes++ // count attempts, including the failed one
	if g.revokeErr != nil {
		return g.revokeErr
	}
	return nil
}

func (g *stubGateway) revokeCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.revokes
}

func (g *stubGateway) setRevokeErr(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revokeErr = err
}

func setupFulfillment(t *testing.T, gw domain.CommercialGateway) (*FulfillmentService, *gorm.DB, *repocommercial.OrderStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1) // serialize SQLite writers; claims still race logically
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{}, &repocommercial.OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	svc, err := NewFulfillmentService(db, gw)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, store
}

// seedPaidOrder drives the real C01 path: pending order, registered attempt,
// confirmed payment — leaving a paid order plus exactly one fulfill outbox
// event, the state a crashed worker would leave behind.
func seedPaidOrder(t *testing.T, store *repocommercial.OrderStore, orderID string, tenant uint64, amountFen int64) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: orderID, TenantID: tenant, QuoteID: "q-" + orderID, AmountFen: amountFen, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att-" + orderID, TenantID: tenant, OrderID: orderID, Provider: "alipay", Merchant: "weknora",
		MerchantOrderID: "mo-" + orderID, AmountFen: amountFen, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: "mo-" + orderID, Provider: "alipay", Merchant: "weknora",
		Transaction: "txn-" + orderID, Amount: domain.CNYFen(amountFen), Currency: "CNY", State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
}

func orderState(t *testing.T, db *gorm.DB, orderID string) string {
	t.Helper()
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", orderID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.State
}

func fulfillmentRecords(t *testing.T, db *gorm.DB, orderID string) []FulfillmentRecord {
	t.Helper()
	var recs []FulfillmentRecord
	if err := db.Where("order_id = ?", orderID).Find(&recs).Error; err != nil {
		t.Fatal(err)
	}
	return recs
}

func fulfillEvents(t *testing.T, db *gorm.DB) []repocommercial.OutboxEvent {
	t.Helper()
	var events []repocommercial.OutboxEvent
	if err := db.Where("kind = ?", repocommercial.OutboxKindFulfill).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

// TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce: the gateway saves
// the benefit and then drops the response. The first pass must not record
// success; once FindBenefit can see the saved benefit (reconciliation), a
// rerun discovers it instead of re-granting, fulfills the order exactly once,
// and leaves no duplicate outbox events.
func TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce(t *testing.T) {
	gw := &stubGateway{applyErr: domain.ErrGatewayIndeterminate}
	svc, db, store := setupFulfillment(t, gw)
	seedPaidOrder(t, store, "ord_drop", 7, 2500)
	ctx := context.Background()

	// Pass 1: apply saved remotely, response lost — nothing is provable.
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord_drop"); got != domain.OrderStatePaid {
		t.Fatalf("unprovable apply must not record success, order state %q", got)
	}
	recs := fulfillmentRecords(t, db, "ord_drop")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention || recs[0].ExternalID != "" {
		t.Fatalf("dropped response must stay attention without receipt: %+v", recs)
	}
	pinnedAt := recs[0].EffectiveAt
	if pinnedAt.IsZero() {
		t.Fatal("effective_at must be pinned on the first attempt")
	}
	if events := fulfillEvents(t, db); len(events) != 1 || events[0].State != repocommercial.OutboxStatePending {
		t.Fatalf("event must await reconciliation: %+v", events)
	}

	// Reconciliation becomes possible: FindBenefit now reports saved benefits.
	gw.setFindable(true)

	// Pass 2 ("restart" + rerun): the saved benefit is discovered, not re-granted.
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord_drop"); got != domain.OrderStateFulfilled {
		t.Fatalf("reconciled order must be fulfilled, got %q", got)
	}
	recs = fulfillmentRecords(t, db, "ord_drop")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied || recs[0].ExternalID == "" {
		t.Fatalf("recovered record must carry an explicit receipt: %+v", recs)
	}
	if !recs[0].EffectiveAt.Equal(pinnedAt) {
		t.Fatalf("recovered grant must reuse the pinned effective_at: %v != %v", recs[0].EffectiveAt, pinnedAt)
	}
	if events := fulfillEvents(t, db); len(events) != 1 || events[0].State != repocommercial.OutboxStateSent {
		t.Fatalf("exactly one sent event expected: %+v", events)
	}
	if got := gw.appliedCount(); got != 1 {
		t.Fatalf("exactly one external benefit expected, got %d applies", got)
	}

	// Pass 3: a full rerun must change nothing.
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := gw.appliedCount(); got != 1 {
		t.Fatalf("rerun re-granted the benefit: %d applies", got)
	}
	if got := len(fulfillEvents(t, db)); got != 1 {
		t.Fatalf("rerun duplicated outbox events: %d", got)
	}
	if got := orderState(t, db, "ord_drop"); got != domain.OrderStateFulfilled {
		t.Fatalf("rerun degraded the order to %q", got)
	}
}

// TestFulfillmentWorkerConcurrentClaimsApplyExactlyOnce: several workers
// race over the same paid order; the durable claim tokens guarantee exactly
// one fulfillment and one outbox event.
func TestFulfillmentWorkerConcurrentClaimsApplyExactlyOnce(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw)
	seedPaidOrder(t, store, "ord_race", 8, 1200)
	ctx := context.Background()

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- svc.Recover(ctx)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := gw.appliedCount(); got != 1 {
		t.Fatalf("exactly one external benefit expected, got %d applies", got)
	}
	if got := orderState(t, db, "ord_race"); got != domain.OrderStateFulfilled {
		t.Fatalf("order must be fulfilled exactly once, got %q", got)
	}
	recs := fulfillmentRecords(t, db, "ord_race")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied || recs[0].ExternalID == "" {
		t.Fatalf("exactly one applied record with receipt expected: %+v", recs)
	}
	events := fulfillEvents(t, db)
	if len(events) != 1 || events[0].State != repocommercial.OutboxStateSent {
		t.Fatalf("exactly one sent event expected: %+v", events)
	}
}

// TestFulfillmentWorkerPaidNotFulfilledStaysRecoverable: a paid order whose
// grant was refused stays paid (never silently "successful") and a later
// pass once the gateway accepts completes it.
func TestFulfillmentWorkerPaidNotFulfilledStaysRecoverable(t *testing.T) {
	gw := &stubGateway{applyErr: domain.ErrGatewayBusinessRefusal}
	svc, db, store := setupFulfillment(t, gw)
	seedPaidOrder(t, store, "ord_ref", 9, 800)
	ctx := context.Background()

	// Pass 1: definitive business refusal is a failure, never a success.
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord_ref"); got != domain.OrderStatePaid {
		t.Fatalf("refused grant must leave the order paid, got %q", got)
	}
	recs := fulfillmentRecords(t, db, "ord_ref")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateRefused || recs[0].ExternalID != "" {
		t.Fatalf("refusal must be recorded as refused without receipt: %+v", recs)
	}

	// The gateway accepts on a later pass (e.g. after a config fix).
	gw.setApplyErr(nil)
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord_ref"); got != domain.OrderStateFulfilled {
		t.Fatalf("paid order must be recoverable, got %q", got)
	}
	recs = fulfillmentRecords(t, db, "ord_ref")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied || recs[0].ExternalID == "" {
		t.Fatalf("recovered record must carry a receipt: %+v", recs)
	}
	if events := fulfillEvents(t, db); len(events) != 1 || events[0].State != repocommercial.OutboxStateSent {
		t.Fatalf("exactly one sent event expected: %+v", events)
	}
	if got := gw.appliedCount(); got != 1 {
		t.Fatalf("recovery must grant exactly once, got %d applies", got)
	}
}
