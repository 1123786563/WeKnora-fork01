# Craft #107 Web Artifact Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved Craft web artifact journey across T00–T20 with authoritative Task/Run state, bounded offline execution, immutable evidence and versions, access control, recovery, and auditable export.

**Architecture:** Keep `Task ID = Session ID` and one persistent Workspace; each Run is one execution within that Task. The Craft domain module owns rules and typed facts, application services enforce authorization and durable transitions, handlers translate HTTP, and the React workbench projects server authority. T00 freezes additive seams and T20 alone composes the lanes into central assembly.

**Tech Stack:** Go/Gin/GORM, React/TypeScript, TDesign, pnpm/tsx, Playwright, isolated OpenCode sandbox.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; issue snapshot `docs/plans/2026-09-23-craft-107-issue-snapshot.json`; DAG `docs/plans/2026-09-23-craft-107-dag.md`; `docs/adr/0004-task-is-session.md`; `CONTEXT.md`; execution prompt `docs/plans/2026-09-23-craft-web-artifact-issues-execution-prompt.md`.

## Global Constraints

- Preserve `Task ID = Session ID`; Run is an execution inside Task; one Task has one persistent Workspace and one primary web artifact.
- Web release accepts any extension within 20 MiB per file, 20 files per round, and 100 MiB per round; accepted and understood are distinct facts.
- Uploaded code is read-only input, never directly executed; archive extraction is bounded and atomic.
- Use selected **and** currently authorized knowledge; source open always checks the current viewer's grant.
- Fixed template and provisioned dependencies build offline; both build and isolated preview deny external and internal egress.
- A version is immutable and becomes default only after independent build, entry, preview reachability, and actual page-load checks pass.
- One Workspace-writing Run at a time; unknown stop/lease/execution outcomes cannot be treated as success or release a fence.
- Private by default; Owner, Collaborator, Viewer have server-enforced roles; restricted sharing and derived-data export need distinct owner consent bound to immutable digests.
- Charge Lead Agent and sandbox work to the same Task Budget; exhaustion durably pauses work.
- Existing assistant-ui projects conversation; WeKnora is authoritative for Task, Run, decisions, version, permission and budget. Use approved TDesign direction for new or migrated controls.
- No public hosting, non-web artifact expansion, historical-version restore/edit, uploaded-code execution, general egress, or independent assistant-ui Task state.
- No local commits were authorized. Use per-task uncommitted checkpoints with full content/hashes and reviewed incremental packages; verify integration before downstream dispatch.

## Review Focus

- Unknown or executable-looking upload within quota remains accepted but unparsed; a decision gate prevents silent use or accidental execution (T01/T03 tests).
- Archive path normalization, special files and partial extraction must fail before any material is visible to a Run (T02 tests).
- A revoked Viewer or changed evidence digest must not retain source, preview, share or export authority through an old URL/decision (T08/T10/T11/T13/T14 tests).
- An unknown cancellation or writer acquisition result must retain the writer fence and leave the previous successful version visible (T16/T17 tests).
- Reconnect with duplicate/gapped/expired events must reload authority and never submit a second Run (T18 tests).

---

## Shared file, interface and resource preflight

| Seam or resource | Sole writer / rule | Consumer contract and scheduling guard |
|---|---|
| `internal/modules/craft/contracts.go`, HTTP DTO/error mapping, `packages/contracts/src/craft/index.ts` | T00 freezes additive public facts; T20 resolves any later central change | Every lane consumes versioned T00 facts; proposed changes go to controller before dispatch. No lane edits these files. |
| `internal/handler/session/craft.go`, route assembly, `internal/container/*` central wiring | T00 creates extension slots; T20 mounts integrated features | Lane-owned handler and service files register through slots. Only T20 changes central route/wiring after lane review. |
| `packages/views/src/craft/workbench.tsx`, main feature composition | T00 creates typed slots; T20 composes | Feature lanes own separate panels/controllers and tests; no concurrent write to main workbench. |
| `internal/database/migration.go`, schema numbers and migration files | T20 assigns and integrates numbering; lane agents submit schema requirements | No parallel migration number allocation. Database tests use per-agent database names. |
| Artifact DTO and evidence manifest | T00 establishes additive shape; T06/T07/T12/T13 own separate typed payloads behind it | Digest identity and immutable Version ID are frozen before downstream work. |
| Sandbox image/config, network namespace and ports | T04 owns build runtime; T14 owns preview policy, in separate isolated Worktrees | No shared sandbox container, port, mutable cache, or network fixture. Reconcile runtime digest and network policy before T15. |
| Writer lease and stop/reconnect/budget state | T16 owns lease storage and fence; T17/T18/T19 consume outcome types | T19 can start at first frontier only in its own budget files; any shared lifecycle edit is routed through T20. |
| Full browser journey and acceptance matrix | T20 only | Lane tests use scoped fixtures, namespaced ports/accounts and independent browser output dirs. |

**Execution correction (recorded in Ledger):** The controller remains the sole writer of shared migration numbering and central assembly. When a Ticket's acceptance requires durable schema or a live central hook, the controller schedules a scoped supporting Task after the lane defines its interface and before that Ticket is marked verified. This avoids making a native predecessor of T20 depend on T20 for its own verification. T20 still owns the final cross-lane journey and any remaining assembly corrections; no feature Agent may independently edit shared files.

