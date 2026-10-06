package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// CraftRunActivity answers from the business database whether the session
// currently has a live main run. Resolve uses it as the database-side half of
// its workspace mutation guard: the lifecycle lock serializes sandbox binding
// transitions per session, and the active-run check keeps a live run's
// working directory from being swapped mid-flight.
type CraftRunActivity func(ctx context.Context, scope craft.Scope) (bool, error)

// CraftOpenCodeDial returns the pinned OpenCode client (R01, 1.18.4) for the
// sandbox a session is bound to. The base URL is always server-derived from
// the sandbox network mapping — never client-supplied — and carries no
// account credentials: model access flows through the server-side gateway the
// executor configures, never through this path.
type CraftOpenCodeDial func(ctx context.Context, binding sandbox.SessionSandboxBinding) (*opencode.Client, error)

// CraftWorkspaceConfig assembles the workspace resolver.
type CraftWorkspaceConfig struct {
	// Store persists the Craft workspace binding (R02).
	Store craft.Store
	// Bindings is the existing session sandbox binding store; Resolve runs
	// under its per-session lifecycle lock.
	Bindings sandbox.SessionSandboxBindingStore
	// ActiveRuns is the database active-run check.
	ActiveRuns CraftRunActivity
	// Dial produces the pinned OpenCode client for the bound sandbox.
	Dial CraftOpenCodeDial
	// RuntimeDigest pins the reproducible runtime identity (locked OpenCode
	// binary/image digest). A workspace whose digest no longer matches was
	// built on a replaced runtime and must be re-bound before reuse.
	RuntimeDigest string
	// WriterLeases is the T16 (#134) durable writer-lease store. Nil leaves
	// the writer admission surface fail-closed.
	WriterLeases CraftWriterLeaseStore
}

// CraftWorkspaceService binds one session to one sandboxed OpenCode session
// and reuses that binding across rounds on the same artwork: the second round
// resolves the same working directory instead of creating a new OpenCode
// session every time.
type CraftWorkspaceService struct {
	store         craft.Store
	bindings      sandbox.SessionSandboxBindingStore
	activeRuns    CraftRunActivity
	dial          CraftOpenCodeDial
	runtimeDigest string
	writerLeases  CraftWriterLeaseStore
}

// NewCraftWorkspaceService validates the assembly and returns the resolver.
func NewCraftWorkspaceService(cfg CraftWorkspaceConfig) (*CraftWorkspaceService, error) {
	if cfg.Store == nil || cfg.Bindings == nil || cfg.ActiveRuns == nil || cfg.Dial == nil {
		return nil, errors.New("craft: workspace service requires store, bindings, active-run check and dialer")
	}
	if strings.TrimSpace(cfg.RuntimeDigest) == "" {
		return nil, errors.New("craft: workspace service requires a pinned runtime digest")
	}
	return &CraftWorkspaceService{
		store: cfg.Store, bindings: cfg.Bindings, activeRuns: cfg.ActiveRuns,
		dial: cfg.Dial, runtimeDigest: cfg.RuntimeDigest, writerLeases: cfg.WriterLeases,
	}, nil
}

// Resolve returns the session's workspace, creating the OpenCode session and
// persisting the binding on first use and reusing the recorded mapping —
// verified against the sandbox generation and pinned runtime digest — on
// every later round without another CreateSession. All work runs under the
// session's existing sandbox lifecycle lock.
func (s *CraftWorkspaceService) Resolve(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Workspace{}, fmt.Errorf("%w: incomplete workspace scope", craft.ErrInvalidInput)
	}
	key := sandbox.SessionSandboxKey{TenantID: scope.TenantID, SessionID: scope.SessionID}
	var resolved craft.Workspace
	err := s.bindings.WithLifecycleLock(ctx, key, func(lockCtx context.Context) error {
		ws, err := s.resolveLocked(lockCtx, scope, key)
		if err != nil {
			return err
		}
		resolved = ws
		return nil
	})
	if err != nil {
		return craft.Workspace{}, fmt.Errorf("craft: resolve workspace for session %s: %w", scope.SessionID, err)
	}
	return resolved, nil
}

// authorizeWorkspaceWrite is the explicit service-side owner guard required
// before any PutWorkspace: the store's CAS update path accepts a changed
// owner_id (R02 review nit-2), so Resolve must itself refuse to write a
// workspace whose recorded owner is not exactly the resolving scope. It never
// relies on the store's read-side guard.
func authorizeWorkspaceWrite(scope craft.Scope, ws craft.Workspace) error {
	if !craft.SameScope(ws.Scope, scope) {
		return fmt.Errorf("%w: workspace %s is bound to tenant %d session %s owner %s",
			craft.ErrForbidden, ws.ID, ws.Scope.TenantID, ws.Scope.SessionID, ws.Scope.UserID)
	}
	return nil
}

