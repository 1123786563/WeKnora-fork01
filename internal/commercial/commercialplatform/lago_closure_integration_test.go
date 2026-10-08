//go:build lago_integration

package commercialplatform

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// TestLagoCloseWorkspaceIntegration: against the REAL pinned Lago v1.53.0
// stack, close_workspace disposes the tenant's objects and de-identifies
// the customer — the subscription reaches terminated (the benefits
// snapshot answers the terminal token, balances gone), the identity STAYS
// linked (US54: financial history addressable, identity never reused),
// the raw customer read shows the de-identified name, and a replay
// converges without error.
func TestLagoCloseWorkspaceIntegration(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY not set (operator stack not provided)")
	}
	// Run-unique lab tenants (the lab isolation convention).
	tenant := uint64(8_090_000_000 + rand.Int63n(8_000_000))
	ext := commercial.ExternalCustomerID(tenant)
	p := NewLagoAdapter(Config{Provider: ProviderLago, BaseURL: baseURL, APIKey: apiKey,
		Release:                lockedRelease(t),
		OutboundAllowLoopback:  strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, "localhost"),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Seed a zero-price monthly plan (raw probe, run-unique code) and the
	// tenant's customer + standard subscription + one next-month wallet
	// through the REAL adapter commands.
	planCode := "weknora-closure-it-plan"
	if status, body := rawIntegrationRequest(t, http.MethodPost, baseURL, apiKey, "/api/v1/plans",
		`{"plan":{"code":"`+planCode+`","name":"Closure IT","interval":"monthly","amount_cents":0,"amount_currency":"CNY","pay_in_advance":true}}`); status >= 300 && !strings.Contains(body, "value_already_exist") {
		t.Fatalf("plan seed: status %d body %s", status, body)
	}
	if _, err := p.SubmitCommand(ctx, commercial.Command{
		Kind: commercial.CommandKindEnsureCustomer, Key: "ensure_customer:" + ext,
		Payload: commercial.EnsureCustomerPayload{
			TenantID: tenant, ExternalCustomerID: ext, DisplayName: "Closure IT Space",
		},
	}); err != nil {
		t.Fatalf("ensure customer: %v", err)
	}
	extSub := commercial.ExternalSubscriptionID(tenant)
	if _, err := p.SubmitCommand(ctx, commercial.Command{
		Kind: commercial.CommandKindEnsureSubscription, Key: "ensure_subscription:" + extSub,
		Payload: commercial.EnsureSubscriptionPayload{
			TenantID: tenant, ExternalCustomerID: ext,
			ExternalSubscriptionID: extSub, PlanCode: planCode,
		},
	}); err != nil {
		t.Fatalf("ensure subscription: %v", err)
	}
	periodStart := time.Now().UTC().AddDate(0, 1, 0)
	period := periodStart.Format("2006-01")
	end, err := commercial.PeriodEnd(period)
	if err != nil {
		t.Fatal(err)
	}
	monthStart := periodStart.AddDate(0, 0, -periodStart.Day()+1)
	if _, err := p.SubmitCommand(ctx, commercial.Command{
		Kind: commercial.CommandKindGrantIncludedCredits,
		Key:  commercial.MonthlyGrantKey(extSub, monthStart),
		Payload: commercial.GrantIncludedCreditsPayload{
			TenantID: tenant, ExternalCustomerID: ext, Period: period,
			CreditsMicro: 1_000_000, ExpiresAt: end,
			Priority: commercial.MonthlyWalletPriorityFor(nil, end),
		},
	}); err != nil {
		t.Fatalf("grant wallet: %v", err)
	}

	closeCmd := commercial.Command{
		Kind: commercial.CommandKindCloseWorkspace,
		Key:  commercial.CloseWorkspaceKey(ext),
		Payload: commercial.CloseWorkspacePayload{
			TenantID: tenant, ExternalCustomerID: ext,
			DisplayName: commercial.DeidentifiedDisplayName(ext),
		},
	}
	if _, err := p.SubmitCommand(ctx, closeCmd); err != nil {
		t.Fatalf("close_workspace: %v", err)
	}

	benefits, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindBenefits, TenantID: tenant,
	})
	if err != nil {
		t.Fatalf("benefits snapshot: %v", err)
	}
	if benefits.Benefits == nil || benefits.Benefits.SubscriptionState != commercial.SubscriptionStateTerminated {
		t.Fatalf("subscription must answer terminated, got %+v", benefits.Benefits)
	}
	if benefits.Benefits.BalanceMicro != 0 {
		t.Fatalf("terminated wallets must not answer balances, got %d", benefits.Benefits.BalanceMicro)
	}

	account, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindAccount, TenantID: tenant,
	})
	if err != nil || account.Account == nil || account.Account.State != commercial.AccountStateLinked {
		t.Fatalf("the identity must stay linked after closure (history addressable): %v %+v", err, account.Account)
	}

	status, body := rawIntegrationRequest(t, http.MethodGet, baseURL, apiKey, "/api/v1/customers/"+ext, "")
	if status != http.StatusOK {
		t.Fatalf("customer read after closure: status %d", status)
	}
	var cust struct {
		Customer struct {
			Name string `json:"name"`
		} `json:"customer"`
	}
	if err := json.Unmarshal([]byte(body), &cust); err != nil {
		t.Fatalf("customer read malformed: %v", err)
	}
	if cust.Customer.Name != commercial.DeidentifiedDisplayName(ext) {
		t.Fatalf("customer name must be de-identified, got %q", cust.Customer.Name)
	}

	// Replay converges (receipt, no error — nothing left to terminate).
	replay, err := p.SubmitCommand(ctx, closeCmd)
	if err != nil || replay.Key != commercial.CloseWorkspaceKey(ext) {
		t.Fatalf("closure replay must converge: %v %+v", err, replay)
	}

	// Cleanup: the de-identified shell customer and its objects (the lab
	// stack's hygiene rule; production retention is the projection's
	// concern).
	rawIntegrationRequest(t, http.MethodDelete, baseURL, apiKey, "/api/v1/customers/"+ext, "")
}
