# #140 Career Main Architecture Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the approved #140 Career Office capability into the module, contract, Workbench, TDesign Web, Expo and Taro architecture represented by the verified `issue30-sweep` baseline.

**Architecture:** Career owns career facts and workflow state; it uses authenticated Identity/Tenant context, existing Workbench Tasks/admission, Artifact, Usage/Budget, notifications and audit through narrow ports. A versioned and decoded `packages/contracts` / `packages/api-client` boundary is shared by Web, Mini Program and Expo clients; a new platform-neutral `@weknora/career-core` Career Desk owns cross-client state and recovery under ADR 0019. The legacy `codex/issue-140-integration` branch is a behavior reference only; no legacy history or UI tree is merged.

**Tech Stack:** Go module services and repositories, PostgreSQL + SQLite migrations, React/TDesign Web, Expo/React Native with `packages/mobile-core`, Taro 4 Mini Program, TypeScript shared contracts/API client, existing Go/Node/pnpm test toolchains.

**Spec:** [`docs/superpowers/specs/2026-09-28-issue-140-main-architecture-port-design.md`](../specs/2026-09-28-issue-140-main-architecture-port-design.md); approved product requirements [`docs/specs/2026-09-23-weknora-job-search-design.md`](../../specs/2026-09-23-weknora-job-search-design.md); issue tree/DAG [`docs/plans/issue-140/2026-09-28-issue-140-main-port-dag.md`](../../plans/issue-140/2026-09-28-issue-140-main-port-dag.md); all issue snapshots `docs/plans/issue-140/issues/issue-*.md`; ADRs 0015–0019; `CONTEXT.md`.

## Global Constraints

- Start from verified `issue30-sweep` committed HEAD `db234c5eb171f2dde7427d382b55b503a038f879`; do not include its untracked worktree files.
- The #140 tree is 33 formal open child Issues with dependency edges from their `Blocked by` bodies; #140 aggregates acceptance and is not an extra implementation feature.
- Use the same WeKnora actor/Tenant and one-member Career space; server derives actor and tenant from auth context.
- User-confirmed profile facts alone may be used for eligibility and material claims; job descriptions and source pages are untrusted data, never Agent instructions.
- Preserve source URL, observation time, completeness and failure state. Do not bypass login, CAPTCHA, or access limits; request a complete JD when a source is restricted or incomplete.
- Eligibility remains tri-state (`eligible`, `ineligible`, `unknown`); hard-condition conflicts cannot be hidden by match score. Explicit user continuation does not erase the original evaluation.
- One opportunity/recruitment batch has one immutable job snapshot, one application and one Workbench Task. Unknown create outcomes reuse the original idempotency key.
- Material versions are immutable and publish only after real PDF and editable DOCX output checks. Submission is user performed and records channel/time/version or explicit unknown.
- Progress is append-only with traceable corrections and event projection. Writes carry request ID and expected revision; late responses and stale private caches are rejected after scope changes.
- Rules are explicit and budget-admitted. Quota exhaustion never blocks reading existing profile/application history. Export/delete revoke Task, Artifact and client-cache access.
- Use current TDesign Web and TDesign MiniProgram controls or record a specific native exception. Mini Program downloads, shares and notifications use platform adapters.
- Expo/iOS, Android and HarmonyOS/native gates require their own real build/device evidence; shared-code tests or browser preview alone never pass these gates.
- No automatic application submission, email sending, cross-site form filling, login/CAPTCHA bypass, or mailbox-based progress inference.
- Preserve brand/design constraints in approved prototype C: light WeKnora TDesign theme and brand `#07c05f`.
- Work only in the managed isolated worktrees. Keep the primary workspace’s pre-existing dirty and untracked files untouched. No push, shared-branch merge, deployment, release, issue close or comment.

## Review Focus

- #144 Expo Task Office must enter and restore an existing authorized Task without creating another; scope changes/late replies fail closed; offline state is governed cache or explicit unavailable. #143 and #145 prerequisites remain explicit.
- #169 Mini timeline preserves shared order and correction history; preparation uses the actual submitted material version or explicitly prompts when unknown; account changes clear the prior private projection.

- Unconfirmed facts, model-inferred facts, or prompt-injected JD text must never become eligibility evidence or a material claim. Test profile confirmation and hostile JD fixtures in P1/P3.
- Tenant/owner spoofing, revoked scope, late async responses and stale local cache must never expose another user’s Career data. Test each service/client boundary in P1/P2/P4/P5/P6/P7.
- A timed-out create/update/export must not silently retry under a new request ID or duplicate a Task, application, event, reminder, or charge. Test same-key reconciliation and request mismatch in P2/P3/P4.
- Missing/partial source data and tri-state unknown must remain visible; a strong skills score must not hide a hard-condition conflict. Test incomplete sources and mixed evaluation evidence in P1.
- Material exports and actual submission version may be unknown or stale. Test PDF/DOCX digest/version parity, immutable edits, unknown-version retention and user-only submission in P3/P5/P6/P7.

