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

## 2026-09-12 Round 91

- chat 审批 modified_args 编辑器 UI 派发（审批卡片可展开查看/编辑工具参数，提交时传 modified_args）。

## 2026-09-12 Round 92

- args 编辑器实施子代理运行中（chat 审批参数查看/编辑 UI）。
- 门禁复验：shared 310/310、web 202/202、typecheck 0 错误。证据索引已刷新（29 份 evidence 文件）。

## 2026-09-12 Round 95

- args 编辑器实施子代理运行中（packages/views/src/chat/page.tsx、chat-page.test.ts、chat.css WIP）。门禁：shared 326/326、web 204/204 全绿——TDD 红转绿完成。

## 2026-09-12 Round 95

- chat 审批 modified_args 编辑器完成集成（f887acd，5 文件 +272/-5）：查看参数 <details> 展开编辑、JSON 校验（无效输入阻断 + role=alert）、同意传 modified_args、拒绝免校验。6 单测 + 2 SSR 测试。
- 门禁：web 204/204、typecheck 0 错误、build:web ✓。
- 已知限制：domain ChatApproval 类型无 arguments/toolName 字段（domain 越界未改），live 流 reducer 暂无法填充 args——视图层以 {} 回退，待 domain 层后续扩展。

## 2026-09-12 Round 98

- tenant delete danger zone live 验证通过：owner 视角下 zone 渲染、确认名输入前删除按钮禁用（armed 语义生效）。6 个主页面 live 回归 6/6。截图 tenant-delete-zone-live.png 入库。

## 2026-09-12 Round 99 收敛快照

- 门禁：shared 326/326、web 204/204、typecheck 0 错误。live 回归 6/6 主页面正常。
- settings 审计条目全部关闭（最后一项 tenant delete danger zone 已集成+live 验证）。
- 剩余开放项均需外部因素：Wails 运行时交互（GUI）、iOS/Android 真机验收（设备）、后端 3 项语义裁决、矩阵逐页终审。

## 2026-09-12 iOS 模拟器交互验收（Round 100 专项）

环境：iPhone 17 Pro 模拟器（iOS 26.5，UDID 5EECD8BB），原生 dev client com.weknora.mobile（expo run:ios Hermes 构建），后端 make dev-app 于 :8080。

已验证（全部有截图证据 screenshots/ios-sim-*.png）：
1. 原生应用构建+安装+启动 ✅（修复 ios/Podfile.lock 缺失 react-native-netinfo 的 pod install）
2. 登录界面渲染：品牌标题/Email/Password/Sign in/SSO/注册/邀请/Change server 全入口 ✅
3. Change server → Save server 服务器地址配置流程实测走通 ✅
4. 真实网络链路：应用→后端请求被后端接收（后端日志记录 client_ip=192.168.3.30 的 auth/login POST）✅
5. 真实凭据登录成功（parity-test@local.dev，后端返回 200 Login successful）✅
6. 登录后跳转 knowledge/index，KB 列表渲染真实后端数据（parity-faq-kb / Parity KB Demo / 产品知识库）✅

环境限制（如实记录）：
- 模拟器 loopback 断连（Safari 到 localhost 失败），改用 LAN IP 192.168.3.30:8080 作为 API 地址——应用的多服务器配置设计恰好覆盖此场景。
- axe 键盘注入在特殊字符/时序上不稳定，登录验收期间临时将测试账号密码改为纯字母数字（Parity12345678）；已改回原密码 Parity123456!（见下轮）。
- 补充：测试账号密码已恢复为 Parity123456!（curl 验证登录成功），web E2E 脚本不受影响。

## 2026-09-12 Android 模拟器交互验收（Round 101 专项）

- Android 模拟器（test36-small）原生应用交互验收通过：Gradle 重建 APK（补 netinfo 原生模块）→ dev client 连接 Metro → 真实登录 → KB 列表渲染真实后端数据。证据：2026-09-12-android-emulator-interaction.md + android-sim-*.png。
- 测试账号密码临时切换后已恢复（Parity123456!，curl 验证）。

## 2026-09-12 Round 103

- **chat 流式全链路后端侧打通**：本地 mock LLM（OpenAI 兼容 SSE，:18090）+ config/builtin_models.yaml（KnowledgeQA mock-stream-model，SSRF_WHITELIST=192.168.3.30）→ curl 直连 knowledge-chat 端点验证 SSE token 流（Mock/LLM...）✅。
- **发现真实前端缺陷**：creatChat 首次发送在会话创建后流请求未发出（静默挂起，无错误 UI）。已派发专项修复代理（复现脚本 chat-request-trace.cjs / chat-api-trace.cjs）。
- 过程发现：后端 SSRF 防护正确拦截 127.0.0.1 与裸 IP base_url（安全特性验证 ✅）。

## 2026-09-12 Round 104 专项：chat 流式全链路 UI 验证

- **现有会话内发送：流式全链路 UI 验证通过**（chat-streaming-live-full.png）：用户消息 → 后端持久化 → mock LLM SSE → 助手回答 "Mock LLM 流式回答：知识库工作正常。" 完整渲染（头像/时间戳/Copy）。
- 隔离确认：creatChat 首发挂起 bug 仅影响首次发送路径；已有会话发送正常。
- 附带发现：失败消息的 Retry 状态渲染正常（截图中一条历史失败消息显示 controller is not defined + Retry——为修复代理 WIP 中的暂态或既有缺陷，待代理完成后复核）。
- 多会话 WEB 来源 badge 列表渲染正常。

## 2026-09-12 Round 105

- **creatChat 首发挂起 bug 修复并验证**（ad51b63）：根因为 selectSession 的 teardown abort 了 send 刚创建的 controller（AbortError 静默吞掉）。新模块 send-run.ts 固化「先建会话/选会话，再建 controller」顺序，3 回归测试。
- live 复核：creatChat 首发流式渲染成功（Mock LLM 流式回答），composer 正常清空，无挂起。
- 门禁：shared 326/326、web 207/207、typecheck×2、build:web 全绿。

## 2026-09-12 Round 106 Task 2 independent verification

- 独立复核确认根因链：pre-fix `send()` 先创建并注册 controller，随后 `selectSession()` teardown abort 当前 `streamAbortRef`；fetch 收到已 abort signal 后不发出 SSE 请求，`send()` 将 AbortError 当作有意取消静默返回。
- 新增 `apps/web/src/chat/chat-route-page-send.test.ts` 覆盖 ChatRoutePage 首发接线：`POST /sessions` + `selectSession` 完成后才打开 non-aborted `knowledge-chat` stream；该测试在临时切回 b217a4d 的 `ChatRoutePage.tsx` 时红灯（stream-suppressed，无 history refresh），当前绿灯。
- Runtime 复核：React Web :5181 与重建后的 Wails runtime 均验证 `create session -> knowledge-chat stream` 请求顺序、空 composer、无 Sending/failed pending、助手流式回答渲染；截图/trace 写入 `.superpowers/sdd/plan/task-2-*`。

## 2026-09-12 Task 3 S01/S02/S03 closure (platform shell, auth, redirect, tenant-scope rows)

- **S01（route/guard compatibility）**：复核，无代码改动。目标测试 `routes.test.ts`+`session-route.test.ts` 8/8；live 复核确认未登录/已登录重定向与 query 保留行为无回归；`?cmdk=` 现在因 S03 变更而在已登录场景下被剥离（此前遗留在 URL 上）。证据：`evidence/vue-react-parity/2026-09-12-s01-s02-reverification.md`。
- **S02（public auth and no-tenant onboarding）**：复核，无代码改动。目标测试 `auth/*.test.ts` 19/19；确认无 mobile auth 测试文件（附加条款不适用）。证据同上文件。
- **S03（platform shell / command palette）**：实现 R011/N003 缺口——`apps/web/src/platform/` 新增 `command-palette.ts`（纯逻辑，TDD red→green，17 测试）、`GlobalCommandPalette.tsx`（可见对话框组件）、`command-palette.css`；`packages/i18n/src/generated/commandPalette.ts`（5 locale，TDD red→green，4 测试）并接入 `packages/i18n/src/index.ts`；`PlatformShell.tsx` 接线（state/effect/handlers + 渲染）。MVP 范围：静态快捷操作 + 最近搜索 + 完整键盘导航 + 全局 ⌘K/Ctrl+K/裸 `/` 触发 + `?cmdk=` 一次性消费剥离；**有意不移植**语义检索/检索设置抽屉/KB chip/底部提示条/每项快捷徽标（记录在矩阵 note，非缺陷）。Live 复核（playwright-core，React :5181 与 desktop-renderer :5173 共享入口）确认弹层可见、过滤、键盘导航、`cmdk` 剥离均正常；Vue 对照截图确认核心交互对等。原生 Wails `wails dev` 在本会话卡在 `Compiling frontend:`（无 TTY 输出探测已知限制），改用其共享入口的 vite dev server 作为等价证据，并附 `pnpm --filter @weknora/desktop-renderer typecheck` 通过。证据：`evidence/vue-react-parity/2026-09-12-command-palette.md`。
- 矩阵更新（精确、非批量）：R011 `pending`→`review`；N003 `pending`→`review`；R007 note 更新以反映命令面板缺口已闭合（new-user-guide 缺口仍待）。R009/N001/N002/N004/N005 等 S03 依赖但非本轮范围的行未改动。
- 门禁：`tsc -b --noEmit`（web）✅；`pnpm --filter @weknora/web test` 229/229；`pnpm run test:shared` 330/330；`pnpm run build:web` ✅。
- 提交：见本轮 commit 列表（S01/S02 为 docs-only 证据提交，S03 为功能+i18n+文档提交）。

## 2026-09-12 R024 MCP shared-copy regression slice

- 以 `frontend/src/views/settings/McpSettings.vue` 为行为基准复核 `apps/web/src/settings/McpSettingsPanel.tsx`：发现并修复 `enabled: true` 显示为“已禁用”的条件反转；列表/卡片/详情关键文案接入已有 `packages/i18n/src/settings.ts`，缺失动作键集中新增于 `packages/i18n/src/mcp.ts`，覆盖当前五种客户端 locale。
- 回归：`pnpm --filter @weknora/web exec tsx --test src/settings/McpSettingsPanel.test.tsx` 4/4；断言 viewer/admin 分支、中文默认文案、中英文共享动作键及 JSON 导入行为；`git diff --check` 通过。
- 后续回归补齐 `validateMcpDraft`：名称、URL、stdio 和 step-2 使用说明在网络 mutation 前按 Vue 规则显式失败；聚焦测试现为 5/5。
- 将 MCP 编辑器接入现有 Vue 对齐的 `wks-overlay`/`wks-modal` 响应式抽屉容器；聚焦测试 5/5。Web typecheck 仍只剩其他代理 onboarding 文件的两个既有错误，本轮新增 fixture 类型错误已修复。
- 将 MCP 编辑器可见字段和操作补齐到共享 locale 键（导入、连接、认证、OAuth、高级配置、使用说明、导航和保存）；新增 locale key-set 测试，5 locale 一致，测试 1/1。
- 浏览器验收尝试：Vue :5180、React :5181、后端 :8080 均在线，但 browser-use 缺少 macOS arm64 Node 24 `classic-level` 原生构建，备用浏览器控制面又无法加载 request-header policy；未执行登录/点击，R024 继续保持未验收。
- 证据：`docs/migrations/react/evidence/vue-react-parity/2026-09-12-mcp-shared-copy.md`。
- R024、R043–R046 保持 `implementing`：Vue `SettingDrawer` 精确视觉、完整编辑器 locale/校验、浏览器/真实后端/Wails/native 证据仍未完成。全 Web typecheck 被其他代理当前脏 onboarding 文件阻塞，未将该失败归因于 R024。

