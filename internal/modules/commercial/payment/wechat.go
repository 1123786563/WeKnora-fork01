package payment

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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
)

// ProviderWechat names the WeChat Pay channel on PaymentFact.Provider.
const ProviderWechat = "wechat"

// WeChat Pay APIv3 defaults: callback timestamps outside the clock-skew
// window are rejected, and channel calls time out into an unknown state.
const (
	wechatDefaultTimeout   = 10 * time.Second
	wechatDefaultClockSkew = 5 * time.Minute
)

var (
	// ErrSignatureRejected reports a callback whose signature does not
	// verify against the configured platform certificate.
	ErrSignatureRejected = errors.New("wechat_signature_rejected")
	// ErrUnknownCertSerial reports a Wechatpay-Serial that matches no
	// CONFIGURED platform certificate reference. Client-supplied keys
	// are never trusted, so an unknown serial is fatal.
	ErrUnknownCertSerial = errors.New("wechat_unknown_cert_serial")
	// ErrStaleTimestamp reports a callback outside the clock-skew window.
	ErrStaleTimestamp = errors.New("wechat_stale_timestamp")
	// ErrMerchantMismatch reports a decrypted mchid/appid that differs
	// from the configured merchant application.
	ErrMerchantMismatch = errors.New("wechat_merchant_mismatch")
	// ErrMalformedCallback reports an unparsable callback envelope,
	// ciphertext, or decrypted resource.
	ErrMalformedCallback = errors.New("wechat_malformed_callback")
	// ErrNotConfigured reports missing key references or material for
	// an operation that requires them.
	ErrNotConfigured = errors.New("wechat_not_configured")
)

// VerifyWechatSignature verifies a WeChat Pay APIv3 callback signature:
// RSA-SHA256 (PKCS#1 v1.5) over ts + "\n" + nonce + "\n" + body + "\n",
// with the signature base64 (std) encoded.
func VerifyWechatSignature(key *rsa.PublicKey, ts, nonce string, body []byte, signature string) error {
	raw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(ts + "\n" + nonce + "\n" + string(body) + "\n"))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], raw)
}

// WechatPlatformCertRef references a WeChat Pay platform certificate by
// serial number and PEM file path. Only references live in configuration;
// key material is resolved once at construction. A callback can select a
// key by serial but can never supply its own public key.
type WechatPlatformCertRef struct {
	Serial        string
	PublicKeyPath string
}

// WechatConfig carries identifiers, endpoints and KEY REFERENCES ONLY —
// serials and file paths, never embedded key material.
type WechatConfig struct {
	AppID         string
	MchID         string
	APIBaseURL    string // defaults to the official APIv3 gateway
	NotifyURL     string
	PlatformCerts []WechatPlatformCertRef
	MchSerial     string // merchant certificate serial for request signing
	MchKeyPath    string // merchant private-key PEM path (reference)
	APIv3KeyPath  string // 32-byte APIv3 symmetric key file (reference)
	Timeout       time.Duration
	MaxClockSkew  time.Duration
}

func (c WechatConfig) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return wechatDefaultTimeout
}

func (c WechatConfig) clockSkew() time.Duration {
	if c.MaxClockSkew > 0 {
		return c.MaxClockSkew
	}
	return wechatDefaultClockSkew
}

func (c WechatConfig) apiBase() string {
	if c.APIBaseURL != "" {
		return strings.TrimRight(c.APIBaseURL, "/")
	}
	return "https://api.mch.weixin.qq.com"
}

// WechatProvider implements Provider against WeChat Pay APIv3.
type WechatProvider struct {
	cfg           WechatConfig
	platformKeys  map[string]*rsa.PublicKey // serial -> key, resolved from configured refs only
	apiv3Key      []byte                    // resolved from APIv3KeyPath; nil when unset
	client        *http.Client
	now           func() time.Time
	keyOnce       sync.Once
	mchPrivateKey *rsa.PrivateKey
	keyErr        error
}

