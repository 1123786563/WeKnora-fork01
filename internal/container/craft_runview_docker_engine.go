package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	dockcontainer "github.com/moby/moby/api/types/container"
	dockmount "github.com/moby/moby/api/types/mount"
	docknetwork "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const craftRunViewDockerDefaultRPCTimeout = 15 * time.Second

// craftRunViewDockerAPI is the smallest pinned-SDK surface this engine uses.
type craftRunViewDockerAPI interface {
	NetworkInspect(context.Context, string, client.NetworkInspectOptions) (client.NetworkInspectResult, error)
	NetworkCreate(context.Context, string, client.NetworkCreateOptions) (client.NetworkCreateResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerCreate(context.Context, client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(context.Context, string, client.ContainerStartOptions) (client.ContainerStartResult, error)
	ImageInspect(context.Context, string, ...client.ImageInspectOption) (client.ImageInspectResult, error)
	ExecCreate(context.Context, string, client.ExecCreateOptions) (client.ExecCreateResult, error)
	ExecAttach(context.Context, string, client.ExecAttachOptions) (client.ExecAttachResult, error)
	ExecInspect(context.Context, string, client.ExecInspectOptions) (client.ExecInspectResult, error)
}

// CraftRunViewDockerEngine is a Docker Engine SDK adapter for the RunView
// provider. It has no production wiring in this change; callers must own and
// close its client explicitly.
type CraftRunViewDockerEngine struct {
	api     craftRunViewDockerAPI
	close   func() error
	timeout time.Duration
}

// NewCraftRunViewDockerEngine constructs a private SDK client for an
// operator-owned Docker endpoint. An empty endpoint is refused so the
// adapter cannot silently select a daemon from ambient process context.
func NewCraftRunViewDockerEngine(endpoint string) (*CraftRunViewDockerEngine, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("RunView Docker endpoint is required")
	}
	api, err := client.New(client.WithHost(endpoint))
	if err != nil {
		return nil, fmt.Errorf("create RunView Docker client: %w", err)
	}
	return &CraftRunViewDockerEngine{api: api, close: api.Close, timeout: craftRunViewDockerDefaultRPCTimeout}, nil
}

func newCraftRunViewDockerEngineForAPI(api craftRunViewDockerAPI, timeout time.Duration) *CraftRunViewDockerEngine {
	if timeout <= 0 {
		timeout = craftRunViewDockerDefaultRPCTimeout
	}
	return &CraftRunViewDockerEngine{api: api, timeout: timeout}
}

// Close releases the SDK transport owned by the production constructor.
func (e *CraftRunViewDockerEngine) Close() error {
	if e == nil || e.close == nil {
		return nil
	}
	return e.close()
}

// The Exec surface below lets the engine serve as the normal-exec provider
// for server-owned command faces (F08 web build dispatch): the moby client
// this engine already owns is the only Docker dial the RunView deployment
// has, so dispatch and the coordinator's live observations can never
// disagree about which daemon they talked to. Plain pass-throughs — the
// exec client owns its own RPC timeouts.
func (e *CraftRunViewDockerEngine) ExecCreate(ctx context.Context, containerID string, opts client.ExecCreateOptions) (client.ExecCreateResult, error) {
	return e.api.ExecCreate(ctx, containerID, opts)
}

func (e *CraftRunViewDockerEngine) ExecAttach(ctx context.Context, execID string, opts client.ExecAttachOptions) (client.ExecAttachResult, error) {
	return e.api.ExecAttach(ctx, execID, opts)
}

func (e *CraftRunViewDockerEngine) ExecInspect(ctx context.Context, execID string, opts client.ExecInspectOptions) (client.ExecInspectResult, error) {
	return e.api.ExecInspect(ctx, execID, opts)
}

func (e *CraftRunViewDockerEngine) rpcContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if e == nil || e.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, e.timeout)
}

func validateRunViewDockerContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("RunView Docker context is required")
	}
	return ctx.Err()
}

