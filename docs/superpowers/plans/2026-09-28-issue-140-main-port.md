# #140 Career Main Architecture Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the approved #140 Career Office capability into the module, contract, Workbench, TDesign Web, Expo and Taro architecture represented by the verified `issue30-sweep` baseline.

**Architecture:** Career owns career facts and workflow state; it uses authenticated Identity/Tenant context, existing Workbench Tasks/admission, Artifact, Usage/Budget, notifications and audit through narrow ports. A versioned and decoded `packages/contracts` / `packages/api-client` boundary is shared by Web, Mini Program and Expo clients. The legacy `codex/issue-140-integration` branch is a behavior reference only; no legacy history or UI tree is merged.

**Tech Stack:** Go module services and repositories, PostgreSQL + SQLite migrations, React/TDesign Web, Expo/React Native with `packages/mobile-core`, Taro 4 Mini Program, TypeScript shared contracts/API client, existing Go/Node/pnpm test toolchains.

**Spec:** [`docs/superpowers/specs/2026-09-28-issue-140-main-architecture-port-design.md`](../specs/2026-09-28-issue-140-main-architecture-port-design.md); approved product requirements [`docs/specs/2026-09-23-weknora-job-search-design.md`](../../specs/2026-09-23-weknora-job-search-design.md); issue tree/DAG [`docs/plans/issue-140/2026-09-28-issue-140-main-port-dag.md`](../../plans/issue-140/2026-09-28-issue-140-main-port-dag.md); all issue snapshots `docs/plans/issue-140/issues/issue-*.md`; ADRs 0015–0018; `CONTEXT.md`.

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

- Unconfirmed facts, model-inferred facts, or prompt-injected JD text must never become eligibility evidence or a material claim. Test profile confirmation and hostile JD fixtures in P1/P3.
- Tenant/owner spoofing, revoked scope, late async responses and stale local cache must never expose another user’s Career data. Test each service/client boundary in P1/P2/P4/P5/P6/P7.
- A timed-out create/update/export must not silently retry under a new request ID or duplicate a Task, application, event, reminder, or charge. Test same-key reconciliation and request mismatch in P2/P3/P4.
- Missing/partial source data and tri-state unknown must remain visible; a strong skills score must not hide a hard-condition conflict. Test incomplete sources and mixed evaluation evidence in P1.
- Material exports and actual submission version may be unknown or stale. Test PDF/DOCX digest/version parity, immutable edits, unknown-version retention and user-only submission in P3/P5/P6/P7.

---

## Starting Facts and File Ownership

- Execution branch: `codex/issue-140-issue30-base`; required original BASE: `db234c5eb171f2dde7427d382b55b503a038f879`. Design commit: `fa0b882def7e78ff3599cc7bef204ad2398063e5`.
- Live issue audit: 2026-09-28; 34 issues, no children deeper than one level; no comments or post-snapshot updates; dependencies match `2026-09-24-issue-140-dag.md`.
- `P0` owns new Career contracts/API-client and module boundary docs; backend domain tasks own `internal/modules/career/**` and paired migration assets; only P2 integration owner may modify central router/container/migration registration files.
- P5 owns `apps/web/src/career/**`, its dedicated route/nav registration, Career locale resources and tests. P6 owns only Mini Program career feature/subpackage/config/platform adapter additions. P7 owns only `apps/mobile/src/career/**`, its app composition additions, and Career-specific native adapter/test files.
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
- Reference: approved Spec, CONTEXT, ADR 0015–0018, issue snapshots, baseline worktree status.

**Consumes:** issue root #140, live issue-audit report, verified base SHA.  
**Produces:** durable source inventory, P0–P8 DAG, acceptance mapping, task-state ledger and isolated-worktree baseline record.

