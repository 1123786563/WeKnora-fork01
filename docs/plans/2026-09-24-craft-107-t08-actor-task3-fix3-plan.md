# T08 actor Task 3 regression coverage fix 3

> **For Codex:** Restore the active historical Craft regression alongside the deleted-Session test, then obtain independent re-review. This is a test-only SDD correction.

**Source:** `2026-09-24-craft-107-t08-actor-task3-fix2-review.md` remaining Medium. No commits; record current HEAD and pre-task file hash/content.

## Global Constraints

No production source change. Preserve both fail-closed cases: active registered unmarked Craft with null actor, and retained registration after Session soft delete. A passing `go test -run` pattern that matches zero tests is not evidence. T05 graph ownership starts only after this review PASS.

## Review Focus

Both test names enumerated by `go test -list`, both executed and assert zero model/tool calls/no follow-up; no weakened assertions or changed production behavior.

## Task 1 — independent active and deleted regressions

**Depends on:** fix2 scoped quality FAIL Medium. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned file:** `internal/application/service/agent_run_graph_test.go` only. **Consumes:** current two-stage IsCraftTask production behavior. **Produces:** two separate active and deleted Session historical Craft tests.

1. Add back the original active registered, unmarked, actorless Craft Run test under its original name and retain the new deleted-Session case under a distinct name. Each must assert no model/capability execution and no follow-up admission despite Owner TaskWrite.
2. Run `go test -list` to prove both names exist, then exact-name focused tests plus affected service package subset and `git diff --check`.
3. Save exact task-local patch, pre/post file hashes and report; independent re-review checks the test actually exercises both rows.

**Acceptance:** both regressions run and pass. **Failure handling:** if active-case fixture cannot coexist, report exact collision instead of replacing one case again.
