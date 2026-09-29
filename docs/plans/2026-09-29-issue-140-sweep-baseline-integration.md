# Issue 140 / issue30-sweep Baseline Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the complete committed issue30-sweep result as an ancestor of the #140 delivery while reconciling the overlapping mini-program and Workbench Artifact contracts without losing either branch's security or behavior.

**Architecture:** Create a true merge of `db234c5eb171f2dde7427d382b55b503a038f879` into the #140 integration line at `9bc0d93685d9809cbfa01afd5e9aaa0ba4a4288c`. Keep non-conflicting merged content intact, then resolve the two disjoint conflict groups in separate worktrees: mini-program file/session adapters, and server Workbench Artifact/container/migration/architecture contracts. Integrate only after each task's validator and independent reviewer approve its exact commit.

**Tech Stack:** Git merge/worktrees, Go, TypeScript, Taro 4, Node test runner, SQLite migration tests.

**Spec:** `docs/plans/issue-140/2026-09-24-issue-140-dag.md`; #140 approved snapshot and #142/#143/#145/#148/#164/#166/#171/#173 child snapshots in `docs/plans/issue-140/issues/`; `docs/specs/2026-09-23-weknora-job-search-design.md`; ADRs 0015–0018; `CONTEXT.md`; issue30-sweep committed final report at `docs/plans/issue30-sweep/FINAL-REPORT.md`.

## Global Constraints

- Merge the verified issue30-sweep HEAD `db234c5eb171f2dde7427d382b55b503a038f879`; do not use `main`, `origin/main`, or a default branch as the new base.
- Preserve all existing issue30-sweep committed behavior and the five pre-existing untracked files in `.worktrees/issue30-sweep` without editing, moving, or importing those untracked files.
- Keep #140's authenticated Task access, tenant/owner checks, scope fencing, immutable Artifact-version grants and revocation semantics.
- Keep issue30-sweep's refresh-once bearer-token behavior, mobile-office/session/task behavior, terminal availability contract, Artifact digest/version listing, and architecture migration intent.
- Do not weaken tests or choose an entire branch side for a conflict when that drops the other side's required behavior.
- Implementers may commit local task changes. No push, shared-branch merge, deployment, publishing, or GitHub mutation.

## Review Focus

- A stale or expired bearer token must refresh once while Artifact download scope checks and cleanup remain enforced; cover in Task 1.
- Deleting the issue30 `services/workbench.ts` must not remove #140's Artifact operations or resurrect obsolete Task/session ownership; cover in Task 1 assembly/service tests.
- Terminal data must only be advertised when its reader is wired, while Artifact version listing/download remains available and revocable; cover in Task 2 handler/container tests.
- The migration test must migrate from the intended historical version to the actual latest schema and back without hard-coding a stale latest version; cover in Task 2 database tests.
- Architecture manifest and route-count expectations must describe the merged router/container graph; cover in Task 2 architecture guard test.

---

## Baseline and Merge Checkpoint

