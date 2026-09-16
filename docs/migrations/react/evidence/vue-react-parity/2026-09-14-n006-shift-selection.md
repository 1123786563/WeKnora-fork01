# N006 文档批量 Shift 选择证据（2026-09-14）

## Vue 基线

`frontend/src/views/knowledge/KnowledgeBase.vue` 的 `toggleSelectRow` 支持按当前列表顺序进行包含端点的 Shift 区间选择/取消选择，并记录最后选择索引；筛选变化和全选操作会清理选择锚点。

## React 实现

新增 `apps/web/src/documents/selection.ts`，由文档列表复用。普通复选框点击只切换当前行；Shift 点击按当前页文档顺序增删闭区间；过滤条件、知识库变化和全选操作清空锚点，避免跨数据集误选。

## 验证

- `selection.test.ts`：3/3，覆盖 Shift 添加、Shift 删除、普通点击。
- `upload-confirm-dialog.test.tsx`：24/24。
- 正式 `pnpm test:web`：761/761。
- `pnpm typecheck:web`：通过。

## 尚未验收

浏览器真实文档列表 Shift/鼠标框选、同条件 Vue/React 视觉对照、真实后端批量 mutation、Wails/native 仍未完成；本项不把 helper 测试当作平台验收。
