package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

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

// PluginPreviewTools is the JSON column shape of plugin_previews.
// tools_snapshot (JSONB on PostgreSQL, TEXT on the SQLite twin): the verified
// tool directory the admin reviewed, exactly as FetchAndVerify computed it.
type PluginPreviewTools []PluginToolSnapshot

// Value implements driver.Valuer for PluginPreviewTools (JSON column).
func (t PluginPreviewTools) Value() (driver.Value, error) {
	if t == nil {
		return nil, nil
	}
	return json.Marshal(t)
}

// Scan implements sql.Scanner for PluginPreviewTools (JSON column). Both
// []byte (PostgreSQL JSONB) and string (SQLite TEXT — the column is TEXT on
// the SQLite twin, whose drivers hand TEXT columns back as string) are
// accepted; any other type is an error, never silently swallowed — the same
// contract as types.JSON.Scan.
func (t *PluginPreviewTools) Scan(value interface{}) error {
	if value == nil {
		*t = nil
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte or string failed")
	}
	if len(b) == 0 {
		*t = nil
		return nil
	}
	return json.Unmarshal(b, t)
}

// PluginPreviewToolReview is one row of the admin's review table, as the
// service layer returns it: the tool's metadata and governance classification
// from the VERIFIED snapshot. The input schema itself is never included —
// only the fact that its digest was verified (a digest mismatch rejects the
// whole preview upstream).
type PluginPreviewToolReview struct {
	Name                 string
	Description          string
	ReadOnly             bool
	RequiresPersonalAuth bool
	Scopes               []string
}

// PluginPreviewResult is the preview payload the admin reviews before
// confirming an installation: identity, version-pinned endpoint, verified
// tool directory, identity fingerprint and the preview's expiry. It is the
// types-layer service contract; the HTTP handler maps it onto its DTO.
// No credentials ever appear here — by construction this type has none.
type PluginPreviewResult struct {
	PreviewID           string
	PluginID            string
	Version             string
	Name                string
	Description         string
	TransportType       string
	EndpointURL         string
	Tools               []PluginPreviewToolReview
	IdentityFingerprint string
	ExpiresAt           time.Time
}

