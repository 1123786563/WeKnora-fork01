package publish

// The Feishu member of the publication seam (#49). Structurally the
// Notion bridge's twin (dispatcher.go): the A03 pipeline sees ONE
// adapter family behind the same three ports. The historical
// Notion-prefixed port type names (NotionScopeSource / NotionPolicyProvider
// / NotionTokenSource) are provider-neutral despite their names — they
// are reused unchanged so #48's surface does not churn.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
)

// FeishuVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive revision-conflict refusal (mapped onto 409
// by the feishu publish handler, mirroring PublishVersionConflictResult).
const FeishuVersionConflictResult = "feishu_docx_revision_conflict"

// appIDFeishu is the installation app identity this bridge serves
// (appOAuthDefaults key, internal/handler/app_connector_oauth.go:49).
const appIDFeishu = "feishu"

// FeishuScopeSource aliases the shared scope port (the DB-backed
// production implementation NewDBNotionScopeSource is provider-neutral:
// connections → installations → app_versions).
type FeishuScopeSource = NotionScopeSource

// FeishuBridge adapts the Feishu docx adapter to the A03 pipeline's
// dispatch boundary. Outcome mapping is identical to NotionBridge.run
// (dispatcher.go:185-220): adapter FAILED → definitive failed outcome,
// adapter UNKNOWN → unknown outcome, wiring gaps → ErrDispatchNotStarted.
type FeishuBridge struct {
	scopes   FeishuScopeSource
	policies NotionPolicyProvider
	tokens   NotionTokenSource
	pubs     PublicationSource
}

func NewFeishuBridge(scopes FeishuScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *FeishuBridge {
	return &FeishuBridge{scopes: scopes, policies: policies, tokens: tokens, pubs: pubs}
}

var _ appconnectorsvc.ActionDispatcher = (*FeishuBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*FeishuBridge)(nil)
var _ NotionRemoteReader = (*FeishuBridge)(nil)

// feishuCapabilityDenied is the fail-closed stand-in adapterFor returns
// when the connection's reviewed scopes lack write_docx (AC2): Execute/
// Query are a definitive FAILED with ZERO network calls. The refusal is
// resolved HERE — before the adapter exists — so it can never be masked
// by an adapter-level config error (the adapter checks its outbound
// policy first), and it surfaces from run as a failed OUTCOME, never a
// wiring-gap error.
type feishuCapabilityDenied struct{ reason error }

func (m *feishuCapabilityDenied) Execute(ctx context.Context, a appconn.Action) (appconn.ActionResult, error) {
	return appconn.ActionResult{State: appconn.ActionFailed}, m.reason
}

func (m *feishuCapabilityDenied) Query(ctx context.Context, a appconn.Action) (appconn.ActionResult, error) {
	return appconn.ActionResult{State: appconn.ActionFailed}, m.reason
}

func (b *FeishuBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.NotionScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDFeishu {
		return nil, fmt.Errorf("%w: connection %q is %q, not a feishu connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	tok, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: token for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	// AC2: the write gate reads the connection's reviewed scopes — a
	// read/sync-only scope list never satisfies it. The gate resolves
	// HERE (into the adapter's capability source AND, on refusal, into
	// the denied stand-in below) so the definitive failed outcome carries
	// the capability reason even before any dial.
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		for _, sc := range scope.Scopes {
			if sc == appconn.FeishuCapabilityWriteDocx {
				return scope.Scopes, nil
			}
		}
		return nil, fmt.Errorf("connection lacks %s", appconn.FeishuCapabilityWriteDocx)
	}
	if _, cerr := caps(ctx, appconn.Action{}); cerr != nil {
		return &feishuCapabilityDenied{reason: fmt.Errorf("%w: %v", appconn.ErrFeishuPublishMissingCapability, cerr)}, nil
	}
	progress := newFeishuPublicationProgress(b.pubs, snap.TenantID, snap.ID)
	return &appconn.FeishuDocxAdapter{
		Policy:                 pol,
		Token:                  func(ctx context.Context) (string, error) { return tok, nil },
		ConnectionCapabilities: caps,
		LoadProgress:           progress.load,
		SaveProgress:           progress.save,
	}, nil
}

// feishuPublicationProgress persists FE-03 checkpoints on the
// publication row's progress_json — the feishu twin of publicationProgress
// (dispatcher.go:155-183); load/save deliberately use context.Background()
// so the dispatch deadline cannot cut a checkpoint short.
type feishuPublicationProgress struct {
	pubs     PublicationSource
	tenantID uint64
	actionID string
}

func newFeishuPublicationProgress(pubs PublicationSource, tenantID uint64, actionID string) *feishuPublicationProgress {
	return &feishuPublicationProgress{pubs: pubs, tenantID: tenantID, actionID: actionID}
}

func (p *feishuPublicationProgress) load(a appconn.Action) appconn.FeishuDocProgress {
	row, err := p.pubs.FindByAction(context.Background(), p.tenantID, p.actionID)
	if err != nil {
		return appconn.FeishuDocProgress{}
	}
	var prog appconn.FeishuDocProgress
	if json.Unmarshal([]byte(row.ProgressJSON), &prog) != nil {
		return appconn.FeishuDocProgress{}
	}
	return prog
}

func (p *feishuPublicationProgress) save(a appconn.Action, prog appconn.FeishuDocProgress) error {
	raw, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	return p.pubs.SaveProgress(context.Background(), p.tenantID, p.actionID, string(raw))
}

func (b *FeishuBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
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
			if errors.Is(aerr, appconn.ErrFeishuPublishRevisionConflict) {
				reason = FeishuVersionConflictResult + ": remote revision moved since approval"
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

func (b *FeishuBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

func (b *FeishuBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadPageVersion is the plan-formation pre-read (AC1 baseline) through
// the same policy/token ports; the interface name is historical.
func (b *FeishuBridge) ReadPageVersion(ctx context.Context, connectionID, documentID string) (string, error) {
	scope, err := b.scopes.NotionScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDFeishu {
		return "", fmt.Errorf("connection %q is not a feishu connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	tok, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	v, err := appconn.ReadFeishuDocumentVersion(ctx, pol, func(ctx context.Context) (string, error) { return tok, nil }, documentID)
	if err != nil {
		if errors.Is(err, appconn.ErrFeishuPublishNotFound) {
			// The create destination is a reviewed FOLDER: it has no
			// document revision. An EMPTY baseline (nil error) is the
			// provider's honest "no version concept" answer — the plan
			// layer accepts it for create and still refuses it for
			// update (whose destination is a previously published
			// document; a deleted one must fail the plan).
			return "", nil
		}
		return "", err
	}
	return v.RevisionID, nil
}

// ---- production ports ----

// constantFeishuPolicyProvider pins the reviewed FE-PUB-01 outbound
// contract: HTTPS to the pinned Feishu host, GET/POST under the docx
// path, 30s per-request timeout.
type constantFeishuPolicyProvider struct{}

func NewConstantFeishuPolicyProvider() NotionPolicyProvider { return constantFeishuPolicyProvider{} }

func (constantFeishuPolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return appconn.HTTPPolicy{
		Scheme:     "https",
		Host:       appconn.FeishuAPIHost,
		Methods:    []string{"GET", "POST"},
		PathPrefix: "/open-apis/docx/",
		Timeout:    30 * time.Second,
	}, nil
}

// feishuCredentialTokenSource reuses the A02 credential resolver via the
// existing constructor; the alias documents the feishu wiring point.
var NewFeishuCredentialTokenSource = NewCredentialTokenSource
