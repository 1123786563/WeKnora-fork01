package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

const (
	rvTestGeneration = "generation-opaque-01"
	rvTestProject    = "global"
)

func rvTestConfig(t *testing.T) CraftRunViewContainerProviderConfig {
	t.Helper()
	return CraftRunViewContainerProviderConfig{
		SandboxRoot:          filepath.Join(t.TempDir(), "craft-runviews"),
		ImageReference:       "registry.example.invalid/craft-runtime@sha256:" + strings.Repeat("a", 64),
		ImageDigest:          "sha256:" + strings.Repeat("a", 64),
		OpenCodeBinarySHA256: strings.Repeat("b", 64),
		RuntimeConfigSHA256:  strings.Repeat("c", 64),
		OpenCodeVersion:      "1.18.4",
		ProjectID:            rvTestProject,
	}
}

func rvTestSpec(generation string) CraftRunViewRuntimeContainerSpec {
	sum := sha256.Sum256([]byte(generation))
	short := hex.EncodeToString(sum[:16])
	return CraftRunViewRuntimeContainerSpec{
		Generation:  generation,
		RuntimeID:   "rv-runtime-" + short,
		ContainerID: "rv-container-" + short,
		Directory:   "/workspace/rv-" + short,
	}
}

func newRVTestProvider(t *testing.T, engine *fakeCraftRunViewContainerEngine) *CraftRunViewContainerProvider {
	t.Helper()
	p, _, err := newRVTestProviderWithConfig(t, engine)
	require.NoError(t, err)
	return p
}

func newRVTestProviderWithConfig(t *testing.T, engine *fakeCraftRunViewContainerEngine) (*CraftRunViewContainerProvider, CraftRunViewContainerProviderConfig, error) {
	t.Helper()
	config := rvTestConfig(t)
	p, err := NewCraftRunViewContainerProvider(config, engine)
	return p, config, err
}

func TestCraftRunViewContainerProviderCreatesDeterministicHardenedContainer(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider := newRVTestProvider(t, engine)
	spec := rvTestSpec(rvTestGeneration)

	got, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, spec.Generation, got.Generation)
	require.Equal(t, spec.RuntimeID, got.RuntimeID)
	require.Equal(t, spec.ContainerID, got.ContainerID)
	require.Equal(t, spec.Directory, got.Directory)
	require.Equal(t, rvTestProject, got.ProjectID)
	require.True(t, got.IdentityVerified)
	require.True(t, got.DedicatedForRun)
	require.True(t, got.DirectoryCanonical)

	require.Equal(t, 1, engine.createCount())
	require.Len(t, engine.createRequests, 1)
	request := engine.createRequests[0]
	require.Equal(t, spec.ContainerID, request.Name)
	require.Equal(t, rvTestConfig(t).ImageReference, request.ImageReference)
	require.Equal(t, "10001:10001", request.User)
	require.Equal(t, spec.Directory, request.WorkingDirectory)
	require.Equal(t, spec.Directory, request.Directory)
	require.Empty(t, request.PublishedPorts)
	require.False(t, request.Privileged)
	require.True(t, request.ReadonlyRootfs)
	require.True(t, request.NoNewPrivileges)
	require.Equal(t, []string{"ALL"}, request.CapDrop)
	require.Empty(t, request.CapAdd)
	require.Equal(t, []string{"HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config", "XDG_DATA_HOME=/home/craft/.local/share", "XDG_STATE_HOME=/home/craft/.local/share/opencode", "XDG_CACHE_HOME=/home/craft/.local/share/opencode/cache", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}, request.Environment)
	require.Equal(t, spec.ContainerID, request.Labels[craftRunViewLabelContainer])
	require.Equal(t, spec.RuntimeID, request.Labels[craftRunViewLabelRuntime])
	generationDigest := sha256.Sum256([]byte(spec.Generation))
	require.Equal(t, hex.EncodeToString(generationDigest[:]), request.Labels[craftRunViewLabelGeneration])
	require.Len(t, request.Mounts, 6)
	requireMount(t, request.Mounts, "/inputs", true)
	requireMount(t, request.Mounts, "/knowledge", true)
	requireMount(t, request.Mounts, "/output", false)
	requireMount(t, request.Mounts, "/home/craft/.local/share/opencode", false)
	requireMount(t, request.Mounts, "/home/craft/.config/opencode", false)
	requireMount(t, request.Mounts, "/tmp", false)
	for _, mount := range request.Mounts {
		if mount.Type == "bind" {
			require.Contains(t, mount.Source, filepath.Join("craft-runviews", "rv-"))
			require.NotContains(t, mount.Source, "/var/run/docker.sock")
			require.NotContains(t, mount.Source, "/home/")
		}
	}
	require.Equal(t, 1, engine.startCount())
	require.Equal(t, []string{"engine-" + spec.ContainerID}, engine.startedIDs)
}

func TestCraftRunViewAdmittedProviderObservationsAreReadOnlyAndFailClosed(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	spec := rvTestSpec("admitted-observe-only")
	layout, err := provider.prepareGenerationLayout(spec)
	require.NoError(t, err)
	created, err := provider.createMarker(layout, spec)
	require.NoError(t, err)
	require.True(t, created)
	networkSpec := provider.networkSpec(spec)
	facts := engine.factsForSpec(t, provider.config, spec)
	facts.ID = "docker-container-observed"
	facts.State = "running"
	engine.seedContainer(spec.ContainerID, facts, networkSpec)
	before := craftRunViewFilesystemSnapshot(t, provider.config.SandboxRoot)

	network, found, err := provider.ObserveNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.True(t, found)
	observed, found, err := provider.ObserveContainer(context.Background(), spec, network)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, facts.ID, observed.DockerID)
	state, found, err := provider.ObserveContainerState(context.Background(), observed)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, facts.ID, state.DockerID)
	require.Equal(t, "running", state.State)

	probe, found, err := provider.ObserveRuntimeProbe(context.Background(), state)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved, "Docker cannot reconstruct prior probe stdout from read-only state")
	require.False(t, found)
	require.Empty(t, probe)
	inventory, err := provider.ObserveSessions(context.Background(), state)
	require.NoError(t, err)
	require.True(t, inventory.Authoritative)
	require.True(t, inventory.Complete)
	require.Empty(t, inventory.Sessions)
	require.Zero(t, api.createCount(), "session observation must never POST")

	engine.mu.Lock()
	require.Zero(t, engine.ensureNetworkCalls, "admitted Observe must not call legacy EnsurePrivateNetwork")
	require.Zero(t, engine.networkCreateCalls)
	require.Empty(t, engine.createRequests)
	require.Empty(t, engine.startedIDs)
	require.Zero(t, engine.probeCalls, "ObserveRuntimeProbe must not call ProbeRuntime")
	engine.mu.Unlock()
	require.Equal(t, before, craftRunViewFilesystemSnapshot(t, provider.config.SandboxRoot), "Observe methods must not mutate generation files")
}

