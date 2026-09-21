package commercial

// Reservation lifecycle states beyond held/settled. dispatched marks a
// reservation whose work was handed to a worker or provider: external
// spend may already exist, so it may never be released on cancel or
// expiry without a provider query confirming no usage. settling marks a
// finalization in progress (late reports may still arrive). released is
// the terminal state of a hold returned without spend.
const (
	ReservationStateDispatched = "dispatched"
	ReservationStateSettling   = "settling"
	ReservationStateReleased   = "released"
)

// MayReleaseWithoutQuery reports whether a reservation in the given state
// may release its protected hold WITHOUT first querying the provider.
// Only an unstarted (held) reservation qualifies: dispatched and settling
// mean external spend may already have occurred, and any unrecognized
// state fails closed: an unknown answer is never a licence to zero spend.
func MayReleaseWithoutQuery(state string) bool { return state == ReservationStateHeld }
