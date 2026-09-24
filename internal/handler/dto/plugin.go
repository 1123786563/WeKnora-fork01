package dto

import (
	"time"
)

// PluginPreviewRequest is the body of POST /plugins/installations/preview:
// the manifest URL the admin pasted. It is the only input the endpoint takes.
// max=512 aligns with plugin_previews.manifest_url varchar(512) (runes, the
// same unit PostgreSQL varchar uses): an oversized URL must die here as a
// 400, not after a full fetch-and-verify round trip as a 500 column overflow.
type PluginPreviewRequest struct {
	ManifestURL string `json:"manifest_url" binding:"required,max=512"`
}

// PluginPreviewTool is one row of the admin's review table: the tool's
// metadata and governance classification from the VERIFIED snapshot. The
// input schema itself is never echoed — only the fact that its digest was
// verified (a digest mismatch rejects the whole preview upstream), so
// untrusted remote schema text cannot reach the admin surface.
type PluginPreviewTool struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes"`
}

// PluginPreviewResponse is the preview payload the admin reviews before
// confirming an installation: identity, version-pinned endpoint, verified
// tool directory, identity fingerprint and the preview's expiry. No
// credentials ever appear here — by construction this type has none.
type PluginPreviewResponse struct {
	PreviewID           string              `json:"preview_id"`
	PluginID            string              `json:"plugin_id"`
	Version             string              `json:"version"`
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	TransportType       string              `json:"transport_type"`
	EndpointURL         string              `json:"endpoint_url"`
	Tools               []PluginPreviewTool `json:"tools"`
	IdentityFingerprint string              `json:"identity_fingerprint"`
	ExpiresAt           time.Time           `json:"expires_at"`
}

// PluginInstallConfirmRequest is the body of POST /plugins/installations:
// the preview ID the admin reviewed. Confirming consumes the preview.
type PluginInstallConfirmRequest struct {
	PreviewID string `json:"preview_id" binding:"required"`
}

// PluginInstallationTool is one row of an installation's tool directory.
// The input schema itself is never echoed — only metadata and the policy
// verdict (enabled: omitted = no explicit row / unknown).
type PluginInstallationTool struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes"`
	Enabled              *bool    `json:"enabled,omitempty"`
}

// PluginInstallationResponse is the full installation payload (confirm,
// state change, get-by-id).
type PluginInstallationResponse struct {
	InstallationID string                   `json:"installation_id"`
	PluginID       string                   `json:"plugin_id"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Version        string                   `json:"version"`
	State          string                   `json:"state"`
	DriftState     string                   `json:"drift_state"`
	TransportType  string                   `json:"transport_type"`
	EndpointURL    string                   `json:"endpoint_url"`
	ServiceID      string                   `json:"service_id"`
	Tools          []PluginInstallationTool `json:"tools"`
}

// PluginInstallationSummary is one row of the member-facing list.
type PluginInstallationSummary struct {
	InstallationID       string `json:"installation_id"`
	PluginID             string `json:"plugin_id"`
	Name                 string `json:"name"`
	Version              string `json:"version"`
	State                string `json:"state"`
	DriftState           string `json:"drift_state"`
	RequiresPersonalAuth bool   `json:"requires_personal_auth"`
	ToolCount            int    `json:"tool_count"`
}

// PluginMyConnection is the member's personal connection view of one
// installation (GET /plugins/installations/:id/connections/me): identity,
// the materialized service binding, the three-state OAuth verdict
// (authorized/expired/unauthorized) and the legacy MCP OAuth endpoint paths
// mapped onto that service_id — the frontend drives authorize/revoke through
// those existing endpoints. No token material ever appears here — by
// construction this type has none.
type PluginMyConnection struct {
	InstallationID       string   `json:"installation_id"`
	PluginID             string   `json:"plugin_id"`
	Name                 string   `json:"name"`
	ServiceID            string   `json:"service_id"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Authorized           bool     `json:"authorized"`
	State                string   `json:"state"`
	AuthorizeURLPath     string   `json:"authorize_url_path"`
	RevokePath           string   `json:"revoke_path"`
	RequiresAuthTools    []string `json:"requires_auth_tools"`
}
