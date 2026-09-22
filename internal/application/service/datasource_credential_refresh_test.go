package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────────────
// SP2-b Task 5 machine credential write-back channel (spec §6.2).
// All credential values below are constructive fakes (test-token-<n>) —
// no real credential literal ever enters source or fixtures.
// ──────────────────────────────────────────────────────────────────────

const credentialRefreshConnectorType = "test-credential-refresh"

// credentialRefreshProbeConnector counts Validate calls so tests can prove
// the machine channel never triggers the live validation the user-facing
// PUT /credentials path runs.
type credentialRefreshProbeConnector struct {
	validateCalls int
}

func (c *credentialRefreshProbeConnector) Type() string { return credentialRefreshConnectorType }
func (c *credentialRefreshProbeConnector) Validate(context.Context, *types.DataSourceConfig) error {
	c.validateCalls++
	return nil
}
func (c *credentialRefreshProbeConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}
func (c *credentialRefreshProbeConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}
func (c *credentialRefreshProbeConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	return nil, nil
}
func (c *credentialRefreshProbeConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return nil, nil, nil
}

// credentialRefreshFixture wires the service around one data source whose
// config jsonb is built through the real ToJSON encryption path.
type credentialRefreshFixture struct {
	ds      *types.DataSource
	repo    *kbDeleteDSRepo
	audit   *purgeAuditSink
	probe   *credentialRefreshProbeConnector
	svc     *DataSourceService
	setAES  func(t *testing.T, key string)
	toJSON  func(t *testing.T, cfg *types.DataSourceConfig) types.JSON
	orgBlob types.JSON
}

func newCredentialRefreshFixture(t *testing.T, dsType string, creds map[string]interface{}) *credentialRefreshFixture {
	t.Helper()
	f := &credentialRefreshFixture{
		probe: &credentialRefreshProbeConnector{},
		audit: &purgeAuditSink{},
	}
	f.setAES = func(t *testing.T, key string) {
		t.Helper()
		t.Setenv("SYSTEM_AES_KEY", key)
	}
	f.toJSON = func(t *testing.T, cfg *types.DataSourceConfig) types.JSON {
		t.Helper()
		blob, err := cfg.ToJSON()
		require.NoError(t, err)
		return blob
	}

	// A distinct 32-byte key per test run keeps the encryption path active
	// without any real secret material.
	f.setAES(t, strings.Repeat("a", 32))
	cfg := &types.DataSourceConfig{Type: dsType, Credentials: creds}
	f.ds = &types.DataSource{
		ID:              "ds-cred-refresh",
		TenantID:        11,
		KnowledgeBaseID: "kb-cred-refresh",
		Name:            "Refresh Source",
		Type:            dsType,
		Config:          f.toJSON(t, cfg),
		Status:          types.DataSourceStatusActive,
	}
	f.orgBlob = f.ds.Config
	f.repo = newKBDeleteDSRepo(f.ds.KnowledgeBaseID, f.ds)

	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(f.probe))
	f.svc = &DataSourceService{
		dsRepo:            f.repo,
		connectorRegistry: registry,
		audit:             f.audit,
	}
	return f
}

// TestRefreshDataSourceCredential_GuardRejectsUnconfigured pins the spec §6.2
// step 2 hard constraint: with nothing usable stored, the machine channel
// must refuse the write instead of letting refreshed-but-empty values
// permanently overwrite real credentials.
func TestRefreshDataSourceCredential_GuardRejectsUnconfigured(t *testing.T) {
	cases := []struct {
		name   string
		dsType string
		creds  map[string]interface{}
	}{
		{"no credentials at all", credentialRefreshConnectorType, nil},
		{"rss without auth headers", types.ConnectorTypeRSS, map[string]interface{}{
			"feed_urls": "https://example.invalid/feed.xml",
		}},
		{"rss with blank auth header", types.ConnectorTypeRSS, map[string]interface{}{
			"auth_headers": "  ",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCredentialRefreshFixture(t, tc.dsType, tc.creds)
			err := f.svc.RefreshDataSourceCredential(context.Background(), f.ds.ID, "access_token", "test-token-3")
			require.ErrorIs(t, err, datasource.ErrCredentialRefreshRejected)
			// Stored row untouched, no audit noise, no live validation.
			assert.Equal(t, f.orgBlob, f.ds.Config, "stored config blob must stay byte-identical")
			assert.Empty(t, f.audit.entries)
			assert.Zero(t, f.probe.validateCalls)
		})
	}
}

