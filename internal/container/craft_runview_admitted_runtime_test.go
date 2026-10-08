package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

type admittedRuntimeAuthorityFake struct {
	view           craft.RunView
	allocateErr    error
	stateful       bool
	issued         map[craft.RunViewEffectKind]bool
	beginErr       map[craft.RunViewEffectKind]error
	maySend        map[craft.RunViewEffectKind]bool
	outcomes       map[craft.RunViewEffectKind]craft.RunViewEffectOutcome
	allocTasks     []craft.Task
	beginTasks     []craft.Task
	claims         []craft.RunViewEffectClaim
	requestDigests map[craft.RunViewEffectKind]string
	finished       []craft.RunViewEffectOutcome
	order          *[]string
	afterBegin     func(craft.RunViewEffectKind)
}

func (a *admittedRuntimeAuthorityFake) AllocateAdmitted(_ context.Context, task craft.Task) (craft.RunView, error) {
	a.allocTasks = append(a.allocTasks, task)
	if a.allocateErr != nil {
		return craft.RunView{}, a.allocateErr
	}
	if a.order != nil {
		*a.order = append(*a.order, "allocate")
	}
	return a.view, nil
}

func (a *admittedRuntimeAuthorityFake) BeginEffect(_ context.Context, task craft.Task, generation string, kind craft.RunViewEffectKind) (craft.RunViewEffectClaim, bool, error) {
	a.beginTasks = append(a.beginTasks, task)
	if a.order != nil {
		*a.order = append(*a.order, "claim:"+string(kind))
	}
	if err := a.beginErr[kind]; err != nil {
		return craft.RunViewEffectClaim{}, false, err
	}
	claim := craft.RunViewEffectClaim{Key: admittedRunViewKey(task), Generation: generation, Kind: kind, Token: "opaque-" + string(kind)}
	a.claims = append(a.claims, claim)
	maySend := a.maySend[kind]
	if a.stateful {
		if a.issued == nil {
			a.issued = map[craft.RunViewEffectKind]bool{}
		}
		maySend = !a.issued[kind]
		a.issued[kind] = true
	}
	if a.afterBegin != nil {
		a.afterBegin(kind)
	}
	return claim, maySend, nil
}

func (a *admittedRuntimeAuthorityFake) BeginEffectWithDigest(_ context.Context, task craft.Task, generation string, kind craft.RunViewEffectKind, digest string) (craft.RunViewEffectClaim, bool, error) {
	if prior := a.requestDigests[kind]; prior != "" && prior != digest {
		return craft.RunViewEffectClaim{}, false, craft.ErrConflict
	}
	if a.requestDigests == nil {
		a.requestDigests = map[craft.RunViewEffectKind]string{}
	}
	a.requestDigests[kind] = digest
	a.beginTasks = append(a.beginTasks, task)
	if a.order != nil {
		*a.order = append(*a.order, "claim:"+string(kind))
	}
	if err := a.beginErr[kind]; err != nil {
		return craft.RunViewEffectClaim{}, false, err
	}
	claim := craft.RunViewEffectClaim{Key: admittedRunViewKey(task), Generation: generation, Kind: kind, RequestDigest: digest, Token: "opaque-" + string(kind)}
	a.claims = append(a.claims, claim)
	maySend := a.maySend[kind]
	if a.stateful {
		if a.issued == nil {
			a.issued = map[craft.RunViewEffectKind]bool{}
		}
		maySend = !a.issued[kind]
		a.issued[kind] = true
	}
	if a.afterBegin != nil {
		a.afterBegin(kind)
	}
	return claim, maySend, nil
}

func (a *admittedRuntimeAuthorityFake) FinishEffect(_ context.Context, claim craft.RunViewEffectClaim, outcome craft.RunViewEffectOutcome) error {
	if a.order != nil {
		*a.order = append(*a.order, "finish:"+string(claim.Kind))
	}
	a.finished = append(a.finished, outcome)
	if prior, ok := a.outcomes[claim.Kind]; ok && prior != outcome {
		return craft.ErrConflict
	}
	if a.outcomes == nil {
		a.outcomes = map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}
	}
	a.outcomes[claim.Kind] = outcome
	return nil
}

type admittedRuntimeStoreFake struct {
	view       craft.RunView
	beginErr   error
	beginCalls int
}

func (s *admittedRuntimeStoreFake) Allocate(context.Context, craft.RunViewKey) (craft.RunView, error) {
	return craft.RunView{}, errors.New("legacy Allocate must not be called")
}
func (s *admittedRuntimeStoreFake) Load(_ context.Context, key craft.RunViewKey) (craft.RunView, error) {
	if s.view.Key != key {
		return craft.RunView{}, craft.ErrConflict
	}
	return s.view, nil
}
func (s *admittedRuntimeStoreFake) BeginSessionCreate(_ context.Context, key craft.RunViewKey, generation string) (craft.RunView, bool, error) {
	s.beginCalls++
	if s.beginErr != nil {
		return craft.RunView{}, false, s.beginErr
	}
	if s.view.Key != key || s.view.Generation != generation {
		return craft.RunView{}, false, craft.ErrConflict
	}
	maySend := s.view.SessionCreateIntentAt == nil && s.view.State == craft.RunViewStateAllocating
	if maySend {
		now := time.Now().UTC()
		s.view.SessionCreateIntentAt = &now
	}
	return s.view, maySend, nil
}
func (s *admittedRuntimeStoreFake) BindRuntime(_ context.Context, key craft.RunViewKey, generation string, runtime craft.RunViewRuntime) (craft.RunView, error) {
	if s.view.Key != key || s.view.Generation != generation {
		return craft.RunView{}, craft.ErrConflict
	}
	s.view.State, s.view.Runtime = craft.RunViewStateBound, runtime
	return s.view, nil
}

