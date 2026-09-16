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

type targetIdentityContextKey struct{}

// TrustedTargetIdentity is injected only by node/service authentication. A
// mobile request cannot manufacture this value through JSON.
type TrustedTargetIdentity struct {
	RuntimeID         string
	ExternalTargetID  string
	CredentialVersion int64
}

func WithTrustedTargetIdentity(ctx context.Context, identity TrustedTargetIdentity) context.Context {
	return context.WithValue(ctx, targetIdentityContextKey{}, identity)
}

func TrustedTargetIdentityFromContext(ctx context.Context) (TrustedTargetIdentity, bool) {
	identity, ok := ctx.Value(targetIdentityContextKey{}).(TrustedTargetIdentity)
	return identity, ok
}

// TargetVerifier is the trust boundary for node/service registration. The
// default context verifier is deliberately fail-closed until an authenticated
// node middleware injects a current identity.
type TargetVerifier interface {
	VerifyTarget(context.Context, Target) error
}

type ContextTargetVerifier struct{}

func (ContextTargetVerifier) VerifyTarget(ctx context.Context, target Target) error {
	identity, ok := TrustedTargetIdentityFromContext(ctx)
	if !ok || identity.RuntimeID != target.RuntimeID || identity.ExternalTargetID != target.ExternalTargetID || identity.CredentialVersion != target.CredentialVersion || identity.CredentialVersion <= 0 {
		return ErrTargetUntrusted
	}
	return nil
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
