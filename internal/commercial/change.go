package commercial

import (
	"encoding/json"
	"time"
)

// ChangeKind names the product-level plan-change outcome. The wire values
// are stable API vocabulary; the type keeps callers from passing an
// arbitrary string where a defined change kind is required.
type ChangeKind string

const (
	// ChangeKindUpgrade settles immediately once paid: the subscription
	// switches plan and the current month is topped up with the prorated
	// monthly-credit delta (design 6.2).
	ChangeKindUpgrade ChangeKind = "upgrade"
	// ChangeKindScheduledSwitch takes effect at the end of the paid
	// interval: the paid term is never cut short and the arrangement is
	// applied by the lifecycle tick at its effective time.
	ChangeKindScheduledSwitch ChangeKind = "scheduled_switch"
)

// String keeps the raw wire value available where projections or logs need
// the plain string.
func (k ChangeKind) String() string { return string(k) }

// ScheduledPlanChange is the durable record of a plan switch that takes
// effect at EffectiveAt (the paid_until the change was cut against). It is
// serialized at the repository boundary — callers never hand-roll the JSON
// and the lifecycle tick never guesses the shape.
type ScheduledPlanChange struct {
	PlanKey     string `json:"plan_key"`
	PlanVersion int64  `json:"plan_version"`
	// PlanSnapshotJSON carries the immutable published definition the
	// switch will apply, so the lifecycle tick never re-resolves (or
	// races) the catalog at effective time.
	PlanSnapshotJSON string    `json:"plan_snapshot"`
	EffectiveAt      time.Time `json:"effective_at"`
	QuoteID          string    `json:"quote_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// JSON renders the record for storage; the single serialization point keeps
// every reader and writer on one shape.
func (c ScheduledPlanChange) JSON() (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ParseScheduledPlanChange is the single deserialization point. ok=false
// covers empty, malformed and semantically invalid payloads — the tick
// treats an unparseable record as absent rather than guessing.
func ParseScheduledPlanChange(raw string) (ScheduledPlanChange, bool) {
	if raw == "" || raw == "{}" {
		return ScheduledPlanChange{}, false
	}
	var c ScheduledPlanChange
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return ScheduledPlanChange{}, false
	}
	if c.PlanKey == "" || c.PlanVersion <= 0 || c.PlanSnapshotJSON == "" || c.EffectiveAt.IsZero() {
		return ScheduledPlanChange{}, false
	}
	return c, true
}

// Due reports whether the change is effective at now.
func (c ScheduledPlanChange) Due(now time.Time) bool {
	return !now.Before(c.EffectiveAt)
}