### Pairwise shared-file and interface check

| Tasks | Shared file or interface | Preflight result |
|---|---|---|
| T00/T20 | `packages/views/src/craft/workbench.tsx`, central handler/container, public contracts | Ordered by the full DAG; T00 freezes slot signatures, T20 alone composes them after reviewed lanes integrate. |
| T05/T06 | `packages/views/src/craft/sources.tsx`, actual-source manifest | Native edge T05→T06; T05 establishes the projection and T06 adds citation rendering against that reviewed version. |
| T07/T15 | `internal/application/service/craft_artifacts.go`, immutable version evidence | Native edge T15→T07; T15 owns promotion, T07 pins historical evidence after its checkpoint is integrated. |
| T16/T17 | `internal/modules/craft/lifecycle.go`, writer/stop state | Native edge T16→T17; T16 freezes the lease/fence and T17 extends stop/draft transitions after verification. |
| T01/T02/T03 | Immutable input ref and quota accounting | Native T01 predecessor for T02 and T03; T02/T03 use separate archive/code modules and separate fixtures once T01 is verified. |
| T04/T14 | Build and preview sandbox network boundary | No native edge; T14 owns preview config and T04 owns build runtime. Run them in separate Worktrees/containers and reconcile the two policies before T15. |
| T05/T08/T14 | `TaskAccessChecker` authority interface | These become parallel only after T00 freezes an injected signature. T08 implements Task membership; T05/T14 consume the interface through separate adapters and must not edit T08 source files. |
| T06/T07/T12/T13 | Source evidence and export manifest | Native chain orders related writes. Historical evidence stays keyed to Version ID and digest; T12/T13 consume immutable records without rewriting T06/T07 payloads. |
| T15/T16/T17/T18/T19 | Run phase, writer fence and budget pause | T15→T16→T17/T18 are native edges; T19 is independent but owns only budget adapter/pause files and consumes T00 typed facts. Shared lifecycle assembly is reserved to T20. |

### Internal Task consistency check

| Task | Test/code/ownership consistency before dispatch |
|---|---|
| T00 | Contract tests require additive parser defaults; slot tests target central files solely owned by T00. |
| T01 | API upload/decision assertions map to `input.go`, `craft_inputs.go` and the dedicated input panel; no archive behavior. |
| T02 | Archive fixtures and staging tests map to archive-only modules; the T01 quota ledger is an input. |
| T03 | Provenance and execution-denial tests map to input-code/delegate adapter; no T02 archive edits. |
| T04 | Offline build tests map to build template/runtime, with no preview-serving edits. |
| T05 | Selection/authorization/source-record tests map to knowledge policy and adapter; T08 supplies the membership implementation. |
| T06 | Citation/inference tests consume T05 source record and T04 generated page; T05's source panel changes occur after T05 verification. |
| T07 | Historical evidence tests consume T06/T15 version facts and extend artifact service only after T15. |
| T08 | Membership tests map to new access policy/service/handler, leaving central routes for T20. |
| T09 | Collaborator admission tests consume reviewed T08 roles and T16 lease, with no role-policy rewrite. |
| T10 | Viewer source-open tests consume T05 records and T08 current role checks. |
| T11 | Restricted-share consent tests consume T06 contribution and T08 Owner authority. |
| T12 | Bundle/manifest tests consume T07 immutable evidence and T10 viewer-specific source links. |
| T13 | Derived-data export tests consume T11 consent rules and T12 fixed manifest. |
| T14 | Origin/capability/no-egress tests map to preview-only files and isolated browser/network fixture. |
| T15 | Four-check promotion tests consume T04 build and T14 preview, without writer-lease changes. |
| T16 | Concurrent-run tests own lease repository and lifecycle file before T17 changes it. |
| T17 | Stop/draft tests extend lifecycle only after T16 review and integration. |
| T18 | Reconnect tests consume T16 authority and own controller/reconnect projection. |
| T19 | Budget admission/pause tests own budget adapter, with no global billing redesign or lease edits. |
| T20 | End-to-end journey composes reviewed lanes and solely owns shared assembly/migrations/browser suite. |

## Execution and evidence contract

After T00's reviewed checkpoint is integrated, compute the frontier. Dispatch T01/T05/T08/T14/T19 concurrently when each has a separate Worktree, disjoint files, stable interfaces and isolated test resources. Recompute immediately on integration; resource conflict leaves a node ready with a recorded lock and deterministic next dispatch, not a fabricated DAG edge. Each task report records input integrated SHA, owned files, checkpoint hash, exact checks/results, Spec compliance and code-quality review; T20 alone integrates all verified lanes. The parent controller maintains `docs/plans/2026-09-23-craft-107-ledger.md` and runs whole-scope OCR after T20.

**Interface freeze gate:** Existing `craft.Check` is generic and `CraftInputView` has no recognition fields. Existing TypeScript `CRAFT_RUN_STATUSES` does not include a paused value, while #138 requires durable budget pause. T00 must record the exact additive wire representation and parser fallback for all three before T01, T15 or T19 is dispatched. The current `craft.go` route registration uses package globals; T00's feature slot must preserve auth wrapping and make duplicate registration/order deterministic. A typed slot without an assembly adapter is insufficient. The approved first release excludes historical-version restore/edit even though older Craft snapshot routes exist; T20 must keep that route outside the new web-artifact journey.