// ObserveNetwork performs an Engine inspect only. Absence is returned as
// (zero, false, nil); it never attempts network creation.
func (e *CraftRunViewDockerEngine) ObserveNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, bool, error) {
	if e == nil || e.api == nil || !validRunViewNetworkSpec(spec) {
		return CraftRunViewContainerNetwork{}, false, errors.New("invalid RunView private network observation")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return CraftRunViewContainerNetwork{}, false, err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	actual, err := e.api.NetworkInspect(rpcCtx, spec.Name, client.NetworkInspectOptions{})
	if errdefs.IsNotFound(err) {
		return CraftRunViewContainerNetwork{}, false, nil
	}
	if err != nil {
		return CraftRunViewContainerNetwork{}, false, fmt.Errorf("inspect RunView network: %w", err)
	}
	got, err := verifyRunViewDockerNetwork(spec, actual.Network, "")
	if err != nil {
		return CraftRunViewContainerNetwork{}, false, err
	}
	if err := e.verifyRunViewDockerNetworkAttachments(rpcCtx, spec, actual.Network); err != nil {
		return CraftRunViewContainerNetwork{}, false, err
	}
	return got, true, nil
}

// CreateGenerationNetwork sends exactly one NetworkCreate request and then
// inspects the returned Docker ID for its receipt. It never retries on an
// ambiguous create or inspection result.
func (e *CraftRunViewDockerEngine) CreateGenerationNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	if e == nil || e.api == nil || !validRunViewNetworkSpec(spec) {
		return CraftRunViewContainerNetwork{}, errors.New("invalid RunView private network create")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return CraftRunViewContainerNetwork{}, err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	created, err := e.api.NetworkCreate(rpcCtx, spec.Name, client.NetworkCreateOptions{Driver: spec.Driver, Scope: "local", Internal: true, Labels: cloneRunViewLabels(spec.Labels)})
	if err != nil {
		return CraftRunViewContainerNetwork{}, fmt.Errorf("RunView network create outcome unknown: %w", err)
	}
	if created.ID == "" || created.ID == "\x00" {
		return CraftRunViewContainerNetwork{}, errors.New("Docker returned an empty RunView network ID")
	}
	actual, err := e.api.NetworkInspect(rpcCtx, created.ID, client.NetworkInspectOptions{})
	if err != nil {
		return CraftRunViewContainerNetwork{}, fmt.Errorf("inspect created RunView network receipt: %w", err)
	}
	got, err := verifyRunViewDockerNetwork(spec, actual.Network, created.ID)
	if err != nil {
		return CraftRunViewContainerNetwork{}, err
	}
	if err := e.verifyRunViewDockerNetworkAttachments(rpcCtx, spec, actual.Network); err != nil {
		return CraftRunViewContainerNetwork{}, err
	}
	return got, nil
}

func verifyRunViewDockerNetwork(spec CraftRunViewContainerNetworkSpec, actual docknetwork.Inspect, expectedID string) (CraftRunViewContainerNetwork, error) {
	got := projectRunViewNetwork(actual)
	if got.ID == "" || got.ID == "\x00" || (expectedID != "" && got.ID != expectedID) || got.Name != spec.Name || got.Driver != spec.Driver || !got.Internal || !equalRunViewLabels(got.Labels, spec.Labels) {
		return CraftRunViewContainerNetwork{}, errors.New("Docker network does not match private generation-scoped specification")
	}
	if actual.Scope != "local" || actual.Attachable || actual.Ingress || actual.ConfigOnly || len(actual.Options) != 0 {
		return CraftRunViewContainerNetwork{}, errors.New("Docker network has unsupported scope or attachment options")
	}
	return got, nil
}

// verifyRunViewDockerNetworkAttachments authenticates each attachment from a
// Docker inspect keyed by the attached Docker container ID. Endpoint names are
// descriptive only and cannot authorize an otherwise foreign ID.
func (e *CraftRunViewDockerEngine) verifyRunViewDockerNetworkAttachments(ctx context.Context, spec CraftRunViewContainerNetworkSpec, actual docknetwork.Inspect) error {
	if len(actual.Containers) > 1 {
		return errors.New("RunView private network has multiple attached containers")
	}
	containerName := spec.Labels[craftRunViewLabelContainer]
	for id, endpoint := range actual.Containers {
		if id == "" || endpoint.Name != containerName {
			return errors.New("RunView private network has an unexpected attached container")
		}
		inspected, err := e.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if err != nil {
			return fmt.Errorf("inspect RunView network attachment %q: %w", id, err)
		}
		container := inspected.Container
		if container.ID != id || strings.TrimPrefix(container.Name, "/") != containerName || container.Config == nil || !equalRunViewLabels(container.Config.Labels, spec.Labels) {
			return errors.New("RunView private network attachment is not the exact generation container")
		}
	}
	return nil
}

// EnsurePrivateNetwork retains the legacy combined API for legacy callers.
// Admitted resolution must use ObserveNetwork and CreateGenerationNetwork as
// distinct claimed operations.
func (e *CraftRunViewDockerEngine) EnsurePrivateNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	got, found, err := e.ObserveNetwork(ctx, spec)
	if err != nil || found {
		return got, err
	}
	return e.CreateGenerationNetwork(ctx, spec)
}

