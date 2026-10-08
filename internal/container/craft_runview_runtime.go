package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/Tencent/WeKnora/internal/craft"
)

// ErrCraftRunViewRuntimeUnresolved means runtime creation or recovery cannot
// be proven safe. Callers must not dispatch a prompt while this error applies.
var ErrCraftRunViewRuntimeUnresolved = errors.New("craft RunView runtime unresolved")

// CraftRunViewRuntimeContainerSpec is a deterministic, server-derived
// identity for the one runtime container assigned to a RunView generation.
// Directory is inside that runtime, not a host path and never caller input.
type CraftRunViewRuntimeContainerSpec struct {
	Generation  string
	RuntimeID   string
	ContainerID string
	Directory   string
}

// CraftRunViewRuntimeContainer is the provider's observed identity for the
// generation-scoped container. A provider must implement
// InspectOrCreateContainer idempotently and must not replace a container for
// an existing generation after an uncertain result.
type CraftRunViewRuntimeContainer struct {
	Generation         string
	RuntimeID          string
	ContainerID        string
	Directory          string
	ProjectID          string
	IdentityVerified   bool
	DedicatedForRun    bool
	DirectoryCanonical bool
}

// CraftRunViewRuntimeSession is server-observed session metadata. Directory
// must be the OpenCode session's reported directory, not merely the request
// header that was sent during creation.
type CraftRunViewRuntimeSession struct {
	ID        string
	ProjectID string
	Directory string
}

// CraftRunViewRuntimeInventory is usable for recovery only when Authoritative
// is true, Directory and ProjectID match the inspected container, and Sessions
// contains the full cursor-paginated set for that exact OpenCode scope. A
// filtered, partial or unavailable inventory is not evidence.
type CraftRunViewRuntimeInventory struct {
	Authoritative bool
	Complete      bool
	Directory     string
	ProjectID     string
	Sessions      []CraftRunViewRuntimeSession
}

// CraftRunViewRuntimeProvider is the narrow boundary around the runtime
// provider. FindSessions must return every page from the pinned OpenCode
// GET /api/session endpoint scoped to the exact directory and project, with
// each Session.Info.location.directory and projectID preserved. It must not
// mark the inventory authoritative unless cursor pagination completed. The
// pinned source also defines GET /api/session/:sessionID for rechecking a
// selected session. These are upstream source routes, not routes verified in
// the checked-in legacy lock or a live binary. This interface is a contract
// only; it does not assert that any current provider implements it.
type CraftRunViewRuntimeProvider interface {
	InspectOrCreateContainer(context.Context, CraftRunViewRuntimeContainerSpec) (CraftRunViewRuntimeContainer, error)
	FindSessions(context.Context, CraftRunViewRuntimeContainer) (CraftRunViewRuntimeInventory, error)
	CreateSession(context.Context, CraftRunViewRuntimeContainer, string) error
}

// CraftRunViewRuntimeHandle is returned only after a persisted bound row has
// been reloaded and its runtime/session identity matched to authoritative
// provider evidence. It is an internal server-side handle, not an API DTO.
// Directory is the canonical in-container project directory for this view.
type CraftRunViewRuntimeHandle struct {
	View      craft.RunView
	Directory string
}

// CraftRunViewRuntimeCoordinator resolves one server-owned runtime generation
// to its unique verified OpenCode session. It is not wired into production
// assembly and does not itself establish OS-level isolation.
type CraftRunViewRuntimeCoordinator struct {
	store           craft.RunViewStore
	authority       craft.RunViewEffectAuthority
	provider        CraftRunViewRuntimeProvider
	privateRootBase string
}

// NewCraftRunViewRuntimeCoordinator creates a coordinator using an absolute,
// canonical, server-configured base directory inside the provider runtime.
// CraftRunViewAdmittedEffectProvider is the split, typed provider contract.
// Every observation is read-only and each send below is separately claimed by
// the coordinator. The legacy combined API cannot authorize this path.
type CraftRunViewAdmittedEffectProvider interface {
	CraftRunViewAdmittedSplitProvider
}

// NewCraftRunViewAdmittedRuntimeCoordinator creates a coordinator that only
// allocates from the original admitted Task and requires typed effect authority.
// It also keeps the legacy RunView store solely for immutable binding/reload; its
// key-only Allocate and create-intent permission are never used to authorize sends.
func NewCraftRunViewAdmittedRuntimeCoordinator(
	store craft.RunViewStore,
	authority craft.RunViewEffectAuthority,
	provider CraftRunViewRuntimeProvider,
	privateRootBase string,
) (*CraftRunViewRuntimeCoordinator, error) {
	if store == nil || authority == nil || provider == nil {
		return nil, fmt.Errorf("%w: missing admitted store, authority, or provider", ErrCraftRunViewRuntimeUnresolved)
	}
	if !validCraftRunViewDirectory(privateRootBase) || privateRootBase == "/" {
		return nil, fmt.Errorf("%w: private root base must be a canonical absolute path below root", ErrCraftRunViewRuntimeUnresolved)
	}
	return &CraftRunViewRuntimeCoordinator{store: store, authority: authority, provider: provider, privateRootBase: privateRootBase}, nil
}

