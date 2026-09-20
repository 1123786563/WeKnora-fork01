# M08: Minimal notification and revalidated deeplink

## Parent

Parent #3

## What to build

Notify a mobile user only that a Run completed, failed or needs approval, then revalidate their current WeKnora permission before opening the related task.

## Acceptance criteria

- [ ] Lock-screen notifications contain no task title, prompt, output, artifact or approval detail.
- [ ] Notification opening validates payload, current scope and current server authorization before navigation.
- [ ] Revocation clears stale cache and opens a forbidden state.
- [ ] Opening a notification never starts a Run or resolves an approval.
- [ ] iOS and Android notification evidence remains distinct from fixture tests.

## Blocked by

- M05: Create, continue and recover Run.
- M06: Approval compare-and-set.
- M07: Encrypted cache, offline draft and revocation.

## Further Notes

Implementation plan: [M08 Minimal notification and revalidated deeplink](../m08-notifications.md). Not published.


## Plan reference

[独立实施计划](../m08-notifications.md)（发布时替换为固定提交链接）。
