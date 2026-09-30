package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type normalExecHandle struct{ id string }

func (h normalExecHandle) ID() string                { return h.id }
func (normalExecHandle) Provider() RemoteProvider    { return SandboxTypeDocker }
func (normalExecHandle) Metadata() map[string]string { return nil }

type normalExecFake struct {
	mu             sync.Mutex
	createOptions  client.ExecCreateOptions
	createCalls    int
	attachCalls    int
	startCalls     int
	attachErr      error
	createResult   client.ExecCreateResult
	inspectResults []client.ExecInspectResult
	inspectErr     error
	inspectCalls   int
	inspectHook    func(int, string)
	attachFn       func() client.ExecAttachResult
	stdinBytes     []byte
	conn           *normalExecTestConn
}

type normalExecTestSink struct {
	mu       sync.Mutex
	sealed   bool
	appendFn func(context.Context, DockerNormalExecReceipt, string, []byte) error
	sealFn   func() error
	sealErr  error
}

func (s *normalExecTestSink) Append(ctx context.Context, receipt DockerNormalExecReceipt, stream string, chunk []byte) error {
	if s.appendFn == nil {
		return nil
	}
	return s.appendFn(ctx, receipt, stream, chunk)
}

func (s *normalExecTestSink) Seal() error {
	s.mu.Lock()
	s.sealed = true
	err := s.sealErr
	s.mu.Unlock()
	if s.sealFn != nil {
		return s.sealFn()
	}
	return err
}

type normalExecFunctionSink func(context.Context, DockerNormalExecReceipt, string, []byte) error

func (f normalExecFunctionSink) Append(ctx context.Context, receipt DockerNormalExecReceipt, stream string, chunk []byte) error {
	return f(ctx, receipt, stream, chunk)
}

func (normalExecFunctionSink) Seal() error { return nil }

func (f *normalExecFake) ExecCreate(_ context.Context, containerID string, options client.ExecCreateOptions) (client.ExecCreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	f.createOptions = options
	if f.createResult.ID == "" {
		f.createResult.ID = "exec-actual"
	}
	return f.createResult, nil
}

func (f *normalExecFake) ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error) {
	f.mu.Lock()
	f.attachCalls++
	fn, attachErr := f.attachFn, f.attachErr
	f.mu.Unlock()
	if fn != nil {
		return fn(), attachErr
	}
	return client.ExecAttachResult{}, attachErr
}

func (f *normalExecFake) ExecStart(context.Context, string, client.ExecStartOptions) (client.ExecStartResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	return client.ExecStartResult{}, nil
}

func (f *normalExecFake) ExecInspect(_ context.Context, execID string, _ client.ExecInspectOptions) (client.ExecInspectResult, error) {
	f.mu.Lock()
	f.inspectCalls++
	call := f.inspectCalls
	hook := f.inspectHook
	if f.inspectErr != nil {
		f.mu.Unlock()
		return client.ExecInspectResult{}, f.inspectErr
	}
	if len(f.inspectResults) == 0 {
		f.mu.Unlock()
		if hook != nil {
			hook(call, execID)
		}
		return client.ExecInspectResult{ID: execID, ContainerID: "container-actual"}, nil
	}
	result := f.inspectResults[0]
	if len(f.inspectResults) > 1 {
		f.inspectResults = f.inspectResults[1:]
	}
	f.mu.Unlock()
	if hook != nil {
		hook(call, execID)
	}
	return result, nil
}

func normalExecTestClient(t *testing.T, f *normalExecFake, maxInput, maxOutput int64) *DockerNormalExecClient {
	t.Helper()
	c, err := NewDockerNormalExecClient(f, DockerNormalExecConfig{
		RPCTimeout: 2 * time.Second, MaxInputBytes: maxInput, MaxOutputBytes: maxOutput,
	})
	require.NoError(t, err)
	return c
}

func normalExecRequest() RemoteExecRequest {
	return RemoteExecRequest{
		Command: "cat", Args: []string{}, Stdin: "hello", User: "10001:10001",
		WorkDir: "/workspace", Env: map[string]string{"MODE": "test"}, Timeout: 4 * time.Second,
	}
}

