package workbench

import (
	"context"
	"errors"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	workbench "github.com/Tencent/WeKnora/internal/workbench"
)

var (
	ErrInteractionNotFound   = errors.New("interaction_not_found")
	ErrInteractionExpired    = errors.New("interaction_expired")
	ErrInteractionRevoked    = errors.New("interaction_revoked")
	ErrCapabilityUnavailable = errors.New("capability_unavailable")
)

// InteractionStore is the durable boundary. Implementations must use a
// tenant+owner predicate and an atomic decision_id/revision compare-and-swap.
// Keeping this port narrow lets the HTTP surface share the existing approval
// service without duplicating persistence or authorization logic.
type InteractionStore interface {
	List(ctx context.Context, tenantID uint64, ownerID, runID string) ([]workbench.InteractionDecision, error)
	Get(ctx context.Context, tenantID uint64, ownerID, id string) (workbench.InteractionDecision, error)
	Decide(ctx context.Context, tenantID uint64, ownerID, id string, decision workbench.InteractionDecision) (workbench.InteractionDecision, error)
}

type SteerPort interface {
	Steer(ctx context.Context, tenantID uint64, ownerID, runID, text string, expectedRevision int64) error
}

type CancelPort interface {
	Cancel(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64) error
}

type Service struct {
	store  InteractionStore
	steer  SteerPort
	cancel CancelPort
}

func NewInteractionService(store InteractionStore, steer SteerPort, cancel CancelPort) *Service {
	return &Service{store: store, steer: steer, cancel: cancel}
}

func identity(ctx context.Context) (uint64, string, error) {
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return 0, "", errors.New("tenant context is required")
	}
	owner, ok := types.UserIDFromContext(ctx)
	if !ok || strings.TrimSpace(owner) == "" {
		return 0, "", errors.New("actor context is required")
	}
	return tenant, owner, nil
}

func (s *Service) List(ctx context.Context, runID string) ([]workbench.InteractionDecision, error) {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil || strings.TrimSpace(runID) == "" {
		return nil, ErrInteractionNotFound
	}
	return s.store.List(ctx, tenant, owner, strings.TrimSpace(runID))
}

func (s *Service) Decide(ctx context.Context, id string, input workbench.InteractionDecision) (workbench.InteractionDecision, error) {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	if s == nil || s.store == nil {
		return workbench.InteractionDecision{}, ErrInteractionNotFound
	}
	current, err := s.store.Get(ctx, tenant, owner, strings.TrimSpace(id))
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	if input.Kind != "" && input.Kind != current.Kind {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	if err := workbench.ValidateInteractionAction(current.Kind, input.Action); err != nil {
		return workbench.InteractionDecision{}, err
	}
	if input.ArgsHash != "" && current.ArgsHash != "" && input.ArgsHash != current.ArgsHash {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	input.ID = current.ID
	input.Kind = current.Kind
	input.ArgsHash = current.ArgsHash
	return s.store.Decide(ctx, tenant, owner, current.ID, input)
}

func (s *Service) Command(ctx context.Context, runID string, command workbench.ExecutionCommand) error {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return err
	}
	if err := command.Validate(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(runID) == "" {
		return ErrInteractionNotFound
	}
	switch command.Action {
	case "cancel":
		if s.cancel == nil {
			return ErrCapabilityUnavailable
		}
		return s.cancel.Cancel(ctx, tenant, owner, runID, command.ExpectedRevision)
	case "steer":
		if s.steer == nil {
			return ErrCapabilityUnavailable
		}
		return s.steer.Steer(ctx, tenant, owner, runID, command.Text, command.ExpectedRevision)
	default:
		return workbench.ErrCommandActionMismatch
	}
}
