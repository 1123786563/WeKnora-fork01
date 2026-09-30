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

	"github.com/Tencent/WeKnora/internal/modules/craft"
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
	spec := craftRunViewSpecForGeneration(runtime.View.Generation)
	if err := p.validateSpec(spec); err != nil ||
		runtime.View.Runtime.RuntimeID != spec.RuntimeID ||
		runtime.View.Runtime.ContainerID != spec.ContainerID ||
		runtime.Directory != spec.Directory {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("runtime identity is not derived from the persisted generation", err)
	}

	container := CraftRunViewRuntimeContainer{
		Generation: spec.Generation, RuntimeID: spec.RuntimeID, ContainerID: spec.ContainerID,
		Directory: spec.Directory, ProjectID: p.config.ProjectID,
		IdentityVerified: true, DedicatedForRun: true, DirectoryCanonical: true,
	}
	binding, err := p.currentBinding(ctx, container)
	if err != nil {
		return CraftRunViewMaterialHandle{}, err
	}
	session, err := binding.sessions.GetSession(ctx, runtime.View.Runtime.OpenCodeSessionID)
	if err != nil || session.ID != runtime.View.Runtime.OpenCodeSessionID ||
		session.ProjectID != p.config.ProjectID || session.Location.Directory != spec.Directory {
		return CraftRunViewMaterialHandle{}, unresolvedCraftRunView("persisted OpenCode session identity is stale or foreign", err)
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
