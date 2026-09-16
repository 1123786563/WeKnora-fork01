# R293 文档状态徽标本地化（2026-09-15）

## 变更

文档卡片状态徽标不再调用 domain 层的英文 `processingStatusLabel` 直接渲染。React 现在将 `pending/processing/finalizing/completed/failed/cancelled` 映射到现有 `knowledgeBase.parseStatus*` 五语言词条；未知或 `deleting` 状态继续使用 `statusUnknown` 安全兜底。

## 证据

- `apps/web/src/documents/page-chrome.test.tsx` 增加中文 `已完成` 徽标断言。
- 文档聚焦测试：14/14 通过。
- `pnpm test:web`：910/910 通过，0 failed/cancelled/skipped。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

## 边界

这是静态与组件级证据，不替代五语言真实浏览器截图、受保护后端数据、响应式矩阵、Wails/native 运行验收。
