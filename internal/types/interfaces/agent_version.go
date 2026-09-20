package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentVersionService is the Agent domain's immutable Tenant agent version
// surface (T28 Wave 1). Freezing copies one tenant-scoped CustomAgent into
// an append-only snapshot — the fixed source the tenant Marketplace later
// references by immutable version ID and source digest. The Marketplace
// stores that reference; it never owns or mutates the version row. All
// reads are tenant-scoped: a version another tenant froze reads as
// not-found, never data.
type AgentVersionService interface {
	// FreezeAgentVersion appends the next immutable version of the agent.
	// The agent must live in the caller's tenant (the tenant-scoped
	// GetAgentByIDAndTenant read), else not-found. It returns the stored
	// view: immutable ID, allocated version number and the canonical source
	// digest.
	FreezeAgentVersion(ctx context.Context, tenantID uint64, actorID, agentID string) (AgentVersionView, error)

	// GetAgentVersion returns one frozen version with its decoded agent
	// snapshot, digest-verified against the stored bytes. Unknown and
	// other-tenant version ids are not-found.
	GetAgentVersion(ctx context.Context, tenantID uint64, versionID string) (AgentVersionSnapshot, error)

	// ListAgentVersions returns every frozen version of one agent inside
	// the tenant, ascending by version number. Another tenant's agent has
	// no versions here.
	ListAgentVersions(ctx context.Context, tenantID uint64, agentID string) ([]AgentVersionView, error)
}

// AgentVersionView is the frozen version as the API answers it: the
// immutable reference (ID + source digest) the Marketplace stores.
type AgentVersionView struct {
	ID            string    `json:"id"`
	AgentID       string    `json:"agent_id"`
	VersionNumber int       `json:"version_number"`
	SourceSHA256  string    `json:"source_sha256"`
	FrozenBy      string    `json:"frozen_by"`
	CreatedAt     time.Time `json:"created_at"`
}

// AgentVersionSnapshot is the full read model: the view plus the decoded
// CustomAgent frozen at freeze time. It never reflects later edits to the
// live agent.
type AgentVersionSnapshot struct {
	AgentVersionView
	// Agent is the CustomAgent decoded from the immutable snapshot bytes.
	Agent *types.CustomAgent `json:"agent"`
}