type admittedLegacyOnlyProvider struct{ CraftRunViewRuntimeProvider }

type admittedRuntimeProviderFake struct {
	container           CraftRunViewRuntimeContainer
	exists              bool
	running             bool
	sessions            []CraftRunViewRuntimeSession
	createErr           error
	createMutations     int
	startErr            error
	persistRunningOnErr bool
	sessionErr          error
	observeContainerErr error
	observeStateErr     error
	calls               []string
	order               *[]string
	createEntered       chan struct{}
	createRelease       <-chan struct{}
}

type admittedSplitRuntimeProviderFake struct {
	*admittedRuntimeProviderFake
	network               CraftRunViewContainerNetwork
	networkExists         bool
	networkCreateErr      error
	persistNetworkOnError bool
	persistOnError        bool
	createCalls           int
	createDirectory       string
	probe                 CraftRunViewRuntimeProbe
	probeExists           bool
	probeCalls            int
	probeErr              error
	persistProbeOnError   bool
	probeObserveErr       error
}

func (p *admittedSplitRuntimeProviderFake) networkFor(spec CraftRunViewRuntimeContainerSpec) CraftRunViewContainerNetwork {
	generation := sha256.Sum256([]byte(spec.Generation))
	short := strings.TrimPrefix(spec.ContainerID, "rv-container-")
	return CraftRunViewContainerNetwork{ID: "network-rv-network-" + short, Name: "rv-network-" + short, Driver: "bridge", Internal: true, Labels: map[string]string{
		craftRunViewLabelGeneration: hex.EncodeToString(generation[:]),
		craftRunViewLabelRuntime:    spec.RuntimeID,
		craftRunViewLabelContainer:  spec.ContainerID,
		craftRunViewLabelImage:      "sha256:" + strings.Repeat("a", 64),
	}}
}

func (p *admittedSplitRuntimeProviderFake) admittedRunViewNetworkSpec(spec CraftRunViewRuntimeContainerSpec) CraftRunViewContainerNetworkSpec {
	network := p.networkFor(spec)
	return CraftRunViewContainerNetworkSpec{Name: network.Name, Driver: network.Driver, Internal: network.Internal, Labels: cloneRunViewLabels(network.Labels)}
}

func (p *admittedSplitRuntimeProviderFake) admittedRunViewProbePins() (string, string, string) {
	return "1.18.4", strings.Repeat("b", 64), strings.Repeat("c", 64)
}

func (p *admittedSplitRuntimeProviderFake) observation(spec CraftRunViewRuntimeContainerSpec, network CraftRunViewContainerNetwork) CraftRunViewContainerObservation {
	state := "created"
	if p.running {
		state = "running"
	}
	runtime := p.container
	runtime.Generation, runtime.RuntimeID, runtime.ContainerID, runtime.Directory = spec.Generation, spec.RuntimeID, spec.ContainerID, spec.Directory
	if runtime.ProjectID == "" {
		runtime.ProjectID = "project-1"
	}
	if !p.exists && runtime.ProjectID == "project-1" {
		runtime.IdentityVerified, runtime.DedicatedForRun, runtime.DirectoryCanonical = true, true, true
	}
	return CraftRunViewContainerObservation{
		Runtime:  runtime,
		DockerID: "docker-" + spec.ContainerID, State: state, Network: cloneRunViewNetwork(network),
		NetworkAttachment: CraftRunViewNetworkAttachment{Name: network.Name, ID: network.ID, Internal: network.Internal, IP: "172.30.0.2"},
	}
}

func (p *admittedSplitRuntimeProviderFake) ObserveNetwork(_ context.Context, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewContainerNetwork, bool, error) {
	p.add("observe-network")
	if !p.networkExists {
		return CraftRunViewContainerNetwork{}, false, nil
	}
	return cloneRunViewNetwork(p.network), true, nil
}

func (p *admittedSplitRuntimeProviderFake) CreateGenerationNetwork(_ context.Context, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewContainerNetwork, error) {
	p.add("create-network")
	if p.networkCreateErr != nil {
		if p.persistNetworkOnError {
			p.network, p.networkExists = p.networkFor(spec), true
		}
		return CraftRunViewContainerNetwork{}, p.networkCreateErr
	}
	p.network, p.networkExists = p.networkFor(spec), true
	return cloneRunViewNetwork(p.network), nil
}

func (p *admittedSplitRuntimeProviderFake) ObserveContainer(_ context.Context, spec CraftRunViewRuntimeContainerSpec, network CraftRunViewContainerNetwork) (CraftRunViewContainerObservation, bool, error) {
	p.add("observe-container")
	if p.observeContainerErr != nil {
		return CraftRunViewContainerObservation{}, false, p.observeContainerErr
	}
	if !p.exists {
		return CraftRunViewContainerObservation{}, false, nil
	}
	p.container.Generation, p.container.RuntimeID, p.container.ContainerID, p.container.Directory = spec.Generation, spec.RuntimeID, spec.ContainerID, spec.Directory
	return p.observation(spec, network), true, nil
}