func normalExecDockerRequest(req RemoteExecRequest) DockerNormalExecRequest {
	return DockerNormalExecRequest{Request: req, StdinEnabled: req.Stdin != ""}
}

func createdExecInspect() client.ExecInspectResult {
	return client.ExecInspectResult{ID: "exec-actual", ContainerID: "container-actual", Running: false}
}

func TestDockerNormalExecCreatesInertAttachedNonTTYExec(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{createdExecInspect()}}
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	require.Equal(t, DockerNormalExecReceipt{ContainerID: "container-actual", ExecID: "exec-actual", StdinEnabled: true, StdinBytes: 5, StdinSHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", Timeout: 4 * time.Second}, receipt)
	require.Equal(t, 1, fake.createCalls)
	require.Zero(t, fake.attachCalls)
	require.Zero(t, fake.startCalls)
	require.True(t, fake.createOptions.AttachStdin)
	require.True(t, fake.createOptions.AttachStdout)
	require.True(t, fake.createOptions.AttachStderr)
	require.False(t, fake.createOptions.TTY)
	require.Zero(t, fake.createOptions.ConsoleSize.Height)
	require.Zero(t, fake.createOptions.ConsoleSize.Width)
	require.Equal(t, "10001:10001", fake.createOptions.User)
	require.Equal(t, "/workspace", fake.createOptions.WorkingDir)
	require.Equal(t, []string{"MODE=test"}, fake.createOptions.Env)
}

func TestDockerNormalExecPreservesEnabledAndDisabledEmptyStdin(t *testing.T) {
	emptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	receipts := map[bool]DockerNormalExecReceipt{}
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			fake := &normalExecFake{inspectResults: []client.ExecInspectResult{
				createdExecInspect(), createdExecInspect(),
				{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 0},
			}}
			fake.attachFn = func() client.ExecAttachResult {
				conn := newNormalExecTestConn()
				fake.conn = conn
				go func() {
					if enabled {
						_, _ = io.ReadAll(conn.stdinReader)
					}
					writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stdout, []byte("done"))
					_ = conn.outputWriter.Close()
				}()
				return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(conn, "application/vnd.docker.raw-stream")}
			}
			c := normalExecTestClient(t, fake, 16, 1024)
			request := DockerNormalExecRequest{Request: RemoteExecRequest{Command: "cat", Timeout: time.Second}, StdinEnabled: enabled}
			receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, request)
			require.NoError(t, err)
			require.Equal(t, enabled, fake.createOptions.AttachStdin)
			require.Equal(t, enabled, receipt.StdinEnabled)
			require.Zero(t, receipt.StdinBytes)
			require.Equal(t, emptyHash, receipt.StdinSHA256)
			receipts[enabled] = receipt

			out, err := c.StartAttachedExecOnce(context.Background(), receipt, enabled, nil, testNormalSink(nil))
			require.NoError(t, err)
			require.Equal(t, DockerNormalExecTransportComplete, out.Transport)
			require.Zero(t, out.InputBytes)
			require.Equal(t, 1, fake.attachCalls)
			if enabled {
				require.Equal(t, 1, fake.conn.closeWriteCount, "enabled empty stdin still sends a half-close")
			} else {
				require.Zero(t, fake.conn.closeWriteCount, "disabled stdin does not attach or half-close stdin")
			}
		})
	}
	require.NotEqual(t, receipts[true], receipts[false], "empty stdin enablement is part of the immutable receipt")
}

