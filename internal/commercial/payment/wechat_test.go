package payment

import (
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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// (R1-V09) 渠道出站必须经过共享 SSRF 校验：环回/私有/保留地址一律拒绝
// （fail-closed ErrNotConfigured），除非运维显式配置 SSRF_WHITELIST(_EXTRA)
// 豁免——flow-evidence-81 的本环回 stub 流程即依赖该可审计豁免。
func TestWechatDoRejectsInternalEgressWithoutWhitelist(t *testing.T) {
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	p := newWechatProvider(WechatConfig{AppID: "wx-test-app", MchID: "1900000001",
		APIBaseURL: "http://127.0.0.1:8291"}, nil, nil)
	err := p.do(context.Background(), http.MethodPost, "/v3/pay/transactions/native", []byte("{}"), nil)
	if !errors.Is(err, ErrNotConfigured) || !strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("loopback egress must fail closed with the SSRF gate, got %v", err)
	}
}

func TestWechatDoAllowsWhitelistedLoopbackStub(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=x"}`))
	}))
	t.Cleanup(srv.Close)
	p := newWechatProvider(WechatConfig{AppID: "wx-test-app", MchID: "1900000001",
		APIBaseURL: srv.URL}, nil, nil)
	err := p.do(context.Background(), http.MethodPost, "/v3/pay/transactions/native", []byte("{}"), nil)
	// 白名单豁免后必须通过 SSRF 门（此后失败只能发生在签名授权环节——测试
	// provider 无商户密钥材料），证明豁免通道可用。
	if err != nil && strings.Contains(err.Error(), "SSRF") {
		t.Fatalf("whitelisted loopback must pass the SSRF gate, got %v", err)
	}
}

func TestWechatQueryRequiresCompleteSucceededCollectionFace(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "merchant.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, payload string
		wantErr       bool
		wantAmount    int64
	}{
		{name: "valid", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"total":100,"currency":"CNY"}}`, wantAmount: 100},
		{name: "missing amount", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"currency":"CNY"}}`},
		{name: "nonpositive amount", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"total":0,"currency":"CNY"}}`},
		{name: "negative wrong-currency amount", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"total":-1,"currency":"USD"}}`},
		{name: "missing currency", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"total":100}}`, wantErr: true},
		{name: "blank currency", payload: `{"trade_state":"SUCCESS","out_trade_no":"order-1","amount":{"total":100,"currency":"  "}}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tc.payload)) }))
			defer srv.Close()
			p := newWechatProvider(WechatConfig{AppID: "wx-test", MchID: "1900000001", MchSerial: "serial", MchKeyPath: keyPath, APIBaseURL: srv.URL}, nil, nil)
			res, err := p.Query(context.Background(), "order-1")
			if tc.wantErr {
				if err == nil || res.State == StateSucceeded {
					t.Fatalf("incomplete successful observation must be rejected: result=%+v err=%v", res, err)
				}
				return
			}
			wantCurrency := "CNY"
			if tc.name == "negative wrong-currency amount" {
				wantCurrency = "USD"
			}
			if err != nil || res.State != StateSucceeded || res.AmountFen != tc.wantAmount || res.AmountCurrency != wantCurrency {
				t.Fatalf("valid collection observation: %+v err=%v", res, err)
			}
		})
	}
}

func TestWechatRejectsChangedCallbackBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("1\nn\n{}\n")
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if VerifyWechatSignature(&key.PublicKey, "1", "n", []byte("{bad}"), base64.StdEncoding.EncodeToString(sig)) == nil {
		t.Fatal("tampered body accepted")
	}
}

// newCallbackFixture builds a provider whose ONLY configured platform
// certificate is the given test key (self-generated RSA is acceptable for
// signature-verification logic only — never as channel acceptance).
func newCallbackFixture(t *testing.T) (*WechatProvider, *rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	apiv3 := []byte("0123456789abcdef0123456789abcdef")
	cfg := WechatConfig{AppID: "wx-test-app", MchID: "1900000001"}
	p := newWechatProvider(cfg, map[string]*rsa.PublicKey{"PLAT-SERIAL-1": {N: key.PublicKey.N, E: key.PublicKey.E}}, apiv3)
	return p, key, apiv3
}

