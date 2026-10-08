package payment

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// captureWechat records one outbound request the provider made against the
// stub gateway (method, path, body, Authorization header).
type captureWechat struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

// nativeFixture wires a WechatProvider against a local stub gateway: a
// throwaway merchant RSA key (PKCS1 PEM in TempDir — never committed), the
// auditable SSRF loopback exemption, and a request capture wrapper. The
// returned slice pointer is guarded by a mutex because the httptest handler
// runs on its own goroutine; read it via captured().
type nativeFixture struct {
	provider *WechatProvider
	calls    []captureWechat
	mu       sync.Mutex
}

func (f *nativeFixture) record(c captureWechat) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *nativeFixture) captured() []captureWechat {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]captureWechat, len(f.calls))
	copy(out, f.calls)
	return out
}

// newNativeFixture builds the fixture with the default native config.
func newNativeFixture(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *nativeFixture {
	t.Helper()
	return newNativeFixtureCfg(t, handle, nil)
}

// newNativeFixtureCfg additionally lets a test mutate the WechatConfig
// (e.g. Timeout for the indeterminate-outcome test).
func newNativeFixtureCfg(t *testing.T, handle func(w http.ResponseWriter, r *http.Request), mutate func(*WechatConfig)) *nativeFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(filepath.Join(dir, "mch_private.pem"), pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)

	f := &nativeFixture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.record(captureWechat{Method: r.Method, Path: r.URL.Path, Body: string(body), Auth: r.Header.Get("Authorization")})
		handle(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := WechatConfig{
		AppID:      "wx-test-app",
		MchID:      "1900000001",
		MchSerial:  "MCH-SERIAL-1",
		MchKeyPath: filepath.Join(dir, "mch_private.pem"),
		APIBaseURL: srv.URL,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	f.provider = newWechatProvider(cfg, nil, nil)
	return f
}

func respondJSON(t *testing.T, w http.ResponseWriter, status int, payload string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, payload)
}

func TestWechatCreatePostsNativeOrderAndMapsCodeURL(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, http.StatusOK, `{"code_url":"weixin://wxpay/bizpayurl?pr=n83"}`)
	})
	res, err := f.provider.Create(context.Background(), OrderRequest{
		OrderID: "ord_83", MerchantOrderID: "mo_83", AmountFen: 9900, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if res.State != StatePending {
		t.Fatalf("native create must be pending, got %q", res.State)
	}
	if res.ProviderID != "mo_83" {
		t.Fatalf("provider id must key the original out_trade_no, got %q", res.ProviderID)
	}
	if res.CheckoutURL != "weixin://wxpay/bizpayurl?pr=n83" {
		t.Fatalf("code_url must map to CheckoutURL, got %q", res.CheckoutURL)
	}
	calls := f.captured()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one outbound call, got %d", len(calls))
	}
	c := calls[0]
	if c.Method != http.MethodPost {
		t.Fatalf("create must POST, got %s", c.Method)
	}
	if c.Path != "/v3/pay/transactions/native" {
		t.Fatalf("create path mismatch: %q", c.Path)
	}
	if !strings.HasPrefix(c.Auth, "WECHATPAY2-SHA256-RSA2048") {
		t.Fatalf("authorization must use the APIv3 scheme, got %q", c.Auth)
	}
	if !strings.Contains(c.Auth, `mchid="1900000001"`) {
		t.Fatalf("authorization must carry the merchant id: %q", c.Auth)
	}
	if !strings.Contains(c.Auth, `serial_str="MCH-SERIAL-1"`) {
		t.Fatalf("authorization must carry the merchant cert serial: %q", c.Auth)
	}
	var body struct {
		AppID       string `json:"appid"`
		MchID       string `json:"mchid"`
		Description string `json:"description"`
		OutTradeNo  string `json:"out_trade_no"`
		Amount      struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.Unmarshal([]byte(c.Body), &body); err != nil {
		t.Fatalf("create body is not JSON: %v (%s)", err, c.Body)
	}
	if body.AppID != "wx-test-app" || body.MchID != "1900000001" || body.OutTradeNo != "mo_83" {
		t.Fatalf("create body identity mismatch: %+v", body)
	}
	if body.Amount.Total != 9900 || body.Amount.Currency != "CNY" {
		t.Fatalf("create body amount mismatch: %+v", body.Amount)
	}
	if !strings.Contains(body.Description, "ord_83") {
		t.Fatalf("description must reference the local order, got %q", body.Description)
	}
}

func TestWechatCreateRejectsInvalidRequestLocally(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("stub must not be reached for locally invalid requests")
	})
	cases := []OrderRequest{
		{OrderID: "o1", MerchantOrderID: "", AmountFen: 9900, Currency: "CNY"},
		{OrderID: "o1", MerchantOrderID: "mo_1", AmountFen: 0, Currency: "CNY"},
	}
	for i, req := range cases {
		if _, err := f.provider.Create(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("case %d: want ErrInvalidRequest, got %v", i, err)
		}
	}
	if n := len(f.captured()); n != 0 {
		t.Fatalf("locally invalid requests must not hit the channel, got %d calls", n)
	}
}

