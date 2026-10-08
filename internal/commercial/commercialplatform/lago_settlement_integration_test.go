//go:build lago_integration

// Tagged real-Lago settle-rail evidence (#82 Task 9). The build tag keeps
// this out of every normal suite run; the test is env-gated on the
// LAGO_INTEGRATION_* family (operator-owned, never committed) and skips
// otherwise, so a missing stack records blocked-env instead of failing or
// faking a pass.
//
// Chain under test (the α dual-track, t11 P-A..P-E):
//
//	ensure customer+binding (3DS pm env) → gated create → the stuck gating
//	PaymentIntent is visible provider-side (status recorded as observed) →
//	settle_purchase_payment (attach settle pm + update + confirm) → the
//	harness delivers the REAL payment_intent.succeeded event through the
//	built-in webhook route (real-secret HMAC, transport-leg stand-in per
//	D8) → the authority finalizes: subscription active + invoice finalized/
//	payment_status succeeded + EXACTLY ONE succeeded payment.
//
// Negative controls: settle replay is a zero-side-effect no-op; duplicate
// webhook delivery is byte-identical no-op; the activation only happens
// through this chain (never a local write).
package commercialplatform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func integrationEnv(names ...string) map[string]string {
	out := map[string]string{}
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			out[name] = value
		}
	}
	return out
}

func TestInboundWebhookReplayGateAndCanonicalCollection(t *testing.T) {
	if paymentProviderCode != "weknora-stripe" {
		t.Fatalf("payment provider code changed: %q", paymentProviderCode)
	}
	// psql runs with -t -A, so the query must return one JSON array cell,
	// including [] when no rows match, rather than delimiter-separated columns.
	if !strings.Contains(inboundWebhookSQL, "json_agg(json_build_object") || !strings.Contains(inboundWebhookSQL, "'[]'") {
		t.Fatalf("inbound webhook SQL must return a JSON array: %s", inboundWebhookSQL)
	}
	emptyRows, err := parseInboundWebhookRows([]byte("[]\n"))
	if err != nil || len(emptyRows) != 0 {
		t.Fatalf("empty SQL result: %#v %v", emptyRows, err)
	}
	rowsFromSQL, err := parseInboundWebhookRows([]byte("[{\"id\":\"base\",\"status\":\"succeeded\"}]\n"))
	if err != nil || len(rowsFromSQL) != 1 || rowsFromSQL[0].ID != "base" {
		t.Fatalf("JSON SQL result: %#v %v", rowsFromSQL, err)
	}
	rows, err := parseInboundWebhookRows([]byte(`[ {"id":"base","status":"succeeded"} ]`))
	if err != nil {
		t.Fatal(err)
	}
	base, err := requireBaselineWebhook(rows)
	if err != nil || base != "base" {
		t.Fatalf("baseline: %q %v", base, err)
	}
	rows, err = parseInboundWebhookRows([]byte(`[{"id":"base","status":"succeeded"},{"id":"new","status":"pending"}]`))
	if err != nil {
		t.Fatal(err)
	}
	newRow, err := requireReplayWebhook(rows, "base")
	if err != nil || newRow.Status != "pending" {
		t.Fatalf("replay: %+v %v", newRow, err)
	}
	if _, err := requireReplayWebhook([]inboundWebhookRow{{ID: "base", Status: "succeeded"}, {ID: "new", Status: "failed"}}, "base"); err == nil {
		t.Fatal("failed replay accepted")
	}
	input := []any{map[string]any{"lago_id": "b"}, map[string]any{"lago_id": "a"}}
	got, err := canonicalCollection(input, true)
	if err != nil || string(got) != `[{"lago_id":"a"},{"lago_id":"b"}]` {
		t.Fatalf("canonical: %s %v", got, err)
	}
	if _, err := canonicalCollection(input, false); err == nil {
		t.Fatal("incomplete collection accepted")
	}
	for _, status := range []string{"processing", "succeeded"} {
		rows := []inboundWebhookRow{{ID: "base", Status: "succeeded"}, {ID: "new", Status: status}}
		row, err := requireReplayWebhook(rows, "base")
		if err != nil || row.Status != status {
			t.Fatalf("status %s: %+v %v", status, row, err)
		}
	}
	if _, err := requireReplayWebhook(nil, "base"); err == nil {
		t.Fatal("missing rows accepted")
	}
	if _, err := requireReplayWebhook([]inboundWebhookRow{{ID: "base", Status: "succeeded"}, {ID: "x", Status: "pending"}, {ID: "y", Status: "pending"}}, "base"); err == nil {
		t.Fatal("duplicate replay rows accepted")
	}
	if _, err := parseInboundWebhookRows([]byte(`[{}]`)); err == nil {
		t.Fatal("malformed status row accepted")
	}
}

func TestPaymentsForInvoiceUsesInvoiceIDsMembership(t *testing.T) {
	rows := []any{
		map[string]any{"lago_id": "pay-target", "invoice_ids": []any{"inv-other", "inv-target"}},
		map[string]any{"lago_id": "pay-other", "invoice_ids": []any{"inv-other"}},
	}
	got, err := paymentsForInvoice(rows, "inv-target")
	if err != nil || len(got) != 1 || got[0].(map[string]any)["lago_id"] != "pay-target" {
		t.Fatalf("target invoice membership: %#v %v", got, err)
	}
	if _, err := paymentsForInvoice([]any{map[string]any{"lago_id": "bad", "invoice_id": "inv-target"}}, "inv-target"); err == nil {
		t.Fatal("malformed invoice_ids accepted")
	}
	if _, err := paymentsForInvoice([]any{map[string]any{"lago_id": "bad", "invoice_ids": []any{7}}}, "inv-target"); err == nil {
		t.Fatal("non-string invoice ID accepted")
	}
}

