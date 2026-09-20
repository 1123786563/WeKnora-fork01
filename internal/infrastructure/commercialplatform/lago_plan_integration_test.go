//go:build lago_integration

// Tagged real-Lago integration evidence (T07, #79). The build tag keeps
// this out of every normal suite run — unit tests stay Docker-free. The
// test is env-gated on LAGO_INTEGRATION_BASE_URL + LAGO_INTEGRATION_API_KEY
// (operator-owned secrets; never committed) and skips otherwise, so a
// missing stack records blocked-env instead of failing or faking a pass.
//
// Four phases in ONE run (uuid-prefixed synthetic keys, the lab isolation
// convention):
//  1. publish v1 through the adapter and read the plan back;
//  2. replay the SAME command key byte-equal → exactly one plan exists;
//  3. a subscription pinned to v1 survives a v2 publish byte-stable;
//  4. the same key with DIFFERENT content is rejected (read-back compare).
package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// integrationClient is the thin raw HTTP client the phases share (the
// adapter's own requests go through the adapter — this one only reads back
// and drives the subscription fixtures).
type integrationClient struct {
	base string
	key  string
	http *http.Client
}

func (c integrationClient) do(method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		blob, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = strings.NewReader(string(blob))
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, data, nil
}

// readPlanByCode reads one plan object back. The single-object GET
// /api/v1/plans/{code} is source-inferred; if the pinned release answers
// differently the list endpoint is the lab-proven fallback — whichever
// answered is recorded in the test log.
func (c integrationClient) readPlanByCode(t *testing.T, code string) (map[string]any, bool) {
	t.Helper()
	status, body, err := c.do(http.MethodGet, "/api/v1/plans/"+url.PathEscape(code), nil)
	if err == nil && status >= 200 && status < 300 {
		var parsed struct {
			Plan map[string]any `json:"plan"`
		}
		if json.Unmarshal(body, &parsed) == nil && parsed.Plan != nil {
			t.Logf("read-back via single-object GET /api/v1/plans/{code}")
			return parsed.Plan, true
		}
	}
	t.Logf("single-object GET did not answer a plan (status=%d, err=%v); falling back to the list endpoint", status, err)
	q := url.Values{}
	q.Set("per_page", "100")
	status, body, err = c.do(http.MethodGet, "/api/v1/plans?"+q.Encode(), nil)
	if err != nil || status < 200 || status >= 300 {
		return nil, false
	}
	var raw struct {
		Plans []map[string]any `json:"plans"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false
	}
	for _, plan := range raw.Plans {
		if plan["code"] == code {
			t.Logf("read-back via list endpoint GET /api/v1/plans?per_page=100")
			return plan, true
		}
	}
	return nil, false
}

// countPlansByCode counts plans carrying the code through the list
// endpoint (the no-double-plan proof).
func (c integrationClient) countPlansByCode(t *testing.T, code string) int {
	t.Helper()
	q := url.Values{}
	q.Set("per_page", "100")
	status, body, err := c.do(http.MethodGet, "/api/v1/plans?"+q.Encode(), nil)
	if err != nil || status < 200 || status >= 300 {
		t.Fatalf("list plans: status=%d err=%v", status, err)
	}
	var raw struct {
		Plans []map[string]any `json:"plans"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, plan := range raw.Plans {
		if plan["code"] == code {
			n++
		}
	}
	return n
}

// assertPlanFields compares the read-back plan against OUR write (the
// pinned identity fields).
func assertPlanFields(t *testing.T, plan map[string]any, payload commercial.PublishPlanVersionPayload) {
	t.Helper()
	if fmt.Sprint(plan["amount_cents"]) != fmt.Sprint(payload.AmountFen) {
		t.Fatalf("read-back amount_cents = %v, want %d", plan["amount_cents"], payload.AmountFen)
	}
	if plan["amount_currency"] != payload.Currency {
		t.Fatalf("read-back amount_currency = %v", plan["amount_currency"])
	}
	if plan["interval"] != payload.Interval {
		t.Fatalf("read-back interval = %v", plan["interval"])
	}
	if plan["pay_in_advance"] != true {
		t.Fatalf("read-back pay_in_advance = %v, want true", plan["pay_in_advance"])
	}
}

// readSubscription reads the subscription by its own external_id. The
// index filters status by default (T02 lab fact), so the read passes the
// broad status set explicitly.
func (c integrationClient) readSubscription(t *testing.T, externalID string) map[string]any {
	t.Helper()
	path := "/api/v1/subscriptions/" + url.PathEscape(externalID) +
		"?status[]=active&status[]=canceled&status[]=incomplete&status[]=pending&status[]=terminated"
	status, body, err := c.do(http.MethodGet, path, nil)
	if err != nil || status < 200 || status >= 300 {
		// Fallback: list and filter client-side.
		q := url.Values{}
		q.Set("per_page", "100")
		q.Set("status[]", "active")
		q.Add("status[]", "canceled")
		q.Add("status[]", "incomplete")
		q.Add("status[]", "pending")
		q.Add("status[]", "terminated")
		status, body, err = c.do(http.MethodGet, "/api/v1/subscriptions?"+q.Encode(), nil)
		if err != nil || status < 200 || status >= 300 {
			t.Fatalf("read subscription %s: status=%d err=%v", externalID, status, err)
		}
		var list struct {
			Subscriptions []map[string]any `json:"subscriptions"`
		}
		if err := json.Unmarshal(body, &list); err != nil {
			t.Fatal(err)
		}
		for _, sub := range list.Subscriptions {
			if sub["external_id"] == externalID {
				t.Logf("subscription read via list endpoint fallback")
				return sub
			}
		}
		t.Fatalf("subscription %s not found", externalID)
	}
	var parsed struct {
		Subscription map[string]any `json:"subscription"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Subscription == nil {
		t.Fatalf("parse subscription read-back: %v (%s)", err, string(body))
	}
	return parsed.Subscription
}

// integrationPayload builds the publish payload for the run's synthetic
// plan key.
func integrationPayload(planKey string, version int64, amountFen int64) commercial.PublishPlanVersionPayload {
	return commercial.PublishPlanVersionPayload{
		PlanKey:              planKey,
		Version:              version,
		PlanCode:             commercial.DeterministicPlanCode(planKey, version),
		Name:                 "T07 Integration " + planKey,
		Interval:             commercial.IntervalMonthly,
		AmountFen:            amountFen,
		Currency:             commercial.CurrencyCNY,
		PayInAdvance:         true,
		IncludedCreditsMicro: 9_900_000,
		Features:             map[string]bool{"api_access": true},
	}
}

// TestLagoPlanPublishIntegration: the four acceptance phases against the
// REAL pinned Lago v1.53.0 stack.
func TestLagoPlanPublishIntegration(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY not set (operator stack not provided)")
	}
	adapter := NewLagoAdapter(Config{
		Provider: ProviderLago,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Release:  lockedRelease(t),
	})
	raw := integrationClient{base: strings.TrimSuffix(baseURL, "/"), key: apiKey, http: &http.Client{Timeout: 60 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	planKey := "t07-" + fmt.Sprintf("%d", time.Now().UnixNano()) // uuid-prefixed synthetic isolation
	commandKey := commercial.PublishCommandKey(planKey, 1)
	submit := func(payload commercial.PublishPlanVersionPayload) (commercial.CommandReceipt, error) {
		return adapter.SubmitCommand(ctx, commercial.Command{
			Kind: commercial.CommandKindPublishPlanVersion,
			Key:  commandKey, // same identity across phases 1/2/4 — replay and conflict
			Actor: "t07-integration", Reason: "t07 real-stack evidence",
			Payload: payload,
		})
	}

	// Phase 1: publish v1 + read-back.
	v1 := integrationPayload(planKey, 1, 9900)
	receipt, err := submit(v1)
	if err != nil {
		t.Fatalf("phase 1 publish: %v", err)
	}
	if receipt.ExternalID != v1.PlanCode {
		t.Fatalf("phase 1 receipt ExternalID = %q, want %q", receipt.ExternalID, v1.PlanCode)
	}
	plan, ok := raw.readPlanByCode(t, v1.PlanCode)
	if !ok {
		t.Fatalf("phase 1: the published plan %s is not readable", v1.PlanCode)
	}
	assertPlanFields(t, plan, v1)

	// Phase 2: idempotent replay — same key, byte-equal payload, exactly
	// one plan with the code afterwards.
	replay, err := submit(v1)
	if err != nil {
		t.Fatalf("phase 2 replay: %v", err)
	}
	if replay.ExternalID != v1.PlanCode {
		t.Fatalf("phase 2 replay receipt ExternalID = %q", replay.ExternalID)
	}
	if n := raw.countPlansByCode(t, v1.PlanCode); n != 1 {
		t.Fatalf("phase 2: EXACTLY ONE plan with code %s must exist, got %d", v1.PlanCode, n)
	}

	// Phase 3: a subscription pinned to v1 survives a v2 publish.
	custExternal := planKey + "-cust"
	if status, body, err := raw.do(http.MethodPost, "/api/v1/customers", map[string]any{
		"customer": map[string]any{"external_id": custExternal, "name": "T07 " + planKey, "currency": "CNY"},
	}); err != nil || status < 200 || status >= 300 {
		t.Fatalf("phase 3 create customer: status=%d err=%v body=%s", status, err, string(body))
	}
	subExternal := planKey + "-sub"
	if status, body, err := raw.do(http.MethodPost, "/api/v1/subscriptions", map[string]any{
		"subscription": map[string]any{
			"external_customer_id": custExternal,
			"plan_code":            v1.PlanCode,
			"name":                 "T07 sub " + planKey,
			"external_id":          subExternal, // own external_id is mandatory (T03 lab fact)
		},
	}); err != nil || status < 200 || status >= 300 {
		t.Fatalf("phase 3 create subscription: status=%d err=%v body=%s", status, err, string(body))
	}
	before := raw.readSubscription(t, subExternal)

	v2Key := commercial.PublishCommandKey(planKey, 2)
	v2 := integrationPayload(planKey, 2, 9900)
	v2Receipt, err := adapter.SubmitCommand(ctx, commercial.Command{
		Kind: commercial.CommandKindPublishPlanVersion, Key: v2Key,
		Actor: "t07-integration", Reason: "t07 real-stack evidence v2", Payload: v2,
	})
	if err != nil {
		t.Fatalf("phase 3 publish v2: %v", err)
	}
	if v2Receipt.ExternalID == v1.PlanCode {
		t.Fatal("phase 3: v2 must carry its OWN plan code")
	}
	after := raw.readSubscription(t, subExternal)
	for _, field := range []string{"plan_code", "external_id", "status", "lago_id"} {
		if fmt.Sprint(before[field]) != fmt.Sprint(after[field]) {
			t.Fatalf("phase 3: subscription %s changed across the v2 publish: %v -> %v",
				field, before[field], after[field])
		}
	}
	if fmt.Sprint(after["plan_code"]) != v1.PlanCode {
		t.Fatalf("phase 3: the subscription must stay pinned to v1, got plan_code=%v", after["plan_code"])
	}

	// Phase 4: the same key with DIFFERENT content is rejected (read-back
	// compare caught the conflict; #76 lab: 422 bodies are indistinguishable).
	conflicting := integrationPayload(planKey, 1, 29900)
	if _, err := submit(conflicting); !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("phase 4: content conflict must be ErrPlatformInvalidResponse, got %v", err)
	}

	// Cleanup duty (the t02 convention, best-effort): remove the fixture
	// objects this run created. Volumes survive per T01 convention; the
	// objects should not.
	for _, del := range []struct{ method, path string }{
		{http.MethodDelete, "/api/v1/subscriptions/" + url.PathEscape(subExternal)},
		{http.MethodDelete, "/api/v1/customers/" + url.PathEscape(custExternal)},
		{http.MethodDelete, "/api/v1/plans/" + url.PathEscape(v1.PlanCode)},
		{http.MethodDelete, "/api/v1/plans/" + url.PathEscape(v2.PlanCode)},
	} {
		if status, _, err := raw.do(del.method, del.path, nil); err != nil || status >= 300 {
			t.Logf("cleanup %s %s: status=%d err=%v (best-effort)", del.method, del.path, status, err)
		}
	}
}
