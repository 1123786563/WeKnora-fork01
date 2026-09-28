package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentMarketplaceLifecycleService owns Tenant marketplace lifecycle actions.
type AgentMarketplaceLifecycleService interface {
	RetireVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (AdoptionVariantView, error)
	EndAdoption(ctx context.Context, tenantID uint64, actorID, adoptionID string) (AdoptionView, error)
	UnlistListing(ctx context.Context, tenantID uint64, actorID, listingID string) (TenantListingView, error)
	DeprecateRelease(ctx context.Context, tenantID uint64, actorID, releaseID, successorReleaseID string) (*types.AgentReleaseEntity, error)
}
