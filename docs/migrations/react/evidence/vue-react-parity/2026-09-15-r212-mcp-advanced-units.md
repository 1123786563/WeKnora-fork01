# R212 MCP 高级配置单位后缀（2026-09-15）

## 修正

MCP 服务编辑器的高级配置输入现在与 Vue `McpServiceDialog.vue` 一致：

- 超时显示“秒”
- 重试次数显示“次”
- 重试延迟显示“秒”

输入值、边界和提交 payload 保持不变，单位仅作为输入框内的非交互视觉后缀呈现，并通过右侧内边距避免覆盖数值。

## 验证

`pnpm exec tsx --test apps/web/src/settings/McpSettingsPanel.test.tsx`：13/13 通过，包含单位后缀断言、Vue 字段顺序、传输类型和保存流程。

同条件 Chrome（1355×720，DPR 2）实测 MCP 添加抽屉：React 与 Vue 均为 `x=675,y=0,w=680,h=720`；React 专用遮罩使用右对齐、全高布局，圆角为 0，与 Vue `t-drawer--right` 几何一致。

真实 MCP 服务同步、响应式、多语言、Wails/native 证据仍待补齐。
