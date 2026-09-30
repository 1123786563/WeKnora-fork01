package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
)

const (
	craftRunViewLabelGeneration = "io.weknora.craft.runview.generation-sha256"
	craftRunViewLabelRuntime    = "io.weknora.craft.runview.runtime-id"
	craftRunViewLabelContainer  = "io.weknora.craft.runview.container-id"
	craftRunViewLabelImage      = "io.weknora.craft.runview.image-digest"

	craftRunViewOpenCodeVersion    = "1.18.4"
	craftRunViewContainerUser      = "10001:10001"
	craftRunViewOpenCodePort       = 4096
	craftRunViewMemoryBytes        = int64(1 << 30)
	craftRunViewPidsLimit          = int64(256)
	craftRunViewTmpfsBytes         = int64(256 << 20)
	craftRunViewCreateMarker       = ".container-create-attempted"
	craftRunViewLayoutIdentityFile = ".layout-identity.json"
)

// CraftRunViewContainerProviderConfig contains server-owned runtime pins and
// the dedicated host volume. An empty image digest is rejected; production
// construction remains unavailable while the checked-in lock has no digest.
type CraftRunViewContainerProviderConfig struct {
	SandboxRoot          string
	ImageReference       string
	ImageDigest          string
	OpenCodeBinarySHA256 string
	RuntimeConfigSHA256  string
	OpenCodeVersion      string
	ProjectID            string
}

// CraftRunViewContainerNetworkSpec asks the engine for one private network
// dedicated to a single RunView generation.
type CraftRunViewContainerNetworkSpec struct {
	Name     string
	Driver   string
	Internal bool
	Labels   map[string]string
}

// CraftRunViewContainerNetwork is inspected network evidence from the engine.
type CraftRunViewContainerNetwork struct {
	ID       string
	Name     string
	Driver   string
	Internal bool
	Labels   map[string]string
}

// CraftRunViewMount is an inspected/configured mount. Bind mount sources are
// always under one host-owned opaque generation root; tmpfs has no host source.
type CraftRunViewMount struct {
	Type        string
	Source      string
	Destination string
	ReadOnly    bool
	Options     []string
}

// CraftRunViewNetworkAttachment is one actual inspected network attachment.
// The engine adapter must report the complete set so the provider can prove
// that a generation container has no second shared network path.
type CraftRunViewNetworkAttachment struct {
	Name     string
	ID       string
	Internal bool
	IP       string
}

// CraftRunViewContainerCreateRequest is the exact desired immutable container
// configuration sent to the injected engine adapter.
type CraftRunViewContainerCreateRequest struct {
	Name             string
	ImageReference   string
	WorkingDirectory string
	Directory        string
	User             string
	Entrypoint       []string
	Command          []string
	Environment      []string
	Labels           map[string]string
	Mounts           []CraftRunViewMount
	NetworkMode      string
	PublishedPorts   []string
	Privileged       bool
	ReadonlyRootfs   bool
	NoNewPrivileges  bool
	CapDrop          []string
	CapAdd           []string
	MemoryBytes      int64
	PidsLimit        int64
}

// CraftRunViewEngineContainer is the actual container inspection projection
// required before the provider can assert identity, isolation, or readiness.
type CraftRunViewEngineContainer struct {
	ID               string
	Name             string
	State            string
	ImageReference   string
	ImageDigest      string
	User             string
	WorkingDirectory string
	Entrypoint       []string
	Command          []string
	Environment      []string
	Labels           map[string]string
	Mounts           []CraftRunViewMount
	Networks         []CraftRunViewNetworkAttachment
	NetworkMode      string
	PublishedPorts   []string
	Privileged       bool
	ReadonlyRootfs   bool
	NoNewPrivileges  bool
	CapDrop          []string
	CapAdd           []string
	MemoryBytes      int64
	PidsLimit        int64
	HostPID          bool
	HostIPC          bool
	HostUTS          bool
	Devices          []string
}

// CraftRunViewRuntimeProbe is collected from inside the running container by
// the engine adapter. It must report the actual binary and baked config, not
// request flags or OpenCode's static /doc version.
type CraftRunViewRuntimeProbe struct {
	OpenCodeVersion     string
	BinarySHA256        string
	RuntimeConfigSHA256 string
}

// CraftRunViewContainerEngine is the narrowly injectable container engine
// seam. InspectContainer returns (nil, nil) only for definitive absence; an
// error means absence/state is uncertain. The adapter must fill inspection and
// probe fields from real engine/exec observations.
type CraftRunViewContainerEngine interface {
	EnsurePrivateNetwork(context.Context, CraftRunViewContainerNetworkSpec) (CraftRunViewContainerNetwork, error)
	InspectContainer(context.Context, string) (*CraftRunViewEngineContainer, error)
	CreateContainer(context.Context, CraftRunViewContainerCreateRequest) (string, error)
	StartContainer(context.Context, string) error
	ProbeRuntime(context.Context, string) (CraftRunViewRuntimeProbe, error)
}

type craftRunViewSessionAPI interface {
	ListSessions(context.Context) ([]opencode.SessionInfo, error)
	GetSession(context.Context, string) (opencode.SessionInfo, error)
	CreateSession(context.Context) (string, error)
}

