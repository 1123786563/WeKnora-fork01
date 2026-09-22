package commercial

import (
	"context"
	"errors"
)

// ResourceQuotaGuard is the commercial growth gate (#80, Lago T08). nil
// (not wired) means no commercial limits exist — the call passes
// (blocked-env posture; older deployments degrade to open, never to
// lockout). A guard answers ErrQuotaGrowthRefused-class errors ONLY for
// growth that a projected hard_limit refuses; delta <= 0 always passes
// (超限状态 cleanup: view/export/clean stay open by construction — read
// paths never consult the gate at all).
type ResourceQuotaGuard interface {
	// ReserveGrowth atomically admits `delta` units of `dimension` for the
	// tenant. On success it returns a release func that compensates the
	// reservation when the caller's write ultimately fails; on refusal it
	// returns ErrQuotaGrowthRefused.
	ReserveGrowth(ctx context.Context, tenantID uint64, dimension string, delta int64) (release func(), err error)
}

// ErrQuotaGrowthRefused is the closed refusal sentinel of the growth gate:
// the projected hard limit (or the zero floor) refuses this delta. It is
// provider-neutral product vocabulary — no provider text ever wraps it.
var ErrQuotaGrowthRefused = errors.New("quota_growth_refused")