// NewWechatProvider resolves every configured key reference exactly once:
// platform certificate public keys and the 32-byte APIv3 key are loaded
// from their configured paths. Missing or malformed references are a
// construction error — a provider never starts half-configured.
func NewWechatProvider(cfg WechatConfig) (*WechatProvider, error) {
	if cfg.MchID == "" || cfg.AppID == "" {
		return nil, fmt.Errorf("%w: mchid and appid are required", ErrNotConfigured)
	}
	keys := make(map[string]*rsa.PublicKey, len(cfg.PlatformCerts))
	for _, ref := range cfg.PlatformCerts {
		if ref.Serial == "" || ref.PublicKeyPath == "" {
			return nil, fmt.Errorf("%w: platform cert reference needs serial and path", ErrNotConfigured)
		}
		raw, err := os.ReadFile(ref.PublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("wechat platform cert %s: %w", ref.Serial, err)
		}
		key, err := parseRSAPublicKey(raw)
		if err != nil {
			return nil, fmt.Errorf("wechat platform cert %s: %w", ref.Serial, err)
		}
		if _, dup := keys[ref.Serial]; dup {
			return nil, fmt.Errorf("%w: duplicate platform cert serial %s", ErrNotConfigured, ref.Serial)
		}
		keys[ref.Serial] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no platform certificates configured", ErrNotConfigured)
	}
	var apiv3 []byte
	if cfg.APIv3KeyPath != "" {
		raw, err := os.ReadFile(cfg.APIv3KeyPath)
		if err != nil {
			return nil, fmt.Errorf("wechat apiv3 key: %w", err)
		}
		apiv3 = bytes.TrimSpace(raw)
		if len(apiv3) != 32 {
			return nil, fmt.Errorf("%w: APIv3 key must be 32 bytes, got %d", ErrNotConfigured, len(apiv3))
		}
	}
	return newWechatProvider(cfg, keys, apiv3), nil
}

// newWechatProvider builds a provider from already-resolved key material.
// It exists for in-process tests of the verification logic; production
// code must go through NewWechatProvider so keys come only from
// configured references.
func newWechatProvider(cfg WechatConfig, platformKeys map[string]*rsa.PublicKey, apiv3Key []byte) *WechatProvider {
	return &WechatProvider{
		cfg:          cfg,
		platformKeys: platformKeys,
		apiv3Key:     apiv3Key,
		client:       &http.Client{Timeout: cfg.timeout()},
		now:          time.Now,
	}
}

type wechatEncryptedResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
}

type wechatCallbackEnvelope struct {
	ID        string                  `json:"id"`
	EventType string                  `json:"event_type"`
	Resource  wechatEncryptedResource `json:"resource"`
}

type wechatAmountJSON struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}

type wechatPaymentResult struct {
	MchID         string           `json:"mchid"`
	AppID         string           `json:"appid"`
	OutTradeNo    string           `json:"out_trade_no"`
	TransactionID string           `json:"transaction_id"`
	TradeState    string           `json:"trade_state"`
	Amount        wechatAmountJSON `json:"amount"`
}

