package payment

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
)

// ProviderAlipay names the Alipay channel on PaymentFact.Provider.
const ProviderAlipay = "alipay"

const (
	alipayDefaultGateway = "https://openapi.alipay.com/gateway.do"
	alipaySignTypeRSA2   = "RSA2"
	alipaySignTypeRSA    = "RSA"
	alipayAPIVersion     = "1.0"
	alipayDefaultTimeout = 10 * time.Second
)

var (
	// ErrAlipaySignatureRejected reports a notify or gateway response whose
	// signature does not verify against the CONFIGURED Alipay public key.
	ErrAlipaySignatureRejected = errors.New("alipay_signature_rejected")
	// ErrAlipayMalformedNotify reports an unparsable notify form or missing
	// mandatory fields (sign, out_trade_no, trade_no, total_amount).
	ErrAlipayMalformedNotify = errors.New("alipay_malformed_notify")
	// ErrAlipayAppMismatch reports an app_id that differs from the
	// configured Alipay application.
	ErrAlipayAppMismatch = errors.New("alipay_app_mismatch")
	// ErrAlipaySellerMismatch reports a seller_id that differs from the
	// configured Alipay merchant account.
	ErrAlipaySellerMismatch = errors.New("alipay_seller_mismatch")
	// ErrAlipayNotConfigured reports missing key references or identifiers
	// for an operation that requires them.
	ErrAlipayNotConfigured = errors.New("alipay_not_configured")
	// ErrAlipaySyncReturn is returned by VerifySyncReturn for EVERY
	// synchronous return_url payload: a customer-facing browser redirect is
	// transport, not proof, and must never confirm a payment. Reconciliation
	// happens only through the signed async notify (Verify) or Query.
	ErrAlipaySyncReturn = errors.New("alipay_sync_return_not_confirmable")
)

// alipayProductionAmount restricts production channel amounts to plain
// decimal yuan with at most two fractional digits BEFORE any Rat parsing:
// fraction expressions (1/2), exponents (1e2), signs, bare ".5", trailing
// "10." and similar are rejected as input-format violations. The big.Rat
// core in ParseCNYAmount remains the exact-arithmetic path.
var alipayProductionAmount = regexp.MustCompile("^[0-9]+(\\.[0-9]{1,2})?$")

// ParseCNYAmount converts a channel decimal-yuan amount into exact integer
// fen. Production inputs are first restricted to the plain-decimal pattern
// above; the big.Rat multiplication below then keeps the conversion exact,
// and values that are not an integer number of fen (or do not fit int64)
// are rejected.
func ParseCNYAmount(s string) (int64, error) {
	if !alipayProductionAmount.MatchString(s) {
		return 0, errors.New("invalid_amount")
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() < 0 {
		return 0, errors.New("invalid_amount")
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, errors.New("invalid_amount_precision")
	}
	return r.Num().Int64(), nil
}

// fenToYuan renders integer fen as the decimal-yuan string Alipay expects.
func fenToYuan(fen int64) string {
	return fmt.Sprintf("%d.%02d", fen/100, fen%100)
}

// AlipayConfig carries identifiers, endpoints and KEY REFERENCES ONLY:
// file paths, never embedded key material. The Alipay public key reference
// authenticates async notifications and gateway responses; the merchant
// private key reference signs outgoing gateway requests.
type AlipayConfig struct {
	AppID               string
	SellerID            string
	GatewayURL          string // defaults to the official openapi gateway
	NotifyURL           string
	SignType            string // RSA2 (default) or RSA
	AlipayPublicKeyPath string
	MerchantPrivKeyPath string
	Timeout             time.Duration
}

func (c AlipayConfig) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return alipayDefaultTimeout
}

func (c AlipayConfig) signType() string {
	if c.SignType == alipaySignTypeRSA {
		return alipaySignTypeRSA
	}
	return alipaySignTypeRSA2
}

func (c AlipayConfig) gateway() string {
	if c.GatewayURL != "" {
		return strings.TrimRight(c.GatewayURL, "/")
	}
	return alipayDefaultGateway
}

