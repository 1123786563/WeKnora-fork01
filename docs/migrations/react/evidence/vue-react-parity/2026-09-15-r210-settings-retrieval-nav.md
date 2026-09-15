# R210 设置导航 retrieval 深链一致性（2026-09-15）

## 发现

Vue 当前 `Settings.vue` 的 `navItems` 不暴露 retrieval 入口，但设置状态仍保留该 section 以兼容历史深链。React 导航构建器此前会把 registry 中的 retrieval 直接展示出来，造成导航项多于 Vue。

## 修正

`settingsNavGroups` 过滤 `retrieval` 仅用于导航展示，保留 section 路由与深链解析能力；未知 section 的 fallback 分组继续生效。新增单测锁定 `general/retrieval/system` 输入只生成 `general/system` 导航项。

## 验证

- `pnpm exec tsx --test apps/web/src/settings/SettingsPage.test.tsx`：16/16 通过。
- 既有 settings 深链、系统、运行时、用户资料和租户断言保持通过。
- 浏览器复核：刷新后 React `http://localhost:5181/platform/settings?section=mcp`（Chrome tab `948800801`）的 AX/DOM 不再包含“搜索设置”；MCP 服务标题、说明和“添加服务”仍可见，与同条件 Vue `http://localhost:5173/platform/settings?section=mcp` 一致。

响应式、多语言、受保护设置后端与桌面/原生宿主证据仍待补齐。
