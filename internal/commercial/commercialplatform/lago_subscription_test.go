package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// ---- stateful stubs (the planStub pattern, extended to subscriptions and
// wallets) ----

// respond records one stub request and answers it — recording and writing
// under one lock keeps the recorded status consistent. reqBody is the
// already-read request body (nil for GETs — the body is read here).
func respond(w http.ResponseWriter, r *http.Request, requests *[]stubReq, mu *sync.Mutex, status int, body string, reqBody []byte) {
	if reqBody == nil {
		reqBody, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
	mu.Lock()
	*requests = append(*requests, stubReq{
		Method: r.Method, Path: r.URL.Path, Body: string(reqBody),
		Auth: r.Header.Get("Authorization"), Status: status,
	})
	mu.Unlock()
}

// stubSubscription is one recorded authority-side subscription.
type stubSubscription struct {
	ExternalID       string
	ExternalCustomer string
	PlanCode         string
	Status           string
}

func subscriptionsJSON(subs []stubSubscription) string {
	out := make([]string, 0, len(subs))
	for _, sub := range subs {
		out = append(out, fmt.Sprintf(
			`{"lago_id":"sub_%s","external_id":%q,"external_customer_id":%q,"plan_code":%q,"status":%q}`,
			sub.ExternalID, sub.ExternalID, sub.ExternalCustomer, sub.PlanCode, sub.Status))
	}
	return `{"subscriptions":[` + strings.Join(out, ",") + `]}`
}

// subscriptionsStub is a STATEFUL mini authority for the subscription API:
// the create POST records, the index lists recorded (plus preloaded)
// subscriptions, and scripted create statuses serve the 422 paths.
type subscriptionsStub struct {
	mu       sync.Mutex
	requests []stubReq
	queries  []url.Values // recorded index queries (the status[] assertions)

	subs             []stubSubscription
	preloaded        []stubSubscription
	revealAfterPosts int // preloaded subs appear on the index only after N create POSTs
	createStatuses   []int
	indexStatus      int
	handler          http.Handler
	server           *httptest.Server
}

func newSubscriptionsStubBuilder() *subscriptionsStub {
	s := &subscriptionsStub{createStatuses: []int{http.StatusCreated}, indexStatus: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.mu.Lock()
			s.queries = append(s.queries, r.URL.Query())
			status := s.indexStatus
			posts := s.countCreatesLocked()
			out := append([]stubSubscription(nil), s.subs...)
			if posts >= s.revealAfterPosts {
				out = append(out, s.preloaded...)
			}
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, status, subscriptionsJSON(out), nil)
		case http.MethodPost:
			blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			var parsed struct {
				Subscription struct {
					ExternalID         string `json:"external_id"`
					ExternalCustomerID string `json:"external_customer_id"`
					PlanCode           string `json:"plan_code"`
				} `json:"subscription"`
			}
			_ = json.Unmarshal(blob, &parsed)
			s.mu.Lock()
			status := s.nextCreateStatus()
			if status >= 200 && status < 300 {
				s.subs = append(s.subs, stubSubscription{
					ExternalID:       parsed.Subscription.ExternalID,
					ExternalCustomer: parsed.Subscription.ExternalCustomerID,
					PlanCode:         parsed.Subscription.PlanCode,
					Status:           "active",
				})
			}
			recorded := append([]stubSubscription(nil), s.subs...)
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, status, subscriptionsJSON(recorded), blob)
		default:
			http.NotFound(w, r)
		}
	})
	s.handler = mux
	return s
}

func newSubscriptionsStub(t *testing.T) *subscriptionsStub {
	t.Helper()
	s := newSubscriptionsStubBuilder()
	s.server = httptest.NewServer(s.handler)
	t.Cleanup(s.server.Close)
	return s
}

func (s *subscriptionsStub) nextCreateStatus() int {
	if len(s.createStatuses) > 1 {
		st := s.createStatuses[0]
		s.createStatuses = s.createStatuses[1:]
		return st
	}
	if len(s.createStatuses) == 1 {
		return s.createStatuses[0]
	}
	return http.StatusCreated
}

func (s *subscriptionsStub) countCreates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countCreatesLocked()
}

func (s *subscriptionsStub) countCreatesLocked() int {
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodPost && r.Path == "/api/v1/subscriptions" {
			n++
		}
	}
	return n
}

func (s *subscriptionsStub) recorded() []stubReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubReq(nil), s.requests...)
}

func (s *subscriptionsStub) indexQueries() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.queries...)
}

func (s *subscriptionsStub) url() string { return s.server.URL }

// stubWallet is one recorded authority-side wallet.
type stubWallet struct {
	LagoID       string
	Customer     string
	Name         string
	Status       string // active|terminated
	GrantedCents int64
	BalanceCents int64
	ExpiresAt    string
	CreatedAt    string
	Priority     int
	Metadata     map[string]string
}

func walletJSON(w stubWallet) string {
	m, _ := json.Marshal(w.Metadata)
	createdAt := w.CreatedAt
	if createdAt == "" {
		createdAt = "2099-01-01T00:00:00Z"
	}
	return fmt.Sprintf(
		`{"lago_id":%q,"name":%q,"status":%q,"balance_cents":%d,"granted_credits":"%s","rate_amount":"1","expiration_at":%q,"created_at":%q,"priority":%d,"metadata":%s,"currency":"CNY"}`,
		w.LagoID, w.Name, w.Status, w.BalanceCents, centsString(w.GrantedCents), w.ExpiresAt, createdAt, w.Priority, string(m))
}

func centsString(cents int64) string { return commercial.FenToDecimalString(cents) }

// parseDecimalCents parses a non-negative "credits.cents" decimal string to
// cents with integer arithmetic (test-side exactness).
func parseDecimalCents(s string) int64 {
	whole, frac, _ := strings.Cut(s, ".")
	for len(frac) < 2 {
		frac += "0"
	}
	if len(frac) > 2 {
		frac = frac[:2]
	}
	var w, f int64
	for _, c := range whole {
		if c < '0' || c > '9' {
			return 0
		}
		w = w*10 + int64(c-'0')
	}
	for _, c := range frac {
		if c < '0' || c > '9' {
			return 0
		}
		f = f*10 + int64(c-'0')
	}
	return w*100 + f
}

