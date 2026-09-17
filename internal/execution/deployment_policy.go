// Package execution contains lifecycle invariants shared by platform and
// remote execution cleanup workers. This file carries the W34 managed
// deployment policy: the code-side statement of which deployment topologies a
// multi-tenant (managed) node must never run with.
package execution

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// ErrManagedPolicyRejected is the sentinel returned when a deployment
// topology violates the managed-node isolation contract. Callers detect it
// with errors.Is; the wrapped context names the offending attribute.
var ErrManagedPolicyRejected = errors.New("managed_policy_rejected")

// DeploymentPolicy records the isolation-relevant attributes of one execution
// node deployment. It describes the REAL container/orchestration posture —
// populated from orchestration facts (compose inspection, node registration
// evidence), never from a config.yaml boolean: a config bool is a claim, not
// isolation evidence.
type DeploymentPolicy struct {
	// Privileged: the container runs privileged (host-equivalent). Never
	// acceptable for a managed node.
	Privileged bool
	// DockerSocket: the host Docker socket (or equivalent container runtime
	// control endpoint) is mounted writable into the node.
	DockerSocket bool
	// SharedTenantHome: execution workspaces of different tenants share one
	// writable home/root path instead of per-tenant isolated volumes.
	SharedTenantHome bool
	// EnforcedEgress: outbound network traffic is restricted by a real
	// enforcement layer (egress gateway allowlist or host network policy) —
	// not merely by application-level configuration.
	EnforcedEgress bool
}

// ValidateManagedPolicy applies the managed-node isolation contract. A nil
// receiver policy (zero value) is unsafe: EnforcedEgress defaults to false,
// so the fail-closed outcome is rejection.
func ValidateManagedPolicy(p DeploymentPolicy) error {
	switch {
	case p.Privileged:
		return fmt.Errorf("%w: privileged execution is forbidden on managed nodes", ErrManagedPolicyRejected)
	case p.DockerSocket:
		return fmt.Errorf("%w: host docker socket must not be mounted into managed nodes", ErrManagedPolicyRejected)
	case p.SharedTenantHome:
		return fmt.Errorf("%w: tenants must not share a writable home volume", ErrManagedPolicyRejected)
	case !p.EnforcedEgress:
		return fmt.Errorf("%w: outbound traffic must be restricted by an enforced egress boundary", ErrManagedPolicyRejected)
	}
	return nil
}

// W34 managed-workbench observability signals.
//
// Naming follows the in-process metrics conventions of internal/metrics
// (Prometheus-style names, low cardinality BY CONSTRUCTION): these five
// series carry NO labels at all. Run ids, tenant ids, command ids and every
// other unbounded identity belong to structured logs/traces where a reader
// can follow one incident — logs must carry correlation IDs only, never
// tokens, prompt text or file contents.
//
// Alarm wiring (deploy/mobile-workbench/README.md "Alerts" maps each series
// to its runbook section):
//   - execution_dispatch_unknown_total       — a dispatch outcome stayed
//     unknown past the crash-window fence; investigate the dispatch store.
//   - execution_stop_unconfirmed_total       — a stop did not confirm
//     before its drain/timeout budget; the target may still be running.
//   - execution_observer_age_seconds         — age of the oldest unresolved
//     remote observation; rising age means the observer loop is stuck.
//   - execution_settlement_backlog           — settlements waiting on
//     evidence; growing backlog delays cleanup (purge stays blocked).
//   - execution_notification_backlog          — notifications pending
//     delivery; growth means the notification lane is unhealthy.
type deploymentMetrics struct {
	dispatchUnknown     atomic.Int64 // counter
	stopUnconfirmed     atomic.Int64 // counter
	observerAgeNanos    atomic.Int64 // gauge (time.Duration)
	settlementBacklog   atomic.Int64 // gauge
	notificationBacklog atomic.Int64 // gauge
}

var deployment = new(deploymentMetrics)

// CountExecutionDispatchUnknown records one dispatch whose outcome remained
// unknown (crash window / fence expiry) instead of being fabricated as a
// success or failure.
func CountExecutionDispatchUnknown() { deployment.dispatchUnknown.Add(1) }

// CountExecutionStopUnconfirmed records one stop (drain/timeout) that ended
// without positive confirmation from the stopped side.
func CountExecutionStopUnconfirmed() { deployment.stopUnconfirmed.Add(1) }

// SetExecutionObserverAge records the age of the oldest unresolved remote
// observation, as refreshed by each observer pass.
func SetExecutionObserverAge(d time.Duration) {
	if d < 0 {
		d = 0
	}
	deployment.observerAgeNanos.Store(int64(d))
}

// SetExecutionSettlementBacklog records how many settlements are waiting on
// terminal evidence. Refreshed by the settlement sweep.
func SetExecutionSettlementBacklog(n int64) {
	if n < 0 {
		n = 0
	}
	deployment.settlementBacklog.Store(n)
}

// SetExecutionNotificationBacklog records how many notifications are pending
// delivery. Refreshed by the notification sweep.
func SetExecutionNotificationBacklog(n int64) {
	if n < 0 {
		n = 0
	}
	deployment.notificationBacklog.Store(n)
}

// ExecutionDeploymentMetricsSnapshot renders the W34 deployment health
// signals with Prometheus-style names for operations/diagnostics views.
func ExecutionDeploymentMetricsSnapshot() map[string]float64 {
	return map[string]float64{
		"execution_dispatch_unknown_total": float64(deployment.dispatchUnknown.Load()),
		"execution_stop_unconfirmed_total": float64(deployment.stopUnconfirmed.Load()),
		"execution_observer_age_seconds":   float64(deployment.observerAgeNanos.Load()) / float64(time.Second),
		"execution_settlement_backlog":     float64(deployment.settlementBacklog.Load()),
		"execution_notification_backlog":   float64(deployment.notificationBacklog.Load()),
	}
}

// ResetExecutionDeploymentMetrics clears every W34 deployment metric. For
// tests and operators only.
func ResetExecutionDeploymentMetrics() {
	deployment.dispatchUnknown.Store(0)
	deployment.stopUnconfirmed.Store(0)
	deployment.observerAgeNanos.Store(0)
	deployment.settlementBacklog.Store(0)
	deployment.notificationBacklog.Store(0)
}
