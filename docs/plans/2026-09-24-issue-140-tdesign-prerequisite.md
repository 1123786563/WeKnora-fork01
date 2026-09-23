# Issue #140 Web TDesign Prerequisite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留 #30 已提交成果的前提下，将已在 main 完成的 Web TDesign 浅色主题实现引入 #140 分支，供求职 Web 页面复用。

**Architecture:** 以 `f7753fa160927195e388c65e1dbbdff7e288506c` 为 #30 BASE。仅移植 `d35583fa324cf0ed584c00b1477bc062da9164cd` 已提交的 React Web/TDesign 源文件及工作区配置，再由 pnpm 重新解算 lockfile，保留 #30 的移动端依赖。此任务不改变 Career Office 业务合同。

**Tech Stack:** React 19、TDesign React、Vite、pnpm 10.28.2、TypeScript。

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md` 第 6 节与 ADR-0018；来源 `d35583fa3:docs/specs/2026-09-21-tdesign-react-migration-design.md`。本任务是 #140 Web 验收的先决实现检查点，不重做 TDesign 迁移设计。

## Global Constraints

- 不从 main 改换起始 BASE；#140 集成分支保持 `codex/issue-140-integration`，原 #30 工作现场不动。
- `apps/web`、`apps/embed`、`packages/views`、`packages/ui`、`packages/design-tokens` 在 #30 BASE 相对共同祖先没有内容差异；仅根 `pnpm-lock.yaml` 有 36 行 #30 依赖变更，必须保留。
- 来源必须固定为 `d35583fa324cf0ed584c00b1477bc062da9164cd` 的已提交文件，不能读取 main 未提交工作区。
- TDesign 浅色主题与 `#07c05f` 品牌色为 #140 约束；不能创建 Career 专用第二套 Tailwind/shadcn UI 栈。
- 仅此迁移所需路径可写；后续 Career Web 页面在本任务通过审查、集成并通过构建后开始。
- 任务本地提交，完整 BASE/HEAD diff 做独立 Spec 和代码质量审查。

## Review Focus

1. lockfile 重新解算后仍包含 #30 Expo/React Native 依赖；`pnpm install --frozen-lockfile` 通过。
2. Web React 构建不再引用已删除 `@weknora/ui`；Web 与 embed 构建通过。
3. `#07c05f` light token 仍进入实际 Web 主题，未被旧 Tailwind CSS 覆盖。
4. import 路径从 `packages/views` 到 `tdesign-react` 完整；类型检查和测试通过。
5. 当前 #140 的 mini TDesign 依赖保留在 lockfile，不因移植主干 Web 文件消失。

---

### Task 1: 移植已批准的 Web TDesign 状态

**Depends:** #140 计划与来源 commit 均已记录；与 T03 backend/T04/T06 不共享生产文件，lockfile 由本任务独占。**Owner/validator:** mechanical_worker / frontend_validator。**Files:** `apps/web/**`、`apps/embed/**`、`packages/views/**`、`packages/ui/**`、`packages/design-tokens/**`、根 `package.json`、`pnpm-workspace.yaml`、`pnpm-lock.yaml`。**Consumes:** main 的已提交 TDesign 实现与 #30 lockfile。**Produces:** 可构建的 TDesign Web/Embed 工作区及浅色主题 token。**Parallel:** 可与 Career 后端修复并行；根 lockfile 与小程序 T06 同时编辑时先集成 T06，再从该集成 HEAD 建立工作区。

- [ ] **Step 1:** 在独立 Worktree 记录 `git diff 29c1e56353b2b36be242018cecb43bcb3a5ef7c8 f7753fa16 -- apps/web apps/embed packages/views packages/ui packages/design-tokens package.json pnpm-workspace.yaml pnpm-lock.yaml`。预期仅 lockfile 36 行 #30 变更；否则停止并重划所有权。
- [ ] **Step 2:** 以 `git restore --source d35583fa324cf0ed584c00b1477bc062da9164cd -- apps/web apps/embed packages/views packages/ui packages/design-tokens package.json pnpm-workspace.yaml` 移植确定源树；不要覆盖当前 `pnpm-lock.yaml`。
- [ ] **Step 3:** 运行 `pnpm install --lockfile-only` 和 `pnpm install --frozen-lockfile`，预期 lockfile 包含 Expo 55、Taro 4、tdesign-react、tdesign-miniprogram，且 frozen 安装通过。
- [ ] **Step 4:** 运行 `pnpm typecheck:web`、`pnpm test:web`、`pnpm build:web`、`pnpm --filter @weknora/embed build` 与 `rg -n '#07c05f|--td-brand-color' apps/web packages/design-tokens packages/views`。预期全通过且实际主题变量可定位；失败逐个修复移植造成的导入/配置问题，不修改 Career 业务代码。
- [ ] **Step 5:** 运行 `git diff --check`，保存完整迁移统计、命令与结果；确认 `git diff f7753fa16 -- apps/mobile packages/mobile-core apps/miniprogram` 只含本 #140 已审查内容，不因本移植改变 #30 核心。提交本地改动，由 frontend_validator 和 reviewer 独立核验。

**Failure handling:** 若移植导致不可收敛的依赖或大量与 #30 的实质冲突，记录具体文件/命令，保留隔离 Worktree，不把不完整移植合入集成分支。T03 Web 子任务继续等待稳定 Web 基线。