// walletsStub is a STATEFUL mini authority for the wallet + entitlement
// APIs: the create POST records (scriptable to wallet_limit_reached 422s),
// the customer wallet list lists recorded wallets, the wallet GET answers
// the settle-poll (scriptable unsettled rounds).
type walletsStub struct {
	mu       sync.Mutex
	requests []stubReq

	wallets              []stubWallet
	createStatuses       []int  // scripted statuses; a 422 answers wallet_limit_reached
	limitExhausted       bool   // every POST answers the limit 422
	settleRounds         int    // initial wallet GETs answering an unsettled balance
	entitlements         string // scripted entitlements body (the BASE "-sub" leg)
	purchaseEntitlements string // scripted purchase-leg body ("" = 404 — no purchase subscription yet)
	// entitlementCustomer scopes the scripted entitlements to ONE customer
	// (per-customer truth; empty = all customers — the direct stub tests).
	entitlementCustomer string
	nextID              int
	handler             http.Handler
	server              *httptest.Server
}

func newWalletsStubBuilder() *walletsStub {
	s := &walletsStub{
		createStatuses: []int{http.StatusCreated},
		// The PINNED v1.53 entitlement shape (F-2' live-stack evidence):
		// the feature code rides `code`, not feature_code/feature.code.
		entitlements: `{"entitlements":[{"code":"api_access","name":"api_access"}]}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/wallets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var parsed struct {
			Wallet struct {
				ExternalCustomerID string            `json:"external_customer_id"`
				Name               string            `json:"name"`
				GrantedCredits     string            `json:"granted_credits"`
				ExpirationAt       string            `json:"expiration_at"`
				Priority           int               `json:"priority"`
				Metadata           map[string]string `json:"metadata"`
			} `json:"wallet"`
		}
		_ = json.Unmarshal(blob, &parsed)
		var resp string
		s.mu.Lock()
		status := s.nextCreateStatus()
		if status == http.StatusUnprocessableEntity || s.limitExhausted {
			resp = `{"status":422,"error":"Unprocessable Entity","code":"wallet_limit_reached"}`
			status = http.StatusUnprocessableEntity
		} else if status >= 200 && status < 300 {
			s.nextID++
			granted := parseDecimalCents(parsed.Wallet.GrantedCredits)
			s.wallets = append(s.wallets, stubWallet{
				LagoID:       fmt.Sprintf("lago-wallet-%d", s.nextID),
				Customer:     parsed.Wallet.ExternalCustomerID,
				Name:         parsed.Wallet.Name,
				Status:       "active",
				GrantedCents: granted,
				BalanceCents: 0, // unsettled until the after-commit job (settle poll)
				ExpiresAt:    parsed.Wallet.ExpirationAt,
				Priority:     parsed.Wallet.Priority,
				Metadata:     parsed.Wallet.Metadata,
			})
			resp = `{"wallet":` + walletJSON(s.wallets[len(s.wallets)-1]) + `}`
		} else {
			resp = `{}`
		}
		s.mu.Unlock()
		respond(w, r, &s.requests, &s.mu, status, resp, blob)
	})
	mux.HandleFunc("/api/v1/wallets/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/wallets/")
		var resp string
		status := http.StatusOK
		if r.Method == http.MethodPut {
			// The priority-calibration write (the #86 rebalance): parse
			// {"wallet":{"priority":N}}, apply onto the stored wallet.
			blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			var parsed struct {
				Wallet struct {
					Priority int `json:"priority"`
				} `json:"wallet"`
			}
			_ = json.Unmarshal(blob, &parsed)
			s.mu.Lock()
			var found *stubWallet
			for i := range s.wallets {
				if s.wallets[i].LagoID == id {
					found = &s.wallets[i]
					break
				}
			}
			if found == nil {
				status, resp = http.StatusNotFound, `{}`
			} else {
				found.Priority = parsed.Wallet.Priority
				resp = `{"wallet":` + walletJSON(*found) + `}`
			}
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, status, resp, blob)
			return
		}
		s.mu.Lock()
		var found *stubWallet
		for i := range s.wallets {
			if s.wallets[i].LagoID == id {
				found = &s.wallets[i]
				break
			}
		}
		switch {
		case found == nil:
			status, resp = http.StatusNotFound, `{}`
		case s.settleRounds > 0:
			s.settleRounds--
			// balance_cents as stored: 0 (the settlement job has not run)
			resp = `{"wallet":` + walletJSON(*found) + `}`
		default:
			found.BalanceCents = found.GrantedCents // the after-commit settlement job ran
			resp = `{"wallet":` + walletJSON(*found) + `}`
		}
		s.mu.Unlock()
		respond(w, r, &s.requests, &s.mu, status, resp, nil)
	})
	mux.HandleFunc("/api/v1/subscriptions/", func(w http.ResponseWriter, r *http.Request) {
		// GET /api/v1/subscriptions/{external_id}/entitlements — the ONLY
		// entitlements index the pinned v1.53 exposes (F-2 flow evidence:
		// the customers-nested route answers 404 on the real release, so
		// the stub models the real route shape, never the absent one).
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions/")
		externalID, subpath, _ := strings.Cut(rest, "/")
		if subpath != "entitlements" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		// Map the WeKnora subscription identities back to the customer for
		// the per-customer scoping the stub models.
		customer := externalID
		for _, suffix := range []string{"-sub", "-purchase"} {
			if strings.HasSuffix(externalID, suffix) {
				customer = strings.TrimSuffix(externalID, suffix)
				break
			}
		}
		s.mu.Lock()
		body, code := s.entitlements, http.StatusOK
		switch {
		case s.entitlementCustomer != "" && customer != s.entitlementCustomer:
			body = `{"entitlements":[]}` // per-customer truth: no entitlements attached
		case strings.HasSuffix(externalID, "-purchase") && s.purchaseEntitlements == "":
			code = http.StatusNotFound // no purchase subscription (yet)
		case strings.HasSuffix(externalID, "-purchase"):
			body = s.purchaseEntitlements
		}
		s.mu.Unlock()
		respond(w, r, &s.requests, &s.mu, code, body, nil)
	})
	mux.HandleFunc("/api/v1/customers/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")
		customer, subpath, _ := strings.Cut(rest, "/")
		if subpath == "wallets" && r.Method == http.MethodGet {
			s.mu.Lock()
			out := make([]stubWallet, 0, len(s.wallets))
			for _, w := range s.wallets {
				if w.Customer == customer {
					out = append(out, w)
				}
			}
			s.mu.Unlock()
			items := make([]string, 0, len(out))
			for _, w := range out {
				items = append(items, walletJSON(w))
			}
			respond(w, r, &s.requests, &s.mu, http.StatusOK, `{"wallets":[`+strings.Join(items, ",")+`],"meta":{"next_page":null}}`, nil)
			return
		}
		http.NotFound(w, r)
	})
	s.handler = mux
	return s
}

func newWalletsStub(t *testing.T) *walletsStub {
	t.Helper()
	s := newWalletsStubBuilder()
	s.server = httptest.NewServer(s.handler)
	t.Cleanup(s.server.Close)
	return s
}

func (s *walletsStub) nextCreateStatus() int {
	if len(s.createStatuses) > 1 {
		st := s.createStatuses[0]
		s.createStatuses = s.createStatuses[1:]
		return st
	}
	if len(s.createStatuses) == 1 {
		return s.createStatuses[0]
	}
	return http.StatusCreated
}

func (s *walletsStub) countCreates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodPost && r.Path == "/api/v1/wallets" {
			n++
		}
	}
	return n
}

func (s *walletsStub) recorded() []stubReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubReq(nil), s.requests...)
}

func (s *walletsStub) url() string { return s.server.URL }

// combinedStub mounts the subscription, wallet and entitlement handlers on
// ONE server (the benefits snapshot needs them all).
type combinedStub struct {
	subs    *subscriptionsStub
	wallets *walletsStub
	server  *httptest.Server
}

func newCombinedStub(t *testing.T) *combinedStub {
	t.Helper()
	c := &combinedStub{subs: newSubscriptionsStubBuilder(), wallets: newWalletsStubBuilder()}
	mux := http.NewServeMux()
	mux.Handle("/api/v1/subscriptions", c.subs.handler)
	// The subscription SUB-paths the wallets stub owns: the v1.53
	// entitlements index (F-2 — the customers-nested route does not exist
	// on the pinned release).
	mux.Handle("/api/v1/subscriptions/", c.wallets.handler)
	mux.Handle("/api/v1/wallets", c.wallets.handler)
	mux.Handle("/api/v1/wallets/", c.wallets.handler)
	mux.Handle("/api/v1/customers/", c.wallets.handler)
	c.server = httptest.NewServer(mux)
	t.Cleanup(c.server.Close)
	return c
}

func (c *combinedStub) url() string { return c.server.URL }

// ---- test constants & helpers ----

const (
	subTenant   = uint64(201)
	subPlanCode = "weknora-base-v1"
)

func subAdapter(baseURL string) *LagoAdapter { return NewLagoAdapter(lagoTestConfig(baseURL)) }

func ensureSubCommand(planCode string) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindEnsureSubscription,
		Key:   commercial.EnsureSubscriptionCommandKey(commercial.ExternalSubscriptionID(subTenant)),
		Actor: "contract", Reason: "first_billing_access",
		Payload: commercial.EnsureSubscriptionPayload{
			TenantID:               subTenant,
			ExternalCustomerID:     commercial.ExternalCustomerID(subTenant),
			ExternalSubscriptionID: commercial.ExternalSubscriptionID(subTenant),
			PlanCode:               planCode,
		},
	}
}

func grantCommand(credits int64) commercial.Command {
	period := "2099-01"
	end := time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)
	return commercial.Command{
		Kind:  commercial.CommandKindGrantIncludedCredits,
		Key:   commercial.GrantCreditsCommandKey(commercial.ExternalCustomerID(subTenant), period),
		Actor: "contract", Reason: "monthly_included_credits",
		Payload: commercial.GrantIncludedCreditsPayload{
			TenantID:           subTenant,
			ExternalCustomerID: commercial.ExternalCustomerID(subTenant),
			Period:             period,
			CreditsMicro:       credits,
			ExpiresAt:          end,
			Priority:           commercial.MonthlyWalletPriority,
		},
	}
}

// TestLagoEnsureSubscriptionHappyPath: the create POST carries external_id,
// plan_code and external_customer_id and NO activation_rules key; the
// receipt echoes the deterministic subscription identity (never a provider
// id); every request is authenticated.
func TestLagoEnsureSubscriptionHappyPath(t *testing.T) {
	stub := newSubscriptionsStub(t)
	receipt, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
	if err != nil {
		t.Fatalf("ensure_subscription: %v", err)
	}
	if receipt.ExternalID != commercial.ExternalSubscriptionID(subTenant) {
		t.Fatalf("receipt ExternalID = %q, want the deterministic subscription identity", receipt.ExternalID)
	}
	posts := 0
	var body string
	for _, r := range stub.recorded() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/subscriptions" {
			posts++
			body = r.Body
		}
	}
	if posts != 1 {
		t.Fatalf("exactly one create POST expected, got %d", posts)
	}
	for _, key := range []string{`"external_id"`, `"plan_code"`, `"external_customer_id"`} {
		if !strings.Contains(body, key) {
			t.Fatalf("create body must carry %s, got %s", key, body)
		}
	}
	if strings.Contains(body, "activation_rules") {
		t.Fatalf("the free Base Plan must never be payment-gated: activation_rules found in %s", body)
	}
	for _, r := range stub.recorded() {
		if r.Auth != "Bearer "+testAPIKey {
			t.Fatalf("authorization header missing on %s %s", r.Method, r.Path)
		}
	}
}

// TestLagoEnsureSubscriptionReplayZeroPosts: an index hit showing the
// identity means ZERO create POSTs — idempotency by identity.
func TestLagoEnsureSubscriptionReplayZeroPosts(t *testing.T) {
	stub := newSubscriptionsStub(t)
	stub.preloaded = []stubSubscription{{
		ExternalID:       commercial.ExternalSubscriptionID(subTenant),
		ExternalCustomer: commercial.ExternalCustomerID(subTenant),
		PlanCode:         subPlanCode,
		Status:           "active",
	}}
	for i := 0; i < 2; i++ {
		receipt, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
		if err != nil {
			t.Fatalf("replay %d: %v", i, err)
		}
		if receipt.ExternalID != commercial.ExternalSubscriptionID(subTenant) {
			t.Fatalf("replay %d receipt = %q", i, receipt.ExternalID)
		}
	}
	if got := stub.countCreates(); got != 0 {
		t.Fatalf("a replayed ensure must issue ZERO create POSTs, got %d", got)
	}
}

// TestLagoEnsureSubscription422ResolvesByIdentity: a create 422 is resolved
// by an identity re-read — found+equal replays, still-absent is a definitive
// invalid response; a second create is never issued.
func TestLagoEnsureSubscription422ResolvesByIdentity(t *testing.T) {
	t.Run("re-read equal replays", func(t *testing.T) {
		stub := newSubscriptionsStub(t)
		stub.createStatuses = []int{http.StatusUnprocessableEntity}
		// The held subscription becomes visible on the index only AFTER the
		// refused create — the lost-response shape the re-read must resolve.
		stub.revealAfterPosts = 1
		stub.preloaded = []stubSubscription{{
			ExternalID:       commercial.ExternalSubscriptionID(subTenant),
			ExternalCustomer: commercial.ExternalCustomerID(subTenant),
			PlanCode:         subPlanCode,
			Status:           "active",
		}}
		receipt, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
		if err != nil {
			t.Fatalf("422 resolved by identity re-read: %v", err)
		}
		if receipt.ExternalID != commercial.ExternalSubscriptionID(subTenant) {
			t.Fatalf("receipt = %q", receipt.ExternalID)
		}
		if got := stub.countCreates(); got != 1 {
			t.Fatalf("exactly one create attempt, got %d", got)
		}
	})
	t.Run("still absent is invalid response", func(t *testing.T) {
		stub := newSubscriptionsStub(t)
		stub.createStatuses = []int{http.StatusUnprocessableEntity}
		_, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("still-absent after 422 must be invalid response, got %v", err)
		}
		if got := stub.countCreates(); got != 1 {
			t.Fatalf("no second create after an unresolved 422, got %d", got)
		}
	})
}

// TestLagoEnsureSubscriptionDifferentPlanConflict: a held subscription on a
// DIFFERENT plan code is a definitive conflict — the seam must never mint a
// parallel subscription.
func TestLagoEnsureSubscriptionDifferentPlanConflict(t *testing.T) {
	stub := newSubscriptionsStub(t)
	stub.preloaded = []stubSubscription{{
		ExternalID:       commercial.ExternalSubscriptionID(subTenant),
		ExternalCustomer: commercial.ExternalCustomerID(subTenant),
		PlanCode:         "weknora-pro-v1",
		Status:           "active",
	}}
	_, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("different plan code must be ErrPlatformInvalidResponse, got %v", err)
	}
	if got := stub.countCreates(); got != 0 {
		t.Fatalf("a conflict must never create, got %d posts", got)
	}
}

// TestLagoSubscriptionIndexUsesExplicitStatuses: the identity read passes
// explicit status[] query params — the index defaults to active only (T02
// cross-ticket fact).
func TestLagoSubscriptionIndexUsesExplicitStatuses(t *testing.T) {
	stub := newSubscriptionsStub(t)
	if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), ensureSubCommand(subPlanCode)); err != nil {
		t.Fatal(err)
	}
	queries := stub.indexQueries()
	if len(queries) == 0 {
		t.Fatal("the identity read must call the subscription index")
	}
	for _, q := range queries {
		statuses := q["status[]"]
		if len(statuses) == 0 {
			t.Fatalf("index query must carry explicit status[] params, got %v", q)
		}
		hasActive, hasIncomplete, hasCanceled, hasTerminated := false, false, false, false
		for _, s := range statuses {
			switch s {
			case "active":
				hasActive = true
			case "incomplete":
				hasIncomplete = true
			case "canceled":
				hasCanceled = true
			case "terminated":
				hasTerminated = true
			}
		}
		if !(hasActive && hasIncomplete && hasCanceled && hasTerminated) {
			t.Fatalf("index must pass the full explicit status set, got %v", statuses)
		}
		if q.Get("external_id") != commercial.ExternalSubscriptionID(subTenant) {
			t.Fatalf("index must address the subscription identity, got %v", q)
		}
	}
}

// TestLagoGrantHappyPath: read-before-create sees no wallet → ONE create
// POST carrying the exact decimal granted_credits, rate_amount "1",
// expiration_at and the recovery metadata; the settle-poll answers the
// credited balance; the receipt echoes the deterministic wallet name.
func TestLagoGrantHappyPath(t *testing.T) {
	stub := newWalletsStub(t)
	receipt, err := subAdapter(stub.url()).SubmitCommand(context.Background(), grantCommand(9_900_000))
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	wantName := commercial.MonthlyWalletName(subTenant, "2099-01")
	if receipt.ExternalID != wantName {
		t.Fatalf("receipt ExternalID = %q, want the deterministic wallet name %q", receipt.ExternalID, wantName)
	}
	var body string
	posts := 0
	for _, r := range stub.recorded() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/wallets" {
			posts++
			body = r.Body
		}
	}
	if posts != 1 {
		t.Fatalf("exactly one wallet create POST expected, got %d", posts)
	}
	for _, want := range []string{
		`"granted_credits":"9.9"`, `"rate_amount":"1"`, `"expiration_at":"2099-02-01T00:00:00Z"`,
		`"currency":"CNY"`, `"weknora_period":"2099-01"`, `"weknora_tenant":"weknora-tenant-201"`,
		`"` + wantName + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wallet create body must carry %s, got %s", want, body)
		}
	}
}