func NewCraftRunViewRuntimeCoordinator(
	store craft.RunViewStore,
	provider CraftRunViewRuntimeProvider,
	privateRootBase string,
) (*CraftRunViewRuntimeCoordinator, error) {
	if store == nil || provider == nil {
		return nil, fmt.Errorf("%w: missing store or provider", ErrCraftRunViewRuntimeUnresolved)
	}
	if !validCraftRunViewDirectory(privateRootBase) || privateRootBase == "/" {
		return nil, fmt.Errorf("%w: private root base must be a canonical absolute path below root", ErrCraftRunViewRuntimeUnresolved)
	}
	return &CraftRunViewRuntimeCoordinator{store: store, provider: provider, privateRootBase: privateRootBase}, nil
}

// admittedRunViewKey derives the durable RunView key from the original
// admitted Task: the Task scope's owner/session plus the fenced Run identity.
func admittedRunViewKey(task craft.Task) craft.RunViewKey {
	return craft.RunViewKey{TenantID: task.Scope.TenantID, OwnerID: task.Scope.UserID, SessionID: task.Scope.SessionID, RunID: task.Fence.RunID}
}

// ResolveAdmittedVerifiedContainer (F08 T-1 bounded interface) resolves the
// admitted RunView and returns its LIVE-OBSERVED, admission-matched running
// container observation — the only identity source allowed for minting a
// provider handle — without driving the OpenCode session machinery. Every
// provider mutation still claims its durable effect first; a replay is
// observe-only and can never resend an unknown operation.
func (c *CraftRunViewRuntimeCoordinator) ResolveAdmittedVerifiedContainer(ctx context.Context, task craft.Task) (craft.RunView, CraftRunViewContainerObservation, error) {
	if ctx == nil || c == nil || c.authority == nil || c.store == nil || c.provider == nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("admitted coordinator is not initialized", nil)
	}
	if err := ctx.Err(); err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("admitted resolution canceled", err)
	}
	key := admittedRunViewKey(task)
	// The Task snapshot digest is the durable admitted Run identity: a raw
	// lowercase SHA-256 hex digest, exactly as AgentRunStore persists it and
	// CraftStore.PrepareTask / the effect store compare it.
	if err := craft.ValidateRunViewKey(key); err != nil || task.Fence.TenantID != key.TenantID || task.Fence.RunID != key.RunID ||
		task.Fence.Owner == "" || task.Fence.Epoch <= 0 || task.SnapshotDigestVersion != 1 ||
		!validSHA256Hex(task.SnapshotDigest) || task.Fence.SnapshotDigestVersion != task.SnapshotDigestVersion ||
		task.Fence.SnapshotDigest != task.SnapshotDigest {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("original admitted Task identity is incomplete or inconsistent", err)
	}

	view, err := c.authority.AllocateAdmitted(ctx, task)
	if err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, fmt.Errorf("%w: allocate admitted RunView: %w", ErrCraftRunViewRuntimeUnresolved, err)
	}
	if err := craft.ValidateRunView(view); err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("admitted allocation is invalid", err)
	}
	if view.Key != key {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("admitted allocation scope differs from original Task", nil)
	}
	provider, ok := c.provider.(CraftRunViewAdmittedEffectProvider)
	if !ok {
		// The legacy provider combines inspect, network/layout setup, Docker
		// create and Docker start. No single typed claim can safely authorize it.
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("provider does not separate observed state, Docker create, and Docker start; physical stage is fail-closed", nil)
	}

	spec := c.containerSpec(view.Generation)
	network, err := c.resolveAdmittedNetwork(ctx, task, provider, spec)
	if err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, err
	}
	container, err := c.resolveAdmittedContainer(ctx, task, provider, spec, network)
	if err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, err
	}
	container, err = c.resolveAdmittedContainerStart(ctx, task, provider, container)
	if err != nil {
		return craft.RunView{}, CraftRunViewContainerObservation{}, err
	}
	container, found, err := provider.ObserveContainerState(ctx, container)
	if err != nil || !found || container.State != "running" || !matchesAdmittedObservation(spec, network, container) {
		return craft.RunView{}, CraftRunViewContainerObservation{}, unresolvedCraftRunView("started container identity cannot be observed", err)
	}
	return view, container, nil
}

