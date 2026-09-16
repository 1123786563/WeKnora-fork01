# R285 文档处理进度入口

日期：2026-09-15

## Vue 基线

Vue `DocumentActionMenu.vue` 在解析中或 trace 可用时提供“查看处理过程”，打开挂载到 body 的右侧二级抽屉。`knowledge-processing-timeline.vue` 读取 `/api/v1/knowledge/:id/spans`，展示阶段状态、加载态和错误态，并在解析终态停止轮询；抽屉支持关闭和宽度调整。

## React 实现

React `KnowledgeDocumentsPage.tsx` 将“解析进度”接入卡片动作菜单。解析中或列表对象带 trace 时显示入口，打开项目 Dialog 后调用现有 `documents.spans`，使用共享 `buildKnowledgeTimeline` 生成文档解析、分块、向量化、多模态和后处理阶段；解析中的文档每 2 秒刷新，服务端进入完成/失败/取消后停止，加载和错误状态显式呈现，Dialog 保留 Escape、焦点恢复和遮罩关闭行为。

## 验证

- 文档 page-chrome：14/14；
- Web 全量回归（前一变更基线）：910/910；
- `pnpm typecheck:web`：通过；
- `pnpm build:web`：通过，仅有既有大 chunk advisory；
- `git diff --check`：通过。

这是静态菜单渲染、已有 spans 归一化逻辑、类型和构建证据；尚未完成真实后端 trace 数据、浏览器轮询/终态交互截图验证。

## 未闭环

React 当前使用项目 Dialog，尚未达到 Vue body-attached、可拖拽宽度的二级 drawer 几何；处理配置详情、失败重试/取消确认和完整 waterfall 细节仍需继续对齐。真实非空数据、RBAC、响应式、Wails 和 React Native 原生验收仍待执行。