// buildPaymentResource encrypts a payment result exactly the way WeChat
// does (AES-256-GCM per WX-02) so Verify must decrypt it to accept.
func buildPaymentResource(t *testing.T, apiv3 []byte, payment map[string]interface{}, resNonce, aad string) string {
	t.Helper()
	plain, err := json.Marshal(payment)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(apiv3)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte(resNonce), plain, []byte(aad))
	return base64.StdEncoding.EncodeToString(ct)
}

func buildNotification(t *testing.T, apiv3 []byte, mchid, appid, outTradeNo, txnID, tradeState string, total int64) []byte {
	t.Helper()
	ciphertext := buildPaymentResource(t, apiv3, map[string]interface{}{
		"mchid":          mchid,
		"appid":          appid,
		"out_trade_no":   outTradeNo,
		"transaction_id": txnID,
		"trade_state":    tradeState,
		"amount":         map[string]interface{}{"total": total, "currency": "CNY"},
	}, "resnonce1234", "transaction")
	body, err := json.Marshal(map[string]interface{}{
		"id":          "evt-1",
		"event_type":  "TRANSACTION.SUCCESS",
		"create_time": "2026-09-11T10:00:00+08:00",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      ciphertext,
			"associated_data": "transaction",
			"nonce":           "resnonce1234",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func signCallback(t *testing.T, key *rsa.PrivateKey, ts, nonce string, body []byte) string {
	t.Helper()
	message := []byte(ts + "\n" + nonce + "\n" + string(body) + "\n")
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func callbackHeaders(serial, ts, nonce, signature string) http.Header {
	h := http.Header{}
	h.Set("Wechatpay-Serial", serial)
	h.Set("Wechatpay-Timestamp", ts)
	h.Set("Wechatpay-Nonce", nonce)
	h.Set("Wechatpay-Signature", signature)
	return h
}

func TestWechatVerifyAcceptsValidSignature(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	fact, err := p.Verify(context.Background(), h, body)
	if err != nil {
		t.Fatalf("valid notification rejected: %v", err)
	}
	if fact.AttemptID != "out-1" || fact.Transaction != "txn-1" || fact.Merchant != "1900000001" ||
		fact.Provider != ProviderWechat || fact.Amount != commercial.CNYFen(100) ||
		fact.Currency != "CNY" || fact.State != StateSucceeded.String() {
		t.Fatalf("unexpected fact: %+v", fact)
	}
	if fact.TenantID != 0 || fact.OrderID != "" {
		t.Fatalf("callback payload must not carry tenant/order identity: %+v", fact)
	}
}

func TestWechatRejectsWrongKey(t *testing.T) {
	p, _, apiv3 := newCallbackFixture(t)
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, wrongKey, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrSignatureRejected) {
		t.Fatalf("wrong key accepted: %v", err)
	}
}

func TestWechatRejectsUnknownSerial(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("ATTACKER-SERIAL", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrUnknownCertSerial) {
		t.Fatalf("unconfigured serial accepted: %v", err)
	}
}

func TestWechatRejectsTamperedTimestampAndNonce(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	// Signed over one ts/nonce pair, delivered with another: signature must not verify.
	h := callbackHeaders("PLAT-SERIAL-1", ts, "OTHER-NONCE", signCallback(t, key, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrSignatureRejected) {
		t.Fatalf("tampered timestamp/nonce accepted: %v", err)
	}
}

func TestWechatRejectsInvalidBase64Signature(t *testing.T) {
	p, _, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", "!!!not-base64!!!")
	if _, err := p.Verify(context.Background(), h, body); err == nil {
		t.Fatal("invalid base64 signature accepted")
	}
}

func TestWechatRejectsStaleTimestamp(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", stale, "hdrnonce", signCallback(t, key, stale, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrStaleTimestamp) {
		t.Fatalf("stale timestamp accepted: %v", err)
	}
}

func TestWechatRejectsMerchantMismatch(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900009999", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrMerchantMismatch) {
		t.Fatalf("foreign merchant accepted: %v", err)
	}
}

func TestWechatDuplicateDeliveryIsIdempotent(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	first, err := p.Verify(context.Background(), h, body)
	if err != nil {
		t.Fatalf("first delivery rejected: %v", err)
	}
	// AC-03: a redelivery of the SAME verified notification must pass
	// verification again and yield the SAME fact, so ConfirmPayment can
	// replay the original success (exactly-once fulfillment) and answer
	// the idempotent success ACK — not a 401 that keeps the provider
	// retrying a payment it already delivered.
	second, err := p.Verify(context.Background(), h, body)
	if err != nil {
		t.Fatalf("duplicate delivery rejected: %v", err)
	}
	if first.Provider != second.Provider || first.AttemptID != second.AttemptID ||
		first.Transaction != second.Transaction || first.Amount != second.Amount ||
		first.State != second.State {
		t.Fatalf("duplicate delivery produced a different fact: %+v vs %+v", first, second)
	}
}

// TestChannelClientsCapRedirectLoop（R1-14）：两个主机互发 302 即成无限重定向
// 环——旧客户端的自定义 CheckRedirect 整体替换了标准库默认的 10 跳上限（该
// 默认仅 CheckRedirect 为 nil 时生效），每次支付/查单会持续跳转直到 Timeout
// 耗尽。共享 SSRFSafe 客户端在 MaxRedirects 上限处终止；环回桩经
// SSRF_WHITELIST 显式豁免（与真实出站策略同一豁免通道）。
func TestChannelClientsCapRedirectLoop(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)

	var hops int64
	var srvA, srvB *httptest.Server
	srvA = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hops, 1)
		http.Redirect(w, r, srvB.URL+"/hop", http.StatusFound)
	}))
	srvB = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hops, 1)
		http.Redirect(w, r, srvA.URL+"/hop", http.StatusFound)
	}))
	t.Cleanup(srvA.Close)
	t.Cleanup(srvB.Close)

	clients := map[string]*http.Client{}
	wechat := newWechatProvider(WechatConfig{AppID: "wx-test", MchID: "1900000001",
		APIBaseURL: srvA.URL}, nil, nil)
	clients["wechat"] = wechat.client
	if p, _, _ := alipayNotifyFixture(t); p != nil {
		clients["alipay"] = p.client
	}

	for name, client := range clients {
		if client == nil || client.CheckRedirect == nil {
			t.Fatalf("%s client must carry the shared SSRF-safe redirect policy", name)
		}
		atomic.StoreInt64(&hops, 0)
		req, err := http.NewRequest(http.MethodGet, srvA.URL+"/start", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Do(req)
		if err == nil {
			t.Fatalf("%s: redirect loop must fail, not return a response", name)
		}
		if !strings.Contains(err.Error(), "stopped after 10 redirects") {
			t.Fatalf("%s: loop must stop at the redirect cap, got %v", name, err)
		}
		if n := atomic.LoadInt64(&hops); n > 12 {
			t.Fatalf("%s: loop must stop at the cap (hops=%d), not run to the client timeout", name, n)
		}
	}
}