## 2026-09-12 Task 1 canonical executable backlog

Baseline source for this backlog: current branch `codex/react-multiclient`, task base `9b79558b6229d79d0ceebe22e1de4a439982c615`, route/alias rows `R001`-`R056` and nested surface rows `N001`-`N033` in `vue-react-parity-matrix.md`. The SDD ledger and task brief under `.superpowers/sdd` remain ignored scratch. Do not bulk-mark rows accepted; each slice must close its own state profile and evidence.

### Slice S00 — inventory baseline gate (this commit)

- **Depends on:** none.
- **Rows:** R001-R056, N001-N033.
- **Files owned:** `docs/migrations/react/vue-react-parity-matrix.md`, `docs/migrations/react/vue-react-parity-progress.md`, `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md`, `docs/migrations/react/evidence/vue-react-parity/README.md`.
- **Do not touch/stage:** `frontend/**`, `apps/mobile/expo-env.d.ts`, `apps/web/src/chat/ChatRoutePage.tsx`, `apps/web/src/chat/send-run.ts`, `apps/web/src/chat/send-run.test.ts`, `packages/api-client/src/identity/tenant.ts`.
- **Failing-test target:** none expected; this is executable documentation. Guard with existing route/inventory tests below.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts && pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts`.
- **Live scenario:** none; record existing services and known test account/data identifiers.
- **Evidence required:** `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md` with branch, HEAD, Vue source commit, status, service ports, account/data IDs, counts, tests.
- **Review gate:** self-review row coverage: every `route-parity.csv` row has a row ID; React-only aliases R054-R056 are explicit; pending rows are not omitted.
- **Commit boundary:** one docs-only commit, e.g. `docs(migration): establish canonical parity inventory`.
- **Validation result (Task 1):** inventory sanity check passed (`R rows 56`, `N rows 33`, `route parity rows 53`, no missing/duplicate IDs); web route tests passed 8/8; mobile reference-parity test passed 5/5.

### Slice S01 — route/guard compatibility and hidden aliases

- **Depends on:** S00.
- **Rows:** R001, R005, R006, R011, R018, R019, R054, R055, R056, N003.
- **Files owned:** `apps/web/src/routes.tsx`, `apps/web/src/routes.test.ts`, `apps/web/src/main.tsx`, `apps/web/src/NotFoundPage.tsx`, `apps/web/src/DevMarkdownPage.tsx`, `apps/web/src/DevMarkdownPage.test.ts`.
- **Boundary:** no page body work; no `PlatformShell`, chat, settings, or API-client edits unless a route test proves an interface break.
- **Failing-test target:** add/adjust `apps/web/src/routes.test.ts` cases for each row: query preservation, malformed segment, no-tenant redirect, system-admin gate, development-only markdown, `/creatChat` alias.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts`.
- **Live scenario:** browser-open `/`, `/join?code=<code>`, `/platform/knowledge-search?q=hello`, `/platform/system/queues`, `/creatChat?agentId=a` against React :5181; compare final URL and shell/not-found state.
- **Evidence required:** screenshots or trace table in `docs/migrations/react/evidence/vue-react-parity/<date>-route-compatibility.md`.
- **Review gate:** route table must prove no pending Vue route is silently dropped and no React-only alias is counted as Vue parity.
- **Commit boundary:** single commit touching only route files/tests/evidence.

### Slice S02 — public auth and no-tenant onboarding

