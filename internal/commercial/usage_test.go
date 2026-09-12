package commercial

import (
	"errors"
	"testing"
	"time"
)

func TestBYOKDoesNotChargeModelTokens(t *testing.T) {
	if BillableModel("byok") {
		t.Fatal("BYOK charged")
	}
	if !BillableModel("platform") {
		t.Fatal("platform model omitted")
	}
	if BillableModel("untrusted") {
		t.Fatal("unknown funding treated as paid")
	}
}

// Funding comes from the server-side credential binding, never from the
// caller's claim: unknown funding must be rejected before dispatch rather
// than silently free-riding as an unpaid call.
func TestUsageFundingValidationRejectsUnknownBeforeDispatch(t *testing.T) {
	if err := ValidateFunding("platform"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFunding("byok"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFunding("untrusted"); !errors.Is(err, ErrUnknownFunding) {
		t.Fatalf("unknown funding accepted: %v", err)
	}
}

func TestUsageFactRequiresKnownFunding(t *testing.T) {
	fact := usageTestFact()
	if err := fact.Validate(); err != nil {
		t.Fatal(err)
	}
	fact.Funding = "spoofed"
	if err := fact.Validate(); !errors.Is(err, ErrUnknownFunding) {
		t.Fatalf("forged funding accepted: %v", err)
	}
}

// BYOK waives ONLY the model dimension; sandbox and parsing stay
// independent billable service facts.
func TestUsageBYOKWaivesOnlyModelDimension(t *testing.T) {
	fact := usageTestFact()
	fact.Funding = "byok"
	billable := fact.BillableDimensions()
	if _, ok := billable[DimensionModel]; ok {
		t.Fatal("BYOK model dimension billed")
	}
	if billable[DimensionSandbox] != 2 {
		t.Fatalf("sandbox units lost: %d", billable[DimensionSandbox])
	}
	if billable[DimensionParsing] != 3 {
		t.Fatalf("parsing units lost: %d", billable[DimensionParsing])
	}
	platform := usageTestFact().BillableDimensions()
	if platform[DimensionModel] != 1 {
		t.Fatalf("platform model units lost: %d", platform[DimensionModel])
	}
}

// Rates are fixed-point per immutable price version; arithmetic stays exact
// in big.Rat and rounds exactly once, at the end of the physical call.
func TestUsageChargeRoundsOnceAtCallEnd(t *testing.T) {
	rates := PriceVersionRates{Version: "pv-1", Rates: map[string]DimensionRate{
		DimensionModel:   {RateMicro: 1, Units: 3},
		DimensionSandbox: {RateMicro: 1, Units: 3},
	}}
	fact := usageTestFact()
	fact.Dimensions = map[string]int64{DimensionModel: 1, DimensionSandbox: 1}
	charge, err := rates.ChargeForCall(fact)
	if err != nil {
		t.Fatal(err)
	}
	// Each dimension is exactly 1/3 micro-credit; rounding per dimension
	// would yield 0 while the single end-of-call rounding yields 1.
	if charge != 1 {
		t.Fatalf("charge=%s want 1 (single end-of-call rounding)", charge)
	}
}

func TestUsageChargeUnknownVersionDimensionIsError(t *testing.T) {
	rates := PriceVersionRates{Version: "pv-1", Rates: map[string]DimensionRate{
		DimensionModel: {RateMicro: 1500, Units: 1000},
	}}
	fact := usageTestFact()
	fact.Dimensions = map[string]int64{DimensionSandbox: 1}
	if _, err := rates.ChargeForCall(fact); !errors.Is(err, ErrUnknownDimensionRate) {
		t.Fatalf("unrated dimension accepted: %v", err)
	}
}

func usageTestFact() UsageFact {
	return UsageFact{
		TenantID:     7,
		RunID:        "run_1",
		DelegationID: "deleg_1",
		CallID:       "call_1",
		AttemptID:    "attempt_1",
		Funding:      "platform",
		Service:      "chat",
		PriceVersion: "pv-1",
		Revision:     1,
		OccurredAt:   time.Now(),
		Dimensions:   map[string]int64{DimensionModel: 1, DimensionSandbox: 2, DimensionParsing: 3},
		Status:       UsageStatusFinal,
	}
}
