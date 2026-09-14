# 负路径第三批 · 403 租户隔离核验 · Vue vs React（S00 维度补齐）

- 日期：2026-09-14
- 分支：codex/react-multiclient（worktree react-multiclient）
- 执行脚本：`.parity-tools/negpath3-tenant.cjs`（playwright-core / chromium，1440×900，locale zh-CN；幂等可重跑）
- 结构化结果：`.parity-tools/negpath3-results.json`（含 dbCounts before/after）
- 截图目录：`docs/migrations/react/evidence/vue-react-parity/screenshots/negpath3-20260914/`（8 张，每格 Vue/React 各一）
- 被测端：Vue `http://localhost:5180`（frontend/，dev server）· React `http://localhost:5181`（apps/web，dev server）——均为会话开始前已在运行，本批次未拉起/未重启任何 dev server
- 后端：共享 :8080（全程只读核验；未停启、未改任何配置；登录仅用既有账号 parity-test@local.dev，POST /api/v1/auth/login）
- 只读 DB 依据：只读 SELECT（postgres WeKnora 库，account 只执行 count/SELECT），用于证明零写入与枚举跨租户资源 id

## ⚠️ 环境事件（先行记录）

会话开始时（约 10:59）`docker ps` 正常；约 11:01 OrbStack（承载 postgres/redis 等 compose 栈）进入 Stopped，host 5432/6379 拒绝连接，导致共享后端 :8080 的 `/auth/login` 一度统一返回 401（后端把 DB 取用失败映射为 "Invalid email or password"，见 `internal/application/service/user.go:235/242/261`）。本切片执行了 `orb start` 恢复 OrbStack VM（恢复式启动，容器按原状回归；未触碰 :8080 进程、未执行任何 compose 变更、未写任何数据），恢复后登录立即正常。事件已同步父会话。

## 零写入证明

| 表 | sweep 前 | sweep 后 |
| --- | --- | --- |
| tenants | 8 | 8 |
| users | 8 | 8 |
| knowledge_bases | 3 | 3 |
| organizations | 2 | 2 |

- sweep 全部 API 调用为 GET + 每个登录上下文一次 POST /api/v1/auth/login；脚本对每个 cell 记录 `suspectWrites`（非 GET 且非 /auth/login|refresh 的 API 调用）：**8 格全部为空**。
- DB 计数由只读 SELECT 取得（psql 不可用，改用 node pg 驱动直连 localhost:5432，见限制）。

## 服务器侧行为基准（API 直探，只读 GET，为断言预设提供依据）

| 请求（带 parity token） | 状态 | 响应体要点 |
| --- | --- | --- |
| GET /api/v1/knowledge-bases | 200 | 自租户 KB 列表 |
| GET /api/v1/knowledge-bases/00000000-…-000000000000（nil UUID） | 404 | `{error:{code:1003,message:"knowledge base not found"}}` |
| GET /api/v1/knowledge-bases/11111111-2222-3333-4444-555555555555（构造合法 UUID） | 404 | 同上（不泄露存在性） |
| GET /api/v1/tenants/10001（真实其它租户 id） | 403 | `{error:{code:1002,message:"Access denied: URL workspace does not match the active workspace"}}` |
| GET /api/v1/knowledge-bases（X-Tenant-ID: 99999） | 403 | `{"error":"Forbidden: insufficient permissions to access target workspace"}`（注意：error 为 **string**） |
| GET /api/v1/auth/me（X-Tenant-ID: 99999） | 403 | 同上 |
| GET /api/v1/knowledge-bases（X-Tenant-ID: abc） | 400 | "Invalid X-Tenant-ID header" |
| GET /api/v1/organizations/preview/NEGP2INVALID01（死码） | 404 | `{error:{code:1003,message:"Invalid invite code"}}` |

DB 事实（只读 SELECT）：3 个 KB 全部属于 tenant 10000（parity 用户自租户）；**不存在其它租户的 KB**。tenant_invitations 中 id=1（status=accepted，"已用过"）token 为空串 → 本部署无"已消费的组织邀请码"可用。

---

## T1 · 无效 tenant 上下文（会话 tenantId 改为不存在的 99999 → 刷新 /platform/knowledge-bases）

### 预设断言（源码，运行前读取）

