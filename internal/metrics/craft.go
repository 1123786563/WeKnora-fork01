// Package metrics hosts WeKnora's in-process operational metrics.
//
// O04 craft observability contract (docs/operations/craft-observability.md):
// every metric here is LOW CARDINALITY BY CONSTRUCTION — each label dimension
// is a closed vocabulary validated at the recording seam, and unbounded
// identities (run id, tenant id, session id, tool call id, delegation id,
// prompt id) NEVER enter a metric label. Those identities belong to the
// controlled structured logs and trace attributes, where a reader can follow
// one incident without exploding the metrics space.
package metrics

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Closed label vocabularies. A value outside the vocabulary is folded onto
// "other" instead of being rejected or creating a new series: metrics must
// never fail the call path, and never grow without bound either.
var (
	craftDelegationStatuses = map[string]bool{"succeeded": true, "failed": true, "canceled": true}
	craftReconcileOutcomes  = map[string]bool{"corrected": true, "matched": true, "mismatched": true}
)

// craftRestoreBuckets are the fixed upper bounds (seconds) of the workspace
// restore histogram. Restores that exceed the last bound land in +Inf.
var craftRestoreBuckets = []float64{0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120}

type craftMetrics struct {
	mu sync.Mutex

	delegations map[string]*atomic.Int64 // craft_delegations_total{status}
	reconciles  map[string]*atomic.Int64 // craft_reconcile_total{outcome}

	pendingDecisions atomic.Int64 // craft_pending_decisions (gauge)

	previewFailures atomic.Int64 // craft_preview_failures_total
	usageUnknown    atomic.Int64 // craft_usage_unknown_total

	restoreCounts []atomic.Int64 // craft_workspace_restore_seconds buckets
	restoreInf    atomic.Int64
	restoreSum    atomic.Int64 // milliseconds, exact
	restoreCount  atomic.Int64
}

var craft = newCraftMetrics()

func newCraftMetrics() *craftMetrics {
	return &craftMetrics{
		delegations: map[string]*atomic.Int64{},
		reconciles:  map[string]*atomic.Int64{},
	}
}

func craftCounter(m map[string]*atomic.Int64, vocabulary map[string]bool, value string) *atomic.Int64 {
	if !vocabulary[value] {
		value = "other"
	}
	craft.mu.Lock()
	defer craft.mu.Unlock()
	c, ok := m[value]
	if !ok {
		c = &atomic.Int64{}
		m[value] = c
	}
	return c
}

// CraftDelegationSettled counts one settled craft delegation by its terminal
// status (succeeded/failed/canceled). Callers count only after the outcome is
// durably persisted (see CraftDelegateService.settle), so a crash between
// classification and persist does not double-count on retry. Two concurrent
// settles of the same delegation remain possible to observe as two settle
// EVENTS (the store's identical-result replay is idempotent); the counter is
// a settle-event counter, not an exactly-once per-delegation census.
func CraftDelegationSettled(status string) {
	craftCounter(craft.delegations, craftDelegationStatuses, status).Add(1)
}

// CraftReconciled counts one usage reconciliation outcome: a late correction
// filed as an explicit revision (corrected), an OC aggregate that matched the
// recorded child ledger (matched), or one that honestly did not (mismatched).
func CraftReconciled(outcome string) {
	craftCounter(craft.reconciles, craftReconcileOutcomes, outcome).Add(1)
}

// SetCraftPendingDecisions sets the gauge of craft interactions currently
// waiting for a user decision. Refreshed by each lifecycle sweep pass.
func SetCraftPendingDecisions(n int64) { craft.pendingDecisions.Store(n) }

// CraftPreviewFailure counts one failed craft preview issuance (the HTTP
// seam answers an error instead of a ticket).
func CraftPreviewFailure() { craft.previewFailures.Add(1) }

// CraftUsageUnknown counts one physical model attempt whose usage could not
// be observed (stream break, cancellation) — the ledger line that must stay
// visible instead of being fabricated as zero.
func CraftUsageUnknown() { craft.usageUnknown.Add(1) }

