package commercial

import (
	"errors"
	"fmt"
)

// Plan lifecycle states. Publishing transitions draft→publishing→published;
// a published definition is immutable and may only move to archived.
const (
	PlanStateDraft      = "draft"
	PlanStatePublishing = "publishing"
	PlanStatePublished  = "published"
	PlanStateArchived   = "archived"
)

var (
	ErrInvalidPlanState = errors.New("invalid_plan_state")
	ErrUnknownPrice     = errors.New("unknown_price")
	ErrMissingBaseTier  = errors.New("missing_base_tier")
	ErrInvalidPlanLimit = errors.New("invalid_plan_limit")
)

// Six publish-validation axis errors (closed tokens; T07). Each failed axis
// carries exactly one of these; ValidateForPublishValidated joins them so a
// caller can itemize every failure with errors.Is.
var (
	// ErrPublishBasePriceTier: the tier is not on the ladder, the price is
	// not a ladder price point, or the price would invert the published
	// tier ordering.
	ErrPublishBasePriceTier = errors.New("base_price_tier")
	// ErrPublishCurrencyCNY: the currency is not the closed token CNY (an
	// empty currency from a pre-T07 definition is NOT silently defaulted).
	ErrPublishCurrencyCNY = errors.New("currency_cny")
	// ErrPublishEntitlements: an entitlement key outside the closed
	// first-slice feature set.
	ErrPublishEntitlements = errors.New("entitlements")
	// ErrPublishResourceQuota: a quota dimension outside the closed set or a
	// negative value (zero is a valid hard zero; absent means unlimited).
	ErrPublishResourceQuota = errors.New("resource_quota")
	// ErrPublishIncludedCredits: non-positive monthly included credits.
	ErrPublishIncludedCredits = errors.New("included_credits")
	// ErrPublishCostUpperBound: a charge outside the computable-upper-bound
	// model set (fixed_unit/package) or with non-explicit fen bounds.
	ErrPublishCostUpperBound = errors.New("cost_upper_bound")
)

// PublishedFeatureKeys is the closed first-slice entitlement key set
// (CONTEXT.md 功能权益). Unknown keys fail publish validation closed; the
// set is a first-slice constant a later ticket may externalize.
var PublishedFeatureKeys = []string{
	"api_access",        // programmatic API usage
	"advanced_models",   // frontier model access
	"priority_support",  // priority support channel
}

// QuotaDimensionKeys is the closed first-slice resource-quota dimension set
// (CONTEXT.md 资源配额: 成员数、存储容量、并发数). Unknown dimensions fail
// publish validation closed.
var QuotaDimensionKeys = []string{
	"members",          // 成员数
	"storage_gb",       // 存储容量
	"concurrent_tasks", // 并发数
}

// PlanVersion is an immutable versioned plan definition. Versions are only
// appended; a published definition can never be edited in place.
type PlanVersion struct {
	Key      string
	Version  int64
	Price    CNYFen
	Monthly  Credits
	Features map[string]bool
	Limits   map[string]int64
	// Currency and Charges are ADDITIVE T07 fields (Lago billing migration,
	// ticket #79): pre-T07 definition_json decodes with zero values, and the
	// empty currency is rejected by ValidateForPublishValidated instead of
	// being silently defaulted.
	Currency string
	Charges  []PlanCharge
}

// Limit returns the configured limit for key. ok=false means the key is
// absent and therefore unlimited; a configured 0 is a hard zero and a
// distinct, stricter statement than leaving the limit out.
func (p PlanVersion) Limit(key string) (limit int64, ok bool) {
	limit, ok = p.Limits[key]
	return limit, ok
}

// ValidatePlanState accepts only the four lifecycle states.
func ValidatePlanState(state string) error {
	switch state {
	case PlanStateDraft, PlanStatePublishing, PlanStatePublished, PlanStateArchived:
		return nil
	default:
		return ErrInvalidPlanState
	}
}

// ValidateForPublish enforces the publish invariants: the base tier identity
// and its monthly credit grant must be configured, the price must be a known
// positive price point, and limits must be non-negative. Zero-versus-absent
// limits are both valid and remain distinct.
//
// NOTE (T07): this is the LEGACY local publish path. The Lago-era publish
// flow runs the six acceptance axes through ValidateForPublishValidated.
func (p PlanVersion) ValidateForPublish() error {
	if p.Key == "" || p.Version <= 0 || p.Monthly <= 0 {
		return ErrMissingBaseTier
	}
	if p.Price <= 0 {
		return ErrUnknownPrice
	}
	for _, v := range p.Limits {
		if v < 0 {
			return ErrInvalidPlanLimit
		}
	}
	return nil
}

// TierPrice is one rung of the tier price ladder.
type TierPrice struct {
	TierKey  string
	PriceFen int64
}

// PriceLadder is the closed, ascending tier order of the first slice:
// strictly ascending prices over unique tier keys (spec: base price
// monotonic with tier).
type PriceLadder []TierPrice

// defaultPriceLadder is the first-slice tier/price constant (values fixed
// alongside the Task 1 tests; a later ticket may externalize them —
// validation is already injection-based).
var defaultPriceLadder = PriceLadder{
	{TierKey: "lite", PriceFen: 1900},
	{TierKey: "pro", PriceFen: 9900},
	{TierKey: "pro-max", PriceFen: 29900},
	{TierKey: "enterprise", PriceFen: 99900},
}

// DefaultPriceLadder returns a copy of the first-slice ladder.
func DefaultPriceLadder() PriceLadder {
	return append(PriceLadder(nil), defaultPriceLadder...)
}