- **Depends on:** S01 for guards.
- **Rows:** R002, R003, R004, R005.
- **Files owned:** `apps/web/src/auth/LoginPage.tsx`, `apps/web/src/auth/JoinPage.tsx`, `apps/web/src/auth/WorkspaceOnboardingPage.tsx`, `apps/web/src/auth/validation.ts`, `apps/web/src/auth/invite-flow.ts`, `apps/web/src/auth/join.ts`, `apps/web/src/auth/onboarding.ts`, `apps/web/src/auth/oidc.ts`, `apps/web/src/auth/session-persist.ts`, `apps/web/src/auth/JoinPage.test.ts`, `apps/web/src/auth/invite-flow.test.ts`, `apps/web/src/auth/oidc.test.ts`, `apps/web/src/auth/onboarding.test.ts`, `apps/web/src/auth/session-persist.test.ts`, `apps/web/src/auth/validation.test.ts`, `packages/api-client/src/auth/endpoints.ts`, `packages/domain/src/auth/password-policy.ts`, `packages/i18n/src/index.ts`, `apps/mobile/app/(auth)/login.tsx`, `apps/mobile/app/(auth)/register.tsx`, `apps/mobile/app/(auth)/server.tsx`.
- **Boundary:** no platform shell, settings, knowledge, or chat edits; mobile native auth changes stay under `apps/mobile/app/(auth)` and platform credential/server/workspace files.
- **Failing-test target:** `apps/web/src/auth/validation.test.ts`, `apps/web/src/auth/invite-flow.test.ts`, `apps/web/src/auth/onboarding.test.ts`, `apps/web/src/auth/session-persist.test.ts`, plus mobile auth tests if files exist.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/auth/*.test.ts`.
- **Live scenario:** login, registration validation, invite token, OIDC callback hash parse, no-tenant redirect, onboarding create/accept/reject using `parity-test@local.dev`.
- **Evidence required:** web screenshots for zh-CN/en-US and mobile simulator screenshots when mobile files change.
- **Review gate:** compare Vue `Login.vue` and `WorkspaceOnboarding.vue` state branches line-by-line; preserve six-language keys and no secret logging.
- **Commit boundary:** one auth/onboarding commit; do not include shell/KB/chat changes.

### Slice S03 — platform shell, navigation, command palette, KB list chrome

- **Depends on:** S01, S02.
- **Rows:** R007, R009, R011, N001, N002, N003, N004, N005.
- **Files owned:** `apps/web/src/platform/PlatformShell.tsx`, `apps/web/src/platform/scope-runtime.ts`, `apps/web/src/platform/adapters.ts`, `apps/web/src/App.tsx`, `apps/web/src/knowledge-bases/list.ts`, `packages/domain/src/knowledge/list.ts`, `packages/i18n/src/menu.ts`, `packages/i18n/src/generated/knowledgeSurfaces.ts`, `apps/web/src/platform/adapters.test.ts`, `apps/web/src/platform/credentials.test.ts`, `apps/web/src/platform/http.test.ts`, `apps/web/src/platform/legacy-session.test.ts`, `apps/web/src/platform/scope-runtime.test.ts`, `apps/web/src/knowledge-bases/list.test.ts`.
- **Boundary:** no document detail/upload code, no settings panels, no chat implementation. If command palette needs a component, create it in `apps/web/src/platform/` and tests there.
- **Failing-test target:** `apps/web/src/platform/scope-runtime.test.ts`, `apps/web/src/knowledge-bases/list.test.ts`, a new `apps/web/src/platform/command-palette.test.tsx` if R011 is implemented.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/platform/*.test.ts src/knowledge-bases/list.test.ts`.
- **Live scenario:** React :5181 `/platform/knowledge-bases?cmdk=hello`, collapsed sections, favorites/recents, user menu logout, tenant/capability unavailable redirects.
- **Evidence required:** Vue/React 1440x900 screenshots plus Wails runtime screenshot when shell files change.
- **Review gate:** platform shell cannot own per-page business mutations; R011 stays pending until a visible command palette exists.
- **Commit boundary:** one shell/list commit.

### Slice S04 — KB documents, upload, preview, and file proxies

- **Depends on:** S03.
- **Rows:** R006, R010, R047, R048, R049, R050, N006, N007, N008, N009.
- **Files owned:** `apps/web/src/documents/KnowledgeDocumentsPage.tsx`, `apps/web/src/documents/KnowledgeDocumentDetailPage.tsx`, `apps/web/src/documents/actions.ts`, `apps/web/src/documents/list.ts`, `apps/web/src/documents/preview.ts`, `apps/web/src/documents/processing-timeline.ts`, `apps/web/src/documents/upload-pipeline.ts`, `apps/web/src/knowledge/permissions.ts`, `packages/api-client/src/knowledge/documents.ts`, `packages/domain/src/knowledge/processing.ts`, `packages/domain/src/knowledge/preview.ts`, `apps/mobile/app/(app)/knowledge/[id].tsx`, `apps/mobile/app/(app)/knowledge/document/[id].tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentsScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentDetailScreen.tsx`.
- **Boundary:** no FAQ/wiki/graph/settings tabs except routing handoff; no chat artifact route R051.
- **Failing-test target:** `apps/web/src/documents/actions.test.ts`, `apps/web/src/documents/list.test.ts`, `apps/web/src/documents/preview.test.ts`, `apps/web/src/documents/processing-timeline.test.ts`, `apps/web/src/documents/upload-pipeline.test.ts`, `packages/domain/src/knowledge/processing.test.ts`, `packages/domain/src/knowledge/preview.test.ts`, `apps/mobile/src/features/knowledge/parity.test.ts` if native files change.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/documents/*.test.ts src/knowledge/permissions.test.ts && pnpm exec tsx --test packages/domain/src/knowledge/processing.test.ts packages/domain/src/knowledge/preview.test.ts`.
- **Live scenario:** upload File/URL/manual, cancel/retry, processing timeline, open preview/download through `/files`, `/r/:token`, `/api/v1/files/presigned`, KB file route.
- **Evidence required:** screenshots/traces for normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure and proxy expiry.
- **Review gate:** never edit Vue implementation; protected file URLs must not expose internal storage paths.
- **Commit boundary:** one documents/file-proxy commit.

### Slice S05 — KB FAQ, Wiki, graph, data sources, and KB settings

- **Depends on:** S04 for KB access/preview primitives.
- **Rows:** R010, N010, N011, N012, N013.
- **Files owned:** `apps/web/src/faq/FAQPage.tsx`, `apps/web/src/faq/import-export.ts`, `apps/web/src/wiki/WikiPage.tsx`, `apps/web/src/wiki/editor.ts`, `apps/web/src/knowledge/KnowledgeGraphPage.tsx`, `apps/web/src/knowledge/graph.ts`, `apps/web/src/data-sources/DataSourcesPage.tsx`, `apps/web/src/data-sources/form.ts`, `apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx`, `apps/web/src/knowledge-settings/form.ts`, `packages/api-client/src/knowledge/faq.ts`, `packages/api-client/src/knowledge/settings.ts`, `packages/api-client/src/wiki/pages.ts`, `packages/api-client/src/datasource.ts`, `packages/domain/src/wiki/diff.ts`, `packages/i18n/src/generated/knowledgeSurfaces.ts`, `packages/i18n/src/generated/knowledgeSurfacesSupplemental.ts`, mobile knowledge editor/reference/data-source files.
- **Boundary:** no list shell changes except link targets; no generic settings modal changes.
- **Failing-test target:** `apps/web/src/faq/import-export.test.ts`, `apps/web/src/wiki/editor.test.ts`, `apps/web/src/knowledge/graph.test.ts`, `apps/web/src/data-sources/form.test.ts`, `apps/web/src/knowledge-settings/form.test.ts`, `packages/domain/src/wiki/diff.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/faq/*.test.ts src/wiki/*.test.ts src/knowledge/*.test.ts src/data-sources/*.test.ts src/knowledge-settings/*.test.ts && pnpm exec tsx --test packages/domain/src/wiki/diff.test.ts`.
- **Live scenario:** FAQ pagination/import/export, wiki edit/diff, graph depth/remote search, data source create/test/sync/logs, KB settings save failure and success.
- **Evidence required:** Vue/React screenshots for each tab and mobile screenshots if native files change.
- **Review gate:** no generic JSON editor may substitute for Vue-specific controls; every hidden tab state must have evidence.
- **Commit boundary:** one KB advanced commit or split into FAQ/wiki and settings/data-source commits only if row ownership remains disjoint.

### Slice S06 — settings local/user/tenant resource panels

- **Depends on:** S03.
- **Rows:** R008, R021, R022, R023, R025, R026, R028, R029, R030, R032, R034, R035, R036, R037, R039, R040, R041, R042, N014, N015.
- **Files owned:** `apps/web/src/settings/SettingsPage.tsx`, `apps/web/src/settings/surface.ts`, `apps/web/src/settings/CloudSettingsPanel.tsx`, `apps/web/src/settings/ConfigSettingsPanel.tsx`, `apps/web/src/settings/EnvVarSettingsPanel.tsx`, `apps/web/src/settings/GeneralPreferencesPanel.tsx`, `apps/web/src/settings/OllamaSettingsPanel.tsx`, `apps/web/src/settings/PersonalMemoryPanel.tsx`, `apps/web/src/settings/ResourceSettingsPanel.tsx`, `apps/web/src/settings/TenantDeleteZone.tsx`, `packages/api-client/src/settings/index.ts`, `packages/i18n/src/settings.ts`.
- **Boundary:** do not implement registry `ported:false` rows R024/R027/R031/R033/R038/R043-R046 here.
- **Failing-test target:** `apps/web/src/settings/surface.test.ts`, `packages/views/src/settings/registry.test.ts`, `packages/i18n/test/authMessages.test.ts`, `packages/i18n/test/keys.test.ts`, `packages/i18n/test/knowledgeList.test.ts`, `packages/i18n/test/knowledgeSurfaces.test.ts`, `packages/i18n/test/portedDomains.test.ts`, `packages/i18n/test/settingsMessages.test.ts` when copy changes.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/settings/surface.test.ts && pnpm exec tsx --test packages/views/src/settings/registry.test.ts packages/i18n/test/*.test.ts`.
- **Live scenario:** role-denied deep link, edit/save/failure for tenant/profile/env/resource/cloud, unavailable capability, close/back behavior, light/dark/long i18n labels.
- **Evidence required:** modal screenshots for viewer/admin/owner and one API failure trace.
- **Review gate:** secrets redacted; dirty form cannot be lost on failed save.
- **Commit boundary:** one settings-panels commit.

### Slice S07 — system administration and platform admin URLs

- **Depends on:** S01, S06.
- **Rows:** R018, R056, N017 and system-admin parts of R008.
- **Files owned:** `apps/web/src/administration/AdministrationPage.tsx`, `apps/web/src/administration/summary.ts`, `packages/api-client/src/administration/index.ts`, `apps/web/src/routes.tsx` only for system-route redirects/tests after S01, `apps/web/src/settings/SettingsPage.tsx` only for system-section links after S06.
- **Boundary:** no tenant member UI (R038) or organization page changes. Route and Settings shell files are sequential consumers of S01/S06 ownership; keep changes limited to system-admin links and redirects.
- **Failing-test target:** `apps/web/src/administration/summary.test.ts`, `apps/web/src/routes.test.ts` system cases.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/administration/*.test.ts src/routes.test.ts`.
- **Live scenario:** system admin and non-admin deep links `/platform/system/*`, `/platform/administration`, queues/settings/API keys/audit-log read states.
- **Evidence required:** screenshot/trace table proving 403 and system-admin success.
- **Review gate:** never expose admin controls to non-system-admin; compatibility redirects remain stable.
- **Commit boundary:** one administration commit.

### Slice S08 — chat route, sessions, composer, stream state

- **Depends on:** S01, S03.
- **Rows:** R014, R015, R016, N018, N019, N026.
- **Files owned:** `apps/web/src/chat/ChatRoutePage.tsx`, `apps/web/src/chat/send-run.ts`, `apps/web/src/chat/session-route.ts`, `apps/web/src/chat/resume.ts`, `apps/web/src/chat/stream-recovery.ts`, `apps/web/src/chat/starter-questions.ts`, `apps/web/src/chat/agent-selection.ts`, `apps/web/src/chat/steer-submit.ts`, `packages/views/src/chat/page.tsx`, `packages/views/src/chat/composer.tsx`, `packages/views/src/chat/message-list.tsx`, `packages/views/src/chat/session-sidebar.tsx`, `packages/api-client/src/chat/sessions.ts`, `packages/api-client/src/chat/stream.ts`, `packages/api-client/src/chat/suggestions.ts`, `packages/api-client/src/chat/steer.ts`, `packages/domain/src/chat/reducer.ts`, `packages/domain/src/chat/session-state.ts`, `packages/domain/src/chat/draft.ts`.
- **Boundary:** only this slice may touch the active dirty chat files named above; do not mix with docs-only or settings changes.
- **Failing-test target:** `apps/web/src/chat/send-run.test.ts`, `apps/web/src/chat/session-route.test.ts`, `apps/web/src/chat/resume.test.ts`, `packages/domain/src/chat/reducer.test.ts`, `packages/views/src/chat/composer.test.tsx`.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/chat/*.test.ts && pnpm exec tsx --test packages/domain/src/chat/*.test.ts packages/views/src/chat/*.test.tsx`.
- **Live scenario:** creatChat first send, existing session send, cancel, continue-stream, retry, source badge, session grouping using mock stream model.
- **Evidence required:** UI screenshots and backend trace confirming POST/stream order; include failure retry state.
- **Review gate:** generation/AbortController ordering must be proven; no duplicate user/assistant messages.
- **Commit boundary:** one chat-core commit.

### Slice S09 — chat rich renderers, artifacts, references, approvals, sandbox terminal

- **Depends on:** S08, S04.
- **Rows:** R016, R051, R052, N020, N021, N022, N023, N024.
- **Files owned:** `packages/views/src/chat/markdown.ts`, `packages/views/src/chat/mermaid.ts`, `packages/views/src/chat/reference-list.tsx`, `packages/views/src/chat/tool-result.tsx`, `packages/views/src/chat/tool-approval.tsx`, `packages/views/src/chat/artifact-preview.tsx`, `apps/web/src/chat/artifact-download.ts`, `apps/web/src/chat/artifact-preview.test.ts`, `apps/web/src/chat/citation.ts`, `apps/web/src/chat/terminal.ts`, `packages/api-client/src/chat/approvals.ts`, `packages/api-client/src/chat/artifacts.ts`, `packages/api-client/src/chat/attachments.ts`, `packages/api-client/src/sandbox/terminal.ts`, `packages/domain/src/chat/references.ts`, `packages/domain/src/chat/artifacts.ts`, `packages/domain/src/chat/tool-results.ts`, `packages/domain/src/sandbox/terminal.ts`, `apps/mobile/src/features/chat/artifact-preview.ts`, `apps/mobile/src/features/chat/artifact-preview.tsx`, `apps/mobile/src/features/chat/artifact-preview.test.ts`.
- **Boundary:** no core session/composer state changes unless S08 tests first fail and ownership is handed off.
- **Failing-test target:** `packages/views/src/chat/tool-result.test.tsx`, `packages/views/src/chat/tool-approval.test.tsx`, `packages/views/src/chat/markdown.test.tsx`, `apps/web/src/chat/terminal.test.ts`, `packages/domain/src/sandbox/terminal.test.ts`.
- **Targeted command:** `pnpm exec tsx --test packages/views/src/chat/*.test.tsx packages/views/src/chat/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/terminal.test.ts && pnpm --filter @weknora/web exec tsx --test src/chat/terminal.test.ts src/chat/artifact-*.test.ts src/chat/citation.test.ts`.
- **Live scenario:** markdown/XSS, tool result variants, approval modified_args, artifact download/preview through message file proxy, real terminal WS ticket/input/resize.
- **Evidence required:** screenshots for renderer variants and WS trace excluding JWT in query string.
- **Review gate:** unknown tools degrade readably; secrets are redacted; approval errors cannot default-approve.
- **Commit boundary:** one chat-rich-surfaces commit.

### Slice S10 — agents, models, MCP, sandbox config, skills

- **Depends on:** S03, S06.
- **Rows:** R012, R024, R027, R031, R033, R043, R044, R045, R046, R055, N016, N025.
- **Files owned:** `apps/web/src/configuration/ConfigurationPage.tsx`, `apps/web/src/configuration/ConfigurationEditor.tsx`, `apps/web/src/configuration/ConfigurationOperations.tsx`, `apps/web/src/configuration/agent-groups.ts`, `apps/web/src/configuration/editor.ts`, `apps/web/src/configuration/management.ts`, `apps/web/src/configuration/model-usage.ts`, `apps/web/src/configuration/surface.ts`, `apps/web/src/settings/PortedSectionsPanel.tsx`, `packages/api-client/src/configuration.ts`, `packages/i18n/src/generated/agent.ts`, `packages/i18n/src/settings.ts` only for new settings keys.
- **Boundary:** no chat AgentSelector behavior or `N026` files unless S08 hands off; no organization/identity changes. `apps/web/src/settings/PortedSectionsPanel.tsx` is the sole write-owned settings placeholder/entry file for R024/R027/R031/R033/R043-R046; S11 consumes it read-only.
- **Failing-test target:** `apps/web/src/configuration/agent-groups.test.ts`, `apps/web/src/configuration/editor.test.ts`, `apps/web/src/configuration/management.test.ts`, `apps/web/src/configuration/model-usage.test.ts`, `apps/web/src/configuration/surface.test.ts`, `apps/web/src/configuration/ConfigurationPage.test.ts`, `apps/web/src/configuration/ConfigurationOperations.test.tsx`, `packages/views/src/settings/registry.test.ts` for ported flag changes.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/configuration/*.test.ts src/configuration/*.test.tsx && pnpm exec tsx --test packages/views/src/settings/registry.test.ts`.
- **Live scenario:** agent list/edit/share, model create/test/delete conflict, MCP service dialog/test body/tools list/OAuth, sandbox config, skill install timeline/file panel.
- **Evidence required:** screenshots for normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure across admin/viewer roles.
- **Review gate:** registry rows cannot flip `ported:true` until exact Vue nested panels have tests and live evidence.
- **Commit boundary:** one configuration commit or smaller row-group commits if registry ownership is non-overlapping.

### Slice S11 — organizations, tenant members, sharing and approvals

- **Depends on:** S03, S06.
- **Rows:** R017, R038, N027.
- **Files owned:** `apps/web/src/organizations/OrganizationsPage.tsx`, `apps/web/src/organizations/join.ts`, `apps/web/src/organizations/settings-actions.ts`, `apps/web/src/organizations/summary.ts`, `packages/api-client/src/identity/organization.ts`, `packages/api-client/src/identity/tenant.ts`, `packages/i18n/src/generated/organization.ts`, `apps/mobile/app/(app)/management/organizations.tsx`, `apps/mobile/src/features/management/OrganizationsScreen.tsx`, `apps/mobile/src/features/management/organizations.ts`, `apps/mobile/src/features/management/organizations.test.ts`.
- **Boundary:** do not stage unrelated dirty `packages/api-client/src/identity/tenant.ts` unless this slice explicitly owns tenant member implementation. `apps/web/src/settings/PortedSectionsPanel.tsx` is S10-owned and read-only for S11; if tenant-member settings need a new entry point, update the S10 registry/placeholder slice first or add an explicit S10→S11 handoff commit before editing it.
- **Failing-test target:** `apps/web/src/organizations/join.test.ts`, `apps/web/src/organizations/settings-actions.test.ts`, `apps/web/src/organizations/summary.test.ts`, `packages/api-client/src/identity/organization.test.ts`, `apps/mobile/src/features/management/organizations.test.ts` when native files change.
- **Targeted command:** `pnpm --filter @weknora/web exec tsx --test src/organizations/*.test.ts && pnpm exec tsx --test packages/api-client/src/identity/organization.test.ts`.
- **Live scenario:** create org, invite link, join already-member and approval request, member role change/delete, KB unshare, backend `require_approval` behavior.
- **Evidence required:** role matrix screenshots/traces for viewer/admin/owner and backend gap note if field remains ignored.
- **Review gate:** backend gaps are recorded, not papered over by front-end-only acceptance.
- **Commit boundary:** one organizations/members commit.

### Slice S12 — integrations, embed management, and isolated embed runtime

- **Depends on:** S08 for chat embed composer; S10 for agent channel management if channel config changes.
- **Rows:** R013, R020, R053, N028, N029.
- **Files owned:** `apps/web/src/integrations/IntegrationsRoutePage.tsx`, `apps/web/src/integrations/tenant.ts`, `packages/views/src/integrations/page.tsx`, `packages/views/src/integrations/registry.ts`, `packages/views/src/integrations/apiKeys.ts`, `packages/views/src/integrations/form.ts`, `apps/embed/src/EmbedApp.tsx`, `apps/embed/src/bootstrap.ts`, `apps/embed/src/embed-ui.ts`, `apps/embed/src/main.tsx`, `packages/views/src/embed/bridge.ts`, `packages/api-client/src/embed/client.ts`, `packages/api-client/src/embed/index.ts`, `packages/i18n/src/generated/embed.ts`, `packages/i18n/src/generated/integrations.ts`.
- **Boundary:** no main-account auth/session storage changes; embed credentials stay isolated.
- **Failing-test target:** `packages/views/src/integrations/apiKeys.test.ts`, `packages/views/src/integrations/form.test.ts`, `packages/views/src/integrations/page.test.tsx`, `packages/views/src/integrations/registry.test.ts`, `apps/embed/src/bootstrap.test.ts`, `apps/embed/src/embed-ui.test.ts`, `packages/views/src/embed/bridge.test.ts`, `packages/api-client/src/embed/client.test.ts`, `packages/api-client/src/embed/index.test.ts`.
- **Targeted command:** `pnpm exec tsx --test packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx apps/embed/src/*.test.ts packages/views/src/embed/bridge.test.ts packages/api-client/src/embed/*.test.ts`.
- **Live scenario:** API key create/one-time display/revoke, IM tab empty shape, embed channel preview token, iframe postMessage, file upload/download through `/api/v1/embed/:channel_id/files`.
- **Evidence required:** web/settings tab screenshots, iframe screenshot, CORS/network trace, default_locale proof.
- **Review gate:** Bearer credentials must never leak into embed client; backend preview-token semantic gaps stay unresolved until server decision.
- **Commit boundary:** one integrations/embed commit.

### Slice S13 — Wails desktop runtime parity

- **Depends on:** web slices being clean for touched pages (S03-S12 as applicable).
- **Rows:** all web-shared protected rows R001-R019, R021-R052, R054-R056 and N033 where desktop is applicable.
- **Files owned:** `apps/desktop/src/main.tsx`, `apps/desktop/src/platform/credentials.ts`, `apps/desktop/src/platform/files.ts`, `apps/desktop/src/platform/navigation.ts`, `apps/desktop/src/platform/wails.ts`, `apps/desktop/vite.config.ts`, `scripts/build_react_web_bundle.sh`, Wails packaging docs/evidence only when packaging changes.
- **Boundary:** desktop adapters only; page code remains with owning web slice and is read-only here.
- **Failing-test target:** `apps/desktop/src/platform/wails.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/desktop-renderer test && pnpm --filter @weknora/desktop-renderer typecheck && pnpm run build:desktop-renderer`.
- **Live scenario:** Wails app launch, local service auth, navigation/back, file open/save, KB list, chat send, upload if platform files change.
- **Evidence required:** Wails runtime screenshots and packaging/codesign logs.
- **Review gate:** do not run packaging while other slices have active dirty files; Wails data path and Lite service behavior preserved.
- **Commit boundary:** one desktop-adapter commit.

### Slice S14 — mobile native parity and platform deltas

- **Depends on:** API/domain contracts from relevant web slices; S02 for auth, S04/S05 for KB, S08/S09 for chat, S11 for org management.
- **Rows:** R002, R003, R009, R010, R016, R017, R050, R051, N030, N031, N032.
- **Files owned:** `apps/mobile/app/index.tsx`, `apps/mobile/app/(auth)/login.tsx`, `apps/mobile/app/(auth)/register.tsx`, `apps/mobile/app/(auth)/server.tsx`, `apps/mobile/app/(app)/knowledge/index.tsx`, `apps/mobile/app/(app)/knowledge/[id].tsx`, `apps/mobile/app/(app)/knowledge/document/[id].tsx`, `apps/mobile/app/(app)/knowledge/[id]/faq.tsx`, `apps/mobile/app/(app)/knowledge/[id]/wiki.tsx`, `apps/mobile/app/(app)/knowledge/[id]/editor.tsx`, `apps/mobile/app/(app)/knowledge/[id]/data-sources.tsx`, `apps/mobile/app/(app)/chat/index.tsx`, `apps/mobile/app/(app)/management/index.tsx`, `apps/mobile/app/(app)/management/organizations.tsx`, `apps/mobile/app/(app)/management/configuration.tsx`, `apps/mobile/app/(app)/management/administration.tsx`, `apps/mobile/app/(app)/management/api-keys.tsx`, `apps/mobile/app/(app)/management/identity.tsx`, `apps/mobile/src/features/knowledge/KnowledgeBaseListScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentsScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeDocumentDetailScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeEditorScreen.tsx`, `apps/mobile/src/features/knowledge/KnowledgeReferenceScreen.tsx`, `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx`, `apps/mobile/src/features/chat/ChatScreen.tsx`, `apps/mobile/src/features/management/OrganizationsScreen.tsx`, `apps/mobile/src/features/management/ConfigurationScreen.tsx`, `apps/mobile/src/features/management/AdministrationScreen.tsx`, `apps/mobile/src/features/management/ApiKeysScreen.tsx`, `apps/mobile/src/features/management/IdentityScreen.tsx`, `apps/mobile/src/platform/credentials.ts`, `apps/mobile/src/platform/network.ts`, `apps/mobile/src/platform/server.ts`, `apps/mobile/src/platform/transport.ts`, `apps/mobile/src/platform/workspace.ts`, mobile package/config only if needed.
- **Boundary:** mobile must not import DOM `packages/views`; no web CSS or Wails files. Mobile repeats API/domain contracts from web slices as consumers; web-owned files stay read-only.
- **Failing-test target:** `apps/mobile/src/features/knowledge/data-source-form.test.ts`, `apps/mobile/src/features/knowledge/data-sources.test.ts`, `apps/mobile/src/features/knowledge/editor.test.ts`, `apps/mobile/src/features/knowledge/header-layout.test.ts`, `apps/mobile/src/features/knowledge/parity.test.ts`, `apps/mobile/src/features/knowledge/reference-parity.test.ts`, `apps/mobile/src/features/knowledge/sign-out.test.ts`, `apps/mobile/src/features/chat/appstate.test.ts`, `apps/mobile/src/features/chat/artifact-preview.test.ts`, `apps/mobile/src/features/chat/parity.test.ts`, `apps/mobile/src/features/chat/run-lifecycle.test.ts`, `apps/mobile/src/features/chat/stop-run.test.ts`, `apps/mobile/src/features/management/administration.test.ts`, `apps/mobile/src/features/management/api-keys.test.ts`, `apps/mobile/src/features/management/capabilities.test.ts`, `apps/mobile/src/features/management/configuration-form.test.ts`, `apps/mobile/src/features/management/organizations.test.ts`.
- **Targeted command:** `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`.
- **Live scenario:** iOS and Android simulator login, server selection, KB list/detail/reference edit, chat send/stop/artifact preview, organization read/join if implemented.
- **Evidence required:** simulator screenshots with device/UDID or emulator name, server URL, account, KB names.
- **Review gate:** if a Vue web feature has no native equivalent, record platform delta explicitly rather than accepting silently.
- **Commit boundary:** one mobile slice commit per feature family.

### Checkpoint 2026-09-12 — continued parity execution

- `fc507a1c` added backend-backed upload chunking overrides (`chunk_size`, `chunk_overlap`, `strategy`) for file, URL, and manual sources; `d4a80d4c` preserves the selected folder by using the existing post-create move route. Evidence remains implementing because parser-engine rules, advanced processing sections, mixed batches, dedicated folder picker, localized copy, browser, and native evidence are open.
- `898b7b70` persists MCP `usage_instructions` on editor save, after rechecking the older independent review against current HEAD. Focused MCP/API tests passed 24/24; full Web regression passed 279/279; Web typecheck passed.
- Mobile KB list work (`4356e9dd`, `db8ebec7`) adds role-gated create, scopes, local favorite/recent state and shared grouping; `Mine` now uses the real `creator=mine` API. Mobile knowledge tests passed 20/20 and mobile typecheck passed. N031 remains implementing pending detail/upload/localization and iOS/Android runtime evidence.
- Parallel command-palette/API-client work remains dirty and excluded from these commits.

### Slice S15 — final acceptance and visual/state evidence gate


## 2026-09-13 Round N+1（全页 live 验收批扫 → 9 并行实施代理）


- 门禁基线复核：shared 344/344、web 404/404、typecheck:web/shared 0 错误、树干净（d4bb59c8）。
- 用 .parity-tools/accept-batch.cjs 对 20 路由 × 双端做截图 + DOM 文本批扫（screenshots/accept-20260913/）。逐对目检确认整页级差异（此前各轮只在功能维度验证，页面形态仍是英文调试壳）：
  1. R012 /platform/agents 渲染 ConfigurationPage，AgentList 从未实现；
  2. R017 organizations 英文调试壳 vs Vue 共享空间卡片栅格；
  3. R013 IM tab 英文表单 vs Vue IM 集成面板；
  4. FAQ 页 raw UUID eyebrow/英文空态/无面包屑；
  5. chat 共享视图（views/chat）仍英文调试形态（Conversations 侧栏/内联沙箱终端块/User-Assistant 卡片/Message composer）；
  6. R027 models 面板调试头 + RAW JSON dump vs Vue 模型配置卡片栅格；
  7. SettingsPage 包装层缺陷：general 从未挂载 GeneralPreferencesPanel、wk-settings-values raw dump 全 section 渲染、tenant/userprofile 表单英文、头部刷新按钮、Loading from {apiDomain} 泄漏；
  8. R031 sandbox 面板调试头/dump/折叠开关残留；
  9. NewUserGuide 7 步产品引导 React 全缺（Vue 首访浮层）。
- 证据：evidence/vue-react-parity/2026-09-13-accept-batch-audit.md（差异目录 A/B/C/D + 根因定位）。矩阵 7 行更新（R012/R013/R016/R017/R023/N010 → implementing，R031 增记残留）。
- 并行派发 9 个实施代理（各自文件所有权 + 边界 + TDD + 证据要求）：N007 upload graph 节、R033 skill 安装时间线 SSE（api-client/domain 缺口，后端 routes_infra.go:70-77 依据）、R012 AgentList 页、NewUserGuide 引导、R017 organizations 页、R013 integrations 三 tab、FAQ 页、chat 形态重建、settings R027+包装层合并切片（首个 settings 代理无改动失败，已重新派发全量清单）。
- 主代理集成队列（代理落地后）：SandboxSettingsPanel 调试残留清除、全量门禁、逐页复拍对比、矩阵/账本收尾。


## 2026-09-13 Round N+1（续3）— 第五切片集成

- chat 形态重建集成（97b78d51）：views/chat 渲染层按 Vue 解剖重建（标题药丸+⋯菜单、时间分组侧栏、右对齐用户气泡、纯文本助手+图标行、直接向模型提问 composer、沙箱终端按需抽屉），chat-copy.ts zh-CN 字节级文案层；逻辑面（stream reducer/resume/cancel/retry/approvals/tool results/artifacts/references/分组持久化/starter/agent 选择）零改动且全绿。主代理独立复核：views chat 48/48、web chat 48/48、typecheck:shared 干净。开放（evidence 2026-09-13-chat-visual-form.md）：会话列表仍在页内（Vue 在平台侧栏——shell 集成项）、收藏/模型 chip 仅展示、附件/@占位、AgentStreamDisplay 时间线、chat.* i18n 键在 chat-copy.ts 本地（views 依赖方向惯例）。

## 2026-09-13 Round N+1（续5）— 第八切片集成

- R012 AgentList 页集成（c45aea9d）：/platform/agents 从 ConfigurationPage 别名切回真实智能体列表页（header/图标轨 全部收藏最近本空间/内置分组卡片/能力 chips/操作行/详情抽屉可编辑基本信息/收藏+最近（用户+租户域，Vue 字节兼容格式）/空态；品牌绿 #07c05f 主色；api-client agents.copy 对齐 routes_agent.go:46）。主代理独立复核：agents 35/35、configuration 22/22；live 对等 cards 4=4、rail 4=4、色值一致。开放：完整 AgentEditorModal（6753 行）、/user/favorites api-client 方法（当前 localStorage）、引导组件归属其他切片。注意：R012 代理在 live 验证中发现并修复真实接线缺陷（raw space vs effectiveSpace 分叉导致内置卡不渲染）——live 复验的价值实证。

## 2026-09-13 Round N+1（续6）— 第九、十切片集成

- R013 integrations 页重建集成（23ce0997）：六 tab（IM/嵌入/API/CLI/Chrome/Claw）按 Vue 抽屉解剖重建（IM 集成标题+查看接入文档+IM 渠道计数+平台徽章卡片+添加渠道虚线卡等）；分层翻译器 messages.ts（共享 formatMessage 优先 + 未迁移键 5 语逐字回退，键名对齐上游自动生效）；cli.ts buildCLIConnectCommand 逐字移植；styles.css channel-panel/landing 移植 + .wk-button 基类。独立复核 18/18；live 双端截图。开放：IM 4 步向导+分平台凭证字段+微信二维码（上线紧凑表单）、embed 配置抽屉、API playground 分步抽屉、角色门控、agentEditor.im/embedPublish 键本地回退。
- FAQ 页重建集成（552f466c）：面包屑（知识库›kbName›问答 + 信息弹卡 + 齿轮）、全宽圆角搜索、全部标签筛选面板、图标按钮组、居中中文空态、导入弹窗、编辑抽屉、批量条、分页；UUID eyebrow/英文空态消除。独立复核 10/10（含 UUID 不外露 + 中文文案逐字节断言）；live 截图含 5 张交互态。开放：分页器 vs 无限滚动、编辑抽屉形态、标签管理链接、信息卡简化（myRole/chunkCount/hitCount 键缺失）、导入预览/进度条、卡片级开关、检索测试抽屉、tooltip。
- 在途（3）：settings 包装层（语法错误已自愈，65/65）、shell 会话列表、R038 members（1 个 TS2739 属其 TDD 中途）。

## 2026-09-13 Round N+1（续7）— 第十一、十二项集成

- R038 members 面板重建集成（780667f0）：Vue TenantMembers.vue 全解剖（成员管理+ⓘ 权限弹卡+审计日志+了解 RBAC 链接、待接受的邀请表+空间成员表+TDesign 式分页脚注、邀请弹窗+分享链接弹窗+懒加载审计区；删除/角色接线保留）。独立复核 9/9（含 zh-CN 无英文泄漏扫描）；项目 tsc 0 错误。开放：审计抽屉形态（可调宽/展开行/无限滚动）、tenantInvitation.*/tenantMember.permissions/pager 键回填（本地 zh+en 逐字回退先行）、原生 select 角色图标、搜索防抖。
- 主代理 i18n 迁移（8df9868e）：uploadConfirm + graphSettings 96 键×5 语从 apps/web 本地表迁入 packages/i18n（upload-pipeline.ts 头注释既定后续）；uploadConfirmT/uploadConfirmMessage 委托共享 formatMessage；键集一致性测试。i18n 套件 35/35、documents 62/62、typecheck:shared 0。
- 在途（2）：settings 包装层（ConfigSettingsPanel 编辑中）、shell 会话列表。