func TestDockerNormalExecRejectsMismatchedEmptyStdinEnablementBeforeAttach(t *testing.T) {
	for _, tc := range []struct {
		name          string
		createEnabled bool
		startEnabled  bool
		startStdin    []byte
	}{
		{name: "enabled receipt replayed as disabled", createEnabled: true, startEnabled: false},
		{name: "disabled receipt replayed as enabled", createEnabled: false, startEnabled: true},
		{name: "enabled empty receipt replayed with bytes", createEnabled: true, startEnabled: true, startStdin: []byte("unexpected")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &normalExecFake{inspectResults: []client.ExecInspectResult{createdExecInspect()}}
			c := normalExecTestClient(t, fake, 16, 1024)
			request := DockerNormalExecRequest{Request: RemoteExecRequest{Command: "true"}, StdinEnabled: tc.createEnabled}
			receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, request)
			require.NoError(t, err)
			_, err = c.StartAttachedExecOnce(context.Background(), receipt, tc.startEnabled, tc.startStdin, testNormalSink(nil))
			require.ErrorIs(t, err, ErrDockerNormalExecInputMismatch)
			require.Equal(t, 1, fake.inspectCalls, "mismatch must fail before start preflight inspect")
			require.Zero(t, fake.attachCalls, "mismatch must fail before Docker attach/start")
		})
	}
}

func TestDockerNormalExecRejectsNonemptyStdinWhenDisabledBeforeCreate(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{createdExecInspect()}}
	c := normalExecTestClient(t, fake, 16, 1024)
	request := DockerNormalExecRequest{Request: normalExecRequest(), StdinEnabled: false}
	_, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, request)
	require.ErrorIs(t, err, ErrDockerNormalExecInputMismatch)
	require.Zero(t, fake.createCalls, "inconsistent enablement must be rejected before ExecCreate")
}

func TestDockerNormalExecRejectsWrongInspectedIdentityBeforeAttach(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*client.ExecInspectResult)
	}{
		{name: "container mismatch", edit: func(r *client.ExecInspectResult) { r.ContainerID = "other-container" }},
		{name: "exec mismatch", edit: func(r *client.ExecInspectResult) { r.ID = "other-exec" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspected := createdExecInspect()
			tc.edit(&inspected)
			fake := &normalExecFake{inspectResults: []client.ExecInspectResult{inspected}}
			c := normalExecTestClient(t, fake, 16, 1024)
			_, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
			require.ErrorIs(t, err, ErrDockerNormalExecIdentityMismatch)
			require.Zero(t, fake.attachCalls)
		})
	}
}

func TestDockerNormalExecRejectsAlteredReceiptBeforeAttach(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{createdExecInspect(), createdExecInspect()}}
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	receipt.ContainerID = "wrong-container"
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), testNormalSink(nil))
	require.ErrorIs(t, err, ErrDockerNormalExecIdentityMismatch)
	require.Equal(t, DockerNormalExecTransportUnavailable, out.Transport)
	require.Zero(t, fake.attachCalls)
}

func TestDockerNormalExecAttachDemuxesStreamsAndHalfClosesStdinOnce(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 7},
	}}
	fake.attachFn = func() client.ExecAttachResult {
		conn := newNormalExecTestConn()
		fake.conn = conn
		go func() {
			stdin, _ := io.ReadAll(conn.stdinReader)
			fake.mu.Lock()
			fake.stdinBytes = append([]byte(nil), stdin...)
			fake.mu.Unlock()
			writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stdout, []byte{0, 0xff, 'x'})
			writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stderr, []byte("err"))
			_ = conn.outputWriter.Close()
		}()
		return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(conn, "application/vnd.docker.raw-stream")}
	}
	c := normalExecTestClient(t, fake, 16, 1024)
	req := normalExecRequest()
	var persisted []string
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.NoError(t, err)
	sink := func(_ context.Context, _ DockerNormalExecReceipt, stream string, chunk []byte) error {
		persisted = append(persisted, stream+":"+string(chunk))
		return nil
	}
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), normalExecFunctionSink(sink))
	require.NoError(t, err)
	require.Equal(t, DockerNormalExecTransportComplete, out.Transport)
	require.Equal(t, DockerNormalExecProcessFailed, out.Observation.State)
	require.Equal(t, 7, *out.Observation.ExitCode)
	require.True(t, out.StartEvidence)
	require.Equal(t, int64(6), out.OutputBytes)
	require.Equal(t, []string{"stdout:\x00\xffx", "stderr:err"}, persisted)
	require.Equal(t, []byte("hello"), fake.stdinBytes)
	require.Equal(t, 1, fake.conn.closeWriteCount)
	require.Equal(t, 1, fake.attachCalls)
	require.Zero(t, fake.startCalls)
}

