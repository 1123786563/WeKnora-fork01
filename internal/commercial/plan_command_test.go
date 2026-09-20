package commercial

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// validPublishPayload is the canonical well-formed first-slice payload; every
// rejection case mutates exactly one field.
func validPublishPayload() PublishPlanVersionPayload {
	return PublishPlanVersionPayload{
		PlanKey:              "pro",
		Version:              1,
		PlanCode:             "weknora-pro-v1",
		Name:                 "Pro",
		Interval:             "monthly",
		AmountFen:            9900,
		Currency:             "CNY",
		PayInAdvance:         true,
		IncludedCreditsMicro: 9_900_000,
		Features:             map[string]bool{PublishedFeatureKeys[0]: true},
		Limits:               map[string]int64{QuotaDimensionKeys[0]: 10},
	}
}

// TestPayloadValidate is the structural table for the typed seam payload:
// closed tokens, slug form, fen/unit bounds and the charge-model whitelist.
func TestPayloadValidate(t *testing.T) {
	charge := func(model string) PlanCharge {
		return PlanCharge{Dimension: "model-units", Model: model, AmountFen: 7, PackageUnits: 100, FreeUnits: 0}
	}
	rejects := []struct {
		name   string
		mutate func(*PublishPlanVersionPayload)
	}{
		{"empty plan key", func(p *PublishPlanVersionPayload) { p.PlanKey = "" }},
		{"empty plan code", func(p *PublishPlanVersionPayload) { p.PlanCode = "" }},
		{"plan key uppercase", func(p *PublishPlanVersionPayload) { p.PlanKey = "Pro"; p.PlanCode = DeterministicPlanCode("Pro", 1) }},
		{"plan key underscore", func(p *PublishPlanVersionPayload) { p.PlanKey = "pro_max"; p.PlanCode = "weknora-pro_max-v1" }},
		{"plan key punctuation", func(p *PublishPlanVersionPayload) { p.PlanKey = "pro!"; p.PlanCode = "weknora-pro!-v1" }},
		{"version zero", func(p *PublishPlanVersionPayload) { p.Version = 0; p.PlanCode = "weknora-pro-v0" }},
		{"version negative", func(p *PublishPlanVersionPayload) { p.Version = -1; p.PlanCode = "weknora-pro-v--1" }},
		{"plan code not deterministic", func(p *PublishPlanVersionPayload) { p.PlanCode = "weknora-pro-v2" }},
		{"plan code foreign form", func(p *PublishPlanVersionPayload) { p.PlanCode = "pro-v1" }},
		{"empty name", func(p *PublishPlanVersionPayload) { p.Name = "" }},
		{"amount zero", func(p *PublishPlanVersionPayload) { p.AmountFen = 0 }},
		{"amount negative", func(p *PublishPlanVersionPayload) { p.AmountFen = -1 }},
		{"currency usd", func(p *PublishPlanVersionPayload) { p.Currency = "USD" }},
		{"currency empty", func(p *PublishPlanVersionPayload) { p.Currency = "" }},
		{"interval yearly", func(p *PublishPlanVersionPayload) { p.Interval = "yearly" }},
		{"interval empty", func(p *PublishPlanVersionPayload) { p.Interval = "" }},
		{"pay in arrears", func(p *PublishPlanVersionPayload) { p.PayInAdvance = false }},
		{"credits zero", func(p *PublishPlanVersionPayload) { p.IncludedCreditsMicro = 0 }},
		{"credits negative", func(p *PublishPlanVersionPayload) { p.IncludedCreditsMicro = -1 }},
		{"charge model graduated", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge("graduated")} }},
		{"charge model percentage", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge("percentage")} }},
		{"charge model minimum_commitment", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge("minimum_commitment")} }},
		{"charge model empty", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge("")} }},
		{"package charge zero package units", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelPackage)
			c.PackageUnits = 0
			p.Charges = []PlanCharge{c}
		}},
		{"package charge negative package units", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelPackage)
			c.PackageUnits = -5
			p.Charges = []PlanCharge{c}
		}},
		{"fixed_unit with package units only still needs amount", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelFixedUnit)
			c.AmountFen = 0
			p.Charges = []PlanCharge{c}
		}},
		{"charge free units negative", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelFixedUnit)
			c.FreeUnits = -1
			p.Charges = []PlanCharge{c}
		}},
		{"charge empty dimension", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelFixedUnit)
			c.Dimension = ""
			p.Charges = []PlanCharge{c}
		}},
	}
	for _, tc := range rejects {
		t.Run("reject "+tc.name, func(t *testing.T) {
			p := validPublishPayload()
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatalf("payload must be rejected (%s), got nil", tc.name)
			}
		})
	}

	accepts := []struct {
		name   string
		mutate func(*PublishPlanVersionPayload)
	}{
		{"fully formed without charges", func(p *PublishPlanVersionPayload) {}},
		{"charges nil", func(p *PublishPlanVersionPayload) { p.Charges = nil }},
		{"fixed_unit charge", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge(ChargeModelFixedUnit)} }},
		{"package charge", func(p *PublishPlanVersionPayload) { p.Charges = []PlanCharge{charge(ChargeModelPackage)} }},
		{"charge free units granted", func(p *PublishPlanVersionPayload) {
			c := charge(ChargeModelFixedUnit)
			c.FreeUnits = 1000
			p.Charges = []PlanCharge{c}
		}},
	}
	for _, tc := range accepts {
		t.Run("accept "+tc.name, func(t *testing.T) {
			p := validPublishPayload()
			tc.mutate(&p)
			if err := p.Validate(); err != nil {
				t.Fatalf("payload must be accepted (%s), got %v", tc.name, err)
			}
		})
	}
}

