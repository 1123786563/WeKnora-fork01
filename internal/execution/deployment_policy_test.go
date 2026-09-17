package execution

import (
	"errors"
	"testing"
	"time"
)

// TestManagedNodeRejectsSharedPrivileges is the W34 RED-first contract: a
// managed (multi-tenant) deployment node must never run with privileges that
// would let one tenant's execution observe or control another's, and must
// have real egress enforcement rather than a configuration-only promise.
func TestManagedNodeRejectsSharedPrivileges(t *testing.T) {
	for _, p := range []DeploymentPolicy{
		{Privileged: true, EnforcedEgress: true},
		{DockerSocket: true, EnforcedEgress: true},
		{SharedTenantHome: true, EnforcedEgress: true},
		{EnforcedEgress: false},
	} {
		if ValidateManagedPolicy(p) == nil {
			t.Fatal("unsafe managed deployment")
		}
	}
	if err := ValidateManagedPolicy(DeploymentPolicy{EnforcedEgress: true}); err != nil {
		t.Fatal(err)
	}
}

// TestValidateManagedPolicyReturnsSentinel keeps the rejection detectable by
// callers (dispatch/registration seams) through errors.Is instead of string
// matching.
func TestValidateManagedPolicyReturnsSentinel(t *testing.T) {
	err := ValidateManagedPolicy(DeploymentPolicy{Privileged: true, EnforcedEgress: true})
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !errors.Is(err, ErrManagedPolicyRejected) {
		t.Fatalf("expected ErrManagedPolicyRejected, got %v", err)
	}
}

// TestExecutionDeploymentMetricsSurface pins the W34 observability contract:
// the five managed-workbench health signals must exist as low-cardinality
// metric names (no labels — identities stay in structured logs).
func TestExecutionDeploymentMetricsSurface(t *testing.T) {
	ResetExecutionDeploymentMetrics()
	defer ResetExecutionDeploymentMetrics()

	CountExecutionDispatchUnknown()
	CountExecutionDispatchUnknown()
	CountExecutionStopUnconfirmed()
	SetExecutionObserverAge(90 * time.Second)
	SetExecutionSettlementBacklog(7)
	SetExecutionNotificationBacklog(3)

	snap := ExecutionDeploymentMetricsSnapshot()
	want := map[string]float64{
		"execution_dispatch_unknown_total": 2,
		"execution_stop_unconfirmed_total": 1,
		"execution_observer_age_seconds":   90,
		"execution_settlement_backlog":     7,
		"execution_notification_backlog":   3,
	}
	if len(snap) != len(want) {
		t.Fatalf("expected exactly %d metric series, got %d: %v", len(want), len(snap), snap)
	}
	for name, value := range want {
		got, ok := snap[name]
		if !ok {
			t.Fatalf("missing metric %q in snapshot %v", name, snap)
		}
		if got != value {
			t.Fatalf("metric %q = %v, want %v", name, got, value)
		}
	}
}

// TestExecutionDeploymentMetricsReset ensures operators and tests can zero
// the gauges between drills without restarting the process.
func TestExecutionDeploymentMetricsReset(t *testing.T) {
	CountExecutionDispatchUnknown()
	SetExecutionSettlementBacklog(5)
	ResetExecutionDeploymentMetrics()
	snap := ExecutionDeploymentMetricsSnapshot()
	for name, value := range snap {
		if value != 0 {
			t.Fatalf("metric %q = %v after reset, want 0", name, value)
		}
	}
}