func TestCraftRunViewAdmittedProviderSplitsClaimsAndReturnsExactReceipts(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	spec := rvTestSpec("admitted-split-send")

	_, found, err := provider.ObserveNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.False(t, found)
	network, err := provider.CreateGenerationNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.NotEmpty(t, network.ID)
	observedNetwork, found, err := provider.ObserveNetwork(context.Background(), spec)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, network, observedNetwork, "create receipt must be the observed Docker identity")

	_, found, err = provider.ObserveContainer(context.Background(), spec, network)
	require.NoError(t, err)
	require.False(t, found)
	container, err := provider.CreateGenerationContainer(context.Background(), spec, network)
	require.NoError(t, err)
	require.Equal(t, "engine-"+spec.ContainerID, container.DockerID)
	require.Equal(t, "created", container.State)
	require.Equal(t, network.ID, container.Network.ID)

	observed, found, err := provider.ObserveContainer(context.Background(), spec, network)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, container.DockerID, observed.DockerID)
	createdState, found, err := provider.ObserveContainerState(context.Background(), observed)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "created", createdState.State)

	running, err := provider.StartGenerationContainer(context.Background(), createdState)
	require.NoError(t, err)
	require.Equal(t, container.DockerID, running.DockerID)
	require.Equal(t, "running", running.State)
	probe, err := provider.SendRuntimeProbe(context.Background(), running)
	require.NoError(t, err)
	require.Equal(t, container.DockerID, probe.ContainerID)
	require.Equal(t, "exec-"+container.DockerID, probe.ExecID)
	require.Equal(t, provider.config.OpenCodeBinarySHA256, probe.BinarySHA256)

	api.getOverride = ptrSession(rvTestOpenCodeSession("ses_0123456789ab0123456789ABCD", spec.Directory, rvTestProject))
	session, err := provider.CreateOpenCodeSession(context.Background(), running, spec.Directory)
	require.NoError(t, err)
	require.Equal(t, CraftRunViewRuntimeSession{ID: "ses_0123456789ab0123456789ABCD", ProjectID: rvTestProject, Directory: spec.Directory}, session)
	require.Equal(t, 1, api.createCount())
	require.Equal(t, 1, api.getCount(), "the create receipt must be confirmed by exact GET metadata")

	engine.mu.Lock()
	defer engine.mu.Unlock()
	require.Zero(t, engine.ensureNetworkCalls, "split path must not call legacy network create/inspect composite")
	require.Equal(t, 1, engine.networkCreateCalls, "exactly one network send")
	require.Len(t, engine.createRequests, 1, "exactly one container create send")
	require.Equal(t, []string{container.DockerID}, engine.startedIDs, "exactly one start send")
	require.Equal(t, 1, engine.probeCalls, "exactly one runtime probe send")
}

func TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider := newRVTestProvider(t, engine)
	spec := rvTestSpec("admitted-start-rechecks-drift")

	network, err := provider.CreateGenerationNetwork(context.Background(), spec)
	require.NoError(t, err)
	created, err := provider.CreateGenerationContainer(context.Background(), spec, network)
	require.NoError(t, err)
	require.Equal(t, "created", created.State)

	// The earlier observation remains structurally valid, but Docker's current
	// container has drifted to a less restrictive isolation profile.
	drifted := engine.containerSnapshot(spec.ContainerID)
	drifted.Privileged = true
	engine.replaceContainer(spec.ContainerID, drifted)

	_, err = provider.StartGenerationContainer(context.Background(), created)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	require.Empty(t, engine.startedIDs, "fresh isolation drift must be rejected before the one start send")
}

func craftRunViewFilesystemSnapshot(t *testing.T, root string) string {
	t.Helper()
	var snapshot strings.Builder
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&snapshot, "%s|%s|%d|%d", relative, info.Mode(), info.Size(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(contents)
			fmt.Fprintf(&snapshot, "|%x", digest)
		}
		snapshot.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	return snapshot.String()
}

func TestCraftRunViewContainerProviderRestartsSameContainerWithPersistentPrivateHome(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider := newRVTestProvider(t, engine)
	spec := rvTestSpec(rvTestGeneration)
	first, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	before := engine.containerSnapshot(spec.ContainerID)
	engine.setState(spec.ContainerID, "exited")

	second, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, engine.createCount(), "restart must not create a replacement")
	require.Equal(t, 2, engine.startCount())
	after := engine.containerSnapshot(spec.ContainerID)
	require.Equal(t, before.ID, after.ID)
	require.Equal(t, before.Mounts, after.Mounts)
	require.Equal(t, before.Networks, after.Networks)
}

func TestCraftRunViewContainerProviderRejectsSandboxRootSymlinkIntoForbiddenHostPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	canonicalHome, err := filepath.EvalSymlinks(home)
	require.NoError(t, err)
	covered := false
	for _, forbidden := range []string{"/home", "/root", "/Users"} {
		if pathWithin(forbidden, canonicalHome) {
			covered = true
			break
		}
	}
	if !covered {
		t.Skip("current home is outside the configured forbidden host-root policy")
	}
	forbiddenTarget, err := os.MkdirTemp(home, ".craft-forbidden-root-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(forbiddenTarget) })
	require.NoError(t, os.Chmod(forbiddenTarget, 0755))
	before, err := os.Stat(forbiddenTarget)
	require.NoError(t, err)

	configured := filepath.Join(t.TempDir(), "allowed-looking-root")
	require.NoError(t, os.Symlink(forbiddenTarget, configured))
	config := rvTestConfig(t)
	config.SandboxRoot = configured
	_, err = NewCraftRunViewContainerProvider(config, newFakeCraftRunViewContainerEngine())
	require.Error(t, err, "the resolved canonical path must be checked against forbidden host roots")

	after, err := os.Stat(forbiddenTarget)
	require.NoError(t, err)
	require.Equal(t, before.Mode().Perm(), after.Mode().Perm(), "rejection must happen before chmod or layout mutation")
}