## 2026-09-13 Round N+1（续8）— 第十三切片集成

- shell 会话列表并入集成（261352fe）：PlatformShell 在全部受保护页面渲染日期分组会话列表（已置顶/今天/昨天/近7天/近30天/更早；route 即选中态；置顶/重命名/清空/删除接线 + Vue 等价确认；chat 路由经 SHELL_SESSION_ROUTE_EVENT 原地切换，页内重复侧栏经 SessionSidebarShellContext 移除；样式迁移 chat.css→platform/shell.css）。独立复核：web platform+chat 132/132、views chat 48/48；live pin 往返、路由驱动 active 验证通过（截图 shell-sessions-slice/ 7 张含 Vue 基线）。开放：仅首页 30 条 web 来源（Vue 滚动分页+来源桶）、rename=prompt、批量管理/运行中 spinner、chatHeader.confirm/menu.deleteSession/time.* 键。

## 2026-09-13 Round N+1（续9）— 第二轮全页复拍验收

- 11 个切片全部集成后重跑全量门禁：shared 370/370、web 515/515、mobile 118/118、desktop 2/2、typecheck web/shared 0、build:web ✓ 3.46s。
- 第二轮 20 路由×双端批扫（screenshots/accept-20260913-round2/）目检确认：agents 页（内置 4 卡+图标轨+品牌绿）、organizations（共享空间卡栅格+图标轨）、chat 会话页（shell 侧栏日期分组会话列表已并入、无重复页内侧栏、气泡/composer/终端抽屉）、FAQ 页（面包屑+中文空态）全部与 Vue 解剖一致；NewUserGuide 欢迎引导双端同现。
- 本会话累计 16 个提交（f97a5126…261352fe 等，见 git log）；矩阵 9 行更新（R012/R013/R016/R017/R023/R031/R033/N010/N018）。
- 下一步（未来轮次）：剩余 review 行的逐行验收证据补全（Wails 运行时交互、iOS/Android 原生、多状态矩阵）、AgentEditorModal/上下文引导/IM 向导等深水区切片、i18n 键回填（font.*/tenantInvitation.*/chat.* 等本地回退键）。