func TestDockerNormalExecSealFailureIsPartialAndObservable(t *testing.T) {
	fake := normalExecWithOutput([]byte("captured"), nil)
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	sealFailure := errors.New("durable seal unavailable")
	sink := &normalExecTestSink{sealErr: sealFailure}
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), sink)
	require.ErrorIs(t, err, sealFailure)
	require.Equal(t, DockerNormalExecTransportPartial, out.Transport, "unconfirmed persistence cutoff cannot be complete")
	sink.mu.Lock()
	require.True(t, sink.sealed)
	sink.mu.Unlock()
	require.Equal(t, 1, fake.attachCalls, "seal failure does not retry Docker attach")
}

func TestDockerNormalExecDoesNotRetryLostAttachResponse(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", Running: true},
	}}
	fake.attachErr = errors.New("response lost after start side effect")
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), testNormalSink(nil))
	require.Error(t, err)
	require.Equal(t, DockerNormalExecTransportUnavailable, out.Transport)
	require.Equal(t, DockerNormalExecProcessRunning, out.Observation.State)
	require.True(t, out.StartEvidence, "running inspection is positive start evidence even when the hijack response was lost")
	_, retryErr := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), testNormalSink(nil))
	require.ErrorIs(t, retryErr, ErrDockerNormalExecAlreadyStarted)
	require.Equal(t, 1, fake.attachCalls)
	require.Zero(t, fake.startCalls)
}

func TestDockerNormalExecOutputQuotaIsExplicitlyPartial(t *testing.T) {
	fake := normalExecWithOutput([]byte("abcdefgh"), nil)
	c := normalExecTestClient(t, fake, 16, 5)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	var saved bytes.Buffer
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), normalExecFunctionSink(func(_ context.Context, _ DockerNormalExecReceipt, _ string, chunk []byte) error {
		_, _ = saved.Write(chunk)
		return nil
	}))
	require.ErrorIs(t, err, ErrDockerNormalExecOutputLimit)
	require.Equal(t, DockerNormalExecTransportPartial, out.Transport)
	require.True(t, out.OutputTruncated)
	require.Equal(t, int64(5), out.OutputBytes)
	require.Equal(t, "abcde", saved.String())
}

func TestDockerNormalExecSinkFailureRemainsPartial(t *testing.T) {
	fake := normalExecWithOutput([]byte("abc"), nil)
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), normalExecFunctionSink(func(context.Context, DockerNormalExecReceipt, string, []byte) error {
		return errors.New("store unavailable")
	}))
	require.ErrorContains(t, err, "store unavailable")
	require.Equal(t, DockerNormalExecTransportPartial, out.Transport)
	require.Equal(t, 1, fake.attachCalls, "failed sink must not trigger a second attach")
}

func TestDockerNormalExecRejectsMalformedMultiplexFramesAsPartial(t *testing.T) {
	fake := normalExecWithOutput(nil, []byte{9, 0, 0, 0, 0, 0, 0, 0})
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), testNormalSink(nil))
	require.Error(t, err)
	require.Equal(t, DockerNormalExecTransportPartial, out.Transport)
}

func TestDockerNormalExecObserveRequiresExactIDsAndPositiveStartEvidence(t *testing.T) {
	tests := []struct {
		name    string
		inspect client.ExecInspectResult
		started bool
		want    DockerNormalExecProcessState
	}{
		{name: "running proves start", inspect: client.ExecInspectResult{ID: "exec-actual", ContainerID: "container-actual", Running: true}, want: DockerNormalExecProcessRunning},
		{name: "false zero without start is unknown", inspect: client.ExecInspectResult{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 0}, want: DockerNormalExecProcessUnknown},
		{name: "false zero after start succeeds", inspect: client.ExecInspectResult{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 0}, started: true, want: DockerNormalExecProcessSucceeded},
		{name: "wrong ID is rejected", inspect: client.ExecInspectResult{ID: "elsewhere", ContainerID: "container-actual", ExitCode: 0}, started: true, want: DockerNormalExecProcessUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &normalExecFake{inspectResults: []client.ExecInspectResult{createdExecInspect(), tc.inspect}}
			c := normalExecTestClient(t, fake, 16, 1024)
			receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
			require.NoError(t, err)
			observation, err := c.ObserveAttachedExec(context.Background(), receipt, tc.started)
			if tc.name == "wrong ID is rejected" {
				require.ErrorIs(t, err, ErrDockerNormalExecIdentityMismatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.want, observation.State)
		})
	}
}

