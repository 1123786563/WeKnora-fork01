## Parent

#3

## What to build

Keep each space within its cloud workspace storage limit with atomic byte reservations, while leaving users able to inspect, export, and clean up their files.

## Acceptance criteria

- [ ] Concurrent additions cannot exceed the space byte limit and failed additions release their reservations.
- [ ] Storage quota remains separate from Credits and other execution billing.
- [ ] At quota, users can still list, read, download or export, and delete files to recover space.

## Blocked by

- M09


## Plan reference

[独立实施计划](../m12-storage-quota.md)（发布时替换为固定提交链接）。
