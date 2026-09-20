package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSourceResponse_OmitsCredentials(t *testing.T) {
	cfg := types.DataSourceConfig{
		Type: "github",
		Credentials: map[string]interface{}{
			"token": "ghp-secret-do-not-leak",
		},
		ResourceIDs: []string{"repo-1"},
		Settings:    map[string]interface{}{"branch": "main"},
	}
	blob, _ := cfg.ToJSON()
	ds := &types.DataSource{
		ID:     "ds-1",
		Name:   "github-prod",
		Type:   "github",
		Config: blob,
	}
	body, err := json.Marshal(NewDataSourceResponse(ds))
	assert.NoError(t, err)
	s := string(body)
	assert.NotContains(t, s, "ghp-secret-do-not-leak")
	// The inner config object must not carry the credentials map (the
	// DataSourceConfigDTO type omits it structurally).
	var raw map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(body, &raw))
	if cfgRaw, ok := raw["config"]; ok {
		var inner map[string]json.RawMessage
		assert.NoError(t, json.Unmarshal(cfgRaw, &inner))
		_, hasCredsInConfig := inner["credentials"]
		assert.False(t, hasCredsInConfig,
			"credentials map must not appear inside the config DTO")
	}
	// Top-level credentials map is just the "configured?" indicator,
	// replaces the removed GET /credentials endpoint.
	assert.Contains(t, s, `"credentials":{"credentials":{"configured":true}}`)
	// Non-secret config fields pass through.
	assert.Contains(t, s, "repo-1")
	assert.Contains(t, s, "branch")
	assert.Contains(t, s, "main")
}

func TestDataSourceResponse_NilSafe(t *testing.T) {
	assert.Nil(t, NewDataSourceResponse(nil))
	assert.Equal(t, []*DataSourceResponse{}, NewDataSourceResponses(nil))
}

func TestDataSourceResponse_RSSFeedURLsFromCredentials(t *testing.T) {
	cfg := types.DataSourceConfig{
		Type: types.ConnectorTypeRSS,
		Credentials: map[string]interface{}{
			"feed_urls": "https://example.com/a.xml\nhttps://example.com/b.xml",
		},
		ResourceIDs: []string{"https://example.com/a.xml"},
	}
	blob, _ := cfg.ToJSON()
	ds := &types.DataSource{
		ID:     "ds-rss",
		Name:   "my-rss",
		Type:   types.ConnectorTypeRSS,
		Config: blob,
	}
	resp := NewDataSourceResponse(ds)
	assert.NotNil(t, resp.Config)
	assert.Equal(t, "https://example.com/a.xml\nhttps://example.com/b.xml", resp.Config.Settings["feed_urls"])
	assert.False(t, resp.Credentials["credentials"].Configured)
	body, err := json.Marshal(resp)
	assert.NoError(t, err)
	s := string(body)
	assert.Contains(t, s, "https://example.com/a.xml")
	assert.NotContains(t, s, "Bearer secret")
}

func TestDataSourceResponse_RSSAuthHeadersConfigured(t *testing.T) {
	cfg := types.DataSourceConfig{
		Type: types.ConnectorTypeRSS,
		Credentials: map[string]interface{}{
			"auth_headers": "Authorization: Bearer secret",
		},
		Settings: map[string]interface{}{
			"feed_urls": "https://example.com/feed.xml",
		},
	}
	blob, _ := cfg.ToJSON()
	ds := &types.DataSource{
		ID:     "ds-rss-auth",
		Name:   "my-rss",
		Type:   types.ConnectorTypeRSS,
		Config: blob,
	}
	resp := NewDataSourceResponse(ds)
	assert.True(t, resp.Credentials["credentials"].Configured)
	body, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.NotContains(t, string(body), "Bearer secret")
}

func TestDataSourceResponse_NoConfig(t *testing.T) {
	ds := &types.DataSource{ID: "x", Name: "x"}
	body, err := json.Marshal(NewDataSourceResponse(ds))
	assert.NoError(t, err)
	// No config jsonb stored → no config object in the response.
	assert.NotContains(t, string(body), `"config":`)
}

