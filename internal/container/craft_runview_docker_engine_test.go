package container

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	dockcontainer "github.com/moby/moby/api/types/container"
	dockimage "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type runViewDockerFake struct {
	network               client.NetworkInspectResult
	networkErr            error
	networkInspectCalls   int
	networkInspectNames   []string
	networkCreateCalls    int
	networkCreate         client.NetworkCreateOptions
	container             client.ContainerInspectResult
	containerErr          error
	containerCreateCalls  int
	containerCreate       client.ContainerCreateOptions
	containerCreateErr    error
	containerStartCalls   int
	containerStartErr     error
	image                 client.ImageInspectResult
	imageErr              error
	execCreateCalls       int
	execOptions           client.ExecCreateOptions
	execAttachCalls       int
	execStream            []byte
	execInspect           client.ExecInspectResult
	execErr               error
	execInspectCalls      int
	containerInspectCalls int
	containerInspectNames []string
}

func (f *runViewDockerFake) NetworkInspect(_ context.Context, name string, _ client.NetworkInspectOptions) (client.NetworkInspectResult, error) {
	f.networkInspectCalls++
	f.networkInspectNames = append(f.networkInspectNames, name)
	return f.network, f.networkErr
}
func (f *runViewDockerFake) NetworkCreate(_ context.Context, name string, o client.NetworkCreateOptions) (client.NetworkCreateResult, error) {
	f.networkCreateCalls++
	f.networkCreate = o
	f.network.Network = network.Inspect{Network: network.Network{ID: "network-id", Name: name, Driver: o.Driver, Internal: o.Internal, Labels: o.Labels, Scope: "local"}}
	f.networkErr = nil
	return client.NetworkCreateResult{ID: "network-id"}, nil
}
func (f *runViewDockerFake) ContainerInspect(_ context.Context, name string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	f.containerInspectCalls++
	f.containerInspectNames = append(f.containerInspectNames, name)
	return f.container, f.containerErr
}
func (f *runViewDockerFake) ContainerCreate(_ context.Context, o client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	f.containerCreateCalls++
	f.containerCreate = o
	return client.ContainerCreateResult{ID: "container-id"}, f.containerCreateErr
}
func (f *runViewDockerFake) ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error) {
	f.containerStartCalls++
	return client.ContainerStartResult{}, f.containerStartErr
}
func (f *runViewDockerFake) ImageInspect(context.Context, string, ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	return f.image, f.imageErr
}
func (f *runViewDockerFake) ExecCreate(_ context.Context, _ string, o client.ExecCreateOptions) (client.ExecCreateResult, error) {
	f.execCreateCalls++
	f.execOptions = o
	return client.ExecCreateResult{ID: "probe-id"}, nil
}
func (f *runViewDockerFake) ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error) {
	f.execAttachCalls++
	left, right := net.Pipe()
	_ = right.Close()
	return client.ExecAttachResult{HijackedResponse: client.HijackedResponse{Conn: left, Reader: bufio.NewReader(bytes.NewReader(f.execStream))}}, nil
}
func (f *runViewDockerFake) ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error) {
	f.execInspectCalls++
	return f.execInspect, f.execErr
}

func runViewDockerNetworkSpec() CraftRunViewContainerNetworkSpec {
	return CraftRunViewContainerNetworkSpec{
		Name: "rv-network-abc", Driver: "bridge", Internal: true,
		Labels: map[string]string{
			craftRunViewLabelGeneration: strings.Repeat("a", 64),
			craftRunViewLabelRuntime:    "rv-runtime-abc",
			craftRunViewLabelContainer:  "rv-container-abc",
			craftRunViewLabelImage:      "sha256:" + strings.Repeat("b", 64),
		},
	}
}

func validRunViewNetworkDockerFake() *runViewDockerFake {
	spec := runViewDockerNetworkSpec()
	return &runViewDockerFake{network: client.NetworkInspectResult{Network: network.Inspect{Network: network.Network{
		ID: "network-id", Name: spec.Name, Driver: spec.Driver, Internal: spec.Internal,
		Labels: cloneRunViewLabels(spec.Labels), Scope: "local",
	}}}}
}

