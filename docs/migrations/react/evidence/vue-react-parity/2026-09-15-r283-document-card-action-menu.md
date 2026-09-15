# R283 文档卡片动作菜单

日期：2026-09-15

## Vue 基线

基线源码为 `frontend/src/views/knowledge/components/DocumentActionMenu.vue`。可变更文档菜单包含下载、重建知识/取消解析、移动到目录、批量管理和删除；手动文档另有编辑入口，存在 trace 时还有“查看处理过程”。菜单项为 14px/20px、8px 12px、6px 圆角，危险操作带分隔线。

## React 实现

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` 新增卡片级 `DocumentCardActionMenu`，使用项目 `Icon`、Button/Dialog 能力和 Tailwind 令牌，提供：

- 文件/手动来源的下载动作，调用现有 `documents.download` 并触发浏览器下载；
- 已完成文档的重建知识、解析中/最终化文档的取消解析；
- 移动到目录、批量管理和单文档删除；
- 外部失焦关闭、`aria-haspopup="menu"`、展开状态、键盘可聚焦按钮及危险文案。

删除动作复用项目 Dialog 并调用现有 `documents.remove`，成功后刷新列表。菜单只在 `canContribute` 下显示，保持只读权限门禁。

## 验证

- 文档 page-chrome：14/14 通过；
- Web 全量：908/908 通过，0 失败、0 取消、0 跳过；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

该项是静态渲染、单元/回归、类型和构建证据，不等同于真实后端下载/删除成功，也不等同于浏览器双端截图验收。

## 未闭环

React 当前没有与 Vue 等价的手动文档编辑器或 trace 详情抽屉，因此菜单暂未暴露这两个入口；不能标记为完整菜单行为对齐。真实非空后端、viewer/contributor RBAC、响应式视口矩阵、Wails 和 React Native 原生运行验证仍待执行。
