# Issue #31 Final Review Repair Plan — Native Framework Closure and Records

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the final independent T31 review findings for non-system Mach-O dependency closure and durable, accurate implementation records.

**Architecture:** Keep the Release gate in the checked-in Python verifier invoked by the canonical iOS build script; its tests execute that production verifier with fake `otool`. Repair the two independent documentation records in separate files: restore the cited complete SDK57/16.4 plan and update the device acceptance current-status section to point to the verified post-repair simulator evidence while retaining external blockers.

**Tech Stack:** Python 3 standard library, Node test runner with `tsx`, Expo SDK57, shell build script, Markdown execution and acceptance records.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; `docs/adr/0005-weknora-native-mobile-client.md`; Issue #31 snapshot `docs/plans/issue30-sweep/issues/issue-31.md`; final review `.superpowers/sdd/plan-t31-ios27/final-review.md`.

## Global Constraints

- Keep the generated `apps/mobile/ios/` tree ignored and uncommitted.
- Release verification must fail closed when a bundled non-system framework load cannot be resolved.
- Keep Expo SDK57 scene support and the approved iOS deployment target at `16.4`.
- Preserve routes, auth/capability gates, custom URL scheme, SecureStore adapters, Android configuration, and package boundaries.
- Keep HTTPS staging password/OIDC, real Deployment capability, and Android hardware acceptance marked pending until observed on those targets.
- Do not claim the retained app has a missing dependency; the confirmed issue is checker coverage for supported loader-relative paths.
- No credentials, push, merge, deployment, or Issue mutation.

## Review Focus

- `@rpath`, `@loader_path`, and `@executable_path` loads resolve to the correct bundled framework binary; missing and outside-bundle targets fail closed.
- Framework install names with unsupported non-system path forms fail closed with a diagnostic instead of being silently skipped.
- The canonical verifier handles valid frameworks whose binaries resolve through symlinks/versioned paths.
- The current T31 plan preserves the original task coverage and explicitly records the approved iOS 16.4 / SDK57 ruling.
- The device acceptance summary distinguishes verified local iOS27 startup/safe-area evidence from pending staging/OIDC/Android acceptance.

## DAG and ownership

| ID | Source | Depends on | Owner role | Validator | Owned files | Interface / acceptance | Status |
|---|---|---|---|---|---|---|---|
| FR1 | T31-F1 | none | frontend_implementer | reviewer | `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md` | Resolve supported bundled loader forms; malformed loads and unresolved dependencies fail closed; exact multi-arch headers are skipped without hiding path-prefix dependencies; valid fixtures and retained app pass | verified and integrated through FR1 round 3 (`d86bc885b`); integrated focused suite 21/21 and retained checker pass |
| DOC1 | T31-F2 | none | mechanical_worker | reviewer | `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`, `.superpowers/sdd/plan-t31-ios27/doc1-report.md` | Restore a durable complete execution plan at cited path, with original task coverage and the approved 16.4 amendment; check every citation resolves | implemented `d02b244fb`, scoped review PASS; integrated |
| DOC2 | T31-F3 | none | mechanical_worker | reviewer | `docs/testing/mobile-runtime-login-device-acceptance.md`, `.superpowers/sdd/plan-t31-ios27/doc2-report.md` | Update current iOS simulator status/evidence links to post-repair Release startup and safe-area proof; keep external auth/OIDC and Android checks pending | implemented `70c8efd92`, scoped review PASS; integrated |

Tasks have disjoint source/document ownership and no produced-code dependencies. Integrate verified checkpoints into the current execution worktree in task ID order before the integrated T31 review.

## Execution Ledger

- Root Issue #30, descendant #31; final review exact base `1e9315773a971dd72fe621c94e308f45a0ca4692`, head `894d456cbf1c3506563204b4e018de02ce4e6700`.
- Current integration worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; branch `codex/issue30-b6-t55-cont-exec`.
- Execution task base: `894d456cbf1c3506563204b4e018de02ce4e6700`.
- Commit strategy: local task commits are authorized; use separate worktrees for concurrent implementation; integrate reviewed task commits into the serial execution worktree. No push/merge-to-main/deploy/Issue mutation.
- Source findings and evidence are in `.superpowers/sdd/plan-t31-ios27/final-review.md`; do not broaden scope beyond T31-F1/F2/F3.

## Task FR1: Close supported loader-relative framework dependencies

**Consumes:** Existing `verify-ios-framework-closure.py <app.bundle> <generated-Podfile.properties.json>` interface and fake-`otool` fixture harness in `ios-framework-closure.test.ts`.

**Produces:** The same CLI interface; closure resolution for framework references from `@rpath`, `@loader_path`, and `@executable_path`, resolving relative to the owner Mach-O image or app executable as appropriate. Every non-system framework reference either maps to a real executable within the named embedded framework or causes a clear failure. Unsupported non-system framework path forms fail closed. System libraries remain ignored.

- [ ] Add failing integration fixtures invoking the checked-in Python checker for unresolved `@loader_path/Frameworks/Missing.framework/Missing` and `@executable_path/Frameworks/Missing.framework/Missing`; assert failure identifies owner and dependency.
- [ ] Add valid loader-relative fixtures where owner and executable-relative paths resolve to the existing declared framework binary; assert `FRAMEWORK_CLOSURE_OK`.
- [ ] Add an escaping/outside-bundle and unsupported non-system framework-load fixture; assert fail-closed behavior.
- [ ] Run the focused checker and Release-script contract tests; observe RED before implementation and GREEN after.
- [ ] Implement path classification/resolution in the production checker without changing the CLI or mode properties contract.
- [ ] Run `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check`; report exact totals/skips.
- [ ] Run the production checker on the retained Release app plus `apps/mobile/ios/Podfile.properties.json`; expected configured source mode and closure pass. Verify app executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
- [ ] Do not rebuild unless the checker change demonstrates a retained-app failure. If rebuild becomes necessary, preserve the canonical full log and install/launch/screenshot proof; do not overwrite T39 evidence.
- [ ] Commit only the owned source/test/report files locally; report commit SHA, review-package range, and validation evidence.

