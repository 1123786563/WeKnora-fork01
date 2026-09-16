# R294 文档状态与 trace 构建回归（2026-09-15）

- `pnpm exec tsx --test apps/web/src/documents/page-chrome.test.tsx apps/web/src/documents/processing-timeline.test.ts`: 17/17 通过。
- `pnpm run typecheck:web`: 通过。
- `pnpm build:web`: 通过；仅保留既有大 chunk advisory，无构建错误。
- `git diff --check`: 通过。

这批证据覆盖文档卡片状态本地化、trace 时间线/可展开节点、失败信息、重试、重建、取消确认以及可调宽度 Sheet 的静态构建回归；不替代真实后端、双端视觉、响应式和 Wails/native 验收。
