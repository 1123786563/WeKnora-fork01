# 负路径状态核验 · 第二批：register / join / onboarding / 网络错误态（S00 维度补齐）

- 日期：2026-09-14
- 分支：codex/react-multiclient（worktree react-multiclient）
- 执行脚本：`.parity-tools/negpath2-sweep.cjs`（playwright-core / chromium，1440×900，locale zh-CN）
- 结构化结果：`.parity-tools/negpath2-results.json`（每格 finalUrl/校验文案/API 4xx/断言明细 + 跨端比较表）
- 截图目录：`docs/migrations/react/evidence/vue-react-parity/screenshots/negpath2-sweep-20260914/`（20 张，每格一张）
- 被测端：Vue `http://localhost:5180`（frontend/，dev server）· React `http://localhost:5181`（apps/web，dev server）
- 后端：共享 :8080（未停启；登录账号 parity-test@local.dev；**运行前后 tenants=8、users=8，无任何用户/租户/知识库数据写入**）
- 前置批次：`2026-09-14-negpath-sweep.md`（登出深链接/无效参数/404，差异 D1-D6）；本批按其「限制」一节建议推进。

## 方法

1. **R 组（register 负路径）**：登出态打开 `/register`（Vue 需先点击登录卡的「创建账户」CTA 切换注册卡；React 直接渲染注册表单），逐格注入（空表单 / 非法邮箱 `not-an-email` / 已存在邮箱 `parity-test@local.dev`）后提交；监听 `POST /api/v1/auth/register` 是否发出、响应状态码与响应体，抓取行内校验文案与 toast 文案。
2. **J 组（join 负路径）**：登出态全新 context 直接访问 `/join`（无 code / 携带无效 code `NEGP2INVALID01`），记录重定向落点与 query；补充格：登录态访问 `/join?code=无效code`，只观察（断言仅发生只读 `GET /api/v1/organizations/preview/{code}`，无任何 join/accept POST）。
3. **O 组（onboarding 负路径）**：
   - `O2`：真实登录（有租户）访问 `/onboarding/workspace`，断言两端均重定向回知识库列表。
   - `O1`：**无租户态为页面内模拟**——复用真实登录 token 的 localStorage 快照（Vue 移除 `weknora_tenant` 等键；React 将 `weknora_react_session_v1.tenantId` 置 null），并用 Playwright route 在页面内改写 `GET /api/v1/auth/me` 响应（tenant→null、can_create_tenant→true）。随后打开「创建空间」表单并**空提交**，断言：页面渲染 onboarding、必填校验出现、`POST /api/v1/tenants` **0 次**（不创建租户）。
4. **N 组（网络错误态）**：真实登录态，用 Playwright `route.abort()` **仅页面内**拦截 `GET /api/v1/knowledge-bases` 与 `GET /api/v1/shared-knowledge-bases` 列表请求（任务原文的 `/api/v1/knowledgebases` 实际代码路径为连字符形式 `/api/v1/knowledge-bases`），访问 `/platform/knowledge-bases` 记录失败态；随后 `unrouteAll` + reload 验证可恢复（断言知识库卡片重新渲染，两端均恢复出 3 张卡片）。
5. 每格（cell）= 一个场景 × 一端，共 **20 格**（10 场景 × Vue/React）；断言在运行前依据两端源码预设（Vue：`frontend/src/router/index.ts`、`views/auth/Login.vue`、`components/CreateTenantDialog.vue`、`stores/auth.ts`；React：`apps/web/src/routes.tsx`、`main.tsx`、`auth/LoginPage.tsx`、`auth/WorkspaceOnboardingPage.tsx`、`App.tsx`）。

## 结果总览

- **格数：20 · 每格预设断言全部通过 20 · 通过率 100%**
- 单端断言全部通过 ≠ 无差异：跨端行为差异以下方差异清单记录（本批共 **4 条跨端差异 + 2 条双端共同问题**）。

### R 组 · register 负路径

| 场景 | Vue | React | 一致性 |
| --- | --- | --- | --- |
| 空表单提交 | 行内必填校验：请输入用户名 / 请输入邮箱地址 / 请输入密码 / 请确认密码；**未发出** register 请求 | 完全相同四条文案；未发出请求 | ✅ 文案逐字一致 |
| 非法邮箱格式 | 行内红字 **请输入正确的邮箱格式**（邮箱字段下方）；未发出请求 | 同文案 **请输入正确的邮箱格式**；未发出请求 | ✅ 文案一致（截图 `r1-register-invalid-email--*.png`） |
| 已存在邮箱提交 | `POST /auth/register` → **400**，响应体 `{"error":{"code":1000,...,"message":"user with this email already exists"}}`；顶部 toast 原样展示该英文消息 | 同 400 + 同响应体；顶部 toast（`.auth-toast`，3s 自动消失）原样展示同英文消息 | ✅ 行为一致（差异见 S-2：英文文案未本地化） |

