package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// NativeAdmissionKey identifies one idempotent admission independently of the
// caller. The session owner, rather than ActorUserID, owns replay identity.
type NativeAdmissionKey struct {
	TenantID                                       uint64
	OwnerID, AppName, UserID, SessionID, RequestID string
}

// NativeAdmissionStore owns the linearization point for native admission.
// Implementations execute lookup, fingerprint comparison, budget reservation,
// revision verification, and run creation in one transaction. Same fingerprints
// return the stored record with replay=true; changed fingerprints return
// ErrConflict without a second reservation.
type NativeAdmissionStore interface {
	Admit(context.Context, NativeAdmissionKey, nativecontract.Admission, int64, func(context.Context) error) (nativecontract.RunRecord, bool, error)
	Get(context.Context, nativecontract.Scope, nativecontract.RunIdentity) (nativecontract.RunRecord, error)
}

type NativeAdmissionBudget interface {
	Reserve(context.Context, nativecontract.Admission) error
	Release(context.Context, nativecontract.Admission) error
}

// NativeAdmissionAuthority supplies server-owned values which callers cannot
// choose in an Admission request.
type NativeAdmissionAuthority interface {
	CurrentConfig(context.Context, nativecontract.Scope) (nativecontract.ConfigBinding, error)
	RequiredGrants(context.Context, nativecontract.Scope, nativecontract.RunIdentity) ([]nativecontract.ResourceGrant, error)
	ValidateSession(context.Context, nativecontract.Scope, session.Key) error
}

// NativeAdmissionDispatcher is a P3 seam only. P2.1 retains P0's NO-GO and
// never calls Dispatch; accepting it makes that absence directly testable.
type NativeAdmissionDispatcher interface {
	Dispatch(context.Context, nativecontract.RunRecord) error
}

type NativeAdmissionService struct {
	controls   nativecontract.AdmissionControlSource
	scopes     nativecontract.ScopeResolver
	authority  NativeAdmissionAuthority
	store      NativeAdmissionStore
	budget     NativeAdmissionBudget
	dispatcher NativeAdmissionDispatcher
}

func NewNativeAdmissionService(
	controls nativecontract.AdmissionControlSource,
	scopes nativecontract.ScopeResolver,
	authority NativeAdmissionAuthority,
	store NativeAdmissionStore,
	budget NativeAdmissionBudget,
	dispatcher NativeAdmissionDispatcher,
) *NativeAdmissionService {
	return &NativeAdmissionService{
		controls: controls, scopes: scopes, authority: authority, store: store, budget: budget, dispatcher: dispatcher,
	}
}

// ValidateAdmissionControls is only for new run creation. Existing result
// reads and valid replays remain available while a deployment drains.
func ValidateAdmissionControls(c nativecontract.AdmissionControls) error {
	if !c.RecoveryEnabled || !c.AdmissionEnabled || c.WorkerDrain {
		return nativeAdmissionFailure(nativecontract.ErrAdmissionClosed, "native admission is closed")
	}
	if !c.DependenciesReady {
		return nativeAdmissionFailure(nativecontract.ErrStore, "native admission dependencies are unavailable")
	}
	if !c.NativeExecutionApproved {
		return nativeAdmissionFailure(nativecontract.ErrExecutionGate, "native execution is not approved")
	}
	if c.Revision < 1 {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "admission controls revision is required")
	}
	return nil
}

