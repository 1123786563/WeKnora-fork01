package commercial

import (
	"errors"
	"time"
)

// Reservation lifecycle states. A reservation is born held; settlement and
// release transitions are driven by U03 against the stored fence.
const ReservationStateHeld = "held"

// BudgetRequest asks the budget store to hold Upper credits for one keyed
// call of one run until Deadline. The store — never the caller — decides
// from the persisted projection whether the hold fits.
type BudgetRequest struct {
	TenantID uint64
	RunID    string
	Key      string
	Upper    Credits
	Deadline time.Time
}

// Reservation is the durable outcome of an accepted BudgetRequest. Fence is
// the token later settlement must present, so a stale worker that lost a
// lease cannot spend a hold a live worker re-acquired.
type Reservation struct {
	ID       string
	RunID    string
	State    string
	Upper    Credits
	Deadline time.Time
	Version  int64
}

// Available projects the spendable credits of a tenant from its balance
// projection: the verified balance reduced by unacknowledged spend, then by
// in-flight reservation holds, then by refund-locked credits. A negative
// component means the projection itself is broken and is an error; a fully
// consumed projection is an ordinary "nothing to spend" zero, not an error,
// so callers never mistake a healthy zero space-credits balance projection for a broken one.
func Available(verified, unreflected, held, refundLocked Credits) (Credits, error) {
	if verified < 0 || unreflected < 0 || held < 0 || refundLocked < 0 {
		return 0, errors.New("invalid_projection")
	}
	remaining := verified
	for _, n := range []Credits{unreflected, held, refundLocked} {
		if n > remaining {
			return 0, nil
		}
		remaining -= n
	}
	return remaining, nil
}
