package container

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// quiescentRunViewArtifactSource adapts the Run-bound no-follow artifact
// source to the capture service's quiescence contract. The durable SQL fences
// in CraftRunCaptureStore.EnsurePending (terminal Run, empty session writer
// slot, no pending tool call or delegation) prove no registered writer
// remains; the filesystem proof here is two complete, identity-checked
// observations of the sealed generation output tree that agree byte for
// byte — a writer mutating the tree between or during the walks breaks the
// per-walk identity invariants or the cross-walk comparison.
type quiescentRunViewArtifactSource struct {
	*runBoundCraftArtifactSource
}

var _ service.QuiescentRunArtifactSource = (*quiescentRunViewArtifactSource)(nil)

func (s *quiescentRunViewArtifactSource) VerifyCraftCaptureQuiescent(ctx context.Context) error {
	first, err := s.ListSessionFiles(ctx, s.task.Scope.SessionID, s.outputDir)
	if err != nil {
		return fmt.Errorf("quiescence list: %w", err)
	}
	firstIdentity := s.snapshotListed()
	second, err := s.ListSessionFiles(ctx, s.task.Scope.SessionID, s.outputDir)
	if err != nil {
		return fmt.Errorf("quiescence re-list: %w", err)
	}
	if len(first) != len(second) {
		return fmt.Errorf("%w: RunView output entry set changed during the quiescence proof", craft.ErrBusy)
	}
	for i := range first {
		if first[i].Name != second[i].Name || first[i].Path != second[i].Path ||
			first[i].Type != second[i].Type || first[i].Size != second[i].Size {
			return fmt.Errorf("%w: RunView output entry %q changed during the quiescence proof", craft.ErrBusy, first[i].Path)
		}
	}
	// The projected-entry comparison above stays for a precise error message,
	// but it is blind to same-size content rewrites: the walks' full identity
	// tables (sha256 digest plus stat identity per entry) must also agree.
	if !s.listedMatches(firstIdentity) {
		return fmt.Errorf("%w: RunView output content identity changed during the quiescence proof", craft.ErrBusy)
	}
	return nil
}

// closedCraftArtifactSource is the deliberately unusable session-wide source
// for capture-only artifact services. The capture and candidate paths always
// supply their own Run-bound source; the legacy session-wide CollectForKind
// route must never run against a capture assembly.
type closedCraftArtifactSource struct{}

func (closedCraftArtifactSource) ListSessionFiles(context.Context, string, string) ([]sandbox.RemoteDirEntry, error) {
	return nil, fmt.Errorf("%w: the session-wide artifact route is closed for RunView capture", craft.ErrForbidden)
}
func (closedCraftArtifactSource) ReadSessionFile(context.Context, string, string) ([]byte, error) {
	return nil, fmt.Errorf("%w: the session-wide artifact route is closed for RunView capture", craft.ErrForbidden)
}

// CraftRunCaptureRunner owns the post-terminal draft-capture assembly: the
// durable receipt outbox written by the terminal-transition trigger is
// drained through CraftRunCaptureService with a quiescent Run-bound source
// re-derived from the persisted bound RunView. It is nil-safe: without the
// RunView production assembly (or any of its dependencies) the runner stays
// inert and receipts remain pending, which is the recorded fail-closed state.
type CraftRunCaptureRunner struct {
	svc      *service.CraftRunCaptureService
	db       *gorm.DB
	store    craft.RunViewStore
	provider *CraftRunViewContainerProvider

	unavailable string
	interval    time.Duration
}

// newCraftRunCaptureRunner assembles the R4 Task3 capture coordinator. The
// resolve callback re-derives, for one durable receipt, the quiescent
// Run-bound source over the sealed generation output.
func newCraftRunCaptureRunner(
	db *gorm.DB,
	files interfaces.FileService,
	versions craft.VersionStore,
	assembly *CraftRunViewProductionAssembly,
) *CraftRunCaptureRunner {
	if db == nil || files == nil || versions == nil || assembly == nil ||
		assembly.Provider == nil || assembly.Store == nil || assembly.Unavailable != "" {
		reason := "capture dependencies are incomplete"
		if assembly != nil && assembly.Unavailable != "" {
			reason = assembly.Unavailable
		}
		return &CraftRunCaptureRunner{unavailable: reason, interval: craftRunCaptureScanInterval}
	}
	artifacts := service.NewCraftArtifactServiceWithCandidates(
		closedCraftArtifactSource{}, files, versions, repository.NewCraftCandidateStore(db), nil,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
	)
	runner := &CraftRunCaptureRunner{
		db: db, store: assembly.Store, provider: assembly.Provider,
		interval: craftRunCaptureScanInterval,
	}
	runner.svc = service.NewCraftRunCaptureService(
		artifacts, repository.NewCraftRunCaptureStore(db), repository.NewCraftDraftHeadStore(db),
		runner.resolveSource,
	)
	return runner
}

