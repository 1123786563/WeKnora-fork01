package types

import (
	"time"

	"gorm.io/gorm"
)

// SubagentSourceBuiltin marks a tenant_subagents row as synced from the
// library shipped under config/subagents/library. The column exists so later
// sources (tenant-authored roles) can coexist with builtin ones.
const SubagentSourceBuiltin = "builtin"

// TenantSubagentEntity is one builtin sub-agent role copied into one tenant's
// durable storage: the (slug, locale) pair, with the role markdown stored
// verbatim. The copy is what the runtime resolves against, so shipping an
// updated library never changes a tenant's roles until a sync writes new
// rows.
type TenantSubagentEntity struct {
	// ID is the row key, stable across reinstalls of the same slot: upserts
	// conflict on (tenant_id, slug, locale) and keep the existing ID.
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"primaryKey"`
	// Slug is the role's filename stem in the library, shared across locales.
	Slug string `gorm:"type:varchar(255);not null"`
	// Locale is "zh" or "en"; one row exists per locale a role ships in.
	Locale string `gorm:"type:varchar(8);not null"`
	// Content is the full role markdown verbatim, frontmatter fence included.
	// The body below the fence becomes the sub-run system prompt.
	Content string `gorm:"type:text;not null"`
	// Division is the division directory name under the locale root.
	Division string `gorm:"type:varchar(64)"`
	// Source records where the role came from; builtin for library roles.
	Source string `gorm:"type:varchar(32)"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (e *TenantSubagentEntity) TableName() string { return "tenant_subagents" }