// ── SP2-b §6.4 credential expiry metadata ───────────────────────────────

func credentialMetadataSource(t *testing.T, expiresAt string) *types.DataSource {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("m", 32))
	creds := map[string]interface{}{
		"access_token":  "test-token-1",
		"refresh_token": "test-token-2",
	}
	if expiresAt != "" {
		creds["expires_at"] = expiresAt
	}
	creds["last_refreshed_at"] = "2026-09-19T10:00:00Z"
	blob, err := (&types.DataSourceConfig{
		Type:        "feishu",
		Credentials: creds,
	}).ToJSON()
	require.NoError(t, err)
	return &types.DataSource{ID: "ds-exp", Type: "feishu", Config: blob}
}

// TestDataSourceCredentialMetadata_NeedsReauthorizationStates pins the three
// derivation states of needs_reauthorization: already expired, expiring
// inside the 7-day window, and long-lived (no expires_at at all).
func TestDataSourceCredentialMetadata_NeedsReauthorizationStates(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name       string
		expiresAt  string
		wantExpiry bool
		wantReauth bool
	}{
		{"expired", now.Add(-time.Hour).Format(time.RFC3339), true, true},
		{"expiring in three days", now.Add(72 * time.Hour).Format(time.RFC3339), true, true},
		{"expiring in a month", now.Add(30 * 24 * time.Hour).Format(time.RFC3339), true, false},
		{"long lived (no expires_at)", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := credentialMetadataSource(t, tc.expiresAt)
			resp := NewDataSourceResponse(ds)
			meta, ok := resp.Credentials["credentials"]
			require.True(t, ok)
			assert.True(t, meta.Configured)
			if tc.wantExpiry {
				require.NotNil(t, meta.ExpiresAt)
				assert.WithinDuration(t, now.Add(parseDurationOrZero(t, tc.expiresAt, now)), *meta.ExpiresAt, time.Minute)
			} else {
				assert.Nil(t, meta.ExpiresAt)
			}
			assert.Equal(t, tc.wantReauth, meta.NeedsReauthorization)
			require.NotNil(t, meta.LastRefreshedAt)
			assert.Equal(t, "2026-09-19T10:00:00Z", meta.LastRefreshedAt.Format(time.RFC3339))
		})
	}
}

// parseDurationOrZero derives the expected offset of an RFC3339 expires_at
// relative to the test's "now" so assertions stay clock-agnostic.
func parseDurationOrZero(t *testing.T, raw string, now time.Time) time.Duration {
	t.Helper()
	if raw == "" {
		return 0
	}
	ts, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	return ts.Sub(now)
}

// TestDataSourceCredentialMetadata_NeverLeaksValues: the metadata is built
// from the DECRYPTED config, so a leak here would expose the raw token —
// assert only bookkeeping fields survive serialization.
func TestDataSourceCredentialMetadata_NeverLeaksValues(t *testing.T) {
	ds := credentialMetadataSource(t, time.Now().Add(72*time.Hour).Format(time.RFC3339))
	body, err := json.Marshal(NewDataSourceResponse(ds))
	assert.NoError(t, err)
	s := string(body)
	assert.NotContains(t, s, "test-token-1")
	assert.NotContains(t, s, "test-token-2")
	assert.Contains(t, s, `"needs_reauthorization":true`)
	assert.Contains(t, s, `"last_refreshed_at":"2026-09-19T10:00:00Z"`)
}

// TestNewDataSourceCredentialFields_Unconfigured keeps the unconfigured
// shape byte-compatible with the pre-SP2-b wire format.
func TestNewDataSourceCredentialFields_Unconfigured(t *testing.T) {
	fields := NewDataSourceCredentialFields("feishu", nil)
	body, err := json.Marshal(fields)
	assert.NoError(t, err)
	assert.Equal(t, `{"credentials":{"configured":false}}`, string(body))
}