// TestDeterministicPlanCodeAndCommandKey: the plan code and command key are
// pure functions of (plan_key, version) — deterministic identity is what
// makes an idempotent replay hit the same external object.
func TestDeterministicPlanCodeAndCommandKey(t *testing.T) {
	if got := DeterministicPlanCode("pro", 3); got != "weknora-pro-v3" {
		t.Fatalf("DeterministicPlanCode(pro,3) = %q, want weknora-pro-v3", got)
	}
	if got := DeterministicPlanCode("pro-max", 12); got != "weknora-pro-max-v12" {
		t.Fatalf("DeterministicPlanCode(pro-max,12) = %q, want weknora-pro-max-v12", got)
	}
	// Purity: identical inputs, identical outputs.
	if DeterministicPlanCode("pro", 3) != DeterministicPlanCode("pro", 3) {
		t.Fatal("DeterministicPlanCode must be pure")
	}
	if got := PublishCommandKey("pro", 3); got != "publish_plan_version:pro:3" {
		t.Fatalf("PublishCommandKey(pro,3) = %q, want publish_plan_version:pro:3", got)
	}
	if got := PublishCommandKey("pro-max", 12); got != "publish_plan_version:pro-max:12" {
		t.Fatalf("PublishCommandKey(pro-max,12) = %q, want publish_plan_version:pro-max:12", got)
	}
}

// publishableVersion is the canonical six-axes-clean plan version.
func publishableVersion() PlanVersion {
	return PlanVersion{
		Key:      "pro",
		Version:  1,
		Price:    9900,
		Monthly:  9_900_000,
		Currency: "CNY",
		Features: map[string]bool{PublishedFeatureKeys[0]: true},
		Limits:   map[string]int64{QuotaDimensionKeys[0]: 10},
	}
}

// fenFromDecimalString parses a two-place decimal string back to fen with
// integer arithmetic only — the test-side of the exactness contract.
func fenFromDecimalString(t *testing.T, s string) int64 {
	t.Helper()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, found := strings.Cut(s, ".")
	if !found || len(frac) != 2 {
		t.Fatalf("decimal string %q must be whole + exactly two fraction places", s)
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		t.Fatalf("whole of %q: %v", s, err)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		t.Fatalf("fraction of %q: %v", s, err)
	}
	fen := w*100 + f
	if neg {
		fen = -fen
	}
	return fen
}