type craftRunViewSessionAPIFactory func(endpoint, directory, projectID string) (craftRunViewSessionAPI, error)

type craftRunViewOpenCodeSessionAPI struct {
	client    *opencode.Client
	inventory *opencode.Inventory
}

func (a *craftRunViewOpenCodeSessionAPI) ListSessions(ctx context.Context) ([]opencode.SessionInfo, error) {
	return a.inventory.ListSessions(ctx)
}

func (a *craftRunViewOpenCodeSessionAPI) GetSession(ctx context.Context, id string) (opencode.SessionInfo, error) {
	return a.inventory.GetSession(ctx, id)
}

func (a *craftRunViewOpenCodeSessionAPI) CreateSession(ctx context.Context) (string, error) {
	return a.client.CreateSession(ctx)
}

type craftRunViewContainerBinding struct {
	spec      CraftRunViewRuntimeContainerSpec
	engineID  string
	network   CraftRunViewContainerNetwork
	endpoint  string
	projectID string
	sessions  craftRunViewSessionAPI
}

type craftRunViewKeyedLock struct {
	mu   sync.Mutex
	refs int
}

// CraftRunViewContainerProvider implements the coordinator's provider
// contract using a server-derived generation name and inspected container
// evidence. It is not assembled into production runtime wiring by this task.
type CraftRunViewContainerProvider struct {
	config     CraftRunViewContainerProviderConfig
	engine     CraftRunViewContainerEngine
	apiFactory craftRunViewSessionAPIFactory

	mu       sync.Mutex
	locks    map[string]*craftRunViewKeyedLock
	bindings map[string]craftRunViewContainerBinding
}

// NewCraftRunViewContainerProvider refuses unpinned images and incomplete
// runtime integrity pins. The live deployment must supply the digest only
// after building and verifying the pinned Linux image.
func NewCraftRunViewContainerProvider(
	config CraftRunViewContainerProviderConfig,
	engine CraftRunViewContainerEngine,
) (*CraftRunViewContainerProvider, error) {
	return newCraftRunViewContainerProvider(config, engine, newCraftRunViewSessionAPI)
}

func newCraftRunViewContainerProvider(
	config CraftRunViewContainerProviderConfig,
	engine CraftRunViewContainerEngine,
	apiFactory craftRunViewSessionAPIFactory,
) (*CraftRunViewContainerProvider, error) {
	if engine == nil || apiFactory == nil {
		return nil, unresolvedCraftRunView("container engine or session API factory is missing", nil)
	}
	if config.OpenCodeVersion != craftRunViewOpenCodeVersion {
		return nil, unresolvedCraftRunView("OpenCode version is not the pinned release", nil)
	}
	if !validCraftRunViewToken(config.ProjectID) {
		return nil, unresolvedCraftRunView("expected project ID is invalid", nil)
	}
	if !validSHA256Digest(config.ImageDigest) || !validSHA256Hex(config.OpenCodeBinarySHA256) || !validSHA256Hex(config.RuntimeConfigSHA256) {
		return nil, unresolvedCraftRunView("image, binary, or runtime config digest is missing or malformed", nil)
	}
	if !strings.HasSuffix(config.ImageReference, "@"+config.ImageDigest) {
		return nil, unresolvedCraftRunView("image reference is not pinned to the expected digest", nil)
	}
	root, err := prepareSandboxRoot(config.SandboxRoot)
	if err != nil {
		return nil, unresolvedCraftRunView("prepare dedicated sandbox root", err)
	}
	config.SandboxRoot = root
	return &CraftRunViewContainerProvider{
		config: config, engine: engine, apiFactory: apiFactory,
		locks: make(map[string]*craftRunViewKeyedLock), bindings: make(map[string]craftRunViewContainerBinding),
	}, nil
}

func newCraftRunViewSessionAPI(endpoint, directory, projectID string) (craftRunViewSessionAPI, error) {
	client, err := opencode.NewClient(endpoint, nil)
	if err != nil {
		return nil, err
	}
	client, err = client.WithDirectory(directory)
	if err != nil {
		return nil, err
	}
	inventory, err := opencode.NewInventory(client, directory, projectID)
	if err != nil {
		return nil, err
	}
	return &craftRunViewOpenCodeSessionAPI{client: client, inventory: inventory}, nil
}

