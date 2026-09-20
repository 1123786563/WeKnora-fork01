package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

// TestLagoAdapterFrozenFamiliesFailClosed: reconcile and every kind outside
// the enabled set (ensure_customer) stay frozen and fail closed.
func TestLagoAdapterFrozenFamiliesFailClosed(t *testing.T) {
	stub := newHealthStub(t, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	if _, err := p.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKind("no_such_kind"), Key: "k-1",
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

// customersStub stands in for the Lago customers API surface (the #73
// runtime-proven contract: POST /api/v1/customers with
// {"customer":{"external_id","name"}} echoes customer.external_id; Bearer
// API-key auth). It records method+path+body+auth per request so tests can
// assert the exact request SEQUENCE — the read-before-create core.
type customersStub struct {
	mu         sync.Mutex
	getStatus  int             // GET /api/v1/customers/{ext} answer for an unknown identity (404 = absent)
	postStatus int             // POST /api/v1/customers answer
	created    map[string]bool // identities created by a 2xx POST (authority state)
	requests   []customersStubRequest
	server     *httptest.Server
}

type customersStubRequest struct {
	Method string
	Path   string
	Body   string
	Auth   string
}

func newCustomersStub(t *testing.T, getStatus, postStatus int) *customersStub {
	t.Helper()
	s := &customersStub{getStatus: getStatus, postStatus: postStatus}
	mux := http.NewServeMux()
	// The stub also serves the /health liveness signal so the SHARED contract
	// table can run both the readiness legs and the W3 customer legs against
	// one stub-backed adapter.
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// One handler for BOTH the create endpoint (exact /api/v1/customers) and
	// the identity-addressed surface (the /... subtree) — ServeMux treats
	// them as distinct patterns. A 2xx create STORES the customer, so the
	// stub behaves like the real authority: a later identity GET finds it.
	customersHandler := func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<10))
		s.requests = append(s.requests, customersStubRequest{
			Method: r.Method, Path: r.URL.Path, Body: string(body),
			Auth: r.Header.Get("Authorization"),
		})
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/customers" {
			var payload struct {
				Customer struct {
					ExternalID string `json:"external_id"`
					Name       string `json:"name"`
				} `json:"customer"`
			}
			_ = json.Unmarshal(body, &payload)
			if s.postStatus >= 200 && s.postStatus < 300 && payload.Customer.ExternalID != "" {
				if s.created == nil {
					s.created = map[string]bool{}
				}
				s.created[payload.Customer.ExternalID] = true
			}
			s.mu.Unlock()
			// Echo the payload's external_id (the #73 proven contract).
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(s.postStatus)
			if s.postStatus >= 200 && s.postStatus < 300 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"customer": map[string]any{"external_id": payload.Customer.ExternalID},
				})
			}
			return
		}
		created := s.created[strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")]
		s.mu.Unlock()
		if r.Method == http.MethodGet && created {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(s.getStatus)
	}
	mux.HandleFunc("/api/v1/customers", customersHandler)
	mux.HandleFunc("/api/v1/customers/", customersHandler)
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *customersStub) recorded() []customersStubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]customersStubRequest(nil), s.requests...)
}

func (s *customersStub) url() string { return s.server.URL }

func ensureCommand(tenant uint64, name, key string) commercial.Command {
	ext := commercial.ExternalCustomerID(tenant)
	return commercial.Command{
		Kind:    commercial.CommandKindEnsureCustomer,
		Key:     key,
		Actor:   "user-1",
		Reason:  "first_billing_access",
		Payload: commercial.EnsureCustomerPayload{TenantID: tenant, ExternalCustomerID: ext, DisplayName: name},
	}
}