// AlipayProvider implements Provider against the Alipay open platform.
//
// Contract structure followed (recorded per ALI-01..05):
//   - Gateway protocol (ALI-01): POST application/x-www-form-urlencoded to
//     gateway.do with common params app_id, method, format=JSON,
//     charset=utf-8, sign_type, timestamp (yyyy-MM-dd HH:mm:ss),
//     version=1.0 and biz_content; the response is a JSON envelope
//     holding "<method>_response", "sign" and "sign_type" keys, whose
//     sign covers the ORIGINAL inner response JSON bytes.
//   - Signatures (ALI-01/02): the signed content is every top-level
//     parameter except sign and sign_type, sorted by key and joined as
//     k=v with & (raw values, no re-encoding); RSA2 = RSA-SHA256
//     (PKCS#1 v1.5), RSA = RSA-SHA1; base64 (std) encoded. Async
//     notifications are application/x-www-form-urlencoded forms whose
//     values are parsed exactly ONCE and never re-encoded or re-signed
//     before verification.
//   - Only TRADE_SUCCESS and TRADE_FINISHED count as trusted success; a
//     synchronous return_url never confirms a payment.
//
// The gate stays: final field-level validation (exact per-product optional
// fields, sandbox behavior) still requires the official readable contract
// plus a merchant environment; no field beyond the documented async-notify
// basics above is guessed. Real-channel acceptance (ALI-01..05) therefore
// remains blocked-env.
type AlipayProvider struct {
	cfg           AlipayConfig
	alipayPubKey  *rsa.PublicKey // resolved from the configured reference only
	client        *http.Client
	now           func() time.Time
	mchKeyOnce    bool
	mchPrivateKey *rsa.PrivateKey
	mchKeyErr     error
}

// NewAlipayProvider resolves the Alipay public key from its configured
// reference exactly once; missing or malformed references are a
// construction error, so the provider never starts half-configured. The
// merchant private key is resolved lazily from its reference on the first
// outgoing gateway call and fails closed there.
func NewAlipayProvider(cfg AlipayConfig) (*AlipayProvider, error) {
	if cfg.AppID == "" || cfg.SellerID == "" {
		return nil, fmt.Errorf("%w: app_id and seller_id are required", ErrAlipayNotConfigured)
	}
	if cfg.AlipayPublicKeyPath == "" {
		return nil, fmt.Errorf("%w: alipay public key path required", ErrAlipayNotConfigured)
	}
	raw, err := os.ReadFile(cfg.AlipayPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("alipay public key: %w", err)
	}
	pub, err := parseRSAPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("alipay public key: %w", err)
	}
	return &AlipayProvider{
		cfg:          cfg,
		alipayPubKey: pub,
		client:       &http.Client{Timeout: cfg.timeout()},
		now:          time.Now,
	}, nil
}

// alipaySignContent builds the signed content per ALI-01/02: all params
// except sign and sign_type, keys sorted, joined as k=v with &. Values are
// used exactly as stored (already decoded once for notifications, plain
// for outgoing requests), never re-encoded.
// alipayRequestSignContent builds the signed content for OUTBOUND gateway
// requests: unlike the async-notify rule, sign_type IS part of the signed
// content (only sign is excluded). Verified against the official sandbox —
// excluding sign_type here is rejected with isv.invalid-signature — and it
// matches the official SDK's get_sign_content behavior.
func alipayRequestSignContent(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+values.Get(k))
	}
	return strings.Join(parts, "&")
}

func alipaySignContent(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+values.Get(k))
	}
	return strings.Join(parts, "&")
}

func alipaySignHash(signType string) crypto.Hash {
	if signType == alipaySignTypeRSA {
		return crypto.SHA1
	}
	return crypto.SHA256
}

