package types

import (
	"time"

	"gorm.io/gorm"
)

// PublishedExpertEntity is one row of the tenant-internal expert market
// (M4 Task 5): an immutable materialized-expert snapshot exported from a
// member's agent and published to the whole workspace. Unlike
// PublishedSkillEntity (a visibility flag over a shared catalog definition),
// this row owns the snapshot: name/description recorded at publish time plus
// the snapshot directory ref and its content digest. Installs copy the
// snapshot into the tenant's installed-experts root and instantiate through
// the M2 ExpertService; nothing about the source agent is re-read.
type PublishedExpertEntity struct {
	// ID is the row key, stable across re-publishes of the same slot:
	// upserts conflict on (tenant_id, agent_id) and keep the existing ID.
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"primaryKey"`
	// AgentID is custom_agents.id of the source agent the snapshot was
	// exported from. The materialized expert's manifest ID derives from it
	// (tenant-expert-<agentID>), which is also the expert_installs slug that
	// links installs back to this row.
	AgentID string `gorm:"type:varchar(36);not null"`
	// Name is the expert display name recorded at publish time (the body
	// override or the agent's name). The zh/en passthrough lives in the
	// snapshot manifest; the row keeps the display string for listings.
	Name string `gorm:"type:varchar(255);not null"`
	// Description is the description recorded at publish time.
	Description string `gorm:"type:text"`
	// SnapshotRef names where the immutable snapshot tree lives (the
	// tenant-scoped directory under the published-experts root).
	SnapshotRef string `gorm:"type:varchar(1024);not null"`
	// SnapshotSHA256 is the content digest of the snapshot tree at write
	// time (experts.MaterializedExpert.SnapshotSHA256); installs verify the
	// snapshot against it before seeding.
	SnapshotSHA256 string `gorm:"type:varchar(64);not null"`
	// PublishedBy records the publishing user (id); empty means a
	// system/API-key principal.
	PublishedBy string `gorm:"type:varchar(255)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (e *PublishedExpertEntity) TableName() string { return "published_experts" }