func TestWaitForReplayWebhookFailsImmediatelyOnFailedRow(t *testing.T) {
	calls := 0
	err := waitForReplayWebhook(context.Background(), func(context.Context) ([]inboundWebhookRow, error) {
		calls++
		return []inboundWebhookRow{{ID: "base", Status: "succeeded"}, {ID: "new", Status: "failed"}}, nil
	}, "base")
	if err == nil || calls != 1 {
		t.Fatalf("failed row must fail immediately, err=%v calls=%d", err, calls)
	}
}

type inboundWebhookRow struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func parseInboundWebhookRows(blob []byte) ([]inboundWebhookRow, error) {
	var rows []inboundWebhookRow
	if err := json.Unmarshal(blob, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == "" || row.Status == "" {
			return nil, fmt.Errorf("inbound webhook row missing id/status")
		}
	}
	return rows, nil
}

func requireBaselineWebhook(rows []inboundWebhookRow) (string, error) {
	if len(rows) != 1 {
		return "", fmt.Errorf("expected one baseline inbound webhook row, got %d", len(rows))
	}
	if rows[0].Status != "succeeded" {
		return "", fmt.Errorf("baseline webhook %s status is %q", rows[0].ID, rows[0].Status)
	}
	return rows[0].ID, nil
}

func requireReplayWebhook(rows []inboundWebhookRow, baselineID string) (inboundWebhookRow, error) {
	if len(rows) != 2 {
		return inboundWebhookRow{}, fmt.Errorf("expected baseline plus one replay row, got %d", len(rows))
	}
	var replay inboundWebhookRow
	for _, row := range rows {
		if row.ID != baselineID {
			if replay.ID != "" {
				return inboundWebhookRow{}, fmt.Errorf("multiple replay rows")
			}
			replay = row
		}
	}
	if replay.ID == "" {
		return inboundWebhookRow{}, fmt.Errorf("new replay row missing")
	}
	if replay.Status == "failed" {
		return replay, fmt.Errorf("replay webhook %s failed", replay.ID)
	}
	return replay, nil
}

// v1.53 stores the event DOUBLE-ENCODED in the jsonb column (a JSON string
// scalar whose text IS the event), so the event-identity predicate unwraps
// one layer before reading the id — a plain payload->>'id' matches nothing.
const inboundWebhookSQL = `SELECT COALESCE(json_agg(json_build_object('id', id::text, 'status', status::text) ORDER BY created_at, id)::text, '[]') FROM inbound_webhooks WHERE organization_id = :'organization_id'::uuid AND source = 'stripe' AND code = :'provider_code' AND (payload #>> '{}')::jsonb ->> 'id' = :'event_id';`

func readInboundWebhookRows(ctx context.Context, dbContainer, dbUser, dbName, orgID, providerCode, eventID string) ([]inboundWebhookRow, error) {
	args := []string{"exec", "-i", dbContainer, "psql", "-X", "-q", "-t", "-A", "-v", "ON_ERROR_STOP=1", "-U", dbUser, "-d", dbName,
		"-v", "organization_id=" + orgID, "-v", "provider_code=" + providerCode, "-v", "event_id=" + eventID, "-f", "-"}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(inboundWebhookSQL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("read inbound webhook rows: docker/psql failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return parseInboundWebhookRows(stdout.Bytes())
}

func waitForReplayWebhook(ctx context.Context, read func(context.Context) ([]inboundWebhookRow, error), baselineID string) error {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		rows, err := read(ctx)
		if err != nil {
			return err
		}
		row, gateErr := requireReplayWebhook(rows, baselineID)
		if gateErr == nil && row.Status == "succeeded" {
			return nil
		}
		if gateErr != nil && row.Status == "failed" {
			return gateErr
		}
		if gateErr != nil && len(rows) > 2 {
			return gateErr
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for replay webhook: %w", ctx.Err())
		case <-tick.C:
		}
	}
}

func normalizeObject(v any) ([]byte, error) { return json.Marshal(v) }

func canonicalCollection(rows []any, complete bool) ([]byte, error) {
	if !complete {
		return nil, fmt.Errorf("collection pagination incomplete")
	}
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok || m["lago_id"] == nil {
			return nil, fmt.Errorf("collection row missing lago_id")
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return fmt.Sprint(rows[i].(map[string]any)["lago_id"]) < fmt.Sprint(rows[j].(map[string]any)["lago_id"])
	})
	return json.Marshal(rows)
}

func paymentsForInvoice(rows []any, invoiceID string) ([]any, error) {
	if invoiceID == "" {
		return nil, fmt.Errorf("target invoice ID missing")
	}
	selected := make([]any, 0)
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok || row["lago_id"] == nil {
			return nil, fmt.Errorf("payment missing lago_id")
		}
		ids, ok := row["invoice_ids"].([]any)
		if !ok {
			return nil, fmt.Errorf("payment %v has malformed invoice_ids", row["lago_id"])
		}
		for _, id := range ids {
			value, ok := id.(string)
			if !ok {
				return nil, fmt.Errorf("payment %v has non-string invoice ID", row["lago_id"])
			}
			if value == invoiceID {
				selected = append(selected, row)
				break
			}
		}
	}
	return selected, nil
}

type t9ObjectSnapshot struct {
	Subscription json.RawMessage `json:"subscription"`
	Invoice      json.RawMessage `json:"invoice"`
	Payments     json.RawMessage `json:"payments"`
	Wallets      json.RawMessage `json:"wallets"`
}

func canonicalAPIObject(body []byte, root string) (map[string]any, error) {
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	obj, ok := envelope[root].(map[string]any)
	if !ok || obj["lago_id"] == nil {
		return nil, fmt.Errorf("%s object missing lago_id", root)
	}
	return obj, nil
}

