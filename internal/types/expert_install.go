package types

import (
	"time"

	"gorm.io/gorm"
)

// ExpertInstallEntity is one row of the per-tenant expert install ledger
// (M4 skill market): which SkillHub skillsets a tenant has installed, where
// the materialized expert tree lives and the content digest it was written
// from. The catalog merge reads the trees, not these rows; the ledger is the
// bookkeeping that makes installs idempotent and auditable.
type ExpertInstallEntity struct {
	// ID is the row key, stable across reinstalls of the same slot: upserts
	// conflict on (tenant_id, slug) and keep the existing ID.
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"primaryKey"`
	// Slug is the SkillHub skillset slug; the install directory and the
	// materialized expert ID (skillhub-skillset-<slug>) derive from it.
	Slug string `gorm:"type:varchar(255);not null"`
	// StorageRef names where the materialized tree lives (the tenant-scoped
	// install directory under the expert-market root).
	StorageRef string `gorm:"type:varchar(1024);not null"`
	// SnapshotSHA256 is the content digest of the materialized tree at write
	// time (experts.MaterializedExpert.SnapshotSHA256), so a reinstall of
	// unchanged content is recognizable without re-reading storage.
	SnapshotSHA256 string `gorm:"type:varchar(64);not null"`
	// CreatedBy records the installing user (id or name); empty means a
	// system-triggered install.
	CreatedBy string `gorm:"type:varchar(255)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (e *ExpertInstallEntity) TableName() string { return "expert_installs" }
