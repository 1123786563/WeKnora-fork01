# R284 手动文档编辑

日期：2026-09-15

## Vue 基线

Vue `frontend/src/components/manual-knowledge-editor.vue` 在 `DocumentActionMenu.vue` 的 `edit` 事件后打开手动文档编辑器，读取 `GET /api/v1/knowledge/:id` 的 metadata.content，并以 `PUT /api/v1/knowledge/manual/:id` 保存标题、正文、状态和标签/处理配置；空标题、空正文和发布内容过短时阻止提交，保存中禁止重复提交。

## React 实现

React `KnowledgeDocumentsPage.tsx` 现在对 `source=manual` 的卡片显示“编辑文档”，打开项目 Dialog 后读取详情 metadata，保留 draft/publish 状态，支持标题与 Markdown 正文编辑、加载/保存状态禁用、空值校验和保存成功后刷新列表。API client 新增 `knowledgeBases.documents.updateManual`，严格使用 Vue 的 `PUT /api/v1/knowledge/manual/:id` 路径。

## 验证

- API client 文档契约：10/10；
- 文档 page-chrome：14/14；
- Web 全量：909/909，0 失败、0 取消、0 跳过；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

上述是静态渲染、契约单测、Web 回归、类型和构建证据；尚未证明真实后端编辑成功、浏览器双端截图或权限服务端拒绝路径。

## 未闭环

当前实现尚未复用 Vue 发布前的完整 UploadConfirmDialog/处理配置流程，且 trace 详情抽屉仍未接入卡片菜单；真实非空数据、RBAC、响应式视口、Wails 和 React Native 原生验收仍待执行。
