## Parent

#3

## What to build

Let a Task owner reopen a task and obtain its existing cloud workspace binding through the mobile workspace boundary, while preserving tenant and owner isolation.

## Acceptance criteria

- [ ] Reopening the same owner Task returns the durable workspace binding rather than creating another workspace.
- [ ] Guessed workspace identifiers and same-space non-owner requests cannot bypass backend scope and owner checks.
- [ ] Mobile projection rejects stale responses after backend, account, tenant, or generation changes.

## Blocked by

- M05


## Plan reference

[独立实施计划](../m09-workspace-binding.md)（发布时替换为固定提交链接）。