// ResolveAdmitted binds a runtime using the original server-admitted Task. Every
// provider mutation requires a matching durable claim immediately before the
// call; a replay is observe-only and can never resend an unknown operation.
func (c *CraftRunViewRuntimeCoordinator) ResolveAdmitted(ctx context.Context, task craft.Task) (CraftRunViewRuntimeHandle, error) {
	view, container, err := c.ResolveAdmittedVerifiedContainer(ctx, task)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	key := admittedRunViewKey(task)
	spec := c.containerSpec(view.Generation)
	provider, ok := c.provider.(CraftRunViewAdmittedEffectProvider)
	if !ok {
		// ResolveAdmittedVerifiedContainer already refused non-split providers.
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("provider does not separate observed state, Docker create, and Docker start; physical stage is fail-closed", nil)
	}
	if err := c.resolveAdmittedProbe(ctx, task, provider, container); err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	observed, err := c.observeAdmittedSession(ctx, provider, container)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	// BindRuntime requires this durable compatibility marker. It is only
	// bookkeeping. It must commit before claiming the external effect, and its
	// maySend result is never used to authorize a provider request.
	intentView, _, err := c.store.BeginSessionCreate(ctx, key, view.Generation)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("record session binding marker before effect claim", err)
	}
	if err := craft.ValidateRunView(intentView); err != nil || intentView.Key != key || intentView.Generation != view.Generation || intentView.SessionCreateIntentAt == nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("session binding marker differs from admitted generation", err)
	}
	view = intentView
	if err := ctx.Err(); err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("cancelled before OpenCode create claim", err)
	}
	claim, maySend, err := c.beginAdmittedEffect(ctx, task, view.Generation, craft.RunViewEffectOpenCodeCreate)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	if !maySend {
		if observed == nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("OpenCode create claim is already used and no session is observable", nil)
		}
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: observed.ID}); err != nil {
			return CraftRunViewRuntimeHandle{}, err
		}
	} else if observed != nil {
		// A complete, exact inventory is a sufficient receipt; do not create a
		// second session in a generation that already contains one.
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: observed.ID}); err != nil {
			return CraftRunViewRuntimeHandle{}, err
		}
	} else {
		created, createErr := provider.CreateOpenCodeSession(ctx, container, spec.Directory)
		observed, err = c.observeAdmittedSession(ctx, provider, container)
		if err != nil {
			_ = c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown})
			return CraftRunViewRuntimeHandle{}, err
		}
		if observed == nil {
			if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
				return CraftRunViewRuntimeHandle{}, finishErr
			}
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("OpenCode create has no exact observed session identity", createErr)
		}
		if created.ID != "" && (created.ID != observed.ID || created.ProjectID != observed.ProjectID || created.Directory != observed.Directory) {
			if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
				return CraftRunViewRuntimeHandle{}, finishErr
			}
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("OpenCode send and read-only session receipts differ", createErr)
		}
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: observed.ID}); err != nil {
			return CraftRunViewRuntimeHandle{}, err
		}
	}
	if observed == nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("OpenCode session identity is unresolved", nil)
	}
	return c.bindAndReload(ctx, key, view.Generation, container.Runtime, *observed)
}

func (c *CraftRunViewRuntimeCoordinator) observeAdmittedSession(
	ctx context.Context, provider CraftRunViewAdmittedEffectProvider, container CraftRunViewContainerObservation,
) (*CraftRunViewRuntimeSession, error) {
	inventory, err := provider.ObserveSessions(ctx, container)
	if err != nil {
		return nil, unresolvedCraftRunView("observe OpenCode sessions", err)
	}
	if !inventory.Authoritative || !inventory.Complete || inventory.Directory != container.Runtime.Directory || inventory.ProjectID != container.Runtime.ProjectID {
		return nil, unresolvedCraftRunView("provider session inventory is not complete and scoped to the inspected directory/project", nil)
	}
	if len(inventory.Sessions) > 1 {
		return nil, unresolvedCraftRunView("multiple sessions exist in the generation container", nil)
	}
	if len(inventory.Sessions) == 0 {
		return nil, nil
	}
	session := inventory.Sessions[0]
	if !validCraftRunViewToken(session.ID) || session.ProjectID != container.Runtime.ProjectID || session.Directory != container.Runtime.Directory {
		return nil, unresolvedCraftRunView("session project/directory affinity is not proven", nil)
	}
	return &session, nil
}

