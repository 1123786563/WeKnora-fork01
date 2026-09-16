package execution

import (
	"context"
	"testing"
)

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

func TestContextTargetVerifierRejectsSelfAssertedOrStaleIdentity(t *testing.T) {
	target := Target{RuntimeID: "runtime-1", ExternalTargetID: "node-1", CredentialVersion: 3}
	verifier := ContextTargetVerifier{}
	if err := verifier.VerifyTarget(context.Background(), target); err == nil {
		t.Fatal("self-asserted target identity was trusted")
	}
	ctx := WithTrustedTargetIdentity(context.Background(), TrustedTargetIdentity{RuntimeID: "runtime-1", ExternalTargetID: "node-1", CredentialVersion: 2})
	if err := verifier.VerifyTarget(ctx, target); err == nil {
		t.Fatal("stale credential version was trusted")
	}
	ctx = WithTrustedTargetIdentity(context.Background(), TrustedTargetIdentity{RuntimeID: "runtime-1", ExternalTargetID: "node-1", CredentialVersion: 3})
	if err := verifier.VerifyTarget(ctx, target); err != nil {
		t.Fatal(err)
	}
}
