# T08 actor Task 3 regression coverage fix 3 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Production source was not edited. The only source/test file changed is the plan-owned `internal/application/service/agent_run_graph_test.go`; prior concurrent edits were preserved.
- Task-local patch: `docs/plans/2026-09-24-craft-107-t08-actor-task3-fix3-task-local.patch`, SHA-256 `28180249dd67e3a80bc9d8f71da8d684cbdcd2ae03979ee931ad729c2d682fa2`.
- `git diff --check -- internal/application/service/agent_run_graph_test.go`: passed.

## Change

The shared historical Craft actorless Run fixture now backs two independently named tests: `TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor` keeps the active registered Craft Session case; `TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete` retains the soft-deleted Session case. Both start with the Owner's live TaskWrite grant, clear the durable Run actor, retain Craft registration, and assert forbidden execution before model/capability calls and no follow-up Run admission. The deleted case alone soft-deletes the Session after the live Run lease is claimed.

## Verification

Test enumeration:

```text
go test ./internal/application/service -list '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor'
```

Output listed both names:

```text
TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor
TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete
```

Both exact-name cases together:

```text
go test ./internal/application/service -run '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$|^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 3.422s`.

Each case was also executed individually:

```text
go test ./internal/application/service -run '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 6.224s`.

```text
go test ./internal/application/service -run '^TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActorAfterSessionDelete$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 3.820s`.

Adjacent active/deleted classification tests:

```text
go test ./internal/application/service -run '^TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant$|^TestCraftTaskLookupFailsClosedForMissingOrDeletedSession$' -count=1
```

Passed: `ok github.com/Tencent/WeKnora/internal/application/service 4.363s`.

## File hash

SHA-256 of complete contents:

| File | Before | After |
|---|---|---|
| `internal/application/service/agent_run_graph_test.go` | `6183f5359cdee61fc1f96994d3b22f19207dd1290ed5b3bf1b045236e1375194` | `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a` |

The before hash was captured at task start and matches the fix2 report's after hash. The patch contains only the delta from that exact task-entry file.

## Remaining gate

Independent review is pending. T05 graph ownership remains reserved until review passes. This is a test-only coverage repair and does not claim T05 or full T08 completion.
