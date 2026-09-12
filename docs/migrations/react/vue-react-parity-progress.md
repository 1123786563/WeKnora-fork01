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

## 2026-09-12 Round 7

- auth 73 键 ×5 locale 文案迁入 packages/i18n（authMessages），LoginPage 切换为共享 formatMessage；新增 3 例 i18n 回归测试。回归全绿（i18n 7/7、web 131/131、typecheck、浏览器交互复测）。提交见 git log。证据：evidence/vue-react-parity/2026-09-12-auth-i18n-shared.md。
- 并行：settings 与 chat 两页逐项差异分析子代理进行中。
- 待办：JoinPage/WorkspaceOnboardingPage MESSAGES 迁移、settings/chat 修复实施、KB 折叠节头、带后端截图对比。

## 2026-09-12 Round 8

- onboarding/tenant/invitation 24 键 ×5 locale 迁入 packages/i18n（onboardingMessages）；JoinPage/WorkspaceOnboardingPage 弃用本地 MESSAGES；keys.test 的 auth.login 期望按 Vue 基准（en-US 'Login'）修正——非降低断言，Vue 为权威。i18n 10/10、shared 239/239、web 131/131 全绿。
- 收到 settings（15 项，必修 8：缺 10 个 section、无角色门控、envvars 无编辑器、模型选择器缺失、chathistory 模型锁丢失、密码策略未移植、websearch 凭证管理缺失、i18n）与 chat（15 项，必修 7：无流取消、无 continue-stream 续传、composer 载荷缺附件/提及/模型、steer 并发缺失、审批卡不持久、session_title 丢弃、artifacts_pending 等事件未处理）两份行级差异报告。
- chat 必修 7 项已派发实施子代理（持有 chat 文件）；settings 实施待 chat 完成后派发（避免 packages/i18n 与文件竞争），其 i18n 项由主代理统一执行。

## 2026-09-12 Round 9

- 搭建隔离真实后端环境（docker 基础设施 + make dev-app :8080 + 测试账号/测试 KB），React/Vue 双端 live 登录→KB 列表全链路打通。证据：evidence/vue-react-parity/2026-09-12-live-backend-e2e.md + screenshots/kblist-{react,vue}-live.png。
- 结构性发现：React 缺少 /platform 全局导航外壳（侧栏/图标轨/toast/新手引导/用户菜单），登记为高优待办（T04/T05 shell 范畴）。
- 修复 KB 列表调试残留 scope key 泄漏。
- chat 必修实施子代理进行中；settings 实施排队中。

## 2026-09-12 Round 10

- packages/i18n 新增 settings.ts：settings 页面 1038 个引用键 ×5 locale 全量迁移（字节级），合并链接入；新增键集一致性测试。i18n 12/12、shared 247/247、typecheck 通过。
- 平台外壳实施子代理与 chat 实施子代理并行进行中。

## 2026-09-12 Round 11

- settings 实施子代理派发（角色门控/缺失 sections/envvars 编辑器/模型选择器/模型锁/密码策略/脏检查/popstate——i18n 键已就绪）。
- 邀请与共享链接 live API E2E 通过：pending-count→accept→membership；invite-links→accept-by-token。证据：evidence/vue-react-parity/2026-09-12-invite-live-e2e.md。
- chat 实施、平台外壳实施子代理继续进行中。

## 2026-09-12 Round 12

- 矩阵更新：/platform/settings、/platform/chat/:chatid、/platform/creatChat 三行写入行级审计结论与实施状态。
- 三个实施子代理并行：chat 必修（进行中）、平台外壳（进行中）、settings 必修（已派发）。
- 观察到 chat 子代理同步修改 vite.config.ts 与 packages/domain/package.json（为新增 chat 模块接 exports/alias），属其文件域。

## 2026-09-12 Round 12（续）

- chat 必修 7 项完成并集成提交 ac8c763（主代理抽查端点与纯模块测试后落盘）。证据：evidence/vue-react-parity/2026-09-12-chat-integration.md。
- 平台外壳、settings 实施子代理继续进行中。

## 2026-09-12 Round 12（续二）

- 平台外壳完成并集成：侧栏/折叠/导航高亮/KB 过滤/用户区包裹全部受保护路由；live 复核截图对比通过。提交 fe85f30。已知偏差（图标轨、图片 logo、会话列表并入、租户切换/铃/命令面板）登记于 evidence/vue-react-parity/2026-09-12-platform-shell.md。
- chat 必修集成提交 ac8c763。settings 实施子代理进行中。