---

## Starting Facts and File Ownership

- Execution branch: `codex/issue-140-issue30-base`; required original BASE: `db234c5eb171f2dde7427d382b55b503a038f879`. Design commit: `fa0b882def7e78ff3599cc7bef204ad2398063e5`.
- Live issue audit: 2026-09-28; 34 issues, no children deeper than one level; no comments or post-snapshot updates; dependencies match `2026-09-24-issue-140-dag.md`.
- `P0` owns new Career contracts/API-client and module boundary docs; backend domain tasks own `internal/career/**` and paired migration assets; only P2 integration owner may modify central router/container/migration registration files.
- P5 owns `apps/web/src/career/**`, its dedicated route/nav registration, Career locale resources and tests. P6 owns only Mini Program career feature/subpackage/config/platform adapter additions. P7 owns only `apps/mobile/src/career/**`, its app composition additions, and Career-specific native adapter/test files.
- Career Desk is an explicitly owned new package, not present in the selected baseline; freeze its ADR 0019 interface and tests before clients fork.
- The following shared files are integration-owner only: `internal/router/router.go`, `internal/container/container.go`, central migration registration, `packages/contracts/src/index.ts`, `packages/api-client/src/client.ts`, Web route/shell registries, Mini Program `app.config.ts` and route registry, Mobile composition root, root workspace/lockfiles. Workers submit patch suggestions for these seams to the integrator.
- A client task starts only after its consumed server/API contract task has passed its independent review and its commit/checkpoint is in that task’s base. Web, Mini Program and Mobile tasks may run in distinct worktrees after contract freeze because their owned files and runtime/build directories are isolated.

## Execution Tasks

### Task 0: Record the migration ledger and pin API/module seams

**Depends on:** none
**Owner / validator:** `mechanical_worker` / `reviewer`
**Concurrency:** serial first task; documentation only.

**Files:**
- Create: `docs/plans/issue-140/main-port-ledger.md`
- Create: `docs/plans/issue-140/2026-09-28-issue-140-main-port-dag.md`
- Reference: approved Spec, CONTEXT, ADR 0015–0019, issue snapshots, baseline worktree status.

**Consumes:** issue root #140, live issue-audit report, verified base SHA.
**Produces:** durable source inventory, P0–P8 DAG plus PH Harmony gate, ADR 0019 Career Desk ownership, acceptance mapping, task-state ledger and isolated-worktree baseline record.

- [ ] Verify branch and `git rev-parse HEAD`; record SHA, worktree path, clean starting state, and the five untouched untracked files in source `.worktrees/issue30-sweep`.
- [ ] Record the live GitHub tree/count/contains/dependency edges, mark old implementation reports as historical evidence, and set each new port task to `pending`.
- [ ] Add files with the exact status, source, owner role, validator role, owned paths, interfaces and verification evidence. Verify #142/#143/#151/#156 acceptance is explicitly mapped to task and named test evidence. Check unique node IDs, all issue references, self-edges, cycles and root-story coverage.
- [ ] Run the DAG/document consistency check (small script or equivalent explicit count); expected 34 issues, 33 contains edges, 33 child dependency nodes covered and 0 cycles.
- [ ] Commit as `docs: record issue 140 main architecture port ledger`.

### Task 0A: Harmony native feasibility trial (#143)

**Depends on:** Task 0 documentation baseline; independent of Career API implementation.
**Owner / validator:** `frontend_implementer` / `frontend_validator`
**Concurrency:** early isolated native trial; must finish before any Harmony-inclusive dependent slice.

**Files:**
- Create: `docs/plans/issue-140/verification/harmony-feasibility.md`, reproducible trial harness/evidence only in a dedicated native validation directory if required; no Career business contract changes.

**Consumes:** current Expo app, Mobile Runtime, authenticated Workbench Task read, ADR 0018, #143 acceptance.
**Produces:** native-vs-Android-compat-vs-WebView compatibility matrix covering target HarmonyOS version, toolchain, Expo SDK, RN and native modules; actual adaptation/build attempt and evidence. If viable, build a native package and verify authenticated Task read, login, scope invalidation, file, share, notification and storage probes. If no viable route, record minimal reproduction, raw failure evidence and technical ruling naming affected tickets and required architecture decision. Never count Android compatibility, WebView, or unavailable hardware alone as a completed feasibility trial.

- [ ] Record exact commands, system/toolchain/SDK versions, device identity and logs; distinguish native Harmony from Android compatibility and WebView.
- [ ] Produce either viable-path capability evidence or a reproducible failure plus options/ruling; absence of a toolchain/device is recorded as an unresolved prerequisite only after documenting the attempted trial and evidence gathered.
- [ ] Independent validator checks the evidence and ruling before Harmony-inclusive work is released.