### Task 0: T00 — Freeze web-artifact contracts and parallel extension seams (#119)

**Depends on:** none. **Owner:** architect-guided backend/frontend implementer, sole Wave W0 writer. **Validator:** backend and frontend validators. **Acceptance:** all eight #119 checkboxes in the DAG; no user-visible behavior change. **Failure handling:** if additive DTO parsing breaks current clients or a route slot bypasses auth, revert that T00 checkpoint, keep downstream nodes pending, and correct the seam before dispatch.

**Files:** Modify `internal/modules/craft/contracts.go`, `internal/handler/session/craft.go`, `packages/contracts/src/craft/index.ts`, `packages/views/src/craft/workbench.tsx`; create focused contract/slot tests and `docs/plans/2026-09-23-craft-107-ownership.md`. Container seam may use a new `internal/container/craft_features.go`; central `internal/container/workbench.go` is T00-only during this step. T00 may split contract additions into `internal/modules/craft/web_contracts.go` and `packages/contracts/src/craft/web-artifact.ts` with re-exports through the frozen public files.

**Interfaces — Consumes:** Existing `craft.Scope`, `craft.Input`, `craft.Check`, `craft.Version`, `craft.ErrUnknown`, `CraftRunView`, `CraftInputView`, `CraftVersionView`, and unchanged current HTTP routes. **Produces:** typed, additive `InputRecognition{accepted,understood,reason}`, independent `WebCheckEvidence{build,entry,preview_reachable,page_loaded}` using `passed|failed|not_run`; `StopOutcome{requested,confirmed,unknown}`; `WriterAcquireOutcome{acquired,conflict,unknown}`; typed `BudgetPause`, `RestrictedContribution`, `ShareDecision`, `ExportManifest`, `ExportDecision`; backend keyed feature route/service slots mounted under existing guards; typed workbench feature slots. Exact Go/TS names and JSON spellings are frozen in the T00 report and copied into each downstream Brief before dispatch. Unknown values reject at the parser; omitted new fields on legacy responses retain compatibility defaults.

- [ ] **Step 1 — RED, contract facts:** Add focused Go and TS tests proving accepted-but-unrecognized input, four independent check outcomes, stop/lease unknown, typed budget and consent records, malformed JSON rejection, and legacy payload compatibility. Assert each fact through the public Craft contract seam, not private helpers.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft/... -run 'Contract|Web|Input|Stop|Budget' -count=1` and `pnpm test:craft:shared`. Expected: the new assertions fail on absent facts or extension slots, while pre-existing cases still run.
- [ ] **Step 3 — GREEN, additive contracts:** Define discriminated Go and TypeScript types with explicit invariants; keep existing fields and error mappings unchanged. Add parser coverage for absent optional fields and invalid enumerants. Do not encode consent or pause in free-form `Check.Detail`.
- [ ] **Step 4 — RED, extension assembly:** Add handler/container and workbench tests mounting two named features in distinct slots. Assert duplicate names fail, deterministic order, nil or unavailable features fail closed, inherited tenant/session guards remain active, and old routes/workbench projection are unchanged.
- [ ] **Step 5 — GREEN, slots:** Add a keyed registration/composition seam whose input is a feature name and route/panel adapter; freeze registration before serving or rendering. Keep concrete feature construction in the container and authorization in the service/handler; no feature package reaches into another lane's internals.
- [ ] **Step 6 — REFACTOR and ownership:** Remove repeated ad-hoc fact strings only where the new types directly replace them. Write `docs/plans/2026-09-23-craft-107-ownership.md` listing T00/T20 central files, lane-owned extensions, migration-number owner and shared test locks. Copy the final exported Go and TS field names, JSON spellings, error mapping, and defaulting rules into the T00 report.
- [ ] **Step 7 — Verify GREEN:** Run `go test ./internal/modules/craft/... -count=1`, `go test ./internal/application/service/... -run Craft -count=1`, `go test ./internal/handler/session/... -run Craft -count=1`, `pnpm test:shared`, and `pnpm typecheck:web`. Expected: all exit 0; route-slot auth test and legacy parse test pass; no new central behavior is visible. Record exact output and checkpoint hash for independent Review.

### Task 1: T01 — Input recognition and decision (#120)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #120 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/input.go; internal/application/service/craft_inputs.go; packages/views/src/craft/files.tsx; matching `*_test.go`/`.test.tsx`. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 `InputRecognition` and typed workbench input slot. **Produces:** Persist immutable input ref plus accepted/understood state and one continue/cancel command.

**Acceptance checks:**

- Unknown extension is accepted within 20 MiB/file, 20 files/round, and 100 MiB/round quotas.
- Accepted and understood are persisted and projected as separate outcomes.
- Executable binary content is not misclassified as harmless parsed text.
- Cancel submits no Run; continue submits exactly once and retains the opaque read-only reference.
- Empty, oversized, over-count, over-total, digest-mismatch, and hostile-name inputs fail closed.
- Content-addressed identity and read-only materialization remain intact.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: unknown extension within quotas accepted/unrecognized; binary not parsed; continue once; cancel zero Runs; empty, hostile name, digest mismatch and all quota edges rejected. Name the Go test `TestCraftT01Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT01Journey -count=1`. Expected: `TestCraftT01Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Implement quota and digest checks at upload seam; store opaque recognition separately from acceptance; require a server-acknowledged decision before Run admission; project warning with TDesign control. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT01Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Input -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftInput|Attachment' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash. **证据命令勘误（2026-09-25，T01 残余推进）**：该 selector 只命中既有附件测试（TestStageSessionAttachments*/TestBuildUserHistoryMessage*），匹配不到 `TestCraftT01*` 命名，对 T01 套件无回归鉴别力（曾掩盖恢复路径红灯）。T01 套件的判定命令以 Step 4 的 `-run TestCraftT01Journey` 为准，并必须附加 `go test ./internal/application/service -run 'TestCraftT01' -count=1`（8 个用例，含恢复路径）。
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Attachment|CraftInput' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any submitted Run after cancel or duplicate Run after continue blocks verification; fix admission idempotency before integration. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 2: T02 — Bounded atomic archive input (#121)

