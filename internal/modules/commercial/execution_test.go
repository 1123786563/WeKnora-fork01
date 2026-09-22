package commercial

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestUnknownFundingCannotBypassChargeGate(t *testing.T) {
	if ValidateFunding("from_model_argument") == nil {
		t.Fatal("untrusted funding accepted")
	}
	if ValidateFunding("platform") != nil || ValidateFunding("byok") != nil {
		t.Fatal("known funding rejected")
	}
}

// TestPerServiceDimensionGating pins that every billable service is gated by
// its OWN settlement dimension: no shared "model" catch-all, and unknown
// services cannot ride an existing dimension.
func TestPerServiceDimensionGating(t *testing.T) {
	want := map[string]string{
		ServiceModel:     DimensionModel,
		ServiceParsing:   DimensionParsing,
		ServiceEmbedding: DimensionEmbedding,
		ServiceRerank:    DimensionRerank,
		ServiceSandbox:   DimensionSandbox,
		ServiceConnector: DimensionConnector,
	}
	for service, dimension := range want {
		got, err := ServiceDimension(service)
		if err != nil {
			t.Fatalf("service %s: %v", service, err)
		}
		if got != dimension {
			t.Fatalf("service %s gated by %s, want %s", service, got, dimension)
		}
	}
	if _, err := ServiceDimension("free_tier_argument"); err == nil {
		t.Fatal("unknown service accepted without a gating dimension")
	}
}

// TestBYOKWaivesOnlyModelDimension: a BYOK model call records 0 platform
// model cost, but parsing/sandbox/embedding/rerank/connector stays billable
// under BYOK funding — the waiver never leaks across dimensions.
func TestBYOKWaivesOnlyModelDimension(t *testing.T) {
	byok := UsageFact{
		TenantID: 1, RunID: "run", CallID: "c", AttemptID: "a1", Funding: FundingBYOK,
		Service: ServiceSandbox, PriceVersion: "v1", Revision: 1, OccurredAt: time.Now(),
		Dimensions: map[string]int64{DimensionModel: 1000, DimensionSandbox: 60}, Status: UsageStatusFinal,
	}
	got := byok.BillableDimensions()
	if _, ok := got[DimensionModel]; ok {
		t.Fatal("byok funding still billed the model dimension")
	}
	if got[DimensionSandbox] != 60 {
		t.Fatal("byok funding waived the sandbox dimension")
	}
}

// TestExecutionLimitsEnforced: model-output and sandbox-duration upper
// bounds are actually enforced at the gate, not advisory.
func TestExecutionLimitsEnforced(t *testing.T) {
	limits := ExecutionLimits{MaxModelOutputTokens: 1000, MaxSandboxSeconds: 30}
	overOutput := UsageFact{Funding: FundingPlatform, Service: ServiceModel,
		Dimensions: map[string]int64{DimensionOutput: 1001}, Status: UsageStatusFinal}
	if !errors.Is(limits.Enforce(overOutput), ErrOverLimit) {
		t.Fatal("model output over the cap was not rejected")
	}
	overDuration := UsageFact{Funding: FundingPlatform, Service: ServiceSandbox,
		Dimensions: map[string]int64{DimensionDuration: 31}, Status: UsageStatusFinal}
	if !errors.Is(limits.Enforce(overDuration), ErrOverLimit) {
		t.Fatal("sandbox duration over the cap was not rejected")
	}
	okModel := UsageFact{Funding: FundingPlatform, Service: ServiceModel,
		Dimensions: map[string]int64{DimensionOutput: 1000}, Status: UsageStatusFinal}
	if err := limits.Enforce(okModel); err != nil {
		t.Fatalf("in-cap model fact rejected: %v", err)
	}
}

// TestTrustedUsageRejectsClientFundingAndRollups: Finish accepts only
// server-derived funding and only final deltas — client-supplied funding and
// display_only parent aggregates (child calls already settled) are rejected,
// so a parent rollup can never double-count its children.
func TestTrustedUsageRejectsClientFundingAndRollups(t *testing.T) {
	client := UsageFact{Funding: "from_model_argument", Status: UsageStatusFinal}
	if !errors.Is(TrustedUsageFact(client), ErrUntrustedUsage) {
		t.Fatal("client-supplied funding accepted at Finish")
	}
	rollup := UsageFact{Funding: FundingPlatform, Status: UsageStatusDisplayOnly}
	if !errors.Is(TrustedUsageFact(rollup), ErrNotBillable) {
		t.Fatal("display_only parent aggregate accepted for settlement")
	}
	final := UsageFact{Funding: FundingPlatform, Status: UsageStatusFinal}
	if err := TrustedUsageFact(final); err != nil {
		t.Fatalf("trusted final fact rejected: %v", err)
	}
}

// TestAbnormalCostStopsFurtherDispatch: a settle above the reserved upper
// bound is abnormal and blocks any later Begin until the run is retried.
func TestAbnormalCostStopsFurtherDispatch(t *testing.T) {
	if !errors.Is(CheckAbnormal(1500, 1000), ErrAbnormalCost) {
		t.Fatal("charge above the reservation upper bound not flagged abnormal")
	}
	if err := CheckAbnormal(1000, 1000); err != nil {
		t.Fatalf("in-bound charge flagged abnormal: %v", err)
	}
}

// fakeGate records Begin/Finish calls so denial semantics are observable.
type fakeGate struct {
	beginErr  error
	finishErr error
	begins    int
	finishes  int
}

func (f *fakeGate) Begin(ctx context.Context, req BudgetRequest) (Reservation, error) {
	f.begins++
	if f.beginErr != nil {
		return Reservation{}, f.beginErr
	}
	return Reservation{ID: req.Key, RunID: req.RunID, State: ReservationStateHeld, Upper: req.Upper}, nil
}

func (f *fakeGate) Finish(ctx context.Context, reservationID string, fact UsageFact) error {
	f.finishes++
	return f.finishErr
}

// TestGateDenialStopsDispatch: when Begin denies (insufficient budget), the
// reservation is zero and the dispatcher must treat the call as stopped.
func TestGateDenialStopsDispatch(t *testing.T) {
	g := &fakeGate{beginErr: ErrInsufficientBudgetGate}
	var gate ExecutionGate = g
	res, err := gate.Begin(context.Background(), BudgetRequest{TenantID: 1, RunID: "r", Key: "k", Upper: 10, Deadline: time.Now().Add(time.Minute)})
	if err == nil {
		t.Fatal("denied Begin returned no error")
	}
	if res.ID != "" {
		t.Fatal("denied Begin returned a usable reservation")
	}
	if g.begins != 1 {
		t.Fatal("gate Begin not exercised")
	}
}