func TestCraftRunViewContainerProviderRejectsForeignAndMisconfiguredContainers(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*CraftRunViewEngineContainer)
	}{
		{name: "same-name foreign generation", mutate: func(c *CraftRunViewEngineContainer) { c.Labels[craftRunViewLabelGeneration] = "foreign" }},
		{name: "wrong image digest", mutate: func(c *CraftRunViewEngineContainer) { c.ImageDigest = "sha256:" + strings.Repeat("d", 64) }},
		{name: "missing observed image ID", mutate: func(c *CraftRunViewEngineContainer) { c.ImageID = "" }},
		{name: "wrong runtime label", mutate: func(c *CraftRunViewEngineContainer) { c.Labels[craftRunViewLabelRuntime] = "foreign" }},
		{name: "wrong mount", mutate: func(c *CraftRunViewEngineContainer) { c.Mounts[0].Source = "/etc" }},
		{name: "writable inputs", mutate: func(c *CraftRunViewEngineContainer) { c.Mounts[0].ReadOnly = false }},
		{name: "wrong user", mutate: func(c *CraftRunViewEngineContainer) { c.User = "0:0" }},
		{name: "wrong network", mutate: func(c *CraftRunViewEngineContainer) { c.Networks[0].Name = "shared-bridge" }},
		{name: "non-private network", mutate: func(c *CraftRunViewEngineContainer) { c.Networks[0].Internal = false }},
		{name: "extra shared network", mutate: func(c *CraftRunViewEngineContainer) {
			c.Networks = append(c.Networks, CraftRunViewNetworkAttachment{Name: "shared-bridge", ID: "shared", IP: "172.29.0.2"})
		}},
		{name: "shared HOME", mutate: func(c *CraftRunViewEngineContainer) { c.Mounts[3].Source = "/home/shared/.local/share/opencode" }},
		{name: "published port", mutate: func(c *CraftRunViewEngineContainer) { c.PublishedPorts = []string{"4096:4096"} }},
		{name: "wrong command", mutate: func(c *CraftRunViewEngineContainer) { c.Command = []string{"sh"} }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeCraftRunViewContainerEngine()
			provider := newRVTestProvider(t, engine)
			spec := rvTestSpec(rvTestGeneration)
			_, err := provider.InspectOrCreateContainer(context.Background(), spec)
			require.NoError(t, err)
			layout, err := provider.prepareGenerationLayout(spec)
			require.NoError(t, err)
			require.FileExists(t, layout.marker, "the mismatch case must reach inspection with valid create intent")
			facts := engine.containerSnapshot(spec.ContainerID)
			tc.mutate(&facts)
			engine.replaceContainer(spec.ContainerID, facts)
			_, err = provider.InspectOrCreateContainer(context.Background(), spec)
			require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
			require.ErrorContains(t, err, "container inspection does not match immutable generation config")
			require.Equal(t, 1, engine.createCount(), "foreign or wrong runtime must never be replaced")
		})
	}
	t.Run("wrong runtime probe", func(t *testing.T) {
		engine := newFakeCraftRunViewContainerEngine()
		provider := newRVTestProvider(t, engine)
		spec := rvTestSpec(rvTestGeneration)
		_, err := provider.InspectOrCreateContainer(context.Background(), spec)
		require.NoError(t, err)
		layout, err := provider.prepareGenerationLayout(spec)
		require.NoError(t, err)
		require.FileExists(t, layout.marker)
		engine.setProbe(CraftRunViewRuntimeProbe{OpenCodeVersion: "1.18.4", BinarySHA256: strings.Repeat("e", 64), RuntimeConfigSHA256: strings.Repeat("c", 64)})
		_, err = provider.InspectOrCreateContainer(context.Background(), spec)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.ErrorContains(t, err, "runtime version/binary/config probe failed")
		require.Equal(t, 1, engine.createCount())
	})
}

func TestCraftRunViewContainerProviderFailsClosedForMissingUncertainAndChangedGeneration(t *testing.T) {
	t.Run("inspect uncertainty", func(t *testing.T) {
		engine := newFakeCraftRunViewContainerEngine()
		engine.inspectErr = errors.New("daemon timeout")
		provider := newRVTestProvider(t, engine)
		_, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec(rvTestGeneration))
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Zero(t, engine.createCount())
	})

	t.Run("missing after prior create intent", func(t *testing.T) {
		engine := newFakeCraftRunViewContainerEngine()
		engine.createErr = errors.New("create response lost")
		provider := newRVTestProvider(t, engine)
		spec := rvTestSpec(rvTestGeneration)
		_, err := provider.InspectOrCreateContainer(context.Background(), spec)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, engine.createCount())
		engine.createErr = nil
		_, err = provider.InspectOrCreateContainer(context.Background(), spec)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, engine.createCount(), "durable create intent forbids blind replacement")
	})

	t.Run("changed generation identity", func(t *testing.T) {
		engine := newFakeCraftRunViewContainerEngine()
		provider := newRVTestProvider(t, engine)
		spec := rvTestSpec(rvTestGeneration)
		spec.RuntimeID = "rv-runtime-foreign"
		_, err := provider.InspectOrCreateContainer(context.Background(), spec)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Zero(t, engine.createCount())
	})
}

func TestCraftRunViewContainerProviderLeavesUncertainCreateUnresolvedThenRecoversSameContainer(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	engine.createErr = errors.New("create response lost")
	engine.persistOnCreateError = true
	provider := newRVTestProvider(t, engine)
	spec := rvTestSpec(rvTestGeneration)
	_, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, engine.createCount())

	engine.createErr = nil
	got, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, spec.ContainerID, got.ContainerID)
	require.Equal(t, 1, engine.createCount())
}

func TestCraftRunViewContainerProviderConcurrentCallsCreateOnce(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider := newRVTestProvider(t, engine)
	spec := rvTestSpec(rvTestGeneration)
	const callers = 8
	results := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			container, err := provider.InspectOrCreateContainer(context.Background(), spec)
			if err == nil && (container.ContainerID != spec.ContainerID || !container.IdentityVerified) {
				err = fmt.Errorf("unverified concurrent result: %+v", container)
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, 1, engine.createCount())
}

func TestCraftRunViewContainerProviderFindSessionsRequiresCompleteScopedInventory(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	spec := rvTestSpec(rvTestGeneration)
	container, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	sessionID := "ses_0123456789ab0123456789ABCD"
	api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, container.Directory, container.ProjectID)}

	inventory, err := provider.FindSessions(context.Background(), container)
	require.NoError(t, err)
	require.True(t, inventory.Authoritative)
	require.True(t, inventory.Complete)
	require.Equal(t, container.Directory, inventory.Directory)
	require.Equal(t, container.ProjectID, inventory.ProjectID)
	require.Equal(t, []CraftRunViewRuntimeSession{{ID: sessionID, Directory: container.Directory, ProjectID: container.ProjectID}}, inventory.Sessions)
	require.Equal(t, 1, api.getCount(), "a unique list row must be confirmed by exact-ID metadata")

	api.listErr = errors.New("second page failed")
	inventory, err = provider.FindSessions(context.Background(), container)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.False(t, inventory.Authoritative)
	require.False(t, inventory.Complete)
}

