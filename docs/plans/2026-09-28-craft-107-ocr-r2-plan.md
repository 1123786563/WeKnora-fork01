# Craft #107 OCR Round 2 Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve independent high/medium OCR findings from Round 1 across budget authorization, promotion liveness, and restricted build parsing.

**Architecture:** Keep server authorization authoritative and separate read permission from mutation permission. Promotion recovery must make bounded progress without scanning the full history on every request. Restricted-build parsing must reject dangerous shell constructs without rejecting ordinary interpreter flags. Each task changes an isolated code domain and uses a separate Worktree.

**Tech Stack:** Go, TypeScript, pnpm node26, Go tests.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; findings and evidence: `docs/plans/craft-107-ocr-final-1.md`; current integration and decisions: `docs/plans/2026-09-23-craft-107-ledger.md`.

## Global Constraints

- TaskRead authorizes eligible collaborator/viewer reads; mutation endpoints must independently authorize their actor.
- Idempotency keys for resource grants must survive retries of the same logical action.
- Promotion/reconciliation work must be bounded and must not silently starve durable pending work.
- Restricted build execution must not permit shell interpretation, network escape, or false rejection of ordinary compiler/interpreter options.
- Use uncommitted checkpoints; no worker may commit, push, or modify the integration Worktree.

## Review Focus

- Collaborator reads a budget pause: show the pause and usage while `can_extend` remains false; mutation still rejects unauthorized actors.
- Repeated/failed extension requests: reuse one idempotency key until a confirmed outcome and prevent concurrent clicks.
- Promotion backlog under repeated calls: prove older pending work eventually receives a scan without unbounded work per request.
- Ordinary `bash -e`, `pwsh -NoProfile`, and PHP positional data arguments: preserve valid invocation while rejecting actual shell scripts/router execution.

---

## Parallel task/file map

- Task A owns `internal/application/service/craft_budget.go` and its focused Go tests only.
- Task B owns promotion service/repository files and their focused Go tests only; it must list exact paths in its report before editing.
- Task C owns restricted build execution/parser files and focused Go tests only; it must list exact paths in its report before editing.
- No migration, common wiring, Docker daemon, browser, or shared test database is required. No task may edit the others' files. Task D (frontend extension idempotency/authorization) remains queued until the current implementation map confirms separate file/test ownership.

### Task A: Restore TaskRead budget-pause visibility without weakening mutation gates

**Status:** implemented, independently reviewed, integrated as `R2-F16-01`; later absorbed into the durable-intent checkpoint `R2-F13-F14-F16-01`.

**Files:** `internal/application/service/craft_budget.go`, exact focused service test file identified before editing.

**Interfaces:** Consumes existing TaskRead handler gate and `authorizeBudgetActor`; produces unchanged `GetCraftBudgetPause` view for authorized readers, while `MayExtendBudget` and `ExtendAndResume` retain mutation authorization.

- [ ] Add a failing test: TaskRead collaborator can read a budget pause with `can_extend=false`; unauthorized extension remains rejected.
- [ ] Run the focused test and confirm the read path fails before the change.
- [ ] Remove only the redundant mutation authorization from the read projection; retain both mutation checks.
- [ ] Run focused Craft budget service tests and `go vet` for service package; expect all pass.

### Task B: Ensure pending promotions cannot starve behind bounded scans

**Status:** F06/F26/F35 implementation Fix Round 4 passed independent review and is integrated as `R2-F06-F26-F35-01`. Focused repository, service, and container verification is running on the integrated checkpoint. PostgreSQL runtime remains unverified because `TRPC_TEST_POSTGRES_DSN` is unset.

**Files:** exact promotion service/repository implementation and test files, named in the task brief after inspection.

**Interfaces:** Preserve current promotion API and ordering semantics; add durable cursor/progress only if existing data supports it without schema change. Do not introduce a global full-history scan or unbounded request work.

