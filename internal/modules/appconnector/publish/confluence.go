package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// ConfluenceRemoteReader is the plan-formation pre-read port (satisfied
// by *ConfluenceBridge.ReadConfluencePageVersion).
type ConfluenceRemoteReader interface {
	ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error)
}

// ConfluencePublishService forms approved publish plans from confirmed
// artifact versions, executes them through the dedicated A03 ActionService
// (whose dispatcher/resolver is the Confluence bridge), and settles the
// publication receipt from the action row's authoritative outcome — the
// receipt is always a projection of the action, never the reverse. The
// plan/receipt views, the publication store and the error sentinels are
// shared with the Notion publish service verbatim; only the destination
// scope source, the remote pre-read, the content projection and the
// receipt parser are provider-specific.
type ConfluencePublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    ConfluenceRemoteReader
	scopes    ConfluenceScopeSource
}

// NewConfluencePublishService builds the publish service.
func NewConfluencePublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote ConfluenceRemoteReader,
	scopes ConfluenceScopeSource,
) *ConfluencePublishService {
	return &ConfluencePublishService{actions: actions, store: store, pubs: pubs, artifacts: artifacts, content: content, remote: remote, scopes: scopes}
}

// FormPlan forms one publish Action Plan (CONTEXT.md「操作计划/外部发布」):
// resolve the confirmed artifact version, derive the Confluence storage
// body deterministically, READ the external current version (AC1
// baseline; the update snapshot binds it as expected_version), then
// Prepare the A03 action whose normalized bytes + digest the approval
// binds, and record the planned publication row. For create the pre-read
// is the PARENT page's current version — a recorded baseline of the
// reviewed destination (nothing existing is overwritten, so it is never
// enforced); for update it is the authoritative conflict anchor.
func (s *ConfluencePublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || in.ConnectionID == "" || in.SessionID == "" ||
		in.ArtifactVersionID == "" || strings.TrimSpace(in.Title) == "" {
		return PublishPlanView{}, fmt.Errorf("%w: tenant, actor, connection, session, artifact version and title are required", ErrPublishInvalidInput)
	}
	if (in.ParentPageID == "") == (in.PageID == "") {
		return PublishPlanView{}, fmt.Errorf("%w: exactly one of parent_page_id (create) or page_id (update)", ErrPublishInvalidInput)
	}
	scope, err := s.scopes.ConfluenceScope(ctx, in.ConnectionID)
	if err != nil {
		return PublishPlanView{}, fmt.Errorf("%w: connection scope: %v", ErrPublishInvalidInput, err)
	}
	if scope.AppID != appIDConfluence {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not confluence", ErrPublishInvalidInput, scope.AppID)
	}
	mode, destination := "create", in.ParentPageID
	if in.PageID != "" {
		mode, destination = "update", in.PageID
		// Authority rule: a page this tenant+connection published before
		// is ours to update; anything else fails closed.
		if _, perr := s.pubs.LatestPublishedByDestination(ctx, in.TenantID, in.ConnectionID, in.PageID); perr != nil {
			return PublishPlanView{}, fmt.Errorf("%w: %s has no prior receipt through this connection", ErrPublishUpdateTargetNotPublished, in.PageID)
		}
	} else {
		approved := false
		for _, p := range scope.ApprovedParents {
			if p == in.ParentPageID {
				approved = true
				break
			}
		}
		if !approved {
			return PublishPlanView{}, fmt.Errorf("%w: parent %q is not in the reviewed destination scope", ErrPublishDestinationOutOfScope, in.ParentPageID)
		}
	}
	version, verr := s.artifacts.ReadableArtifactVersion(ctx, in.TenantID, in.SessionID, in.ArtifactVersionID)
	if verr != nil {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishArtifactNotReady, verr)
	}
	if !publishableMIME(version.MIME) {
		return PublishPlanView{}, fmt.Errorf("%w: mime %q", ErrPublishUnsupportedArtifact, version.MIME)
	}
	content, cerr := s.content.ReadArtifactContent(ctx, in.TenantID, version)
	if cerr != nil {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishArtifactNotReady, cerr)
	}
	if len(content) > MaxPublishArtifactBytes {
		return PublishPlanView{}, fmt.Errorf("%w: %d bytes", ErrPublishContentTooLarge, len(content))
	}
	storage, berr := ConfluenceStorageBody(string(content))
	if berr != nil {
		return PublishPlanView{}, berr
	}
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot; for create it is the
	// destination's recorded baseline.
	remoteVersion, rerr := s.remote.ReadConfluencePageVersion(ctx, in.ConnectionID, destination)
	if rerr != nil || remoteVersion == "" {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	var args []byte
	if mode == "update" {
		args, err = json.Marshal(map[string]any{
			"page_id": in.PageID, "expected_version": remoteVersion,
			"title": in.Title, "storage": storage,
		})
	} else {
		args, err = json.Marshal(map[string]any{
			"parent": in.ParentPageID, "title": in.Title, "storage": storage,
		})
	}
	if err != nil {
		return PublishPlanView{}, err
	}
	actionID, perr := s.actions.Prepare(ctx, appconn.Action{
		ID: "", TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Version: "confluence/v1", Target: destination, Risk: appconn.RiskWrite,
		AuthVersion: scope.AuthVersion, Args: args,
	})
	if perr != nil {
		return PublishPlanView{}, perr
	}
	if uerr := s.pubs.CreatePublication(ctx, repoappconn.PublicationRow{
		TenantID: in.TenantID, ActionID: actionID, ConnectionID: in.ConnectionID,
		Provider: "confluence", Mode: mode, Destination: destination,
		ExpectedVersion: remoteVersion, ArtifactVersionID: version.ID,
		ArtifactDigest: version.Digest, State: repoappconn.PublicationPlanned,
	}); uerr != nil {
		// The prepared action stays awaiting_approval (never approved,
		// never dispatched) — an orphan plan row is harmless, a fabricated
		// receipt is not.
		return PublishPlanView{}, uerr
	}
	row, aerr := s.store.FindAction(ctx, actionID)
	if aerr != nil {
		return PublishPlanView{}, aerr
	}
	return PublishPlanView{
		ActionID: actionID, Digest: row.ArgsDigest, State: row.State, Fence: row.Fence,
		Mode: mode, Destination: destination, ExpectedExternalVersion: remoteVersion,
		Title:    in.Title,
		Artifact: PublishArtifactView{VersionID: version.ID, Digest: version.Digest, MIME: version.MIME, Size: version.Size},
	}, nil
}

