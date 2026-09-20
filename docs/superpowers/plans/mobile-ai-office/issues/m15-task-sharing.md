## Parent

#3

## What to build

Let a Task owner share task visibility with members of the current space as read-only collaborators and revoke it when needed.

## Acceptance criteria

- [ ] Shared members can view permitted Task conversation and Run results but cannot run, use terminal, approve, or mutate files.
- [ ] Grants only target current-space members and authorization is checked by the backend on every request.
- [ ] Revocation takes effect on the next access and clears the mobile task projection; published copies keep their separate access policy.

## Blocked by

- M05


## Plan reference

[独立实施计划](../m15-task-sharing.md)（发布时替换为固定提交链接）。
