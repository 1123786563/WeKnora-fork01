package service

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
)

// NativeRecoveryLeaseStore is the small P2.2 repository seam. Recovery
// acquires a fence only; P3 remains responsible for any execution or dispatch.
type NativeRecoveryLeaseStore interface {
	ScanRecoverable(context.Context, int) ([]nativecontract.RunIdentity, error)
	Claim(context.Context, nativecontract.RunIdentity, string, time.Duration) (nativecontract.Fence, error)
}

type NativeRecoveryService struct {
	controls nativecontract.AdmissionControlSource
	leases   NativeRecoveryLeaseStore
}

func NewNativeRecoveryService(controls nativecontract.AdmissionControlSource, leases NativeRecoveryLeaseStore) *NativeRecoveryService {
	return &NativeRecoveryService{controls: controls, leases: leases}
}

func nativeRecoveryFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

// ValidateNativeRecoveryControls keeps failed or unapproved execution work
// held. Worker drain deliberately does not close recovery: it permits already
// admitted work to terminate or remain safely held while new admission stops.
func ValidateNativeRecoveryControls(controls nativecontract.AdmissionControls) error {
	if !controls.RecoveryEnabled {
		return nativeRecoveryFailure(nativecontract.ErrAdmissionClosed, "native recovery is closed")
	}
	if !controls.DependenciesReady {
		return nativeRecoveryFailure(nativecontract.ErrStore, "native recovery dependencies are unavailable")
	}
	if !controls.NativeExecutionApproved {
		return nativeRecoveryFailure(nativecontract.ErrExecutionGate, "native execution is not approved")
	}
	if controls.Revision < 1 {
		return nativeRecoveryFailure(nativecontract.ErrInvalid, "recovery controls revision is required")
	}
	return nil
}

// Recover qualifies expired in-flight records, then atomically claims them.
// A competing worker may win between scan and claim; that is normal and not a
// recovery error. This method never invokes a Runner or dispatches side effects.
func (s *NativeRecoveryService) Recover(ctx context.Context, owner string, ttl time.Duration, limit int) ([]nativecontract.Fence, error) {
	if s == nil || s.controls == nil || s.leases == nil {
		return nil, nativeRecoveryFailure(nativecontract.ErrStore, "native recovery dependencies are unavailable")
	}
	if owner == "" || ttl < time.Millisecond || limit <= 0 {
		return nil, nativeRecoveryFailure(nativecontract.ErrInvalid, "worker, lease duration, and scan limit are required")
	}
	controls, err := s.controls.Current(ctx)
	if err != nil {
		return nil, nativeRecoveryFailure(nativecontract.ErrStore, "cannot read recovery controls")
	}
	if err := ValidateNativeRecoveryControls(controls); err != nil {
		return nil, err
	}
	runs, err := s.leases.ScanRecoverable(ctx, limit)
	if err != nil {
		return nil, err
	}
	fences := make([]nativecontract.Fence, 0, len(runs))
	for _, run := range runs {
		fence, err := s.leases.Claim(ctx, run, owner, ttl)
		if err != nil {
			var failure *nativecontract.Failure
			if errors.As(err, &failure) && failure.Code == nativecontract.ErrLeaseLost {
				continue
			}
			return nil, err
		}
		fences = append(fences, fence)
	}
	return fences, nil
}

func nativeRecoveryOpenControls() nativecontract.AdmissionControls {
	return nativecontract.AdmissionControls{Revision: 1, RecoveryEnabled: true, AdmissionEnabled: true, NativeExecutionApproved: true, DependenciesReady: true}
}
