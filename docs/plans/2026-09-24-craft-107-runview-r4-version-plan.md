# T01 RunView R4: Workspace continuation and Run-bound collection

> **For Codex:** Execute sequential SDD tasks with RED → GREEN → REFACTOR, exact uncommitted checkpoints, and independent read-only review. This plan does not enable the production runtime.

**Sources:** approved `docs/specs/2026-09-23-craft-web-artifact-spec.md` (#107/T01); `CONTEXT.md`; ADR-0004/0008/0009; `2026-09-23-craft-107-runview-runtime-adapter-plan.md`; `2026-09-24-craft-107-runview-r3r4-design.md`; R3 Task 1/fix 1 reviews; #107 DAG. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Before dispatch, record live HEAD, status, owned-file hashes, and task-local baseline. No commits.

**Ruling — supersedes the earlier R4 seed assumption:** The approved Spec requires first-release edits to continue from the current persistent Workspace. A failed, stopped, or unknown Run may retain a repairable Workspace draft while the last successful Version stays the default preview. Seeding every Run from the last promoted Version would discard that draft and create a T01 → T15 → T01 cycle because T15 follows T04/T14 and owns promotion. The R4 seed recommendation in the prior adapter/design notes is superseded here. Historical-Version restore/edit remains deferred.

## Global Constraints

- The admitted `(tenant, owner, session, Run)`, writer fence, and verified RunView generation determine every host path. A client, model, or OpenCode response supplies neither root nor predecessor identity.
- The Workspace owns a durable **current editable revision**. Before B executes, admission freezes its exact predecessor revision and digest-sealed file snapshot. Retry/restart uses that predecessor even if later state changes. Initial Run has an explicit empty predecessor. A missing/unknown predecessor fails closed; no fallback to `VersionStore.List()[0]`, a promoted Version, or a previous Run directory.
- Seed only predecessor output files into B's generation-private writable `output/`. Do not copy or mount A's Run directory, HOME, OpenCode state, `inputs/`, or `knowledge/`. B's selected inputs/knowledge remain exact to B.
- Persist B's resulting editable revision/draft under its writer fence independently of candidate collection and default promotion. A quiescent failed/stopped Run may retain draft files; an unknown Run retains its fence until authoritative recovery establishes a safe snapshot. T15 separately owns the four-check default promotion pointer.
- Collection reads only the verified current generation's output. No process-global mutable root, prompt-hash directory, shared `output` symlink, or `workDir` fallback may participate.

## Review Focus

Frozen predecessor and writer fence; Workspace scope; manifest digest, path, size and file type; retry idempotence; draft retention without false success; symlink/hard-link and list/read replacement; same-Task cross-Run substitution; V1 immutable bytes and default pointer during B failure/unknown; no shared pointer. Fake provider tests prove Go-side bounds only. Real Linux mount and A→B acceptance remain release gates.

## Task 1 — freeze predecessor and seed current output

**Depends on:** R3 material Task 2 and fix 1 independently reviewed PASS, plus stable `CraftRunViewMaterialHandle`. **Owner:** `backend_implementer`. **Validator:** `backend_validator`. **Owned production file:** `internal/container/craft_runtime.go` after R3 releases it. **Owned tests:** focused `internal/container/craft_runtime*_test.go` file(s) assigned exclusively. If durable Workspace revision/snapshot selection is absent, scope a preceding application/repository task with separate ownership; this task cannot pass with in-memory selection. **Consumes:** admitted Task/Run and writer fence, exact durable predecessor revision (or explicit empty), digest-sealed file snapshot, `FileService.GetFile` where objects are external, verified material handle. **Produces:** exact predecessor files in this generation's `output/` before prompt.

1. RED: initial Run has empty output. A fails with draft D1; B seeds D1 and can repair it. If V1 was promoted before failed A, B still seeds A's retained draft while V1 remains default preview. A later promotion/revision change cannot change B's frozen predecessor. Wrong tenant/workspace/fence, missing predecessor, malformed/duplicate paths, bad bytes/size/digest, quota overflow, symlink component, pre-existing extra output, and stale/foreign material handle reject before prompt. Identical retry is byte-identical; partial seed safely resumes from the frozen snapshot or fails closed. B writes cannot mutate D1's sealed snapshot or V1 objects.
2. Resolve predecessor through admitted Workspace authority. Verify revision identity, scope, canonical output-relative paths (`craft.ValidateArtifactPath`), regular-file type, byte count, SHA-256, and bounds. Fetch only pinned objects. Prepare using no-follow directory/file operations and same-directory temporary files plus atomic rename. Verify the complete output tree. Never treat a missing predecessor as empty output.
3. Run focused RED/GREEN tests, focused race where appropriate, `gofmt -d`, and `git diff --check`. Save commands/exits, owned-file hashes, task-local patch, and independent Spec/code-quality review before Task 2.

**Failure handling:** Missing durable revision/snapshot seam or unsafe provider root blocks R4. Keep runtime disabled; do not add a promoted-Version fallback.

## Task 2 — bind collection and draft capture to the Run

**Depends on:** Task 1 reviewed PASS and integrated. **Owner:** `backend_implementer`. **Validator:** `backend_validator`. **Owned production file:** `internal/container/craft_runtime.go` serially after Task 1. `internal/application/service/craft_artifacts.go` is separately owned only if a per-Run source cannot safely satisfy the existing `SandboxArtifactSource` contract. **Owned tests:** focused container source/collection tests, plus service tests only for a changed contract. **Consumes:** verified handle, B Task/fence, existing `CraftArtifactService.CollectForKind` and `SandboxArtifactSource`. **Produces:** B-bound list/read source, sealed editable Workspace revision/draft, and an immutable candidate for later T15 promotion.

1. RED: B-bound source rejects A despite same Task session, foreign tenant, stale generation, changed container/mount/layout identity, traversal, symlink, hard link/device, missing root, extra file, and list/read replacement. Upload or output-validation failure cannot advance default. Failed/stopped/unknown B retains V1 as default; when safely quiescent it may retain draft D2 that C seeds. B success yields a candidate; only T15's four-check gate can make V2 default. V1 bytes/history remain unchanged. Assert absence of shared output pointer and prompt-hash lookup.
2. Construct the source for the exact verified Run at collection time; compare session, Run and generation before list/read. Use no-follow opens and file identity checks; accept only regular files under `handle.output`. Remove `pointWorkspaceOutput` from this path. Persist a sealed current editable revision under the writer fence when the Run is authoritatively quiescent, including a failed/stopped Run with repairable files. Do not release an unknown writer fence or expose an unverified partial snapshot. Preserve bounded collection, upload-before-candidate-publication, manifest and evidence behavior. If needed, add the smallest per-Run factory/interface change, never a global mutable source.
3. Run focused source/collector tests, race, formatting and diff checks. Save exact checkpoint/hash and independent Spec plus quality review. Attribute unrelated whole-package failures without claiming a package pass.

**Failure handling:** Changed identity between list/read fails before candidate publication. A partial object upload is subject to existing deferred reclamation. Unknown execution remains fenced pending reconciliation. No shared-source fallback.

## Task 3 — integrated acceptance and handoff gate

**Depends on:** Tasks 1–2 reviewed PASS; R2 binding, R3 material, T05 accepted package/ACL handoff, real engine, pinned Linux image, and central assembly. **Owner:** central assembly owner under a separate reviewed plan. **Writable files in this R4 plan:** none beyond its report.

Run A→B on one Task with distinct Run directories/sessions. After A fails with draft D1, B sees D1 plus only B/K_B; it cannot read A/K_A, A HOME/state, sibling root, or write read-only mounts. In a second sequence V1 remains default while B fails and leaves D2; C continues from D2. B success yields a candidate, and V2 becomes default only under T15's build/entry/preview/page-load gate. Restart before prompt and collection retains the frozen predecessor and Run binding. Verify actual mounts and no egress; record commands and deployment evidence.

**Release decision:** Keep the production RunView dial off until integrated acceptance, independent reviews, and complete OCR coverage pass. Existing R3 scoped evidence does not establish R4 or production completion.
