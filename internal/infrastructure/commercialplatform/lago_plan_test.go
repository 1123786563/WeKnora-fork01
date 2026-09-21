package commercialplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// stubReq is one recorded stub request.
type stubReq struct {
	Method string
	Path   string
	Body   string
	Auth   string
	Status int
}

// planStub stands in for the Lago API on the publish path (the healthStub
// pattern): it records every request and serves scripted statuses. The
// create statuses pop in order and then repeat the last, so a [201, 422]
// script answers the first POST 201 and every replay 422 — the
// indistinguishable-422 fact from the #76/#74 labs.
type planStub struct {
	mu       sync.Mutex
	requests []stubReq

	createStatuses     []int
	readbackStatus     int
	readbackAmountCen  int64
	readbackCurrency   string
	readbackInterval   string
	readbackPayAdvance bool

	metricStatus int
	metricID     string

	featureStatus int
	entitleStatus int

	server *httptest.Server
}

func newPlanStub(t *testing.T) *planStub {
	t.Helper()
	s := &planStub{
		createStatuses:     []int{http.StatusCreated},
		readbackStatus:     http.StatusOK,
		readbackCurrency:   "CNY",
		readbackInterval:   "monthly",
		readbackPayAdvance: true,
		metricStatus:       http.StatusOK,
		metricID:           "lagoid-metric-42",
		featureStatus:      http.StatusOK,
		entitleStatus:      http.StatusOK,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/plans", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		s.record(w, r, s.nextCreateStatus(), `{"plan":{"code":"weknora-stub"}}`)
	})
	mux.HandleFunc("/api/v1/plans/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/")
		code, sub, _ := strings.Cut(rest, "/")
		switch {
		case r.Method == http.MethodGet:
			s.recordRaw(w, r, func() int {
				body := map[string]any{
					"plan": map[string]any{
						"code":            code,
						"amount_cents":    s.readbackAmountCen,
						"amount_currency": s.readbackCurrency,
						"interval":        s.readbackInterval,
						"pay_in_advance":  s.readbackPayAdvance,
					},
				}
				writeJSON(w, s.readbackStatus, body)
				return s.readbackStatus
			})
		case sub == "entitlements" && r.Method == http.MethodPost:
			s.record(w, r, s.entitleStatus, `{}`)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/v1/billable_metrics/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		status, id := s.metricStatus, s.metricID
		s.mu.Unlock()
		s.record(w, r, status, fmt.Sprintf(`{"billable_metric":{"lago_id":%q}}`, id))
	})
	mux.HandleFunc("/api/v1/features", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		status := s.featureStatus
		s.mu.Unlock()
		s.record(w, r, status, `{}`)
	})
	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

func (s *planStub) record(w http.ResponseWriter, r *http.Request, status int, body string) {
	s.recordRaw(w, r, func() int {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		return status
	})
}

func (s *planStub) recordRaw(w http.ResponseWriter, r *http.Request, respond func() int) {
	blob, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	s.mu.Lock()
	s.requests = append(s.requests, stubReq{
		Method: r.Method, Path: r.URL.Path, Body: string(blob),
		Auth: r.Header.Get("Authorization"),
	})
	s.mu.Unlock()
	s.requests[len(s.requests)-1].Status = respond()
}

func (s *planStub) nextCreateStatus() int {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *planStub) recorded() []stubReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubReq(nil), s.requests...)
}

// countCreatePosts counts ACCEPTED (2xx) plan creates — the no-double-plan
// proof. A replay POST that answers 422 is not a create; it is the
// indistinguishable-already-exists signal the adapter resolves by read-back.
func (s *planStub) countCreatePosts() int {
	n := 0
	for _, r := range s.recorded() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/plans" && r.Status >= 200 && r.Status < 300 {
			n++
		}
	}
	return n
}