// TestLagoGrantSettleWaitWaitsForBalance: the settle-poll keeps polling
// while the after-commit settlement job has not credited the balance — the
// a1 fact — and completes once it has.
func TestLagoGrantSettleWaitWaitsForBalance(t *testing.T) {
	defer func(d time.Duration) { walletSettleTick = d }(walletSettleTick)
	walletSettleTick = time.Millisecond
	stub := newWalletsStub(t)
	stub.settleRounds = 2 // two unsettled polls, then settled
	if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), grantCommand(1_000_000)); err != nil {
		t.Fatalf("grant after settle wait: %v", err)
	}
	gets := 0
	for _, r := range stub.recorded() {
		if r.Method == http.MethodGet && strings.HasPrefix(r.Path, "/api/v1/wallets/lago-wallet") {
			gets++
		}
	}
	if gets != 3 {
		t.Fatalf("two unsettled polls then one settled read expected, got %d GETs", gets)
	}
}

// TestLagoGrantReplayNoSecondPost: the wallets list showing the
// deterministic name is a REPLAY — no second POST, no balance change.
func TestLagoGrantReplayNoSecondPost(t *testing.T) {
	stub := newWalletsStub(t)
	cmd := grantCommand(9_900_000)
	for i := 0; i < 2; i++ {
		if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), cmd); err != nil {
			t.Fatalf("grant %d: %v", i, err)
		}
	}
	if got := stub.countCreates(); got != 1 {
		t.Fatalf("a replayed grant must never issue a second create POST, got %d", got)
	}
}