## 2026-09-12 Round 13

- settings 双端 live 基线截图入库（settings-{react,vue}-live.png）：证实审计结论（Vue 抽屉+分组+中文 vs React 英文平铺只读）。settings 实施子代理进行中。
- org/agents/integrations/embed 四页差异审计子代理派发（只读）。

## 2026-09-12 Round 15

- Vue 顶层 menu.* 26 键 ×5 locale 迁入 packages/i18n（menu.ts）；PlatformShell 本地 NAV_LABELS 删除，改用 formatMessage（个人设置用 general.personalSettings，对齐 Vue UserMenu.vue:79）。提交 5b4bd51。
- 收到 organizations/agents/integrations/embed 四页审计（A 部分：审批门控 join 与 org settings 弹窗缺失；B：AgentEditor 6753 行与聊天侧 AgentSelector 缺失，页面为 JSON 编辑器 stub；C：API-key 管理 API 表缺失、playground 非 SSE；D：embed 缺 markdown/引用渲染与文件上传/ignore default_locale）。待派发实施。
- web 全量 146/146；typecheck:web 现存错误仅 settings 实施子代理 WIP 文件（surface.test.ts 重复标识符，其收尾时自愈）。

## 2026-09-12 Round 16

- settings 必修 8 项完成并集成（d1aa583 + 面板补交）：角色门控（scopeRuntime.role() 新增）、9 缺失 section（live 数据或显式未迁移提示）、envvars 编辑器、模型选择器+allow-list、chathistory 模型锁、密码策略（domain/auth/password-policy）、脏检查保存、popstate。vite alias 补齐后 build 恢复。
- live 验证：ModelManagement 新 section + not-yet-ported 提示 + 真实数据渲染。
- 四页（org/agents/integrations/embed）必修实施子代理进行中。

## 2026-09-12 Round 17

- 四页（org/agents/integrations/embed）live 基线截图入库：live-{agents,organizations,integrations-embed,integrations-api}.png。发现 live 缺陷并转告实施代理：integrations API tab 拉取 im-channels 且空数据 shape 触发解析错误；agents 页为共享/模型调试功能（英文，无列表）与审计一致。
- 四页实施子代理进行中。

## 2026-09-12 Round 19

- KB 列表折叠分组节头完成（KB 后续项 #11 关闭）：domain 分组函数 + App.tsx 折叠 chips；live 验证通过。提交 01bccd0。证据：evidence/vue-react-parity/2026-09-12-kb-sections.md。
- 四页实施子代理继续（embed/organizations 已见其修改）。

## 2026-09-12 Round 20

- /register UI 级 live E2E 通过：客户端校验拦截（0 请求 + 4 内联错误）、有效注册恰好 1 请求、切回登录预填邮箱。证据：evidence/vue-react-parity/2026-09-12-register-live-e2e.md + screenshots/register-flow-live.png；脚本 .parity-tools/register-flow.cjs。
- 四页实施子代理继续（organizations + packages/i18n 新域进行中）。

## 2026-09-12 Round 21

- live 验证未初始化 KB 卡片 → settings 路由（KB 必修 #16）。KB 详情面深入 E2E 待租户配置测试模型。
- KB 详情面（documents/document detail/preview/processing timeline/wiki/FAQ/graph）只读审计子代理派发。
- 四页实施子代理继续。

## 2026-09-12 Round 22

- live 发现并转告 KB 详情审计代理：①设置按钮（App.tsx:398 无条件 /knowledgeBase/:id/settings）与卡片点击（openCard 模型就绪分支 → /platform/settings）对同一未初始化 KB 路由不一致；②Documents 页调试文案 "tags loaded: none…" 泄漏。截图 live-kb-documents.png。
- 文档页功能骨架（上传/文件夹/过滤/批量）在 live 可用。

## 2026-09-12 Round 24