func (s *NativeAdmissionService) Admit(ctx context.Context, requested nativecontract.Admission) (nativecontract.RunRecord, error) {
	if s == nil || s.controls == nil || s.scopes == nil || s.authority == nil || s.store == nil || s.budget == nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "native admission dependencies are unavailable")
	}
	if err := validateNativeAdmissionBinding(requested); err != nil {
		return nativecontract.RunRecord{}, err
	}

	// Admission.Scope contains an initial frozen snapshot, never a final grant.
	// Recheck starts from it but rebuilds authority server-side before any
	// budget reservation or storage transaction.
	scope, err := s.scopes.Recheck(ctx, requested.Scope, nil)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrForbidden, "scope is no longer authorized")
	}
	if err := validateNativeAdmissionBindingForScope(requested, scope); err != nil {
		return nativecontract.RunRecord{}, err
	}
	config, err := s.authority.CurrentConfig(ctx, scope)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "current configuration is unavailable")
	}
	if !sameNativeAdmissionConfig(requested.Config, config) {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrForbidden, "requested configuration is no longer current")
	}
	grants, err := s.authority.RequiredGrants(ctx, scope, requested.Run)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrForbidden, "required grants are unavailable")
	}
	scope, err = s.scopes.Recheck(ctx, scope, grants)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrForbidden, "required grants were revoked")
	}
	if err := s.authority.ValidateSession(ctx, scope, requested.Session); err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrForbidden, "session slot is not authorized")
	}

	key := nativeAdmissionKey(scope, requested)
	first, err := s.controls.Current(ctx)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "cannot read admission controls")
	}
	if err := ValidateAdmissionControls(first); err != nil {
		return nativecontract.RunRecord{}, err
	}
	second, err := s.controls.Current(ctx)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "cannot read admission controls")
	}
	if err := ValidateAdmissionControls(second); err != nil {
		return nativecontract.RunRecord{}, err
	}

	record, _, err := s.store.Admit(ctx, key, requested, second.Revision, func(ctx context.Context) error {
		return s.budget.Reserve(ctx, requested)
	})
	if err != nil {
		return nativecontract.RunRecord{}, err
	}
	return record, nil
}

func (s *NativeAdmissionService) Get(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity) (nativecontract.RunRecord, error) {
	if s == nil || s.store == nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "native admission store is unavailable")
	}
	return s.store.Get(ctx, scope, run)
}

func nativeAdmissionKey(scope nativecontract.Scope, admission nativecontract.Admission) NativeAdmissionKey {
	return NativeAdmissionKey{
		TenantID: scope.TenantID, OwnerID: scope.SessionOwnerID,
		AppName: admission.Session.AppName, UserID: admission.Session.UserID,
		SessionID: admission.Session.SessionID, RequestID: admission.Run.RequestID,
	}
}

func validateNativeAdmissionBinding(admission nativecontract.Admission) error {
	if admission.Scope.TenantID == 0 || admission.Scope.SessionOwnerID == "" || admission.Run.RequestID == "" || admission.Run.RunID == "" {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "tenant, session owner, run ID, and request ID are required")
	}
	if admission.Run.TenantID != admission.Scope.TenantID || admission.Run.SessionID != admission.Session.SessionID ||
		admission.Session.SessionID == "" || admission.Session.UserID != admission.Scope.SessionOwnerID || admission.Session.AppName == "" {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "run, session, and frozen scope are not bound")
	}
	if admission.InputHash == "" || admission.Config.ConfigHash == "" || admission.Config.ModelConfigVersion == "" {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "input hash and configuration version/hash are required")
	}
	return nil
}

func validateNativeAdmissionBindingForScope(admission nativecontract.Admission, scope nativecontract.Scope) error {
	if scope.TenantID != admission.Run.TenantID || scope.SessionOwnerID != admission.Session.UserID {
		return nativeAdmissionFailure(nativecontract.ErrForbidden, "current scope no longer owns the run session")
	}
	return nil
}

func sameNativeAdmissionConfig(requested, current nativecontract.ConfigBinding) bool {
	return requested.ConfigHash == current.ConfigHash &&
		requested.ModelConfigVersion == current.ModelConfigVersion &&
		requested.CredentialVersion == current.CredentialVersion
}

func sameNativeAdmissionFingerprint(existing, requested nativecontract.Admission) bool {
	return existing.InputHash == requested.InputHash && sameNativeAdmissionConfig(existing.Config, requested.Config)
}

func nativeAdmissionFailure(code nativecontract.ErrorCode, message string) error {
	return nativecontract.Failure{Code: code, Message: message}
}