const craftRunCaptureScanInterval = 15 * time.Second

// resolveSource rebuilds the quiescent Run-bound artifact source for one
// durable capture receipt.
func (r *CraftRunCaptureRunner) resolveSource(ctx context.Context, receipt repository.CraftRunCapture) (service.QuiescentRunArtifactSource, error) {
	if r == nil || r.db == nil || r.store == nil || r.provider == nil {
		return nil, fmt.Errorf("%w: RunView capture assembly is unavailable", craft.ErrConflict)
	}
	key := craft.RunViewKey{
		TenantID: receipt.Scope.TenantID, OwnerID: receipt.Scope.UserID,
		SessionID: receipt.Scope.SessionID, RunID: receipt.RunID,
	}
	view, err := r.store.Load(ctx, key)
	if err != nil {
		return nil, err
	}
	if view.State != craft.RunViewStateBound {
		return nil, unresolvedCraftRunView("capture source requires a bound RunView generation", nil)
	}
	material, err := r.provider.MaterialHandleForCapture(ctx, r.store, key)
	if err != nil {
		return nil, err
	}
	var epoch int64
	if err := r.db.WithContext(ctx).Table("agent_runs").Select("epoch").
		Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&epoch).Error; err != nil {
		return nil, err
	}
	// The Run-bound source re-checks the durable writer fence on every list
	// and read; a minimal runtime binding carrying the database is sufficient
	// for that check.
	task := craft.Task{
		Scope: receipt.Scope, WorkspaceID: receipt.WorkspaceID,
		Fence: runtime.Fence{RunKey: runtime.RunKey{TenantID: key.TenantID, RunID: key.RunID}, Epoch: epoch},
	}
	source, err := newRunBoundCraftArtifactSourceForCapture(ctx, &localCraftRuntime{db: r.db}, task, material, craftLocalOutputDir)
	if err != nil {
		return nil, err
	}
	return &quiescentRunViewArtifactSource{runBoundCraftArtifactSource: source}, nil
}

// AfterTerminal drains the durable capture outbox right after a graph run
// finished and its Run transitioned terminal. Best-effort: the periodic
// recovery scan retries anything this immediate pass could not finish.
func (r *CraftRunCaptureRunner) AfterTerminal(ctx context.Context, _ runtime.Fence) {
	r.Recover(ctx, 32)
}

// Recover drains up to limit durable capture receipts. Missing receipts for
// terminal Runs are synthesized by the store's recovery scan, so a process
// that died between the terminal commit and the enqueue is healed here too.
func (r *CraftRunCaptureRunner) Recover(ctx context.Context, limit int) {
	if r == nil {
		return
	}
	if r.svc == nil {
		logger.Infof(ctx, "[CraftRunCapture] inert: %s", r.unavailable)
		return
	}
	if err := r.svc.RecoverPending(ctx, limit); err != nil {
		logger.Warnf(ctx, "[CraftRunCapture] recovery pass left receipts pending: %v", err)
	}
}

// Start launches the periodic recovery scan until ctx is canceled. The scan
// is the crash-recovery half of the outbox: receipts stranded by a worker
// that died after the terminal commit are advanced here.
func (r *CraftRunCaptureRunner) Start(ctx context.Context) {
	if r == nil || r.svc == nil {
		return
	}
	interval := r.interval
	if interval <= 0 {
		interval = craftRunCaptureScanInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), interval)
				r.Recover(scanCtx, 100)
				cancel()
			}
		}
	}()
}

// Unavailable reports why the runner is inert, if it is.
func (r *CraftRunCaptureRunner) Unavailable() string {
	if r == nil || r.svc != nil {
		return ""
	}
	if r.unavailable == "" {
		return "capture runner is not assembled"
	}
	return r.unavailable
}

// craftCaptureAfterTerminalBudget bounds the immediate post-terminal drain.
// AfterTerminal runs synchronously inside the worker's executor goroutine;
// without a deadline a stuck engine, database or object store would pin the
// worker's concurrency slot indefinitely. Whatever the budget cannot finish
// stays pending for the periodic recovery scan, which has its own interval
// budget.
const craftCaptureAfterTerminalBudget = 15 * time.Second

// drainCraftCaptureAfterTerminal applies the bounded budget around the
// executor-seam drain so a hung dependency cannot hold the worker slot.
func drainCraftCaptureAfterTerminal(ctx context.Context, drain craftCaptureAfterTerminal, fence runtime.Fence) {
	if drain == nil {
		return
	}
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftCaptureAfterTerminalBudget)
	defer cancel()
	drain.AfterTerminal(drainCtx, fence)
}

// craftCaptureAfterTerminal is the executor seam: the container's graph
// executor wrapper calls it once the run graph returned.
type craftCaptureAfterTerminal interface {
	AfterTerminal(context.Context, runtime.Fence)
}

var _ craftCaptureAfterTerminal = (*CraftRunCaptureRunner)(nil)
