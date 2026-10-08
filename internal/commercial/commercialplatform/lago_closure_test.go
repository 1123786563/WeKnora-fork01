// #102 / Lago 30 — the authority-side closure disposal: close_workspace
// terminates the customer's not-yet-terminated subscriptions and ANCHORED
// active wallets (a replay converges to zero terminations), then rewrites
// the customer's display name to the deterministic de-identified form. A
// mismatched identity or a non-de-identified name is refused before any
// request leaves the adapter.
package commercialplatform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// closureStub models the authority surfaces close_workspace touches: the
// subscription index (customer-scoped), subscription termination, the
// customer wallet index, wallet termination, and the customer update.
type closureStub struct {
	mu          sync.Mutex
	subs        []lagoSubscription
	wallets     []lagoWallet
	terminatedS map[string]bool
	terminatedW map[string]bool
	customerPut []string // request bodies of the customer de-identification PUTs
	server      *httptest.Server
}

func newClosureStub(t *testing.T) *closureStub {
	t.Helper()
	c := &closureStub{terminatedS: map[string]bool{}, terminatedW: map[string]bool{}}
	c.subs = []lagoSubscription{
		{LagoID: "lago_s_1", ExternalID: "weknora-tenant-7-sub", ExternalCustomerID: "weknora-tenant-7", PlanCode: "base", Status: "active"},
		{LagoID: "lago_s_2", ExternalID: "weknora-tenant-7-old", ExternalCustomerID: "weknora-tenant-7", PlanCode: "base", Status: "terminated"},
	}
	c.wallets = []lagoWallet{
		{LagoID: "lago_w_1", Name: "m-7", Status: "active", MetadataMap: map[string]string{commercial.WalletMetaTenant: "weknora-tenant-7"}},
		{LagoID: "lago_w_2", Name: "foreign", Status: "active", MetadataMap: map[string]string{commercial.WalletMetaTenant: "weknora-tenant-8"}},
		{LagoID: "lago_w_3", Name: "m-7-old", Status: "terminated", MetadataMap: map[string]string{commercial.WalletMetaTenant: "weknora-tenant-7"}},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/customers", func(w http.ResponseWriter, r *http.Request) {
		// v1.53 create-on-external_id is UPSERT (the t06 runtime verdict) —
		// the de-identification vehicle for a pinned stack with no update
		// route.
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		name, _ := body["customer"].(map[string]any)["name"].(string)
		c.mu.Lock()
		defer c.mu.Unlock()
		c.customerPut = append(c.customerPut, name)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("external_customer_id") != "weknora-tenant-7" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		out := make([]lagoSubscription, 0, len(c.subs))
		for _, s := range c.subs {
			if c.terminatedS[s.ExternalID] {
				s.Status = "terminated"
			}
			out = append(out, s)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": out})
	})
	mux.HandleFunc("/api/v1/subscriptions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/subscriptions/")
		c.mu.Lock()
		defer c.mu.Unlock()
		c.terminatedS[id] = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/customers/weknora-tenant-7/wallets", func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		out := make([]lagoWallet, 0, len(c.wallets))
		for _, wlt := range c.wallets {
			if c.terminatedW[wlt.LagoID] {
				wlt.Status = "terminated"
			}
			out = append(out, wlt)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"wallets": out, "meta": map[string]any{"next_page": nil}})
	})
	mux.HandleFunc("/api/v1/wallets/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/wallets/")
		c.mu.Lock()
		defer c.mu.Unlock()
		c.terminatedW[id] = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/customers/weknora-tenant-7", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		name, _ := body["customer"].(map[string]any)["name"].(string)
		c.mu.Lock()
		defer c.mu.Unlock()
		c.customerPut = append(c.customerPut, name)
		w.WriteHeader(http.StatusOK)
	})
	c.server = httptest.NewServer(mux)
	t.Cleanup(c.server.Close)
	return c
}

func closureCommand(tenant uint64) commercial.Command {
	ext := commercial.ExternalCustomerID(tenant)
	return commercial.Command{
		Kind: commercial.CommandKindCloseWorkspace,
		Key:  commercial.CloseWorkspaceKey(ext),
		Payload: commercial.CloseWorkspacePayload{
			TenantID: tenant, ExternalCustomerID: ext,
			DisplayName: commercial.DeidentifiedDisplayName(ext),
		},
	}
}

