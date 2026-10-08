package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/Tencent/WeKnora/internal/craft"
)

// CraftRunViewMaterialHandle is an opaque, server-only capability for the
// material directories belonging to one persisted RunView generation. Its
// host paths are intentionally unexported and cannot be supplied by callers.
type CraftRunViewMaterialHandle struct {
	key        craft.RunViewKey
	generation string
	directory  string
	root       string
	inputs     string
	knowledge  string
	output     string
	provider   *CraftRunViewContainerProvider
	layoutID   []byte
}

// MaterialHandle returns host material paths only after reloading the exact
// admitted Run binding, re-inspecting the current container and runtime probe,
// and rechecking the exact bound OpenCode session.
func (p *CraftRunViewContainerProvider) MaterialHandle(
	ctx context.Context,
	store craft.RunViewStore,
	admittedKey craft.RunViewKey,
	runtime CraftRunViewRuntimeHandle,
) (CraftRunViewMaterialHandle, error) {
	if p == nil || store == nil || p.engine == nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("material provider or RunView store is missing", nil)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("material handle request canceled", err)
	}
	if err := craft.ValidateRunViewKey(admittedKey); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("admitted Run identity is invalid", err)
	}
	if err := craft.ValidateRunView(runtime.View); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("runtime handle carries an invalid persisted RunView", err)
	}
	if runtime.View.Key != admittedKey || runtime.View.State != craft.RunViewStateBound {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("runtime handle is not bound to the admitted Run", nil)
	}

	stored, err := store.Load(ctx, admittedKey)
	if err != nil || !reflect.DeepEqual(stored, runtime.View) {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("persisted RunView differs from supplied runtime handle", err)
	}
	generationSpec := craftRunViewSpecForGeneration(stored.Generation)
	if err := p.validateSpec(generationSpec); err != nil || runtime.Directory != generationSpec.Directory {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("runtime identity is not derived from the persisted generation", err)
	}
	if err := p.verifyBoundGenerationLive(ctx, stored); err != nil {
		return CraftRunViewMaterialHandle{}, err
	}
	return p.materialHandleFromBoundView(ctx, store, admittedKey, stored)
}

// MaterialHandleForCapture re-issues the generation material for one already
// terminal Run's post-terminal draft capture. It verifies the same persisted
// bound RunView, generation-derived runtime identity and immutable host
// layout identity as MaterialHandle, but intentionally does not require a
// running container, live OpenCode session or runtime probe: the Run is
// already terminal, the receipt's SQL fences proved no writer remains, and
// the sealed generation output lives on the host independently of the
// container lifecycle.
func (p *CraftRunViewContainerProvider) MaterialHandleForCapture(
	ctx context.Context,
	store craft.RunViewStore,
	admittedKey craft.RunViewKey,
) (CraftRunViewMaterialHandle, error) {
	if p == nil || store == nil || p.engine == nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("material provider or RunView store is missing", nil)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("capture material handle request canceled", err)
	}
	if err := craft.ValidateRunViewKey(admittedKey); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("admitted Run identity is invalid", err)
	}
	view, err := store.Load(ctx, admittedKey)
	if err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("load persisted RunView for capture", err)
	}
	if err := craft.ValidateRunView(view); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("persisted RunView is invalid", err)
	}
	if view.Key != admittedKey || view.State != craft.RunViewStateBound {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("persisted RunView is not bound to the admitted Run", nil)
	}
	return p.materialHandleFromBoundView(ctx, store, admittedKey, view)
}

// materialHandleFromBoundView derives the host layout material for a
// durably bound generation: the runtime binding must be exactly the
// generation-derived deterministic identity, and the on-disk layout must
// verify against its immutable identity bytes.
func (p *CraftRunViewContainerProvider) materialHandleFromBoundView(
	ctx context.Context,
	store craft.RunViewStore,
	admittedKey craft.RunViewKey,
	view craft.RunView,
) (CraftRunViewMaterialHandle, error) {
	spec := craftRunViewSpecForGeneration(view.Generation)
	if err := p.validateSpec(spec); err != nil ||
		view.Runtime.RuntimeID != spec.RuntimeID ||
		view.Runtime.ContainerID != spec.ContainerID {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("runtime identity is not derived from the persisted generation", err)
	}
	if err := ctx.Err(); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("material handle request canceled after verification", err)
	}

	layout := generationLayoutPaths(p.config.SandboxRoot, spec)
	if err := verifyGenerationLayout(layout, spec); err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("verified generation material layout is unsafe or changed", err)
	}
	layoutID, err := os.ReadFile(layout.identity)
	if err != nil {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("verified generation identity could not be captured", err)
	}
	root, err := filepath.EvalSymlinks(layout.root)
	if err != nil || root != layout.root || !pathWithin(p.config.SandboxRoot, root) {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("generation material root is noncanonical or escaped sandbox root", err)
	}
	return CraftRunViewMaterialHandle{
		key: admittedKey, generation: spec.Generation, directory: spec.Directory,
		root: root, inputs: layout.inputs, knowledge: layout.knowledge, output: layout.output,
		provider: p, layoutID: append([]byte(nil), layoutID...),
	}, nil
}

