package payment

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
)

func TestAlipayAmountIsExactFen(t *testing.T) {
	got, err := ParseCNYAmount("10.01")
	if err != nil || got != 1001 {
		t.Fatalf("%d %v", got, err)
	}
	if _, err := ParseCNYAmount("10.001"); err == nil {
		t.Fatal("fractional fen accepted")
	}
}

// Production channel amounts are restricted to ^[0-9]+(\.[0-9]{1,2})?$ BEFORE
// any exact arithmetic: trailing dots, bare fractions, exponent and fraction
// expressions, negatives, and int64-overflowing values are all rejected.
func TestAlipayAmountRejectsNonProductionFormats(t *testing.T) {
	bad := []string{"10.", ".5", "1e2", "-1", "-0.01", "1/2", "", " 10", "10.001", "0x10", "123456789012345678901.00"}
	for _, in := range bad {
		if got, err := ParseCNYAmount(in); err == nil {
			t.Fatalf("ParseCNYAmount(%q) = %d, want error", in, got)
		}
	}
	if got, err := ParseCNYAmount("10"); err != nil || got != 1000 {
		t.Fatalf("ParseCNYAmount(\"10\") = %d, %v; want 1000, nil", got, err)
	}
	if got, err := ParseCNYAmount("0.5"); err != nil || got != 50 {
		t.Fatalf("ParseCNYAmount(\"0.5\") = %d, %v; want 50, nil", got, err)
	}
}

// alipayNotifyFixture builds a provider whose ONLY configured Alipay public
// key is the given test key file (self-generated RSA is acceptable for
// signature-verification logic only — never as real-channel acceptance).
func alipayNotifyFixture(t *testing.T) (*AlipayProvider, *rsa.PrivateKey, AlipayConfig) {
	t.Helper()
	alipayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&alipayKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(t.TempDir(), "alipay_pub.pem")
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := AlipayConfig{
		AppID:               "2021000000000001",
		SellerID:            "2088000000000001",
		AlipayPublicKeyPath: pubPath,
	}
	p, err := NewAlipayProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p, alipayKey, cfg
}

// alipaySignNotify signs an async-notify form exactly per the ALI-02
// contract: sign (and sign_type) are excluded from the signed content, the
// remaining keys are sorted and joined as k=v with &, RSA2 = RSA-SHA256
// (PKCS#1 v1.5), base64 encoded.
func alipaySignNotify(t *testing.T, key *rsa.PrivateKey, values url.Values) []byte {
	t.Helper()
	content := alipaySignContent(values)
	digest := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	values.Set("sign_type", "RSA2")
	values.Set("sign", base64.StdEncoding.EncodeToString(sig))
	return []byte(values.Encode())
}

func alipayNotifyValues(appID, sellerID, outTradeNo, tradeNo, tradeStatus, totalAmount string) url.Values {
	v := url.Values{}
	v.Set("app_id", appID)
	v.Set("seller_id", sellerID)
	v.Set("out_trade_no", outTradeNo)
	v.Set("trade_no", tradeNo)
	v.Set("trade_status", tradeStatus)
	v.Set("total_amount", totalAmount)
	v.Set("notify_id", "notify-1")
	v.Set("notify_type", "trade_status_sync")
	return v
}

func TestAlipayVerifyRejectsTamperedNotifyBody(t *testing.T) {
	p, key, _ := alipayNotifyFixture(t)
	body := string(alipaySignNotify(t, key, alipayNotifyValues("2021000000000001", "2088000000000001", "out-1", "trade-1", "TRADE_SUCCESS", "10.01")))
	// Tamper AFTER signing: the signature no longer covers the delivered body.
	tampered := strings.Replace(body, "total_amount=10.01", "total_amount=0.01", 1)
	if tampered == body {
		t.Fatal("fixture failed to embed the tampered field")
	}
	if _, err := p.Verify(context.Background(), http.Header{}, []byte(tampered)); !errors.Is(err, ErrAlipaySignatureRejected) {
		t.Fatalf("tampered body accepted: %v", err)
	}
}

