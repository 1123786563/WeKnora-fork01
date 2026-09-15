# R211 MCP 高级配置单位后缀（2026-09-15）

## 发现

同条件 Chrome 中打开 Vue 与 React 的 MCP“添加服务”弹窗。Vue 的高级配置数字输入在超时、重试次数、重试延迟后分别显示“秒、次、秒”；React 原先只有数字输入，视觉信息不完整。

## 修正

React MCP 弹窗为三个数字输入增加 Vue 同款单位后缀，保留原有范围、数值归一化、保存 payload、键盘和表单结构。

## 验证

- `pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx`：13/13 通过。
- `pnpm run typecheck:web`：通过，无 TypeScript 错误输出。
- Vue 源码 `frontend/src/views/settings/components/McpServiceDialog.vue:285-325` 明确提供三个单位后缀；React 运行时弹窗继续显示四个 fieldset、两步导航和保存/取消操作。

受保护 MCP 创建/编辑后端、服务真实连接、响应式、多语言及桌面宿主证据仍待补齐。
