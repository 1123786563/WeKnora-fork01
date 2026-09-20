## Parent

#3

## What to build

Prevent interactive terminal or manual workspace writers from racing an active task Run, while preserving read-only terminal observation.

## Acceptance criteria

- [ ] Starting a Run closes an existing interactive PTY before Run write authority begins.
- [ ] Opening a PTY waits for a confirmed Run stop; an unknown stop outcome stays blocked and is not retried automatically.
- [ ] Observers cannot send terminal input and every action remains tenant and owner checked.

## Blocked by

- M09


## Plan reference

[独立实施计划](../m13-terminal-exclusion.md)（发布时替换为固定提交链接）。