func TestCraftRunViewContainerProviderRequiresExactGetForUniqueListedSession(t *testing.T) {
	const sessionID = "ses_0123456789ab0123456789ABCD"
	tests := []struct {
		name       string
		getError   error
		getSession *opencode.SessionInfo
		wantError  bool
	}{
		{name: "exact metadata matches"},
		{name: "GET missing", getError: errors.New("404 not found"), wantError: true},
		{name: "GET ID changed", getSession: ptrSession(rvTestOpenCodeSession("ses_0123456789ab0123456789ABCE", "/workspace/other", rvTestProject)), wantError: true},
		{name: "GET project changed", getSession: ptrSession(rvTestOpenCodeSession(sessionID, "/workspace/rv", "foreign-project")), wantError: true},
		{name: "GET directory changed", getSession: ptrSession(rvTestOpenCodeSession(sessionID, "/workspace/foreign", rvTestProject)), wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeCraftRunViewContainerEngine()
			provider, api := newRVTestProviderWithSessionAPI(t, engine)
			container, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec(rvTestGeneration))
			require.NoError(t, err)
			api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, container.Directory, container.ProjectID)}
			api.getErr = tc.getError
			api.getOverride = tc.getSession

			inventory, err := provider.FindSessions(context.Background(), container)
			if tc.wantError {
				require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
				require.False(t, inventory.Authoritative)
				require.False(t, inventory.Complete)
				return
			}
			require.NoError(t, err)
			require.True(t, inventory.Authoritative)
			require.True(t, inventory.Complete)
			require.Equal(t, 1, api.getCount())
		})
	}
}

func ptrSession(value opencode.SessionInfo) *opencode.SessionInfo { return &value }

func TestCraftRunViewContainerProviderCreateSessionRequiresExactDirectory(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	spec := rvTestSpec(rvTestGeneration)
	container, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	api.createErr = errors.New("response lost")
	require.ErrorIs(t, provider.CreateSession(context.Background(), container, "/workspace/foreign"), ErrCraftRunViewRuntimeUnresolved)
	require.Zero(t, api.createCount())
	require.ErrorIs(t, provider.CreateSession(context.Background(), container, container.Directory), ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, api.createCount())
}

func requireMount(t *testing.T, mounts []CraftRunViewMount, destination string, readOnly bool) {
	t.Helper()
	for _, mount := range mounts {
		if mount.Destination == destination || strings.HasSuffix(mount.Destination, destination) {
			require.Equal(t, readOnly, mount.ReadOnly, "mount %s read-only state", destination)
			return
		}
	}
	t.Fatalf("missing mount destination %s in %+v", destination, mounts)
}

type fakeCraftRunViewContainerEngine struct {
	mu                   sync.Mutex
	networks             map[string]CraftRunViewContainerNetwork
	containers           map[string]CraftRunViewEngineContainer
	probes               map[string]CraftRunViewRuntimeProbe
	ensureNetworkCalls   int
	networkObserveCalls  int
	networkCreateCalls   int
	createRequests       []CraftRunViewContainerCreateRequest
	startedIDs           []string
	probeCalls           int
	inspectErr           error
	createErr            error
	persistOnCreateError bool
	startErr             error
	probeErr             error
	beforeCreate         func(CraftRunViewContainerCreateRequest)
}

func newFakeCraftRunViewContainerEngine() *fakeCraftRunViewContainerEngine {
	return &fakeCraftRunViewContainerEngine{
		networks:   make(map[string]CraftRunViewContainerNetwork),
		containers: make(map[string]CraftRunViewEngineContainer),
		probes:     make(map[string]CraftRunViewRuntimeProbe),
	}
}

func (e *fakeCraftRunViewContainerEngine) EnsurePrivateNetwork(_ context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensureNetworkCalls++
	if got, exists := e.networks[spec.Name]; exists {
		return cloneCraftRunViewNetwork(got), nil
	}
	got := CraftRunViewContainerNetwork{ID: "network-" + spec.Name, Name: spec.Name, Driver: spec.Driver, Internal: spec.Internal, Labels: cloneStringMap(spec.Labels)}
	e.networks[spec.Name] = got
	return cloneCraftRunViewNetwork(got), nil
}

func (e *fakeCraftRunViewContainerEngine) ObserveNetwork(_ context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.networkObserveCalls++
	got, exists := e.networks[spec.Name]
	if !exists {
		return CraftRunViewContainerNetwork{}, false, nil
	}
	return cloneCraftRunViewNetwork(got), true, nil
}

func (e *fakeCraftRunViewContainerEngine) CreateGenerationNetwork(_ context.Context, spec CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.networkCreateCalls++
	if _, exists := e.networks[spec.Name]; exists {
		return CraftRunViewContainerNetwork{}, errors.New("network already exists")
	}
	got := CraftRunViewContainerNetwork{ID: "network-" + spec.Name, Name: spec.Name, Driver: spec.Driver, Internal: spec.Internal, Labels: cloneStringMap(spec.Labels)}
	e.networks[spec.Name] = got
	return cloneCraftRunViewNetwork(got), nil
}

func (e *fakeCraftRunViewContainerEngine) InspectContainer(_ context.Context, name string) (*CraftRunViewEngineContainer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inspectErr != nil {
		return nil, e.inspectErr
	}
	container, exists := e.containers[name]
	if !exists {
		return nil, nil
	}
	copy := cloneCraftRunViewContainer(container)
	return &copy, nil
}

