package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"

	"gorm.io/gorm"
)

// ConfluenceVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive version-conflict refusal; the service layer
// maps it onto the 409 PUBLISH_VERSION_CONFLICT response.
const ConfluenceVersionConflictResult = "confluence_version_conflict"

// appIDConfluence is the installation app identity the Confluence write
// adapter family serves.
const appIDConfluence = "confluence"

// ConfluenceConnectionScope is everything the publish seam needs to know
// about one connection before any Confluence call: the installation's app
// identity (routing), the reviewed destination scope (approved parent
// pages), the reviewed write capability, the connection's auth generation
// (strict credential binding) and the reviewed outbound endpoint derived
// from the installation's config_json.base_url.
type ConfluenceConnectionScope struct {
	AppID           string
	ConnectionKind  string
	OwnerID         string
	AuthVersion     int64
	ApprovedParents []string
	WriteCapability bool
	Scheme          string
	Host            string
	Port            string
	APIBasePath     string
	Edition         string
}

// ConfluenceScopeSource resolves the scope of one connection from the
// authoritative rows. The production implementation is
// NewDBConfluenceScopeSource (connections → installations → app_versions).
type ConfluenceScopeSource interface {
	ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error)
}

// ConfluencePolicyProvider returns the reviewed outbound HTTP policy for
// one connection's Confluence calls (A04). Production derives it from the
// same scope rows (NewConfluencePolicyProvider); tests inject the
// loopback-authorized policy.
type ConfluencePolicyProvider interface {
	PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error)
}

// ConfluenceTokenSource returns the decrypted Basic credential for
// exactly one connection at one auth generation — the A02 credential
// resolution AFTER the permission guard has passed.
type ConfluenceTokenSource interface {
	Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error)
}

// ConfluenceBridge adapts the Confluence adapter family to the A03
// pipeline's dispatch boundary. It routes by the approved snapshot's
// shape (page_id+expected_version+storage → update; parent+storage →
// create) and maps adapter outcomes onto DispatchOutcome:
//
//   - adapter FAILED (provider-refused, incl. version conflict) → a
//     definitive failed outcome, nil error — the service settles failed;
//   - adapter UNKNOWN (transport failure / 5xx / unprovable) → an unknown
//     outcome, nil error — the action parks for a provider query;
//   - wiring gaps (scope/token/policy unavailable, non-confluence
//     connection) → an ErrDispatchNotStarted error: provably pre-send,
//     nothing left the process.
type ConfluenceBridge struct {
	scopes   ConfluenceScopeSource
	policies ConfluencePolicyProvider
	tokens   ConfluenceTokenSource
}

// NewConfluenceBridge builds the bridge over its three ports (the
// single-step adapters carry no progress record, so unlike the Notion
// bridge there is no publication-progress port here).
func NewConfluenceBridge(scopes ConfluenceScopeSource, policies ConfluencePolicyProvider, tokens ConfluenceTokenSource) *ConfluenceBridge {
	return &ConfluenceBridge{scopes: scopes, policies: policies, tokens: tokens}
}

var _ appconnectorsvc.ActionDispatcher = (*ConfluenceBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*ConfluenceBridge)(nil)

// adapterFor builds the adapter instance for one dispatch/query. Scope,
// policy and credential are resolved fresh per call so a revoked
// connection (auth version bump) can never be reached with a stale
// secret.
func (b *ConfluenceBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.ConfluenceScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDConfluence {
		return nil, fmt.Errorf("%w: connection %q is %q, not a confluence connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	cred, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: credential for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		if !scope.WriteCapability {
			return nil, fmt.Errorf("connection lacks %s", appconn.ConfluenceCapabilityWrite)
		}
		return []string{appconn.ConfluenceCapabilityWrite}, nil
	}
	credFn := func(ctx context.Context) (appconn.ConfluenceCredential, error) { return cred, nil }
	if appconn.IsConfluenceUpdateArgs(json.RawMessage(snap.Args)) {
		return &appconn.ConfluenceUpdateAdapter{
			Policy: pol, Credential: credFn, Edition: scope.Edition,
			APIBasePath: scope.APIBasePath, ConnectionCapabilities: caps,
		}, nil
	}
	return &appconn.ConfluenceCreateAdapter{
		Policy: pol, Credential: credFn, Edition: scope.Edition,
		APIBasePath: scope.APIBasePath, ApprovedParents: scope.ApprovedParents,
		ConnectionCapabilities: caps,
	}, nil
}