func (c *CraftRunViewRuntimeCoordinator) resolveAdmittedNetwork(
	ctx context.Context,
	task craft.Task,
	provider CraftRunViewAdmittedEffectProvider,
	spec CraftRunViewRuntimeContainerSpec,
) (CraftRunViewContainerNetwork, error) {
	want, err := admittedNetworkEffectSpec(provider, spec)
	if err != nil || !validRunViewNetworkSpec(want) {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("derive canonical network effect request", err)
	}
	digest, err := runViewNetworkRequestDigest(want)
	if err != nil {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("digest canonical network effect request", err)
	}
	observed, found, observeErr := provider.ObserveNetwork(ctx, spec)
	if observeErr != nil {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("observe generation network before create", observeErr)
	}
	if found && validateNetwork(want, observed) != nil {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("observed network identity differs from admitted generation", nil)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("cancelled before Docker network create claim", err)
	}
	claim, maySend, err := c.beginAdmittedEffectWithDigest(ctx, task, spec.Generation, craft.RunViewEffectDockerNetworkCreate, digest)
	if err != nil {
		return CraftRunViewContainerNetwork{}, err
	}
	if found {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewNetworkReceipt(observed)}); err != nil {
			return CraftRunViewContainerNetwork{}, err
		}
		return observed, nil
	}
	if !maySend {
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("network create claim is already used and the generation network is absent", nil)
	}
	if err := ctx.Err(); err != nil {
		if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
			return CraftRunViewContainerNetwork{}, finishErr
		}
		return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("cancelled after Docker network create claim", err)
	}
	created, createErr := provider.CreateGenerationNetwork(ctx, spec)
	observed, found, observeErr = provider.ObserveNetwork(ctx, spec)
	if observeErr == nil && found && validateNetwork(want, observed) == nil && (created.ID == "" || created.ID == observed.ID) {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewNetworkReceipt(observed)}); err != nil {
			return CraftRunViewContainerNetwork{}, err
		}
		return observed, nil
	}
	if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
		return CraftRunViewContainerNetwork{}, finishErr
	}
	return CraftRunViewContainerNetwork{}, unresolvedCraftRunView("Docker network create has no exact observed receipt", errors.Join(createErr, observeErr))
}

func (c *CraftRunViewRuntimeCoordinator) resolveAdmittedContainer(
	ctx context.Context,
	task craft.Task,
	provider CraftRunViewAdmittedEffectProvider,
	spec CraftRunViewRuntimeContainerSpec,
	network CraftRunViewContainerNetwork,
) (CraftRunViewContainerObservation, error) {
	wantNetwork, err := admittedNetworkEffectSpec(provider, spec)
	if err != nil || validateNetwork(wantNetwork, network) != nil {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("generation network evidence differs from canonical request", err)
	}
	container, exists, observeErr := provider.ObserveContainer(ctx, spec, network)
	if observeErr != nil {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("observe generation container before create", observeErr)
	}
	if exists && !matchesAdmittedObservation(spec, network, container) {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("observed container identity differs from admitted generation", nil)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("cancelled before Docker create claim", err)
	}
	claim, maySend, err := c.beginAdmittedEffect(ctx, task, spec.Generation, craft.RunViewEffectDockerCreate)
	if err != nil {
		return CraftRunViewContainerObservation{}, err
	}
	if exists {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewContainerReceipt(container)}); err != nil {
			return CraftRunViewContainerObservation{}, err
		}
		return container, nil
	}
	if !maySend {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("Docker create claim is already used and the generation container is absent", nil)
	}
	if err := ctx.Err(); err != nil {
		if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
			return CraftRunViewContainerObservation{}, finishErr
		}
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("cancelled after Docker create claim", err)
	}
	created, createErr := provider.CreateGenerationContainer(ctx, spec, network)
	observed, found, observeErr := provider.ObserveContainer(ctx, spec, network)
	createdIDMatches := created.DockerID == "" || created.DockerID == observed.DockerID
	if observeErr == nil && found && matchesAdmittedObservation(spec, network, observed) && createdIDMatches {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewContainerReceipt(observed)}); err != nil {
			return CraftRunViewContainerObservation{}, err
		}
		return observed, nil
	}
	if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
		return CraftRunViewContainerObservation{}, finishErr
	}
	return CraftRunViewContainerObservation{}, unresolvedCraftRunView("Docker create has no exact observed container receipt", errors.Join(createErr, observeErr))
}

func (c *CraftRunViewRuntimeCoordinator) resolveAdmittedContainerStart(
	ctx context.Context,
	task craft.Task,
	provider CraftRunViewAdmittedEffectProvider,
	container CraftRunViewContainerObservation,
) (CraftRunViewContainerObservation, error) {
	spec := container.RuntimeSpec()
	observed, found, err := provider.ObserveContainerState(ctx, container)
	if err != nil || !found || !matchesAdmittedObservation(spec, container.Network, observed) {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("observe container state before start", err)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("cancelled before Docker start claim", err)
	}
	claim, maySend, err := c.beginAdmittedEffect(ctx, task, spec.Generation, craft.RunViewEffectDockerStart)
	if err != nil {
		return CraftRunViewContainerObservation{}, err
	}
	if observed.State == "running" {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewStartReceipt(observed)}); err != nil {
			return CraftRunViewContainerObservation{}, err
		}
		return observed, nil
	}
	if !maySend {
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("Docker start claim is already used but the container is not running", nil)
	}
	if err := ctx.Err(); err != nil {
		if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
			return CraftRunViewContainerObservation{}, finishErr
		}
		return CraftRunViewContainerObservation{}, unresolvedCraftRunView("cancelled after Docker start claim", err)
	}
	started, startErr := provider.StartGenerationContainer(ctx, observed)
	current, currentFound, observeErr := provider.ObserveContainerState(ctx, observed)
	startedIDMatches := started.DockerID == "" || started.DockerID == current.DockerID
	if observeErr == nil && currentFound && current.State == "running" && startedIDMatches && matchesAdmittedObservation(spec, container.Network, current) {
		if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewStartReceipt(current)}); err != nil {
			return CraftRunViewContainerObservation{}, err
		}
		return current, nil
	}
	if err := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); err != nil {
		return CraftRunViewContainerObservation{}, err
	}
	return CraftRunViewContainerObservation{}, unresolvedCraftRunView("Docker start has no exact observed running receipt", errors.Join(startErr, observeErr))
}