- [ ] Verify branch and `git rev-parse HEAD`; record SHA, worktree path, clean starting state, and the five untouched untracked files in source `.worktrees/issue30-sweep`.
- [ ] Record the live GitHub tree/count/contains/dependency edges, mark old implementation reports as historical evidence, and set each new port task to `pending`.
- [ ] Add files with the exact status, source, owner role, validator role, owned paths, interfaces and verification evidence. Check unique node IDs, all issue references, self-edges, cycles and root-story coverage.
- [ ] Run the DAG/document consistency check (small script or equivalent explicit count); expected 34 issues, 33 contains edges, 33 child dependency nodes covered and 0 cycles.
- [ ] Commit as `docs: record issue 140 main architecture port ledger`.

### Task 1: Define versioned Career contracts and API client

**Depends on:** Task 0  
**Owner / validator:** `implementer` (shared contracts/API boundary) / `reviewer`  
**Concurrency:** serial before backend/client feature tasks; owns shared contract indexes.

**Files:**
- Create: `packages/contracts/src/career/{index.ts,profile.ts,opportunity.ts,application.ts,search.ts,privacy.ts,index.test.ts}`
- Modify: `packages/contracts/src/index.ts`
- Create: `packages/api-client/src/career/{index.ts,types.ts,decode.ts,index.test.ts}`
- Modify: `packages/api-client/src/client.ts`
- Tests/fixtures: `packages/contracts/test/career/` and `packages/api-client` fixture tests; keep the wire fixture beside its canonical contracts and do not add a redundant `career-core` package to this baseline.

**Consumes:** product Spec and ADRs 0015–0018.  
**Produces:** `CareerApi` methods and strict wire DTOs used by server and all clients. Define exported factory `createCareerApi(request: ClientRequest): CareerApi`; response decoders reject malformed object shape, unknown enum, invalid revision/digest, receipt/request ID mismatch, and tenant/owner fields supplied as trusted client authority.

- [ ] Add failing fixture tests for all Career DTO variants, tri-state evaluation, immutable snapshots, material version/digest, expected revision and request receipt mismatch. Check package exports and consumers first; keep fixtures at `packages/contracts/test/career/`.
- [ ] Run `pnpm --filter @weknora/contracts test` and `pnpm --filter @weknora/api-client test`; verify each fails at the missing Career exports/decoder.
- [ ] Implement DTOs and decoder functions in the listed package paths; keep server-derived actor/tenant out of client request authority.
- [ ] Run both package test suites and package type checks; expected existing tests and new tests pass.
- [ ] Commit `feat: add versioned career wire contracts and api client`.

### Task 2: Establish Career module ownership, migrations and authorization seam

**Depends on:** Task 1  
**Owner / validator:** `backend_implementer` / `backend_validator`  
**Concurrency:** serial; only backend task allowed to define schema foundation or central migration registration.

**Files:**
- Create: `internal/modules/career/{README.md,module.go,manifest.go}` and `internal/modules/career/{repository,service,handler}/` scaffolding plus tests.
- Modify: `docs/architecture/backend-modules.yaml`, `docs/architecture/moves/README.md`; add `docs/architecture/moves/career.yaml`.
- Create: paired PostgreSQL `migrations/versioned/*_career_*.{up,down}.sql` and SQLite `migrations/sqlite/*_career_*.{up,down}.sql` with currently free sequence numbers; add narrowly owned registration only where repository convention requires it.
- Central registration edits, if needed: only integration owner in `internal/container/container.go`, `internal/router/router.go`, central migration code.

**Consumes:** `CareerApi` DTOs and current Identity/Tenant/Workbench/Artifact/Usage contracts.  
**Produces:** module manifest, migration ownership policy, owner-scoped repository/service base and module handler registration interface.