// Execute runs the approved plan through the dedicated ActionService and
// settles the receipt from the action row's authoritative outcome.
// ErrDispatchUnknown is NOT an error here: the outcome (unknown + receipt
// projection) reports the honest parked state for reconciliation.
func (s *ConfluencePublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.Execute(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Reconcile resolves an unknown outcome by querying the PROVIDER first —
// never a re-dispatch — then settles the receipt from the action row.
func (s *ConfluencePublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.ResolveUnknown(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Receipt returns the publication record's current projection.
func (s *ConfluencePublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error) {
	pub, err := s.pubs.FindByAction(ctx, tenantID, actionID)
	if err != nil {
		return PublishReceiptView{}, err
	}
	return publicationView(pub), nil
}

// project settles the receipt FROM the action row's authoritative state.
// Order matters: the action row is written by the ActionService first;
// only then does the receipt follow. A settle failure surfaces while the
// action state stays durable (the receipt can be re-settled).
func (s *ConfluencePublishService) project(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	row, err := s.store.FindAction(ctx, actionID)
	if err != nil {
		return PublishExecuteOutcome{}, err
	}
	if row.TenantID != tenantID {
		return PublishExecuteOutcome{}, repoappconn.ErrActionNotFound
	}
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: strings.HasPrefix(row.ProviderResult, ConfluenceVersionConflictResult)}
	switch row.State {
	case appconn.ActionSucceeded:
		rcpt, rerr := appconn.ParseConfluencePageReceipt([]byte(row.ProviderResult))
		if rerr == nil {
			if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationPublished, rcpt.ExternalID, rcpt.ExternalVersion, row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
				return out, serr
			}
		}
	case appconn.ActionFailed:
		if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationFailed, "", "", row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
			return out, serr
		}
	case appconn.ActionUnknown:
		if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationUnknown, "", "", ""); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
			return out, serr
		}
	}
	pub, perr := s.pubs.FindByAction(ctx, tenantID, actionID)
	if perr != nil {
		return out, perr
	}
	out.Receipt = publicationView(pub)
	return out, nil
}