func TestDockerNormalExecRejectsInputOverflowBeforeCreate(t *testing.T) {
	fake := &normalExecFake{}
	c := normalExecTestClient(t, fake, 4, 1024)
	req := normalExecRequest()
	_, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.ErrorIs(t, err, ErrDockerNormalExecInputLimit)
	require.Zero(t, fake.createCalls)
}

func TestDockerNormalExecRejectsEmptySinkBeforeAttach(t *testing.T) {
	fake := normalExecWithOutput([]byte("abc"), nil)
	c := normalExecTestClient(t, fake, 16, 1024)
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(normalExecRequest()))
	require.NoError(t, err)
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte("hello"), nil)
	require.ErrorIs(t, err, ErrDockerNormalExecSinkRequired)
	require.Equal(t, DockerNormalExecTransportUnavailable, out.Transport)
	require.Zero(t, fake.attachCalls)
}

func TestDockerNormalExecCleanEOFWhileRunningIsPartial(t *testing.T) {
	fake := normalExecWithOutput([]byte("complete-looking-output"), nil)
	fake.inspectResults = []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", Running: true},
	}
	c := normalExecTestClient(t, fake, 16, 1024)
	req := normalExecRequest()
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.NoError(t, err)
	out, err := c.StartAttachedExecOnce(context.Background(), receipt, receipt.StdinEnabled, []byte(req.Stdin), testNormalSink(nil))
	require.Error(t, err, "clean EOF is not complete while the exact exec remains running")
	require.Equal(t, DockerNormalExecTransportPartial, out.Transport)
	require.Equal(t, DockerNormalExecProcessRunning, out.Observation.State)
	require.True(t, out.StartEvidence)
	require.Equal(t, 1, fake.attachCalls)
	require.Zero(t, fake.startCalls)
}

func TestDockerNormalExecCancellationSealsBlockedSinkBeforeReturn(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", Running: true},
	}}
	entered := make(chan struct{})
	release := make(chan struct{})
	var persisted atomic.Int64
	fake.attachFn = func() client.ExecAttachResult {
		conn := newNormalExecTestConn()
		fake.conn = conn
		go func() {
			_, _ = io.ReadAll(conn.stdinReader)
			writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stdout, []byte("blocked"))
			_ = conn.outputWriter.Close()
		}()
		return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(conn, "application/vnd.docker.raw-stream")}
	}
	c := normalExecTestClient(t, fake, 16, 1024)
	req := normalExecRequest()
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	startDone := make(chan struct {
		out DockerNormalExecOutcome
		err error
	}, 1)
	go func() {
		sink := &normalExecTestSink{}
		sink.appendFn = func(context.Context, DockerNormalExecReceipt, string, []byte) error {
			close(entered)
			<-release
			sink.mu.Lock()
			if !sink.sealed {
				persisted.Add(1)
			}
			sink.mu.Unlock()
			return nil
		}
		out, startErr := c.StartAttachedExecOnce(ctx, receipt, receipt.StdinEnabled, []byte(req.Stdin), sink)
		startDone <- struct {
			out DockerNormalExecOutcome
			err error
		}{out, startErr}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		cancel()
		close(release)
		t.Fatal("sink was not entered")
	}
	cancel()
	select {
	case result := <-startDone:
		require.Error(t, result.err)
		require.Equal(t, DockerNormalExecTransportPartial, result.out.Transport)
	case <-time.After(7 * time.Second):
		close(release)
		t.Fatal("provider did not return within its bounded drain")
	}
	close(release) // emulate the blocked storage call unblocking after return.
	time.Sleep(25 * time.Millisecond)
	require.Zero(t, persisted.Load(), "sealed sink must refuse a late append")
	require.Equal(t, 1, fake.attachCalls)
}