func TestCraftRunViewDockerEngineObserveNetworkIsReadOnlyAndReturnsExactIdentity(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		fake := validRunViewNetworkDockerFake()
		engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
		observer, ok := any(engine).(interface {
			ObserveNetwork(context.Context, CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, bool, error)
		})
		require.True(t, ok, "admitted provider requires a split read-only network observation")
		spec := runViewDockerNetworkSpec()
		got, found, err := observer.ObserveNetwork(context.Background(), spec)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "network-id", got.ID)
		require.Equal(t, spec.Name, got.Name)
		require.Equal(t, spec.Driver, got.Driver)
		require.True(t, got.Internal)
		require.Equal(t, spec.Labels, got.Labels)
		require.Equal(t, []string{spec.Name}, fake.networkInspectNames)
		require.Equal(t, 0, fake.networkCreateCalls)
		require.Equal(t, 0, fake.containerCreateCalls)
		require.Equal(t, 0, fake.containerStartCalls)
		require.Equal(t, 0, fake.execCreateCalls)
	})

	t.Run("not found", func(t *testing.T) {
		fake := &runViewDockerFake{networkErr: cerrdefs.ErrNotFound}
		engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
		observer, ok := any(engine).(interface {
			ObserveNetwork(context.Context, CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, bool, error)
		})
		require.True(t, ok)
		got, found, err := observer.ObserveNetwork(context.Background(), runViewDockerNetworkSpec())
		require.NoError(t, err)
		require.False(t, found)
		require.Empty(t, got.ID)
		require.Equal(t, 0, fake.networkCreateCalls, "absence observation must not create the network")
	})
}

func TestCraftRunViewDockerEngineObserveNetworkRejectsForeignAttachmentWithExpectedName(t *testing.T) {
	fake := validRunViewNetworkDockerFake()
	spec := runViewDockerNetworkSpec()
	const foreignDockerID = "foreign-docker-container-id"
	fake.network.Network.Containers = map[string]network.EndpointResource{
		foreignDockerID: {Name: spec.Labels[craftRunViewLabelContainer]},
	}
	fake.container = client.ContainerInspectResult{Container: dockcontainer.InspectResponse{
		ID: foreignDockerID, Name: "/" + spec.Labels[craftRunViewLabelContainer],
		Config: &dockcontainer.Config{Labels: map[string]string{
			craftRunViewLabelGeneration: strings.Repeat("f", 64),
			craftRunViewLabelRuntime:    "rv-runtime-foreign",
			craftRunViewLabelContainer:  spec.Labels[craftRunViewLabelContainer],
			craftRunViewLabelImage:      spec.Labels[craftRunViewLabelImage],
		}},
	}}

	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	got, found, err := engine.ObserveNetwork(context.Background(), spec)
	require.Error(t, err, "an endpoint name alone must not authorize an attached Docker ID")
	require.False(t, found)
	require.Empty(t, got)
	require.Equal(t, []string{foreignDockerID}, fake.containerInspectNames, "the attachment map key must be inspected as the Docker ID")
}

func TestCraftRunViewDockerEngineObserveNetworkAcceptsInspectedGenerationAttachment(t *testing.T) {
	fake := validRunViewNetworkDockerFake()
	spec := runViewDockerNetworkSpec()
	const dockerID = "generation-docker-container-id"
	fake.network.Network.Containers = map[string]network.EndpointResource{
		dockerID: {Name: spec.Labels[craftRunViewLabelContainer]},
	}
	fake.container = client.ContainerInspectResult{Container: dockcontainer.InspectResponse{
		ID: dockerID, Name: "/" + spec.Labels[craftRunViewLabelContainer],
		Config: &dockcontainer.Config{Labels: cloneRunViewLabels(spec.Labels)},
	}}

	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	got, found, err := engine.ObserveNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "network-id", got.ID)
	require.Equal(t, []string{dockerID}, fake.containerInspectNames)
}

func TestCraftRunViewDockerEngineCreateGenerationNetworkSendsOnceThenInspectsReceipt(t *testing.T) {
	fake := &runViewDockerFake{}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	creator, ok := any(engine).(interface {
		CreateGenerationNetwork(context.Context, CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error)
	})
	require.True(t, ok, "network mutation needs a separate claimed-send operation")
	spec := runViewDockerNetworkSpec()
	got, err := creator.CreateGenerationNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, "network-id", got.ID)
	require.Equal(t, spec.Name, got.Name)
	require.Equal(t, 1, fake.networkCreateCalls)
	require.Equal(t, "network-id", fake.networkInspectNames[len(fake.networkInspectNames)-1], "receipt must be independently inspected by returned Docker ID")
	require.True(t, fake.networkCreate.Internal)
	require.Equal(t, spec.Driver, fake.networkCreate.Driver)
	require.Equal(t, spec.Labels, fake.networkCreate.Labels)
}

func TestCraftRunViewDockerEngineObserveRuntimeProbeNeverExecutes(t *testing.T) {
	fake := &runViewDockerFake{}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	observer, ok := any(engine).(interface {
		ObserveRuntimeProbe(context.Context, string) (CraftRunViewRuntimeProbe, bool, error)
	})
	require.True(t, ok, "runtime probe observation must be a distinct read-only operation")
	probe, found, err := observer.ObserveRuntimeProbe(context.Background(), "container-id")
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.False(t, found)
	require.Empty(t, probe)
	require.Zero(t, fake.execCreateCalls)
	require.Zero(t, fake.execAttachCalls)
	require.Zero(t, fake.execInspectCalls)
}

