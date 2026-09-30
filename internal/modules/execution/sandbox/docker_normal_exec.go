package sandbox

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/moby/moby/client"
)

const (
	DefaultDockerNormalExecMaxInputBytes  int64 = 1 << 20
	DefaultDockerNormalExecMaxOutputBytes int64 = 8 << 20
	dockerNormalExecFrameBufferSize             = 32 << 10
)

var (
	ErrDockerNormalExecIdentityMismatch    = errors.New("Docker normal exec identity mismatch")
	ErrDockerNormalExecAlreadyStarted      = errors.New("Docker normal exec start already attempted")
	ErrDockerNormalExecInputMismatch       = errors.New("Docker normal exec stdin differs from created receipt")
	ErrDockerNormalExecInputLimit          = errors.New("Docker normal exec stdin exceeds configured byte limit")
	ErrDockerNormalExecOutputLimit         = errors.New("Docker normal exec output exceeds configured byte limit")
	ErrDockerNormalExecSinkRequired        = errors.New("Docker normal exec output sink is required")
	ErrDockerNormalExecInvalidFrame        = errors.New("Docker normal exec stream contains an invalid frame")
	ErrDockerNormalExecNotTerminal         = errors.New("Docker normal exec output ended before exact exec became terminal")
	ErrDockerNormalExecCallbackUnsupported = errors.New("Docker normal exec does not accept direct output callbacks")
)

// DockerNormalExecEngine is the complete Docker API surface this provider
// needs. In particular, it deliberately has no detached ExecStart method.
type DockerNormalExecEngine interface {
	ExecCreate(context.Context, string, client.ExecCreateOptions) (client.ExecCreateResult, error)
	ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error)
	ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error)
}

type DockerNormalExecConfig struct {
	RPCTimeout     time.Duration
	MaxInputBytes  int64
	MaxOutputBytes int64
}

// DockerNormalExecReceipt is the immutable ExecCreate receipt. Stdin content
// is not retained here; callers must stage it durably and supply matching
// bytes to StartAttachedExecOnce after their one-send claim.
type DockerNormalExecReceipt struct {
	ContainerID  string        `json:"container_id"`
	ExecID       string        `json:"exec_id"`
	StdinEnabled bool          `json:"stdin_enabled"`
	StdinBytes   int64         `json:"stdin_bytes"`
	StdinSHA256  string        `json:"stdin_sha256"`
	Timeout      time.Duration `json:"timeout_ns"`
}

type DockerNormalExecTransportState string

const (
	DockerNormalExecTransportComplete    DockerNormalExecTransportState = "complete"
	DockerNormalExecTransportPartial     DockerNormalExecTransportState = "partial"
	DockerNormalExecTransportUnavailable DockerNormalExecTransportState = "unavailable"
)

type DockerNormalExecProcessState string

const (
	DockerNormalExecProcessUnknown   DockerNormalExecProcessState = "unknown"
	DockerNormalExecProcessRunning   DockerNormalExecProcessState = "running"
	DockerNormalExecProcessSucceeded DockerNormalExecProcessState = "succeeded"
	DockerNormalExecProcessFailed    DockerNormalExecProcessState = "failed"
)

type DockerNormalExecObservation struct {
	State    DockerNormalExecProcessState
	ExitCode *int
}

type DockerNormalExecOutcome struct {
	Receipt         DockerNormalExecReceipt
	Transport       DockerNormalExecTransportState
	Observation     DockerNormalExecObservation
	StartEvidence   bool
	InputBytes      int64
	OutputBytes     int64
	OutputTruncated bool
}

// DockerNormalExecChunkSink must durably append each chunk before returning.
// Seal must be prompt and linearize against Append: after it returns, no
// in-flight or future Append may mutate the durable output. Append receives a
// cancellable context and implementations must check it before committing.
// Live projections must read this durable output separately; transport never
// invokes caller callbacks.
type DockerNormalExecChunkSink interface {
	Append(context.Context, DockerNormalExecReceipt, string, []byte) error
	Seal() error
}

