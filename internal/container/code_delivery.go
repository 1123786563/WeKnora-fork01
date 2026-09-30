// Package container: code delivery wiring (T22 #52). The delivery-dedicated
// ActionService instance rides its OWN dispatcher so the global OC pipeline
// is untouched; the workspace adapter maps the sandbox session file surface
// onto the codedelivery port (same pattern as the artifact collector).
package container

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// sandboxWorkspaceSource adapts the deployment's sandbox managers onto the
// codedelivery workspace port: per call, the tenant resolver wins over the
// process default, and only *sandbox.SessionBoundManager (Cube/E2B/Docker)
// can serve — anything else is ErrWorkspaceUnavailable (fail closed).
type sandboxWorkspaceSource struct {
	mgr      sandbox.Manager
	resolver sandbox.TenantSandboxResolver
}

// manager resolves the effective manager for one tenant (the tenant resolver
// wins over the process default) and only accepts *sandbox.SessionBoundManager
// — anything else is ErrWorkspaceUnavailable (fail closed).
func (s *sandboxWorkspaceSource) manager(ctx context.Context, tenantID uint64) (*sandbox.SessionBoundManager, error) {
	mgr := s.mgr
	if s.resolver != nil {
		if resolved, err := s.resolver.Resolve(ctx, tenantID, ""); err == nil {
			mgr = resolved
		}
	}
	if bound, ok := mgr.(*sandbox.SessionBoundManager); ok {
		return bound, nil
	}
	return nil, codedelivery.ErrWorkspaceUnavailable
}

// tenantOf reads the tenant from the request context (all three workspace
// methods run inside a request context; a missing tenant fails closed).
func tenantOf(ctx context.Context) (uint64, error) {
	if tenantID, ok := types.TenantIDFromContext(ctx); ok && tenantID != 0 {
		return tenantID, nil
	}
	return 0, codedelivery.ErrWorkspaceUnavailable
}

func (s *sandboxWorkspaceSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]codedelivery.WorkspaceDirEntry, error) {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	src, err := s.manager(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	entries, err := src.ListSessionFiles(ctx, sessionID, dir)
	if err != nil {
		return nil, err
	}
	out := make([]codedelivery.WorkspaceDirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, codedelivery.WorkspaceDirEntry{Path: e.Path, IsDir: e.Type == sandbox.RemoteEntryDir, Size: e.Size})
	}
	return out, nil
}

func (s *sandboxWorkspaceSource) ReadSessionFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	src, err := s.manager(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return src.ReadSessionFile(ctx, sessionID, path)
}

func (s *sandboxWorkspaceSource) WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []codedelivery.WorkspaceFileWrite) error {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return err
	}
	src, err := s.manager(ctx, tenantID)
	if err != nil {
		return err
	}
	mapped := make([]sandbox.SessionWorkspaceFile, 0, len(files))
	for _, f := range files {
		mapped = append(mapped, sandbox.SessionWorkspaceFile{Path: f.Path, Content: f.Content})
	}
	return src.WriteSessionWorkspaceFiles(ctx, sessionID, mapped)
}

// newCodeDeliveryService is the dig provider: it builds the dedicated
// dispatcher + ActionService + service in one place (no dig type collisions
// with the global A03 instance).
func newCodeDeliveryService(
	db *gorm.DB,
	actionStore appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	connections appconnectorsvc.ConnectionCredentialSource,
	runs *repository.AgentRunStore,
	sandboxMgr sandbox.Manager,
	resolver sandbox.TenantSandboxResolver,
) (*codedelivery.CodeDeliveryService, error) {
	workspace := &sandboxWorkspaceSource{mgr: sandboxMgr, resolver: resolver}
	store := deliveryrepo.NewDeliveryStore(db)
	creds := appconnectorsvc.NewCredentialResolver(connections)
	factory := codedelivery.NewGitHubClientFactory(nil, codedelivery.GitHubAPIBaseURL)
	gitlabFactory := codedelivery.NewGitLabClientFactory(nil, codedelivery.GitLabAPIBaseURL)
	providers := appconnectorrepo.NewInstallationStore(db)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: codeDeliveryConnections{source: connections}, Creds: creds, Guard: codeDeliveryGuard{guard: guard},
		GitHub: factory, GitLab: gitlabFactory, Workspace: workspace, Store: store,
		ActionRows: codeDeliveryActionRows{store: actionStore}, Runs: codeDeliveryRunReader{runs: runs},
	})
	bridge := codeDeliveryActionBridge{delivery: dispatcher}
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, bridge, bridge)
	return codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: codeDeliveryLifecycle{actions: actions}, ActionRows: codeDeliveryActionRows{store: actionStore},
		Connections: codeDeliveryConnections{source: connections}, Creds: creds,
		GitHub: factory, GitLab: gitlabFactory, Providers: codeDeliveryProviderSource{store: providers},
		Workspace: workspace, Runs: codeDeliveryRunReader{runs: runs}, Dispatcher: dispatcher,
	}), nil
}

// NewWorkbenchDeliveryHandler wires the handler to the run store (owner
// predicate) and the granted-read fallback (#42 face).
func NewWorkbenchDeliveryHandler(
	runs *repository.AgentRunStore,
	service *codedelivery.CodeDeliveryService,
) *session.WorkbenchDeliveryHandler {
	return session.NewWorkbenchDeliveryHandler(runs, runs, service)
}