func (c *CraftRunViewRuntimeCoordinator) resolveAdmittedProbe(
	ctx context.Context,
	task craft.Task,
	provider CraftRunViewAdmittedEffectProvider,
	container CraftRunViewContainerObservation,
) error {
	if container.State != "running" {
		return unresolvedCraftRunView("runtime probe requires a running observed container", nil)
	}
	pins, err := admittedProbeEffectPins(provider)
	if err != nil || !validSHA256Hex(pins.BinarySHA256) || !validSHA256Hex(pins.RuntimeConfigSHA256) || !validCraftRunViewToken(pins.OpenCodeVersion) {
		return unresolvedCraftRunView("expected runtime probe identity is unavailable", err)
	}
	digest, err := runViewProbeRequestDigest(container, pins)
	if err != nil {
		return unresolvedCraftRunView("digest canonical runtime probe request", err)
	}
	observed, found, observeErr := provider.ObserveRuntimeProbe(ctx, container)
	if observeErr != nil && !errors.Is(observeErr, ErrCraftRunViewRuntimeUnresolved) {
		return unresolvedCraftRunView("observe prior runtime probe", observeErr)
	}
	if found && !matchesAdmittedProbe(container.DockerID, pins, observed) {
		return unresolvedCraftRunView("observed runtime probe receipt differs from pinned request", nil)
	}
	if err := ctx.Err(); err != nil {
		return unresolvedCraftRunView("cancelled before runtime probe claim", err)
	}
	claim, maySend, err := c.beginAdmittedEffectWithDigest(ctx, task, container.Runtime.Generation, craft.RunViewEffectDockerProbe, digest)
	if err != nil {
		return err
	}
	if found {
		return c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewProbeReceipt(observed)})
	}
	if !maySend {
		return unresolvedCraftRunView("runtime probe claim is already used but read-only probe evidence is unavailable", observeErr)
	}
	if err := ctx.Err(); err != nil {
		if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
			return finishErr
		}
		return unresolvedCraftRunView("cancelled after runtime probe claim", err)
	}
	sent, sendErr := provider.SendRuntimeProbe(ctx, container)
	if sendErr == nil && matchesAdmittedProbe(container.DockerID, pins, sent) {
		return c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateSucceeded, Receipt: runViewProbeReceipt(sent)})
	}
	if finishErr := c.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown}); finishErr != nil {
		return finishErr
	}
	return unresolvedCraftRunView("runtime probe has no exact measured send receipt", errors.Join(sendErr, observeErr))
}

type craftRunViewAdmittedRequestIdentityProvider interface {
	admittedRunViewNetworkSpec(CraftRunViewRuntimeContainerSpec) CraftRunViewContainerNetworkSpec
	admittedRunViewProbePins() (string, string, string)
}

type craftRunViewProbePins struct {
	OpenCodeVersion     string
	BinarySHA256        string
	RuntimeConfigSHA256 string
}

func admittedNetworkEffectSpec(provider CraftRunViewAdmittedEffectProvider, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewContainerNetworkSpec, error) {
	if concrete, ok := provider.(*CraftRunViewContainerProvider); ok {
		return concrete.networkSpec(spec), nil
	}
	if identity, ok := provider.(craftRunViewAdmittedRequestIdentityProvider); ok {
		return identity.admittedRunViewNetworkSpec(spec), nil
	}
	return CraftRunViewContainerNetworkSpec{}, errors.New("provider does not expose immutable network request identity")
}

func admittedProbeEffectPins(provider CraftRunViewAdmittedEffectProvider) (craftRunViewProbePins, error) {
	if concrete, ok := provider.(*CraftRunViewContainerProvider); ok {
		return craftRunViewProbePins{OpenCodeVersion: concrete.config.OpenCodeVersion, BinarySHA256: concrete.config.OpenCodeBinarySHA256, RuntimeConfigSHA256: concrete.config.RuntimeConfigSHA256}, nil
	}
	if identity, ok := provider.(craftRunViewAdmittedRequestIdentityProvider); ok {
		version, binary, config := identity.admittedRunViewProbePins()
		return craftRunViewProbePins{OpenCodeVersion: version, BinarySHA256: binary, RuntimeConfigSHA256: config}, nil
	}
	return craftRunViewProbePins{}, errors.New("provider does not expose immutable probe identity")
}

