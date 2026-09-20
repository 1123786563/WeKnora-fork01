package commercialplatform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// healthStub stands in for the Lago health signal. #73 probe fact: /health
// is an unauthenticated liveness signal — version identity is deployment
// config, NEVER text parsed from the response — so the stub records whether
// any credential rode on the request.
type healthStub struct {
	mu     sync.Mutex
	status int
	paths  []string
	auths  []string
	server *httptest.Server
}

func newHealthStub(t *testing.T, status int) *healthStub {
	t.Helper()
	h := &healthStub{status: status}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.paths = append(h.paths, r.URL.Path)
		h.auths = append(h.auths, r.Header.Get("Authorization"))
		h.mu.Unlock()
		w.WriteHeader(h.status)
	})
	h.server = httptest.NewServer(mux)
	t.Cleanup(h.server.Close)
	return h
}

func (h *healthStub) recordedPaths() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.paths...)
}

func (h *healthStub) recordedAuth() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.auths...)
}

func (h *healthStub) url() string { return h.server.URL }

const testAPIKey = "secret-for-test-only"

func lagoTestConfig(baseURL string) Config {
	return Config{Provider: ProviderLago, BaseURL: baseURL, APIKey: testAPIKey, Release: "v1.53.0"}
}

// TestLagoAdapterReadinessFromHealthSignal: a 2xx /health answers a ready
// snapshot whose Release is the CONFIGURED deployment pin (not response
// text); the request goes to <base>/health and carries no credential (the
// liveness signal is unauthenticated, #73 probe fact).
func TestLagoAdapterReadinessFromHealthSignal(t *testing.T) {
	stub := newHealthStub(t, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if snap.Kind != commercial.SnapshotKindReadiness || snap.Readiness == nil {
		t.Fatalf("snapshot shape mismatch: %+v", snap)
	}
	if snap.Readiness.State != commercial.ReadinessReady {
		t.Fatalf("state = %q, want ready", snap.Readiness.State)
	}
	if snap.Readiness.Release != "v1.53.0" {
		t.Fatalf("release = %q, want the configured deployment pin v1.53.0", snap.Readiness.Release)
	}
	if snap.Readiness.CheckedAt.IsZero() || time.Since(snap.Readiness.CheckedAt) > time.Minute {
		t.Fatalf("CheckedAt must be the check time, got %v", snap.Readiness.CheckedAt)
	}
	if snap.Readiness.Reason != "" {
		t.Fatalf("ready snapshot must carry an empty reason, got %q", snap.Readiness.Reason)
	}
	paths := stub.recordedPaths()
	if len(paths) != 1 || paths[0] != "/health" {
		t.Fatalf("the adapter must GET <base>/health exactly once, got %v", paths)
	}
	for _, auth := range stub.recordedAuth() {
		if auth != "" {
			t.Fatalf("the unauthenticated health signal must not carry credentials, got %q", auth)
		}
	}
}

// TestLagoAdapterHealthFailuresAreUnreachableNeverReady: 5xx, a refused
// connection and an expired context are all unreachable — the adapter never
// fabricates a ready state — and none of the error strings leaks the key.
func TestLagoAdapterHealthFailuresAreUnreachableNeverReady(t *testing.T) {
	unreachable := func(t *testing.T, p *LagoAdapter, ctx context.Context) {
		t.Helper()
		snap, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindReadiness})
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want ErrPlatformUnreachable, got %v (snapshot %+v)", err, snap.Readiness)
		}
		if strings.Contains(err.Error(), testAPIKey) {
			t.Fatalf("error string leaks the API key: %v", err)
		}
	}

	t.Run("health 5xx", func(t *testing.T) {
		stub := newHealthStub(t, http.StatusServiceUnavailable)
		unreachable(t, NewLagoAdapter(lagoTestConfig(stub.url())), context.Background())
	})

	t.Run("connection refused", func(t *testing.T) {
		// Port 1 on loopback answers connection-refused without any network
		// dependency or flaky timing.
		unreachable(t, NewLagoAdapter(lagoTestConfig("http://127.0.0.1:1")), context.Background())
	})

	t.Run("expired context", func(t *testing.T) {
		stub := newHealthStub(t, http.StatusOK)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		unreachable(t, NewLagoAdapter(lagoTestConfig(stub.url())), ctx)
	})
}