func (s *CraftWorkspaceService) resolveLocked(
	ctx context.Context,
	scope craft.Scope,
	key sandbox.SessionSandboxKey,
) (craft.Workspace, error) {
	binding, err := s.bindings.Get(ctx, key)
	if err != nil {
		return craft.Workspace{}, fmt.Errorf("read sandbox binding: %w", err)
	}
	if binding == nil {
		return craft.Workspace{}, fmt.Errorf("%w: session %s has no bound sandbox", craft.ErrUnsupported, key.SessionID)
	}
	active, err := s.activeRuns(ctx, scope)
	if err != nil {
		return craft.Workspace{}, fmt.Errorf("check active run: %w", err)
	}
	ws, err := s.store.GetWorkspace(ctx, scope)
	if errors.Is(err, craft.ErrNotFound) {
		return s.provision(ctx, scope, binding, nil)
	}
	if err != nil {
		return craft.Workspace{}, err
	}
	// nit-2: explicit owner write check on the row we are about to CAS.
	if err := authorizeWorkspaceWrite(scope, ws); err != nil {
		return craft.Workspace{}, err
	}
	if workspaceMatches(ws, binding, s.runtimeDigest) {
		// Verified reuse: same sandbox, current generation, pinned runtime
		// digest and a recorded OpenCode session. No CreateSession this round.
		return ws, nil
	}
	if active {
		return craft.Workspace{}, fmt.Errorf("%w: session %s has an active run on workspace %s; refusing to swap sandbox %s for %s",
			craft.ErrBusy, scope.SessionID, ws.ID, ws.SandboxID, binding.SandboxID)
	}
	return s.provision(ctx, scope, binding, &ws)
}

// workspaceMatches reports whether a persisted workspace is the controlled
// mapping for exactly this binding under the pinned runtime digest.
func workspaceMatches(ws craft.Workspace, binding *sandbox.SessionSandboxBinding, runtimeDigest string) bool {
	return ws.OpenCodeSessionID != "" &&
		ws.SandboxID == binding.SandboxID &&
		ws.Generation == binding.Generation &&
		ws.RuntimeDigest == runtimeDigest
}

// provision creates a fresh OpenCode session in the bound sandbox and
// persists the mapping with compare-and-swap semantics. existing is nil for a
// first binding and the currently read row for a re-bind.
func (s *CraftWorkspaceService) provision(
	ctx context.Context,
	scope craft.Scope,
	binding *sandbox.SessionSandboxBinding,
	existing *craft.Workspace,
) (craft.Workspace, error) {
	// nit-2 guard again at the write site: only the recorded owner may put.
	if existing != nil {
		if err := authorizeWorkspaceWrite(scope, *existing); err != nil {
			return craft.Workspace{}, err
		}
	}
	client, err := s.dial(ctx, *binding)
	if err != nil {
		return craft.Workspace{}, fmt.Errorf("dial opencode for sandbox %s: %w", binding.SandboxID, err)
	}
	ocSessionID, err := client.CreateSession(ctx)
	if err != nil {
		// The create response was lost: the sandbox may hold a session we can
		// no longer identify, because the pinned create route carries no
		// client-controlled identity. Reconcile only against controlled
		// metadata — the persisted workspace row. A freshly created empty
		// session is never adopted as the recovery.
		if winner, ok, rerr := s.reconcile(ctx, scope, binding); rerr != nil {
			return craft.Workspace{}, rerr
		} else if ok {
			return winner, nil
		}
		return craft.Workspace{}, fmt.Errorf("%w: opencode create session response lost for sandbox %s: %v",
			craft.ErrUnknown, binding.SandboxID, err)
	}
	next := craft.Workspace{
		Scope:             scope,
		SandboxID:         binding.SandboxID,
		Generation:        binding.Generation,
		OpenCodeSessionID: ocSessionID,
		RuntimeDigest:     s.runtimeDigest,
	}
	expected := int64(0)
	if existing != nil {
		next.ID = existing.ID
		expected = existing.Revision
	}
	stored, err := s.store.PutWorkspace(ctx, next, expected)
	if err != nil {
		// Lost the first-create race (a cross-process writer without the
		// shared lock): adopt the winner only if it is a controlled mapping
		// for this same binding.
		if existing == nil && errors.Is(err, craft.ErrConflict) {
			if winner, ok, rerr := s.reconcile(ctx, scope, binding); rerr != nil {
				return craft.Workspace{}, rerr
			} else if ok {
				return winner, nil
			}
		}
		return craft.Workspace{}, err
	}
	return stored, nil
}

