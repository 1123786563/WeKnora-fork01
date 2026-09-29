package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// AgentAdoptionService coordinates the Tenant Adoption and Variant
// governance workflow (docs/specs/2026-09-20-agent-marketplace-domain-model.md
// §8). HTTP authorization remains the route boundary; Tenant and actor
// identity always arrive from the authenticated request context.
type AgentAdoptionService interface {
	// Adopt accepts a Listing (and by default its current Release) for the
	// Tenant. Idempotent per (tenant, listing); the bool reports whether the
	// Adoption row was created by this call.
	Adopt(ctx context.Context, tenantID uint64, actorID string, input AdoptInput) (AdoptionView, bool, error)
	ListAdoptions(ctx context.Context, tenantID uint64) ([]AdoptionView, error)
	CreateVariant(ctx context.Context, tenantID uint64, actorID, adoptionID string, input VariantDraftInput) (AdoptionVariantView, error)
	UpdateCapabilityMapping(ctx context.Context, tenantID uint64, actorID, variantID string, mappings []CapabilityMapping) (AdoptionVariantView, error)
	TestVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (AdoptionVariantView, error)
	PublishVariant(ctx context.Context, tenantID uint64, actorID, variantID string) (PublishVariantResult, error)
	ListAvailableAgents(ctx context.Context, tenantID uint64) ([]AvailableAgentView, error)
}

type AdoptInput struct{ ListingID, ReleaseID string }

type VariantDraftInput struct{ Name, ReleaseID string }

// CapabilityMapping is one local binding of a Manifest capability
// requirement: a local model, knowledge bases and/or connections. At least
// one non-empty binding must be present for the capability to count as
// covered.
type CapabilityMapping struct {
	Capability       string
	ModelID          string
	KnowledgeBaseIDs []string
	ConnectionIDs    []string
}

type AdoptionView struct {
	types.AgentAdoptionEntity
	Variants []AdoptionVariantView `json:"variants"`
}

type AdoptionVariantView struct {
	types.AgentAdoptionVariantEntity
	// MissingCapabilities names every Manifest capability requirement that
	// no mapping binds to a local resource, sorted for determinism.
	MissingCapabilities []string `json:"missing_capabilities"`
}

type PublishVariantResult struct {
	Variant AdoptionVariantView `json:"variant"`
}

type AgentCapabilityVerdict struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// AvailableAgentView is the adoption-aware mobile read model row: one
// published Variant's live local agent. Field names mirror the wire the
// mobile Resource Shelf already consumes (GET /api/v1/agents rows) plus the
// governance lineage ids.
type AvailableAgentView struct {
	AgentID     string                 `json:"agent_id"`
	VariantID   string                 `json:"variant_id"`
	AdoptionID  string                 `json:"adoption_id"`
	ReleaseID   string                 `json:"release_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	IsBuiltin   bool                   `json:"is_builtin"`
	Capability  AgentCapabilityVerdict `json:"capability"`
}
