package interfaces

import (
	"context"
	"time"
)

// TenantExpertMarketService exposes the tenant-internal expert market to
// the API layer (M4 Task 5): a member exports one of their agents as an
// immutable expert snapshot every member of the SAME tenant can browse and
// install into their own agent via the M2 ExpertService instantiation.
// The snapshot follows the M2 §5 whitelist (persona/system prompt, skill
// references, subagents, starters) — KB bindings, model keys, sandbox
// bindings and memory never leave the publisher's workspace.
type TenantExpertMarketService interface {
	// PublishAgentExpert exports the agent as a materialized-expert
	// snapshot (written atomically into the tenant's published-experts
	// root) and upserts the publish row. Re-publishing the same agent
	// refreshes the same row with a new snapshot (idempotent slot). An
	// agent the tenant does not have is a 404.
	PublishAgentExpert(
		ctx context.Context, tenantID uint64, agentID, publishedBy string, req PublishAgentExpertRequest,
	) (*PublishedExpertView, error)

	// UnpublishExpert soft-deletes the publish row addressed by its row id;
	// the snapshot stays on disk (immutable history). An unknown id is a 404.
	UnpublishExpert(ctx context.Context, tenantID uint64, publishedID string) error

	// ListPublishedExperts serves the tenant market listing: every
	// published expert with the metadata recorded at publish, the
	// publisher's display name and whether this workspace already installed
	// it (an expert_installs row on the snapshot's manifest slug).
	ListPublishedExperts(ctx context.Context, tenantID uint64) (*PublishedExpertIndex, error)

	// InstallPublishedExpert seeds the tenant's installed-experts root from
	// the row's snapshot (digest-verified), records the expert_installs
	// row and instantiates the agent through the M2 ExpertService.
	// Unpublished or unknown experts are a 404.
	InstallPublishedExpert(
		ctx context.Context, tenantID uint64, publishedID string, req InstantiateRequest,
	) (*InstantiateResult, error)
}

// PublishAgentExpertRequest is the optional publish-body override: the
// display name/description recorded on the publish row and the snapshot
// manifest instead of the agent's own values.
type PublishAgentExpertRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PublishedExpertView is the publish row as the API answers it.
type PublishedExpertView struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agent_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	SnapshotSHA256 string    `json:"snapshot_sha256"`
	PublishedBy    string    `json:"published_by"`
	PublishedAt    time.Time `json:"published_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PublishedExpertEntry is one row of the tenant market listing.
type PublishedExpertEntry struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	PublisherName string    `json:"publisher_name"`
	Installed     bool      `json:"installed"`
	CreatedAt     time.Time `json:"created_at"`
}

// PublishedExpertIndex is the listing answer.
type PublishedExpertIndex struct {
	Experts []PublishedExpertEntry `json:"experts"`
}