- [ ] Add a regression test with older and newer pending candidates across repeated bounded promotion scans; older work must eventually be considered.
- [ ] Run the focused test and confirm current starvation.
- [ ] Implement the smallest bounded progress mechanism compatible with the existing repository/query contract.
- [ ] Run focused promotion tests and package tests; report query bound and fairness evidence.

### Task C: Tighten restricted command classification without rejecting valid flags

**Status:** implementation and both `-cp`/`-classpath` regressions independently reviewed and integrated as `R2-F37-01`.

**Files:** restricted build command parsing/execution implementation and focused tests only.

**Interfaces:** Preserve the current allowlisted executable boundary; reject shell/router interpretation and dangerous script paths; accept ordinary `bash -e`, `pwsh -NoProfile`, and `php gen.php inputs/data.csv` invocation.

- [ ] Add regression tests for accepted ordinary flags/positional files and rejected shell/router forms.
- [ ] Run focused tests and confirm the regression.
- [ ] Implement argument-aware classification instead of rejecting short flag substrings in program text.
- [ ] Run focused restricted-exec tests and package tests.

### Task D: Keep extension authorization and idempotency aligned in the budget UI

**Status:** frontend implementation independently reviewed and integrated with backend as `R2-F13-F14-F16-01`; route and API tests plus Node 26 web typecheck pass.

**Files:** `apps/web/src/features/craft/routes.tsx` and its direct focused test file.

**Interfaces:** Consume server `can_extend` for action visibility. Preserve one idempotency key for a logical extension retry and prevent duplicate in-flight submits. F14 is split out: the current pause response has no authoritative suggested quantum, and the extension request accepts caller-supplied quantities; no deployment quantum may be invented in the UI.

- [ ] Add tests for non-owner billing administrator visibility, retry-key reuse, and in-flight disable; confirm current behavior fails.
- [ ] Implement F12/F13 using server authorization projection and run the focused Craft route tests plus node26 `typecheck:web`.
- [ ] Report F14's backend source/interface gap and do not claim it resolved until the server projects a configured quantum or an approved shared constant is introduced with evidence.

### Task E: Keep egress attempts parked when response evidence cannot prove a terminal outcome

**Status:** F07/F28 fixes independently reviewed and integrated as `R2-F07-F28-01`; explicit provenance ruling parks all 5xx responses.

**Files:** `internal/modules/craftegress/adapter.go` and its focused adapter tests.

**Interfaces:** Preserve existing journal identity/digest contract and 409/502 unresolved states. Treat proxy-generated or torn/unparseable 5xx responses as unresolved; only a verified terminal gateway envelope or unambiguous non-5xx response may release the attempt.

- [ ] Add response-matrix tests for 503/504, torn 502/2xx bodies, known unresolved and definitive envelopes.
- [ ] Run RED, implement status/envelope evidence classification, run focused and package tests.

### Task F: Close the web build trust boundary and sanitize staged style-bearing content

**Status:** F30/F31 and fail-closed F08 evidence guard independently reviewed and integrated as `R2-F30-F31-01`. F08 remains open pending the server-owned execution receipt/dispatch seam in `docs/plans/2026-09-28-craft-107-f08-execution-receipt-design.md`.

**Files:** `internal/container/craft_web_build.go`, `internal/container/craft_web_build_review.go`, their direct tests, `docker/craft/web/build.py`, Python build tests.

**Interfaces:** Build evidence must be bound to server-observed execution, never accepted solely from writable `/workspace/output/build-log.json`. Build command paths must remain inside the workspace, single-use, and bounded. Staged HTML sanitizer must reject unsafe `style`/`link` tags and style attributes according to the existing offline artifact contract.

- [ ] Add RED tests for forged log evidence, repeated/out-of-root value flags, and CSS-bearing staged input.
- [ ] Reuse an existing server-owned execution receipt if one exists; otherwise fail build evidence closed and report the missing receipt seam for a separate integration task.
- [ ] Implement F30/F31 and the safe F08 boundary, then run focused Go and Python tests; do not use Docker.

