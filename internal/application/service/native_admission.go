package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
)

// NativeAdmissionStore is the durable boundary needed before a native run may
// execute. Its CreateAdmission implementation must atomically enforce the
// (tenant, owner, request ID) uniqueness rule and verify controlRevision at
// commit; this package deliberately does not select a storage schema.
type NativeAdmissionStore interface {
	FindAdmission(context.Context, uint64, string, string) (nativecontract.RunRecord, bool, error)
	CreateAdmission(context.Context, nativecontract.Admission, int64) (nativecontract.RunRecord, error)
	Get(context.Context, nativecontract.Scope, nativecontract.RunIdentity) (nativecontract.RunRecord, error)
}

// NativeAdmissionBudget reserves the admission budget before a run exists.
// P2.1 never invokes a model or Runner: an unsuccessful reservation therefore
// has no dispatch path.
type NativeAdmissionBudget interface {
	Reserve(context.Context, nativecontract.Admission) error
	Release(context.Context, nativecontract.Admission) error
}

// NativeAdmissionService provides the Admit/Get portion of RunControl. Scope,
// admission, and input/config hashes are already server-frozen inputs at this
// boundary; no request fields are accepted to choose tenant or grants.
type NativeAdmissionService struct {
	controls nativecontract.AdmissionControlSource
	store    NativeAdmissionStore
	budget   NativeAdmissionBudget
}

var _ interface {
	Admit(context.Context, nativecontract.Admission) (nativecontract.RunRecord, error)
	Get(context.Context, nativecontract.Scope, nativecontract.RunIdentity) (nativecontract.RunRecord, error)
} = (*NativeAdmissionService)(nil)

func NewNativeAdmissionService(
	controls nativecontract.AdmissionControlSource,
	store NativeAdmissionStore,
	budget NativeAdmissionBudget,
) *NativeAdmissionService {
	return &NativeAdmissionService{controls: controls, store: store, budget: budget}
}

// ValidateAdmissionControls rejects every state that cannot create a new
// native run. Read, cancel, cleanup, and an authorized idempotent replay do
// not call this function, so a drain never hides existing work.
func ValidateAdmissionControls(c nativecontract.AdmissionControls) error {
	if !c.RecoveryEnabled || !c.AdmissionEnabled || c.WorkerDrain {
		return nativeAdmissionFailure(nativecontract.ErrAdmissionClosed, "native admission is closed")
	}
	if !c.NativeExecutionApproved || !c.DependenciesReady {
		return nativeAdmissionFailure(nativecontract.ErrExecutionGate, "native execution gate is closed")
	}
	if c.Revision < 1 {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "admission controls revision is required")
	}
	return nil
}

func (s *NativeAdmissionService) Admit(ctx context.Context, admission nativecontract.Admission) (nativecontract.RunRecord, error) {
	if err := validateNativeAdmission(admission); err != nil {
		return nativecontract.RunRecord{}, err
	}
	if s == nil || s.controls == nil || s.store == nil || s.budget == nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "native admission dependencies are unavailable")
	}

	// Replay is an authorized read of an already admitted run. It remains
	// available while drain/rollout gates are closed, but a changed payload or
	// configuration fingerprint is never allowed to reuse the request ID.
	existing, found, err := s.store.FindAdmission(ctx, admission.Scope.TenantID, admission.Scope.ActorUserID, admission.Run.RequestID)
	if err != nil {
		return nativecontract.RunRecord{}, err
	}
	if found {
		if sameNativeAdmissionFingerprint(existing.Admission, admission) {
			return existing, nil
		}
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrConflict, "request ID was already admitted with different input or configuration")
	}

	controls, err := s.controls.Current(ctx)
	if err != nil {
		return nativecontract.RunRecord{}, nativeAdmissionFailure(nativecontract.ErrStore, "cannot read admission controls")
	}
	if err := ValidateAdmissionControls(controls); err != nil {
		return nativecontract.RunRecord{}, err
	}
	if err := s.budget.Reserve(ctx, admission); err != nil {
		return nativecontract.RunRecord{}, err
	}

	// Re-read immediately before the durable linearization point. A store must
	// check the supplied revision in the same transaction as CreateAdmission;
	// if deployment switches to drain/closed here, release this uncommitted hold.
	controls, err = s.controls.Current(ctx)
	if err == nil {
		err = ValidateAdmissionControls(controls)
	}
	if err != nil {
		releaseErr := s.budget.Release(ctx, admission)
		if releaseErr != nil {
			return nativecontract.RunRecord{}, fmt.Errorf("%w; budget release failed: %v", err, releaseErr)
		}
		return nativecontract.RunRecord{}, err
	}
	record, err := s.store.CreateAdmission(ctx, admission, controls.Revision)
	if err != nil {
		releaseErr := s.budget.Release(ctx, admission)
		if releaseErr != nil {
			return nativecontract.RunRecord{}, fmt.Errorf("%w; budget release failed: %v", err, releaseErr)
		}
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

func validateNativeAdmission(admission nativecontract.Admission) error {
	if admission.Scope.TenantID == 0 || admission.Scope.ActorUserID == "" || admission.Run.RequestID == "" || admission.Run.RunID == "" {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "tenant, owner, run ID, and request ID are required")
	}
	if admission.Run.TenantID != admission.Scope.TenantID {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "run tenant does not match frozen scope")
	}
	if admission.InputHash == "" || admission.Config.ConfigHash == "" {
		return nativeAdmissionFailure(nativecontract.ErrInvalid, "input and configuration hashes are required")
	}
	return nil
}

func sameNativeAdmissionFingerprint(existing, requested nativecontract.Admission) bool {
	return existing.InputHash == requested.InputHash && existing.Config.ConfigHash == requested.Config.ConfigHash
}

func nativeAdmissionFailure(code nativecontract.ErrorCode, message string) error {
	return nativecontract.Failure{Code: code, Message: message}
}
