package handler

// POST /api/v1/commercial/callbacks/:provider 的回调入账链回归（issue #82
// 真实流程验证缺陷 2）：下单侧 attempt.merchant 必须写渠道商户身份
// （SellerID/MchID，即 Provider.MerchantID()），回调侧 resolveByMerchantOrderID
// 与 ConfirmPayment 都按 (provider, merchant, merchant_order_id) 三元组解析——
// 任何一侧写 provider 名字面量都会让真实渠道回调永久 404（验证实录：注册
// 'alipay' vs 通知 SellerID → no registered payment attempt）。
//
// 本文件用真实 OrderService（走完整 openOrder 落库路径）+ 真实
// PaymentCallbacksHandler + 真实 ConfirmPayment 事务，按支付宝分支验收：
// 匿名签名通知形状 → 验签事实 → attempt 解析 → 订单 paid → outbox 恰一。

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/commercial/commercialplatform"
	"github.com/Tencent/WeKnora/internal/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// callbackAlipayStub 模拟支付宝渠道边界：MerchantID 返回支付宝形状的
// SellerID（绝不能等于 provider 名），Verify 返回同源签名事实。
type callbackAlipayStub struct{ fact commercial.PaymentFact }

func (p *callbackAlipayStub) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending, ProviderID: "qr"}, nil
}
func (p *callbackAlipayStub) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) Close(context.Context, string) error { return nil }
func (p *callbackAlipayStub) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return p.fact, nil
}
func (p *callbackAlipayStub) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackAlipayStub) MerchantID() string { return "2088000000000000" }

func newCallbackTestEnv(t *testing.T) (*gorm.DB, *callbackAlipayStub, *gin.Engine, *commercialsvc.OrderService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	stub := &callbackAlipayStub{}
	providers := map[string]payment.Provider{payment.ProviderAlipay: stub}
	orders, err := commercialsvc.NewOrderService(db, providers)
	if err != nil {
		t.Fatal(err)
	}
	callbacks := NewPaymentCallbacksHandler(db, providers, nil)
	engine := gin.New()
	engine.POST("/api/v1/commercial/callbacks/:provider", callbacks.HandleProviderCallback)
	return db, stub, engine, orders
}

