package commercialplatform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// ---- create_purchase_subscription (W5, #81) ----
// Provider vocabulary (activation_rules, billing_configuration,
// payment_provider, the provider's public API host and its customer shape)
// lives ONLY in this section — it never crosses into the commercial port
// types or any caller-visible response. Receipts echo WeKnora-derived
// identities only; provider ids stay inside the seam.
//
// Binding decisions (t09 evidence + the 2026-09-23 user ruling): the
// payment-activation channel is a SUPPORTED provider integration. The
// adapter ensures the authority customer carries a provider binding before
// the gated create (a gated create without a bound provider answers 422
// no_linked_payment_provider, F1), sourcing the provider customer id from
// the provider API (key in env) or a dev/test placeholder prefix — both
// missing fails closed unconfigured (spec: stop new purchases).
//
// Fail-closed contract (lago.go precedent): error strings carry no URL, no
// status text, no response body and no credential — only the shared
// sentinel and a short closed description.

// purchaseRequestTimeout bounds one create_purchase_subscription command:
// the binding read/write, the possible provider-customer round trip, the
// identity read and the gated create.
const purchaseRequestTimeout = subscriptionRequestTimeout + 10*time.Second

// outboundProviderHost is the default host of the provider's public API.
const outboundProviderHost = "https://api.stripe.com"

// validateOutboundHost enforces the S1 egress rule for every server-side
// outbound request this adapter issues: the scheme must be http/https and
// the host must not resolve to localhost, a loopback, a private, a
// link-local or another reserved range. It validates BEFORE the request is
// built, so a hostile configuration can never point the egress at internal
// infrastructure.
func validateOutboundHost(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("outbound url invalid")
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("outbound scheme %q not allowed", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("outbound host required")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return fmt.Errorf("outbound host %q not allowed", host)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A named host is allowed (SSRF hardening stops IP literals and
		// reserved names; a DNS name that later resolves internal is out of
		// scope for this fixed-configuration egress).
		return nil
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("outbound host %q not allowed", host)
	}
	// Reserved ranges net.IP does not classify (0.0.0.0/8, 100.64/10,
	// 198.18/15, 192.0.0/24, 192.0.2/24, 198.51.100/24, 203.0.113/24,
	// 240/4, 255.255.255.255) plus IPv4-mapped IPv6 forms.
	if v4 := ip.To4(); v4 != nil {
		for _, cidr := range []string{
			"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16",
			"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16",
			"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
			"100.64.0.0/10",
		} {
			_, network, _ := net.ParseCIDR(cidr)
			if network.Contains(v4) {
				return fmt.Errorf("outbound host %q not allowed", host)
			}
		}
		return nil
	}
	// Raw IPv6 reserved space (beyond IsLoopback/IsLinkLocal above).
	for _, cidr := range []string{"fc00::/7", "fe80::/10", "::/128", "::1/128"} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return fmt.Errorf("outbound host %q not allowed", host)
		}
	}
	return nil
}