- KB 详情审计报告收货（成熟度：documents/detail partial、timeline missing、wiki/graph stub、FAQ partial；必修 10 项）。
- 主代理即刻修复两项：#4 设置按钮统一模型就绪门控（openKbSettings helper，对齐 Vue handleCardClick）；#10 loading 哨兵替换为独立 status:'loading'。提交 16b4437。
- 其余必修（reparse/cancel+timeline、FAQ/Wiki 分页、权限门控、KB 类型路由、上传流程、批量确认、i18n 移植）待四页代理释放 packages/i18n 后派发实施。

## 2026-09-12 Round 25

- 四页必修实施完成并集成（ab4af20，25 文件；含 i18n organization/agent/integrations/embed 四新域 ×5 locale + 键集一致性测试；API-key 表格、审批门控 join、org 设置精要、agent 类型化编辑器+分组、embed markdown/上传/i18n、API-tab live 缺陷双层修复）。
- 门禁：shared 266/266、web 165/165、typecheck×2、build:web+embed 全绿；*.tsbuildinfo 入 gitignore。
- 四页 live 复测（审批 join/API-key 创建/embed 渲染）下一轮执行；矩阵四行更新。

## 2026-09-12 Round 25

- 四页 live 复测通过：API tab 解析错误与眉题消失、API-key 创建表单渲染、agents 分组列表渲染、organizations 正常。证据：evidence/vue-react-parity/2026-09-12-fourpage-live-retest.md + screenshots/live-*-after.png。
- 全局门禁（Round 25 起点）：shared 266/266、web 165/165、typecheck×2、build 全绿。

## 2026-09-12 Round 25（续）

- Organizations live E2E：建组织→invite_code→直接 join 全链路通过；发现后端创建组织时忽略 require_approval 字段（审批门控分支实际不可达）——登记为后端待决项，前端分支保留。证据：evidence/vue-react-parity/2026-09-12-org-join-live-e2e.md。

## 2026-09-12 Round 26

- KB 详情面必修实施子代理派发（reparse/cancel+timeline、FAQ/Wiki 分页、权限门控、KB 类型路由、i18n 移植 knowledgeBase 等域）。
- 上轮 org join live E2E 发现的 require_approval 后端忽略问题已登记后端待决项。

## 2026-09-12 Round 26（续）

- 多端门禁复验：desktop 测试 0 失败 + typecheck 干净；mobile 85/85 + typecheck 干净。共享层改动未破坏桌面/移动端。

## 2026-09-12 Round 27

- chat 会话页 live 基线（真实 session id 80fd9cf5…）：会话侧栏/沙箱终端控制/消息输入可用（screenshots/live-chat-session.png），但视觉形态与 Vue（botmsg 排版/引用卡片/输入区组合）差距大——登记为「chat 视觉形态」大项，排在 KB 详情实施之后。
- KB 详情审计结论已入矩阵（Round 26），KB 详情实施子代理进行中。

## 2026-09-12 Round 28

- Organizations live 复测：invite_code 参数触发的预览卡正确显示 already-member 分支（“您已经是该共享空间的成员”），组织列表含成员计数/角色（org-invite-preview-live.png）。
- chat 视觉形态实施子代理派发（packages/views/chat + ChatRoutePage 渲染层；禁改 packages/i18n）。
- KB 详情实施子代理继续。

## 2026-09-12 Round 29

- embed 渠道隔离链路搭建：agents/:id/embed-channels 创建渠道（allowed_origins 需含前端 origin）→ preview-session 签发 ems_ 令牌 → embed dev :5182（VITE_API_BASE_URL=:8080）。
- 发现 embed 冷恢复缺陷：会话 resume 请求 GET /api/v1/embed/:ch/messages/:sid/load… 网络层 ERR_FAILED（初始 token 交换/会话创建成功）。已登记 embed 缺陷待查（后续结合 EmbedApp session probe 逻辑定位）。
- chat 视觉形态实施完成并提交 54f2e59（web 175/175、build 绿）。

## 2026-09-12 Round 30

- embed 冷恢复缺陷根因定位：后端 CORS credentials+通配 origin 无效组合拦截带 Authorization 响应（后端共性缺陷，Vue 亦受影响）。登记后端待决项；embed 前端回退逻辑本身正确。证据：evidence/vue-react-parity/2026-09-12-embed-resume-defect.md。

## 2026-09-12 Round 30（补）

- embed 缺陷根因修正：非 CORS。preview-session 签发的 ems_ 令牌被 POST /embed/sessions 拒绝（401 invalid or expired token，curl 复现）——后端 embed 预览令牌语义待裁决。证据已补入 embed-resume-defect.md。

