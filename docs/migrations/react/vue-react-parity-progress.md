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

## 2026-09-12 Round 3

- 完成 /onboarding/workspace 功能化：策略加载/重试、can_create_tenant 分叉、创建工作区（新 api-client tenants.admin.create，后端 tenant.go:89-90 依据）、我的邀请接受/拒绝与计数联动、有效租户自动重定向。证据：evidence/vue-react-parity/2026-09-12-onboarding-functional-fixes.md；提交 b95e622、c5607f4。
- 测试：web 128/128（新增 onboarding 4 例）；typecheck 通过。
- KB list 实现子代理仍在进行；auth i18n 键迁移待其释放 packages/i18n 后由主代理执行。

## 2026-09-12 Round 4

- /login 视觉层与多语言对齐：完整移植渐变背景/动画节点/页头/语言切换/轮播/卡片样式（auth.css 逐值移植），5 locale 73 键文案程序化提取（locales.ts，零缺失），必填星号与注册卡对齐。证据：evidence/vue-react-parity/2026-09-12-login-visual-i18n.md + screenshots/login-react-visual-round4c.png。
- web 131/131；auth 文件 typecheck 干净（App.tsx 错误为 KB 子代理 WIP）。
- 仍开放：SVG 节点图标部分占位、showcase 垂直位置/分页圆点微调、locales.ts 迁入 packages/i18n、KB list 集成、其余页面。

## 2026-09-12 Round 5

- 修复轮播分页圆点可见性；浏览器交互验证语言切换（localStorage 持久化 + 整页文案切换 + 切回）与轮播分页点击，均通过。证据：evidence/vue-react-parity/2026-09-12-login-interactions.md + screenshots/login-round5-{default,en}.png；脚本 .parity-tools/verify-login.cjs。
- KB list 实现子代理继续进行中（工作区持续有其修改）。

## 2026-09-12 Round 6

- /login 背景节点图标错位修复（node-1 双 path 合并，crop 对比证据）；提交 dc169d7、e0cc0a7。
- KB list 子代理完成全部 8 项必修 + pin/duplicate/?scope=；主代理独立复核（全量测试/边界/端点真实性抽查）通过后集成提交 39a9180。证据：evidence/vue-react-parity/2026-09-12-kblist-integration.md。
- 当前基线：shared 236/236、web 131/131、typecheck x2 干净、build:web 成功。
- 下一步：KB 折叠分组节头、带后端的 KB 页截图对比、auth locales.ts 迁入 packages/i18n、继续矩阵后续页面（settings/chat/agents/organizations/integrations/embed）。
