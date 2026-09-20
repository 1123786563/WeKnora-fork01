package types

import (
	"time"

	"gorm.io/gorm"
)

// PublishedSkillEntity is one row of the tenant-internal skill market
// (M4 Task 4): which catalog skills an admin has published to the
// workspace-internal market. Publisher and installer are members of the SAME
// tenant and already share the tenant_skill_catalog definition, so this row
// is a visibility FLAG over that shared definition, not a content copy —
// installs go through InstallCatalogToConfigs on the shared catalog row and
// no second bundle object is ever minted.
type PublishedSkillEntity struct {
	// ID is the row key, stable across re-publishes of the same slot:
	// upserts conflict on (tenant_id, catalog_id) and keep the existing ID.
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"primaryKey"`
	// CatalogID is tenant_skill_catalog.id of the shared definition this row
	// flags as market-visible.
	CatalogID string `gorm:"type:varchar(36);not null"`
	// PublishedBy records the publishing user (id); empty means a
	// system/API-key principal.
	PublishedBy string `gorm:"type:varchar(255)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (e *PublishedSkillEntity) TableName() string { return "published_skills" }
