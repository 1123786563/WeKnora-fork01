# M17: Publish immutable current-space artifact

## Parent

#3

## What to build

Let an owner publish a fixed artifact copy to a current-space resource with an independent reader ACL; Task unshare must not revoke it. Plan: `docs/superpowers/plans/mobile-ai-office/m17-artifact-publication.md`.

## Acceptance criteria

- [ ] Current-tenant owner can publish one immutable digest-bound copy.
- [ ] Tenant/viewer/quota/replay cases are behavior-tested.
- [ ] Task unshare leaves authorized publication read intact.

## Blocked by

- M10 FilesRead
- M15 TaskShare

**Status:** draft — not published.

## Plan reference

[独立实施计划](../m17-artifact-publication.md)（发布时替换为固定提交链接）。