## 2026-09-13 Round N+1（续10）— i18n 回填集成

- i18n 键回填集成（dcb62825）：102 键×5 语（font.sans/mono 16、members/邀请/审计/分页 32、agentEditor.im/embed/embedPublish 39、shell 会话列表 14、common.noMoreData 1）全部逐字节对齐 Vue locales；可复用提取器 + 独立逐字节校验脚本入库（510/510 BYTE-EXACT OK）；i18n 套件 42/42、typecheck:shared 0。各切片本地回退表自此被共享键遮蔽（formatMessage 优先）。
- 在途（1）：kb-documents 页形态（页级 TDD 实施中）。

## 2026-09-13 Round N+1（续11）— kb-documents 页形态集成

- documents 页 chrome 重建集成（f648d33b）：面包屑 知识库›kbName›文档（caret/信息弹卡/齿轮）+ 副标题 + ⚠ 无解析引擎警告行（由 parserEngines+parser_engine_rules 派生未支持扩展名，前往配置→）+ 全宽搜索 搜索文档名称... + 视图切换 + 全部标签/类型/状态/来源 + 日期范围筛选（列表参数与后端 knowledge.go:913-928 均已支持）+ 插画空态 知识为空，拖放上传（含大小限制文案）；RAW UUID eyebrow 移除；N007 上传下拉与字节进度遮罩保留。documents 套件 79/79（自 62 增）；live 截图 documents-page-slice/ 与 Vue 逐项一致（主代理独立目检 react-documents.png）。
- 在途：该代理将补写证据文档 2026-09-13-documents-page-shape.md。

## 2026-09-13 Round N+1（续12）— 第二波派发

- Wails S13 运行验证入库（4869c476）：打包+codesign+CDP 登录页冒烟；打包链路修复 esbuild 依赖声明（7db7e4d8）与 desktop vite 别名（bbc892ba）。
- 第二波派发（2）：AgentEditorModal 核心移植（分区轨/校验/载荷，分阶段交付）；Wails CDP 运行时交互取证（登录→KB→智能体→共享空间→设置→聊天 全链路截图+断言，仅取证不改代码）。

## 2026-09-13 Round N+1（续14）— KB 卡点击语义对齐

- 修复（5ce821fd）：KB 卡标题/卡体点击与 设置 动作语义反转（Wails CDP 取证发现 #2）——对齐 Vue KnowledgeBaseList：卡点击 handleCardClick（已初始化→文档页，未初始化→该 KB 设置页），设置动作 handleSettingsById→goSettings 无条件进设置（废止 Audit #4 的租户模型门，Vue 中该门仅在创建向导 initialSection）；顺带移除死代码 modelsReady（type==='llm' 永假——后端词表 KnowledgeQA/Embedding/Rerank/VLLM/ASR，取证发现 #3）。web 全量 564/564、tsc 0；:5181 live 复验卡点击按 Vue 规则导航。
- 观察登记：登录后引导聚光背板（wk-guide__backdrop）覆盖卡片属既定引导行为（首访状态），穿透点击仅在引导完成后可用。

## 2026-09-13 Round N+1（续13）— AgentEditorModal 集成与证据补录

- AgentEditorModal 核心移植集成确认：其源文件随 bfefe805 入库（代理自行提交，含 agent-editor-fallback.ts 570 行 5 语回退表、agent-editor.ts 分区/校验/载荷纯逻辑、AgentEditorModal.tsx 分区轨模态、agent-editor.css、双测试文件）。主代理独立复核：agents 35/35 + configuration 22/22；live 对等 cards 4=4、rail 4=4、品牌绿一致；R012 review-round-1 三问题（内置卡不渲染/主色/轨道标签）已修。证据文档由主代理代笔补录（3ca8ba42）：2026-09-13-agent-editor-modal.md。开放：DB 收藏（/user/favorites 接线）、agentEditor.* 键回填 packages/i18n、引导组件。
- Wails CDP 交互取证入库（77b6d6f5）：9/9 页面 PASS（登录→KB列表→文档→智能体→共享空间→设置模型→creatChat→会话切换→行操作菜单），pageerror=0、API≥400=0；fixtures 经应用自身 API 预置。登记缺口：用户菜单原始键名渲染（已修 e4e74e58）、KB 卡标题点击语义反转、modelsReady 门控 latent、昨天分组需 sqlite 直写（沙箱限制）。