// createPurchaseSubscription runs the payment-gated purchase algorithm:
// ensure the provider binding → identity read (explicit statuses) → found
// means replay-or-conflict (NEVER a re-POST, F7) → absent means ONE gated
// create whose 422 resolves by an identity re-read (never a second create).
func (a *LagoAdapter) createPurchaseSubscription(ctx context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := a.configured(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	payload, ok := cmd.Payload.(commercial.CreatePurchaseSubscriptionPayload)
	if !ok {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
	if err := payload.Validate(); err != nil {
		return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
	}
	ctx, cancel := context.WithTimeout(ctx, purchaseRequestTimeout)
	defer cancel()

	if err := a.ensureProviderBinding(ctx, payload.ExternalCustomerID); err != nil {
		return commercial.CommandReceipt{}, err
	}

	receipt := commercial.CommandReceipt{
		Key:        cmd.Key,
		ExternalID: payload.ExternalPurchaseSubscriptionID,
		RecordedAt: time.Now().UTC(),
	}
	sub, found, err := a.readSubscriptionByIdentity(ctx, payload.ExternalPurchaseSubscriptionID)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	if found {
		if sub.PlanCode != payload.PlanCode {
			// Concurrent plan change (AC4): a held purchase on a different
			// plan code is a definitive conflict — the late caller re-quotes.
			return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase plan conflict", commercial.ErrPlatformInvalidResponse)
		}
		// Identity replay: same purchase + same plan → the same receipt,
		// never a second subscription or a second gating invoice (F7).
		return receipt, nil
	}
	body := map[string]any{
		"subscription": map[string]any{
			"external_customer_id": payload.ExternalCustomerID,
			"external_id":          payload.ExternalPurchaseSubscriptionID,
			"plan_code":            payload.PlanCode,
			"name":                 "WeKnora Space " + strconv.FormatUint(payload.TenantID, 10) + " Purchase",
			// F1/F8: the payment gate with timeout 0 — the authority never
			// auto-cancels; cancellation timing stays with the coordinator.
			"activation_rules": []map[string]any{{"type": "payment", "timeout_hours": 0}},
		},
	}
	status, _, err := a.do(ctx, http.MethodPost, "/api/v1/subscriptions", body)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch {
	case status >= 200 && status < 300:
		return receipt, nil
	case status == http.StatusUnprocessableEntity:
		// The indistinguishable 422 (#76 lab fact, F7 race): resolve by an
		// identity re-read + plan-code compare — never a second create.
		sub, found, err := a.readSubscriptionByIdentity(ctx, payload.ExternalPurchaseSubscriptionID)
		if err != nil {
			return commercial.CommandReceipt{}, err
		}
		if found && sub.PlanCode == payload.PlanCode {
			return receipt, nil
		}
		return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase replay cannot be verified", commercial.ErrPlatformInvalidResponse)
	default:
		return commercial.CommandReceipt{}, classifySubscriptionStatus(status, "purchase create")
	}
}

// ensureProviderBinding guarantees the authority customer carries a
// provider binding before any gated create (D3): GET the customer — bound
// means done; otherwise derive the provider customer id from the provider
// API (key present) or the placeholder prefix, and upsert the binding.
// Unbound AND no source → ErrPlatformUnconfigured (fail closed).
func (a *LagoAdapter) ensureProviderBinding(ctx context.Context, externalCustomerID string) error {
	bound, err := a.customerProviderBound(ctx, externalCustomerID)
	if err != nil {
		return err
	}
	if bound {
		return nil
	}
	providerCustomerID, err := a.deriveProviderCustomerID(ctx, externalCustomerID)
	if err != nil {
		return err
	}
	body := map[string]any{
		"customer": map[string]any{
			"external_id": externalCustomerID,
			"name":        externalCustomerID,
			"billing_configuration": map[string]any{
				"payment_provider":     "stripe",
				"provider_customer_id": providerCustomerID,
			},
		},
	}
	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/customers", body)
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnprocessableEntity && strings.Contains(string(respBody), "payment_provider_not_found"):
		// F2: the org has no registered provider — configuration, not a
		// transient failure; fail closed and stop the purchase.
		return fmt.Errorf("%w: no provider registered", commercial.ErrPlatformUnconfigured)
	case status >= 500:
		return fmt.Errorf("%w: provider binding unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: provider binding rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// customerProviderBinding reads the customer and answers whether it already
// carries a provider customer id (the replay short-circuit: a bound
// customer is never re-POSTed).
func (a *LagoAdapter) customerProviderBound(ctx context.Context, externalCustomerID string) (bool, error) {
	status, body, err := a.do(ctx, http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(externalCustomerID), nil)
	if err != nil {
		return false, err
	}
	switch {
	case status == http.StatusNotFound:
		return false, nil
	case status >= 200 && status < 300:
	case status >= 500:
		return false, fmt.Errorf("%w: binding read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return false, fmt.Errorf("%w: binding read rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Customer struct {
			BillingConfiguration struct {
				ProviderCustomerID string `json:"provider_customer_id"`
			} `json:"billing_configuration"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, fmt.Errorf("%w: binding read malformed", commercial.ErrPlatformInvalidResponse)
	}
	return parsed.Customer.BillingConfiguration.ProviderCustomerID != "", nil
}

// deriveProviderCustomerID resolves the provider-side customer id: the
// provider API creates (idempotency-keyed) and returns its id when the key
// is configured; the placeholder prefix answers for dev/test stacks without
// one; neither source → ErrPlatformUnconfigured (fail closed, never a
// fabricated binding).
func (a *LagoAdapter) deriveProviderCustomerID(ctx context.Context, externalCustomerID string) (string, error) {
	if a.cfg.StripeAPIKey != "" {
		return a.providerCreateCustomer(ctx, externalCustomerID)
	}
	if a.cfg.ProviderCustomerPrefix != "" {
		return a.cfg.ProviderCustomerPrefix + "-" + externalCustomerID, nil
	}
	return "", fmt.Errorf("%w: no provider binding source", commercial.ErrPlatformUnconfigured)
}

// providerCreateCustomer issues the outbound provider customer create with
// the S1 host validation and the credential ONLY in the Authorization
// header (Basic scheme). The idempotency key is the WeKnora external
// customer id — a lost response replays to the same provider customer.
func (a *LagoAdapter) providerCreateCustomer(ctx context.Context, externalCustomerID string) (string, error) {
	base := a.cfg.StripeAPIBase
	if base == "" {
		base = outboundProviderHost
	}
	if err := validateOutboundHost(base); err != nil {
		return "", fmt.Errorf("%w: outbound host policy violation", commercial.ErrPlatformUnconfigured)
	}
	payload, _ := json.Marshal(map[string]any{
		"metadata": map[string]string{"weknora_customer": externalCustomerID},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(base, "/")+"/v1/customers", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("%w: outbound request invalid", commercial.ErrPlatformUnconfigured)
	}
	// The provider's API authenticates with the key as the Basic username.
	// The credential rides ONLY this header — never a URL, body, error or
	// log.
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.cfg.StripeAPIKey+":")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", externalCustomerID)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: provider customer create not reachable", commercial.ErrPlatformUnreachable)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 500 {
			return "", fmt.Errorf("%w: provider customer create unavailable", commercial.ErrPlatformUnreachable)
		}
		return "", fmt.Errorf("%w: provider customer create rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.ID == "" {
		return "", fmt.Errorf("%w: provider customer create malformed", commercial.ErrPlatformInvalidResponse)
	}
	return parsed.ID, nil
}