// TestLagoGrantWalletLimitRetry: a wallet_limit_reached 422 is a bounded
// retry across the hourly termination tick; success lands the receipt, an
// exhausted budget is the closed indeterminate unreachable (the grant
// replays safely by identity next access).
func TestLagoGrantWalletLimitRetry(t *testing.T) {
	t.Run("success after retry window", func(t *testing.T) {
		defer func(d time.Duration) { walletLimitRetryTick = d }(walletLimitRetryTick)
		walletLimitRetryTick = time.Millisecond
		stub := newWalletsStub(t)
		stub.createStatuses = []int{http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, http.StatusCreated}
		if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), grantCommand(1_000_000)); err != nil {
			t.Fatalf("grant after bounded retries: %v", err)
		}
		if got := stub.countCreates(); got != 3 {
			t.Fatalf("two refused attempts then one create expected, got %d", got)
		}
	})
	t.Run("exhausted budget is unreachable", func(t *testing.T) {
		defer func(d time.Duration) { walletLimitRetryTick = d }(walletLimitRetryTick)
		walletLimitRetryTick = time.Millisecond
		stub := newWalletsStub(t)
		stub.limitExhausted = true
		_, err := subAdapter(stub.url()).SubmitCommand(context.Background(), grantCommand(1_000_000))
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("an exhausted wallet-limit budget must be the closed indeterminate unreachable, got %v", err)
		}
	})
}