// InspectOrCreateContainer inspects the exact deterministic name first. It
// creates only after definitive absence and a durable filesystem create
// marker wins O_EXCL. If create may have happened but the engine now reports
// absence, the marker prevents a replacement container for this generation.
func (p *CraftRunViewContainerProvider) InspectOrCreateContainer(
	ctx context.Context,
	spec CraftRunViewRuntimeContainerSpec,
) (CraftRunViewRuntimeContainer, error) {
	if p == nil || p.engine == nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("provider is not initialized", nil)
	}
	unlock := p.lockGeneration(spec.ContainerID)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("container operation canceled", err)
	}
	if err := p.validateSpec(spec); err != nil {
		return CraftRunViewRuntimeContainer{}, err
	}
	layout := generationLayoutPaths(p.config.SandboxRoot, spec)
	networkSpec := p.networkSpec(spec)
	network, err := p.engine.EnsurePrivateNetwork(ctx, networkSpec)
	if err != nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("ensure private generation network", err)
	}
	if err := validateNetwork(networkSpec, network); err != nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("network identity is not private and generation-scoped", err)
	}

	facts, err := p.engine.InspectContainer(ctx, spec.ContainerID)
	if err != nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("inspect deterministic container", err)
	}
	markerExists, err := p.checkCreateMarker(layout, spec)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("validate container create marker", err)
	}
	if facts == nil {
		if markerExists {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("container is missing after a prior create attempt", nil)
		}
		layout, err = p.prepareGenerationLayout(spec)
		if err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("prepare first generation layout", err)
		}
		if marker, markerErr := p.checkCreateMarker(layout, spec); marker || (markerErr != nil && !errors.Is(markerErr, os.ErrNotExist)) {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("generation create intent appeared during layout preparation", markerErr)
		}
		created, err := p.createMarker(layout, spec)
		if err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("claim one-shot container create attempt", err)
		}
		if !created {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("another provider owns the container create attempt", nil)
		}
		markerExists = true
		if err := verifyGenerationLayout(layout, spec); err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("generation layout identity changed before container create", err)
		}
		request := p.createRequest(spec, networkSpec.Name, layout)
		engineID, createErr := p.engine.CreateContainer(ctx, request)
		if createErr != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("container create outcome is unknown", createErr)
		}
		if engineID == "" {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("engine returned an empty container ID", nil)
		}
		facts, err = p.engine.InspectContainer(ctx, spec.ContainerID)
		if err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("inspect created container", err)
		}
		if facts == nil || facts.ID != engineID {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("created container cannot be reconciled by deterministic name", nil)
		}
	} else {
		if !markerExists || err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("existing container has no valid durable create marker", err)
		}
		if err := verifyGenerationLayout(layout, spec); err != nil {
			return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("existing container generation layout identity is missing or changed", err)
		}
	}
	if !markerExists {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("deterministic container has no durable create marker", nil)
	}
	if err := verifyGenerationLayout(layout, spec); err != nil {
		return CraftRunViewRuntimeContainer{}, unresolvedCraftRunView("generation layout identity changed before runtime start", err)
	}
	binding, err := p.startAndVerify(ctx, spec, network, layout, facts)
	if err != nil {
		return CraftRunViewRuntimeContainer{}, err
	}
	p.mu.Lock()
	p.bindings[spec.ContainerID] = binding
	p.mu.Unlock()
	return runtimeContainerFromBinding(binding), nil
}

func (p *CraftRunViewContainerProvider) FindSessions(
	ctx context.Context,
	container CraftRunViewRuntimeContainer,
) (CraftRunViewRuntimeInventory, error) {
	binding, err := p.currentBinding(ctx, container)
	if err != nil {
		return CraftRunViewRuntimeInventory{}, err
	}
	sessions, err := binding.sessions.ListSessions(ctx)
	if err != nil {
		return CraftRunViewRuntimeInventory{}, unresolvedCraftRunView("OpenCode session inventory is incomplete or unavailable", err)
	}
	out := make([]CraftRunViewRuntimeSession, 0, len(sessions))
	for _, session := range sessions {
		if !validPinnedOpenCodeSessionID(session.ID) || session.ProjectID != container.ProjectID || session.Location.Directory != container.Directory {
			return CraftRunViewRuntimeInventory{}, unresolvedCraftRunView("OpenCode inventory contains foreign session metadata", nil)
		}
		out = append(out, CraftRunViewRuntimeSession{ID: session.ID, ProjectID: session.ProjectID, Directory: session.Location.Directory})
	}
	if len(out) == 1 {
		candidate := out[0]
		confirmed, err := binding.sessions.GetSession(ctx, candidate.ID)
		if err != nil {
			return CraftRunViewRuntimeInventory{}, unresolvedCraftRunView("recheck unique OpenCode session by exact ID", err)
		}
		if confirmed.ID != candidate.ID || confirmed.ProjectID != candidate.ProjectID || confirmed.Location.Directory != candidate.Directory {
			return CraftRunViewRuntimeInventory{}, unresolvedCraftRunView("exact OpenCode session metadata differs from complete list", nil)
		}
	}
	return CraftRunViewRuntimeInventory{
		Authoritative: true, Complete: true, Directory: container.Directory,
		ProjectID: container.ProjectID, Sessions: out,
	}, nil
}

// CreateSession sends exactly one legacy create request. The coordinator is
// the only caller and records BeginSessionCreate intent before invoking this
// method; unknown outcomes are returned unresolved and must be reconciled by
// a complete FindSessions before another call can be considered.
func (p *CraftRunViewContainerProvider) CreateSession(
	ctx context.Context,
	container CraftRunViewRuntimeContainer,
	directory string,
) error {
	if directory == "" || directory != container.Directory {
		return unresolvedCraftRunView("session create directory differs from inspected generation", nil)
	}
	binding, err := p.currentBinding(ctx, container)
	if err != nil {
		return err
	}
	id, err := binding.sessions.CreateSession(ctx)
	if err != nil {
		return unresolvedCraftRunView("OpenCode CreateSession outcome is unknown", err)
	}
	if !validPinnedOpenCodeSessionID(id) {
		return unresolvedCraftRunView("OpenCode CreateSession returned malformed session ID", nil)
	}
	return nil
}