// alipayVerifySignature checks an RSA2/RSA PKCS#1 v1.5 signature over the
// given content against the CONFIGURED Alipay public key.
func (p *AlipayProvider) alipayVerifySignature(signType, sign, content string) error {
	raw, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return fmt.Errorf("%w: bad signature encoding: %v", ErrAlipaySignatureRejected, err)
	}
	hash := alipaySignHash(signType)
	var digest []byte
	if hash == crypto.SHA1 {
		sum := sha1.Sum([]byte(content))
		digest = sum[:]
	} else {
		sum := sha256.Sum256([]byte(content))
		digest = sum[:]
	}
	if err := rsa.VerifyPKCS1v15(p.alipayPubKey, hash, digest, raw); err != nil {
		return fmt.Errorf("%w: %v", ErrAlipaySignatureRejected, err)
	}
	return nil
}

// Verify authenticates one asynchronous payment notification (ALI-02). The
// RAW form body is parsed exactly once; the signature is recomputed over
// the received values without re-encoding, so any double-decode or re-sign
// mismatch fails closed. app_id and seller_id must match the configured
// application and merchant; only TRADE_SUCCESS and TRADE_FINISHED produce
// a succeeded PaymentFact. The returned fact carries no local identity:
// the caller resolves TenantID/OrderID server-side (C01 registry). A
// re-delivered signed notification verifies again: Alipay retries until it
// sees "success", and dedupe is owned by ConfirmPayment (idempotent replay).
func (p *AlipayProvider) Verify(ctx context.Context, header http.Header, body []byte) (commercial.PaymentFact, error) {
	var fact commercial.PaymentFact
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return fact, fmt.Errorf("%w: %v", ErrAlipayMalformedNotify, err)
	}
	sign := values.Get("sign")
	if sign == "" {
		return fact, fmt.Errorf("%w: missing sign", ErrAlipayMalformedNotify)
	}
	signType := values.Get("sign_type")
	if signType == "" {
		signType = p.cfg.signType()
	}
	if err := p.alipayVerifySignature(signType, sign, alipaySignContent(values)); err != nil {
		return fact, err
	}
	if appID := values.Get("app_id"); appID != p.cfg.AppID {
		return fact, fmt.Errorf("%w: %q", ErrAlipayAppMismatch, appID)
	}
	if seller := values.Get("seller_id"); seller == "" || seller != p.cfg.SellerID {
		return fact, fmt.Errorf("%w: %q", ErrAlipaySellerMismatch, seller)
	}
	outTradeNo := values.Get("out_trade_no")
	tradeNo := values.Get("trade_no")
	tradeStatus := values.Get("trade_status")
	totalAmount := values.Get("total_amount")
	if outTradeNo == "" || tradeNo == "" || tradeStatus == "" || totalAmount == "" {
		return fact, fmt.Errorf("%w: out_trade_no/trade_no/trade_status/total_amount required", ErrAlipayMalformedNotify)
	}
	fen, err := ParseCNYAmount(totalAmount)
	if err != nil {
		return fact, fmt.Errorf("%w: total_amount %q: %v", ErrAlipayMalformedNotify, totalAmount, err)
	}
	fact = commercial.PaymentFact{
		Provider:    ProviderAlipay,
		Merchant:    p.cfg.SellerID, // trusted: matched against the configured seller
		AttemptID:   outTradeNo,
		Transaction: tradeNo,
		Amount:      commercial.CNYFen(fen),
		Currency:    "CNY",
		State:       mapAlipayTradeStatus(tradeStatus).String(),
	}
	return fact, nil
}

// VerifySyncReturn NEVER confirms a payment: the synchronous return_url is
// an unsigned customer-side redirect. Every call fails with
// ErrAlipaySyncReturn so callers cannot accidentally treat it as a notify.
func (p *AlipayProvider) VerifySyncReturn(ctx context.Context, header http.Header, body []byte) (commercial.PaymentFact, error) {
	return commercial.PaymentFact{}, ErrAlipaySyncReturn
}

// mapAlipayTradeStatus projects channel trade statuses onto domain states.
// Only TRADE_SUCCESS and TRADE_FINISHED are trusted success.
func mapAlipayTradeStatus(status string) AttemptState {
	switch status {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		return StateSucceeded
	case "WAIT_BUYER_PAY":
		return StatePending
	case "TRADE_CLOSED":
		return StateClosed
	default:
		return AttemptState(strings.ToLower(status))
	}
}

