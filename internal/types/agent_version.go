package types

import (
	"time"
)

// AgentVersionEntity is one immutable, append-only frozen snapshot of a
// tenant-scoped CustomAgent (T28 Task 1, Agent domain). Freezing copies the
// agent's state — never a pointer to the live row — so later edits (or
// deletion) of the source agent never rewrite history. The row is the fixed
// source the tenant Marketplace later references by ID and digest; the
// Marketplace stores that reference but never owns or mutates this row.
//
// Immutability is structural: there is no UpdatedAt (nothing may change) and
// no DeletedAt (nothing may be soft-deleted back into existence), and the
// repository exposes only append/read operations. The (tenant_id, agent_id,
// version_number) unique index enforced by the twin migrations is the hard
// guarantee that version numbers never collide or rewind.
type AgentVersionEntity struct {
	// ID is the immutable version identifier the Marketplace references.
	ID       string `gorm:"type:varchar(36);primaryKey"`
	TenantID uint64 `gorm:"primaryKey"`
	// AgentID is custom_agents.id of the source agent at freeze time.
	AgentID string `gorm:"type:varchar(36);not null"`
	// VersionNumber is the 1-based, monotonically increasing sequence
	// number within one (tenant, agent) scope.
	VersionNumber int `gorm:"not null"`
	// Snapshot is the canonical JSON serialization of the frozen
	// types.CustomAgent. Stored as text — not a JSON-normalizing type — so
	// the bytes read back are the exact bytes hashed at freeze time.
	Snapshot string `gorm:"type:text;not null"`
	// SourceSHA256 is the SHA-256 hex digest over the Snapshot bytes at
	// freeze time; reads verify against it to prove immutability.
	SourceSHA256 string `gorm:"type:varchar(64);not null"`
	// FrozenBy records the freezing user id (empty means a system/API-key
	// principal).
	FrozenBy string `gorm:"type:varchar(255);not null;default:''"`

	CreatedAt time.Time
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (AgentVersionEntity) TableName() string { return "agent_versions" }
