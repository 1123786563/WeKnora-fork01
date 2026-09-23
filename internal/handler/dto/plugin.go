package dto

import (
	"time"
)

// PluginPreviewRequest is the body of POST /plugins/installations/preview:
// the manifest URL the admin pasted. It is the only input the endpoint takes.
type PluginPreviewRequest struct {
	ManifestURL string `json:"manifest_url" binding:"required"`
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