func (s *planStub) find(path, method string) []stubReq {
	var out []stubReq
	for _, r := range s.recorded() {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func (s *planStub) url() string { return s.server.URL }

func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	blob, _ := json.Marshal(body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(blob)
}

// planPayload builds a valid publish payload; charges/features optional.
func planPayload() commercial.PublishPlanVersionPayload {
	return commercial.PublishPlanVersionPayload{
		PlanKey:              "stub-pro",
		Version:              1,
		PlanCode:             commercial.DeterministicPlanCode("stub-pro", 1),
		Name:                 "Stub Pro",
		Interval:             "monthly",
		AmountFen:            9900,
		Currency:             "CNY",
		PayInAdvance:         true,
		IncludedCreditsMicro: 9_900_000,
	}
}

func submitPublish(ctx context.Context, p interface {
	SubmitCommand(context.Context, commercial.Command) (commercial.CommandReceipt, error)
}, payload commercial.PublishPlanVersionPayload) (commercial.CommandReceipt, error) {
	return p.SubmitCommand(ctx, commercial.Command{
		Kind:    commercial.CommandKindPublishPlanVersion,
		Key:     commercial.PublishCommandKey(payload.PlanKey, payload.Version),
		Actor:   "operator-1",
		Reason:  "t07",
		Payload: payload,
	})
}

func decodeStubBody(t *testing.T, r stubReq) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(r.Body))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", r.Body, err)
	}
	return m
}

// TestLagoPublishPlanHappyPath: the create POST carries the exact pinned
// contract — pay_in_advance true, trial_period 0, integer amount_cents,
// CNY, Authorization header — and the receipt echoes the payload's plan
// code.
func TestLagoPublishPlanHappyPath(t *testing.T) {
	stub := newPlanStub(t)
	stub.readbackAmountCen = 9900
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	payload := planPayload()
	payload.Charges = []commercial.PlanCharge{{
		Dimension: "model-units", Model: commercial.ChargeModelFixedUnit, AmountFen: 7,
	}}
	payload.Features = map[string]bool{"api_access": true}

	receipt, err := submitPublish(context.Background(), p, payload)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if receipt.ExternalID != payload.PlanCode {
		t.Fatalf("receipt ExternalID = %q, want the payload plan code %q", receipt.ExternalID, payload.PlanCode)
	}
	if receipt.Key != commercial.PublishCommandKey(payload.PlanKey, payload.Version) {
		t.Fatalf("receipt Key = %q", receipt.Key)
	}
	if receipt.RecordedAt.IsZero() {
		t.Fatal("receipt RecordedAt must be set")
	}

	creates := stub.find("/api/v1/plans", http.MethodPost)
	// one plan create + one entitlement attach
	if stub.countCreatePosts() != 1 {
		t.Fatalf("exactly one POST /api/v1/plans expected, got %d", stub.countCreatePosts())
	}
	create := decodeStubBody(t, creates[0])
	if creates[0].Auth != "Bearer "+testAPIKey {
		t.Fatalf("Authorization header = %q, want Bearer <api key>", creates[0].Auth)
	}
	plan := create["plan"].(map[string]any)
	if plan["pay_in_advance"] != true {
		t.Fatalf("pay_in_advance = %v, want true", plan["pay_in_advance"])
	}
	if n, ok := plan["trial_period"].(json.Number); !ok || n.String() != "0" {
		t.Fatalf("trial_period = %v, want 0", plan["trial_period"])
	}
	if n, ok := plan["amount_cents"].(json.Number); !ok || n.String() != "9900" {
		t.Fatalf("amount_cents = %v, want integer 9900", plan["amount_cents"])
	}
	if plan["amount_currency"] != "CNY" {
		t.Fatalf("amount_currency = %v", plan["amount_currency"])
	}
	if plan["code"] != payload.PlanCode {
		t.Fatalf("plan code = %v", plan["code"])
	}
}

// TestLagoPublishPlanReplayReadBack: a 422 on the create POST is resolved
// by a read-back GET whose compared fields match — idempotent replay
// returns a receipt, and the adapter provably issued the read-back.
func TestLagoPublishPlanReplayReadBack(t *testing.T) {
	stub := newPlanStub(t)
	stub.createStatuses = []int{http.StatusCreated, http.StatusUnprocessableEntity}
	stub.readbackAmountCen = 9900
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	payload := planPayload()

	if _, err := submitPublish(context.Background(), p, payload); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	receipt, err := submitPublish(context.Background(), p, payload)
	if err != nil {
		t.Fatalf("replay publish: %v", err)
	}
	if receipt.ExternalID != payload.PlanCode {
		t.Fatalf("replay receipt ExternalID = %q", receipt.ExternalID)
	}
	if stub.countCreatePosts() != 1 {
		t.Fatalf("the 422 replay must not create a second plan, got %d accepted creates", stub.countCreatePosts())
	}
	// The read-back GET is the proof the adapter verified the replay.
	reads := stub.find("/api/v1/plans/"+payload.PlanCode, http.MethodGet)
	if len(reads) != 1 {
		t.Fatalf("the adapter must issue exactly one read-back GET, got %d", len(reads))
	}
}