// ObserveCraftWorkspaceRestore records the wall-clock duration of one
// snapshot workspace restore.
func ObserveCraftWorkspaceRestore(d time.Duration) {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	idx := sort.SearchFloat64s(craftRestoreBuckets, float64(ms)/1000)
	craft.mu.Lock()
	for len(craft.restoreCounts) <= idx && idx < len(craftRestoreBuckets) {
		craft.restoreCounts = append(craft.restoreCounts, atomic.Int64{})
	}
	// Copy the slice header UNDER the lock (O04 review nit-1): another
	// observer's locked append may reallocate the shared header, so reading
	// craft.restoreCounts[idx] outside the lock raced. The local header is
	// stable, and appends never mutate existing elements — the atomic bucket
	// it points at is safe to increment without the lock.
	counts := craft.restoreCounts
	craft.mu.Unlock()
	if idx >= len(craftRestoreBuckets) {
		craft.restoreInf.Add(1)
	} else {
		counts[idx].Add(1)
	}
	craft.restoreSum.Add(ms)
	craft.restoreCount.Add(1)
}

// CraftMetricsSnapshot renders every craft metric with Prometheus-style
// names as an operations/diagnostics view. Label values are only the closed
// vocabularies above (plus "other"), so the snapshot size is bounded no
// matter how busy the system is.
func CraftMetricsSnapshot() map[string]float64 {
	craft.mu.Lock()
	delegationPtrs := make(map[string]*atomic.Int64, len(craft.delegations))
	for k, v := range craft.delegations {
		delegationPtrs[k] = v
	}
	reconcilePtrs := make(map[string]*atomic.Int64, len(craft.reconciles))
	for k, v := range craft.reconciles {
		reconcilePtrs[k] = v
	}
	buckets := append([]atomic.Int64(nil), craft.restoreCounts...)
	inf, sum, count := craft.restoreInf.Load(), craft.restoreSum.Load(), craft.restoreCount.Load()
	craft.mu.Unlock()

	out := map[string]float64{}
	for k, v := range delegationPtrs {
		out["craft_delegations_total{status="+strconv.Quote(k)+"}"] = float64(v.Load())
	}
	for k, v := range reconcilePtrs {
		out["craft_reconcile_total{outcome="+strconv.Quote(k)+"}"] = float64(v.Load())
	}
	out["craft_pending_decisions"] = float64(craft.pendingDecisions.Load())
	out["craft_preview_failures_total"] = float64(craft.previewFailures.Load())
	out["craft_usage_unknown_total"] = float64(craft.usageUnknown.Load())
	var cumulative float64
	for i := range buckets {
		cumulative += float64(buckets[i].Load())
		out["craft_workspace_restore_seconds_bucket{le="+strconv.Quote(formatBucket(i))+"}"] = cumulative
	}
	out["craft_workspace_restore_seconds_bucket{le="+strconv.Quote("+Inf")+"}"] = cumulative + float64(inf)
	out["craft_workspace_restore_seconds_sum"] = float64(sum) / 1000
	out["craft_workspace_restore_seconds_count"] = float64(count)
	return out
}

func formatBucket(i int) string {
	if i >= len(craftRestoreBuckets) {
		return "+Inf"
	}
	b := craftRestoreBuckets[i]
	if b == float64(int64(b)) {
		return strconv.FormatInt(int64(b), 10)
	}
	return strconv.FormatFloat(b, 'g', -1, 64)
}

// ResetCraftMetrics clears every craft metric. For tests and operators only.
func ResetCraftMetrics() {
	craft.mu.Lock()
	defer craft.mu.Unlock()
	craft.delegations = map[string]*atomic.Int64{}
	craft.reconciles = map[string]*atomic.Int64{}
	craft.pendingDecisions.Store(0)
	craft.previewFailures.Store(0)
	craft.usageUnknown.Store(0)
	craft.restoreCounts = nil
	craft.restoreInf.Store(0)
	craft.restoreSum.Store(0)
	craft.restoreCount.Store(0)
}
