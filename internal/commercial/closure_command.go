package commercial

import (
	"errors"
)

// CommandKindCloseWorkspace is the #102 (Lago 30) additive terminal command:
// stop charging the closed space, dispose of its commercial objects under
// the retention policy, and de-identify the customer's display data on the
// authority — while invoices, payments, refunds, credit notes and the audit
// trail stay intact and addressable (the retention policy is
// financial-history-first: nothing is ever deleted).
const CommandKindCloseWorkspace CommandKind = "close_workspace"

// ErrWorkspaceClosed is the #102 refusal sentinel: a workspace whose
// closure tombstone exists never accepts a new commercial command, a new
// purchase, or a charge execution — and its customer identity is never
// re-ensured (US54: Customer identity is never reused).
var ErrWorkspaceClosed = errors.New("workspace_closed")

// CloseWorkspacePayload is the typed payload of close_workspace.
// ExternalCustomerID must equal ExternalCustomerID(TenantID); DisplayName
// must be the DE-IDENTIFIED name (DeidentifiedDisplayName) — the authority
// must never receive, or keep, the workspace's real display name.
type CloseWorkspacePayload struct {
	TenantID           uint64
	ExternalCustomerID string
	DisplayName        string
}

// Validate refuses an identity inconsistent with the derivation (the same
// payload guard every tenant-addressed command applies) and a display name
// that is not the deterministic de-identified form.
func (p CloseWorkspacePayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid close_workspace payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid close_workspace payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	if p.DisplayName != DeidentifiedDisplayName(p.ExternalCustomerID) {
		return errors.New("invalid close_workspace payload: display_name must be the de-identified name")
	}
	return nil
}

// CloseWorkspaceKey is the stable idempotency key: exactly one closure per
// identity, ever; a replay re-runs the disposal and converges.
func CloseWorkspaceKey(externalCustomerID string) string {
	return string(CommandKindCloseWorkspace) + ":" + externalCustomerID
}

// DeidentifiedDisplayName derives the deterministic de-identified display
// name. The identity anchor stays (financial history must remain
// addressable); no workspace-owned display text ever rides along.
func DeidentifiedDisplayName(externalCustomerID string) string {
	return "closed:" + externalCustomerID
}