// TestChannelTransportKeepsEnvironmentProxy（R2-17/R2-19）：切换到共享
// SSRFSafe 客户端时静默丢了 ProxyFromEnvironment（NewSSRFSafeTransport 不设
// Proxy 字段，而默认 Transport 带环境代理）——仅允许代理出网的部署会从「经
// 代理可达」退化为「直连超时」。两个渠道的 transport 现在都显式挂回环境
// 代理（SSRF 拨号/校验层保持不变）。
func TestChannelTransportKeepsEnvironmentProxy(t *testing.T) {
	assertProxy := func(name string, client *http.Client) {
		t.Helper()
		if client == nil || client.Transport == nil {
			t.Fatalf("%s: client/transport missing", name)
		}
		rt, ok := client.Transport.(*secutils.SSRFValidatingRoundTripper)
		if !ok {
			t.Fatalf("%s: transport must be the SSRF-validating wrapper, got %T", name, client.Transport)
		}
		tr, ok := rt.Base.(*http.Transport)
		if !ok {
			t.Fatalf("%s: base transport must be *http.Transport, got %T", name, rt.Base)
		}
		if tr.Proxy == nil {
			t.Fatalf("%s: base transport must carry the environment proxy (R2-17/R2-19 regression)", name)
		}
		if tr.DialContext == nil {
			t.Fatalf("%s: the SSRF-safe dial layer must stay mounted", name)
		}
	}
	wechat := newWechatProvider(WechatConfig{AppID: "wx-test", MchID: "1900000001"}, nil, nil)
	assertProxy("wechat", wechat.client)
	alipay, _, _ := alipayNotifyFixture(t)
	assertProxy("alipay", alipay.client)
}

