// Package moauth provides a test-only mock OAuth connector (SP2-b §6.4): the
// acceptance vehicle for the credential auto-refresh chain —
// expiry → CredentialsRefresher → encrypted write-back → audit → binding
// auth-version bump → stale-cursor invalidation — without any real OAuth
// provider.
//
// It is deliberately NEVER registered in the production container: tests
// assemble their own connector registry around it. Every credential it issues
// is a constructive fake ("mock-access-<n>"); no real credential literal
// exists in this package.
//
// The connector implements exactly the base Connector interface plus
// CredentialsRefresher, and intentionally none of the other optional
// interfaces (StreamingConnector, FullSyncWithCursor, TargetedFetcher), so
// acceptance runs always exercise the plain batch sync path.
package moauth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

// Type is the connector type identifier of the mock. It is deliberately
// outside the production connector vocabulary and mock_-prefixed so it can
// never be confused with a real data source type.
const Type = "mock_oauth"

// refreshLead mirrors the service's proactive refresh window: nextRefreshAt
// hints a refresh this long before the new token expires.
const refreshLead = 5 * time.Minute

// Compile-time proofs: base connector plus the optional refresher protocol,
// and nothing else (batch-only by design).
var (
	_ datasource.Connector            = (*Connector)(nil)
	_ datasource.CredentialsRefresher = (*Connector)(nil)
)

// Connector is the mock OAuth data source connector. Construct it with New and
// inspect it through the accessors; all state is mutex-guarded because the
// service may call fetch and refresh from the same run.
type Connector struct {
	mu sync.Mutex
	// tokenTTL is stamped into each refreshed token's expires_at.
	tokenTTL time.Duration
	// refreshErr, when non-nil, makes RefreshCredentials fail with it (the
	// "临期刷新失败" and "过期未续" acceptance cases).
	refreshErr error
	// refreshCalls counts RefreshCredentials invocations; the nth call issues
	// "mock-access-<n>".
	refreshCalls int
	// fetchToken records the access_token the last fetch executed with.
	fetchToken string
	// fetchCursorNil records whether the last fetch received a nil cursor
	// (observable proof that a stale cursor was dropped for full
	// reconciliation).
	fetchCursorNil bool
}

// New creates the mock. tokenTTL is the lifetime stamped into each refreshed
// token's expires_at; a non-nil refreshErr makes every refresh fail with it.
func New(tokenTTL time.Duration, refreshErr error) *Connector {
	return &Connector{tokenTTL: tokenTTL, refreshErr: refreshErr}
}

// Type returns the mock connector type identifier.
func (c *Connector) Type() string { return Type }

// Validate accepts any configuration: the mock has no upstream to probe.
func (c *Connector) Validate(context.Context, *types.DataSourceConfig) error { return nil }

// ListResources returns nothing: the mock exposes no pickable resources.
func (c *Connector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}

// ResolveResourceAncestors returns nothing: the mock has no hierarchy.
func (c *Connector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}

// FetchAll returns no items and records the credentials it ran with — the
// mock's job is the credential chain, not content.
func (c *Connector) FetchAll(_ context.Context, config *types.DataSourceConfig, _ []string) ([]types.FetchedItem, error) {
	c.recordFetch(config, nil)
	return nil, nil
}

// FetchIncremental returns no items, passing the received cursor through, and
// records both the credentials and whether the cursor was nil.
func (c *Connector) FetchIncremental(
	_ context.Context, config *types.DataSourceConfig, cursor *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	c.recordFetch(config, cursor)
	return nil, cursor, nil
}

func (c *Connector) recordFetch(config *types.DataSourceConfig, cursor *types.SyncCursor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if config != nil {
		c.fetchToken, _ = config.Credentials["access_token"].(string)
	}
	c.fetchCursorNil = cursor == nil
}

// RefreshCredentials issues the next constructive fake token pair:
// "mock-access-<n>" plus a fresh expires_at tokenTTL from now, and a
// nextRefreshAt hint refreshLead before that expiry. It fails with the
// configured refreshErr, or when there is no access_token stored to rotate.
func (c *Connector) RefreshCredentials(
	_ context.Context, config *types.DataSourceConfig,
) (map[string]string, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshCalls++
	if c.refreshErr != nil {
		return nil, time.Time{}, c.refreshErr
	}
	if config == nil {
		return nil, time.Time{}, errors.New("mock oauth: nil config")
	}
	if current, _ := config.Credentials["access_token"].(string); current == "" {
		return nil, time.Time{}, errors.New("mock oauth: no access_token stored to refresh")
	}
	expiresAt := time.Now().UTC().Add(c.tokenTTL)
	updated := map[string]string{
		"access_token": fmt.Sprintf("mock-access-%d", c.refreshCalls),
		"expires_at":   expiresAt.Format(time.RFC3339),
	}
	return updated, expiresAt.Add(-refreshLead), nil
}

// RefreshCalls reports how many times RefreshCredentials has been invoked.
func (c *Connector) RefreshCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshCalls
}

// LastFetchAccessToken reports the access_token the most recent fetch ran with
// ("" before the first fetch), so tests can prove which token a sync executed
// on.
func (c *Connector) LastFetchAccessToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fetchToken
}

// LastFetchCursorWasNil reports whether the most recent fetch was started
// without a cursor — the observable half of "a stale cursor was dropped for
// full reconciliation".
func (c *Connector) LastFetchCursorWasNil() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fetchCursorNil
}
