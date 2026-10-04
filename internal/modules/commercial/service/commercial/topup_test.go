package commercial

// #85 充值 Credits：四条验收 + R-GA 商品面测试。
// Top-up 履约走 commercial platform 钱包命令轨（FakeAdapter 仿真），
// OpenMeter gateway 仅作为对照断言（never applied）。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/commercialplatform"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTopUpFulfillment(t *testing.T) (*FulfillmentService, *gorm.DB, *repocommercial.OrderStore, *commercialplatform.FakeAdapter, *stubGateway) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	closeSQLiteDBOnCleanup(t, db)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PaymentAnomalyRow{}, &repocommercial.QuoteRow{}); err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewOrderStore(db)
	gw := &stubGateway{}
	fake := commercialplatform.NewFakeAdapter()
	svc, err := NewFulfillmentService(db, gw, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, store, fake, gw
}

// seedPaidTopUpOrder drives the real #85 C01 shape: a top_up-quoted
// PURCHASE order with a registered winner attempt and confirmed payment.
func seedPaidTopUpOrder(t *testing.T, store *repocommercial.OrderStore, db *gorm.DB, orderID string, tenant uint64, amountFen int64) {
	t.Helper()
	ctx := context.Background()
	quoteID := "q-" + orderID
	snap := fmt.Sprintf(`{"plan_key":"credits","price_fen":%d,"credits_micro":%d,"currency":"CNY","line_items":[{"kind":"top_up","name":"充值 Credits","amount_fen":%d}]}`,
		amountFen, amountFen*100, amountFen)
	if err := db.Create(&repocommercial.QuoteRow{ID: quoteID, TenantID: tenant, SnapshotJSON: snap}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: orderID, TenantID: tenant, QuoteID: quoteID, Kind: domain.OrderKindPurchase,
		AmountFen: amountFen, Currency: "CNY",
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

func topUpSnapshot(t *testing.T, fake *commercialplatform.FakeAdapter, tenant uint64) domain.BenefitsSnapshot {
	t.Helper()
	snap, err := fake.ReadSnapshot(context.Background(), domain.SnapshotQuery{Kind: domain.SnapshotKindBenefits, TenantID: tenant})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Benefits == nil {
		t.Fatal("nil benefits snapshot")
	}
	return *snap.Benefits
}

// AC1: 未支付订单绝不入账。
func TestTopUpAC1UnpaidOrderGrantsNoCredits(t *testing.T) {
	svc, db, store, fake, gw := setupTopUpFulfillment(t)
	ctx := context.Background()
	if err := db.Create(&repocommercial.QuoteRow{ID: "q-unpaid", TenantID: 7, SnapshotJSON: `{"line_items":[{"kind":"top_up"}]}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: "ord-unpaid", TenantID: 7, QuoteID: "q-unpaid", Kind: domain.OrderKindPurchase, AmountFen: 500, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att-unpaid", TenantID: 7, OrderID: "ord-unpaid", Provider: "alipay", Merchant: "weknora",
		MerchantOrderID: "mo-unpaid", AmountFen: 500, Currency: "CNY",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord-unpaid"); got != domain.OrderStatePending {
		t.Fatalf("unpaid order must stay pending, got %s", got)
	}
	if recs := fulfillmentRecords(t, db, "ord-unpaid"); len(recs) != 0 {
		t.Fatalf("unpaid order must claim no fulfillment records, got %+v", recs)
	}
	b := topUpSnapshot(t, fake, 7)
	if len(b.Batches) != 0 || b.BalanceMicro != 0 {
		t.Fatalf("unpaid order must grant nothing, batches=%+v balance=%d", b.Batches, b.BalanceMicro)
	}
	if gw.appliedCount() != 0 {
		t.Fatalf("legacy gateway rail must stay untouched, applies=%d", gw.appliedCount())
	}
}

// AC2: 支付成功后 credits 恰好一次到达，带 source 与 expiry。
func TestTopUpAC2PaidArrivesExactlyOnceWithSourceAndExpiry(t *testing.T) {
	svc, db, store, fake, gw := setupTopUpFulfillment(t)
	ctx := context.Background()
	seedPaidTopUpOrder(t, store, db, "ord-85-ac2", 7, 500) // 5 CNY
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord-85-ac2"); got != domain.OrderStateFulfilled {
		t.Fatalf("paid top-up must fulfill, got %s", got)
	}
	recs := fulfillmentRecords(t, db, "ord-85-ac2")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateApplied {
		t.Fatalf("exactly one applied record expected, got %+v", recs)
	}
	want := domain.TopUpWalletName(7, "ord-85-ac2")
	if recs[0].ExternalID != want {
		t.Fatalf("receipt must be the top-up wallet identity %q, got %q", want, recs[0].ExternalID)
	}
	if recs[0].Credits != 500*100 {
		t.Fatalf("credits must follow R-GA fen×100, got %d", recs[0].Credits)
	}
	b := topUpSnapshot(t, fake, 7)
	if len(b.Batches) != 1 {
		t.Fatalf("exactly one batch expected, got %+v", b.Batches)
	}
	got := b.Batches[0]
	if got.Source != domain.BatchSourceTopUp {
		t.Fatalf("batch must carry source=topup, got %q", got.Source)
	}
	if got.BalanceMicro != 500*100 {
		t.Fatalf("balance must be fen×100 micro, got %d", got.BalanceMicro)
	}
	if got.ExpiresAt.Before(time.Now().AddDate(0, 11, 29)) {
		t.Fatalf("top-up expiry must be the 12-month TTL class, got %s", got.ExpiresAt)
	}
	if got.WalletRef != want {
		t.Fatalf("batch wallet ref must be the granted identity, got %q", got.WalletRef)
	}
	// 恰好一次：重放一遍，余额与批次数不变。
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	b2 := topUpSnapshot(t, fake, 7)
	if len(b2.Batches) != 1 || b2.BalanceMicro != 500*100 {
		t.Fatalf("replay must not double-grant, batches=%+v balance=%d", b2.Batches, b2.BalanceMicro)
	}
	if gw.appliedCount() != 0 {
		t.Fatalf("legacy gateway rail must stay untouched, applies=%d", gw.appliedCount())
	}
}

// AC3: 支付成功但到账未确认 → 订单停留在 paid（权益处理中），绝不记成功。
func TestTopUpAC3UnconfirmedArrivalStaysProcessing(t *testing.T) {
	svc, db, store, fake, _ := setupTopUpFulfillment(t)
	ctx := context.Background()
	seedPaidTopUpOrder(t, store, db, "ord-85-ac3", 7, 300)
	fake.FailSubmitsWith(errors.New("simulated transport loss"))
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord-85-ac3"); got != domain.OrderStatePaid {
		t.Fatalf("unconfirmed arrival must keep the order paid (权益处理中), got %s", got)
	}
	recs := fulfillmentRecords(t, db, "ord-85-ac3")
	if len(recs) != 1 || recs[0].State != domain.FulfillmentStateAttention {
		t.Fatalf("unconfirmed arrival must surface attention, never success, got %+v", recs)
	}
	if recs[0].ExternalID != "" {
		t.Fatalf("no receipt may be recorded on an unconfirmed arrival, got %q", recs[0].ExternalID)
	}
}

// AC4: 响应丢失 → 以原始身份（同一钱包名/命令键）恢复，不重复入账。
func TestTopUpAC4LostResponseRecoversByOriginalIdentity(t *testing.T) {
	svc, db, store, fake, gw := setupTopUpFulfillment(t)
	ctx := context.Background()
	seedPaidTopUpOrder(t, store, db, "ord-85-ac4", 7, 800)
	fake.FailSubmitsWith(domain.ErrPlatformUnreachable)
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err) // attention path never aborts the drain
	}
	if got := orderState(t, db, "ord-85-ac4"); got != domain.OrderStatePaid {
		t.Fatalf("lost response must keep the order paid, got %s", got)
	}
	fake.FailSubmitsWith(nil)
	if err := svc.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if got := orderState(t, db, "ord-85-ac4"); got != domain.OrderStateFulfilled {
		t.Fatalf("replay by original identity must fulfill, got %s", got)
	}
	b := topUpSnapshot(t, fake, 7)
	if len(b.Batches) != 1 || b.BalanceMicro != 800*100 {
		t.Fatalf("recovery must count the grant exactly once, batches=%+v balance=%d", b.Batches, b.BalanceMicro)
	}
	if b.Batches[0].WalletRef != domain.TopUpWalletName(7, "ord-85-ac4") {
		t.Fatalf("recovery must reuse the original wallet identity, got %q", b.Batches[0].WalletRef)
	}
	if gw.appliedCount() != 0 {
		t.Fatalf("legacy gateway rail must stay untouched, applies=%d", gw.appliedCount())
	}
}

// R-GA 兑换率：1 CNY = 10,000 micro-credits。
func TestTopUpConversionFenTimesHundred(t *testing.T) {
	cases := []struct {
		fen   domain.CNYFen
		micro domain.Credits
	}{
		{100, 10_000},
		{500, 50_000},
		{12_345, 1_234_500},
	}
	for _, c := range cases {
		if got := TopUpCredits(c.fen); got != c.micro {
			t.Fatalf("TopUpCredits(%d) = %d, want %d", c.fen, got, c.micro)
		}
	}
}

func TestTopUpWalletNameDeterministicPerOrder(t *testing.T) {
	a := domain.TopUpWalletName(7, "ord-x")
	if a != domain.TopUpWalletName(7, "ord-x") {
		t.Fatal("wallet name must be deterministic for replay safety")
	}
	if a == domain.TopUpWalletName(7, "ord-y") {
		t.Fatal("distinct orders must address distinct wallets")
	}
	if prefix := domain.ExternalCustomerID(7) + "-topup-"; !strings.HasPrefix(a, prefix) {
		t.Fatalf("wallet name must carry the tenant top-up prefix %q, got %q", prefix, a)
	}
}

// G-A: 充值 quote 面冻结 fen×100、单条 top_up 行、整元校验。
func TestCreateTopUpQuoteConversionAndFreeze(t *testing.T) {
	svc, _, _ := newOrderTestEnv(t)
	ctx := context.Background()
	q, err := svc.CreateTopUpQuote(ctx, 101, 500)
	if err != nil {
		t.Fatal(err)
	}
	if q.AmountFen != 500 || q.CreditsMicro != 50_000 {
		t.Fatalf("quote face must be fen×100, got %+v", q)
	}
	if len(q.LineItems) != 1 || q.LineItems[0].Kind != "top_up" || q.LineItems[0].AmountFen != 500 {
		t.Fatalf("quote must freeze exactly one top_up line, got %+v", q.LineItems)
	}
	if q.Currency != "CNY" {
		t.Fatalf("closed currency expected, got %q", q.Currency)
	}
	// 订单面：同一 quote→order 链，amount 即 quote 冻结价。
	order, err := svc.CreateOrder(ctx, 101, q.ID, "wechat")
	if err != nil {
		t.Fatal(err)
	}
	if order.AmountFen != 500 || order.State != domain.OrderStatePending {
		t.Fatalf("top-up order must open payable from the frozen quote, got %+v", order)
	}
	if _, err := svc.CreateOrder(ctx, 101, q.ID, "wechat"); !errors.Is(err, repocommercial.ErrQuoteAlreadyUsed) {
		t.Fatalf("quote reuse must be refused: %v", err)
	}
}

func TestCreateTopUpQuoteRejectsNonWholeCNY(t *testing.T) {
	svc, _, _ := newOrderTestEnv(t)
	ctx := context.Background()
	for _, fen := range []int64{0, -100, 150, 99} {
		if _, err := svc.CreateTopUpQuote(ctx, 101, fen); !errors.Is(err, repocommercial.ErrInvalidQuoteRow) {
			t.Fatalf("CreateTopUpQuote(%d) must be rejected as invalid, got %v", fen, err)
		}
	}
}