func (b *ConfluenceBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
	adapter, err := b.adapterFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	action := appconn.Action{
		ID: snap.ID, TenantID: snap.TenantID, ActorID: snap.ActorID,
		ConnectionID: snap.ConnectionID, Version: snap.Version,
		Target: snap.Target, Risk: snap.Risk, AuthVersion: snap.AuthVersion,
		Args: json.RawMessage(snap.Args),
	}
	var res appconn.ActionResult
	var aerr error
	if query {
		res, aerr = adapter.Query(ctx, action)
	} else {
		res, aerr = adapter.Execute(ctx, action)
	}
	if aerr != nil {
		switch res.State {
		case appconn.ActionFailed:
			reason := aerr.Error()
			if errors.Is(aerr, appconn.ErrConfluenceVersionConflict) {
				reason = ConfluenceVersionConflictResult + ": remote version moved since approval"
			}
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: reason, ExecutionID: res.ExternalID}, nil
		case appconn.ActionUnknown:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: aerr.Error(), ExecutionID: res.ExternalID}, nil
		case appconn.ActionAwaitingApproval:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "approval revoked before send", ExecutionID: res.ExternalID}, nil
		default:
			return appconnectorsvc.DispatchOutcome{}, aerr
		}
	}
	return appconnectorsvc.DispatchOutcome{Status: res.State, ProviderResult: string(res.Output), ExecutionID: res.ExternalID}, nil
}

// Dispatch implements appconnectorsvc.ActionDispatcher.
func (b *ConfluenceBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

// QueryProvider implements appconnectorsvc.UnknownResolver: the provider
// (never the local queue) answers what became of an unknown dispatch.
func (b *ConfluenceBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadConfluencePageVersion is the plan-formation pre-read (AC1): the
// external current version of one page, read through the same
// policy/credential ports. It performs no write of any kind.
func (b *ConfluenceBridge) ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	scope, err := b.scopes.ConfluenceScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDConfluence {
		return "", fmt.Errorf("connection %q is not a confluence connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	cred, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	return appconn.ReadConfluencePageVersion(ctx, pol, func(context.Context) (appconn.ConfluenceCredential, error) { return cred, nil },
		scope.Edition, scope.APIBasePath, pageID)
}

// ---- production port implementations ----

// dbConfluenceScopeSource resolves a connection's publish scope from the
// authoritative rows: connections → installations (app id + reviewed
// base_url config) → app_versions.schema_json (reviewed scopes + approved
// destination pages).
type dbConfluenceScopeSource struct{ db *gorm.DB }

// NewDBConfluenceScopeSource builds the production scope source.
func NewDBConfluenceScopeSource(db *gorm.DB) ConfluenceScopeSource {
	return &dbConfluenceScopeSource{db: db}
}

// ConfluenceScope's first lookup keys on connection id ALONE — no tenant
// predicate. Tenant isolation here is a caller contract, not defense this
// query provides: every reachable path tenant-checks BEFORE calling
// (FormPlan's handler 404s cross-tenant connection ids at
// app_connector_confluence_publish.go; dispatch acts only on the snapshot
// of an action row whose tenant was bound and validated at Prepare), and
// the resolved scope never crosses back out to an external caller. This
// "tenant check first, then ConfluenceScope" order is inherited from the
// Notion bridge (dispatcher.go NotionScope) and MUST be kept by reusers.
func (s *dbConfluenceScopeSource) ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error) {
	var conn repoappconn.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&conn).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var inst repoappconn.InstallationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", conn.InstallationID, conn.TenantID).First(&inst).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var ver repoappconn.AppVersion
	if err := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", inst.AppID, inst.AppVersion).First(&ver).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var schema struct {
		Scopes          []string `json:"scopes"`
		ApprovedParents []string `json:"approved_parents"`
	}
	if err := json.Unmarshal([]byte(ver.SchemaJSON), &schema); err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var cfg struct {
		BaseURL string `json:"base_url"`
		Edition string `json:"edition"`
	}
	if err := json.Unmarshal([]byte(inst.ConfigJSON), &cfg); err != nil {
		return ConfluenceConnectionScope{}, fmt.Errorf("installation %q config_json: %v", inst.ID, err)
	}
	ep, err := appconn.ParseConfluenceBaseURL(cfg.BaseURL, cfg.Edition)
	if err != nil {
		// A connection whose reviewed endpoint does not parse fails closed:
		// no scope, no dispatch.
		return ConfluenceConnectionScope{}, fmt.Errorf("installation %q base_url: %v", inst.ID, err)
	}
	scope := ConfluenceConnectionScope{
		AppID: inst.AppID, ConnectionKind: conn.Kind, OwnerID: conn.OwnerID,
		AuthVersion: conn.AuthVersion, ApprovedParents: schema.ApprovedParents,
		Scheme: ep.Scheme, Host: ep.Host, Port: ep.Port,
		APIBasePath: ep.APIBasePath, Edition: ep.Edition,
	}
	for _, sc := range schema.Scopes {
		if sc == appconn.ConfluenceCapabilityWrite {
			scope.WriteCapability = true
		}
	}
	return scope, nil
}