**Vue**
- X-Tenant-ID 头来源 `weknora_selected_tenant_id`：`frontend/src/utils/request.ts:97-102`
- `isLoggedIn = !!token && !!user`、`hasValidTenant = !!tenant?.id`：`frontend/src/stores/auth.ts:65-71` —— 把 `weknora_tenant.id` 改为 99999 后 hasValidTenant 仍为真（id 非空即真，服务端不复核）
- 刷新时 store 从 localStorage 恢复（token+user 在），路由守卫**不再走 hydrate**：`frontend/src/router/index.ts:363-394`；hasValidTenant 为真 → 放行 KB 列表页（`index.ts:396-399` 不触发）
- 列表页请求 403 后静默吞掉、渲染空态：`frontend/src/views/knowledge/KnowledgeBaseList.vue:1227-1242`（fetchList 无错误 UI）
- **预设**：停留在 /platform/knowledge-bases，列表空、无自愈、不清会话。

**React**
- 会话键 `weknora_react_session_v1`：`apps/web/src/platform/legacy-session.ts:12`；tenantId 即会话中的激活空间
- transport 以 scopeController 的 tenantId 附加 `x-tenant-id`：`apps/web/src/platform/http.ts:68-81`；`apps/web/src/main.tsx:82-89`
- 受保护路由启动时必调 `client.auth.me()`（`packages/api-client/src/auth/endpoints.ts:169-170`），**任何**失败 → 清凭据 + 渲染登录页：`apps/web/src/main.tsx:265-274`
- **预设**：auth/me 403（x-tenant-id: 99999）→ credential 清为 anonymous → 原 URL 渲染登录页（SPA 渲染，URL 不变）。

### 运行结果

| 端 | 最终 URL | 关键观测 | 判定 |
| --- | --- | --- | --- |
| Vue | /platform/knowledge-bases（不变） | KB 卡片 0；**12+ 个请求全部 403**（auth/me×2、knowledge-bases、shared-knowledge-bases、organizations、system/capabilities、system/info、models、user/favorites×2、web-search-providers、tenants/kv/retrieval-config），全部携带 x-tenant-id: 99999；**token 保留**（weknora_token 未清除）；页面渲染"暂无知识库"空态，无错误提示、无跳转、无自愈 | PASS（符合源码预设） |
| React | /platform/knowledge-bases（不变，SPA 渲染登录页） | **仅 1 个请求**：GET /api/v1/auth/me → 403（x-tenant-id: 99999）；credential 清为 `anonymous`（session JSON 复核确认）、weknora_token 移除；登录表单可见；页面错误 0 | PASS（符合源码预设） |

截图：`t1-invalid-tenant--vue.png`、`t1-invalid-tenant--react.png`

**跨端语义**：恢复策略相反（见 T-1）。两端最终 URL 相同，但 Vue 是"带毒会话继续跑"（每个后续请求都会 403，直到 token 过期前的所有功能都不可用且无提示），React 是"一次性强制重登"。

---

## T2 · 越权对象访问（页面 fetch + 路由级）

### 预设断言

- 后端隔离行为已由 API 直探固定（见基准表）：无效/构造 KB id → 404 code 1003；真实其它租户 id → 403 code 1002。两端走同一后端，页面 fetch 状态码应完全一致。
- React 侧错误对象映射：`packages/api-client/src/errors.ts:46-68`（errorFromResult）。
- 路由级：两端 KB 详情路由均接受任意 id（Vue `frontend/src/router/index.ts:119-124`；React `apps/web/src/routes.tsx:50-56` + `main.tsx:191-195`）。

### 运行结果（页面上下文 fetch，真实 token，两端观测完全一致）

| 探测 | Vue | React | 响应体（两端相同） |
| --- | --- | --- | --- |
| GET /api/v1/knowledge-bases（自租户，对照） | 200 | 200 | data 数组 |
| GET /api/v1/knowledge-bases/00000000-…-0000（nil UUID） | **404** | **404** | code 1003 "knowledge base not found" |
| GET /api/v1/knowledge-bases/11111111-…（构造 UUID） | **404** | **404** | code 1003 |
| GET /api/v1/tenants/10001（真实其它租户） | **403** | **403** | code 1002 "Access denied: URL workspace does not match the active workspace" |

**T2b 路由级**（/platform/knowledge-bases/<nil-uuid> 两端）：URL 均不变、无跳转、无 toast；后端 4 个请求 404（KB 本体、/knowledge、/knowledge/folders、/tags）；两端均渲染"空文档态"（面包屑 + 上传引导文案），无红字错误。与 negpath1 批次记录的 React"knowledge base not found 红字 + 重试"表现不同（该批用非 UUID 串 nonexistent-id，且为较早构建）——本批两端行为一致，作为 UX 缺陷候选另记（不占 T-* 跨端差异）。

截图：`t2-cross-tenant-fetch--{vue,react}.png`、`t2b-nonexistent-kb-route--{vue,react}.png`

---