### J 组 · join 负路径

| 场景 | Vue 落点 | React 落点 | 判定 |
| --- | --- | --- | --- |
| 登出访问 /join（无 code） | `/login`（**无 query**） | `/login?next=%2Fjoin` | ❌ 差异 N-1 |
| 登出访问 /join?code=NEGP2INVALID01 | `/login`（**无 query，code 丢失**） | `/login?next=%2Fjoin%3Fcode%3DNEGP2INVALID01`（code 保留） | ❌ 差异 N-1 |
| 登录态 /join?code=NEGP2INVALID01（补充格，只观察） | `/platform/organizations?invite_code=NEGP2INVALID01`；仅触发只读 `GET /api/v1/organizations/preview/NEGP2INVALID01`（404），无 join POST | 同落点；同只读 preview 404；无 join POST | ✅ 一致 |

### O 组 · onboarding 负路径

| 场景 | Vue | React | 一致性 |
| --- | --- | --- | --- |
| 有租户登录态访问 /onboarding/workspace | 重定向 `/platform/knowledge-bases` | 同 | ✅ |
| 无租户态（页面内模拟）空提交创建表单 | onboarding 页渲染（选择你的工作空间）→「创建空间」→ **模态弹窗**「创建新空间」→ 空提交 → 弹窗内红字 **请输入空间名称**；`POST /api/v1/tenants` 0 次 | onboarding 页渲染（同题）→「创建空间」→ **页内嵌卡片表单** → 空提交 → 字段下方红字 **请输入空间名称**；`POST /api/v1/tenants` 0 次 | ✅ 语义/文案一致（形态差异见 N-2） |

### N 组 · 网络错误态（KB 列表）

| 场景 | Vue | React | 一致性 |
| --- | --- | --- | --- |
| 列表 GET 被 abort | 被拦截请求：`/api/v1/knowledge-bases`、`/api/v1/shared-knowledge-bases`；页面渲染**空态**「暂无知识库 / 点击左侧快捷操作"新建知识库"按钮创建第一个知识库 + 新建知识库按钮」；**无错误文案、无重试入口**；console 产生 **2 条未捕获 Promise rejection**（pageerror "Object"） | 相同两个请求被拦截；**同样渲染空态**（App.tsx 注释明确以 Vue 为 authority：失败态复用空态，不输出原始错误）；无错误文案、无重试；**无未捕获异常** | ✅ UI 一致（双端共同问题 S-1；console 差异 N-3） |
| 恢复（unroute + reload） | 知识库卡片恢复渲染（3 张） | 同（3 张） | ✅ |

## 差异清单（移交协调者，本切片不修代码）

| # | 场景 | Vue 行为 | React 行为 | 严重度建议 | 代码锚点（读码定位，供复核） |
| --- | --- | --- | --- | --- | --- |
| N-1 | 登出访问 /join（含 code） | 路由记录级 redirect 先转跳 `/platform/organizations[?invite_code=…]`，随后鉴权守卫 `next('/login')` **不携带 next/原始 query** → 登录后**邀请码永久丢失**，用户落在知识库列表，加入组织流程中断 | 未认证 → `/login?next=<原始 /join 路径+code>`；登录后 guardRoute 继续消费 next → `/platform/organizations?invite_code=…`，流程可续 | 中（功能性：Vue 登录后无法继续受邀加入组织；与首批 D2 同根因——Vue `next('/login')` 丢弃目标——但在 join 场景有实际业务损失） | Vue router/index.ts:77-89（record redirect）+ :391（`next('/login')` 无参数）；React routes.tsx:126-128 + main.tsx:250-259 |
| N-2 | onboarding 创建表单形态 | **居中模态弹窗**（CreateTenantDialog，带遮罩、取消/创建按钮） | **页面内嵌卡片表单**（无遮罩，创建/Cancel 并排） | 低（视觉/交互形态不一致；文案与校验语义已一致） | Vue frontend/src/components/CreateTenantDialog.vue（t-dialog）；React apps/web/src/auth/WorkspaceOnboardingPage.tsx:121-138（内联 Card+form） |
| N-3 | 列表请求失败时的 console 行为 | 2 条未捕获 Promise rejection（pageerror "Object"，来自被 abort 的列表请求未捕获） | 0 条（失败被 catch 进 pageState.error） | 低（用户不可见；工程健壮性差异） | Vue KnowledgeBaseList.vue 列表加载链路（未 catch abort）；React apps/web/src/App.tsx:225-233（catch → pageState.error） |
| N-4 | React onboarding/邀请链路存在硬编码英文 | 全量 zh-CN 文案 | zh-CN 界面混入英文：创建表单按钮 `Cancel`、提交中 `Creating…`、`Loading…`、`Close`（WorkspaceOnboardingPage）；JoinPage 整页英文（当前主路径不可达，仅 /login?token 自助注册分支使用） | 低-中（i18n 完整性；与首批 D6 同类） | React WorkspaceOnboardingPage.tsx:134/135/143/150；JoinPage.tsx:22/73-88 |

