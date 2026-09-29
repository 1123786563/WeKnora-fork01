# T63 Task 3 review repair — round 3

Source: independent review of round-2 package `.superpowers/sdd/plan-t63/task-3-fix-round2.patch`, SHA-256 `1c1ad2b4fc8e4de0f2ff1e400d0f4d008da51ff6f90883bf82b0202947e637da`. Verdict Spec FAIL / Quality FAIL. This plan is limited to findings below.

## F1 — High: validate Adoption lifecycle/source at accepted CAS

In `TransitionProposal`, within the same transaction as tenant-scoped listing/release guards and proposal CAS, lock/guard the Adoption row and require it remains `active` and `AcceptedReleaseID == proposal.FromReleaseID`. This closes races after service prechecks and Variant creation. Preserve permitted orphan draft semantics; proposal remains open on conflict. Add deterministic controlled interleaving tests for EndAdoption and accepted-pointer advance between draft creation/precheck and proposal CAS.

## F2 — Medium: preserve storage errors

Map only known lifecycle conflict/missing row sentinels to proposal transition conflict. Propagate database/lock errors unchanged. Add a test for non-conflict repository error propagation at the seam if deterministic injection is available.

## F3 — Medium: repair existing CAS fixture and verify it

Update `internal/application/repository/agent_upgrade_test.go` fixture for accepted transition to create matching tenant listing, from/to Releases, and active Adoption with the expected accepted release. Run the existing CAS test plus focused F1/F2 repository/service suites and race interleavings.

## Task

- Owner backend_implementer; validator backend_validator; independent reviewer reviewer.
- Worktree `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t63-task3`; branch `codex/issue30-b6-t63-task3`.
- Repair base `1cbfea5fbb57badf936f71354a4906abdfb11bbf`.
- Owned files: `internal/application/repository/agent_upgrade.go`, focused repository/service tests, and minimum supporting repository guard if needed. Report before widening beyond T63 Task 3 files.
- Keep tenant predicates, stable lock order, and conflict semantics; don't merge draft Variant creation with proposal CAS.
- RED→GREEN, focused/relevant race tests, gofmt, diff-check. Commit implementation, report exact commands/results, exact base-to-code patch/hash. No subagents.

## Review focus

Adoption active/from-release invariants are atomic with accepted CAS; storage failures remain storage failures; tests reflect required relational fixture and prove interleavings deterministically; existing orphan-draft policy is preserved.