func TestCraftRunViewDockerEngineCanceledSplitSendsDoNotCallDocker(t *testing.T) {
	fake := validRunViewNetworkDockerFake()
	fake.execStream = stdcopyFrame(1, []byte("1.18.4\n"+strings.Repeat("b", 64)+"  /usr/local/bin/opencode\n"+strings.Repeat("c", 64)+"  /etc/craft/runtime-config.json\n"))
	fake.execInspect = client.ExecInspectResult{ID: "probe-id", ContainerID: "container-id", ExitCode: 0}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, networkErr := engine.CreateGenerationNetwork(ctx, runViewDockerNetworkSpec())
	require.ErrorIs(t, networkErr, context.Canceled)
	_, probeErr := engine.SendRuntimeProbe(ctx, "container-id")
	require.ErrorIs(t, probeErr, context.Canceled)
	require.Zero(t, fake.networkCreateCalls)
	require.Zero(t, fake.execCreateCalls)
	require.Zero(t, fake.execAttachCalls)
}

func TestCraftRunViewDockerEngineEnsuresOnlyPinnedInternalGenerationNetwork(t *testing.T) {
	fake := &runViewDockerFake{networkErr: cerrdefs.ErrNotFound}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	digest := "sha256:" + strings.Repeat("a", 64)
	spec := CraftRunViewContainerNetworkSpec{Name: "rv-network-abc", Driver: "bridge", Internal: true, Labels: map[string]string{craftRunViewLabelGeneration: strings.Repeat("a", 64), craftRunViewLabelRuntime: "rv-runtime-abc", craftRunViewLabelContainer: "rv-container-abc", craftRunViewLabelImage: digest}}
	got, err := engine.EnsurePrivateNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, "network-id", got.ID)
	require.Equal(t, 1, fake.networkCreateCalls)
	require.True(t, fake.networkCreate.Internal)
	require.Equal(t, "bridge", fake.networkCreate.Driver)
	require.Equal(t, spec.Labels, fake.networkCreate.Labels)
	fake.networkErr = errors.New("permission denied")
	_, err = engine.EnsurePrivateNetwork(context.Background(), spec)
	require.Error(t, err)
	require.Equal(t, 1, fake.networkCreateCalls, "uncertain inspect cannot trigger network create")
}

func TestCraftRunViewDockerEngineCreateTranslatesRestrictedSpecAndDoesNotRetry(t *testing.T) {
	fake := &runViewDockerFake{containerCreateErr: nil}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	req := runViewDockerCreateRequest()
	id, err := engine.CreateContainer(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "container-id", id)
	require.Equal(t, 1, fake.containerCreateCalls)
	cfg := fake.containerCreate.Config
	require.Equal(t, req.ImageReference, cfg.Image)
	require.Equal(t, req.User, cfg.User)
	require.Equal(t, req.WorkingDirectory, cfg.WorkingDir)
	require.Equal(t, req.Entrypoint, cfg.Entrypoint)
	require.Equal(t, req.Command, cfg.Cmd)
	require.Equal(t, req.Environment, cfg.Env)
	require.Equal(t, req.Labels, cfg.Labels)
	host := fake.containerCreate.HostConfig
	require.Equal(t, req.NetworkMode, string(host.NetworkMode))
	require.True(t, host.ReadonlyRootfs)
	require.True(t, host.Privileged == false)
	require.Equal(t, []string{"ALL"}, host.CapDrop)
	require.Empty(t, host.CapAdd)
	require.Equal(t, req.MemoryBytes, host.Resources.Memory)
	require.NotNil(t, host.PidsLimit)
	require.Equal(t, req.PidsLimit, *host.PidsLimit)
	require.Len(t, host.Mounts, len(req.Mounts))
	require.Equal(t, mount.TypeBind, host.Mounts[0].Type)
	require.True(t, host.Mounts[0].ReadOnly)
	require.Equal(t, mount.TypeTmpfs, host.Mounts[5].Type)
	require.Error(t, validateCraftRunViewDockerCreateRequest(func() CraftRunViewContainerCreateRequest {
		bad := req
		bad.ImageReference = "example/runtime:latest"
		return bad
	}()))
	fake.containerCreateErr = errors.New("connection reset after create")
	_, err = engine.CreateContainer(context.Background(), req)
	require.Error(t, err)
	require.Equal(t, 2, fake.containerCreateCalls, "one API call per invocation; adapter never retries an ambiguous create")
}

