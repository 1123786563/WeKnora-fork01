# R228 Wiki 能力路由分流（2026-09-15）

React 原先在知识库详情和 Wiki 深链中直接挂载 Wiki 页面，未读取知识库的 `indexing_strategy.wiki_enabled`。现在通过能力查询分流：明确启用时进入 Wiki；禁用或未配置时回到 Vue 文档入口 `/knowledgeBase/:id`；查询失败保留原 Wiki 入口，避免后端暂时不可用时破坏已有深链。

验证：

- Wiki 路由能力测试：2/2
- `pnpm run typecheck:web`：通过（包含生产构建的 `tsc -b`）
- `pnpm build:web`：通过；Vite 仍报告既有大 chunk advisory
- `git diff --check`：通过

受保护知识库真实能力数据、浏览器双端截图、响应式及 Wails/native 证据仍待补齐。
