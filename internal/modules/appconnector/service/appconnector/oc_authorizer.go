package appconnector

import (
	"context"
	"errors"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"

	"gorm.io/gorm"
)

// InstallationStateSource reports whether the installation behind a
// connection is still active. A disabled (or reauthorization-parked)
// installation must reject every new call even for otherwise-authorized
// actors.
type InstallationStateSource interface {
	InstallationActive(ctx context.Context, installationID string, tenantID uint64) (bool, error)
}

// SpaceGrantSource reports whether an actor holds an EXPLICIT grant on a
// space connection. Space connections are shared but never implicitly:
// without a grant the answer is false, no matter the actor's role.
type SpaceGrantSource interface {
	SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error)
}

// ocAuthorizer implements appconn.OCAuthorizer: it validates the subject
// against the local connection (and, for open-connector connections, the
// tenant-scoped binding) WITHOUT ever loading credential material. The
// optional lookups may be nil:
//   - installations nil → installation state is not verified (matches the
//     pre-OC container wiring, which had no installation store beside the
//     guard);
//   - grants nil → space connections FAIL CLOSED (with no grant source no
//     grant can ever be established);
//   - bindings nil → open-connector binding verification is skipped (no OC
//     runtime is wired in this deployment yet; wiring lands with the OC
//     runtime tasks, which should pass a real appconn.OCBindingStore).
type ocAuthorizer struct {
	src           ConnectionCredentialSource
	installations InstallationStateSource
	grants        SpaceGrantSource
	bindings      appconn.OCBindingStore
}

var _ appconn.OCAuthorizer = (*ocAuthorizer)(nil)

// NewOCAuthorizer builds the subject authorization checker. src is required;
// the installation, grant and binding lookups follow the nil semantics
// documented on ocAuthorizer.
func NewOCAuthorizer(
	src ConnectionCredentialSource,
	installations InstallationStateSource,
	grants SpaceGrantSource,
	bindings appconn.OCBindingStore,
) appconn.OCAuthorizer {
	return &ocAuthorizer{src: src, installations: installations, grants: grants, bindings: bindings}
}

// subjectGuard adapts an appconn.OCAuthorizer into the A02Guard shape the
// action pipeline consumes; the authorizing binding is discarded there
// because Execute only needs the permission decision.
type subjectGuard struct{ authz appconn.OCAuthorizer }

var _ A02Guard = subjectGuard{}

// NewA02Guard adapts an OCAuthorizer into the ActionService guard shape.
func NewA02Guard(authz appconn.OCAuthorizer) A02Guard { return subjectGuard{authz: authz} }

// Check runs the authorizer's full permission chain and drops the binding.
func (g subjectGuard) Check(
	ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64,
) error {
	_, err := g.authz.Check(ctx, subject, connectionID, expectedVersion)
	return err
}

// NewSubjectGuard builds the container's A02 guard over a credential source
// alone: the subject/tenant/strict-version/membership chain runs
// permission-only (native branch), space connections fail closed, and OC
// binding verification stays deferred until a binding store is wired.
func NewSubjectGuard(src ConnectionCredentialSource) A02Guard {
	return NewA02Guard(NewOCAuthorizer(src, nil, nil, nil))
}

// Check authorizes one connection use for subject WITHOUT ever touching
// credential material. Rejection order (plan T04):
//
//	subject validation → tenant + strict AuthVersion → live subject
//	membership → active installation → explicit space grant →
//	appconn.CanUseConnection → tenant-scoped OC binding.
//
// REVOCATION AND PERMISSION-EPOCH CONTRACT (plan T08): every input above is
// re-queried from its authoritative row on EVERY call — this checker holds
// no cached positive authorization, so an installation disabled, a scope
// change (lost space grant), a member removal, or a revocation (which flips
// the connection AND its binding to revoked while bumping AuthVersion, all
// in one transaction) invalidates the very next Check. The local rows are
// the authority: an unavailable control worker (pending remote cleanup in
// the outbox) never re-opens a revoked connection — revoked connections
// reject via CanUseConnection, revoked or pending bindings reject via
// ErrConnectionRevoked, and a moved AuthVersion rejects via
// ErrConnectionVersionStale. Operations already claimed before a revocation
// may still complete (audit-only); nothing new is authorized after it.
//
// The AuthVersion comparison is deliberately STRICT (F-02): any inequality —
// a stale caller or a moved counter — fails closed with
// ErrConnectionVersionStale. T03's SaveBinding deliberately allows
// AuthVersion equality because this strict gate is what makes that safe;
// the same strictness is applied to the binding row's mirrored counter.
//
// Caller contract (T03 quality review QR-F1/QR-F2): any non-nil error from
// a store is a failure and the caller re-reads on retry. Concurrent losers
// may surface raw driver errors; Check maps none of them into
// authorization outcomes and retries nothing internally. Only gorm's typed
// record-not-found sentinel carries a meaning — no binding row exists, i.e.
// the NATIVE branch: the permission chain above is the whole check and the
// native executor resolves credentials only after it passed.
func (a *ocAuthorizer) Check(
	ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64,
) (appconn.OCBinding, error) {
	if subject.TenantID == 0 || subject.ActorID == "" {
		return appconn.OCBinding{}, ErrMissingSubject
	}
	conn, err := a.src.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return appconn.OCBinding{}, err
	}
	if conn.TenantID != subject.TenantID || conn.AuthVersion != expectedVersion {
		return appconn.OCBinding{}, ErrConnectionVersionStale
	}
	member, err := a.src.MemberActive(ctx, subject.TenantID, subject.ActorID)
	if err != nil {
		return appconn.OCBinding{}, err
	}
	if !member {
		return appconn.OCBinding{}, ErrSubjectNotMember
	}
	if a.installations != nil {
		active, err := a.installations.InstallationActive(ctx, conn.InstallationID, conn.TenantID)
		if err != nil {
			return appconn.OCBinding{}, err
		}
		if !active {
			return appconn.OCBinding{}, ErrInstallationNotActive
		}
	}
	var spaceGrant bool
	if conn.Kind == appconn.ConnectionKindSpace && a.grants != nil {
		granted, err := a.grants.SpaceConnectionGranted(ctx, subject.TenantID, connectionID, subject.ActorID)
		if err != nil {
			return appconn.OCBinding{}, err
		}
		spaceGrant = granted
	}
	if !appconn.CanUseConnection(conn, subject.TenantID, subject.ActorID, spaceGrant) {
		return appconn.OCBinding{}, ErrConnectionForbidden
	}
	if a.bindings == nil {
		// No OC runtime is wired in this deployment: the permission chain
		// above is the whole check (native branch). See ocAuthorizer.
		return appconn.OCBinding{}, nil
	}
	binding, err := a.bindings.GetBinding(ctx, subject.TenantID, connectionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Native connection: no open-connector binding exists; the
			// native executor resolves credentials after this pass.
			return appconn.OCBinding{}, nil
		}
		return appconn.OCBinding{}, err
	}
	if binding.TenantID != subject.TenantID || binding.AuthVersion != expectedVersion {
		return appconn.OCBinding{}, ErrConnectionVersionStale
	}
	if binding.State != appconn.OCBindingActive {
		return appconn.OCBinding{}, ErrConnectionRevoked
	}
	return binding, nil
}
