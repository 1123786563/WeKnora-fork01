package container

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestCraftRunViewProviderCreatedLinuxContainerKnowledgeMount(t *testing.T) {
	if os.Getenv("CRAFT_RUNVIEW_LINUX_MOUNT_TEST") != "1" {
		t.Skip("set CRAFT_RUNVIEW_LINUX_MOUNT_TEST=1 to run the live provider-created Linux container proof")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI is unavailable")
	}
	engineOS, err := exec.Command("docker", "info", "--format", "{{.OSType}}").Output()
	require.NoError(t, err)
	require.Equal(t, "linux", strings.TrimSpace(string(engineOS)))

	imageReferenceBytes, err := exec.Command("docker", "image", "inspect", "--format", "{{index .RepoDigests 0}}", "weknora-craft-runtime:1.18.4").Output()
	require.NoError(t, err)
	imageReference := strings.TrimSpace(string(imageReferenceBytes))
	imageDigest := imageReference[strings.LastIndex(imageReference, "@")+1:]
	lockBytes, err := os.ReadFile(filepath.Join("..", "..", "docker", "craft", "opencode.lock.json"))
	require.NoError(t, err)
	var lock struct {
		BinarySHA256 string `json:"binary_sha256"`
	}
	require.NoError(t, json.Unmarshal(lockBytes, &lock))
	runtimeConfig, err := os.ReadFile(filepath.Join("..", "..", "docker", "craft", "runtime-config.json"))
	require.NoError(t, err)
	runtimeConfigDigest := sha256.Sum256(runtimeConfig)

	root := filepath.Join(t.TempDir(), "sandbox")
	engine := &h2DockerProviderEngine{networks: make(map[string]CraftRunViewContainerNetwork)}
	t.Cleanup(func() {
		engine.cleanup()
		_ = filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && entry != nil && entry.Type()&os.ModeSymlink == 0 {
				mode := os.FileMode(0o600)
				if entry.IsDir() {
					mode = 0o700
				}
				_ = os.Chmod(name, mode)
			}
			return nil
		})
	})
	config := CraftRunViewContainerProviderConfig{
		SandboxRoot: root, ImageReference: imageReference, ImageDigest: imageDigest,
		OpenCodeBinarySHA256: lock.BinarySHA256, RuntimeConfigSHA256: hex.EncodeToString(runtimeConfigDigest[:]),
		OpenCodeVersion: "1.18.4", ProjectID: "global",
	}
	api := &h2LiveSessionAPI{}
	provider, err := newCraftRunViewContainerProvider(config, engine, func(_, directory, projectID string) (craftRunViewSessionAPI, error) {
		api.directory, api.projectID = directory, projectID
		return api, nil
	})
	require.NoError(t, err)

	var generationBytes [10]byte
	_, err = rand.Read(generationBytes[:])
	require.NoError(t, err)
	generation := "h2-live-" + hex.EncodeToString(generationBytes[:])
	spec := craftRunViewSpecForGeneration(generation)
	container, err := provider.InspectOrCreateContainer(context.Background(), spec)
	if err != nil {
		diagnostics, _ := exec.Command("docker", "inspect", "--format", "{{json .State}}", spec.ContainerID).CombinedOutput()
		logs, _ := exec.Command("docker", "logs", spec.ContainerID).CombinedOutput()
		t.Fatalf("provider container setup failed: %v\nstate=%s\nlogs=%s", err, diagnostics, logs)
	}
	require.True(t, container.IdentityVerified)
	require.True(t, container.DedicatedForRun)
	require.True(t, container.DirectoryCanonical)

	const runID = "run-live-proof"
	const sessionID = "ses_0123456789ab0123456789ABCD"
	api.sessionID = sessionID
	key := craft.RunViewKey{TenantID: 7, OwnerID: "owner-live", SessionID: "task-live", RunID: runID}
	view := boundMaterialTestView(key, container, generation, sessionID)
	store := &materialTestStore{view: view}
	material, err := provider.MaterialHandle(context.Background(), store, key,
		CraftRunViewRuntimeHandle{View: view, Directory: container.Directory})
	require.NoError(t, err)

	scope := craft.Scope{TenantID: 7, UserID: "owner-live", SessionID: "task-live"}
	workspace := craft.Workspace{ID: "workspace-live", Scope: scope}
	knowledgeStore := &craftRunViewKnowledgeStore{ownerScope: scope, workspace: workspace}
	records := &craftRunViewKnowledgeRecords{}
	taskAccess := &craftRunViewKnowledgeTaskAccess{allowed: true}
	builder, err := NewCraftKnowledgeRunViewBuilder(CraftKnowledgeRunViewConfig{
		Store:      knowledgeStore,
		Access:     func(context.Context, uint64, []string) ([]*types.Knowledge, error) { return nil, nil },
		Search:     func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) { return nil, nil },
		TaskAccess: taskAccess, Records: records,
	})
	require.NoError(t, err)
	raw, err := service.BuildDurableCraftRunSnapshotWithKnowledgeSelection(
		"model-composed prompt", nil, "model-1", "", &types.AgentConfig{AllowedTools: []string{"thinking"}}, []craft.Input{},
		service.CraftKnowledgeSelectionSnapshot{Query: "approved empty selection", KnowledgeBaseIDs: []string{}},
	)
	require.NoError(t, err)
	run := agentruntime.Run{Key: agentruntime.RunKey{TenantID: scope.TenantID, RunID: runID}, SessionID: scope.SessionID,
		UserID: scope.UserID, ActorUserID: scope.UserID, Snapshot: json.RawMessage(raw)}
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: scope.TenantID, UserID: scope.UserID})
	accepted, err := builder.BuildForRun(ctx, run, material)
	require.NoError(t, err)
	require.NoError(t, builder.VerifyAcceptedForRun(ctx, run, material, accepted))
	require.NoError(t, verifyRunViewKnowledgeRoot(material, runID))

	facts, err := engine.InspectContainer(context.Background(), container.ContainerID)
	require.NoError(t, err)
	require.NotNil(t, facts)
	knowledgeMount := path.Join(container.Directory, craft.KnowledgeDir)
	var mounted CraftRunViewMount
	for _, mount := range facts.Mounts {
		if mount.Destination == knowledgeMount {
			mounted = mount
		}
	}
	require.Equal(t, material.knowledge, mounted.Source)
	require.True(t, mounted.ReadOnly)
	runDestination := path.Join(container.Directory, filepath.ToSlash(craft.KnowledgeRunDir(runID)))
	manifest := path.Join(runDestination, "manifest.json")
	readCmd := exec.Command("docker", "exec", container.ContainerID, "/bin/sh", "-c", "test -s "+shellQuote(manifest))
	output, err := readCmd.CombinedOutput()
	require.NoError(t, err, "provider-created container must read accepted Run package: %s", output)

	// Challenge the actual mount with a host-writable file at the mounted root.
	// It is removed before exact-tree verification; the container must reject
	// even a root process write because the provider mount itself is read-only.
	writeChallenge := filepath.Join(material.knowledge, "write-challenge")
	require.NoError(t, os.WriteFile(writeChallenge, []byte("unchanged"), 0o666))
	require.NoError(t, os.Chmod(writeChallenge, 0o666))
	writeCmd := exec.Command("docker", "exec", "--user", "0:0", container.ContainerID, "/bin/sh", "-c",
		"printf changed >> "+shellQuote(path.Join(knowledgeMount, "write-challenge")))
	writeOutput, writeErr := writeCmd.CombinedOutput()
	require.Error(t, writeErr, "a root process must be unable to write through the read-only provider mount: %s", writeOutput)
	unchanged, err := os.ReadFile(writeChallenge)
	require.NoError(t, err)
	require.Equal(t, []byte("unchanged"), unchanged)
	require.NoError(t, os.Remove(writeChallenge))
	require.NoError(t, verifyRunViewKnowledgeRoot(material, runID))
	require.NoError(t, builder.VerifyAcceptedForRun(ctx, run, material, accepted))
	require.NoError(t, provider.RevalidateMaterialHandle(context.Background(), material))

	require.NoError(t, engine.cleanup())
	_, inspectErr := exec.Command("docker", "inspect", container.ContainerID).Output()
	require.Error(t, inspectErr, "live provider-created container must be removed")
	_, networkErr := exec.Command("docker", "network", "inspect", engine.networkNameFor(spec)).Output()
	require.Error(t, networkErr, "live provider-created network must be removed")
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