// --- #97 (Lago 25): wechat refund parity with the alipay commercial
// semantics — stable out_refund_no key, status mapping onto the shared
// unknown-stays-locked vocabulary, and the REFUND.* notification face. ---

// newRefundStubProvider wires a provider against a stub gateway that
// captures the request line and body (the TestWechatQueryRequires… stub
// pattern) with the SSRF exemption a loopback stub needs.
func newRefundStubProvider(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) (*WechatProvider, *capturedRequest) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.body = body
		respond(w, r)
	}))
	t.Cleanup(srv.Close)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "merchant.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	return newWechatProvider(WechatConfig{AppID: "wx-test", MchID: "1900000001", MchSerial: "serial", MchKeyPath: keyPath, APIBaseURL: srv.URL}, nil, nil), captured
}

type capturedRequest struct {
	method, path string
	body         []byte
}

// TestWechatRefundUsesStableRefundKey (#97 AC1/AC3): the refund is filed
// under the refund's OWN out_refund_no against the original out_trade_no,
// and the reconciliation query addresses the ORIGINAL refund key — retries
// and re-queries therefore never re-key, which is the duplicate-payout
// guard on the request side (alipay out_request_no parity).
func TestWechatRefundUsesStableRefundKey(t *testing.T) {
	p, captured := newRefundStubProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"out_refund_no":"rf-1","status":"PROCESSING"}`))
	})
	res, err := p.Refund(context.Background(), RefundRequest{RefundID: "rf-1", ProviderID: "order-1", AmountFen: 100})
	if err != nil || res.State != StatePending || res.ProviderID != "rf-1" {
		t.Fatalf("refund result: %+v err=%v", res, err)
	}
	var sent struct {
		OutRefundNo string `json:"out_refund_no"`
		OutTradeNo  string `json:"out_trade_no"`
		Amount      struct {
			Refund   int64  `json:"refund"`
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(captured.body, &sent); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost || captured.path != "/v3/refund/domestic/refunds" {
		t.Fatalf("refund request line: %s %s", captured.method, captured.path)
	}
	if sent.OutRefundNo != "rf-1" || sent.OutTradeNo != "order-1" ||
		sent.Amount.Refund != 100 || sent.Amount.Total != 100 || sent.Amount.Currency != "CNY" {
		t.Fatalf("refund request body: %+v", sent)
	}
	qres, err := p.QueryRefund(context.Background(), "rf-1")
	if err != nil || qres.ProviderID != "rf-1" || qres.State != StatePending {
		t.Fatalf("refund query result: %+v err=%v", qres, err)
	}
	if captured.method != http.MethodGet || captured.path != "/v3/refund/domestic/refunds/rf-1" {
		t.Fatalf("refund query must address the ORIGINAL refund key: %s %s", captured.method, captured.path)
	}
}

// TestWechatRefundMapsStatus (#97 AC1): wechat refund statuses map onto the
// same commercial vocabulary alipay uses; anything unproven (ABNORMAL,
// unknown transport outcome) must land in the state mapChannelState folds
// into RefundChannelUnknown —受理≠成功, unknown stays locked.
func TestWechatRefundMapsStatus(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   AttemptState
	}{
		{"SUCCESS", StateSucceeded},
		{"PROCESSING", StatePending},
		{"ACCEPT", StatePending},
		{"ABNORMAL", AttemptState("abnormal")},
		{"CLOSED", StateClosed},
		{"", ""},
	} {
		if got := mapWechatRefundState(tc.status); got != tc.want {
			t.Fatalf("mapWechatRefundState(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	key, kerr := rsa.GenerateKey(rand.Reader, 2048)
	if kerr != nil {
		t.Fatal(kerr)
	}
	keyPath := filepath.Join(t.TempDir(), "merchant.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		t.Fatal(err)
	}
	p := newWechatProvider(WechatConfig{AppID: "wx-test", MchID: "1900000001", MchSerial: "serial", MchKeyPath: keyPath, APIBaseURL: srv.URL, Timeout: 50 * time.Millisecond}, nil, nil)
	res, err := p.QueryRefund(context.Background(), "rf-1")
	if err == nil {
		t.Fatal("transport failure must surface an error")
	}
	if res.State != StateUnknown {
		t.Fatalf("timeout outcome must stay unknown (locked), got %q", res.State)
	}
}

// buildRefundNotification encrypts a REFUND.* resource exactly per WX-02.
func buildRefundNotification(t *testing.T, apiv3 []byte, mchid, appid, outTradeNo, outRefundNo, refundStatus string) []byte {
	t.Helper()
	ciphertext := buildPaymentResource(t, apiv3, map[string]interface{}{
		"mchid":         mchid,
		"appid":         appid,
		"out_trade_no":  outTradeNo,
		"out_refund_no": outRefundNo,
		"refund_status": refundStatus,
		"amount":        map[string]interface{}{"refund": 100, "total": 100, "currency": "CNY"},
	}, "resnonce1234", "refund")
	body, err := json.Marshal(map[string]interface{}{
		"id":         "evt-r1",
		"event_type": "REFUND.SUCCESS",
		"resource": map[string]interface{}{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      ciphertext,
			"associated_data": "refund",
			"nonce":           "resnonce1234",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// TestWechatVerifyAcceptsRefundNotification (#97 AC3): a correctly signed
// REFUND.* notification verifies through the SAME WX-02 chain and reports
// the refund fact keyed by out_refund_no — the notification is only a
// re-read trigger, never a second payout instruction.
func TestWechatVerifyAcceptsRefundNotification(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildRefundNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "rf-1", "SUCCESS")
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	fact, err := p.Verify(context.Background(), h, body)
	if err != nil {
		t.Fatalf("refund notification rejected: %v", err)
	}
	if fact.RefundID != "rf-1" || fact.State != StateSucceeded.String() || fact.AttemptID != "out-1" {
		t.Fatalf("refund fact: %+v", fact)
	}
}

// TestWechatVerifyRejectsUnknownEventFamily (#97): neither payment nor
// refund — still malformed, so the provider keeps retrying.
func TestWechatVerifyRejectsUnknownEventFamily(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	ciphertext := buildPaymentResource(t, apiv3, map[string]interface{}{"x": 1}, "resnonce1234", "other")
	body, err := json.Marshal(map[string]interface{}{
		"id":         "evt-o1",
		"event_type": "COUPON.SENT",
		"resource": map[string]interface{}{"algorithm": "AEAD_AES_256_GCM", "ciphertext": ciphertext, "associated_data": "other", "nonce": "resnonce1234"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrMalformedCallback) {
		t.Fatalf("unknown event family must stay malformed, got %v", err)
	}
}
