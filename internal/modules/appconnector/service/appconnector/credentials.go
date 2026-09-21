// Package appconnector hosts the dispatch-time authorization and credential
// resolution services for app-connector connections. Authorization (who may
// use a connection, oc_authorizer.go) is deliberately decoupled from
// credential loading (what secret the call needs, this file): the guard on
// every dispatch NEVER loads credential material, and the native executor
// resolves credentials only after that permission check has passed.
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

// Sentinel authorization/resolution errors. None of them ever carries
// credential material. The first three are raised by the native resolver
// chain; the rest are raised by the subject authorization chain in
// oc_authorizer.go, which shares this block so callers have one place to
// match failures from either branch.
var (
	// ErrConnectionRevoked: the connection is revoked (or otherwise not
	// active) — no refresh and no dispatch may proceed.
	ErrConnectionRevoked = errors.New("connection_revoked")
	// ErrConnectionVersionStale: the caller's expected auth_version no
	// longer matches — the reference must be re-fetched. The comparison is
	// STRICT (F-02): any inequality fails closed, which is what makes
	// T03's SaveBinding safe to accept AuthVersion equality.
	ErrConnectionVersionStale = errors.New("connection_version_stale")
	// ErrConnectionOwnerNotMember: the owning principal lost their tenant
	// membership, so the connection is dead even though its row is intact.
	ErrConnectionOwnerNotMember = errors.New("connection_owner_not_member")
	// ErrMissingSubject: the authorization subject carried no tenant or no
	// actor. Subjects are only constructed by authenticated handlers or
	// controlled background jobs, so a malformed subject is a hard reject.
	ErrMissingSubject = errors.New("missing_subject")
	// ErrSubjectNotMember: the subject lost their live tenant membership;
	// their connection use is dead even though the rows are intact.
	ErrSubjectNotMember = errors.New("subject_not_member")
	// ErrInstallationNotActive: the installation behind the connection is
	// disabled (or parked in reauthorization) — no new call may proceed,
	// even for otherwise-authorized actors.
	ErrInstallationNotActive = errors.New("installation_not_active")
	// ErrConnectionForbidden: the subject may not use this connection — a
	// personal connection of another owner, or a space connection without
	// an explicit grant.
	ErrConnectionForbidden = errors.New("connection_forbidden")
)

// ConnectionCredentialSource is the persistence surface the resolver needs.
// It is implemented by repository.MCPOAuthBindingStore.
type ConnectionCredentialSource interface {
	FindConnectionByID(ctx context.Context, connectionID string) (appconn.Connection, error)
	LoadCredential(ctx context.Context, c appconn.Connection) ([]byte, error)
	MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error)
	TryAcquireRefreshLease(ctx context.Context, c appconn.Connection, leaseID string, until time.Time) (bool, error)
}

// credentialResolver is the NATIVE branch's credential path: a revoked
// connection never resolves, a stale auth_version never resolves, and a
// connection whose owner is no longer an active member never resolves. It
// stays separate from the subject authorization guard (oc_authorizer.go):
// adapters of native connectors call Resolve for the secret AFTER the
// permission check has passed; the guard itself never calls Resolve.
type credentialResolver struct{ src ConnectionCredentialSource }

// NewCredentialResolver builds the internal resolver over a credential
// source. It is retained for the native executors' adapters; the dispatch
// guard consumes the authorizer, not the resolver.
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