## 2026-09-13 Round N+1（续4）— 第六、七切片集成

- 登记：chat 渲染层 chat-copy.ts 当前仅 zh-CN 单语（其余 4 语用户会看到中文）——列为 chat 5 语切片待办（从 Vue locale 提取 chat/input/messages/createChat 域全量键入 packages/i18n + 消费端切 formatMessage）。

## 2026-09-13 Round N+1（续4）— 第六、七切片集成

- upload progress transport 集成（499ec9a8）：api-client NativeMultipartFileRequest.onProgress + 浏览器 XHR sendMultipartFile（字节级、abort→AbortError、fetch 回退）+ platform/http observeUploadProgress 桥；SkillSettingsPanel skillUploading 百分比+进度条；documents UploadProgressMask 遮罩 + 批次均值百分比（KnowledgeBaseList.vue:1586-1595 语义）。独立复核：transport 5/5、http 14/14、panel 31/31、pipeline 30/30、dialog 20/20、api-client 43/43、documents 62/62。该共享层缺口关闭后 N031 移动端可直接消费 transport onProgress。开放：UploadFilesPanel 行内百分比、浏览器 E2E。
- R017 organizations 页重建集成（c2dc9f84）：英文调试壳 → Vue 共享空间 页解剖（header+加入/创建图标按钮、sticky section chips+计数、owners-first 卡片栅格+渐变头像/星点装饰/徽标行/所有者标签、创建/编辑设置弹窗、加入弹窗（邀请码预览/审批门控请求/搜索+joinById/已是成员态）、各过滤空态+移植插画、确认弹窗、toast；~1200 行 CSS 移植）。独立复核 19/19；live 截图 org-page-slice/。开放：PlatformShell 侧栏子筛选合并、RBAC 角色门控 role prop、设置弹窗为 3464 行 Vue 弹窗的功能性简化。
- 在途（6）：R012（目检三问题修复+最终截图）、Integrations/FAQ（最终取证）、settings 包装层（65/65 绿，语法错误已自愈）、shell 会话列表、R038 members。

## 2026-09-13 Round N+1（续2）— 第四切片集成

- NewUserGuide 欢迎引导集成（773e723b）：packages/views/src/guides（存储语义 weknora:new-user-guide-done:v1、7 步目录+5 语字节级文案、聚光几何 Vue 常量、组件 700ms 双检自动打开/键盘/定位重试/可选步自动跳过）+ PlatformShell 挂载与 data-guide 锚点 + shell 动作。独立复核：guides 14/14、jsdom 组件 6/6、platform 目录 75/75；live 复验（清 key→重载→步骤1/2→跳过持久化）通过并截图。开放：contextual guides 未移植、user-menu 重新打开入口、i18n 债 newUserGuide.* 26 键×5 语。

## 2026-09-13 Round N+1（续）— 首批三切片集成

- R033 skill 安装时间线 SSE 集成（f97a5126）：api-client sandbox skills API（install-events/transcript SSE 复用 chat stream parser + 现有 transport 链，guidance GET/steer POST snake_case ≤10000，后端 routes_infra.go:70-77 + internal/handler/sandbox_skill.go 协议核实）；domain 纯进度帧/时间线归并模块；SkillSettingsPanel 时间线 + guidance 轮询 + steer 去重 + 重装重试 + 进度环 + 抽屉拖拽宽度持久化 + markdown 文件预览。主代理独立复核：domain 7/7、api-client 7/7、panel 27/27、client 16/16。开放：upload progress% 需 transport 层 XHR onProgress（共享层缺口，同时解锁 N031）、pick-row 百分比扇出、浏览器/原生证据。
- N007 upload graph 节集成（30f6008d）：UploadGraphSettings 全量 GraphSettings.vue 移植、systemInfo.graph_database_engine + KB indexing_strategy.graph_enabled 门控与导航回退、保存载荷 graph_enabled+extract_config（enabled 钳制）、URL 列表顺序上传、UploadSourceDropdown 文件/文件夹/URL 菜单 + URL 子对话校验、documents.css +290。独立复核：focused 45/45、documents 57/57。开放：extraction 端点 live e2e 需图库启用（当前部署关闭，门控已端到端验证）、错误提示 inline vs Vue toast、单节显示模型差异（导航顺序/门控一致）。
- 主代理直接修复：SandboxSettingsPanel 开关对齐 Vue（5f272a1f）——折叠 details 换 单向 switch + 警告 popconfirm（Vue:36-55 语义），ⓘ 提示气泡样式，样式独立 sandbox-settings.css；24/24。
- i18n 债登记：upload-pipeline.ts 本地字节级拷贝表（graphSettings.*/uploadConfirm.* 块）待迁 packages/i18n 共享包（N007 报告，不阻塞）。
- 其余代理（R012/Guide/R017/integrations/FAQ/chat/settings 包装层）继续并行中。

### Checkpoint 2026-09-13 — R038 member list header repair

- `f0340a3f` adds a Vue-aligned, always-mounted member-list header to
  `apps/web/src/settings/TenantMembersPanel.tsx`, including the server-backed
  member count badge and persistent search controls. The focused regression
  test first failed because the header/count markers were absent, then passed
  3/3 after the repair.
- `git diff --check` passed. `pnpm typecheck:web` remains blocked by the
  pre-existing `@weknora/domain/auth/onboarding` module-resolution error and
  an implicit-any diagnostic in `WorkspaceOnboardingPage.tsx`; these are
  outside the R038 two-file commit. No browser screenshot or full Vue visual
  comparison is claimed, so R038 remains `review`.
- R038 fix round 1 (`ea39080a`, report `bea2f635`) adds React-owned responsive
  member-header CSS, localized title/search labels, an accessible clear action,
  and jsdom interaction coverage for loading/search/clear state. Focused tests
  pass 4/4; Web regression is 285/286 because the pre-existing onboarding
  module-resolution test still fails. Independent scoped re-review is pending,
  and no visual/browser acceptance is claimed.

- **R038 follow-up build gate:** added the explicit
  `@weknora/domain/auth/onboarding` aliases to both Web TypeScript and Vite
  resolution. `pnpm typecheck:web`, `pnpm test:web` (289/289), and
  `pnpm build:web` now pass. `node scripts/check-react-boundaries.mjs` still
  fails on the pre-existing legacy-frontend reachability list and one shared
  host-global finding; this is recorded as an unresolved repository gate, not
  as page acceptance. Browser/Vue screenshot comparison and native evidence
  remain absent, so R038 stays `review`.

- **Mobile gate audit:** running `pnpm --filter @weknora/mobile typecheck` in
  this target worktree still fails because `apps/mobile/app/onboarding.tsx`
  imports the missing `apps/mobile/src/features/auth/OnboardingScreen.tsx`,
  and Expo route typing rejects `/onboarding` in the current route manifest.
  The existing onboarding component test also imports that missing screen.
  A delegated attempt landed on the repository default `main` worktree and
  made no target changes; its result is discarded. This remains an
  implementing/blocked-by-code gap and is not accepted.

- **N005 share dialog slice:** the React knowledge-base share dialog now has
  the Vue two-state form/shared-list interaction, back navigation, loading and
  empty states, permission labels, and synchronous duplicate-mutation
  protection. Focused DOM tests pass 2/2; `pnpm test:web` passes 290/290 and
  `pnpm typecheck:web` passes. No browser/Vue screenshot or native evidence
  was collected, so N005 remains `review`.

- **N005 review fix:** loading/empty mutual exclusion, viewer-only
  organization filtering, create payload/failure side effects, and unshare
  confirmation rejection/failure coverage were added. Focused DOM tests pass
  7/7; Web regression passes 295/295 and Web typecheck passes. No browser,
  Vue screenshot, real-backend, Wails, iOS, or Android evidence exists, so
  N005 remains `review`.

- **Mobile onboarding gate repair:** added the native
  `apps/mobile/src/features/auth/OnboardingScreen.tsx` implementation for
  policy loading/retry, invite-only and create-workspace actions, invitation
  empty state, and logout. The existing native-host onboarding tests now pass
  as part of mobile test 89/89; `pnpm --filter @weknora/mobile typecheck`
  passes after preserving the valid `/onboarding` runtime route through the
  generated-route typing boundary. Simulator/device screenshots and real
  backend onboarding flows are still required; this row remains review.

- **Mobile onboarding review follow-up:** invitation rows now expose
  accept/decline actions, remove accepted/declined entries after server
  confirmation, and logout navigates to the auth route. Mobile test is now
  90/90 and typecheck passes. The generated Expo route manifest still needs a
  durable `/onboarding` declaration instead of the current narrow cast, and
  simulator/device plus real-backend evidence remains outstanding.

- **Mobile onboarding review repair:** accepted/declined invitation actions
  now use the existing typed invitation APIs, successful acceptance refreshes
  workspace state, and logout navigates to the auth route. Expo redirects now
  use type-safe relative route strings rather than forced casts. Mobile test
  and typecheck both pass (90/90); native simulator/device and real-backend
  evidence remain open, so onboarding is still `review`.

- **Mobile test dependency repair:** replaced the temporary ambient jsdom
  declaration with a direct `@types/jsdom` dev dependency in the mobile
  package and lockfile. Mobile test remains 90/90 and typecheck passes; this
  removes the package-boundary masking identified by review. Device and real
  backend evidence remain outstanding.

- **Mobile onboarding error-state repair:** invitation loading failures now
  remain an explicit error rather than rendering the empty inbox; logout
  failures remain on the page with an alert. Native tests pass 91/91 and
  mobile typecheck passes. This is still review pending simulator/device and
  real-backend evidence.

- **Mobile data-source permission guard:** `DataSourcesScreen` now derives
  write capability from the active workspace role, hides Add/Edit/Sync/
  Pause/Resume/Delete for viewers, and fails closed before any write action;
  Logs and refresh remain available for read-only users. Mobile typecheck and
  full native-host tests pass (91/91). Connector-specific forms, resource
  selection, device evidence, and real backend 403 verification remain open.

- **Mobile data-source rule extraction:** the owner/admin mutation rule is now
  centralized in `canManageDataSources` and covered by role-matrix tests,
  while the screen consumes that helper. Focused tests pass 2/2 and mobile
  typecheck passes. Full connector/resource/form parity and native/backend
  evidence remain open.

- **Mobile data-source screen wiring:** added a native-host DOM harness for
  `DataSourcesScreen`, proving viewer controls are read-only while admin
  controls remain available against a mocked typed client. Focused tests pass
  2/2; the full mobile suite passes 92/92 and typecheck passes. Connector
  specific form parity, real 403/backend behavior, and iOS/Android evidence
  remain open.

- **Mobile data-source connector selection:** the native editor now renders
  server-returned connector types as selectable cards with name/description,
  falling back to a text field only when the server returns no types. This
  keeps the submitted type within the existing connector contract. Mobile
  test 92/92 and typecheck pass; connector-specific credentials/resources,
  backend 403, and device evidence remain open.

