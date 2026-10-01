package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentReleaseDeprecated                = errors.New("agent release is deprecated")
	ErrAgentMarketplaceLifecycleInvalidInput = errors.New("invalid agent marketplace lifecycle request")
)

const AgentAdoptionStateEnded = "ended"

type AgentMarketplaceLifecycleService struct {
	adoptions repository.AgentAdoptionRepository
	listings  repository.AgentMarketplaceRepository
	now       func() time.Time
}

var _ interfaces.AgentMarketplaceLifecycleService = (*AgentMarketplaceLifecycleService)(nil)

func NewAgentMarketplaceLifecycleService(adoptions repository.AgentAdoptionRepository, listings repository.AgentMarketplaceRepository) *AgentMarketplaceLifecycleService {
	return &AgentMarketplaceLifecycleService{adoptions: adoptions, listings: listings, now: time.Now}
}

func (s *AgentMarketplaceLifecycleService) RetireVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (interfaces.AdoptionVariantView, error) {
	actorID, variantID = strings.TrimSpace(actorID), strings.TrimSpace(variantID)
	if tenantID == 0 || actorID == "" || variantID == "" {
		return interfaces.AdoptionVariantView{}, ErrAgentMarketplaceLifecycleInvalidInput
	}
	variant, err := s.adoptions.GetVariant(ctx, tenantID, variantID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	if variant == nil {
		return interfaces.AdoptionVariantView{}, ErrAgentAdoptionNotFound
	}
	updated, err := s.adoptions.UpdateVariantState(ctx, tenantID, variantID,
		[]string{AgentVariantStateDraft, AgentVariantStateMapped, AgentVariantStateTested, AgentVariantStatePublished},
		"retired", map[string]any{"retired_by": actorID})
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return s.variantViewOf(ctx, tenantID, updated)
}

func (s *AgentMarketplaceLifecycleService) EndAdoption(ctx context.Context, tenantID uint64, actorID, adoptionID string) (interfaces.AdoptionView, error) {
	actorID, adoptionID = strings.TrimSpace(actorID), strings.TrimSpace(adoptionID)
	if tenantID == 0 || actorID == "" || adoptionID == "" {
		return interfaces.AdoptionView{}, ErrAgentMarketplaceLifecycleInvalidInput
	}
	row, err := s.adoptions.TransitionAdoption(ctx, tenantID, adoptionID, AgentAdoptionStateActive, AgentAdoptionStateEnded, map[string]any{"ended_by": actorID})
	if err != nil {
		if errors.Is(err, repository.ErrAgentAdoptionEndPrecondition) {
			return interfaces.AdoptionView{}, fmt.Errorf("%w: %v", ErrAgentAdoptionStateConflict, err)
		}
		return interfaces.AdoptionView{}, err
	}
	return s.adoptionViewOf(ctx, tenantID, row)
}

func (s *AgentMarketplaceLifecycleService) UnlistListing(ctx context.Context, tenantID uint64, actorID, listingID string) (interfaces.TenantListingView, error) {
	actorID, listingID = strings.TrimSpace(actorID), strings.TrimSpace(listingID)
	if tenantID == 0 || actorID == "" || listingID == "" {
		return interfaces.TenantListingView{}, ErrAgentMarketplaceLifecycleInvalidInput
	}
	row, err := s.listings.TransitionListingState(ctx, tenantID, listingID, "listed", "unlisted", map[string]any{"unlisted_by": actorID})
	if err != nil {
		return interfaces.TenantListingView{}, err
	}
	return interfaces.TenantListingView{AgentMarketplaceListingEntity: *row}, nil
}

func (s *AgentMarketplaceLifecycleService) DeprecateRelease(ctx context.Context, tenantID uint64, actorID, releaseID, successorReleaseID string) (*types.AgentReleaseEntity, error) {
	actorID, releaseID, successorReleaseID = strings.TrimSpace(actorID), strings.TrimSpace(releaseID), strings.TrimSpace(successorReleaseID)
	if tenantID == 0 || actorID == "" || releaseID == "" || successorReleaseID == "" || releaseID == successorReleaseID {
		return nil, ErrAgentMarketplaceLifecycleInvalidInput
	}
	release, err := s.listings.GetRelease(ctx, tenantID, releaseID)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, repository.ErrAgentMarketplaceNotFound
	}
	successor, err := s.listings.GetRelease(ctx, tenantID, successorReleaseID)
	if err != nil {
		return nil, err
	}
	if successor == nil || successor.ListingID != release.ListingID || successor.DeprecatedAt != nil {
		return nil, fmt.Errorf("%w: successor release must exist, be active, and belong to the same listing", ErrAgentMarketplaceLifecycleInvalidInput)
	}
	row, err := s.listings.DeprecateRelease(ctx, tenantID, releaseID, actorID, successorReleaseID)
	if errors.Is(err, repository.ErrAgentReleaseSuccessorInvalid) {
		return nil, fmt.Errorf("%w: successor release must remain active and belong to the same listing", ErrAgentMarketplaceLifecycleInvalidInput)
	}
	return row, err
}

func (s *AgentMarketplaceLifecycleService) adoptionViewOf(ctx context.Context, tenantID uint64, row *types.AgentAdoptionEntity) (interfaces.AdoptionView, error) {
	view := interfaces.AdoptionView{AgentAdoptionEntity: *row, Variants: []interfaces.AdoptionVariantView{}}
	variants, err := s.adoptions.ListVariantsByAdoption(ctx, tenantID, row.ID)
	if err != nil {
		return interfaces.AdoptionView{}, err
	}
	for i := range variants {
		variant, err := s.variantViewOf(ctx, tenantID, &variants[i])
		if err != nil {
			return interfaces.AdoptionView{}, err
		}
		view.Variants = append(view.Variants, variant)
	}
	return view, nil
}

func (s *AgentMarketplaceLifecycleService) variantViewOf(ctx context.Context, tenantID uint64, variant *types.AgentAdoptionVariantEntity) (interfaces.AdoptionVariantView, error) {
	missing, err := missingCapabilitiesOf(ctx, s.adoptions, tenantID, variant)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return interfaces.AdoptionVariantView{AgentAdoptionVariantEntity: *variant, MissingCapabilities: missing}, nil
}

func missingCapabilitiesOf(ctx context.Context, repo repository.AgentAdoptionRepository, tenantID uint64, variant *types.AgentAdoptionVariantEntity) ([]string, error) {
	manifest, err := releaseManifest(ctx, repo, tenantID, variant.ReleaseID)
	if err != nil {
		return nil, err
	}
	mappings, err := repo.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return nil, err
	}
	return missingCapabilities(manifest.CapabilityRequirements, mappings), nil
}
