# React 前端 Vue 一致性优化实施计划

> **For agentic workers:** Execute task-by-task with independent review. Preserve existing worktree changes.

**Goal:** 保持 Vue 为视觉与交互基准，修正 React 页面中的布局、样式、状态和交互差异。

**Architecture:** 先建立双端基线，再收敛实际生效的 UI 样式和基础控件，最后按侧栏、知识库、设置、知识库内部页、对话和兼容页面逐项验证。桌面端复用 Web 入口，必须做共享样式回归。

**Tech Stack:** React 19、TypeScript、Tailwind v4、Radix UI、现有 @weknora/ui、@weknora/views、@weknora/i18n。

**Spec:** Vue 基准与现有 `docs/migrations/react/vue-react-parity-matrix.md`、`docs/migrations/react/vue-react-parity-progress.md`。

## Global Constraints

- 保留 Vue 页面、React 旧 URL、权限、租户、五种语言和现有后端接口。
- 不以 shadcn 默认样式替代 Vue 视觉标准。
- 不把构建或单测结果当作浏览器、Wails、Embed、移动端验收。
- 现有未提交修改归用户所有，必须保留。

## Tasks

- [ ] T01 建立 Vue/React 基线并记录侧栏、知识库列表、设置弹窗差异（代码基线已记录；实时双端截图/computed-style 仍 blocked-env）。
- [x] T02 核对并收敛实际生效的主题变量、CSS 导入顺序和公共样式归属。
- [x] T03 对齐基础控件、Dialog/Sheet/Dropdown 的状态、焦点、遮罩和关闭行为（Dialog Tab/Shift+Tab 焦点循环已落地；浏览器层仍待验收）。
- [ ] T04 对齐平台侧栏、用户菜单、会话列表和命令面板。
- [ ] T05 对齐知识库列表、创建/编辑/分享弹窗及上传状态。
- [ ] T06 对齐设置弹窗外壳、导航、深链接、关闭和角色门禁。
- [ ] T07 分批对齐所有设置分区和集成界面。
- [ ] T08 对齐文档、预览、FAQ、Wiki、图谱、数据源及上传流程。
- [ ] T09 对齐新对话、会话、消息、工具结果、审批、附件和沙箱界面。
- [ ] T10 对齐认证、引导、智能体、组织、系统管理和兼容入口（公开 login/register 已完成 Vue/React 截图、AX、computed-style 对照；受保护入口与角色矩阵仍待验收）。
- [ ] T11 执行全路由、多语言、主题、角色、桌面和 Embed 集成验收。

## Verification

`pnpm test:shared`, `pnpm typecheck:shared`, `pnpm test:web`, `pnpm typecheck:web`, `pnpm build:web`; affected desktop/embed/mobile checks are required when shared code changes.

## Execution checkpoint (2026-09-14)

T02/T03 的代码级实现已完成并通过 shared/Web 测试、类型检查、Web 构建和 `git diff --check`。T04–T10 在当前分支已有迁移实现，后续只修复由 Vue/React 同条件证据确认的差异。T01 与 T11 仍需可用的 Vue `:5173`、React `:5181` 和浏览器远程调试权限，不能用静态测试替代。
