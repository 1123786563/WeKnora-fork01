package moauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All credential values are constructive fakes ("mock-access-<n>" /
// repeated-letter keys) — no real credential literal ever enters the source.
func TestRefreshCredentials_IssuesSequentialFakeTokens(t *testing.T) {
	c := New(time.Hour, nil)
	config := &types.DataSourceConfig{Credentials: map[string]interface{}{
		"access_token": "mock-access-0",
		"expires_at":   time.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339),
	}}

	updated, nextRefreshAt, err := c.RefreshCredentials(context.Background(), config)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"access_token": "mock-access-1",
		"expires_at":   updated["expires_at"],
	}, updated, "the rotated pair must carry a sequential fake token and its expiry")

	expiresAt, parseErr := time.Parse(time.RFC3339, updated["expires_at"])
	require.NoError(t, parseErr, "expires_at must stay a parseable RFC3339 stamp")
	assert.WithinDuration(t, time.Now().UTC().Add(time.Hour), expiresAt, time.Minute,
		"the configured token TTL is stamped into the new expires_at")
	assert.True(t, nextRefreshAt.Before(expiresAt),
		"nextRefreshAt must hint a refresh before the token expires")
	assert.False(t, nextRefreshAt.IsZero(), "nextRefreshAt must be set on success")

	// The second refresh advances the sequence.
	updated2, _, err := c.RefreshCredentials(context.Background(), config)
	require.NoError(t, err)
	assert.Equal(t, "mock-access-2", updated2["access_token"])
	assert.Equal(t, 2, c.RefreshCalls())
}

func TestRefreshCredentials_ConfigurableFailure(t *testing.T) {
	sentinel := errors.New("mock refresh endpoint unreachable")
	c := New(time.Hour, sentinel)
	config := &types.DataSourceConfig{Credentials: map[string]interface{}{
		"access_token": "mock-access-0",
	}}

	updated, nextRefreshAt, err := c.RefreshCredentials(context.Background(), config)
	require.ErrorIs(t, err, sentinel)
	assert.Nil(t, updated)
	assert.True(t, nextRefreshAt.IsZero())
	assert.Equal(t, 1, c.RefreshCalls())
}

func TestRefreshCredentials_RejectsMissingAccessToken(t *testing.T) {
	c := New(time.Hour, nil)
	_, _, err := c.RefreshCredentials(context.Background(), &types.DataSourceConfig{})
	require.Error(t, err)
}

// FetchAll/FetchIncremental return no items (the mock's job is the credential
// chain, not content) but must record the credentials and cursor they ran with
// so acceptance tests can prove which token a sync executed on and whether the
// consumer dropped a stale cursor for a full reconciliation.
func TestFetchRecordsTokenAndCursor(t *testing.T) {
	c := New(time.Hour, nil)
	config := &types.DataSourceConfig{Credentials: map[string]interface{}{
		"access_token": "mock-access-7",
	}}
	cursor := &types.SyncCursor{ConnectorCursor: map[string]interface{}{"marker": "resume"}}

	items, err := c.FetchAll(context.Background(), config, []string{"res-1"})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, "mock-access-7", c.LastFetchAccessToken())

	_, next, err := c.FetchIncremental(context.Background(), config, cursor)
	require.NoError(t, err)
	assert.Same(t, cursor, next, "the cursor passes through unchanged")
	assert.False(t, c.LastFetchCursorWasNil())

	_, _, err = c.FetchIncremental(context.Background(), config, nil)
	require.NoError(t, err)
	assert.True(t, c.LastFetchCursorWasNil(), "a dropped (nil) cursor must be observable")
}

// The mock must stay batch-only: implementing any other optional interface
// would change which sync path the acceptance chain exercises.
func TestConnectorImplementsNoOtherOptionalInterface(t *testing.T) {
	c := New(time.Hour, nil)
	assert.Equal(t, Type, c.Type())

	var conn interface{} = c
	if _, ok := conn.(datasource.CredentialsRefresher); !ok {
		t.Fatal("mock must implement CredentialsRefresher")
	}
	if _, ok := conn.(datasource.StreamingConnector); ok {
		t.Fatal("mock must stay batch-only (no StreamingConnector)")
	}
	if _, ok := conn.(datasource.FullStreamingConnector); ok {
		t.Fatal("mock must stay batch-only (no FullStreamingConnector)")
	}
	if _, ok := conn.(datasource.FullSyncWithCursor); ok {
		t.Fatal("mock must stay batch-only (no FullSyncWithCursor)")
	}
	if _, ok := conn.(datasource.TargetedFetcher); ok {
		t.Fatal("mock must stay batch-only (no TargetedFetcher)")
	}
}

// Validate/ListResources/ResolveResourceAncestors are inert so the acceptance
// fixture never depends on network-shaped behaviour.
func TestInertSurfaces(t *testing.T) {
	c := New(time.Hour, nil)
	require.NoError(t, c.Validate(context.Background(), &types.DataSourceConfig{}))
	resources, err := c.ListResources(context.Background(), &types.DataSourceConfig{}, "")
	require.NoError(t, err)
	assert.Empty(t, resources)
	ancestors, err := c.ResolveResourceAncestors(context.Background(), &types.DataSourceConfig{}, nil)
	require.NoError(t, err)
	assert.Empty(t, ancestors)
}

// The package must never register itself anywhere global; Type is deliberately
// outside the production connector vocabulary.
func TestTypeIsNotAProductionConnectorType(t *testing.T) {
	production := []string{
		"feishu", "lark", "feishu_drive", "lark_drive", "notion", "confluence",
		"yuque", "ima", "dingtalk", "rss", "gitlab",
	}
	assert.NotContains(t, production, Type)
	assert.True(t, strings.HasPrefix(Type, "mock_"), "test-only type must be namespaced as mock_*")
}
