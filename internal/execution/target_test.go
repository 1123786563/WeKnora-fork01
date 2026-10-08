package execution

import "testing"

func TestTargetCannotCrossSpace(t *testing.T) {
	target := Target{ID: "n", TenantID: 1, OwnerID: "u", Kind: "managed_node", State: "active", CredentialVersion: 1}
	if AuthorizeTarget(target, 2, "u") == nil {
		t.Fatal("cross-space target authorized")
	}
	if err := AuthorizeTarget(target, 1, "u"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeTargetRejectsWrongOwnerRevokedAndInvalidCredential(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		tenant uint64
		actor  string
	}{
		{"wrong owner", Target{TenantID: 1, OwnerID: "u1", State: "active", CredentialVersion: 1}, 1, "u2"},
		{"revoked", Target{TenantID: 1, OwnerID: "u1", State: "revoked", CredentialVersion: 1}, 1, "u1"},
		{"missing credential", Target{TenantID: 1, OwnerID: "u1", State: "active", CredentialVersion: 0}, 1, "u1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AuthorizeTarget(tt.target, tt.tenant, tt.actor); err == nil {
				t.Fatal("target unexpectedly authorized")
			}
		})
	}
}
