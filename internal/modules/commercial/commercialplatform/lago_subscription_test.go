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

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
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
	Metadata     map[string]string
}

func walletJSON(w stubWallet) string {
	m, _ := json.Marshal(w.Metadata)
	return fmt.Sprintf(
		`{"lago_id":%q,"name":%q,"status":%q,"balance_cents":%d,"granted_credits":"%s","rate_amount":"1","expiration_at":%q,"metadata":%s,"currency":"CNY"}`,
		w.LagoID, w.Name, w.Status, w.BalanceCents, centsString(w.GrantedCents), w.ExpiresAt, string(m))
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

	wallets        []stubWallet
	createStatuses []int  // scripted statuses; a 422 answers wallet_limit_reached
	limitExhausted bool   // every POST answers the limit 422
	settleRounds   int    // initial wallet GETs answering an unsettled balance
	entitlements   string // scripted entitlements body
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
		entitlements:   `{"entitlements":[{"feature_code":"api_access","value":"base"}]}`,
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
	mux.HandleFunc("/api/v1/customers/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/customers/")
		customer, subpath, _ := strings.Cut(rest, "/")
		switch {
		case subpath == "wallets" && r.Method == http.MethodGet:
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
		case subpath == "entitlements" && r.Method == http.MethodGet:
			s.mu.Lock()
			body := s.entitlements
			if s.entitlementCustomer != "" && customer != s.entitlementCustomer {
				body = `{"entitlements":[]}` // per-customer truth: no entitlements attached
			}
			s.mu.Unlock()
			respond(w, r, &s.requests, &s.mu, http.StatusOK, body, nil)
		default:
			http.NotFound(w, r)
		}
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