- **N005 share dialog race/localization repair:** the Web share dialog now uses
  the existing organization-share translations across title, form, list,
  permissions, feedback, and unshare confirmation. Loads are protected by a
  request generation so stale knowledge-base responses cannot overwrite the
  current dialog. Mutation callbacks and success notices now occur only after
  the post-mutation reload succeeds. Focused tests pass 10/10 and Web
  typecheck passes. Browser/Vue screenshot, real-backend, Wails, iOS, and
  Android evidence remain absent; N005 stays `review`.

- **Mobile data-source pre-save connection test:** the native data-source editor
  now exposes a guarded, non-persisting connector test for existing and new
  drafts. It reuses the typed `validateCredentials` transport, reports server
  failures in the page error state, disables conflicting save/test/cancel
  actions while pending, and clears stale success state when draft fields
  change. Focused wiring tests pass 3/3; the full mobile suite passes 92/92
  and mobile typecheck passes. Connector-specific fields/resources,
  localization, real backend, and iOS/Android evidence remain open.

- **Mobile bundle smoke check:** Expo Android export completed and emitted the
  bundle at `/tmp/weknora-mobile-export.jQ1yLt/_expo/static/js/android/entry-ee85a671e5d16ea91ea6b6f282f537c1.hbc`.
  This is bundle evidence only; it is not Android native build, launch, or
  interaction acceptance.

- **Mobile data-source resource selection:** editing an existing native data
  source now loads resources through the existing typed `resources(id)` API,
  renders selectable resource rows, preserves existing `resource_ids`, and
  writes the selected IDs into the update config. Focused wiring tests pass
  4/4; the full mobile suite remains 92/92 and typecheck passes. Vue's lazy
  hierarchical loading, Drive root-token flow, new-source temporary resource
  setup, localization, real backend, and native-device evidence remain open.

- **Mobile data-source hierarchical resource expansion:** resource rows now
  expose expand/collapse controls and request children lazily through the
  existing `resources(id, parentId)` API, while keeping selection rows
  independently actionable. Focused tests pass 8/8; the full mobile suite
  passes 94/94 and typecheck passes. Parent/descendant cover-set semantics,
  Drive root-token setup, new-source temporary browsing, and native/backend
  evidence remain open.

- **Mobile Drive resource root:** the native editor now recognizes bare
  Feishu/Lark folder tokens and `/drive/folder/<token>` URLs, normalizes the
  value, persists it as the selected root resource for an existing source, and
  loads the root resources through the existing API. Focused screen/policy
  tests pass 10/10; full mobile tests pass 94/94 and typecheck passes. New
  source temporary setup, connector-specific credentials, localized copy,
  backend error classification, and native-device evidence remain open.

- **Mobile new-Drive temporary browsing:** loading a Drive root from a new
  draft now creates a paused temporary source, browses resources through the
  existing API, and removes that temporary source when the draft is canceled;
  successful save promotes it through the normal update path. Focused screen
  and policy tests pass 12/12; full mobile tests pass 94/94 and typecheck
  passes. Cleanup on app termination, connector-specific forms, localization,
  backend error classification, and native-device evidence remain open.

- **Mobile GitLab connector form:** the native data-source editor now exposes
  GitLab project ID/ref/paths fields, supports adding/removing project rows,
  validates that at least one project ID is present, and serializes the
  connector's server-backed `settings.projects` shape on save. Focused screen
  and policy tests pass 13/13; full mobile tests pass 94/94 and typecheck
  passes. Exact localized copy, credential-field parity, live backend
  validation, and native-device evidence remain open.

- **Mobile RSS connector form:** the native editor now exposes feed URLs and
  optional `Name: Value` request headers, requires feed URLs before save or
  connection test, hydrates editable feed settings without exposing stored
  secrets, and serializes the existing RSS backend credential/settings shape.
  Focused screen and policy tests pass 14/14; full mobile tests pass 94/94
  and typecheck passes. Exact localized copy, other connector fields, live
  backend validation, and native-device evidence remain open.

- **Mobile connector credential fields:** the native editor now maps the
  existing backend connector contracts to dedicated write-only fields for
  Feishu/Lark and Drive, Notion, Yuque, Tencent IMA, and GitLab. Required
  fields are validated for new sources; existing sources validate by ID when
  no new secret is entered, while explicit changes use the non-persisting
  credential validation endpoint and then the dedicated credentials endpoint.
  RSS test validation includes `feed_urls` without persisting it as a secret.
  Connector changes clear unsaved secret values, while unknown connectors
  retain the legacy key/value fallback. Focused screen/policy tests pass
  20/20; the full mobile suite passes 95/95; mobile typecheck and diff check
  pass. Exact localized copy, Vue/React screenshots, real backend, Wails,
  iOS, and Android evidence remain open; N031 stays `implementing`.

- **N005 Web upload progress/highlight slice:** aligned the knowledge-base list
  with Vue-compatible upload lifecycle events. Tasks are aggregated by KB,
  successful and failed tasks remain visible for 10 seconds, refresh is
  debounced and cleaned up on unmount, and all-failed batches do not trigger a
  refresh. `highlightKbId` now clears from the URL after successful loading and
  jumps to the target page across all list scopes. Focused tests pass 4/4, Web
  regression passes 301/301, Web build and diff check pass, and independent
  review passes. Browser/Vue screenshot, real-backend, Wails, iOS, and Android
  evidence remain absent; N005 remains `implementing`.

- **N031 mobile KB list localization slice:** the native knowledge-base list now
  restores and persists a validated shared locale, injects it into the existing
  transport `Accept-Language` header, and uses shared translations for scope
  labels, navigation, empty/loading/error states, creation form copy, counts,
  and accessibility labels. New favorites/recents/load-failure/accessibility
  keys are present in all five repository-supported locales. Mobile tests pass
  96/96, the i18n knowledge-list suite passes 4/4, typecheck and diff check
  pass, and independent review passes. Remaining N031 detail/data-source/
  upload/graph parity and iOS/Android runtime evidence keep the row
  `implementing`.

- **N031 mobile data-source connector-boundary slice:** the picker now filters
  server connector metadata to the nine connector definitions explicitly
  available in the Vue editor (`feishu`, `lark`, both Drive variants, `notion`,
  `yuque`, `ima`, `rss`, and `gitlab`). Existing unknown/legacy source rows are
  still displayed read-only rather than being silently removed. Regression
  coverage proves all nine supported types survive and the unknown row remains
  visible: mobile tests 97/97, DataSources DOM harness 16/16, typecheck and
  diff check pass; independent review passes. N031 remains `implementing` for
  the remaining data-source lifecycle, detail/graph parity and native evidence.

- **N031 mobile resource-tree selection slice:** resource rows now render the
  existing three-state selection semantics from the shared helper: checked
  (`✓`), indeterminate (`−`), and unchecked. Partial selections are highlighted
  without changing the minimal-cover payload or resource API. The helper suite
  passes 7/7 and the DataSources DOM harness passes 16/16; mobile typecheck and
  diff check pass, and independent review passes. N031 remains `implementing`
  for lifecycle, localization, remaining resource behavior and native evidence.

- **N031 mobile sync-status slice:** the API model now carries the server's
  `latest_sync_log` status and counters; the mobile inventory renders the latest
  result and performs a cleanup-safe 3-second silent refresh while a sync is
  running. Mobile full 99/99, helper 8/8, DOM harness 17/17, shared typecheck,
  focused shared tests 339/339 and diff check pass. N031 remains `implementing`.

- **N031 mobile sync-log slice:** the native log panel now requests 50-row pages
  with guarded offsets, appends additional pages, shows summary counters, and
  expands a row to expose item counters. DataSources DOM harness 19/19,
  typecheck and diff check pass. N031 remains `implementing` because date/status
  localization, full Vue log presentation, visual/runtime and native evidence are
  still open.

- **N031 mobile sync-log i18n slice:** shared i18n now supplies the Vue sync
  history, summary, status, detail, empty-state, close, and refresh labels in all
  five repository locales; the native log panel consumes the active mobile locale.
  i18n suite 1/1, DataSources DOM 20/20, mobile full 99/99, both typechecks and
  diff check pass.
  Main data-source/editor copy and runtime evidence remain open.


## 2026-09-13 Round N（并行五切片 + MCP live 取证 + 共享层补齐）

- 环境：docker dev 基础设施复用；make dev-app :8080（本轮以 SSRF_WHITELIST_EXTRA=mcp.parity-invalid.example 重启以创建失败态取证服务）；Vue :5180 / React :5181 双端同库同账号（parity-test@local.dev，tenant 10000）。
- 并行切片全部落地（各自 focused 测试绿，主代理独立复核后集成）：
  1. R031 Sandbox：wizard/模板目录/深度检查/精确校验/本地化（24/24）；主代理补 SettingsPage→dockerBackendEnabled 能力接线。
  2. R033 Skill：目录卡片/两步向导/安装与管理抽屉/文件浏览器/轮询（23/23）；主代理补通 web zip 上传链路（api-client register 接受 Blob→objectURL 桥 + browser transport sendMultipartFile，后端字段 file）。
  3. R027 Model：签名 rerank/WeKnoraCloud 门控/维度覆盖/thinking-control/校验/载荷/列表/连接测试/Ollama/调试面板（36/36）。
  4. N007 Upload：FolderPickerMenu 目的地选择/文件面板/分节导航/完整 reparse 配置/五语文案（45/45）。
  5. N031 Mobile：dataSource i18n 149 键×5 语、上传遮罩端口、持久化 pins/recents、页面本地化（75/75 + i18n 5/5，typecheck 干净）。
- 共享层：packages/i18n settings 包新增 584 键×5 locale（model./modelSettings./settings.sandbox./settings.skills./settings.weknoraCloud./uploadConfirm./knowledgeStages. 前缀全量补齐 + common.remove 等，字节级来自 Vue locales；脚本 scripts/parity/merge-settings-keys.mjs 可重复执行）；i18n 33/33。
- MCP live 取证与缺陷修复（R024/R043-R046/N016）：同后端双端浏览器证据（列表/新增/编辑/两步流/失败态截图+DOM 文本，evidence/2026-09-13-mcp-live-browser-evidence.md）；发现并修复 React 连接步保存必 400（payload 携带空 usage_instructions；Vue buildPayload 从不发送该字段）——buildMcpConnectionPayload 纯函数 + 回归测试 + 浏览器复验 PUT 200。同时移除 Vue 没有的“描述”字段；登记 stdio 选项、测试连接可达性、0/16000 计数器等待决项。
- 基线：web 399/399、shared 344/344、mobile 75/75、i18n 33/33、typecheck:shared/web/mobile 全绿、build:web 成功。
- 后端事实登记：PUT /api/v1/mcp-services 对 usage_instructions 强制 1..16000（空串 400；省略字段保持原值）；SSRF 校验默认拒绝不可解析域名（白名单可解）。
- 下一步：R045 测试连接可达性裁决、MCP stdio 选项裁决、skill 安装时间线 SSE（api-client 缺口）、upload 图谱节、model combobox、各 implementing 行的浏览器/Wails/原生证据推进，以及 74 个 review 行的验收证据批量收集。

- **并行提示（2026-09-13 17:45）**：另一代理正在 McpSettingsPanel.tsx 上进行 stdio→SSE 收敛与 McpMetadataPanel 结构对齐的未提交 WIP（正是本矩阵登记的待决项方向）；其进行中状态会使 McpSettingsPanel.test.tsx 暂时失败。本行以上提交（…781c81d8）均为全

