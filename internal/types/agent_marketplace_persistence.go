package types

import "time"

type AgentMarketplaceListingEntity struct {
	ID               string  `gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64  `gorm:"primaryKey"`
	SourceAgentID    string  `gorm:"type:varchar(36);not null"`
	DisplayName      string  `gorm:"type:varchar(255);not null"`
	Summary          string  `gorm:"type:text;not null;default:''"`
	State            string  `gorm:"type:varchar(32);not null;default:'listed'"`
	CurrentReleaseID *string `gorm:"type:varchar(36)"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AgentMarketplaceListingEntity) TableName() string { return "agent_marketplace_listings" }

type AgentReleaseSubmissionEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64 `gorm:"primaryKey"`
	ListingID          string `gorm:"type:varchar(36);not null"`
	AgentVersionID     string `gorm:"type:varchar(36);not null"`
	SourceAgentID      string `gorm:"type:varchar(36);not null"`
	AuthorID           string `gorm:"type:varchar(255);not null;default:''"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	Status             string `gorm:"type:varchar(32);not null;default:'submitted'"`
	CreatedAt          time.Time
}

func (AgentReleaseSubmissionEntity) TableName() string { return "agent_release_submissions" }

type AgentReleaseReviewEntity struct {
	ID             string `gorm:"type:varchar(36);primaryKey"`
	TenantID       uint64 `gorm:"primaryKey"`
	SubmissionID   string `gorm:"type:varchar(36);not null"`
	ReviewerID     string `gorm:"type:varchar(255);not null"`
	ReviewedDigest string `gorm:"type:varchar(64);not null"`
	Decision       string `gorm:"type:varchar(32);not null"`
	Reason         string `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time
}

func (AgentReleaseReviewEntity) TableName() string { return "agent_release_reviews" }

type AgentReleaseEntity struct {
	ID                 string `gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64 `gorm:"primaryKey"`
	ListingID          string `gorm:"type:varchar(36);not null"`
	SubmissionID       string `gorm:"type:varchar(36);not null"`
	AgentVersionID     string `gorm:"type:varchar(36);not null"`
	SourceAgentID      string `gorm:"type:varchar(36);not null"`
	ReleaseNumber      int    `gorm:"not null"`
	SemanticVersion    string `gorm:"type:varchar(64);not null"`
	BundleDigest       string `gorm:"type:varchar(64);not null"`
	ManifestJSON       string `gorm:"type:text;not null"`
	DependencyLockJSON string `gorm:"type:text;not null"`
	Bundle             []byte `gorm:"type:blob;not null"`
	PublishedBy        string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt          time.Time
}

func (AgentReleaseEntity) TableName() string { return "agent_releases" }

type AgentReleaseReviewDecision struct {
	ReviewerID string
	Decision   string
	Reason     string
}
