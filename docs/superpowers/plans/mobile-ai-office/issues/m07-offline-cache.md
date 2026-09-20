# M07: Encrypted cache, offline draft and revocation

## Parent

Parent #3

## What to build

Protect permitted local task reads and unsent drafts with scope-isolated encryption, give users control over offline storage and sending, and remove cached content when authentication or online authorization is revoked.

## Acceptance criteria

- [ ] Cache and key material are isolated by backend, account and workspace scope.
- [ ] Corrupt or stale encrypted data fails closed and is removed.
- [ ] Logout and online revocation clear readable cached data and drafts.
- [ ] Reconnection asks the user to send or discard each draft and never auto-approves.
- [ ] Users can disable offline caching.

## Blocked by

- M05: Create, continue and recover Run.
- M06: Approval compare-and-set.

## Further Notes

Implementation plan: [M07 Encrypted cache, offline draft and revocation](../m07-offline-cache.md). Not published.


## Plan reference

[独立实施计划](../m07-offline-cache.md)（发布时替换为固定提交链接）。