func runViewNetworkRequestDigest(spec CraftRunViewContainerNetworkSpec) (string, error) {
	if !validRunViewNetworkSpec(spec) {
		return "", errors.New("network request identity is invalid")
	}
	request := struct {
		Domain     string            `json:"domain"`
		Name       string            `json:"name"`
		Driver     string            `json:"driver"`
		Internal   bool              `json:"internal"`
		Labels     map[string]string `json:"labels"`
		Scope      string            `json:"scope"`
		Attachable bool              `json:"attachable"`
		Ingress    bool              `json:"ingress"`
		Options    []string          `json:"options"`
	}{"weknora.craft.runview.network-create.v1", spec.Name, spec.Driver, spec.Internal, spec.Labels, "local", false, false, []string{}}
	return runViewCanonicalRequestDigest(request)
}

func runViewProbeRequestDigest(container CraftRunViewContainerObservation, pins craftRunViewProbePins) (string, error) {
	if container.DockerID == "" || container.DockerID == "\x00" {
		return "", errors.New("runtime probe target Docker ID is missing")
	}
	request := struct {
		Domain               string   `json:"domain"`
		ContainerID          string   `json:"container_id"`
		Command              []string `json:"command"`
		User                 string   `json:"user"`
		WorkingDirectory     string   `json:"working_directory"`
		BinaryPath           string   `json:"binary_path"`
		ConfigPath           string   `json:"config_path"`
		ExpectedVersion      string   `json:"expected_version"`
		ExpectedBinarySHA256 string   `json:"expected_binary_sha256"`
		ExpectedConfigSHA256 string   `json:"expected_config_sha256"`
	}{"weknora.craft.runview.runtime-probe.v1", container.DockerID, []string{"/bin/sh", "-c", "set -eu; opencode --version; sha256sum /usr/local/bin/opencode; sha256sum /etc/craft/runtime-config.json"}, craftRunViewContainerUser, "/", "/usr/local/bin/opencode", "/etc/craft/runtime-config.json", pins.OpenCodeVersion, pins.BinarySHA256, pins.RuntimeConfigSHA256}
	return runViewCanonicalRequestDigest(request)
}

