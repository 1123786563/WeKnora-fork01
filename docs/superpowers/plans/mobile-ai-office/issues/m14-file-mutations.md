## Parent

#3

## What to build

Let a Task owner safely write, delete, and explicitly import task files on mobile once workspace capacity and execution state allow it.

## Acceptance criteria

- [ ] Every mutation is owner-authorized, idempotent, revision checked, quota checked, and blocked while Run/PTY state forbids writing.
- [ ] Failed admission writes no sandbox bytes and releases any provisional storage reservation.
- [ ] Cross-task import is an authorized copy; it never creates a shared writable workspace.

## Blocked by

- M10
- M12
- M13


## Plan reference

[独立实施计划](../m14-file-mutations.md)（发布时替换为固定提交链接）。