func (p *admittedSplitRuntimeProviderFake) CreateGenerationContainer(ctx context.Context, spec CraftRunViewRuntimeContainerSpec, network CraftRunViewContainerNetwork) (CraftRunViewContainerObservation, error) {
	p.add("create-container")
	if p.createEntered != nil {
		close(p.createEntered)
		<-p.createRelease
	}
	if p.createErr != nil {
		if p.persistOnError {
			p.exists = true
		}
		return CraftRunViewContainerObservation{}, p.createErr
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewContainerObservation{}, err
	}
	p.createMutations++
	p.exists = true
	p.container.Generation, p.container.RuntimeID, p.container.ContainerID, p.container.Directory = spec.Generation, spec.RuntimeID, spec.ContainerID, spec.Directory
	return p.observation(spec, network), nil
}

func (p *admittedSplitRuntimeProviderFake) ObserveContainerState(_ context.Context, expected CraftRunViewContainerObservation) (CraftRunViewContainerObservation, bool, error) {
	p.add("observe-container-state")
	if p.observeStateErr != nil {
		return CraftRunViewContainerObservation{}, false, p.observeStateErr
	}
	if !p.exists {
		return CraftRunViewContainerObservation{}, false, nil
	}
	return p.observation(expected.RuntimeSpec(), expected.Network), true, nil
}

func (p *admittedSplitRuntimeProviderFake) StartGenerationContainer(_ context.Context, expected CraftRunViewContainerObservation) (CraftRunViewContainerObservation, error) {
	p.add("start-container")
	if p.startErr != nil {
		if p.persistRunningOnErr {
			p.running = true
		}
		return CraftRunViewContainerObservation{}, p.startErr
	}
	p.running = true
	return p.observation(expected.RuntimeSpec(), expected.Network), nil
}

func (p *admittedSplitRuntimeProviderFake) ObserveRuntimeProbe(_ context.Context, expected CraftRunViewContainerObservation) (CraftRunViewRuntimeProbe, bool, error) {
	p.add("observe-probe")
	if p.probeObserveErr != nil {
		return CraftRunViewRuntimeProbe{}, false, p.probeObserveErr
	}
	if !p.probeExists {
		return CraftRunViewRuntimeProbe{}, false, unresolvedCraftRunView("fake cannot recover previous probe output", nil)
	}
	return p.probe, true, nil
}

func (p *admittedSplitRuntimeProviderFake) SendRuntimeProbe(_ context.Context, expected CraftRunViewContainerObservation) (CraftRunViewRuntimeProbe, error) {
	p.add("send-probe")
	p.probeCalls++
	if p.probeErr != nil {
		if p.persistProbeOnError {
			p.probe, p.probeExists = p.validProbeFor(expected), true
		}
		return CraftRunViewRuntimeProbe{}, p.probeErr
	}
	p.probe, p.probeExists = p.validProbeFor(expected), true
	return p.probe, nil
}

func (p *admittedSplitRuntimeProviderFake) validProbeFor(expected CraftRunViewContainerObservation) CraftRunViewRuntimeProbe {
	version, binaryHash, configHash := p.admittedRunViewProbePins()
	return CraftRunViewRuntimeProbe{ContainerID: expected.DockerID, ExecID: "exec-" + expected.DockerID, OpenCodeVersion: version, BinarySHA256: binaryHash, RuntimeConfigSHA256: configHash}
}

func (p *admittedSplitRuntimeProviderFake) ObserveSessions(_ context.Context, expected CraftRunViewContainerObservation) (CraftRunViewRuntimeInventory, error) {
	p.add("observe-sessions")
	return CraftRunViewRuntimeInventory{Authoritative: true, Complete: true, Directory: expected.Runtime.Directory, ProjectID: expected.Runtime.ProjectID, Sessions: append([]CraftRunViewRuntimeSession(nil), p.sessions...)}, nil
}

func (p *admittedSplitRuntimeProviderFake) CreateOpenCodeSession(_ context.Context, expected CraftRunViewContainerObservation, directory string) (CraftRunViewRuntimeSession, error) {
	p.add("create-session")
	if p.sessionErr != nil {
		return CraftRunViewRuntimeSession{}, p.sessionErr
	}
	session := CraftRunViewRuntimeSession{ID: "session-1", ProjectID: expected.Runtime.ProjectID, Directory: directory}
	p.sessions = append(p.sessions, session)
	p.createCalls++
	p.createDirectory = directory
	return session, nil
}