// Verify authenticates one payment callback end to end (WX-02): serial
// selection from CONFIGURED references only, timestamp window,
// RSA-SHA256 signature over the raw body, AES-256-GCM resource decryption
// and mchid/appid matching. A redelivery of an ALREADY VERIFIED
// notification is not rejected here: authenticity still comes from the
// signature, freshness from the clock-skew window, and exactly-once
// fulfillment from ConfirmPayment's transactional replay (C01), so the
// duplicate reaches the store and gets the idempotent success ACK AC-03
// requires instead of a 401 that keeps the provider retrying. The
// returned fact carries no local identity — the caller resolves
// TenantID/OrderID server-side.
func (p *WechatProvider) Verify(ctx context.Context, header http.Header, body []byte) (commercial.PaymentFact, error) {
	var fact commercial.PaymentFact
	serial := header.Get("Wechatpay-Serial")
	ts := header.Get("Wechatpay-Timestamp")
	nonce := header.Get("Wechatpay-Nonce")
	signature := header.Get("Wechatpay-Signature")
	if serial == "" || ts == "" || nonce == "" || signature == "" {
		return fact, fmt.Errorf("%w: missing callback headers", ErrMalformedCallback)
	}
	key, ok := p.platformKeys[serial]
	if !ok {
		return fact, fmt.Errorf("%w: %s", ErrUnknownCertSerial, serial)
	}
	tsUnix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fact, fmt.Errorf("%w: bad timestamp %q", ErrMalformedCallback, ts)
	}
	skew := p.cfg.clockSkew()
	now := p.now()
	if delta := now.Sub(time.Unix(tsUnix, 0)); delta > skew || delta < -skew {
		return fact, fmt.Errorf("%w: timestamp %s", ErrStaleTimestamp, ts)
	}
	if err := VerifyWechatSignature(key, ts, nonce, body, signature); err != nil {
		return fact, fmt.Errorf("%w: %v", ErrSignatureRejected, err)
	}

	var envelope wechatCallbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fact, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	if !strings.HasPrefix(envelope.EventType, "TRANSACTION.") {
		return fact, fmt.Errorf("%w: unsupported event_type %q", ErrMalformedCallback, envelope.EventType)
	}
	plain, err := p.decryptResource(envelope.Resource)
	if err != nil {
		return fact, err
	}
	var result wechatPaymentResult
	if err := json.Unmarshal(plain, &result); err != nil {
		return fact, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	if result.MchID != p.cfg.MchID || result.AppID != p.cfg.AppID {
		return fact, fmt.Errorf("%w: mchid=%q appid=%q", ErrMerchantMismatch, result.MchID, result.AppID)
	}
	currency := result.Amount.Currency
	if currency == "" {
		currency = "CNY"
	}
	fact = commercial.PaymentFact{
		Provider:    ProviderWechat,
		Merchant:    result.MchID, // trusted: matched against the configured merchant
		AttemptID:   result.OutTradeNo,
		Transaction: result.TransactionID,
		Amount:      commercial.CNYFen(result.Amount.Total),
		Currency:    currency,
		State:       mapWechatTradeState(result.TradeState).String(),
	}
	return fact, nil
}

// decryptResource opens the AES-256-GCM ciphertext exactly per WX-02:
// APIv3 key, resource nonce, associated data as AAD.
func (p *WechatProvider) decryptResource(r wechatEncryptedResource) ([]byte, error) {
	if len(p.apiv3Key) != 32 {
		return nil, fmt.Errorf("%w: APIv3 key not configured", ErrNotConfigured)
	}
	raw, err := base64.StdEncoding.DecodeString(r.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	block, err := aes.NewCipher(p.apiv3Key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	if len(r.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("%w: nonce size %d", ErrMalformedCallback, len(r.Nonce))
	}
	plain, err := gcm.Open(nil, []byte(r.Nonce), raw, []byte(r.AssociatedData))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedCallback, err)
	}
	return plain, nil
}

// mapWechatTradeState projects channel trade states onto domain states.
func mapWechatTradeState(state string) AttemptState {
	switch state {
	case "SUCCESS":
		return StateSucceeded
	case "NOTPAY", "USERPAYING", "ACCEPT":
		return StatePending
	case "CLOSED":
		return StateClosed
	case "REFUND":
		// The transaction succeeded once; refunds are tracked by
		// Refund/QueryRefund, so the fact still reports success.
		return StateSucceeded
	default:
		return AttemptState(strings.ToLower(state))
	}
}

type wechatNativeOrderRequest struct {
	AppID       string           `json:"appid"`
	MchID       string           `json:"mchid"`
	Description string           `json:"description"`
	OutTradeNo  string           `json:"out_trade_no"`
	NotifyURL   string           `json:"notify_url,omitempty"`
	Amount      wechatAmountJSON `json:"amount"`
}

type wechatNativeOrderResponse struct {
	CodeURL string `json:"code_url"`
}

type wechatTransactionResponse struct {
	OutTradeNo string           `json:"out_trade_no"`
	TradeState string           `json:"trade_state"`
	Amount     wechatAmountJSON `json:"amount"`
}

type wechatCloseRequest struct {
	MchID string `json:"mchid"`
}

