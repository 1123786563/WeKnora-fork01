package interfaces

import (
	"context"
)

// SkillMarketService exposes the remote SkillHub marketplace to the API layer
// (M4 skill market): skill search/rankings with cached stale-tolerance, remote
// skill installs through the tenant catalog, the skillset (expert) index, and
// the skillset-to-expert install flow that materializes a skillset and
// instantiates it as a tenant agent.
//
// Listing methods answer 200 with Stale=true when the cached wrapper served
// the last good value after a failed upstream refresh, and map an unreachable
// upstream with no cached value onto a 503 app error — staleness is never
// silent and unreachability is never a plain 500.
type SkillMarketService interface {
	// SearchMarketSkills queries the registry; limit is clamped by the
	// client to [1, 100].
	SearchMarketSkills(ctx context.Context, query string, limit int) (*SkillMarketListing, error)

	// MarketSkillRankings fetches one showcase list; kind must be one of
	// hot|featured|newest|recommended|trending|paid (anything else is a 400).
	MarketSkillRankings(ctx context.Context, kind string) (*SkillMarketListing, error)

	// InstallMarketSkill downloads the slug's package, validates it, records
	// it in the tenant catalog and installs it onto the named sandbox
	// configs. Partial per-config failures answer with Errors filled in.
	InstallMarketSkill(ctx context.Context, tenantID uint64, slug string, sandboxConfigIDs []string) (*MarketSkillInstallResult, error)

	// ListMarketSkillsets serves the skillset index for the tenant, marking
	// the slugs that already have an install-ledger row.
	ListMarketSkillsets(ctx context.Context, tenantID uint64) (*MarketSkillsetIndex, error)

	// GetMarketSkillset serves one skillset detail; unknown slugs are a 404.
	GetMarketSkillset(ctx context.Context, tenantID uint64, slug string) (*MarketSkillsetDetail, error)

	// InstallMarketSkillset materializes the skillset as a tenant expert
	// (write + ledger upsert) and instantiates it as a custom agent through
	// the M2 ExpertService.
	InstallMarketSkillset(ctx context.Context, tenantID uint64, slug string, req InstantiateRequest) (*MarketSkillsetInstallResult, error)
}

// SkillMarketResult is one marketplace skill listing row: the typed fields
// consumers bind plus the stringified registry entry for display passthrough.
type SkillMarketResult struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Raw         map[string]string `json:"raw,omitempty"`
}

// SkillMarketListing is the search/rankings answer.
type SkillMarketListing struct {
	Results []SkillMarketResult `json:"results"`
	Stale   bool                `json:"stale"`
}

// MarketSkillInstallResult is the skill-install outcome: the catalog
// definition the package was registered under, the started install IDs (in
// the order the configs were requested) and any per-config failures.
type MarketSkillInstallResult struct {
	CatalogID  string            `json:"catalog_id"`
	InstallIDs []string          `json:"install_ids"`
	Errors     map[string]string `json:"errors,omitempty"`
}

// MarketSkillset is one skillset index row.
type MarketSkillset struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SkillSlugs  []string `json:"skill_slugs"`
	Installed   bool     `json:"installed"`
}

// MarketSkillsetIndex is the skillset listing answer.
type MarketSkillsetIndex struct {
	Skillsets []MarketSkillset `json:"skillsets"`
	Stale     bool             `json:"stale"`
}

// MarketSkillsetDetail is one skillset's detail view: the index row plus the
// registry's English metadata (the same fields materialization consumes).
type MarketSkillsetDetail struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	NameEn        string   `json:"name_en,omitempty"`
	DescriptionEn string   `json:"description_en,omitempty"`
	SkillSlugs    []string `json:"skill_slugs"`
	Installed     bool     `json:"installed"`
	Stale         bool     `json:"stale"`
}

// MarketSkillsetInstallResult is the skillset-install outcome: the M2
// instantiation result (agent + pending skills + install jobs) plus the
// materialized expert's manifest ID and content digest.
type MarketSkillsetInstallResult struct {
	InstantiateResult
	// ExpertID is the materialized expert's manifest ID
	// (skillhub-skillset-<slug>).
	ExpertID string `json:"expert_id"`
	// SnapshotSHA256 is the content digest of the materialized tree.
	SnapshotSHA256 string `json:"snapshot_sha256,omitempty"`
}