// TestLagoBenefitsSnapshot: the benefits snapshot joins the subscription
// truth, the entitlements map and the active wallet balances (terminated
// wallets excluded, not-yet-terminated expired ones included — raw authority
// truth; the coordinator overlays expiry).
func TestLagoBenefitsSnapshot(t *testing.T) {
	stub := newCombinedStub(t)
	stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
	stub.subs.preloaded = []stubSubscription{{
		ExternalID:       commercial.ExternalSubscriptionID(subTenant),
		ExternalCustomer: commercial.ExternalCustomerID(subTenant),
		PlanCode:         subPlanCode,
		Status:           "active",
	}}
	stub.wallets.mu.Lock()
	stub.wallets.wallets = []stubWallet{
		{
			LagoID: "lago-wallet-a", Customer: commercial.ExternalCustomerID(subTenant),
			Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
			GrantedCents: 990, BalanceCents: 990,
			ExpiresAt: "2099-02-01T00:00:00Z",
			Metadata: map[string]string{
				commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant),
				commercial.WalletMetaPeriod: "2099-01",
			},
		},
		{
			LagoID: "lago-wallet-b", Customer: commercial.ExternalCustomerID(subTenant),
			Name: commercial.MonthlyWalletName(subTenant, "2098-12"), Status: "terminated",
			GrantedCents: 500, BalanceCents: 500,
			ExpiresAt: "2099-01-01T00:00:00Z",
			Metadata: map[string]string{
				commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant),
				commercial.WalletMetaPeriod: "2098-12",
			},
		},
	}
	stub.wallets.mu.Unlock()
	snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant,
	})
	if err != nil {
		t.Fatalf("benefits snapshot: %v", err)
	}
	if snap.Kind != commercial.SnapshotKindBenefits || snap.Benefits == nil {
		t.Fatalf("snapshot must carry the Benefits section, got %+v", snap)
	}
	b := snap.Benefits
	if b.SubscriptionState != commercial.SubscriptionStateActive {
		t.Fatalf("state = %q, want active", b.SubscriptionState)
	}
	if b.PlanCode != subPlanCode {
		t.Fatalf("plan code = %q", b.PlanCode)
	}
	if !b.Features["api_access"] {
		t.Fatalf("features must carry the entitlement map, got %+v", b.Features)
	}
	// 990 cents = 9_900_000 micro; the TERMINATED wallet's 500 cents is
	// excluded.
	if b.BalanceMicro != commercial.CentsToMicro(990) {
		t.Fatalf("balance = %d micro, want %d", b.BalanceMicro, commercial.CentsToMicro(990))
	}
	if len(b.Batches) != 1 || b.Batches[0].Period != "2099-01" || b.Batches[0].BalanceMicro != commercial.CentsToMicro(990) {
		t.Fatalf("batches = %+v", b.Batches)
	}
	if !b.Batches[0].ExpiresAt.Equal(time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("batch expiry = %v", b.Batches[0].ExpiresAt)
	}
	// Honest absence for an untouched tenant: pending state, zero balance.
	other, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant + 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if other.Benefits == nil || other.Benefits.SubscriptionState != commercial.SubscriptionStatePending ||
		other.Benefits.BalanceMicro != 0 || len(other.Benefits.Batches) != 0 {
		t.Fatalf("untouched tenant must answer pending/zero honestly, got %+v", other.Benefits)
	}
}