func TestCraftRunViewDockerEngineInspectProjectsActualStateAndDigest(t *testing.T) {
	fake := validRunViewDockerInspectFake()
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	facts, err := engine.InspectContainer(context.Background(), "rv-container-abc")
	require.NoError(t, err)
	require.NotNil(t, facts)
	require.Equal(t, "actual-container-id", facts.ID)
	require.Equal(t, "rv-container-abc", facts.Name)
	require.Equal(t, "running", facts.State)
	require.Equal(t, "image-id", facts.ImageID)
	require.Equal(t, "example/runtime@sha256:"+strings.Repeat("a", 64), facts.ImageReference)
	require.Equal(t, "sha256:"+strings.Repeat("a", 64), facts.ImageDigest)
	fake.image.ID = "different-image-id"
	facts, err = engine.InspectContainer(context.Background(), "rv-container-abc")
	require.ErrorContains(t, err, "actual image ID")
	require.Nil(t, facts)
	fake.image.ID = "image-id"
	facts, err = engine.InspectContainer(context.Background(), "rv-container-abc")
	require.NoError(t, err)
	require.Equal(t, "10001:10001", facts.User)
	require.Equal(t, int64(1<<30), facts.MemoryBytes)
	require.Equal(t, int64(256), facts.PidsLimit)
	require.True(t, facts.ReadonlyRootfs)
	require.True(t, facts.NoNewPrivileges)
	require.Len(t, facts.Mounts, 6)
	require.Len(t, facts.Networks, 1)
	require.True(t, facts.Networks[0].Internal)
	require.Empty(t, facts.PublishedPorts)
	fake.image.RepoDigests = nil
	facts, err = engine.InspectContainer(context.Background(), "rv-container-abc")
	require.NoError(t, err)
	require.Empty(t, facts.ImageDigest, "missing repository digest is observed as unpinned and is rejected by provider validation")
	fake.containerErr = cerrdefs.ErrNotFound
	facts, err = engine.InspectContainer(context.Background(), "rv-container-abc")
	require.NoError(t, err)
	require.Nil(t, facts, "only definitive 404 is absence")
	fake.containerErr = context.DeadlineExceeded
	facts, err = engine.InspectContainer(context.Background(), "rv-container-abc")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, facts)
}

func TestCraftRunViewDockerEngineRejectsNonPrivateNamespaces(t *testing.T) {
	tests := []struct {
		name string
		set  func(*dockcontainer.HostConfig)
	}{
		{name: "host PID", set: func(h *dockcontainer.HostConfig) { h.PidMode = "host" }},
		{name: "container PID", set: func(h *dockcontainer.HostConfig) { h.PidMode = "container:another-container" }},
		{name: "unknown PID mode", set: func(h *dockcontainer.HostConfig) { h.PidMode = "future-pid-mode" }},
		{name: "host IPC", set: func(h *dockcontainer.HostConfig) { h.IpcMode = dockcontainer.IPCModeHost }},
		{name: "container IPC", set: func(h *dockcontainer.HostConfig) { h.IpcMode = "container:another-container" }},
		{name: "shareable IPC", set: func(h *dockcontainer.HostConfig) { h.IpcMode = dockcontainer.IPCModeShareable }},
		{name: "unknown IPC mode", set: func(h *dockcontainer.HostConfig) { h.IpcMode = "future-ipc-mode" }},
		{name: "host UTS", set: func(h *dockcontainer.HostConfig) { h.UTSMode = "host" }},
		{name: "unknown UTS mode", set: func(h *dockcontainer.HostConfig) { h.UTSMode = "future-uts-mode" }},
		{name: "host user namespace", set: func(h *dockcontainer.HostConfig) { h.UsernsMode = "host" }},
		{name: "unknown user namespace", set: func(h *dockcontainer.HostConfig) { h.UsernsMode = "future-userns-mode" }},
		{name: "host cgroup namespace", set: func(h *dockcontainer.HostConfig) { h.CgroupnsMode = "host" }},
		{name: "unknown cgroup namespace", set: func(h *dockcontainer.HostConfig) { h.CgroupnsMode = "future-cgroupns-mode" }},
		{name: "container cgroup namespace", set: func(h *dockcontainer.HostConfig) { h.CgroupnsMode = "container:another-container" }},
	}
	for _, mode := range []string{"", "private"} {
		t.Run("allowed private IPC mode "+mode, func(t *testing.T) {
			fake := validRunViewDockerInspectFake()
			fake.container.Container.HostConfig.IpcMode = dockcontainer.IpcMode(mode)
			engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
			facts, err := engine.InspectContainer(context.Background(), "rv-container-abc")
			require.NoError(t, err)
			require.NotNil(t, facts)
		})
	}
	t.Run("allowed explicit private cgroup namespace", func(t *testing.T) {
		fake := validRunViewDockerInspectFake()
		fake.container.Container.HostConfig.CgroupnsMode = dockcontainer.CgroupnsModePrivate
		engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
		facts, err := engine.InspectContainer(context.Background(), "rv-container-abc")
		require.NoError(t, err)
		require.NotNil(t, facts)
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := validRunViewDockerInspectFake()
			tt.set(fake.container.Container.HostConfig)
			engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
			facts, err := engine.InspectContainer(context.Background(), "rv-container-abc")
			require.ErrorContains(t, err, "unsupported host access or process settings")
			require.Nil(t, facts)
		})
	}
}