func apiCollection(ctx context.Context, a *LagoAdapter, path, root string) ([]any, error) {
	all := []any{}
	for page := 1; page <= 10000; page++ {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		status, body, err := a.do(ctx, http.MethodGet, fmt.Sprintf("%s%spage=%d&per_page=100", path, sep, page), nil)
		if err != nil || status != http.StatusOK {
			return nil, fmt.Errorf("%s page %d HTTP %d: %v", root, page, status, err)
		}
		var envelope struct {
			Meta struct {
				TotalCount *int `json:"total_count"`
			} `json:"meta"`
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw["meta"], &envelope.Meta); err != nil || envelope.Meta.TotalCount == nil {
			return nil, fmt.Errorf("%s page %d missing pagination total_count", root, page)
		}
		collection, ok := raw[root]
		if !ok {
			return nil, fmt.Errorf("%s response missing collection", root)
		}
		var batch []any
		if err := json.Unmarshal(collection, &batch); err != nil {
			return nil, fmt.Errorf("%s collection malformed: %w", root, err)
		}
		all = append(all, batch...)
		if len(all) == *envelope.Meta.TotalCount {
			return all, nil
		}
		if len(all) > *envelope.Meta.TotalCount || len(batch) == 0 {
			return nil, fmt.Errorf("%s pagination count inconsistent: received %d of %d", root, len(all), *envelope.Meta.TotalCount)
		}
	}
	return nil, fmt.Errorf("%s pagination exceeded safety bound", root)
}

func t9LagoSnapshot(ctx context.Context, a *LagoAdapter, extPurchase, extCustomer, invoiceID string) ([]byte, error) {
	status, body, err := a.do(ctx, http.MethodGet, "/api/v1/subscriptions/"+url.PathEscape(extPurchase), nil)
	if err != nil || status != http.StatusOK {
		return nil, fmt.Errorf("subscription read HTTP %d: %v", status, err)
	}
	sub, err := canonicalAPIObject(body, "subscription")
	if err != nil {
		return nil, err
	}
	if sub["status"] != "active" {
		return nil, fmt.Errorf("purchase subscription not active")
	}
	status, body, err = a.do(ctx, http.MethodGet, "/api/v1/invoices/"+url.PathEscape(invoiceID), nil)
	if err != nil || status != http.StatusOK {
		return nil, fmt.Errorf("invoice read HTTP %d: %v", status, err)
	}
	inv, err := canonicalAPIObject(body, "invoice")
	if err != nil {
		return nil, err
	}
	if inv["lago_id"] != invoiceID || inv["status"] != "finalized" || inv["payment_status"] != "succeeded" {
		return nil, fmt.Errorf("target invoice is not exact finalized+succeeded invoice")
	}
	payments, err := apiCollection(ctx, a, "/api/v1/payments?external_customer_id="+url.QueryEscape(extCustomer), "payments")
	if err != nil {
		return nil, err
	}
	filteredPayments, err := paymentsForInvoice(payments, invoiceID)
	if err != nil {
		return nil, err
	}
	succeededPayments := 0
	if len(filteredPayments) == 0 {
		return nil, fmt.Errorf("target invoice has no payment records")
	}
	for _, item := range filteredPayments {
		if item.(map[string]any)["status"] == "succeeded" {
			succeededPayments++
		}
	}
	if succeededPayments != 1 {
		return nil, fmt.Errorf("target invoice must have exactly one succeeded payment, got %d", succeededPayments)
	}
	paymentJSON, err := canonicalCollection(filteredPayments, true)
	if err != nil {
		return nil, err
	}
	wallets, err := apiCollection(ctx, a, "/api/v1/wallets?external_customer_id="+url.QueryEscape(extCustomer), "wallets")
	if err != nil {
		return nil, err
	}
	if len(wallets) == 0 {
		return nil, fmt.Errorf("customer wallets missing")
	}
	purchaseWalletFound := false
	for _, item := range wallets {
		wallet, ok := item.(map[string]any)
		if !ok || wallet["lago_id"] == nil {
			return nil, fmt.Errorf("wallet missing lago_id")
		}
		name, _ := wallet["name"].(string)
		if strings.HasPrefix(name, extPurchase+"-") {
			if wallet["status"] != "active" {
				return nil, fmt.Errorf("purchase wallet %s is not active", name)
			}
			purchaseWalletFound = true
		}
		txPath := "/api/v1/wallets/" + url.PathEscape(fmt.Sprint(wallet["lago_id"])) + "/wallet_transactions"
		txs, err := apiCollection(ctx, a, txPath, "wallet_transactions")
		if err != nil {
			return nil, err
		}
		txJSON, err := canonicalCollection(txs, true)
		if err != nil {
			return nil, err
		}
		wallet["wallet_transactions"] = json.RawMessage(txJSON)
	}
	if !purchaseWalletFound {
		return nil, fmt.Errorf("active purchase wallet for %q missing", extPurchase)
	}
	walletJSON, err := canonicalCollection(wallets, true)
	if err != nil {
		return nil, err
	}
	subJSON, _ := normalizeObject(sub)
	invJSON, _ := normalizeObject(inv)
	return json.Marshal(t9ObjectSnapshot{Subscription: subJSON, Invoice: invJSON, Payments: paymentJSON, Wallets: walletJSON})
}

// stripeForm issues one form-encoded Stripe API call with the credential
// ONLY in the Authorization header.
func stripeForm(t *testing.T, apiKey, method, path string, form url.Values) (int, map[string]any) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		var reader io.Reader
		if form != nil {
			reader = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequest(method, "https://api.stripe.com"+path, reader)
		if err != nil {
			t.Fatalf("stripe request: %v", err)
		}
		req.Header.Set("Authorization", "Basic "+base64Header(apiKey))
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			lastErr = err // transport flake: bounded retry (HTTPError is a real answer)
			time.Sleep(time.Duration(attempt+1) * 1500 * time.Millisecond)
			continue
		}
		defer resp.Body.Close()
		blob, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var body map[string]any
		_ = json.Unmarshal(blob, &body)
		return resp.StatusCode, body
	}
	t.Fatalf("stripe %s %s: %v", method, path, lastErr)
	return 0, nil
}

