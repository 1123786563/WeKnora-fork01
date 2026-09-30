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

var _ interfaces.AgentMarketplaceLifecycleService = (*AgentMarketplaceLifecycleService)(nil)

type AgentMarketplaceLifecycleService struct {
	adoptions repository.AgentAdoptionRepository
	listings  repository.AgentMarketplaceRepository
	now       func() time.Time
}

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
		return interfaces.AdoptionVariantView{}, repository.ErrAgentAdoptionNotFound
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
	row, err := s.adoptions.EndAdoption(ctx, tenantID, adoptionID, actorID, "ended")
	if err != nil {
		if errors.Is(err, repository.ErrAgentAdoptionEndPrecondition) || errors.Is(err, repository.ErrAgentAdoptionTransition) {
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
	row, err := s.listings.UnlistTenantListing(ctx, tenantID, listingID, actorID, "unlisted")
	if err != nil {
		return interfaces.TenantListingView{}, err
	}
	return interfaces.TenantListingView{AgentMarketplaceListingEntity: *row}, nil
}

func (s *AgentMarketplaceLifecycleService) DeprecateRelease(ctx context.Context, tenantID uint64, actorID, releaseID, successorReleaseID string) (*types.AgentReleaseEntity, error) {
	actorID = strings.TrimSpace(actorID)
	releaseID = strings.TrimSpace(releaseID)
	successorReleaseID = strings.TrimSpace(successorReleaseID)
	if tenantID == 0 || actorID == "" || releaseID == "" || successorReleaseID == "" || successorReleaseID == releaseID {
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
	if successor == nil {
		return nil, fmt.Errorf("%w: successor release %s not found", ErrAgentMarketplaceLifecycleInvalidInput, successorReleaseID)
	}
	if successor.ListingID != release.ListingID {
		return nil, fmt.Errorf("%w: successor release must belong to the same listing", ErrAgentMarketplaceLifecycleInvalidInput)
	}
	if successor.DeprecatedAt != nil {
		return nil, fmt.Errorf("%w: successor release %s is already deprecated", ErrAgentMarketplaceLifecycleInvalidInput, successorReleaseID)
	}
	return s.listings.DeprecateTenantRelease(ctx, tenantID, releaseID, actorID, successorReleaseID, "deprecated")
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
	manifest, err := releaseManifest(ctx, s.adoptions, tenantID, variant.ReleaseID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	mappings, err := s.adoptions.ListCapabilityMappings(ctx, tenantID, variant.ID)
	if err != nil {
		return interfaces.AdoptionVariantView{}, err
	}
	return interfaces.AdoptionVariantView{
		AgentAdoptionVariantEntity: *variant,
		MissingCapabilities:        missingCapabilities(manifest.CapabilityRequirements, mappings),
	}, nil
}

func successorHint(successorReleaseID string) string {
	if successorReleaseID == "" {
		return "none declared"
	}
	return successorReleaseID
}
