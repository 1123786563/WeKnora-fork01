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
}
