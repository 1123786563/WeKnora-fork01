package sandbox

import (
	"context"
	"errors"
	"testing"
)

type syncOnlyOperationClient struct {
	RemoteSandboxClient
	createCalls int
	execCalls   int
}

func (*syncOnlyOperationClient) Provider() RemoteProvider { return "e2b" }

func (c *syncOnlyOperationClient) Create(context.Context, RemoteCreateRequest) (RemoteSandboxHandle, error) {
	c.createCalls++
	return nil, nil
}

func (c *syncOnlyOperationClient) Exec(context.Context, RemoteSandboxHandle, RemoteExecRequest) (*RemoteExecResult, error) {
	c.execCalls++
	return nil, nil
}

func TestRemoteOperationStartFailsClosedForSynchronousOnlyClient(t *testing.T) {
	client := &syncOnlyOperationClient{}
	if _, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-1", "e2b"), RemoteExecRequest{Command: "true"}, "op-exec-1"); !errors.Is(err, ErrRemoteOperationUnsupported) {
		t.Fatalf("StartRemoteExec error = %v, want unsupported", err)
	}
	if _, err := StartRemoteCreate(context.Background(), client, RemoteCreateRequest{}, "op-create-1"); !errors.Is(err, ErrRemoteOperationUnsupported) {
		t.Fatalf("StartRemoteCreate error = %v, want unsupported", err)
	}
	if client.execCalls != 0 || client.createCalls != 0 {
		t.Fatalf("synchronous calls exec=%d create=%d; initiation must not fall back", client.execCalls, client.createCalls)
	}
}

func TestRemoteOperationCubeCreateFailsClosed(t *testing.T) {
	client := &CubeRemoteClient{}
	if _, err := StartRemoteCreate(context.Background(), client, RemoteCreateRequest{}, "op-create-cube"); !errors.Is(err, ErrRemoteOperationUnsupported) {
		t.Fatalf("Cube StartRemoteCreate error = %v, want unsupported", err)
	}
}

type ambiguousExecStarter struct {
	*syncOnlyOperationClient
	starts int
}

func (c *ambiguousExecStarter) StartExec(context.Context, RemoteSandboxHandle, RemoteExecRequest, string) (RemoteOperationRef, error) {
	c.starts++
	return RemoteOperationRef{}, &RemoteOperationError{State: RemoteOperationUnknown, Err: errors.New("response lost")}
}
func (*ambiguousExecStarter) WaitExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (*RemoteExecResult, error) {
	return nil, nil
}
func (*ambiguousExecStarter) ObserveExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (RemoteOperationObservation, error) {
	return RemoteOperationObservation{State: RemoteOperationUnknown}, nil
}

func TestRemoteOperationUnknownStartIsNotRetried(t *testing.T) {
	starter := &ambiguousExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}}
	if _, err := StartRemoteExec(context.Background(), starter, testRemoteHandle("sb-1", "e2b"), RemoteExecRequest{Command: "true"}, "stable-op-key"); !IsRemoteOperationUnknown(err) {
		t.Fatalf("error = %v, want unknown", err)
	}
	if starter.starts != 1 {
		t.Fatalf("physical starts = %d, want 1 after ambiguous start", starter.starts)
	}
}

type receiptExecStarter struct {
	*syncOnlyOperationClient
	ref   RemoteOperationRef
	err   error
	calls int
}

func (c *receiptExecStarter) StartExec(context.Context, RemoteSandboxHandle, RemoteExecRequest, string) (RemoteOperationRef, error) {
	c.calls++
	return c.ref, c.err
}
func (*receiptExecStarter) WaitExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (*RemoteExecResult, error) {
	return nil, nil
}
func (*receiptExecStarter) ObserveExec(context.Context, RemoteSandboxHandle, RemoteOperationRef) (RemoteOperationObservation, error) {
	return RemoteOperationObservation{}, nil
}