- **MCP 抽屉结构对齐落地（2026-09-13 round 2，McpSettingsPanel/McpToolsDirectory）**：三项待决全部向 Vue 基准收敛并落地——(1) stdio：编辑器仅保留 SSE/HTTP-Streamable，stdio 服务加载时按 McpServiceDialog.vue:855 强制转 sse，列表标签改 `Stdio`；(2) 测试连接：Vue 基准中 McpTestResultBody.vue/testMCPService 为孤儿代码（无导入/无全局注册），React 抽屉不再挂载测试入口，保留孤儿组件+单测（R045 → review）；(3) 第 2 步：usage textarea 可编辑 + 0/16000 计数 + AI 生成（仅填充字段，保存才落库）+ generateHint 文案，保存以 toolsSynced 门控并复用 syncRequired/instructionsRequired 警示。同轮完成：步骤 0 结构对齐（基本信息含启用开关+hint、连接配置、认证配置含授权状态块与"先保存后授权"、高级配置）、页脚取消→确认顺序+上一步居左、可点击步骤条、元数据面板仅第 2 步挂载、创建密钥内联（buildPayload(true) 对齐）。工具目录按共享五语 Vue 文案本地化（merge-settings-keys.mjs 移交 mcpMetadata.description/parameters/fullSchema/fetch/refresh）。证据：jsdom 交互测试 4 例（结构/stdio 强转/同步门控/step2 保存），web 404/404、typecheck:web、shared 344/344 全绿。浏览器复验（:5181/:8080，mcp-round2-verify.cjs + 截图 mcp-react-round2-*）通过：结构顺序/传输选项/页脚顺序/无测试连接/计数器/同步门控全部符合 Vue 基准；同轮修复 toggle 标签被 fieldset grid 压缩成竖排的样式缺陷。遗留：shadcn/ui 视觉扫、Wails 证据、可同步服务下的已同步态取证；编辑态 CredentialResource 卡片差异保持登记。绿基线；集成其 WIP 前先等其稳定并复跑 focused 测试。

- **Depends on:** S00 and all row-owning slices for rows being accepted.
- **Rows:** any rows proposed for `accepted`; never all rows by default.
- **Files owned:** `docs/migrations/react/vue-react-parity-matrix.md`, `docs/migrations/react/vue-react-parity-progress.md`, `docs/migrations/react/evidence/vue-react-parity/README.md`, `docs/migrations/react/evidence/vue-react-parity/screenshot-matrix.md`, and the row-specific `docs/migrations/react/evidence/vue-react-parity/<date>-<row-group>.md` evidence file only.
- **Boundary:** no product code changes in acceptance-only commits; failed evidence goes back to owning slice.
- **Failing-test target:** row-specific tests from owning slices plus full gates only after targeted success.
- **Targeted command:** minimum `pnpm run test:shared && pnpm run test:web && pnpm run typecheck:shared && pnpm run typecheck:web && pnpm run build:web`; add desktop/mobile/embed commands when accepting platform rows.
- **Live scenario:** Vue :5180 and React :5181 same backend :8080, 1440x900 zh-CN plus en-US spot checks; Wails/iOS/Android/embed where applicable.
- **Evidence required:** screenshot matrix and state checklist showing normal/loading/empty/error/no-permission/disabled/editing/submitting/success/failure for each row.
- **Review gate:** a row can move to `accepted` only when all applicable platform evidence and unresolved backend decisions are closed or explicitly out-of-scope by user decision.
- **Commit boundary:** one acceptance-docs commit per row group.

## 2026-09-13 Round N+3（续2）— creatChat 居中修复 + contextualGuide 回填 + R031 集成 + 证据守护

- creatChat 空态布局真实分歧修复（42a67935，协调者直接实施）：5 语扫描目检发现 React 把欢迎语钉顶、composer 钉底，Vue 是 .dialogue-wrap 双轴居中簇——像素排序曾对其打 3.96% 低分（稀疏内容下百分比失真），文本指纹+并排目检才暴露。TDD 红（无 --empty 类+渲染滚动容器）→绿 16/16；live 复拍与 Vue 构图一致（簇中心尚差 ~40px，列 polish）。
- contextualGuide i18n 回填集成（d33cc559，子代理实施+独立复核）：89 键×5 语入 packages/i18n generated + 三层字节级钉死测试（shared↔Vue、merged、local↔shared），审计 0 漂移；shared 392/392。遗留：renderContextualGuideMessage 共享优先分层（文件头+证据已记）。
- R031 会话标题集成（04296bee，子代理实施+独立复核）：api-client sessions.get + 面板并行标题加载/未命名回退/无 raw id 闪烁；shared 392/392、面板 26/26、sessions 7/7。pick-row percent fan-out 移交 R033（精确交接在证据文档）；R031 新增开放项：openSession 行导航需集成层 prop、inventory 内联 Card vs SettingDrawer 另案。
- 证据守护：并行 agent 的 891673ea（标题为删旧截图）误删了 round-3 双端批扫证据且卷入了 R031 在途文件——已恢复 40 文件（13899d48）并在 R031 集成提交中清理其临时探针文件。登记约定：证据删除类提交必须逐路径核对被引用证据。
- 5 语扫描入库（539c4c4a）：.parity-tools/locale-sweep.cjs + 30 张双端截图 + 证据文档；语言覆盖证据从"仅 zh"推进到 5 语×3 路由×双端。
- 在途（2）：shell-sessions-header（PlatformShell/shell.css，tsc 尚有 2 错误收敛中）、settings-visual-polish（GeneralPreferencesPanel/styles.css .wks-*）。


## 2026-09-13 Round N+3（续）— 移动端运行器修复集成 + 指纹取证

- 移动端测试运行器修复集成（55fdb0d6，子代理实施 + 主代理独立复核 118/118 + typecheck 0）：根因为裸 import esbuild 未声明 + npx tsx 的 NODE_PATH 意外泄漏；改走 createRequire(apps/web) 主机解析 + tsconfig paths（顺带修复既有 typecheck:mobile 红）。遗留（协调者所有）：knowledge/*.test.tsx 同款裸 import（现 glob 不执行 .tsx）、esbuild/jsdom 声明为 mobile devDeps 需根 lockfile 集成。
- round3 批扫文本指纹比对（vue-only/react-only 前 5 项）发现外壳会话区差异：Vue 有可见 我的对话 标题 + 新建对话⌘1 提示；React 仅 aria-label。派发 shell-sessions-header 切片（menu.vue 侧栏解剖对齐）。会话行菜单 置顶/修改标题/清空消息/删除记录 在 react-only 中出现属 DOM 常驻 vs Vue 按需渲染的指纹噪声，非缺陷（删除记录 已是 Vue 权威文案）。
- 派发（3，与 R031 并行）：shell-sessions-header（session-sidebar.tsx + PlatformShell.tsx）、settings-visual-polish（GeneralPreferencesPanel + styles.css .wks-* 块：标题节奏 10px/分段控件选中态/select 外观）、contextualguide-i18n-backfill（packages/i18n generated + views guides 表）。


## 2026-09-13 Round N+3 — 主题默认对齐 + live 批扫 round3 + R007 收尾集成

- live 环境重建：backend :8080（make dev-app）、Vue :5180、React :5181（需 VITE_API_BASE_URL=http://localhost:8080，无 dev proxy，app 直连后端；无此环境变量时同源 /api 全 404——已登记为启动事实）。parity-test@local.dev 经 API 与浏览器双侧验证有效（tenant 10000）。
- 批扫 round3：accept-batch.cjs 增加双端 guide-done 键种子（weknora:new-user-guide-done:v1 + 7 个 contextual 键，两端同键方案），消除首访引导浮层污染；20 路由 ×双端 40/40 截图入 screenshots/accept-20260913-round3/。
- 像素排序结论：kb/agents/org/chat/faq/creatChat/integrations(除 settings) 3.4–6.6%；settings 系 + integrations ~38–42%。热图归因：settings 差异主体来自背板模糊放大底页内容噪声（两端 overlay CSS 逐值一致 rgba(0,0,0,.5)+blur(4px)，React styles.css:560 = Vue Settings.vue:652-659），非可行动缺陷；可行动差异登记为 settings 视觉切片待办：(a) 抽屉内标题纵向节奏 ~10px；(b) 字体大小分段控件选中态（Vue 实心绿底白字 vs React 浅绿底绿字）；(c) 原生 select vs TDesign select 外观（已知项）。
- 主题默认修复（8493557e）：live 对比发现 settings-general 主题模式 Vue=浅色 vs React=跟随系统；TDD 修复 packages/domain local-preferences 缺省/非法值回退 light（Vue useTheme.ts:14-15 权威）+ GeneralPreferencesPanel catch 回退；domain 4/4、settings 面板 149/149；修复后 live 双端均显示 浅色。
- R007 收尾集成（80db6754，子代理实施 + 主代理独立复核）：用户菜单重新打开引导入口（Vue UserMenu.vue:45-50，文案权威为 新手引导 而非简报猜测的 重新查看引导——子代理纠正简报错误）；newUserGuide i18n 块 20 键×5 语字节级钉死（发现并修复 ru-RU steps.agents.desc 漂移）；views guideMessage 共享包优先分层。独立复核：shared 387/387、platform 目录 97/97、i18n+layering 8/8、typecheck 0。
- 在途（2）：移动端测试运行器修复（node --import tsx --test 下 onboarding-component MODULE_NOT_FOUND）、R031 会话标题 api-client 方法。

## 2026-09-13 Round N+2 — chat 5 语切片集成核验 + 会话行删除标签对齐

- 在途工作核实：并行 agent 将 chat 5 语文案切片与上下文引导系统提交为 c25089a8（chat-copy.ts 5 语表 + packages/i18n/src/generated/chat.ts 生成层 + ContextualGuide/Host/contextual-guides.ts 全套 + tests + Vue 基准截图）。主代理独立复核：shared 382/382、web 574/574、desktop 2/2、embed 7/7、typecheck shared/web 0、build:web ✓ 3.27s；contextual guides 16/16 + web jsdom 10/10 + i18n/chat-copy 12/12。
- 集成修复（5c7e0c6b）：(1) 生成层 chat.ts zh-CN chatHeader.deleteSession 字节漂移（删除会话→删除对话，zh-CN.ts:6937 权威）；(2) TDD 对齐 Vue menu.vue 会话行菜单删除项 = upload.deleteRecord（删除记录，5 语字节级）——chat-copy 新增 deleteRecord 键、session-sidebar 行菜单改用、shell 测试先行改断言红→绿；(3) Node 26 navigator.language=en-US 使共享 5 语解析偏离 zh 基准，chat-page/artifact-preview/shell-list 等测试显式 pin zh-CN locale/copy；views index 导出 chat-copy 助手；web views-stub 补 openContextualGuide no-op。
- 已知问题（移交移动端切片）：apps/mobile onboarding-component.test.ts 在 `node --import tsx --test` 下 MODULE_NOT_FOUND 失败（pass 0/fail 1），`npx tsx --test` 下 4/4 通过；已在 HEAD（不含本轮修改）复现，判定为在途/运行器兼容问题，非本轮引入。恢复命令：`cd apps/mobile && node --import tsx --test src/features/auth/onboarding-component.test.ts`。
- 矩阵更新：R007 上下文引导 LANDED；剩余：user-menu 重新打开入口、引导 i18n 键回填 packages/i18n、各引导 live 复验。
- 下一步：R007 收尾切片、R031 会话标题 api-client、N005 浏览器/原生证据、74 个 review 行验收证据、移动端 onboarding 测试运行器修复。

