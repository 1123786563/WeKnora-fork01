# R214 MCP 单位修正回归（2026-09-15）

MCP 高级配置单位后缀修正后完成 Web 构建和类型检查：

- `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx`：13/13
- `pnpm run typecheck:web`：通过
- `pnpm build:web`：通过，Vite 转换 2448 个模块并生成生产包

Vite 仍报告既有大 chunk advisory；不影响本次构建结果。真实 MCP 服务连接/同步与跨宿主证据仍开放。