func (p *CraftRunViewContainerProvider) currentBinding(
	ctx context.Context,
	container CraftRunViewRuntimeContainer,
) (craftRunViewContainerBinding, error) {
	unlock := p.lockGeneration(container.ContainerID)
	defer unlock()
	p.mu.Lock()
	binding, ok := p.bindings[container.ContainerID]
	p.mu.Unlock()
	if !ok || !sameRuntimeIdentity(runtimeContainerFromBinding(binding), container) {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("container is not a verified current provider binding", nil)
	}
	if err := p.verifyCurrent(ctx, binding); err != nil {
		return craftRunViewContainerBinding{}, err
	}
	return binding, nil
}

func (p *CraftRunViewContainerProvider) verifyCurrent(ctx context.Context, binding craftRunViewContainerBinding) error {
	if err := ctx.Err(); err != nil {
		return unresolvedCraftRunView("container verification canceled", err)
	}
	layout := generationLayoutPaths(p.config.SandboxRoot, binding.spec)
	if err := verifyGenerationLayout(layout, binding.spec); err != nil {
		return unresolvedCraftRunView("generation layout identity changed", err)
	}
	if marker, err := p.checkCreateMarker(layout, binding.spec); err != nil || !marker {
		return unresolvedCraftRunView("container create marker is missing or changed", err)
	}
	networkSpec := p.networkSpec(binding.spec)
	network, err := p.engine.EnsurePrivateNetwork(ctx, networkSpec)
	if err != nil || validateNetwork(networkSpec, network) != nil || network.ID != binding.network.ID {
		return unresolvedCraftRunView("generation network changed", err)
	}
	facts, err := p.engine.InspectContainer(ctx, binding.spec.ContainerID)
	if err != nil || facts == nil {
		return unresolvedCraftRunView("verified generation container is missing or uncertain", err)
	}
	if err := p.validateContainerFacts(binding.spec, network, layout, *facts, true); err != nil {
		return unresolvedCraftRunView("generation container inspection changed", err)
	}
	if facts.ID != binding.engineID {
		return unresolvedCraftRunView("engine container identity changed", nil)
	}
	probe, err := p.engine.ProbeRuntime(ctx, facts.ID)
	if err != nil || !p.validProbe(probe) {
		return unresolvedCraftRunView("runtime binary or baked config is not verified", err)
	}
	return nil
}

func (p *CraftRunViewContainerProvider) startAndVerify(
	ctx context.Context,
	spec CraftRunViewRuntimeContainerSpec,
	network CraftRunViewContainerNetwork,
	layout craftRunViewHostLayout,
	facts *CraftRunViewEngineContainer,
) (craftRunViewContainerBinding, error) {
	if err := p.validateContainerFacts(spec, network, layout, *facts, false); err != nil {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("container inspection does not match immutable generation config", err)
	}
	if facts.State != "running" {
		if facts.State != "created" && facts.State != "exited" {
			return craftRunViewContainerBinding{}, unresolvedCraftRunView("container state cannot be safely resumed", nil)
		}
		if err := p.engine.StartContainer(ctx, facts.ID); err != nil {
			return craftRunViewContainerBinding{}, unresolvedCraftRunView("start existing generation container", err)
		}
	}
	current, err := p.engine.InspectContainer(ctx, spec.ContainerID)
	if err != nil || current == nil {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("inspect started generation container", err)
	}
	if current.ID != facts.ID || current.State != "running" {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("started container identity/state is uncertain", nil)
	}
	if err := p.validateContainerFacts(spec, network, layout, *current, true); err != nil {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("started container failed security inspection", err)
	}
	probe, err := p.engine.ProbeRuntime(ctx, current.ID)
	if err != nil || !p.validProbe(probe) {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("runtime version/binary/config probe failed", err)
	}
	endpoint, err := openCodeEndpoint(current.Networks[0].IP)
	if err != nil {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("private OpenCode endpoint is invalid", err)
	}
	api, err := p.apiFactory(endpoint, spec.Directory, p.config.ProjectID)
	if err != nil {
		return craftRunViewContainerBinding{}, unresolvedCraftRunView("create scoped OpenCode client", err)
	}
	return craftRunViewContainerBinding{spec: spec, engineID: current.ID, network: network, endpoint: endpoint, projectID: p.config.ProjectID, sessions: api}, nil
}