func (p *admittedRuntimeProviderFake) add(call string) {
	p.calls = append(p.calls, call)
	if p.order != nil {
		*p.order = append(*p.order, "provider:"+call)
	}
}
func (p *admittedRuntimeProviderFake) InspectOrCreateContainer(context.Context, CraftRunViewRuntimeContainerSpec) (CraftRunViewRuntimeContainer, error) {
	p.add("legacy-inspect-or-create")
	return p.container, nil
}
func (p *admittedRuntimeProviderFake) FindSessions(context.Context, CraftRunViewRuntimeContainer) (CraftRunViewRuntimeInventory, error) {
	p.add("find-sessions")
	return CraftRunViewRuntimeInventory{Authoritative: true, Complete: true, Directory: p.container.Directory, ProjectID: p.container.ProjectID, Sessions: append([]CraftRunViewRuntimeSession(nil), p.sessions...)}, nil
}
func (p *admittedRuntimeProviderFake) ObserveSessions(ctx context.Context, container CraftRunViewRuntimeContainer) (CraftRunViewRuntimeInventory, error) {
	p.add("observe-sessions")
	return CraftRunViewRuntimeInventory{Authoritative: true, Complete: true, Directory: container.Directory, ProjectID: container.ProjectID, Sessions: append([]CraftRunViewRuntimeSession(nil), p.sessions...)}, nil
}
func (p *admittedRuntimeProviderFake) CreateSession(context.Context, CraftRunViewRuntimeContainer, string) error {
	p.add("create-session")
	if p.sessionErr != nil {
		return p.sessionErr
	}
	p.sessions = []CraftRunViewRuntimeSession{{ID: "session-1", ProjectID: p.container.ProjectID, Directory: p.container.Directory}}
	return nil
}
func (p *admittedRuntimeProviderFake) CreateOpenCodeSession(ctx context.Context, container CraftRunViewRuntimeContainer, directory string) error {
	return p.CreateSession(ctx, container, directory)
}
func (p *admittedRuntimeProviderFake) ObserveContainer(_ context.Context, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewRuntimeContainer, bool, error) {
	p.add("observe-container")
	if p.observeContainerErr != nil {
		return CraftRunViewRuntimeContainer{}, false, p.observeContainerErr
	}
	p.container.Generation, p.container.RuntimeID, p.container.ContainerID, p.container.Directory = spec.Generation, spec.RuntimeID, spec.ContainerID, spec.Directory
	return p.container, p.exists, nil
}
func (p *admittedRuntimeProviderFake) CreateGenerationContainer(ctx context.Context, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewRuntimeContainer, error) {
	p.add("create-container")
	if p.createEntered != nil {
		close(p.createEntered)
		<-p.createRelease
	}
	if p.createErr != nil {
		return CraftRunViewRuntimeContainer{}, p.createErr
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewRuntimeContainer{}, err
	}
	p.createMutations++
	p.exists = true
	p.container.Generation, p.container.RuntimeID, p.container.ContainerID, p.container.Directory = spec.Generation, spec.RuntimeID, spec.ContainerID, spec.Directory
	return p.container, nil
}
func (p *admittedRuntimeProviderFake) ObserveContainerState(_ context.Context, container CraftRunViewRuntimeContainer) (CraftRunViewRuntimeContainer, bool, error) {
	p.add("observe-container-state")
	if p.observeStateErr != nil {
		return CraftRunViewRuntimeContainer{}, false, p.observeStateErr
	}
	p.container = container
	return p.container, p.running, nil
}
func (p *admittedRuntimeProviderFake) StartGenerationContainer(context.Context, CraftRunViewRuntimeContainer) (CraftRunViewRuntimeContainer, error) {
	p.add("start-container")
	if p.startErr != nil {
		return CraftRunViewRuntimeContainer{}, p.startErr
	}
	p.running = true
	return p.container, nil
}

type admittedSplitRuntimeProvider interface {
	CraftRunViewAdmittedSplitProvider
}

func admittedTask() craft.Task {
	// The durable admitted Run snapshot digest is raw lowercase hex, exactly
	// as AgentRunStore persists it.
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	fence := runtime.Fence{RunKey: runtime.RunKey{TenantID: 8, RunID: "run-1"}, Owner: "writer-1", Epoch: 3, SnapshotDigestVersion: 1, SnapshotDigest: digest}
	return craft.Task{Scope: craft.Scope{TenantID: 8, UserID: "owner-1", SessionID: "session-main"}, Fence: fence, SnapshotDigestVersion: 1, SnapshotDigest: digest, WorkspaceID: "workspace-1"}
}
func admittedView(task craft.Task) craft.RunView {
	return craft.RunView{Key: admittedRunViewKey(task), Generation: "generation-1", State: craft.RunViewStateAllocating}
}
func admittedProvider() *admittedSplitRuntimeProviderFake {
	return &admittedSplitRuntimeProviderFake{admittedRuntimeProviderFake: &admittedRuntimeProviderFake{exists: false, container: CraftRunViewRuntimeContainer{Generation: "generation-1", RuntimeID: "runtime-1", ContainerID: "container-1", Directory: "/srv/craft/view-1", ProjectID: "project-1", IdentityVerified: true, DedicatedForRun: true, DirectoryCanonical: true}}}
}
func admittedCoordinator(t *testing.T, task craft.Task, authority *admittedRuntimeAuthorityFake, provider CraftRunViewRuntimeProvider, store *admittedRuntimeStoreFake) *CraftRunViewRuntimeCoordinator {
	t.Helper()
	coordinator, err := NewCraftRunViewAdmittedRuntimeCoordinator(store, authority, provider, "/srv/craft")
	require.NoError(t, err)
	return coordinator
}

