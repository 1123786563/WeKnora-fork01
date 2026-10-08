//go:build lago_integration

// Tagged real-Lago consumption-order evidence (#86 Task 6). The build tag
// keeps this out of every normal suite run; the test is env-gated on
// LAGO_INTEGRATION_BASE_URL + LAGO_INTEGRATION_API_KEY (operator-owned,
// never committed) and skips otherwise, so a missing stack records
// blocked-env instead of failing or faking a pass.
//
// Phases (ONE test, run in order — the TestLagoBasePlanIntegration
// single-test precedent):
//
//	a. Full chain through the REAL adapter + BenefitsService: the monthly
//	   wallet is created with the priority-1 initial encoding — verified by
//	   a direct GET of the wallet on the pinned runtime.
//	b. A top-up shaped wallet (#85's payment-confirmed object shape, created
//	   directly through the authority API: no period metadata, priority 2,
//	   +12-month expiry) joins the benefits snapshot as source=topup and the
//	   balance carries it.
//	c. A FRESH tenant: an aging top-up (expiring BEFORE this month's end,
//	   priority 2) seeded first, then EnsureBenefits — the monthly wallet's
//	   creation initial must be the yielding class 3.
//	d. The r1-review High counterexample ON THE REAL RUNTIME (spec L132's
//	   "runtime verification" gate): aging A(2) + monthly M(3) + fresh B(2)
//	   coexist; one rebalance_credits_order must PUT the priorities onto the
//	   true expiry order A=1 M=2 B=3; a second rebalance changes nothing.
//	   If the pinned v1.53 refuses the priority PUT (422/403 or unchanged),
//	   the test FAILS LOUDLY — per the plan's BLOCKED gate this point may
//	   never silently degrade.
package commercialplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// rawWalletView is the probe-side wallet shape (name/priority/status only —
// probe plumbing, never the product path).
type rawWalletView struct {
	LagoID   string `json:"lago_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
}

// listRawWallets reads the customer's wallets directly (probe side).
func listRawWallets(t *testing.T, baseURL, apiKey string, tenantID uint64) []rawWalletView {
	t.Helper()
	status, body := rawIntegrationRequest(t, http.MethodGet, baseURL, apiKey,
		fmt.Sprintf("/api/v1/customers/%s/wallets?page=1", commercial.ExternalCustomerID(tenantID)), "")
	if status != http.StatusOK {
		t.Fatalf("wallet list: status %d body=%.200s", status, body)
	}
	var parsed struct {
		Wallets []rawWalletView `json:"wallets"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("wallet list parse: %v", err)
	}
	return parsed.Wallets
}

// rawWalletPriority answers one wallet's current priority by name.
func rawWalletPriority(t *testing.T, baseURL, apiKey string, tenantID uint64, name string) int {
	t.Helper()
	for _, w := range listRawWallets(t, baseURL, apiKey, tenantID) {
		if w.Name == name {
			return w.Priority
		}
	}
	t.Fatalf("wallet %s not found on tenant %d", name, tenantID)
	return 0
}

// seedTopUpWalletDirect creates a TOP-UP shaped wallet straight through the
// authority API — exactly the object the #85 payment-confirmed issuance
// will create (ADR-0012 revision): this tenant's anchor metadata, NO period
// key, granted credits, twelve-month-class expiry, priority 2.
func seedTopUpWalletDirect(t *testing.T, baseURL, apiKey string, tenantID uint64, name string, granted string, expiresAt time.Time, priority int) {
	t.Helper()
	body := fmt.Sprintf(`{"wallet":{"external_customer_id":%q,"name":%q,"currency":"CNY","granted_credits":%q,"rate_amount":"1","expiration_at":%q,"priority":%d,"metadata":{"%s":%q}}}`,
		commercial.ExternalCustomerID(tenantID), name, granted,
		expiresAt.UTC().Format(time.RFC3339), priority,
		commercial.WalletMetaTenant, commercial.ExternalCustomerID(tenantID))
	status, resp := rawIntegrationRequest(t, http.MethodPost, baseURL, apiKey, "/api/v1/wallets", body)
	if status < 200 || status >= 300 {
		t.Fatalf("seed top-up wallet %s: status %d body=%.300s", name, status, resp)
	}
}

