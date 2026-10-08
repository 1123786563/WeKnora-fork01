package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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

// purchaseRequestTimeout bounds one create_purchase_subscription command.
// (R1-12) The budget must cover the whole chain it actually runs: the
// binding read/write, up to THREE provider round trips (customer create,
// attach, set-default, each bounded by outboundProviderTimeout), the
// bounded payment-method import wait (pmSyncWait) and the identity read +
// gated create (subscriptionRequestTimeout). A budget smaller than the sum
// starves the import poll — the outer deadline expires mid-poll and the
// transport layer wraps it as an unreachable. Computed at CALL time (not
// package init) so tests that shorten pmSyncWait keep the budget
// self-consistent.
func purchaseRequestTimeout() time.Duration {
	return subscriptionRequestTimeout + 3*outboundProviderTimeout + pmSyncWait
}

// pmSyncWait bounds the poll for the authority's import of the tenant's
// default payment method (F11: the gated create answers
// no_default_payment_method until the authority's worker has pulled the
// provider-side payment method; t09/t02 run evidence).
var pmSyncWait = 20 * time.Second

// pmSyncTick is the poll cadence for the payment-method import.
var pmSyncTick = 500 * time.Millisecond

// outboundProviderHost is the default host of the provider's public API.
const outboundProviderHost = "https://api.stripe.com"

// paymentProviderCode is the registration code shared by the payment
// binding payload and the webhook route/database lookup.
const paymentProviderCode = "weknora-stripe"

// outboundProviderTimeout bounds ONE provider API round trip. It is
// deliberately NOT the shared Lago client's health-grade 5 s budget: a
// cross-continent TLS+HTTP round trip to the provider's public endpoint
// routinely exceeds it (t09 run evidence: ~3.5 s), so the outbound call
// rides its own client, never the Lago one.
const outboundProviderTimeout = 15 * time.Second