func TestCraftRunViewDockerEngineStartIsBoundedSingleCall(t *testing.T) {
	fake := &runViewDockerFake{containerStartErr: errors.New("connection lost after start")}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	err := engine.StartContainer(context.Background(), "container-id")
	require.Error(t, err)
	require.ErrorContains(t, err, "outcome unknown")
	require.Equal(t, 1, fake.containerStartCalls, "uncertain start is never retried")
}

func TestCraftRunViewDockerEngineAcceptsDockerNormalizedDefaultsOnly(t *testing.T) {
	fake := validRunViewDockerInspectFake()
	container := fake.container.Container
	container.HostConfig.Runtime = "runc"
	container.HostConfig.RestartPolicy.Name = "no"
	require.False(t, hasUnsafeRunViewContainerSettings(container), "Docker reports its normal default runtime and restart policy explicitly")
	container.HostConfig.Runtime = "kata"
	require.True(t, hasUnsafeRunViewContainerSettings(container), "unrequested alternate runtime must be refused")
	container.HostConfig.Runtime = "runc"
	container.HostConfig.RestartPolicy.Name = "always"
	require.True(t, hasUnsafeRunViewContainerSettings(container), "automatic restart is outside the generation lifecycle")
}

func TestCraftRunViewDockerEngineProbeHashesActualRuntimeAndRequiresSuccess(t *testing.T) {
	version := "1.18.4\n"
	binary := strings.Repeat("b", 64) + "  /usr/local/bin/opencode\n"
	config := strings.Repeat("c", 64) + "  /etc/craft/runtime-config.json\n"
	fake := &runViewDockerFake{execStream: stdcopyFrame(1, []byte(version+binary+config)), execInspect: client.ExecInspectResult{ID: "probe-id", ContainerID: "container-id", ExitCode: 0}}
	engine := newCraftRunViewDockerEngineForAPI(fake, time.Second)
	probe, err := engine.ProbeRuntime(context.Background(), "container-id")
	require.NoError(t, err)
	require.Equal(t, "1.18.4", probe.OpenCodeVersion)
	require.Equal(t, strings.Repeat("b", 64), probe.BinarySHA256)
	require.Equal(t, strings.Repeat("c", 64), probe.RuntimeConfigSHA256)
	require.Equal(t, 1, fake.execCreateCalls)
	require.Equal(t, 1, fake.execAttachCalls)
	require.False(t, fake.execOptions.AttachStdin)
	require.True(t, fake.execOptions.AttachStdout)
	require.Equal(t, "10001:10001", fake.execOptions.User)
	fake.execInspect.ExitCode = 1
	_, err = engine.ProbeRuntime(context.Background(), "container-id")
	require.Error(t, err)
	require.Equal(t, 2, fake.execCreateCalls)
}

