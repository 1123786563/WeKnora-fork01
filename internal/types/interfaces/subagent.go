package interfaces

import (
	"context"
)

// SubagentService exposes the builtin sub-agent role catalog (see
// internal/agent/subagents) and the per-agent install/remove operations to
// the API layer. Catalog reads are static views over the shared,
// process-cached library plus the tenant's installed rows; the agent-scoped
// mutations go through CustomAgentService so tenant scoping and persistence
// match every other agent-config change.
type SubagentService interface {
	// ListCatalog returns the whole role library for one tenant's view:
	// divisions in divisions.json order with per-division entry counts, the
	// slug-sorted entries, and Installed flags derived from the tenant's
	// tenant_subagents rows.
	ListCatalog(ctx context.Context, tenantID uint64) (*SubagentCatalog, error)

	// GetCatalogEntry returns one role's detail projection: the full
	// per-locale bodies (present-or-empty) plus the frontmatter scalars and
	// the tenant's installed state. Implementations return the service
	// package's ErrSubagentNotFound sentinel when the slug is unknown.
	GetCatalogEntry(ctx context.Context, tenantID uint64, slug string) (*SubagentCatalogDetail, error)

	// ListAgentSubagents returns the agent's configured subagent slugs,
	// config order preserved. Implementations return the service package's
	// ErrAgentNotFound sentinel when the agent is unknown.
	ListAgentSubagents(ctx context.Context, agentID string) ([]string, error)

	// InstallForAgent resolves slug through the catalog (locale — the
	// requested locale, then en, then zh), upserts the resolved role into the
	// tenant's tenant_subagents rows (content verbatim, frontmatter fence
	// included, fresh row ID), and appends the slug to the agent's
	// Config.Subagents when absent. Idempotent: reinstalling an already
	// configured slug refreshes the row and leaves the config untouched.
	InstallForAgent(ctx context.Context, tenantID uint64, agentID, slug, locale string) ([]string, error)

	// RemoveFromAgent removes slug from the agent's Config.Subagents. The
	// tenant row is KEPT: rows are shared across agents and outlive any one
	// agent's configuration; deleting rows is tenant-admin scope, not this
	// operation. Absent slugs are a no-op.
	RemoveFromAgent(ctx context.Context, agentID, slug string) ([]string, error)
}

// SubagentCatalogEntry is the list projection of one role: the fields a
// gallery card needs, with both locales' names carried so presentation-side
// locale filtering stays client-side. An absent locale is an empty string,
// never omitted.
type SubagentCatalogEntry struct {
	Slug      string `json:"slug"`
	Division  string `json:"division"`
	NameZh    string `json:"name_zh"`
	NameEn    string `json:"name_en"`
	Emoji     string `json:"emoji"`
	Color     string `json:"color"`
	Installed bool   `json:"installed"`
}

// SubagentCatalogDivision is one division of the library with its entry
// count (roles whose division matches, any locale).
type SubagentCatalogDivision struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
	Count int    `json:"count"`
}

// SubagentCatalog is the list payload: divisions in divisions.json order,
// the total number of distinct slugs, and the slug-sorted entries.
type SubagentCatalog struct {
	Divisions []SubagentCatalogDivision `json:"divisions"`
	Total     int                       `json:"total"`
	Entries   []SubagentCatalogEntry    `json:"entries"`
}

// SubagentCatalogDetail is the detail projection of one role: frontmatter
// scalars (emoji/color/vibe/tools resolved from the zh definition, falling
// back to en for en-only roles) plus BOTH locale bodies, present-or-empty.
type SubagentCatalogDetail struct {
	Slug      string `json:"slug"`
	Division  string `json:"division"`
	NameZh    string `json:"name_zh"`
	NameEn    string `json:"name_en"`
	Emoji     string `json:"emoji"`
	Color     string `json:"color"`
	Vibe      string `json:"vibe"`
	ToolsRaw  string `json:"tools_raw"`
	BodyZh    string `json:"body_zh"`
	BodyEn    string `json:"body_en"`
	Installed bool   `json:"installed"`
}