// TestLagoPublishPlanConflict: 422 + a read-back with DIFFERENT content is
// a content conflict — ErrPlatformInvalidResponse (spec story 59).
func TestLagoPublishPlanConflict(t *testing.T) {
	stub := newPlanStub(t)
	stub.createStatuses = []int{http.StatusUnprocessableEntity}
	stub.readbackAmountCen = 19_900 // different from the payload's 9900
	p := NewLagoAdapter(lagoTestConfig(stub.url()))

	_, err := submitPublish(context.Background(), p, planPayload())
	if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
		t.Fatalf("content conflict must be ErrPlatformInvalidResponse, got %v", err)
	}
}

// TestLagoPublishPlanFailureClasses: other 4xx is a definitive wrong
// answer, 5xx/transport are unreachable, missing config is unconfigured.
func TestLagoPublishPlanFailureClasses(t *testing.T) {
	ctx := context.Background()

	t.Run("other 4xx", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.createStatuses = []int{http.StatusBadRequest}
		_, err := submitPublish(ctx, NewLagoAdapter(lagoTestConfig(stub.url())), planPayload())
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("want invalid response, got %v", err)
		}
	})
	t.Run("5xx", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.createStatuses = []int{http.StatusInternalServerError}
		_, err := submitPublish(ctx, NewLagoAdapter(lagoTestConfig(stub.url())), planPayload())
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})
	t.Run("connection refused", func(t *testing.T) {
		_, err := submitPublish(ctx, NewLagoAdapter(lagoTestConfig("http://127.0.0.1:1")), planPayload())
		if !errors.Is(err, commercial.ErrPlatformUnreachable) {
			t.Fatalf("want unreachable, got %v", err)
		}
	})
	t.Run("unconfigured", func(t *testing.T) {
		p := NewLagoAdapter(Config{Provider: ProviderLago})
		_, err := submitPublish(ctx, p, planPayload())
		if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
			t.Fatalf("want unconfigured, got %v", err)
		}
	})
}

// TestLagoPublishPlanChargeResolution: the dimension resolves to a
// billable_metric_id reference (T04 fact: id, not code) and the fen amount
// crosses as the exact decimal string ×100; an unprovisioned dimension
// fails closed with NO plan create.
func TestLagoPublishPlanChargeResolution(t *testing.T) {
	stub := newPlanStub(t)
	stub.readbackAmountCen = 9900
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	payload := planPayload()
	payload.Charges = []commercial.PlanCharge{{
		Dimension: "model-units", Model: commercial.ChargeModelFixedUnit, AmountFen: 7,
	}}

	if _, err := submitPublish(context.Background(), p, payload); err != nil {
		t.Fatalf("publish with charge: %v", err)
	}
	creates := stub.find("/api/v1/plans", http.MethodPost)
	plan := decodeStubBody(t, creates[0])["plan"].(map[string]any)
	charges := plan["charges"].([]any)
	if len(charges) != 1 {
		t.Fatalf("want one charge, got %v", charges)
	}
	charge := charges[0].(map[string]any)
	if charge["billable_metric_id"] != "lagoid-metric-42" {
		t.Fatalf("billable_metric_id = %v, want the resolved id", charge["billable_metric_id"])
	}
	props := charge["properties"].(map[string]any)
	if props["amount"] != "0.07" {
		t.Fatalf("properties.amount = %v, want decimal string 0.07", props["amount"])
	}
	if charge["charge_model"] != "standard" {
		t.Fatalf("fixed_unit maps to standard, got %v", charge["charge_model"])
	}

	t.Run("package charge maps with package_size", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.readbackAmountCen = 9900
		payload := planPayload()
		payload.Charges = []commercial.PlanCharge{{
			Dimension: "tool-calls", Model: commercial.ChargeModelPackage,
			AmountFen: 1500, PackageUnits: 100, FreeUnits: 10,
		}}
		if _, err := submitPublish(context.Background(), NewLagoAdapter(lagoTestConfig(stub.url())), payload); err != nil {
			t.Fatalf("package publish: %v", err)
		}
		creates := stub.find("/api/v1/plans", http.MethodPost)
		charge := decodeStubBody(t, creates[0])["plan"].(map[string]any)["charges"].([]any)[0].(map[string]any)
		if charge["charge_model"] != "package" {
			t.Fatalf("package model mapping, got %v", charge["charge_model"])
		}
		props := charge["properties"].(map[string]any)
		if props["amount"] != "15.00" || props["package_size"].(json.Number).String() != "100" || props["free_units"].(json.Number).String() != "10" {
			t.Fatalf("package properties mismatch: %v", props)
		}
	})

	t.Run("unprovisioned dimension fails closed before create", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.metricStatus = http.StatusNotFound
		payload := planPayload()
		payload.Charges = []commercial.PlanCharge{{Dimension: "ghost", Model: commercial.ChargeModelFixedUnit, AmountFen: 7}}
		_, err := submitPublish(context.Background(), NewLagoAdapter(lagoTestConfig(stub.url())), payload)
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("unprovisioned dimension must fail closed invalid, got %v", err)
		}
		if stub.countCreatePosts() != 0 {
			t.Fatal("no plan may be created when a dimension cannot be resolved")
		}
	})
}

