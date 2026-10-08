package publish

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
)

// Publish sentinel errors. None of them ever carries credential material.
var (
	ErrPublishInvalidInput             = errors.New("publish_invalid_input")
	ErrPublishArtifactNotReady         = errors.New("publish_artifact_not_ready")
	ErrPublishUnsupportedArtifact      = errors.New("publish_unsupported_artifact")
	ErrPublishDestinationOutOfScope    = errors.New("publish_destination_out_of_scope")
	ErrPublishDestinationUnreadable    = errors.New("publish_destination_unreadable")
	ErrPublishUpdateTargetNotPublished = errors.New("publish_update_target_not_published")
)

// MaxPublishArtifactBytes bounds the artifact bytes a plan may read.
const MaxPublishArtifactBytes = 1 << 20

// ArtifactVersionReader reads immutable, published (ready) artifact
// versions — the「确定 Artifact」anchor. Satisfied by
// *repository.ArtifactVersionStore.
type ArtifactVersionReader interface {
	ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error)
}

// ArtifactContentReader reads one version's bytes from tenant storage.
type ArtifactContentReader interface {
	ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error)
}

// NotionRemoteReader is the plan-formation pre-read port (satisfied by
// *NotionBridge.ReadPageVersion).
type NotionRemoteReader interface {
	ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error)
}

// PublishPlanInput forms one publish plan: exactly one destination shape
// (ParentPageID for create, PageID for update).
type PublishPlanInput struct {
	TenantID          uint64
	ActorID           string
	ConnectionID      string
	SessionID         string
	ArtifactVersionID string
	Title             string
	ParentPageID      string
	PageID            string
}