type h2LiveSessionAPI struct{ sessionID, directory, projectID string }

func (a *h2LiveSessionAPI) ListSessions(context.Context) ([]opencode.SessionInfo, error) {
	return []opencode.SessionInfo{rvTestOpenCodeSession(a.sessionID, a.directory, a.projectID)}, nil
}
func (a *h2LiveSessionAPI) GetSession(_ context.Context, id string) (opencode.SessionInfo, error) {
	if id != a.sessionID {
		return opencode.SessionInfo{}, craft.ErrNotFound
	}
	return rvTestOpenCodeSession(a.sessionID, a.directory, a.projectID), nil
}
func (*h2LiveSessionAPI) CreateSession(context.Context) (string, error) {
	return "ses_0123456789ab0123456789ABCD", nil
}

type h2DockerProviderEngine struct {
	mu       sync.Mutex
	networks map[string]CraftRunViewContainerNetwork
}

func (e *h2DockerProviderEngine) EnsurePrivateNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	if _, err := e.inspectNetwork(ctx, spec.Name); err != nil {
		args := []string{"network", "create", "--driver", spec.Driver}
		if spec.Internal {
			args = append(args, "--internal")
		}
		for name, value := range spec.Labels {
			args = append(args, "--label", name+"="+value)
		}
		args = append(args, spec.Name)
		if _, createErr := e.command(ctx, args...); createErr != nil {
			return CraftRunViewContainerNetwork{}, createErr
		}
	}
	actual, err := e.inspectNetwork(ctx, spec.Name)
	if err == nil {
		e.mu.Lock()
		e.networks[spec.Name] = actual
		e.mu.Unlock()
	}
	return actual, err
}

