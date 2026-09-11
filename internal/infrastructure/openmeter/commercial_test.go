package openmeter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
)

// grantServer stands in for the one OpenMeter deployment family V03 selected
// (official_v3). It models exactly the three operations the adapter uses:
// create-credit-grant (idempotent on idempotencyKey), list-credit-grant
// filtered by idempotencyKey, and the failure mode that matters for recovery:
// persisting the grant and then dropping the connection before the client
// can read the response.
//
// OM-01..OM-10 real-model evidence stays blocked-env: no OpenMeter service
// or merchant credentials exist in this environment, so nothing here talks to
// a live provider and no live grant is ever fabricated.
type grantServer struct {
	mu       sync.Mutex
	saved    map[string]savedGrant // idempotencyKey -> grant
	applyN   int                   // apply requests handled (idempotent replays included)
	dropN    int                   // drop the next N applies after saving
	bodies   []grantRequest        // decoded apply bodies in order
	lookedUp []string              // idempotencyKeys queried
	server   *httptest.Server
	nextSeq  int
}

type savedGrant struct {
	id         string
	customer   string
	effective  string
	tenantMeta string
}

func newGrantServer(t *testing.T) *grantServer {
	t.Helper()
	g := &grantServer{saved: map[string]savedGrant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/openmeter/customers/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test_key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/v3/openmeter/customers/")
		customer := strings.TrimSuffix(rest, "/credits/grants")
		if customer == "" || customer == rest {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req grantRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			g.mu.Lock()
			g.applyN++
			g.bodies = append(g.bodies, req)
			existing, ok := g.saved[req.IdempotencyKey]
			var grant savedGrant
			if ok {
				grant = existing
			} else {
				g.nextSeq++
				grant = savedGrant{
					id:         fmt.Sprintf("grant_%d", g.nextSeq),
					customer:   customer,
					effective:  req.EffectiveAt,
					tenantMeta: req.Metadata["tenantId"],
				}
				g.saved[req.IdempotencyKey] = grant
			}
			drop := !ok && g.dropN > 0
			if drop {
				g.dropN--
			}
			g.mu.Unlock()
			if drop {
				// The mandated failure mode: the grant is durable server-side,
				// but the connection dies before the client reads anything.
				hj, canHijack := w.(http.Hijacker)
				if !canHijack {
					panic("grant server cannot hijack connections")
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					panic(err)
				}
				_ = conn.Close()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(grantResponse{ID: grant.id, EffectiveAt: grant.effective})
		case http.MethodGet:
			key := r.URL.Query().Get("idempotencyKey")
			g.mu.Lock()
			g.lookedUp = append(g.lookedUp, key)
			grant, ok := g.saved[key]
			items := []grantResponse{}
			if ok && grant.customer == customer {
				items = append(items, grantResponse{ID: grant.id, EffectiveAt: grant.effective})
			}
			g.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(grantListResponse{Items: items})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	g.server = httptest.NewServer(mux)
	t.Cleanup(g.server.Close)
	return g
}

func (g *grantServer) snapshot() (applies int, saved int, lookedUp int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.applyN, len(g.saved), len(g.lookedUp)
}

func testBenefitRequest(key string) domain.BenefitRequest {
	return domain.BenefitRequest{
		Key:         key,
		TenantID:    4242,
		CustomerID:  "tenant_4242",
		Kind:        domain.BenefitKindTopUp,
		PlanRef:     "credits",
		Credits:     domain.Credits(1_000_000_000),
		EffectiveAt: time.Date(2028, 1, 10, 10, 0, 0, 0, time.UTC),
	}
}

func testGateway(t *testing.T, g *grantServer) *Gateway {
	t.Helper()
	gw, err := NewGateway(Config{Family: FamilyOfficialV3, BaseURL: g.server.URL, APIKey: "test_key"})
	if err != nil {
		t.Fatal(err)
	}
	return gw
}

// TestApplyBenefitMapsGrantAndLookupFields proves the field mapping against
// the official_v3 schema: idempotencyKey carries the FulfillmentKey, amount
// is the fixed-point credit string, effectiveAt is RFC3339Nano UTC, metadata
// carries the order-derived tenant, and the receipt maps the provider id and
// effective time. FindBenefit reads the customer scope from the context and
// filters the lookup by idempotencyKey.
func TestApplyBenefitMapsGrantAndLookupFields(t *testing.T) {
	g := newGrantServer(t)
	gw := testGateway(t, g)
	ctx := context.Background()
	key := domain.FulfillmentKey("order_1", "credits")

	receipt, err := gw.ApplyBenefit(ctx, testBenefitRequest(key))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExternalID != "grant_1" {
		t.Fatalf("receipt external id not mapped, got %q", receipt.ExternalID)
	}
	wantEffective := "2028-01-10T10:00:00Z"
	if !receipt.EffectiveAt.Equal(time.Date(2028, 1, 10, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("receipt effective time not mapped, got %v", receipt.EffectiveAt)
	}

	g.mu.Lock()
	body := g.bodies[0]
	g.mu.Unlock()
	if body.IdempotencyKey != key {
		t.Fatalf("idempotency key must be the fulfillment key, got %q", body.IdempotencyKey)
	}
	if body.Amount != "1000.000000" {
		t.Fatalf("amount must map fixed-point credits, got %q", body.Amount)
	}
	if body.FeatureKey != "credits" {
		t.Fatalf("feature key must map plan ref, got %q", body.FeatureKey)
	}
	if body.EffectiveAt != wantEffective {
		t.Fatalf("effective time must be RFC3339Nano UTC, got %q", body.EffectiveAt)
	}
	if body.Metadata["tenantId"] != "4242" || body.Metadata["kind"] != domain.BenefitKindTopUp {
		t.Fatalf("tenant/kind metadata not mapped: %+v", body.Metadata)
	}

	// Lookup discovers by key within the context's customer scope.
	found, err := gw.FindBenefit(domain.WithBenefitCustomer(ctx, "tenant_4242"), key)
	if err != nil {
		t.Fatal(err)
	}
	if found.ExternalID != receipt.ExternalID || !found.EffectiveAt.Equal(receipt.EffectiveAt) {
		t.Fatalf("find must return the same grant, got %+v want %+v", found, receipt)
	}

	// An unknown key is a provable miss, not an error-of-transport.
	if _, err := gw.FindBenefit(domain.WithBenefitCustomer(ctx, "tenant_4242"), domain.FulfillmentKey("nope", "credits")); err != domain.ErrBenefitNotFound {
		t.Fatalf("missing grant must be ErrBenefitNotFound, got %v", err)
	}

	// A lookup with no customer scope cannot prove anything: reporting a miss
	// would license a duplicate grant, so it stays indeterminate.
	if _, err := gw.FindBenefit(ctx, key); domain.ClassifyFulfillment(err) != domain.FulfillmentUnknown {
		t.Fatalf("unscoped lookup must be indeterminate, got %v", err)
	}
}

// TestApplyBenefitSavedThenDroppedRecoversViaFindBenefit is the mandated
// recovery scenario: the server persists the benefit and then drops the
// connection. ApplyBenefit must surface an indeterminate result — never a
// success — and FindBenefit must then discover the saved benefit, which is
// what lets the worker recover without granting twice.
func TestApplyBenefitSavedThenDroppedRecoversViaFindBenefit(t *testing.T) {
	g := newGrantServer(t)
	g.mu.Lock()
	g.dropN = 1
	g.mu.Unlock()
	gw := testGateway(t, g)
	ctx := context.Background()
	key := domain.FulfillmentKey("order_drop", "credits")

	receipt, err := gw.ApplyBenefit(ctx, testBenefitRequest(key))
	if err == nil {
		t.Fatalf("a dropped response must not report success, got receipt %+v", receipt)
	}
	if got := domain.ClassifyFulfillment(err); got != domain.FulfillmentUnknown {
		t.Fatalf("dropped connection must classify unknown, got %v (%v)", got, err)
	}
	g.mu.Lock()
	savedGrant, ok := g.saved[key]
	g.mu.Unlock()
	if !ok {
		t.Fatal("server must have saved the benefit before dropping the connection")
	}

	found, err := gw.FindBenefit(domain.WithBenefitCustomer(ctx, "tenant_4242"), key)
	if err != nil {
		t.Fatal(err)
	}
	if found.ExternalID != savedGrant.id {
		t.Fatalf("find must discover the saved benefit, got %q want %q", found.ExternalID, savedGrant.id)
	}
	if !found.EffectiveAt.Equal(time.Date(2028, 1, 10, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("discovered effective time not mapped, got %v", found.EffectiveAt)
	}

	// A replay of the same idempotency key must converge on the same grant
	// instead of creating a second one.
	replayed, err := gw.ApplyBenefit(ctx, testBenefitRequest(key))
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ExternalID != savedGrant.id {
		t.Fatalf("idempotent replay returned a different grant: %q vs %q", replayed.ExternalID, savedGrant.id)
	}
	if applies, saved, _ := g.snapshot(); saved != 1 || applies != 2 {
		t.Fatalf("want 1 saved grant after replay (applies=%d saved=%d)", applies, saved)
	}
}

// TestApplyBenefitBusinessRefusalIsNeverSuccess pins the refusal path: a 4xx
// from the provider is a definitive business rejection, and a 5xx or a lost
// response stays indeterminate. Neither may be recorded as a success.
func TestApplyBenefitBusinessRefusalIsNeverSuccess(t *testing.T) {
	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("invalid plan"))
	}))
	t.Cleanup(refused.Close)
	gw, err := NewGateway(Config{Family: FamilyOfficialV3, BaseURL: refused.URL, APIKey: "test_key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.ApplyBenefit(context.Background(), testBenefitRequest(domain.FulfillmentKey("o", "l")))
	if got := domain.ClassifyFulfillment(err); got != domain.FulfillmentRefused {
		t.Fatalf("4xx must classify refused, got %v (%v)", got, err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	t.Cleanup(broken.Close)
	gw2, err := NewGateway(Config{Family: FamilyOfficialV3, BaseURL: broken.URL, APIKey: "test_key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw2.ApplyBenefit(context.Background(), testBenefitRequest(domain.FulfillmentKey("o", "l")))
	if got := domain.ClassifyFulfillment(err); got != domain.FulfillmentUnknown {
		t.Fatalf("aborted response must classify unknown, got %v (%v)", got, err)
	}
}

// TestApplyBenefitUnconfiguredGatewayStaysBlockedEnv pins the blocked-env
// boundary: booting with the connector unconfigured is legal (so operators
// can start the system), but every call fails fast with
// ErrGatewayUnconfigured — the real OpenMeter OM-04/OM-09 idempotency rerun
// stays blocked-env until a live service and merchant credentials exist, and
// this adapter never fabricates that evidence. Only the V03-selected
// official_v3 family is accepted.
func TestApplyBenefitUnconfiguredGatewayStaysBlockedEnv(t *testing.T) {
	gw, err := NewGateway(Config{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.ApplyBenefit(context.Background(), testBenefitRequest(domain.FulfillmentKey("o", "l")))
	if got := domain.ClassifyFulfillment(err); got != domain.FulfillmentUnknown {
		t.Fatalf("unconfigured apply must stay unknown, got %v (%v)", got, err)
	}
	_, err = gw.FindBenefit(domain.WithBenefitCustomer(context.Background(), "tenant_1"), domain.FulfillmentKey("o", "l"))
	if got := domain.ClassifyFulfillment(err); got != domain.FulfillmentUnknown {
		t.Fatalf("unconfigured find must stay unknown, got %v (%v)", got, err)
	}

	if _, err := NewGateway(Config{Family: "stripe_legacy"}); err != ErrUnsupportedFamily {
		t.Fatalf("non-selected family must be rejected, got %v", err)
	}
}
