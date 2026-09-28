# T64 Task2 Review Fix — Assert exact dual-source lock list

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow test-only repair for one independently confirmed LOW finding.

**Goal:** Make the dual-source release lock test fail if the store returns duplicate rows for one Release ID before its map conversion hides them.

**Evidence:** T64 Task2 implementation checkpoint `da8a26181e4735b3611864704e66aa590a9bb617` passed Spec and code-quality review with no critical/high/medium findings. Reviewer found that `agent_security_test.go` converts `ListTenantReleaseLocks` to a map before checking results, so duplicate IDs can be overwritten and go undetected.

## Global Constraints

- Existing T64 Task2 worktree only, based on `da8a26181e4735b3611864704e66aa590a9bb617`.
- Only owned file: `internal/application/repository/agent_security_test.go`.
- No production code, entity, migration, service, router or other test changes.
- Local commit authorized. Independent reviewer follows. No further delegation by implementer.

## Review Focus

1. Assert exact lock row count before map construction.
2. Assert the local release ID and introduced release ID each occur exactly once; retain local-row precedence assertion for the collision case if fixture supports it.
3. Keep fixture on real public release/introduction path and tenant-scoped.

## Task 1 — Strengthen exact list assertions

**Role:** `mechanical_worker`; then independent `reviewer`.

**Worktree:** `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t64`.

**Step 1:** In `TestAgentSecurityStoreReleaseFactsCoversLocalAndIntroducedReleases`, assert `len(locks)==2` before map conversion and count rows per ReleaseID, asserting each expected ID appears exactly once. Preserve existing lock content assertions and the tenant fixture.

**Step 2 — Verify:** Run `go test ./internal/application/repository/ -run 'TestAgentSecurityStoreReleaseFactsCoversLocalAndIntroducedReleases' -count=1` and `git diff --check`.

**Step 3 — Commit:** Commit only the owned test file as `test(security): assert release lock deduplication` and report SHA, commands, outcomes and clean status.

**Acceptance:** Duplicate rows cannot be silently hidden by the test's map conversion; the focused regression passes.

## Interface and Scope Preflight

- No interface or production behavior changes.
- T64 Task3 remains blocked until this finding repair is reviewed and integrated.
