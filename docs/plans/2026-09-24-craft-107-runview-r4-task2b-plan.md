# Craft #107 T01 R4 Task 2b Candidate and Quiescent Draft Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist a Run-private immutable candidate and a repairable Workspace draft from a verifiably quiescent terminal Run, while preserving the prior default Version until T15's four-check promotion.

**Architecture:** Split collection/upload from `CraftArtifactService.CollectForKind` publication. A separately injected Run-bound source can create a private candidate record without touching `CraftVersionStore.Publish`/List/Get. A durable post-terminal capture coordinator records exact output bytes/refs and advances the current editable draft only after the Run is terminal, the writer slot is empty, and all tool/delegation and sandbox writers are quiescent. An outbox/recovery scan handles terminal-commit-to-capture crashes. T15 later promotes the candidate after build, entry, preview reachability and page-load evidence, using the same manifest digest.

**Tech Stack:** Go/Gorm, paired PG/SQLite migrations, content-addressed file storage, Run-bound no-follow source, deterministic crash/race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r4-task2b-architecture.md`; R4 Task2a Fix1 plan/review; approved Craft #107 T01/W01/T15; ADR-0008/0009.

## Global Constraints

- Candidate, current editable draft, and default published Version are distinct durable authorities. No call to `VersionStore.Publish` or default-pointer update in Tasks 1–3. `CollectForKind` legacy behavior remains for existing callers and is not silently repurposed.
- Run/generation/source/frozen predecessor are server owned. No shared output symlink, session-wide source, previous Run directory, client/model path or in-memory-only callback can define the capture.
- An active, waiting, recovering or unknown Run never advances draft. Terminal delegation is not terminal Run. A canceled/failed Run may advance only after authoritative no-writer/quiescence proof; another Run cannot bypass pending capture and lose its repairable predecessor.
- Object upload precedes sealed row. Same Run/revision/content replays idempotently; changed bytes, digest, generation or predecessor conflict. Orphans use deferred reclamation. No commit, deploy or production dial until integrated gates.
- Reserve migration SQLite `000122`/PG `000201` for private candidate and SQLite `000123`/PG `000202` for capture receipts. These follow R5 reserved pairs; do not reuse its numbers.

## Review Focus

- V1 remains default/List/Get while B fails or has an unpromoted candidate; A/B generation swap never changes candidate bytes.
- A failed/stopped, truly quiescent B can leave D2 for C to seed; unknown or pending tool cannot.
- Terminal transaction crash, upload-before-seal crash, seal-before-Advance crash and Advance-before-receipt crash all replay without a second version or silent draft loss.
- Same candidate digest drives T15's future checks; its existence is not preview success.

---

### Task 1: Private candidate collection and store

**Depends on:** R4 Task1 frozen predecessor reviewed PASS and the Task2b architecture report; this application/repository task uses an injected fake source and is independent of Task2a Fix1, which gates only Task3 wiring. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000122_craft_candidate.{up,down}.sql`, `migrations/versioned/000201_craft_candidate.{up,down}.sql`, `internal/modules/craft/candidate.go`, `internal/application/repository/craft_candidate.go` and focused tests, `internal/application/service/craft_artifacts.go` and focused artifact/candidate tests. No `container.go` or `craft_runtime.go`.

**Consumes / produces:** Explicit per-call `SandboxArtifactSource` and admitted Task/kind; produces immutable private candidate with `(tenant, owner, session, workspace, run, generation, manifest digest, files, checks/evidence)` identity, not a published `craft.Version`.