func base64Header(apiKey string) string {
	const b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	raw := []byte(apiKey + ":")
	var out strings.Builder
	for i := 0; i < len(raw); i += 3 {
		var b [3]byte
		n := copy(b[:], raw[i:])
		out.WriteByte(b64chars[b[0]>>2])
		out.WriteByte(b64chars[(b[0]&0x03)<<4|b[1]>>4])
		if n > 1 {
			out.WriteByte(b64chars[(b[1]&0x0f)<<2|b[2]>>6])
		} else {
			out.WriteByte('=')
		}
		if n > 2 {
			out.WriteByte(b64chars[b[2]&0x3f])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

type paymentIntentCandidate struct {
	id        string
	status    string
	invoiceID string
}

// selectExpectedPaymentIntent picks only the exact succeeded, invoice-linked
// intent that was observed before settlement.
func selectExpectedPaymentIntent(expectedID string, rows []paymentIntentCandidate) (paymentIntentCandidate, error) {
	for _, row := range rows {
		if row.id == expectedID && row.status == "succeeded" && row.invoiceID != "" {
			return row, nil
		}
	}
	return paymentIntentCandidate{}, fmt.Errorf("expected succeeded invoice-linked PaymentIntent %q not found", expectedID)
}

func paymentIntentCandidates(rows []any) ([]paymentIntentCandidate, map[string]struct{}) {
	var eligible []paymentIntentCandidate
	observed := make(map[string]struct{})
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row == nil {
			continue
		}
		id, _ := row["id"].(string)
		if id != "" {
			observed[id] = struct{}{}
		}
		meta, _ := row["metadata"].(map[string]any)
		invoiceID, _ := meta["lago_invoice_id"].(string)
		status, _ := row["status"].(string)
		if id == "" || invoiceID == "" {
			continue
		}
		switch status {
		case "requires_payment_method", "requires_action", "requires_confirmation":
			eligible = append(eligible, paymentIntentCandidate{id: id, status: status, invoiceID: invoiceID})
		}
	}
	return eligible, observed
}

func requireUniquePaymentIntentCandidate(candidates []paymentIntentCandidate) (paymentIntentCandidate, error) {
	if len(candidates) != 1 {
		return paymentIntentCandidate{}, fmt.Errorf("expected exactly one pre-settle invoice-linked unsettled PaymentIntent, found %d: %+v", len(candidates), candidates)
	}
	return candidates[0], nil
}

func succeededPaymentIntent(expectedID string, preSettleIDs map[string]struct{}, rows []any) (map[string]any, error) {
	candidates := make([]paymentIntentCandidate, 0, len(rows))
	intents := make(map[string]map[string]any)
	var observed []string
	for _, raw := range rows {
		intent, _ := raw.(map[string]any)
		if intent == nil || intent["status"] != "succeeded" {
			continue
		}
		id, _ := intent["id"].(string)
		meta, _ := intent["metadata"].(map[string]any)
		invoiceID, _ := meta["lago_invoice_id"].(string)
		if invoiceID == "" {
			continue
		}
		observed = append(observed, id)
		if _, existed := preSettleIDs[id]; !existed {
			return nil, fmt.Errorf("new succeeded invoice-linked PaymentIntent %q appeared after settle; pre-settle IDs=%v", id, sortedPaymentIntentIDs(preSettleIDs))
		}
		candidates = append(candidates, paymentIntentCandidate{id: id, status: "succeeded", invoiceID: invoiceID})
		intents[id] = intent
	}
	selected, err := selectExpectedPaymentIntent(expectedID, candidates)
	if err != nil {
		return nil, fmt.Errorf("%w; observed succeeded invoice-linked IDs=%v", err, observed)
	}
	return intents[selected.id], nil
}

func sortedPaymentIntentIDs(ids map[string]struct{}) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func TestPaymentIntentCandidatesCaptureAllIDsAndFilterEligibility(t *testing.T) {
	rows := []any{
		map[string]any{"id": "pi_method", "status": "requires_payment_method", "metadata": map[string]any{"lago_invoice_id": "in_1"}},
		map[string]any{"id": "pi_action", "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_2"}},
		map[string]any{"id": "pi_confirmation", "status": "requires_confirmation", "metadata": map[string]any{"lago_invoice_id": "in_3"}},
		map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
		map[string]any{"id": "pi_unlinked", "status": "requires_action"},
		map[string]any{"id": "pi_other_status", "status": "processing", "metadata": map[string]any{"lago_invoice_id": "in_4"}},
		map[string]any{"status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_missing_id"}},
	}
	eligible, observed := paymentIntentCandidates(rows)
	if len(eligible) != 3 {
		t.Fatalf("eligible candidates = %+v, want exactly the three unsettled invoice-linked candidates", eligible)
	}
	wantEligible := map[string]string{"pi_method": "requires_payment_method", "pi_action": "requires_action", "pi_confirmation": "requires_confirmation"}
	for _, candidate := range eligible {
		if wantEligible[candidate.id] != candidate.status || candidate.invoiceID == "" {
			t.Errorf("unexpected eligible candidate: %+v", candidate)
		}
	}
	for _, id := range []string{"pi_method", "pi_action", "pi_confirmation", "pi_old", "pi_unlinked", "pi_other_status"} {
		if _, ok := observed[id]; !ok {
			t.Errorf("pre-settle ID %q not captured; observed=%v", id, sortedPaymentIntentIDs(observed))
		}
	}
	if _, ok := observed[""]; ok {
		t.Fatal("empty PaymentIntent ID was captured")
	}
}

