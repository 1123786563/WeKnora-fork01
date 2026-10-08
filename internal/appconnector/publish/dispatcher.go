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

// PublishVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive version-conflict refusal; the service layer
// maps it onto the 409 PUBLISH_VERSION_CONFLICT response.
const PublishVersionConflictResult = "notion_version_conflict"

// NotionConnectionScope is everything the publish seam needs to know
// about one connection before any Notion call: the installation's app
// identity (routing), the reviewed destination scope (approved parent
// pages), the reviewed insert-content capability, and the connection's
// auth generation (strict credential binding).
type NotionConnectionScope struct {
	AppID            string
	ConnectionKind   string
	OwnerID          string
	AuthVersion      int64
	ApprovedParents  []string
	InsertCapability bool
	// Scopes carries the connection's reviewed scopes verbatim (AC2 data
	// plane): downstream providers (#49 feishu) gate their own write
	// capability on this list; the notion-specific InsertCapability
	// projection above is unchanged for #48 behavior.
	Scopes []string
}

// NotionScopeSource resolves the scope of one connection from the
// authoritative rows. The production implementation is
// NewDBNotionScopeSource (connections → installations → app_versions).
type NotionScopeSource interface {
	NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error)
}

// NotionPolicyProvider returns the reviewed outbound HTTP policy for one
// connection's Notion calls (A04). Production uses the pinned Notion
// contract; tests inject the loopback-authorized policy.
type NotionPolicyProvider interface {
	PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error)
}

// NotionTokenSource returns the decrypted Notion token for exactly one
// connection at one auth generation — the A02 credential resolution AFTER
// the permission guard has passed.
type NotionTokenSource interface {
	Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error)
}

// PublicationSource is the receipt-store subset the bridge needs (the
// NO-03 progress checkpoint rides the publication row).
type PublicationSource interface {
	FindByAction(ctx context.Context, tenantID uint64, actionID string) (repoappconn.PublicationRow, error)
	SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error
}

// NotionBridge adapts the Notion adapter family to the A03 pipeline's
// dispatch boundary. It routes by the approved snapshot's shape
// (page_id+expected_version → update; parent → create) and maps adapter
// outcomes onto DispatchOutcome:
//
//   - adapter FAILED (provider-refused, incl. version conflict) → a
//     definitive failed outcome, nil error — the service settles failed;
//   - adapter UNKNOWN (transport failure / 5xx / unprovable) → an unknown
//     outcome, nil error — the action parks for a provider query;
//   - wiring gaps (scope/token/policy unavailable, non-notion connection)
//     → an ErrDispatchNotStarted error: provably pre-send, nothing left
//     the process.
type NotionBridge struct {
	scopes   NotionScopeSource
	policies NotionPolicyProvider
	tokens   NotionTokenSource
	pubs     PublicationSource
}

