# Task 0 plan/DAG review-fix report

## Scope and checkpoint

- Task brief: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-fix-brief.md`.
- Worktree/branch: `/Users/wuyongjun/.codex/worktrees/issue-140-t0-plan-fix/WeKnora-fork01`, `codex/issue-140-t0-plan-fix`.
- BASE: `6baa186283b39703123b675b14b448e6a4559586`.
- Correction checkpoint: `e94fd8c1dafbc0aef21449cb0bd1e15868cf300a` (`docs: close issue 140 task 0 review gaps`). Report/ledger evidence is recorded in the following docs-only record commit.
- Task 0 stays `running` until the controller records an independent passing re-review. A subsequent parent review identified a P6 Mini Program seam omission; the follow-up correction and its separate commit are recorded below.

## Findings addressed

- **#142 high:** P1 assigns Workbench Artifact authority/adapter ownership. The plan calls for signed short-lived exact-version grants bound to tenant/owner/resource/version, authorization and revocation recheck at download, rejection tests for cross-tenant/expired/tampered/revoked/deleted versions, and an existing Task artifact download regression plus real Web download digest.
- **#143 high:** PH is an early Harmony native feasibility gate. It requires a compatibility matrix and real adaptation/build/authenticated Task/endpoint probes or reproducible failure, logs, affected ticket list and technical ruling. Android compatibility and WebView are explicitly distinct. Missing device/toolchain alone cannot finish the trial or justify marking #143 blocked. Harmony portions of downstream work are gated by its independent ruling.
- **#151 high:** P2 source reconciliation follows search results. Exact acceptance and named service tests cover evidence-only merge, separate uncertain duplicates, URL/check-time preservation, JD change/expiry/removal while old application snapshots remain, actual source/city coverage, and last-success/stale-time on failure. P5 has a named source-diff/coverage Web E2E.
- **#156 high:** P4 adds Career Preparation service/API after submission and before client completion. Named tests cover confirmed facts + immutable job snapshot, actual submitted V2 vs latest V3, explicit unknown version, editable sources, model-failure recovery, and no send/commit side effect. P5 includes named source/edit/recovery Web E2Es.
- **Career Desk medium:** ADR 0019 and the technical port design specify new platform-neutral `@weknora/career-core` absent from this BASE. Contracts own decoded DTOs, API client owns HTTP, Career Desk owns cross-client open/list/act/observe, revision and durable intent/scope recovery. The frozen interface and tests precede client tasks; Web has no `mobile-core` dependency.
- **Task 0/evidence medium:** Task 0 remains `running`; fix BASE and checkpoint SHA are exact. The old failing `git diff --check fa0b882d..79df6990` is recorded truthfully, and clean result below is bound to the corrected checkpoint/report content. The Task 1 shared-test/typecheck baseline is preserved as specified.

## Verification

Commands run in the fix worktree:

- `git diff --check` — PASS, exit 0 after the whitespace correction and before the first correction commit.
- Structural Python checks — PASS: specified issue snapshots plus spec, plan, DAG, ledger and ADR paths exist; owned Markdown links resolve; required #142/#143/#151/#156 named test/task mappings exist; Task 0 is `running`, not `verified`.
- `git diff --check e94fd8c1dafbc0aef21449cb0bd1e15868cf300a^ e94fd8c1dafbc0aef21449cb0bd1e15868cf300a` — PASS, exit 0 after report/ledger edits.
- `git diff --check` — PASS, exit 0 for final staged report/ledger delta.
- No code tests were applicable to this documentation-only task. Preserved Task 1 evidence: `pnpm test:shared` PASS 1126 (1122 pass, 4 skipped); `pnpm typecheck:shared` baseline FAIL only at unchanged `packages/views/src/chat/mermaid.ts:127,158`.

## Remaining state

Independent re-review has not run in this implementation worktree. Task 0 is intentionally not verified; controller review is required before downstream release.

## P6 follow-up correction

The Mini Program task and P6 DAG now require consumption of the shared `@weknora/career-core` Career Desk. The Desk owns `open/list/act/observe/reconcilePending` and scope invalidation; Taro supplies scope mapping, intent-store persistence and platform adapters only. Named test: `MiniCareerDesk_Assembly_ScopeSwitchInvalidatesPendingIntentsAndDropsLateReplies`.

- Follow-up correction commit: `73022bb85a4f75a3d23a1a6bb92e3f3bc3727ecd` (`docs: require career desk in mini program slice`).
- `git diff --check` — PASS, exit 0 before the follow-up commit; no whitespace errors.
- Production code remains unchanged.

## Latest independent re-review fixes

Source report: `.superpowers/sdd/2026-09-28-issue-140-main-port/task-0-re-review.md`, whose reviewed HEAD was `d23246a9ba4f8aa91e97ad7610afe125bad133bc`. The documentation correction checkpoint is commit `c544e4392f31569a0a390ae5fc54c838462ab9e7`, tree `ba1d31fd358329c0a713e2119a35fb44f842dbbe`.

- **#144 / #145:** P7 and Task 9 own Expo Task Office. P2 only supplies the consumed authorized Task read seam; #144 is removed from P2/P6. Task 9 orders #145's authenticated Task boundary before #144 and names tests for authorized entry, same-Task restart without creation, scope/late-response rejection and governed offline cache/unavailable. #144 closure waits for both #143's PH ruling and #145 evidence.
- **#169:** P6 and Task 8 explicitly own Mini timeline and preparation after Task 6 #156 service and the #166 application/material slice. Named tests cover shared event order and correction history, actual submitted version vs unknown prompt, account-switch invalidation, and WeChat DevTools/device behavior.
- **#156 ownership:** Task 5 produces the actual submitted-version record and explicitly owns no `preparation/**`; Task 6/P4 is the sole owner of `CareerPreparationService/API`, `preparation/**`, and all five service tests.
- **Career Desk:** Web Task 7 and Expo Task 9 explicitly consume `@weknora/career-core`; each has a named assembly/scope test. Their platform roles provide adapters/presentation and do not copy Desk behavior.
- **Broken provenance link:** confirmed `docs/design/job-search/2026-09-23-job-search-discovery.md` is absent from this baseline. The approved Spec now retains the user-confirmed provenance as plain text and explicitly reports the raw notes are absent; no replacement source was invented.

Commands and results:

- `python3 - <<'PY' ...` structural assertions over plan/DAG/spec — PASS: #144/#169/#156 ownership, dependency gates and required named tests; Web/Expo Career Desk consumption; source-link correction.
- Inline Python relative Markdown link check over the changed plan, DAG and approved spec — PASS; every remaining local link resolves.
- `git diff --check` before correction commit — PASS, exit 0.
- `git diff --check d23246a9ba4f8aa91e97ad7610afe125bad133bc..c544e4392f31569a0a390ae5fc54c838462ab9e7` — PASS, exit 0 for the committed correction range.
- `git diff --check` with the final Ledger/report delta present — PASS, exit 0.
- No production code changed; no implementation/device tests were run. Task 0 remains `running` until an independent re-review passes.

## Final-review fix: independent #145 native slice

The latest independent final review found that P7 still gated #145 on PH/#143 and Career backend slices. Corrected checkpoint: commit `2f5014621c7edd6f9d1c67887c174201bbffde68`, tree `d97b1766a12e6487804f1e17af12917d54adb317`.

- Added independent P145 / Task 0B for #145; it has no Issue blocker and no DAG dependencies. It consumes only the existing Expo app, Mobile Runtime scope lease and authorized Task public read. It can proceed beside PH/#143 and Career API/backend work.
- P7 / Task 9 no longer owns #145. It consumes the integrated P145 checkpoint and owns #144. #144 still requires both PH/#143 ruling and verified #145 evidence to close. PH gates Harmony-specific implementation/evidence, not independent #145 iOS/Android work.
- Updated the Mermaid graph, dependency table, acceptance mapping, Task 9 preconditions, W0/W2 schedule, Task 0 status ledger and this report. Task 0 remains `running` pending the next independent re-review.

Exact dependency assertions run in the fix worktree:

- `python3 - <<'PY'` structural assertions over plan/DAG/issue-145 snapshot — PASS: P145 dependency cell equals `—`; P145 has no PH/P0–P4 prerequisite; P7 depends on P145 with PH conditional; #144 closes only after both; Task 0B has no dependencies; Task 9 waits for P145; the issue snapshot lists no blockers.
- `git diff --check` — PASS, exit 0 before the correction commit.
- Correction checkpoint: `git rev-parse HEAD` → `2f5014621c7edd6f9d1c67887c174201bbffde68`; `git rev-parse HEAD^{tree}` → `d97b1766a12e6487804f1e17af12917d54adb317`.
- Docs-only changes; no implementation/device tests run. Await independent re-review before changing Task 0 from `running`.