func seedCallbackPublishedPlan(t *testing.T, db *gorm.DB) {
	t.Helper()
	def, _ := json.Marshal(commercial.PlanVersion{Key: "pro", Version: 1, Price: 99_00, Monthly: 9_900_000})
	if err := db.Create(&repocommercial.PlanRow{
		PlanKey: "pro", Version: 1, DefinitionJSON: string(def),
		ExternalID: "ext-pro-1", State: commercial.PlanStatePublished,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func notifyFact(t *testing.T, db *gorm.DB, stub *callbackAlipayStub, orderID string) commercial.PaymentFact {
	t.Helper()
	var att repocommercial.PaymentAttemptRow
	if err := db.Where("order_id = ?", orderID).First(&att).Error; err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	if att.Merchant != stub.MerchantID() {
		t.Fatalf("attempt.merchant must be the channel merchant id %q (Provider.MerchantID), got %q — a provider-name literal here makes every genuine callback unresolvable (issue #82 defect 2)",
			stub.MerchantID(), att.Merchant)
	}
	return commercial.PaymentFact{
		Provider: payment.ProviderAlipay, Merchant: stub.MerchantID(),
		AttemptID: att.MerchantOrderID, Transaction: "2026092500000000",
		Amount: commercial.CNYFen(att.AmountFen), Currency: att.Currency,
		State: "succeeded",
	}
}

func outboxCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.OutboxEvent{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// ---- #82 Task 14 (OCR r2): callback body cap ----

// TestPaymentCallbackRejectsOversizedBody（D14 / r2 auth.go:71 medium）：回调
// 端点是匿名可达面，签名校验发生在读完整个 body 之后——无上限的 io.ReadAll
// 让任何匿名客户端在验签失败前把任意大请求体完整缓冲进内存（内存耗尽 DoS，
// 还可放大 RSA 验签的 CPU 开销）。Alipay/WeChat notify 报文只有几 KB：1 MiB
// 封顶后超大报文走既有 unreadable-body 失败面（非 5xx、零落库、不进验签）。
func TestPaymentCallbackRejectsOversizedBody(t *testing.T) {
	db, _, engine, _ := newCallbackTestEnv(t)
	// 2 MiB 匿名超大报文（无需任何有效签名形状）。
	big := bytes.Repeat([]byte("x"), 2<<20)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/callbacks/alipay", bytes.NewReader(big))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code >= 500 {
		t.Fatalf("an oversized body must answer the existing failure face (never 5xx), got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an oversized body must be rejected by the cap (unreadable-body face), got %d: %s", rec.Code, rec.Body.String())
	}
	// 零落库：attempts/orders/outbox 均无新增行。
	var attempts, orders int64
	if err := db.Model(&repocommercial.PaymentAttemptRow{}).Count(&attempts).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&repocommercial.OrderRow{}).Count(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || orders != 0 || outboxCount(t, db) != 0 {
		t.Fatalf("an oversized body must persist NOTHING, attempts=%d orders=%d outbox=%d", attempts, orders, outboxCount(t, db))
	}
}

// TestAlipayCallbackConfirmsOrderRegisteredWithChannelMerchant: 下单（真实
// openOrder）→ 匿名签名通知 → 200 "success" + 订单 paid + outbox 恰一；重放
// 幂等。整链复刻流程验证第 9/12 断言（修复前在 attempt 解析步 404）。
func TestAlipayCallbackConfirmsOrderRegisteredWithChannelMerchant(t *testing.T) {
	db, stub, engine, orders := newCallbackTestEnv(t)
	seedCallbackPublishedPlan(t, db)
	ctx := context.Background()
	q, err := orders.CreateQuote(ctx, 601, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, 601, q.ID, payment.ProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	stub.fact = notifyFact(t, db, stub, order.ID)

	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/commercial/callbacks/alipay", strings.NewReader("notify=1"))
		engine.ServeHTTP(w, req)
		return w
	}

	w := post()
	if w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("verified alipay notify must confirm the order (200 \"success\"), got %d %q", w.Code, w.Body.String())
	}
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePaid {
		t.Fatalf("order state = %s, want paid", row.State)
	}
	if n := outboxCount(t, db); n != 1 {
		t.Fatalf("exactly one fulfill outbox event expected, got %d", n)
	}

	// 重放同一签名通知：幂等 200，不产生第二个事件。
	if w := post(); w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("duplicate notify must replay the original success, got %d %q", w.Code, w.Body.String())
	}
	if n := outboxCount(t, db); n != 1 {
		t.Fatalf("duplicate notify must not add events, got %d", n)
	}
}

// TestCallbackMerchantMismatchNeverResolves: 三元组中 merchant 是纵深防御
// 维度——一个不同商户身份的（哪怕验签通过的）事实永远解析不到他人 attempt。
func TestCallbackMerchantMismatchNeverResolves(t *testing.T) {
	db, stub, engine, orders := newCallbackTestEnv(t)
	seedCallbackPublishedPlan(t, db)
	ctx := context.Background()
	q, err := orders.CreateQuote(ctx, 602, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, 602, q.ID, payment.ProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	fact := notifyFact(t, db, stub, order.ID)
	fact.Merchant = "2088999999999999" // a different (foreign) merchant account
	stub.fact = fact
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/commercial/callbacks/alipay", strings.NewReader("notify=1"))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign-merchant fact must not resolve any attempt, got %d %q", w.Code, w.Body.String())
	}
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePending {
		t.Fatalf("foreign-merchant fact must leave the order pending, got %s", row.State)
	}
	if n := outboxCount(t, db); n != 0 {
		t.Fatalf("no event expected, got %d", n)
	}
}

// ---- issue #83 Task 2: the WeChat leg of the same callback chain ----
//
// 与支付宝腿同一 handler、同一 ConfirmPayment 事务、同一 outbox——只是回调
// 验签换成真实 WechatProvider.Verify（serial 选择/时钟窗/RSA-SHA256 签名/
// AES-256-GCM 解密/商户匹配全部真实走），落单侧用渠道商户号一致的 create
// stub（MerchantID()=="1900000001"，与真实微信 provider 同一商户身份）。