type alipayPrecreateBiz struct {
	OutTradeNo  string "json:\"out_trade_no\""
	TotalAmount string "json:\"total_amount\""
	Subject     string "json:\"subject\""
}

type alipayPrecreateResponse struct {
	alipayResponseHead
	QRCode string "json:\"qr_code\""
}

type alipayQueryBiz struct {
	OutTradeNo string "json:\"out_trade_no\""
}

type alipayQueryResponse struct {
	alipayResponseHead
	OutTradeNo  string "json:\"out_trade_no\""
	TradeNo     string "json:\"trade_no\""
	TradeStatus string "json:\"trade_status\""
	TotalAmount string "json:\"total_amount\""
}

type alipayCloseBiz struct {
	OutTradeNo string "json:\"out_trade_no\""
}

type alipayRefundBiz struct {
	OutTradeNo   string "json:\"out_trade_no\""
	OutRequestNo string "json:\"out_request_no\""
	RefundAmount string "json:\"refund_amount\""
}

type alipayRefundResponse struct {
	alipayResponseHead
	TradeNo    string "json:\"trade_no\""
	OutTradeNo string "json:\"out_trade_no\""
	FundChange string "json:\"fund_change\""
}

type alipayRefundQueryBiz struct {
	OutTradeNo   string "json:\"out_trade_no\""
	OutRequestNo string "json:\"out_request_no\""
}

type alipayRefundQueryResponse struct {
	alipayResponseHead
	OutRequestNo string "json:\"out_request_no\""
	OutTradeNo   string "json:\"out_trade_no\""
	RefundStatus string "json:\"refund_status\""
}

// alipayResponseHead is the shared gateway response status block.
// alipayCloseResponse carries the shared status block so Close FAILS on
// business errors (e.g. ACQ.TRADE_NOT_EXIST) instead of swallowing them:
// call() skips business-code validation when out is nil.
type alipayCloseResponse struct {
	alipayResponseHead
}

type alipayResponseHead struct {
	Code    string "json:\"code\""
	Msg     string "json:\"msg\""
	SubCode string "json:\"sub_code\""
	SubMsg  string "json:\"sub_msg\""
}

// Create registers a precreate (ALI-01) channel order and returns the QR
// code URL as CheckoutURL. On transport timeout the result is StateUnknown
// keyed by the ORIGINAL MerchantOrderID: the caller must Query that
// identifier, never re-key.
func (p *AlipayProvider) Create(ctx context.Context, req OrderRequest) (AttemptResult, error) {
	if req.MerchantOrderID == "" || req.AmountFen <= 0 {
		return AttemptResult{}, fmt.Errorf("%w: merchant order id and positive amount required", ErrInvalidRequest)
	}
	var out alipayPrecreateResponse
	err := p.call(ctx, "alipay.trade.precreate", alipayPrecreateBiz{
		OutTradeNo:  req.MerchantOrderID,
		TotalAmount: fenToYuan(req.AmountFen),
		Subject:     "WeKnora order " + req.OrderID,
	}, &out)
	if err != nil {
		return AttemptResult{State: stateIfTimeout(err, StateUnknown), ProviderID: req.MerchantOrderID},
			fmt.Errorf("alipay precreate %s: %w", req.MerchantOrderID, err)
	}
	return AttemptResult{State: StatePending, ProviderID: req.MerchantOrderID, CheckoutURL: out.QRCode}, nil
}

// Query reconciles an attempt by its ORIGINAL out_trade_no (ALI-03); it is
// the recovery path when a notify was missed.
func (p *AlipayProvider) Query(ctx context.Context, providerID string) (AttemptResult, error) {
	if providerID == "" {
		return AttemptResult{}, fmt.Errorf("%w: empty provider id", ErrInvalidRequest)
	}
	var out alipayQueryResponse
	err := p.call(ctx, "alipay.trade.query", alipayQueryBiz{OutTradeNo: providerID}, &out)
	if err != nil {
		return AttemptResult{State: stateIfTimeout(err, StateUnknown), ProviderID: providerID},
			fmt.Errorf("alipay query %s: %w", providerID, err)
	}
	id := out.OutTradeNo
	if id == "" {
		id = providerID
	}
	return AttemptResult{State: mapAlipayTradeStatus(out.TradeStatus), ProviderID: id}, nil
}