// (F-2 flow evidence) The entitlements read rides the v1.53 SUBSCRIPTION
// routes — the pinned release exposes NO customer-nested entitlements
// route (the old GET /api/v1/customers/:id/entitlements answered 404 on
// the real stack, leaving Benefits.Features permanently empty). The
// feature face is the UNION over the base "-sub" and purchase "-purchase"
// identities, and a 404 purchase leg (no purchase yet) contributes
// nothing.
func TestLagoBenefitsFeaturesReadSubscriptionEntitlementRoutes(t *testing.T) {
	stub := newCombinedStub(t)
	stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
	stub.wallets.mu.Lock()
	// Both legs in the PINNED v1.53 shape (F-2' evidence): {"entitlements":
	// [{"code":…}]}.
	stub.wallets.entitlements = `{"entitlements":[{"code":"api_access"}]}`
	stub.wallets.purchaseEntitlements = `{"entitlements":[{"code":"advanced_models"}]}`
	stub.wallets.mu.Unlock()
	snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant,
	})
	if err != nil {
		t.Fatalf("benefits snapshot: %v", err)
	}
	if !snap.Benefits.Features["api_access"] || !snap.Benefits.Features["advanced_models"] {
		t.Fatalf("features must union BOTH subscription legs, got %+v", snap.Benefits.Features)
	}
	// The read rode the subscription entitlements routes — never the
	// customers-nested route the pinned release does not expose.
	stub.wallets.mu.Lock()
	defer stub.wallets.mu.Unlock()
	sawBase, sawPurchase, sawAbsentRoute := false, false, false
	for _, req := range stub.wallets.requests {
		switch {
		case req.Method == http.MethodGet && req.Path == "/api/v1/subscriptions/"+commercial.ExternalSubscriptionID(subTenant)+"/entitlements":
			sawBase = true
		case req.Method == http.MethodGet && req.Path == "/api/v1/subscriptions/"+commercial.ExternalPurchaseSubscriptionID(subTenant)+"/entitlements":
			sawPurchase = true
		case strings.HasPrefix(req.Path, "/api/v1/customers/") && strings.HasSuffix(req.Path, "/entitlements"):
			sawAbsentRoute = true
		}
	}
	if !sawBase || !sawPurchase {
		t.Fatalf("both subscription entitlement legs must be read, base=%v purchase=%v (requests: %+v)", sawBase, sawPurchase, stub.wallets.requests)
	}
	if sawAbsentRoute {
		t.Fatalf("the customers-nested entitlements route must never be called (absent on pinned v1.53)")
	}
}

// (F-2') The parser answers the pinned v1.53 `code` shape FIRST and keeps
// tolerating the older/newer shapes (feature_code, nested feature.code)
// — never assumed, never required.
func TestLagoBenefitsFeaturesParseToleratedShapes(t *testing.T) {
	shapes := []string{
		`{"entitlements":[{"code":"feat_code"}]}`,               // pinned v1.53 (live-stack evidence)
		`{"entitlements":[{"feature_code":"feat_feature"}]}`,    // tolerated
		`{"entitlements":[{"feature":{"code":"feat_nested"}}]}`, // tolerated
	}
	for i, body := range shapes {
		stub := newCombinedStub(t)
		stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
		stub.wallets.mu.Lock()
		stub.wallets.entitlements = body
		stub.wallets.mu.Unlock()
		snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindBenefits, TenantID: subTenant,
		})
		if err != nil {
			t.Fatalf("shape %d: %v", i, err)
		}
		if len(snap.Benefits.Features) != 1 {
			t.Fatalf("shape %d: exactly one feature must parse, got %+v", i, snap.Benefits.Features)
		}
		if !snap.Benefits.Features["feat_code"] && !snap.Benefits.Features["feat_feature"] && !snap.Benefits.Features["feat_nested"] {
			t.Fatalf("shape %d: feature code must parse, got %+v", i, snap.Benefits.Features)
		}
	}
}

// (F-4 flow evidence) The PURCHASE first-period wallet (its own
// deterministic name "<ext>-purchase-<YYYY-MM>" and meta key
// weknora_purchase_period — D4) joins the benefits batches beside the
// base monthly wallet of the SAME period: both batches answer, the
// balance carries both.
func TestLagoBenefitsSnapshotIncludesPurchaseBatch(t *testing.T) {
	stub := newCombinedStub(t)
	base := stubWallet{
		LagoID: "lago-wallet-base", Customer: commercial.ExternalCustomerID(subTenant),
		Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
		GrantedCents: 100, BalanceCents: 100,
		ExpiresAt: "2099-02-01T00:00:00Z",
		Metadata: map[string]string{
			commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant),
			commercial.WalletMetaPeriod: "2099-01",
		},
	}
	purchase := stubWallet{
		LagoID: "lago-wallet-purchase", Customer: commercial.ExternalCustomerID(subTenant),
		Name: commercial.PurchaseWalletName(subTenant, "2099-01"), Status: "active",
		GrantedCents: 990, BalanceCents: 990,
		ExpiresAt: "2099-02-01T00:00:00Z",
		Metadata: map[string]string{
			commercial.WalletMetaTenant:         commercial.ExternalCustomerID(subTenant),
			commercial.WalletMetaPurchasePeriod: "2099-01",
		},
	}
	stub.wallets.mu.Lock()
	stub.wallets.wallets = []stubWallet{base, purchase}
	stub.wallets.mu.Unlock()
	snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant,
	})
	if err != nil {
		t.Fatalf("benefits snapshot: %v", err)
	}
	b := snap.Benefits
	if len(b.Batches) != 2 {
		t.Fatalf("the base AND purchase batches must both answer for the same period, got %+v", b.Batches)
	}
	for _, batch := range b.Batches {
		if batch.Period != "2099-01" {
			t.Fatalf("both batches belong to the activation month, got %+v", b.Batches)
		}
	}
	if b.BalanceMicro != commercial.CentsToMicro(100)+commercial.CentsToMicro(990) {
		t.Fatalf("balance must carry both wallets, got %d", b.BalanceMicro)
	}
}

