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
	GetReleaseBySubmission(ctx context.Context, tenantID uint64, submissionID string) (*types.AgentReleaseEntity, error)
	// FindDerivation resolves the adoption lineage of a variant-published
	// local agent (T32 #62): nil when the agent is not derived from an
	// adopted Release — original content has no lineage.
	FindDerivation(context.Context, uint64, string) (*types.AgentForkDerivation, error)
	// GetLicense returns the deployment license row, nil when unregistered.
	GetLicense(context.Context, string) (*types.AgentLicenseEntity, error)
	UpsertLicense(context.Context, *types.AgentLicenseEntity) (*types.AgentLicenseEntity, error)
	ListLicenses(context.Context) ([]types.AgentLicenseEntity, error)
	GetListing(ctx context.Context, tenantID uint64, listingID string) (*types.AgentMarketplaceListingEntity, error)
}

// AgentMarketplaceService coordinates Tenant-authorized release submission
// and review transitions. HTTP authorization remains the route boundary.
type AgentMarketplaceService interface {
	SubmitRelease(ctx context.Context, tenantID uint64, actorID, versionID string, input SubmitReleaseInput) (ReleaseSubmissionView, error)
	ListReviewQueue(ctx context.Context, tenantID uint64) ([]ReleaseSubmissionView, error)
	ReviewSubmission(ctx context.Context, tenantID uint64, actorID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (ReleaseReviewResult, error)
	ListTenantCatalog(ctx context.Context, tenantID uint64) ([]TenantListingView, error)
	GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)
	// RegisterLicense records (or re-records) one deployment license term;
	// re-registering is how a redistribution flip propagates (T32 #62).
	RegisterLicense(ctx context.Context, actorID string, input LicenseInput) (types.AgentLicenseEntity, error)
	ListLicenses(ctx context.Context) ([]types.AgentLicenseEntity, error)
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

// LicenseInput registers one deployment license term. AllowsRedistribution
// is the flag the fork re-submission gate consults (T32 #62).
type LicenseInput struct {
	ID                   string
	Name                 string
	AllowsRedistribution bool
}