// TestLagoEnsureCustomerReadsBeforeCreating: a first submit for an absent
// customer issues GET /api/v1/customers/weknora-tenant-42 (404) and then
// POST /api/v1/customers (200) — exactly that sequence — and the receipt
// carries the requested identity.
func TestLagoEnsureCustomerReadsBeforeCreating(t *testing.T) {
	stub := newCustomersStub(t, http.StatusNotFound, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	receipt, err := p.SubmitCommand(context.Background(), ensureCommand(42, "Space 42", "ensure_customer:weknora-tenant-42"))
	if err != nil {
		t.Fatalf("ensure_customer: %v", err)
	}
	if receipt.Key != "ensure_customer:weknora-tenant-42" || receipt.ExternalID != "weknora-tenant-42" {
		t.Fatalf("receipt identity mismatch: %+v", receipt)
	}
	if receipt.RecordedAt.IsZero() {
		t.Fatalf("receipt must carry RecordedAt")
	}
	got := stub.recorded()
	if len(got) != 2 {
		t.Fatalf("read-before-create must issue exactly GET then POST, got %v", got)
	}
	if got[0].Method != http.MethodGet || got[0].Path != "/api/v1/customers/weknora-tenant-42" {
		t.Fatalf("first request must be the identity GET, got %+v", got[0])
	}
	if got[1].Method != http.MethodPost || got[1].Path != "/api/v1/customers" {
		t.Fatalf("second request must be the create POST, got %+v", got[1])
	}
	if !strings.Contains(got[1].Body, `"external_id":"weknora-tenant-42"`) ||
		!strings.Contains(got[1].Body, `"name":"Space 42"`) {
		t.Fatalf("create payload must carry the #73 contract shape, got %q", got[1].Body)
	}
	if got[1].Auth != "Bearer "+testAPIKey {
		t.Fatalf("the create POST must carry the Bearer API key, got %q", got[1].Auth)
	}
}

// TestLagoEnsureCustomerIdentityPresentNeverPosts: when the identity GET
// finds the customer (200), the adapter answers from the read alone — NO
// POST is ever issued, so even non-upsert authority behavior cannot
// duplicate the customer.
func TestLagoEnsureCustomerIdentityPresentNeverPosts(t *testing.T) {
	stub := newCustomersStub(t, http.StatusOK, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	receipt, err := p.SubmitCommand(context.Background(), ensureCommand(42, "Space 42", "ensure_customer:weknora-tenant-42"))
	if err != nil {
		t.Fatalf("ensure_customer: %v", err)
	}
	if receipt.ExternalID != "weknora-tenant-42" {
		t.Fatalf("receipt identity mismatch: %+v", receipt)
	}
	got := stub.recorded()
	if len(got) != 1 || got[0].Method != http.MethodGet {
		t.Fatalf("a present identity must issue exactly the GET (no POST), got %v", got)
	}
}

// TestLagoEnsureCustomerFailuresMapToSentinels: a POST 5xx or a dropped
// connection is unreachable (indeterminate — retry by identity); a POST
// 404/422 is a definitive invalid response; missing config fails fast
// unconfigured.
func TestLagoEnsureCustomerFailuresMapToSentinels(t *testing.T) {
	submit := func(t *testing.T, p commercial.CommercialPlatform) error {
		t.Helper()
		_, err := p.SubmitCommand(context.Background(), ensureCommand(42, "Space 42", "ensure_customer:weknora-tenant-42"))
		return err
	}

	t.Run("post 5xx is unreachable", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusNotFound, http.StatusBadGateway)
		err := submit(t, NewLagoAdapter(lagoTestConfig(stub.url())))
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})

	t.Run("dropped connection is unreachable", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusNotFound, http.StatusOK)
		p := NewLagoAdapter(lagoTestConfig(stub.url()))
		stub.server.CloseClientConnections()
		// A closed client connection surfaces as a transport error: kill the
		// server so the next dial fails.
		stub.server.Close()
		err := submit(t, p)
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})

	t.Run("post 4xx is invalid response", func(t *testing.T) {
		for _, status := range []int{http.StatusNotFound, http.StatusUnprocessableEntity} {
			stub := newCustomersStub(t, http.StatusNotFound, status)
			err := submit(t, NewLagoAdapter(lagoTestConfig(stub.url())))
			if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
				t.Fatalf("status %d: want invalid response, got %v", status, err)
			}
			if errors.Is(err, commercial.ErrPlatformUnreachable) {
				t.Fatalf("status %d must not classify as unreachable: %v", status, err)
			}
		}
	})

	t.Run("get 5xx is unreachable", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusBadGateway, http.StatusOK)
		err := submit(t, NewLagoAdapter(lagoTestConfig(stub.url())))
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})

	t.Run("missing config is unconfigured", func(t *testing.T) {
		p := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: "", APIKey: testAPIKey})
		if err := submit(t, p); !errors.Is(err, commercial.ErrPlatformUnconfigured) {
			t.Fatalf("want unconfigured, got %v", err)
		}
	})

	t.Run("errors never leak the API key", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusNotFound, http.StatusBadGateway)
		err := submit(t, NewLagoAdapter(lagoTestConfig(stub.url())))
		if err == nil || strings.Contains(err.Error(), testAPIKey) {
			t.Fatalf("error must exist and never leak the key: %v", err)
		}
	})
}