// TestValidateForPublishValidatedSixAxes: one case per acceptance axis and
// per escape; every failed axis carries its own closed-token error.
func TestValidateForPublishValidatedSixAxes(t *testing.T) {
	ctx := func(published map[string]int64) PublishValidationContext {
		return PublishValidationContext{Ladder: DefaultPriceLadder(), PublishedPrices: published}
	}
	rejects := []struct {
		name string
		axis error
		mut  func(*PlanVersion)
		pub  map[string]int64
	}{
		{"price off the ladder", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Price = 12345 }, nil},
		{"price below zero", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Price = -9900 }, nil},
		{"unknown tier key fails closed", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Key = "mystery"; p.Price = 9900 }, nil},
		{"inversion vs higher published tier", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Price = 99900 },
			map[string]int64{"pro-max": 29900}},
		{"inversion vs lower published tier", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Key = "pro-max"; p.Price = 1900 },
			map[string]int64{"pro": 9900}},
		{"equal price to published neighbor inverts", ErrPublishBasePriceTier,
			func(p *PlanVersion) { p.Price = 29900 },
			map[string]int64{"pro-max": 29900}},
		{"non-CNY currency", ErrPublishCurrencyCNY,
			func(p *PlanVersion) { p.Currency = "USD" }, nil},
		{"empty currency is not defaulted", ErrPublishCurrencyCNY,
			func(p *PlanVersion) { p.Currency = "" }, nil},
		{"unknown entitlement key", ErrPublishEntitlements,
			func(p *PlanVersion) { p.Features = map[string]bool{"teleport": true} }, nil},
		{"empty entitlement key", ErrPublishEntitlements,
			func(p *PlanVersion) { p.Features = map[string]bool{"": true} }, nil},
		{"unknown quota dimension", ErrPublishResourceQuota,
			func(p *PlanVersion) { p.Limits = map[string]int64{"ponies": 1} }, nil},
		{"negative quota value", ErrPublishResourceQuota,
			func(p *PlanVersion) { p.Limits = map[string]int64{QuotaDimensionKeys[0]: -1} }, nil},
		{"zero included credits", ErrPublishIncludedCredits,
			func(p *PlanVersion) { p.Monthly = 0 }, nil},
		{"negative included credits", ErrPublishIncludedCredits,
			func(p *PlanVersion) { p.Monthly = -1 }, nil},
		{"graduated charge model", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: "graduated", AmountFen: 7}} }, nil},
		{"percentage charge model", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: "percentage", AmountFen: 7}} }, nil},
		{"minimum_commitment charge model", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: "minimum_commitment", AmountFen: 7}} }, nil},
		{"charge without explicit fen amount", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: ChargeModelFixedUnit, AmountFen: 0}} }, nil},
		{"package charge without package units", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: ChargeModelPackage, AmountFen: 7, PackageUnits: 0}} }, nil},
		{"negative free units", ErrPublishCostUpperBound,
			func(p *PlanVersion) { p.Charges = []PlanCharge{{Dimension: "model-units", Model: ChargeModelFixedUnit, AmountFen: 7, FreeUnits: -1}} }, nil},
	}
	for _, tc := range rejects {
		t.Run("reject "+tc.name, func(t *testing.T) {
			v := publishableVersion()
			tc.mut(&v)
			err := v.ValidateForPublishValidated(ctx(tc.pub))
			if err == nil {
				t.Fatalf("%s must fail validation", tc.name)
			}
			if !errors.Is(err, tc.axis) {
				t.Fatalf("%s must carry axis error %v, got %v", tc.name, tc.axis, err)
			}
		})
	}

	accepts := []struct {
		name string
		mut  func(*PlanVersion)
		pub  map[string]int64
	}{
		{"clean version with no charges", func(p *PlanVersion) {}, nil},
		{"zero limit is a hard zero, valid", func(p *PlanVersion) { p.Limits[QuotaDimensionKeys[0]] = 0 }, nil},
		{"absent limit is unlimited, valid", func(p *PlanVersion) { p.Limits = nil }, nil},
		{"no entitlements is valid", func(p *PlanVersion) { p.Features = nil }, nil},
		{"accepted fixed_unit charge", func(p *PlanVersion) {
			p.Charges = []PlanCharge{{Dimension: "model-units", Model: ChargeModelFixedUnit, AmountFen: 7}}
		}, nil},
		{"accepted package charge", func(p *PlanVersion) {
			p.Charges = []PlanCharge{{Dimension: "tool-calls", Model: ChargeModelPackage, AmountFen: 1500, PackageUnits: 100, FreeUnits: 10}}
		}, nil},
		{"legitimate on-ladder price change", func(p *PlanVersion) { p.Price = 29900 },
			map[string]int64{"pro": 9900}},
		{"same price republish", func(p *PlanVersion) {},
			map[string]int64{"pro": 9900}},
	}
	for _, tc := range accepts {
		t.Run("accept "+tc.name, func(t *testing.T) {
			v := publishableVersion()
			tc.mut(&v)
			if err := v.ValidateForPublishValidated(ctx(tc.pub)); err != nil {
				t.Fatalf("%s must pass validation, got %v", tc.name, err)
			}
		})
	}

	t.Run("multiple failed axes are all itemized", func(t *testing.T) {
		v := publishableVersion()
		v.Currency = "USD"
		v.Monthly = 0
		err := v.ValidateForPublishValidated(ctx(nil))
		if err == nil {
			t.Fatal("expected failure")
		}
		for _, axis := range []error{ErrPublishCurrencyCNY, ErrPublishIncludedCredits} {
			if !errors.Is(err, axis) {
				t.Fatalf("joined error must itemize %v, got %v", axis, err)
			}
		}
	})

	t.Run("FenToDecimalString round-trips exactly", func(t *testing.T) {
		for _, fen := range []int64{0, 7, 99, 100, 9900, 12345, 29900, 99900, 9_999_999} {
			s := FenToDecimalString(fen)
			if back := fenFromDecimalString(t, s); back != fen {
				t.Fatalf("FenToDecimalString(%d) = %q round-trips to %d", fen, s, back)
			}
		}
		if got := FenToDecimalString(7); got != "0.07" {
			t.Fatalf("FenToDecimalString(7) = %q, want 0.07", got)
		}
	})
}

