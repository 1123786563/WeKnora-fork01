package sandbox

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type restrictedExecEngine struct {
	dockerEngineAPI
	createCalls   int
	startCalls    int
	attachCalls   int
	createOptions client.ExecCreateOptions
	inspect       client.ExecInspectResult
	inspectErr    error
	startErr      error
}

func (f *restrictedExecEngine) ExecCreate(_ context.Context, containerID string, options client.ExecCreateOptions) (client.ExecCreateResult, error) {
	f.createCalls++
	f.createOptions = options
	return client.ExecCreateResult{ID: "exec-restricted"}, nil
}
func (f *restrictedExecEngine) ExecStart(_ context.Context, _ string, options client.ExecStartOptions) (client.ExecStartResult, error) {
	f.startCalls++
	if !options.Detach || options.TTY {
		return client.ExecStartResult{}, errors.New("bad start options")
	}
	return client.ExecStartResult{}, f.startErr
}
func (f *restrictedExecEngine) ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error) {
	f.attachCalls++
	return client.ExecAttachResult{}, errors.New("attach must not be called")
}
func (f *restrictedExecEngine) ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error) {
	return f.inspect, f.inspectErr
}

func TestDockerRestrictedExecRejectsStreamsBeforeCreate(t *testing.T) {
	engine := &restrictedExecEngine{}
	docker := newDockerRemoteClientWithAPI(engine, dockerRuntimeSettings{})
	handle := testHandle("container-restricted")
	callback := func(string, []byte) {}
	for _, tc := range []struct {
		name string
		req  DockerOutputlessExecRequest
	}{
		{"implicit output", DockerOutputlessExecRequest{Request: RemoteExecRequest{Command: "true"}}},
		{"stdin", DockerOutputlessExecRequest{DiscardOutput: true, Request: RemoteExecRequest{Command: "true", Stdin: "x"}}},
		{"callback", DockerOutputlessExecRequest{DiscardOutput: true, Request: RemoteExecRequest{Command: "true", OnOutput: callback}}},
		{"empty command", DockerOutputlessExecRequest{DiscardOutput: true, Request: RemoteExecRequest{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := docker.CreateOutputlessExec(context.Background(), handle, tc.req)
			require.Error(t, err)
			require.Zero(t, engine.createCalls)
		})
	}
}

func TestDockerRestrictedExecUsesDetachedStartOnceAndNeverAttach(t *testing.T) {
	engine := &restrictedExecEngine{}
	docker := newDockerRemoteClientWithAPI(engine, dockerRuntimeSettings{})
	receipt, err := docker.CreateOutputlessExec(context.Background(), testHandle("container-restricted"), DockerOutputlessExecRequest{DiscardOutput: true, Request: RemoteExecRequest{Command: "true", Timeout: time.Second}})
	require.NoError(t, err)
	require.Equal(t, 1, engine.createCalls)
	require.False(t, engine.createOptions.AttachStdout)
	require.False(t, engine.createOptions.AttachStderr)
	require.NoError(t, docker.StartOutputlessExec(context.Background(), receipt, time.Second))
	require.Equal(t, 1, engine.startCalls)
	require.Zero(t, engine.attachCalls)
	engine.startErr = context.DeadlineExceeded
	require.Error(t, docker.StartOutputlessExec(context.Background(), receipt, time.Second), "ambiguous failed start must be surfaced")
	require.Equal(t, 2, engine.startCalls, "each API invocation makes at most one call; coordinator must forbid retry after claim")
	require.Zero(t, engine.attachCalls)
}

func TestDockerRestrictedObserveRequiresSameIDAndPositiveRunningEvidence(t *testing.T) {
	engine := &restrictedExecEngine{inspect: client.ExecInspectResult{ID: "exec-restricted", ContainerID: "container-restricted", ExitCode: 0}}
	docker := newDockerRemoteClientWithAPI(engine, dockerRuntimeSettings{})
	receipt := DockerOutputlessExecReceipt{ContainerID: "container-restricted", ExecID: "exec-restricted"}
	obs, err := docker.ObserveOutputlessExec(context.Background(), receipt, false)
	require.NoError(t, err)
	require.Equal(t, DockerOutputlessUnknown, obs.State, "false/zero without positive start evidence is unknown")
	require.False(t, obs.OutputAvailable)
	engine.inspect.Running = true
	obs, err = docker.ObserveOutputlessExec(context.Background(), receipt, false)
	require.NoError(t, err)
	require.Equal(t, DockerOutputlessRunning, obs.State)
	engine.inspect.Running = false
	obs, err = docker.ObserveOutputlessExec(context.Background(), receipt, true)
	require.NoError(t, err)
	require.Equal(t, DockerOutputlessSucceeded, obs.State)
	require.Equal(t, 0, *obs.ExitCode)
	engine.inspect.ContainerID = "other-container"
	obs, err = docker.ObserveOutputlessExec(context.Background(), receipt, true)
	// Identity mismatch is an EXPLICIT error (the inspectRaw precedent and
	// the code comment's own invariant), never a silent unknown that burns
	// the whole wait window polling.
	require.ErrorContains(t, err, "RestrictedExecIdentityMismatch")
	require.Equal(t, DockerOutputlessUnknown, obs.State)
	engine.inspectErr = errors.New("404 not found")
	obs, err = docker.ObserveOutputlessExec(context.Background(), receipt, true)
	require.Error(t, err)
	require.Equal(t, DockerOutputlessUnknown, obs.State)
}

// Set WEKNORA_DOCKER_RESTRICTED_INTEGRATION=1 to exercise one real detached
// start against the pinned local sandbox image. The test creates and removes
// one isolated network-none container.
func TestDockerRestrictedExecRealDockerMarkerOnce(t *testing.T) {
	if os.Getenv("WEKNORA_DOCKER_RESTRICTED_INTEGRATION") != "1" {
		t.Skip("real Docker probe is opt-in")
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	api, err := client.New(client.WithHost(host))
	require.NoError(t, err)
	defer api.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	created, err := api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &container.Config{Image: "wechatopenai/weknora-sandbox:main", Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"exec sleep infinity"}, User: "root"},
		HostConfig: &container.HostConfig{NetworkMode: "none"},
	})
	require.NoError(t, err)
	defer func() {
		_, _ = api.ContainerRemove(context.Background(), created.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	}()
	_, err = api.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	require.NoError(t, err)
	docker := newDockerRemoteClientWithAPI(api, dockerRuntimeSettings{})
	marker := "/tmp/weknora-restricted-marker"
	markerReceipt, err := docker.CreateOutputlessExec(ctx, &dockerSandboxHandle{id: created.ID}, DockerOutputlessExecRequest{DiscardOutput: true, Request: RemoteExecRequest{Command: "/bin/sh", Args: []string{"-c", "sleep 1; printf 'marker\\n' >> " + marker}, Timeout: 5 * time.Second}})
	require.NoError(t, err)
	require.NoError(t, docker.StartOutputlessExec(ctx, markerReceipt, 3*time.Second))
	seenRunning := false
	var final DockerOutputlessExecObservation
	deadline := time.Now().Add(7 * time.Second)
	for time.Now().Before(deadline) {
		obs, observeErr := docker.ObserveOutputlessExec(ctx, markerReceipt, seenRunning)
		if observeErr == nil {
			final = obs
		}
		if observeErr == nil && obs.State == DockerOutputlessRunning {
			seenRunning = true
		}
		if observeErr == nil && (obs.State == DockerOutputlessSucceeded || obs.State == DockerOutputlessFailed) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.True(t, seenRunning, "detached exec should be observed running before terminal inspect")
	require.Equal(t, DockerOutputlessSucceeded, final.State)
	require.False(t, final.OutputAvailable)
	archive, err := api.CopyFromContainer(ctx, created.ID, client.CopyFromContainerOptions{SourcePath: marker})
	require.NoError(t, err)
	defer archive.Content.Close()
	tr := tar.NewReader(archive.Content)
	_, err = tr.Next()
	require.NoError(t, err)
	content, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, "marker\n", string(content), "marker side effect occurs exactly once")
}