func TestPaymentIntentCandidatesExposeAmbiguity(t *testing.T) {
	single := paymentIntentCandidate{id: "pi_unique", status: "requires_action", invoiceID: "in_unique"}
	got, err := requireUniquePaymentIntentCandidate([]paymentIntentCandidate{single})
	if err != nil {
		t.Fatalf("unique candidate error = %v, want nil", err)
	}
	if got != single {
		t.Fatalf("unique candidate = %+v, want exact candidate %+v", got, single)
	}

	rows := []any{
		map[string]any{"id": "pi_a", "status": "requires_action", "metadata": map[string]any{"lago_invoice_id": "in_a"}},
		map[string]any{"id": "pi_b", "status": "requires_confirmation", "metadata": map[string]any{"lago_invoice_id": "in_b"}},
	}
	eligible, _ := paymentIntentCandidates(rows)
	if len(eligible) != 2 {
		t.Fatalf("ambiguous candidates = %+v, want both candidates exposed", eligible)
	}
	if _, err := requireUniquePaymentIntentCandidate(eligible); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("ambiguous candidate error = %v, want count diagnostic", err)
	}
	if _, err := requireUniquePaymentIntentCandidate(nil); err == nil || !strings.Contains(err.Error(), "found 0") {
		t.Fatalf("missing candidate error = %v, want count diagnostic", err)
	}
}

func TestSucceededPaymentIntentEnforcesPreSettleIdentity(t *testing.T) {
	tests := []struct {
		name      string
		expected  string
		preSettle []string
		rows      []any
		wantID    string
		wantErr   string
	}{
		{name: "expected plus older captured success", expected: "pi_expected", preSettle: []string{"pi_expected", "pi_old"}, rows: []any{
			map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
		}, wantID: "pi_expected"},
		{name: "new linked success rejected", expected: "pi_expected", preSettle: []string{"pi_expected"}, rows: []any{
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
			map[string]any{"id": "pi_new", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_new"}},
		}, wantErr: "pi_new"},
		{name: "new unlinked success ignored", expected: "pi_expected", preSettle: []string{"pi_expected"}, rows: []any{
			map[string]any{"id": "pi_new", "status": "succeeded"},
			map[string]any{"id": "pi_expected", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_expected"}},
		}, wantID: "pi_expected"},
		{name: "absent expected reports expected and observed", expected: "pi_expected", preSettle: []string{"pi_expected", "pi_old"}, rows: []any{
			map[string]any{"id": "pi_old", "status": "succeeded", "metadata": map[string]any{"lago_invoice_id": "in_old"}},
		}, wantErr: "expected succeeded invoice-linked PaymentIntent \"pi_expected\" not found; observed succeeded invoice-linked IDs=[pi_old]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preSettle := make(map[string]struct{}, len(tt.preSettle))
			for _, id := range tt.preSettle {
				preSettle[id] = struct{}{}
			}
			got, err := succeededPaymentIntent(tt.expected, preSettle, tt.rows)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selection error = %v", err)
			}
			if got == nil || got["id"] != tt.wantID {
				t.Fatalf("selected intent ID = %v, want %q", got["id"], tt.wantID)
			}
		})
	}
}

// deliverWebhookEvent POSTs the REAL PI event through the built-in webhook
// route, signed with the provider's webhook secret (the transport-leg
// stand-in per D8: production's leg is Stripe's own delivery).
func deliverWebhookEvent(t *testing.T, baseURL, orgID, providerCode, secret string, event map[string]any) int {
	t.Helper()
	payload, _ := json.Marshal(event)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(append([]byte(ts+"."), payload...))
	signature := fmt.Sprintf("t=%s,v1=%x", ts, mac.Sum(nil))
	endpoint := fmt.Sprintf("%s/webhooks/stripe/%s?code=%s", baseURL, orgID, url.QueryEscape(providerCode))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signature)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("webhook deliver: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	return resp.StatusCode
}

