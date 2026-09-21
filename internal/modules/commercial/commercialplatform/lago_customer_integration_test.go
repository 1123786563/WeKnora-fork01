//go:build lago_integration

// Tagged real-Lago customer integration evidence (T06, #78). The build tag
// keeps this out of every normal suite run; the test is env-gated on
// LAGO_INTEGRATION_BASE_URL + LAGO_INTEGRATION_API_KEY (operator-owned,
// never committed) and skips otherwise, so a missing stack records
// blocked-env instead of failing or faking a pass.
//
// Split of proof with the unit suites: a dropped-response simulation
// (persisted-but-response-lost) is NOT run here — against the shared stack
// it would orphan real authority objects. That path is proven by the
// fault-injection tests (Task 2 stub recovery cases + Task 3 recovery
// test). This file proves what only the real pinned v1.53.0 can: the
// identity round-trip, replay stability, the RAW create-on-external_id
// upsert verdict (#74 research), the rename probe, and lenient cleanup.
package commercialplatform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// integrationTenantID is the lab-only tenant the probes run under — a value
// that cannot collide with product spaces in a shared organization
// (overridable via LAGO_INTEGRATION_TENANT_ID).
func integrationTenantID(t *testing.T) uint64 {
	t.Helper()
	if v := os.Getenv("LAGO_INTEGRATION_TENANT_ID"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil || id == 0 {
			t.Fatalf("LAGO_INTEGRATION_TENANT_ID must be a positive integer, got %q", v)
		}
		return id
	}
	return 780001
}

// rawCustomerRequest performs a direct REST call against the pinned stack
// (probe plumbing only — the product path never bypasses the adapter).
func rawCustomerRequest(t *testing.T, method, baseURL, apiKey, path, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("build raw request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("raw %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	blob, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, string(blob)
}

// TestLagoCustomerIntegration: against the REAL pinned Lago v1.53.0 stack,
// ensure_customer is idempotent by identity — create, snapshot linked,
// same-Key replay identity-stable, RAW upsert verdict recorded, rename
// probe identity-stable, cleanup lenient.
func TestLagoCustomerIntegration(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY not set (operator stack not provided)")
	}
	tenant := integrationTenantID(t)
	ext := commercial.ExternalCustomerID(tenant)
	p := NewLagoAdapter(Config{
		Provider: ProviderLago,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Release:  lockedRelease(t),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Lenient pre-cleanup: a previously crashed run may have left the
	// customer behind (404 counts as absent — the #73 carryover rule).
	rawCustomerRequest(t, http.MethodDelete, baseURL, apiKey, "/api/v1/customers/"+ext, "")

	// (1) First ensure under the canonical Key: receipt with the REQUESTED
	// deterministic identity.
	cmd := commercial.Command{
		Kind:    commercial.CommandKindEnsureCustomer,
		Key:     "ensure_customer:" + ext,
		Actor:   "t06-integration",
		Reason:  "first_billing_access",
		Payload: commercial.EnsureCustomerPayload{TenantID: tenant, ExternalCustomerID: ext, DisplayName: "T06 Integration Space"},
	}
	receipt, err := p.SubmitCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("first ensure_customer against the real stack: %v", err)
	}
	if receipt.ExternalID != ext || receipt.Key != cmd.Key {
		t.Fatalf("receipt must carry the requested identity %q, got %+v", ext, receipt)
	}

	// (2) The account snapshot answers linked for the same tenant.
	snap, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: tenant})
	if err != nil {
		t.Fatalf("account snapshot against the real stack: %v", err)
	}
	if snap.Account == nil || snap.Account.State != commercial.AccountStateLinked {
		t.Fatalf("real stack must answer linked after ensure, got %+v", snap.Account)
	}

	// (3) Same-Key replay: identity unchanged.
	replay, err := p.SubmitCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("same-Key replay: %v", err)
	}
	if replay.ExternalID != ext {
		t.Fatalf("same-Key replay must keep the identity, got %+v", replay)
	}

	// (3b) RAW upsert probe — the RUNTIME verdict on #74's research claim
	// that create is upsert-on-external_id. This deliberately bypasses the
	// adapter (the adapter's read-before-create never POSTs an existing
	// identity, so it cannot observe this). Whatever the verdict, the
	// product path is safe: still exactly one customer afterwards.
	status, body := rawCustomerRequest(t, http.MethodPost, baseURL, apiKey, "/api/v1/customers",
		`{"customer":{"external_id":"`+ext+`","name":"T06 Upsert Probe"}}`)
	switch {
	case status >= 200 && status < 300:
		t.Logf("upsert verdict: create-on-external_id is UPSERT on this stack (POST answered %d)", status)
	case status == http.StatusNotFound || status == http.StatusUnprocessableEntity || status == http.StatusBadRequest || status == http.StatusConflict:
		t.Logf("upsert verdict: create-on-external_id is NOT upsert on this stack (POST answered %d: %s)", status, firstLine(body))
	default:
		t.Logf("upsert verdict: INCONCLUSIVE (POST answered %d)", status)
	}

	// (3c) Still exactly one customer under the identity, whatever the
	// upsert verdict: the snapshot stays linked and the direct identity
	// read returns one object carrying the expected external_id.
	snap, err = p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: tenant})
	if err != nil || snap.Account == nil || snap.Account.State != commercial.AccountStateLinked {
		t.Fatalf("account truth must stay linked after the upsert probe: %v %+v", err, snap.Account)
	}
	status, body = rawCustomerRequest(t, http.MethodGet, baseURL, apiKey, "/api/v1/customers/"+ext, "")
	if status != http.StatusOK {
		t.Fatalf("direct identity read after upsert probe: status %d", status)
	}
	var one struct {
		Customer struct {
			ExternalID string `json:"external_id"`
		} `json:"customer"`
	}
	if err := json.Unmarshal([]byte(body), &one); err != nil || one.Customer.ExternalID != ext {
		t.Fatalf("identity read must return the one customer with external_id %q (raw parse err=%v)", ext, err)
	}

	// (4) Rename probe: a DIFFERENT display name under the canonical Key
	// keeps the identity (advisory metadata, never identity).
	renameCmd := cmd
	renameCmd.Payload = commercial.EnsureCustomerPayload{TenantID: tenant, ExternalCustomerID: ext, DisplayName: "T06 Renamed Space"}
	renamed, err := p.SubmitCommand(ctx, renameCmd)
	if err != nil {
		t.Fatalf("rename probe: %v", err)
	}
	if renamed.ExternalID != ext {
		t.Fatalf("rename must never move identity: %+v", renamed)
	}
	snap, err = p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: tenant})
	if err != nil || snap.Account == nil || snap.Account.State != commercial.AccountStateLinked {
		t.Fatalf("snapshot must stay linked after rename: %v %+v", err, snap.Account)
	}

	// (5) Lenient cleanup: DELETE the lab customer; 404 counts as absent.
	status, _ = rawCustomerRequest(t, http.MethodDelete, baseURL, apiKey, "/api/v1/customers/"+ext, "")
	if status != http.StatusOK && status != http.StatusNotFound {
		t.Fatalf("cleanup DELETE answered %d (200 deleted or 404 already-absent expected)", status)
	}
	snap, err = p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindAccount, TenantID: tenant})
	if err != nil || snap.Account == nil || snap.Account.State != commercial.AccountStateAbsent {
		t.Fatalf("post-cleanup truth must be absent: %v %+v", err, snap.Account)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
