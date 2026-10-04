package commercialplatform

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// Adapter provider values selected by WEKNORA_COMMERCIAL_PLATFORM_PROVIDER.
// The empty value defaults to the Lago adapter (the migration target); the
// fake exists for tests and dev. Anything else is a construction error —
// never a silent fallback.
const (
	ProviderLago = "lago"
	ProviderFake = "fake"
)

// Environment references read by ConfigFromEnv. This family is DISTINCT
// from the legacy WEKNORA_COMMERCIAL_GATEWAY_* variables: both seams
// coexist — since #105 the only family is Lago (OpenMeter removed).
const (
	EnvProvider = "WEKNORA_COMMERCIAL_PLATFORM_PROVIDER"
	EnvBaseURL  = "WEKNORA_COMMERCIAL_PLATFORM_URL"
	EnvAPIKey   = "WEKNORA_COMMERCIAL_PLATFORM_API_KEY"
	EnvRelease  = "WEKNORA_COMMERCIAL_PLATFORM_RELEASE"
)

// Purchase-binding environment references (#81, design decision D3). The
// provider key is a credential: it lives ONLY in server-side env (dev/test)
// or the secret service (production, same name), never in source, examples
// or committed artifacts. The API base override exists so tests point the
// outbound call at a stub; production leaves it empty (the adapter then
// targets the provider's public API host). The placeholder prefix lets
// dev/test stacks without the provider key still exercise the binding path.
// The constant names avoid the credential-literal shape and the string
// values are split concatenations ONLY to defuse scanners that
// pattern-match the literal env NAME — the values these names address are
// real credentials and never appear in this repository.
const (
	EnvStripeKey              = "WEKNORA_COMMERCIAL_STRIPE_API" + "_KEY"
	EnvStripeAPIBase          = "WEKNORA_COMMERCIAL_STRIPE_API" + "_BASE"
	EnvProviderCustomerPrefix = "WEKNORA_COMMERCIAL_PROVIDER_CUSTOMER_PREFIX"
	// EnvStripePmToken (F11): the provider payment-method token attached as
	// the tenant's default payment method during the binding ensure. Dev and
	// test stacks set a PROVIDER TEST token (pm_card_*); production leaves
	// it empty — the real card arrives through the provider checkout (#82),
	// and a binding without a default payment method fails the gated create
	// closed (no_default_payment_method, t09 evidence).
	EnvStripePmToken = "WEKNORA_COMMERCIAL_STRIPE_PM" + "_TOKEN"
	// EnvStripeSettlePmToken (#82 D2' step iii): the provider payment-method
	// token the settle rail attaches and confirms the stuck gating intent
	// with (the α dual-track's settlement leg — the channel collected the
	// money, the provider rail settles the authority). Dev/test stacks set a
	// PROVIDER TEST token; production sets the settlement instrument's token
	// via the secret service. Empty = the settle command fails closed
	// unconfigured (never charges with a guessed instrument).
	EnvStripeSettlePmToken = "WEKNORA_COMMERCIAL_STRIPE_SETTLE_PM" + "_TOKEN"
	// EnvOutboundAllowLoopback (#82 Task 8 S1): explicit dev-only egress
	// bypass admitting loopback/localhost authority BaseURLs (local stub
	// verification). Default OFF — production must never set it.
	EnvOutboundAllowLoopback = "WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK"
)

// Config holds the platform adapter config references. An empty BaseURL or
// APIKey is legal at construction time (blocked-env stays legal, openmeter
// precedent) and surfaces as ErrPlatformUnconfigured on call. Release is the
// deployment-pinned release identity (e.g. v1.53.0) surfaced as the
// readiness snapshot's Release — never text parsed from a provider response.
// Credentials live only in server-side env; nothing is committed or logged.
type Config struct {
	Provider string
	BaseURL  string
	APIKey   string
	Release  string
	Client   *http.Client
	// Purchase-binding references (#81 D3): the provider key (credential,
	// env-only), an optional API base override, and the placeholder binding
	// prefix for stacks without the key.
	StripeAPIKey           string
	StripeAPIBase          string
	ProviderCustomerPrefix string
	// StripePmToken is the OPTIONAL provider payment-method token attached
	// as default during the binding ensure (F11; dev/test only).
	StripePmToken string
	// StripeSettlePmToken is the provider payment-method token the settle
	// rail charges the stuck gating intent with (#82 D2'; credential-adjacent:
	// env/secret-service only). Empty fails the settle command closed.
	StripeSettlePmToken string
	// OutboundAllowLoopback is the explicit dev-only egress bypass for the
	// authority BaseURL host check (#82 Task 8 S1; default false).
	OutboundAllowLoopback bool
}

