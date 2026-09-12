package payment

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Alipay sandbox integration evidence (ALI-01/03/04/05, interface-verification
// spec section 7). Gated on the official sandbox credentials via environment
// variables; without them the tests SKIP — a skip is never a pass and the
// checks stay pending/blocked-env in the acceptance report.
//
// Required env:
//
//	ALIPAY_SANDBOX_APPID       sandbox app id (open.alipay.com 控制台-沙箱)
//	ALIPAY_SANDBOX_SELLER_ID   sandbox seller (pid)
//	ALIPAY_PUBLIC_KEY_PATH     alipay public key PEM (PKIX) — verifies responses
//	ALIPAY_MERCHANT_KEY_PATH   merchant private key PEM (PKCS8) — signs requests
//
// Optional env:
//
//	ALIPAY_SANDBOX_GATEWAY     default https://openapi-sandbox.dl.alipaydev.com/gateway.do
//	ALIPAY_NOTIFY_URL          public notify url (needed only for callback tests)
//	ALIPAY_SANDBOX_INTERACTIVE set to 1 to run the paid flow: prints the QR
//	                           code, waits for a real sandbox-wallet payment
//	                           (ALIPAY_SANDBOX_PAY_TIMEOUT, default 180s),
//	                           then refunds it in full and reconciles.
func alipaySandboxConfig(t *testing.T) AlipayConfig {
	t.Helper()
	appID := os.Getenv("ALIPAY_SANDBOX_APPID")
	seller := os.Getenv("ALIPAY_SANDBOX_SELLER_ID")
	pub := os.Getenv("ALIPAY_PUBLIC_KEY_PATH")
	priv := os.Getenv("ALIPAY_MERCHANT_KEY_PATH")
	if appID == "" || seller == "" || pub == "" || priv == "" {
		t.Skip("alipay sandbox credentials not configured (ALIPAY_SANDBOX_APPID / ALIPAY_SANDBOX_SELLER_ID / ALIPAY_PUBLIC_KEY_PATH / ALIPAY_MERCHANT_KEY_PATH); skip is not a pass — ALI-01/03/04/05 stay pending")
	}
	cfg := AlipayConfig{
		AppID:               appID,
		SellerID:            seller,
		AlipayPublicKeyPath: pub,
		MerchantPrivKeyPath: priv,
		NotifyURL:           os.Getenv("ALIPAY_NOTIFY_URL"),
	}
	if g := os.Getenv("ALIPAY_SANDBOX_GATEWAY"); g != "" {
		cfg.GatewayURL = g
	} else {
		cfg.GatewayURL = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	}
	return cfg
}

func alipaySandboxProvider(t *testing.T) *AlipayProvider {
	t.Helper()
	p, err := NewAlipayProvider(alipaySandboxConfig(t))
	if err != nil {
		t.Fatalf("sandbox provider: %v", err)
	}
	return p
}

func alipaySandboxOrderID(prefix string) string {
	return fmt.Sprintf("omsb-%s-%d", prefix, time.Now().UnixNano()%1e12)
}

// ALI-01: precreate a real sandbox order, reconcile it by the ORIGINAL
// out_trade_no (response-lost recovery path), then close it and observe the
// closed state. Amount stays integer fen end-to-end.
func TestAlipaySandboxCreateQueryClose(t *testing.T) {
	p := alipaySandboxProvider(t)
	ctx := context.Background()
	out := alipaySandboxOrderID("c1")

	created, err := p.Create(ctx, OrderRequest{OrderID: "sb-1", MerchantOrderID: out, AmountFen: 1, Currency: "CNY"})
	if err != nil {
		t.Fatalf("precreate: %v", err)
	}
	if created.State != StatePending || created.ProviderID != out || created.CheckoutURL == "" {
		t.Fatalf("create result: %+v", created)
	}
	t.Logf("qr_code=%s out_trade_no=%s", created.CheckoutURL, out)

	// Response-lost recovery: a later Query on the SAME id is the only
	// reconciliation path; a new key must never be created.
	q1, err := p.Query(ctx, out)
	if err != nil {
		t.Fatalf("query after create: %v", err)
	}
	if q1.ProviderID != out || q1.State != StatePending {
		t.Fatalf("query state after create: %+v (want pending on the same id)", q1)
	}

	if err := p.Close(ctx, out); err != nil {
		t.Fatalf("close: %v", err)
	}
	q2, err := p.Query(ctx, out)
	if err != nil {
		t.Fatalf("query after close: %v", err)
	}
	if q2.State != StateClosed {
		t.Fatalf("query state after close: %+v (want closed)", q2)
	}
}

// ALI-03 (negative): querying an order that never existed must surface the
// channel's answer as an error — never a fabricated state.
func TestAlipaySandboxQueryNotFound(t *testing.T) {
	p := alipaySandboxProvider(t)
	missing := alipaySandboxOrderID("nf")
	res, err := p.Query(context.Background(), missing)
	if err == nil {
		t.Fatalf("query of nonexistent order must fail, got %+v", res)
	}
	if res.State == StateSucceeded {
		t.Fatalf("nonexistent order must never map to succeeded: %+v", res)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error must reference the original id: %v", err)
	}
}

// ALI-04 (unpaid leg + same-key retry): refunding an order that was never
// paid is rejected by the channel; retrying the SAME refund key must stay
// consistent (no double effect, no state flip-flop).
func TestAlipaySandboxRefundUnpaidAndRetry(t *testing.T) {
	p := alipaySandboxProvider(t)
	ctx := context.Background()
	out := alipaySandboxOrderID("rf")
	created, err := p.Create(ctx, OrderRequest{OrderID: "sb-2", MerchantOrderID: out, AmountFen: 1, Currency: "CNY"})
	if err != nil {
		t.Fatalf("precreate: %v", err)
	}
	if created.ProviderID != out || created.CheckoutURL == "" {
		t.Fatalf("precreate result: %+v", created)
	}
	refundKey := out + "-r1"

	r1, err1 := p.Refund(ctx, RefundRequest{RefundID: refundKey, ProviderID: out, AmountFen: 1})
	if err1 == nil || r1.State == StateSucceeded {
		t.Fatalf("refund on unpaid order must be rejected: %+v %v", r1, err1)
	}
	r2, err2 := p.Refund(ctx, RefundRequest{RefundID: refundKey, ProviderID: out, AmountFen: 1})
	if err2 == nil || r2.State == StateSucceeded {
		t.Fatalf("refund retry on unpaid order must stay rejected: %+v %v", r2, err2)
	}
	t.Logf("unpaid refund rejected consistently: first=%v retry=%v", err1, err2)
}

// ALI-05 core (interactive, opt-in): real sandbox payment -> full refund ->
// refund reconciliation, plus idempotent re-file of the same refund key.
func TestAlipaySandboxInteractivePaidRefundFlow(t *testing.T) {
	if os.Getenv("ALIPAY_SANDBOX_INTERACTIVE") != "1" {
		t.Skip("interactive paid flow not requested (ALIPAY_SANDBOX_INTERACTIVE=1); requires paying the printed QR with the sandbox buyer wallet")
	}
	p := alipaySandboxProvider(t)
	ctx := context.Background()
	out := alipaySandboxOrderID("pay")

	created, err := p.Create(ctx, OrderRequest{OrderID: "sb-5", MerchantOrderID: out, AmountFen: 1, Currency: "CNY"})
	if err != nil {
		t.Fatalf("precreate: %v", err)
	}
	t.Logf("PAY THIS QR WITH THE SANDBOX BUYER WALLET: %s", created.CheckoutURL)

	timeout := 180 * time.Second
	if v := os.Getenv("ALIPAY_SANDBOX_PAY_TIMEOUT"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			timeout = time.Duration(secs) * time.Second
		}
	}
	deadline := time.Now().Add(timeout)
	var paid AttemptResult
	for time.Now().Before(deadline) {
		q, err := p.Query(ctx, out)
		if err == nil && q.State == StateSucceeded {
			paid = q
			break
		}
		time.Sleep(3 * time.Second)
	}
	if paid.State != StateSucceeded {
		t.Fatalf("payment not observed within %s (last query error above); rerun with a longer ALIPAY_SANDBOX_PAY_TIMEOUT", timeout)
	}

	refundKey := out + "-r1"
	rr, err := p.Refund(ctx, RefundRequest{RefundID: refundKey, ProviderID: out, AmountFen: 1})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	if rr.State != StateSucceeded {
		t.Fatalf("refund state: %+v", rr)
	}

	qr, err := p.QueryRefund(ctx, refundKey)
	if err != nil {
		t.Fatalf("refund query: %v", err)
	}
	if qr.State != StateSucceeded {
		t.Fatalf("refund query state: %+v", qr)
	}

	// Idempotent re-file of the SAME refund key: channel must report the
	// already-applied refund (fund_change N), still succeeded — never a
	// second money movement.
	rr2, err := p.Refund(ctx, RefundRequest{RefundID: refundKey, ProviderID: out, AmountFen: 1})
	if err != nil {
		t.Fatalf("refund re-file: %v", err)
	}
	if rr2.State != StateSucceeded {
		t.Fatalf("refund re-file state: %+v", rr2)
	}
}
