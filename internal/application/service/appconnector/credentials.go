// Package appconnector hosts the dispatch-time credential resolution
// service for app-connector connections.
package appconnector

import (
	"context"
	"errors"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
)

// CredentialResolver resolves the decrypted credential for a connection at
// dispatch time. This is an INTERNAL interface: the returned bytes are handed
// to the calling adapter only — they are never a legal HTTP response body,
// never logged, and never injected into model context as a raw object.
type CredentialResolver interface {
	Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error)
}

// Sentinel resolution errors. None of them ever carries credential material.
var (
	// ErrConnectionRevoked: the connection is revoked (or otherwise not
	// active) — no refresh and no dispatch may proceed.
	ErrConnectionRevoked = errors.New("connection_revoked")
	// ErrConnectionVersionStale: the caller's expected auth_version no
	// longer matches — the reference must be re-fetched.
	ErrConnectionVersionStale = errors.New("connection_version_stale")
	// ErrConnectionOwnerNotMember: the owning principal lost their tenant
	// membership, so the connection is dead even though its row is intact.
	ErrConnectionOwnerNotMember = errors.New("connection_owner_not_member")
)

// ConnectionCredentialSource is the persistence surface the resolver needs.
// It is implemented by repository.MCPOAuthBindingStore.
type ConnectionCredentialSource interface {
	FindConnectionByID(ctx context.Context, connectionID string) (appconn.Connection, error)
	LoadCredential(ctx context.Context, c appconn.Connection) ([]byte, error)
	MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error)
	TryAcquireRefreshLease(ctx context.Context, c appconn.Connection, leaseID string, until time.Time) (bool, error)
}

// credentialResolver guards every dispatch: a revoked connection never
// resolves, a stale auth_version never resolves, and a connection whose
// owner is no longer an active member never resolves.
type credentialResolver struct{ src ConnectionCredentialSource }

// NewCredentialResolver builds the internal resolver over a credential
// source.
func NewCredentialResolver(src ConnectionCredentialSource) CredentialResolver {
	return &credentialResolver{src: src}
}

// Resolve runs the full pre-dispatch guard chain and, only when every check
// passes, returns the decrypted credential bytes to the calling adapter.
func (r *credentialResolver) Resolve(
	ctx context.Context, connectionID string, expectedVersion int64,
) ([]byte, error) {
	c, err := r.src.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if c.State != appconn.ConnectionActive {
		return nil, ErrConnectionRevoked
	}
	if c.AuthVersion != expectedVersion {
		return nil, ErrConnectionVersionStale
	}
	active, err := r.src.MemberActive(ctx, c.TenantID, c.OwnerID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrConnectionOwnerNotMember
	}
	return r.src.LoadCredential(ctx, c)
}

// RefreshLeaseGuarded runs the same pre-dispatch guard chain and then tries
// to claim the EXISTING single-winner refresh lease for the connection's
// underlying token. Revoked connections and stale versions are rejected
// before the lease is ever attempted, so a revoked connection is never
// refreshed. Exactly one concurrent caller wins the lease.
func (r *credentialResolver) RefreshLeaseGuarded(
	ctx context.Context, connectionID string, expectedVersion int64, leaseID string, until time.Time,
) (bool, error) {
	if _, err := r.Resolve(ctx, connectionID, expectedVersion); err != nil {
		return false, err
	}
	c, err := r.src.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return false, err
	}
	return r.src.TryAcquireRefreshLease(ctx, c, leaseID, until)
}

// ConnectionView is the redacted, HTTP- and audit-safe projection of a
// connection. It intentionally carries no credential material and not even
// the credential reference — anything rendered to a user or fed into model
// context must go through this shape.
type ConnectionView struct {
	ID             string `json:"id"`
	InstallationID string `json:"installation_id"`
	Kind           string `json:"kind"`
	OwnerID        string `json:"owner_id"`
	State          string `json:"state"`
	TenantID       uint64 `json:"tenant_id"`
	AuthVersion    int64  `json:"auth_version"`
}

// SafeConnectionView projects a connection onto its redacted view.
func SafeConnectionView(c appconn.Connection) ConnectionView {
	return ConnectionView{
		ID:             c.ID,
		InstallationID: c.InstallationID,
		Kind:           c.Kind,
		OwnerID:        c.OwnerID,
		State:          c.State,
		TenantID:       c.TenantID,
		AuthVersion:    c.AuthVersion,
	}
}
