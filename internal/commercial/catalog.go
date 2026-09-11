package commercial

import "errors"

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

// PlanVersion is an immutable versioned plan definition. Versions are only
// appended; a published definition can never be edited in place.
type PlanVersion struct {
	Key      string
	Version  int64
	Price    CNYFen
	Monthly  Credits
	Features map[string]bool
	Limits   map[string]int64
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