func (p *CraftRunViewContainerProvider) validateContainerFacts(
	spec CraftRunViewRuntimeContainerSpec,
	network CraftRunViewContainerNetwork,
	layout craftRunViewHostLayout,
	facts CraftRunViewEngineContainer,
	requireRunning bool,
) error {
	if facts.ID == "" || facts.Name != spec.ContainerID || facts.ImageReference != p.config.ImageReference || facts.ImageDigest != p.config.ImageDigest {
		return errors.New("container name or pinned image identity differs")
	}
	if !reflect.DeepEqual(facts.Labels, p.labels(spec)) {
		return errors.New("container labels differ from generation identity")
	}
	if facts.User != craftRunViewContainerUser || facts.WorkingDirectory != spec.Directory || facts.Privileged || !facts.ReadonlyRootfs || !facts.NoNewPrivileges {
		return errors.New("container user, working directory, or privilege policy differs")
	}
	if !sameStrings(facts.Entrypoint, []string{"/usr/local/bin/opencode"}) ||
		!sameStrings(facts.Command, []string{"serve", "--hostname", "0.0.0.0", "--port", fmt.Sprint(craftRunViewOpenCodePort)}) ||
		!safeCraftRunViewEnvironment(facts.Environment) {
		return errors.New("container OpenCode command or environment differs")
	}
	if len(facts.Networks) != 1 {
		return errors.New("container must have exactly one inspected network attachment")
	}
	attachment := facts.Networks[0]
	if attachment.Name != network.Name || !attachment.Internal {
		return errors.New("container is not attached exclusively to its private generation network")
	}
	// Docker does not allocate the network ID/IP in NetworkSettings until a
	// created container starts. Before start, bind that pending attachment to
	// the exact inspected HostConfig.NetworkMode; after start require the full
	// engine-issued attachment identity and private address.
	if requireRunning {
		if attachment.ID != network.ID || !isPrivateIP(attachment.IP) {
			return errors.New("running container network attachment identity differs")
		}
	} else if facts.NetworkMode != network.Name || (attachment.ID != "" && attachment.ID != network.ID) ||
		(attachment.IP != "" && !isPrivateIP(attachment.IP)) {
		return errors.New("created container network mode is not its private generation network")
	}
	if len(facts.PublishedPorts) != 0 || facts.HostPID || facts.HostIPC || facts.HostUTS || len(facts.Devices) != 0 {
		return errors.New("container publishes ports or shares host devices/namespaces")
	}
	if !sameStrings(facts.CapDrop, []string{"ALL"}) || len(facts.CapAdd) != 0 || facts.MemoryBytes != craftRunViewMemoryBytes || facts.PidsLimit != craftRunViewPidsLimit {
		return errors.New("container capability or resource limits differ")
	}
	if err := validateInspectedMounts(layout, spec, facts.Mounts); err != nil {
		return err
	}
	if requireRunning && facts.State != "running" {
		return errors.New("container is not running")
	}
	if !requireRunning && facts.State != "running" && facts.State != "created" && facts.State != "exited" {
		return errors.New("container state is not recoverable")
	}
	return nil
}

func validateInspectedMounts(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec, got []CraftRunViewMount) error {
	want := expectedCraftRunViewMounts(layout, spec)
	if len(got) != len(want) {
		return fmt.Errorf("container mount count %d differs from expected %d", len(got), len(want))
	}
	byDestination := make(map[string]CraftRunViewMount, len(got))
	for _, mount := range got {
		if _, exists := byDestination[mount.Destination]; exists {
			return fmt.Errorf("duplicate container mount destination %q", mount.Destination)
		}
		byDestination[mount.Destination] = mount
	}
	for _, expected := range want {
		actual, ok := byDestination[expected.Destination]
		if !ok || actual.Type != expected.Type || actual.Source != expected.Source || actual.ReadOnly != expected.ReadOnly || !sameStrings(actual.Options, expected.Options) {
			return fmt.Errorf("container mount %q differs from generation layout", expected.Destination)
		}
		if actual.Type == "bind" && !pathWithin(layout.root, actual.Source) {
			return fmt.Errorf("container mount %q escapes generation root", expected.Destination)
		}
	}
	return nil
}

func expectedCraftRunViewMounts(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) []CraftRunViewMount {
	return []CraftRunViewMount{
		{Type: "bind", Source: layout.inputs, Destination: path.Join(spec.Directory, "inputs"), ReadOnly: true},
		{Type: "bind", Source: layout.knowledge, Destination: path.Join(spec.Directory, "knowledge"), ReadOnly: true},
		{Type: "bind", Source: layout.output, Destination: path.Join(spec.Directory, "output")},
		{Type: "bind", Source: layout.homeData, Destination: "/home/craft/.local/share/opencode"},
		{Type: "bind", Source: layout.homeConfig, Destination: "/home/craft/.config/opencode"},
		{Type: "tmpfs", Destination: "/tmp", Options: []string{"mode=1777", fmt.Sprintf("size=%d", craftRunViewTmpfsBytes)}},
	}
}