// TestLagoEnsureCustomerPayloadGuard: a payload without a tenant, without an
// external id, or with an external id INCONSISTENT with the tenant derivation
// is refused ErrPlatformUnsupported before any request leaves the adapter.
func TestLagoEnsureCustomerPayloadGuard(t *testing.T) {
	stub := newCustomersStub(t, http.StatusNotFound, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	for name, cmd := range map[string]commercial.Command{
		"wrong payload type": {Kind: commercial.CommandKindEnsureCustomer, Key: "k", Payload: map[string]any{}},
		"zero tenant": {Kind: commercial.CommandKindEnsureCustomer, Key: "k",
			Payload: commercial.EnsureCustomerPayload{TenantID: 0, ExternalCustomerID: "weknora-tenant-0"}},
		"empty external id": {Kind: commercial.CommandKindEnsureCustomer, Key: "k",
			Payload: commercial.EnsureCustomerPayload{TenantID: 42, ExternalCustomerID: ""}},
		"mismatched identity": {Kind: commercial.CommandKindEnsureCustomer, Key: "k",
			Payload: commercial.EnsureCustomerPayload{TenantID: 42, ExternalCustomerID: "weknora-tenant-43"}},
	} {
		if _, err := p.SubmitCommand(context.Background(), cmd); !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("%s: want ErrPlatformUnsupported, got %v", name, err)
		}
	}
	if got := stub.recorded(); len(got) != 0 {
		t.Fatalf("a refused payload must issue zero requests, got %v", got)
	}
}

// TestLagoAccountSnapshotTruth: the account snapshot answers linked on a
// 200 identity read, absent on 404, and unreachable on transport failure —
// never a fabricated truth.
func TestLagoAccountSnapshotTruth(t *testing.T) {
	q := commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: 42}

	t.Run("present is linked", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusOK, http.StatusOK)
		snap, err := NewLagoAdapter(lagoTestConfig(stub.url())).ReadSnapshot(context.Background(), q)
		if err != nil {
			t.Fatalf("account snapshot: %v", err)
		}
		if snap.Account == nil || snap.Account.State != commercial.AccountStateLinked || snap.Account.TenantID != 42 {
			t.Fatalf("want linked truth for tenant 42, got %+v", snap.Account)
		}
		if snap.Account.CheckedAt.IsZero() {
			t.Fatalf("CheckedAt must be set")
		}
	})

	t.Run("absent is absent", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusNotFound, http.StatusOK)
		snap, err := NewLagoAdapter(lagoTestConfig(stub.url())).ReadSnapshot(context.Background(), q)
		if err != nil {
			t.Fatalf("account snapshot: %v", err)
		}
		if snap.Account == nil || snap.Account.State != commercial.AccountStateAbsent {
			t.Fatalf("want absent truth, got %+v", snap.Account)
		}
	})

	t.Run("transport failure is unreachable", func(t *testing.T) {
		_, err := NewLagoAdapter(lagoTestConfig("http://127.0.0.1:1")).ReadSnapshot(context.Background(), q)
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})

	t.Run("zero tenant is refused unsupported", func(t *testing.T) {
		stub := newCustomersStub(t, http.StatusOK, http.StatusOK)
		_, err := NewLagoAdapter(lagoTestConfig(stub.url())).ReadSnapshot(context.Background(),
			commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: 0})
		if !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("want unsupported for zero tenant, got %v", err)
		}
		if got := stub.recorded(); len(got) != 0 {
			t.Fatalf("a refused snapshot must issue zero requests, got %v", got)
		}
	})
}

// TestLagoRemainingKindsStillFailClosed: Reconcile and unknown command kinds
// stay frozen (ErrPlatformUnsupported) even with the customers surface
// enabled; no request is issued for them.
func TestLagoRemainingKindsStillFailClosed(t *testing.T) {
	stub := newCustomersStub(t, http.StatusOK, http.StatusOK)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	if _, err := p.SubmitCommand(context.Background(), commercial.Command{
		Kind: commercial.CommandKind("publish_plan_version"), Key: "k",
	}); !errors.Is(err, commercial.ErrPlatformUnsupported) {
		t.Fatalf("unknown kind must fail closed, got %v", err)
	}
	if _, err := p.Reconcile(context.Background(), commercial.ReconciliationCursor{Stream: "billing"}); !errors.Is(err, commercial.ErrPlatformUnsupported) {
		t.Fatalf("Reconcile must fail closed, got %v", err)
	}
	if got := stub.recorded(); len(got) != 0 {
		t.Fatalf("frozen families must issue no requests, got %v", got)
	}
}