// CRAFT_RUNVIEW_R5_DOCKER_TEST=1 runs the real provider against the local
// Linux/arm64 image and removes its generation container and network.
func TestCraftRunViewDockerEngineRealPinnedProviderIsolation(t *testing.T) {
	if os.Getenv("CRAFT_RUNVIEW_R5_DOCKER_TEST") != "1" {
		t.Skip("set CRAFT_RUNVIEW_R5_DOCKER_TEST=1 for disposable real Docker proof")
	}
	endpoint := os.Getenv("DOCKER_HOST")
	if endpoint == "" {
		t.Skip("DOCKER_HOST must select the intended Docker daemon explicitly")
	}
	engine, err := NewCraftRunViewDockerEngine(endpoint)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	localImage, err := engine.api.ImageInspect(ctx, "weknora-craft-runtime:1.18.4")
	require.NoError(t, err)
	require.Equal(t, "linux", localImage.Os)
	require.Equal(t, "arm64", localImage.Architecture)
	require.NotEmpty(t, localImage.RepoDigests, "the integration proof must use a Docker repo digest, not a mutable tag or image ID")
	imageReference := localImage.RepoDigests[0]
	_, imageDigest, ok := strings.Cut(imageReference, "@")
	require.True(t, ok)
	require.True(t, validSHA256Digest(imageDigest))
	lockBytes, err := os.ReadFile(filepath.Join("..", "..", "docker", "craft", "opencode.lock.json"))
	require.NoError(t, err)
	var lock struct {
		BinarySHA256 string `json:"binary_sha256"`
		Version      string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(lockBytes, &lock))
	configBytes, err := os.ReadFile(filepath.Join("..", "..", "docker", "craft", "runtime-config.json"))
	require.NoError(t, err)
	configSum := sha256.Sum256(configBytes)
	root := filepath.Join(t.TempDir(), "runview-r5")
	providerConfig := CraftRunViewContainerProviderConfig{SandboxRoot: root, ImageReference: imageReference, ImageDigest: imageDigest, OpenCodeBinarySHA256: lock.BinarySHA256, RuntimeConfigSHA256: hex.EncodeToString(configSum[:]), OpenCodeVersion: lock.Version, ProjectID: "global"}
	provider, err := newCraftRunViewContainerProvider(providerConfig, engine, func(_, _, _ string) (craftRunViewSessionAPI, error) { return &r5NoopSessionAPI{}, nil })
	require.NoError(t, err)
	generation := fmt.Sprintf("r5-live-%x", time.Now().UnixNano())
	spec := craftRunViewSpecForGeneration(generation)
	networkName := "rv-network-" + strings.TrimPrefix(spec.ContainerID, "rv-container-")
	t.Cleanup(func() {
		cleanup, ok := engine.api.(interface {
			ContainerRemove(context.Context, string, client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
			NetworkRemove(context.Context, string, client.NetworkRemoveOptions) (client.NetworkRemoveResult, error)
		})
		require.True(t, ok, "real Docker client must expose resource cleanup")
		_, containerRemoveErr := cleanup.ContainerRemove(context.Background(), spec.ContainerID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		require.True(t, containerRemoveErr == nil || cerrdefs.IsNotFound(containerRemoveErr), "container cleanup failed: %v", containerRemoveErr)
		_, containerInspectErr := engine.api.ContainerInspect(context.Background(), spec.ContainerID, client.ContainerInspectOptions{})
		require.True(t, cerrdefs.IsNotFound(containerInspectErr), "container should be absent after cleanup, got: %v", containerInspectErr)
		_, networkRemoveErr := cleanup.NetworkRemove(context.Background(), networkName, client.NetworkRemoveOptions{})
		require.True(t, networkRemoveErr == nil || cerrdefs.IsNotFound(networkRemoveErr), "network cleanup failed: %v", networkRemoveErr)
		_, networkInspectErr := engine.api.NetworkInspect(context.Background(), networkName, client.NetworkInspectOptions{})
		require.True(t, cerrdefs.IsNotFound(networkInspectErr), "network should be absent after cleanup, got: %v", networkInspectErr)
		_ = engine.Close()
	})
	network, found, err := provider.ObserveNetwork(ctx, spec)
	require.NoError(t, err)
	require.False(t, found, "the fresh generation network must be absent before its one claimed create")
	network, err = provider.CreateGenerationNetwork(ctx, spec)
	require.NoError(t, err)
	require.NotEmpty(t, network.ID)
	observedNetwork, found, err := provider.ObserveNetwork(ctx, spec)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, network, observedNetwork, "network observe must return exact Docker identity")

	_, found, err = provider.ObserveContainer(ctx, spec, network)
	require.NoError(t, err)
	require.False(t, found, "the fresh generation container must be absent before its one claimed create")
	created, err := provider.CreateGenerationContainer(ctx, spec, network)
	require.NoError(t, err)
	require.Equal(t, "created", created.State)
	created, found, err = provider.ObserveContainerState(ctx, created)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "created", created.State)
	created, err = provider.StartGenerationContainer(ctx, created)
	require.NoError(t, err)
	require.Equal(t, "running", created.State)
	_, probeFound, observeProbeErr := provider.ObserveRuntimeProbe(ctx, created)
	require.Error(t, observeProbeErr)
	require.False(t, probeFound, "read-only Docker observation cannot prove prior probe stdout hashes")
	require.True(t, created.Runtime.IdentityVerified)
	require.True(t, created.Runtime.DedicatedForRun)
	require.True(t, created.Runtime.DirectoryCanonical)
	facts, err := engine.InspectContainer(ctx, created.DockerID)
	require.NoError(t, err)
	require.NotNil(t, facts)
	require.Equal(t, spec.ContainerID, facts.Name)
	require.Equal(t, imageDigest, facts.ImageDigest)
	require.Equal(t, craftRunViewContainerUser, facts.User)
	require.Equal(t, spec.Directory, facts.WorkingDirectory)
	require.True(t, facts.ReadonlyRootfs)
	require.True(t, facts.NoNewPrivileges)
	require.False(t, facts.Privileged)
	require.Empty(t, facts.PublishedPorts)
	require.Len(t, facts.Networks, 1)
	require.True(t, facts.Networks[0].Internal)
	expectedKnowledgeDestination := filepath.ToSlash(filepath.Join(spec.Directory, "knowledge"))
	knowledgeDestination := ""
	knowledgeSource := ""
	for _, m := range facts.Mounts {
		if m.Destination == expectedKnowledgeDestination {
			knowledgeDestination = m.Destination
			knowledgeSource = m.Source
			require.True(t, m.ReadOnly)
		}
	}
	require.NotEmpty(t, knowledgeDestination, "the inspected container must mount its generation knowledge directory at the expected destination")
	require.NotEmpty(t, knowledgeSource)
	layout := generationLayoutPaths(root, spec)
	challengeName := "r5-write-challenge"
	challenge := filepath.Join(layout.knowledge, challengeName)
	require.NoError(t, os.WriteFile(challenge, []byte("original"), 0o666))
	require.NoError(t, os.Chmod(challenge, 0o666))
	containerChallenge := filepath.ToSlash(filepath.Join(knowledgeDestination, challengeName))
	readExec, err := engine.api.ExecCreate(ctx, facts.ID, client.ExecCreateOptions{User: "0:0", WorkingDir: "/", Cmd: []string{"/bin/sh", "-c", "cat -- " + shellQuoteRunView(containerChallenge)}, AttachStdout: true, AttachStderr: true, TTY: false})
	require.NoError(t, err)
	readAttached, err := engine.api.ExecAttach(ctx, readExec.ID, client.ExecAttachOptions{})
	require.NoError(t, err)
	var readOutput bytes.Buffer
	_, streamErr := stdcopy.StdCopy(&readOutput, &readOutput, readAttached.Reader)
	readAttached.Close()
	require.NoError(t, streamErr)
	readFinished, err := engine.api.ExecInspect(ctx, readExec.ID, client.ExecInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, readExec.ID, readFinished.ID)
	require.Equal(t, facts.ID, readFinished.ContainerID)
	require.Zero(t, readFinished.ExitCode, "challenge must exist and be readable through the inspected container destination")
	require.Equal(t, "original", readOutput.String())
	writeExec, err := engine.api.ExecCreate(ctx, facts.ID, client.ExecCreateOptions{User: "0:0", WorkingDir: "/", Cmd: []string{"/bin/sh", "-c", "printf changed >> " + shellQuoteRunView(containerChallenge)}, AttachStdout: true, AttachStderr: true, TTY: false})
	require.NoError(t, err)
	writeAttached, err := engine.api.ExecAttach(ctx, writeExec.ID, client.ExecAttachOptions{})
	require.NoError(t, err)
	var writeOutput bytes.Buffer
	_, streamErr = stdcopy.StdCopy(&writeOutput, &writeOutput, writeAttached.Reader)
	writeAttached.Close()
	require.NoError(t, streamErr)
	writeFinished, err := engine.api.ExecInspect(ctx, writeExec.ID, client.ExecInspectOptions{})
	require.NoError(t, err)
	require.Equal(t, writeExec.ID, writeFinished.ID)
	require.Equal(t, facts.ID, writeFinished.ContainerID)
	require.NotZero(t, writeFinished.ExitCode)
	require.Contains(t, strings.ToLower(writeOutput.String()), "read-only file system", "the write must fail specifically because the mounted knowledge destination is read-only")
	content, err := os.ReadFile(challenge)
	require.NoError(t, err)
	require.Equal(t, "original", string(content))
	require.NoError(t, os.Remove(challenge))
	actualProbe, err := provider.SendRuntimeProbe(ctx, created)
	require.NoError(t, err)
	require.Equal(t, facts.ID, actualProbe.ContainerID)
	require.NotEmpty(t, actualProbe.ExecID)
	require.Equal(t, lock.Version, actualProbe.OpenCodeVersion)
	require.Equal(t, lock.BinarySHA256, actualProbe.BinarySHA256)
	require.Equal(t, providerConfig.RuntimeConfigSHA256, actualProbe.RuntimeConfigSHA256)
	t.Logf("real Docker identity: imageID=%s imageReference=%s imageDigest=%s containerID=%s networkID=%s OpenCodeVersion=%s binarySHA256=%s configSHA256=%s knowledgeWriteDenied=true", facts.ImageID, facts.ImageReference, facts.ImageDigest, facts.ID, facts.Networks[0].ID, actualProbe.OpenCodeVersion, actualProbe.BinarySHA256, actualProbe.RuntimeConfigSHA256)
}