- [ ] Add migration tests proving next available sequence IDs for both database tracks are unique and up/down pairs are complete; prove existing migration status remains stable.
- [ ] Add failing owner/tenant tests for unauthenticated, wrong-tenant, wrong-owner and missing-resource reads/writes.
- [ ] Implement Career module manifest and migration schema for single-member space, confirmed profile facts, idempotency receipts and append-only evidence with composite tenant/owner keys.
- [ ] Register dependency ports without importing Workbench implementation internals or adding a second task runner.
- [ ] Run `go test ./internal/modules/career/... ./internal/database/... ./internal/container/...`; run architecture boundary tests. Expected all pass, no new forbidden dependency.
- [ ] Commit `feat: establish career module ownership and persistence`.

### Task 3: Profile confirmation, job intake, source snapshots and tri-state evaluation

**Depends on:** Task 2  
**Owner / validator:** `backend_implementer` / `backend_validator`  
**Concurrency:** serial backend domain task; owns profile/source/opportunity/evaluation packages.

**Files:**
- Create/modify: `internal/modules/career/profile/**`, `source/**`, `opportunity/**`, `evaluation/**`, handlers/routes local to Career and scoped tests.
- Add corresponding up/down migrations only through the Task 2 migration owner.

**Consumes:** Task 1 DTOs and Task 2 actor/tenant persistence seam.  
**Produces:** profile draft/confirm APIs, JD paste/link import, source observation and snapshot APIs, eligibility API returning `eligible|ineligible|unknown`, evidence separated from assertions.

- [ ] Write tests proving extraction creates unconfirmed facts and only explicit confirmation permits use by evaluation/material services.
- [ ] Write source tests for preserved URL/observed-at/completeness/error, restricted source prompting for full JD, no login bypass, and immutable snapshot digest.
- [ ] Write eligibility tests for each tri-state value, hard conflict remaining visible despite high skill score, and user override preserving original evaluation.
- [ ] Implement repository/service/handler behavior; reject untrusted JD instruction content from the Agent instruction path.
- [ ] Run `go test ./internal/modules/career/...`; expected all new domain tests pass and package race test passes for repository scope.
- [ ] Commit `feat: add confirmed career facts and evidence based evaluation`.

### Task 4: Search, application/task admission and quota behavior

**Depends on:** Task 3  
**Owner / validator:** `backend_implementer` / `backend_validator`  
**Concurrency:** serial around Workbench adapter and central route/container wiring.

**Files:**
- Create/modify: `internal/modules/career/search/**`, `rule/**`, `admission/**`, `internal/modules/career/handler/**` and tests.
- Integration-owner only: `internal/container/container.go`, `internal/router/router.go`, migration runner registration if applicable.
- Reuse existing Workbench admission in `internal/modules/workbench/service/workbench/admission.go` and `TaskBudgetPort`; do not edit shared Workbench internals unless separately reviewed.

**Consumes:** profile/opportunity/evaluation APIs; Workbench `AdmissionCoordinator.Start` and TaskBudget seam.  
**Produces:** one-shot search, explicit enable/disable continuous rule, opportunity batch-to-application admission, same-key unknown-outcome reconciliation, one application-to-one Workbench Task association.

- [ ] Test same `(tenant,owner,request_id)` replay returns the original result and creates at most one Task/Application; a different request ID cannot overwrite an unknown result.
- [ ] Test budget denial prevents a new billable Run but leaves profile and existing application reads available; subscriptions/reminders do not create duplicate rules.
- [ ] Implement Career owned admission adapter, search endpoints and route wiring; charge or enqueue only through existing Workbench/Usage port.
- [ ] Run `go test ./internal/modules/career/... ./internal/modules/workbench/... ./internal/router/... ./internal/container/...`; expected all pass with no second execution runtime.
- [ ] Commit `feat: integrate career search and application admission with workbench`.

### Task 5: Applications, immutable material versions, PDF/DOCX and user submission

**Depends on:** Task 4  
**Owner / validator:** `backend_implementer` / `backend_validator`  
**Concurrency:** serial backend task; owns material and application records.

**Files:**
- Create/modify: `internal/modules/career/application/**`, `material/**`, `submission/**`, export/Artifact adapters and tests.