// ObserveNetwork inspects the generation network read-only; absence is found=false.
func (e *h2DockerProviderEngine) ObserveNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, bool, error) {
	actual, err := e.inspectNetwork(ctx, spec.Name)
	if err != nil {
		return CraftRunViewContainerNetwork{}, false, nil
	}
	e.mu.Lock()
	e.networks[spec.Name] = actual
	e.mu.Unlock()
	return actual, true, nil
}

// CreateGenerationNetwork sends exactly one network create and inspects the receipt.
func (e *h2DockerProviderEngine) CreateGenerationNetwork(ctx context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	args := []string{"network", "create", "--driver", spec.Driver}
	if spec.Internal {
		args = append(args, "--internal")
	}
	for name, value := range spec.Labels {
		args = append(args, "--label", name+"="+value)
	}
	args = append(args, spec.Name)
	if _, createErr := e.command(ctx, args...); createErr != nil {
		return CraftRunViewContainerNetwork{}, createErr
	}
	return e.inspectNetwork(ctx, spec.Name)
}

func (e *h2DockerProviderEngine) ObserveRuntimeProbe(context.Context, string) (CraftRunViewRuntimeProbe, bool, error) {
	return CraftRunViewRuntimeProbe{}, false, unresolvedCraftRunView("live test engine cannot reconstruct probe output read-only", nil)
}

func (e *h2DockerProviderEngine) SendRuntimeProbe(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	probe, err := e.ProbeRuntime(ctx, id)
	if err != nil {
		return CraftRunViewRuntimeProbe{}, err
	}
	probe.ContainerID = id
	probe.ExecID = "exec-" + id
	return probe, nil
}

func (e *h2DockerProviderEngine) inspectNetwork(ctx context.Context, name string) (CraftRunViewContainerNetwork, error) {
	var rows []struct {
		ID, Name, Driver string
		Internal         bool
		Labels           map[string]string
	}
	if err := e.jsonCommand(ctx, &rows, "network", "inspect", name); err != nil {
		return CraftRunViewContainerNetwork{}, err
	}
	if len(rows) != 1 {
		return CraftRunViewContainerNetwork{}, errors.New("Docker returned ambiguous network inspection")
	}
	row := rows[0]
	return CraftRunViewContainerNetwork{ID: row.ID, Name: row.Name, Driver: row.Driver, Internal: row.Internal, Labels: row.Labels}, nil
}

