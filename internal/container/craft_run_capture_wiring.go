package container

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
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
	// promoter is the T20 (#139) post-terminal four-check promotion pass:
	// after a drain seals a Run's output into the Workspace draft head it
	// attempts the promotion of exactly that sealed head (fail-closed on
	// every incomplete fact; nil keeps the trigger inert).
	promoter *craftPostTerminalPromoter

	unavailable string
	interval    time.Duration

	inertOnce sync.Once
	// inertLog defaults to nil and logInertOnce then stays silent; the
	// container wiring installs logger.Infof, tests install a counter.
	inertLog func(ctx context.Context, format string, args ...any)
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
		return &CraftRunCaptureRunner{unavailable: reason, interval: craftRunCaptureScanInterval, inertLog: logger.Infof}
	}
	drafts := repository.NewCraftDraftHeadStore(db)
	artifacts := service.NewCraftArtifactServiceWithCandidates(
		closedCraftArtifactSource{}, files, versions, repository.NewCraftCandidateStore(db), nil,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
		// T20 (#139): the SAME service carries the four-check promotion
		// dependencies — the draft-head revision fence plus the externally
		// observed page probe (nil until the T14 implementation registers;
		// promotion then fails closed on the not_run page facts).
	).WithWebPromotion(drafts, RegisteredCraftWebPageLoadProbe())
	runner := &CraftRunCaptureRunner{
		db: db, store: assembly.Store, provider: assembly.Provider,
		interval: craftRunCaptureScanInterval, inertLog: logger.Infof,
		promoter: newCraftPostTerminalPromoter(db, artifacts, drafts, versions),
	}
	runner.svc = service.NewCraftRunCaptureService(
		artifacts, repository.NewCraftRunCaptureStore(db), drafts,
		runner.resolveSource,
	)
	return runner
}

const craftRunCaptureScanInterval = 15 * time.Second

// craftRunCaptureScanBudget is the per-round budget of the periodic recovery
// scan, deliberately decoupled from (and much larger than) the scan interval
// and the immediate post-terminal drain budget: sealing one receipt performs
// repeated full-tree quiescence walks (sha256 over every byte) plus complete
// object uploads, so a budget equal to the interval would cut large receipts
// off mid-flight on both paths and force a full redo every tick, preventing
// convergence ("retries anything this immediate pass could not finish" only
// holds if the retry path has materially more budget than the drain).
const craftRunCaptureScanBudget = 5 * time.Minute

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

// AfterTerminal drains the durable capture outbox for the Run that just
// finished, immediately and best-effort. The periodic recovery scan owns the
// global pass (including missing-receipt synthesis) and retries anything
// this immediate, per-Run pass could not finish.
func (r *CraftRunCaptureRunner) AfterTerminal(ctx context.Context, fence runtime.Fence) {
	r.RecoverRun(ctx, fence)
}

// RecoverRun drains only the receipts of one Run. An unrelated Run's
// completion must never pay for a global recovery scan (multi-table JOIN
// synthesis plus arbitrary tenants' receipts) inside its worker executor
// slot.
func (r *CraftRunCaptureRunner) RecoverRun(ctx context.Context, fence runtime.Fence) {
	if r == nil {
		return
	}
	if r.svc == nil {
		r.logInertOnce(ctx)
		return
	}
	if err := r.svc.RecoverRun(ctx, fence.TenantID, fence.RunID); err != nil {
		logger.Warnf(ctx, "[CraftRunCapture] per-run recovery left receipts pending: %v", err)
	}
	// T20: the drain sealed this Run's output; attempt the four-check
	// promotion of the sealed head (best-effort, idempotent, fail-closed).
	r.promoter.promoteReceiptsForRun(ctx, fence.TenantID, fence.RunID)
}

// RecoverTick drains one periodic-scan round (synthesis only every Nth
// tick; freshness gate keeps it off receipts the drain owns).
func (r *CraftRunCaptureRunner) RecoverTick(ctx context.Context, limit int) {
	if r == nil {
		return
	}
	if r.svc == nil {
		r.logInertOnce(ctx)
		return
	}
	if err := r.svc.RecoverPendingTick(ctx, limit); err != nil {
		logger.Warnf(ctx, "[CraftRunCapture] recovery tick left receipts pending: %v", err)
	}
	r.promoter.promoteTerminalReceipts(ctx, craftPromotionScanLimit)
}

// Recover drains up to limit durable capture receipts. Missing receipts for
// terminal Runs are synthesized by the store's recovery scan, so a process
// that died between the terminal commit and the enqueue is healed here too.
func (r *CraftRunCaptureRunner) Recover(ctx context.Context, limit int) {
	if r == nil {
		return
	}
	if r.svc == nil {
		r.logInertOnce(ctx)
		return
	}
	if err := r.svc.RecoverPending(ctx, limit); err != nil {
		logger.Warnf(ctx, "[CraftRunCapture] recovery pass left receipts pending: %v", err)
	}
	r.promoter.promoteTerminalReceipts(ctx, craftPromotionScanLimit)
}

// logInertOnce explains the fail-closed inert state at most once per
// process: AfterTerminal runs after every graph run, and repeating the same
// "inert" line for each completion in a default (unconfigured) deployment is
// pure log noise. Tests replace inertLog to observe the dedup.
func (r *CraftRunCaptureRunner) logInertOnce(ctx context.Context) {
	r.inertOnce.Do(func() {
		if r.inertLog != nil {
			r.inertLog(ctx, "[CraftRunCapture] inert: %s", r.unavailable)
		}
	})
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
				scanCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftRunCaptureScanBudget)
				r.RecoverTick(scanCtx, 100)
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
// stays pending for the periodic recovery scan, whose per-round budget
// (craftRunCaptureScanBudget) is materially larger so large receipts can
// still converge there.
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