**Consumes:** admitted application, frozen opportunity snapshot, confirmed profile facts, Workbench Task ID, Artifact permission port.  
**Produces:** versioned structured material body, immutable published PDF/DOCX artifact refs and digest, explicit user submission record with actual channel/time/version or unknown.

- [ ] Test same source facts + edits yield a new version; old digest/content remains immutable; unconfirmed facts and unsupported claims are blocked or marked for review.
- [ ] Test generated PDF and editable DOCX against the same structured-body digest and inspect real files before publish; corrupt/missing export remains unpublished.
- [ ] Test user submission never invokes an external platform, accepts unknown actual version without guessing latest, and duplicate confirmation yields one submission event.
- [ ] Implement handlers and service/repository logic using Artifact for protected files; enforce actor/tenant on every download grant.
- [ ] Run `go test ./internal/modules/career/...`; include real fixture file parser/inspection test and API client response test.
- [ ] Commit `feat: add immutable career materials and submission records`.

### Task 6: Append-only progress, reminders, quota visibility, export and deletion

**Depends on:** Task 5  
**Owner / validator:** `backend_implementer` / `backend_validator`  
**Concurrency:** serial backend lifecycle task; owns timeline, reminder, privacy and final migrations.

**Files:**
- Create/modify: `internal/modules/career/timeline/**`, `reminder/**`, `privacy/**`, handlers, paired migrations and focused tests.

**Consumes:** Task/Artifact ownership adapters, application/material APIs, Usage/Budget and notification delivery ports.  
**Produces:** revision-guarded append-only event log and projection, idempotent in-app todos/minimal notifications, complete data export, deletion receipt and revocation.

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

**Consumes:** strict `CareerApi` client, task 3–6 API schemas, current TDesign/platform shell patterns.  
**Produces:** profile/JD import, discovery/evaluation, application/material/submission, timeline/reminder and privacy pages with loading/empty/error/forbidden/conflict/unknown/recovery states.

- [ ] Add service/state tests for scope switch, stale request discard, mismatched receipts, malformed DTO, unknown mutation reconciliation and export/deletion recovery.
- [ ] Add page tests for tri-state/hard conflict permanence, confirmed-facts-only material content, explicit user submission, version unknown and quota-read-only history.
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

**Consumes:** canonical CareerApi, `services/runtime.ts` scope/transport, `platform/files.ts`, storage namespace and current `wk-*` mini components.  
**Produces:** profile/discovery/evaluation/application/material/timeline/rule/privacy flows within package/navigation limits, with `wk:` scoped private cache cleanup.

- [ ] Add assembly tests through existing Taro transport stubs for scope switch, GET refresh/no write replay, request receipt recovery, stale response rejection and file grant refetch.
- [ ] Add route/config tests ensuring every Career route maps once to the declared career subpackage.
- [ ] Resolve TDesign Miniprogram runtime/build integration explicitly; use native exception only with recorded component-level justification, not by silently reusing primitive wrappers.
- [ ] Implement pages through current features/subpackages and platform adapters; never use Web DOM APIs.
- [ ] Run `pnpm --filter @weknora/miniprogram test`, `tokens:check`, `typecheck`, `build:weapp`; expected all pass. Record WeChat DevTools import/device evidence separately; Node stubs do not pass that gate.
- [ ] Commit `feat: add career office mini program workflow` in the Mini worktree; produce BASE/HEAD Review Package and evidence report.

### Task 9: Expo Mobile Career workflow and native capability adapters

**Depends on:** Tasks 1–6 reviewed and integrated  
**Owner / validator:** `frontend_implementer` / `frontend_validator`  
**Concurrency:** independent Mobile worktree; may run alongside Tasks 7 and 8; record missing physical/native environments as blocked.

**Files:**
- Create: `apps/mobile/src/career/**`, screens/controllers and Career tests.
- Modify via integration owner: `apps/mobile/src/composition.ts`, mobile route/screen registries.
- Modify owned Career adapters only for document share/download, notification, secure cache and app lifecycle behavior.
- Reuse `packages/mobile-core` runtime/task-office/material/vault ports; no copy of domain rules into screens.

