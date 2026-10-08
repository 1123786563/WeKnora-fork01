package appconnector

import "testing"

// TestPersonalConnectionCannotBeSharedByInstallation is the brief's Step 1
// failing assertion, kept verbatim in behavior: a personal connection is
// owner-only, tenant-scoped, and independent of any space grant.
func TestPersonalConnectionCannotBeSharedByInstallation(t *testing.T) {
	c := Connection{TenantID: 7, Kind: "personal", OwnerID: "u1", State: "active"}
	if CanUseConnection(c, 7, "u2", true) {
		t.Fatal("personal token shared")
	}
	if CanUseConnection(c, 8, "u1", true) {
		t.Fatal("cross-space token used")
	}
	if !CanUseConnection(c, 7, "u1", false) {
		t.Fatal("owner rejected")
	}
}

func TestInstallationSpaceConnectionAuthorizationMatrix(t *testing.T) {
	space := Connection{TenantID: 7, Kind: "space", OwnerID: "u1", State: "active"}
	cases := []struct {
		name       string
		tenant     uint64
		actor      string
		spaceGrant bool
		want       bool
	}{
		{"granted member", 7, "u2", true, true},
		{"owner without grant", 7, "u1", false, false},
		{"member without explicit grant", 7, "u3", false, false},
		{"grant across tenant boundary", 8, "u2", true, false},
	}
	for _, tc := range cases {
		if got := CanUseConnection(space, tc.tenant, tc.actor, tc.spaceGrant); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestInstallationConnectionDisabledStateRejected(t *testing.T) {
	states := []string{"revoked", "pending_reauthorization", ""}
	for _, state := range states {
		personal := Connection{TenantID: 7, Kind: "personal", OwnerID: "u1", State: state}
		if CanUseConnection(personal, 7, "u1", false) {
			t.Fatalf("personal state %q usable", state)
		}
		space := Connection{TenantID: 7, Kind: "space", OwnerID: "u1", State: state}
		if CanUseConnection(space, 7, "u2", true) {
			t.Fatalf("space state %q usable", state)
		}
	}
}

func TestInstallationConnectionCrossTenantRejected(t *testing.T) {
	personal := Connection{TenantID: 7, Kind: "personal", OwnerID: "u1", State: "active"}
	space := Connection{TenantID: 7, Kind: "space", OwnerID: "u1", State: "active"}
	if CanUseConnection(personal, 8, "u1", false) {
		t.Fatal("personal connection used across tenants")
	}
	if CanUseConnection(space, 8, "u2", true) {
		t.Fatal("space connection used across tenants")
	}
}

func TestInstallationConnectionUnknownKindRejected(t *testing.T) {
	for _, kind := range []string{"shared", "org", ""} {
		c := Connection{TenantID: 7, Kind: kind, OwnerID: "u1", State: "active"}
		if CanUseConnection(c, 7, "u1", true) {
			t.Fatalf("unknown kind %q usable", kind)
		}
	}
}

func TestInstallationPermissionSeparateFromBilling(t *testing.T) {
	// Owner/Admin may install or upgrade; regular members may only request.
	// This permission is deliberately distinct from commercial billing
	// permission (it never consults CanManageBilling).
	allowed := map[string]bool{
		"owner":       true,
		"admin":       true,
		"member":      false,
		"contributor": false,
		"viewer":      false,
		"":            false,
	}
	for role, want := range allowed {
		if got := CanInstallInstallation(role); got != want {
			t.Fatalf("role %q: got %v want %v", role, got, want)
		}
	}
}
