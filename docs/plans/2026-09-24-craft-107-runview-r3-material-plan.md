# T01 RunView R3: verified material boundary

> **For Codex:** Execute sequential SDD tasks with RED→GREEN→REFACTOR, exact uncommitted checkpoints and independent read-only review after each task. This plan creates the material boundary; actual Linux mount and Run A→B acceptance remain release gates.

**Sources:** approved Craft Spec #107/T01 and T05, ADR-0004/0008/0009, `2026-09-24-craft-107-runview-r3r4-design.md`, R1/provider fix1 and R2 independent reviews. Integration original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record live HEAD and pre-task owned-file content/hashes. No commits.

## Global Constraints

Only persisted tenant/owner/session/Run and verified RunView generation choose the host layout; model/client paths never do. Material preparation precedes Prompt. `inputs/` and `knowledge/` are exact selected read-only mounts, `output/` is writable for this Run alone. Existing `localCraftRuntime.workDir` and shared output symlink are not authorization. A missing/unknown container, stale generation or inspected mount mismatch fails closed. Do not claim production isolation from fake-engine tests.

## Review Focus

Host path substitution, symlink components, cross-Run same-Task handle, container reinspection, exact input manifest and bytes/digest, empty manifest, extra files, Run-bound knowledge package seal, rollback after partial preparation, output seed isolation.

## Task 1 — verified generation material handle

**Depends on:** R1/provider fix1 and R2 scoped Review PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runview_container_provider.go`, its focused tests, and a new `craft_runview_material.go/_test.go` only if needed. **Consumes:** persisted `CraftRunViewRuntimeHandle` and provider's deterministic spec/inspected engine facts. **Produces:** opaque server-only generation-bound material handle with canonical host inputs/knowledge/output layout; no caller-supplied root.

1. RED tests: valid bound generation returns its own layout; same Task different Run produces different roots; forged generation/container/session, stale/missing container, changed mount/network/image/probe, symlinked root child and unbound runtime are rejected before a path is handed to a writer.
2. Implement one provider method that reloads or validates the bound identity, re-inspects full container facts, derives host paths from `SandboxRoot+Generation`, checks each path against canonical root and no symlink, and returns an opaque handle. Keep host paths internal to `internal/container`; do not put them in API DTOs or prompts.
3. Focused provider/material tests and race checks, `gofmt`, `git diff --check`; save exact checkpoint/hash/report for independent review.

**Acceptance:** a material writer cannot acquire a host root from an unverified or foreign RunView. **Failure handling:** if `CraftRunViewRuntimeHandle` lacks binding proof, stop and request the smallest store/coordinator seam rather than trusting a string.

## Task 2 — exact selected input staging

**Depends on:** Task1 reviewed PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runtime.go`, focused runtime/material tests and Task1 material file if its API must be extended. **Consumes:** immutable admitted input manifest and current tenant/owner/workspace association, verified material handle, `FileService`. **Produces:** exact digest-sealed `inputs/` tree under current generation, ready for inspected read-only mount.

RED tests include empty manifest, delegated extra, revoked association, corrupt bytes/size/digest, unsafe filename, symlink parent/target, extra pre-existing file, cross-Run handle and identical retry. GREEN writes via no-follow same-directory temp+rename, validates full directory and rejects extra objects. Do not write under shared `workDir`. Focused tests, exact checkpoint and independent review.

## Task 3 — knowledge handoff and read-only mount proof

**Depends on:** Task2 reviewed PASS; T05 durable KB selection/package and ACL dispatch reviewed; real engine adapter available. **Owner:** backend_implementer coordinated with T05. **Validator:** backend_validator. **Owned files:** assigned after shared assembly ownership release.

Bind T05's accepted RunID+digest package to the same verified generation; validate seal and exact paths and mount it read-only with current source authority. Missing/partial/foreign package fails closed. Live engine mount inspection and Run A→B evidence are required before production enablement.
