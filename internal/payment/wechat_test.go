package payment

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
)

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
		fact.Currency != "CNY" || fact.State != StateSucceeded {
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

func TestWechatRejectsDuplicateNotification(t *testing.T) {
	p, key, apiv3 := newCallbackFixture(t)
	body := buildNotification(t, apiv3, "1900000001", "wx-test-app", "out-1", "txn-1", "SUCCESS", 100)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := callbackHeaders("PLAT-SERIAL-1", ts, "hdrnonce", signCallback(t, key, ts, "hdrnonce", body))
	if _, err := p.Verify(context.Background(), h, body); err != nil {
		t.Fatalf("first delivery rejected: %v", err)
	}
	if _, err := p.Verify(context.Background(), h, body); !errors.Is(err, ErrReplayedNonce) {
		t.Fatalf("replayed notification accepted: %v", err)
	}
}