type wechatRefundAmount struct {
	Refund   int64  `json:"refund"`
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}

type wechatRefundRequest struct {
	OutRefundNo string             `json:"out_refund_no"`
	OutTradeNo  string             `json:"out_trade_no"`
	NotifyURL   string             `json:"notify_url,omitempty"`
	Amount      wechatRefundAmount `json:"amount"`
}

type wechatRefundResponse struct {
	OutRefundNo string `json:"out_refund_no"`
	Status      string `json:"status"`
}

// Create registers a Native (WX-01 field set) channel order. On transport
// timeout the result is StateUnknown keyed by the ORIGINAL
// MerchantOrderID: the caller must Query that identifier, never re-key.
func (p *WechatProvider) Create(ctx context.Context, req OrderRequest) (AttemptResult, error) {
	if req.MerchantOrderID == "" || req.AmountFen <= 0 {
		return AttemptResult{}, fmt.Errorf("%w: merchant order id and positive amount required", ErrInvalidRequest)
	}
	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	payload := wechatNativeOrderRequest{
		AppID:       p.cfg.AppID,
		MchID:       p.cfg.MchID,
		Description: "WeKnora order " + req.OrderID,
		OutTradeNo:  req.MerchantOrderID,
		NotifyURL:   p.cfg.NotifyURL,
		Amount:      wechatAmountJSON{Total: req.AmountFen, Currency: currency},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return AttemptResult{}, err
	}
	var out wechatNativeOrderResponse
	if err := p.do(ctx, http.MethodPost, "/v3/pay/transactions/native", body, &out); err != nil {
		return AttemptResult{State: stateIfTimeout(err, StateUnknown), ProviderID: req.MerchantOrderID},
			fmt.Errorf("wechat create %s: %w", req.MerchantOrderID, err)
	}
	return AttemptResult{State: StatePending, ProviderID: req.MerchantOrderID, CheckoutURL: out.CodeURL}, nil
}

// Query reconciles an attempt by its ORIGINAL out_trade_no (WX-03).
func (p *WechatProvider) Query(ctx context.Context, providerID string) (AttemptResult, error) {
	if providerID == "" {
		return AttemptResult{}, fmt.Errorf("%w: empty provider id", ErrInvalidRequest)
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(providerID) + "?mchid=" + url.QueryEscape(p.cfg.MchID)
	var out wechatTransactionResponse
	if err := p.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return AttemptResult{State: stateIfTimeout(err, StateUnknown), ProviderID: providerID},
			fmt.Errorf("wechat query %s: %w", providerID, err)
	}
	id := out.OutTradeNo
	if id == "" {
		id = providerID
	}
	return AttemptResult{State: mapWechatTradeState(out.TradeState), ProviderID: id}, nil
}

