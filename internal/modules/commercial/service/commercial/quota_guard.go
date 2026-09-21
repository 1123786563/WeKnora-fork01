package commercial

import (
	"context"
	"errors"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

// QuotaGuard is the production ResourceQuotaGuard: the conditional-UPDATE
// counter CAS behind the interface (the ONE place the counter CAS SQL
// lives — BenefitsStore.ReserveCounterDelta). Two concurrent adds at the
// boundary admit exactly the allowed number; a negative delta clamps at
// the zero floor (removal always passes); an unprojected dimension
// (hard_limit NULL) is unlimited — the fail-open degraded posture.
type QuotaGuard struct {
	store *repocommercial.BenefitsStore
}

// NewQuotaGuard builds the guard over the database and bootstraps the
// counter table with portable DDL (the NewCommercialHandler precedent —
// valid next to the versioned migrations, safe when the table exists).
func NewQuotaGuard(db *gorm.DB) *QuotaGuard {
	if db != nil {
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS commercial_resource_counters (
			tenant_id BIGINT NOT NULL,
			resource TEXT NOT NULL,
			used BIGINT NOT NULL DEFAULT 0,
			hard_limit BIGINT NULL,
			PRIMARY KEY (tenant_id, resource)
		)`).Error
	}
	return &QuotaGuard{store: repocommercial.NewBenefitsStore(db)}
}

// ReserveGrowth implements commercial.ResourceQuotaGuard. The returned
// release runs OUTSIDE the caller's (possibly cancelled) context so the
// compensating decrement always lands.
func (g *QuotaGuard) ReserveGrowth(ctx context.Context, tenantID uint64, dimension string, delta int64) (func(), error) {
	if g == nil || g.store == nil {
		return func() {}, nil // not wired = fail open
	}
	err := g.store.ReserveCounterDelta(ctx, tenantID, dimension, delta)
	switch {
	case err == nil:
		if delta <= 0 {
			return func() {}, nil // decrements carry no compensation
		}
		release := func() {
			// Compensating decrement: best-effort, bounded context.
			rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = g.store.ReserveCounterDelta(rctx, tenantID, dimension, -delta)
		}
		return release, nil
	case errors.Is(err, repocommercial.ErrCounterLimitReached):
		if delta <= 0 {
			// Zero-floor clamp on cleanup: the counter is already honest at
			// zero — removal always passes.
			return func() {}, nil
		}
		return nil, domain.ErrQuotaGrowthRefused
	default:
		return nil, err
	}
}

var _ domain.ResourceQuotaGuard = (*QuotaGuard)(nil)