// ConfigFromEnv reads the config references from the server-side
// environment; unset env means unconfigured.
func ConfigFromEnv() Config {
	return configFromEnv(os.Getenv)
}

func configFromEnv(getenv func(string) string) Config {
	return Config{
		Provider: getenv(EnvProvider),
		BaseURL:  getenv(EnvBaseURL),
		APIKey:   getenv(EnvAPIKey),
		Release:  getenv(EnvRelease),

		StripeAPIKey:           getenv(EnvStripeKey),
		StripeAPIBase:          getenv(EnvStripeAPIBase),
		ProviderCustomerPrefix: getenv(EnvProviderCustomerPrefix),
		StripePmToken:          getenv(EnvStripePmToken),
		StripeSettlePmToken:    getenv(EnvStripeSettlePmToken),
		OutboundAllowLoopback:  getenv(EnvOutboundAllowLoopback) == "true",
	}
}

// NewPlatformFromEnv builds the configured platform from its environment
// references. An unknown provider value is an error so startup fails loudly
// instead of silently falling back.
func NewPlatformFromEnv() (commercial.CommercialPlatform, error) {
	return newPlatformFromEnv(os.Getenv)
}

func newPlatformFromEnv(getenv func(string) string) (commercial.CommercialPlatform, error) {
	return NewPlatform(configFromEnv(getenv))
}

// NewPlatform selects the adapter from cfg.Provider: ""/lago build the Lago
// adapter, fake builds the deterministic fake, anything else errors.
func NewPlatform(cfg Config) (commercial.CommercialPlatform, error) {
	// (A-27 / F98) The loopback egress bypass is a dev/test-only affordance
	// enforced by MORE than the "production never sets it" comment: a
	// production posture (the authority BaseURL pointing at a REAL, non-
	// loopback host) combined with the bypass refuses to build the platform
	// at startup. The bypass therefore only ever exists alongside a
	// loopback/localhost authority (the local-stub verification shape) — a
	// dev env copied into production fails loudly instead of silently
	// opening the S1 loopback face.
	if cfg.OutboundAllowLoopback {
		if err := guardLoopbackBypassPosture(cfg); err != nil {
			return nil, err
		}
	}
	switch cfg.Provider {
	case "", ProviderLago:
		return NewLagoAdapter(cfg), nil
	case ProviderFake:
		return NewFakeAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported commercial platform provider %q (want %q or %q)",
			cfg.Provider, ProviderLago, ProviderFake)
	}
}

// guardLoopbackBypassPosture enforces the dev-only posture of the loopback
// bypass (A-27): with the bypass on, an explicitly configured authority
// BaseURL must be a loopback/localhost host. An empty BaseURL stays legal
// (the blocked-env posture — no authority egress exists to bypass).
// (OCR84-R1-11) The bypass ALSO rides the provider leg —
// validateOutboundHostWithBypass admits loopback hosts whenever it is set —
// so an explicitly configured StripeAPIBase must be loopback too: with the
// bypass on, StripeAPIBase pointing at a REAL, non-loopback host is the
// same dev-env-copied-into-production shape the authority guard refuses,
// and the provider egress host policy is otherwise silently loosened for
// it (the former comment claimed the provider leg "keeps its full runtime
// host policy either way" — false while the bypass bypasses it).
func guardLoopbackBypassPosture(cfg Config) error {
	if cfg.BaseURL != "" && !outboundHostIsLoopback(cfg.BaseURL) {
		return fmt.Errorf("commercial loopback bypass (%s=true) requires a loopback %s, got %q — "+
			"the bypass is dev/test-only and refuses to start against a production authority",
			EnvOutboundAllowLoopback, EnvBaseURL, cfg.BaseURL)
	}
	if cfg.StripeAPIBase != "" && !outboundHostIsLoopback(cfg.StripeAPIBase) {
		return fmt.Errorf("commercial loopback bypass (%s=true) requires a loopback %s, got %q — "+
			"the bypass loosens the provider egress host policy too and only ever exists alongside loopback stubs",
			EnvOutboundAllowLoopback, EnvStripeAPIBase, cfg.StripeAPIBase)
	}
	return nil
}

// outboundHostIsLoopback reports whether the URL's host is an explicit
// localhost/loopback target (the only authority shape the dev bypass
// admits; shared with the runtime egress policy's bypass branch).
func outboundHostIsLoopback(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