// TestPriceLadderValidate: the tier ladder invariant is ascending, unique
// tier keys; DefaultPriceLadder satisfies it.
func TestPriceLadderValidate(t *testing.T) {
	if err := DefaultPriceLadder().Validate(); err != nil {
		t.Fatalf("DefaultPriceLadder must be valid, got %v", err)
	}
	if err := (PriceLadder{}).Validate(); err == nil {
		t.Fatal("an empty ladder is not a usable ladder")
	}
	descending := PriceLadder{{TierKey: "pro", PriceFen: 29900}, {TierKey: "lite", PriceFen: 1900}}
	if err := descending.Validate(); err == nil {
		t.Fatal("descending ladder must be rejected")
	}
	duplicate := PriceLadder{{TierKey: "pro", PriceFen: 9900}, {TierKey: "pro", PriceFen: 29900}}
	if err := duplicate.Validate(); err == nil {
		t.Fatal("duplicate tier keys must be rejected")
	}
	equalNeighbors := PriceLadder{{TierKey: "lite", PriceFen: 9900}, {TierKey: "pro", PriceFen: 9900}}
	if err := equalNeighbors.Validate(); err == nil {
		t.Fatal("flat neighbors (equal prices) must be rejected — the ladder is strictly ascending")
	}
	// Mutating a returned copy must not corrupt the default.
	l := DefaultPriceLadder()
	l[0].PriceFen = 1
	if DefaultPriceLadder()[0].PriceFen == 1 {
		t.Fatal("DefaultPriceLadder must return a copy")
	}
}

// TestPlanVersionJSONRoundTripAddsFields: pre-T07 definition_json (no
// currency/charges) still decodes; the missing currency is NOT silently
// defaulted and fails the publish validation instead.
func TestPlanVersionJSONRoundTripAddsFields(t *testing.T) {
	preT07 := `{"Key":"pro","Version":1,"Price":9900,"Monthly":9900000,"Features":{"api_access":true},"Limits":{"members":10}}`
	var legacy PlanVersion
	if err := json.Unmarshal([]byte(preT07), &legacy); err != nil {
		t.Fatalf("pre-T07 definition must decode: %v", err)
	}
	if legacy.Currency != "" || legacy.Charges != nil {
		t.Fatalf("pre-T07 decode must leave the additive fields zero, got %+v", legacy)
	}
	if err := legacy.ValidateForPublishValidated(PublishValidationContext{Ladder: DefaultPriceLadder()}); !errors.Is(err, ErrPublishCurrencyCNY) {
		t.Fatalf("a legacy definition without currency must fail the CNY axis, got %v", err)
	}

	withCharges := PlanVersion{
		Key: "pro", Version: 2, Price: 9900, Monthly: 1, Currency: "CNY",
		Charges: []PlanCharge{{Dimension: "model-units", Model: ChargeModelFixedUnit, AmountFen: 7}},
	}
	blob, err := json.Marshal(withCharges)
	if err != nil {
		t.Fatal(err)
	}
	var back PlanVersion
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatal(err)
	}
	if back.Currency != "CNY" || len(back.Charges) != 1 || back.Charges[0] != withCharges.Charges[0] {
		t.Fatalf("charges must round-trip, got %+v", back)
	}
}
