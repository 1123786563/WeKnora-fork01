// Package execution contains the ownership and capability model for execution
// targets used by the mobile workbench.
package execution

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrTargetForbidden = errors.New("target_forbidden")
var ErrTargetUntrusted = errors.New("target_untrusted")

// TargetIdentityProvider is the non-bypassable trust boundary for node
// registration. Implementations must resolve identity and the current
// credential version from a trusted persistent/provider source; request JSON
// and request context are never accepted as the source of truth.
type TargetIdentityProvider interface {
	VerifyTarget(context.Context, uint64, string, Target) error
}

// Target is the public projection of a registered execution target. Secrets,
// private network addresses, and node root paths are deliberately absent.
type Target struct {
	ID                string     `json:"id"`
	TenantID          uint64     `json:"tenant_id"`
	OwnerID           string     `json:"owner_id"`
	Kind              string     `json:"kind"`
	State             string     `json:"state"`
	CredentialVersion int64      `json:"credential_version"`
	RuntimeID         string     `json:"runtime_id,omitempty"`
	ExternalTargetID  string     `json:"external_target_id,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}

// Workspace is an opaque workspace binding. RootRef is a server-resolved
// reference and must never be populated from an arbitrary client path.
type Workspace struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	RootRef  string `json:"root_ref"`
	TenantID uint64 `json:"tenant_id"`
}

// AuthorizeTarget applies the first-release personal-owner policy. Shared
// targets require an explicit ACL in a later task; being an administrator is
// not an implicit bypass for this boundary.
func AuthorizeTarget(target Target, tenant uint64, actor string) error {
	if target.TenantID != tenant || strings.TrimSpace(actor) == "" || target.OwnerID != actor || target.State != "active" || target.CredentialVersion <= 0 {
		return ErrTargetForbidden
	}
	return nil
}