func (e *fakeCraftRunViewContainerEngine) CreateContainer(_ context.Context, request CraftRunViewContainerCreateRequest) (string, error) {
	if e.beforeCreate != nil {
		e.beforeCreate(cloneCraftRunViewRequest(request))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.createRequests = append(e.createRequests, cloneCraftRunViewRequest(request))
	if e.createErr != nil && !e.persistOnCreateError {
		return "", e.createErr
	}
	network := e.networks[request.NetworkMode]
	facts := CraftRunViewEngineContainer{
		ID: "engine-" + request.Name, Name: request.Name, State: "created",
		ImageID:        "test-image-id",
		ImageReference: request.ImageReference, ImageDigest: request.Labels[craftRunViewLabelImage],
		User: request.User, WorkingDirectory: request.WorkingDirectory,
		Entrypoint: append([]string(nil), request.Entrypoint...), Command: append([]string(nil), request.Command...),
		Environment: append([]string(nil), request.Environment...), Labels: cloneStringMap(request.Labels),
		Mounts: cloneCraftRunViewMounts(request.Mounts), Networks: []CraftRunViewNetworkAttachment{{Name: network.Name, ID: network.ID, Internal: network.Internal, IP: "172.30.0.2"}},
		NetworkMode:    request.NetworkMode,
		PublishedPorts: append([]string(nil), request.PublishedPorts...),
		Privileged:     request.Privileged, ReadonlyRootfs: request.ReadonlyRootfs, NoNewPrivileges: request.NoNewPrivileges,
		CapDrop: append([]string(nil), request.CapDrop...), CapAdd: append([]string(nil), request.CapAdd...),
		MemoryBytes: request.MemoryBytes, PidsLimit: request.PidsLimit,
	}
	e.containers[request.Name] = facts
	e.probes[facts.ID] = e.validProbe()
	if e.createErr != nil {
		return "", e.createErr
	}
	return facts.ID, nil
}

func (e *fakeCraftRunViewContainerEngine) StartContainer(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.startErr != nil {
		return e.startErr
	}
	e.startedIDs = append(e.startedIDs, id)
	for name, container := range e.containers {
		if container.ID == id {
			container.State = "running"
			e.containers[name] = container
			return nil
		}
	}
	return errors.New("container not found")
}

func (e *fakeCraftRunViewContainerEngine) ProbeRuntime(_ context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.probeCalls++
	if e.probeErr != nil {
		return CraftRunViewRuntimeProbe{}, e.probeErr
	}
	probe, exists := e.probes[id]
	if !exists {
		return CraftRunViewRuntimeProbe{}, errors.New("probe target not found")
	}
	return probe, nil
}

func (e *fakeCraftRunViewContainerEngine) ObserveRuntimeProbe(_ context.Context, _ string) (CraftRunViewRuntimeProbe, bool, error) {
	return CraftRunViewRuntimeProbe{}, false, unresolvedCraftRunView("test engine cannot reconstruct probe output read-only", nil)
}

func (e *fakeCraftRunViewContainerEngine) SendRuntimeProbe(ctx context.Context, id string) (CraftRunViewRuntimeProbe, error) {
	probe, err := e.ProbeRuntime(ctx, id)
	if err != nil {
		return CraftRunViewRuntimeProbe{}, err
	}
	probe.ContainerID = id
	probe.ExecID = "exec-" + id
	return probe, nil
}

func (e *fakeCraftRunViewContainerEngine) factsForSpec(t *testing.T, config CraftRunViewContainerProviderConfig, spec CraftRunViewRuntimeContainerSpec) CraftRunViewEngineContainer {
	t.Helper()
	digest := sha256.Sum256([]byte(spec.Generation))
	short := hex.EncodeToString(digest[:16])
	root := filepath.Join(config.SandboxRoot, "rv-"+short)
	labels := map[string]string{
		craftRunViewLabelGeneration: hex.EncodeToString(digest[:]),
		craftRunViewLabelRuntime:    spec.RuntimeID,
		craftRunViewLabelContainer:  spec.ContainerID,
		craftRunViewLabelImage:      config.ImageDigest,
	}
	networkName := "rv-network-" + short
	return CraftRunViewEngineContainer{
		ID: "foreign-engine-id", Name: spec.ContainerID, State: "created",
		ImageID:        "test-image-id",
		ImageReference: config.ImageReference, ImageDigest: config.ImageDigest,
		User: craftRunViewContainerUser, WorkingDirectory: spec.Directory,
		Entrypoint:  []string{"/usr/local/bin/opencode"},
		Command:     []string{"serve", "--hostname", "0.0.0.0", "--port", "4096"},
		Environment: []string{"HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config", "XDG_DATA_HOME=/home/craft/.local/share", "XDG_STATE_HOME=/home/craft/.local/share/opencode", "XDG_CACHE_HOME=/home/craft/.local/share/opencode/cache", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Labels:      labels,
		Mounts: []CraftRunViewMount{
			{Type: "bind", Source: filepath.Join(root, "inputs"), Destination: path.Join(spec.Directory, "inputs"), ReadOnly: true},
			{Type: "bind", Source: filepath.Join(root, "knowledge"), Destination: path.Join(spec.Directory, "knowledge"), ReadOnly: true},
			{Type: "bind", Source: filepath.Join(root, "output"), Destination: path.Join(spec.Directory, "output")},
			{Type: "bind", Source: filepath.Join(root, "home-data"), Destination: "/home/craft/.local/share/opencode"},
			{Type: "bind", Source: filepath.Join(root, "home-config"), Destination: "/home/craft/.config/opencode"},
			{Type: "tmpfs", Destination: "/tmp", Options: []string{"mode=1777", fmt.Sprintf("size=%d", craftRunViewTmpfsBytes)}},
		},
		Networks: []CraftRunViewNetworkAttachment{{Name: networkName, ID: "network-" + networkName, Internal: true, IP: "172.30.0.2"}}, NetworkMode: networkName,
		PublishedPorts: []string{}, Privileged: false, ReadonlyRootfs: true, NoNewPrivileges: true,
		CapDrop: []string{"ALL"}, MemoryBytes: craftRunViewMemoryBytes, PidsLimit: craftRunViewPidsLimit,
	}
}

func (e *fakeCraftRunViewContainerEngine) seedContainer(name string, facts CraftRunViewEngineContainer, expectedNetwork CraftRunViewContainerNetworkSpec) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.containers[name] = cloneCraftRunViewContainer(facts)
	e.probes[facts.ID] = e.validProbe()
	e.networks[expectedNetwork.Name] = CraftRunViewContainerNetwork{ID: "network-" + expectedNetwork.Name, Name: expectedNetwork.Name, Driver: expectedNetwork.Driver, Internal: expectedNetwork.Internal, Labels: cloneStringMap(expectedNetwork.Labels)}
}

func (e *fakeCraftRunViewContainerEngine) removeContainer(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.containers, name)
}

func (e *fakeCraftRunViewContainerEngine) replaceContainer(name string, facts CraftRunViewEngineContainer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.containers[name] = cloneCraftRunViewContainer(facts)
	e.probes[facts.ID] = e.validProbe()
}

func (e *fakeCraftRunViewContainerEngine) setState(name, state string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	container := e.containers[name]
	container.State = state
	e.containers[name] = container
}

func (e *fakeCraftRunViewContainerEngine) setProbe(probe CraftRunViewRuntimeProbe) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id := range e.probes {
		e.probes[id] = probe
	}
}

func (e *fakeCraftRunViewContainerEngine) validProbe() CraftRunViewRuntimeProbe {
	return CraftRunViewRuntimeProbe{OpenCodeVersion: "1.18.4", BinarySHA256: strings.Repeat("b", 64), RuntimeConfigSHA256: strings.Repeat("c", 64)}
}

func (e *fakeCraftRunViewContainerEngine) createCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.createRequests)
}

func (e *fakeCraftRunViewContainerEngine) startCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.startedIDs)
}

func (e *fakeCraftRunViewContainerEngine) containerSnapshot(name string) CraftRunViewEngineContainer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return cloneCraftRunViewContainer(e.containers[name])
}

