package craft

import "testing"

// TestSameScopeIncludesUserAndTenant is the R02 RED-first contract test: the
// execution binding identity must include both the tenant and the user.
func TestSameScopeIncludesUserAndTenant(t *testing.T) {
	a := Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	b := a
	b.TenantID = 2
	if SameScope(a, b) {
		t.Fatal("cross tenant accepted")
	}
	b = a
	b.UserID = "u2"
	if SameScope(a, b) {
		t.Fatal("cross owner accepted")
	}
}

func TestSameScopeAcceptsIdenticalCompleteScope(t *testing.T) {
	a := Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	if !SameScope(a, a) {
		t.Fatal("identical scope rejected")
	}
	b := a
	b.SessionID = ""
	if SameScope(a, b) {
		t.Fatal("incomplete scope accepted")
	}
}