func TestCraftRunViewAdmittedCoordinatorRequiresSplitMutationProvider(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task)}
	provider := admittedProvider()
	store := &admittedRuntimeStoreFake{view: authority.view}
	coordinator := admittedCoordinator(t, task, authority, admittedLegacyOnlyProvider{CraftRunViewRuntimeProvider: provider}, store)
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Len(t, authority.allocTasks, 1)
	require.Empty(t, authority.claims)
	require.Empty(t, provider.calls, "combined InspectOrCreateContainer cannot be safely claimed as separate Docker create/start effects")
}

func TestCraftRunViewAdmittedCoordinatorClaimsEachMutationBeforeCall(t *testing.T) {
	task := admittedTask()
	order := []string{}
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}, order: &order}
	provider := admittedProvider()
	provider.order = &order
	store := &admittedRuntimeStoreFake{view: authority.view}
	coordinator := admittedCoordinator(t, task, authority, provider, store)
	handle, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, craft.RunViewStateBound, handle.View.State)
	require.Equal(t, "session-1", handle.View.Runtime.OpenCodeSessionID)
	require.Equal(t, []craft.RunViewEffectKind{craft.RunViewEffectDockerNetworkCreate, craft.RunViewEffectDockerCreate, craft.RunViewEffectDockerStart, craft.RunViewEffectDockerProbe, craft.RunViewEffectOpenCodeCreate}, effectKinds(authority.claims))
	require.Len(t, authority.finished, 5)
	require.Equal(t, task, authority.allocTasks[0], "allocation receives the original admitted Task")
	for _, claimedTask := range authority.beginTasks {
		require.Equal(t, task, claimedTask, "each effect claim reuses the original Task fence and digest")
	}
	for _, kind := range []craft.RunViewEffectKind{craft.RunViewEffectDockerNetworkCreate, craft.RunViewEffectDockerCreate, craft.RunViewEffectDockerStart, craft.RunViewEffectDockerProbe, craft.RunViewEffectOpenCodeCreate} {
		claimAt, providerAt := -1, -1
		for i, item := range order {
			if item == "claim:"+string(kind) {
				claimAt = i
			}
			if item == "provider:create-container" && kind == craft.RunViewEffectDockerCreate {
				providerAt = i
			}
			if item == "provider:create-network" && kind == craft.RunViewEffectDockerNetworkCreate {
				providerAt = i
			}
			if item == "provider:start-container" && kind == craft.RunViewEffectDockerStart {
				providerAt = i
			}
			if item == "provider:send-probe" && kind == craft.RunViewEffectDockerProbe {
				providerAt = i
			}
			if item == "provider:create-session" && kind == craft.RunViewEffectOpenCodeCreate {
				providerAt = i
			}
		}
		require.NotEqual(t, -1, claimAt)
		require.NotEqual(t, -1, providerAt)
		require.Less(t, claimAt, providerAt, "claim %s precedes provider call", kind)
	}
	require.Less(t, indexAdmitted(order, "provider:create-network"), indexAdmitted(order, "provider:create-container"))
	require.Less(t, indexAdmitted(order, "provider:start-container"), indexAdmitted(order, "provider:send-probe"))
	require.Less(t, indexAdmitted(order, "provider:send-probe"), indexAdmitted(order, "provider:create-session"))
}

func TestCraftRunViewAdmittedClaimDenialAndUnknownNeverResend(t *testing.T) {
	t.Run("transition-first", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), beginErr: map[craft.RunViewEffectKind]error{craft.RunViewEffectDockerNetworkCreate: runtime.ErrLeaseLost}}
		provider := admittedProvider()
		store := &admittedRuntimeStoreFake{view: authority.view}
		coordinator := admittedCoordinator(t, task, authority, provider, store)
		_, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, runtime.ErrLeaseLost)
		require.Zero(t, countCall(provider.calls, "create-container"))
		require.Zero(t, countCall(provider.calls, "start-container"))
		require.Zero(t, countCall(provider.calls, "create-session"))
	})
	t.Run("unknown-create-replay", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
		provider := admittedProvider()
		provider.createErr = errors.New("connection reset after send")
		store := &admittedRuntimeStoreFake{view: authority.view}
		coordinator := admittedCoordinator(t, task, authority, provider, store)
		_, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, countCall(provider.calls, "create-container"))
		provider.createErr = nil
		for kind := range authority.maySend {
			authority.maySend[kind] = false
		}
		_, err = coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, countCall(provider.calls, "create-container"), "unknown create is observed but never sent again")
	})
}
func countCall(calls []string, want string) int {
	n := 0
	for _, call := range calls {
		if call == want {
			n++
		}
	}
	return n
}