### Task 0B: Expo iOS/Android authenticated Task boundary (#145)

**Depends on:** none; #145 has no Issue blocker. Start independently of PH/#143, Career API and Career backend tasks using the existing Expo app, Mobile Runtime and authorized Task read.
**Owner / validator:** `frontend_implementer` / `frontend_validator`
**Concurrency:** early isolated Expo native slice; may run alongside Task 0A and Tasks 1–6. Keep device/build output isolated from other Mobile work.

**Files:**
- Create/modify: `packages/mobile-core/src/task-office/**` for the public Task Office seam and focused tests; `apps/mobile/src/task-office/native-boundary/**` for Expo composition/adapters and acceptance probes. Shared public exports remain integration-owner reviewed.

**Consumes:** current Expo app, Mobile Runtime scope lease, authenticated Task public read, #145 acceptance and ADR 0018. It does not consume Career API or wait for the Harmony trial.
**Produces:** reusable authorized existing-Task boundary on iOS and Android, including login/tenant rejection, space-switch invalidation, navigation, file select/download, share, notification and secure-storage capability adapter evidence. Its native Task entry UI uses React Native components mapped to TDesign Mobile React visual tokens and interactions; it must not import mobile Web components. This UI/import check is part of #145 and must pass before the checkpoint is verified/integrated or consumed by Task 9. Harmony is outside #145 scope.

- [ ] Add `MobileTaskOffice_PublicSeam_RejectsCrossTenantAndRestoresSameTask` and capability-probe tests for scope revocation/late reads, file, share, notification and secure storage.
- [ ] Implement the #145 native Task entry with React Native components mapped to TDesign Mobile React tokens and interactions; do not import mobile Web components. Add `ExpoTaskOffice_NativeBoundary_UsesReactNativeTDesignTokensAndInteractions` and `ExpoTaskOffice_NativeBoundary_NoMobileWebComponentImports`.
- [ ] Before verifying/integrating #145, capture iOS and Android screen/interaction evidence for the Task entry visual states and interactions; include token/component mapping and import-boundary results in the independent checkpoint review.
- [ ] Run real iOS and Android development builds with the existing Expo SDK. Record commands, SDK/dependency versions, device/simulator, authorized existing Task read and all probe results; absent device/build environment stays explicitly blocked for that platform.
- [ ] Independently review and integrate the #145 checkpoint before Task 9 implements #144.

### Task 1: Define versioned Career contracts, Career Desk and API client

**Depends on:** Task 0
**Owner / validator:** `implementer` (shared contracts/API boundary) / `reviewer`
**Concurrency:** serial before backend/client feature tasks; owns shared contract indexes.

**Files:**
- Create: `packages/contracts/src/career/{index.ts,profile.ts,opportunity.ts,application.ts,search.ts,privacy.ts,index.test.ts}`
- Modify: `packages/contracts/src/index.ts`
- Create: `packages/api-client/src/career/{index.ts,types.ts,decode.ts,index.test.ts}`
- Modify: `packages/api-client/src/client.ts`
- Create: `packages/career-core/{package.json,src/**,test/**}` with package name/export `@weknora/career-core`; focused tests cover durable request ID before write, unknown same-ID reconciliation, explicit conflict rebase, scope change invalidation and late response rejection.
- Tests/fixtures: `packages/contracts/test/career/` and `packages/api-client` fixture tests.

**Consumes:** product Spec and ADRs 0015–0019.
**Produces:** strict wire DTOs/`CareerApi` plus the new `@weknora/career-core` package API from ADR 0019. `packages/contracts` owns decoded DTOs; `packages/api-client` owns HTTP; Career Desk owns open/list/act/observe, revisions, durable pending-intent reconciliation and scope invalidation. Freeze and test the interface before any client worktree forks; Web must not depend on `mobile-core`, and Mobile adapts its current scope lease. Define exported factory `createCareerApi(request: ClientRequest): CareerApi`; response decoders reject malformed object shape, unknown enum, invalid revision/digest, receipt/request ID mismatch, and tenant/owner fields supplied as trusted client authority.

- [ ] Add failing fixture tests for all Career DTO variants, tri-state evaluation, immutable snapshots, material version/digest, expected revision and request receipt mismatch. Check package exports and consumers first; keep fixtures at `packages/contracts/test/career/`.
- [ ] Run targeted existing tests through the root test runner (`pnpm exec tsx --test <new-contract-test> <new-api-client-test>`) and verify they fail because the Career exports/decoder and Career Desk package are missing.
- [ ] Implement DTOs and decoder functions in the listed package paths; keep server-derived actor/tenant out of client request authority.
- [ ] Run `pnpm test:shared` and `pnpm typecheck:shared`; expected all existing shared tests and type checks plus the new Career tests pass.
- [ ] Commit `feat: add versioned career contracts and shared desk`.