func TestRemoteOperationStartPreservesProviderReceipt(t *testing.T) {
	want := RemoteOperationRef{Provider: "e2b", ID: "pid-42", OperationKey: "stable-op-key", SandboxID: "sb-7"}
	client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, ref: want}
	got, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-7", "e2b"), RemoteExecRequest{Command: "true"}, want.OperationKey)
	if err != nil {
		t.Fatalf("StartRemoteExec: %v", err)
	}
	if got != want {
		t.Fatalf("receipt = %#v, want %#v", got, want)
	}
}

func TestRemoteOperationRejectsReceiptForDifferentOperationKey(t *testing.T) {
	ref := RemoteOperationRef{Provider: "e2b", ID: "pid-42", OperationKey: "other-key", SandboxID: "sb-7"}
	client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, ref: ref}
	if _, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-7", "e2b"), RemoteExecRequest{Command: "true"}, "stable-op-key"); !errors.Is(err, ErrRemoteOperationInvalidReceipt) || !IsRemoteOperationUnknown(err) {
		t.Fatalf("error = %v, want invalid receipt and unknown", err)
	}
}

func TestRemoteOperationRejectsInvalidExecTargetBeforeStart(t *testing.T) {
	client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}}
	for _, tc := range []struct {
		name   string
		handle RemoteSandboxHandle
	}{
		{name: "nil handle"},
		{name: "empty sandbox id", handle: testRemoteHandle("", "e2b")},
		{name: "foreign provider", handle: testRemoteHandle("sb-7", "docker")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := StartRemoteExec(context.Background(), client, tc.handle, RemoteExecRequest{Command: "true"}, "stable-op-key"); !errors.Is(err, ErrRemoteOperationInvalidReceipt) || IsRemoteOperationUnknown(err) {
				t.Fatalf("error = %v, want pre-dispatch invalid target", err)
			}
		})
	}
	if client.calls != 0 {
		t.Fatalf("provider start calls = %d, want 0", client.calls)
	}
}

func TestRemoteOperationMalformedExecReceiptsRemainUnknown(t *testing.T) {
	wantKey := "stable-op-key"
	for _, tc := range []struct {
		name string
		ref  RemoteOperationRef
	}{
		{name: "missing receipt"},
		{name: "missing operation id", ref: RemoteOperationRef{Provider: "e2b", OperationKey: wantKey, SandboxID: "sb-7"}},
		{name: "mismatched provider", ref: RemoteOperationRef{Provider: "docker", ID: "pid-1", OperationKey: wantKey, SandboxID: "sb-7"}},
		{name: "mismatched sandbox", ref: RemoteOperationRef{Provider: "e2b", ID: "pid-1", OperationKey: wantKey, SandboxID: "sb-other"}},
		{name: "mismatched key", ref: RemoteOperationRef{Provider: "e2b", ID: "pid-1", OperationKey: "other", SandboxID: "sb-7"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, ref: tc.ref}
			if _, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-7", "e2b"), RemoteExecRequest{Command: "true"}, wantKey); !errors.Is(err, ErrRemoteOperationInvalidReceipt) || !IsRemoteOperationUnknown(err) {
				t.Fatalf("error = %v, want invalid receipt and unknown", err)
			}
			if client.calls != 1 {
				t.Fatalf("provider start calls = %d, want 1", client.calls)
			}
		})
	}
}

func TestRemoteOperationRawPostDispatchErrorsRemainUnknown(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, context.Canceled, errors.New("connection reset")} {
		t.Run(cause.Error(), func(t *testing.T) {
			client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, err: cause}
			_, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-7", "e2b"), RemoteExecRequest{Command: "true"}, "stable-op-key")
			if !IsRemoteOperationUnknown(err) || !errors.Is(err, cause) {
				t.Fatalf("error = %v, want unknown wrapping %v", err, cause)
			}
			if client.calls != 1 {
				t.Fatalf("provider start calls = %d, want 1", client.calls)
			}
		})
	}
}