func cloneCraftRunViewNetwork(value CraftRunViewContainerNetwork) CraftRunViewContainerNetwork {
	value.Labels = cloneStringMap(value.Labels)
	return value
}

func cloneCraftRunViewContainer(value CraftRunViewEngineContainer) CraftRunViewEngineContainer {
	value.Entrypoint = append([]string(nil), value.Entrypoint...)
	value.Command = append([]string(nil), value.Command...)
	value.Environment = append([]string(nil), value.Environment...)
	value.Labels = cloneStringMap(value.Labels)
	value.Mounts = cloneCraftRunViewMounts(value.Mounts)
	value.Networks = append([]CraftRunViewNetworkAttachment(nil), value.Networks...)
	value.PublishedPorts = append([]string(nil), value.PublishedPorts...)
	value.CapDrop = append([]string(nil), value.CapDrop...)
	value.CapAdd = append([]string(nil), value.CapAdd...)
	value.Devices = append([]string(nil), value.Devices...)
	return value
}

func cloneCraftRunViewMounts(value []CraftRunViewMount) []CraftRunViewMount {
	result := make([]CraftRunViewMount, len(value))
	copy(result, value)
	for index := range result {
		result[index].Options = append([]string(nil), result[index].Options...)
	}
	return result
}

func cloneCraftRunViewRequest(value CraftRunViewContainerCreateRequest) CraftRunViewContainerCreateRequest {
	value.Entrypoint = append([]string(nil), value.Entrypoint...)
	value.Command = append([]string(nil), value.Command...)
	value.Environment = append([]string(nil), value.Environment...)
	value.Labels = cloneStringMap(value.Labels)
	value.Mounts = cloneCraftRunViewMounts(value.Mounts)
	value.PublishedPorts = append([]string(nil), value.PublishedPorts...)
	value.CapDrop = append([]string(nil), value.CapDrop...)
	value.CapAdd = append([]string(nil), value.CapAdd...)
	return value
}

func cloneStringMap(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

type fakeCraftRunViewSessionAPI struct {
	mu          sync.Mutex
	sessions    []opencode.SessionInfo
	listErr     error
	getErr      error
	getOverride *opencode.SessionInfo
	gets        int
	createErr   error
	creates     int
}

func (a *fakeCraftRunViewSessionAPI) ListSessions(context.Context) ([]opencode.SessionInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listErr != nil {
		return nil, a.listErr
	}
	return append([]opencode.SessionInfo(nil), a.sessions...), nil
}

func (a *fakeCraftRunViewSessionAPI) CreateSession(context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if a.createErr != nil {
		return "", a.createErr
	}
	return "ses_0123456789ab0123456789ABCD", nil
}

func (a *fakeCraftRunViewSessionAPI) GetSession(_ context.Context, id string) (opencode.SessionInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gets++
	if a.getErr != nil {
		return opencode.SessionInfo{}, a.getErr
	}
	if a.getOverride != nil {
		return *a.getOverride, nil
	}
	for _, session := range a.sessions {
		if session.ID == id {
			return session, nil
		}
	}
	return opencode.SessionInfo{}, errors.New("session not found")
}

func (a *fakeCraftRunViewSessionAPI) getCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.gets
}

func (a *fakeCraftRunViewSessionAPI) createCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.creates
}

func newRVTestProviderWithSessionAPI(t *testing.T, engine *fakeCraftRunViewContainerEngine) (*CraftRunViewContainerProvider, *fakeCraftRunViewSessionAPI) {
	t.Helper()
	api := &fakeCraftRunViewSessionAPI{}
	provider, err := newCraftRunViewContainerProvider(rvTestConfig(t), engine, func(string, string, string) (craftRunViewSessionAPI, error) {
		return api, nil
	})
	require.NoError(t, err)
	return provider, api
}

func rvTestOpenCodeSession(id, directory, projectID string) opencode.SessionInfo {
	return opencode.SessionInfo{ID: id, ProjectID: projectID, Location: opencode.SessionLocation{Directory: directory}}
}

func TestCraftRunViewMaterialHandleBindsVerifiedGenerationAndRun(t *testing.T) {
	const sessionID = "ses_0123456789ab0123456789ABCD"
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	keyA := craft.RunViewKey{TenantID: 7, OwnerID: "owner", SessionID: "task-session", RunID: "run-a"}
	keyB := keyA
	keyB.RunID = "run-b"

	containerA, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec("generation-a"))
	require.NoError(t, err)
	containerB, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec("generation-b"))
	require.NoError(t, err)
	api.sessions = []opencode.SessionInfo{
		rvTestOpenCodeSession(sessionID, containerA.Directory, containerA.ProjectID),
	}

	viewA := boundMaterialTestView(keyA, containerA, "generation-a", sessionID)
	storeA := &materialTestStore{view: viewA}
	handleA := CraftRunViewRuntimeHandle{View: viewA, Directory: containerA.Directory}
	materialA, err := provider.MaterialHandle(context.Background(), storeA, keyA, handleA)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(materialA.root, filepath.Join("craft-runviews", "rv-"+shortGeneration("generation-a"))))
	require.Equal(t, materialA.root, filepath.Dir(materialA.inputs))
	require.Equal(t, filepath.Join(materialA.root, "knowledge"), materialA.knowledge)
	require.Equal(t, filepath.Join(materialA.root, "output"), materialA.output)

	sessionB := "ses_0123456789ab0123456789ABCE"
	api.sessions = append(api.sessions, rvTestOpenCodeSession(sessionB, containerB.Directory, containerB.ProjectID))
	viewB := boundMaterialTestView(keyB, containerB, "generation-b", sessionB)
	storeB := &materialTestStore{view: viewB}
	handleB := CraftRunViewRuntimeHandle{View: viewB, Directory: containerB.Directory}
	materialB, err := provider.MaterialHandle(context.Background(), storeB, keyB, handleB)
	require.NoError(t, err)
	require.NotEqual(t, materialA.root, materialB.root, "same Task's Runs must never share a material root")
	_, err = provider.MaterialHandle(context.Background(), storeB, keyB, handleA)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved, "a valid Run A handle cannot be used for Run B")
}