func runViewCanonicalRequestDigest(request any) (string, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func runViewNetworkReceipt(network CraftRunViewContainerNetwork) string {
	encoded, _ := json.Marshal(network)
	return string(encoded)
}

func runViewContainerReceipt(container CraftRunViewContainerObservation) string {
	encoded, _ := json.Marshal(struct {
		DockerID  string `json:"docker_id"`
		NetworkID string `json:"network_id"`
	}{container.DockerID, container.Network.ID})
	return string(encoded)
}

func runViewStartReceipt(container CraftRunViewContainerObservation) string {
	return container.DockerID + "|running"
}

func runViewProbeReceipt(probe CraftRunViewRuntimeProbe) string {
	encoded, _ := json.Marshal(probe)
	return string(encoded)
}

func matchesAdmittedObservation(spec CraftRunViewRuntimeContainerSpec, network CraftRunViewContainerNetwork, observed CraftRunViewContainerObservation) bool {
	if !matchesAdmittedContainer(spec, observed.Runtime) || observed.DockerID == "" || observed.DockerID == "\x00" ||
		validateNetwork(CraftRunViewContainerNetworkSpec{Name: network.Name, Driver: network.Driver, Internal: network.Internal, Labels: network.Labels}, observed.Network) != nil {
		return false
	}
	return observed.NetworkAttachment.Name == network.Name && observed.NetworkAttachment.ID == network.ID && observed.NetworkAttachment.Internal && isPrivateIP(observed.NetworkAttachment.IP)
}

func matchesAdmittedProbe(containerID string, pins craftRunViewProbePins, probe CraftRunViewRuntimeProbe) bool {
	return probe.ContainerID == containerID && probe.ExecID != "" && probe.ExecID != "\x00" && probe.OpenCodeVersion == pins.OpenCodeVersion &&
		probe.BinarySHA256 == pins.BinarySHA256 && probe.RuntimeConfigSHA256 == pins.RuntimeConfigSHA256
}

func (c *CraftRunViewRuntimeCoordinator) beginAdmittedEffect(
	ctx context.Context, task craft.Task, generation string, kind craft.RunViewEffectKind,
) (craft.RunViewEffectClaim, bool, error) {
	claim, maySend, err := c.authority.BeginEffect(ctx, task, generation, kind)
	if err != nil {
		return craft.RunViewEffectClaim{}, false, fmt.Errorf("%w: claim %s: %w", ErrCraftRunViewRuntimeUnresolved, kind, err)
	}
	if claim.Key != admittedRunViewKey(task) || claim.Generation != generation || claim.Kind != kind || claim.Token == "" {
		return craft.RunViewEffectClaim{}, false, unresolvedCraftRunView("effect authority returned a claim for another Task, generation or kind", nil)
	}
	return claim, maySend, nil
}

func (c *CraftRunViewRuntimeCoordinator) beginAdmittedEffectWithDigest(
	ctx context.Context, task craft.Task, generation string, kind craft.RunViewEffectKind, digest string,
) (craft.RunViewEffectClaim, bool, error) {
	authority, ok := c.authority.(craft.RunViewEffectRequestAuthority)
	if !ok || !validSHA256Hex(digest) {
		return craft.RunViewEffectClaim{}, false, unresolvedCraftRunView("effect authority cannot bind the canonical request digest", nil)
	}
	claim, maySend, err := authority.BeginEffectWithDigest(ctx, task, generation, kind, digest)
	if err != nil {
		return craft.RunViewEffectClaim{}, false, fmt.Errorf("%w: claim %s: %w", ErrCraftRunViewRuntimeUnresolved, kind, err)
	}
	if claim.Key != admittedRunViewKey(task) || claim.Generation != generation || claim.Kind != kind || claim.Token == "" || claim.RequestDigest != digest {
		return craft.RunViewEffectClaim{}, false, unresolvedCraftRunView("effect authority returned a claim for another Task, generation, kind, or request digest", nil)
	}
	return claim, maySend, nil
}

func (c *CraftRunViewRuntimeCoordinator) finishAdmittedEffect(
	ctx context.Context, claim craft.RunViewEffectClaim, outcome craft.RunViewEffectOutcome,
) error {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := c.authority.FinishEffect(finishCtx, claim, outcome); err != nil {
		return fmt.Errorf("%w: finish %s: %w", ErrCraftRunViewRuntimeUnresolved, claim.Kind, err)
	}
	return nil
}

func matchesAdmittedContainer(spec CraftRunViewRuntimeContainerSpec, got CraftRunViewRuntimeContainer) bool {
	return got.Generation == spec.Generation && got.RuntimeID == spec.RuntimeID && got.ContainerID == spec.ContainerID &&
		got.Directory == spec.Directory && validCraftRunViewToken(got.ProjectID) && got.IdentityVerified &&
		got.DedicatedForRun && got.DirectoryCanonical
}

// Resolve allocates or loads the stable generation, deterministically resolves
// its container, observes session inventory, and binds only one session whose
// generation/container/directory metadata all match. BeginSessionCreate is the
// only path to a first CreateSession call. After an intent exists, resolution
// is observe-only; zero or ambiguous inventory stays unresolved.
func (c *CraftRunViewRuntimeCoordinator) Resolve(ctx context.Context, key craft.RunViewKey) (CraftRunViewRuntimeHandle, error) {
	view, err := c.store.Allocate(ctx, key)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	if err := craft.ValidateRunView(view); err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("stored allocation is invalid", err)
	}
	if view.Key != key {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("stored allocation scope differs from admitted Run", nil)
	}

	spec := c.containerSpec(view.Generation)
	container, err := c.provider.InspectOrCreateContainer(ctx, spec)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("inspect or create generation container", err)
	}
	if container.Generation != spec.Generation || container.RuntimeID != spec.RuntimeID ||
		container.ContainerID != spec.ContainerID || container.Directory != spec.Directory ||
		!validCraftRunViewToken(container.ProjectID) ||
		!container.IdentityVerified || !container.DedicatedForRun || !container.DirectoryCanonical {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("provider container identity does not match generation", nil)
	}

	observed, err := c.observeSession(ctx, container)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	if view.State == craft.RunViewStateBound {
		if observed == nil || view.Runtime != runtimeForSession(container, *observed) {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("bound runtime does not match unique observed session", nil)
		}
		return c.reloadBound(ctx, key, view.Generation, container, *observed)
	}

	if observed == nil {
		if view.SessionCreateIntentAt != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("session create was already attempted but no session is observable", nil)
		}
		intentView, maySend, err := c.store.BeginSessionCreate(ctx, key, view.Generation)
		if err != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("claim one-shot session-create intent", err)
		}
		if err := craft.ValidateRunView(intentView); err != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("stored create intent is invalid", err)
		}
		if intentView.Generation != view.Generation || intentView.Key != key {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("create intent changed generation or scope", nil)
		}
		if !maySend {
			return c.reconcileAfterLostCreatePermission(ctx, key, intentView, container)
		}
		if intentView.SessionCreateIntentAt == nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("store granted permission without durable intent", nil)
		}
		view = intentView
		if err := ctx.Err(); err != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("cancelled after durable create intent", err)
		}
		if err := c.provider.CreateSession(ctx, container, spec.Directory); err != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("CreateSession outcome is unknown", err)
		}
		observed, err = c.observeSession(ctx, container)
		if err != nil {
			return CraftRunViewRuntimeHandle{}, err
		}
		if observed == nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("CreateSession returned but no session is observable", nil)
		}
	}

	// Existing unique session evidence may be recovered without a new create,
	// but the durable marker is still required before BindRuntime.
	if view.SessionCreateIntentAt == nil {
		intentView, _, err := c.store.BeginSessionCreate(ctx, key, view.Generation)
		if err != nil {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("record intent before binding observed session", err)
		}
		view = intentView
	}
	return c.bindAndReload(ctx, key, view.Generation, container, *observed)
}

