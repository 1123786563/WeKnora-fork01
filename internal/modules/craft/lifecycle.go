package craft

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// O03: Sandbox residency, snapshots and resource reclamation — the pure
// rules. The product constraint is absolute: ACTIVE, UNKNOWN and
// DECISION-PENDING resources are never reclaimed, and generation locks and
// reference protection stay effective. Reclamation decisions always derive
// from the real run relationships; a caller-supplied boolean is never the
// only fact.

// Lifecycle resource kinds recorded by the sweep.
const (
	// LifecycleResourceSandbox is one session's bound remote sandbox.
	LifecycleResourceSandbox = "sandbox"
	// LifecycleResourceObject is one storage object no longer referenced by
	// any version, snapshot or workspace input (an orphan).
	LifecycleResourceObject = "object"
)

// Lifecycle state vocabulary of a swept resource: candidate waits out its
// eligibility window, deleting is the compare-and-swat mark that blocks new
// dispatch/restore while the provider delete runs, deleted is terminal
// success, failed keeps the retry record for the next sweep, kept records a
// resource the sweep deliberately did not reclaim, and risk records an
// unreclaimable resource with the concrete risk (an unknown remote task).
const (
	LifecycleStateCandidate = "candidate"
	LifecycleStateDeleting  = "deleting"
	LifecycleStateDeleted   = "deleted"
	LifecycleStateFailed    = "failed"
	LifecycleStateKept      = "kept"
	LifecycleStateRisk      = "risk"
)

// Default policy values — configurable initial suggestions, not secrets of
// the universe. Dormancy waits for 30 minutes of sandbox idleness, an orphan
// object is recorded as a candidate for 24 hours before its references are
// re-checked, and one sweep batch processes at most 100 resources.
const (
	DefaultSandboxIdleThreshold  = 30 * time.Minute
	DefaultOrphanCandidateWindow = 24 * time.Hour
	DefaultSweepBatchSize        = 100
	// DefaultSweepRetryBackoff spaces retries of a failed provider delete so
	// a wedged provider is not hammered on every sweep tick.
	DefaultSweepRetryBackoff = 5 * time.Minute
)

// LifecyclePolicy is the deployment-owned configuration of the lifecycle
// sweep. The zero value normalizes to the defaults above.
type LifecyclePolicy struct {
	// SandboxIdleThreshold is how long a sandbox must be idle before it may
	// be dormanted (paused) — only with a verified complete snapshot.
	SandboxIdleThreshold time.Duration
	// OrphanCandidateWindow is how long an orphan object stays a recorded
	// candidate before its references are re-checked and it may be deleted.
	OrphanCandidateWindow time.Duration
	// SweepBatchSize bounds one Sweep batch; <= 0 normalizes to 100.
	SweepBatchSize int
	// SweepRetryBackoff spaces retries of failed deletions.
	SweepRetryBackoff time.Duration
}

// Normalize fills every unset field with its default suggestion.
func (p LifecyclePolicy) Normalize() LifecyclePolicy {
	if p.SandboxIdleThreshold <= 0 {
		p.SandboxIdleThreshold = DefaultSandboxIdleThreshold
	}
	if p.OrphanCandidateWindow <= 0 {
		p.OrphanCandidateWindow = DefaultOrphanCandidateWindow
	}
	if p.SweepBatchSize <= 0 {
		p.SweepBatchSize = DefaultSweepBatchSize
	}
	if p.SweepRetryBackoff <= 0 {
		p.SweepRetryBackoff = DefaultSweepRetryBackoff
	}
	return p
}

// ResourceState is the reclaim verdict input of ONE resource, derived from
// the real run relationships at decision time: Active marks a live main run,
// Unknown an outcome-undetermined sub-execution, DecisionPending a parked
// human decision, Referenced a live version/snapshot/input reference.
// EligibleAt is the earliest moment reclamation may even be considered.
type ResourceState struct {
	Active, Unknown, DecisionPending, Referenced bool
	EligibleAt                                   time.Time
}

// CanDeleteResource reports whether the resource may be reclaimed right now.
// Nothing live, nothing undetermined, nothing decision-pending, nothing
// referenced — and only once the eligibility window has fully elapsed.
func CanDeleteResource(s ResourceState, now time.Time) bool {
	return !s.Active && !s.Unknown && !s.DecisionPending && !s.Referenced && !now.Before(s.EligibleAt)
}

// DormancyDecision is the verdict of the sandbox dormancy policy.
type DormancyDecision struct {
	// MayDormant reports the sandbox may be paused now.
	MayDormant bool
	// NeedsRestart is always true when MayDormant: a dormanted sandbox loses
	// its process state; resuming is an explicit restart, never a resume of
	// running processes.
	NeedsRestart bool
	// Reason names what blocked dormancy (empty when it is allowed).
	Reason string
}

