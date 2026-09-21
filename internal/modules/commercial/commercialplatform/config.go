package commercialplatform

import (
	"fmt"
	"net/http"
	"os"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
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
// coexist until the OpenMeter removal ticket.
const (
	EnvProvider = "WEKNORA_COMMERCIAL_PLATFORM_PROVIDER"
	EnvBaseURL  = "WEKNORA_COMMERCIAL_PLATFORM_URL"
	EnvAPIKey   = "WEKNORA_COMMERCIAL_PLATFORM_API_KEY"
	EnvRelease  = "WEKNORA_COMMERCIAL_PLATFORM_RELEASE"
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