// submitRebalance drives the adapter's rebalance command (the product path).
func submitRebalance(ctx context.Context, t *testing.T, adapter *LagoAdapter, tenantID uint64) {
	t.Helper()
	if _, err := adapter.SubmitCommand(ctx, commercial.Command{
		Kind:  commercial.CommandKindRebalanceCreditsOrder,
		Key:   commercial.RebalanceCreditsOrderCommandKey(commercial.ExternalCustomerID(tenantID)),
		Actor: "integration-test", Reason: "credits_order_evidence",
		Payload: commercial.RebalanceCreditsOrderPayload{
			TenantID:           tenantID,
			ExternalCustomerID: commercial.ExternalCustomerID(tenantID),
		},
	}); err != nil {
		t.Fatalf("rebalance submit for tenant %d: %v", tenantID, err)
	}
}

// TestLagoCreditsOrder runs the four consumption-order evidence phases
// against the pinned stack (v1.53.0 per deploy/lago/images.lock.json).
func TestLagoCreditsOrder(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY not set (operator stack not provided)")
	}
	release := lockedRelease(t)
	t.Logf("pinned release: %s", release)
	runBase := uint64(8_090_000_000 + rand.Int63n(8_000_000))
	tenantA, tenantC := runBase, runBase+1

	svc, _ := newIntegrationBenefitsService(t, baseURL, apiKey)
	// The dev-stack loopback egress bypass (same as the harness above).
	adapter := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: baseURL, APIKey: apiKey, Release: release, OutboundAllowLoopback: true})
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	// ---- Phase a: the monthly wallet's priority-1 initial encoding ----
	statusA, err := svc.EnsureBenefits(ctx, tenantA, "T14 Order Space A", "integration-operator")
	if err != nil {
		t.Fatalf("[a] EnsureBenefits(A): %v", err)
	}
	if statusA.Account.State != "linked" || statusA.Reason != "" {
		t.Fatalf("[a] the chain must complete cleanly, got state=%s reason=%s", statusA.Account.State, statusA.Reason)
	}
	monthlyA := commercial.MonthlyWalletName(tenantA, commercial.MonthlyPeriod(time.Now().UTC()))
	if p := rawWalletPriority(t, baseURL, apiKey, tenantA, monthlyA); p != commercial.MonthlyWalletPriority {
		t.Fatalf("[a] monthly wallet priority = %d, want the encoded initial %d", p, commercial.MonthlyWalletPriority)
	}
	t.Logf("[a] monthly wallet %s created at priority %d", monthlyA, commercial.MonthlyWalletPriority)

	// ---- Phase b: a top-up shaped wallet joins the snapshot ----
	topUpName := commercial.ExternalCustomerID(tenantA) + "-topup-ev1"
	seedTopUpWalletDirect(t, baseURL, apiKey, tenantA, topUpName, "5",
		time.Now().UTC().AddDate(1, 0, 0), commercial.TopUpWalletPriority)
	snap, err := adapter.ReadSnapshot(ctx, commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: tenantA,
	})
	if err != nil {
		t.Fatalf("[b] snapshot: %v", err)
	}
	sawTopUp := false
	for _, batch := range snap.Benefits.Batches {
		if batch.WalletRef == topUpName || (batch.Source == commercial.BatchSourceTopUp && batch.Period == "") {
			if batch.Source != commercial.BatchSourceTopUp {
				t.Fatalf("[b] the top-up wallet must classify source=topup, got %q", batch.Source)
			}
			sawTopUp = true
		}
	}
	if !sawTopUp {
		t.Fatalf("[b] the top-up batch must join the snapshot, got %+v", snap.Benefits.Batches)
	}
	if snap.Benefits.BalanceMicro < commercial.CentsToMicro(500) {
		t.Fatalf("[b] balance must carry the top-up, got %d", snap.Benefits.BalanceMicro)
	}
	t.Logf("[b] top-up batch listed: balance=%d", snap.Benefits.BalanceMicro)

	// ---- Phase c: the yielding initial on a fresh tenant ----
	// The customer must exist before a wallet can attach: one ensure_customer.
	if _, err := adapter.SubmitCommand(ctx, commercial.Command{
		Kind:  commercial.CommandKindEnsureCustomer,
		Key:   "ensure_customer:" + commercial.ExternalCustomerID(tenantC),
		Actor: "integration-test", Reason: "seed_customer",
		Payload: commercial.EnsureCustomerPayload{
			TenantID: tenantC, ExternalCustomerID: commercial.ExternalCustomerID(tenantC),
			DisplayName: "T14 Order Space C",
		},
	}); err != nil {
		t.Fatalf("[c] ensure_customer(C): %v", err)
	}
	// An AGING top-up: expires strictly before this month's period end.
	now := time.Now().UTC()
	periodEnd, err := commercial.PeriodEnd(commercial.MonthlyPeriod(now))
	if err != nil {
		t.Fatal(err)
	}
	agingExpiry := periodEnd.Add(-24 * time.Hour) // inside the month, before the end
	agingName := commercial.ExternalCustomerID(tenantC) + "-topup-aging"
	seedTopUpWalletDirect(t, baseURL, apiKey, tenantC, agingName, "3", agingExpiry, commercial.TopUpWalletPriority)
	if _, err := svc.EnsureBenefits(ctx, tenantC, "T14 Order Space C", "integration-operator"); err != nil {
		t.Fatalf("[c] EnsureBenefits(C): %v", err)
	}
	monthlyC := commercial.MonthlyWalletName(tenantC, commercial.MonthlyPeriod(now))
	// The grant-time yielding initial (3) is proven at the service level
	// (TestMonthlyGrantEncodesYieldPriority — the value between the grant
	// and the chain-tail rebalance is not observable over a full
	// EnsureBenefits). On the REAL runtime the observable end state is the
	// rebalanced absolute order: the aging top-up ranks 1 and the monthly
	// wallet ranks 2 — the monthly batch sits ABOVE nothing (it yielded),
	// exactly the yielding month's converged order.
	if pAging := rawWalletPriority(t, baseURL, apiKey, tenantC, agingName); pAging != 1 {
		t.Fatalf("[c] the aging top-up must rank 1 after the chain, got %d", pAging)
	}
	if p := rawWalletPriority(t, baseURL, apiKey, tenantC, monthlyC); p != 2 {
		t.Fatalf("[c] the yielding month's monthly wallet priority = %d, want the converged rank 2 (above nothing — it yielded to the aging top-up)", p)
	}
	t.Logf("[c] converged after the chain: aging=1 monthly=2 (the yielding month; the grant-time initial %d is service-level evidence)", commercial.TopUpWalletPriority+1)

	// ---- Phase d: the mixed-family rebalance on the real runtime ----
	freshName := commercial.ExternalCustomerID(tenantC) + "-topup-fresh"
	seedTopUpWalletDirect(t, baseURL, apiKey, tenantC, freshName, "2",
		now.AddDate(0, 6, 0), commercial.TopUpWalletPriority)
	// Initials now read A(aging)=2, M(monthly)=3, B(fresh)=2 — statically
	// unorderable. One rebalance must land the true expiry order.
	submitRebalance(ctx, t, adapter, tenantC)
	gotA := rawWalletPriority(t, baseURL, apiKey, tenantC, agingName)
	gotM := rawWalletPriority(t, baseURL, apiKey, tenantC, monthlyC)
	gotB := rawWalletPriority(t, baseURL, apiKey, tenantC, freshName)
	if gotA != 1 || gotM != 2 || gotB != 3 {
		t.Fatalf("[d] BLOCKED-relevant: rebalanced priorities A=%d M=%d B=%d, want A=1 M=2 B=3 (the PUT priority calibration did not take effect on the pinned runtime)", gotA, gotM, gotB)
	}
	t.Logf("[d] mixed-family rebalance converged: aging=%d monthly=%d fresh=%d", gotA, gotM, gotB)
	// A second rebalance changes nothing (the convergent no-op).
	submitRebalance(ctx, t, adapter, tenantC)
	if p := rawWalletPriority(t, baseURL, apiKey, tenantC, agingName); p != gotA {
		t.Fatalf("[d] the second rebalance must be a no-op, aging moved %d -> %d", gotA, p)
	}
	if p := rawWalletPriority(t, baseURL, apiKey, tenantC, monthlyC); p != gotM {
		t.Fatalf("[d] the second rebalance must be a no-op, monthly moved %d -> %d", gotM, p)
	}
	if p := rawWalletPriority(t, baseURL, apiKey, tenantC, freshName); p != gotB {
		t.Fatalf("[d] the second rebalance must be a no-op, fresh moved %d -> %d", gotB, p)
	}
	if strings.Contains(t.Name(), "never") {
		t.Fatal("unreachable")
	}
}