func (e *h2DockerProviderEngine) InspectContainer(ctx context.Context, name string) (*CraftRunViewEngineContainer, error) {
	var rows []struct {
		ID, Name, Image string
		State           struct{ Status string }
		Config          struct {
			Image, User, WorkingDir string
			Entrypoint, Cmd, Env    []string
			Labels                  map[string]string
		}
		HostConfig struct {
			Privileged, ReadonlyRootfs bool
			SecurityOpt                []string
			CapDrop, CapAdd            []string
			Memory, PidsLimit          int64
			PidMode, IpcMode, UTSMode  string
			Devices                    []struct{ PathOnHost string }
			PortBindings               map[string][]json.RawMessage
			Tmpfs                      map[string]string
			NetworkMode                string
		}
		Mounts []struct {
			Type, Source, Destination string
			RW                        bool
		}
		NetworkSettings struct {
			Networks map[string]struct{ NetworkID, IPAddress string }
		}
	}
	if err := e.jsonCommand(ctx, &rows, "inspect", name); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such object") || strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return nil, nil
		}
		return nil, err
	}
	if len(rows) != 1 {
		return nil, errors.New("Docker returned ambiguous container inspection")
	}
	row := rows[0]
	facts := &CraftRunViewEngineContainer{
		ID: row.ID, Name: strings.TrimPrefix(row.Name, "/"), State: row.State.Status, ImageID: row.Image,
		ImageReference: row.Config.Image, ImageDigest: row.Config.Labels[craftRunViewLabelImage],
		User: row.Config.User, WorkingDirectory: row.Config.WorkingDir,
		Entrypoint: row.Config.Entrypoint, Command: row.Config.Cmd, Environment: row.Config.Env,
		Labels: row.Config.Labels, Privileged: row.HostConfig.Privileged, ReadonlyRootfs: row.HostConfig.ReadonlyRootfs,
		NetworkMode:     row.HostConfig.NetworkMode,
		NoNewPrivileges: containsString(row.HostConfig.SecurityOpt, "no-new-privileges:true"),
		CapDrop:         row.HostConfig.CapDrop, CapAdd: row.HostConfig.CapAdd, MemoryBytes: row.HostConfig.Memory, PidsLimit: row.HostConfig.PidsLimit,
		HostPID: row.HostConfig.PidMode == "host", HostIPC: row.HostConfig.IpcMode == "host", HostUTS: row.HostConfig.UTSMode == "host",
		PublishedPorts: []string{}, Devices: []string{},
	}
	if len(row.HostConfig.Devices) != 0 {
		for _, device := range row.HostConfig.Devices {
			facts.Devices = append(facts.Devices, device.PathOnHost)
		}
	}
	for _, mount := range row.Mounts {
		projected := CraftRunViewMount{Type: mount.Type, Source: mount.Source, Destination: mount.Destination, ReadOnly: !mount.RW}
		if mount.Type == "tmpfs" {
			for _, option := range strings.Split(row.HostConfig.Tmpfs[mount.Destination], ",") {
				if option != "" {
					projected.Options = append(projected.Options, option)
				}
			}
		}
		facts.Mounts = append(facts.Mounts, projected)
	}
	for destination, options := range row.HostConfig.Tmpfs {
		if !containsCraftRunViewMountDestination(facts.Mounts, destination) {
			facts.Mounts = append(facts.Mounts, CraftRunViewMount{Type: "tmpfs", Destination: destination, Options: strings.Split(options, ",")})
		}
	}
	for networkName, attachment := range row.NetworkSettings.Networks {
		network, err := e.inspectNetwork(ctx, networkName)
		if err != nil {
			return nil, err
		}
		facts.Networks = append(facts.Networks, CraftRunViewNetworkAttachment{
			Name: networkName, ID: attachment.NetworkID, Internal: network.Internal, IP: attachment.IPAddress,
		})
	}
	return facts, nil
}