// Close cancels a pending channel order by its original identifier.
func (p *WechatProvider) Close(ctx context.Context, providerID string) error {
	if providerID == "" {
		return fmt.Errorf("%w: empty provider id", ErrInvalidRequest)
	}
	body, err := json.Marshal(wechatCloseRequest{MchID: p.cfg.MchID})
	if err != nil {
		return err
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(providerID) + "/close"
	if err := p.do(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("wechat close %s: %w", providerID, err)
	}
	return nil
}

// Refund files a refund (WX-04) against the original order identifier.
// The C02 RefundRequest field set carries no original total, so the
// refund is filed as full-amount; partial refunds extend the type later.
func (p *WechatProvider) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	if req.RefundID == "" || req.ProviderID == "" || req.AmountFen <= 0 {
		return RefundResult{}, fmt.Errorf("%w: refund id, provider id and positive amount required", ErrInvalidRequest)
	}
	payload := wechatRefundRequest{
		OutRefundNo: req.RefundID,
		OutTradeNo:  req.ProviderID,
		NotifyURL:   p.cfg.NotifyURL,
		Amount:      wechatRefundAmount{Refund: req.AmountFen, Total: req.AmountFen, Currency: "CNY"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return RefundResult{}, err
	}
	var out wechatRefundResponse
	if err := p.do(ctx, http.MethodPost, "/v3/refund/domestic/refunds", body, &out); err != nil {
		return RefundResult{State: stateIfTimeout(err, StateUnknown), ProviderID: req.RefundID},
			fmt.Errorf("wechat refund %s: %w", req.RefundID, err)
	}
	return RefundResult{State: mapWechatRefundState(out.Status), ProviderID: req.RefundID}, nil
}

// QueryRefund reconciles a refund by its ORIGINAL out_refund_no (WX-05).
func (p *WechatProvider) QueryRefund(ctx context.Context, refundID string) (RefundResult, error) {
	if refundID == "" {
		return RefundResult{}, fmt.Errorf("%w: empty refund id", ErrInvalidRequest)
	}
	var out wechatRefundResponse
	if err := p.do(ctx, http.MethodGet, "/v3/refund/domestic/refunds/"+url.PathEscape(refundID), nil, &out); err != nil {
		return RefundResult{State: stateIfTimeout(err, StateUnknown), ProviderID: refundID},
			fmt.Errorf("wechat query refund %s: %w", refundID, err)
	}
	id := out.OutRefundNo
	if id == "" {
		id = refundID
	}
	return RefundResult{State: mapWechatRefundState(out.Status), ProviderID: id}, nil
}

func mapWechatRefundState(status string) AttemptState {
	switch status {
	case "SUCCESS":
		return StateSucceeded
	case "PROCESSING", "ACCEPT":
		return StatePending
	case "ABNORMAL":
		return AttemptState("abnormal")
	case "CLOSED":
		return StateClosed
	default:
		return AttemptState(strings.ToLower(status))
	}
}

// stateIfTimeout keeps StateUnknown for transport timeouts only; other
// errors return an empty state so the caller treats them as failures.
func stateIfTimeout(err error, timeoutState AttemptState) AttemptState {
	if isTimeoutErr(err) {
		return timeoutState
	}
	return ""
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// do sends one signed APIv3 request. The Authorization header is signed
// with the merchant private key resolved from its configured path; a
// missing reference fails closed with ErrNotConfigured.
func (p *WechatProvider) do(ctx context.Context, method, path string, body []byte, out interface{}) error {
	auth, err := p.authorization(method, path, body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, p.cfg.apiBase()+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WeKnora")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &apiErr)
		return fmt.Errorf("wechat api status %d code=%s: %s", resp.StatusCode, apiErr.Code, apiErr.Message)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// authorization builds the WECHATPAY2-SHA256-RSA2048 header value.
func (p *WechatProvider) authorization(method, path string, body []byte) (string, error) {
	key, err := p.merchantKey()
	if err != nil {
		return "", err
	}
	ts := strconv.FormatInt(p.now().Unix(), 10)
	nonce, err := wechatRandomNonce()
	if err != nil {
		return "", err
	}
	message := []byte(method + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + string(body) + "\n")
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("WECHATPAY2-SHA256-RSA2048 mchid=\"%s\",nonce_str=\"%s\",signature=\"%s\",timestamp=\"%s\",serial_str=\"%s\"",
		p.cfg.MchID, nonce, base64.StdEncoding.EncodeToString(sig), ts, p.cfg.MchSerial), nil
}

// merchantKey lazily resolves the merchant private key from its
// configured path reference; the result is memoized for the process.
func (p *WechatProvider) merchantKey() (*rsa.PrivateKey, error) {
	p.keyOnce.Do(func() {
		if p.cfg.MchKeyPath == "" || p.cfg.MchSerial == "" {
			p.keyErr = fmt.Errorf("%w: merchant key path/serial not configured", ErrNotConfigured)
			return
		}
		raw, err := os.ReadFile(p.cfg.MchKeyPath)
		if err != nil {
			p.keyErr = err
			return
		}
		p.mchPrivateKey, p.keyErr = parseRSAPrivateKey(raw)
	})
	return p.mchPrivateKey, p.keyErr
}

func wechatRandomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func parseRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("PEM block is not an RSA public key")
		}
		return rsaKey, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate public key is not RSA")
	}
	return key, nil
}

func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return rsaKey, nil
}
