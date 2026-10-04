package commercial

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubGateway is the external-boundary fake for these worker tests. This
// file proves worker STATE transitions against the real SQLite stores
// (orders, outbox, fulfillment records); channel acceptance for the real
// provider is proven separately by the httptest evidence in
// internal/infrastructure/openmeter/commercial_test.go.
type stubGateway struct {
	mu               sync.Mutex
	saved            map[string]domain.BenefitReceipt // benefits the "remote" persisted
	applies          int                              // grants that actually stored a benefit
	finds            int                              // reconciliation lookups
	requiredCustomer string                           // optional FindBenefit customer assertion
	applyErr         error                            // returned AFTER saving (dropped response)
	findable         bool                             // FindBenefit reports saved benefits
	revokeErr        error                            // returned by RevokeBenefit (revocation failure)
	revokes          int                              // precise-credits revocation attempts (incl. failures)
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

func (g *stubGateway) FindBenefit(ctx context.Context, key string) (domain.BenefitReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.finds++
	if g.requiredCustomer != "" && domain.BenefitCustomerFrom(ctx) != g.requiredCustomer {
		return domain.BenefitReceipt{}, fmt.Errorf("FindBenefit customer=%q, want %q", domain.BenefitCustomerFrom(ctx), g.requiredCustomer)
	}
	if g.findable {
		if r, ok := g.saved[key]; ok {
			return r, nil
		}
	}
	return domain.BenefitReceipt{}, domain.ErrBenefitNotFound
}

func (g *stubGateway) findCount() int { g.mu.Lock(); defer g.mu.Unlock(); return g.finds }

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

func setupFulfillment(t *testing.T, gw domain.CommercialGateway, platform domain.CommercialPlatform) (*FulfillmentService, *gorm.DB, *repocommercial.OrderStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1) // serialize SQLite writers; claims still race logically
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PaymentAnomalyRow{}, &repocommercial.QuoteRow{}); err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	svc, err := NewFulfillmentService(db, gw, platform, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, store
}

// seedPaidOrder drives the real C01 path: pending order, registered attempt,
// confirmed payment — leaving a paid order plus exactly one fulfill outbox
// event, the state a crashed worker would leave behind.
func seedPaidOrder(t *testing.T, store *repocommercial.OrderStore, orderID string, tenant uint64, amountFen int64, dbOpt ...*gorm.DB) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: orderID, TenantID: tenant, QuoteID: "q-" + orderID, AmountFen: amountFen, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	// These fixtures explicitly represent the supported legacy top-up quote
	// semantics while retaining a real registered payment winner.
	if len(dbOpt) > 0 && dbOpt[0] != nil {
		if err := dbOpt[0].WithContext(ctx).Model(&repocommercial.OrderRow{}).Where("id = ?", orderID).Update("quote_id", "").Error; err != nil {
			t.Fatal(err)
		}
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

func TestFulfillmentTopUpRejectsContradictoryWinningAttempt(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord-topup-mismatch", 7, 2500, db)
	seedPaidOrder(t, store, "ord-topup-following", 7, 400)
	if err := db.Create(&repocommercial.QuoteRow{ID: "q-ord-topup-following", TenantID: 7, SnapshotJSON: `{"line_items":[{"kind":"top_up"}]}`}).Error; err != nil {
		t.Fatal(err)
	}
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-mismatch").First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	payload["transaction"] = "txn-losing-attempt"
	if err := db.Model(&ev).Update("payload_json", mustJSON(t, payload)).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rejected repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-mismatch").First(&rejected).Error; err != nil {
		t.Fatal(err)
	}
	if rejected.State != repocommercial.OutboxStatePending || orderState(t, db, "ord-topup-mismatch") != domain.OrderStatePaid || len(fulfillmentRecords(t, db, "ord-topup-mismatch")) != 0 {
		t.Fatalf("mismatch was not retained pending: event=%+v records=%+v", rejected, fulfillmentRecords(t, db, "ord-topup-mismatch"))
	}
	var exceptions []FulfillmentExceptionRow
	if err := db.Where("order_id = ?", "ord-topup-mismatch").Find(&exceptions).Error; err != nil {
		t.Fatal(err)
	}
	if len(exceptions) != 1 || exceptions[0].Kind != "top_up" || exceptions[0].Reason != "invalid_winning_payment" || exceptions[0].State != "open" {
		t.Fatalf("exception rows=%+v", exceptions)
	}
	if gw.findCount() != 1 || gw.appliedCount() != 1 {
		t.Fatalf("only following valid event should call gateway once: finds=%d applies=%d", gw.findCount(), gw.appliedCount())
	}
	if got := orderState(t, db, "ord-topup-following"); got != domain.OrderStateFulfilled {
		t.Fatalf("following order state=%s", got)
	}
	// A repaired, verified event is the only path that may resolve the exception.
	contradictFulfillTransaction(t, db, "fulfill:ord-topup-mismatch", "txn-ord-topup-mismatch")
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("event_key = ?", "fulfill:ord-topup-mismatch").First(&rejected).Error; err != nil {
		t.Fatal(err)
	}
	if rejected.State != repocommercial.OutboxStateSent || orderState(t, db, "ord-topup-mismatch") != domain.OrderStateFulfilled {
		t.Fatalf("corrected winner did not fulfill: event=%+v", rejected)
	}
	if err := db.Where("order_id = ?", "ord-topup-mismatch").First(&exceptions[0]).Error; err != nil {
		t.Fatal(err)
	}
	if exceptions[0].State != "resolved" || exceptions[0].ResolvedAt == nil || gw.appliedCount() != 2 {
		t.Fatalf("resolution=%+v gateway applies=%d", exceptions[0], gw.appliedCount())
	}
}