// TestRefreshDataSourceCredential_GuardRejectsAfterKeyRotation covers the
// key-rotation trap behind the guard: when SYSTEM_AES_KEY no longer decrypts
// the stored blob, ParseConfig blanks values to "" and an unconditional
// write-back would destroy the surviving ciphertext.
func TestRefreshDataSourceCredential_GuardRejectsAfterKeyRotation(t *testing.T) {
	f := newCredentialRefreshFixture(t, credentialRefreshConnectorType, map[string]interface{}{
		"access_token":  "test-token-1",
		"refresh_token": "test-token-2",
	})
	// Operator rotates the key between write and refresh.
	f.setAES(t, strings.Repeat("b", 32))

	err := f.svc.RefreshDataSourceCredential(context.Background(), f.ds.ID, "access_token", "test-token-3")
	require.ErrorIs(t, err, datasource.ErrCredentialRefreshRejected)
	assert.Equal(t, f.orgBlob, f.ds.Config,
		"undecryptable blob must be preserved verbatim for the operator, not blanked")
	assert.Empty(t, f.audit.entries)
	assert.Zero(t, f.probe.validateCalls)
}

// TestRefreshDataSourceCredential_SingleKeyUpdateIsTheWholeContract walks the
// happy path: one key rotates, every other key survives the encrypted
// round-trip, the refresh is audited as its own action, and the live
// validator never runs.
func TestRefreshDataSourceCredential_SingleKeyUpdateIsTheWholeContract(t *testing.T) {
	f := newCredentialRefreshFixture(t, credentialRefreshConnectorType, map[string]interface{}{
		"access_token":  "test-token-1",
		"refresh_token": "test-token-2",
		"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})

	// RFC3339 stamps carry second precision; truncate the lower bound to match.
	before := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, f.svc.RefreshDataSourceCredential(context.Background(), f.ds.ID, "access_token", "test-token-3"))

	// The stored blob is re-encrypted ciphertext, never plaintext.
	assert.NotContains(t, string(f.ds.Config), "test-token-3")
	assert.NotContains(t, string(f.ds.Config), "test-token-2")

	// Decrypting the persisted blob recovers the FULL key set: the rotated
	// key carries the new value, every other key is untouched.
	parsed, err := f.ds.ParseConfig()
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, "test-token-3", parsed.Credentials["access_token"])
	assert.Equal(t, "test-token-2", parsed.Credentials["refresh_token"])
	assert.NotEmpty(t, parsed.Credentials["expires_at"], "bookkeeping keys must survive too")

	// last_refreshed_at is stamped by the write-back itself.
	refreshedAt, ok := types.ParseCredentialTimestamp(parsed, types.CredentialKeyLastRefreshedAt)
	require.True(t, ok, "write-back must stamp last_refreshed_at")
	assert.False(t, refreshedAt.Before(before), "last_refreshed_at should be now-ish, got %v", refreshedAt)

	// Independent audit action, distinct from the user PUT's
	// datasource.updated changed_fields=credentials event.
	entries := f.audit.findByAction(types.AuditActionDataSourceCredentialAutoRefreshed)
	require.Len(t, entries, 1)
	assert.Equal(t, types.AuditOutcomeSuccess, entries[0].Outcome)
	assert.Equal(t, f.ds.ID, entries[0].TargetID)
	assert.Contains(t, string(entries[0].Details), `"field":"access_token"`)

	// Machine channel never runs live validation (spec §6.2 step 5).
	assert.Zero(t, f.probe.validateCalls)
}

// TestRefreshDataSourceCredential_InputValidation: blank ids/keys/values are
// rejected up front — an empty value would blank the key, defeating the
// anti-overwrite guard for that field.
func TestRefreshDataSourceCredential_InputValidation(t *testing.T) {
	f := newCredentialRefreshFixture(t, credentialRefreshConnectorType, map[string]interface{}{
		"access_token": "test-token-1",
	})
	cases := []struct{ name, id, key, value string }{
		{"empty id", "", "access_token", "test-token-3"},
		{"empty key", f.ds.ID, "", "test-token-3"},
		{"empty value", f.ds.ID, "access_token", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := f.svc.RefreshDataSourceCredential(context.Background(), tc.id, tc.key, tc.value)
			require.ErrorIs(t, err, datasource.ErrDataSourceInvalid)
			assert.Equal(t, f.orgBlob, f.ds.Config)
			assert.Empty(t, f.audit.entries)
		})
	}
}