// PublishArtifactView is the immutable internal artifact anchor carried by
// the plan and the receipt.
type PublishArtifactView struct {
	VersionID string `json:"version_id"`
	Digest    string `json:"digest"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
}

// PublishPlanView is the formed Action Plan as returned to the caller: the
// prepared action (digest + fence are what an approval binds) plus the
// destination and the external version the plan read.
type PublishPlanView struct {
	ActionID                string              `json:"action_id"`
	Digest                  string              `json:"digest"`
	State                   string              `json:"state"`
	Fence                   int64               `json:"expected_version"`
	Mode                    string              `json:"mode"`
	Destination             string              `json:"destination"`
	ExpectedExternalVersion string              `json:"expected_external_version"`
	Title                   string              `json:"title"`
	Artifact                PublishArtifactView `json:"artifact"`
}

// PublishReceiptView is the durable 外部发布 record's projection.
type PublishReceiptView struct {
	ActionID          string `json:"action_id"`
	State             string `json:"state"`
	Mode              string `json:"mode"`
	Destination       string `json:"destination"`
	ExternalID        string `json:"external_id,omitempty"`
	ExternalVersion   string `json:"external_version,omitempty"`
	ArtifactVersionID string `json:"artifact_version_id"`
}

// PublishExecuteOutcome is the result of Execute/Reconcile: the ACTION
// row's authoritative state, the conflict marker, and the receipt
// projection.
type PublishExecuteOutcome struct {
	ActionState string             `json:"action_state"`
	Conflict    bool               `json:"conflict"`
	Receipt     PublishReceiptView `json:"publication"`
}

// NotionPublishService forms approved publish plans from confirmed
// artifact versions, executes them through the dedicated A03 ActionService
// (whose dispatcher/resolver is the Notion bridge), and settles the
// publication receipt from the action row's authoritative outcome — the
// receipt is always a projection of the action, never the reverse.
type NotionPublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    NotionRemoteReader
	scopes    NotionScopeSource
	// profile carries every provider-specific decision. AC1: the
	// provider difference lives only in the adapter/bridge layer — the
	// service body below has zero provider branches.
	profile ProviderProfile
}

// NewNotionPublishService builds the publish service over the Notion
// profile (#48 signature unchanged — all existing call sites keep
// compiling).
func NewNotionPublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote NotionRemoteReader,
	scopes NotionScopeSource,
) *NotionPublishService {
	return NewProviderPublishService(actions, store, pubs, artifacts, content, scopes, NotionProfile(remote))
}

// NewProviderPublishService builds the provider-neutral publish service
// for any provider profile (#49 feishu, #50 confluence).
func NewProviderPublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	scopes NotionScopeSource,
	profile ProviderProfile,
) *NotionPublishService {
	return &NotionPublishService{actions: actions, store: store, pubs: pubs, artifacts: artifacts, content: content, scopes: scopes, profile: profile}
}

func publishableMIME(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	return strings.HasPrefix(m, "text/") || m == "application/json"
}

// FormPlan forms one publish Action Plan (CONTEXT.md「操作计划/外部发布」):
// resolve the confirmed artifact version, derive the Notion blocks
// deterministically, READ the external current version (AC1 baseline; the
// update snapshot binds it as expected_version), then Prepare the A03
// action whose normalized bytes + digest the approval binds, and record
// the planned publication row.
func (s *NotionPublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || in.ConnectionID == "" || in.SessionID == "" ||
		in.ArtifactVersionID == "" || strings.TrimSpace(in.Title) == "" {
		return PublishPlanView{}, fmt.Errorf("%w: tenant, actor, connection, session, artifact version and title are required", ErrPublishInvalidInput)
	}
	if (in.ParentPageID == "") == (in.PageID == "") {
		return PublishPlanView{}, fmt.Errorf("%w: exactly one of parent_page_id (create) or page_id (update)", ErrPublishInvalidInput)
	}
	scope, err := s.scopes.NotionScope(ctx, in.ConnectionID)
	if err != nil {
		return PublishPlanView{}, fmt.Errorf("%w: connection scope: %v", ErrPublishInvalidInput, err)
	}
	if scope.AppID != s.profile.AppID {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not %s", ErrPublishInvalidInput, scope.AppID, s.profile.AppID)
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
	blocks, berr := s.profile.BlocksOf(string(content))
	if berr != nil {
		return PublishPlanView{}, berr
	}
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot and MUST succeed.
	// For create the baseline is provider-defined: Notion reads the parent
	// page's version; Feishu's reviewed destination is a FOLDER with no
	// document revision, which the adapter reports as the typed
	// not-found shape and the plan records an EMPTY baseline. Any other
	// create pre-read failure (transport, permission) still fails closed.
	remoteVersion, rerr := s.profile.ReadRemoteVersion(ctx, in.ConnectionID, destination)
	if rerr != nil && !errors.Is(rerr, appconn.ErrFeishuPublishNotFound) {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	if mode == "update" && (rerr != nil || remoteVersion == "") {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	var args []byte
	if mode == "update" {
		args, err = s.profile.UpdateArgs(in.PageID, remoteVersion, in.Title, blocks)
	} else {
		args, err = s.profile.CreateArgs(in.ParentPageID, in.Title, blocks)
	}
	if err != nil {
		return PublishPlanView{}, err
	}
	actionID, perr := s.actions.Prepare(ctx, appconn.Action{
		ID: "", TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Version: s.profile.ActionVersion, Target: destination, Risk: appconn.RiskWrite,
		AuthVersion: scope.AuthVersion, Args: args,
	})
	if perr != nil {
		return PublishPlanView{}, perr
	}
	if uerr := s.pubs.CreatePublication(ctx, repoappconn.PublicationRow{
		TenantID: in.TenantID, ActionID: actionID, ConnectionID: in.ConnectionID,
		Provider: s.profile.Provider, Mode: mode, Destination: destination,
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
// settles the receipt from the action row's authoritative outcome. ErrDispatchUnknown
// is NOT an error here: the outcome (unknown + receipt projection) reports
// the honest parked state for reconciliation.
func (s *NotionPublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.Execute(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Reconcile resolves an unknown outcome by querying the PROVIDER first —
// never a re-dispatch — then settles the receipt from the action row.
func (s *NotionPublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.ResolveUnknown(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Receipt returns the publication record's current projection.
func (s *NotionPublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error) {
	pub, err := s.pubs.FindByAction(ctx, tenantID, actionID)
	if err != nil {
		return PublishReceiptView{}, err
	}
	return publicationView(pub), nil
}

func publicationView(pub repoappconn.PublicationRow) PublishReceiptView {
	return PublishReceiptView{
		ActionID: pub.ActionID, State: pub.State, Mode: pub.Mode,
		Destination: pub.Destination, ExternalID: pub.ExternalID,
		ExternalVersion: pub.ExternalVersion, ArtifactVersionID: pub.ArtifactVersionID,
	}
}

// project settles the receipt FROM the action row's authoritative state.
// Order matters: the action row is written by the ActionService first;
// only then does the receipt follow. A settle failure surfaces while the
// action state stays durable (the receipt can be re-settled).
func (s *NotionPublishService) project(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	row, err := s.store.FindAction(ctx, actionID)
	if err != nil {
		return PublishExecuteOutcome{}, err
	}
	if row.TenantID != tenantID {
		return PublishExecuteOutcome{}, repoappconn.ErrActionNotFound
	}
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: s.profile.ConflictResultPrefix != "" && strings.HasPrefix(row.ProviderResult, s.profile.ConflictResultPrefix)}
	switch row.State {
	case appconn.ActionSucceeded:
		// Defensive: a succeeded payload that no longer parses as a page
		// receipt skips settle rather than guessing a state. Unreachable
		// today — every succeeded ProviderResult is the adapter's read-back
		// receipt — and safe on re-entry: any later Execute/Reconcile re-runs
		// project, which retries the settle from the durable action row.
		rcptExternalID, rcptExternalVersion, rerr := s.profile.ParseReceipt(row.ProviderResult)
		if rerr == nil {
			if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationPublished, rcptExternalID, rcptExternalVersion, row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
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
