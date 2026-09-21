package commercial

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// This file grows the frozen Commercial Platform seam (internal/commercial/
// platform.go) ADDITIVELY per ADR-0014 and the T05 Task 2 freeze: exactly
// one new CommandKind constant (publish_plan_version) plus its typed
// provider-neutral payload live here — the frozen file itself is never
// edited and the three method signatures never change. Per-object wrappers
// (GetPlan/CreatePlan style) are forbidden; publishing a plan version is a
// COMMAND, not a getter.
//
// Spec: docs/specs/2026-09-20-lago-billing-migration-design.md —
// "Catalog, subscriptions, and entitlements" (every published Plan Version
// a distinct external plan code; published version immutable; base price
// monotonic with tier), "Commercial authority and module seam" (CNY-only
// single Billing Entity, integer minor units), "Credits and Task
// admission" (the computable-upper-bound charge-model restriction).

// CommandKindPublishPlanVersion publishes one immutable plan version to the
// commercial billing authority. The coordinator (never the provider) owns
// the idempotency identity: Key = PublishCommandKey(plan_key, version).
const CommandKindPublishPlanVersion CommandKind = "publish_plan_version"

// Closed first-slice charge models (spec "Credits and Task admission": a
// task is only admitted when its worst-case cost is computable — graduated,
// percentage, minimum-commitment and commitment-spend models are out of
// scope and rejected fail-closed by validation).
const (
	// ChargeModelFixedUnit prices each unit at a fixed fen amount.
	ChargeModelFixedUnit = "fixed_unit"
	// ChargeModelPackage prices each package of PackageUnits units at a
	// fixed fen amount.
	ChargeModelPackage = "package"
)

// CurrencyCNY is the closed currency token of the first slice (spec:
// CNY-only single Billing Entity).
const CurrencyCNY = "CNY"

// IntervalMonthly is the closed billing interval token of the first slice.
const IntervalMonthly = "monthly"

// PlanCharge is one usage-pricing component of a plan version. AmountFen is
// integer fen per unit (fixed_unit) or per package (package). Optional in
// T07 — a plan version without charges prices only its base fee.
type PlanCharge struct {
	// Dimension is the product service-dimension code; the adapter resolves
	// it to the provider metric at submit time.
	Dimension string
	// Model is ChargeModelFixedUnit or ChargeModelPackage (closed set).
	Model string
	// AmountFen is the explicit integer fen price (> 0).
	AmountFen int64
	// PackageUnits is the units per package (package model only, > 0).
	PackageUnits int64
	// FreeUnits is the free allowance per period (>= 0).
	FreeUnits int64
}

// Validate enforces the computable-upper-bound bounds on one charge.
func (c PlanCharge) Validate() error {
	if c.Dimension == "" {
		return errors.New("invalid plan charge: dimension is required")
	}
	switch c.Model {
	case ChargeModelFixedUnit, ChargeModelPackage:
	default:
		return fmt.Errorf("invalid plan charge: model %q is outside the closed set fixed_unit/package", c.Model)
	}
	if c.AmountFen <= 0 {
		return errors.New("invalid plan charge: amount_fen must be a positive integer")
	}
	if c.FreeUnits < 0 {
		return errors.New("invalid plan charge: free_units must be >= 0")
	}
	if c.Model == ChargeModelPackage && c.PackageUnits <= 0 {
		return errors.New("invalid plan charge: package model requires package_units > 0")
	}
	return nil
}

// PublishPlanVersionPayload is the typed W3 payload for the frozen seam —
// provider-neutral WeKnora product vocabulary only. Money is integer fen
// end-to-end; the adapter converts with FenToDecimalString, never binary
// float.
type PublishPlanVersionPayload struct {
	PlanKey string
	Version int64
	// PlanCode is the deterministic weknora-<slug>-v<n> external identity;
	// the adapter echoes it back as the receipt ExternalID. It lives inside
	// the seam and never crosses the admin API.
	PlanCode             string
	Name                 string
	Interval             string // closed token "monthly" in T07
	AmountFen            int64  // base subscription price, CNY fen, >= 0 (zero only for the base rung — the six-axis check decides)
	Currency             string // closed token "CNY"
	PayInAdvance         bool   // always true in T07 (initial subscriptions pay in advance)
	IncludedCreditsMicro int64  // monthly included credits, > 0
	Features             map[string]bool
	Limits               map[string]int64 // resource quotas (absent = unlimited, 0 = hard zero)
	Charges              []PlanCharge
}

// planKeyPattern is the slug form of a plan key: lowercase letters, digits
// and hyphens — the same alphabet DeterministicPlanCode embeds.
var planKeyPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// Validate enforces the structural contract: closed tokens, slug form,
// fen/unit bounds and the charge-model whitelist. The six business axes
// (tier ladder, entitlement keys, quota dimensions) live in
// PlanVersion.ValidateForPublishValidated.
func (p PublishPlanVersionPayload) Validate() error {
	if p.PlanKey == "" {
		return errors.New("invalid publish payload: plan_key is required")
	}
	if !planKeyPattern.MatchString(p.PlanKey) {
		return fmt.Errorf("invalid publish payload: plan_key %q must match [a-z0-9-]+", p.PlanKey)
	}
	if p.Version <= 0 {
		return errors.New("invalid publish payload: version must be positive")
	}
	if p.PlanCode != DeterministicPlanCode(p.PlanKey, p.Version) {
		return errors.New("invalid publish payload: plan_code must equal DeterministicPlanCode(plan_key, version)")
	}
	if p.Name == "" {
		return errors.New("invalid publish payload: name is required")
	}
	if p.Interval != IntervalMonthly {
		return fmt.Errorf("invalid publish payload: interval %q is outside the closed token monthly", p.Interval)
	}
	if p.AmountFen < 0 {
		return errors.New("invalid publish payload: amount_fen must not be negative")
	}
	if p.Currency != CurrencyCNY {
		return fmt.Errorf("invalid publish payload: currency %q is outside the closed token CNY", p.Currency)
	}
	if !p.PayInAdvance {
		return errors.New("invalid publish payload: first-slice plans are pay_in_advance")
	}
	if p.IncludedCreditsMicro <= 0 {
		return errors.New("invalid publish payload: included_credits_micro must be positive")
	}
	for i, c := range p.Charges {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("invalid publish payload: charge %d: %w", i, err)
		}
	}
	return nil
}

// DeterministicPlanCode derives the external plan code
// "weknora-<slug>-v<n>" purely from (plan_key, version): the same version
// always maps to the same code, so a replayed publish command addresses the
// same external object instead of minting a second one.
func DeterministicPlanCode(planKey string, version int64) string {
	return "weknora-" + planKey + "-v" + strconv.FormatInt(version, 10)
}

// PublishCommandKey derives the seam command idempotency identity
// "publish_plan_version:<plan_key>:<version>" — the coordinator (not the
// provider) owns publish identity.
func PublishCommandKey(planKey string, version int64) string {
	return string(CommandKindPublishPlanVersion) + ":" + planKey + ":" + strconv.FormatInt(version, 10)
}

// FenToDecimalString converts integer fen to Lago's decimal-string currency
// form ("yuan.fen", e.g. 7 -> "0.07") with exact integer→string formatting
// (the formatFixed precedent in amount.go). Monetary values never pass
// through binary floating point (spec: integer minor units; T04 lab ×100
// fact — Lago multiplies the decimal string by 100 itself).
func FenToDecimalString(fen int64) string {
	return formatFixed(fen, 2)
}