### Task 2: Establish Career module ownership, migrations and authorization seam

**Depends on:** Task 1
**Owner / validator:** `backend_implementer` / `backend_validator`
**Concurrency:** serial; only backend task allowed to define schema foundation or central migration registration.

**Files:**
- Create: `internal/career/{README.md,module.go,manifest.go}` and `internal/career/{repository,service,handler}/` scaffolding plus tests.
- Modify: `docs/architecture/backend-modules.yaml`, `docs/architecture/moves/README.md`; add `docs/architecture/moves/career.yaml`.
- Create: paired PostgreSQL `migrations/versioned/*_career_*.{up,down}.sql` and SQLite `migrations/sqlite/*_career_*.{up,down}.sql` with currently free sequence numbers; add narrowly owned registration only where repository convention requires it.
- Central registration edits, if needed: only integration owner in `internal/container/container.go`, `internal/router/router.go`, central migration code.

**Consumes:** `CareerApi` DTOs and current Identity/Tenant/Workbench/Artifact/Usage contracts.
**Produces:** module manifest, migration ownership policy, owner-scoped repository/service base, module handler registration interface and the #142 Workbench Artifact version-bound download grant lifecycle. Artifact authority owns signing/validation and must bind tenant, owner, resource and exact immutable version; download rechecks authorization and revocation. Before Task 5 material download, reject cross-tenant, expired, signature-tampered, revoked/deleted versions while preserving existing Workbench Task artifact downloads.

- [ ] Add migration tests proving next available sequence IDs for both database tracks are unique and up/down pairs are complete; prove existing migration status remains stable.
- [ ] Add failing owner/tenant tests for unauthenticated, wrong-tenant, wrong-owner and missing-resource reads/writes.
- [ ] Add Workbench Artifact public-interface tests for exact-version grant binding, download-time authorization/revocation recheck, cross-tenant/expired/tampered/revoked/deleted rejection, and regression success for existing Task artifact downloads; save one real Web download digest as evidence.
- [ ] Implement Career module manifest and migration schema for single-member space, confirmed profile facts, idempotency receipts and append-only evidence with composite tenant/owner keys.
- [ ] Register dependency ports without importing Workbench implementation internals or adding a second task runner.
- [ ] Run `go test ./internal/career/... ./internal/database/... ./internal/container/...`; run architecture boundary tests. Expected all pass, no new forbidden dependency.
- [ ] Commit `feat: establish career module and artifact grants`.

### Task 3: Profile confirmation, job intake, source snapshots and tri-state evaluation

**Depends on:** Task 2
**Owner / validator:** `backend_implementer` / `backend_validator`
**Concurrency:** serial backend domain task; owns profile/source/opportunity/evaluation packages.

**Files:**
- Create/modify: `internal/career/profile/**`, `source/**`, `opportunity/**`, `evaluation/**`, handlers/routes local to Career and scoped tests.
- Add corresponding up/down migrations only through the Task 2 migration owner.

**Consumes:** Task 1 DTOs and Task 2 actor/tenant persistence seam.
**Produces:** profile draft/confirm APIs, JD paste/link import, source observation and snapshot APIs, eligibility API returning `eligible|ineligible|unknown`, evidence separated from assertions.

- [ ] Write tests proving extraction creates unconfirmed facts and only explicit confirmation permits use by evaluation/material services.
- [ ] Write source tests for preserved URL/observed-at/completeness/error, restricted source prompting for full JD, no login bypass, and immutable snapshot digest.
- [ ] Write eligibility tests for each tri-state value, hard conflict remaining visible despite high skill score, and user override preserving original evaluation.
- [ ] Implement repository/service/handler behavior; reject untrusted JD instruction content from the Agent instruction path.
- [ ] Run `go test ./internal/career/...`; expected all new domain tests pass and package race test passes for repository scope.
- [ ] Commit `feat: add confirmed career facts and evidence based evaluation`.

### Task 4: Search, application/task admission and quota behavior

**Depends on:** Task 3
**Owner / validator:** `backend_implementer` / `backend_validator`
**Concurrency:** serial around Workbench adapter and central route/container wiring.

**Files:**
- Create/modify: `internal/career/search/**`, `rule/**`, `admission/**`, `internal/career/handler/**` and tests.
- Integration-owner only: `internal/container/container.go`, `internal/router/router.go`, migration runner registration if applicable.
- Reuse existing Workbench admission in `internal/workbench/service/workbench/admission.go` and `TaskBudgetPort`; do not edit shared Workbench internals unless separately reviewed.