// TestLagoIntegrationSettleActivatesGatedSubscription: the full dual-track
// chain on the real pinned stack. Skips (never fails) without the env.
func TestLagoIntegrationSettleActivatesGatedSubscription(t *testing.T) {
	env := integrationEnv(
		"LAGO_INTEGRATION_BASE_URL", "LAGO_INTEGRATION_API_KEY",
		"LAGO_INTEGRATION_STRIPE_KEY", "LAGO_INTEGRATION_STRIPE_SETTLE_PM",
		"LAGO_INTEGRATION_WEBHOOK_SECRET", "LAGO_INTEGRATION_ORG_ID",
		"LAGO_INTEGRATION_GATE_PM",
		"LAGO_INTEGRATION_DB_CONTAINER", "LAGO_INTEGRATION_DB_USER", "LAGO_INTEGRATION_DB_NAME",
	)
	for _, name := range []string{
		"LAGO_INTEGRATION_BASE_URL", "LAGO_INTEGRATION_API_KEY",
		"LAGO_INTEGRATION_STRIPE_KEY", "LAGO_INTEGRATION_STRIPE_SETTLE_PM",
		"LAGO_INTEGRATION_WEBHOOK_SECRET", "LAGO_INTEGRATION_ORG_ID",
		"LAGO_INTEGRATION_DB_CONTAINER", "LAGO_INTEGRATION_DB_USER", "LAGO_INTEGRATION_DB_NAME",
	} {
		if env[name] == "" {
			t.Skip("lago integration env not configured")
		}
	}
	baseURL := env["LAGO_INTEGRATION_BASE_URL"]
	apiKey := env["LAGO_INTEGRATION_API_KEY"]
	stripeKey := env["LAGO_INTEGRATION_STRIPE_KEY"]
	settlePm := env["LAGO_INTEGRATION_STRIPE_SETTLE_PM"]
	webhookSecret := env["LAGO_INTEGRATION_WEBHOOK_SECRET"]
	orgID := env["LAGO_INTEGRATION_ORG_ID"]
	gatePm := env["LAGO_INTEGRATION_GATE_PM"]
	if gatePm == "" {
		gatePm = "pm_card_threeDSecure2Required" // the stuck-window test card (F7)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	tenant := uint64(time.Now().UnixNano() % 1_000_000)
	extCustomer := commercial.ExternalCustomerID(tenant)
	extPurchase := commercial.ExternalPurchaseSubscriptionID(tenant)
	planKey := "pro" // a ladder tier (the publish validation requires it)
	// The channel transaction ids must be unique per RUN: they key the
	// provider Idempotency-Key, and Stripe binds a key to its first
	// request's parameters forever (a replayed key on a different intent
	// answers 400 idempotency_error).
	txn := fmt.Sprintf("t9-txn-%d", time.Now().UnixNano())

	a := NewLagoAdapter(Config{
		Provider: ProviderLago, BaseURL: baseURL, APIKey: apiKey, Release: lockedRelease(t),
		StripeAPIKey: stripeKey, StripeSettlePmToken: settlePm,
		OutboundAllowLoopback: strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, "localhost"),
	})

	// The plan must exist on the authority before the gated create — the
	// real #79 draft+publish flow (the t09 integration precedent).
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
		&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &commercialsvc.FulfillmentRecord{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	plans, err := commercialsvc.NewPlanVersionService(db, a)
	if err != nil {
		t.Fatal(err)
	}
	view, err := plans.CreateDraft(ctx, "integration:t9", commercialsvc.DraftInput{
		PlanKey: planKey, Name: "T9 Pro", AmountFen: 9900, IncludedCreditsMicro: 9_900_000,
		Features: map[string]bool{"advanced_models": true}, Currency: commercial.CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := plans.Publish(ctx, "integration:t9", "t9 evidence", planKey, view.Version); err != nil {
		t.Fatalf("publish: %v", err)
	}
	planCode := commercial.DeterministicPlanCode(planKey, view.Version)

	// The application-side composition behind the wallet grant: a REAL
	// quote cut from the published plan, then the paid purchase order
	// through the repository chain whose ConfirmPayment writes the
	// fulfill:<order> outbox event. The active-state branch below drives
	// the real PurchaseFulfiller over that event — its GrantIncludedCredits
	// creates the wallet the pre-replay snapshot asserts (never a manual
	// wallet seed).
	ordersSvc, err := commercialsvc.NewOrderService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := ordersSvc.CreateQuote(ctx, tenant, planKey)
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	t9OrderID := "ord-" + extPurchase
	seedPaidT9PurchaseOrder(t, db, tenant, quote.ID, t9OrderID, 9900)

	t.Run("gated create leaves the purchase incomplete with a stuck intent", func(t *testing.T) {
		// The gate card rides the provider customer: bind through the
		// purchase create (the adapter attaches StripePmToken as default).
		a.cfg.StripePmToken = gatePm
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindCreatePurchaseSubscription,
			Key:   commercial.CreatePurchaseSubscriptionCommandKey(extPurchase, planCode),
			Actor: "t9", Reason: "integration",
			Payload: commercial.CreatePurchaseSubscriptionPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("gated create: %v", err)
		}
		snap, err := a.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenant})
		if err != nil {
			t.Fatal(err)
		}
		if snap.Purchase.State != commercial.PurchaseStateAwaitingPayment {
			t.Fatalf("fresh gated purchase must be awaiting payment, got %q", snap.Purchase.State)
		}
	})

	t.Run("settle drives the rails and the webhook finalizes", func(t *testing.T) {
		// The authority creates the gating PaymentIntent ASYNC
		// (Invoices::Payments::CreateJob): bounded-wait until the provider
		// rails answer an unsettled intent (the settle command itself
		// stays fail-closed — this wait is the test's own pacing).
		providerCustomerID := providerCustomerOf(t, a, extCustomer)
		var status int
		var body map[string]any
		var expectedID string
		var preSettleIDs map[string]struct{}
		waitDeadline := time.Now().Add(60 * time.Second)
		for {
			status, body = stripeForm(t, stripeKey, http.MethodGet,
				"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=10", nil)
			found := false
			if status == 200 {
				rows, _ := body["data"].([]any)
				eligible, observed := paymentIntentCandidates(rows)
				candidate, candidateErr := requireUniquePaymentIntentCandidate(eligible)
				if candidateErr == nil {
					found = true
					expectedID = candidate.id
					preSettleIDs = observed
				} else if time.Now().After(waitDeadline) {
					t.Fatalf("pre-settle PaymentIntent identity unresolved: %v", candidateErr)
				}
			}
			if found || time.Now().After(waitDeadline) {
				break
			}
			time.Sleep(3 * time.Second)
		}
		if expectedID == "" {
			t.Fatalf("no unique pre-settle invoice-linked unsettled PaymentIntent found before settle (HTTP %d)", status)
		}
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txn),
			Actor: "t9", Reason: "integration settle",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("settle: %v", err)
		}

		// The transport-leg stand-in (D8): read the REAL PI back from the
		// provider and deliver the built-in chain the event it expects.
		// (providerCustomerID/status/body/rows were declared by the async
		// wait above — re-listing answers the succeeded intent.)
		status, body = stripeForm(t, stripeKey, http.MethodGet,
			"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=10", nil)
		if status != 200 {
			t.Fatalf("intent list: HTTP %d", status)
		}
		rows, _ := body["data"].([]any)
		intent, err := succeededPaymentIntent(expectedID, preSettleIDs, rows)
		if err != nil {
			t.Fatal(err)
		}
		event := map[string]any{
			"id":          fmt.Sprintf("evt_t9_%d", time.Now().UnixNano()),
			"object":      "event",
			"api_version": "2024-06-20",
			"created":     time.Now().Unix(),
			"livemode":    false,
			"type":        "payment_intent.succeeded",
			"data":        map[string]any{"object": intent},
		}
		providerCode := paymentProviderCode
		if code := deliverWebhookEvent(t, baseURL, orgID, providerCode, webhookSecret, event); code != 200 {
			t.Fatalf("webhook delivery answered HTTP %d", code)
		}

		finalizeDeadline := time.Now().Add(120 * time.Second)
		for {
			snap, err := a.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindPurchase, TenantID: tenant})
			if err != nil {
				t.Fatal(err)
			}
			if snap.Purchase.State == commercial.PurchaseStateActive {
				if len(snap.Purchase.InvoiceFees) != 1 ||
					snap.Purchase.InvoiceFees[0].Kind != "subscription_fee" ||
					snap.Purchase.InvoiceFees[0].AmountFen <= 0 ||
					snap.Purchase.InvoiceFees[0].AmountFen > 9900 ||
					snap.Purchase.InvoicePaymentStatus != "succeeded" {
					t.Fatalf("D6' re-check failed: %+v", snap.Purchase)
				}
				invoiceID := fmt.Sprint(intent["metadata"].(map[string]any)["lago_invoice_id"])
				// The wallet exists because the application GRANTED it: the
				// real fulfiller drives settle (short-circuits on the active
				// purchase), re-checks the finalized invoice, and grants the
				// first-period credits into the purchase wallet.
				driveT9PurchaseFulfiller(t, ctx, db, a, tenant, t9OrderID)
				beforeReplay, snapshotErr := t9LagoSnapshot(ctx, a, extPurchase, extCustomer, invoiceID)
				if snapshotErr != nil {
					t.Fatalf("capture complete pre-replay Lago snapshot: %v", snapshotErr)
				}
				authorityBeforeReplay := purchaseSnapshotJSON(t, a, tenant)
				webhookRows, readErr := readInboundWebhookRows(ctx, env["LAGO_INTEGRATION_DB_CONTAINER"], env["LAGO_INTEGRATION_DB_USER"], env["LAGO_INTEGRATION_DB_NAME"], orgID, providerCode, event["id"].(string))
				if readErr != nil {
					t.Fatalf("read baseline inbound webhook: %v", readErr)
				}
				baselineWebhookID, gateErr := requireBaselineWebhook(webhookRows)
				if gateErr != nil {
					t.Fatalf("baseline inbound webhook gate: %v", gateErr)
				}
				paymentsBeforeReplay := countLagoSucceededPayments(t, a, extCustomer)
				if paymentsBeforeReplay != 1 {
					t.Fatalf("setup: exactly one succeeded Lago payment expected before webhook replay, got %d", paymentsBeforeReplay)
				}
				if code := deliverWebhookEvent(t, baseURL, orgID, providerCode, webhookSecret, event); code != 200 {
					t.Fatalf("duplicate webhook delivery answered HTTP %d", code)
				}
				replayCtx, replayCancel := context.WithTimeout(ctx, 90*time.Second)
				gateErr = waitForReplayWebhook(replayCtx, func(readCtx context.Context) ([]inboundWebhookRow, error) {
					return readInboundWebhookRows(readCtx, env["LAGO_INTEGRATION_DB_CONTAINER"], env["LAGO_INTEGRATION_DB_USER"], env["LAGO_INTEGRATION_DB_NAME"], orgID, providerCode, event["id"].(string))
				}, baselineWebhookID)
				replayCancel()
				if gateErr != nil {
					t.Fatalf("duplicate inbound webhook did not complete: %v", gateErr)
				}
				afterReplay, snapshotErr := t9LagoSnapshot(ctx, a, extPurchase, extCustomer, invoiceID)
				if snapshotErr != nil {
					t.Fatalf("capture complete post-replay Lago snapshot: %v", snapshotErr)
				}
				if !bytes.Equal(afterReplay, beforeReplay) {
					t.Fatalf("duplicate webhook changed the authority state:\nbefore %s\nafter  %s", beforeReplay, afterReplay)
				}
				if authorityAfterReplay := purchaseSnapshotJSON(t, a, tenant); authorityAfterReplay != authorityBeforeReplay {
					t.Fatalf("duplicate webhook changed authority state:\nbefore %s\nafter %s", authorityBeforeReplay, authorityAfterReplay)
				}
				paymentsAfterReplay := countLagoSucceededPayments(t, a, extCustomer)
				if paymentsAfterReplay != 1 {
					t.Fatalf("duplicate webhook must leave exactly one succeeded Lago payment: before=%d after=%d", paymentsBeforeReplay, paymentsAfterReplay)
				}
				return
			}
			if time.Now().After(finalizeDeadline) {
				t.Fatalf("authority did not finalize within 120s, state=%q", snap.Purchase.State)
			}
			time.Sleep(3 * time.Second)
		}
	})

	t.Run("settle replay is a zero-side-effect no-op", func(t *testing.T) {
		before := purchaseSnapshotJSON(t, a, tenant)
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txn),
			Actor: "t9", Reason: "integration settle replay",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txn, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("replay settle: %v", err)
		}
		after := purchaseSnapshotJSON(t, a, tenant)
		if before != after {
			t.Fatalf("replayed settle changed the authority state:\nbefore %s\nafter  %s", before, after)
		}
	})

	// (#84 Task 6 / AC3 authority side) A SECOND settle under a DIFFERENT
	// channel transaction — the multiple-success second payment's shape, the
	// command a buggy coordinator would drive — must short-circuit on the
	// already-active purchase (step (i)): nil error + receipt, and EXACTLY
	// ONE succeeded Lago Payment before and after (never a second Stripe
	// charge, never a second Payment row). Merged as a subtest of the
	// activation case on purpose: reaching active REQUIRES the full
	// gated-create → settle → webhook chain (local activation is forbidden,
	// spec L123-125), so an independent function would re-run the whole
	// multi-minute chain for the same assertion.
	t.Run("TestSettleSecondChannelTransactionShortCircuitsWhenActive", func(t *testing.T) {
		before := countLagoSucceededPayments(t, a, extCustomer)
		if before != 1 {
			t.Fatalf("setup: exactly one succeeded Lago payment expected after the single activation, got %d", before)
		}
		txnOther := txn + "-OTHER"
		if _, err := a.SubmitCommand(ctx, commercial.Command{
			Kind:  commercial.CommandKindSettlePurchasePayment,
			Key:   commercial.SettlePurchasePaymentCommandKey(extPurchase, txnOther),
			Actor: "t9", Reason: "integration second-channel settle must short-circuit",
			Payload: commercial.SettlePurchasePaymentPayload{
				TenantID: tenant, ExternalCustomerID: extCustomer,
				ExternalPurchaseSubscriptionID: extPurchase, PlanCode: planCode,
				ChannelTransaction: txnOther, AmountFen: 9900, Currency: commercial.CurrencyCNY,
			},
		}); err != nil {
			t.Fatalf("the second-channel settle must short-circuit with nil error (idempotent receipt), got %v", err)
		}
		after := countLagoSucceededPayments(t, a, extCustomer)
		if after != before {
			t.Fatalf("the short-circuited second settle must not mint another Lago payment: before=%d after=%d", before, after)
		}
	})
}

