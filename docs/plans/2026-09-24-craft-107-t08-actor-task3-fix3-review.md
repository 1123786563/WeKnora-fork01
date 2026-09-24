# T08 actor Task 3 fix 3: independent scoped re-review

Date: 2026-09-24. Read-only review of the fix 3 plan, report, exact task-local patch, prior fix 2 review, and the single owned test file. No source/test edits, OCR, or delegation. Integration HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; this is an uncommitted shared-worktree checkpoint.

## Checkpoint and verdict

The task-local patch SHA-256 is `28180249dd67e3a80bc9d8f71da8d684cbdcd2ae03979ee931ad729c2d682fa2`, matching the report. The reported before hash `6183f5359cdee61fc1f96994d3b22f19207dd1290ed5b3bf1b045236e1375194` matches the fix 2 reviewed final hash. Current complete `internal/application/service/agent_run_graph_test.go` SHA-256 is `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a`, matching the reported after hash. The patch changes only this test file; `git diff --check` on it passed.

- **Scoped Spec compliance: PASS.** The prior fix 2 production classification verdict remains supported; fix 3 does not change production code.
- **Scoped quality: PASS.** The remaining Medium test gap is closed. No new scoped finding.
- **T05/full T08: NOT VERIFIED.** This review does not claim source-record authorization or all T08 acceptance gates.

## Evidence

`go test ./internal/application/service -list '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor'` listed both independently named tests:

```text
TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor
TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete
```

I ran each exact name separately with `-count=1 -v`; each emitted its own `=== RUN` and `--- PASS` line. Both call the shared fixture in `agent_run_graph_test.go:467-521`. The active case leaves the registered Craft Session live; the deleted case soft-deletes it after claiming a live Run lease. In both, the Run snapshot is historical/unmarked, the durable actor is null, and the Owner had TaskWrite before any deletion. Assertions require `craft.ErrForbidden`, zero model calls, no MCP capability observation, and no new follow-up Run after direct `admitAfterFollowUps`. This covers the active regression and the retained-registration deletion case without a regex that silently matches zero tests.