// validateOutboundHost enforces the S1 egress rule for every server-side
// outbound request this adapter issues: the scheme must be http/https and
// the host must not resolve to localhost, a loopback, a private, a
// link-local or another reserved range. It validates BEFORE the request is
// built, so a hostile configuration can never point the egress at internal
// infrastructure.
// validateOutboundHostWithBypass is the S1 check with the explicit dev-only
// loopback exemption (#82 Task 8): when allowed, ONLY loopback/localhost
// hosts skip the reserved-range refusal (local stub verification); every
// other host still runs the full policy. Production never sets the bypass.
func validateOutboundHostWithBypass(rawURL string, allowLoopback bool) error {
	if !allowLoopback {
		return validateOutboundHost(rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("outbound url invalid")
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("outbound scheme %q not allowed", parsed.Scheme)
	}
	// (OCR84-R1-11) The loopback admission reuses outboundHostIsLoopback —
	// ONE shared loopback-shape implementation, the same one the startup
	// posture guard uses; two copies of the shape would drift apart.
	if outboundHostIsLoopback(rawURL) {
		return nil
	}
	return validateOutboundHost(rawURL)
}

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
		// (R1-V08) net.ParseIP rejects the inet_aton shorthand forms
		// ("127.1", "2130706433", "0x7f000001") that some system resolvers
		// still answer with loopback/private addresses at dial time. A host
		// that is neither a ParseIP literal nor a plausible FQDN (letters
		// required — RFC 1123 forbids numeric-only names) is refused.
		if !isPlausibleHostname(host) {
			return fmt.Errorf("outbound host %q not allowed", host)
		}
		// A named host is otherwise allowed (SSRF hardening stops IP
		// literals and reserved names; a DNS name that later resolves
		// internal is out of scope for this fixed-configuration egress).
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

// isPlausibleHostname reports whether host has the shape of a legal RFC
// 1123 FQDN: dot-separated labels of alphanumerics/hyphens (no leading or
// trailing hyphen, no empty label) whose TOP-LEVEL label starts with a
// letter — every real TLD does (alphabetic or xn-- punycode), while the
// inet_aton shorthand shapes ("127.1", "2130706433", "0x7f000001",
// "0177.0.0.1") all end in a digit-led label and are refused here even
// though net.ParseIP rejects them as literals too.
func isPlausibleHostname(host string) bool {
	if host == "" {
		return false
	}
	labels := strings.Split(strings.ToLower(host), ".")
	for _, label := range labels {
		if label == "" {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	if len(labels) < 2 {
		// (OCR r4) A single-label name is not an FQDN — intranet-style
		// single labels refuse here rather than riding the named-host
		// allowance.
		return false
	}
	tld := labels[len(labels)-1]
	return tld[0] >= 'a' && tld[0] <= 'z'
}

// providerOutboundClient returns the egress-hardened provider client
// (R1-V08): redirects are NOT followed blindly — every hop's target URL is
// re-validated against the same S1 host policy, so a hostile/compromised
// endpoint cannot 302 the egress onto internal infrastructure. Crossing the
// redirect policy answers a transport-class error (unreachable), which the
// caller classifies like any other failed round trip.
func providerOutboundClient() *http.Client {
	return &http.Client{
		Timeout: outboundProviderTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateOutboundHost(req.URL.String()); err != nil {
				return fmt.Errorf("outbound redirect host policy violation: %w", err)
			}
			if len(via) >= 10 {
				return errors.New("outbound redirect limit exceeded")
			}
			return nil
		},
	}
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
	ctx, cancel := context.WithTimeout(ctx, purchaseRequestTimeout())
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
		// (OCR final audit 7) A TERMINAL held purchase never answers a
		// success receipt: a canceled/terminated identity would mint a dead
		// checkout link the channel can never settle. active/incomplete on
		// the SAME plan keep the R1-V03 verbatim-replay semantics.
		switch sub.Status {
		case "canceled", "terminated":
			return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase subscription terminal", commercial.ErrPlatformInvalidResponse)
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
	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/subscriptions", body)
	if err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch {
	case status >= 200 && status < 300:
		return receipt, nil
	case status == http.StatusUnprocessableEntity &&
		strings.Contains(string(respBody), "no_default_payment_method"):
		// (OCR r4) STRUCTURAL posture first: with no provider key or no
		// payment-method token configured, no import can EVER land — the
		// retryable classification would loop forever on a wiring gap, so
		// the adapter fails closed unconfigured instead.
		if a.cfg.StripeAPIKey == "" || a.cfg.StripePmToken == "" {
			return commercial.CommandReceipt{}, fmt.Errorf(
				"%w: no settle-able default payment method without provider key/pm token",
				commercial.ErrPlatformUnconfigured)
		}
		// (A-34 / F118) A configured PROCESS does not prove THIS customer
		// ever got a default payment method: a binding created while the
		// token was empty (production posture skips attach/sync) leaves the
		// customer bound but PM-less, and ensureProviderBinding's bound
		// short-circuit never re-runs the attach after the operator adds
		// the token — the 422 would then loop forever mislabeled as a
		// transient unreachable. Re-drive the attach HERE (idempotent:
		// re-attaching the same pm and re-setting the same default are
		// no-ops on the provider), then re-sync the import, and only then
		// answer the retryable sentinel — the next attempt's create
		// succeeds because the import now has something to land. Both legs
		// ride the injected seams (the established test pattern) so a
		// bound-but-PM-less customer is drivable under stubs.
		pcid, cerr := a.boundProviderCustomerID(ctx, payload.ExternalCustomerID)
		if cerr != nil {
			return commercial.CommandReceipt{}, cerr
		}
		if aerr := a.reAttachDefaultPM(ctx, pcid, a.cfg.StripePmToken); aerr != nil {
			return commercial.CommandReceipt{}, aerr
		}
		if serr := a.syncPaymentMethods(ctx, payload.ExternalCustomerID); serr != nil {
			return commercial.CommandReceipt{}, serr
		}
		return commercial.CommandReceipt{}, fmt.Errorf(
			"%w: default payment method not imported yet", commercial.ErrPlatformUnreachable)
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

// providerCustomerSource names where the provider-side customer id came
// from: the provider API itself (a real provider customer exists) or the
// dev/test placeholder prefix (NO provider customer exists).
type providerCustomerSource string

const (
	providerCustomerAPI         providerCustomerSource = "provider_api"
	providerCustomerPlaceholder providerCustomerSource = "placeholder_prefix"
)

// ensureProviderBinding guarantees the authority customer carries a
// provider binding before any gated create (D3): GET the customer — bound
// means done; otherwise derive the provider customer id from the provider
// API (key present) or the placeholder prefix and write ONLY the binding
// through the customers collection POST, whose upsert semantics
// (Customers::UpsertFromApiService) apply billing_configuration to an
// EXISTING customer and overwrite a field only when its key is present —
// so the onboarding display name survives because no name key is sent
// (R1-V05 intent, restated on the pinned v1.53.0 contract: the shared API
// exposes customers as create/index/show/destroy ONLY — there is no update
// route, and PUT /api/v1/customers/:external_id 404s; issue #82 flow
// defect 3 made every existing-but-unbound customer's first purchase die
// 503 invalid_response there). An ABSENT customer is created (with its
// name). Unbound AND no source → ErrPlatformUnconfigured (fail closed).
func (a *LagoAdapter) ensureProviderBinding(ctx context.Context, externalCustomerID string) error {
	exists, bound, err := a.customerProviderBound(ctx, externalCustomerID)
	if err != nil {
		return err
	}
	if bound {
		return nil
	}
	providerCustomerID, source, err := a.deriveProviderCustomer(ctx, externalCustomerID)
	if err != nil {
		return err
	}
	// F11 contract shape (t02/t09 evidence): the linkage carries the
	// provider, its registration code and the card payment-method class —
	// the gated create then requires the authority's synced default
	// payment method, polled below.
	billing := map[string]any{
		"payment_provider":         "stripe",
		"payment_provider_code":    paymentProviderCode,
		"provider_customer_id":     providerCustomerID,
		"provider_payment_methods": []string{"card"},
	}
	// One collection POST serves both branches (pinned v1.53.0 upsert):
	// absent → create (name required); existing → update-only — the body
	// deliberately carries NO name key so the onboarding display name is
	// never overwritten. The bound short-circuit above keeps the replay
	// path from ever re-POSTing.
	body := map[string]any{
		"customer": map[string]any{
			"external_id":           externalCustomerID,
			"billing_configuration": billing,
		},
	}
	if !exists {
		body["customer"].(map[string]any)["name"] = externalCustomerID
	}
	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/customers", body)
	if err != nil {
		return err
	}
	switch {
	case status >= 200 && status < 300:
		// F11: the authority imports the provider payment method
		// asynchronously; the gated create is refused with
		// no_default_payment_method until the import lands. Poll bounded —
		// but ONLY when the binding references a REAL provider customer
		// (R1-V06): a placeholder-prefix id has none, the import can never
		// land, and polling would burn the whole pmSyncWait on every
		// purchase before answering the wrong transient sentinel. The
		// placeholder binding fails fast at the gated create instead.
		if source == providerCustomerPlaceholder {
			return nil
		}
		// (R1-24) The same stall has a second source in the production
		// posture: StripePmToken empty means NO default payment method was
		// attached during the provider customer create (the real card arrives
		// through the provider checkout, #82) — the import can never land
		// either, and polling would burn the whole pmSyncWait on every
		// purchase before answering the wrong transient unreachable. The
		// contract (config.go F11 note, providerCreateCustomer) promises the
		// opposite: a binding without a default PM reaches the gated create
		// and fails CLOSED there (no_default_payment_method).
		if a.cfg.StripePmToken == "" {
			return nil
		}
		return a.syncPaymentMethods(ctx, externalCustomerID)
	case status == http.StatusUnprocessableEntity && strings.Contains(string(respBody), "payment_provider_not_found"):
		// F2: the org has no registered provider — configuration, not a
		// transient failure; fail closed and stop the purchase.
		return fmt.Errorf("%w: no provider registered", commercial.ErrPlatformUnconfigured)
	case status == http.StatusTooManyRequests || status >= 500:
		return fmt.Errorf("%w: provider binding unavailable", commercial.ErrPlatformUnreachable)
	default:
		return fmt.Errorf("%w: provider binding rejected", commercial.ErrPlatformInvalidResponse)
	}
}

// waitForPaymentMethodSync polls the authority's payment-method list for the
// tenant until the provider-side default payment method has been imported
// (F11, bounded). An exhausted budget is the closed indeterminate
// unreachable — the purchase replays safely by identity.
func (a *LagoAdapter) waitForPaymentMethodSync(ctx context.Context, externalCustomerID string) error {
	// (OCR final audit 6) The poll budget derives from the CALLER's
	// remaining deadline: min(pmSyncWait, ctx budget - 1s guard), so a
	// request already near its budget never nominally polls past the
	// caller's own cancellation.
	deadline := time.Now().Add(pmSyncWait)
	if d, ok := ctx.Deadline(); ok {
		if guarded := d.Add(-time.Second); guarded.Before(deadline) {
			deadline = guarded
		}
	}
	for {
		status, body, err := a.do(ctx, http.MethodGet,
			"/api/v1/customers/"+url.PathEscape(externalCustomerID)+"/payment_methods", nil)
		if err != nil {
			return err
		}
		switch {
		case status >= 200 && status < 300:
			var parsed struct {
				PaymentMethods []json.RawMessage `json:"payment_methods"`
			}
			if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.PaymentMethods) > 0 {
				return nil
			}
		case status == http.StatusTooManyRequests || status >= 500:
			return fmt.Errorf("%w: payment method read unavailable", commercial.ErrPlatformUnreachable)
		default:
			return fmt.Errorf("%w: payment method read rejected", commercial.ErrPlatformInvalidResponse)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: default payment method not imported in time", commercial.ErrPlatformUnreachable)
		}
		if err := sleepCtx(ctx, pmSyncTick); err != nil {
			// (R1-24) A cancelled caller context (client disconnect, command
			// budget deadline) is the caller's own end — it is NOT a platform
			// unreachability and must not carry that sentinel (a retrying
			// coordinator would otherwise re-enter a poll it just cancelled).
			// Surface ctx.Err() on its own error chain.
			return fmt.Errorf("payment method sync interrupted: %w", err)
		}
	}
}

// customerProviderBinding reads the customer and reports whether it exists
// at all and whether it already carries a provider customer id (the replay
// short-circuit: a bound customer is never rewritten). The exists/bound
// split is what lets the binding write choose create vs update-only
// (R1-V05).
func (a *LagoAdapter) customerProviderBound(ctx context.Context, externalCustomerID string) (bool, bool, error) {
	status, body, err := a.do(ctx, http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(externalCustomerID), nil)
	if err != nil {
		return false, false, err
	}
	switch {
	case status == http.StatusNotFound:
		return false, false, nil
	case status >= 200 && status < 300:
	case status == http.StatusTooManyRequests || status >= 500:
		return false, false, fmt.Errorf("%w: binding read unavailable", commercial.ErrPlatformUnreachable)
	default:
		return false, false, fmt.Errorf("%w: binding read rejected", commercial.ErrPlatformInvalidResponse)
	}
	var parsed struct {
		Customer struct {
			BillingConfiguration struct {
				ProviderCustomerID string `json:"provider_customer_id"`
			} `json:"billing_configuration"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, false, fmt.Errorf("%w: binding read malformed", commercial.ErrPlatformInvalidResponse)
	}
	bound := parsed.Customer.BillingConfiguration.ProviderCustomerID != ""
	return true, bound, nil
}

// readPurchaseSnapshot answers the purchase truth for one tenant: the
// payment-gated subscription state and its frozen price face, read by
// identity with the EXPLICIT status set (the index defaults to active —
// F6). Unknown provider statuses fail closed invalid response; the
// open-stage invoice lines are NEVER fabricated (F3-F5) — they stay empty
// until the finalized-stage read fills them (#82/#84 interface, D2
// condition 3).
func (a *LagoAdapter) readPurchaseSnapshot(ctx context.Context, tenantID uint64) (commercial.Snapshot, error) {
	if err := a.configured(); err != nil {
		return commercial.Snapshot{}, err
	}
	if tenantID == 0 {
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, subscriptionRequestTimeout)
	defer cancel()

	p := &commercial.PurchaseSnapshot{
		TenantID:  tenantID,
		State:     commercial.PurchaseStateAbsent,
		CheckedAt: time.Now().UTC(),
	}
	sub, found, err := a.readSubscriptionByIdentity(ctx, commercial.ExternalPurchaseSubscriptionID(tenantID))
	if err != nil {
		return commercial.Snapshot{}, err
	}
	if found {
		switch sub.Status {
		case "incomplete":
			p.State = commercial.PurchaseStateAwaitingPayment
		case "active":
			p.State = commercial.PurchaseStateActive
		case "canceled", "terminated":
			p.State = commercial.PurchaseStateCanceled
		default:
			return commercial.Snapshot{}, fmt.Errorf("%w: unknown purchase status", commercial.ErrPlatformInvalidResponse)
		}
		p.PlanCode = sub.PlanCode
		p.Currency = sub.PlanAmountCurrency
		if sub.PlanAmountCents != "" {
			amount, err := sub.PlanAmountCents.Int64()
			if err != nil {
				return commercial.Snapshot{}, fmt.Errorf("%w: purchase amount not an integer", commercial.ErrPlatformInvalidResponse)
			}
			p.AmountFen = amount
		}
		if p.State == commercial.PurchaseStateActive {
			// Finalized-stage interface (D2 condition 3): the open stage
			// answers empty (never fabricated); #82 completes the real
			// finalized-invoice read.
			fees, paymentStatus, err := a.readPurchaseInvoiceFees(ctx, tenantID)
			if err != nil {
				return commercial.Snapshot{}, err
			}
			p.InvoiceFees = fees
			p.InvoicePaymentStatus = paymentStatus
		}
	}
	return commercial.Snapshot{Kind: commercial.SnapshotKindPurchase, Purchase: p}, nil
}

// readPurchaseInvoiceFees is the finalized-stage invoice line read (#82
// interface, D2 condition 3 / D6' review input): once the gating payment
// finalizes the invoice (a VISIBLE status), the adapter fills the lines
// authoritatively from the subscription fee rows and reports the invoice's
// payment_status. During the awaiting-payment stage this answers empty —
// the pinned authority version keeps open invoices invisible to every API
// path (t09 evidence F3-F5) and an invisible line is never fabricated.
//
// Location predicate (F-5 flow evidence): the customer's finalized index
// lists EVERY finalized subscription invoice — the Base plan's monthly
// 0-amount invoices included — and the pinned v1.53 exposes NO subscription
// filter on the index (both candidate params answered the full set on the
// live stack). The PURCHASE gating invoice is therefore identified on the
// single-invoice read by its fee lines' subscription identity
// (fees→subscription: fee.external_subscription_id), never by the index row
// count.
//
// Selection rule (A-24 / F95, D13' explicit ordering / r2:624): MORE THAN
// ONE finalized invoice may legitimately carry a purchase subscription fee
// — the authority's own recurring billing issues a renewal invoice (same
// external_subscription_id) at every monthly boundary of the purchase
// subscription, and a repurchase reuses the same identity. The GATING
// answer the callers need is: the NEWEST invoice whose payment_status is
// "succeeded" (an active purchase necessarily settled its gating invoice;
// a later still-open renewal never shadows it), falling back to the newest
// match of any payment_status. (r2:624, correctness half) "Newest" is
// ranked by the invoice detail's OWN created_at, EXPLICITLY compared —
// never the index row order (an unrequested server-side sort convention
// the read must not depend on). The ranking therefore detail-reads every
// index row; the N+1 shape stays disclosed (R-23 — read path, not hot).
type purchaseInvoiceMatch struct {
	createdAt     time.Time
	lines         []commercial.InvoiceLineSnapshot
	paymentStatus string
}

func (a *LagoAdapter) readPurchaseInvoiceFees(ctx context.Context, tenantID uint64) ([]commercial.InvoiceLineSnapshot, string, error) {
	purchaseSubscriptionID := commercial.ExternalPurchaseSubscriptionID(tenantID)
	ids, err := a.finalizedInvoiceIDs(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	var bestSucceeded, bestAny *purchaseInvoiceMatch
	for _, id := range ids {
		// …read the single invoice (t9 integration evidence: the pinned
		// runtime's INDEX answer carries no fees; only the single-invoice
		// read does — the same shape Lago's own front consumes).
		status, body, err := a.do(ctx, http.MethodGet, "/api/v1/invoices/"+url.PathEscape(id), nil)
		if err != nil {
			return nil, "", err
		}
		// (A-25 / F96) A transient authority failure (429/5xx) is
		// unreachable — retryable — never the definitive invalid-response
		// sentinel that would park a settle replay in attention.
		if status == http.StatusTooManyRequests || status >= 500 {
			return nil, "", fmt.Errorf("%w: invoice read unavailable (HTTP %d)", commercial.ErrPlatformUnreachable, status)
		}
		if status != http.StatusOK {
			return nil, "", fmt.Errorf("%w: invoice read answered HTTP %d", commercial.ErrPlatformInvalidResponse, status)
		}
		var parsed struct {
			Invoice struct {
				PaymentStatus string `json:"payment_status"`
				CreatedAt     string `json:"created_at"`
				Fees          []struct {
					AmountCents int64 `json:"amount_cents"` // integer minor units, never float (GC-6)
					Item        struct {
						Type string `json:"type"`
						Name string `json:"name"`
					} `json:"item"`
					ExternalSubscriptionID string `json:"external_subscription_id"`
				} `json:"fees"`
			} `json:"invoice"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, "", fmt.Errorf("%w: invoice body malformed", commercial.ErrPlatformInvalidResponse)
		}
		lines := make([]commercial.InvoiceLineSnapshot, 0, len(parsed.Invoice.Fees))
		for _, fee := range parsed.Invoice.Fees {
			if fee.Item.Type != "subscription" || fee.ExternalSubscriptionID != purchaseSubscriptionID {
				continue // a Base (or foreign) subscription's line — never the purchase gate
			}
			lines = append(lines, commercial.InvoiceLineSnapshot{
				Kind:      "subscription_fee", // the closed port word, never the provider's raw type
				Name:      fee.Item.Name,
				AmountFen: fee.AmountCents,
			})
		}
		if len(lines) == 0 {
			continue // this finalized invoice bills no purchase subscription fee
		}
		// (r2:624) A matching row the selection cannot rank is a malformed
		// authority shape — the unparsable row may be the NEWEST — so the
		// read fails closed instead of silently guessing.
		createdAt, cerr := time.Parse(time.RFC3339, parsed.Invoice.CreatedAt)
		if cerr != nil {
			return nil, "", fmt.Errorf("%w: invoice created_at unparsable", commercial.ErrPlatformInvalidResponse)
		}
		match := &purchaseInvoiceMatch{createdAt: createdAt, lines: lines, paymentStatus: parsed.Invoice.PaymentStatus}
		if match.paymentStatus == "succeeded" {
			// The newest succeeded purchase invoice IS the gating answer.
			if bestSucceeded == nil || createdAt.After(bestSucceeded.createdAt) {
				bestSucceeded = match
			}
		}
		if bestAny == nil || createdAt.After(bestAny.createdAt) {
			bestAny = match
		}
	}
	if bestSucceeded != nil {
		return bestSucceeded.lines, bestSucceeded.paymentStatus, nil
	}
	if bestAny != nil {
		return bestAny.lines, bestAny.paymentStatus, nil
	}
	return nil, "", nil // awaiting semantics: no finalized purchase invoice visible yet
}

// finalizedInvoiceIDs pages through the customer's VISIBLE finalized
// invoice index (newest first per the authority's convention) and answers
// the lago_ids. The page budget bounds a runaway iteration (a tenant with
// a very long invoice history) — exhausting it fails closed instead of
// silently truncating.
func (a *LagoAdapter) finalizedInvoiceIDs(ctx context.Context, tenantID uint64) ([]string, error) {
	const (
		perPage    = 100
		pageBudget = 10 // 1000 finalized invoices — beyond is an anomaly, never silent
	)
	ids := make([]string, 0, 8)
	for page := 1; page <= pageBudget; page++ {
		status, body, err := a.do(ctx, http.MethodGet,
			"/api/v1/invoices?external_customer_id="+url.PathEscape(commercial.ExternalCustomerID(tenantID))+
				"&status[]=finalized&per_page="+strconv.Itoa(perPage)+"&page="+strconv.Itoa(page), nil)
		if err != nil {
			return nil, err
		}
		// (A-25 / F96) The index read follows the same transient/definitive
		// split as every other read path in this file: a 429/5xx is a
		// retryable unreachability, never the definitive sentinel.
		if status == http.StatusTooManyRequests || status >= 500 {
			return nil, fmt.Errorf("%w: invoice index unavailable (HTTP %d)", commercial.ErrPlatformUnreachable, status)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%w: invoice index answered HTTP %d", commercial.ErrPlatformInvalidResponse, status)
		}
		var index struct {
			Invoices []struct {
				LagoID string `json:"lago_id"`
			} `json:"invoices"`
			Meta struct {
				NextPage *int `json:"next_page"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &index); err != nil {
			return nil, fmt.Errorf("%w: invoice index body malformed", commercial.ErrPlatformInvalidResponse)
		}
		for _, inv := range index.Invoices {
			if inv.LagoID != "" {
				ids = append(ids, inv.LagoID)
			}
		}
		if index.Meta.NextPage == nil {
			return ids, nil
		}
	}
	return nil, fmt.Errorf("%w: finalized invoice index exceeded the page budget", commercial.ErrPlatformInvalidResponse)
}

// deriveProviderCustomerID resolves the provider-side customer id AND its
// source (R1-V06): the provider API creates (idempotency-keyed) and
// returns its id when the key is configured; the placeholder prefix
// answers for dev/test stacks without one — the source lets the caller
// skip the payment-method import poll a placeholder can never satisfy.
// Neither source → ErrPlatformUnconfigured (fail closed, never a
// fabricated binding).
func (a *LagoAdapter) deriveProviderCustomerID(ctx context.Context, externalCustomerID string) (string, providerCustomerSource, error) {
	if a.cfg.StripeAPIKey != "" {
		id, err := a.providerCreateCustomer(ctx, externalCustomerID)
		return id, providerCustomerAPI, err
	}
	if a.cfg.ProviderCustomerPrefix != "" {
		return a.cfg.ProviderCustomerPrefix + "-" + externalCustomerID, providerCustomerPlaceholder, nil
	}
	return "", "", fmt.Errorf("%w: no provider binding source", commercial.ErrPlatformUnconfigured)
}

// providerCreateCustomer issues the outbound provider customer create with
// the S1 host validation and the credential ONLY in the Authorization
// header (Basic scheme). The idempotency key is the WeKnora external
// customer id — a lost response replays to the same provider customer.
func (a *LagoAdapter) providerCreateCustomer(ctx context.Context, externalCustomerID string) (string, error) {
	status, data, err := a.providerOutboundCall(ctx, "/v1/customers",
		"metadata[weknora_customer]="+url.QueryEscape(externalCustomerID), externalCustomerID)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		if status >= 500 {
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
	// F11 (dev/test stacks): a configured payment-method token is attached
	// and set as the customer's default so the authority can import it and
	// the gated create can pass. Production leaves the token empty — the
	// real card arrives through the provider checkout (#82/#83) and a
	// binding without a default payment method fails closed at the gated
	// create.
	if a.cfg.StripePmToken != "" {
		// (R1-V07) The caller's request-scoped ctx rides along so the attach
		// stops when the command budget is already over — the two outbound
		// calls then run under the SHORTER of the caller's deadline and
		// their own outboundProviderTimeout.
		if err := a.providerAttachDefaultPaymentMethod(ctx, parsed.ID, a.cfg.StripePmToken); err != nil {
			return "", err
		}
	}
	return parsed.ID, nil
}

// providerOutboundCall issues one pre-validated POST to the provider API
// (S1 host check inside) with the credential ONLY in the Authorization
// header. A thin wrapper over providerOutboundRequest (the settle rail
// generalized the method; every existing caller stays POST-only).
func (a *LagoAdapter) providerOutboundCall(ctx context.Context, path, form string, idempotencyKey string) (int, []byte, error) {
	return a.providerOutboundRequest(ctx, http.MethodPost, path, form, idempotencyKey)
}

// providerAttachDefaultPaymentMethod attaches the payment-method token to
// the provider customer and promotes it to the default (F11). The token is
// a provider payment-method id (test mode pre-creates pm_card_*); the
// provider CLONES it on attach and answers a customer-scoped pm_ id — the
// default update must reference THAT id (the t02 evidence). ctx is the
// caller's command deadline (R1-V07): each outbound call runs under the
// shorter of that deadline and outboundProviderTimeout, never an
// unbounded Background-derived budget.
func (a *LagoAdapter) providerAttachDefaultPaymentMethod(ctx context.Context, providerCustomerID, pmToken string) error {
	ctx, cancel := context.WithTimeout(ctx, outboundProviderTimeout)
	defer cancel()
	status, data, err := a.providerOutboundCall(ctx, "/v1/payment_methods/"+url.PathEscape(pmToken)+"/attach",
		"customer="+url.QueryEscape(providerCustomerID), "")
	if err != nil {
		return err
	}
	// (R1-V20) The adapter's fail-closed contract (lago.go precedent): a
	// provider 5xx is transient unavailability (ErrPlatformUnreachable) —
	// folding it into the terminal invalid-response sentinel would stop
	// callers from ever retrying a Stripe 503.
	if status >= 500 {
		return fmt.Errorf("%w: provider payment method attach unavailable", commercial.ErrPlatformUnreachable)
	}
	var attached struct {
		ID string `json:"id"`
	}
	if status < 200 || status >= 300 || json.Unmarshal(data, &attached) != nil || attached.ID == "" {
		return fmt.Errorf("%w: provider payment method attach rejected", commercial.ErrPlatformInvalidResponse)
	}
	status, _, err = a.providerOutboundCall(ctx, "/v1/customers/"+url.PathEscape(providerCustomerID),
		"invoice_settings[default_payment_method]="+url.QueryEscape(attached.ID), "")
	if err != nil {
		return err
	}
	if status >= 500 {
		return fmt.Errorf("%w: provider default payment method update unavailable", commercial.ErrPlatformUnreachable)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("%w: provider default payment method update rejected", commercial.ErrPlatformInvalidResponse)
	}
	return nil
}