func TestFulfillmentTopUpMissingWinningAttemptIsRetained(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord-topup-no-attempt", 7, 2500, db)
	if err := db.Where("id = ?", "att-ord-topup-no-attempt").Delete(&repocommercial.PaymentAttemptRow{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-no-attempt").First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.State != repocommercial.OutboxStatePending {
		t.Fatalf("event state=%s", ev.State)
	}
	var exceptions []FulfillmentExceptionRow
	if err := db.Where("order_id = ?", "ord-topup-no-attempt").Find(&exceptions).Error; err != nil {
		t.Fatal(err)
	}
	if len(exceptions) != 1 || exceptions[0].Reason != "invalid_winning_payment" {
		t.Fatalf("exceptions=%+v", exceptions)
	}
	if gw.findCount() != 0 || gw.appliedCount() != 0 {
		t.Fatalf("gateway called: finds=%d applies=%d", gw.findCount(), gw.appliedCount())
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	var replay []FulfillmentExceptionRow
	if err := db.Where("order_id = ?", "ord-topup-no-attempt").Find(&replay).Error; err != nil {
		t.Fatal(err)
	}
	if len(replay) != 1 || !replay[0].CreatedAt.Equal(exceptions[0].CreatedAt) {
		t.Fatalf("replay was not idempotent: %+v", replay)
	}
}

func TestFulfillmentTopUpFulfilledOpenExceptionRequiresWinnerAndReceipt(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord-topup-fulfilled-exception", 7, 2500, db)
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-fulfilled-exception").First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	contradictFulfillTransaction(t, db, ev.EventKey, "txn-invalid-winner")
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&repocommercial.OrderRow{}).Where("id = ?", "ord-topup-fulfilled-exception").Update("state", domain.OrderStateFulfilled).Error; err != nil {
		t.Fatal(err)
	}

	assertPendingOpen := func(stage string) {
		t.Helper()
		if err := db.Where("event_key = ?", ev.EventKey).First(&ev).Error; err != nil {
			t.Fatal(err)
		}
		var exception FulfillmentExceptionRow
		if err := db.Where("event_key = ?", ev.EventKey).First(&exception).Error; err != nil {
			t.Fatal(err)
		}
		if ev.State != repocommercial.OutboxStatePending || exception.State != "open" {
			t.Fatalf("%s acknowledged unresolved event: event=%+v exception=%+v", stage, ev, exception)
		}
	}

	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPendingOpen("invalid winner")
	if gw.appliedCount() != 0 {
		t.Fatalf("invalid winner applied benefits=%d", gw.appliedCount())
	}

	contradictFulfillTransaction(t, db, ev.EventKey, "txn-ord-topup-fulfilled-exception")
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPendingOpen("missing receipt")
	if gw.appliedCount() != 0 {
		t.Fatalf("recovery applied benefits=%d", gw.appliedCount())
	}

	key := domain.FulfillmentKey("ord-topup-fulfilled-exception", "credits")
	gw.mu.Lock()
	if gw.saved == nil {
		gw.saved = map[string]domain.BenefitReceipt{}
	}
	gw.saved[key] = domain.BenefitReceipt{}
	gw.mu.Unlock()
	gw.setFindable(true)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPendingOpen("empty receipt")

	gw.mu.Lock()
	gw.saved[key] = domain.BenefitReceipt{ExternalID: "existing-topup-receipt"}
	gw.requiredCustomer = OrderCustomerID(7)
	gw.mu.Unlock()
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("event_key = ?", ev.EventKey).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	var exception FulfillmentExceptionRow
	if err := db.Where("event_key = ?", ev.EventKey).First(&exception).Error; err != nil {
		t.Fatal(err)
	}
	if ev.State != repocommercial.OutboxStateSent || exception.State != "resolved" || exception.ResolvedAt == nil {
		t.Fatalf("verified recovery did not complete atomically: event=%+v exception=%+v", ev, exception)
	}
	if gw.appliedCount() != 0 {
		t.Fatalf("recovery duplicated ApplyBenefit calls=%d", gw.appliedCount())
	}
}

func TestFulfillmentTopUpExceptionStorageFailureDoesNotAcknowledge(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord-topup-exception-write-failure", 7, 2500, db)
	if err := db.Where("id = ?", "att-ord-topup-exception-write-failure").Delete(&repocommercial.PaymentAttemptRow{}).Error; err != nil {
		t.Fatal(err)
	}
	callback := "test/fail_fulfillment_exception"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == (FulfillmentExceptionRow{}).TableName() {
			tx.AddError(fmt.Errorf("injected exception storage failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err == nil {
		t.Fatal("expected exception persistence error")
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-exception-write-failure").First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.State != repocommercial.OutboxStatePending {
		t.Fatalf("event acknowledged: %+v", ev)
	}
	var n int64
	if err := db.Model(&FulfillmentExceptionRow{}).Where("order_id = ?", "ord-topup-exception-write-failure").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 || gw.findCount() != 0 || gw.appliedCount() != 0 {
		t.Fatalf("failure side effects: rows=%d finds=%d applies=%d", n, gw.findCount(), gw.appliedCount())
	}
}

func TestFulfillmentTopUpTransientWinningAttemptReadRetries(t *testing.T) {
	gw := &stubGateway{}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord-topup-retry", 7, 2500, db)
	beforeAttempt := attemptForTest(t, db, "att-ord-topup-retry")
	var beforeEvent repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-retry").First(&beforeEvent).Error; err != nil {
		t.Fatal(err)
	}
	var fail atomic.Bool
	fail.Store(true)
	var reads atomic.Int32
	callback := "test/transient_topup_winner_read"
	if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "commercial_payment_attempts" && fail.CompareAndSwap(true, false) {
			reads.Add(1)
			tx.AddError(fmt.Errorf("temporary attempt query failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := fulfillEvents(t, db)
	if len(events) != 1 || events[0].State != repocommercial.OutboxStatePending {
		t.Fatalf("event not pending: %+v", events)
	}
	if len(fulfillmentRecords(t, db, "ord-topup-retry")) != 0 || gw.findCount() != 0 || gw.appliedCount() != 0 {
		t.Fatalf("side effects after transient read: records=%+v finds=%d applies=%d", fulfillmentRecords(t, db, "ord-topup-retry"), gw.findCount(), gw.appliedCount())
	}
	if reads.Load() != 1 {
		t.Fatalf("attempt query faults=%d, want exactly one", reads.Load())
	}
	assertTopUpWinnerUnchanged(t, db, beforeAttempt, beforeEvent.PayloadJSON)
	if err := db.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord-topup-retry"); got != domain.OrderStateFulfilled {
		t.Fatalf("retry order state=%s", got)
	}
	assertTopUpWinnerUnchanged(t, db, beforeAttempt, beforeEvent.PayloadJSON)
	if gw.appliedCount() != 1 {
		t.Fatalf("replayed event benefits=%d, want one", gw.appliedCount())
	}
}

func attemptForTest(t *testing.T, db *gorm.DB, id string) repocommercial.PaymentAttemptRow {
	t.Helper()
	var row repocommercial.PaymentAttemptRow
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func assertTopUpWinnerUnchanged(t *testing.T, db *gorm.DB, want repocommercial.PaymentAttemptRow, payload string) {
	t.Helper()
	if got := attemptForTest(t, db, want.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("attempt changed: got=%+v want=%+v", got, want)
	}
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:ord-topup-retry").First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.PayloadJSON != payload {
		t.Fatalf("outbox payload changed: got=%s want=%s", ev.PayloadJSON, payload)
	}
}

func contradictFulfillTransaction(t *testing.T, db *gorm.DB, eventKey, txn string) {
	t.Helper()
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", eventKey).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	payload["transaction"] = txn
	if err := db.Model(&ev).Update("payload_json", mustJSON(t, payload)).Error; err != nil {
		t.Fatal(err)
	}
}

func TestFulfillmentSubscriptionRejectsContradictoryWinnerBeforeNilPurchaser(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const orderID = "ord-sub-nil-purchaser-mismatch"
	seedPaidPurchase(t, store, db, 101, orderID, "plan-nil-purchaser", "pub-nil-purchaser", 9900)
	contradictFulfillTransaction(t, db, "fulfill:"+orderID, "txn-loser")
	svc.purchaser = nil
	gateway := svc.gateway.(*stubGateway)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionAttentionPending(t, db, orderID)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionAttentionPending(t, db, orderID)
	if gateway.findCount() != 0 || gateway.appliedCount() != 0 || len(fake.Wallets()) != 0 {
		t.Fatalf("outbound effects: find=%d apply=%d wallets=%d", gateway.findCount(), gateway.appliedCount(), len(fake.Wallets()))
	}
}

func TestFulfillmentSubscriptionRejectsContradictoryWinnerWhenQuoteReadFails(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const orderID = "ord-sub-quote-read-mismatch"
	seedPaidPurchase(t, store, db, 102, orderID, "plan-quote-read", "pub-quote-read", 9900)
	contradictFulfillTransaction(t, db, "fulfill:"+orderID, "txn-loser")
	svc.purchaser = nil
	var quoteReads atomic.Int32
	callback := "test/quote_read_must_not_precede_winner"
	if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "commercial_quotes" {
			quoteReads.Add(1)
			tx.AddError(fmt.Errorf("injected quote-store outage"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	gateway := svc.gateway.(*stubGateway)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Query().Remove(callback); err != nil {
		t.Fatal(err)
	}
	assertSubscriptionAttentionPending(t, db, orderID)
	if quoteReads.Load() != 0 {
		t.Fatalf("quote reads=%d; winner should be validated first", quoteReads.Load())
	}
	if gateway.findCount() != 0 || gateway.appliedCount() != 0 || len(fake.Wallets()) != 0 {
		t.Fatalf("outbound effects: find=%d apply=%d wallets=%d", gateway.findCount(), gateway.appliedCount(), len(fake.Wallets()))
	}
}

func TestFulfillmentSubscriptionAttentionWriteFailureDoesNotAcknowledge(t *testing.T) {
	_, svc, fake, db, store := setupPurchaseFulfillment(t)
	const orderID = "ord-sub-attention-write-failure"
	seedPaidPurchase(t, store, db, 103, orderID, "plan-attention-write", "pub-attention-write", 9900)
	contradictFulfillTransaction(t, db, "fulfill:"+orderID, "txn-loser")
	svc.purchaser = nil
	fixedNow := time.Now()
	svc.now = func() time.Time { return fixedNow }
	callback := "test/attention_record_insert_error"
	var fail atomic.Bool
	fail.Store(true)
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == (FulfillmentRecord{}).TableName() && fail.CompareAndSwap(true, false) {
			tx.AddError(fmt.Errorf("injected attention insert failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(context.Background()); err == nil {
		t.Fatal("expected attention insert failure to propagate")
	}
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:"+orderID).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.State != repocommercial.OutboxStatePending || ev.LeaseToken == "" || !ev.LeaseUntil.After(fixedNow) {
		t.Fatalf("failed write was acknowledged/released: %+v", ev)
	}
	if recs := fulfillmentRecords(t, db, orderID); len(recs) != 0 {
		t.Fatalf("unexpected attention record after failed insert: %+v", recs)
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	fixedNow = ev.LeaseUntil.Add(time.Second)
	if err := svc.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	var replay repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:"+orderID).First(&replay).Error; err != nil {
		t.Fatal(err)
	}
	if replay.State != repocommercial.OutboxStatePending || replay.LeaseUntil.After(fixedNow) {
		t.Fatalf("replayed event not released pending: %+v", replay)
	}
	if recs := fulfillmentRecords(t, db, orderID); len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("attention records=%+v", recs)
	}
	if len(fake.Wallets()) != 0 || svc.gateway.(*stubGateway).appliedCount() != 0 {
		t.Fatalf("outbound effects: wallets=%d gateway=%d", len(fake.Wallets()), svc.gateway.(*stubGateway).appliedCount())
	}
}

func assertSubscriptionAttentionPending(t *testing.T, db *gorm.DB, orderID string) {
	t.Helper()
	var ev repocommercial.OutboxEvent
	if err := db.Where("event_key = ?", "fulfill:"+orderID).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.State != repocommercial.OutboxStatePending {
		t.Fatalf("event state=%s", ev.State)
	}
	if got := orderState(t, db, orderID); got != domain.OrderStatePaid {
		t.Fatalf("order state=%s", got)
	}
	recs := fulfillmentRecords(t, db, orderID)
	wantKey := domain.FulfillmentKey(orderID, benefitKindPurchaseActivation)
	if len(recs) != 1 || recs[0].Key != wantKey || recs[0].Kind != benefitKindPurchaseActivation || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("attention record=%+v", recs)
	}
}

func assertTopUpQuarantinedWithoutGateway(t *testing.T, db *gorm.DB, gw *stubGateway, orderID string) {
	t.Helper()
	events := fulfillEvents(t, db)
	var found bool
	for _, ev := range events {
		if ev.EventKey == "fulfill:"+orderID {
			found = true
			if ev.State != repocommercial.OutboxStateDead {
				t.Fatalf("event state=%s", ev.State)
			}
		}
	}
	if !found {
		t.Fatalf("fulfill event missing for %s", orderID)
	}
	if got := orderState(t, db, orderID); got != domain.OrderStatePaid {
		t.Fatalf("order state=%s", got)
	}
	if recs := fulfillmentRecords(t, db, orderID); len(recs) != 0 {
		t.Fatalf("fulfillment records=%+v", recs)
	}
	if gw.findCount() != 0 || gw.appliedCount() != 0 {
		t.Fatalf("gateway calls: finds=%d applies=%d", gw.findCount(), gw.appliedCount())
	}
}

func fulfillEvents(t *testing.T, db *gorm.DB) []repocommercial.OutboxEvent {
	t.Helper()
	var events []repocommercial.OutboxEvent
	if err := db.Where("kind = ?", repocommercial.OutboxKindFulfill).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

// TestFulfillEventKeepsPendingWhenDiscriminationUnprovable（OCR84-R1-05 high）：
// 三态报价判定必须区分可重试的读取故障与确定性损坏的报价。两者都不能
// 落穿进充值结算：误路由一次即不可恢复（按簿记汇率折算额度 + MarkFulfilled
// + 事件 Sent，此后无任何路径回到 settle/observe/grant 链）。
func TestFulfillEventFailsClosedWhenDiscriminationUnprovable(t *testing.T) {
	for _, tc := range []struct {
		name           string
		wantEventState string
		inject         func(t *testing.T, db *gorm.DB)
	}{
		{"corrupt quote snapshot", repocommercial.OutboxStatePending, func(t *testing.T, db *gorm.DB) {
			if err := db.Create(&repocommercial.QuoteRow{
				ID: "q_ord_disc", TenantID: 71, SubscriptionVersion: 1,
				SnapshotJSON: `{"line_items":[`, // 解析必然失败
			}).Error; err != nil {
				t.Fatal(err)
			}
		}},
		{"quote read failure", repocommercial.OutboxStatePending, func(t *testing.T, db *gorm.DB) {
			// 非 NotFound 的读取错误：表被移走，First 直接报错。
			if err := db.Migrator().DropTable(&repocommercial.QuoteRow{}); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gw := &stubGateway{findable: true}
			svc, db, store := setupFulfillment(t, gw, nil)
			if err := db.AutoMigrate(&repocommercial.QuoteRow{}); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if err := store.CreateOrder(ctx, repocommercial.OrderRow{
				ID: "ord_disc", TenantID: 71, QuoteID: "q_ord_disc", Kind: domain.OrderKindPurchase,
				AmountFen: 9900, Currency: domain.CurrencyCNY,
			}); err != nil {
				t.Fatal(err)
			}
			if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
				ID: "att_ord_disc", TenantID: 71, OrderID: "ord_disc", Provider: "alipay", Merchant: "weknora",
				MerchantOrderID: "mo_ord_disc", AmountFen: 9900, Currency: domain.CurrencyCNY,
			}); err != nil {
				t.Fatal(err)
			}
			if err := store.ConfirmPayment(ctx, domain.PaymentFact{
				TenantID: 71, OrderID: "ord_disc", AttemptID: "mo_ord_disc", Provider: "alipay", Merchant: "weknora",
				Transaction: "txn_ord_disc", Amount: domain.CNYFen(9900), Currency: domain.CurrencyCNY, State: "succeeded",
			}); err != nil {
				t.Fatal(err)
			}
			tc.inject(t, db)
			if err := svc.Recover(ctx); err != nil {
				t.Fatalf("drain: %v", err)
			}
			// 订单绝不能被充值结算误路由成 fulfilled。
			if state := orderState(t, db, "ord_disc"); state != domain.OrderStatePaid {
				t.Fatalf("unprovable discrimination must NOT fulfill (top-up misroute), got state=%q", state)
			}
			// Deterministic malformed quotes remain pending with operator Attention;
			// transient quote-store failures remain pending without an Attention record.
			events := fulfillEvents(t, db)
			if len(events) != 1 || events[0].State != tc.wantEventState {
				t.Fatalf("fulfill event state = %+v, want %s", events, tc.wantEventState)
			}
			records := fulfillmentRecords(t, db, "ord_disc")
			if tc.name == "corrupt quote snapshot" {
				if len(records) != 1 || records[0].Kind != benefitKindPurchaseActivation || records[0].State != domain.FulfillmentStateAttention {
					t.Fatalf("deterministic malformed quote must create purchase_activation attention, got %+v", records)
				}
			} else if len(records) != 0 {
				t.Fatalf("transient quote read must not create attention, got %+v", records)
			}
			// 充值腿的 gateway 授予一次都不能发生。
			if n := gw.appliedCount(); n != 0 {
				t.Fatalf("no top-up grant may fire while discrimination is unprovable, got %d applies", n)
			}
		})
	}
}

// TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce: the gateway saves
// the benefit and then drops the response. The first pass must not record
// success; once FindBenefit can see the saved benefit (reconciliation), a
// rerun discovers it instead of re-granting, fulfills the order exactly once,
// and leaves no duplicate outbox events.
// TestFulfillmentUpgradeSwitchesPlanAndGrantsDelta proves the upgrade
// settlement (design 6.2): a PAID upgrade order switches the subscription
// to the target plan — keeping anchor and paid_until, never re-issuing used
// monthly grants — and grants exactly ONE prorated monthly-credit delta for
// the remaining part of the current month. A replay pass neither switches
// nor grants again.
func TestFulfillmentUpgradeSwitchesPlanAndGrantsDelta(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	if err := db.AutoMigrate(&repocommercial.Subscription{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ctx := context.Background()

	// Target plan definition (published) + quote snapshot for it.
	target := domain.PlanVersion{Key: "premium", Version: 1, Price: 199_00, Monthly: 19_900_000}
	targetJSON, _ := json.Marshal(target)
	if err := db.Create(&repocommercial.PlanRow{PlanKey: "premium", Version: 1,
		DefinitionJSON: string(targetJSON), State: domain.PlanStatePublished}).Error; err != nil {
		t.Fatal(err)
	}
	snap := `{"plan_key":"premium","plan_version":1,"price_fen":19900,"credits_micro":19900000}`
	if err := db.Create(&repocommercial.QuoteRow{ID: "qt_up", TenantID: 101,
		SubscriptionVersion: 1, SnapshotJSON: snap, ExpiresAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	// Current subscription on the cheaper plan; anchor == now so the
	// current month is fully remaining and the delta equals the full
	// monthly difference.
	current := domain.PlanVersion{Key: "pro", Version: 3, Price: 99_00, Monthly: 9_900_000}
	currentJSON, _ := json.Marshal(current)
	paidUntil := now.Add(30 * 24 * time.Hour)
	sub := repocommercial.Subscription{
		ID: "sub-101", TenantID: 101, PlanKey: "pro", PlanVersion: 3,
		PlanSnapshotJSON: string(currentJSON), Anchor: now, PaidUntil: paidUntil,
		FutureIntervalJSON: "{}", Version: 1,
	}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}

	// Paid upgrade order through the real C01 path.
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: "ord_up", TenantID: 101, QuoteID: "qt_up", Kind: domain.OrderKindUpgrade,
		AmountFen: 50_00, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att_up", TenantID: 101, OrderID: "ord_up", Provider: "alipay", Merchant: "weknora",
		MerchantOrderID: "mo_up", AmountFen: 50_00, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: 101, OrderID: "ord_up", AttemptID: "mo_up", Provider: "alipay", Merchant: "weknora",
		Transaction: "txn_up", Amount: 50_00, Currency: "CNY", State: "succeeded"}); err != nil {
		t.Fatal(err)
	}

	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}

	var after repocommercial.Subscription
	if err := db.Where("tenant_id = ?", 101).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.PlanKey != "premium" || after.PlanVersion != 1 {
		t.Fatalf("plan not switched: %+v", after)
	}
	if !after.PaidUntil.Equal(paidUntil) || !after.Anchor.Equal(now) {
		t.Fatalf("paid interval was reset: anchor=%v paid_until=%v", after.Anchor, after.PaidUntil)
	}
	if after.Version != 2 || after.PlanSnapshotJSON != string(targetJSON) {
		t.Fatalf("snapshot/version not switched: v=%d snap=%s", after.Version, after.PlanSnapshotJSON)
	}
	if got := orderState(t, db, "ord_up"); got != domain.OrderStateFulfilled {
		t.Fatalf("upgrade order state = %s, want fulfilled", got)
	}
	// Exactly one delta grant, for the FULL monthly difference (the whole
	// current month remained).
	if gw.appliedCount() != 1 {
		t.Fatalf("delta grants = %d, want 1; records=%+v", gw.appliedCount(), fulfillmentRecords(t, db, "ord_up"))
	}
	recs := fulfillmentRecords(t, db, "ord_up")
	var deltaSeen bool
	for _, r := range recs {
		if r.LineID == "upgrade_delta" {
			deltaSeen = true
			// Full monthly difference minus sub-millisecond proration loss.
			if r.Credits < 9_999_000 || r.Credits > 10_000_000 {
				t.Fatalf("delta credits = %d, want ~10000000", r.Credits)
			}
			if r.State != domain.FulfillmentStateApplied {
				t.Fatalf("delta state = %s", r.State)
			}
		}
	}
	if !deltaSeen {
		t.Fatalf("no upgrade_delta record: %+v", recs)
	}

	// Replay: neither the switch nor the grant happens twice.
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if gw.appliedCount() != 1 {
		t.Fatalf("replay granted again: %d", gw.appliedCount())
	}
	var after2 repocommercial.Subscription
	if err := db.Where("tenant_id = ?", 101).First(&after2).Error; err != nil {
		t.Fatal(err)
	}
	if after2.Version != 2 {
		t.Fatalf("replay bumped version again: %d", after2.Version)
	}
}
func TestFulfillmentWorkerSavedThenDroppedRecoversExactlyOnce(t *testing.T) {
	gw := &stubGateway{applyErr: domain.ErrGatewayIndeterminate}
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord_drop", 7, 2500, db)
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
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord_race", 8, 1200, db)
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
	svc, db, store := setupFulfillment(t, gw, nil)
	seedPaidOrder(t, store, "ord_ref", 9, 800, db)
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

// ---- #84 Task 3: the over_payment outbox consumer (G3 / AC2) ----

// seedSecondChannelSuccess registers a SECOND-channel attempt on an order
// whose first channel already confirmed (seedPaidOrder), then confirms the
// second fact — the multiple-success shape ConfirmPayment audits as a
// kind=over_payment outbox event (#84 AC2).
func seedSecondChannelSuccess(t *testing.T, store *repocommercial.OrderStore, orderID string, tenant uint64, amountFen int64) {
	t.Helper()
	ctx := context.Background()
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att2-" + orderID, TenantID: tenant, OrderID: orderID, Provider: "wechat", Merchant: "1900000109",
		MerchantOrderID: "mo2-" + orderID, AmountFen: amountFen, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: "mo2-" + orderID, Provider: "wechat", Merchant: "1900000109",
		Transaction: "txn2-" + orderID, Amount: domain.CNYFen(amountFen), Currency: "CNY", State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
}

func overPaymentEvent(t *testing.T, db *gorm.DB) repocommercial.OutboxEvent {
	t.Helper()
	var ev repocommercial.OutboxEvent
	if err := db.Where("kind = ?", repocommercial.OutboxKindOverPaid).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	return ev
}

// TestOverPaymentDrainConsumesEventIntoAwaitingDisposal（AC2 / G3）：多收款的
// 第二笔成功由 ConfirmPayment 在同一事务写入异常事实和 over_payment 事件；
// drain 验证该事实并把事件标记 sent，不重置异常处置状态；权益不扩大——
// fulfillment_records 恰一行 applied、订单只履约一次。
func TestOverPaymentDrainConsumesEventIntoAwaitingDisposal(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	ctx := context.Background()
	const orderID = "ord-over-1"
	seedPaidOrder(t, store, orderID, 7, 9900, db)
	seedSecondChannelSuccess(t, store, orderID, 7, 9900)

	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	ok, err := store.HasUnresolvedPaymentAnomaly(ctx, orderID)
	if err != nil || !ok {
		t.Fatalf("the over_payment must land as an unresolved anomaly, got %v %v", ok, err)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", orderID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != repocommercial.PaymentAnomalyKindOverPaid ||
		anomaly.ExpectedAmountFen != 0 || anomaly.ActualAmountFen != 9900 ||
		anomaly.Transaction != "txn2-"+orderID {
		t.Fatalf("over_payment snapshot mismatch: %+v", anomaly)
	}
	ev := overPaymentEvent(t, db)
	if ev.State != repocommercial.OutboxStateSent {
		t.Fatalf("the consumed over_payment event must be sent, got %s", ev.State)
	}
	// 权益不扩大：恰一行 applied 的履约记录（top-up 面），订单只履约一次。
	recs := fulfillmentRecords(t, db, orderID)
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied {
		t.Fatalf("exactly one applied fulfillment record expected, got %+v", recs)
	}
	if s := orderState(t, db, orderID); s != domain.OrderStateFulfilled {
		t.Fatalf("the first payment must still fulfill the order exactly once, got %s", s)
	}
}

func TestOverPaymentSameAttemptDifferentTransactionUsesDurableAnomalyBinding(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	ctx := context.Background()
	const orderID = "ord-over-same-attempt"
	const tenant = uint64(79)
	seedPaidOrder(t, store, orderID, tenant, 9900, db)
	winner := attemptForTest(t, db, "att-"+orderID)
	if winner.ProviderTransactionID == nil {
		t.Fatal("setup requires the first successful transaction on the attempt")
	}
	firstTransaction := *winner.ProviderTransactionID
	laterTransaction := "txn-later-" + orderID
	later := domain.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: winner.MerchantOrderID, Provider: winner.Provider, Merchant: winner.Merchant,
		Transaction: laterTransaction, Amount: domain.CNYFen(winner.AmountFen), Currency: winner.Currency, State: "succeeded",
	}
	if err := store.ConfirmPayment(ctx, later); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, later); err != nil {
		t.Fatal(err)
	}
	var anomalies []repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", orderID).Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 || anomalies[0].Transaction != laterTransaction || anomalies[0].AttemptID != winner.MerchantOrderID || anomalies[0].State != repocommercial.PaymentAnomalyStateAwaiting {
		t.Fatalf("same-attempt later collection must be atomically bound once, got %+v", anomalies)
	}
	resolved, err := store.ResolvePaymentAnomaly(ctx, anomalies[0].ID, anomalies[0].Version)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != repocommercial.PaymentAnomalyStateResolved {
		t.Fatalf("setup must resolve anomaly before consumer drain, got %+v", resolved)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var event repocommercial.OutboxEvent
	if err := db.Where("kind = ?", repocommercial.OutboxKindOverPaid).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.State != repocommercial.OutboxStateSent {
		t.Fatalf("validated same-attempt event should be sent, got %s", event.State)
	}
	winnerAfter := attemptForTest(t, db, winner.ID)
	if winnerAfter.ProviderTransactionID == nil || *winnerAfter.ProviderTransactionID != firstTransaction {
		t.Fatalf("later collection replaced the immutable attempt winner: %+v", winnerAfter)
	}
	if err := db.Where("order_id = ?", orderID).Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 1 || anomalies[0].Transaction != laterTransaction || anomalies[0].State != repocommercial.PaymentAnomalyStateResolved {
		t.Fatalf("consumer replay changed or duplicated the durable anomaly: %+v", anomalies)
	}
}

func TestOverPaymentContradictoryIdentityIsDeadWithoutAnomaly(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mutate        func(*repocommercial.OutboxEvent, map[string]any)
		attemptUpdate map[string]any
	}{
		{name: "inline merchant order id", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) {
			p["merchant_order_id"] = "mo-some-other-attempt"
		}},
		{name: "inline transaction", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["transaction"] = "txn-contradiction" }},
		{name: "attempt binding", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["attempt_id"] = "att-unbound" }},
		{name: "payload order", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["order_id"] = "ord-other" }},
		{name: "amount", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["amount_fen"] = float64(8800) }},
		{name: "currency", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["currency"] = "USD" }},
		{name: "attempt state", attemptUpdate: map[string]any{"state": repocommercial.PaymentAttemptStateClosed}},
		{name: "attempt provider", attemptUpdate: map[string]any{"provider": "alipay"}},
		{name: "attempt merchant", attemptUpdate: map[string]any{"merchant": "other-merchant"}},
		{name: "attempt merchant order id", attemptUpdate: map[string]any{"merchant_order_id": "mo-other"}},
		{name: "attempt transaction", attemptUpdate: map[string]any{"provider_transaction_id": "txn-other"}},
		{name: "attempt amount", attemptUpdate: map[string]any{"amount_fen": 8800}},
		{name: "attempt currency", attemptUpdate: map[string]any{"currency": "USD"}},
		{name: "event tenant", mutate: func(ev *repocommercial.OutboxEvent, _ map[string]any) { ev.TenantID++ }},
		{name: "payload tenant", mutate: func(_ *repocommercial.OutboxEvent, p map[string]any) { p["tenant_id"] = float64(999) }},
		{name: "canonical key", mutate: func(ev *repocommercial.OutboxEvent, _ map[string]any) { ev.EventKey += ":extra" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gw := &stubGateway{findable: true}
			svc, db, store := setupFulfillment(t, gw, nil)
			const orderID = "ord-over-contradiction"
			seedPaidOrder(t, store, orderID, 77, 9900, db)
			seedSecondChannelSuccess(t, store, orderID, 77, 9900)
			var originalAnomalies []repocommercial.PaymentAnomalyRow
			if err := db.Where("order_id = ?", orderID).Find(&originalAnomalies).Error; err != nil {
				t.Fatal(err)
			}
			if len(originalAnomalies) != 1 {
				t.Fatalf("confirmation should persist one immutable fact before drain, got %+v", originalAnomalies)
			}
			ev := overPaymentEvent(t, db)
			originalKey := ev.EventKey
			var payload map[string]any
			if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(&ev, payload)
			}
			if tc.name == "inline transaction" || tc.name == "payload order" {
				ev.EventKey = fmt.Sprintf("%s:%s:%s:%s:%s", repocommercial.OutboxKindOverPaid, payload["order_id"], payload["provider"], payload["merchant"], payload["transaction"])
			}
			if len(tc.attemptUpdate) != 0 {
				if err := db.Model(&repocommercial.PaymentAttemptRow{}).Where("id = ?", "att2-"+orderID).Updates(tc.attemptUpdate).Error; err != nil {
					t.Fatal(err)
				}
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&repocommercial.OutboxEvent{}).Where("event_key = ?", originalKey).Updates(map[string]any{
				"event_key": ev.EventKey, "tenant_id": ev.TenantID, "payload_json": string(encoded),
			}).Error; err != nil {
				t.Fatal(err)
			}
			if err := svc.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			var anomalies []repocommercial.PaymentAnomalyRow
			if err := db.Where("order_id = ?", orderID).Find(&anomalies).Error; err != nil {
				t.Fatal(err)
			}
			if len(anomalies) != 1 || !reflect.DeepEqual(anomalies[0], originalAnomalies[0]) {
				t.Fatalf("contradictory event must not create or mutate an anomaly, got %+v (before %+v)", anomalies, originalAnomalies)
			}
			var stored repocommercial.OutboxEvent
			if err := db.Where("event_key = ?", ev.EventKey).First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			if stored.State != repocommercial.OutboxStateDead {
				t.Fatalf("contradictory event must be quarantined dead, got %s", stored.State)
			}
		})
	}
}