### FR1 review repair rounds 2–3

- Scoped review `1134dda07..97cf8dc86`: `.superpowers/sdd/plan-t31-ios27/fr1-review.md` gives Spec FAIL / Quality FAIL on two reproduced Medium false passes.
- FR1-1: `@rpath/Missing.framework` without an executable suffix is not matched, and the fallback misses it because it checks only `.framework/`. Add exact malformed-load rejection and fixture.
- FR1-2: `inspect()`'s prefix filter drops real dependency rows when an install name shares the inspected image path prefix. Skip only exact `otool` image headers and add universal-header plus path-prefix dependency fixture.
- Brief `.superpowers/sdd/plan-t31-ios27/fr1-r2-brief.md`. File scope stays checker/test/FR1 report. Round 2 running; integrate FR1 before DOC1 and DOC2 once scoped review passes.
- Round 2 commits `677f5e792` / `4dfe529c7`; scoped report `.superpowers/sdd/plan-t31-ios27/fr1-r2-review.md` marks prior findings resolved and code quality pass with Low FR1-R2-1. The substring fallback rejects `@rpath/Foo.framework.dylib`, a non-framework library.
- Round 3 brief `.superpowers/sdd/plan-t31-ios27/fr1-r3-brief.md` switches to complete `.framework` path-component recognition and pins the dylib positive case. Same files/owner/worktree; status running. FR1 remains unintegrated until round-3 review passes.
- Round-3 implementation/report commits `362b474e8` / `d9482bb7c`; review `.superpowers/sdd/plan-t31-ios27/fr1-r3-review.md` gives Spec PASS / Quality PASS with no open findings. It confirms both Medium false passes and Low false rejection are resolved, and tests contain a dependency row after each architecture header.
- FR1 status: implementation and all scoped repairs verified; integrate the FR1 commits into the serial execution worktree before DOC1 and DOC2, then obtain the final T31 integrated review.

## Task DOC1: Restore the cited current T31 execution plan

**Consumes:** predecessor plan `docs/superpowers/plans/2026-09-21-t01-ios27-scene-lifecycle.md`, `.superpowers/sdd/plan-t31-ios27/progress.md`, `task-brief.md`, `task-report.md`, review repair plan/ledger, Issue #31 snapshot, approved spec and ADR.

**Produces:** `docs/plans/issue30-sweep/plans/plan-t31-ios27.md` as the complete durable current plan already cited by the T31 progress, task brief/report, and repair plan. Preserve predecessor/source provenance. Include the approved SDK57 scene support, iOS deployment target `16.4`, task/file ownership and verifications actually covered, R1–R4 dependency/order, runtime closure/safe-area review acceptance, and explicit external acceptance blockers. Do not invent missing decisions: where original amendment evidence is absent, label the source fact and uncertainty.

- [ ] Read and compare all listed fact sources; map each predecessor-plan task/acceptance to the delivered T31 work and note any change with evidence.
- [ ] Create the plan at the cited path using the project standard header, Global Constraints, Review Focus, task interfaces/steps/checks, and self-review checklist. Preserve the prior plan as a provenance citation rather than deleting or rewriting it.
- [ ] Replace stale T31 plan references only if required; verify every cited path exists with `test -f` or `rg --files`.
- [ ] Ensure the plan does not claim staging/OIDC/Android acceptance or make absent evidence appear verified.
- [ ] Run Markdown/link-path checks available in the repository and `git diff --check`; record results and commit only owned documentation.

## Task DOC2: Correct the current iOS device-acceptance summary

**Consumes:** final review local simulator evidence; tracked files under `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/` and their hashes recorded in `.superpowers/sdd/plan-t31-ios27/fix-task-4-report.md`.

**Produces:** an accurate current-status heading and evidence pointers in `docs/testing/mobile-runtime-login-device-acceptance.md`.

- [ ] Read the full checklist and linked evidence. Update only the stale current-status heading/pointers so they reference the post-repair Release startup transcript and safe-area screenshot; distinguish the timeout-bounded console capture limitation from successful app launch evidence.
- [ ] Preserve explicit pending staging password/OIDC, real Deployment capability, and Android physical-device requirements, and preserve historical failed-launch notes as history rather than current status.
- [ ] Check every referenced evidence path exists; run `git diff --check`; commit only the owned acceptance document.

## Self-review / completion gate

- Map findings T31-F1, T31-F2, and T31-F3 one-to-one to FR1, DOC1, DOC2; no unrelated branch changes.
- FR1's new fixtures prove previously ignored inputs fail, not only that string patterns are present; valid runtime layouts still pass.
- DOC1 cites original approved inputs and actual amendment evidence; DOC2 cannot overstate what the bounded simulator capture proves.
- FR1, DOC1, DOC2 implementation and scoped-review checkpoints are integrated into the current serial worktree. Obtain an independent final review of the complete T31 slice from `1e9315773a971dd72fe621c94e308f45a0ca4692` through integrated HEAD. Only then mark T31 locally verified and release dependent #32.