- Current #140 line: `codex/issue-140-integration`, HEAD `9bc0d93685d9809cbfa01afd5e9aaa0ba4a4288c`.
- Required issue30-sweep line: `codex/issue30-mobile-office`, HEAD `db234c5eb171f2dde7427d382b55b503a038f879`.
- Shared ancestor: `f7753fa160927195e388c65e1dbbdff7e288506c`; neither tip contains the other.
- Execution worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-sweep-integration/WeKnora-fork01`, branch `codex/issue-140-sweep-integration`.
- The merge is in progress. Auto-merged paths are retained. The 11 conflicted paths are partitioned below. Seed checkpoint resolution selects the #140 side only as a temporary compilation/integration checkpoint; task agents must restore the issue30 behavior in their disjoint owned paths before that checkpoint can be treated as reviewed or shippable.

## Task 1: Mini-program protected file and Workbench service merge

**Dependency:** Merge checkpoint commit containing the full db234 parent and all auto-merged paths; no dependency on Task 2 code.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files owned:**
- `apps/miniprogram/src/platform/files.ts`
- `apps/miniprogram/src/services/workbench.ts`
- `apps/miniprogram/tests/assembly.test.mjs`
- `apps/miniprogram/tests/core.test.mjs`
- `apps/miniprogram/tests/helpers/taro-stub.mjs`

**Consumes:** issue30 `currentBearerToken()` / refresh-once behavior and `mobile-office.ts` / `session.ts` ownership at `db234c5`; #140 Artifact service exports currently in `services/workbench.ts` at `9bc0d93`.

**Produces:**
- `listTaskArtifacts(runId: string): Promise<TaskArtifact[]>` and `taskArtifactDownloadPath(runId: string, item: Pick<TaskArtifact, 'index'>): Promise<string>` remain exported by `services/workbench.ts` for the existing artifact screen; the source remains Artifact-only and does not reclaim the removed Task/session service ownership.
- `files.ts` keeps issue30 bearer refresh-once, then enforces #140 active-scope identity before/after awaits, private `USER_DATA_PATH` staging, normalized access errors and cleanup.
- Mobile office Task/session API ownership stays in issue30 `mobile-office.ts` and `session.ts`; do not restore the deleted legacy `services/workbench.ts` Task/session layer.

**Steps:**

- [ ] Inspect stage-2 (`9bc0d93`) and stage-3 (`db234c5`) versions, callers, and tests. Record the retained exports and scope/token ordering in the task report.
- [ ] Add/adjust focused regression tests for refresh-once plus scope-switch rejection/private-file cleanup, and for both Artifact download API entry points remaining reachable through Career service assembly.
- [ ] Run those focused tests and capture expected failing assertions before implementation.
- [ ] Implement a focused Artifact module at `services/workbench.ts` only if still needed by callers; keep it Artifact-only and delegate auth/session/task work to the issue30 services. Compose the protected file download behavior without weakening either side's guards.
- [ ] Run `pnpm --filter @weknora/miniprogram test` and `pnpm --filter @weknora/miniprogram build:weapp`; report any pre-existing typecheck failures separately from owned-file failures.
- [ ] Run `git diff --check`, self-review the five owned files, commit with message `fix(miniprogram): preserve issue30 session and career artifact flows`, and provide exact BASE/HEAD, test output, and changed-file list.

**Acceptance:** Focused regressions and mini-program suite/build pass; the tests prove session/task behavior remains assembled from issue30 services and Career Artifact download remains scope-bound; no files outside ownership change.

## Task 2: Workbench Artifact server contract and migration integration

**Dependency:** Same merge checkpoint; no dependency on Task 1 code.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Files owned:**
- `internal/container/workbench.go`
- `internal/handler/session/workbench_artifacts.go`
- `internal/database/semantic_migration_test.go`
- `docs/architecture/moves/workbench.yaml`
- `tools/architectureguard/discovery_test.go`

**Consumes:** Existing #140 `WithArtifactVersions(versions)` and `WithArtifactVersionRevoker(versions)` handler options at `9bc0d93`; issue30 `WithTerminalLog(snapshots)` and terminal/version digest listing behavior at `db234c5`; current migration registry and route graph in the merge checkpoint.

**Produces:** `NewWorkbenchArtifactHandler(runs, refs).WithTerminalLog(reader).WithArtifactVersions(versions).WithArtifactVersionRevoker(versions)` composes all three optional capabilities. List responses retain `terminal.available`, digest and version metadata; signed version reads still bind authenticated tenant, owner, run and revocation state. Container wiring passes both terminal and Artifact-version stores. Architecture manifest is the union of valid merged paths; expected route counts derive from the merged registered routes. Migration test validates rollback to 106 and migrates to the computed current head.

**Steps:**

- [ ] Compare stage-2/stage-3 handler constructors, options and container call sites; inspect all migration versions and registered Workbench routes.
- [ ] Add/adjust regression tests for terminal availability plus immutable artifact version listing/grant/revoke and for migrate-106 → latest → rollback/reapply.
- [ ] Run focused tests to capture expected failing assertions before implementation.
- [ ] Resolve handler/container wiring and response fields while preserving all authorization and revocation checks. Union the architecture manifest and derive, rather than guess, route expectations from the merged graph. Make migration tests select the actual latest registered version dynamically and exercise rollback.
- [ ] Run `go test -count=1 ./internal/handler/session ./internal/container ./internal/database ./tools/architectureguard` and `git diff --check`.
- [ ] Self-review only the five owned paths, commit with message `fix(workbench): preserve sweep and career artifact contracts`, and provide exact BASE/HEAD, test output, and changed-file list.

**Acceptance:** Focused tests and all four package suites pass; Workbench Artifact terminal/read/version/revocation behavior is covered; migrations prove up/down/up at the merged schema head; no files outside ownership change.

## Parallelism, Integration, and Failure Handling

- Task 1 and Task 2 are ready concurrently after the merge checkpoint: their owned paths, test processes and generated outputs do not overlap. They use independent worktrees and local commits. No shared test database or server is used.
- Main controller records each agent type/model, BASE/HEAD and reports, then dispatches validators and reviewers independently against each task commit. No task is integrated before both checks pass.
- Integrate Task 2 then Task 1 by cherry-picking the reviewed commits into this merge branch; verify no unintended files, then run both acceptance command sets together and relevant mobile/backend checks. The final commit must have `db234c5eb171f2dde7427d382b55b503a038f879` as an ancestor.
- If an implementation uncovers an incompatible public seam, stop the affected task, update this plan before code changes, and keep the other task moving only if its interfaces remain independent.
- The final OCR must review the new full range from the issue30-sweep base through the final merge/fix HEAD; do not reuse the earlier 76df-based reports as proof of this corrected range.