// DockerNormalExecOutputCallback is retained only so callers can state that
// they require direct projection. The provider rejects it before ExecCreate;
// output projection must read the durable sink under a separate cursor contract.
type DockerNormalExecOutputCallback interface {
	Deliver(context.Context, string, []byte) error
	Seal()
}

// DockerNormalExecRequest fixes stdin attachment independently from its byte
// count, so enabled-empty stdin remains distinct from disabled stdin. It also
// keeps optional callback intent visible at create time so unsupported
// projection is rejected before any Docker side effect. Accepted output is
// delivered only to the durable chunk sink.
type DockerNormalExecRequest struct {
	Request        RemoteExecRequest
	StdinEnabled   bool
	OutputCallback DockerNormalExecOutputCallback
}

type DockerNormalExecClient struct {
	api            DockerNormalExecEngine
	rpcTimeout     time.Duration
	maxInputBytes  int64
	maxOutputBytes int64
	started        sync.Map // exec ID -> struct{}; remains set after any attach attempt.
}

func NewDockerNormalExecClient(api DockerNormalExecEngine, config DockerNormalExecConfig) (*DockerNormalExecClient, error) {
	if api == nil {
		return nil, errors.New("Docker normal exec engine is required")
	}
	if config.RPCTimeout <= 0 {
		config.RPCTimeout = DefaultDockerHTTPTimeout
	}
	if config.MaxInputBytes <= 0 {
		config.MaxInputBytes = DefaultDockerNormalExecMaxInputBytes
	}
	if config.MaxOutputBytes <= 0 {
		config.MaxOutputBytes = DefaultDockerNormalExecMaxOutputBytes
	}
	return &DockerNormalExecClient{api: api, rpcTimeout: config.RPCTimeout, maxInputBytes: config.MaxInputBytes, maxOutputBytes: config.MaxOutputBytes}, nil
}

// CreateAttachedExec creates an inert Exec with streams fixed at create time,
// then validates the exact daemon-issued IDs. It never starts the command.
func (c *DockerNormalExecClient) CreateAttachedExec(ctx context.Context, handle RemoteSandboxHandle, request DockerNormalExecRequest) (DockerNormalExecReceipt, error) {
	if c == nil || c.api == nil {
		return DockerNormalExecReceipt{}, errors.New("Docker normal exec client is unavailable")
	}
	if request.OutputCallback != nil || request.Request.OnOutput != nil {
		return DockerNormalExecReceipt{}, ErrDockerNormalExecCallbackUnsupported
	}
	req := request.Request
	containerID, err := dockerHandleID("NormalExecCreate", handle)
	if err != nil {
		return DockerNormalExecReceipt{}, err
	}
	if strings.TrimSpace(req.Command) == "" {
		return DockerNormalExecReceipt{}, dockerInvalidRequest("NormalExecCreate", "command is required")
	}
	if req.Shell && len(req.Args) > 0 {
		return DockerNormalExecReceipt{}, dockerInvalidRequest("NormalExecCreate", "shell requests must not carry args")
	}
	if int64(len(req.Stdin)) > c.maxInputBytes {
		return DockerNormalExecReceipt{}, ErrDockerNormalExecInputLimit
	}
	if !request.StdinEnabled && req.Stdin != "" {
		return DockerNormalExecReceipt{}, ErrDockerNormalExecInputMismatch
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if timeout > time.Duration(1<<62) {
		return DockerNormalExecReceipt{}, dockerInvalidRequest("NormalExecCreate", "timeout is out of range")
	}
	rpcCtx, cancel := context.WithTimeout(ctx, c.rpcTimeout)
	defer cancel()
	created, err := c.api.ExecCreate(rpcCtx, containerID, client.ExecCreateOptions{
		Cmd: dockerExecCommand(req, timeout), User: dockerExecUser(req.User),
		WorkingDir: req.WorkDir, Env: dockerEnvSlice(req.Env),
		AttachStdin: request.StdinEnabled, AttachStdout: true, AttachStderr: true,
		TTY: false,
	})
	if err != nil {
		return DockerNormalExecReceipt{}, dockerError("NormalExecCreate", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		return DockerNormalExecReceipt{}, errors.New("Docker normal exec create returned empty ID")
	}
	receipt := DockerNormalExecReceipt{
		ContainerID: containerID, ExecID: created.ID, StdinEnabled: request.StdinEnabled,
		StdinBytes: int64(len(req.Stdin)), StdinSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(req.Stdin))), Timeout: timeout,
	}
	inspectCtx, inspectCancel := context.WithTimeout(ctx, c.rpcTimeout)
	inspected, err := c.api.ExecInspect(inspectCtx, receipt.ExecID, client.ExecInspectOptions{})
	inspectCancel()
	if err != nil {
		return DockerNormalExecReceipt{}, dockerError("NormalExecCreateInspect", err)
	}
	if inspected.ID != receipt.ExecID || inspected.ContainerID != receipt.ContainerID {
		return DockerNormalExecReceipt{}, ErrDockerNormalExecIdentityMismatch
	}
	if inspected.Running {
		return DockerNormalExecReceipt{}, errors.New("new Docker normal exec was already running before the send claim")
	}
	return receipt, nil
}

