package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
)

// ExpertService exposes the expert-template library (see
// internal/agent/experts) to the API layer and instantiates tenant agents
// from it via CustomAgentService, so tenant scoping, defaults and
// persistence match every other agent-creation path.
//
// Read methods return the shared, process-cached experts owned by
// experts.LoadBuiltinExperts — callers must treat them as read-only.
type ExpertService interface {
	// ListExperts returns every available expert, ordered by directory name.
	ListExperts(ctx context.Context) ([]*experts.Expert, error)

	// GetExpert returns one expert by manifest ID. Implementations return
	// the service package's ErrExpertNotFound sentinel when no expert
	// matches.
	GetExpert(ctx context.Context, id string) (*experts.Expert, error)

	// Instantiate creates a tenant-owned custom agent from an expert
	// template. Locale-sensitive fields (name, description, starter
	// prompts) are resolved from ctx; req.AgentName, when set, overrides
	// the locale-resolved label, and req.SandboxConfigID, when set, points
	// the agent's skill scripts at a workspace sandbox config.
	//
	// The created agent carries Config.ExpertSource provenance; the result
	// also reports any skills that were installed (IDs) or left pending
	// (slugs) while assembling the agent.
	Instantiate(ctx context.Context, tenantID uint64, expertID string, req InstantiateRequest) (*InstantiateResult, error)
}

// InstantiateRequest is the optional per-instantiation customization. Every
// field is optional: an empty AgentName falls back to the expert's
// locale-resolved label, and an empty SandboxConfigID leaves sandbox
// execution disabled (the agent default).
type InstantiateRequest struct {
	// AgentName overrides the expert's locale-resolved label.
	AgentName string `json:"agent_name"`
	// SandboxConfigID selects the sandbox config for skill execution.
	SandboxConfigID string `json:"sandbox_config_id"`
}

// InstantiateResult reports the outcome of an expert instantiation.
type InstantiateResult struct {
	// Agent is the created custom agent (with ExpertSource provenance set).
	Agent *types.CustomAgent `json:"agent"`
	// PendingSkills lists bundled skill slugs whose installation was kicked
	// off but has not completed yet.
	PendingSkills []string `json:"pending_skills,omitempty"`
	// SkillInstallIDs identifies the install jobs started for the expert's
	// bundled skills, for status polling.
	SkillInstallIDs []string `json:"skill_install_ids,omitempty"`
}
