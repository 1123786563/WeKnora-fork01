package container

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ResourceCleaner is a resource cleaner that can be used to clean up resources
type ResourceCleaner struct {
	mu       sync.Mutex
	cleanups []types.CleanupFunc
}

// NewResourceCleaner creates a new resource cleaner
func NewResourceCleaner() interfaces.ResourceCleaner {
	return &ResourceCleaner{
		cleanups: make([]types.CleanupFunc, 0),
	}
}

// Register registers a cleanup function
// Note: the cleanup function will be executed in reverse order (the last registered will be executed first)
func (c *ResourceCleaner) Register(cleanup types.CleanupFunc) {
	if cleanup == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanups = append(c.cleanups, cleanup)
}

// RegisterWithName registers a cleanup function with a name, for logging tracking
func (c *ResourceCleaner) RegisterWithName(name string, cleanup types.CleanupFunc) {
	if cleanup == nil {
		return
	}

	wrappedCleanup := func() error {
		log.Printf("Cleaning up resource: %s", name)
		err := cleanup()
		if err != nil {
			log.Printf("Error cleaning up resource %s: %v", name, err)
		} else {
			log.Printf("Successfully cleaned up resource: %s", name)
		}
		return err
	}

	c.Register(wrappedCleanup)
}

// Cleanup executes all cleanup functions
// Even if a cleanup function fails, other cleanup functions will still be executed
func (c *ResourceCleaner) Cleanup(ctx context.Context) (errs []error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Execute cleanup functions in reverse order (the last registered will be executed first)
	for i := len(c.cleanups) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
			return errs
		default:
			if err := c.cleanups[i](); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

// Reset clears all registered cleanup functions
func (c *ResourceCleaner) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanups = make([]types.CleanupFunc, 0)
}

// -----------------------------------------------------------------------------
// O03: periodic resource-reclamation sweeps
// -----------------------------------------------------------------------------

// PeriodicSweepFn is one reclamation pass. It must be safe to run
// concurrently with normal traffic: the craft lifecycle sweep derives every
// deletion decision from the real run relationships under the per-session
// lifecycle lock, so a sweep tick never needs to pause the service.
type PeriodicSweepFn func(ctx context.Context) error

// ExecutionCleanupWorker is the production cleanup port. Its observation
// source is assembled by the repository; callers cannot inject hand-written
// facts into the periodic worker.
type ExecutionCleanupWorker interface {
	RunCleanupOnce(ctx context.Context, worker string, lease time.Duration) error
}

// StartExecutionCleanupSweep drives stop/usage/replay/restore reconciliation.
// File/blob/backup deletion remains fail-closed until the W26 adapter is
// installed on the repository.
func StartExecutionCleanupSweep(worker ExecutionCleanupWorker, cleaner interfaces.ResourceCleaner) {
	if worker == nil || cleaner == nil || parseCraftLifecycleBoolEnv("EXECUTION_CLEANUP_DISABLED") {
		return
	}
	interval := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("EXECUTION_CLEANUP_INTERVAL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	StartPeriodicSweep(cleaner, "ExecutionCleanupSweep", interval, func(ctx context.Context) error {
		return worker.RunCleanupOnce(ctx, "execution-cleanup-worker", 2*interval)
	})
}

// StartPeriodicSweep runs fn every interval until the cleaner stops it.
// One pass at a time: a slow pass (a wedged provider, a big batch) makes the
// next tick skip instead of piling up goroutines, and shutdown waits for the
// in-flight pass by closing the stop channel the loop selects on. The first
// pass runs after one interval, matching the temporary-document cleanup
// convention.
func StartPeriodicSweep(
	cleaner interfaces.ResourceCleaner,
	name string,
	interval time.Duration,
	fn PeriodicSweepFn,
) {
	if cleaner == nil || fn == nil || interval <= 0 {
		return
	}
	stop := make(chan struct{})
	var sweeping sync.Mutex
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Never overlap passes: a straggler sweep must not double
				// its own batch against itself.
				if !sweeping.TryLock() {
					log.Printf("[%s] previous sweep still running; skipping tick", name)
					continue
				}
				if err := fn(context.Background()); err != nil {
					log.Printf("[%s] sweep failed: %v", name, err)
				}
				sweeping.Unlock()
			case <-stop:
				return
			}
		}
	}()
	cleaner.RegisterWithName(name, func() error {
		close(stop)
		return nil
	})
}

