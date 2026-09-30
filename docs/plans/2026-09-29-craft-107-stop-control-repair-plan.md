# Craft #107 OCR Control-Plane Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining actionable OCR control-plane findings F17 and F20 without granting one collaborator authority over another collaborator’s Run or breaking read/recovery paths.

**Architecture:** Keep stop authorization scoped to the Run initiator or Task Owner. Keep stop intent separate from terminal Run cancellation. Serialize StopIntent writes and fresh delegation preparation on the exact tenant/session/Run row: `PutStopIntent` takes the Run lock before the intent lock; `PrepareTask` checks the matching intent after taking that same Run lock. Existing result reuse and observation paths stay available. This plan guarantees no fresh delegation preparation after a committed Stop intent; it does not claim to cancel an external provider call already authorized/prepared before the intent commit.

**Tech Stack:** Go, GORM, SQLite tests; PostgreSQL tests are DSN-gated.

**Sources:** `CONTEXT.md`; approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`; ADR `docs/adr/0004-task-is-session.md`; OCR evidence `docs/plans/craft-107-ocr-final-1.md`; current state and decision trace `docs/plans/2026-09-23-craft-107-ledger.md`; F20 architecture audit by `/root/craft_stop_dispatch_guard_arch`.

## Global Constraints

- No Task Owner, Collaborator, or Viewer authorization may be inferred from `run.UserID` alone when `ActorUserID` identifies the Run initiator.
- Adopt the least-privilege policy assumption for F17: only current TaskWrite members may stop; the Task Owner may stop any Run in the Task, while a Collaborator may stop only a Run whose ActorUserID matches the caller. Any current TaskRead member (Owner, Collaborator, or Viewer) may query delegation status as a read operation. Nonmembers are denied. Role checks must use the existing TaskAccessChecker/caller identity, not Session owner scope alone. If the user supplies a different policy before implementation completes, revise this plan before code edits.
- Stop intent storage and fresh delegation preparation must use the same Run-row lock and lock order. A service-only check is not an atomic fence.
- Any storage error fails closed. Only `errors.Is(err, craft.ErrNotFound)` means there is no stop intent.
- Preserve stored-result reuse, prepared-task observation, run recovery, lifecycle tombstone checks, and normal non-stop cancellation semantics.
- Do not claim that a previously prepared delegation can never reach the provider after Stop; the guarantee in this plan is that no fresh preparation commits after the Stop intent.
- No commits, pushes, or changes to the integration worktree. Use an uncommitted checkpoint; preserve any unrelated worktree changes.
- Do not fabricate PostgreSQL execution evidence when `TRPC_TEST_POSTGRES_DSN` is unavailable.

## Review Focus

- Collaborator can stop/query a Run they initiated even though persisted Task ownership remains the owner; collaborator cannot control another actor’s Run; owner retains Task-wide control; Viewer and nonmember mutations remain denied.
- `PrepareTask` with a pre-existing stop intent for its exact Run returns the typed refusal before delegation insertion; a different Run in the same Session is unaffected.
- `PutStopIntent` and `PrepareTask` serialize on the Run row in the same transaction and use consistent Run→Session/intent lock order.
- Delegate result reuse and observation/recovery paths do not become blocked by a stop intent.
- The production container wires the same durable stop-intent repository contract into control and dispatch preparation; no nil or permissive fallback exists.

## Task 1: Align stop authorization with the initiating collaborator and atomically fence fresh preparation

**Dependencies:** T17 stop-intent repository and T19 delegation preparation are present at BASE `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`. F17 policy uses the least-privilege assumption above; no reply was available when this plan was prepared.

**Owner role:** `backend_implementer`.

**Validator role:** `backend_validator`; independent reviewer for Spec compliance and code quality.

**Owned files:** `internal/application/service/craft_control.go` and focused tests; `internal/application/repository/craft_stop_intent.go`, `craft_workspace.go` and their focused tests; `internal/application/service/craft_delegate.go` only if a Run-aware early guard is needed; central wiring file(s) only if required by the chosen safe seam. Record the exact file list in the Task Brief before editing. Do not touch unrelated budget, promotion, F08 receipt or renderer files.

**Consumes:** The current `CraftStopIntentStore`, `CraftStore.PrepareTask` transaction, `lockToolRun`, Run ownership fields (`UserID`, `ActorUserID`), and control-service caller scope.

**Produces:** TaskRead status and TaskWrite stop authorization, with owner-wide and actor-scoped collaborator Stop; atomic run-scoped StopIntent-vs-PrepareTask ordering; typed refusal for a stopped Run before a fresh delegation is inserted or executed.

- [ ] Add RED service tests for owner control, collaborator actor control, collaborator denial for another actor’s Run, and Viewer/nonmember mutation denial. Pin the persisted Task owner and `ActorUserID` separately.
- [ ] Run the focused service tests and record the expected F17 failures before implementation.
- [ ] Add RED repository tests proving existing requested/unknown/confirmed intent blocks fresh `PrepareTask`, a different Run in the same Session remains allowed, and intent lookup/storage failure fails closed.
- [ ] Add a deterministic Run-lock ordering test: concurrent `PutStopIntent` and `PrepareTask` must serialize so either preparation commits before Stop acceptance or later preparation refuses. Retain SQLite runtime evidence; add DSN-gated PostgreSQL evidence using the repository’s supported fixture if available.
- [ ] Run the repository tests and record expected F20 failures before implementation.
- [ ] Implement only the authorization predicates and shared Run-lock ordering described above. Preserve read/recovery branches and Run cancellation semantics.
- [ ] If adding a service guard, keep the repository transaction check authoritative and preserve lifecycle tombstone checks. Do not rely on a service preflight to establish atomicity.
- [ ] Run focused service, repository, container wiring, and relevant HTTP journey tests; run `go test` on each affected package and `git diff --check`. Expected: all pass.
- [ ] Review transaction lock order, typed error mapping, exact tenant/session/Run scoping, and absence of a new provider POST guarantee beyond the plan.
- [ ] Save a task report and complete uncommitted checkpoint with HEAD, owned file list, hashes, commands/results, and any PostgreSQL skip reason. No commit.

## Failure Handling

- If the current interfaces cannot guarantee the Run-row lock order without broad transaction refactoring, stop and report the exact conflict and safest next interface seam; do not ship a racy service-only check.
- If policy tests show Task membership/role cannot be checked for the caller’s actor, preserve fail-closed authorization and return the missing caller evidence requirement.
- If an existing read/recovery journey must be denied to implement the fence, stop and redesign rather than blocking recovery.
- PostgreSQL unavailable is a verification limitation, not a reason to weaken SQLite tests or claim cross-dialect runtime proof.

## Pre-Dispatch Consistency Check

- Acceptance coverage: F17 mapped to service authorization tests; F20 mapped to repository transaction/race tests plus service/non-regression tests.
- Shared seam: exact `(tenant_id, session_id, run_id)` key and Run-row lock; store reads use `ErrNotFound` as the only permissive absence result.
- Task boundaries: single backend task is necessary because Stop authorization, intent locking, preparation locking and container wiring share control interfaces and tests.
- Review focus: authorization isolation, serializable ordering, retained recovery paths, no expanded provider-boundary claim.
- DAG: T17 intent contract and T19 `PrepareTask` are verified inputs; F17/F20 repair task depends on both; no cycle found. T14 work remains independent.