func TestCraftRunViewAdmittedNetworkProbeEffectsAreClaimedAndSequenced(t *testing.T) {
	task := admittedTask()
	order := []string{}
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}, order: &order}
	provider := admittedProvider()
	provider.order = &order
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})

	handle, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, craft.RunViewStateBound, handle.View.State)
	require.Equal(t, "session-1", handle.View.Runtime.OpenCodeSessionID)
	require.Equal(t, []craft.RunViewEffectKind{
		craft.RunViewEffectDockerNetworkCreate,
		craft.RunViewEffectDockerCreate,
		craft.RunViewEffectDockerStart,
		craft.RunViewEffectDockerProbe,
		craft.RunViewEffectOpenCodeCreate,
	}, effectKinds(authority.claims))
	require.Equal(t, 1, countCall(provider.calls, "create-network"))
	require.Equal(t, 1, countCall(provider.calls, "create-container"))
	require.Equal(t, 1, countCall(provider.calls, "start-container"))
	require.Equal(t, 1, countCall(provider.calls, "send-probe"))
	require.Equal(t, 1, countCall(provider.calls, "create-session"))
	require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
	require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerProbe].State)
	require.Regexp(t, `^[0-9a-f]{64}$`, authority.claims[0].RequestDigest)
	require.Regexp(t, `^[0-9a-f]{64}$`, authority.claims[3].RequestDigest)
	require.Equal(t, authority.claims[0].RequestDigest, authority.requestDigests[craft.RunViewEffectDockerNetworkCreate])
	require.Equal(t, authority.claims[3].RequestDigest, authority.requestDigests[craft.RunViewEffectDockerProbe])

	for _, pair := range [][2]string{{"claim:docker_network_create", "provider:create-network"}, {"claim:docker_create", "provider:create-container"}, {"claim:docker_start", "provider:start-container"}, {"claim:docker_probe", "provider:send-probe"}, {"claim:opencode_create", "provider:create-session"}} {
		require.Less(t, indexAdmitted(order, pair[0]), indexAdmitted(order, pair[1]), "%s must precede %s", pair[0], pair[1])
	}
}

func TestCraftRunViewAdmittedNetworkUnknownAndForeignObservationNeverCreate(t *testing.T) {
	t.Run("unknown-create-replay", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
		provider := admittedProvider()
		provider.networkCreateErr = errors.New("connection reset after Docker network create")
		coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
		_, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, countCall(provider.calls, "create-network"))
		require.Equal(t, craft.RunViewEffectStateUnknown, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
		_, err = coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Equal(t, 1, countCall(provider.calls, "create-network"), "unknown network creation must never resend")
		require.Len(t, authority.claims, 2)
		require.Equal(t, authority.claims[0].RequestDigest, authority.claims[1].RequestDigest)
		require.Zero(t, countCall(provider.calls, "create-container"))
	})

	t.Run("foreign-observed-network", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
		provider := admittedProvider()
		coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
		spec := coordinator.containerSpec(authority.view.Generation)
		provider.networkExists = true
		// The name intentionally collides with this generation's deterministic
		// name, while labels and the observed resource ID belong to another
		// generation. A name-only lookup must never authorize attachment.
		foreignSpec := spec
		foreignSpec.Generation, foreignSpec.RuntimeID = "foreign-generation", "runtime-foreign"
		provider.network = provider.networkFor(foreignSpec)
		provider.network.Name = provider.admittedRunViewNetworkSpec(spec).Name
		_, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
		require.Empty(t, authority.claims, "a foreign or duplicate network is rejected before effect claim")
		require.Zero(t, countCall(provider.calls, "create-network"))
		// The first observation itself must stop progression to container creation.
		require.Zero(t, countCall(provider.calls, "create-container"))
	})
}

func TestCraftRunViewAdmittedProbeUnknownReplayNeverExecutesAgain(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	provider.probeErr = errors.New("connection lost after ExecCreate")
	provider.persistProbeOnError = true
	provider.probeObserveErr = unresolvedCraftRunView("read-only Docker inspection cannot recover exec output", nil)
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, countCall(provider.calls, "send-probe"))
	require.Equal(t, craft.RunViewEffectStateUnknown, authority.outcomes[craft.RunViewEffectDockerProbe].State)
	for kind := range authority.maySend {
		authority.maySend[kind] = false
	}
	provider.probeErr = nil
	_, err = coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, countCall(provider.calls, "send-probe"), "read-only unsupported probe recovery must not permit another exec")
	require.Zero(t, countCall(provider.calls, "create-session"))
	require.Equal(t, 2, countCall(provider.calls, "observe-probe"), "replay remains inspect-only")
	require.Equal(t, 2, countEffectClaim(authority.claims, craft.RunViewEffectDockerNetworkCreate))
	require.Equal(t, 2, countEffectClaim(authority.claims, craft.RunViewEffectDockerProbe))
	require.Equal(t, authority.claims[len(authority.claims)-1].RequestDigest, authority.requestDigests[craft.RunViewEffectDockerProbe])
}

func TestCraftRunViewAdmittedResponseLossUsesExactObservationOrStaysUnknown(t *testing.T) {
	t.Run("network", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
		provider := admittedProvider()
		provider.networkCreateErr = errors.New("connection reset after network create")
		provider.persistNetworkOnError = true
		coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})

		handle, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.NoError(t, err)
		require.Equal(t, "session-1", handle.View.Runtime.OpenCodeSessionID)
		require.Equal(t, 1, countCall(provider.calls, "create-network"))
		require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
		spec := coordinator.containerSpec("generation-1")
		require.Equal(t, runViewNetworkReceipt(provider.networkFor(spec)), authority.outcomes[craft.RunViewEffectDockerNetworkCreate].Receipt)
	})

	t.Run("start", func(t *testing.T) {
		task := admittedTask()
		authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
		provider := admittedProvider()
		provider.startErr = errors.New("connection reset after container start")
		provider.persistRunningOnErr = true
		coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})

		handle, err := coordinator.ResolveAdmitted(context.Background(), task)
		require.NoError(t, err)
		require.Equal(t, "session-1", handle.View.Runtime.OpenCodeSessionID)
		require.Equal(t, 1, countCall(provider.calls, "start-container"))
		require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerStart].State)
		spec := coordinator.containerSpec("generation-1")
		expected := provider.observation(spec, provider.networkFor(spec))
		require.Equal(t, expected.DockerID+"|running", authority.outcomes[craft.RunViewEffectDockerStart].Receipt)
	})
}