**Consumes:** profile/opportunity/evaluation APIs; Workbench `AdmissionCoordinator.Start` and TaskBudget seam.
**Produces:** one-shot search, explicit enable/disable continuous rule, opportunity batch-to-application admission, same-key unknown-outcome reconciliation, one application-to-one Workbench Task association, and #151 source reconciliation after search results exist. Merge only with strong job/employer/location/batch evidence; keep uncertain duplicates separate; preserve each source URL/check time; mark changed/expired/taken-down JD while retaining prior application snapshots; expose actual connected sources/city coverage; on failure preserve last success and stale time.

- [ ] Test same `(tenant,owner,request_id)` replay returns the original result and creates at most one Task/Application; a different request ID cannot overwrite an unknown result.
- [ ] Test budget denial prevents a new billable Run but leaves profile and existing application reads available; subscriptions/reminders do not create duplicate rules.
- [ ] Add named #151 tests `TestSearch_ReconcilesOnlyStronglySupportedDuplicateBatches`, `TestSearch_KeepsUncertainDuplicatesSeparate`, `TestSource_PreservesURLsAndCheckTimes`, `TestSource_AnnotatesChangedExpiredAndRemovedJDsWithoutChangingApplicationSnapshot`, `TestSearch_ReportsConnectedSourcesAndCityCoverage`, and `TestSourceFailure_PreservesLastSuccessAndStaleTime`; add Web evidence-diff/coverage tests in Task 7.
- [ ] Implement Career owned admission adapter, search endpoints and route wiring; charge or enqueue only through existing Workbench/Usage port.
- [ ] Run `go test ./internal/career/... ./internal/workbench/... ./internal/router/... ./internal/container/...`; expected all pass with no second execution runtime.
- [ ] Commit `feat: integrate career search and application admission with workbench`.

### Task 5: Applications, immutable material versions, PDF/DOCX and user submission

**Depends on:** Task 4
**Owner / validator:** `backend_implementer` / `backend_validator`
**Concurrency:** serial backend task; owns material and application records.

**Files:**
- Create/modify: `internal/career/application/**`, `material/**`, `submission/**`, export/Artifact adapters and tests.

**Consumes:** admitted application, frozen opportunity snapshot, confirmed profile facts, Workbench Task ID, Artifact permission port.
**Produces:** versioned structured material body, immutable published PDF/DOCX artifact refs and digest, explicit user submission record with actual channel/time/version or unknown. This task establishes the submitted material-version input consumed by Task 6 Career Preparation; it owns no `preparation/**` files.

- [ ] Test same source facts + edits yield a new version; old digest/content remains immutable; unconfirmed facts and unsupported claims are blocked or marked for review.
- [ ] Test generated PDF and editable DOCX against the same structured-body digest and inspect real files before publish; corrupt/missing export remains unpublished.
- [ ] Test user submission never invokes an external platform, accepts unknown actual version without guessing latest, and duplicate confirmation yields one submission event.
- [ ] Implement handlers and service/repository logic using Artifact for protected files; enforce actor/tenant on every download grant.
- [ ] Run `go test ./internal/career/...`; include real fixture file parser/inspection test and API client response test.
- [ ] Commit `feat: add immutable career materials and submission records`.

### Task 6: Append-only progress, reminders, quota visibility, export and deletion

**Depends on:** Task 5
**Owner / validator:** `backend_implementer` / `backend_validator`
**Concurrency:** serial backend lifecycle task; owns timeline, reminder, privacy and final migrations.

**Files:**
- Create/modify: `internal/career/preparation/**`, `timeline/**`, `reminder/**`, `privacy/**`, handlers, paired migrations and focused tests. Task 6 is the sole owner of Career Preparation service/API and `preparation/**`.

**Consumes:** Task 5 submitted-material record, confirmed facts, immutable job snapshot, Task/Artifact ownership adapters, Usage/Budget and notification delivery ports.
**Produces:** #156 Career Preparation service/API plus revision-guarded append-only event log/projection, idempotent in-app todos/minimal notifications, complete export, deletion receipt and revocation. Preparation drafts use only confirmed facts and immutable job snapshot, bind to actual submitted material version or explicitly unknown, expose editable source references, retain request/recovery after model failure, and never send or commit facts.

- [ ] Add #156 tests owned here: `TestPreparation_UsesConfirmedFactsAndImmutableJobSnapshot`, `TestPreparation_UsesActualSubmittedV2NotLatestV3`, `TestPreparation_UnknownSubmittedVersionRemainsUnknown`, `TestPreparation_SourcesAreVisibleAndEditable`, and `TestPreparation_ModelFailureRetainsRequestForRecovery`; assert there is no send/commit side effect.
- [ ] Test append/replay/correction with `expected_revision`; concurrent stale writes return conflict and never overwrite history.
- [ ] Test duplicate opportunity/event yields one todo; notification payload excludes company/job/interview details; unsubscribed users receive no push while inbox remains readable.
- [ ] Test export contains confirmed profile facts, immutable opportunity snapshots, all application events and material versions; deletion revokes old Task/Artifact links and returns retention exceptions/status.
- [ ] Implement lifecycle behavior and ensure quota exhaustion blocks new paid Run only; every existing read remains available.
- [ ] Run all Career, Workbench, notification, Artifact and migration tests. Expected all pass and no sensitive payload appears in notification fixtures.
- [ ] Commit `feat: add career progress privacy and reminders`.