func TestAlipayVerifyRejectsWrongKey(t *testing.T) {
	p, _, _ := alipayNotifyFixture(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body := alipaySignNotify(t, other, alipayNotifyValues("2021000000000001", "2088000000000001", "out-1", "trade-1", "TRADE_SUCCESS", "10.01"))
	if _, err := p.Verify(context.Background(), http.Header{}, body); !errors.Is(err, ErrAlipaySignatureRejected) {
		t.Fatalf("foreign key accepted: %v", err)
	}
}

func TestAlipayVerifyRejectsWrongAppID(t *testing.T) {
	p, key, _ := alipayNotifyFixture(t)
	body := alipaySignNotify(t, key, alipayNotifyValues("2099888877776666", "2088000000000001", "out-1", "trade-1", "TRADE_SUCCESS", "10.01"))
	if _, err := p.Verify(context.Background(), http.Header{}, body); !errors.Is(err, ErrAlipayAppMismatch) {
		t.Fatalf("foreign app_id accepted: %v", err)
	}
}

func TestAlipayVerifyRejectsWrongSeller(t *testing.T) {
	p, key, _ := alipayNotifyFixture(t)
	body := alipaySignNotify(t, key, alipayNotifyValues("2021000000000001", "2088999900001111", "out-1", "trade-1", "TRADE_SUCCESS", "10.01"))
	if _, err := p.Verify(context.Background(), http.Header{}, body); !errors.Is(err, ErrAlipaySellerMismatch) {
		t.Fatalf("foreign seller accepted: %v", err)
	}
}

func TestAlipayVerifyMapsTradeStatuses(t *testing.T) {
	p, key, _ := alipayNotifyFixture(t)
	cases := map[string]string{
		"TRADE_SUCCESS":  StateSucceeded,
		"TRADE_FINISHED": StateSucceeded,
		"WAIT_BUYER_PAY": StatePending,
		"TRADE_CLOSED":   StateClosed,
	}
	for status, want := range cases {
		body := alipaySignNotify(t, key, alipayNotifyValues("2021000000000001", "2088000000000001", "out-1", "trade-1", status, "10.01"))
		fact, err := p.Verify(context.Background(), http.Header{}, body)
		if err != nil {
			t.Fatalf("status %s rejected: %v", status, err)
		}
		if fact.State != want {
			t.Fatalf("status %s mapped to %q, want %q", status, fact.State, want)
		}
		if fact.Provider != ProviderAlipay || fact.Merchant != "2088000000000001" ||
			fact.AttemptID != "out-1" || fact.Transaction != "trade-1" ||
			fact.Amount != commercial.CNYFen(1001) || fact.Currency != "CNY" {
			t.Fatalf("status %s unexpected fact: %+v", status, fact)
		}
		if fact.TenantID != 0 || fact.OrderID != "" {
			t.Fatalf("notify payload must not carry tenant/order identity: %+v", fact)
		}
	}
}

// A re-delivered signed notification is NOT rejected at the Verify gate:
// Alipay retries the same notify when a previous ack was lost, and dedupe is
// owned by C01 ConfirmPayment (idempotent same-transaction replay).
func TestAlipayDuplicateNotifyIsIdempotent(t *testing.T) {
	p, key, _ := alipayNotifyFixture(t)
	body := alipaySignNotify(t, key, alipayNotifyValues("2021000000000001", "2088000000000001", "out-1", "trade-1", "TRADE_SUCCESS", "10.01"))
	first, err := p.Verify(context.Background(), http.Header{}, body)
	if err != nil {
		t.Fatalf("first delivery rejected: %v", err)
	}
	second, err := p.Verify(context.Background(), http.Header{}, body)
	if err != nil {
		t.Fatalf("re-delivery rejected (Alipay would retry forever): %v", err)
	}
	if first != second {
		t.Fatalf("re-delivery produced a different fact: %+v vs %+v", first, second)
	}
}

func TestAlipaySyncReturnNeverConfirms(t *testing.T) {
	p, _, _ := alipayNotifyFixture(t)
	fact, err := p.VerifySyncReturn(context.Background(), http.Header{}, []byte("out_trade_no=out-1&trade_no=trade-1"))
	if err == nil || fact != (commercial.PaymentFact{}) {
		t.Fatalf("synchronous return must never confirm: fact=%+v err=%v", fact, err)
	}
}

// alipayGatewayFixture stands up an offline gateway that answers signed
// envelopes with the test "Alipay" key and records the last request form.
func alipayGatewayFixture(t *testing.T, respond func(r *http.Request) (string, error)) (*AlipayProvider, *url.Values, *httptest.Server) {
	t.Helper()
	_, alipayKey, cfg := alipayNotifyFixture(t)
	lastForm := &url.Values{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		*lastForm = r.PostForm
		innerJSON, err := respond(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		digest := sha256.Sum256([]byte(innerJSON))
		sig, err := rsa.SignPKCS1v15(rand.Reader, alipayKey, crypto.SHA256, digest[:])
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"alipay_mock_response": json.RawMessage(innerJSON),
			"sign":                 base64.StdEncoding.EncodeToString(sig),
			"sign_type":            "RSA2",
		})
	}))
	t.Cleanup(srv.Close)
	merchantKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privPath := filepath.Join(t.TempDir(), "merchant.pem")
	if err := os.WriteFile(privPath, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(merchantKey),
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.GatewayURL = srv.URL
	cfg.MerchantPrivKeyPath = privPath
	cfg.Timeout = 2 * time.Second
	gateway, err := NewAlipayProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return gateway, lastForm, srv
}