// codeDeliveryActionRows maps persistence rows to the codedelivery-owned record.
type codeDeliveryActionRows struct {
	store appconnectorsvc.ActionStoreSource
}

func (a codeDeliveryActionRows) FindAction(ctx context.Context, id string) (codedelivery.ActionRecord, error) {
	r, err := a.store.FindAction(ctx, id)
	if err != nil {
		return codedelivery.ActionRecord{}, err
	}
	return codedelivery.ActionRecord{ID: r.ID, TenantID: r.TenantID, ActorID: r.ActorID, ConnectionID: r.ConnectionID, AppVersion: r.AppVersion, Target: r.Target, Risk: r.Risk, ArgsDigest: r.ArgsDigest, State: r.State, Fence: r.Fence, ArgsSnapshot: r.ArgsSnapshot, AuthVersion: r.AuthVersion, DigestVersion: int(r.DigestVersion), ProviderResult: r.ProviderResult}, nil
}

type codeDeliveryRunReader struct{ runs *repository.AgentRunStore }

func (r codeDeliveryRunReader) GetOwnedRun(ctx context.Context, tenant uint64, owner, id string) (codedelivery.RunIdentity, error) {
	run, err := r.runs.GetOwnedRun(ctx, tenant, owner, id)
	return codedelivery.RunIdentity{SessionID: run.SessionID}, err
}

type codeDeliveryProviderSource struct {
	store *appconnectorrepo.InstallationStore
}

func (p codeDeliveryProviderSource) GetInstallationByID(ctx context.Context, tenant uint64, id string) (codedelivery.ProviderInstallation, error) {
	i, err := p.store.GetInstallationByID(ctx, tenant, id)
	return codedelivery.ProviderInstallation{AppID: i.AppID}, err
}

type codeDeliveryActionBridge struct {
	delivery *codedelivery.DeliveryDispatcher
}

func (b codeDeliveryActionBridge) Dispatch(ctx context.Context, s appconnectorsvc.ActionSnapshot, key string) (appconnectorsvc.DispatchOutcome, error) {
	o, e := b.delivery.Dispatch(ctx, codedelivery.ActionSnapshot{ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target, AuthVersion: s.AuthVersion, Args: s.Args}, key)
	if e != nil && errors.Is(e, codedelivery.ErrDispatchNotStarted) {
		e = fmt.Errorf("%w: %w", appconnectorsvc.ErrDispatchNotStarted, e)
	}
	if e != nil && errors.Is(e, codedelivery.ErrDispatchUnknown) {
		e = fmt.Errorf("%w: %w", appconnectorsvc.ErrDispatchUnknown, e)
	}
	return appconnectorsvc.DispatchOutcome{Status: o.Status, ProviderResult: o.ProviderResult}, e
}
func (b codeDeliveryActionBridge) QueryProvider(ctx context.Context, s appconnectorsvc.ActionSnapshot, key string) (appconnectorsvc.DispatchOutcome, error) {
	o, e := b.delivery.QueryProvider(ctx, codedelivery.ActionSnapshot{ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target, AuthVersion: s.AuthVersion, Args: s.Args}, key)
	if e != nil && errors.Is(e, codedelivery.ErrDispatchUnknown) {
		e = fmt.Errorf("%w: %w", appconnectorsvc.ErrDispatchUnknown, e)
	}
	return appconnectorsvc.DispatchOutcome{Status: o.Status, ProviderResult: o.ProviderResult}, e
}

type codeDeliveryLifecycle struct {
	actions *appconnectorsvc.ActionService
}

func (a codeDeliveryLifecycle) Prepare(ctx context.Context, in codedelivery.ActionInput) (string, error) {
	return a.actions.Prepare(ctx, appconnector.Action{TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID, Target: in.Target, Risk: in.Risk, AuthVersion: in.AuthVersion, Args: in.Args})
}
func (a codeDeliveryLifecycle) Execute(ctx context.Context, id string) error {
	err := a.actions.Execute(ctx, id)
	if errors.Is(err, appconnectorsvc.ErrDispatchNotStarted) {
		return fmt.Errorf("%w: %w", codedelivery.ErrDispatchNotStarted, err)
	}
	if errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return fmt.Errorf("%w: %w", codedelivery.ErrDispatchUnknown, err)
	}
	return err
}
func (a codeDeliveryLifecycle) ResolveUnknown(ctx context.Context, id string) error {
	err := a.actions.ResolveUnknown(ctx, id)
	if errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return fmt.Errorf("%w: %w", codedelivery.ErrDispatchUnknown, err)
	}
	return err
}

type codeDeliveryConnections struct {
	source appconnectorsvc.ConnectionCredentialSource
}

func (c codeDeliveryConnections) FindConnectionByID(ctx context.Context, id string) (codedelivery.ConnectionIdentity, error) {
	v, e := c.source.FindConnectionByID(ctx, id)
	return codedelivery.ConnectionIdentity{ID: v.ID, InstallationID: v.InstallationID, Kind: v.Kind, OwnerID: v.OwnerID, State: v.State, TenantID: v.TenantID, AuthVersion: v.AuthVersion}, e
}

type codeDeliveryGuard struct{ guard appconnectorsvc.A02Guard }

func (g codeDeliveryGuard) Check(ctx context.Context, s codedelivery.ActionSubject, id string, v int64) error {
	return g.guard.Check(ctx, appconnector.OCSubject{TenantID: s.TenantID, ActorID: s.ActorID}, id, v)
}