// TestFakeEnsureCustomerIdempotentPerKey: replaying the SAME Key returns the
// ORIGINAL receipt (unchanged RecordedAt) and leaves exactly one stored
// customer.
func TestFakeEnsureCustomerIdempotentPerKey(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	first, err := f.SubmitCommand(ctx, ensureCommand(7, "Space 7", "ensure_customer:weknora-tenant-7"))
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := f.SubmitCommand(ctx, ensureCommand(7, "Space 7 renamed", "ensure_customer:weknora-tenant-7"))
	if err != nil {
		t.Fatalf("replay submit: %v", err)
	}
	if first != second {
		t.Fatalf("replay must return the ORIGINAL receipt, got %+v then %+v", first, second)
	}
	customers := f.Customers()
	if len(customers) != 1 {
		t.Fatalf("exactly one stored customer, got %d", len(customers))
	}
}

// TestFakeEnsureCustomerUpsertsUnderNewKey: a DIFFERENT Key addressing the
// same external id updates the single stored customer (advisory metadata
// refresh) — never a second entry.
func TestFakeEnsureCustomerUpsertsUnderNewKey(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	if _, err := f.SubmitCommand(ctx, ensureCommand(7, "Old Name", "ensure_customer:weknora-tenant-7")); err != nil {
		t.Fatal(err)
	}
	receipt, err := f.SubmitCommand(ctx, ensureCommand(7, "New Name", "ensure_customer:weknora-tenant-7#2"))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExternalID != "weknora-tenant-7" {
		t.Fatalf("upsert receipt keeps the identity, got %+v", receipt)
	}
	customers := f.Customers()
	if len(customers) != 1 {
		t.Fatalf("upsert must never create a second customer, got %d", len(customers))
	}
	if customers[0].Name != "New Name" {
		t.Fatalf("advisory name must be refreshed, got %+v", customers[0])
	}
}

// TestFakeFaultKnobSurfacesInjectedSentinel: FailSubmitsWith drives the
// recovery tests — every submit fails with the injected sentinel.
func TestFakeFaultKnobSurfacesInjectedSentinel(t *testing.T) {
	f := NewFakeAdapter()
	f.FailSubmitsWith(commercial.ErrPlatformUnreachable)
	_, err := f.SubmitCommand(context.Background(), ensureCommand(7, "Space 7", "ensure_customer:weknora-tenant-7"))
	if !errors.Is(err, commercial.ErrPlatformUnreachable) {
		t.Fatalf("fault knob must surface the injected sentinel, got %v", err)
	}
	f.FailSubmitsWith(nil)
	if _, err := f.SubmitCommand(context.Background(), ensureCommand(7, "Space 7", "ensure_customer:weknora-tenant-7")); err != nil {
		t.Fatalf("clearing the knob must restore submits, got %v", err)
	}
}

// TestFakeAccountSnapshotReflectsStoreTruth: the account snapshot derives
// from stored customers — linked once ensured, absent before (never a
// fabricated linked), and honest again for a different tenant.
func TestFakeAccountSnapshotReflectsStoreTruth(t *testing.T) {
	f := NewFakeAdapter()
	ctx := context.Background()
	snap, err := f.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: 7})
	if err != nil {
		t.Fatalf("absent snapshot: %v", err)
	}
	if snap.Account == nil || snap.Account.State != commercial.AccountStateAbsent {
		t.Fatalf("unprimed store must answer absent, got %+v", snap.Account)
	}
	if _, err := f.SubmitCommand(ctx, ensureCommand(7, "Space 7", "ensure_customer:weknora-tenant-7")); err != nil {
		t.Fatal(err)
	}
	snap, err = f.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: 7})
	if err != nil {
		t.Fatalf("linked snapshot: %v", err)
	}
	if snap.Account == nil || snap.Account.State != commercial.AccountStateLinked {
		t.Fatalf("ensured tenant must answer linked, got %+v", snap.Account)
	}
	snap, err = f.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: 8})
	if err != nil {
		t.Fatalf("other tenant snapshot: %v", err)
	}
	if snap.Account == nil || snap.Account.State != commercial.AccountStateAbsent {
		t.Fatalf("a different tenant must answer absent, got %+v", snap.Account)
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