func effectKinds(claims []craft.RunViewEffectClaim) []craft.RunViewEffectKind {
	kinds := make([]craft.RunViewEffectKind, len(claims))
	for i, claim := range claims {
		kinds[i] = claim.Kind
	}
	return kinds
}

func indexAdmitted(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func TestCraftRunViewAdmittedReplayKeepsGenerationAndNeverResends(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{
		view: admittedView(task),
		maySend: map[craft.RunViewEffectKind]bool{
			craft.RunViewEffectDockerNetworkCreate: true,
			craft.RunViewEffectDockerCreate:        true,
			craft.RunViewEffectDockerStart:         true,
			craft.RunViewEffectDockerProbe:         true,
			craft.RunViewEffectOpenCodeCreate:      true,
		},
		outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{},
	}
	provider := admittedProvider()
	store := &admittedRuntimeStoreFake{view: authority.view}
	coordinator := admittedCoordinator(t, task, authority, provider, store)
	first, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	createCount, startCount, sessionCount := countCall(provider.calls, "create-container"), countCall(provider.calls, "start-container"), countCall(provider.calls, "create-session")
	require.Equal(t, 1, createCount)
	require.Equal(t, 1, startCount)
	require.Equal(t, 1, sessionCount)

	for kind := range authority.maySend {
		authority.maySend[kind] = false
	}
	second, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, first.View.Generation, second.View.Generation, "admitted replay keeps the original generation")
	require.Equal(t, createCount, countCall(provider.calls, "create-container"))
	require.Equal(t, startCount, countCall(provider.calls, "start-container"))
	require.Equal(t, sessionCount, countCall(provider.calls, "create-session"))
}

func TestCraftRunViewAdmittedRejectsMutatedOriginalTaskFenceBeforeAllocation(t *testing.T) {
	task := admittedTask()
	task.Fence.SnapshotDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task)}
	provider := admittedProvider()
	store := &admittedRuntimeStoreFake{view: authority.view}
	coordinator := admittedCoordinator(t, task, authority, provider, store)
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Empty(t, authority.allocTasks, "an inconsistent Task fence cannot allocate a view")
	require.Empty(t, provider.calls)
}

func TestCraftRunViewAdmittedUnknownAtEveryProviderStageNeverResends(t *testing.T) {
	stages := []struct {
		name string
		kind craft.RunViewEffectKind
		fail func(*admittedSplitRuntimeProviderFake)
		call string
	}{
		{name: "docker create", kind: craft.RunViewEffectDockerCreate, fail: func(p *admittedSplitRuntimeProviderFake) { p.createErr = errors.New("lost create response") }, call: "create-container"},
		{name: "docker start", kind: craft.RunViewEffectDockerStart, fail: func(p *admittedSplitRuntimeProviderFake) { p.startErr = errors.New("lost start response") }, call: "start-container"},
		{name: "opencode create", kind: craft.RunViewEffectOpenCodeCreate, fail: func(p *admittedSplitRuntimeProviderFake) { p.sessionErr = errors.New("lost session response") }, call: "create-session"},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			task := admittedTask()
			authority := &admittedRuntimeAuthorityFake{
				view: admittedView(task),
				maySend: map[craft.RunViewEffectKind]bool{
					craft.RunViewEffectDockerNetworkCreate: true,
					craft.RunViewEffectDockerCreate:        true,
					craft.RunViewEffectDockerStart:         true,
					craft.RunViewEffectDockerProbe:         true,
					craft.RunViewEffectOpenCodeCreate:      true,
				},
				outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{},
			}
			provider := admittedProvider()
			stage.fail(provider)
			store := &admittedRuntimeStoreFake{view: authority.view}
			coordinator := admittedCoordinator(t, task, authority, provider, store)
			_, err := coordinator.ResolveAdmitted(context.Background(), task)
			require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
			require.Equal(t, 1, countCall(provider.calls, stage.call), "the claimed provider operation is sent once")
			require.Equal(t, craft.RunViewEffectStateUnknown, authority.outcomes[stage.kind].State)

			for kind := range authority.maySend {
				authority.maySend[kind] = false
			}
			stage.fail(provider)
			_, err = coordinator.ResolveAdmitted(context.Background(), task)
			require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
			require.Equal(t, 1, countCall(provider.calls, stage.call), "unknown operations are observed but never resent")
		})
	}
}

func TestCraftRunViewAdmittedChangedContainerReceiptStaysUnknown(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{
		view:     admittedView(task),
		stateful: true,
		outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{},
	}
	provider := admittedProvider()
	provider.exists = true
	provider.container.IdentityVerified = false
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectDockerNetworkCreate))
	require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
	require.Zero(t, countEffectClaim(authority.claims, craft.RunViewEffectDockerCreate), "an unclaimed wrong identity cannot be recorded as an unknown send")
	require.Zero(t, countCall(provider.calls, "create-container"))
	require.Zero(t, countCall(provider.calls, "start-container"))
	require.Zero(t, countCall(provider.calls, "create-session"))
}

