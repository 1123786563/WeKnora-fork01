# R321 文档批量管理模式对齐

## Vue 基线

`frontend/src/views/knowledge/KnowledgeBase.vue` 通过 `batchMode` 控制卡片复选框与 `DocumentBatchBar` 的显示：普通卡片视图不显示复选框和批量工具栏；从卡片操作菜单进入“批量管理”后显示工具栏，即使当前选中数为 0，“取消选择”仍可退出批量模式。

## React 修正

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` 新增页面级 `batchMode` 状态。卡片/列表复选框和批量工具栏仅在批量模式或已有选择时显示；卡片操作菜单的“批量管理”进入批量模式，移动操作同步进入批量模式；“取消选择”在 0 项时保持可用并退出批量模式。

## 验证

- React Web 与 Vue 同认证账号 `parity-test@local.dev`、zh-CN、知识库 `Parity KB Demo`、1355×720 Chrome 实时对照：普通卡片视图的 React 批量工具栏已隐藏，与 Vue 一致。
- React AX 交互：打开文档操作菜单 → `批量管理` 后出现 `全选`、`本页已选 0 项`、批量动作和卡片复选框；点击 `取消选择` 后工具栏重新隐藏。
- 文档 chrome 与标签界面专项测试：23/23 通过。
- Web typecheck：通过。
- `git diff --check`：通过。

## 边界

本切片验证了批量模式的显示状态与退出交互；批量删除、重解析、移动和标签操作的真实后端/RBAC 结果仍需受保护运行时证据。