// NewNotionBridge builds the bridge over its four ports.
func NewNotionBridge(scopes NotionScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *NotionBridge {
	return &NotionBridge{scopes: scopes, policies: policies, tokens: tokens, pubs: pubs}
}

var _ appconnectorsvc.ActionDispatcher = (*NotionBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*NotionBridge)(nil)

// appIDNotion is the installation app identity the first write adapter
// family serves (appOAuthDefaults key, internal/handler/app_connector_oauth.go).
const appIDNotion = "notion"

// adapterFor builds the adapter instance for one dispatch/query. Token and
// policy are resolved fresh per call so a revoked connection (auth version
// bump) can never be reached with a stale secret.
func (b *NotionBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.NotionScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDNotion {
		return nil, fmt.Errorf("%w: connection %q is %q, not a notion connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	tok, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: token for connection %q: %w", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		if !scope.InsertCapability {
			return nil, fmt.Errorf("connection lacks %s", appconn.NotionCapabilityInsert)
		}
		return []string{appconn.NotionCapabilityInsert}, nil
	}
	progress := newPublicationProgress(b.pubs, snap.TenantID, snap.ID)
	action := appconn.Action{
		ID: snap.ID, TenantID: snap.TenantID, ActorID: snap.ActorID,
		ConnectionID: snap.ConnectionID, Version: snap.Version,
		Target: snap.Target, Risk: snap.Risk, AuthVersion: snap.AuthVersion,
		Args: json.RawMessage(snap.Args),
	}
	if appconn.IsNotionUpdateArgs(action.Args) {
		up := &appconn.NotionUpdateAdapter{
			Policy:                 pol,
			Token:                  func(ctx context.Context) (string, error) { return tok, nil },
			ConnectionCapabilities: caps,
			LoadProgress:           progress.load,
			SaveProgress:           progress.save,
		}
		return up, nil
	}
	cr := &appconn.NotionCreateAdapter{
		Policy:                 pol,
		Token:                  func(ctx context.Context) (string, error) { return tok, nil },
		ApprovedParents:        scope.ApprovedParents,
		ConnectionCapabilities: caps,
		LoadProgress:           progress.load,
		SaveProgress:           progress.save,
	}
	return cr, nil
}

// publicationProgress adapts the publication row's progress_json onto the
// adapters' LoadProgress/SaveProgress hooks.
// publicationProgress persists adapter checkpoints on the publication row.
// load/save deliberately use context.Background(), NOT the dispatch ctx: a
// checkpoint write must not be cut short by the dispatch deadline (30s), so
// an interrupted publish resumes from durable progress instead of redoing
// committed blocks (NO-03 recovery semantics).
type publicationProgress struct {
	pubs     PublicationSource
	tenantID uint64
	actionID string
}

func newPublicationProgress(pubs PublicationSource, tenantID uint64, actionID string) *publicationProgress {
	return &publicationProgress{pubs: pubs, tenantID: tenantID, actionID: actionID}
}

func (p *publicationProgress) load(a appconn.Action) appconn.NotionPageProgress {
	row, err := p.pubs.FindByAction(context.Background(), p.tenantID, p.actionID)
	if err != nil {
		return appconn.NotionPageProgress{}
	}
	var prog appconn.NotionPageProgress
	if json.Unmarshal([]byte(row.ProgressJSON), &prog) != nil {
		return appconn.NotionPageProgress{}
	}
	return prog
}

func (p *publicationProgress) save(a appconn.Action, prog appconn.NotionPageProgress) error {
	raw, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	return p.pubs.SaveProgress(context.Background(), p.tenantID, p.actionID, string(raw))
}

func (b *NotionBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
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
			if errors.Is(aerr, appconn.ErrNotionVersionConflict) {
				reason = PublishVersionConflictResult + ": remote version moved since approval"
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
func (b *NotionBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

// QueryProvider implements appconnectorsvc.UnknownResolver: the provider
// (never the local queue) answers what became of an unknown dispatch.
func (b *NotionBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadPageVersion is the plan-formation pre-read (AC1): the external
// current version of one page, read through the same policy/token ports.
func (b *NotionBridge) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	scope, err := b.scopes.NotionScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDNotion {
		return "", fmt.Errorf("connection %q is not a notion connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	tok, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	reader := &appconn.NotionUpdateAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return tok, nil },
	}
	return appconn.ReadNotionPageVersion(ctx, reader.Policy, reader.Token, pageID)
}

// ---- production port implementations ----

// dbNotionScopeSource resolves a connection's publish scope from the
// authoritative rows: connections → installations (app id) →
// app_versions.schema_json (reviewed scopes + approved destination pages).
type dbNotionScopeSource struct{ db *gorm.DB }

// NewDBNotionScopeSource builds the production scope source.
func NewDBNotionScopeSource(db *gorm.DB) NotionScopeSource { return &dbNotionScopeSource{db: db} }

// NotionScope's first lookup keys on connection id ALONE — no tenant
// predicate. Tenant isolation here is a caller contract, not defense this
// query provides: every reachable path tenant-checks BEFORE calling
// (FormPlan's handler 404s cross-tenant connection ids at
// app_connector_notion_publish.go; dispatch acts only on the snapshot of an
// action row whose tenant was bound and validated at Prepare), and the
// resolved scope never crosses back out to an external caller. Reusers
// (#49/#50) MUST keep this "tenant check first, then NotionScope" order.
func (s *dbNotionScopeSource) NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) {
	var conn repoappconn.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&conn).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var inst repoappconn.InstallationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", conn.InstallationID, conn.TenantID).First(&inst).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var ver repoappconn.AppVersion
	if err := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", inst.AppID, inst.AppVersion).First(&ver).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var parsed struct {
		Scopes          []string `json:"scopes"`
		ApprovedParents []string `json:"approved_parents"`
	}
	if err := json.Unmarshal([]byte(ver.SchemaJSON), &parsed); err != nil {
		return NotionConnectionScope{}, err
	}
	scope := NotionConnectionScope{
		AppID:           inst.AppID,
		ConnectionKind:  conn.Kind,
		OwnerID:         conn.OwnerID,
		AuthVersion:     conn.AuthVersion,
		ApprovedParents: parsed.ApprovedParents,
		Scopes:          append([]string(nil), parsed.Scopes...),
	}
	for _, sc := range parsed.Scopes {
		if sc == appconn.NotionCapabilityInsert {
			scope.InsertCapability = true
		}
	}
	return scope, nil
}

// constantNotionPolicyProvider pins the reviewed NO-01 outbound contract.
type constantNotionPolicyProvider struct{}

// NewConstantNotionPolicyProvider returns the production policy provider:
// HTTPS to the pinned Notion API host, GET/POST/PATCH under /v1/, 30s
// per-request timeout (inside the pipeline's 30s dispatch deadline).
func NewConstantNotionPolicyProvider() NotionPolicyProvider { return constantNotionPolicyProvider{} }

func (constantNotionPolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return appconn.HTTPPolicy{
		Scheme:     "https",
		Host:       appconn.NotionAPIHost,
		Methods:    []string{"GET", "POST", "PATCH"},
		PathPrefix: "/v1/",
		Timeout:    30 * time.Second,
	}, nil
}

// credentialTokenSource resolves the Notion token through the A02
// credential resolver (revoked / stale-version connections never resolve).
type credentialTokenSource struct {
	resolver appconnectorsvc.CredentialResolver
}

// NewCredentialTokenSource adapts the internal credential resolver onto
// the bridge's token port.
func NewCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) NotionTokenSource {
	return &credentialTokenSource{resolver: resolver}
}

func (s *credentialTokenSource) Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error) {
	raw, err := s.resolver.Resolve(ctx, connectionID, expectedVersion)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