## 2026-09-12 Round 32

- KB 详情面必修 6 项集成（eb47a9c，主代理门禁复核 + live 复测文档页 i18n/无调试泄漏）。证据：evidence/vue-react-parity/2026-09-12-kbdetail-integration.md。
- 用户既有未提交修改（apps/mobile、docs/migrations/react/*.csv/md、package-lock）按要求原样保留。

## 2026-09-12 Round 33

- KB 上传流程 parity（多文件/拖拽/上传确认）与聊天侧 AgentSelector + creatChat 建议问题两个后续切片合并派发一个实施子代理（文件域：documents/chat/configuration 只读核查）。
- embed preview-session 令牌被 /embed/sessions 拒绝的问题已登记后端待决（Round 30）；待后端裁决后补 embed 预览 live 验证。

## 2026-09-12 Round 34

- FAQ 型 KB 路由 live 验证通过：/knowledgeBase/{faqId} 自动 replace 到 /faq（screenshots/live-faq-routing.png）；测试 FAQ KB（parity-faq-kb）入隔离库。
- 矩阵标记 KB 详情必修 #1/#2/#3/#4/#7/#9/#10 已实施，#5/#6/#8 开放项登记。
- 上传流程 + AgentSelector 实施子代理进行中。

## 2026-09-12 Round 35

- settings 视觉形态（抽屉/分组导航/图标/关闭行为）实施子代理派发：要求复用 packages/i18n settings.* 键、保留全部功能接线（角色门控/popstate/面板）、live 前后截图对比 settings-vue-live.png。
- 上传流程 + AgentSelector 实施子代理继续。

## 2026-09-12 Round 36

- 上传 pipeline、AgentSelector、starter questions、embed CORS 修复集成（40a5d86）。门禁：shared 276/276、web 194/194、build 全绿；embed live 渲染成功。
- 后端待决项维持登记：POST /sessions 不支持 agent_id；require_approval 创建字段被忽略；冷刷新审批持久端点。

## 2026-09-12 Round 40

- settings 抽屉视觉形态完成并收尾：固定遮罩 + 1080x780 模态 + 分组导航（账户/空间/模型/数据与扩展/系统管理/平台，图标 + settings.* 标签）+ ✕ 关闭回知识库列表；角色门控/popstate/全部面板功能接线保留；styles.css 追加式 wks-* 样式。遗留中间态（重复 import）已修复（aa77f7e）。
- 门禁：web 194/194、typecheck:web 0 错误、build:web ✓。证据：screenshots/settings-form-{before,after}.png 对比 settings-vue-live.png。
- settings 页状态：视觉形态 + 功能必修完成后仍维持 review（待全页截图矩阵与多端验收收敛）。

## 2026-09-12 Round 41

- chat UX 收尾切片派发：消息时间戳分隔（messageTimestamp 移植 domain）、助手复制按钮、流式打字指示器、回到底部按钮、会话分组模式持久化、creatChat 空态建议问题 chips + 建会话后首条消息带 agent_id（后端 sessions 不收 agent_id 的缺口保持如实记录）。
- settings 抽屉完成收尾（aa77f7e/0a946c2）后，主要页面域实施全部闭环；进入 UX 细节与验收收敛阶段。

## 2026-09-12 Round 43

- chat UX 切片首个实施代理中途失败（未留下任何变更，树保持干净全绿）；同范围重新派发（retry 代理），要求逐项报告状态。

## 2026-09-12 Round 45

- 尝试 Wails macOS 打包（REACT_FRONTEND=1 package-mac-app.sh）：web 构建步被 chat UX retry 代理进行中变更阻断（@weknora/domain/chat/copy-answer 模块未建、message-list readonly 断言）——时序问题非缺陷；chat UX 完成集成后重跑打包验证。
- 顺带确认打包脚本对工作树清洁度敏感：与在途子代理并行时会互相干扰，后续打包统一在无在途实施时执行。

## 2026-09-12 Round 47

- 集成核对基准确认：Vue 会话分组持久化键为 `weknora:session-group-mode`，默认模式 `none`（frontend/src/components/sessionGrouping.ts:5-6）。chat UX retry 代理交付时按此键名核验。

## 2026-09-12 Round 48

- chat UX 收尾切片完成集成（62a77a9：时间戳分隔/复制按钮/打字指示器/回到底部/分组持久化/creatChat 建议 + domain message-timestamps/copy-answer/session-grouping 模块与 3 个新测试文件）。
- **根治 gitignore 缺陷**：裸 `web/` 规则锚定为 `/web/`（该规则此前静默吞掉 apps/web 下全部新源文件——resume/steer-submit/starter-questions/actions/processing-timeline/upload-pipeline/permissions/pagination/i18n.ts 等已全部补交入库），并为 apps/{web,embed,desktop}/dist 增加显式忽略。共享/前端门禁复验：shared 293/293、web 198/198、typecheck 干净。

## 2026-09-12 Round 50

- chat UX 收尾切片完成集成（62a77a9）+ gitignore web/ 规则锚定（/web/），被吞源文件全部补交（c58caac/3340ff7）。
- Wails macOS 打包验证通过：React bundle 构建 + wails build + 自签名 .app（codesign 校验通过）；集成修复 desktop vite alias 缺失（6c8775e）+ 后端 CORS X-Embed-Visitor。
- 门禁：shared 293/293、web 198/198、typecheck×2、build:web 全绿。
- 待办：embed CORS 修复后的 live 复测、四页剩余后续项、Wails 运行时交互验收、iOS/Android 原生验收。

## 2026-09-12 Round 52

- settings 抽屉 section 标题本地化：SECTION_TITLE_KEYS 映射（models→模型管理 等），live 验证 h2 不再出现英文键名（live-settings-models-title.png）。提交 784b25a。
- chat UX retry 代理进行中（session-state/message-list/chat-page 等文件持续更新）。

## 2026-09-12 Round 53

- Wails macOS 打包在干净树上复跑成功（含 chat UX 收尾内容），自签名 .app 生成。embed-resume-defect 记录文件追加复跑确认。
- chat UX retry 代理的工作已于 62a77a9 集成，无遗留未提交内容。

## 2026-09-12 Round 54

- chat 会话页双端矩阵截图入库（同 session 80fd9cf5，用户消息双端一致呈现）。Vue 侧流式 404 为环境限制（租户无 chat 模型 + dev 代理路径），非 React 缺陷；React 侧显示 continue-stream 恢复提示（必修 #2 行为生效）。矩阵索引已更新。
- Upload/AgentSelector retry 代理完成（内容已在 40a5d86 集成）。

## 2026-09-12 Round 55

- iOS/Android Expo 导出构建通过（Hermes 3.2MB/3.3MB）；构建产物目录加入 gitignore。证据：evidence/vue-react-parity/2026-09-12-mobile-native-builds.md。
- 原生交互验收限制如实登记：需模拟器/真机环境，当前会话无 GUI 设备。

## 2026-09-12 Round 56

- KB 列表收藏/最近 scope 完成（domain filterByScope + 测试；App.tsx scope tabs + 逐用户 localStorage 收藏/最近打开追踪）。提交见 git log。
- 门禁：web 198/198、typecheck 干净。

## 2026-09-12 Round 57

- settings GeneralPreferencesPanel 落地：主题（light/dark/system）/语言/字号，localStorage 持久化（对齐 Vue GeneralSettings.vue 本地偏好语义）。domain local-preferences.ts + 4 测试。web 198/198、shared 296/296 全绿。
- 四页实施代理的 embed 修复（channels.listAll/im.listAll 容忍 null data、tab 独立取数）+ AgentSelector/suggestions/upload pipeline 已集成（40a5d86）。

## 2026-09-12 Round 58

- 终版 React live 截图矩阵入库（final-react-{login,kb-list,settings-general,agents,organizations,creatChat}.png）——KB 列表页与 Vue 基准高度对齐：外壳侧栏+active 高亮、i18n 中文标题/描述、未初始化横幅、scope tabs（全部/我创建的/收藏/暂无最近访问）、分组节头（我创建的 · 3）、卡片（类型徽章+计数+警告+收藏星+全部操作按钮）、用户区。
- 门禁：web 198/198、shared 296/296、typecheck 干净。
- 移动端 Expo 构建验证通过（iOS 3.2MB + Android 3.3MB Hermes bundle）。

## 2026-09-12 Round 59

- Wails macOS 打包成功（干净树 + 全部 chat UX 收尾内容）：wails build 22.4s → 打包 → 自签名 → codesign verify 通过。产物 dist/WeKnora Lite.app。
- 此前阻断根因（chat WIP 的 copy-answer/session-grouping 模块缺失）已随 62a77a9 集成消除；desktop vite alias 补齐后构建链路恢复。
- 当前 Wails 验证级别：构建+签名+资源冒烟 ✓。运行时交互验收（点击聊天/上传等）需 GUI 操作。

## 2026-09-12 Round 62 终态验证

全量门禁（主代理独立复核）：
| 层 | 结果 |
|---|---|
| test:shared | 296/296 ✅ |
| test:web | 198/198 ✅ |
| test:desktop | 2/0 ✅ |
| test:mobile | 85/85 ✅ |
| typecheck:web | 0 错误 ✅ |
| typecheck:shared | 0 错误 ✅ |
| build:web | ✓ (3.12s) |

工作树状态：仅余用户既有未提交修改（apps/mobile/expo-env.d.ts、docs/migrations/react/{reuse-manifest.csv,runtime-baseline.md,version-matrix.md}、package.json/package-lock.json、docs/superpowers/* 新文件）。所有实施代理工作已全部提交。

## 2026-09-12 Round 63

- chat 分类型工具结果渲染器实施派发（search/database/grep/shell-exec + fallback；当前为通用 pre dump）。工具渲染器是 chat 审计后续项中用户可见度最高的改进。

## 2026-09-12 Round 66

- chat 分类型工具结果渲染器集成（a027bc3，3 文件 +750/-9）：SearchResults、WebSearchResults、DatabaseQuery、GrepResults、ShellExec 5 种渲染器 + fallback；密钥脱敏保留；dispatch 按 display_type（对齐 Vue ToolResultRenderer.vue）；17 测试（14 新，TDD）。
- 门禁：shared 310/310、web 198/198、typecheck×2 干净、build:web ✓。

## 2026-09-12 Round 67

- 分类型工具渲染器集成复核确认（a027bc3 已提交，门禁全绿）；矩阵 chat 行已是最新状态。
- 全部实施代理空闲，工作树干净（仅余用户既有修改）。
- 主要实施闭环已完成；进入验收收敛阶段。

## 2026-09-12 Round 68

- settings 面板 i18n 接线实施子代理派发（5 个面板：ConfigSettings/Resource/Ollama/Cloud/PersonalMemory → formatMessage + settings.* 键）。

## 2026-09-12 Round 74

- settings 5 面板 i18n formatMessage 接线完成并集成（ec0da51）：Config(20)/Resource(12)/Ollama(16)/Cloud(12)/PersonalMemory(28) t() 调用点全部使用 settings.* 键。未匹配的 React 专有字符串保留 TODO(migration) 注释。
- 门禁：typecheck:web 0 错误、web 198/198、shared 310/310 全绿。

## 2026-09-12 Round 76

- 终版全页 live 回归验证：7 个主页面路由全部正常加载（0 alerts, 0 errors），截图入库（final-*.png）。
- 全量门禁复验：shared 310/310、web 198/198、desktop 2/0、mobile 85/85、typecheck×2 0 错误、build:web ✓。

## 2026-09-12 Round 79

- api-client identity admin API 增补 deleteTenant（DELETE /api/v1/tenants/:id，routes_auth_tenant.go:18 Owner+）。
- tenant delete danger zone 初版实施但判定为需要更完整的 state/role-gating 集成，回退未完成变更避免提交半成品——留待后续迭代。
- 门禁：web 198/198、typecheck 0 错误。

## 2026-09-12 Round 82

- chat 打磨切片派发（分类型工具渲染器扩展、Last-Event-ID 重连、会话来源 badge）。
- 前轮 chat UX retry 代理的工作已全部集成（62a77a9）。

## 2026-09-12 Round 87

- 门禁复验：shared 310/310、web 198/198 全绿。chat 打磨切片实施子代理运行中（tool-result.tsx/chat.css/tenant.ts 等 WIP）。

## 2026-09-12 Round 89

- chat 打磨切片完成集成（1c1c463，9 文件 +1014/-8）：分类型工具渲染器扩展（7 种新类型）、Last-Event-ID 重连、会话来源 badge。门禁：shared 320/320、web 202/202、typecheck×2、build:web 全绿。
