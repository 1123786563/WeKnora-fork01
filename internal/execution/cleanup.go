// Package execution contains lifecycle invariants shared by platform and
// remote execution cleanup workers.
package execution

// CleanupFacts is the evidence required before destructive cleanup. A false
// value is deliberately fail-closed: an unknown or unobserved outcome is not
// equivalent to a successful stop or settlement.
type CleanupFacts struct {
	Stopped          bool
	Settled          bool
	RetentionElapsed bool
}

// CanPurge reports whether all irreversible cleanup preconditions hold.
func CanPurge(f CleanupFacts) bool {
	return f.Stopped && f.Settled && f.RetentionElapsed
}