// Validate enforces the ladder invariant: non-empty, unique tier keys and
// strictly ascending prices.
func (l PriceLadder) Validate() error {
	if len(l) == 0 {
		return fmt.Errorf("%w: empty tier ladder", ErrPublishBasePriceTier)
	}
	seen := make(map[string]bool, len(l))
	for i, t := range l {
		if t.TierKey == "" || t.PriceFen <= 0 {
			return fmt.Errorf("%w: rung %d is malformed", ErrPublishBasePriceTier, i)
		}
		if seen[t.TierKey] {
			return fmt.Errorf("%w: duplicate tier key %q", ErrPublishBasePriceTier, t.TierKey)
		}
		seen[t.TierKey] = true
		if i > 0 && l[i-1].PriceFen >= t.PriceFen {
			return fmt.Errorf("%w: prices must be strictly ascending (%q at %d)", ErrPublishBasePriceTier, t.TierKey, t.PriceFen)
		}
	}
	return nil
}

// PublishValidationContext feeds the cross-version tier rules: the closed
// ladder and the prices of the CURRENTLY published version of every other
// tier (tier key -> price_fen).
type PublishValidationContext struct {
	Ladder          PriceLadder
	PublishedPrices map[string]int64
}

// ValidateForPublishValidated enforces the SIX acceptance axes (T07, ticket
// #79) and returns an itemized, closed-token error per failed axis
// (errors.Is answers each):
//
//	base_price_tier  — ladder membership + no inversion vs PublishedPrices neighbors
//	currency_cny     — the closed CNY token, no silent default
//	entitlements     — closed first-slice feature keys
//	resource_quota   — closed quota dimensions, non-negative values
//	included_credits — positive monthly credits
//	cost_upper_bound — computable-upper-bound charge whitelist
//
// A failed axis never publishes: the caller keeps the version in draft.
func (p PlanVersion) ValidateForPublishValidated(ctx PublishValidationContext) error {
	var failed []error
	if err := p.validateBasePriceTier(ctx); err != nil {
		failed = append(failed, err)
	}
	if p.Currency != CurrencyCNY {
		failed = append(failed, ErrPublishCurrencyCNY)
	}
	if err := validateEntitlementKeys(p.Features); err != nil {
		failed = append(failed, err)
	}
	if err := validateQuotaLimits(p.Limits); err != nil {
		failed = append(failed, err)
	}
	if p.Monthly <= 0 {
		failed = append(failed, ErrPublishIncludedCredits)
	}
	if err := validateChargesUpperBound(p.Charges); err != nil {
		failed = append(failed, err)
	}
	return errors.Join(failed...)
}

// validateBasePriceTier enforces: the tier is on the ladder, the price is
// one of the ladder's price points, and publishing this price would not
// invert the ordering against any currently published neighbor tier. A
// price change for the version's OWN tier is the legitimate republish case
// and never counts as a neighbor.
func (p PlanVersion) validateBasePriceTier(ctx PublishValidationContext) error {
	ladder := ctx.Ladder
	idx := -1
	for i, t := range ladder {
		if t.TierKey == p.Key {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("%w: tier %q is not on the price ladder", ErrPublishBasePriceTier, p.Key)
	}
	price := int64(p.Price)
	onLadder := false
	for _, t := range ladder {
		if t.PriceFen == price {
			onLadder = true
			break
		}
	}
	if !onLadder || price <= 0 {
		return fmt.Errorf("%w: price %d is not a ladder price point", ErrPublishBasePriceTier, price)
	}
	for neighborKey, neighborPrice := range ctx.PublishedPrices {
		if neighborKey == p.Key {
			continue
		}
		neighborIdx := -1
		for i, t := range ladder {
			if t.TierKey == neighborKey {
				neighborIdx = i
				break
			}
		}
		if neighborIdx < 0 {
			// A published tier unknown to the ladder cannot be ordered:
			// fail closed instead of guessing monotonicity.
			return fmt.Errorf("%w: published tier %q is not on the price ladder", ErrPublishBasePriceTier, neighborKey)
		}
		if neighborIdx > idx && price >= neighborPrice {
			return fmt.Errorf("%w: price %d would invert tier order vs published %q at %d",
				ErrPublishBasePriceTier, price, neighborKey, neighborPrice)
		}
		if neighborIdx < idx && price <= neighborPrice {
			return fmt.Errorf("%w: price %d would invert tier order vs published %q at %d",
				ErrPublishBasePriceTier, price, neighborKey, neighborPrice)
		}
	}
	return nil
}

func validateEntitlementKeys(features map[string]bool) error {
	for key := range features {
		known := false
		for _, k := range PublishedFeatureKeys {
			if k == key {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("%w: entitlement key %q is outside the closed feature set", ErrPublishEntitlements, key)
		}
	}
	return nil
}

func validateQuotaLimits(limits map[string]int64) error {
	for key, value := range limits {
		known := false
		for _, k := range QuotaDimensionKeys {
			if k == key {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("%w: quota dimension %q is outside the closed dimension set", ErrPublishResourceQuota, key)
		}
		if value < 0 {
			return fmt.Errorf("%w: quota %q must be non-negative", ErrPublishResourceQuota, key)
		}
	}
	return nil
}

func validateChargesUpperBound(charges []PlanCharge) error {
	for i, c := range charges {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("%w: charge %d: %v", ErrPublishCostUpperBound, i, err)
		}
	}
	return nil
}