// callbackWechatCreateStub 只替身落单侧的渠道 Create 边界；回调验签一律用
// 真实 WechatProvider。
type callbackWechatCreateStub struct{}

func (p *callbackWechatCreateStub) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending, ProviderID: "wx-native",
		CheckoutURL: "weixin://wxpay/bizpayurl?pr=cb83"}, nil
}
func (p *callbackWechatCreateStub) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StatePending}, nil
}
func (p *callbackWechatCreateStub) Close(context.Context, string) error { return nil }
func (p *callbackWechatCreateStub) Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, errors.New("create stub never verifies callbacks")
}
func (p *callbackWechatCreateStub) Refund(context.Context, payment.RefundRequest) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackWechatCreateStub) QueryRefund(context.Context, string) (payment.RefundResult, error) {
	return payment.RefundResult{State: payment.StatePending}, nil
}
func (p *callbackWechatCreateStub) MerchantID() string { return "1900000001" }

// wechatCallbackEnv carries the WeChat-leg fixture: a REAL WechatProvider
// (platform key + APIv3 key loaded through NewWechatProvider from TempDir
// files), the signing key kept test-side, a real OrderService for placing
// the order, and the callback engine.
type wechatCallbackEnv struct {
	db       *gorm.DB
	provider *payment.WechatProvider
	platKey  *rsa.PrivateKey
	apiv3    []byte
	engine   *gin.Engine
	orders   *commercialsvc.OrderService
}