func TestCraftRunViewMaterialHandleRejectsRecreatedOrMissingBoundDirectories(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, craftRunViewHostLayout)
	}{
		{name: "replaced input child", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			replaceDirectoryAtSamePath(t, layout.inputs, 0755)
		}},
		{name: "replaced knowledge child", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			replaceDirectoryAtSamePath(t, layout.knowledge, 0755)
		}},
		{name: "replaced output child", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			replaceDirectoryAtSamePath(t, layout.output, 0777)
		}},
		{name: "missing input child", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			require.NoError(t, os.RemoveAll(layout.inputs))
		}},
		{name: "replaced generation root", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			require.NoError(t, os.Rename(layout.root, layout.root+".old"))
			require.NoError(t, os.Mkdir(layout.root, 0700))
		}},
		{name: "missing generation root", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			require.NoError(t, os.RemoveAll(layout.root))
		}},
		{name: "tampered layout identity", mutate: func(t *testing.T, layout craftRunViewHostLayout) {
			identity := filepath.Join(layout.root, ".layout-identity.json")
			require.FileExists(t, identity)
			require.NoError(t, os.WriteFile(identity, []byte(`{"version":999}`), 0600))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider, engine, api, store, runtime, layout := newMaterialHandleFixture(t, "generation-layout-test")
			before, statErr := os.Stat(layout.inputs)
			require.NoError(t, statErr)
			tc.mutate(t, layout)
			_, err := provider.MaterialHandle(context.Background(), store, store.view.Key, runtime)
			require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
			if tc.name == "missing input child" || tc.name == "missing generation root" {
				_, statErr = os.Lstat(layout.inputs)
				require.ErrorIs(t, statErr, os.ErrNotExist, "bound verification must not recreate missing paths")
			}
			if tc.name == "replaced input child" {
				after, err := os.Stat(layout.inputs)
				require.NoError(t, err)
				require.False(t, os.SameFile(before, after), "replacement inode must remain rejected")
			}
			if tc.name == "replaced generation root" {
				_, statErr = os.Lstat(layout.identity)
				require.ErrorIs(t, statErr, os.ErrNotExist, "replacement root must not receive a regenerated identity")
			}
			_ = engine
			_ = api
		})
	}
}

func TestCraftRunViewMaterialLayoutIdentityIsDurableBeforeCreateAndAcrossRestart(t *testing.T) {
	engine := newFakeCraftRunViewContainerEngine()
	config := rvTestConfig(t)
	var firstAPI *fakeCraftRunViewSessionAPI
	provider, err := newCraftRunViewContainerProvider(config, engine, func(string, string, string) (craftRunViewSessionAPI, error) {
		firstAPI = &fakeCraftRunViewSessionAPI{}
		return firstAPI, nil
	})
	require.NoError(t, err)
	spec := rvTestSpec("generation-durable-identity")
	layout := generationLayoutPaths(config.SandboxRoot, spec)
	engine.beforeCreate = func(CraftRunViewContainerCreateRequest) {
		info, err := os.Lstat(layout.identity)
		require.NoError(t, err, "layout identity must exist before Docker Create")
		require.True(t, info.Mode().IsRegular())
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		contents, err := os.ReadFile(layout.identity)
		require.NoError(t, err)
		require.Contains(t, string(contents), spec.Generation)
	}
	container, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	require.NotNil(t, firstAPI)

	secondAPI := &fakeCraftRunViewSessionAPI{}
	restarted, err := newCraftRunViewContainerProvider(config, engine, func(string, string, string) (craftRunViewSessionAPI, error) {
		return secondAPI, nil
	})
	require.NoError(t, err)
	resumed, err := restarted.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, container, resumed)
	require.Equal(t, 1, engine.createCount(), "restart must verify the durable identity without another Create")
	const sessionID = "ses_0123456789ab0123456789ABCD"
	secondAPI.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, resumed.Directory, resumed.ProjectID)}
	key := craft.RunViewKey{TenantID: 7, OwnerID: "owner", SessionID: "task-session", RunID: "run-after-restart"}
	view := boundMaterialTestView(key, resumed, spec.Generation, sessionID)
	material, err := restarted.MaterialHandle(context.Background(), &materialTestStore{view: view}, key, CraftRunViewRuntimeHandle{View: view, Directory: resumed.Directory})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(restarted.config.SandboxRoot, filepath.Base(layout.root), "inputs"), material.inputs)

	replaceDirectoryAtSamePath(t, layout.inputs, 0755)
	replacement, err := os.Stat(layout.inputs)
	require.NoError(t, err)
	thirdProvider, err := newCraftRunViewContainerProvider(config, engine, func(string, string, string) (craftRunViewSessionAPI, error) {
		return &fakeCraftRunViewSessionAPI{}, nil
	})
	require.NoError(t, err)
	_, err = thirdProvider.InspectOrCreateContainer(context.Background(), spec)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved, "restart must reject a replacement inode")
	afterFailure, statErr := os.Stat(layout.inputs)
	require.NoError(t, statErr)
	require.True(t, os.SameFile(replacement, afterFailure), "failed recovery must not replace or bless the new inode")
}