// TestLagoAdapterHealth4xxIsInvalidResponse: a 4xx is a definitive wrong
// answer (ErrPlatformInvalidResponse), not a retryable miss and never ready.
func TestLagoAdapterHealth4xxIsInvalidResponse(t *testing.T) {
	stub := newHealthStub(t, http.StatusNotFound)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	_, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("want ErrPlatformInvalidResponse, got %v", err)
	}
	if errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("a 4xx must not classify as unreachable: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error string leaks the API key: %v", err)
	}
}

// TestLagoAdapterUnconfiguredFailsClosed: construction succeeds with empty
// config (blocked-env stays legal), but the call fails fast with
// ErrPlatformUnconfigured and never mentions the credential.
func TestLagoAdapterUnconfiguredFailsClosed(t *testing.T) {
	p := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: "", APIKey: testAPIKey})
	_, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("missing URL must fail with ErrPlatformUnconfigured, got %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error string leaks the API key: %v", err)
	}

	p = NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: "http://127.0.0.1:9", APIKey: ""})
	_, err = p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("missing API key must fail with ErrPlatformUnconfigured, got %v", err)
	}
}

// TestLagoAdapterFrozenFamiliesFailClosed: command and reconcile stay frozen
// in T05 even with a fully configured adapter.
func TestLagoAdapterFrozenFamiliesFailClosed(t *testing.T) {
	stub := newHealthStub(t, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	if _, err := p.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKind("ensure_customer"), Key: "k-1",
	}); !errors.Is(err, commercial.ErrPlatformUnsupported) {
		t.Fatalf("SubmitCommand must fail closed unsupported, got %v", err)
	}
	if _, err := p.Reconcile(context.Background(), commercial.ReconciliationCursor{Stream: "billing"}); !errors.Is(err, commercial.ErrPlatformUnsupported) {
		t.Fatalf("Reconcile must fail closed unsupported, got %v", err)
	}
	if paths := stub.recordedPaths(); len(paths) != 0 {
		t.Fatalf("frozen families must issue no requests, got %v", paths)
	}
}

// TestNewPlatformSelectsAdapterByProvider: ""/lago select the Lago adapter,
// fake selects the fake, and any other value is a construction error — no
// silent fallback.
func TestNewPlatformSelectsAdapterByProvider(t *testing.T) {
	for _, provider := range []string{"", ProviderLago} {
		p, err := NewPlatform(Config{Provider: provider})
		if err != nil {
			t.Fatalf("provider %q: %v", provider, err)
		}
		if _, ok := p.(*LagoAdapter); !ok {
			t.Fatalf("provider %q must select the Lago adapter, got %T", provider, p)
		}
	}
	p, err := NewPlatform(Config{Provider: ProviderFake})
	if err != nil {
		t.Fatalf("fake provider: %v", err)
	}
	if _, ok := p.(*FakeAdapter); !ok {
		t.Fatalf("fake provider must select the fake adapter, got %T", p)
	}
	if _, err := NewPlatform(Config{Provider: "someone-else"}); err == nil {
		t.Fatalf("unknown provider must be a construction error, not a silent fallback")
	}
}

// TestConfigFromEnvReadsThePlatformFamily: the WEKNORA_COMMERCIAL_PLATFORM_*
// family is read from the (server-side) environment; nothing is committed.
func TestConfigFromEnvReadsThePlatformFamily(t *testing.T) {
	env := map[string]string{
		EnvProvider: ProviderLago,
		EnvBaseURL:  "http://127.0.0.1:48897",
		EnvAPIKey:   testAPIKey,
		EnvRelease:  "v1.53.0",
	}
	getenv := func(name string) string { return env[name] }

	cfg := configFromEnv(getenv)
	if cfg.Provider != ProviderLago || cfg.BaseURL != "http://127.0.0.1:48897" ||
		cfg.APIKey != testAPIKey || cfg.Release != "v1.53.0" {
		t.Fatalf("config mismatch: %+v", cfg)
	}

	// Empty environment stays legal: unconfigured Lago adapter, calls fail
	// closed (blocked-env, openmeter precedent).
	p, err := newPlatformFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("empty env must construct, got %v", err)
	}
	if _, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	}); !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("empty env must fail closed unconfigured, got %v", err)
	}

	// An unknown provider value is a startup error.
	if _, err := newPlatformFromEnv(func(name string) string {
		if name == EnvProvider {
			return "nope"
		}
		return ""
	}); err == nil {
		t.Fatalf("unknown provider must fail construction")
	}
}
