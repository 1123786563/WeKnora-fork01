package commercial

import (
	"errors"
	"testing"
)

// TestCloseWorkspaceKeyAndDeidentifiedName pin the #102 derivations: the
// idempotency key is kind-scoped to the identity, and the de-identified
// display name carries the identity anchor and no workspace-owned text.
func TestCloseWorkspaceKeyAndDeidentifiedName(t *testing.T) {
	if got := CloseWorkspaceKey("weknora-tenant-7"); got != "close_workspace:weknora-tenant-7" {
		t.Fatalf("key = %q", got)
	}
	if got := DeidentifiedDisplayName("weknora-tenant-7"); got != "closed:weknora-tenant-7" {
		t.Fatalf("name = %q", got)
	}
}

// TestCloseWorkspacePayloadValidate pins the payload guard: only the exact
// deterministic identity plus its exact de-identified name passes.
func TestCloseWorkspacePayloadValidate(t *testing.T) {
	ok := CloseWorkspacePayload{
		TenantID:           7,
		ExternalCustomerID: ExternalCustomerID(7),
		DisplayName:        DeidentifiedDisplayName(ExternalCustomerID(7)),
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid payload refused: %v", err)
	}
	wrongIdentity := ok
	wrongIdentity.ExternalCustomerID = "weknora-tenant-8"
	if err := wrongIdentity.Validate(); err == nil {
		t.Fatal("mismatched identity must be refused")
	}
	realName := ok
	realName.DisplayName = "Acme Corp"
	if err := realName.Validate(); err == nil {
		t.Fatal("a non-de-identified name must be refused")
	}
	zeroTenant := ok
	zeroTenant.TenantID = 0
	if err := zeroTenant.Validate(); err == nil {
		t.Fatal("a zero tenant must be refused")
	}
	if !errors.Is(ErrWorkspaceClosed, ErrWorkspaceClosed) || ErrWorkspaceClosed.Error() != "workspace_closed" {
		t.Fatalf("sentinel: %v", ErrWorkspaceClosed)
	}
}