// CraftLifecycleSweeper is the sweep surface the container drives. The O03
// *service.CraftLifecycle satisfies it; the narrow port keeps the container
// decoupled from the service assembly.
type CraftLifecycleSweeper interface {
	Sweep(ctx context.Context, limit int) error
}

// Craft lifecycle sweep environment switches (the CRAFT_* runtime
// convention, see craft_runtime.go):
//
//	CRAFT_LIFECYCLE_SWEEP_DISABLED=true  disables the periodic sweep (the
//	     tombstone entry and the guards stay available; only the automatic
//	     passes stop)
//	CRAFT_LIFECYCLE_SWEEP_INTERVAL       pass spacing, Go duration
//	     (default 10m)
//	CRAFT_LIFECYCLE_SWEEP_BATCH          batch size per pass
//	     (default craft.DefaultSweepBatchSize = 100)
const (
	craftLifecycleSweepDisabledEnv = "CRAFT_LIFECYCLE_SWEEP_DISABLED"
	craftLifecycleSweepIntervalEnv = "CRAFT_LIFECYCLE_SWEEP_INTERVAL"
	craftLifecycleSweepBatchEnv    = "CRAFT_LIFECYCLE_SWEEP_BATCH"
	// craftLifecycleDefaultInterval is the suggested initial pass spacing.
	craftLifecycleDefaultInterval = 10 * time.Minute
)

// StartCraftLifecycleSweep drives the O03 resource reclamation loop: every
// interval the sweep batches over the durable lifecycle ledger — tombstoned
// sessions, failed deletes awaiting retry, matured orphan-object candidates —
// and reclaims exactly what the protection rules allow (never an active,
// unknown, decision-pending or referenced resource). A nil service (craft not
// assembled, or the fail-closed deployment) starts nothing.
func StartCraftLifecycleSweep(svc CraftLifecycleSweeper, cleaner interfaces.ResourceCleaner) {
	if svc == nil || cleaner == nil {
		return
	}
	if parseCraftLifecycleBoolEnv(craftLifecycleSweepDisabledEnv) {
		log.Printf("[CraftLifecycle] periodic sweep disabled by %s", craftLifecycleSweepDisabledEnv)
		return
	}
	interval := craftLifecycleSweepInterval()
	batch := craftLifecycleSweepBatch()
	StartPeriodicSweep(cleaner, "CraftLifecycleSweep", interval,
		func(ctx context.Context) error {
			return svc.Sweep(ctx, batch)
		})
	log.Printf("[CraftLifecycle] periodic sweep started: interval=%s batch=%d", interval, batch)
}

func parseCraftLifecycleBoolEnv(name string) bool {
	return os.Getenv(name) == "1" || strings.EqualFold(os.Getenv(name), "true")
}

// craftLifecycleSweepInterval reads the pass spacing, falling back to the
// suggested default on a missing or malformed value.
func craftLifecycleSweepInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv(craftLifecycleSweepIntervalEnv)); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Printf("[CraftLifecycle] malformed %s=%q; using default %s",
			craftLifecycleSweepIntervalEnv, raw, craftLifecycleDefaultInterval)
	}
	return craftLifecycleDefaultInterval
}

// craftLifecycleSweepBatch reads the batch size, falling back to the default.
func craftLifecycleSweepBatch() int {
	if raw := strings.TrimSpace(os.Getenv(craftLifecycleSweepBatchEnv)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
		log.Printf("[CraftLifecycle] malformed %s=%q; using default %d",
			craftLifecycleSweepBatchEnv, raw, craft.DefaultSweepBatchSize)
	}
	return craft.DefaultSweepBatchSize
}
