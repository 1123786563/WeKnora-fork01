package commercial

import "testing"

func TestBillingAccessDoesNotInheritAdmin(t *testing.T) {
	if CanManageBilling("admin", true, false) {
		t.Fatal("admin inherited billing")
	}
	if !CanManageBilling("viewer", true, true) {
		t.Fatal("explicit grant ignored")
	}
	if CanManageBilling("owner", false, true) {
		t.Fatal("inactive member accepted")
	}
}
