package types

import "time"

// Agent Adoption persistence entities (T29, Ticket #59).
//
// The Marketplace owns Adoption, Variant and the local capability mapping
// rows (docs/specs/2026-09-20-agent-marketplace-domain-model.md §4, §8).
// The local Agent Definition and Agent Version stay in the Agent domain:
// agent_adoption_variants references them by ID only, with no FK, so a
// soft-deleted local agent simply disappears from the available-agent read
// model instead of blocking governance history.

// AgentAdoptionEntity is the Tenant's governance relationship to one
// Listing: unique per (tenant, listing), holding the accepted Release.
type AgentAdoptionEntity struct {
	ID                string `gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64 `gorm:"primaryKey"`
	ListingID         string `gorm:"type:varchar(36);not null"`
	AcceptedReleaseID string `gorm:"type:varchar(36);not null"`
	State             string `gorm:"type:varchar(32);not null;default:'active'"`
	CreatedBy         string `gorm:"type:varchar(255);not null;default:''"`
	EndedBy           string `gorm:"type:varchar(255);not null;default:''"`
	EndedAt           *time.Time
	EndReason         string `gorm:"type:text;not null;default:''"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (AgentAdoptionEntity) TableName() string { return "agent_adoptions" }

// AgentAdoptionVariantEntity is one local Variant derived from an Adoption,
// pinned to one immutable Release. State machine: draft -> mapped -> tested
// -> published (retire belongs to #63). LocalAgentID/LocalAgentVersionID are
// set exactly once, at publication.
type AgentAdoptionVariantEntity struct {
	ID                  string `gorm:"type:varchar(36);primaryKey"`
	TenantID            uint64 `gorm:"primaryKey"`
	AdoptionID          string `gorm:"type:varchar(36);not null"`
	ReleaseID           string `gorm:"type:varchar(36);not null"`
	Name                string `gorm:"type:varchar(255);not null"`
	State               string `gorm:"type:varchar(32);not null;default:'draft'"`
	LocalAgentID        string `gorm:"type:varchar(36)"`
	LocalAgentVersionID string `gorm:"type:varchar(36)"`
	CreatedBy           string `gorm:"type:varchar(255);not null;default:''"`
	TestedBy            string `gorm:"type:varchar(255);not null;default:''"`
	TestedAt            *time.Time
	PublishedBy         string `gorm:"type:varchar(255);not null;default:''"`
	PublishedAt         *time.Time
	RetiredBy           string `gorm:"type:varchar(255);not null;default:''"`
	RetiredAt           *time.Time
	RetirementReason    string `gorm:"type:text;not null;default:''"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (AgentAdoptionVariantEntity) TableName() string { return "agent_adoption_variants" }

// AgentVariantCapabilityMappingEntity is one local capability binding of a
// Variant: the Manifest capability name mapped to local model / knowledge /
// connection choices. ID lists are canonical JSON arrays ("[]"/["id",...]).
type AgentVariantCapabilityMappingEntity struct {
	ID               string `gorm:"type:varchar(36);primaryKey"`
	TenantID         uint64 `gorm:"primaryKey"`
	VariantID        string `gorm:"type:varchar(36);not null"`
	Capability       string `gorm:"type:varchar(255);not null"`
	ModelID          string `gorm:"type:varchar(255);not null;default:''"`
	KnowledgeBaseIDs string `gorm:"type:text;not null;default:'[]'"`
	ConnectionIDs    string `gorm:"type:text;not null;default:'[]'"`
	UpdatedBy        string `gorm:"type:varchar(255);not null;default:''"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AgentVariantCapabilityMappingEntity) TableName() string {
	return "agent_variant_capability_mappings"
}
