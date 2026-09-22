package commercial

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestFulfillmentKeySeparatesOrderLines(t *testing.T) {
	if FulfillmentKey("o1", "l1") == FulfillmentKey("o1", "l2") {
		t.Fatal("line collision")
	}
	if FulfillmentKey("o1", "l1") != FulfillmentKey("o1", "l1") {
		t.Fatal("unstable")
	}
}

func TestFulfillmentKeySeparatesOrdersAndResistsSeparatorInjection(t *testing.T) {
	if FulfillmentKey("o1", "l1") == FulfillmentKey("o2", "l1") {
		t.Fatal("order collision")
	}
	// ("o1\x00l1","") and ("o1","l1\x00") must not both collapse onto
	// ("o1","l1"): the separator byte is unambiguous in exactly one position.
	if FulfillmentKey("o1\x00l1", "") == FulfillmentKey("o1", "l1") {
		t.Fatal("separator injection collision")
	}
	if FulfillmentKey("o1", "l1\x00") == FulfillmentKey("o1", "l1") {
		t.Fatal("trailing separator collision")
	}
	if got := FulfillmentKey("o1", "l1"); len(got) < len("fulfill:") || got[:len("fulfill:")] != "fulfill:" {
		t.Fatalf("key %q missing fulfill: namespace prefix", got)
	}
}

func TestFulfillmentBenefitRequestValidation(t *testing.T) {
	valid := BenefitRequest{
		Key:         FulfillmentKey("o1", "l1"),
		TenantID:    7,
		CustomerID:  "cust_1",
		Kind:        BenefitKindTopUp,
		PlanRef:     "pro",
		Credits:     Credits(1_000_000),
		EffectiveAt: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	cases := map[string]func(BenefitRequest) BenefitRequest{
		"empty key":        func(r BenefitRequest) BenefitRequest { r.Key = ""; return r },
		"zero tenant":      func(r BenefitRequest) BenefitRequest { r.TenantID = 0; return r },
		"empty customer":   func(r BenefitRequest) BenefitRequest { r.CustomerID = ""; return r },
		"empty kind":       func(r BenefitRequest) BenefitRequest { r.Kind = ""; return r },
		"unknown kind":     func(r BenefitRequest) BenefitRequest { r.Kind = "mystery"; return r },
		"zero credits":     func(r BenefitRequest) BenefitRequest { r.Credits = 0; return r },
		"negative credits": func(r BenefitRequest) BenefitRequest { r.Credits = -1; return r },
		"zero effective":   func(r BenefitRequest) BenefitRequest { r.EffectiveAt = time.Time{}; return r },
	}
	for name, mutate := range cases {
		if err := mutate(valid).Validate(); !errors.Is(err, ErrInvalidBenefitRequest) {
			t.Fatalf("%s: want ErrInvalidBenefitRequest, got %v", name, err)
		}
	}
}

func TestFulfillmentClassifyGatewayOutcome(t *testing.T) {
	if got := ClassifyFulfillment(nil); got != FulfillmentApplied {
		t.Fatalf("nil error must classify applied, got %v", got)
	}
	if got := ClassifyFulfillment(fmt.Errorf("wrap: %w", ErrBenefitNotFound)); got != FulfillmentMissing {
		t.Fatalf("not-found must classify missing, got %v", got)
	}
	if got := ClassifyFulfillment(fmt.Errorf("wrap: %w", ErrGatewayBusinessRefusal)); got != FulfillmentRefused {
		t.Fatalf("business refusal must classify refused, got %v", got)
	}
	// A timeout is indeterminate: the remote may or may not have persisted.
	// It must never be treated as success and never as a definitive refusal.
	unknowns := []error{
		fmt.Errorf("wrap: %w", ErrGatewayIndeterminate),
		errors.New("context deadline exceeded"),
	}
	for _, err := range unknowns {
		got := ClassifyFulfillment(err)
		if got != FulfillmentUnknown {
			t.Fatalf("timeout %v must classify unknown, got %v", err, got)
		}
		if got == FulfillmentApplied {
			t.Fatal("business error classified as success")
		}
	}
}

func TestFulfillmentTopUpEffectiveAtFixedOnFirstSuccess(t *testing.T) {
	first := time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)
	retry := first.Add(48 * time.Hour)
	if got := FirstEffectiveAt(first, retry); !got.Equal(first) {
		t.Fatalf("first success must pin effective_at, got %v", got)
	}
	if got := FirstEffectiveAt(time.Time{}, retry); !got.Equal(retry) {
		t.Fatalf("unset effective_at must adopt the retry time, got %v", got)
	}
}