- [ ] Capture exact preimage. RED: injected A source cannot be silently reused for B; source nil or wrong Run fails; upload and validation errors leave no candidate; same exact bytes replay same row; changed bytes under same Run/generation conflict; candidate is absent from `VersionStore.List/Get` and default preview while V1 remains intact.
- [ ] Extract bounded list/read/validate/upload logic into a private helper shared by legacy `CollectForKind` and new `CollectCandidate(ctx, task, kind, source, verifiedGeneration)`. Require a server-owned source identity interface that reports its bound Run and generation, then compare both with the admitted Task/verified generation before any read or upload; a generic session-only `SandboxArtifactSource` is insufficient because A and B share a session. The container Task3 must implement the identity on its Run-bound source. Keep per-file/total/path/manifest/evidence rules. Do not store a mutable source on the service for the new route.
- [ ] Persist candidate in a private immutable table with scoped GET/unique key and file manifest. Derive content ID from Workspace, Run and digest; include generation and scope in a non-mutable row. Atomic insert after all referenced objects exist; identical replay adopts stored row, any mismatch rejects. T15 receives a typed candidate interface only, with no publish side effect.
- [ ] Run focused service/repository and paired migration up/down/up tests, gofmt/diff, exact checkpoint/patch/report and independent review.

**Acceptance / failure handling:** A candidate is durable and private; no default/history change. If current `VersionStore` has an implicit default side effect beyond Publish/List, prove the candidate path avoids it.

### Task 2: Durable post-terminal draft capture and replay

**Depends on:** Task1 independently reviewed PASS and frozen predecessor Task1 R4. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `migrations/sqlite/000123_craft_run_capture.{up,down}.sql`, `migrations/versioned/000202_craft_run_capture.{up,down}.sql`, new `internal/application/repository/craft_run_capture.go`/tests, new `internal/application/service/craft_run_capture.go`/tests, narrow `internal/application/repository/agent_run_lifecycle.go` terminal/outbox integration and focused tests. No container source files.

**Consumes / produces:** Authoritative terminal Run, empty writer slot, no pending tool/delegation, frozen predecessor, exact candidate/draft file refs and opaque verified-generation source contract; produces sealed capture receipt and `DraftHeadStore.Advance` idempotently.

- [ ] RED: terminal failed/canceled Run with quiescent output advances D2; active/waiting/unknown, pending tool/delegation or live sandbox writer does not. Later Run admission cannot pass an unresolved predecessor. Crash at each terminal/enqueue, upload/seal, Advance/receipt boundary replays; different bytes/Run/generation/predecessor conflicts.
- [ ] Write a durable receipt keyed by tenant/workspace/Run, recording predecessor revision/digest, generation identity, state and sealed manifest/ref set. Terminal transitions enqueue transactionally or leave a recovery-scan-detectable missing receipt. Recheck all quiescence facts under current authority immediately before `Advance`; never infer from delegation `Execute` return.
- [ ] On retry, use sealed refs rather than rereading mutable files. If `Advance` succeeded but receipt failed, compare `ReadRevision` and current head source Run/digest before adopting; reject nonidentical conflict. Keep unknown fenced and do not admit C before B's capture is resolved.
- [ ] Focused real SQLite/PG transaction/crash tests, migration up/down/up, race where relevant, checkpoint/report and independent review.

**Acceptance / failure handling:** Repairable draft survives process crash without replacing V1. If authoritative sandbox quiescence cannot be demonstrated by the injected source contract, remain pending and report interface block.

### Task 3: Wire Run source, terminal trigger and recovery

**Depends on:** Tasks 1–2 and R4 Task2a Fix1 independently reviewed PASS, plus R5 effect-fence Task3/real generation identity. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go`, focused RunView tests, central worker/assembly hook files granted serially after their current owners release them.

**Consumes / produces:** Task2a verified per-Run source, candidate collector, capture coordinator/outbox. Successful `Execute` stages only a candidate; terminal worker/recovery drives draft capture with generation and quiescence proof. No `pointWorkspaceOutput` or singleton collector fallback.

- [ ] RED A→B/C same Task sequence with distinct Run generation: A failed D1, B sees D1 plus only B inputs/knowledge; V1 stays default when B fails D2; C sees D2. Success creates candidate but no default. Restart before prompt/collection retains frozen identity.
- [ ] Wire source factory to candidate staging and terminal/recovery capture. Revalidate exact Run/generation and quiescence at each boundary; handle unknown as fenced. Keep output evidence/checks bound to candidate digest.
- [ ] Focused/race and real Linux mount acceptance, exact checkpoint/review. T15 four-check promotion and full OCR remain subsequent gates.

**Acceptance / failure handling:** Missing real generation/quiescence proof keeps production dial off and the Run unresolved; never publish from active Execute.