func (e *h2DockerProviderEngine) CreateContainer(ctx context.Context, request CraftRunViewContainerCreateRequest) (string, error) {
	args := []string{"create", "--name", request.Name, "--network", request.NetworkMode, "--user", request.User,
		"--workdir", request.WorkingDirectory, "--entrypoint", request.Entrypoint[0]}
	if request.ReadonlyRootfs {
		args = append(args, "--read-only")
	}
	if request.NoNewPrivileges {
		args = append(args, "--security-opt", "no-new-privileges:true")
	}
	if request.Privileged {
		args = append(args, "--privileged")
	}
	for _, capability := range request.CapDrop {
		args = append(args, "--cap-drop", capability)
	}
	for _, capability := range request.CapAdd {
		args = append(args, "--cap-add", capability)
	}
	if request.MemoryBytes > 0 {
		args = append(args, "--memory", fmt.Sprint(request.MemoryBytes))
	}
	if request.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprint(request.PidsLimit))
	}
	for _, env := range request.Environment {
		args = append(args, "--env", env)
	}
	for key, value := range request.Labels {
		args = append(args, "--label", key+"="+value)
	}
	for _, mount := range request.Mounts {
		if mount.Type == "tmpfs" {
			args = append(args, "--tmpfs", mount.Destination+":"+strings.Join(mount.Options, ","))
			continue
		}
		value := fmt.Sprintf("type=bind,src=%s,dst=%s", mount.Source, mount.Destination)
		if mount.ReadOnly {
			value += ",readonly"
		}
		args = append(args, "--mount", value)
	}
	args = append(args, request.ImageReference)
	args = append(args, request.Command...)
	id, err := e.command(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(id)), nil
}

func (e *h2DockerProviderEngine) StartContainer(ctx context.Context, id string) error {
	_, err := e.command(ctx, "start", id)
	return err
}

func (e *h2DockerProviderEngine) ProbeRuntime(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	output, err := e.command(ctx, "exec", id, "/bin/sh", "-c",
		"/usr/local/bin/opencode --version && sha256sum /usr/local/bin/opencode | cut -d ' ' -f 1 && sha256sum /etc/craft/runtime-config.json | cut -d ' ' -f 1")
	if err != nil {
		return CraftRunViewRuntimeProbe{}, err
	}
	lines := strings.Fields(string(output))
	if len(lines) != 3 {
		return CraftRunViewRuntimeProbe{}, fmt.Errorf("Docker runtime probe returned %d values", len(lines))
	}
	return CraftRunViewRuntimeProbe{OpenCodeVersion: lines[0], BinarySHA256: lines[1], RuntimeConfigSHA256: lines[2]}, nil
}

func (e *h2DockerProviderEngine) networkNameFor(spec CraftRunViewRuntimeContainerSpec) string {
	return "rv-network-" + strings.TrimPrefix(spec.ContainerID, "rv-container-")
}

func (e *h2DockerProviderEngine) cleanup() error {
	e.mu.Lock()
	names := make([]string, 0, len(e.networks))
	for name := range e.networks {
		names = append(names, name)
	}
	e.mu.Unlock()
	var errs []error
	for _, name := range names {
		suffix := strings.TrimPrefix(name, "rv-network-")
		if _, err := e.command(context.Background(), "rm", "-f", "rv-container-"+suffix); err != nil {
			errs = append(errs, err)
		}
		_, netErr := e.command(context.Background(), "network", "rm", name)
		if netErr != nil {
			errs = append(errs, netErr)
		}
	}
	return errors.Join(errs...)
}

func (e *h2DockerProviderEngine) command(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (e *h2DockerProviderEngine) jsonCommand(ctx context.Context, target any, args ...string) error {
	output, err := e.command(ctx, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(output, target)
}

func containsCraftRunViewMountDestination(mounts []CraftRunViewMount, destination string) bool {
	for _, mount := range mounts {
		if mount.Destination == destination {
			return true
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
