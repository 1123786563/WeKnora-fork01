package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// TenantSkillMarketService exposes the tenant-internal skill market to the
// API layer (M4 Task 4): admins publish workspace catalog skills to a
// market every member of the SAME tenant can browse, and members install a
// published skill onto sandbox configs through the existing catalog install
// pipeline. Publisher and installer share the tenant's catalog rows, so
// publishing is a visibility flag over the shared definition — never a
// content copy — and install never re-registers the bundle.
type TenantSkillMarketService interface {
	// PublishSkill flags one catalog skill as market-visible. Re-publishing
	// is idempotent: the same row is refreshed (publisher + timestamp) and
	// returned. A catalog the tenant does not have is a 404.
	PublishSkill(ctx context.Context, tenantID uint64, catalogID, publishedBy string) (*PublishedSkillView, error)

	// UnpublishSkill soft-deletes the publish row. Unpublishing a skill that
	// is not published is an idempotent no-op while the catalog exists; a
	// catalog the tenant never had is a 404. An orphaned row (catalog
	// deleted after publishing) is still cleaned up.
	UnpublishSkill(ctx context.Context, tenantID uint64, catalogID string) error

	// ListPublishedSkills serves the tenant market listing: every published
	// skill with its definition metadata, the publisher's display name and
	// whether any sandbox of this workspace already runs it. Publish rows
	// whose catalog definition is gone are skipped.
	ListPublishedSkills(ctx context.Context, tenantID uint64) (*PublishedSkillIndex, error)

	// InstallPublishedSkill installs a published skill onto the named
	// sandbox configs through the catalog install pipeline. Unpublished or
	// unknown skills are a 404; per-config failures ride the accepted
	// result's Errors (the catalog-install contract).
	InstallPublishedSkill(ctx context.Context, tenantID uint64, catalogID string, sandboxConfigIDs []string) (*TenantSkillInstallResult, error)
}

// PublishedSkillView is the publish row as the API answers it.
type PublishedSkillView struct {
	CatalogID   string    `json:"catalog_id"`
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PublishedSkillEntry is one row of the tenant market listing.
type PublishedSkillEntry struct {
	CatalogID     string `json:"catalog_id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Version       string `json:"version,omitempty"`
	PublisherName string `json:"publisher_name"`
	Installed     bool   `json:"installed"`
}

// PublishedSkillIndex is the listing answer.
type PublishedSkillIndex struct {
	Skills []PublishedSkillEntry `json:"skills"`
}

// TenantSkillInstallResult is the per-sandbox outcome of one tenant-market
// install — the catalog-install answer shape re-declared for the interface
// boundary (partial failures ride Errors on an accepted result).
type TenantSkillInstallResult struct {
	Installs map[string]string `json:"installs"`
	Errors   map[string]string `json:"errors,omitempty"`
}

// TenantSkillPublisherNames resolves publisher user ids to display names;
// interfaces.UserRepository satisfies it.
type TenantSkillPublisherNames interface {
	GetUsersByIDs(ctx context.Context, ids []string) (map[string]*types.User, error)
}
