package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// PublicMarketplaceService coordinates the Public Marketplace governance
// workflow (spec §2/§7/§13): Verified Publisher submissions, platform
// review, the public catalog read model and cross-tenant Adoption of
// public releases. HTTP authorization remains the route boundary; the
// platform reviewer endpoints are gated SystemAdmin at the router, tenant
// identity always arrives from the authenticated request context.
type PublicMarketplaceService interface {
	// VerifyPublisher registers (or re-verifies) a tenant as a Verified
	// Publisher; the bool reports whether the registry row was created.
	VerifyPublisher(ctx context.Context, actorID string, tenantID uint64, note string) (VerifiedPublisherView, bool, error)
	RevokePublisher(ctx context.Context, actorID string, tenantID uint64) error
	ListVerifiedPublishers(ctx context.Context) ([]VerifiedPublisherView, error)
	// SubmitPublicRelease promotes one of the caller tenant's own immutable
	// tenant releases for platform review (verbatim portable copy).
	SubmitPublicRelease(ctx context.Context, tenantID uint64, actorID, sourceListingID, releaseID string) (PublicSubmissionView, error)
	// ListPublicSubmissions returns ONLY the caller tenant's own public
	// submissions (publisher isolation: no other tenants' rows).
	ListPublicSubmissions(ctx context.Context, tenantID uint64) ([]PublicSubmissionView, error)
	ListPublicReviewQueue(ctx context.Context) ([]PublicSubmissionView, error)
	ReviewPublicSubmission(ctx context.Context, reviewerID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (PublicReviewResult, error)
	// ListPublicCatalog is the discoverable public catalog read model. It
	// carries listing/release/publisher trust data ONLY — no adopter
	// identity, counts, mappings or task data ever enter it (spec §12).
	ListPublicCatalog(ctx context.Context) ([]PublicCatalogEntryView, error)
	GetPublicListing(ctx context.Context, listingID string) (*PublicListingDetailView, error)
	// AdoptPublicListing is the cross-tenant adoption entry: it propagates
	// the chosen public release (default: the listing's current release)
	// into the adopter tenant and creates/updates the tenant's Adoption;
	// the bool reports whether the introduction row was created.
	AdoptPublicListing(ctx context.Context, tenantID uint64, actorID, listingID, releaseID string) (PublicAdoptionResult, bool, error)
	// RecordEvaluation delegates platform structured Evaluation authoring
	// to the AgentEvaluationService (T35 #65 Task 4). The reviewer identity
	// always comes from the authenticated request context; HTTP authoring
	// is SystemAdmin-only at the router.
	RecordEvaluation(ctx context.Context, reviewerID string, evaluation types.AgentEvaluationEntity) (AgentEvaluationView, error)
}

type VerifiedPublisherView struct {
	types.VerifiedPublisherEntity
}

type PublicSubmissionView struct {
	types.PublicReleaseSubmissionEntity
}

type PublicReviewResult struct {
	Review  *types.PublicReleaseReviewEntity `json:"review"`
	Release *types.PublicAgentReleaseEntity  `json:"release,omitempty"`
}

// PublicReleaseSummary is the portable documentation of one public
// release: version, digest, Manifest and Dependency Lock plus the
// allowlisted manifest projection (compatibility floor, required
// capabilities, license). No bundle bytes cross the service boundary
// through views.
type PublicReleaseSummary struct {
	ID                       string
	SemanticVersion          string
	BundleDigest             string
	ManifestJSON             string
	DependencyLockJSON       string
	MinimumWeKnoraCapability string
	CapabilityRequirements   []string
	LicenseID                string
	CreatedAt                time.Time
}

// PublicReleaseReviewSummary is the platform review decision surfaced with
// a public release: decision/reviewer/time ONLY — never the review Reason.
type PublicReleaseReviewSummary struct {
	SubmissionID string
	ReviewerID   string
	Decision     string
	ReviewedAt   time.Time
}

// PublicCatalogEntryView is one public catalog row: listing identity,
// display data, publisher trust signal, the current release summary, the
// safe review summary, the release-pinned Evaluation summaries and the
// bucketed metrics projection (spec §12: no adopter-derived identity).
type PublicCatalogEntryView struct {
	ListingID            string
	DisplayName          string
	Summary              string
	State                string
	PublisherTenantID    uint64
	PublisherVerified    bool
	CurrentRelease       *PublicReleaseSummary
	CurrentReleaseReview *PublicReleaseReviewSummary
	Evaluations          []AgentEvaluationView
	Metrics              *MarketplaceMetricsView
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PublicListingDetailView struct {
	PublicCatalogEntryView
}

// PublicAdoptionResult pairs the adopter-side introduction with the
// resulting Adoption row. The full variant-bearing adoption view stays on
// GET /marketplace/tenant/adoptions (#59).
type PublicAdoptionResult struct {
	Introduction types.TenantIntroducedReleaseEntity `json:"introduction"`
	Adoption     types.AgentAdoptionEntity           `json:"adoption"`
}