// StartAttachedExecOnce is the only method in this provider that starts a
// Docker exec. Moby ExecAttach sends one non-detached /exec/{id}/start POST.
// The first attempt is permanently consumed before that call; errors never
// retry or reattach.
func (c *DockerNormalExecClient) StartAttachedExecOnce(
	ctx context.Context,
	receipt DockerNormalExecReceipt,
	stdinEnabled bool,
	stdin []byte,
	sink DockerNormalExecChunkSink,
) (DockerNormalExecOutcome, error) {
	out := DockerNormalExecOutcome{Receipt: receipt, Transport: DockerNormalExecTransportUnavailable, Observation: DockerNormalExecObservation{State: DockerNormalExecProcessUnknown}}
	if c == nil || c.api == nil || !validDockerNormalExecReceipt(receipt) {
		return out, errors.New("invalid Docker normal exec receipt")
	}
	if sink == nil {
		return out, ErrDockerNormalExecSinkRequired
	}
	if stdinEnabled != receipt.StdinEnabled || int64(len(stdin)) != receipt.StdinBytes || fmt.Sprintf("%x", sha256.Sum256(stdin)) != receipt.StdinSHA256 || int64(len(stdin)) > c.maxInputBytes {
		return out, ErrDockerNormalExecInputMismatch
	}
	if _, loaded := c.started.Load(receipt.ExecID); loaded {
		return out, ErrDockerNormalExecAlreadyStarted
	}
	// The preflight inspect gets its OWN timeout: sharing the create
	// budget let a slow daemon leave only残余 deadline for this cheap call,
	// failing the whole creation while the daemon keeps the already-created
	// (never-started, lazy) exec behind.
	preflightCtx, preflightCancel := context.WithTimeout(ctx, c.rpcTimeout)
	preflight, err := c.inspectRaw(preflightCtx, receipt)
	preflightCancel()
	if err != nil {
		return out, err
	}
	if preflight.Running {
		return out, errors.New("Docker normal exec is already running before its claimed start")
	}
	if _, loaded := c.started.LoadOrStore(receipt.ExecID, struct{}{}); loaded {
		return out, ErrDockerNormalExecAlreadyStarted
	}
	startTimeout := receipt.Timeout + dockerExecGrace
	if startTimeout <= 0 {
		startTimeout = DefaultTimeout + dockerExecGrace
	}
	startCtx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	attached, err := c.api.ExecAttach(startCtx, receipt.ExecID, client.ExecAttachOptions{TTY: false})
	if err != nil {
		observeCtx, observeCancel := c.observationContext(ctx)
		observation, inspectErr := c.ObserveAttachedExec(observeCtx, receipt, false)
		observeCancel()
		out.Observation = observation
		out.StartEvidence = observation.State == DockerNormalExecProcessRunning
		if inspectErr != nil {
			return out, errors.Join(fmt.Errorf("Docker normal ExecAttach outcome unknown: %w", err), inspectErr)
		}
		return out, fmt.Errorf("Docker normal ExecAttach outcome unknown: %w", err)
	}
	out.StartEvidence = true
	if attached.Conn == nil || attached.Reader == nil {
		if attached.Conn != nil {
			attached.Close()
		}
		observeCtx, observeCancel := c.observationContext(ctx)
		observation, inspectErr := c.ObserveAttachedExec(observeCtx, receipt, true)
		observeCancel()
		out.Observation = observation
		return out, errors.Join(errors.New("Docker normal ExecAttach returned no hijacked stream"), inspectErr)
	}
	var closeOnce sync.Once
	closeStream := func() { closeOnce.Do(attached.Close) }
	defer closeStream()
	writer := &dockerNormalExecBoundedWriter{ctx: startCtx, receipt: receipt, sink: sink, remaining: c.maxOutputBytes}
	streamDone := make(chan error, 1)
	go func() {
		streamDone <- demuxDockerExecStream(attached.Reader,
			dockerNormalExecMuxWriter{writer: writer, stream: "stdout"},
			dockerNormalExecMuxWriter{writer: writer, stream: "stderr"},
		)
	}()
	var inputDone <-chan dockerNormalExecInputResult
	inputChannel := make(chan dockerNormalExecInputResult, 1)
	if receipt.StdinEnabled {
		inputDone = inputChannel
		go func() {
			result := dockerNormalExecInputResult{}
			closer, ok := attached.Conn.(interface{ CloseWrite() error })
			if !ok {
				// A TLS-hijacked remote daemon connection has no
				// CloseWrite. Degrade like the legacy streamExec path: skip
				// the half-close (the server reads stdin to EOF on detach)
				// instead of failing an attach that already consumed the
				// durable send claim.
				logger.Warnf(ctx, "[DockerNormalExec] stream does not support stdin half-close; skipping CloseWrite")
				written, writeErr := io.Copy(attached.Conn, bytes.NewReader(stdin))
				result.bytes = written
				result.err = writeErr
				inputChannel <- result
				return
			}
			written, writeErr := io.Copy(attached.Conn, bytes.NewReader(stdin))
			result.bytes = written
			closeErr := closer.CloseWrite()
			if writeErr != nil {
				result.err = fmt.Errorf("write Docker normal exec stdin: %w", writeErr)
			} else if closeErr != nil {
				result.err = fmt.Errorf("half-close Docker normal exec stdin: %w", closeErr)
			}
			inputChannel <- result
		}()
	}
	var streamErr error
	var inputErr error
	streamFinished := false
	inputFinished := inputDone == nil
	var waitErr error
	for !streamFinished || !inputFinished {
		select {
		case streamErr = <-streamDone:
			streamFinished = true
			if streamErr != nil {
				closeStream()
			}
		case inputResult := <-inputDone:
			inputFinished = true
			out.InputBytes = inputResult.bytes
			inputErr = inputResult.err
			if inputErr != nil {
				closeStream()
			}
		case <-startCtx.Done():
			waitErr = startCtx.Err()
			closeStream()
			goto drain
		}
	}
drain:
	if !streamFinished {
		select {
		case streamErr = <-streamDone:
			streamFinished = true
		case <-time.After(dockerExecDrainGrace):
			streamErr = errors.New("Docker normal exec output drain timed out")
		}
	}
	if !inputFinished && inputDone != nil {
		select {
		case inputResult := <-inputDone:
			inputFinished = true
			out.InputBytes = inputResult.bytes
			inputErr = inputResult.err
		case <-time.After(dockerExecDrainGrace):
			inputErr = errors.New("Docker normal exec stdin drain timed out")
		}
	}
	if waitErr == nil {
		waitErr = startCtx.Err()
	}
	sealErr := writer.freezeAndSeal()
	writerState := writer.snapshot()
	out.OutputBytes = writerState.bytesWritten
	out.OutputTruncated = writerState.truncated
	if streamErr == nil && inputErr == nil && waitErr == nil && !writerState.truncated && sealErr == nil {
		out.Transport = DockerNormalExecTransportComplete
	} else {
		out.Transport = DockerNormalExecTransportPartial
	}
	observeCtx, observeCancel := c.observationContext(ctx)
	observation, inspectErr := c.ObserveAttachedExec(observeCtx, receipt, out.StartEvidence)
	observeCancel()
	out.Observation = observation
	if cancelErr := startCtx.Err(); cancelErr != nil {
		out.Transport = DockerNormalExecTransportPartial
		return out, cancelErr
	}
	if sealErr != nil {
		out.Transport = DockerNormalExecTransportPartial
		return out, fmt.Errorf("seal durable Docker normal exec output: %w", sealErr)
	}
	if writerState.err != nil {
		return out, writerState.err
	}
	if streamErr != nil {
		return out, fmt.Errorf("read Docker normal exec output: %w", streamErr)
	}
	if inputErr != nil {
		return out, inputErr
	}
	if waitErr != nil {
		return out, waitErr
	}
	if out.OutputTruncated {
		return out, ErrDockerNormalExecOutputLimit
	}
	if inspectErr != nil {
		out.Transport = DockerNormalExecTransportPartial
		return out, inspectErr
	}
	if out.Transport == DockerNormalExecTransportComplete && out.Observation.State != DockerNormalExecProcessSucceeded && out.Observation.State != DockerNormalExecProcessFailed {
		out.Transport = DockerNormalExecTransportPartial
		return out, ErrDockerNormalExecNotTerminal
	}
	if cancelErr := startCtx.Err(); cancelErr != nil {
		out.Transport = DockerNormalExecTransportPartial
		return out, cancelErr
	}
	return out, nil
}

