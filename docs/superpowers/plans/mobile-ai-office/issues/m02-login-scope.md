# M02: Trusted login and workspace scope

## Parent

Parent #3

## What to build

Make a successful WeKnora login and workspace selection establish the only client scope used by mobile task, resource and execution requests, with safe invalidation when the workspace changes or the user logs out.

## Acceptance criteria

- [ ] Cloud requests are constructed with backend, account, tenant and generation scope from successful authentication.
- [ ] Tenant changes and logout invalidate old scoped client state before new state is shown.
- [ ] Stale old-workspace responses cannot update the current screen.
- [ ] Credentials remain outside URLs, logs, chat and ordinary persistent state.

## Blocked by

- M01: Paseo source-controlled mobile shell.

## Further Notes

Implementation plan: [M02 trusted login and workspace scope](../m02-login-scope.md). Not published.


## Plan reference

[独立实施计划](../m02-login-scope.md)（发布时替换为固定提交链接）。