// TestLagoGrantWalletCarriesEncodedPriority (#86 Task 2): the wallet create
// POST body must carry payload.Priority — the consumption-order class is
// explicitly encoded at creation, never left to the provider default.
func TestLagoGrantWalletCarriesEncodedPriority(t *testing.T) {
	stub := newWalletsStub(t)
	cmd := grantCommand(9_900_000)
	cmd.Payload = commercial.GrantIncludedCreditsPayload{
		TenantID:           subTenant,
		ExternalCustomerID: commercial.ExternalCustomerID(subTenant),
		Period:             "2099-01",
		CreditsMicro:       9_900_000,
		ExpiresAt:          time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
		Priority:           3, // a yielding month's monthly batch
	}
	if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), cmd); err != nil {
		t.Fatalf("grant: %v", err)
	}
	var body string
	for _, r := range stub.recorded() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/wallets" {
			body = r.Body
		}
	}
	if !strings.Contains(body, `"priority":3`) {
		t.Fatalf("wallet create body must carry encoded priority, got %s", body)
	}
}

// TestLagoRebalancePutsMixedFamiliesInExpiryOrder (#86 Task 2, the r1-review
// High counterexample on the wire): three wallets stored at their
// creation-time initials (aging top-up A=2, fresh top-up B=2, monthly M=3)
// — the static encoding's consumption order A→B→M is WRONG (B must be
// consumed after M). One rebalance command must PUT exactly three wallets to
// A=1, M=2, B=3 (the true expiry order).
func TestLagoRebalancePutsMixedFamiliesInExpiryOrder(t *testing.T) {
	stub := newWalletsStub(t)
	ext := commercial.ExternalCustomerID(subTenant)
	stub.mu.Lock()
	stub.wallets = []stubWallet{
		{ // A — aging top-up (expires before the monthly period end)
			LagoID: "w-a", Customer: ext,
			Name: ext + "-topup-a", Status: "active",
			GrantedCents: 500, BalanceCents: 500,
			ExpiresAt: "2099-01-15T00:00:00Z", CreatedAt: "2098-01-15T00:00:00Z",
			Priority: commercial.TopUpWalletPriority,
			Metadata: map[string]string{commercial.WalletMetaTenant: ext},
		},
		{ // M — monthly (expires at the period end)
			LagoID: "w-m", Customer: ext,
			Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
			GrantedCents: 100, BalanceCents: 100,
			ExpiresAt: "2099-02-01T00:00:00Z", CreatedAt: "2099-01-01T00:00:00Z",
			Priority: commercial.TopUpWalletPriority + 1,
			Metadata: map[string]string{
				commercial.WalletMetaTenant: ext,
				commercial.WalletMetaPeriod: "2099-01",
			},
		},
		{ // B — fresh top-up (expires after the period end)
			LagoID: "w-b", Customer: ext,
			Name: ext + "-topup-b", Status: "active",
			GrantedCents: 500, BalanceCents: 500,
			ExpiresAt: "2099-07-10T00:00:00Z", CreatedAt: "2099-01-10T00:00:00Z",
			Priority: commercial.TopUpWalletPriority,
			Metadata: map[string]string{commercial.WalletMetaTenant: ext},
		},
	}
	stub.mu.Unlock()
	if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(),
		commercial.Command{
			Kind:  commercial.CommandKindRebalanceCreditsOrder,
			Key:   commercial.RebalanceCreditsOrderCommandKey(commercial.ExternalCustomerID(subTenant)),
			Actor: "test", Reason: "refresh_calibration",
			Payload: commercial.RebalanceCreditsOrderPayload{
				TenantID: subTenant, ExternalCustomerID: commercial.ExternalCustomerID(subTenant),
			},
		}); err != nil {
		t.Fatalf("rebalance: %v", err)
	}
	puts := map[string]int{} // wallet lago_id → the PUT body's priority
	for _, r := range stub.recorded() {
		if r.Method == http.MethodPut && strings.HasPrefix(r.Path, "/api/v1/wallets/") {
			var p struct {
				Wallet struct {
					Priority int `json:"priority"`
				} `json:"wallet"`
			}
			if err := json.Unmarshal([]byte(r.Body), &p); err != nil {
				t.Fatalf("PUT body malformed: %v (%s)", err, r.Body)
			}
			puts[strings.TrimPrefix(r.Path, "/api/v1/wallets/")] = p.Wallet.Priority
		}
	}
	if len(puts) != 3 || puts["w-a"] != 1 || puts["w-m"] != 2 || puts["w-b"] != 3 {
		t.Fatalf("rebalance PUTs = %+v, want w-a=1 w-m=2 w-b=3", puts)
	}
	// The stub's stored priorities converged too (the authority state).
	stub.mu.Lock()
	defer stub.mu.Unlock()
	for _, w := range stub.wallets {
		want := map[string]int{"w-a": 1, "w-m": 2, "w-b": 3}[w.LagoID]
		if w.Priority != want {
			t.Fatalf("stored priority of %s = %d, want %d", w.LagoID, w.Priority, want)
		}
	}
}

// TestLagoRebalanceSkipsAlignedWallets (#86 Task 2): priorities already
// equal to the WalletRank ranks answer ZERO PUTs — the calibration is an
// idempotent no-op when converged (no gratuitous writes on the hot refresh
// path).
func TestLagoRebalanceSkipsAlignedWallets(t *testing.T) {
	stub := newWalletsStub(t)
	ext := commercial.ExternalCustomerID(subTenant)
	stub.mu.Lock()
	stub.wallets = []stubWallet{
		{
			LagoID: "w-x", Customer: ext,
			Name: ext + "-topup-x", Status: "active",
			GrantedCents: 500, BalanceCents: 500,
			ExpiresAt: "2099-01-15T00:00:00Z", CreatedAt: "2098-01-15T00:00:00Z",
			Priority: 1,
			Metadata: map[string]string{commercial.WalletMetaTenant: ext},
		},
		{
			LagoID: "w-y", Customer: ext,
			Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
			GrantedCents: 100, BalanceCents: 100,
			ExpiresAt: "2099-02-01T00:00:00Z", CreatedAt: "2099-01-01T00:00:00Z",
			Priority: 2,
			Metadata: map[string]string{
				commercial.WalletMetaTenant: ext,
				commercial.WalletMetaPeriod: "2099-01",
			},
		},
	}
	stub.mu.Unlock()
	if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(),
		commercial.Command{
			Kind:  commercial.CommandKindRebalanceCreditsOrder,
			Key:   commercial.RebalanceCreditsOrderCommandKey(commercial.ExternalCustomerID(subTenant)),
			Actor: "test", Reason: "refresh_calibration",
			Payload: commercial.RebalanceCreditsOrderPayload{
				TenantID: subTenant, ExternalCustomerID: commercial.ExternalCustomerID(subTenant),
			},
		}); err != nil {
		t.Fatalf("rebalance: %v", err)
	}
	for _, r := range stub.recorded() {
		if r.Method == http.MethodPut {
			t.Fatalf("an aligned wallet set must answer ZERO PUTs, saw %s %s", r.Method, r.Path)
		}
	}
}