// Close cancels a pending channel order by its original identifier.
func (p *AlipayProvider) Close(ctx context.Context, providerID string) error {
	if providerID == "" {
		return fmt.Errorf("%w: empty provider id", ErrInvalidRequest)
	}
	if err := p.call(ctx, "alipay.trade.close", alipayCloseBiz{OutTradeNo: providerID}, &alipayCloseResponse{}); err != nil {
		return fmt.Errorf("alipay close %s: %w", providerID, err)
	}
	return nil
}

// Refund files a refund (ALI-04) keyed by Alipay's own stable refund
// identifier out_request_no against the original out_trade_no. No WeChat
// field names or semantics are reused.
func (p *AlipayProvider) Refund(ctx context.Context, req RefundRequest) (RefundResult, error) {
	if req.RefundID == "" || req.ProviderID == "" || req.AmountFen <= 0 {
		return RefundResult{}, fmt.Errorf("%w: refund id, provider id and positive amount required", ErrInvalidRequest)
	}
	var out alipayRefundResponse
	err := p.call(ctx, "alipay.trade.refund", alipayRefundBiz{
		OutTradeNo:   req.ProviderID,
		OutRequestNo: req.RefundID,
		RefundAmount: fenToYuan(req.AmountFen),
	}, &out)
	if err != nil {
		return RefundResult{State: stateIfTimeout(err, StateUnknown), ProviderID: req.RefundID},
			fmt.Errorf("alipay refund %s: %w", req.RefundID, err)
	}
	// fund_change Y means funds moved now, N means the refund was already
	// applied earlier (idempotent re-file); both leave it recorded on the
	// channel.
	return RefundResult{State: StateSucceeded, ProviderID: req.RefundID}, nil
}

// QueryRefund reconciles a refund by its ORIGINAL out_request_no (ALI-05).
// The C02 refund query contract passes only the refund identifier; the
// paired out_trade_no travels with the refund record server-side and is
// filled by production wiring when the channel requires both, an extension
// point recorded here rather than guessed.
func (p *AlipayProvider) QueryRefund(ctx context.Context, refundID string) (RefundResult, error) {
	if refundID == "" {
		return RefundResult{}, fmt.Errorf("%w: empty refund id", ErrInvalidRequest)
	}
	var out alipayRefundQueryResponse
	err := p.call(ctx, "alipay.trade.fastpay.refund.query", alipayRefundQueryBiz{
		OutRequestNo: refundID,
	}, &out)
	if err != nil {
		return RefundResult{State: stateIfTimeout(err, StateUnknown), ProviderID: refundID},
			fmt.Errorf("alipay query refund %s: %w", refundID, err)
	}
	id := out.OutRequestNo
	if id == "" {
		id = refundID
	}
	return RefundResult{State: mapAlipayRefundStatus(out.RefundStatus), ProviderID: id}, nil
}

// mapAlipayRefundStatus projects refund statuses onto domain states.
func mapAlipayRefundStatus(status string) AttemptState {
	switch status {
	case "REFUND_SUCCESS":
		return StateSucceeded
	case "REFUND_PROCESSING":
		return StatePending
	case "REFUND_FAIL":
		return StateClosed
	default:
		if status == "" {
			// The channel answers code=10000 with an empty refund_status
			// while the refund is still being booked.
			return StatePending
		}
		return AttemptState(strings.ToLower(status))
	}
}