func (p *CraftRunViewContainerProvider) createRequest(spec CraftRunViewRuntimeContainerSpec, network string, layout craftRunViewHostLayout) CraftRunViewContainerCreateRequest {
	return CraftRunViewContainerCreateRequest{
		Name: spec.ContainerID, ImageReference: p.config.ImageReference,
		WorkingDirectory: spec.Directory, Directory: spec.Directory,
		User:       craftRunViewContainerUser,
		Entrypoint: []string{"/usr/local/bin/opencode"},
		Command:    []string{"serve", "--hostname", "0.0.0.0", "--port", fmt.Sprint(craftRunViewOpenCodePort)},
		Environment: []string{
			"HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config", "XDG_DATA_HOME=/home/craft/.local/share",
			"XDG_STATE_HOME=/home/craft/.local/share/opencode",
			"XDG_CACHE_HOME=/home/craft/.local/share/opencode/cache",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		Labels: p.labels(spec), Mounts: expectedCraftRunViewMounts(layout, spec),
		NetworkMode: network, Privileged: false, ReadonlyRootfs: true, NoNewPrivileges: true,
		CapDrop: []string{"ALL"}, MemoryBytes: craftRunViewMemoryBytes, PidsLimit: craftRunViewPidsLimit,
	}
}

func (p *CraftRunViewContainerProvider) labels(spec CraftRunViewRuntimeContainerSpec) map[string]string {
	generation := sha256.Sum256([]byte(spec.Generation))
	return map[string]string{
		craftRunViewLabelGeneration: hex.EncodeToString(generation[:]),
		craftRunViewLabelRuntime:    spec.RuntimeID,
		craftRunViewLabelContainer:  spec.ContainerID,
		craftRunViewLabelImage:      p.config.ImageDigest,
	}
}

func (p *CraftRunViewContainerProvider) networkSpec(spec CraftRunViewRuntimeContainerSpec) CraftRunViewContainerNetworkSpec {
	short := strings.TrimPrefix(spec.ContainerID, "rv-container-")
	return CraftRunViewContainerNetworkSpec{
		Name: "rv-network-" + short, Driver: "bridge", Internal: true,
		Labels: p.labels(spec),
	}
}

func validateNetwork(spec CraftRunViewContainerNetworkSpec, got CraftRunViewContainerNetwork) error {
	if got.ID == "" || got.Name != spec.Name || got.Driver != spec.Driver || !got.Internal || !reflect.DeepEqual(got.Labels, spec.Labels) {
		return errors.New("network name, driver, isolation, or generation labels differ")
	}
	return nil
}

func (p *CraftRunViewContainerProvider) validateSpec(spec CraftRunViewRuntimeContainerSpec) error {
	if spec.Generation == "" || !validCraftRunViewToken(spec.Generation) {
		return unresolvedCraftRunView("generation token is invalid", nil)
	}
	digest := sha256.Sum256([]byte(spec.Generation))
	short := hex.EncodeToString(digest[:16])
	if spec.RuntimeID != "rv-runtime-"+short || spec.ContainerID != "rv-container-"+short || spec.Directory != path.Join("/workspace", "rv-"+short) {
		return unresolvedCraftRunView("container identity is not deterministically derived from generation", nil)
	}
	if !validCraftRunViewDirectory(spec.Directory) {
		return unresolvedCraftRunView("container directory is not canonical", nil)
	}
	return nil
}

func (p *CraftRunViewContainerProvider) validProbe(probe CraftRunViewRuntimeProbe) bool {
	return probe.OpenCodeVersion == p.config.OpenCodeVersion &&
		probe.BinarySHA256 == p.config.OpenCodeBinarySHA256 &&
		probe.RuntimeConfigSHA256 == p.config.RuntimeConfigSHA256
}

func (p *CraftRunViewContainerProvider) prepareGenerationLayout(spec CraftRunViewRuntimeContainerSpec) (craftRunViewHostLayout, error) {
	layout := generationLayoutPaths(p.config.SandboxRoot, spec)
	root := layout.root
	if err := secureDirectory(root, 0700); err != nil {
		return craftRunViewHostLayout{}, err
	}
	for _, dir := range []struct {
		path string
		mode os.FileMode
	}{
		{layout.inputs, 0755}, {layout.knowledge, 0755}, {layout.output, 0777}, {layout.homeData, 0777}, {layout.homeConfig, 0777},
	} {
		if err := secureDirectory(dir.path, dir.mode); err != nil {
			return craftRunViewHostLayout{}, err
		}
	}
	if err := persistOrVerifyLayoutIdentity(layout, spec); err != nil {
		return craftRunViewHostLayout{}, err
	}
	return layout, nil
}

type craftRunViewHostLayout struct {
	root, inputs, knowledge, output, homeData, homeConfig, marker, identity string
}

type craftRunViewDirectoryIdentity struct {
	Path   string `json:"path"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type craftRunViewLayoutIdentity struct {
	Version     int                             `json:"version"`
	Generation  string                          `json:"generation"`
	Directories []craftRunViewDirectoryIdentity `json:"directories"`
}

func generationLayoutPaths(sandboxRoot string, spec CraftRunViewRuntimeContainerSpec) craftRunViewHostLayout {
	digest := sha256.Sum256([]byte(spec.Generation))
	short := hex.EncodeToString(digest[:16])
	root := filepath.Join(sandboxRoot, "rv-"+short)
	return craftRunViewHostLayout{
		root: root, inputs: filepath.Join(root, "inputs"), knowledge: filepath.Join(root, "knowledge"),
		output: filepath.Join(root, "output"), homeData: filepath.Join(root, "home-data"),
		homeConfig: filepath.Join(root, "home-config"), marker: filepath.Join(root, craftRunViewCreateMarker),
		identity: filepath.Join(root, craftRunViewLayoutIdentityFile),
	}
}

func verifyGenerationLayout(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) error {
	for _, dir := range generationLayoutDirectories(layout) {
		if _, err := verifySecureDirectory(dir.path, dir.mode); err != nil {
			return err
		}
	}
	expected, err := layoutIdentityBytes(layout, spec)
	if err != nil {
		return err
	}
	info, err := os.Lstat(layout.identity)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4096 {
		return errors.New("generation layout identity manifest is not a private regular file")
	}
	actual, err := os.ReadFile(layout.identity)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("generation layout identity manifest differs from directory device/inode identities")
	}
	return nil
}

func persistOrVerifyLayoutIdentity(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) error {
	expected, err := layoutIdentityBytes(layout, spec)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(layout.identity); err == nil {
		return verifyGenerationLayout(layout, spec)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(layout.root, ".layout-identity-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(expected); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Link(tempPath, layout.identity); err != nil {
		if errors.Is(err, os.ErrExist) {
			return verifyGenerationLayout(layout, spec)
		}
		return err
	}
	if err := os.Remove(tempPath); err != nil {
		return err
	}
	root, err := os.Open(layout.root)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Sync(); err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(layout.root))
	if err != nil {
		return err
	}
	defer parent.Close()
	if err := parent.Sync(); err != nil {
		return err
	}
	return verifyGenerationLayout(layout, spec)
}

func layoutIdentityBytes(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) ([]byte, error) {
	identity := craftRunViewLayoutIdentity{Version: 1, Generation: spec.Generation}
	for _, dir := range generationLayoutDirectories(layout) {
		item, err := verifySecureDirectory(dir.path, dir.mode)
		if err != nil {
			return nil, err
		}
		identity.Directories = append(identity.Directories, item)
	}
	return json.Marshal(identity)
}

func generationLayoutDirectories(layout craftRunViewHostLayout) []struct {
	path string
	mode os.FileMode
} {
	return []struct {
		path string
		mode os.FileMode
	}{
		{layout.root, 0700}, {layout.inputs, 0755}, {layout.knowledge, 0755},
		{layout.output, 0777}, {layout.homeData, 0777}, {layout.homeConfig, 0777},
	}
}

func verifySecureDirectory(path string, mode os.FileMode) (craftRunViewDirectoryIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return craftRunViewDirectoryIdentity{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != mode.Perm() {
		return craftRunViewDirectoryIdentity{}, errors.New("generation host path is not the expected real directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return craftRunViewDirectoryIdentity{}, errors.New("generation host path resolves through a symlink")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return craftRunViewDirectoryIdentity{}, errors.New("filesystem does not expose stable device/inode identity")
	}
	return craftRunViewDirectoryIdentity{Path: path, Device: uint64(stat.Dev), Inode: uint64(stat.Ino)}, nil
}

func secureDirectory(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	created := false
	if errors.Is(err, os.ErrNotExist) {
		mkdirErr := os.Mkdir(path, mode)
		if mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
			return mkdirErr
		}
		created = mkdirErr == nil
		if created {
			if err := os.Chmod(path, mode); err != nil {
				return err
			}
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("generation host path is not a real directory")
	}
	if info.Mode().Perm() != mode.Perm() {
		return fmt.Errorf("generation host directory mode is %04o, expected %04o", info.Mode().Perm(), mode.Perm())
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return errors.New("generation host path resolves through a symlink")
	}
	return nil
}

func prepareSandboxRoot(root string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || root == string(filepath.Separator) {
		return "", errors.New("sandbox root must be a non-root canonical absolute path")
	}
	if forbidden := forbiddenCraftRunViewSandboxRoot(root); forbidden != "" {
		return "", fmt.Errorf("sandbox root is under forbidden host path %s", forbidden)
	}
	canonicalCandidate, err := canonicalProspectivePath(root)
	if err != nil {
		return "", fmt.Errorf("resolve sandbox root before creation: %w", err)
	}
	if forbidden := forbiddenCraftRunViewSandboxRoot(canonicalCandidate); forbidden != "" {
		return "", fmt.Errorf("canonical sandbox root is under forbidden host path %s", forbidden)
	}
	if err := os.MkdirAll(canonicalCandidate, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(canonicalCandidate)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(resolved) || resolved == string(filepath.Separator) {
		return "", errors.New("sandbox root resolves to an unsafe path")
	}
	if resolved != canonicalCandidate {
		return "", errors.New("canonical sandbox root changed through a symlink during creation")
	}
	if forbidden := forbiddenCraftRunViewSandboxRoot(resolved); forbidden != "" {
		return "", fmt.Errorf("canonical sandbox root is under forbidden host path %s", forbidden)
	}
	if err := os.Chmod(resolved, 0700); err != nil {
		return "", err
	}
	return resolved, nil
}

// canonicalProspectivePath resolves the longest existing prefix of a path and
// appends missing segments without creating them. The resulting path lets us
// apply the forbidden-root policy before MkdirAll or chmod can touch a
// symlink's target.
func canonicalProspectivePath(candidate string) (string, error) {
	missing := make([]string, 0)
	existing := candidate
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(existing)
			if err != nil {
				return "", err
			}
			for index := len(missing) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missing[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", errors.New("sandbox root has no existing path prefix")
		}
		missing = append(missing, filepath.Base(existing))
		existing = parent
	}
}

func forbiddenCraftRunViewSandboxRoot(root string) string {
	for _, forbidden := range []string{"/home", "/root", "/etc", "/proc", "/sys", "/dev", "/var/run", "/var/lib/docker", "/Users"} {
		if pathWithin(forbidden, root) {
			return forbidden
		}
	}
	return ""
}

func (p *CraftRunViewContainerProvider) checkCreateMarker(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) (bool, error) {
	info, err := os.Lstat(layout.marker)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return false, errors.New("container create marker is not a private regular file")
	}
	content, err := os.ReadFile(layout.marker)
	if err != nil {
		return false, err
	}
	if string(content) != p.markerContents(spec) {
		return false, errors.New("container create marker does not match generation")
	}
	return true, nil
}

func (p *CraftRunViewContainerProvider) createMarker(layout craftRunViewHostLayout, spec CraftRunViewRuntimeContainerSpec) (bool, error) {
	file, err := os.OpenFile(layout.marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		_, checkErr := p.checkCreateMarker(layout, spec)
		return false, checkErr
	}
	if err != nil {
		return false, err
	}
	content := []byte(p.markerContents(spec))
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return false, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return false, err
	}
	if err := file.Close(); err != nil {
		return false, err
	}
	directory, err := os.Open(layout.root)
	if err != nil {
		return false, err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return false, err
	}
	return true, nil
}

func (p *CraftRunViewContainerProvider) markerContents(spec CraftRunViewRuntimeContainerSpec) string {
	generation := sha256.Sum256([]byte(spec.Generation))
	return hex.EncodeToString(generation[:]) + "\n" + spec.ContainerID + "\n" + p.config.ImageDigest + "\n"
}

func (p *CraftRunViewContainerProvider) lockGeneration(key string) func() {
	p.mu.Lock()
	entry := p.locks[key]
	if entry == nil {
		entry = &craftRunViewKeyedLock{}
		p.locks[key] = entry
	}
	entry.refs++
	p.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		p.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(p.locks, key)
		}
		p.mu.Unlock()
	}
}

func runtimeContainerFromBinding(binding craftRunViewContainerBinding) CraftRunViewRuntimeContainer {
	return CraftRunViewRuntimeContainer{
		Generation: binding.spec.Generation, RuntimeID: binding.spec.RuntimeID,
		ContainerID: binding.spec.ContainerID, Directory: binding.spec.Directory,
		ProjectID:        binding.sessionsProjectID(),
		IdentityVerified: true, DedicatedForRun: true, DirectoryCanonical: true,
	}
}

func (b craftRunViewContainerBinding) sessionsProjectID() string {
	return b.projectID
}

func sameRuntimeIdentity(left, right CraftRunViewRuntimeContainer) bool {
	return left.Generation == right.Generation && left.RuntimeID == right.RuntimeID &&
		left.ContainerID == right.ContainerID && left.Directory == right.Directory &&
		left.ProjectID == right.ProjectID && left.IdentityVerified == right.IdentityVerified &&
		left.DedicatedForRun == right.DedicatedForRun && left.DirectoryCanonical == right.DirectoryCanonical
}

func safeCraftRunViewEnvironment(values []string) bool {
	want := map[string]string{
		"HOME":            "/home/craft",
		"XDG_CONFIG_HOME": "/home/craft/.config",
		"XDG_DATA_HOME":   "/home/craft/.local/share",
		"XDG_STATE_HOME":  "/home/craft/.local/share/opencode",
		"XDG_CACHE_HOME":  "/home/craft/.local/share/opencode/cache",
		"PATH":            "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	got := make(map[string]string, len(values))
	for _, value := range values {
		key, content, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			return false
		}
		if _, exists := got[key]; exists {
			return false
		}
		got[key] = content
	}
	return reflect.DeepEqual(got, want)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSHA256Digest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validSHA256Hex(strings.TrimPrefix(value, "sha256:"))
}

func pathWithin(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func isPrivateIP(value string) bool {
	ip := net.ParseIP(value)
	return ip != nil && ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified()
}

func openCodeEndpoint(address string) (string, error) {
	ip := net.ParseIP(address)
	if ip == nil || !isPrivateIP(address) {
		return "", errors.New("OpenCode network address is not a private IP")
	}
	return "http://" + net.JoinHostPort(ip.String(), fmt.Sprint(craftRunViewOpenCodePort)), nil
}

func validPinnedOpenCodeSessionID(id string) bool {
	if len(id) != len("ses_")+12+14 || !strings.HasPrefix(id, "ses_") {
		return false
	}
	for index, r := range id[len("ses_"):] {
		switch {
		case index < 12 && (r >= '0' && r <= '9' || r >= 'a' && r <= 'f'):
		case index >= 12 && (r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'):
		default:
			return false
		}
	}
	return true
}