func newWechatCallbackEnv(t *testing.T) *wechatCallbackEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.Subscription{}, &commercialsvc.FulfillmentRecord{},
		&repocommercial.PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	// Platform key pair: private stays test-side for signing, the public key
	// goes through the configured-reference constructor exactly like prod.
	platKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pubBytes, err := x509.MarshalPKIXPublicKey(&platKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	if err := os.WriteFile(filepath.Join(dir, "platform_pub.pem"), pubPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	apiv3 := []byte("0123456789abcdef0123456789abcdef")
	if err := os.WriteFile(filepath.Join(dir, "apiv3.key"), apiv3, 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := payment.NewWechatProvider(payment.WechatConfig{
		AppID: "wx-test-app", MchID: "1900000001",
		PlatformCerts: []payment.WechatPlatformCertRef{
			{Serial: "PLAT-SERIAL-83", PublicKeyPath: filepath.Join(dir, "platform_pub.pem")},
		},
		APIv3KeyPath: filepath.Join(dir, "apiv3.key"),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Order placement goes through the real OrderService with the create
	// stub (same merchant identity); callbacks verify through the REAL
	// provider — the two maps are deliberately distinct.
	orders, err := commercialsvc.NewOrderService(db, map[string]payment.Provider{
		payment.ProviderWechat: &callbackWechatCreateStub{},
	})
	if err != nil {
		t.Fatal(err)
	}
	callbacks := NewPaymentCallbacksHandler(db, map[string]payment.Provider{
		payment.ProviderWechat: provider,
	}, nil)
	engine := gin.New()
	engine.POST("/api/v1/commercial/callbacks/:provider", callbacks.HandleProviderCallback)
	return &wechatCallbackEnv{db: db, provider: provider, platKey: platKey, apiv3: apiv3, engine: engine, orders: orders}
}

// wechatCBNotify builds one SIGNED WeChat TRANSACTION.SUCCESS notification
// for (outTradeNo, txnID, totalFen, tradeState) — the same construction
// wechat_test.go's buildNotification/signCallback/callbackHeaders perform
// (copied across the package boundary; kept in sync with WX-02).
func wechatCBNotify(t *testing.T, env *wechatCallbackEnv, outTradeNo, txnID string, totalFen int64, tradeState string) ([]byte, http.Header) {
	t.Helper()
	plain, err := json.Marshal(map[string]interface{}{
		"mchid":          "1900000001",
		"appid":          "wx-test-app",
		"out_trade_no":   outTradeNo,
		"transaction_id": txnID,
		"trade_state":    tradeState,
		"amount":         map[string]interface{}{"total": totalFen, "currency": "CNY"},
	})
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(env.apiv3)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte("resnonce1234"), plain, []byte("transaction"))
	body, err := json.Marshal(map[string]interface{}{
		"id":         "evt-cb-83",
		"event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": "transaction",
			"nonce":           "resnonce1234",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	const hdrNonce = "hdrnonce83"
	message := []byte(ts + "\n" + hdrNonce + "\n" + string(body) + "\n")
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, env.platKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Wechatpay-Serial", "PLAT-SERIAL-83")
	h.Set("Wechatpay-Timestamp", ts)
	h.Set("Wechatpay-Nonce", hdrNonce)
	h.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(sig))
	return body, h
}

func postWechatNotify(engine *gin.Engine, body []byte, h http.Header) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commercial/callbacks/wechat", bytes.NewReader(body))
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	engine.ServeHTTP(w, req)
	return w
}

// placeWechatOrder seeds the published plan, cuts a quote and opens the
// order through the REAL OrderService (the create stub answers the channel
// leg). Returns the order view and the registered attempt.
func placeWechatOrder(t *testing.T, env *wechatCallbackEnv, tenant uint64) (commercialsvc.OrderView, repocommercial.PaymentAttemptRow) {
	t.Helper()
	seedCallbackPublishedPlan(t, env.db)
	ctx := context.Background()
	q, err := env.orders.CreateQuote(ctx, tenant, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := env.orders.CreateOrder(ctx, tenant, q.ID, payment.ProviderWechat)
	if err != nil {
		t.Fatal(err)
	}
	var att repocommercial.PaymentAttemptRow
	if err := env.db.Where("order_id = ?", order.ID).First(&att).Error; err != nil {
		t.Fatal(err)
	}
	if att.Merchant != "1900000001" || att.Provider != payment.ProviderWechat {
		t.Fatalf("attempt must be registered under the channel merchant identity, got provider=%q merchant=%q", att.Provider, att.Merchant)
	}
	return order, att
}

func TestWechatCallbackConfirmsOrderAndEmitsFulfillEvent(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 701)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_83", 9900, "SUCCESS")

	w := postWechatNotify(env.engine, body, h)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":"SUCCESS"`) {
		t.Fatalf("verified wechat notify must confirm, got %d %q", w.Code, w.Body.String())
	}
	var after repocommercial.PaymentAttemptRow
	if err := env.db.Where("id = ?", att.ID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.State != repocommercial.PaymentAttemptStateSucceeded || after.ProviderTransactionID == nil || *after.ProviderTransactionID != "wx_txn_83" {
		t.Fatalf("attempt must record the channel transaction, got state=%s txn=%v", after.State, after.ProviderTransactionID)
	}
	var row repocommercial.OrderRow
	if err := env.db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePaid {
		t.Fatalf("order must be paid, got %s", row.State)
	}
	var events []repocommercial.OutboxEvent
	if err := env.db.Where("kind = ?", repocommercial.OutboxKindFulfill).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventKey != repocommercial.OutboxKindFulfill+":"+order.ID {
		t.Fatalf("exactly one fulfill event keyed by the order expected, got %+v", events)
	}
	var payload struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(events[0].PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Provider != payment.ProviderWechat {
		t.Fatalf("fulfill payload must carry the wechat provider, got %q", payload.Provider)
	}
}

// TestWechatCallbackDrainsToFulfilledWithFakePlatform is the composite pin of
// issue #83's core promise — 复用同一激活流程: the WeChat fact drives the
// VERY SAME #82 chain (outbox → PurchaseFulfiller settle → observe → D6' →
// grant → activation claim) to a fulfilled order, exactly once.
func TestWechatCallbackDrainsToFulfilledWithFakePlatform(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 702)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_83f", 9900, "SUCCESS")
	if w := postWechatNotify(env.engine, body, h); w.Code != http.StatusOK {
		t.Fatalf("callback failed: %d %q", w.Code, w.Body.String())
	}
	// The published plan's publication row (the fulfiller's PlanCode source).
	if err := env.db.Create(&repocommercial.PublicationRow{
		CommandKey: "publish_cb83:pro:1", PlanKey: "pro", Version: 1, PlanCode: "pro",
		ReceiptJSON: "{}", PublishedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// The authority side: the gated purchase subscription exists, the
	// webhook-finalize stand-in activates it, and the finalized invoice
	// lines answer the D6' re-check.
	fake := commercialplatform.NewFakeAdapter()
	ext := commercial.ExternalPurchaseSubscriptionID(702)
	if _, err := fake.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKindCreatePurchaseSubscription,
		Key:  commercial.CreatePurchaseSubscriptionCommandKey(ext, "pro"),
		Payload: commercial.CreatePurchaseSubscriptionPayload{
			TenantID: 702, ExternalCustomerID: commercial.ExternalCustomerID(702),
			ExternalPurchaseSubscriptionID: ext, PlanCode: "pro",
			AmountFen: 9900, Currency: commercial.CurrencyCNY,
		},
	}); err != nil {
		t.Fatal(err)
	}
	fake.SetPurchaseInvoiceFees(ext, []commercial.InvoiceLineSnapshot{
		{Kind: "subscription_fee", Name: "pro", AmountFen: 9900},
	})
	fake.ActivatePurchase(ext) // the built-in webhook finalize stand-in

	fulfiller, err := commercialsvc.NewPurchaseFulfiller(env.db, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []repocommercial.OutboxEvent
	if err := env.db.Where("kind = ?", repocommercial.OutboxKindFulfill).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("one fulfill event expected, got %d", len(events))
	}
	if err := fulfiller.Fulfill(context.Background(), events[0]); err != nil {
		t.Fatalf("the wechat fact must drain through the shared #82 fulfiller: %v", err)
	}
	var row repocommercial.OrderRow
	if err := env.db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStateFulfilled {
		t.Fatalf("order must be fulfilled by the reused activation chain, got %s", row.State)
	}
	var recs []commercialsvc.FulfillmentRecord
	if err := env.db.Where("order_id = ?", order.ID).Find(&recs).Error; err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Kind != "purchase_activation" || recs[0].State != commercial.FulfillmentStateApplied {
		t.Fatalf("exactly one applied purchase_activation record expected, got %+v", recs)
	}
}

func TestWechatCallbackDuplicateDeliveryIdempotent(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 703)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_83d", 9900, "SUCCESS")
	for i := 0; i < 2; i++ {
		w := postWechatNotify(env.engine, body, h)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":"SUCCESS"`) {
			t.Fatalf("delivery %d must ack success, got %d %q", i+1, w.Code, w.Body.String())
		}
	}
	var nFulfill, nAttempts int64
	if err := env.db.Model(&repocommercial.OutboxEvent{}).Where("kind = ?", repocommercial.OutboxKindFulfill).Count(&nFulfill).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.db.Model(&repocommercial.PaymentAttemptRow{}).
		Where("state = ?", repocommercial.PaymentAttemptStateSucceeded).Count(&nAttempts).Error; err != nil {
		t.Fatal(err)
	}
	if nFulfill != 1 || nAttempts != 1 {
		t.Fatalf("duplicate delivery must not duplicate anything: fulfill=%d succeeded_attempts=%d", nFulfill, nAttempts)
	}
	var row repocommercial.OrderRow
	if err := env.db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePaid || row.Version != 2 {
		t.Fatalf("order must stay paid at the original version, got %s v%d", row.State, row.Version)
	}
}

func TestWechatCallbackDifferentTransactionOverPaidAudit(t *testing.T) {
	env := newWechatCallbackEnv(t)
	_, att := placeWechatOrder(t, env, 704)
	first, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_a", 9900, "SUCCESS")
	if w := postWechatNotify(env.engine, first, h); w.Code != http.StatusOK {
		t.Fatalf("first confirm failed: %d %q", w.Code, w.Body.String())
	}
	// A SECOND success on the same out_trade_no under a DIFFERENT channel
	// transaction (the user double-pays): audited, never re-fulfilled.
	second, h2 := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_b", 9900, "SUCCESS")
	w := postWechatNotify(env.engine, second, h2)
	if w.Code != http.StatusOK {
		t.Fatalf("over-payment is an audit fact, not an error: %d %q", w.Code, w.Body.String())
	}
	var overs []repocommercial.OutboxEvent
	if err := env.db.Where("kind = ?", repocommercial.OutboxKindOverPaid).Find(&overs).Error; err != nil {
		t.Fatal(err)
	}
	if len(overs) != 1 || !strings.Contains(overs[0].EventKey, "wx_txn_b") {
		t.Fatalf("exactly one over-payment audit keyed by the new transaction expected, got %+v", overs)
	}
	var nFulfill int64
	if err := env.db.Model(&repocommercial.OutboxEvent{}).Where("kind = ?", repocommercial.OutboxKindFulfill).Count(&nFulfill).Error; err != nil {
		t.Fatal(err)
	}
	if nFulfill != 1 {
		t.Fatalf("over-payment must not mint a second fulfillment right, got %d", nFulfill)
	}
}

func TestWechatCallbackRejectsTamperedSignature(t *testing.T) {
	env := newWechatCallbackEnv(t)
	_, att := placeWechatOrder(t, env, 705)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_t", 9900, "SUCCESS")
	// Flip one byte of the signed ENVELOPE (the transaction id lives inside
	// the ciphertext, so the tamper targets a plaintext field): the
	// signature is over the raw body and must stop verifying.
	tampered := []byte(strings.Replace(string(body), "evt-cb-83", "evt-cb-8X", 1))
	w := postWechatNotify(env.engine, tampered, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("tampered body must be rejected 401, got %d %q", w.Code, w.Body.String())
	}
	if n := outboxCount(t, env.db); n != 0 {
		t.Fatalf("a rejected callback must persist nothing, got %d events", n)
	}
	var att2 repocommercial.PaymentAttemptRow
	if err := env.db.Where("id = ?", att.ID).First(&att2).Error; err != nil {
		t.Fatal(err)
	}
	if att2.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("a rejected callback must leave the attempt pending, got %s", att2.State)
	}
}

