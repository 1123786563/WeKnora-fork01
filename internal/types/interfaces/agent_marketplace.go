package interfaces

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
)

type AgentMarketplaceRepository interface {
	CreateSubmission(ctx context.Context, listing *types.AgentMarketplaceListingEntity, submission *types.AgentReleaseSubmissionEntity) (*types.AgentReleaseSubmissionEntity, error)
	ListReviewQueue(ctx context.Context, tenantID uint64) ([]types.AgentReleaseSubmissionEntity, error)
	ReviewAndPublishTx(ctx context.Context, tenantID uint64, expectedPriorReleaseID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (*types.AgentReleaseReviewEntity, *types.AgentReleaseEntity, error)
	ListTenantCatalog(ctx context.Context, tenantID uint64) ([]types.AgentMarketplaceListingEntity, error)
	GetSubmission(ctx context.Context, tenantID uint64, submissionID string) (*types.AgentReleaseSubmissionEntity, error)
	GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)
}

// AgentMarketplaceService coordinates Tenant-authorized release submission
// and review transitions. HTTP authorization remains the route boundary.
type AgentMarketplaceService interface {
	SubmitRelease(ctx context.Context, tenantID uint64, actorID, versionID string, input SubmitReleaseInput) (ReleaseSubmissionView, error)
	ListReviewQueue(ctx context.Context, tenantID uint64) ([]ReleaseSubmissionView, error)
	ReviewSubmission(ctx context.Context, tenantID uint64, actorID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (ReleaseReviewResult, error)
	ListTenantCatalog(ctx context.Context, tenantID uint64) ([]TenantListingView, error)
	GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)
}

type SubmitReleaseInput struct{ Metadata types.ReleaseMetadata }

type ReleaseDependencyResolver interface {
	Resolve(ctx context.Context, tenantID uint64, version types.AgentVersionSnapshot) (types.DependencyLock, error)
}

type ReleaseSubmissionView struct {
	types.AgentReleaseSubmissionEntity
}
type TenantListingView struct {
	types.AgentMarketplaceListingEntity
}
type ReleaseReviewResult struct {
	Review  *types.AgentReleaseReviewEntity `json:"review"`
	Release *types.AgentReleaseEntity       `json:"release,omitempty"`
}