// countLagoSucceededPayments reads the authority's Payment ledger for one
// external customer (#84 Task 6): the authoritative count of succeeded
// payments behind the "never a second Lago Payment" assertion.
func countLagoSucceededPayments(t *testing.T, a *LagoAdapter, extCustomer string) int {
	t.Helper()
	status, body, err := a.do(context.Background(), http.MethodGet,
		"/api/v1/payments?external_customer_id="+url.QueryEscape(extCustomer)+"&per_page=100", nil)
	if err != nil || status != 200 {
		t.Fatalf("lago payments read: HTTP %d err=%v", status, err)
	}
	var parsed struct {
		Payments []struct {
			Status string `json:"status"`
		} `json:"payments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("lago payments parse: %v", err)
	}
	n := 0
	for _, p := range parsed.Payments {
		if p.Status == "succeeded" {
			n++
		}
	}
	return n
}

// seedPaidT9PurchaseOrder plants the paid purchase order through the REAL
// repository chain (CreateOrder / RegisterAttempt / ConfirmPayment — the
// same seam the unit suite drives): ConfirmPayment itself writes the
// pending fulfill:<order> outbox event a crashed worker would leave.
func seedPaidT9PurchaseOrder(t *testing.T, db *gorm.DB, tenant uint64, quoteID, orderID string, amountFen int64) {
	t.Helper()
	store := repocommercial.NewOrderStore(db)
	ctx := context.Background()
	if err := store.CreateOrder(ctx, repocommercial.OrderRow{
		ID: orderID, TenantID: tenant, QuoteID: quoteID, Kind: commercial.OrderKindPurchase,
		AmountFen: amountFen, Currency: commercial.CurrencyCNY,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att-" + orderID, TenantID: tenant, OrderID: orderID, Provider: "stripe", Merchant: "weknora",
		MerchantOrderID: "mo-" + orderID, AmountFen: amountFen, Currency: commercial.CurrencyCNY,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmPayment(ctx, commercial.PaymentFact{
		TenantID: tenant, OrderID: orderID, AttemptID: "mo-" + orderID, Provider: "stripe", Merchant: "weknora",
		Transaction: "t9-" + orderID, Amount: commercial.CNYFen(amountFen), Currency: commercial.CurrencyCNY, State: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
}

// driveT9PurchaseFulfiller runs the REAL application fulfillment seam
// (commercialsvc.PurchaseFulfiller over this adapter + sqlite) on the
// pending fulfill event: settle short-circuits against the already-active
// purchase and GrantIncludedCredits creates the purchase wallet in the
// real authority. A silent pending outcome (nil without fulfillment)
// fails the final order-state assert — the wallet must come from the
// application path, not from a seed.
func driveT9PurchaseFulfiller(t *testing.T, parent context.Context, db *gorm.DB, platform commercial.CommercialPlatform, tenant uint64, orderID string) {
	t.Helper()
	var ev repocommercial.OutboxEvent
	if err := db.WithContext(parent).Where("event_key = ?", repocommercial.OutboxKindFulfill+":"+orderID).First(&ev).Error; err != nil {
		t.Fatalf("fulfill outbox event missing: %v", err)
	}
	fulfiller, err := commercialsvc.NewPurchaseFulfiller(db, platform, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fulfiller.Fulfill(parent, ev); err != nil {
		t.Fatalf("purchase fulfiller: %v", err)
	}
	row, err := repocommercial.NewOrderStore(db).GetOrder(parent, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Domain().State != commercial.OrderStateFulfilled {
		t.Fatalf("purchase fulfiller left the order %q — the wallet grant must complete through the real application path", row.Domain().State)
	}
}

func providerCustomerOf(t *testing.T, a *LagoAdapter, extCustomer string) string {	t.Helper()
	status, body, err := a.do(context.Background(), http.MethodGet,
		"/api/v1/customers/"+url.PathEscape(extCustomer), nil)
	if err != nil || status != 200 {
		t.Fatalf("customer read: HTTP %d err=%v", status, err)
	}
	var parsed struct {
		Customer struct {
			BillingConfiguration struct {
				ProviderCustomerID string `json:"provider_customer_id"`
			} `json:"billing_configuration"`
		} `json:"customer"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil ||
		parsed.Customer.BillingConfiguration.ProviderCustomerID == "" {
		t.Fatalf("customer binding malformed")
	}
	return parsed.Customer.BillingConfiguration.ProviderCustomerID
}

func purchaseSnapshotJSON(t *testing.T, a *LagoAdapter, tenant uint64) string {
	t.Helper()
	snap, err := a.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindPurchase, TenantID: tenant,
	})
	if err != nil {
		t.Fatal(err)
	}
	// CheckedAt is the read's own clock (always different); the no-op
	// comparison is over every AUTHORITY-sourced field.
	stable := struct {
		State                string                           `json:"state"`
		PlanCode             string                           `json:"plan_code"`
		AmountFen            int64                            `json:"amount_fen"`
		Currency             string                           `json:"currency"`
		InvoiceFees          []commercial.InvoiceLineSnapshot `json:"invoice_fees"`
		InvoicePaymentStatus string                           `json:"invoice_payment_status"`
	}{
		State:                snap.Purchase.State,
		PlanCode:             snap.Purchase.PlanCode,
		AmountFen:            snap.Purchase.AmountFen,
		Currency:             snap.Purchase.Currency,
		InvoiceFees:          snap.Purchase.InvoiceFees,
		InvoicePaymentStatus: snap.Purchase.InvoicePaymentStatus,
	}
	blob, _ := json.Marshal(stable)
	return string(blob)
}