func TestWechatCallbackRejectsUnknownSerialNothingPersisted(t *testing.T) {
	env := newWechatCallbackEnv(t)
	_, att := placeWechatOrder(t, env, 706)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_s", 9900, "SUCCESS")
	// A serial that matches no CONFIGURED platform certificate: even a
	// perfectly-signed body is fatal (client-supplied keys are never trusted).
	h.Set("Wechatpay-Serial", "UNCONFIGURED-SERIAL")
	w := postWechatNotify(env.engine, body, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown serial must be rejected 401, got %d %q", w.Code, w.Body.String())
	}
	if n := outboxCount(t, env.db); n != 0 {
		t.Fatalf("nothing may be persisted, got %d events", n)
	}
	var att2 repocommercial.PaymentAttemptRow
	if err := env.db.Where("id = ?", att.ID).First(&att2).Error; err != nil {
		t.Fatal(err)
	}
	if att2.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("attempt must stay pending, got %s", att2.State)
	}
}

func TestWechatCallbackUnknownAttemptNotFound(t *testing.T) {
	env := newWechatCallbackEnv(t)
	placeWechatOrder(t, env, 707)
	body, h := wechatCBNotify(t, env, "mo_never_registered", "wx_txn_u", 9900, "SUCCESS")
	w := postWechatNotify(env.engine, body, h)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"FAIL"`) {
		t.Fatalf("unregistered out_trade_no must 404 FAIL, got %d %q", w.Code, w.Body.String())
	}
	if n := outboxCount(t, env.db); n != 0 {
		t.Fatalf("nothing may be persisted, got %d events", n)
	}
}

// countAnomalies counts retained abnormal payment facts (#84).
func countAnomalies(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.PaymentAnomalyRow{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// assertOrderPendingNoFulfillEvent（#84 AC1）：异常收款绝不履约——订单仍
// pending、attempt 仍 pending、fulfill 事件为零。
func assertOrderPendingNoFulfillEvent(t *testing.T, db *gorm.DB, orderID string) {
	t.Helper()
	var row repocommercial.OrderRow
	if err := db.Where("id = ?", orderID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != commercial.OrderStatePending {
		t.Fatalf("an abnormal payment must never advance the order, got %s", row.State)
	}
	var nFulfill int64
	if err := db.Model(&repocommercial.OutboxEvent{}).
		Where("kind = ?", repocommercial.OutboxKindFulfill).Count(&nFulfill).Error; err != nil {
		t.Fatal(err)
	}
	if nFulfill != 0 {
		t.Fatalf("abnormal payment must not mint a fulfill event, got %d", nFulfill)
	}
}

// ---- #84 Task 2: terminal idempotent ack for abnormal callbacks ----

// TestWechatCallbackAmountMismatchRecordsAnomalyAndAcksIdempotent（G1 / AC1c
// / AC3 前半）：错金额的已验签通知——事实已独立事务落库（Task 1）后回调面
// 答 200 终态（spec L171：渠道停止重试风暴），重复投递同样 200 且 anomaly
// 行数不变；订单 pending、无 fulfill 事件。
func TestWechatCallbackAmountMismatchRecordsAnomalyAndAcksIdempotent(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 708)
	// A genuine signature over a SUCCESS whose amount (5000 fen, a partial
	// payment) does not match the order face (9900).
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_m", 5000, "SUCCESS")
	w := postWechatNotify(env.engine, body, h)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":"SUCCESS"`) {
		t.Fatalf("a retained mismatch must answer the terminal 200 ack, got %d %q", w.Code, w.Body.String())
	}
	if n := countAnomalies(t, env.db); n != 1 {
		t.Fatalf("the mismatch fact must be retained exactly once, got %d", n)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := env.db.Where("order_id = ?", order.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != repocommercial.PaymentAnomalyKindPartial ||
		anomaly.ExpectedAmountFen != 9900 || anomaly.ActualAmountFen != 5000 ||
		anomaly.Transaction != "wx_txn_m" {
		t.Fatalf("partial-payment snapshot mismatch: %+v", anomaly)
	}
	// Redelivery of the SAME notification: terminal 200 again, still one row.
	w2 := postWechatNotify(env.engine, body, h)
	if w2.Code != http.StatusOK {
		t.Fatalf("redelivery must ack 200 again, got %d %q", w2.Code, w2.Body.String())
	}
	if n := countAnomalies(t, env.db); n != 1 {
		t.Fatalf("redelivery must not mint a second anomaly row, got %d", n)
	}
	assertOrderPendingNoFulfillEvent(t, env.db, order.ID)
	var att2 repocommercial.PaymentAttemptRow
	if err := env.db.Where("id = ?", att.ID).First(&att2).Error; err != nil {
		t.Fatal(err)
	}
	if att2.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("the attempt must stay pending, got %s", att2.State)
	}
}