// TestCloseWorkspaceDisposesAndDeidentifies pins the disposal shape: the
// active subscription and ONLY the tenant-anchored active wallet are
// terminated, already-terminated objects are skipped, and the customer
// display name is rewritten to the de-identified form.
func TestCloseWorkspaceDisposesAndDeidentifies(t *testing.T) {
	stub := newClosureStub(t)
	a := NewLagoAdapter(lagoTestConfig(stub.server.URL))

	receipt, err := a.SubmitCommand(context.Background(), closureCommand(7))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExternalID != "weknora-tenant-7" || receipt.Key != commercial.CloseWorkspaceKey("weknora-tenant-7") {
		t.Fatalf("receipt: %+v", receipt)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if !stub.terminatedS["weknora-tenant-7-sub"] {
		t.Fatal("the active subscription must be terminated")
	}
	if stub.terminatedS["weknora-tenant-7-old"] {
		t.Fatal("an already-terminated subscription must never be re-terminated")
	}
	if !stub.terminatedW["lago_w_1"] {
		t.Fatal("the tenant-anchored active wallet must be terminated")
	}
	if stub.terminatedW["lago_w_2"] {
		t.Fatal("a foreign-anchored wallet must never be touched")
	}
	if stub.terminatedW["lago_w_3"] {
		t.Fatal("an already-terminated wallet must never be re-terminated")
	}
	if len(stub.customerPut) != 1 || stub.customerPut[0] != "closed:weknora-tenant-7" {
		t.Fatalf("customer display name must be de-identified once, got %v", stub.customerPut)
	}
}

// TestCloseWorkspaceReplayConverges pins idempotency: a replay of the same
// closure terminates nothing (everything already terminal) and the
// de-identification PUT stays the same deterministic value.
func TestCloseWorkspaceReplayConverges(t *testing.T) {
	stub := newClosureStub(t)
	a := NewLagoAdapter(lagoTestConfig(stub.server.URL))
	ctx := context.Background()

	if _, err := a.SubmitCommand(ctx, closureCommand(7)); err != nil {
		t.Fatal(err)
	}
	stub.mu.Lock()
	putsAfterFirst := len(stub.customerPut)
	stub.mu.Unlock()

	replay, err := a.SubmitCommand(ctx, closureCommand(7))
	if err != nil {
		t.Fatal(err)
	}
	if replay.Key != commercial.CloseWorkspaceKey("weknora-tenant-7") {
		t.Fatalf("replay receipt key: %+v", replay)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.terminatedS["weknora-tenant-7-old"] || stub.terminatedW["lago_w_2"] || stub.terminatedW["lago_w_3"] {
		t.Fatal("a replay must terminate nothing new")
	}
	if len(stub.customerPut) != putsAfterFirst+1 || stub.customerPut[len(stub.customerPut)-1] != "closed:weknora-tenant-7" {
		t.Fatalf("replay must re-assert the same de-identified name, got %v", stub.customerPut)
	}
}

// TestCloseWorkspaceRefusesWrongIdentityAndRealName pins the payload guard:
// a mismatched identity, and a display name that is not the de-identified
// derivation, are both refused before any request leaves.
func TestCloseWorkspaceRefusesWrongIdentityAndRealName(t *testing.T) {
	stub := newClosureStub(t)
	a := NewLagoAdapter(lagoTestConfig(stub.server.URL))

	wrongIdentity := closureCommand(7)
	wrongIdentity.Payload = commercial.CloseWorkspacePayload{
		TenantID: 7, ExternalCustomerID: "weknora-tenant-8",
		DisplayName: commercial.DeidentifiedDisplayName("weknora-tenant-8"),
	}
	if _, err := a.SubmitCommand(context.Background(), wrongIdentity); err == nil {
		t.Fatal("a mismatched identity must be refused")
	}

	realName := closureCommand(7)
	realName.Payload = commercial.CloseWorkspacePayload{
		TenantID: 7, ExternalCustomerID: "weknora-tenant-7", DisplayName: "Acme Corp",
	}
	if _, err := a.SubmitCommand(context.Background(), realName); err == nil {
		t.Fatal("a non-de-identified display name must be refused")
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.customerPut) != 0 {
		t.Fatal("no request may leave the adapter for a refused payload")
	}
}