func (c *DockerNormalExecClient) ObserveAttachedExec(ctx context.Context, receipt DockerNormalExecReceipt, positiveStartEvidence bool) (DockerNormalExecObservation, error) {
	unknown := DockerNormalExecObservation{State: DockerNormalExecProcessUnknown}
	if c == nil || c.api == nil || !validDockerNormalExecReceipt(receipt) {
		return unknown, errors.New("invalid Docker normal exec receipt")
	}
	inspected, err := c.inspectRaw(ctx, receipt)
	if err != nil {
		return unknown, err
	}
	if inspected.Running {
		return DockerNormalExecObservation{State: DockerNormalExecProcessRunning}, nil
	}
	if !positiveStartEvidence {
		return unknown, nil
	}
	exitCode := inspected.ExitCode
	state := DockerNormalExecProcessSucceeded
	if exitCode != 0 {
		state = DockerNormalExecProcessFailed
	}
	return DockerNormalExecObservation{State: state, ExitCode: &exitCode}, nil
}

func (c *DockerNormalExecClient) inspectRaw(ctx context.Context, receipt DockerNormalExecReceipt) (client.ExecInspectResult, error) {
	// Every inspect gets its OWN timeout budget (never a residual slice of
	// a caller's already-consumed deadline).
	inspectCtx, inspectCancel := context.WithTimeout(ctx, c.rpcTimeout)
	defer inspectCancel()
	inspected, err := c.api.ExecInspect(inspectCtx, receipt.ExecID, client.ExecInspectOptions{})
	if err != nil {
		return client.ExecInspectResult{}, dockerError("NormalExecInspect", err)
	}
	if inspected.ID != receipt.ExecID || inspected.ContainerID != receipt.ContainerID {
		return client.ExecInspectResult{}, ErrDockerNormalExecIdentityMismatch
	}
	return inspected, nil
}

