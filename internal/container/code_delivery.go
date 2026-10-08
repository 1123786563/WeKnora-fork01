// Package container: code delivery wiring (T22 #52). The delivery-dedicated
// ActionService instance rides its OWN dispatcher so the global OC pipeline
// is untouched; the workspace adapter maps the sandbox session file surface
// onto the codedelivery port (same pattern as the artifact collector).
package container

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/sandbox"
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
		Connections: connections, Creds: creds, Guard: guard,
		GitHub: factory, GitLab: gitlabFactory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	return codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: creds,
		GitHub: factory, GitLab: gitlabFactory, Providers: providers,
		Workspace: workspace, Runs: runs, Dispatcher: dispatcher,
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
