package types

// PluginManifest is the weknora.plugin/1 protocol document served by the
// plugin developer at a stable URL. It describes ONE version; the endpoint
// must stay reachable for that version's lifetime (spec: developers keep old
// version endpoints available). A manifest is review material, never an
// execution grant.
type PluginManifest struct {
	Protocol      string           `json:"protocol"`
	PluginID      string           `json:"plugin_id"`
	Version       string           `json:"version"`
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Transport     PluginTransport  `json:"transport"`
	Auth          *PluginAuth      `json:"auth,omitempty"`
	Tools         []PluginToolDecl `json:"tools"`
	ContentDigest string           `json:"content_digest,omitempty"`
}

type PluginTransport struct {
	Type     string `json:"type"`     // "http-streamable" | "sse"
	Endpoint string `json:"endpoint"` // version-pinned MCP endpoint
}

type PluginAuth struct {
	PersonalOAuth bool     `json:"personal_oauth"`
	Scopes        []string `json:"scopes,omitempty"`
}

type PluginToolDecl struct {
	Name                 string   `json:"name"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes,omitempty"`
	// InputSchemaDigest is the plugin developer's self-reported canonical-JSON
	// SHA-256 of the tool input schema: 64 lowercase hex chars, the same digest
	// convention as OCSchemaDigest/MCPConfigFingerprint. Canonical form:
	// object keys sorted, no insignificant whitespace, number literals kept
	// VERBATIM (1, 1.0, 1e2 are different documents — NOT RFC 8785 JCS; see
	// plugins.CanonicalJSON). It is only used for consistency checks against
	// the LIVE ListTools result — the authoritative snapshot digest is always
	// recomputed from the live endpoint.
	InputSchemaDigest string `json:"input_schema_digest"`
}

// PluginToolSnapshot is the authoritative capability record the tenant
// accepted at install/upgrade time. Schema digests are recomputed from the
// LIVE ListTools result, never copied from the manifest self-report.
type PluginToolSnapshot struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	InputSchemaDigest    string   `json:"input_schema_digest"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes,omitempty"`
}
