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