## 双端共同问题（非跨端差异，建议协调者裁定是否共同改进）

| # | 问题 | 说明 |
| --- | --- | --- |
| S-1 | KB 列表网络失败被呈现为「暂无知识库」空态 | 两端一致（React 注释表明以 Vue 为 authority 有意对齐）：用户实际拥有 3 个知识库，但请求失败后页面显示「暂无知识库」并保留「新建知识库」入口——存在误导用户在异常态下重复建库的风险；无任何错误提示或重试入口。属共享 UX 弱点，对齐本身无偏差。 |
| S-2 | register 已存在邮箱的服务端错误原样展示英文 | 两端一致：toast 原样展示后端消息 `user with this email already exists`（error code 1000），未映射为中文文案；与站内 zh-CN 不一致。 |

## 观察备注（非差异，供复核参考）

1. Vue `/register`（无 token）落地的是**登录卡**，需点击「创建账户」CTA 才切换注册卡（Login.vue isRegisterMode 机制）；React `/register` 直接渲染注册表单。对用户可达性等价，未列为差异。
2. 注册表单校验文案（空值 4 条 + 邮箱格式 1 条）两端**逐字一致**（请输入用户名 / 请输入邮箱地址 / 请输入密码 / 请确认密码 / 请输入正确的邮箱格式），客户端均在发请求前拦截。
3. 登录态 `/join?code=无效` 两端都只发只读 `GET /api/v1/organizations/preview/{code}`（404），落点一致（organizations 列表页），未观察到明显的无效邀请码错误提示（截图 `j3-join-loggedin--*.png` 存证）。
4. 无租户态 onboarding 两端都会请求邀请 pending 数（真实接口、真实 token），不影响断言。
5. O1 的无租户态为页面内 mock（见方法 3），若协调者需要真实无租户账号验证，建议在专用测试环境进行（本环境注册会自动建个人租户，见限制 1）。

## 限制

- **未真实执行新邮箱注册**：本部署 `auth.default_tenant_mode` 落到硬默认 `create_personal`（system_settings 无 auth.* 行，已只读查询确认），成功注册会自动创建个人租户，违反本切片「不创建租户」约束；故 `yourname+negpath2@local.dev` 的一次性真实注册未执行，改用已存在账号（parity-test@local.dev）覆盖「服务端拒绝」路径。注册成功→自动登录取代路径的行为（两端均应切回登录卡并预填邮箱）未验证。
- onboarding 无租户态为页面内 mock（真实 token + route 改写 /auth/me），守卫与页面逻辑按真实代码执行，但 /auth/me 响应体为改写后的合成数据。
- dev server 内存态：两端均为 vite dev server，非构建产物；路由守卫行为与构建态一致，首屏时序与生产可能有差异。
- 网络错误态仅覆盖 KB 列表页的列表 GET；上传、agent 等其它网络失败路径不在本批范围。
- N1 的「空态即失败态」断言基于卡片计数为 0；若未来列表改为骨架/错误卡呈现，需同步调整断言。

## 附件清单

- 脚本：`.parity-tools/negpath2-sweep.cjs`（可重复执行：`node .parity-tools/negpath2-sweep.cjs`，需 :5180/:5181/:8080 在线；幂等、无数据写入）
- 结果：`.parity-tools/negpath2-results.json`（每格 finalUrl/校验文案/服务端响应样本/断言明细 + 10 组跨端比较）
- 截图：`screenshots/negpath2-sweep-20260914/` 20 张 PNG（r0/r1/r2/j1/j2/j3/o1/o2/n1/n2 × vue/react）