// TestAlipayCallbackAmountMismatchRecordsAnomalyAndAcksSuccess：支付宝腿同
// 契约——篡改 total_amount 的已验签通知答纯文本 "success"（终态），事实落库，
// 订单 pending、无 fulfill 事件（AC1a 的 Alipay 面）。
func TestAlipayCallbackAmountMismatchRecordsAnomalyAndAcksSuccess(t *testing.T) {
	db, stub, engine, orders := newCallbackTestEnv(t)
	seedCallbackPublishedPlan(t, db)
	ctx := context.Background()
	q, err := orders.CreateQuote(ctx, 603, "pro")
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, 603, q.ID, payment.ProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	fact := notifyFact(t, db, stub, order.ID)
	fact.Amount = 50_00 // order face 9900, notified 5000 → partial payment
	stub.fact = fact
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/commercial/callbacks/alipay", strings.NewReader("notify=1"))
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("a retained mismatch must answer 200 \"success\", got %d %q", w.Code, w.Body.String())
	}
	if n := countAnomalies(t, db); n != 1 {
		t.Fatalf("the alipay mismatch fact must be retained, got %d", n)
	}
	var anomaly repocommercial.PaymentAnomalyRow
	if err := db.Where("order_id = ?", order.ID).First(&anomaly).Error; err != nil {
		t.Fatal(err)
	}
	if anomaly.Kind != repocommercial.PaymentAnomalyKindPartial {
		t.Fatalf("kind=%s, want partial_payment", anomaly.Kind)
	}
	assertOrderPendingNoFulfillEvent(t, db, order.ID)
}