type r5NoopSessionAPI struct{}

func (*r5NoopSessionAPI) ListSessions(context.Context) ([]opencode.SessionInfo, error) {
	return nil, nil
}
func (*r5NoopSessionAPI) GetSession(context.Context, string) (opencode.SessionInfo, error) {
	return opencode.SessionInfo{}, errors.New("unused in engine proof")
}
func (*r5NoopSessionAPI) CreateSession(context.Context) (string, error) {
	return "", errors.New("unused in engine proof")
}
func shellQuoteRunView(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func runViewDockerCreateRequest() CraftRunViewContainerCreateRequest {
	digest := "sha256:" + strings.Repeat("a", 64)
	return CraftRunViewContainerCreateRequest{
		Name: "rv-container-abc", ImageReference: "example/runtime@" + digest, WorkingDirectory: "/workspace/rv-abc", Directory: "/workspace/rv-abc", User: "10001:10001",
		Entrypoint: []string{"/usr/local/bin/opencode"}, Command: []string{"serve", "--hostname", "0.0.0.0", "--port", "4096"},
		Environment: []string{"HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config", "XDG_DATA_HOME=/home/craft/.local/share", "XDG_STATE_HOME=/home/craft/.local/share/opencode", "XDG_CACHE_HOME=/home/craft/.local/share/opencode/cache", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Labels:      map[string]string{craftRunViewLabelGeneration: strings.Repeat("a", 64), craftRunViewLabelRuntime: "rv-runtime-abc", craftRunViewLabelContainer: "rv-container-abc", craftRunViewLabelImage: digest},
		Mounts:      []CraftRunViewMount{{Type: "bind", Source: "/tmp/rv/inputs", Destination: "/workspace/rv-abc/inputs", ReadOnly: true}, {Type: "bind", Source: "/tmp/rv/knowledge", Destination: "/workspace/rv-abc/knowledge", ReadOnly: true}, {Type: "bind", Source: "/tmp/rv/output", Destination: "/workspace/rv-abc/output"}, {Type: "bind", Source: "/tmp/rv/home-data", Destination: "/home/craft/.local/share/opencode"}, {Type: "bind", Source: "/tmp/rv/home-config", Destination: "/home/craft/.config/opencode"}, {Type: "tmpfs", Destination: "/tmp", Options: []string{"mode=1777", "size=268435456"}}},
		NetworkMode: "rv-network-abc", PublishedPorts: []string{}, Privileged: false, ReadonlyRootfs: true, NoNewPrivileges: true, CapDrop: []string{"ALL"}, CapAdd: []string{}, MemoryBytes: 1 << 30, PidsLimit: 256,
	}
}

func validRunViewDockerInspectFake() *runViewDockerFake {
	digest := "sha256:" + strings.Repeat("a", 64)
	pidLimit := int64(256)
	return &runViewDockerFake{
		container: client.ContainerInspectResult{Container: dockcontainer.InspectResponse{ID: "actual-container-id", Name: "/rv-container-abc", Image: "image-id", State: &dockcontainer.State{Status: "running", Running: true}, Config: &dockcontainer.Config{Image: "example/runtime@" + digest, User: "10001:10001", WorkingDir: "/workspace/rv-abc", Entrypoint: []string{"/usr/local/bin/opencode"}, Cmd: []string{"serve", "--hostname", "0.0.0.0", "--port", "4096"}, Env: runViewDockerCreateRequest().Environment, Labels: runViewDockerCreateRequest().Labels}, HostConfig: &dockcontainer.HostConfig{NetworkMode: "rv-network-abc", ReadonlyRootfs: true, SecurityOpt: []string{"no-new-privileges:true"}, CapDrop: []string{"ALL"}, Resources: dockcontainer.Resources{Memory: 1 << 30, PidsLimit: &pidLimit}, Tmpfs: map[string]string{"/tmp": "mode=1777,size=268435456"}}, Mounts: []dockcontainer.MountPoint{{Type: mount.TypeBind, Source: "/tmp/rv/inputs", Destination: "/workspace/rv-abc/inputs", RW: false}, {Type: mount.TypeBind, Source: "/tmp/rv/knowledge", Destination: "/workspace/rv-abc/knowledge", RW: false}, {Type: mount.TypeBind, Source: "/tmp/rv/output", Destination: "/workspace/rv-abc/output", RW: true}, {Type: mount.TypeBind, Source: "/tmp/rv/home-data", Destination: "/home/craft/.local/share/opencode", RW: true}, {Type: mount.TypeBind, Source: "/tmp/rv/home-config", Destination: "/home/craft/.config/opencode", RW: true}, {Type: mount.TypeTmpfs, Destination: "/tmp"}}, NetworkSettings: &dockcontainer.NetworkSettings{Ports: network.PortMap{}, Networks: map[string]*network.EndpointSettings{"rv-network-abc": {NetworkID: "network-id", IPAddress: netip.MustParseAddr("172.20.0.2")}}}}},
		image:     client.ImageInspectResult{InspectResponse: dockimage.InspectResponse{ID: "image-id", RepoDigests: []string{"example/runtime@" + digest}}},
		network:   client.NetworkInspectResult{Network: network.Inspect{Network: network.Network{ID: "network-id", Name: "rv-network-abc", Driver: "bridge", Internal: true, Labels: map[string]string{"generation": strings.Repeat("a", 64)}}}},
	}
}

func stdcopyFrame(stream byte, payload []byte) []byte {
	var out bytes.Buffer
	_ = out.WriteByte(stream)
	_, _ = out.Write([]byte{0, 0, 0})
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(payload)))
	_, _ = out.Write(size[:])
	_, _ = out.Write(payload)
	return out.Bytes()
}
