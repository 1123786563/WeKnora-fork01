package types

import "time"

// Agent Upgrade Proposal persistence entity (T31, Ticket #61; spec §9).
//
// A proposal is generated when a Listing's current Release advances past an
// Adoption's accepted Release (tenant-local publish or #60 introduction
// ledger — both resolve through the adoption repository's read fallbacks).
// Acceptance only ever creates a NEW Variant draft pinned to ToReleaseID;
// the previously accepted release, other Variants and existing Tasks never
// change (CONTEXT.md「Agent 升级建议」_避免_: 自动升级 / 覆盖当前 Agent
// Version). ToReleaseID carries no FK to agent_releases for the same reason
// migration 000114/000193 relaxed the adoption FKs: introduced releases live
// in tenant_introduced_releases.
type AgentUpgradeProposalEntity struct {
	ID                string `gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64 `gorm:"primaryKey"`
	AdoptionID        string `gorm:"type:varchar(36);not null"`
	ListingID         string `gorm:"type:varchar(36);not null"`
	FromReleaseID     string `gorm:"type:varchar(36);not null"`
	ToReleaseID       string `gorm:"type:varchar(36);not null"`
	ToSemanticVersion string `gorm:"type:varchar(64);not null;default:''"`
	// DiffJSON is the canonical serialization of AgentUpgradeDiff, computed
	// once at materialization from the two immutable releases. Releases are
	// immutable, so the stored diff is the stable review record.
	DiffJSON          string `gorm:"type:text;not null;default:'{}'"`
	State             string `gorm:"type:varchar(32);not null;default:'open'"`
	AcceptedVariantID string `gorm:"type:varchar(36)"`
	ResolvedBy        string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AgentUpgradeProposalEntity) TableName() string { return "agent_upgrade_proposals" }