type adversarialNormalExecCallback struct {
	sealCalls    atomic.Int64
	deliverCalls atomic.Int64
	blockSeal    bool
	release      chan struct{}
}

func (c *adversarialNormalExecCallback) Deliver(context.Context, string, []byte) error {
	c.deliverCalls.Add(1)
	return nil
}

func (c *adversarialNormalExecCallback) Seal() {
	c.sealCalls.Add(1)
	if c.blockSeal {
		<-c.release
	}
}

func TestDockerNormalExecRejectsTypedCallbackBeforeCreate(t *testing.T) {
	tests := []struct {
		name     string
		callback *adversarialNormalExecCallback
	}{
		{name: "blocking Seal", callback: &adversarialNormalExecCallback{blockSeal: true, release: make(chan struct{})}},
		{name: "ineffective Seal", callback: &adversarialNormalExecCallback{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := normalExecWithOutput([]byte("callback"), nil)
			c := normalExecTestClient(t, fake, 16, 1024)
			request := DockerNormalExecRequest{Request: normalExecRequest(), OutputCallback: tc.callback}
			returned := make(chan error, 1)
			go func() {
				_, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, request)
				returned <- err
			}()
			select {
			case err := <-returned:
				require.ErrorIs(t, err, ErrDockerNormalExecCallbackUnsupported)
			case <-time.After(250 * time.Millisecond):
				if tc.callback.blockSeal {
					close(tc.callback.release)
				}
				t.Fatal("unsupported callback blocked CreateAttachedExec; callback code must never run")
			}
			require.Zero(t, fake.createCalls, "callback intent must be rejected before Docker create")
			require.Zero(t, fake.attachCalls)
			require.Zero(t, tc.callback.deliverCalls.Load())
			require.Zero(t, tc.callback.sealCalls.Load())
		})
	}
}

func TestDockerNormalExecRejectsLegacyCallbackBeforeCreate(t *testing.T) {
	fake := normalExecWithOutput([]byte("callback"), nil)
	c := normalExecTestClient(t, fake, 16, 1024)
	req := normalExecRequest()
	req.OnOutput = func(string, []byte) {}
	_, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.ErrorIs(t, err, ErrDockerNormalExecCallbackUnsupported)
	require.Zero(t, fake.createCalls, "legacy callback is rejected before Docker create")
	require.Zero(t, fake.attachCalls)
}

func TestDockerNormalExecCancellationAfterDrainBeforeInspectIsPartial(t *testing.T) {
	fake := &normalExecFake{inspectResults: []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 0},
	}}
	inspectEntered, releaseInspect := make(chan struct{}), make(chan struct{})
	fake.inspectHook = func(call int, _ string) {
		if call == 3 {
			close(inspectEntered)
			<-releaseInspect
		}
	}
	fake.attachFn = func() client.ExecAttachResult {
		conn := newNormalExecTestConn()
		fake.conn = conn
		go func() {
			_, _ = io.ReadAll(conn.stdinReader)
			writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stdout, []byte("complete"))
			_ = conn.outputWriter.Close()
		}()
		return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(conn, "application/vnd.docker.raw-stream")}
	}
	c := normalExecTestClient(t, fake, 16, 1024)
	req := normalExecRequest()
	receipt, err := c.CreateAttachedExec(context.Background(), normalExecHandle{id: "container-actual"}, normalExecDockerRequest(req))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		out DockerNormalExecOutcome
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, startErr := c.StartAttachedExecOnce(ctx, receipt, receipt.StdinEnabled, []byte(req.Stdin), testNormalSink(nil))
		done <- result{out: out, err: startErr}
	}()
	select {
	case <-inspectEntered:
	case <-time.After(2 * time.Second):
		cancel()
		close(releaseInspect)
		t.Fatal("final observation did not start after stream drain")
	}
	cancel()
	close(releaseInspect)
	got := <-done
	require.ErrorIs(t, got.err, context.Canceled)
	require.Equal(t, DockerNormalExecTransportPartial, got.out.Transport)
	require.Equal(t, DockerNormalExecProcessSucceeded, got.out.Observation.State)
	require.Equal(t, 1, fake.attachCalls)
}