// verifyBoundGenerationLive re-derives the live engine evidence for a bound
// generation: the private network, the marker/layout-pinned running container,
// the pinned runtime probe and the exact bound OpenCode session.
func (p *CraftRunViewContainerProvider) verifyBoundGenerationLive(
	ctx context.Context,
	view craft.RunView,
) error {
	spec := craftRunViewSpecForGeneration(view.Generation)
	// Verify the bound generation through the durable split observation chain
	// instead of the legacy in-memory binding registry: the network, the
	// marker/layout-pinned container facts and the private endpoint are all
	// re-derived from the engine, so a restarted process reaches the same
	// conclusion without legacy composite operations.
	network, found, err := p.ObserveNetwork(ctx, spec)
	if err != nil {
		return err
	}
	if !found {
		return unresolvedCraftRunView("bound generation network is not observable", nil)
	}
	observation, found, err := p.ObserveContainer(ctx, spec, network)
	if err != nil {
		return err
	}
	if !found || observation.State != "running" {
		return unresolvedCraftRunView("bound generation container is not observable and running", nil)
	}
	// Re-verify the pinned runtime binary/config probe exactly as the legacy
	// binding verification did; the admitted coordinator has already claimed
	// and sent the probe for this generation before dispatch.
	probe, probeErr := p.engine.ProbeRuntime(ctx, observation.DockerID)
	if probeErr != nil || !p.validProbe(probe) {
		return unresolvedCraftRunView("runtime binary or baked config is not verified", probeErr)
	}
	bindingAPI, err := p.sessionAPIForObservation(observation)
	if err != nil {
		return err
	}
	session, err := bindingAPI.GetSession(ctx, view.Runtime.OpenCodeSessionID)
	if err != nil || session.ID != view.Runtime.OpenCodeSessionID ||
		session.ProjectID != p.config.ProjectID || session.Location.Directory != spec.Directory {
		return unresolvedCraftRunView("persisted OpenCode session identity is stale or foreign", err)
	}
	return ctx.Err()
}

// verifyMaterialHandle re-establishes that a previously issued handle still
// names the same provider-derived generation layout. The stored identity bytes
// make a valid replacement layout at the same path fail even if it uses the
// same generation token and carries a newly generated manifest.
func (p *CraftRunViewContainerProvider) verifyMaterialHandle(handle CraftRunViewMaterialHandle) error {
	if p == nil || handle.provider != p || len(handle.layoutID) == 0 {
		return unresolvedCraftRunView("material handle was not issued by this provider", nil)
	}
	spec := craftRunViewSpecForGeneration(handle.generation)
	if err := p.validateSpec(spec); err != nil {
		return unresolvedCraftRunView("material handle generation is invalid", err)
	}
	layout := generationLayoutPaths(p.config.SandboxRoot, spec)
	if handle.directory != spec.Directory || handle.root != layout.root || handle.inputs != layout.inputs ||
		handle.knowledge != layout.knowledge || handle.output != layout.output {
		return unresolvedCraftRunView("material handle paths differ from the generation-derived layout", nil)
	}
	if err := verifyGenerationLayout(layout, spec); err != nil {
		return unresolvedCraftRunView("current generation layout is unsafe or changed", err)
	}
	identity, err := os.ReadFile(layout.identity)
	if err != nil || !bytes.Equal(identity, handle.layoutID) {
		return unresolvedCraftRunView("current generation layout differs from the issued material handle", err)
	}
	root, err := filepath.EvalSymlinks(layout.root)
	if err != nil || root != layout.root || root != handle.root || !pathWithin(p.config.SandboxRoot, root) {
		return unresolvedCraftRunView("generation root is noncanonical or escaped sandbox root", err)
	}
	return nil
}

// RevalidateMaterialHandle rechecks the issued generation layout and the live
// engine inspection immediately before delegate dispatch. In particular, the
// inspected bind mount list must still name this generation's knowledge path
// with its read-only flag intact.
func (p *CraftRunViewContainerProvider) RevalidateMaterialHandle(
	ctx context.Context,
	handle CraftRunViewMaterialHandle,
) error {
	if err := ctx.Err(); err != nil {
		return unresolvedCraftRunView("material revalidation canceled", err)
	}
	if err := p.verifyMaterialHandle(handle); err != nil {
		return err
	}
	spec := craftRunViewSpecForGeneration(handle.generation)
	container := CraftRunViewRuntimeContainer{
		Generation: spec.Generation, RuntimeID: spec.RuntimeID, ContainerID: spec.ContainerID,
		Directory: spec.Directory, ProjectID: p.config.ProjectID,
		IdentityVerified: true, DedicatedForRun: true, DirectoryCanonical: true,
	}
	if _, err := p.currentBinding(ctx, container); err != nil {
		return err
	}
	return ctx.Err()
}

func craftRunViewSpecForGeneration(generation string) CraftRunViewRuntimeContainerSpec {
	digest := sha256.Sum256([]byte(generation))
	short := hex.EncodeToString(digest[:16])
	return CraftRunViewRuntimeContainerSpec{
		Generation:  generation,
		RuntimeID:   "rv-runtime-" + short,
		ContainerID: "rv-container-" + short,
		Directory:   path.Join("/workspace", "rv-"+short),
	}
}