### Task G: Persist promotion scan progress and prove the revision race fence

**Status:** same as Task B; durable cursor, bounded raw-page scans, matching retry index/order, tail wrap, and revision fence are integrated. See `docs/plans/2026-09-28-craft-107-ocr-r2-promotion-report.md`.

**Files:** promotion scan source/repository, migration files for SQLite and PostgreSQL, and their focused service/repository tests.

**Interfaces:** Global recovery progress must survive promoter process restart and retry failed candidates after a bounded cooldown. Preserve run-targeted 20-second promotion from Task F26. The version publish fence must reject a publish validated against revision 1 after the persisted head advances to revision 2.

- [ ] Add a restart regression with more than one scan page: process one oldest page, construct a new promoter, and prove the later pending receipt is reached without a shared in-memory cursor.
- [ ] Add migration and dual-dialect storage for durable progress/attempt time only if existing receipt state cannot represent progress; preserve retry of failed work.
- [ ] Add a head-advance-before-publish regression and a deterministic concurrent version/head mutation test for SQLite; add Postgres-gated coverage if an existing test harness supports it.
- [ ] Run focused tests, SQLite migration coverage and available PG-gated test. If `TRPC_TEST_POSTGRES_DSN` is absent, report PG runtime as unverified.

### Task H: Make budget extension intent and quantum server-owned

**Status:** backend and frontend independently reviewed and integrated as `R2-F13-F14-F16-01`; SQLite migrations and web typecheck pass. PostgreSQL runtime validation is unavailable without `TRPC_TEST_POSTGRES_DSN`.

**Files:** backend pause/extension service, handler DTO/endpoint, SQLite and PostgreSQL migrations/repository, contract/API client types, Craft UI route tests.

**Interfaces:** A TaskRead pause projection includes one durable pending extension action `{key, extra_calls, extra_credits}`. Repeated GETs while the same pause is unresolved return the same action. The service applies only the matching pending action and records completion only after successful resume; subsequent budget pauses receive a new action. A request replay after client reload/storage loss cannot create a distinct grant. The response owns the deployment quantity; UI sends those values and does not duplicate defaults.

- [ ] Add a backend RED test for GET → intent, ambiguous grant-applied/reconcile-failed → GET returns same key and quantum, successful resume → later pause returns new key.
- [ ] Add persistence/migration coverage for SQLite and PostgreSQL, including concurrent intent creation and amount mismatch/foreign actor rejection.
- [ ] Implement server-owned intent/quantum projection and mutation binding; define exact response and request fields before front-end dispatch.
- [ ] Add front-end contract/API/UI test that browser storage is not the authority, reload retrieves the same pending server intent, and later pause consumes a fresh server intent.
- [ ] Run service/handler/package and focused web Craft tests plus shared typecheck; independently review backend and frontend slices.

### Task I: Screen HTML and pinned assets for every previewable artifact kind

**Status:** implemented, independently reviewed and integrated as `R2-F09-01`; focused tests pass. Full service package remains unverified.

**Files:** `internal/application/service/craft_artifacts.go`, `internal/application/service/craft_web_screen.go`, and focused artifact/screen tests only.

**Interfaces:** The previewable kinds (web, document, spreadsheet, slides) share the same untrusted Agent-writable output tree and preview security boundary. Apply HTML-family and pinned template asset checks independently of kind; preserve each kind's manifest validation and fail-closed errors.

- [ ] Add tests for HTML/script navigation payloads in document/slides kinds and for pinned `assets/craft-web.js` enforcement in each eligible kind.
- [ ] Run RED then implement kind-independent screening at collection time.
- [ ] Run focused service package tests and verify existing web/document/sheet/slides previews remain accepted when safe.

### Task J: Bound decompressed pax metadata in archive extraction

**Status:** implementation, trailing-stream review fix, and independent Spec/code Review complete; integrated as `R2-F10-01`.