### Task 7: Web Career vertical workflow

**Depends on:** Tasks 1–6 reviewed and integrated
**Owner / validator:** `frontend_implementer` / `frontend_validator`
**Concurrency:** independent Web worktree; may run alongside Task 8 and Task 9 after the API contract freeze.

**Files:**
- Create: `apps/web/src/career/**` (feature services, pages, state, components, tests).
- Modify via integration owner: Web route registry, `routes.tsx` guard, `PlatformShell.tsx` navigation, command palette if registered, and all five locale resources under `packages/i18n/src/generated/`.

**Consumes:** decoded DTOs via strict `CareerApi` and shared `@weknora/career-core` Career Desk, plus current TDesign/platform shell. The Desk owns `open/list/act/observe/reconcilePending`, revisions and scope invalidation; Web supplies injected remote/intent-store/browser adapters and presentation only, without copying Desk behavior.
**Produces:** Web flows call the shared Desk for profile/JD import, discovery/evaluation, application/material/submission, preparation, timeline/reminder and privacy with loading/empty/error/forbidden/conflict/unknown/recovery states.

- [ ] Add `WebCareerDesk_Assembly_ScopeSwitchReconcilesUnknownAndDropsLateReplies` to prove Web assembly uses shared Desk ports and preserves same-ID reconciliation/scope invalidation; test stale response discard, mismatched receipts, malformed DTO and export/deletion recovery.
- [ ] Add page tests for tri-state/hard conflict permanence, confirmed-facts-only material content, explicit user submission, version unknown and quota-read-only history.
- [ ] Add `PreparationPage_E2E_ShowsSourcesAndAllowsDraftRevision`, `PreparationPage_E2E_ModelFailureOffersRecovery`, and `SourceEvidencePage_E2E_ShowsChangeDiffCoverageAndStaleTime` Web tests; show submitted V2 vs unknown without guessing V3.
- [ ] Implement feature pages with current TDesign and route/i18n patterns; do not copy old Career page tree/styles.
- [ ] Run `pnpm --filter @weknora/api-client test`, Career web tests, `pnpm test:web`, `pnpm typecheck:web`, and production Web build. Expected all pass.
- [ ] Commit `feat: add career office web workflow` in the Web worktree; produce BASE/HEAD Review Package and evidence report.

### Task 8: Taro Mini Program Career vertical workflow

**Depends on:** Tasks 1–6 reviewed and integrated
**Owner / validator:** `frontend_implementer` / `frontend_validator`
**Concurrency:** independent Mini Program worktree; may run alongside Tasks 7 and 9; no shared generated outputs.

**Files:**
- Create: `apps/miniprogram/src/features/career/**`, `src/subpackages/career/**`, Career tests.
- Modify via integration owner: `src/app.config.ts`, `src/core/routes.ts`, `src/services/runtime.ts`, plus any TDesign MiniProgram dependency/config and lockfile changes.
- Modify only owned mini adapter files for Career-specific file/share/notification operations.

**Consumes:** decoded DTOs through canonical CareerApi and shared `@weknora/career-core` Career Desk. Career Desk owns `open/list/act/observe/reconcilePending` and scope invalidation. Taro supplies adapters only: `services/runtime.ts` maps the active deployment/tenant/actor to CareerScope and transport; a Taro-backed intent store persists scoped pending intents; `platform/files.ts`, scoped storage and current `wk-*` components provide platform behavior.
**Produces:** Career screens call the shared Desk for business state and recovery; Taro implements only scope, intent-store and platform adapters plus presentation, with `wk:` scoped private cache cleanup. It does not reimplement open/list/act/observe/reconcile or scope invalidation. Sequence #166 application/material/submission screens before #169 timeline/preparation within this client slice; #169 also consumes Task 6 #156 service and verified #166 state.

