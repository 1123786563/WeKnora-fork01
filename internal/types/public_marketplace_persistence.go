package types

import "time"

// Public Marketplace persistence entities (T30, Ticket #60).
//
// The platform owns the public catalog (Verified Publisher registry,
// public listings/submissions/reviews/releases); the adopter tenant owns
// its introduction ledger. Cross-tenant Adoptions reference the PUBLIC
// listing id and the introduced local release id, which is why the three
// FKs from agent_adoptions/agent_adoption_variants to the tenant
// marketplace tables were relaxed in migration 000193/000114 — tenant
// isolation remains enforced by every query's tenant_id binding.

// VerifiedPublisherEntity is the platform registry row: one tenant that
// may submit releases to the Public Marketplace (CONTEXT.md「Verified
// Publisher」). state: verified | revoked (revocation propagation is #64).
type VerifiedPublisherEntity struct {
	TenantID   uint64 `gorm:"primaryKey"`
	State      string `gorm:"type:varchar(32);not null;default:'verified'"`
	VerifiedBy string `gorm:"type:varchar(255);not null;default:''"`
	Note       string `gorm:"type:text;not null;default:''"`
	VerifiedAt time.Time
	UpdatedAt  time.Time
}

func (VerifiedPublisherEntity) TableName() string { return "public_marketplace_verified_publishers" }

// PublicMarketplaceListingEntity is the public catalog identity derived
// from one publisher tenant listing (UNIQUE (publisher_tenant_id,
// source_listing_id)). state stays "listed" in this ticket; unlist/
// deprecate belong to #63, security revocation to #64.
type PublicMarketplaceListingEntity struct {
	ID                string  `gorm:"type:varchar(36);primaryKey"`
	PublisherTenantID uint64  `gorm:"not null"`
	SourceListingID   string  `gorm:"type:varchar(36);not null"`
	DisplayName       string  `gorm:"type:varchar(255);not null"`
	Summary           string  `gorm:"type:text;not null;default:''"`
	State             string  `gorm:"type:varchar(32);not null;default:'listed'"`
	CurrentReleaseID  *string `gorm:"type:varchar(36)"`
	UnlistedBy        string  `gorm:"type:varchar(255);not null;default:''"`
	UnlistedAt        *time.Time
	UnlistReason      string `gorm:"type:text;not null;default:''"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PublicMarketplaceListingEntity) TableName() string { return "public_marketplace_listings" }

// PublicReleaseSubmissionEntity is a Verified Publisher promoting one of
// its own immutable tenant releases for platform review. The bundle is a
// verbatim copy of the tenant release's portable bundle; the digest is
// re-verified server-side on both submit and review.
type PublicReleaseSubmissionEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	PublisherTenantID  uint64 `gorm:"not null"`
	PublicListingID    string `gorm:"type:varchar(36);not null"`
	SourceListingID    string `gorm:"type:varchar(36);not null"`
	SourceReleaseID    string `gorm:"type:varchar(36);not null"`
	PublisherActorID   string `gorm:"type:varchar(255);not null;default:''"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	Status             string `gorm:"type:varchar(32);not null;default:'submitted'"`
	CreatedAt          time.Time
}

func (PublicReleaseSubmissionEntity) TableName() string { return "public_release_submissions" }

// PublicReleaseReviewEntity records the platform review decision exactly
// like the tenant review rows: reviewer, reviewed digest, decision, reason
// (spec §7 审核记录保留要求). One review decides a submission.
type PublicReleaseReviewEntity struct {
	ID             string `gorm:"type:varchar(36);primaryKey"`
	SubmissionID   string `gorm:"type:varchar(36);not null"`
	ReviewerID     string `gorm:"type:varchar(255);not null"`
	ReviewedDigest string `gorm:"type:varchar(64);not null"`
	Decision       string `gorm:"type:varchar(32);not null"`
	Reason         string `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time
}

func (PublicReleaseReviewEntity) TableName() string { return "public_release_reviews" }

// PublicAgentReleaseEntity is an approved, immutable public release. Like
// the tenant release it pins semantic version, digest, Manifest, Dependency
// Lock and the portable bundle bytes.
type PublicAgentReleaseEntity struct {
	ID                   string `gorm:"type:varchar(36);primaryKey"`
	ListingID            string `gorm:"type:varchar(36);not null"`
	SubmissionID         string `gorm:"type:varchar(36);not null"`
	PublisherTenantID    uint64 `gorm:"not null"`
	ReleaseNumber        int    `gorm:"not null"`
	SemanticVersion      string `gorm:"type:varchar(64);not null"`
	BundleDigest         string `gorm:"type:varchar(64);not null"`
	ManifestJSON         string `gorm:"type:text;not null"`
	DependencyLockJSON   string `gorm:"type:text;not null"`
	Bundle               []byte `gorm:"type:blob;not null"`
	PublishedBy          string `gorm:"type:varchar(255);not null;default:''"`
	DeprecatedBy         string `gorm:"type:varchar(255);not null;default:''"`
	DeprecatedAt         *time.Time
	DeprecationReason    string `gorm:"type:text;not null;default:''"`
	ReplacementReleaseID string `gorm:"type:varchar(36);not null;default:''"`
	CreatedAt            time.Time
}

func (PublicAgentReleaseEntity) TableName() string { return "public_agent_releases" }

// TenantIntroducedReleaseEntity is the adopter-side introduction ledger:
// the verbatim portable copy of ONE public release inside one tenant,
// unique per (tenant, public release). The #59 chain reads it through the
// adoption repository fallback; the Adoption references the public listing
// id and this row's local id.
type TenantIntroducedReleaseEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64 `gorm:"primaryKey"`
	PublicListingID    string `gorm:"type:varchar(36);not null"`
	PublicReleaseID    string `gorm:"type:varchar(36);not null"`
	DisplayName        string `gorm:"type:varchar(255);not null"`
	Summary            string `gorm:"type:text;not null;default:''"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	IntroducedBy       string `gorm:"type:varchar(255);not null;default:''"`
	IntroducedAt       time.Time
}

func (TenantIntroducedReleaseEntity) TableName() string { return "tenant_introduced_releases" }
