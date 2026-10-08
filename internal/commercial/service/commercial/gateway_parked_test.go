package commercial

import (
	"context"
	"errors"
	"testing"

	domain "github.com/Tencent/WeKnora/internal/commercial"
)

// #105: the OpenMeter gateway is removed; the rail is parked fail-closed.
// Every method must return exactly ErrGatewayUnconfigured so downstream
// postures (pending/attention, never success, never a guessed replay)
// match the pre-removal unconfigured behavior byte for byte.
func TestParkedGatewayFailsClosedOnEveryMethod(t *testing.T) {
	g := NewParkedGateway()
	ctx := context.Background()

	if _, err := g.ApplyBenefit(ctx, domain.BenefitRequest{Key: "k"}); !errors.Is(err, domain.ErrGatewayUnconfigured) {
		t.Fatalf("ApplyBenefit: got %v, want ErrGatewayUnconfigured", err)
	}
	if _, err := g.FindBenefit(ctx, "k"); !errors.Is(err, domain.ErrGatewayUnconfigured) {
		t.Fatalf("FindBenefit: got %v, want ErrGatewayUnconfigured", err)
	}
	if err := g.RevokeBenefit(ctx, "k", domain.Credits(1)); !errors.Is(err, domain.ErrGatewayUnconfigured) {
		t.Fatalf("RevokeBenefit: got %v, want ErrGatewayUnconfigured", err)
	}
	if _, err := g.Settle(ctx, domain.Settlement{ID: "s"}); !errors.Is(err, domain.ErrGatewayUnconfigured) {
		t.Fatalf("Settle: got %v, want ErrGatewayUnconfigured", err)
	}
	if _, err := g.ConfirmSettlement(ctx, "s"); !errors.Is(err, domain.ErrGatewayUnconfigured) {
		t.Fatalf("ConfirmSettlement: got %v, want ErrGatewayUnconfigured", err)
	}
}

// ClassifyFulfillment must keep reading the parked error as unknown (hold
// in attention), never as refusal (a business failure) — the same reading
// the unconfigured gateway had.
func TestParkedGatewayClassifiesAsUnknownNotRefused(t *testing.T) {
	if got := domain.ClassifyFulfillment(domain.ErrGatewayUnconfigured); got != domain.FulfillmentUnknown {
		t.Fatalf("classification = %v, want FulfillmentUnknown", got)
	}
}
