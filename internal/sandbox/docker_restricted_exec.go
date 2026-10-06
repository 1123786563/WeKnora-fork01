package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

// DockerOutputlessExecRequest is deliberately separate from RemoteExecRequest:
// callers must explicitly accept that Docker cannot recover detached output.
type DockerOutputlessExecRequest struct {
	Request       RemoteExecRequest
	DiscardOutput bool
}

// DockerOutputlessExecReceipt identifies the one inert exec configuration.
type DockerOutputlessExecReceipt struct {
	ContainerID string
	ExecID      string
}

type DockerOutputlessExecState string

const (
	DockerOutputlessUnknown   DockerOutputlessExecState = "unknown"
	DockerOutputlessRunning   DockerOutputlessExecState = "running"
	DockerOutputlessSucceeded DockerOutputlessExecState = "succeeded"
	DockerOutputlessFailed    DockerOutputlessExecState = "failed"
)

// DockerOutputlessExecObservation never implies empty stdout or stderr.
type DockerOutputlessExecObservation struct {
	State           DockerOutputlessExecState
	ExitCode        *int
	OutputAvailable bool
}

// DockerOutputlessExecClient is the intentionally restricted provider seam
// used by Craft composition; ordinary Docker clients do not implement the
// full RemoteExecInitiator contract through it.
type DockerOutputlessExecClient interface {
	CreateOutputlessExec(context.Context, RemoteSandboxHandle, DockerOutputlessExecRequest) (DockerOutputlessExecReceipt, error)
	StartOutputlessExec(context.Context, DockerOutputlessExecReceipt, time.Duration) error
	ObserveOutputlessExec(context.Context, DockerOutputlessExecReceipt, bool) (DockerOutputlessExecObservation, error)
}

// CreateOutputlessExec creates an inert exec. It rejects stream-dependent
// requests before contacting Docker and never restarts a stopped container.
// dockerExecStarterAPI pins the detached-start method set at compile time:
// a moby client upgrade that changes the ExecStart signature must break the
// build here instead of degrading every restricted exec to "unsupported".
type dockerExecStarterAPI = interface {
	ExecStart(context.Context, string, client.ExecStartOptions) (client.ExecStartResult, error)
}

var _ dockerExecStarterAPI = (*client.Client)(nil)

func (c *DockerRemoteClient) CreateOutputlessExec(ctx context.Context, handle RemoteSandboxHandle, restricted DockerOutputlessExecRequest) (DockerOutputlessExecReceipt, error) {
	req := restricted.Request
	if !restricted.DiscardOutput || req.Stdin != "" || req.OnOutput != nil {
		return DockerOutputlessExecReceipt{}, dockerInvalidRequest("RestrictedExec", "requires explicit output discard and does not support stdin or output callbacks")
	}
	id, err := dockerHandleID("RestrictedExec", handle)
	if err != nil {
		return DockerOutputlessExecReceipt{}, err
	}
	if req.Shell && len(req.Args) > 0 {
		return DockerOutputlessExecReceipt{}, dockerInvalidRequest("RestrictedExec", "shell requests must not carry args")
	}
	if strings.TrimSpace(req.Command) == "" {
		return DockerOutputlessExecReceipt{}, dockerInvalidRequest("RestrictedExec", "command is required")
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if timeout > time.Duration(1<<62) {
		return DockerOutputlessExecReceipt{}, dockerInvalidRequest("RestrictedExec", "timeout exceeds the supported upper bound")
	}
	created, err := c.api.ExecCreate(ctx, id, client.ExecCreateOptions{
		Cmd: dockerExecCommand(req, timeout), User: dockerExecUser(req.User),
		WorkingDir: req.WorkDir, Env: dockerEnvSlice(req.Env),
		AttachStdin: false, AttachStdout: false, AttachStderr: false,
	})
	if err != nil {
		return DockerOutputlessExecReceipt{}, dockerError("RestrictedExecCreate", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		return DockerOutputlessExecReceipt{}, errors.New("restricted Docker exec create returned empty ID")
	}
	return DockerOutputlessExecReceipt{ContainerID: id, ExecID: created.ID}, nil
}

// StartOutputlessExec issues exactly one bounded detached ExecStart. The
// caller must durably claim the receipt before calling this method.
func (c *DockerRemoteClient) StartOutputlessExec(ctx context.Context, receipt DockerOutputlessExecReceipt, rpcTimeout time.Duration) error {
	if c == nil || c.api == nil || receipt.ContainerID == "" || receipt.ExecID == "" {
		return fmt.Errorf("invalid restricted Docker exec receipt")
	}
	if rpcTimeout <= 0 {
		rpcTimeout = DefaultDockerHTTPTimeout
	}
	startCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	api := c.api
	// The normal client is wrapped for short RPCs. Unwrap only to reach the
	// versioned ExecStart method, while preserving the caller's local deadline.
	if wrapped, ok := api.(*dockerRPCTimeoutAPI); ok {
		api = wrapped.inner
	}
	starter, ok := api.(dockerExecStarterAPI)
	if !ok {
		return fmt.Errorf("restricted Docker ExecStart unsupported")
	}
	_, err := starter.ExecStart(startCtx, receipt.ExecID, client.ExecStartOptions{Detach: true, TTY: false})
	if err != nil {
		return fmt.Errorf("restricted Docker ExecStart outcome unknown: %w", err)
	}
	return nil
}

// ObserveOutputlessExec inspects only the bound exec ID. A terminal state is
// accepted only when this process has positive prior running evidence.
func (c *DockerRemoteClient) ObserveOutputlessExec(ctx context.Context, receipt DockerOutputlessExecReceipt, previouslyRunning bool) (DockerOutputlessExecObservation, error) {
	unknown := DockerOutputlessExecObservation{State: DockerOutputlessUnknown, OutputAvailable: false}
	if c == nil || c.api == nil || receipt.ContainerID == "" || receipt.ExecID == "" {
		return unknown, fmt.Errorf("invalid restricted Docker exec receipt")
	}
	inspected, err := c.api.ExecInspect(ctx, receipt.ExecID, client.ExecInspectOptions{})
	if err != nil {
		return unknown, dockerError("RestrictedExecInspect", err)
	}
	if inspected.ID != receipt.ExecID || inspected.ContainerID != receipt.ContainerID {
		// Identity mismatch is an explicit error, not a silent unknown: a
		// persistent identity anomaly must surface now, not burn the whole
		// wait window polling (the docker_normal inspectRaw precedent).
		return unknown, dockerError("RestrictedExecIdentityMismatch",
			fmt.Errorf("inspected %s/%s does not match receipt %s/%s",
				inspected.ID, inspected.ContainerID, receipt.ExecID, receipt.ContainerID))
	}
	if inspected.Running {
		return DockerOutputlessExecObservation{State: DockerOutputlessRunning, OutputAvailable: false}, nil
	}
	if !previouslyRunning {
		return unknown, nil
	}
	exit := inspected.ExitCode
	state := DockerOutputlessSucceeded
	if exit != 0 {
		state = DockerOutputlessFailed
	}
	return DockerOutputlessExecObservation{State: state, ExitCode: &exit, OutputAvailable: false}, nil
}