## T3 · 组织邀请码越权（/platform/organizations?invite_code=<死码>）

### 运行结果

| 端 | preview 请求 | 呈现 | 跳转？ |
| --- | --- | --- | --- |
| Vue | GET /api/v1/organizations/preview/NEGP2INVALID01 → **404** | 邀请预览 modal 打开（`modalVisible=true`），modal 内联错误 **"Invalid invite code"**（invite-preview-error-inline） | 否，停留 /platform/organizations |
| React | 同上 → **404** | 加入 modal 打开，`.org-join-error` 内联 **"Invalid invite code"** | 否，停留 /platform/organizations |

两端一致 ✅（后端 message 原样展示，i18n fallback 未被触发）。

### 静态核对（含「他人租户的有效码不可知」补充项）

- URL → 页面参数：React `apps/web/src/routes.tsx:90-91`（organizationInviteCode）+ `main.tsx:186`；Vue `frontend/src/views/organization/OrganizationList.vue:1096-1099`（onMounted 读 `route.query.invite_code`）。
- preview 调用与失败分支：
  - Vue `OrganizationList.vue:860-883`（handleInvitePreview）：非 success → `result.message || t('organization.invite.invalidCode')`（:876）；抛错 → `e?.message || t('organization.invite.previewFailed')`（:879）；内联渲染 `OrganizationList.vue:295`（样式 :2318-2345）。
  - React `OrganizationsPage.tsx:302-316`（URL 码 preview effect）：`setJoinPreviewError(errorText(reason, t('organization.invite.invalidCode')))`（:313）；modal 内联渲染 :931；输入码路径 :374-381。
  - preview API 路径两端一致：Vue `frontend/src/api/organization/index.ts:418-423`、React `packages/api-client/src/identity/organization.ts:43`（GET /api/v1/organizations/preview/{code}）。
- **「他人租户的有效码不可知」**：invite code 仅由目标空间 owner/管理端生成（POST /organizations/{id}/invite-code，React `identity/organization.ts:48`）；普通用户侧无跨租户枚举码的接口（organizations list/search 只覆盖自己可见/可搜索的组织，不返回他人 invite_code）。因此"拿着他人租户的有效码访问"对普通用户不可构造；即便码泄露，preview 仅回包名称/描述/审批模式（最小暴露），加入动作仍走后端成员/审批校验。两端静态行为等价。
- 失败 fallback 文案分歧（仅非标准错误可见）：Vue previewFailed vs React invalidCode → 见 T-4。

截图：`t3-org-invite-invalid--{vue,react}.png`

---

## T4 · 403 API 错误统一错误映射（静态核对，无运行时格）

| 维度 | Vue（frontend/src/utils/request.ts） | React（packages/api-client + apps/web） |
| --- | --- | --- |
| 403 拦截层 | **无全局 403 分支**（仅 public-auth 短路 :142-148）；403 走通用 reject，`data.error` 为 string 时直接作为 message（:196-197）→ **后端 message 保真** | 无全局 403 分支；`errorFromResult`（errors.ts:46-68）：`error` 为 **string** 时 message 退化为 "Request failed with status 403"（errors.ts:56）、code=HTTP_403；`error` 为对象时 nested.code 保留（1002/1003） |
| 是否统一 toast | 否（页面各自处理） | 否（页面 showToast 各自处理） |
| 是否登出 | **403 不登出**（仅 401；401 → refresh，refresh 失败 → 清 9 个 weknora_* 键 + location='/login'，authRefresh.ts:64-80,142-146 + request.ts:160-175） | 403 不主动登出；但 **bootstrap auth/me 任何失败 → 清凭据 + 渲染登录**（main.tsx:265-274）；401 refresh 仅限幂等读（http.ts:83-89,104-116），失败由 refresh-coordinator 清凭据（refresh-coordinator.ts:101-108）但**无跳转无提示** |
| 错误码映射表 | 无 code 字段提取（message 直出） | ApiError{status, code: nested.code ?? HTTP_<status>}（1002/1003 可编程消费） |

## T-* 差异清单

