package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentMarketplaceLifecycleService performs tenant-scoped lifecycle changes
// for adopted variants, adoptions, listings, and releases.
type AgentMarketplaceLifecycleService interface {
	RetireVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (AdoptionVariantView, error)
	EndAdoption(ctx context.Context, tenantID uint64, actorID, adoptionID string) (AdoptionView, error)
	UnlistListing(ctx context.Context, tenantID uint64, actorID, listingID string) (TenantListingView, error)
	DeprecateRelease(ctx context.Context, tenantID uint64, actorID, releaseID, successorReleaseID string) (*types.AgentReleaseEntity, error)
}
