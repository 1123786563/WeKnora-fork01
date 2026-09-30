package sandbox

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrRemoteOperationUnsupported    = errors.New("remote operation initiation unsupported")
	ErrRemoteOperationUnknown        = errors.New("remote operation outcome unknown")
	ErrRemoteOperationInvalidReceipt = errors.New("invalid remote operation receipt")
)

// RemoteOperationRef is a provider-issued receipt for a single accepted
// operation. ID must identify the provider operation, not merely its sandbox.
// OperationKey is the stable caller key used for recovery and must be returned
// unchanged. SandboxID is required for exec receipts; create receipts may not
// know it until the provider finishes provisioning.
type RemoteOperationRef struct {
	Provider     RemoteProvider
	ID           string
	OperationKey string
	SandboxID    string
}

// RemoteOperationState is the provider-neutral state of an accepted operation.
type RemoteOperationState string

const (
	RemoteOperationPending   RemoteOperationState = "pending"
	RemoteOperationRunning   RemoteOperationState = "running"
	RemoteOperationSucceeded RemoteOperationState = "succeeded"
	RemoteOperationFailed    RemoteOperationState = "failed"
	RemoteOperationUnknown   RemoteOperationState = "unknown"
)

// RemoteOperationObservation reports authoritative provider state. Unknown
// means the operation may have been accepted and must not be started again.
type RemoteOperationObservation struct {
	State  RemoteOperationState
	Ref    RemoteOperationRef
	Exec   *RemoteExecResult
	Handle RemoteSandboxHandle
	Err    error
}

// RemoteOperationError carries whether a start outcome is ambiguous. An
// adapter must use Unknown whenever the provider may have accepted the request
// but no durable receipt was returned. It must never map a timeout or lost
// response to definitely-not-started.
type RemoteOperationError struct {
	State RemoteOperationState
	Ref   RemoteOperationRef
	Err   error
}

func (e *RemoteOperationError) Error() string {
	if e == nil || e.Err == nil {
		return ErrRemoteOperationUnknown.Error()
	}
	// The prefix mirrors the carried State so the message never claims
	// "unknown" for a succeeded/failed/running operation.
	switch e.State {
	case RemoteOperationSucceeded:
		return fmt.Sprintf("remote operation succeeded: %v", e.Err)
	case RemoteOperationFailed:
		return fmt.Sprintf("remote operation failed: %v", e.Err)
	case RemoteOperationRunning:
		return fmt.Sprintf("remote operation running: %v", e.Err)
	case RemoteOperationPending:
		return fmt.Sprintf("remote operation pending: %v", e.Err)
	}
	return fmt.Sprintf("%s: %v", ErrRemoteOperationUnknown, e.Err)
}

func (e *RemoteOperationError) Is(target error) bool {
	return target == ErrRemoteOperationUnknown && e != nil && e.State == RemoteOperationUnknown
}

func (e *RemoteOperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsRemoteOperationUnknown(err error) bool {
	return errors.Is(err, ErrRemoteOperationUnknown)
}

// RemoteExecInitiator is an optional, complete exec capability. Implementers
// must supply bounded start, completion wait and authoritative observation.
// They must persist/recover the operation receipt or guarantee authoritative
// lookup by OperationKey before allowing another physical start. Implementing
// only StartExec is deliberately insufficient to advertise this capability.
type RemoteExecInitiator interface {
	StartExec(context.Context, RemoteSandboxHandle, RemoteExecRequest, string) (RemoteOperationRef, error)
	WaitExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (*RemoteExecResult, error)
	ObserveExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (RemoteOperationObservation, error)
}

// RemoteCreateInitiator is an optional, complete create capability with the
// same receipt and same-key replay requirements as RemoteExecInitiator.
type RemoteCreateInitiator interface {
	StartCreate(context.Context, RemoteCreateRequest, string) (RemoteOperationRef, error)
	WaitCreate(context.Context, RemoteOperationRef) (RemoteSandboxHandle, error)
	ObserveCreate(context.Context, RemoteOperationRef) (RemoteOperationObservation, error)
}

// StartRemoteExec invokes only the optional initiation capability. It never
// falls back to RemoteSandboxClient.Exec. Every error returned after invoking
// StartExec is classified as unknown because this seam cannot prove that an
// ordinary transport error means the provider did not accept the operation.
func StartRemoteExec(ctx context.Context, client RemoteSandboxClient, handle RemoteSandboxHandle, req RemoteExecRequest, operationKey string) (RemoteOperationRef, error) {
	if operationKey == "" {
		return RemoteOperationRef{}, fmt.Errorf("%w: empty operation key", ErrRemoteOperationInvalidReceipt)
	}
	if handle == nil || handle.ID() == "" || handle.Provider() != client.Provider() {
		return RemoteOperationRef{}, fmt.Errorf("%w: exec target must have a non-empty ID and match client provider", ErrRemoteOperationInvalidReceipt)
	}
	initiator, ok := client.(RemoteExecInitiator)
	if !ok {
		return RemoteOperationRef{}, ErrRemoteOperationUnsupported
	}
	ref, err := initiator.StartExec(ctx, handle, req, operationKey)
	if err != nil {
		return ref, unknownOperationError(ref, err)
	}
	if ref.Provider != client.Provider() || ref.ID == "" || ref.OperationKey != operationKey || ref.SandboxID != handle.ID() {
		diagnostic := fmt.Errorf("%w: exec receipt provider, operation ID, key, or sandbox ID does not match the request", ErrRemoteOperationInvalidReceipt)
		return ref, unknownOperationError(ref, diagnostic)
	}
	return ref, nil
}

// StartRemoteCreate invokes only the optional create initiation capability. It
// never falls back to RemoteSandboxClient.Create.
func StartRemoteCreate(ctx context.Context, client RemoteSandboxClient, req RemoteCreateRequest, operationKey string) (RemoteOperationRef, error) {
	if operationKey == "" {
		return RemoteOperationRef{}, fmt.Errorf("%w: empty operation key", ErrRemoteOperationInvalidReceipt)
	}
	initiator, ok := client.(RemoteCreateInitiator)
	if !ok {
		return RemoteOperationRef{}, ErrRemoteOperationUnsupported
	}
	ref, err := initiator.StartCreate(ctx, req, operationKey)
	if err != nil {
		return ref, unknownOperationError(ref, err)
	}
	if ref.Provider != client.Provider() || ref.ID == "" || ref.OperationKey != operationKey {
		diagnostic := fmt.Errorf("%w: create receipt provider, operation ID, or key does not match the request", ErrRemoteOperationInvalidReceipt)
		return ref, unknownOperationError(ref, diagnostic)
	}
	return ref, nil
}

func unknownOperationError(ref RemoteOperationRef, cause error) *RemoteOperationError {
	return &RemoteOperationError{State: RemoteOperationUnknown, Ref: ref, Err: cause}
}
