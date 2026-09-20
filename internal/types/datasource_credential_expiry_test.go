package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCredentialsExpiry_LenientISOParse covers the SP2-b §6.1 storage
// protocol: expires_at is a plain ISO timestamp string inside the encrypted
// credentials map, and consumers must tolerate the common variants upstream
// token endpoints emit rather than demanding strict RFC3339.
func TestCredentialsExpiry_LenientISOParse(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Time
	}{
		{"rfc3339 zulu", "2026-10-01T12:30:00Z", time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)},
		{"rfc3339 offset", "2026-10-01T12:30:00+08:00", time.Date(2026, 10, 1, 4, 30, 0, 0, time.UTC)},
		{"rfc3339 fractional", "2026-10-01T12:30:00.25Z", time.Date(2026, 10, 1, 12, 30, 0, 250000000, time.UTC)},
		{"offset without colon", "2026-10-01T12:30:00+0800", time.Date(2026, 10, 1, 4, 30, 0, 0, time.UTC)},
		{"naive datetime assumed utc", "2026-10-01T12:30:00", time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)},
		{"space separator", "2026-10-01 12:30:00", time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)},
		{"date only", "2026-10-01", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &DataSourceConfig{Credentials: map[string]interface{}{
				CredentialKeyExpiresAt: tc.raw,
			}}
			got, ok := CredentialsExpiry(cfg)
			require.True(t, ok, "expected parseable expires_at %q", tc.raw)
			assert.True(t, got.Equal(tc.want), "got %v want %v", got, tc.want)
		})
	}
}

// TestCredentialsExpiry_MissingOrBadMeansLongLived pins the "missing/bad =
// long-lived credential" contract: unknown formats never error out to the
// caller, they simply report no expiry so refresh logic skips them.
func TestCredentialsExpiry_MissingOrBadMeansLongLived(t *testing.T) {
	cases := []struct {
		name string
		cfg  *DataSourceConfig
	}{
		{"nil config", nil},
		{"nil credentials map", &DataSourceConfig{}},
		{"key absent", &DataSourceConfig{Credentials: map[string]interface{}{"token": "test-token-1"}}},
		{"non string value", &DataSourceConfig{Credentials: map[string]interface{}{CredentialKeyExpiresAt: 1790000000}}},
		{"empty string", &DataSourceConfig{Credentials: map[string]interface{}{CredentialKeyExpiresAt: ""}}},
		{"garbage", &DataSourceConfig{Credentials: map[string]interface{}{CredentialKeyExpiresAt: "not-a-timestamp"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CredentialsExpiry(tc.cfg)
			assert.False(t, ok)
			assert.True(t, got.IsZero())
		})
	}
}

// TestParseCredentialTimestamp reads arbitrary timestamp keys (the
// last_refreshed_at bookkeeping key uses the same lenient parser).
func TestParseCredentialTimestamp(t *testing.T) {
	cfg := &DataSourceConfig{Credentials: map[string]interface{}{
		CredentialKeyLastRefreshedAt: "2026-09-20T08:00:00Z",
	}}
	got, ok := ParseCredentialTimestamp(cfg, CredentialKeyLastRefreshedAt)
	require.True(t, ok)
	assert.True(t, got.Equal(time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)))

	_, ok = ParseCredentialTimestamp(cfg, CredentialKeyExpiresAt)
	assert.False(t, ok, "absent key must report false")
}
