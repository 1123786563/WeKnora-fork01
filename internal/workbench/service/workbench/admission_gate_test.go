package workbench

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

func w34GateBoolPtr(v bool) *bool { return &v }

// TestWorkbenchCapabilityGateDrainsAndClosesPlatform pins the W34 gate
// factory used by the container assembly: drain closes every target lane,
// the platform switch closes only the platform lane, and an unset config
// keeps admission open.
func TestWorkbenchCapabilityGateDrainsAndClosesPlatform(t *testing.T) {
	if err := NewWorkbenchCapabilityGate(nil)("platform"); err != nil {
		t.Fatalf("nil config must keep admission open, got %v", err)
	}
	if err := NewWorkbenchCapabilityGate(&config.Config{})("platform"); err != nil {
		t.Fatalf("unset workbench section must keep admission open, got %v", err)
	}

	draining := &config.Config{Workbench: &config.WorkbenchConfig{WorkerDrain: w34GateBoolPtr(true)}}
	if err := NewWorkbenchCapabilityGate(draining)("platform"); !errors.Is(err, ErrAdmissionDraining) {
		t.Fatalf("expected ErrAdmissionDraining, got %v", err)
	}

	platformClosed := &config.Config{Workbench: &config.WorkbenchConfig{PlatformAdmission: w34GateBoolPtr(false)}}
	if err := NewWorkbenchCapabilityGate(platformClosed)("platform"); !errors.Is(err, ErrPlatformAdmissionClosed) {
		t.Fatalf("expected ErrPlatformAdmissionClosed, got %v", err)
	}
	// The platform switch must not leak onto other target lanes.
	if err := NewWorkbenchCapabilityGate(platformClosed)("other"); err != nil {
		t.Fatalf("platform switch must not close other lanes, got %v", err)
	}
}

// TestAdmissionCoordinatorConsultsGateBeforePersisting proves the production
// admission path consumes the gate: a draining deployment rejects Start
// before identity, budget or durable writes, while an open gate lets the
// call proceed to the (here unconfigured) coordinator check.
func TestAdmissionCoordinatorConsultsGateBeforePersisting(t *testing.T) {
	draining := NewAdmissionCoordinator(nil, nil, nil, nil)
	draining.SetAdmissionGate(NewWorkbenchCapabilityGate(
		&config.Config{Workbench: &config.WorkbenchConfig{WorkerDrain: w34GateBoolPtr(true)}},
	))
	_, err := draining.Start(context.Background(), StartInput{SessionID: "s1", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	if !errors.Is(err, ErrAdmissionDraining) {
		t.Fatalf("expected ErrAdmissionDraining from Start, got %v", err)
	}

	// Same coordinator shape, open gate: the failure must be the ordinary
	// unconfigured-coordinator error, never a gate error.
	open := NewAdmissionCoordinator(nil, nil, nil, nil)
	open.SetAdmissionGate(NewWorkbenchCapabilityGate(nil))
	_, err = open.Start(context.Background(), StartInput{SessionID: "s1", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	if err == nil || errors.Is(err, ErrAdmissionDraining) || errors.Is(err, ErrPlatformAdmissionClosed) {
		t.Fatalf("expected unconfigured-coordinator failure, got %v", err)
	}

	// No gate installed: legacy behaviour is untouched.
	legacy := NewAdmissionCoordinator(nil, nil, nil, nil)
	_, err = legacy.Start(context.Background(), StartInput{SessionID: "s1", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	if err == nil {
		t.Fatal("expected unconfigured-coordinator failure for gate-less coordinator")
	}
}
