## Parent

#3

## What to build

Allow an idle task workspace to resume only when its cloud sandbox provider has proven that the persisted workspace can actually be restored.

## Acceptance criteria

- [ ] Unsupported or unknown provider outcomes leave the task blocked and never silently create a replacement workspace.
- [ ] Resume remains scoped to the current authorized owner, tenant, workspace generation, and expected revision.
- [ ] A real configured-provider test proves a persisted marker survives pause and restore before resume is advertised.

## Blocked by

- M09


## Plan reference

[独立实施计划](../m11-workspace-resume.md)（发布时替换为固定提交链接）。