// TestWechatCreateTimeoutReturnsUnknownWithOriginalKey pins spec L165 for
// the create leg: a transport timeout is an INDETERMINATE outcome — the
// result must come back StateUnknown keyed by the ORIGINAL
// merchant_order_id so recovery queries that identifier and never re-keys.
func TestWechatCreateTimeoutReturnsUnknownWithOriginalKey(t *testing.T) {
	f := newNativeFixtureCfg(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}, func(cfg *WechatConfig) { cfg.Timeout = 200 * time.Millisecond })
	res, err := f.provider.Create(context.Background(), OrderRequest{
		OrderID: "ord_t", MerchantOrderID: "mo_t", AmountFen: 9900, Currency: "CNY",
	})
	if err == nil {
		t.Fatal("timeout must surface an error")
	}
	if res.State != StateUnknown {
		t.Fatalf("timeout must map to StateUnknown, got %q", res.State)
	}
	if res.ProviderID != "mo_t" {
		t.Fatalf("indeterminate outcome must key the ORIGINAL merchant order id, got %q", res.ProviderID)
	}
}

func TestWechatQueryMapsTradeStates(t *testing.T) {
	cases := []struct {
		tradeState string
		want       AttemptState
	}{
		{"SUCCESS", StateSucceeded},
		{"NOTPAY", StatePending},
		{"CLOSED", StateClosed},
		// REFUND means the transaction succeeded once; the fact still
		// reports success (refunds live on the refund lifecycle).
		{"REFUND", StateSucceeded},
	}
	for _, tc := range cases {
		t.Run(tc.tradeState, func(t *testing.T) {
			f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				respondJSON(t, w, http.StatusOK, `{"out_trade_no":"mo_q","trade_state":"`+tc.tradeState+`","amount":{"total":9900,"currency":"CNY"}}`)
			})
			res, err := f.provider.Query(context.Background(), "mo_q")
			if err != nil {
				t.Fatalf("query failed: %v", err)
			}
			if res.State != tc.want {
				t.Fatalf("trade_state %s must map to %s, got %s", tc.tradeState, tc.want, res.State)
			}
			if res.ProviderID != "mo_q" {
				t.Fatalf("query must key the out_trade_no, got %q", res.ProviderID)
			}
			calls := f.captured()
			if len(calls) != 1 || calls[0].Method != http.MethodGet {
				t.Fatalf("query must be one GET, got %+v", calls)
			}
			if calls[0].Path != "/v3/pay/transactions/out-trade-no/mo_q" {
				t.Fatalf("query path mismatch: %q", calls[0].Path)
			}
		})
	}
}

func TestWechatQueryFallsBackToRequestedIDWhenResponseOmitsOutTradeNo(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, http.StatusOK, `{"trade_state":"NOTPAY","amount":{"total":9900,"currency":"CNY"}}`)
	})
	res, err := f.provider.Query(context.Background(), "mo_fallback")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if res.ProviderID != "mo_fallback" {
		t.Fatalf("missing out_trade_no must fall back to the requested id, got %q", res.ProviderID)
	}
}