// scopePolicyProvider derives the reviewed outbound policy from the same
// scope the bridge already trusts: HTTPS-or-HTTP to the reviewed origin,
// exactly GET/POST/PUT under the configured context path, 30s per-request
// timeout (inside the pipeline's 30s dispatch deadline). Dial-time
// public-address enforcement stays with HTTPPolicy.
type scopePolicyProvider struct{ scopes ConfluenceScopeSource }

// NewConfluencePolicyProvider builds the production policy provider over
// a scope source.
func NewConfluencePolicyProvider(scopes ConfluenceScopeSource) ConfluencePolicyProvider {
	return scopePolicyProvider{scopes: scopes}
}

func (p scopePolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	scope, err := p.scopes.ConfluenceScope(ctx, connectionID)
	if err != nil {
		return appconn.HTTPPolicy{}, err
	}
	prefix := "/"
	if scope.APIBasePath != "" {
		prefix = scope.APIBasePath + "/"
	}
	return appconn.HTTPPolicy{
		Scheme: scope.Scheme, Host: scope.Host, Port: scope.Port,
		Methods:    []string{"GET", "POST", "PUT"},
		PathPrefix: prefix,
		Timeout:    30 * time.Second,
	}, nil
}

// confluenceCredentialTokenSource resolves the connection credential
// through the A02 credential resolver (revoked / stale-version
// connections never resolve) and validates the reviewed JSON shape.
type confluenceCredentialTokenSource struct {
	resolver appconnectorsvc.CredentialResolver
}

// NewConfluenceCredentialTokenSource adapts the internal credential
// resolver onto the bridge's token port.
func NewConfluenceCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) ConfluenceTokenSource {
	return &confluenceCredentialTokenSource{resolver: resolver}
}

func (s *confluenceCredentialTokenSource) Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error) {
	raw, err := s.resolver.Resolve(ctx, connectionID, expectedVersion)
	if err != nil {
		return appconn.ConfluenceCredential{}, err
	}
	return appconn.ParseConfluenceCredential(raw)
}