func (c *CraftRunViewRuntimeCoordinator) reconcileAfterLostCreatePermission(
	ctx context.Context,
	key craft.RunViewKey,
	view craft.RunView,
	container CraftRunViewRuntimeContainer,
) (CraftRunViewRuntimeHandle, error) {
	observed, err := c.observeSession(ctx, container)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, err
	}
	if observed == nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("another caller owns or completed session creation but no session is observable", nil)
	}
	if view.State == craft.RunViewStateBound {
		if view.Runtime != runtimeForSession(container, *observed) {
			return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("concurrent binding differs from observed session", nil)
		}
		return c.reloadBound(ctx, key, view.Generation, container, *observed)
	}
	if view.SessionCreateIntentAt == nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("losing caller received no durable create intent", nil)
	}
	return c.bindAndReload(ctx, key, view.Generation, container, *observed)
}

func (c *CraftRunViewRuntimeCoordinator) observeSession(
	ctx context.Context,
	container CraftRunViewRuntimeContainer,
) (*CraftRunViewRuntimeSession, error) {
	inventory, err := c.provider.FindSessions(ctx, container)
	if err != nil {
		return nil, unresolvedCraftRunView("observe OpenCode sessions", err)
	}
	if !inventory.Authoritative || !inventory.Complete || inventory.Directory != container.Directory || inventory.ProjectID != container.ProjectID {
		return nil, unresolvedCraftRunView("provider session inventory is not complete and scoped to the inspected directory/project", nil)
	}
	if len(inventory.Sessions) > 1 {
		return nil, unresolvedCraftRunView("multiple sessions exist in the generation container", nil)
	}
	if len(inventory.Sessions) == 0 {
		return nil, nil
	}
	session := inventory.Sessions[0]
	if !validCraftRunViewToken(session.ID) || session.ProjectID != container.ProjectID ||
		session.Directory != container.Directory {
		return nil, unresolvedCraftRunView("session project/directory affinity is not proven", nil)
	}
	return &session, nil
}

func (c *CraftRunViewRuntimeCoordinator) bindAndReload(
	ctx context.Context,
	key craft.RunViewKey,
	generation string,
	container CraftRunViewRuntimeContainer,
	session CraftRunViewRuntimeSession,
) (CraftRunViewRuntimeHandle, error) {
	runtime := runtimeForSession(container, session)
	if err := craft.ValidateRunViewRuntime(runtime); err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("observed runtime identity is invalid", err)
	}
	if _, err := c.store.BindRuntime(ctx, key, generation, runtime); err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("bind verified runtime identity", err)
	}
	return c.reloadBound(ctx, key, generation, container, session)
}

func (c *CraftRunViewRuntimeCoordinator) reloadBound(
	ctx context.Context,
	key craft.RunViewKey,
	generation string,
	container CraftRunViewRuntimeContainer,
	session CraftRunViewRuntimeSession,
) (CraftRunViewRuntimeHandle, error) {
	view, err := c.store.Load(ctx, key)
	if err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("reload bound RunView", err)
	}
	if err := craft.ValidateRunView(view); err != nil {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("reloaded RunView is invalid", err)
	}
	if view.State != craft.RunViewStateBound || view.Generation != generation || view.Key != key ||
		view.Runtime != runtimeForSession(container, session) {
		return CraftRunViewRuntimeHandle{}, unresolvedCraftRunView("persisted runtime differs from provider evidence", nil)
	}
	return CraftRunViewRuntimeHandle{View: view, Directory: container.Directory}, nil
}

func (c *CraftRunViewRuntimeCoordinator) containerSpec(generation string) CraftRunViewRuntimeContainerSpec {
	digest := sha256.Sum256([]byte(generation))
	short := hex.EncodeToString(digest[:16])
	return CraftRunViewRuntimeContainerSpec{
		Generation:  generation,
		RuntimeID:   "rv-runtime-" + short,
		ContainerID: "rv-container-" + short,
		Directory:   path.Join(c.privateRootBase, "rv-"+short),
	}
}

func runtimeForSession(container CraftRunViewRuntimeContainer, session CraftRunViewRuntimeSession) craft.RunViewRuntime {
	return craft.RunViewRuntime{RuntimeID: container.RuntimeID, ContainerID: container.ContainerID, OpenCodeSessionID: session.ID}
}

func validCraftRunViewDirectory(value string) bool {
	return value != "" && path.IsAbs(value) && path.Clean(value) == value &&
		!strings.Contains(value, "\\") && !strings.ContainsRune(value, '\x00')
}

func validCraftRunViewToken(value string) bool {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func unresolvedCraftRunView(stage string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", ErrCraftRunViewRuntimeUnresolved, stage)
	}
	return fmt.Errorf("%w: %s: %v", ErrCraftRunViewRuntimeUnresolved, stage, cause)
}