// PluginPreview persists one verified manifest preview: the admin's review
// artifact between "pasted a manifest URL" and "confirmed an installation".
// It is TTL-bound and single-use — installation confirm (migration 000190)
// consumes it exactly once; Expired() treats a consumed preview as expired.
// A preview is review material, never an execution grant.
type PluginPreview struct {
	ID                  string             `json:"id"                   gorm:"type:varchar(36);primaryKey"`
	TenantID            uint64             `json:"tenant_id"            gorm:"not null;index:idx_plugin_previews_tenant,priority:1"`
	ManifestURL         string             `json:"manifest_url"         gorm:"type:varchar(512);not null"`
	PluginID            string             `json:"plugin_id"            gorm:"type:varchar(128);not null;index:idx_plugin_previews_tenant,priority:2"`
	Version             string             `json:"version"              gorm:"type:varchar(64);not null"`
	Name                string             `json:"name"                 gorm:"type:varchar(255);not null"`
	TransportType       string             `json:"transport_type"       gorm:"type:varchar(50);not null"`
	EndpointURL         string             `json:"endpoint_url"         gorm:"type:varchar(512);not null"`
	ToolsSnapshot       PluginPreviewTools `json:"tools_snapshot"       gorm:"type:json;not null"`
	ToolsDigest         string             `json:"tools_digest"         gorm:"type:varchar(64);not null"`
	IdentityFingerprint string             `json:"identity_fingerprint" gorm:"type:varchar(64);not null"`
	CreatedBy           string             `json:"created_by"           gorm:"type:varchar(255);not null"`
	ExpiresAt           time.Time          `json:"expires_at"`
	ConsumedAt          *time.Time         `json:"consumed_at,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
}

// Expired reports whether this preview can no longer be confirmed against.
// A preview is dead when its TTL has passed OR when it was already consumed
// — consumption is one-shot, so "already used" is the same verdict as
// "too late". The boundary is inclusive: ExpiresAt == now is expired.
func (p *PluginPreview) Expired(now time.Time) bool {
	return p.ConsumedAt != nil || !now.Before(p.ExpiresAt)
}

// PluginInstallation states. active: the accepted version is live and its
// materialized MCP service is enabled. disabled: the admin paused the
// installation — the materialized service flips Enabled=false, which removes
// it from the agent-visible MCP directory on the next registration pass.
const (
	PluginInstallationActive   = "active"
	PluginInstallationDisabled = "disabled"
)

// PluginInstallation drift states. The runtime verification baseline is the
// accepted tools snapshot; drift handling lands with the drift slice (T17) —
// the column and constants are fixed here so the migration and model agree
// from day one.
const (
	PluginDriftNone     = "none"
	PluginDriftDetected = "detected"
)

// PluginInstallation persists ONE accepted version of a plugin for ONE
// tenant: the tenant's runtime verification baseline (accepted_version +
// tools_snapshot + tools_digest), the long-lived manifest source
// (manifest_url, copied from the consumed preview at confirm time — the
// preview row itself is TTL-bound and single-use, while upgrades re-fetch
// from this URL), and the materialized MCP service binding (service_id).
// Migration 000190 writes NO approval rows: legacy manual tools are never
// auto-promoted into approved plugin versions.
type PluginInstallation struct {
	ID              string             `json:"id"               gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64             `json:"tenant_id"        gorm:"not null;uniqueIndex:uq_plugin_installations_tenant_plugin,priority:1"`
	PluginID        string             `json:"plugin_id"        gorm:"type:varchar(128);not null;uniqueIndex:uq_plugin_installations_tenant_plugin,priority:2"`
	Name            string             `json:"name"             gorm:"type:varchar(255);not null"`
	Description     string             `json:"description"      gorm:"type:text;not null;default:''"`
	ManifestURL     string             `json:"manifest_url"     gorm:"type:varchar(512);not null"`
	AcceptedVersion string             `json:"accepted_version" gorm:"type:varchar(64);not null"`
	TransportType   string             `json:"transport_type"   gorm:"type:varchar(50);not null"`
	EndpointURL     string             `json:"endpoint_url"     gorm:"type:varchar(512);not null"`
	ToolsSnapshot   PluginPreviewTools `json:"tools_snapshot"   gorm:"type:json;not null"`
	ToolsDigest     string             `json:"tools_digest"     gorm:"type:varchar(64);not null"`
	ServiceID       string             `json:"service_id"       gorm:"type:varchar(36);not null;default:''"`
	DriftState      string             `json:"drift_state"      gorm:"type:varchar(16);not null;default:'none'"`
	DriftDetail     json.RawMessage    `json:"drift_detail,omitempty" gorm:"type:json"`
	State           string             `json:"state"            gorm:"type:varchar(16);not null;default:'active'"`
	CreatedBy       string             `json:"created_by"       gorm:"type:varchar(255);not null"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// PluginInstallationToolView is one row of an installation's tool directory
// as the service returns it: snapshot metadata plus the CURRENT policy
// verdict when one is known (Enabled: true=exposed, false=disabled — write
// tools install as false per B5; nil=no explicit row, legacy default
// enabled). The input schema itself is never included — schema text is
// untrusted remote data and only its verified digest is meaningful here.
type PluginInstallationToolView struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	ReadOnly             bool     `json:"read_only"`
	RequiresPersonalAuth bool     `json:"requires_personal_auth"`
	Scopes               []string `json:"scopes"`
	Enabled              *bool    `json:"enabled,omitempty"`
}

// PluginInstallationResult is the full installation payload (confirm, state
// change, get-by-id): identity, accepted version, materialized service
// binding and the tool directory. Types-layer service contract per the F2
// ruling — the handler maps it onto its DTO.
type PluginInstallationResult struct {
	InstallationID string                       `json:"installation_id"`
	PluginID       string                       `json:"plugin_id"`
	Name           string                       `json:"name"`
	Description    string                       `json:"description"`
	Version        string                       `json:"version"`
	State          string                       `json:"state"`
	DriftState     string                       `json:"drift_state"`
	TransportType  string                       `json:"transport_type"`
	EndpointURL    string                       `json:"endpoint_url"`
	ServiceID      string                       `json:"service_id"`
	Tools          []PluginInstallationToolView `json:"tools"`
}

// PluginInstallationSummary is one row of the member-facing installation
// list: no snapshot payload, no endpoint echo — members discover WHAT is
// installed and its state, not the verified directory detail.
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

// Member personal-connection states (T11, GAP-4). The connection view reuses
// the per-principal MCP OAuth storage (mcp_oauth_tokens keyed by
// (tenant, principal, service)); these constants name the member-facing
// verdicts over that storage.
const (
	// PluginConnectionAuthorized: usable now — a valid access token, or an
	// expired one whose refresh token lets the runtime renew under the
	// member's existing consent (oauthRuntime.ensureFresh).
	PluginConnectionAuthorized = "authorized"
	// PluginConnectionExpired: the stored token is past expiry with no
	// refresh token — only a NEW member consent (re-authorization) recovers.
	PluginConnectionExpired = "expired"
	// PluginConnectionUnauthorized: this principal has no token for the
	// materialized service.
	PluginConnectionUnauthorized = "unauthorized"
)

// PluginMyConnection is ONE member's personal connection view of ONE
// installation (GET /plugins/installations/:id/connections/me): identity,
// the materialized service binding, the three-state OAuth verdict and the
// LEGACY MCP OAuth endpoint paths (AuthorizeURLPath/RevokePath map onto the
// materialized service_id — the plugin domain owns no OAuth endpoints of its
// own, GAP-4). No token material appears here — by construction this type
// has none.
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