// call sends one signed gateway request (ALI-01 common params) and verifies
// the response envelope signature over the ORIGINAL inner JSON bytes with
// the configured Alipay public key before unmarshalling it.
func (p *AlipayProvider) call(ctx context.Context, method string, biz interface{}, out interface{}) error {
	bizJSON, err := json.Marshal(biz)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("app_id", p.cfg.AppID)
	form.Set("method", method)
	form.Set("format", "JSON")
	form.Set("charset", "utf-8")
	form.Set("sign_type", p.cfg.signType())
	form.Set("timestamp", p.now().Format("2006-01-02 15:04:05"))
	form.Set("version", alipayAPIVersion)
	if p.cfg.NotifyURL != "" {
		form.Set("notify_url", p.cfg.NotifyURL)
	}
	form.Set("biz_content", string(bizJSON))
	mchKey, err := p.merchantKey()
	if err != nil {
		return err
	}
	hash := alipaySignHash(p.cfg.signType())
	var digest []byte
	if hash == crypto.SHA1 {
		sum := sha1.Sum([]byte(alipayRequestSignContent(form)))
		digest = sum[:]
	} else {
		sum := sha256.Sum256([]byte(alipayRequestSignContent(form)))
		digest = sum[:]
	}
	sig, err := rsa.SignPKCS1v15(rand.Reader, mchKey, hash, digest[:])
	if err != nil {
		return err
	}
	form.Set("sign", base64.StdEncoding.EncodeToString(sig))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.gateway(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1048576))
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("alipay gateway status %d: %s", resp.StatusCode, string(data))
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("alipay gateway envelope: %w", err)
	}
	sign := ""
	if raw, ok := envelope["sign"]; ok {
		_ = json.Unmarshal(raw, &sign)
	}
	var inner json.RawMessage
	for key, raw := range envelope {
		if key == "sign" || key == "sign_type" {
			continue
		}
		if strings.HasSuffix(key, "_response") {
			inner = raw
			break
		}
	}
	if len(inner) == 0 {
		return fmt.Errorf("alipay gateway envelope: no response payload")
	}
	// Fail closed: an unsigned response body is never trusted.
	if sign == "" {
		return fmt.Errorf("%w: gateway response unsigned", ErrAlipaySignatureRejected)
	}
	signType := p.cfg.signType()
	if raw, ok := envelope["sign_type"]; ok {
		_ = json.Unmarshal(raw, &signType)
	}
	if err := p.alipayVerifySignature(signType, sign, string(inner)); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(inner, out); err != nil {
		return fmt.Errorf("alipay gateway response: %w", err)
	}
	if head := responseHeadOf(out); head != nil && head.Code != "" && head.Code != "10000" {
		return fmt.Errorf("alipay code=%s sub_code=%s: %s", head.Code, head.SubCode, head.SubMsg)
	}
	return nil
}

// responseHeadOf extracts the shared status block from a typed response.
func responseHeadOf(out interface{}) *alipayResponseHead {
	switch v := out.(type) {
	case *alipayPrecreateResponse:
		return &v.alipayResponseHead
	case *alipayQueryResponse:
		return &v.alipayResponseHead
	case *alipayRefundResponse:
		return &v.alipayResponseHead
	case *alipayRefundQueryResponse:
		return &v.alipayResponseHead
	case *alipayCloseResponse:
		return &v.alipayResponseHead
	default:
		return nil
	}
}

// merchantKey lazily resolves the merchant private key from its configured
// path reference; the result is memoized for the process.
func (p *AlipayProvider) merchantKey() (*rsa.PrivateKey, error) {
	if !p.mchKeyOnce {
		p.mchKeyOnce = true
		if p.cfg.MerchantPrivKeyPath == "" {
			p.mchKeyErr = fmt.Errorf("%w: merchant private key path not configured", ErrAlipayNotConfigured)
			return nil, p.mchKeyErr
		}
		raw, err := os.ReadFile(p.cfg.MerchantPrivKeyPath)
		if err != nil {
			p.mchKeyErr = err
			return nil, p.mchKeyErr
		}
		p.mchPrivateKey, p.mchKeyErr = parseRSAPrivateKey(raw)
	}
	return p.mchPrivateKey, p.mchKeyErr
}