func (c *DockerNormalExecClient) observationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), c.rpcTimeout)
}

func validDockerNormalExecReceipt(receipt DockerNormalExecReceipt) bool {
	return strings.TrimSpace(receipt.ContainerID) != "" && strings.TrimSpace(receipt.ExecID) != "" && receipt.StdinBytes >= 0 && (receipt.StdinEnabled || receipt.StdinBytes == 0) && receipt.StdinSHA256 != "" && receipt.Timeout > 0
}

type dockerNormalExecInputResult struct {
	bytes int64
	err   error
}

type dockerNormalExecBoundedWriter struct {
	mu           sync.Mutex
	ctx          context.Context
	receipt      DockerNormalExecReceipt
	sink         DockerNormalExecChunkSink
	remaining    int64
	bytesWritten int64
	truncated    bool
	err          error
	frozen       bool
}

func (w *dockerNormalExecBoundedWriter) writeStream(stream string, p []byte) (int, error) {
	originalLen := len(p)
	w.mu.Lock()
	if w.frozen {
		w.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	if err := w.ctx.Err(); err != nil {
		w.mu.Unlock()
		return 0, err
	}
	remaining := w.remaining
	w.mu.Unlock()
	if remaining <= 0 {
		w.mu.Lock()
		if !w.frozen {
			w.truncated = w.truncated || originalLen > 0
		}
		w.mu.Unlock()
		return originalLen, nil
	}
	truncated := int64(len(p)) > remaining
	if truncated {
		p = p[:int(remaining)]
	}
	if len(p) == 0 {
		return originalLen, nil
	}
	chunk := append([]byte(nil), p...)
	if err := w.sink.Append(w.ctx, w.receipt, stream, chunk); err != nil {
		writeErr := fmt.Errorf("persist Docker normal exec output: %w", err)
		w.mu.Lock()
		if !w.frozen {
			w.err = writeErr
		}
		w.mu.Unlock()
		return 0, writeErr
	}
	w.mu.Lock()
	if w.frozen {
		w.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	w.bytesWritten += int64(len(chunk))
	w.remaining -= int64(len(chunk))
	w.truncated = w.truncated || truncated
	w.mu.Unlock()
	return originalLen, nil
}

type dockerNormalExecWriterSnapshot struct {
	bytesWritten int64
	truncated    bool
	err          error
}

func (w *dockerNormalExecBoundedWriter) freezeAndSeal() error {
	w.mu.Lock()
	w.frozen = true
	w.mu.Unlock()
	return w.sink.Seal()
}

func (w *dockerNormalExecBoundedWriter) snapshot() dockerNormalExecWriterSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return dockerNormalExecWriterSnapshot{bytesWritten: w.bytesWritten, truncated: w.truncated, err: w.err}
}

func demuxDockerExecStream(reader io.Reader, stdout, stderr io.Writer) error {
	var header [8]byte
	buffer := make([]byte, dockerNormalExecFrameBufferSize)
	for {
		_, err := io.ReadFull(reader, header[:])
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("truncated Docker multiplex header: %w", err)
		}
		if header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return ErrDockerNormalExecInvalidFrame
		}
		var destination io.Writer
		switch header[0] {
		case 1:
			destination = stdout
		case 2:
			destination = stderr
		case 3:
			return errors.New("Docker daemon reported an exec stream error")
		default:
			return ErrDockerNormalExecInvalidFrame
		}
		remaining := uint64(binary.BigEndian.Uint32(header[4:8]))
		for remaining > 0 {
			chunkLen := len(buffer)
			if uint64(chunkLen) > remaining {
				chunkLen = int(remaining)
			}
			chunk := buffer[:chunkLen]
			if _, err := io.ReadFull(reader, chunk); err != nil {
				return fmt.Errorf("truncated Docker multiplex payload: %w", err)
			}
			n, err := destination.Write(chunk)
			if err != nil {
				return err
			}
			if n != len(chunk) {
				return io.ErrShortWrite
			}
			remaining -= uint64(chunkLen)
		}
	}
}

type dockerNormalExecMuxWriter struct {
	writer *dockerNormalExecBoundedWriter
	stream string
}

func (w dockerNormalExecMuxWriter) Write(p []byte) (int, error) {
	return w.writer.writeStream(w.stream, p)
}

var _ DockerNormalExecEngine = (*client.Client)(nil)
