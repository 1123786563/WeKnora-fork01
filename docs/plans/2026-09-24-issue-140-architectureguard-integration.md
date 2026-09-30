# Career Architecture Guard Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the whole-repository architectureguard gate after reviewed T03/T04/T07 routes and earlier Task archive routes are integrated, with explicit Career ownership.

**Architecture:** Add Career as a 17th in-place domain manifest because approved Career design assigns its Office/routes/tables to `internal/modules/career`, separate from Workbench. Declare the three Workbench task-state files under Workbench's existing legacy file convention. Update the current guard counts only after all seven Career routes land, and preserve F0's 16/633 historical snapshot as dated provenance.

**Tech Stack:** Go architectureguard tests, YAML move manifests, architecture documentation.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; `docs/design/job-search/2026-09-23-career-codebase-design.md` §§2–3; ADR-0015; `docs/plans/issue-140/reviews/task-7-backend-review-r3.md` whole-repo gate. Implementation depends on T07 reviewed backend code integration and no source/route edits during this task.

## Global Constraints

- Career facts/rules/routes remain Career-owned; do not assign them to Workbench or Agent Runtime to silence diagnostics.
- The original F0 baseline remains labeled with its original base SHA and numbers; document the later Career/Task/Artifact additions as a dated extension.
- Do not increase route counts until code discovery verifies exactly seven Career route registrations (five T03, two T07), and explain all 11 routes added since the locked 633 count.
- Local commits authorized; no push, merge, deployment, Issue comment or closure.

## Review Focus

- A new route registration outside declared files still fails route-file coverage after this update.
- The new Career manifest covers only Career routes and does not claim Workbench assets.
- The three Workbench files remain explicit Pass B legacy obligations with navigation labels.
- Route total mismatch still fails if an unaccounted route is added or removed.
- Focused guard and full `go test ./...` pass on the same integrated code checkpoint.

---

### Task 1: Record Career ownership and reconcile guard baseline

**Depends:** T07 backend reviewed and integrated, including source/upload routes. **Owner/validator:** mechanical_worker or implementer / backend_validator. **Files:** create `docs/architecture/moves/career.yaml`; modify `docs/architecture/moves/workbench.yaml`, `docs/architecture/moves/README.md` or dated architecture addendum, `tools/architectureguard/check_test.go`, `tools/architectureguard/discovery_test.go`. **Consumes:** exact integrated route discovery counts and approved Career boundary. **Produces:** manifest coverage and lock-step, documented current baseline.

- [ ] **Step 1 PRECHECK:** Run `go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals' -v` and save exact diagnostics. Confirm 10 expected violations (three Workbench files, seven Career registrations), literal 575 + API-key 69 + handle 0 = 644. If numbers differ, stop baseline edit and identify added/removed routes first.
- [ ] **Step 2 RED:** Add/adjust focused guard tests to require Career route ownership and the exact current count, while preserving a test that unexpected new routes fail. Run tests and observe the manifest/count failure before editing manifests.
- [ ] **Step 3 MANIFEST:** Create `career.yaml` under the locked schema with `module: career`, no artificial move/alias/legacy entries, `go test ./internal/modules/career/... -count=1`, route `RegisterCareerRoutes — internal/router/routes_career.go:8`, no workers/hooks, and standard forbidden shared paths. Add `workbench_task_facts.go`, `workbench_task_state.go`, and `handler/session/workbench_task_state.go` to Workbench `legacy_files` with descriptive reason/navigation label and `passb_task: B-workbench`.
- [ ] **Step 4 BASELINE:** Update guard's current manifest count 16→17, literal route count 564→575 and total 633→644; keep API-key 69, handle 0, worker/hook counts unchanged. Explain in a dated addendum that current 644 = locked 633 + 2 pre-BASE Task archive + 2 T04 Artifact + 7 Career routes. Preserve the original F0 snapshot text as historical evidence.
- [ ] **Step 5 VERIFY:** Run the focused guard tests, `go test -count=1 ./tools/architectureguard/...`, then bounded `go test -count=1 ./...` (capture exact output and any unrelated failure), plus `git diff --check`. Independent reviewer checks manifest scope and no hidden baseline drift; commit only owned files and record SHA/evidence in Issue #140 ledger. Any new diagnostics remain a gate, not a reason to update counts blindly.

## Shared-file and interface preflight

This task owns architecture documents and guard tests only; Career handler/router files and ResourceCatalog remain owned by T07 until reviewed integration. Route-count update depends on that exact integrated code SHA. No parallel implementation task writes these manifests or guard tests. The current integrated worktree before T07 has 642 discovered routes and five Career registrations, so this plan must not be executed against that earlier state.