// TestLagoPublishPlanEntitlementMapForm: the entitlement attach body is a
// MAP keyed by feature code (T02 lab fact: an array makes the runtime 500).
func TestLagoPublishPlanEntitlementMapForm(t *testing.T) {
	stub := newPlanStub(t)
	stub.readbackAmountCen = 9900
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	payload := planPayload()
	payload.Features = map[string]bool{"api_access": true, "priority_support": false}

	if _, err := submitPublish(context.Background(), p, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}
	attaches := stub.find("/api/v1/plans/"+payload.PlanCode+"/entitlements", http.MethodPost)
	if len(attaches) != 1 {
		t.Fatalf("exactly one entitlement attach expected, got %d", len(attaches))
	}
	body := decodeStubBody(t, attaches[0])
	ent, ok := body["entitlements"].(map[string]any)
	if !ok {
		t.Fatalf("entitlements must be a MAP (hash keyed by feature code), got %T", body["entitlements"])
	}
	if _, has := ent["api_access"]; !has {
		t.Fatalf("entitlements map must key the feature codes, got %v", ent)
	}
	if _, has := ent["priority_support"]; !has {
		t.Fatalf("every payload feature key must attach, got %v", ent)
	}
	// The feature registration also happened (ensure step).
	if regs := stub.find("/api/v1/features", http.MethodPost); len(regs) != 2 {
		t.Fatalf("each feature must be ensured once, got %d", len(regs))
	}
}

// TestLagoPublishPlanErrorsLeakNothing: no provider URL, path, marker or
// credential rides along in any error string, across every failure class.
func TestLagoPublishPlanErrorsLeakNothing(t *testing.T) {
	assertClean := func(t *testing.T, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, marker := range []string{"lago", "http://", "/api/", testAPIKey, "127.0.0.1"} {
			if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(marker)) {
				t.Fatalf("error %q leaks %q", err.Error(), marker)
			}
		}
	}
	t.Run("conflict", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.createStatuses = []int{http.StatusUnprocessableEntity}
		stub.readbackAmountCen = 1
		_, err := submitPublish(context.Background(), NewLagoAdapter(lagoTestConfig(stub.url())), planPayload())
		assertClean(t, err)
	})
	t.Run("unprovisioned dimension", func(t *testing.T) {
		stub := newPlanStub(t)
		stub.metricStatus = http.StatusNotFound
		payload := planPayload()
		payload.Charges = []commercial.PlanCharge{{Dimension: "model-units", Model: commercial.ChargeModelFixedUnit, AmountFen: 7}}
		_, err := submitPublish(context.Background(), NewLagoAdapter(lagoTestConfig(stub.url())), payload)
		assertClean(t, err)
	})
	t.Run("unreachable", func(t *testing.T) {
		_, err := submitPublish(context.Background(), NewLagoAdapter(lagoTestConfig("http://127.0.0.1:1")), planPayload())
		assertClean(t, err)
	})
	t.Run("unconfigured", func(t *testing.T) {
		_, err := submitPublish(context.Background(), NewLagoAdapter(Config{Provider: ProviderLago, APIKey: testAPIKey}), planPayload())
		assertClean(t, err)
	})
}
