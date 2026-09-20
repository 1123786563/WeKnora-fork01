## Parent

#3

## What to build

Allow an active read-only Task collaborator to view and download only the files and artifacts that belong to the shared Task, with immediate loss of access after revocation.

## Acceptance criteria

- [ ] Shared readers receive only resources bound to their granted Task and cannot mutate or use execution capabilities.
- [ ] A revoked reader cannot download an item that appeared in an earlier list, and the mobile projection removes cached shared resources.
- [ ] Task sharing does not widen unrelated Task resources or fixed-version publication access.

## Blocked by

- M10
- M15


## Plan reference

[独立实施计划](../m16-shared-resources.md)（发布时替换为固定提交链接）。
