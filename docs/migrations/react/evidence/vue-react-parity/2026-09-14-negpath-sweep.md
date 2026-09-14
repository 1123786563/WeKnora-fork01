# 负路径状态核验 · Vue vs React（S00 维度补齐批次）

- 日期：2026-09-14
- 分支：codex/react-multiclient（worktree react-multiclient）
- 执行脚本：`.parity-tools/negpath-sweep.cjs`（playwright-core / chromium，1440×900，locale zh-CN）
- 结构化结果：`.parity-tools/negpath-results.json`
- 截图目录：`docs/migrations/react/evidence/vue-react-parity/screenshots/negpath-sweep-20260914/`（14 张，每格一张）
- 被测端：Vue `http://localhost:5180`（frontend/，dev server）· React `http://localhost:5181`（apps/web，dev server）
- 后端：共享 :8080（未做任何停启/数据变更；登录账号 parity-test@local.dev，POST /api/v1/auth/login 预验证成功）

## 方法

1. **A 组（登出深链接）**：每条深链接使用全新 browser context（无任何 storage，保证登出态）直接导航，等待跳转稳定后记录最终 URL、登录表单可见性，并断言重定向目标。
2. **B/C 组（登录后无效参数 / 不存在路由）**：先经 UI 登录（复用 parity 套件既有选择器），再逐条导航；记录最终 URL、页面文案锚点、正文文本长度、后端 ≥400 响应、未捕获 JS 错误，并截屏。
3. 每格（cell）= 一个 URL × 一端；共 **14 格**（任务要求的 10 格 + 4 格补充：登出访问不存在路由两端、登录后无效 KB id 不带 /documents 两端）。
4. 断言期望在运行前依据两端路由源码预先设定（Vue：`frontend/src/router/index.ts`；React：`apps/web/src/routes.tsx` + `main.tsx`），运行结果与期望比对；"FAIL" 格即运行时推翻了代码推导的预期，本身就是差异证据。

## 结果总览

- **格数：14 · 断言通过 13 · 失败 1 · 通过率 92.9%**
- 唯一 FAIL = `A:vue:a4-no-such-route`（登出访问不存在路由，预期"留在原地渲染"，实际被重定向到登录页）→ 见差异 D1。

### A 组 · 登出深链接（断言：重定向到 /login + 登录表单可见）

| 深链接 | Vue 最终 URL | React 最终 URL | 重定向目标一致？ | 判定 |
| --- | --- | --- | --- | --- |
| /platform/knowledge-bases | /login | /login?next=%2Fplatform%2Fknowledge-bases | 路径一致，query 不一致 | 两端 PASS |
| /platform/agents | /login | /login?next=%2Fplatform%2Fagents | 路径一致，query 不一致 | 两端 PASS |
| /platform/settings | /login | /login?next=%2Fplatform%2Fsettings | 路径一致，query 不一致 | 两端 PASS |
| /platform/no-such-route（补充格） | **/login（被重定向）** | /platform/no-such-route（渲染 404 页） | **不一致** | Vue FAIL / React PASS |

截图：`a-loggedout-a{1,2,3,4}-*--{vue,react}.png`

### B 组 · 登录后无效 KB 深链接

| URL | Vue 表现 | React 表现 | 状态锚点一致？ |
| --- | --- | --- | --- |
| /platform/knowledge-bases/nonexistent-id/documents | **整页空白**（body 文本长度 0，无侧栏、无 404、无报错；URL 不变） | 应用壳 + 红字锚点 `Page not found: /platform/knowledge-bases/nonexistent-id/documents` + "Back to knowledge bases" 返回链接 | 否 |
| /platform/knowledge-bases/nonexistent-id（补充格） | KB 详情页正常壳（面包屑「知识库 > (空名) > 文档」）+ **空态「知识为空，拖放上传」**；后端 5 个请求全部 404（/knowledge-bases/{id}、/knowledge、/folders、/tags），页面**无任何错误提示** | KB 详情页 + 红字 **"knowledge base not found"**（文件夹树与列表区各一处）+ **重试按钮**；后端 4 个请求全部 404 | 错误语义相反：Vue=空态，React=错误态 |

截图：`b1-invalid-kb-documents--{vue,react}.png`、`b2-invalid-kb-id--{vue,react}.png`

### C 组 · 登录后不存在路由 /platform/no-such-route

| 端 | 表现 |
| --- | --- |
| Vue | **整页空白**（body 文本长度 0，无壳、无 404 页、无报错；URL 不变） |
| React | 应用壳 + 红字锚点 `Page not found: /platform/no-such-route` + "Back to knowledge bases" |

截图：`c1-no-such-route--{vue,react}.png`