func TestOverPaymentTransientPersistenceFailuresRemainPending(t *testing.T) {
	for _, tc := range []struct {
		name    string
		install func(*testing.T, *gorm.DB)
	}{
		{name: "order read", install: func(t *testing.T, db *gorm.DB) {
			callback := "test:overpay_order_read_failure"
			if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "commercial_orders" {
					tx.AddError(fmt.Errorf("temporary order read failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Callback().Query().Remove(callback) })
		}},
		{name: "attempt read", install: func(t *testing.T, db *gorm.DB) {
			callback := "test:overpay_attempt_read_failure"
			if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "commercial_payment_attempts" {
					tx.AddError(fmt.Errorf("temporary attempt read failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Callback().Query().Remove(callback) })
		}},
		{name: "anomaly read", install: func(t *testing.T, db *gorm.DB) {
			callback := "test:overpay_anomaly_read_failure"
			var fail atomic.Bool
			fail.Store(true)
			if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "commercial_payment_anomalies" && fail.CompareAndSwap(true, false) {
					tx.AddError(fmt.Errorf("temporary anomaly read failure"))
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Callback().Query().Remove(callback) })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gw := &stubGateway{findable: true}
			svc, db, store := setupFulfillment(t, gw, nil)
			const orderID = "ord-over-transient"
			seedPaidOrder(t, store, orderID, 78, 9900, db)
			seedSecondChannelSuccess(t, store, orderID, 78, 9900)
			ev := overPaymentEvent(t, db)
			var before []repocommercial.PaymentAnomalyRow
			if err := db.Where("order_id = ?", orderID).Find(&before).Error; err != nil {
				t.Fatal(err)
			}
			if len(before) != 1 {
				t.Fatalf("confirmation should persist one immutable fact before drain, got %+v", before)
			}
			tc.install(t, db)
			if err := svc.disposeOverPayment(context.Background(), ev, time.Now()); err == nil {
				t.Fatal("injected transient failure must be returned")
			}
			var stored repocommercial.OutboxEvent
			if err := db.Where("event_key = ?", ev.EventKey).First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			if stored.State != repocommercial.OutboxStatePending {
				t.Fatalf("transient failure must leave event pending, got %s", stored.State)
			}
			var after []repocommercial.PaymentAnomalyRow
			if err := db.Where("order_id = ?", orderID).Find(&after).Error; err != nil {
				t.Fatal(err)
			}
			if len(after) != 1 || !reflect.DeepEqual(after[0], before[0]) {
				t.Fatalf("failed disposal must preserve its single durable fact, got %+v", after)
			}
		})
	}
}

// TestOverPaymentAnomalyCarriesMerchantOrderID（OCR84-R1-13 medium）：
// PaymentAnomalyRow.AttemptID 列的文档语义是 merchant_order_id（mo_ 渠道身份），
// 同表另两个写入方（buildMismatchAnomaly / recoverMismatchedCollection）写的都是
// mo_ 前缀。新 over_payment 载荷内联携带 merchant_order_id；缺该键的旧载荷由
// disposeOverPayment 回读 attempt 行取回——两条路径都不允许把 att_ 内部主键写进
// 该列（运营按 merchant_order_id 关联渠道单据时对不上的口径分裂）。
func TestOverPaymentAnomalyCarriesMerchantOrderID(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	ctx := context.Background()
	const orderID = "ord-over-mid"
	seedPaidOrder(t, store, orderID, 7, 9900)
	seedSecondChannelSuccess(t, store, orderID, 7, 9900)
	// 先把 over_payment 事件载荷改写为旧形态（剥掉 merchant_order_id 键），
	// 验证回读兜底；随后再以内联载荷路径复验（同一断言面）。
	ev := overPaymentEvent(t, db)
	var fields map[string]any
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "merchant_order_id")
	legacy, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&repocommercial.OutboxEvent{}).Where("event_key = ?", ev.EventKey).
		Update("payload_json", string(legacy)).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", orderID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.AttemptID != "mo2-"+orderID {
		t.Fatalf("legacy payload: the anomaly attempt_id must recover the mo_ channel identity by attempt re-read, got %q", anomaly.AttemptID)
	}
	// 内联载荷路径：重放一笔新的 over_payment（第二个渠道成功），事件载荷现在
	// 自带 merchant_order_id，直接落 mo_ 身份。
	seedSecondChannelSuccessOn(t, store, orderID, 7, 9900, "mo3-"+orderID, "txn3-"+orderID)
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var anomalies []repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", orderID).Find(&anomalies).Error; err != nil {
		t.Fatal(err)
	}
	if len(anomalies) != 2 {
		t.Fatalf("expect the inline-payload over_payment to land its own anomaly row, got %+v", anomalies)
	}
	for _, row := range anomalies {
		if !strings.HasPrefix(row.AttemptID, "mo") {
			t.Fatalf("every over_payment anomaly must carry a mo_ identity, got %+v", anomalies)
		}
	}
}

// seedSecondChannelSuccessOn 是 seedSecondChannelSuccess 的可参数化变体
// （第三个渠道成功：独立 merchant order / transaction 身份）。
func seedSecondChannelSuccessOn(t *testing.T, store *repocommercial.OrderStore, orderID string, tenant uint64, amountFen int64, merchantOrderID, txn string) {
	t.Helper()
	ctx := context.Background()
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att-" + merchantOrderID, TenantID: tenant, OrderID: orderID, Provider: "alipay", Merchant: "weknora",
		MerchantOrderID: merchantOrderID, AmountFen: amountFen, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, domain.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: merchantOrderID, Provider: "alipay", Merchant: "weknora",
		Transaction: txn, Amount: domain.CNYFen(amountFen), Currency: "CNY", State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
}

// TestOverPaymentDrainReplayYieldsSingleAnomaly（Review Focus 3）：同一
// over_payment 事件被重置 pending 后再 drain 一轮——唯一键幂等保证 anomaly
// 行数仍为 1。
func TestOverPaymentDrainReplayYieldsSingleAnomaly(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	ctx := context.Background()
	const orderID = "ord-over-2"
	seedPaidOrder(t, store, orderID, 7, 9900, db)
	seedSecondChannelSuccess(t, store, orderID, 7, 9900)
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	// Replay the event: reset it to pending (lease expired) and drain again.
	if err := db.Model(&repocommercial.OutboxEvent{}).
		Where("kind = ?", repocommercial.OutboxKindOverPaid).
		Updates(map[string]interface{}{"state": repocommercial.OutboxStatePending, "lease_until": time.Now().Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&repocommercial.PaymentAnomalyRow{}).Where("order_id = ?", orderID).Count(&n)
	if n != 1 {
		t.Fatalf("a replayed over_payment event must not mint a second anomaly row, got %d", n)
	}
}

// TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain（Review Focus 5 /
// A-32 纪律）：单条 over_payment dispose 失败（anomaly 表故障）不得中止整批
// drain——同批的 fulfill 事件仍被处理（订单推进 fulfilled），over_payment
// 事件保持 pending 等下一轮，整体不报错不 panic。
func TestOverPaymentDisposeFailureDoesNotBlockFulfillDrain(t *testing.T) {
	gw := &stubGateway{findable: true}
	svc, db, store := setupFulfillment(t, gw, nil)
	ctx := context.Background()
	const orderID = "ord-over-3"
	seedPaidOrder(t, store, orderID, 7, 9900, db)
	seedSecondChannelSuccess(t, store, orderID, 7, 9900)
	// The queue now holds BOTH a fulfill event and an over_payment event.
	if got := len(fulfillEvents(t, db)); got != 1 {
		t.Fatalf("setup: want 1 fulfill event, got %d", got)
	}
	if err := db.Migrator().DropTable(&repocommercial.PaymentAnomalyRow{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatalf("a single dispose failure must not fail the whole drain pass: %v", err)
	}
	// The fulfill event was still processed to completion.
	if s := orderState(t, db, orderID); s != domain.OrderStateFulfilled {
		t.Fatalf("the fulfill event must not be starved by the dispose failure, order=%s", s)
	}
	// The over_payment event stays pending for the next pass.
	ev := overPaymentEvent(t, db)
	if ev.State != repocommercial.OutboxStatePending {
		t.Fatalf("the failed dispose must keep the event pending, got %s", ev.State)
	}
}