**Files:** `internal/modules/craft/archive.go` and focused archive tests.

**Interfaces:** Enforce the existing expanded-byte ceiling on pax metadata and total decompressed tar stream, with overflow-safe accounting. Keep standard tar/gzip support, per-file/entry limits and existing input errors.

- [ ] Add compressed pax-only expansion regression that exceeds the configured limit and an in-bound pax regression.
- [ ] Run RED; implement checked byte accounting covering header metadata and file data with no unbounded allocation.
- [ ] Run focused archive and `internal/modules/craft` package tests, including race if current test harness uses it.

### Task K: Bind export-consent reads to the actual caller

**Status:** implemented, independently reviewed and integrated as `R2-F23-01`.

**Files:** `internal/application/service/craft_export_consent.go` and its focused service tests only.

**Interfaces:** Preserve TaskRead authorization and the existing immutable consent view. After TaskRead succeeds, require the authenticated caller tenant/user to equal the requested scope, matching `DecideExport` and `ExportBundle`; emit the existing caller-identity denial audit action on mismatch.

- [ ] Add a failing direct-service regression where an authorized TaskRead scope is supplied with a different authenticated caller; expect `ErrForbidden` and a deny audit record.
- [ ] Run the focused test and confirm the current missing caller binding.
- [ ] Add the caller identity check after TaskRead and before reading the consent view.
- [ ] Run focused export-consent service tests, relevant HTTP journey, and `git diff --check`; independently review the scoped behavior.

### Task L: Produce and revalidate a server-owned web build receipt

**Status:** Receipt persistence implementation Fix Round 2 passed independent review and is integrated as `R2-F08-RECEIPT-01`. SQLite repository and migration tests pass; live PostgreSQL remains unverified without `TRPC_TEST_POSTGRES_DSN`. Production dispatch, receipt evidence projection, and promotion revalidation remain blocked on an unsanctioned billing binding and provider-issued live Docker handle seam. The newest call's ModelID/Funding cannot be borrowed: no Spec or budget policy defines that as the billing rule. See the bounded seam decision in the Ledger.

**Files:** new build-receipt repository/service and SQLite/PostgreSQL migrations; T20 fixed build dispatch assembly; `internal/container/craft_web_build.go`, artifact/capture wiring and promotion revalidation; focused service/container/repository tests only. Coordinate `craft_run_capture_promotion.go` with Task G's current owner before any overlapping edit.

**Interfaces:** consume the exact T04 pinned build request through `CraftWebBuildCommandGate` (which enforces T03), then execute through `CraftDockerNormalExecService` with the real `CraftCallBinding` and Run's Docker handle. Persist an append-only receipt bound to tenant/Task/workspace/Run/activity/request digest, deployment pins, provider/container/exec identity, observed process outcome, complete output, and exact output generation/candidate manifest. Both evidence sources and promotion-time validation must use that receipt. Writable `output/build-log.json` remains diagnostic only.

- [ ] Add failing integrated journey for a forged success log with no receipt, a real successful observed receipt, and receipt mismatch/cross-Run/output-generation changes.
- [ ] Add receipt persistence migration/repository RED tests for uniqueness, conflicting retries, and restart recovery; select migration versions only after checking the complete integration ledger and active lane ownership.
- [ ] Route only the fixed build command through T04/T03 and normal Docker exec; persist the observed complete outcome before exposing build evidence.
- [ ] Bind build proof to the same sealed candidate generation and re-read it at promotion, rejecting mutation or foreign request identity.
- [ ] Run SQLite migration up/down/up, targeted journeys and affected package tests; run PostgreSQL runtime tests if configured; obtain independent review and update F08 disposition.

**Validation and review:** Each task reports changed files, exact command outputs, checkpoint ID (HEAD plus diff hashes), and remaining risks. A separate reviewer must issue both Spec compliance and code-quality verdicts before integration. The integration owner will apply reviewed changes, rerun affected tests, update the Ledger, then schedule the next ready findings.