func TestRemoteOperationPreservesReceiptReturnedWithStartError(t *testing.T) {
	want := RemoteOperationRef{Provider: "e2b", ID: "pid-77", OperationKey: "stable-op-key", SandboxID: "sb-7"}
	client := &receiptExecStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, ref: want, err: context.DeadlineExceeded}
	got, err := StartRemoteExec(context.Background(), client, testRemoteHandle("sb-7", "e2b"), RemoteExecRequest{Command: "true"}, want.OperationKey)
	if !IsRemoteOperationUnknown(err) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want unknown wrapping timeout", err)
	}
	if got != want {
		t.Fatalf("returned ref = %#v, want %#v", got, want)
	}
	var operationErr *RemoteOperationError
	if !errors.As(err, &operationErr) || operationErr.Ref != want {
		t.Fatalf("error receipt = %#v, want %#v", operationErr, want)
	}
}

type receiptCreateStarter struct {
	*syncOnlyOperationClient
	ref   RemoteOperationRef
	err   error
	calls int
}

func (c *receiptCreateStarter) StartCreate(context.Context, RemoteCreateRequest, string) (RemoteOperationRef, error) {
	c.calls++
	return c.ref, c.err
}
func (*receiptCreateStarter) WaitCreate(context.Context, RemoteOperationRef) (RemoteSandboxHandle, error) {
	return nil, nil
}
func (*receiptCreateStarter) ObserveCreate(context.Context, RemoteOperationRef) (RemoteOperationObservation, error) {
	return RemoteOperationObservation{}, nil
}

func TestRemoteOperationCreateErrorsAndMalformedReceiptsRemainUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		ref  RemoteOperationRef
		err  error
	}{
		{name: "missing receipt"},
		{name: "missing id", ref: RemoteOperationRef{Provider: "e2b", OperationKey: "key"}},
		{name: "mismatched provider", ref: RemoteOperationRef{Provider: "docker", ID: "operation", OperationKey: "key"}},
		{name: "mismatched key", ref: RemoteOperationRef{Provider: "e2b", ID: "operation", OperationKey: "other"}},
		{name: "raw timeout", err: context.DeadlineExceeded},
		{name: "raw timeout with receipt", ref: RemoteOperationRef{Provider: "e2b", ID: "operation", OperationKey: "key"}, err: context.DeadlineExceeded},
		{name: "raw cancellation", err: context.Canceled},
		{name: "raw transport error", err: errors.New("connection reset")},
		{name: "typed unknown", err: &RemoteOperationError{State: RemoteOperationUnknown, Err: errors.New("response lost")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &receiptCreateStarter{syncOnlyOperationClient: &syncOnlyOperationClient{}, ref: tc.ref, err: tc.err}
			got, err := StartRemoteCreate(context.Background(), client, RemoteCreateRequest{}, "key")
			if !IsRemoteOperationUnknown(err) {
				t.Fatalf("error = %v, want unknown", err)
			}
			if tc.ref.ID == "" || tc.ref.OperationKey != "key" || tc.ref.Provider != "e2b" {
				if !errors.Is(err, ErrRemoteOperationInvalidReceipt) && tc.err == nil {
					t.Fatalf("error = %v, want invalid receipt diagnostic", err)
				}
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("error = %v, want underlying error %v", err, tc.err)
			}
			if client.calls != 1 {
				t.Fatalf("provider start calls = %d, want 1", client.calls)
			}
			if got != tc.ref {
				t.Fatalf("returned ref = %#v, want %#v", got, tc.ref)
			}
		})
	}
}

type operationTestHandle struct {
	id       string
	provider RemoteProvider
}

func (h operationTestHandle) ID() string                { return h.id }
func (h operationTestHandle) Provider() RemoteProvider  { return h.provider }
func (operationTestHandle) Metadata() map[string]string { return nil }

func testRemoteHandle(id string, provider RemoteProvider) RemoteSandboxHandle {
	return operationTestHandle{id: id, provider: provider}
}