**Consumes:** canonical CareerApi, scope/epoch guards, TaskOffice, material and Vault interfaces.  
**Produces:** mobile profile/import/discovery/evaluation/application/material/submission/timeline/rule/privacy routes and native adapter contracts.

- [ ] Add controller/service tests for tenant/deployment switch, stale response discard, same-ID mutation recovery, permission revocation, material digest/version unknown and deletion cache invalidation.
- [ ] Implement routed UI and adapters with mobile domain logic kept behind feature/controller seam.
- [ ] Run mobile unit tests, TypeScript/lint and iOS/Android build gates available in repository; save exact logs.
- [ ] Run actual iOS and Android workflows when tool/device permits; record each missing physical capability check as `blocked` with required environment rather than marking complete.
- [ ] Commit `feat: add career office expo mobile workflow` in the Mobile worktree; produce BASE/HEAD Review Package and evidence report.

### Task 10: Cross-client integration, Harmony/native gates and complete issue verification

**Depends on:** Tasks 7–9 integrated  
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
- [ ] Run real Harmony native build and compatibility workflow only if native toolchain/device is available. Otherwise mark #143/#145/#163/#165/#167/#168/#171/#172 dependent gate items blocked and state exact evidence needed.
- [ ] Map all 43 root stories and #141–#173 to verified evidence or an explicit environment block; no silent inheritance from historical OCR reports.
- [ ] Run final independent branch review and OCR `--audience agent` across the complete allowed delivery range plus workspace changes; save reports and all rulings in the issue plan directory.
- [ ] Commit final verification/ledger update only after all local reviews and gates have explicit outcomes.

## Parallel Wave and Conflict Matrix

| Wave | Ready work | Parallel rule | Shared-state guard |
|---|---|---|---|
| W0 | Task 0, then Task 1 | Sequential | Issue docs and contract indexes are integration-critical. |
| W1 | Tasks 2–6 | Sequential | Shared Go module, migration sequences, router/container/Workbench seams, and shared DB tests. |
| W2 | Tasks 7, 8, 9 | Parallel after Tasks 1–6 commit and review | Separate managed worktrees, disjoint client files, isolated node_modules/build output/device/report dirs. Shared API/route/config indexes are integration-owner only. |
| W3 | Task 10 | Sequential integration; optional parallel device validation | One integration worktree; no shared DevTools port or build cache while validators run. |

The old branch’s implemented behavior can guide expectations and fixtures, but every task must pass fresh TDD, validator, independent Spec-compliance/code-quality review and current-baseline verification. No old `verified` status is carried forward.

## Consistency Self-Review

- **Coverage:** all 33 child Issues map to P0–P8 in the companion DAG. The #140 43-story acceptance matrix is included there. Tasks 2–6 cover Career server truth and Task coordination; 7–9 cover the three clients; Task 10 covers Android/iOS/Harmony/WeChat native and cross-client evidence.
- **Interfaces:** Task 1 creates the contract before module/client implementation; Workbench admission comes only from existing `AdmissionCoordinator.Start`/`TaskBudgetPort`; Task/Application identity is one-to-one; Artifact refs are not client URLs. Shared registries remain integrator-owned.
- **Dependencies/cycles:** Task order is 0→1→2→3→4→5→6→(7,8,9)→10. No cycles. The Issue-level DAG preserves separate `contains` and `depends_on` edges.
- **Review focus coverage:** confirmation/injection in Task 3/5; ownership and scope in Task 2/4/7/8/9; idempotency in Task 4/5/6; source completeness and tri-state in Task 3; material version and unknown submission in Task 5/7/8/9.
- **Safety and proportion:** no external release or auto-submission is introduced; device gate remains evidence-bound. This plan fixes seams and behavior tests without copying the legacy implementation wholesale.
