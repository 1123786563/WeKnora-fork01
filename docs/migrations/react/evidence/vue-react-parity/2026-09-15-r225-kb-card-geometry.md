# R225 知识库列表卡片几何（2026-09-15）

## 发现

同条件 Chrome（1355×720，DPR 2）对照 Vue `KnowledgeBaseList.vue` 与 React 知识库列表：React 未初始化知识库卡片因固定高度未使用 border-box，且主列表缺少右侧 28px 内边距，导致卡片宽度和警告横幅高度/位置偏离 Vue。

## 修正

React 知识库列表主区域增加 Vue 同款 `padding-right: 28px`；初始化提示行高改为 Vue 默认 20px；知识库卡片固定高度改为 border-box，保留现有 Tailwind/shadcn 结构、操作菜单和响应式断点。

## 验证

- Vue 与 React 实测一致：警告横幅 `x=344,y=92,w=983,h=46`；网格 `x=344,y=158,w=983`；未初始化卡片 `x=344,y=202,w=319.664,h=136`。
- `pnpm exec tsx --test apps/web/src/knowledge-bases/kb-list-anatomy.test.tsx apps/web/src/knowledge-bases/list.test.ts`：23/23 通过。
- `pnpm run typecheck:web`：通过。

Wiki/文档详情入口的受保护后端状态、响应式、多语言和桌面/native 证据仍需继续补齐。
