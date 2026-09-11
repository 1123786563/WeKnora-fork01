package appconnector

import (
	"testing"
	"time"
)

// TestOAuthCallbackCannotRebindSpace is the brief's Step 1 failing assertion,
// kept verbatim in behavior: a binding state issued inside one space can
// never be redeemed against another space, and a consumed state can never be
// replayed.
func TestOAuthCallbackCannotRebindSpace(t *testing.T) {
	b := OAuthBinding{State: "random", TenantID: 7, ActorID: "u1", ExpiresAt: time.Now().Add(time.Minute)}
	if ValidateOAuthBinding(b, 8, "u1", time.Now()) == nil {
		t.Fatal("cross-space OAuth accepted")
	}
	b.Used = true
	if ValidateOAuthBinding(b, 7, "u1", time.Now()) == nil {
		t.Fatal("state replay accepted")
	}
}

// TestOAuthBindingExpiryRejected pins that a state expiring exactly now is
// already invalid: the window is strictly now.Before(ExpiresAt).
func TestOAuthBindingExpiryRejected(t *testing.T) {
	now := time.Now()
	b := OAuthBinding{State: "random", TenantID: 7, ActorID: "u1", ExpiresAt: now}
	if ValidateOAuthBinding(b, 7, "u1", now) == nil {
		t.Fatal("expired binding accepted")
	}
	b.ExpiresAt = now.Add(-time.Second)
	if ValidateOAuthBinding(b, 7, "u1", now) == nil {
		t.Fatal("already-expired binding accepted")
	}
}

// TestOAuthBindingWrongActorRejected pins that only the initiator may redeem
// their own state.
func TestOAuthBindingWrongActorRejected(t *testing.T) {
	b := OAuthBinding{State: "random", TenantID: 7, ActorID: "u1", ExpiresAt: time.Now().Add(time.Minute)}
	if ValidateOAuthBinding(b, 7, "attacker", time.Now()) == nil {
		t.Fatal("wrong actor accepted")
	}
}

// TestOAuthBindingEmptyStateRejected pins that a missing state value is never
// redeemable.
func TestOAuthBindingEmptyStateRejected(t *testing.T) {
	b := OAuthBinding{State: "", TenantID: 7, ActorID: "u1", ExpiresAt: time.Now().Add(time.Minute)}
	if ValidateOAuthBinding(b, 7, "u1", time.Now()) == nil {
		t.Fatal("empty state accepted")
	}
}

// TestOAuthBindingValidPasses pins the happy path so the validator cannot
// drift into rejecting everything.
func TestOAuthBindingValidPasses(t *testing.T) {
	b := OAuthBinding{State: "random", InstallationID: "inst-1", TenantID: 7, ActorID: "u1", ExpiresAt: time.Now().Add(time.Minute)}
	if err := ValidateOAuthBinding(b, 7, "u1", time.Now()); err != nil {
		t.Fatalf("valid binding rejected: %v", err)
	}
}
