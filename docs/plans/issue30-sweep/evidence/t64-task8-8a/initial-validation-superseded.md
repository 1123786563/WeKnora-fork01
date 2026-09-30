# Task 8A Backend Validation

## Result

**DONE_WITH_CONCERNS** — the requested focused tests and whitespace checks pass at the assigned revision. No acceptance gap was observed for this checkpoint. PostgreSQL row-lock behavior remains unverified by the explicitly limited validation commands.

## Revision and integrity

- Worktree: `/Users/wuyongjun/.codex/worktrees/t64-task8-8a/WeKnora-fork01`
- HEAD: `ffd4bc10f7aaf226b7f3272b91b41212ddde8ac8`
- Brief: `.superpowers/sdd/plan-t64-task8-atomic-admission/task-8A-brief.md`
- Implementation report: `.superpowers/sdd/plan-t64-task8-atomic-admission/task-8A-report.md`
- SHA-256 `internal/application/repository/agent_security_guard.go`: `290cfc865f01a0e9d580dc92de9b21e64f95cae0b96fd0cca0c58a7225eaa8b0`
- SHA-256 `internal/application/repository/agent_security_guard_test.go`: `249564f73697432f4353a98bc144a26c7dc576a75dd990ad7f111ad2f868abc6`
- Worktree was clean before validation; no tracked source or test files were modified.

## Commands and exact results

1. `go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1`

   Exit: `0`

   ```text
   ok  	github.com/Tencent/WeKnora/internal/application/repository	11.989s
   ```

2. `go test ./internal/application/repository -run '^TestTenantSecurityGuardSerializesDecisiveWriteFamilies$' -count=1`

   Exit: `0`

   ```text
   ok  	github.com/Tencent/WeKnora/internal/application/repository	8.884s
   ```

3. `git diff --check`

   Exit: `0`; no output.

4. `git diff 283022dd30ba9fa5f73a238d1d6c5e6a5d4fa1e9..ffd4bc10f7aaf226b7f3272b91b41212ddde8ac8 --check`

   Exit: `0`; no output.

   Both whitespace checks and the following HEAD/status/hash capture ran in one shell command. HEAD output was the assigned SHA above; `git status --short` emitted no output; the two hashes matched those listed above.

## Independent contract review

- Helper reads are issued through the supplied `tx`; it does not start or commit transactions. The caller-owned ordered tenant guard remains responsible for serialization. The focused helper test invokes it with the test DB directly, while the separate required serialization test passed.
- Variant lookup is tenant + local Agent scoped, ordered by ID, and requests `FOR UPDATE` before inspecting mapping state. It rejects zero/ambiguous/stale/unpublished mappings and requires the supplied immutable Version ID to match the selected Variant.
- Version lookup is additionally tenant + Agent + Version scoped and requests a row lock. Release admission remains tenant-scoped and checks revocation plus the fixed release's exact dependency tuples.
- Ordinary-agent fallback requires no Variant lineage, an empty Version input, and exactly one live tenant-owned CustomAgent row. No display name or current catalog pointer is used.
- The focused table-driven test passed cases for active publication, ordinary Agent, missing/mismatched Version, retired/draft mapping, revoked Release, exact dependency tuple, allowed different version/digest, and duplicate mappings.
- No migration is introduced by this checkpoint. API authentication/authorization is outside this repository helper's contract; tenant scoping and the caller-owned guard are the relevant controls here.

## Acceptance gaps and risks

- No acceptance gap observed from the task brief or the two required test cases.
- PostgreSQL locking was not executed: focused tests use the repository's SQLite fixture path, so this evidence does not establish cross-dialect runtime locking semantics. The implementation does request GORM `FOR UPDATE` on Variant and Version queries; production PostgreSQL behavior was outside the authorized command set.
- No broader repository suite was run, as required by the checkpoint brief.