| # | 严重度 | 差异 | 证据 | 建议归属 |
| --- | --- | --- | --- | --- |
| T-1 | **中** | 无效租户上下文恢复策略相反：Vue 保留受损会话继续运行（此后每个请求 403、无提示、无自愈、token 不清）；React auth/me 403 → 立即清凭据并强制回登录（自愈）。安全侧 React 更稳，UX 侧 Vue 避免了"突然被踢出"；但 Vue 的静默毒化态（12+ 个 403 连发、页面伪装成正常空态）既不安全也不可用 | 本文档 T1 表 + results.json T1 两格；源码行号见 T1 预设 | 语义需协调者裁定"预期一致性锚点"；若以 React 为锚：记 Vue 缺陷（建议 Vue 在 403(x-tenant 隔离) 时清会话引导重登）；若以 Vue 为锚：React 属行为偏差 |
| T-2 | 低 | 403 消息保真度：后端 header 隔离 403 的 body 是 `{"error":"<string>"}`，Vue 把字符串提取为 message（"Forbidden: insufficient permissions…"），React 退化为通用 "Request failed with status 403"（后端文案丢失）；URL 隔离 403（nested code 1002）两端均可取到 message | errors.ts:46-56 vs request.ts:194-205；基准表 | React 侧修复：errorFromResult 增加 `record.error` 为 string 的分支（`frontend/src` 不动） |
| T-3 | 低 | 401 refresh 失败后的登出 UX：Vue 立即清存储并 `location='/login'`（authRefresh.ts:142-146）；React 仅清凭据（refresh-coordinator.ts:106-107），无跳转无提示，用户停留当前页直到下一次整页导航 | 源码行号 | React 侧补齐（redirect 或提示）；低优先（正常过期前会被 T-1 路径兜住） |
| T-4 | 低 | org preview 失败 fallback 文案不一致：Vue `organization.invite.previewFailed`（OrganizationList.vue:879）vs React `organization.invite.invalidCode`（OrganizationsPage.tsx:313/379）；仅当错误无 message（如网络层）时可见 | 源码行号 | i18n 归一，建议以 Vue 文案为准改 React（或反向，由 i18n owner 定夺） |

（T2b 的"两端都不提示不存在 KB"为两端一致的 UX 缺陷候选，不列入跨端差异；另行记录。）

## 结果总览

- **格数 8 · 通过 8 · 失败 0 · 通过率 100%**（每格均为"源码预设 + 运行时验证"双确认；两端运行时状态码全对齐，差异在恢复语义与错误消息映射层，见 T-*）
- 断言明细见 `.parity-tools/negpath3-results.json`（cells[].checks）

## 限制

1. **OrbStack 中途停止事件**（见环境事件）：约 11:01-11:0x 期间共享后端登录短暂 401；已用 `orb start` 恢复（恢复式，非重建），之后全程正常。未重启 :8080 进程。
2. **psql/docker exec 不可用**：OrbStack 恢复后 docker exec 仍受 socket 路径问题影响，DB 只读访问改用 node `pg` 驱动（复用仓库内 `.oc-upstream-33dd4ad/node_modules/pg`，仅 SELECT/count）。
3. **无跨租户 KB 可测**：部署内 3 个 KB 全属 parity 用户自租户（只读 SELECT 证实），KB 级"他人租户 403"路径在本部署不存在；后端对不可见 KB 一律 404（不泄露存在性），已用 nil-UUID/构造 UUID + 真实其它租户 tenant id（10001 → 403 code 1002）覆盖越权两态。
4. **无已消费的组织邀请码**：tenant_invitations 唯一 accepted 记录 token 为空串，故 T3 用死码 NEGP2INVALID01 代替；直探证实"无效码"与该路径同为 404 code 1003，行为等价。
5. **toast 采样窗口**：React T1 的 initialError toast（LoginPage.tsx:91，3s 自动消失）在 +5s 采样点已消失，截图未捕捉；该行为以源码核实（LoginPage.tsx:57-68,88-93）。Vue 端 T1 未观察到任何错误 toast（静默失败）。
6. **文件归属**：本批次未修改 `frontend/**`、`packages/ui/src/**`、`apps/web/src/settings/**`、`packages/api-client/src/settings/**`（他人 WIP 保持原样）；git 工作区中 settings 相关改动为他人既有 WIP。所有产物均未 commit。

## 产物

- 脚本：`.parity-tools/negpath3-tenant.cjs`（幂等；重跑示例 `NEGPATH3_DB_BEFORE=... NEGPATH3_DB_AFTER=... node .parity-tools/negpath3-tenant.cjs`）
- 结果：`.parity-tools/negpath3-results.json`
- 辅助探针（只读）：`.parity-tools/negpath3-api-probe.sh`、`.parity-tools/negpath3-tenant-header-probe.sh`、`.parity-tools/negpath3-preview-probe.sh`
- 截图（8 张）：`docs/migrations/react/evidence/vue-react-parity/screenshots/negpath3-20260914/`：t1-invalid-tenant / t2-cross-tenant-fetch / t2b-nonexistent-kb-route / t3-org-invite-invalid × {vue,react}
- 本文档