func TestAlipayQueryReconcilesMissedNotification(t *testing.T) {
	p, lastForm, _ := alipayGatewayFixture(t, func(r *http.Request) (string, error) {
		return "{\"code\":\"10000\",\"msg\":\"Success\",\"out_trade_no\":\"out-9\",\"trade_no\":\"trade-9\",\"trade_status\":\"TRADE_SUCCESS\",\"total_amount\":\"10.01\",\"seller_id\":\"2088000000000001\"}", nil
	})
	res, err := p.Query(context.Background(), "out-9")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateSucceeded || res.ProviderID != "out-9" {
		t.Fatalf("query result: %+v", res)
	}
	if lastForm.Get("method") != "alipay.trade.query" {
		t.Fatalf("query used method %q", lastForm.Get("method"))
	}
	if lastForm.Get("biz_content") == "" || lastForm.Get("sign") == "" {
		t.Fatalf("query request not signed: %v", lastForm)
	}
}

func TestAlipayQueryTimeoutReturnsUnknownForOriginalIdentifier(t *testing.T) {
	release := make(chan struct{})
	p, _, _ := alipayGatewayFixture(t, func(r *http.Request) (string, error) {
		<-release
		return "{\"code\":\"10000\"}", nil
	})
	defer func() { close(release) }()
	p.cfg.Timeout = 50 * time.Millisecond
	res, err := p.Query(context.Background(), "out-timeout")
	if err == nil {
		t.Fatal("timeout must surface an error")
	}
	if res.State != StateUnknown || res.ProviderID != "out-timeout" {
		t.Fatalf("timeout must key the ORIGINAL identifier with unknown state: %+v", res)
	}
}

func TestAlipayRefundUsesAlipayIdentifiers(t *testing.T) {
	p, lastForm, _ := alipayGatewayFixture(t, func(r *http.Request) (string, error) {
		return "{\"code\":\"10000\",\"msg\":\"Success\",\"trade_no\":\"trade-1\",\"out_trade_no\":\"out-1\",\"fund_change\":\"Y\",\"refund_fee\":\"0.01\"}", nil
	})
	res, err := p.Refund(context.Background(), RefundRequest{RefundID: "rf-1", ProviderID: "out-1", AmountFen: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateSucceeded || res.ProviderID != "rf-1" {
		t.Fatalf("refund result: %+v", res)
	}
	if lastForm.Get("method") != "alipay.trade.refund" {
		t.Fatalf("refund used method %q", lastForm.Get("method"))
	}
	biz := lastForm.Get("biz_content")
	if biz == "" {
		t.Fatal("refund biz_content missing")
	}
	for _, want := range []string{"\"out_request_no\":\"rf-1\"", "\"out_trade_no\":\"out-1\"", "\"refund_amount\":\"0.01\""} {
		if !strings.Contains(biz, want) {
			t.Fatalf("refund biz_content %q missing %q", biz, want)
		}
	}
	// Alipay identifiers only: no WeChat field semantics may leak in.
	for _, banned := range []string{"out_refund_no", "transaction_id", "mchid"} {
		if strings.Contains(biz, banned) {
			t.Fatalf("refund biz_content reuses WeChat field %q", banned)
		}
	}
}

func TestAlipayQueryRefundMapsRefundStatus(t *testing.T) {
	p, lastForm, _ := alipayGatewayFixture(t, func(r *http.Request) (string, error) {
		return "{\"code\":\"10000\",\"msg\":\"Success\",\"out_request_no\":\"rf-1\",\"out_trade_no\":\"out-1\",\"refund_status\":\"REFUND_SUCCESS\",\"total_amount\":\"0.01\"}", nil
	})
	res, err := p.QueryRefund(context.Background(), "rf-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != StateSucceeded || res.ProviderID != "rf-1" {
		t.Fatalf("refund query result: %+v", res)
	}
	if lastForm.Get("method") != "alipay.trade.fastpay.refund.query" {
		t.Fatalf("refund query used method %q", lastForm.Get("method"))
	}
}

func TestNewAlipayProviderRequiresKeyReference(t *testing.T) {
	if _, err := NewAlipayProvider(AlipayConfig{AppID: "a", SellerID: "s"}); err == nil {
		t.Fatal("missing public key reference accepted")
	}
}