## 差异清单（移交协调者，本切片不修代码）

| # | 场景 | Vue 行为 | React 行为 | 严重度建议 | 代码锚点（读码定位，供复核） |
| --- | --- | --- | --- | --- | --- |
| D1 | 登出访问不存在路由 | beforeEach 将无 meta 的未知路径按需登录处理 → 重定向 /login | `not-found` 在鉴权检查**之前**放行 → 未登录也渲染公开 404 页 | 中（信息暴露面不一致；React 会以登出态渲染壳并发出 /auth/me、/sessions 等 401 探测请求） | Vue router/index.ts:352-393（无 catch-all 路由，to.meta 为空走 requiresAuth 分支）；React routes.tsx:143（not-found allow 先于 :146 的鉴权 redirect） |
| D2 | 登出深链接重定向目标 | `/login`（丢弃原始目标） | `/login?next=<原始路径>`（登录后可回跳） | 低-中（功能差异：Vue 登录后无法恢复深链接目标） | Vue router/index.ts:391（`next('/login')` 未携带 redirect）；React routes.tsx:127/135/146 + main.tsx:153-156 |
| D3 | 登录后不存在路由 | 无 catch-all 路由 → router-view 空，**整页空白**（连应用壳都没有） | `NotFoundPage`：壳 + `Page not found: <path>` + 返回链接 | 高（用户完全无反馈，疑似死页面） | Vue router/index.ts:48-220（无 `pathMatch` 兜底）；React apps/web/src/NotFoundPage.tsx + main.tsx:200-203 |
| D4 | 无效 KB id（/platform/knowledge-bases/{id}/documents） | 该嵌套路径无路由 → **整页空白** | 路径不匹配任何 pattern → NotFoundPage 404 页 | 高（同 D3，空白死页面） | Vue router/index.ts:120/166（仅有 :kbId 与 :kbId/creatChat 两级，无 /documents 子路径）；React routes.tsx:53-59 |
| D5 | 无效 KB id（不带 /documents） | 详情页渲染为**空态**「知识为空，拖放上传」，后端 5×404 被吞掉，无错误提示，上传入口仍可用 | 红字 **"knowledge base not found"** + 重试按钮，明确错误态 | 高（Vue 把"不存在的知识库"当"空知识库"，存在误导用户向无效 KB 上传的风险） | Vue views/knowledge/KnowledgeBase.vue（加载失败未进入 error 分支，仍渲染上传空态）；React 文案 "knowledge base not found"（KnowledgeDocumentsPage 错误分支） |
| D6 | 404 文案锚点 | 无任何 404 文案（空白） | `Page not found: <path>` 英文文案 | 低（文案语言与站内 zh-CN 不一致，且无"回到知识库"中文提示） | React apps/web/src/NotFoundPage.tsx |

## 观察备注（非差异，供复核参考）

1. Vue 登出访问受保护路径时会触发一次 `POST /api/v1/auth/auto-setup`（本环境返回 403），随后落 /login；属既有 lite 自动装配探测，非本切片引入。
2. React 登出态渲染 404 页时壳会发出 /api/v1/auth/me、/api/v1/sessions 请求（401）；与 D1 相关，随 D1 一并评估。
3. React 登出态 404 页上还会弹出「欢迎使用 WeKnora」新手引导浮层（1/7，见 `a-loggedout-a4-no-such-route--react.png`）；引导未读时 404 页也会触发引导，是否合理建议一并复核。
4. 两端登录页均复用同一营销页布局（绿底+登录卡），视觉一致；React 表单标签带 `*` 必填星标，Vue 无（此前维度已有覆盖，此处仅存证）。

## 限制

- dev server 内存态：两端均为 vite dev server，非构建产物；路由守卫行为与构建态一致，但首屏加载时序与生产可能有差异。
- React 的 `/platform/no-such-route` 登出格因壳需要 session 探测，截图时序含引导浮层；浮层出现与否不影响 404 锚点断言。
- 未覆盖：register/join/onboarding 的负路径、无权限（403）租户隔离深链接、网络错误态——建议后续切片。
- 断言中"登出访问不存在路由应留在原地"的预期仅用于暴露 D1，不构成规范结论；两端应以协调者裁定的一方为准。

## 附件清单

- 脚本：`.parity-tools/negpath-sweep.cjs`（可重复执行：`node .parity-tools/negpath-sweep.cjs`，需 :5180/:5181/:8080 在线）
- 结果：`.parity-tools/negpath-results.json`（含每格 finalUrl/文案样本/API 4xx/断言明细 + 跨端比较表）
- 截图：`screenshots/negpath-sweep-20260914/` 14 张 PNG
