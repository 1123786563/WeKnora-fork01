package commercial

import (
	"context"
	"errors"
	"fmt"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

// ErrGateReservationUnknown rejects Finish for a reservation the store has
// no record of.
var ErrGateReservationUnknown = errors.New("gate_reservation_unknown")

// DefaultExecutionLimits are the conservative caps applied when no explicit
// limits are configured. They are intentionally finite: an unbounded
// model-output or sandbox-duration dimension must never be the default.
var DefaultExecutionLimits = domain.ExecutionLimits{
	MaxModelOutputTokens: 32768,
	MaxSandboxSeconds:    3600,
}

// ExecutionGateService implements domain.ExecutionGate on top of the U02
// budget store and the U03 settlement service. Begin reserves and persists
// the dispatched intent BEFORE the real outbound call; Finish accepts only
// trusted final usage and settles it, flagging abnormal cost so callers stop
// further dispatch.
type ExecutionGateService struct {
	db      *gorm.DB
	budget  *repocommercial.BudgetStore
	settle  *SettlementService
	gateway domain.CommercialGateway
	limits  domain.ExecutionLimits
	rates   repocommercial.RateResolver
	stopped func(domain.Credits, domain.Credits) error
}

// NewExecutionGateService validates its wiring and builds the gate from the
// budget store and settlement dependencies. A nil rate resolver is legal for
// wiring (blocked-env): Finish then rejects unpriced final facts instead of
// zero-charging them — use WithRates once pricing administration exists.
func NewExecutionGateService(db *gorm.DB, gateway domain.CommercialGateway) (*ExecutionGateService, error) {
	if db == nil {
		return nil, ErrSettlementDatabaseMissing
	}
	if gateway == nil {
		return nil, ErrSettlementGatewayMissing
	}
	settle, err := NewSettlementService(db, repocommercial.NewBudgetStore(db), gateway, unavailableRates)
	if err != nil {
		return nil, err
	}
	return &ExecutionGateService{
		db:      db,
		budget:  repocommercial.NewBudgetStore(db),
		settle:  settle,
		gateway: gateway,
		limits:  DefaultExecutionLimits,
		rates:   unavailableRates,
		stopped: func(charged, upper domain.Credits) error {
			return domain.CheckAbnormal(charged, upper)
		},
	}, nil
}

func unavailableRates(string) (domain.PriceVersionRates, error) {
	return domain.PriceVersionRates{}, repocommercial.ErrUsageRatesUnavailable
}

// WithRates replaces the rate resolver used to price final facts.
func (s *ExecutionGateService) WithRates(r repocommercial.RateResolver) (*ExecutionGateService, error) {
	if r == nil {
		return s, nil
	}
	settle, err := NewSettlementService(s.db, s.budget, s.gateway, r)
	if err != nil {
		return s, err
	}
	s.rates = r
	s.settle = settle
	return s, nil
}

// WithLimits replaces the enforced upper bounds.
func (s *ExecutionGateService) WithLimits(l domain.ExecutionLimits) *ExecutionGateService {
	s.limits = l
	return s
}

// Begin holds req.Upper credits for the keyed call (U02 Reserve) and then
// persists the dispatched intent (MarkReservationDispatched) BEFORE the real
// outbound call happens. A denial from the store is a hard stop: it is
// wrapped so dispatchers classify it as insufficient budget and never fall
// back to an ungated path.
func (s *ExecutionGateService) Begin(ctx context.Context, req domain.BudgetRequest) (domain.Reservation, error) {
	res, err := s.budget.Reserve(ctx, req)
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("%w: %v", domain.ErrInsufficientBudgetGate, err)
	}
	if err := s.budget.MarkReservationDispatched(ctx, req.TenantID, res.ID); err != nil {
		return domain.Reservation{}, err
	}
	res.State = "dispatched"
	return res, nil
}

// Finish settles one reservation with a trusted final usage fact. Order:
// trust check (server-derived funding only, final deltas only — display_only
// parent rollups are rejected so child calls count exactly once), hard bound
// enforcement, U03 Finalize, then abnormal-cost detection against the
// reserved upper bound.
func (s *ExecutionGateService) Finish(ctx context.Context, reservationID string, fact domain.UsageFact) error {
	if err := domain.TrustedUsageFact(fact); err != nil {
		return err
	}
	if _, err := domain.ServiceDimension(fact.Service); err != nil {
		return err
	}
	if err := s.limits.Enforce(fact); err != nil {
		return err
	}
	var res repocommercial.ReservationRow
	err := s.db.WithContext(ctx).Where("key = ?", reservationID).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrGateReservationUnknown
	}
	if err != nil {
		return err
	}
	st, err := s.settle.Finalize(ctx, fact, reservationID)
	if err != nil {
		return err
	}
	return s.stopped(st.Amount, domain.Credits(res.UpperMicro))
}