// TestWechatCallbackNonSucceededStatusStaysRejectedNoAnomaly（Review Focus 1
// / 审查 R1）：金额币种身份全对、仅 trade_state=CLOSED 的已验签通知——非
// succeeded 独立分类，维持非 2xx（渠道重试是正确姿态）、零落库、订单
// pending。改造前后均须 PASS：它是防止 ErrPaymentNotSucceeded 被误并入 200
// 终态分支的回归锁。
func TestWechatCallbackNonSucceededStatusStaysRejectedNoAnomaly(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 709)
	body, h := wechatCBNotify(t, env, att.MerchantOrderID, "wx_txn_c", 9900, "CLOSED")
	w := postWechatNotify(env.engine, body, h)
	if w.Code == http.StatusOK {
		t.Fatalf("a non-succeeded fact must NOT answer the terminal 200, got %d %q", w.Code, w.Body.String())
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("a non-succeeded fact keeps the 409 failure face, got %d %q", w.Code, w.Body.String())
	}
	if n := countAnomalies(t, env.db); n != 0 {
		t.Fatalf("a non-succeeded fact is not an abnormal collection: %d anomaly rows", n)
	}
	assertOrderPendingNoFulfillEvent(t, env.db, order.ID)
	var att2 repocommercial.PaymentAttemptRow
	if err := env.db.Where("id = ?", att.ID).First(&att2).Error; err != nil {
		t.Fatal(err)
	}
	if att2.State != repocommercial.PaymentAttemptStatePending {
		t.Fatalf("the attempt must stay pending, got %s", att2.State)
	}
}

