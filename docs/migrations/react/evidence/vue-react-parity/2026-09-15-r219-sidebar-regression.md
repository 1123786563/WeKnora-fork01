# R219 侧栏展开标签回归（2026-09-15）

PlatformShell 收起状态展开按钮本地化后完成回归：

- shell focused tests：4/4
- `pnpm test:web`：896/896
- `pnpm run typecheck:web`：通过

收起状态仍隐藏会话区域，展开按钮行为未改变；仅 aria-label/title 改为当前 locale。
