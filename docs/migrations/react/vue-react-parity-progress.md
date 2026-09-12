# Vue → React 逐页验收进度账本（vue-react-parity-progress）

## 2026-09-12 Round 1 基线

- 工作区：`.worktrees/react-multiclient`，分支 `codex/react-multiclient`，HEAD `b2d0cf6`。
- 已保留既有未提交修改（apps/mobile/expo-env.d.ts、reuse-manifest.csv、runtime-baseline.md、version-matrix.md 及 docs/superpowers/* 新文件）。
- 建立本矩阵与账本；evidence 目录 `docs/migrations/react/evidence/vue-react-parity/`。
- 初始状态：route-parity.csv 全部 53 条路由/入口均有 React 对应或明确 special-route 去向；无“React 尚无对应页面”项（待逐页核对确认非空壳）。
- 全部页面状态 review：已有测试/证据覆盖契约与路由分发，未覆盖逐页功能/布局/视觉/校验/文案/业务逻辑一致性与各端验收。
- 基线测试：见 evidence/vue-react-parity/2026-09-12-round1-baseline.md。

## 待办顺序（下一轮起逐页闭环）

1. /login + /register + /onboarding/workspace（auth 流，T03/T04）
2. /platform/knowledge-bases 列表页（T06）
3. /platform/knowledge-bases/:kbId 详情（T06–T09）
4. /platform/settings 各 section（T05/T17）
5. chat 全链路（T10–T14）
6. agents（T15）、organizations（T16）、integrations（T18）
7. embed 入口（T18）、移动端对应页（T20–T23）

## 2026-09-12 Round 1（续）

- 建立截图对比环境（.parity-tools/shoot.cjs + playwright-core，复用全局 chromium 缓存），证据：evidence/vue-react-parity/2026-09-12-visual-harness.md。
- Vue(5180)/React(5181) dev server 同视口截图 /login：React 与 Vue 基准存在整页级差异（无品牌区/页头/语言切换/i18n，英文硬编码）。证据：evidence/vue-react-parity/2026-09-12-login-visual-gap.md 与 screenshots/。
- /login 状态改 implementing；并行差异分析（login、KB list）进行中，下轮据报告实施修复。

## 2026-09-12 Round 2

- 收到 /login 与 KB list 两份逐项差异报告（13 项与 20 项，含 Vue/React 行级引用）。
- 完成鉴权页功能必修项：邀请 token 全链路（/login?token、/register?token、/join、OIDC sessionStorage 桥接、已登录自动兑换）、登录持久化租户 override + onboarding 分流、autoSetup lite 流、完整表单校验（含复杂密码模式与确认密码）、注册后预填 email、OIDC ?next。证据：evidence/vue-react-parity/2026-09-12-auth-functional-fixes.md。
- 共享层新增：api-client acceptInvitationByToken（后端 routes_auth_tenant.go:175 依据）。
- 测试：web 124/124（新增 12 例回归）；typecheck web/shared 通过。
- KB list 页实现由并行子代理进行中（App.tsx/knowledge-bases/domain/api-client/i18n knowledgeList 键），完成后主代理集成复核。
- 仍开放：auth 文案 i18n（5 locale）+ 语言切换、品牌视觉/轮播、WorkspaceOnboardingPage 功能化、KB list 全部必修项、其余页面矩阵推进。
