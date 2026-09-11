package commercial

import (
	"context"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

var (
	// ErrBudgetDatabaseMissing rejects wiring without durable storage.
	ErrBudgetDatabaseMissing = errors.New("budget_database_missing")
	// ErrBudgetUnauthorized pauses a budget operation whose tenant lost
	// the plan/space entitlement (for example a space downgrade): the
	// wrapped reason is explicit and the caller must pause, never
	// silently continue on an unauthorized budget.
	ErrBudgetUnauthorized = errors.New("budget_unauthorized")
	// ErrReconcileNeedsConfirmation reports a reservation whose external
	// outcome is not yet confirmed: it is retained (never zeroed) until a
	// provider query confirms whether external usage occurred.
	ErrReconcileNeedsConfirmation = errors.New("reconcile_needs_confirmation")
	// ErrReservationUnknown rejects reconciling an unknown reservation.
	ErrReservationUnknown = errors.New("reservation_unknown")
)

// EntitlementCheck reports whether tenantID still holds the plan/space
// entitlement its budget depends on (F02 access facts). A nil check
// allows everything; a downgrade returns an error carrying the reason.
type EntitlementCheck func(ctx context.Context, tenantID uint64) error

// BudgetService applies the U04 policy rules on top of the budget
// store primitives: extension authorization with idempotency, and
// cancel/expiry reconciliation that releases only query-confirmed or
// provably-unstarted holds. A space downgrade blocks NEW steps
// (Extend) only: existing reconciliations always proceed.
type BudgetService struct {
	db       *gorm.DB
	store    *repocommercial.BudgetStore
	entitled EntitlementCheck
	now      func() time.Time
}

func NewBudgetService(db *gorm.DB, store *repocommercial.BudgetStore, entitled EntitlementCheck) (*BudgetService, error) {
	if db == nil {
		return nil, ErrBudgetDatabaseMissing
	}
	if store == nil {
		store = repocommercial.NewBudgetStore(db)
	}
	if err := db.AutoMigrate(&repocommercial.TaskBudgetExtensionRow{}); err != nil {
		return nil, err
	}
	return &BudgetService{db: db, store: store, entitled: entitled, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Extend raises the task budget limit of one run by extra credits.
// The entitlement check runs FIRST and its failure pauses the operation
// with an explicit reason (a space downgrade blocks new steps); the
// store transaction then verifies current funded authorization and
// applies the increase exactly once per idempotency key. Expired quota
// never extends validity: the deadline is untouched and an expired task
// budget refuses.
func (s *BudgetService) Extend(ctx context.Context, tenant uint64, runID, key string, extra domain.Credits) error {
	if s.entitled != nil {
		if err := s.entitled(ctx, tenant); err != nil {
			return fmt.Errorf("%w: %v", ErrBudgetUnauthorized, err)
		}
	}
	return s.store.ExtendTaskLimit(ctx, tenant, runID, key, extra)
}

// Reconcile resolves one reservation after cancel or expiry. Rules:
//   - late usage reports are matched against the reservation ACTUAL
//     occurrence interval [created, deadline]; only reports inside it
//     belong to this reservation;
//   - any in-interval settlement not yet confirmed means external usage
//     is possible: the reservation and its protection are retained
//     (ErrReconcileNeedsConfirmation), never zeroed;
//   - a reservation whose state is dispatched or settling is retained
//     the same way: query first, release only when confirmed;
//   - only an unstarted (held) reservation with no unconfirmed
//     in-interval settlement releases directly.
//
// A space downgrade never blocks reconciliation: only new steps pause.
func (s *BudgetService) Reconcile(ctx context.Context, reservationID string) error {
	var res repocommercial.ReservationRow
	err := s.db.WithContext(ctx).Where("key = ?", reservationID).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReservationUnknown
	}
	if err != nil {
		return err
	}
	var recs []SettlementRecord
	if err := s.db.WithContext(ctx).
		Where("reservation_key = ? AND occurred_at >= ? AND occurred_at <= ?", reservationID, res.CreatedAt, res.Deadline).
		Find(&recs).Error; err != nil {
		return err
	}
	for _, r := range recs {
		if r.State != domain.SettlementStateConfirmed {
			return fmt.Errorf("%w: settlement %s in state %s", ErrReconcileNeedsConfirmation, r.Key, r.State)
		}
	}
	if !domain.MayReleaseWithoutQuery(res.State) {
		if res.State == domain.ReservationStateReleased || res.State == domain.ReservationStateSettled {
			return nil // already terminal: nothing left to release here
		}
		return fmt.Errorf("%w: reservation state %s", ErrReconcileNeedsConfirmation, res.State)
	}
	return s.store.ReleaseReservation(ctx, res.TenantID, reservationID)
}