**Depends on:** T01 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #121 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/archive.go; internal/application/service/craft_archive.go; internal/modules/craft/testdata/archive/*; focused `*_test.go`. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T01 immutable input ref and round quota ledger. **Produces:** Atomic expanded input refs or typed rejection; no partial material.

**Acceptance checks:**

- Normal archives expand into content-addressed read-only inputs.
- Absolute paths, traversal, normalized duplicates, symlinks, hard links, devices, FIFOs, and special files are rejected.
- Expanded bytes, file count, nesting depth, compression ratio, CPU, and memory are bounded.
- Expanded content cannot bypass the 100 MiB per-round limit.
- Recursive archives and compression bombs fail deterministically.
- Mid-extraction failure leaves no material usable by a Run.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: valid archive, absolute/traversal/duplicate paths, symlink/hardlink/device/FIFO, nested archive, bomb, CPU/memory ceilings and forced mid-extraction failure. Name the Go test `TestCraftT02Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT02Journey -count=1`. Expected: `TestCraftT02Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Inspect archive into a quarantined staging tree; canonicalize and check each entry before commit; count cumulative bytes/files/depth/ratio and wall/CPU/memory budgets; atomically publish only complete content-addressed read-only members. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT02Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Archive -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftArchive -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any partial input visible after extraction error blocks integration; retain test fixture and correct staging transaction. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 3: T03 — Uploaded code execution denial (#122)

**Depends on:** T01 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #122 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/input_code.go; internal/application/service/craft_delegate.go; sandbox material policy adapter; focused `*_test.go`. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T01 immutable input refs; T00 input classification. **Produces:** Read-only input capability, execution-denial audit and user refusal.

**Acceptance checks:**

- Uploaded code remains in the read-only input tree without executable capability.
- Direct interpreter and shell invocation of the uploaded path are denied.
- Copy-then-execute, symlink, alias, and temporary-directory bypasses are denied.
- Generated code may execute only from the writable Workspace/output boundary.
- Audit evidence distinguishes reading uploaded code from executing generated code.
- The member sees a clear refusal and allowed alternative.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: read script as data; reject interpreter/shell direct execution, copied payload, symlink/alias/temp bypass; allow generated Workspace code. Name the Go test `TestCraftT03Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT03Journey -count=1`. Expected: `TestCraftT03Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Mount uploads as read-only with noexec where supported; label input provenance; deny execution based on provenance and canonical identity across copy/link operations; allow execution only from generated Workspace/output and record distinct audit events. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT03Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Input|Code' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftInput|CraftDelegate' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/container/... -run Craft -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** A real sandbox bypass blocks verification even if unit tests pass; keep T04 offline build pending. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 4: T04 — Pinned offline web build (#123)

**Depends on:** T01, T03 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #123 criteria below and mapped parent stories in the DAG.

**Files owned:** fixed Craft web template and dependency lock/image; OpenCode web skill; internal/container/craft_runtime.go extension; offline build fixtures/tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T01 material refs; T03 uploaded-code policy; T00 runtime digest fact. **Produces:** Pinned runtime digest, build exit/evidence, web entry and local-asset manifest.

**Acceptance checks:**

- Template and dependency versions are fixed and represented in the runtime digest.
- A clean sandbox builds without downloading dependencies or using host caches/home directories.
- HTTP, DNS, Git, package-manager, and shell network attempts fail.
- A successful build produces a valid web entry and local assets only.
- Build logs identify template/runtime digest and real exit status.
- Missing provisioned dependency and absent entry fail honestly.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: clean sandbox build without home/cache/network; HTTP/DNS/Git/package manager/shell egress denied; missing dependency and missing entry fail. Name the Go test `TestCraftT04Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT04Journey -count=1`. Expected: `TestCraftT04Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Pin template and package versions in image; mount provisioned dependencies read-only; enforce network namespace/egress deny in actual build container; collect real exit, digest, entry and local asset list. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT04Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/container/... -run Craft -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Release|Web' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `Provide a real offline sandbox build log and a denied-egress probe`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Environment evidence:** In a fresh isolated sandbox, run the pinned build with empty home/cache and capture real build log, digest, exit code, local entry/assets and denied HTTP/DNS/Git/package-manager/shell egress probes. Expected: build succeeds without network and every probe is denied.
- [ ] **Failure handling:** If deployment image permits egress or needs download, fail task and repair the runtime policy before T06/T15. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 5: T05 — Selected knowledge and actual sources (#124)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #124 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/knowledge.go; internal/application/service/craft_knowledge.go; packages/views/src/craft/sources.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 source/restricted-contribution facts; current caller grant. **Produces:** Selected∩authorized retrieval and immutable actual-source record with ref,digest,time,bounded excerpt.

**Acceptance checks:**

- Retrieval scope equals selected scope intersected with current caller authorization.
- Unselected and unauthorized knowledge never enters the material bundle.
- Actual sources persist stable reference, digest, acquisition time, and bounded excerpt metadata.
- Empty and truncated source results are disclosed.
- Every source open re-resolves through the authorization chain rather than following a model URL.
- Permission revocation is honored on later access.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: select KB A while B exists; revoke A; empty/truncated results; direct source open as unauthorized Viewer. Name the Go test `TestCraftT05Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT05Journey -count=1`. Expected: `TestCraftT05Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Intersect Task selection with current caller authorization before retrieval and again at open; persist only actually used refs and bounded metadata; project empty/truncated warnings. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT05Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Knowledge -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftKnowledge -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Craft.*Source' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any unselected or unauthorized source entering delegated material blocks T06 and T10. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 6: T06 — Evidence-backed web citations (#125)

**Depends on:** T04, T05 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #125 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/citation.go; internal/application/service/craft_citations.go; generated-web citation view; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T04 web entry/assets; T05 actual-source records. **Produces:** Citation manifest mapping stable IDs to actual sources and distinct inference markers.

**Acceptance checks:**

- Each cited fact uses a stable citation ID mapped to an actual recorded source.
- Inference uses an explicit distinct marker and is never presented as source fact.
- Web citations and Workbench evidence agree.
- Missing, revoked, or inaccessible source retains a non-leaking placeholder.
- Citation manifest is stored with the immutable artifact and excludes full restricted originals.
- Fabricated citation IDs are detected and cannot silently resolve.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: valid fact citation, inference marker, fabricated ID, inaccessible/revoked source placeholder, workbench/web consistency. Name the Go test `TestCraftT06Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT06Journey -count=1`. Expected: `TestCraftT06Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Generate citation IDs only from recorded actual sources; validate candidate manifest before version packaging; render fact links and explicit inference labels; store bounded manifest with immutable candidate. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT06Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Knowledge|Citation' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftKnowledge|CraftArtifact' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Fabricated ID resolving or restricted original embedded in manifest blocks T07. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 7: T07 — Version-pinned evidence (#131)

**Depends on:** T06, T15 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #131 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/version.go; internal/application/repository/craft_version.go; internal/application/service/craft_artifacts.go extension; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T06 citation manifest; T15 promoted immutable version. **Produces:** Historical evidence snapshot with source digest/acquisition time; new Run refreshes current grant.

**Acceptance checks:**

- Version evidence is immutable and contains source version/digest and acquisition time.
- New Runs re-evaluate current authorization and source state.
- Updated knowledge can produce different evidence in a later version without mutating the old version.
- Deletion/revocation preserves historical evidence facts but source opening still reauthorizes.
- Historical evidence is never reconstructed from mutable Workspace files.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: publish V1; update/delete/revoke source; run V2; assert V1 digest/time stable, V2 may differ and opens still reauthorize. Name the Go test `TestCraftT07Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT07Journey -count=1`. Expected: `TestCraftT07Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Persist evidence bytes/digest as version member in the same commit as promotion; never rebuild history from Workspace or current KB; query original snapshot by Version ID. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT07Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Version|Knowledge|Evidence' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftArtifact|CraftKnowledge' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any mutation of V1 evidence after V2 blocks T12 and T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 8: T08 — Private Task roles (#126)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #126 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/access.go; internal/application/service/craft_access.go; internal/handler/session/craft_access.go; packages/views/src/craft/access.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 ACL/route/workbench slots; session authentication. **Produces:** Private-by-default Owner/Collaborator/Viewer grant and revocation audit.

**Acceptance checks:**

- Task is private to its owner by default.
- Owner can add/revoke Collaborator and Viewer roles; cross-tenant membership is refused.
- Viewer is read-only and Collaborator does not inherit owner credentials, personal connectors, or source access.
- Same-tenant membership and ordinary admin status do not grant private content access.
- Revocation applies to listing, direct access, preview, and download.
- Authorization is server enforced and audited.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: same-tenant nonmember, tenant admin, cross-tenant add, Viewer write, revoked preview/download/list/direct read. Name the Go test `TestCraftT08Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT08Journey -count=1`. Expected: `TestCraftT08Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Persist explicit membership scoped by tenant+Task; enforce on every service operation; audit add/revoke; project role-limited controls via TDesign. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT08Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'SessionShare|Craft.*Access' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Share|Craft.*Access' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any unauthorized direct HTTP access blocks all downstream access tasks. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 9: T09 — Collaborator serialized edit (#135)

**Depends on:** T08, T16 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #135 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/application/service/craft_collaborator_run.go; packages/views/src/craft/workbench-edit.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T08 role grant; T16 writer lease; T05 caller-scoped source grant. **Produces:** New Run in same Workspace initiated by Collaborator with narrowed delegated grant.

**Acceptance checks:**

- Collaborator can request a Run and Viewer cannot.
- Run uses the same Workspace and must acquire its writer lease.
- Collaborator does not inherit owner personal connections, keys, or inaccessible knowledge.
- Timeline records the actual initiating member.
- Revoked collaborator cannot start a new Run; historical audit remains.
- Concurrent edit conflict is surfaced without corrupting the Workspace.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: Collaborator starts; Viewer denied; owner personal connector unavailable; revoked Collaborator denied; two concurrent edits conflict. Name the Go test `TestCraftT09Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT09Journey -count=1`. Expected: `TestCraftT09Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** On Run request derive initiating user, current role, Task Grant intersection and caller resources; acquire T16 lease before dispatch; audit initiator and project conflict. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT09Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftRun|Craft.*Access' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'CraftRun|Share' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** If collaborator inherits owner secrets or bypasses writer lease, block T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 10: T10 — Viewer-specific source open (#127)

**Depends on:** T05, T08 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #127 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/application/service/craft_source_open.go; internal/handler/session/artifact_reference.go; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T05 actual-source ref; T08 current task role. **Produces:** Authenticated source-open resolver and stable non-leaking deny.

**Acceptance checks:**

- Every source open performs fresh viewer authorization.
- Artifact access never implies original-source access.
- No reusable provider/storage URL is exposed or cached.
- Permission revocation applies to the next open.
- Missing and forbidden outcomes are stable and non-leaking.
- Citation placeholder/integrity remains visible when source access is denied.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: Viewer can see citation but lacks original-source grant; revoke grant then reopen; tamper ref; missing source. Name the Go test `TestCraftT10Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT10Journey -count=1`. Expected: `TestCraftT10Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Resolve source ID through artifact evidence and current viewer authorization on every open; proxy bytes rather than expose provider URL; preserve placeholder on deny. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT10Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftKnowledge|Source' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Source|ArtifactReference' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** If a cached URL still opens after revocation, block sharing/export integration. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 11: T11 — Restricted-result sharing consent (#128)

**Depends on:** T06, T08 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #128 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/share.go; internal/application/service/craft_share.go; internal/handler/session/craft_share.go; packages/views/src/craft/share.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T06 restricted contribution/evidence digest; T08 current Owner. **Produces:** Owner share decision bound to Version ID and evidence digest.

**Acceptance checks:**

- Restricted contribution is computed server-side from recorded evidence.
- Consent binds immutable Version ID and Evidence digest.
- Only the current Task Owner can consent.
- Changed evidence invalidates prior consent.
- Reject, expiry, replay, and revocation do not create sharing authority.
- Sharing derived output grants no original-source access and is audited.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: restricted result default private; owner consents; changed evidence, expired/replayed decision, revoked owner, nonowner consent denied. Name the Go test `TestCraftT11Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT11Journey -count=1`. Expected: `TestCraftT11Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Classify contribution server-side from recorded evidence; persist signed or canonical digest-bound decision; check current ownership and grant at read; audit allow/reject. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT11Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Evidence|Share' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'Craft.*Share|Decision' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run Share -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any reused consent for changed evidence or original-source access blocks T13. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 12: T12 — Immutable source bundle (#132)

**Depends on:** T10, T07 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #132 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/export_manifest.go; internal/application/service/craft_export.go; internal/handler/session/artifact_download.go; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T07 immutable Version/evidence; T10 authenticated source-open resolver. **Produces:** Version-bound bundle, citation manifest and stable digest without restricted originals.

**Acceptance checks:**

- Bundle is bound to Version ID and reads immutable artifact members only.
- Bundle contains source, build metadata, and citation/source manifest.
- Manifest includes stable citation identity, title, digest/time metadata, and an authenticated source reference.
- Restricted originals are excluded by default.
- A historical version's bundle digest stays stable after later edits.
- Authorization and audit record member, Version, and manifest digest.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: V1 bundle before/after V2; traversal/path injection; denied Viewer; manifest reference remains authenticated. Name the Go test `TestCraftT12Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT12Journey -count=1`. Expected: `TestCraftT12Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Package only Version file manifest members; include pinned source list, title/digest/time and authenticated source reference; compute manifest digest, authorize and audit download. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT12Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Version|Export' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftArtifact|Export' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Artifact.*Download|Craft.*Download' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** If mutable Workspace file changes bundle digest, block T13. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 13: T13 — Restricted derived export consent (#133)

**Depends on:** T11, T12 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #133 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/export_consent.go; internal/application/service/craft_export_consent.go; internal/handler/session/craft_export_consent.go; packages/views/src/craft/export.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T11 owner/restricted evidence; T12 export manifest digest. **Produces:** Version/manifest-digest-bound owner decision and safe fallback bundle.

**Acceptance checks:**

- Server classifies derived files and records their origins.
- Consent binds Version ID and Export Manifest digest and can only be given by current owner.
- Without valid consent no restricted derived bytes are returned.
- Reject produces an optional safe bundle without restricted derived files.
- Manifest change invalidates old consent; replay cannot broaden scope.
- Original restricted inputs remain excluded and decision is audited.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: restricted derived file withheld without consent; owner consent; reject safe bundle; mutate manifest; replay; nonowner request. Name the Go test `TestCraftT13Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT13Journey -count=1`. Expected: `TestCraftT13Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Classify derived files/origins server-side; present exact manifest; persist decision bound to Version ID and digest; filter bytes before streaming; always omit restricted originals and audit. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT13Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Export -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'Export|Decision' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Export|Download' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any restricted derived byte streamed on rejection blocks T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 14: T14 — Isolated no-egress preview (#129)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #129 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/preview.go; internal/application/service/craft_preview.go; internal/handler/session/craft_preview.go; deployment preview network policy; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 preview/Version typed facts and extension slot. **Produces:** Short-lived scoped ticket and isolated-origin manifest-only file service.

**Acceptance checks:**

- Preview origin differs from app origin and receives no app cookies/storage/authorization headers.
- Short-lived capability binds Task, tenant, Version, and allowed files.
- Traversal, escaped paths, cross-version and cross-tenant use fail.
- Infrastructure denies HTTP, WebSocket, DNS, internet, and enterprise-network egress; CSP is defense in depth.
- Only version-manifest files are served.
- A real browser loads valid local HTML/CSS/JS/assets.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: same-origin cookie leak, token replay/cross-tenant/cross-version, traversal/escaped path, real browser local assets, HTTP/WS/DNS/internal egress probes. Name the Go test `TestCraftT14Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT14Journey -count=1`. Expected: `TestCraftT14Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Bind capability to tenant/Task/Version/file set and expiry; serve only immutable manifest paths on separate origin with no app credentials; enforce network isolation at deployment layer plus CSP. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT14Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Preview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftPreview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run CraftPreview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `Provide browser-level denied-egress evidence`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Environment evidence:** Launch the configured preview sandbox and a real browser on the isolated origin; capture local asset load plus denied HTTP/WebSocket/DNS/enterprise-network probes. Expected: page loads and all egress probes fail at infrastructure level.
- [ ] **Failure handling:** CSP-only denial or default Docker bridge egress is failure; do not let T15 promote. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 15: T15 — Four-check release gate (#130)

**Depends on:** T04, T14 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #130 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/release.go; internal/application/service/craft_artifacts.go; internal/application/repository/craft_preview_check.go; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T04 real build result; T14 preview reachability/load probe; T00 WebCheckEvidence. **Produces:** Idempotent promotion of candidate with four independently passed checks.

**Acceptance checks:**

- Each check records passed/failed/not_run independently.
- Preview reachability cannot substitute for actual page load.
- Any failed or not-run check prevents promotion and keeps prior default version.
- Evidence binds Run, Workspace revision, and candidate Version.
- Promotion is idempotent and duplicate callbacks cannot create duplicate versions.
- Generated files without a usable page remain a draft, not successful delivery.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: build pass/entry pass/preview reach pass/page load fail; not_run; stale Workspace revision; duplicate callback; old default retained. Name the Go test `TestCraftT15Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT15Journey -count=1`. Expected: `TestCraftT15Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Store check facts bound to Run, Workspace revision and candidate Version; CAS promote only with all four passed; preserve old default and draft on every incomplete path. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT15Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Version|Preview|Check' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftArtifact|CraftPreview' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Artifact.*Completion|CraftPreview' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any generated file promoted without page load blocks T07/T16. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 16: T16 — Durable Workspace writer lease (#134)

**Depends on:** T15 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #134 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/lifecycle.go; internal/application/repository/craft_workspace.go; internal/application/service/craft_workspace.go; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T15 version promotion; T00 WriterAcquireOutcome. **Produces:** Database lease bound to Task, Workspace, Run, revision fence; acquired/conflict/unknown.

**Acceptance checks:**

- Lease is database-backed and binds Task, Workspace, Run, and revision.
- Two concurrent writers yield exactly one acquisition and one stable conflict.
- Read-only preview/version/download never acquires the writer lease.
- Lease releases only after verified completion, confirmed stop, or authoritative recovery.
- Unknown outcome retains the fence and cannot permit a conflicting writer.
- Second successful edit creates a new version without mutating the old one.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: two concurrent starts exactly one acquired; preview/version/download unaffected; unknown abort retains fence; second successful edit creates V2. Name the Go test `TestCraftT16Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT16Journey -count=1`. Expected: `TestCraftT16Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Use transactional compare-and-swap on unique Workspace lease; fence every write and release only after verified finish, confirmed stop, or authoritative recovery. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT16Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Lifecycle|Workspace|Busy' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftWorkspace|CraftLifecycle' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'CraftRun|WorkbenchStart' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** If two writers enter same Workspace or unknown releases lease, block T09/T17/T18. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 17: T17 — Durable stop and draft (#136)

**Depends on:** T16 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #136 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/lifecycle.go extension; internal/application/service/craft_control.go; packages/views/src/craft/status-notice.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T16 writer fence; T00 StopOutcome; T15 promotion gate. **Produces:** Persisted stop intent, executor confirmation/unknown and Run-linked repairable draft.

**Acceptance checks:**

- Stop intent persists before executor abort is requested.
- Accepted HTTP response and stopping are nonterminal.
- Confirmed cancellation and unknown outcome remain distinct.
- Stopped/unknown Run cannot promote a version or release writer ownership prematurely.
- Draft retains source Run identity and prior successful version remains default.
- Repeated stop is idempotent and state survives page refresh.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: repeat stop; HTTP accepted but executor still active; abort unknown; refresh; prior V1 remains default. Name the Go test `TestCraftT17Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT17Journey -count=1`. Expected: `TestCraftT17Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Persist intent before abort; reconcile executor state; map confirmed cancel versus unknown distinctly; retain draft files and fence until authoritative outcome. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT17Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run 'Stop|Lifecycle' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftControl -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run StopCraftRun -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** If stopping becomes terminal on HTTP ack or replaces V1, block T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 18: T18 — Authoritative reconnect (#137)

**Depends on:** T16 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #137 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/recovery.go; internal/application/service/craft_recovery.go; packages/domain/src/craft/reconnect.ts; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T16 lease/fence; T00 Run outcome DTO; server snapshot/event cursor. **Produces:** Snapshot-first continuation with sequence dedupe/gap reload and zero resubmit.

**Acceptance checks:**

- Reconnect loads authoritative snapshot before event continuation.
- Duplicate events are ignored and sequence gaps trigger snapshot reload.
- Expired cursor never calls submit.
- One user intent yields one Run across refresh/reconnect.
- Unknown remains unknown and delegation completion cannot finish the main Run.
- UI shows syncing and then the current successful version/draft state.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: refresh during Run, duplicate event, gap, expired cursor, unknown executor, child delegation finished before main Run. Name the Go test `TestCraftT18Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT18Journey -count=1`. Expected: `TestCraftT18Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Read snapshot before SSE; ignore seq≤watermark; reload on gap/expiry; keep same Run ID and last successful version/draft projection. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT18Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Recovery -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftRecovery -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'AgentRunEvents|WorkbenchStream' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any reconnect path calling submit or finishing main Run from child event blocks T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 19: T19 — Durable Task Budget pause (#138)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #138 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/budget.go; internal/application/service/craft_budget.go; packages/views/src/craft/usage.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 BudgetPause typed record; tenant policy and Task Budget. **Produces:** Idempotent model/sandbox charge attribution and paused Run with authorized extension.

**Acceptance checks:**

- Budget admission occurs before model or sandbox activity.
- Every chargeable activity is attributed once to tenant and Task Budget.
- Limit prevents new chargeable calls and persists budget-paused reason.
- Paused Run cannot promote a version and survives reload.
- UI identifies authorized owner/billing-admin action without exposing money/secrets improperly.
- Unauthorized member cannot extend/resume; authorized extension resumes safely without replaying unknown effects.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: limit at admission; delegated sandbox/model both charge once; refresh paused; nonowner extend denied; authorized resume avoids unknown replay. Name the Go test `TestCraftT19Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT19Journey -count=1`. Expected: `TestCraftT19Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Reserve budget before chargeable work; attribute each activity once by stable ID; persist pause reason and allowed action; check owner/billing-admin grant before extension. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT19Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Budget -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftBudget|CraftExecutionBudget' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run CraftUsage -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any chargeable call after limit or paused Run promotion blocks T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

### Task 20: T20 — Integrated web artifact journey (#139)

**Depends on:** T02, T03, T07, T13, T09, T17, T18, T19 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #139 criteria below and mapped parent stories in the DAG.

**Files owned:** central Craft assembly/routes/migrations/workbench; apps/web/e2e/craft-web-artifact.spec.ts; acceptance evidence. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** All verified T02/T03/T07/T09/T13/T17/T18/T19 and their transitive ancestors. **Produces:** Production wiring and real API/DB/browser evidence for all 36 stories.

**Acceptance checks:**

- Selected knowledge plus CSV produces an offline region-filterable webpage with evidence-backed facts.
- Unknown input is disclosed and continue/cancel does not duplicate submission.
- Four checks gate promotion; prior successful version remains during edits, stop, failure, pause, or unknown result.
- Refresh reconnects to the same Run; old version remains immutable and downloadable.
- Owner/Collaborator/Viewer behavior, source reauthorization, restricted sharing, and export consent work together.
- Budget exhaustion persists pause and causes no false delivery.
- All 36 parent user stories map to automated or reproducible evidence.
- New/migrated workbench controls follow the approved TDesign direction.
- Full validation suite passes and final diff contains only authorized scope.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: selected KB+CSV → offline region-filterable page; two edits; citations/history/download; reconnect; role/consent/revocation; budget/stop/failure. Name the Go test `TestCraftT20Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT20Journey -count=1`. Expected: `TestCraftT20Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Integrate reviewed checkpoints; assign migrations and central route/workbench slots; exercise real offline build and preview; map each story to test or reproducible evidence in acceptance record. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT20Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run Craft -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Craft|Artifact|AgentRun' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/container/... -run Craft -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm build:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:craft:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any unmapped story, mocked permission boundary, failing gate, or unintended diff blocks completion and OCR. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.