- [ ] Add `MiniCareerDesk_Assembly_ScopeSwitchInvalidatesPendingIntentsAndDropsLateReplies` through Taro runtime/intent-store stubs; prove Desk `open/list/act/observe/reconcilePending` delegates through the ports, pending writes retain the original request ID, scope switch invalidates private projection/cursor, and late replies are discarded. Add focused file-grant refetch and GET refresh/no write replay assertions.
- [ ] Add #169 tests: `MiniTimeline_ShowsCrossClientOrderAndCorrectionHistory` checks shared event ordering and traceable corrections; `MiniPreparation_UsesSubmittedVersionOrPromptsWhenUnknown` uses the actual submitted version and prompts when unknown; `MiniCareer169_AccountSwitchClearsPriorTimelineAndPreparationCache` proves prior-user cache invalidation.
- [ ] Add route/config tests ensuring every Career route maps once to the declared career subpackage.
- [ ] Resolve TDesign Miniprogram runtime/build integration explicitly; use native exception only with recorded component-level justification, not by silently reusing primitive wrappers.
- [ ] Implement pages through current features/subpackages and platform adapters; never use Web DOM APIs.
- [ ] Run `pnpm --filter @weknora/miniprogram test`, `tokens:check`, `typecheck`, `build:weapp`; expected all pass. Record `Mini169_WeChatDevToolsAndDevice_TimelineCorrectionAndPreparationKnownUnknown` evidence in WeChat DevTools and on a real device; Node stubs do not pass that gate.
- [ ] Commit `feat: add career office mini program workflow` in the Mini worktree; produce BASE/HEAD Review Package and evidence report.

### Task 9: Expo iOS/Android Career workflow and native capability adapters

**Depends on:** Tasks 1–6 and Task 0B/#145 reviewed and integrated. iOS/Android Career work can proceed without PH/#143. #144 is formally blocked by both #143 and #145: its overall closure requires the PH/#143 ruling and verified #145 evidence; only Harmony-specific implementation/evidence waits for PH, while iOS/Android #144 work may proceed once #145 is integrated.
**Owner / validator:** `frontend_implementer` / `frontend_validator`
**Concurrency:** independent Mobile worktree; may run alongside Tasks 7 and 8; record missing physical/native environments as blocked.

**Files:**
- Create: `apps/mobile/src/career/**`, `apps/mobile/src/task-office/career-entry/**`, screens/controllers and Career tests.
- Consume Task 0B `packages/mobile-core/src/task-office/**` public seam; Task 9 does not re-own the #145 native-boundary files.
- Modify via integration owner: `apps/mobile/src/composition.ts`, mobile route/screen registries.
- Modify owned Career adapters only for document share/download, notification, secure cache and app lifecycle behavior.
- Reuse `packages/mobile-core` runtime/task-office/material/vault ports; no copy of domain rules into screens.

**Consumes:** decoded DTOs via canonical CareerApi and shared `@weknora/career-core` Career Desk for Career `open/list/act/observe/reconcilePending`, revision and scope recovery. Expo supplies scope-lease mapping, durable intent-store, `packages/mobile-core` TaskOffice/runtime/file/share/notification/secure-storage adapters, and presentation; it does not reimplement Career Desk semantics.
**Produces:** consumes the already verified #145 native authenticated Task/runtime boundary from Task 0B, including its React Native/TDesign token-and-interaction mapping, no-Web-import check, and iOS/Android visual-interaction evidence. Then #144 adds a governed Task Office entry that opens an authorized existing Task, restores that same Task on refresh/restart without creating another, invalidates prior Task data and late responses on scope switch, and shows a governed offline cache or explicit unavailability. Career screens also use the shared Desk. #144 cannot close until both Task 0A/#143 ruling and verified #145 evidence are recorded; only its Harmony-specific implementation waits for PH.

- [ ] Add public seam and app tests: `MobileTaskOffice_PublicSeam_RejectsCrossTenantAndRestoresSameTask`, `ExpoTaskOffice_EntryOpensAuthorizedExistingTask`, `ExpoTaskOffice_RestartRestoresSameTaskWithoutCreatingTask`, `ExpoTaskOffice_ScopeSwitchDiscardsTaskAndLateResponses`, and `ExpoTaskOffice_OfflineShowsGovernedCacheOrUnavailable`; verify no success state is inferred offline.
- [ ] Add Career Desk controller test `ExpoCareerDesk_Assembly_UsesSharedDeskForReconcileAndScopeInvalidation` plus tenant/deployment switch, permission revocation, material digest/version unknown, preparation source/recovery behavior, and deletion cache invalidation tests.
- [ ] Implement routed UI and adapters with mobile domain logic kept behind feature/controller seam.
- [ ] Run mobile unit tests, TypeScript/lint and iOS/Android build gates available in repository; save exact logs.
- [ ] Run actual iOS and Android workflows: consume Task 0B/#145 authorized Task evidence, then #144 demonstrates entry/restart/offline/scope behavior on both targets; record build, SDK, device, authorization and each probe. Record missing physical capability as `blocked` with the exact environment requirement. Harmony remains separate under PH.
- [ ] Commit `feat: add career office expo mobile workflow` in the Mobile worktree; produce BASE/HEAD Review Package and evidence report.

### Task 10: Cross-client integration, post-trial Harmony gates and complete issue verification

**Depends on:** Tasks 7–9 integrated and Task 0A independently reviewed
**Owner / validator:** `implementer` / `backend_validator` + `frontend_validator`
**Concurrency:** final integration is serial; device-only validations may run in parallel only on independent devices and report directories.