// wechatRefundCBNotify builds one SIGNED WeChat REFUND.SUCCESS
// notification (#97) — the WX-02 construction with the refund resource
// face (out_refund_no + refund_status).
func wechatRefundCBNotify(t *testing.T, env *wechatCallbackEnv, outTradeNo, outRefundNo, refundStatus string) ([]byte, http.Header) {
	t.Helper()
	plain, err := json.Marshal(map[string]interface{}{
		"mchid":         "1900000001",
		"appid":         "wx-test-app",
		"out_trade_no":  outTradeNo,
		"out_refund_no": outRefundNo,
		"refund_status": refundStatus,
		"amount":        map[string]interface{}{"refund": 9900, "total": 9900, "currency": "CNY"},
	})
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(env.apiv3)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte("resnonce1234"), plain, []byte("refund"))
	body, err := json.Marshal(map[string]interface{}{
		"id":         "evt-cb-97",
		"event_type": "REFUND.SUCCESS",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": "refund",
			"nonce":           "resnonce1234",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	const hdrNonce = "hdrnonce97"
	message := []byte(ts + "\n" + hdrNonce + "\n" + string(body) + "\n")
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, env.platKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Wechatpay-Serial", "PLAT-SERIAL-83")
	h.Set("Wechatpay-Timestamp", ts)
	h.Set("Wechatpay-Nonce", hdrNonce)
	h.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(sig))
	return body, h
}

// TestWechatRefundCallbackDispatchesToRefundService (#97): a verified
// REFUND.* fact never enters the payment-confirmation path (no attempt
// resolution, no ConfirmPayment) — with the refund service unwired it
// fails closed 503 so the provider retries, and nothing is persisted.
func TestWechatRefundCallbackDispatchesToRefundService(t *testing.T) {
	env := newWechatCallbackEnv(t)
	order, att := placeWechatOrder(t, env, 971)
	body, h := wechatRefundCBNotify(t, env, att.MerchantOrderID, "rf-cb-97", "SUCCESS")

	w := postWechatNotify(env.engine, body, h)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("refund fact with unwired refund service: status %d, want 503", w.Code)
	}
	if n := outboxCount(t, env.db); n != 0 {
		t.Fatalf("refund fact must not emit payment fulfillment events: %d", n)
	}
	var row repocommercial.OrderRow
	if err := env.db.Where("id = ?", order.ID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State == commercial.OrderStatePaid {
		t.Fatal("a refund notification must never confirm the order's payment")
	}
}