func projectRunViewNetwork(n docknetwork.Inspect) CraftRunViewContainerNetwork {
	return CraftRunViewContainerNetwork{ID: n.ID, Name: n.Name, Driver: n.Driver, Internal: n.Internal, Labels: cloneRunViewLabels(n.Labels)}
}

// InspectContainer returns nil only on a definitive Docker not-found. It
// derives every field from Engine inspections and never echoes requested
// configuration as observed state.
func (e *CraftRunViewDockerEngine) InspectContainer(ctx context.Context, name string) (*CraftRunViewEngineContainer, error) {
	if e == nil || e.api == nil || strings.TrimSpace(name) == "" {
		return nil, errors.New("invalid RunView container inspect request")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return nil, err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	inspected, err := e.api.ContainerInspect(rpcCtx, name, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect RunView container: %w", err)
	}
	c := inspected.Container
	if c.ID == "" || c.Config == nil || c.HostConfig == nil || c.State == nil || c.NetworkSettings == nil {
		return nil, errors.New("Docker returned incomplete RunView container inspection")
	}
	image, err := e.api.ImageInspect(rpcCtx, c.Image)
	if err != nil {
		return nil, fmt.Errorf("inspect actual RunView image: %w", err)
	}
	if strings.TrimSpace(image.ID) == "" || image.ID != c.Image {
		return nil, errors.New("Docker image inspection identity differs from the container's actual image ID")
	}
	actualDigest := observedRunViewImageDigest(c.Config.Image, image.RepoDigests)
	if _, err := parseRunViewDigest(c.Config.Image); err != nil {
		return nil, fmt.Errorf("container image reference is not digest pinned: %w", err)
	}

	facts := &CraftRunViewEngineContainer{
		ID: c.ID, Name: strings.TrimPrefix(c.Name, "/"), State: string(c.State.Status), ImageID: image.ID,
		ImageReference: c.Config.Image, ImageDigest: actualDigest, User: c.Config.User, WorkingDirectory: c.Config.WorkingDir,
		Entrypoint: append([]string(nil), c.Config.Entrypoint...), Command: append([]string(nil), c.Config.Cmd...), Environment: append([]string(nil), c.Config.Env...), Labels: cloneRunViewLabels(c.Config.Labels),
		NetworkMode: string(c.HostConfig.NetworkMode), Privileged: c.HostConfig.Privileged, ReadonlyRootfs: c.HostConfig.ReadonlyRootfs,
		NoNewPrivileges: containsRunViewSecurityOpt(c.HostConfig.SecurityOpt, "no-new-privileges:true"),
		CapDrop:         append([]string(nil), c.HostConfig.CapDrop...), CapAdd: append([]string(nil), c.HostConfig.CapAdd...),
		MemoryBytes: c.HostConfig.Resources.Memory, PidsLimit: 0,
		HostPID: c.HostConfig.PidMode.IsHost(), HostIPC: c.HostConfig.IpcMode.IsHost(), HostUTS: c.HostConfig.UTSMode.IsHost(),
		Devices: []string{}, PublishedPorts: []string{},
	}
	if c.HostConfig.Resources.PidsLimit != nil {
		facts.PidsLimit = *c.HostConfig.Resources.PidsLimit
	}
	if c.HostConfig.Resources.Devices != nil || len(c.HostConfig.Resources.DeviceRequests) > 0 {
		facts.Devices = append(facts.Devices, "device request")
	}
	for _, d := range c.HostConfig.Resources.Devices {
		facts.Devices = append(facts.Devices, d.PathOnHost)
	}
	for _, mountPoint := range c.Mounts {
		projected := CraftRunViewMount{Type: string(mountPoint.Type), Source: mountPoint.Source, Destination: mountPoint.Destination, ReadOnly: !mountPoint.RW}
		if projected.Type == "tmpfs" {
			projected.Options = observedRunViewTmpfsOptions(c.HostConfig, mountPoint.Destination)
		}
		facts.Mounts = append(facts.Mounts, projected)
	}
	for destination, options := range c.HostConfig.Tmpfs {
		if !hasRunViewMount(facts.Mounts, destination) {
			facts.Mounts = append(facts.Mounts, CraftRunViewMount{Type: "tmpfs", Destination: destination, Options: parseRunViewTmpfsOptions(options)})
		}
	}
	for port, bindings := range c.NetworkSettings.Ports {
		for _, binding := range bindings {
			ip := ""
			if binding.HostIP.IsValid() {
				ip = binding.HostIP.String()
			}
			facts.PublishedPorts = append(facts.PublishedPorts, fmt.Sprintf("%s:%s:%s", port, ip, binding.HostPort))
		}
	}
	sort.Strings(facts.PublishedPorts)
	for networkName, endpoint := range c.NetworkSettings.Networks {
		if endpoint == nil {
			return nil, fmt.Errorf("Docker returned empty RunView network endpoint %q", networkName)
		}
		networkInspect, inspectErr := e.api.NetworkInspect(rpcCtx, networkName, client.NetworkInspectOptions{})
		if inspectErr != nil {
			return nil, fmt.Errorf("inspect attached RunView network %q: %w", networkName, inspectErr)
		}
		if endpoint.NetworkID != "" && endpoint.NetworkID != networkInspect.Network.ID {
			return nil, errors.New("Docker network endpoint ID differs from inspected network")
		}
		ip := ""
		if endpoint.IPAddress.IsValid() {
			ip = endpoint.IPAddress.String()
		}
		facts.Networks = append(facts.Networks, CraftRunViewNetworkAttachment{Name: networkInspect.Network.Name, ID: networkInspect.Network.ID, Internal: networkInspect.Network.Internal, IP: ip})
	}
	sort.Slice(facts.Networks, func(i, j int) bool { return facts.Networks[i].Name < facts.Networks[j].Name })
	if hasUnsafeRunViewContainerSettings(c) {
		return nil, errors.New("Docker inspection contains unsupported host access or process settings")
	}
	return facts, nil
}

func observedRunViewTmpfsOptions(host *dockcontainer.HostConfig, destination string) []string {
	if host == nil {
		return nil
	}
	if configured, ok := host.Tmpfs[destination]; ok {
		return parseRunViewTmpfsOptions(configured)
	}
	for _, configured := range host.Mounts {
		if string(configured.Type) != "tmpfs" || configured.Target != destination || configured.TmpfsOptions == nil {
			continue
		}
		options := make([]string, 0, 2+len(configured.TmpfsOptions.Options))
		if configured.TmpfsOptions.Mode != 0 {
			options = append(options, fmt.Sprintf("mode=%o", configured.TmpfsOptions.Mode))
		}
		if configured.TmpfsOptions.SizeBytes != 0 {
			options = append(options, fmt.Sprintf("size=%d", configured.TmpfsOptions.SizeBytes))
		}
		for _, group := range configured.TmpfsOptions.Options {
			options = append(options, strings.Join(group, "="))
		}
		sort.Strings(options)
		return options
	}
	return nil
}

// CreateContainer applies the exact generation request in one SDK request.
// No retry is made after any create error because Docker may have accepted it.
func (e *CraftRunViewDockerEngine) CreateContainer(ctx context.Context, request CraftRunViewContainerCreateRequest) (string, error) {
	if e == nil || e.api == nil {
		return "", errors.New("RunView Docker engine unavailable")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return "", err
	}
	if err := validateCraftRunViewDockerCreateRequest(request); err != nil {
		return "", err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	config := &dockcontainer.Config{Image: request.ImageReference, User: request.User, WorkingDir: request.WorkingDirectory, Entrypoint: append([]string(nil), request.Entrypoint...), Cmd: append([]string(nil), request.Command...), Env: append([]string(nil), request.Environment...), Labels: cloneRunViewLabels(request.Labels)}
	pids := request.PidsLimit
	host := &dockcontainer.HostConfig{NetworkMode: dockcontainer.NetworkMode(request.NetworkMode), Privileged: request.Privileged, ReadonlyRootfs: request.ReadonlyRootfs, SecurityOpt: []string{"no-new-privileges:true"}, CapDrop: append([]string(nil), request.CapDrop...), CapAdd: append([]string(nil), request.CapAdd...), Resources: dockcontainer.Resources{Memory: request.MemoryBytes, PidsLimit: &pids}, Mounts: make([]dockmount.Mount, 0, len(request.Mounts))}
	for _, requested := range request.Mounts {
		m := dockmount.Mount{Target: requested.Destination, ReadOnly: requested.ReadOnly}
		switch requested.Type {
		case "bind":
			m.Type = dockmount.TypeBind
			m.Source = requested.Source
			m.BindOptions = &dockmount.BindOptions{Propagation: dockmount.PropagationRPrivate}
			if requested.ReadOnly {
				m.BindOptions.ReadOnlyForceRecursive = true
			}
		case "tmpfs":
			m.Type = dockmount.TypeTmpfs
			options := parseRunViewOptions(requested.Options)
			m.TmpfsOptions = &dockmount.TmpfsOptions{Mode: options.mode, SizeBytes: options.size}
		default:
			return "", errors.New("unsupported RunView mount type")
		}
		host.Mounts = append(host.Mounts, m)
	}
	created, err := e.api.ContainerCreate(rpcCtx, client.ContainerCreateOptions{Config: config, HostConfig: host, Name: request.Name})
	if err != nil {
		return "", fmt.Errorf("RunView container create outcome unknown: %w", err)
	}
	if created.ID == "" {
		return "", errors.New("Docker returned empty RunView container ID")
	}
	return created.ID, nil
}

// StartContainer is one bounded Engine start call; callers re-inspect exact
// state and must treat transport errors as unresolved.
func (e *CraftRunViewDockerEngine) StartContainer(ctx context.Context, id string) error {
	if e == nil || e.api == nil || strings.TrimSpace(id) == "" {
		return errors.New("invalid RunView container start")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	if _, err := e.api.ContainerStart(rpcCtx, id, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("RunView container start outcome unknown: %w", err)
	}
	return nil
}

// ProbeRuntime executes a read-only bounded command and hashes the actual
// binary/config bytes inside the running generation container.
func (e *CraftRunViewDockerEngine) ProbeRuntime(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	if e == nil || e.api == nil || strings.TrimSpace(id) == "" {
		return CraftRunViewRuntimeProbe{}, errors.New("invalid RunView runtime probe")
	}
	if err := validateRunViewDockerContext(ctx); err != nil {
		return CraftRunViewRuntimeProbe{}, err
	}
	rpcCtx, cancel := e.rpcContext(ctx)
	defer cancel()
	command := []string{"/bin/sh", "-c", "set -eu; opencode --version; sha256sum /usr/local/bin/opencode; sha256sum /etc/craft/runtime-config.json"}
	created, err := e.api.ExecCreate(rpcCtx, id, client.ExecCreateOptions{User: craftRunViewContainerUser, WorkingDir: "/", Cmd: command, AttachStdout: true, AttachStderr: true, AttachStdin: false, TTY: false})
	if err != nil {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("create RunView runtime probe: %w", err)
	}
	if created.ID == "" {
		return CraftRunViewRuntimeProbe{}, errors.New("Docker returned empty RunView probe exec ID")
	}
	attached, err := e.api.ExecAttach(rpcCtx, created.ID, client.ExecAttachOptions{TTY: false})
	if err != nil {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("RunView runtime probe outcome unknown: %w", err)
	}
	var closeOnce sync.Once
	closeStream := func() { closeOnce.Do(attached.Close) }
	defer closeStream()
	watcherDone := make(chan struct{})
	go func() {
		select {
		case <-rpcCtx.Done():
			closeStream()
		case <-watcherDone:
		}
	}()
	defer close(watcherDone)
	stdout := &runViewBoundedBuffer{limit: 4096}
	stderr := &runViewBoundedBuffer{limit: 4096}
	if _, err := stdcopy.StdCopy(stdout, stderr, attached.Reader); err != nil {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("read RunView runtime probe: %w", err)
	}
	if stderr.Len() != 0 {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("RunView runtime probe wrote stderr: %s", strings.TrimSpace(stderr.String()))
	}
	inspected, err := e.api.ExecInspect(rpcCtx, created.ID, client.ExecInspectOptions{})
	if err != nil {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("inspect RunView runtime probe: %w", err)
	}
	if inspected.ID != created.ID || inspected.ContainerID != id || inspected.Running || inspected.ExitCode != 0 {
		return CraftRunViewRuntimeProbe{}, errors.New("RunView runtime probe did not complete successfully on the same container")
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("RunView runtime probe returned %d lines, expected 3", len(lines))
	}
	version := strings.TrimSpace(lines[0])
	version = strings.TrimPrefix(version, "opencode ")
	version = strings.TrimPrefix(version, "OpenCode ")
	binary := strings.Fields(lines[1])
	config := strings.Fields(lines[2])
	if len(binary) != 2 || len(config) != 2 || binary[1] != "/usr/local/bin/opencode" || config[1] != "/etc/craft/runtime-config.json" || !validSHA256Hex(strings.ToLower(binary[0])) || !validSHA256Hex(strings.ToLower(config[0])) {
		return CraftRunViewRuntimeProbe{}, errors.New("RunView runtime probe returned malformed binary or config hashes")
	}
	return CraftRunViewRuntimeProbe{ContainerID: id, ExecID: created.ID, OpenCodeVersion: version, BinarySHA256: strings.ToLower(binary[0]), RuntimeConfigSHA256: strings.ToLower(config[0])}, nil
}

// ObserveRuntimeProbe is deliberately unresolved. Docker exposes no
// read-only operation that recovers the measured stdout for an old exec; an
// ExecInspect status alone cannot prove the binary/config hashes.
func (e *CraftRunViewDockerEngine) ObserveRuntimeProbe(ctx context.Context, id string) (CraftRunViewRuntimeProbe, bool, error) {
	if ctx == nil {
		return CraftRunViewRuntimeProbe{}, false, fmt.Errorf("%w: missing RunView probe observation context", ErrCraftRunViewRuntimeUnresolved)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewRuntimeProbe{}, false, err
	}
	if e == nil || e.api == nil || strings.TrimSpace(id) == "" {
		return CraftRunViewRuntimeProbe{}, false, errors.New("invalid RunView runtime probe observation")
	}
	return CraftRunViewRuntimeProbe{}, false, unresolvedCraftRunView("Docker cannot reconstruct runtime probe output from read-only inspection", nil)
}

// SendRuntimeProbe performs the bounded exec probe exactly once. The returned
// receipt includes the exact container and Docker exec IDs plus measured hashes.
func (e *CraftRunViewDockerEngine) SendRuntimeProbe(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	return e.ProbeRuntime(ctx, id)
}

func validRunViewNetworkSpec(s CraftRunViewContainerNetworkSpec) bool {
	if !strings.HasPrefix(s.Name, "rv-network-") || s.Driver != "bridge" || !s.Internal || len(s.Labels) != 4 {
		return false
	}
	if !validSHA256Hex(s.Labels[craftRunViewLabelGeneration]) || s.Labels[craftRunViewLabelRuntime] == "" || s.Labels[craftRunViewLabelContainer] == "" || !validSHA256Digest(s.Labels[craftRunViewLabelImage]) {
		return false
	}
	return s.Name == "rv-network-"+strings.TrimPrefix(s.Labels[craftRunViewLabelContainer], "rv-container-")
}

func validateCraftRunViewDockerCreateRequest(r CraftRunViewContainerCreateRequest) error {
	if r.Name == "" || !validSHA256Hex(r.Labels[craftRunViewLabelGeneration]) || len(r.Labels) != 4 || !strings.HasPrefix(r.Name, "rv-container-") || r.Name != r.Labels[craftRunViewLabelContainer] || r.NetworkMode != "rv-network-"+strings.TrimPrefix(r.Name, "rv-container-") || r.Labels[craftRunViewLabelRuntime] == "" {
		return errors.New("RunView container/network identity is invalid")
	}
	if _, err := parseRunViewDigest(r.ImageReference); err != nil {
		return err
	}
	if r.Labels[craftRunViewLabelImage] != "sha256:"+strings.TrimPrefix(r.ImageReference[strings.LastIndex(r.ImageReference, "@")+1:], "sha256:") {
		return errors.New("RunView image or generation labels are invalid")
	}
	if r.User != craftRunViewContainerUser || r.WorkingDirectory != r.Directory || !strings.HasPrefix(r.Directory, "/workspace/rv-") || !r.ReadonlyRootfs || !r.NoNewPrivileges || r.Privileged || r.MemoryBytes != craftRunViewMemoryBytes || r.PidsLimit != craftRunViewPidsLimit || !sameStrings(r.CapDrop, []string{"ALL"}) || len(r.CapAdd) != 0 || len(r.PublishedPorts) != 0 {
		return errors.New("RunView container isolation request differs from pinned profile")
	}
	if !sameStrings(r.Entrypoint, []string{"/usr/local/bin/opencode"}) || !sameStrings(r.Command, []string{"serve", "--hostname", "0.0.0.0", "--port", strconv.Itoa(craftRunViewOpenCodePort)}) || !sameStrings(r.Environment, []string{"HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config", "XDG_DATA_HOME=/home/craft/.local/share", "XDG_STATE_HOME=/home/craft/.local/share/opencode", "XDG_CACHE_HOME=/home/craft/.local/share/opencode/cache", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}) {
		return errors.New("RunView runtime command/environment differs from pinned profile")
	}
	want := map[string]struct {
		kind, base string
		readOnly   bool
	}{
		pathJoin(r.Directory, "inputs"): {"bind", "inputs", true}, pathJoin(r.Directory, "knowledge"): {"bind", "knowledge", true}, pathJoin(r.Directory, "output"): {"bind", "output", false},
		"/home/craft/.local/share/opencode": {"bind", "home-data", false}, "/home/craft/.config/opencode": {"bind", "home-config", false}, "/tmp": {"tmpfs", "", false},
	}
	if len(r.Mounts) != len(want) {
		return errors.New("RunView mount set is incomplete or contains extras")
	}
	root := ""
	for _, m := range r.Mounts {
		expected, ok := want[m.Destination]
		if !ok || m.Type != expected.kind || m.ReadOnly != expected.readOnly {
			return errors.New("RunView mount destination/type/access differs from pinned profile")
		}
		if m.Type == "bind" {
			if !filepath.IsAbs(m.Source) || filepath.Clean(m.Source) != m.Source || filepath.Base(m.Source) != expected.base {
				return errors.New("RunView bind source is not a direct generation child")
			}
			parent := filepath.Dir(m.Source)
			if root == "" {
				root = parent
			} else if root != parent {
				return errors.New("RunView bind sources do not share one generation root")
			}
		} else if m.Source != "" || !sameStrings(m.Options, []string{"mode=1777", fmt.Sprintf("size=%d", craftRunViewTmpfsBytes)}) {
			return errors.New("RunView tmpfs options differ from pinned profile")
		}
	}
	return nil
}

type runViewTmpfsConfig struct {
	mode os.FileMode
	size int64
}

func parseRunViewOptions(values []string) runViewTmpfsConfig {
	var out runViewTmpfsConfig
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		switch k {
		case "mode":
			n, _ := strconv.ParseUint(val, 8, 32)
			out.mode = os.FileMode(n)
		case "size":
			out.size, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	return out
}
func parseRunViewTmpfsOptions(value string) []string {
	parts := strings.Split(value, ",")
	sort.Strings(parts)
	return parts
}
func hasRunViewMount(mounts []CraftRunViewMount, destination string) bool {
	for _, m := range mounts {
		if m.Destination == destination {
			return true
		}
	}
	return false
}
func containsRunViewSecurityOpt(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func observedRunViewImageDigest(reference string, repoDigests []string) string {
	repo, _, ok := strings.Cut(reference, "@")
	if !ok {
		return ""
	}
	for _, candidate := range repoDigests {
		prefix := repo + "@"
		if strings.HasPrefix(candidate, prefix) {
			digest := strings.TrimPrefix(candidate, prefix)
			if validSHA256Digest(digest) {
				return digest
			}
		}
	}
	return ""
}
func parseRunViewDigest(reference string) (string, error) {
	repo, digest, ok := strings.Cut(reference, "@")
	if !ok || repo == "" || strings.ContainsAny(repo, " \t\r\n") || !validSHA256Digest(digest) {
		return "", errors.New("RunView image reference must use a verified sha256 digest")
	}
	return digest, nil
}
func cloneRunViewLabels(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}
func equalRunViewLabels(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
func hasUnsafeRunViewContainerSettings(c dockcontainer.InspectResponse) bool {
	h := c.HostConfig
	cfg := c.Config
	return cfg.AttachStdin || cfg.AttachStdout || cfg.AttachStderr || cfg.Tty || cfg.OpenStdin || cfg.StdinOnce || cfg.NetworkDisabled ||
		h.AutoRemove || h.PublishAllPorts || len(h.PortBindings) > 0 || len(h.Binds) > 0 || len(h.VolumesFrom) > 0 || len(h.Links) > 0 || len(h.ExtraHosts) > 0 || len(h.DNS) > 0 || len(h.DNSOptions) > 0 || len(h.DNSSearch) > 0 || len(h.Resources.DeviceRequests) > 0 || len(h.Resources.Devices) > 0 ||
		!isRunViewPrivatePIDMode(h.PidMode) || !isRunViewPrivateIPCMode(h.IpcMode) || h.UTSMode != "" || h.UsernsMode != "" || (h.CgroupnsMode != "" && h.CgroupnsMode != dockcontainer.CgroupnsModePrivate) ||
		(h.Runtime != "" && h.Runtime != "runc") || !sameStrings(h.SecurityOpt, []string{"no-new-privileges:true"}) ||
		(h.RestartPolicy.Name != "" && h.RestartPolicy.Name != "no") || h.RestartPolicy.MaximumRetryCount != 0
}

// An empty PID/IPC mode is Docker's default private namespace for this create
// profile. Explicit host, container-shared, special, and unknown modes are not
// accepted; the adapter cannot infer private isolation from IsHost alone.
func isRunViewPrivatePIDMode(mode dockcontainer.PidMode) bool {
	return mode == ""
}

func isRunViewPrivateIPCMode(mode dockcontainer.IpcMode) bool {
	return mode == "" || mode == dockcontainer.IPCModePrivate
}

type runViewBoundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *runViewBoundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("RunView runtime probe output exceeded limit")
	}
	return b.Buffer.Write(p)
}

func pathJoin(a, b string) string { return filepath.ToSlash(filepath.Join(a, b)) }

var _ CraftRunViewContainerEngine = (*CraftRunViewDockerEngine)(nil)