func TestDockerNormalExecRealPinnedNoEgressTransport(t *testing.T) {
	if testing.Short() || strings.TrimSpace(os.Getenv("DOCKER_HOST")) == "" {
		t.Skip("set DOCKER_HOST explicitly and omit -short to run disposable Docker transport proof")
	}
	if os.Getenv("CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST") != "1" {
		t.Skip("set CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 to run real Docker transport proof")
	}
	host := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if host == "" {
		t.Skip("DOCKER_HOST is required for real Docker probe")
	}
	const imageID = "sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11"
	api, err := client.New(client.WithHost(host))
	require.NoError(t, err)
	defer api.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	image, err := api.ImageInspect(ctx, imageID)
	require.NoError(t, err)
	require.Equal(t, imageID, image.ID, "must use the locally verified Craft image identity")
	require.Equal(t, "linux", image.Os)
	require.Equal(t, "arm64", image.Architecture)

	created, err := api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: imageID, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"exec sleep infinity"},
			User: "root", OpenStdin: false, Tty: false,
		},
		HostConfig: &container.HostConfig{NetworkMode: "none", AutoRemove: false},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Logf("disposable image_id=%s container_id=%s", imageID, created.ID)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, removeErr := api.ContainerRemove(cleanupCtx, created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		if removeErr != nil {
			t.Errorf("remove disposable container %s: %v", created.ID, removeErr)
			return
		}
		inspection, inspectErr := api.ContainerInspect(cleanupCtx, created.ID, client.ContainerInspectOptions{})
		if inspectErr == nil {
			t.Errorf("disposable container %s remains after removal (name=%s)", created.ID, inspection.Container.Name)
			return
		}
		t.Logf("disposable container cleanup verified absent container_id=%s", created.ID)
	})
	_, err = api.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	require.NoError(t, err)

	docker, err := NewDockerNormalExecClient(api, DockerNormalExecConfig{RPCTimeout: 5 * time.Second, MaxInputBytes: 1024, MaxOutputBytes: 4096})
	require.NoError(t, err)
	markerPath := "/tmp/craft-t19-normal-output-marker"
	marker := "craft-t19-normal-output-" + strings.ReplaceAll(t.Name(), "/", "-")
	script := "IFS= read -r input; test \"$input\" = stdin-marker || exit 41; printf '%s\\n' '" + marker + "' >> '" + markerPath + "'; printf '%s\\n' 'stdout:" + marker + "'; printf '%s\\n' 'stderr:" + marker + "' >&2"
	request := RemoteExecRequest{
		Command: "/bin/sh", Args: []string{"-c", script}, Stdin: "stdin-marker\n", Timeout: 5 * time.Second,
		User: "10001:10001", WorkDir: "/workspace",
	}
	receipt, err := docker.CreateAttachedExec(ctx, normalExecHandle{id: created.ID}, normalExecDockerRequest(request))
	require.NoError(t, err)
	t.Logf("normal-output exec_id=%s container_id=%s", receipt.ExecID, receipt.ContainerID)
	var stdout, stderr strings.Builder
	sink := func(_ context.Context, _ DockerNormalExecReceipt, stream string, chunk []byte) error {
		switch stream {
		case "stdout":
			_, _ = stdout.Write(chunk)
		case "stderr":
			_, _ = stderr.Write(chunk)
		default:
			return errors.New("unexpected output stream")
		}
		return nil
	}
	outcome, err := docker.StartAttachedExecOnce(ctx, receipt, receipt.StdinEnabled, []byte(request.Stdin), normalExecFunctionSink(sink))
	require.NoError(t, err)
	require.Equal(t, DockerNormalExecTransportComplete, outcome.Transport)
	require.Equal(t, DockerNormalExecProcessSucceeded, outcome.Observation.State)
	require.NotNil(t, outcome.Observation.ExitCode)
	require.Zero(t, *outcome.Observation.ExitCode)
	require.True(t, outcome.StartEvidence)
	require.Equal(t, "stdout:"+marker+"\n", stdout.String())
	require.Equal(t, "stderr:"+marker+"\n", stderr.String())
	require.Equal(t, int64(len(stdout.String())+len(stderr.String())), outcome.OutputBytes)
	require.False(t, outcome.OutputTruncated)

	archive, err := api.CopyFromContainer(ctx, created.ID, client.CopyFromContainerOptions{SourcePath: markerPath})
	require.NoError(t, err)
	defer archive.Content.Close()
	tarReader := tar.NewReader(archive.Content)
	_, err = tarReader.Next()
	require.NoError(t, err)
	contents, err := io.ReadAll(io.LimitReader(tarReader, 1024))
	require.NoError(t, err)
	require.Equal(t, marker+"\n", string(contents), "marker side effect must occur exactly once")
}

