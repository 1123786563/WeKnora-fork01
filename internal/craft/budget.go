package craft

import (
	"context"
	"errors"
	"time"
)

// Craft budget admission vocabulary. This file is a CRAFT-INTERNAL port: it is
// NOT an OpenMeter API and NOT a balance ledger. Money and Credits conversion
// is owned entirely by the commercial implementation behind the port; the
// craft model carries no balance fields — BudgetGrant counts CALLS against a
// deadline, and every money-bearing decision (funds, holds, settlement)
// happens inside the commercial interfaces the adapter maps to.
var (
	// ErrBudgetDenied reports a new charging operation or call refused by the
	// commercial budget authority: insufficient funds, a missing/unfunded
	// budget account, an exhausted task budget or a failed verification
	// window. It is a hard stop for NEW work — never a silent fallback to an
	// ungated path.
	ErrBudgetDenied = errors.New("craft budget denied")
	// ErrGrantExpired reports a call against a grant whose deadline has
	// passed. Expired quota never regains validity; admission of a new run
	// needs a fresh grant.
	ErrGrantExpired = errors.New("craft budget grant expired")
	// ErrGrantExhausted reports a call beyond the grant's call cap.
	ErrGrantExhausted = errors.New("craft budget grant exhausted")
	// ErrGrantRevoked reports a call against a cancelled run's grant. After a
	// revoke, NEW calls are forbidden; already-authorized calls may finish and
	// settle inside their deadline, and outcomes that stayed unknown keep
	// their holds until reconciliation confirms them.
	ErrGrantRevoked = errors.New("craft budget grant revoked")
	// ErrReconcilePending reports a reconciliation that found reservations
	// whose external outcome is not confirmed yet: their holds (and the spend
	// they protect) are retained, never released early.
	ErrReconcilePending = errors.New("craft budget reconcile pending confirmation")
)

// BudgetGrant is the admission verdict for ONE run's chargeable model calls.
// ID is the opaque durable identity AuthorizeCall and Reconcile address;
// Deadline bounds when authorized calls may still be forwarded; MaxCalls caps
// the number of authorized calls (a quick craft-side fence — the strict
// concurrent balance control lives in the commercial reservation transaction
// behind the port); UsedCalls counts already-authorized calls; Allowed is
// false once the grant was revoked (a revoked grant never resurrects).
type BudgetGrant struct {
	ID        string
	Deadline  time.Time
	MaxCalls  int
	UsedCalls int
	Allowed   bool
}

// BudgetAllows is the FAST pre-check of a grant: identity present, admission
// still allowed, deadline not reached and call headroom left. It is only a
// quick check — the authoritative concurrent decision (funds, racing last
// quotas, revocation durability) is made by the commercial interface inside
// BudgetPort.AuthorizeCall, which must be consulted before every real
// outbound model call.
func BudgetAllows(g BudgetGrant, now time.Time) bool {
	return g.Allowed && g.ID != "" && now.Before(g.Deadline) && g.UsedCalls < g.MaxCalls
}

// BudgetPort is the craft-internal admission boundary every chargeable model
// call of a run must pass through. Implementations MUST make AuthorizeCall
// atomic across workers and processes: the same callID retried never admits
// twice, two racing calls for the last quota yield exactly one winner, and a
// denial blocks the real forward. Reads, downloads and cleanup are NOT gated
// here — they follow the commercial over-limit semantics of their own
// surfaces instead of a blanket space lock.
type BudgetPort interface {
	// Admit admits one run of one scope to chargeable model calls and
	// registers its commercial task budget. It is idempotent per
	// (tenant, run): re-admitting returns the same durable grant, a revoked
	// grant stays revoked, and an expired grant never regains validity.
	Admit(ctx context.Context, scope Scope, runID string) (BudgetGrant, error)
	// AuthorizeCall atomically reserves ONE logical model call (and the
	// commercial budget behind it) under the grant, BEFORE the real forward.
	// Retrying the SAME callID is idempotent — it never reserves twice; a
	// different callID is a new call with its own new reservation and spend.
	// Insufficient funds, a revoked grant or a passed deadline reject with
	// ErrBudgetDenied / ErrGrantRevoked / ErrGrantExpired and the caller MUST
	// NOT forward.
	AuthorizeCall(ctx context.Context, grantID, callID string) error
	// Reconcile resolves the grant's reservations after cancel, expiry or
	// completion. Reservations whose outcome is confirmed release; outcomes
	// that stayed unknown RETAIN their holds (ErrReconcilePending) until a
	// later confirmation — spend protection is never released early.
	Reconcile(ctx context.Context, grantID string) error
}