// reconcile re-reads the workspace and adopts it only when it is controlled
// metadata for exactly this binding: owned by the resolving scope, pointing
// at the same sandbox, current generation, pinned runtime digest, and a
// recorded OpenCode session id.
func (s *CraftWorkspaceService) reconcile(
	ctx context.Context,
	scope craft.Scope,
	binding *sandbox.SessionSandboxBinding,
) (craft.Workspace, bool, error) {
	ws, err := s.store.GetWorkspace(ctx, scope)
	if errors.Is(err, craft.ErrNotFound) {
		return craft.Workspace{}, false, nil
	}
	if err != nil {
		return craft.Workspace{}, false, err
	}
	if err := authorizeWorkspaceWrite(scope, ws); err != nil {
		return craft.Workspace{}, false, err
	}
	if !workspaceMatches(ws, binding, s.runtimeDigest) {
		return craft.Workspace{}, false, nil
	}
	return ws, true, nil
}

// CraftWriterLeaseStore is the T16 (#134) durable writer-lease seam. The
// concrete *repository.CraftStore satisfies it; every method is
// scope-guarded by the owning Task session.
type CraftWriterLeaseStore interface {
	// AcquireWriterLease admits at most one writing Run per Workspace; a
	// concurrent loser receives a stable conflict naming the holder.
	AcquireWriterLease(context.Context, craft.Scope, string, string) (craft.WriterAcquisition, error)
	// ReleaseWriterLease releases the lease only after a verified outcome;
	// an unknown outcome retains the fence.
	ReleaseWriterLease(context.Context, craft.Scope, string, string, string) error
	// GetWriterLease projects the durable lease row without acquiring it.
	GetWriterLease(context.Context, craft.Scope, string) (*craft.WriterLease, error)
}

// AcquireWriter admits one writing Run on the scope's Workspace. The lease
// binds the Task (session), Workspace, Run and the draft-head revision; a
// concurrent writer receives a stable conflict instead of an error. The
// read-only workspace paths (Resolve, GetWorkspace, version reads) never
// come through here.
func (s *CraftWorkspaceService) AcquireWriter(ctx context.Context, scope craft.Scope, runID string) (craft.WriterAcquisition, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || runID == "" {
		return craft.WriterAcquisition{}, fmt.Errorf("%w: incomplete writer admission", craft.ErrInvalidInput)
	}
	if s.writerLeases == nil {
		return craft.WriterAcquisition{}, fmt.Errorf("%w: workspace service is not assembled with a writer lease store", craft.ErrUnsupported)
	}
	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.WriterAcquisition{}, err
	}
	acquisition, err := s.writerLeases.AcquireWriterLease(ctx, scope, workspace.ID, runID)
	if err != nil {
		return craft.WriterAcquisition{}, fmt.Errorf("craft: acquire writer for session %s: %w", scope.SessionID, err)
	}
	return acquisition, nil
}

// ReleaseWriter releases the scope's Workspace writer lease under the named
// basis. An unknown basis retains the fence; a non-holder is forbidden.
func (s *CraftWorkspaceService) ReleaseWriter(ctx context.Context, scope craft.Scope, runID, basis string) error {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || runID == "" {
		return fmt.Errorf("%w: incomplete writer release", craft.ErrInvalidInput)
	}
	if s.writerLeases == nil {
		return fmt.Errorf("%w: workspace service is not assembled with a writer lease store", craft.ErrUnsupported)
	}
	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return err
	}
	if err := s.writerLeases.ReleaseWriterLease(ctx, scope, workspace.ID, runID, basis); err != nil {
		return fmt.Errorf("craft: release writer for session %s: %w", scope.SessionID, err)
	}
	return nil
}

// WriterLease projects the scope's durable writer lease without acquiring
// it — the read-only seam for status surfaces.
func (s *CraftWorkspaceService) WriterLease(ctx context.Context, scope craft.Scope) (*craft.WriterLease, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return nil, fmt.Errorf("%w: incomplete lease projection", craft.ErrInvalidInput)
	}
	if s.writerLeases == nil {
		return nil, fmt.Errorf("%w: workspace service is not assembled with a writer lease store", craft.ErrUnsupported)
	}
	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return nil, err
	}
	lease, err := s.writerLeases.GetWriterLease(ctx, scope, workspace.ID)
	if err != nil {
		return nil, fmt.Errorf("craft: read writer lease for session %s: %w", scope.SessionID, err)
	}
	return lease, nil
}