func TestCraftRunViewMaterialHandleRejectsForgedStaleAndUnsafeBindings(t *testing.T) {
	const sessionID = "ses_0123456789ab0123456789ABCD"
	tests := []struct {
		name   string
		mutate func(*testing.T, *CraftRunViewContainerProvider, *fakeCraftRunViewContainerEngine, *fakeCraftRunViewSessionAPI, *materialTestStore, craft.RunViewKey, *CraftRunViewRuntimeHandle)
	}{
		{name: "forged generation", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.View.Generation = "forged"
		}},
		{name: "forged container", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.View.Runtime.ContainerID = "rv-container-forged"
		}},
		{name: "forged session", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.View.Runtime.OpenCodeSessionID = "ses_0123456789ab0123456789ABCDEE"
		}},
		{name: "forged directory", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.Directory = "/workspace/foreign"
		}},
		{name: "unbound runtime", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.View.State = craft.RunViewStateAllocating
			h.View.Runtime = craft.RunViewRuntime{}
		}},
		{name: "stale persisted session", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, store *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			store.view.Runtime.OpenCodeSessionID = "ses_0123456789ab0123456789ABCDEE"
		}},
		{name: "missing container", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, engine *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			engine.removeContainer(rvTestSpec("generation-a").ContainerID)
		}},
		{name: "changed mount", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, engine *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			facts := engine.containerSnapshot(rvTestSpec("generation-a").ContainerID)
			facts.Mounts[0].Source = "/tmp/foreign"
			engine.replaceContainer(rvTestSpec("generation-a").ContainerID, facts)
		}},
		{name: "changed network", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, engine *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			facts := engine.containerSnapshot(rvTestSpec("generation-a").ContainerID)
			facts.Networks[0].Name = "shared"
			engine.replaceContainer(rvTestSpec("generation-a").ContainerID, facts)
		}},
		{name: "changed image", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, engine *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			facts := engine.containerSnapshot(rvTestSpec("generation-a").ContainerID)
			facts.ImageDigest = "sha256:" + strings.Repeat("d", 64)
			engine.replaceContainer(rvTestSpec("generation-a").ContainerID, facts)
		}},
		{name: "changed runtime probe", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, engine *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			engine.setProbe(CraftRunViewRuntimeProbe{OpenCodeVersion: "1.18.4", BinarySHA256: strings.Repeat("d", 64), RuntimeConfigSHA256: strings.Repeat("c", 64)})
		}},
		{name: "symlinked child", mutate: func(t *testing.T, provider *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			layout, err := provider.prepareGenerationLayout(rvTestSpec("generation-a"))
			require.NoError(t, err)
			require.NoError(t, os.RemoveAll(layout.inputs))
			target := t.TempDir()
			require.NoError(t, os.Symlink(target, layout.inputs))
		}},
		{name: "wrong persisted scope", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, _ *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, h *CraftRunViewRuntimeHandle) {
			h.View.Key.RunID = "run-other"
		}},
		{name: "session GET error", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, api *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			api.getErr = errors.New("session endpoint unavailable")
		}},
		{name: "session GET missing", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, api *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			api.sessions = nil
		}},
		{name: "session GET ID changed", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, api *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			api.getOverride = ptrSession(rvTestOpenCodeSession("ses_0123456789ab0123456789ABCE", rvTestSpec("generation-a").Directory, rvTestProject))
		}},
		{name: "session GET project changed", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, api *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			api.getOverride = ptrSession(rvTestOpenCodeSession("ses_0123456789ab0123456789ABCD", rvTestSpec("generation-a").Directory, "foreign-project"))
		}},
		{name: "session GET directory changed", mutate: func(_ *testing.T, _ *CraftRunViewContainerProvider, _ *fakeCraftRunViewContainerEngine, api *fakeCraftRunViewSessionAPI, _ *materialTestStore, _ craft.RunViewKey, _ *CraftRunViewRuntimeHandle) {
			api.getOverride = ptrSession(rvTestOpenCodeSession("ses_0123456789ab0123456789ABCD", "/workspace/foreign", rvTestProject))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newFakeCraftRunViewContainerEngine()
			provider, api := newRVTestProviderWithSessionAPI(t, engine)
			container, err := provider.InspectOrCreateContainer(context.Background(), rvTestSpec("generation-a"))
			require.NoError(t, err)
			api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, container.Directory, container.ProjectID)}
			key := craft.RunViewKey{TenantID: 7, OwnerID: "owner", SessionID: "task-session", RunID: "run-a"}
			view := boundMaterialTestView(key, container, "generation-a", sessionID)
			store := &materialTestStore{view: view}
			runtime := CraftRunViewRuntimeHandle{View: view, Directory: container.Directory}
			tc.mutate(t, provider, engine, api, store, key, &runtime)
			_, err = provider.MaterialHandle(context.Background(), store, key, runtime)
			require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		})
	}
}

func newMaterialHandleFixture(t *testing.T, generation string) (*CraftRunViewContainerProvider, *fakeCraftRunViewContainerEngine, *fakeCraftRunViewSessionAPI, *materialTestStore, CraftRunViewRuntimeHandle, craftRunViewHostLayout) {
	t.Helper()
	const sessionID = "ses_0123456789ab0123456789ABCD"
	engine := newFakeCraftRunViewContainerEngine()
	provider, api := newRVTestProviderWithSessionAPI(t, engine)
	spec := rvTestSpec(generation)
	container, err := provider.InspectOrCreateContainer(context.Background(), spec)
	require.NoError(t, err)
	api.sessions = []opencode.SessionInfo{rvTestOpenCodeSession(sessionID, container.Directory, container.ProjectID)}
	key := craft.RunViewKey{TenantID: 7, OwnerID: "owner", SessionID: "task-session", RunID: "run-layout"}
	view := boundMaterialTestView(key, container, generation, sessionID)
	store := &materialTestStore{view: view}
	runtime := CraftRunViewRuntimeHandle{View: view, Directory: container.Directory}
	config := provider.config
	return provider, engine, api, store, runtime, generationLayoutPaths(config.SandboxRoot, spec)
}

func replaceDirectoryAtSamePath(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	old := path + ".old"
	require.NoError(t, os.Rename(path, old))
	require.NoError(t, os.Mkdir(path, mode))
}

func boundMaterialTestView(key craft.RunViewKey, container CraftRunViewRuntimeContainer, generation, sessionID string) craft.RunView {
	intent := time.Now().UTC()
	return craft.RunView{
		Key: key, Generation: generation, State: craft.RunViewStateBound,
		SessionCreateIntentAt: &intent,
		Runtime:               craft.RunViewRuntime{RuntimeID: container.RuntimeID, ContainerID: container.ContainerID, OpenCodeSessionID: sessionID},
	}
}

func shortGeneration(generation string) string {
	sum := sha256.Sum256([]byte(generation))
	return hex.EncodeToString(sum[:16])
}

type materialTestStore struct{ view craft.RunView }

func (s *materialTestStore) Allocate(context.Context, craft.RunViewKey) (craft.RunView, error) {
	return s.view, nil
}
func (s *materialTestStore) Load(_ context.Context, key craft.RunViewKey) (craft.RunView, error) {
	if key != s.view.Key {
		return craft.RunView{}, craft.ErrNotFound
	}
	return s.view, nil
}
func (s *materialTestStore) BeginSessionCreate(_ context.Context, key craft.RunViewKey, generation string) (craft.RunView, bool, error) {
	// Mirrors the durable CraftRunViewStore semantics the admitted coordinator
	// relies on: one create intent per allocating generation, idempotent once
	// claimed, never granted for an unknown key/generation.
	if key != s.view.Key || generation != s.view.Generation {
		return craft.RunView{}, false, craft.ErrConflict
	}
	if s.view.State != craft.RunViewStateAllocating && s.view.State != craft.RunViewStateBound {
		return craft.RunView{}, false, craft.ErrConflict
	}
	maySend := s.view.SessionCreateIntentAt == nil && s.view.State == craft.RunViewStateAllocating
	if maySend {
		now := time.Now().UTC()
		s.view.SessionCreateIntentAt = &now
	}
	return s.view, maySend, nil
}
func (s *materialTestStore) BindRuntime(_ context.Context, key craft.RunViewKey, generation string, runtime craft.RunViewRuntime) (craft.RunView, error) {
	// Mirrors the durable CAS: an allocating generation with a claimed intent
	// binds exactly once; a bound generation replays only the same runtime.
	if key != s.view.Key || generation != s.view.Generation {
		return craft.RunView{}, craft.ErrConflict
	}
	switch s.view.State {
	case craft.RunViewStateAllocating:
		if s.view.SessionCreateIntentAt == nil {
			return craft.RunView{}, craft.ErrConflict
		}
		s.view.State = craft.RunViewStateBound
		s.view.Runtime = runtime
	case craft.RunViewStateBound:
		if s.view.Runtime != runtime {
			return craft.RunView{}, craft.ErrConflict
		}
	default:
		return craft.RunView{}, craft.ErrConflict
	}
	return s.view, nil
}