// TestWechatQueryPrefersTransactionID pins the reconciliation identity
// (issue #83 flow defect): the channel's query answer carries BOTH
// out_trade_no and transaction_id, and the RESULT must key the channel
// transaction id — ConfirmPayment's exactly-once guard compares the
// verified callback fact's transaction id against what the recovery query
// recorded, so a query that recorded out_trade_no instead would misread the
// SAME payment as a different transaction on the callback's arrival (an
// over-payment audit for one payment, or a unique-constraint failure).
func TestWechatQueryPrefersTransactionID(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, http.StatusOK,
			`{"out_trade_no":"mo_q","transaction_id":"4200001234202609271234567890","trade_state":"SUCCESS","amount":{"total":9900,"currency":"CNY"}}`)
	})
	res, err := f.provider.Query(context.Background(), "mo_q")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if res.ProviderID != "4200001234202609271234567890" {
		t.Fatalf("query must key the channel transaction_id, got %q", res.ProviderID)
	}
}

func TestWechatQueryEmptyProviderIDRejected(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("stub must not be reached for an empty provider id")
	})
	if _, err := f.provider.Query(context.Background(), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
	if n := len(f.captured()); n != 0 {
		t.Fatalf("empty id must not hit the channel, got %d calls", n)
	}
}

func TestWechatClosePostsMchIDBody(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if err := f.provider.Close(context.Background(), "mo_c"); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	calls := f.captured()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one outbound call, got %d", len(calls))
	}
	c := calls[0]
	if c.Method != http.MethodPost {
		t.Fatalf("close must POST, got %s", c.Method)
	}
	if c.Path != "/v3/pay/transactions/out-trade-no/mo_c/close" {
		t.Fatalf("close path mismatch: %q", c.Path)
	}
	var body struct {
		MchID string `json:"mchid"`
	}
	if err := json.Unmarshal([]byte(c.Body), &body); err != nil {
		t.Fatalf("close body is not JSON: %v (%s)", err, c.Body)
	}
	if body.MchID != "1900000001" {
		t.Fatalf("close body must carry the merchant id, got %q", body.MchID)
	}
}

// TestWechatClosePropagatesOrderPaidCode pins the ORDER_PAID error shape
// (`wechat api status 400 code=ORDER_PAID`): the close-race orchestration
// (task 3) keys its query-for-decision branch on exactly this channel code,
// and the task 4 stub speaks the same contract.
func TestWechatClosePropagatesOrderPaidCode(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, http.StatusBadRequest, `{"code":"ORDER_PAID","message":"订单已支付，禁止关单"}`)
	})
	err := f.provider.Close(context.Background(), "mo_race")
	if err == nil {
		t.Fatal("ORDER_PAID close must fail")
	}
	if !strings.Contains(err.Error(), "ORDER_PAID") {
		t.Fatalf("error must carry the channel ORDER_PAID code, got %v", err)
	}
}

// TestWechatQueryReportsCollectedAmount（#84 / G2）：Query 必须回传渠道实收额
// （amount.total）——恢复/关单路径据此把「渠道实收 ≠ 开单面额」的付款分流到
// anomaly 处置而不是按 attempt 金额盲目确认。
func TestWechatQueryReportsCollectedAmount(t *testing.T) {
	f := newNativeFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(t, w, http.StatusOK, `{"out_trade_no":"mo_amt","transaction_id":"txn_amt","trade_state":"SUCCESS","amount":{"total":5000,"currency":"CNY"}}`)
	})
	res, err := f.provider.Query(context.Background(), "mo_amt")
	if err != nil {
		t.Fatal(err)
	}
	if res.AmountFen != 5000 {
		t.Fatalf("query must report the collected amount.total, got %d", res.AmountFen)
	}
	if res.State != StateSucceeded || res.ProviderID != "txn_amt" {
		t.Fatalf("state/provider unchanged by the additive field: %+v", res)
	}
}
