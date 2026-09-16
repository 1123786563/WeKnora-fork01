# R216 聊天无障碍标签跨包回归（2026-09-15）

共享聊天标签本地化后完成跨包验证：

- `pnpm test:web`：896/896
- `pnpm test:shared`：469/469
- `pnpm typecheck:shared`：通过
- `pnpm run typecheck:web`：通过

消息列表与会话分页现在从五语言 copy 表读取 aria-label，未改变聊天布局和业务行为。