// TestLagoBenefitsSnapshotListsTopUpBatch (#86 Task 1): an active wallet
// carrying NO weknora_period metadata but THIS tenant's weknora_tenant key
// (the #85 top-up batch shape) must join Batches with Source=topup and
// GrantedAt = the wallet's created_at; the monthly batch answers
// Source=monthly. The balance sum still carries both (existing behavior).
func TestLagoBenefitsSnapshotListsTopUpBatch(t *testing.T) {
	stub := newCombinedStub(t)
	stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
	stub.subs.preloaded = []stubSubscription{{
		ExternalID:       commercial.ExternalSubscriptionID(subTenant),
		ExternalCustomer: commercial.ExternalCustomerID(subTenant),
		PlanCode:         subPlanCode,
		Status:           "active",
	}}
	stub.wallets.mu.Lock()
	stub.wallets.wallets = []stubWallet{
		{ // top-up shape: no period key, this tenant's tenant key
			LagoID: "w-topup", Customer: commercial.ExternalCustomerID(subTenant),
			Name: commercial.ExternalCustomerID(subTenant) + "-topup-ord1", Status: "active",
			GrantedCents: 5000, BalanceCents: 5000,
			ExpiresAt: "2100-01-31T00:00:00Z", CreatedAt: "2099-01-10T00:00:00Z",
			Metadata: map[string]string{commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant)},
		},
		{ // monthly shape (the established seed convention)
			LagoID: "w-monthly", Customer: commercial.ExternalCustomerID(subTenant),
			Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
			GrantedCents: 990, BalanceCents: 990, ExpiresAt: "2099-02-01T00:00:00Z",
			CreatedAt: "2099-01-01T00:00:00Z",
			Metadata: map[string]string{
				commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant),
				commercial.WalletMetaPeriod: "2099-01",
			},
		},
	}
	stub.wallets.mu.Unlock()
	snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant})
	if err != nil {
		t.Fatal(err)
	}
	var sawTopUp, sawMonthly bool
	for _, b := range snap.Benefits.Batches {
		switch b.Source {
		case commercial.BatchSourceTopUp:
			sawTopUp = true
			if b.BalanceMicro != commercial.CentsToMicro(5000) {
				t.Fatalf("topup balance: %d", b.BalanceMicro)
			}
			if !b.GrantedAt.Equal(time.Date(2099, 1, 10, 0, 0, 0, 0, time.UTC)) {
				t.Fatalf("topup granted_at: %v", b.GrantedAt)
			}
			if b.Period != "" {
				t.Fatalf("topup batch carries no calendar period, got %q", b.Period)
			}
		case commercial.BatchSourceMonthly:
			sawMonthly = true
		default:
			t.Fatalf("unknown source %q", b.Source)
		}
	}
	if !sawTopUp || !sawMonthly {
		t.Fatalf("batches incomplete: topup=%v monthly=%v", sawTopUp, sawMonthly)
	}
	if snap.Benefits.BalanceMicro != commercial.CentsToMicro(5990) { // both count (existing behavior)
		t.Fatalf("balance = %d", snap.Benefits.BalanceMicro)
	}
}

// The 404 purchase leg (no purchase subscription yet) must contribute
// nothing — the base features still answer, never an error.
func TestLagoBenefitsFeaturesTolerateMissingPurchaseLeg(t *testing.T) {
	stub := newCombinedStub(t)
	stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
	// purchaseEntitlements stays "" — the stub answers the purchase leg 404.
	snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: subTenant,
	})
	if err != nil {
		t.Fatalf("a missing purchase leg must not fail the snapshot, got %v", err)
	}
	if !snap.Benefits.Features["api_access"] {
		t.Fatalf("base features must still answer, got %+v", snap.Benefits.Features)
	}
	if snap.Benefits.Features["advanced_models"] {
		t.Fatalf("no purchase entitlements may be fabricated, got %+v", snap.Benefits.Features)
	}
}

// TestLagoNewKindsErrorHygiene: no error string on the new kinds leaks a
// URL, a provider path or the credential.
func TestLagoNewKindsErrorHygiene(t *testing.T) {
	stub := newCombinedStub(t)
	stub.subs.indexStatus = http.StatusBadGateway
	p := subAdapter(stub.url())
	_, subErr := p.SubmitCommand(context.Background(), ensureSubCommand(subPlanCode))
	if subErr == nil {
		t.Fatal("expected an error from a 502 index")
	}
	for _, s := range []string{stub.url(), "/api/v1/", "lago", testAPIKey} {
		if strings.Contains(subErr.Error(), s) {
			t.Fatalf("subscription error leaks %q: %v", s, subErr)
		}
	}
	stub.wallets.mu.Lock()
	stub.wallets.createStatuses = []int{http.StatusInternalServerError}
	stub.wallets.mu.Unlock()
	_, grantErr := p.SubmitCommand(context.Background(), grantCommand(1_000_000))
	if grantErr == nil {
		t.Fatal("expected an error from a 500 create")
	}
	for _, s := range []string{stub.url(), "/api/v1/", "lago", testAPIKey} {
		if strings.Contains(grantErr.Error(), s) {
			t.Fatalf("grant error leaks %q: %v", s, grantErr)
		}
	}
}