func testNormalSink(_ *bytes.Buffer) DockerNormalExecChunkSink {
	return normalExecFunctionSink(func(context.Context, DockerNormalExecReceipt, string, []byte) error { return nil })
}

func normalExecWithOutput(stdout, malformed []byte) *normalExecFake {
	f := &normalExecFake{inspectResults: []client.ExecInspectResult{
		createdExecInspect(), createdExecInspect(),
		{ID: "exec-actual", ContainerID: "container-actual", ExitCode: 0},
	}}
	f.attachFn = func() client.ExecAttachResult {
		conn := newNormalExecTestConn()
		f.conn = conn
		go func() {
			_, _ = io.ReadAll(conn.stdinReader)
			if malformed != nil {
				_, _ = conn.outputWriter.Write(malformed)
			} else if len(stdout) > 0 {
				writeNormalExecFrameSplit(conn.outputWriter, stdcopy.Stdout, stdout)
			}
			_ = conn.outputWriter.Close()
		}()
		return client.ExecAttachResult{HijackedResponse: client.NewHijackedResponse(conn, "application/vnd.docker.raw-stream")}
	}
	return f
}

func writeNormalExecFrameSplit(w io.Writer, stream stdcopy.StdType, payload []byte) {
	frame := make([]byte, 8+len(payload))
	frame[0] = byte(stream)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	for _, size := range []int{1, 2, 1, 3, len(frame)} {
		if len(frame) == 0 {
			return
		}
		if size > len(frame) {
			size = len(frame)
		}
		_, _ = w.Write(frame[:size])
		frame = frame[size:]
	}
	if len(frame) > 0 {
		_, _ = w.Write(frame)
	}
}

type normalExecTestConn struct {
	stdinWriter     *io.PipeWriter
	stdinReader     *io.PipeReader
	outputWriter    *io.PipeWriter
	outputReader    *io.PipeReader
	closeWriteCount int
	mu              sync.Mutex
}

func newNormalExecTestConn() *normalExecTestConn {
	stdinReader, stdinWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	return &normalExecTestConn{stdinWriter: stdinWriter, stdinReader: stdinReader, outputWriter: outputWriter, outputReader: outputReader}
}

func (c *normalExecTestConn) Read(p []byte) (int, error)  { return c.outputReader.Read(p) }
func (c *normalExecTestConn) Write(p []byte) (int, error) { return c.stdinWriter.Write(p) }
func (c *normalExecTestConn) Close() error {
	_ = c.stdinWriter.Close()
	_ = c.outputReader.Close()
	return nil
}
func (c *normalExecTestConn) CloseWrite() error {
	c.mu.Lock()
	c.closeWriteCount++
	c.mu.Unlock()
	return c.stdinWriter.Close()
}
func (*normalExecTestConn) LocalAddr() net.Addr              { return testNormalAddr("local") }
func (*normalExecTestConn) RemoteAddr() net.Addr             { return testNormalAddr("remote") }
func (*normalExecTestConn) SetDeadline(time.Time) error      { return nil }
func (*normalExecTestConn) SetReadDeadline(time.Time) error  { return nil }
func (*normalExecTestConn) SetWriteDeadline(time.Time) error { return nil }

type testNormalAddr string

func (a testNormalAddr) Network() string { return "test" }
func (a testNormalAddr) String() string  { return string(a) }