**Files:**
- Modify only integration-owner registries/fixtures needed for client wiring.
- Create: `docs/plans/issue-140/verification/**`, `docs/plans/issue-140/final-delivery-report.md`.

**Consumes:** reviewed backend and all client worktree commits, DAG/ledger.
**Produces:** integrated branch, root story matrix, five-environment evidence, unresolved environment/release gates, complete review packages and OCR input range.

- [ ] Integrate only task-reviewed commits in dependency order; verify each source SHA and owned path before cherry-pick; rerun affected tests after conflicts.
- [ ] Run Go module/migrations/router/container tests and repository Go suite; contracts/API-client suites; Web full test/typecheck/build; Mini test/typecheck/tokens/build; Mobile test/typecheck/platform builds available.
- [ ] Verify tenant/owner authorization, cross-client event/snapshot versions, same request ID recovery, old Task/Artifact invalidation and no sensitive notification payload using integrated tests.
- [ ] For #143, cite Task 0A trial, matrix and independent ruling. Do not mark #143 blocked solely because a toolchain/device is unavailable without the attempted trial, available evidence and ruling. If a technically supported path still lacks a specific device/build prerequisite, mark only the affected native verification blocked and identify that prerequisite; if the trial rules out a maintainable path, block only affected Harmony acceptance for #144 and Harmony-dependent tickets (#163, #165, #167, #168, #171, #172) with the ruling; #145 iOS/Android remains independently verifiable. Run real Harmony native build/device workflow when the trial supports it; Android compatibility and WebView do not qualify.
- [ ] Map all 43 root stories and #141–#173 to verified evidence or an explicit environment block; no silent inheritance from historical OCR reports.
- [ ] Run final independent branch review and OCR `--audience agent` across the complete allowed delivery range plus workspace changes; save reports and all rulings in the issue plan directory.
- [ ] Commit final verification/ledger update only after all local reviews and gates have explicit outcomes.

## Parallel Wave and Conflict Matrix

| Wave | Ready work | Parallel rule | Shared-state guard |
|---|---|---|---|
| W0 | Task 0, Task 0A (#143), Task 0B (#145), then Task 1 | Task 0A follows the documentation baseline; Task 0B has no issue blocker and may run independently alongside Task 0A and Tasks 1–6. | Keep Harmony and iOS/Android native builds/devices isolated; client API indexes remain integration-critical. |
| W1 | Tasks 2–6 | Sequential | Shared Go module, migration sequences, router/container/Workbench seams, and shared DB tests. |
| W2 | Tasks 7, 8, 9 | Tasks 7/8 after Tasks 1–6; Task 9 after Tasks 1–6 + Task 0B #145. PH gates only Harmony parts/#144 closure. Task 0B may run in parallel with Task 0A and Tasks 1–6. | Separate managed worktrees, disjoint client files, isolated node_modules/build output/device/report dirs. Shared API/route/config indexes are integration-owner only. |
| W3 | Task 10 | Sequential integration; optional parallel device validation | One integration worktree; no shared DevTools port or build cache while validators run. |

The old branch’s implemented behavior can guide expectations and fixtures, but every task must pass fresh TDD, validator, independent Spec-compliance/code-quality review and current-baseline verification. No old `verified` status is carried forward.

## Consistency Self-Review

- **Coverage:** all 33 child Issues map to P0–P8 plus independent PH/#143 and P145/#145 nodes in the companion DAG. The #140 43-story acceptance matrix is included there. Tasks 2–6 cover Career server truth and Task coordination; 7–9 cover the three clients; Task 8 owns #169 Mini timeline/preparation; Task 0B independently owns #145; Task 9 consumes that evidence (including native UI mapping/import checks) and owns #144 Expo Task Office; Task 10 covers platform evidence and cross-client integration.
- **Interfaces:** Task 1 creates the contract before module/client implementation; Workbench admission comes only from existing `AdmissionCoordinator.Start`/`TaskBudgetPort`; Task/Application identity is one-to-one; Artifact refs are not client URLs. Shared registries remain integrator-owned.
- **Dependencies/cycles:** Task 0B/#145 is independently ready without PH or P0–P4; Task 0A/#143 follows the Task 0 docs baseline. Core order is 0→1→2→3→4→5→6→(7,8,9)→10; Task 9 additionally consumes verified Task 0B. PH gates only Harmony-specific work and #144 closure. No cycles. The Issue-level DAG preserves separate `contains` and `depends_on` edges.
- **Review focus coverage:** confirmation/injection in Task 3/5; ownership and scope in Task 2/4/7/8/9; idempotency in Task 4/5/6; source completeness and tri-state in Task 3; material version and unknown submission in Task 5/7/8/9.
- **Safety and proportion:** no external release or auto-submission is introduced; device gate remains evidence-bound. This plan fixes seams and behavior tests without copying the legacy implementation wholesale.