// SandboxDormancy decides whether one sandbox may be dormanted (paused).
// Dormancy is a STORAGE policy, strictly separated from deletion: it demands
// idleness beyond the threshold AND a verified complete recovery snapshot
// (the C05 files+session quiescent capture) — pausing the only copy of the
// OpenCode session state is never allowed. An idle sandbox without a
// verified snapshot stays running until one is captured. Artifacts and
// snapshots are NEVER deleted by dormancy: versions follow the session
// retention policy, not the sandbox TTL.
func SandboxDormancy(idleFor time.Duration, snapshotVerified bool, policy LifecyclePolicy) DormancyDecision {
	p := policy.Normalize()
	if idleFor < p.SandboxIdleThreshold {
		return DormancyDecision{Reason: fmt.Sprintf(
			"sandbox idle for %s, threshold is %s", idleFor, p.SandboxIdleThreshold)}
	}
	if !snapshotVerified {
		return DormancyDecision{Reason: "no verified complete snapshot; pausing would risk the only copy of the session state"}
	}
	return DormancyDecision{MayDormant: true, NeedsRestart: true}
}

// TTLRisk is the early workspace risk surfaced when a provider sandbox
// cannot be kept alive until the next verified snapshot.
type TTLRisk struct {
	AtRisk bool
	Reason string
}

// ProviderTTLRisk decides whether the workspace must be shown a sandbox-TTL
// risk NOW: the provider's TTL ends before the sandbox could accumulate the
// configured idle window again AND the TTL cannot be extended, or the TTL is
// unknown — a container is never assumed to live forever. An extendable or
// far-future TTL is not a risk.
func ProviderTTLRisk(ttlExpiresAt time.Time, extendable bool, now time.Time, policy LifecyclePolicy) TTLRisk {
	p := policy.Normalize()
	if ttlExpiresAt.IsZero() {
		return TTLRisk{AtRisk: true, Reason: "provider TTL is unknown; the sandbox must not be assumed permanent — capture a verified snapshot"}
	}
	if now.Before(ttlExpiresAt) && ttlExpiresAt.Sub(now) > p.SandboxIdleThreshold {
		return TTLRisk{}
	}
	if extendable {
		return TTLRisk{}
	}
	return TTLRisk{AtRisk: true, Reason: fmt.Sprintf(
		"provider TTL ends at %s and cannot be extended; the sandbox (and any unsnapshotted session state) disappears with it",
		ttlExpiresAt.UTC().Format(time.RFC3339))}
}

// Lifecycle usage event kinds. Sandbox start/stop pair up into dwell time;
// storage observations carry the bytes held at that moment.
const (
	LifecycleEventSandboxStart = "sandbox_start"
	LifecycleEventSandboxStop  = "sandbox_stop"
	LifecycleEventStorageBytes = "storage_bytes"
)

// LifecycleEventKey is the dedup identity of ONE lifecycle usage event per
// tenant: the same (tenant, sandbox, kind, moment) redelivered — an
// at-least-once observation, a worker replay — keys identically and is
// counted once; a different moment or a different sandbox is a new fact.
func LifecycleEventKey(tenant uint64, sandboxID, kind string, occurredAt time.Time) string {
	raw, _ := json.Marshal([]string{
		strconv.FormatUint(tenant, 10), sandboxID, kind,
		strconv.FormatInt(occurredAt.UTC().UnixNano(), 10),
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// SandboxDwell measures one sandbox's residency from its real start and stop
// moments. A stop before its start is caller corruption, not negative usage.
func SandboxDwell(start, stop time.Time) (time.Duration, error) {
	if start.IsZero() {
		return 0, fmt.Errorf("%w: dwell requires a start moment", ErrInvalidInput)
	}
	if stop.IsZero() {
		return 0, fmt.Errorf("%w: dwell requires a stop moment", ErrInvalidInput)
	}
	if stop.Before(start) {
		return 0, fmt.Errorf("%w: dwell stop %s precedes start %s", ErrInvalidInput, stop, start)
	}
	return stop.Sub(start), nil
}

// BytesDay converts bytes held for a dwell into storage byte-days: the
// storage-cost unit of account. One byte held one full day is one byte-day;
// a partial day is a partial byte-day.
func BytesDay(bytes int64, dwell time.Duration) float64 {
	if bytes <= 0 || dwell <= 0 {
		return 0
	}
	return float64(bytes) * dwell.Hours() / 24
}

// Lifecycle actions the quota gate distinguishes.
const (
	// LifecycleActionStartSandbox admits a NEW sandbox for a session.
	LifecycleActionStartSandbox = "start_sandbox"
	// LifecycleActionDownload reads an already-authorized artifact.
	LifecycleActionDownload = "download"
	// LifecycleActionCleanup reclaims resources.
	LifecycleActionCleanup = "cleanup"
)

// QuotaAllows reports whether the action may proceed under the quota state.
// An over-limit quota gates ONLY new sandbox starts: existing authorized
// downloads and cleanup stay available, so an over-limit workspace can
// always shed resources instead of being locked in with them.
func QuotaAllows(action string, sandboxOverLimit, storageOverLimit bool) bool {
	if action != LifecycleActionStartSandbox {
		return true
	}
	return !sandboxOverLimit && !storageOverLimit
}