func TestCraftRunViewAdmittedCancellationAfterClaimRecordsUnknownWithoutSend(t *testing.T) {
	task := admittedTask()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	authority := &admittedRuntimeAuthorityFake{
		view:     admittedView(task),
		stateful: true,
		outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{},
		afterBegin: func(kind craft.RunViewEffectKind) {
			if kind == craft.RunViewEffectDockerCreate {
				cancel()
			}
		},
	}
	provider := admittedProvider()
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(ctx, task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, craft.RunViewEffectStateUnknown, authority.outcomes[craft.RunViewEffectDockerCreate].State)
	require.Zero(t, countCall(provider.calls, "create-container"), "cancellation after claim stops before provider send")
	require.Zero(t, provider.createMutations, "provider cancellation stops before the fake side effect")
}

func TestCraftRunViewAdmittedClaimFirstBarrierExposesIntentBeforeProviderSend(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{
		view:     admittedView(task),
		stateful: true,
		outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{},
	}
	provider := admittedProvider()
	provider.createEntered = make(chan struct{})
	release := make(chan struct{})
	provider.createRelease = release
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	result := make(chan error, 1)
	go func() { _, err := coordinator.ResolveAdmitted(context.Background(), task); result <- err }()
	select {
	case <-provider.createEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not reach the claimed create stage")
	}
	require.Len(t, authority.claims, 2)
	require.Equal(t, craft.RunViewEffectDockerNetworkCreate, authority.claims[0].Kind)
	require.Equal(t, craft.RunViewEffectDockerCreate, authority.claims[1].Kind)
	require.Equal(t, admittedRunViewKey(task), authority.claims[0].Key)
	require.Equal(t, "generation-1", authority.claims[0].Generation)
	close(release)
	require.NoError(t, <-result)
}

func TestCraftRunViewAdmittedStaleRunFailsBeforeProvider(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), allocateErr: runtime.ErrLeaseLost}
	// AllocateAdmitted is the first live Run/fence check in the real authority.
	// A stale writer must fail there without allowing the coordinator to call a provider.
	provider := admittedProvider()
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, runtime.ErrLeaseLost)
	require.Empty(t, provider.calls)
}

func TestCraftRunViewAdmittedPreclaimContainerObservationFailureCanRetry(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	provider.observeContainerErr = errors.New("temporary inspect failure")
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectDockerNetworkCreate))
	require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
	require.Zero(t, countEffectClaim(authority.claims, craft.RunViewEffectDockerCreate), "container observation fails before the container send claim")
	require.Zero(t, countCall(provider.calls, "create-container"))

	provider.observeContainerErr = nil
	_, err = coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err, "the exact generation can be retried after a pre-send read failure")
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectDockerCreate))
	require.Equal(t, 1, countCall(provider.calls, "create-container"))
}

func TestCraftRunViewAdmittedWrongPreclaimContainerIdentityCanRetry(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	provider.exists = true
	provider.container.IdentityVerified = false
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Equal(t, craft.RunViewEffectStateSucceeded, authority.outcomes[craft.RunViewEffectDockerNetworkCreate].State)
	require.Zero(t, countEffectClaim(authority.claims, craft.RunViewEffectDockerCreate), "identity mismatch is a failed observation, not a provider send")

	provider.exists = false
	provider.container.IdentityVerified = true
	_, err = coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectDockerCreate))
	require.Equal(t, 1, countCall(provider.calls, "create-container"))
}

func TestCraftRunViewAdmittedPreclaimStateObservationFailureCanRetry(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	provider.observeStateErr = errors.New("temporary state inspection failure")
	coordinator := admittedCoordinator(t, task, authority, provider, &admittedRuntimeStoreFake{view: authority.view})
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Zero(t, countEffectClaim(authority.claims, craft.RunViewEffectDockerStart), "start is not claimed until its read-only state observation succeeds")
	require.Empty(t, authority.outcomes[craft.RunViewEffectDockerStart])
	require.Zero(t, countCall(provider.calls, "start-container"))

	provider.observeStateErr = nil
	_, err = coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectDockerStart))
	require.Equal(t, 1, countCall(provider.calls, "start-container"))
}

func TestCraftRunViewAdmittedSessionMarkerFailureDoesNotClaimOpenCodeEffect(t *testing.T) {
	task := admittedTask()
	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	store := &admittedRuntimeStoreFake{view: authority.view, beginErr: errors.New("temporary marker write failure")}
	coordinator := admittedCoordinator(t, task, authority, provider, store)
	_, err := coordinator.ResolveAdmitted(context.Background(), task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Zero(t, countEffectClaim(authority.claims, craft.RunViewEffectOpenCodeCreate), "DB-only marker failure must not consume the provider claim")
	require.Empty(t, authority.outcomes[craft.RunViewEffectOpenCodeCreate])
	require.Zero(t, countCall(provider.calls, "create-session"))

	store.beginErr = nil
	_, err = coordinator.ResolveAdmitted(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, 1, countEffectClaim(authority.claims, craft.RunViewEffectOpenCodeCreate))
	require.Equal(t, 1, countCall(provider.calls, "create-session"))
}

func countEffectClaim(claims []craft.RunViewEffectClaim, kind craft.RunViewEffectKind) int {
	count := 0
	for _, claim := range claims {
		if claim.Kind == kind {
			count++
		}
	}
	return count
}
