# open-connector 契约（固定版本运行时证据）

> 上游固定版本：`33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a`（`@oomol-lab/open-connector` v1.5.0，commit "chore: update star history [skip ci]", 2026-09-12）。
> 本文所有事实来自对固定 SHA 的**运行时探针 + 源码核对**（2026-09-12，隔离 clone `.oc-upstream-33dd4ad`），非 README 推断。
> 证据 fixtures：`scripts/open-connector/fixtures/`；场景清单：`scripts/open-connector/contract-cases.json`；门禁：`python3 scripts/open-connector/contract_gate.py scripts/open-connector/fixtures/contract-report.json`（齐全时 exit 0，否则 exit 2）。

## 1. 版本与运行时事实

| 项 | 事实 | 来源 |
| --- | --- | --- |
| 包名/版本 | `@oomol-lab/open-connector` `1.5.0`（private, ESM, workspaces:["web"]） | package.json:2-8 |
| engines / packageManager | **均未声明（ABSENT）**。Node 24 为事实标准：`docker/Dockerfile:1` `FROM node:24-alpine`、CI `NODE_VERSION: "24"`（.github/workflows/ci.yml:27） | package.json（无该字段）+ Dockerfile/CI |
| Lockfile | 仅 `package-lock.json`（357,591 B），`lockfileVersion: 3` → npm；Bun 仅用于可选单文件构建（`.bun-version` 1.4.0） | package-lock.json:4 |
| LICENSE | Apache License 2.0（LICENSE.txt:1；NOTICE.md 商标例外） | LICENSE.txt |
| 真实 start | `node scripts/ensure-generated.ts && node src/server/index.ts`（原生 Node TS 执行，无 tsx；探针即用此命令，Node 26 亦可运行） | package.json scripts.start |
| 真实 build | `build` = `npm run typecheck`（仅类型检查，不产出工件）；`build:web`/`build:binary` 为控制台/单文件可选项 | package.json scripts |
| 测试 | `vitest run` | package.json scripts.test |
| 数据库 | 默认 sqlite（`OOMOL_CONNECT_DATA_DIR` 下 connect.sqlite），可选 Postgres（`OOMOL_CONNECT_DATABASE_URL` + `npm run runtime:migrate`） | src/server/index.ts:33-43 |

## 2. 镜像 digest（协调者裁定 R9）

- **本地构建 digest（探针实际使用的工件）**：`sha256:fcd8d2b871360efcfc43ec051f3359b9de69c04be5d93eebd382633a2418d130`
  - 构建方式：在隔离 clone（构建前 `git rev-parse HEAD` = pinned SHA）执行 `docker build -f docker/Dockerfile -t oc-t01-open-connector:33dd4ad .`，取 `docker image inspect --format '{{.Id}}'`（manifest-list digest；buildx attestations 存在，镜像为多 manifest 产物）。
- **上游发布工件状况**：`ghcr.io/oomol-lab/open-connector` **当前无法取回该 pinned SHA 的不可变 digest**——registry tag 列表（经 ghcr token API）不含 `33dd4ad`，也不含 `v1.5.0`；`tip` 存在但可变且无法证明由该 commit 构建（协调者 2026-09-12 观测 `tip`→`sha256:13e48e5eb49d…` amd64，仅作 T16/T18 部署决策的追踪记录，**不得**当作本 SHA 的证据）。
- 结论：本契约的 image_digest = 本地从 pinned commit 构建的 digest（可复现：同 clone + 同命令）；T16/T18 若需 ghcr 官方镜像须等上游发布或自行推送并另行锁定。

## 3. Wire 契约（全部运行时证实）

### 3.1 认证模型

- Admin scope = `OOMOL_CONNECT_ADMIN_TOKEN`（`Authorization: Bearer`，常量时间比较）+ 控制台 cookie 会话；覆盖 `/api/*`、`/docs`、web 控制台。
- Runtime scope = `/v1/*`、`/mcp`：接受引导 env token（`OOMOL_CONNECT_RUNTIME_TOKEN`）、存储型 `oct_` token（附带 RuntimeGrant）、或 JWT（JWKS 三件套）。
- Admin token 可调用 `POST /v1/actions/*`（唯一的 admin→runtime 越权面）。
- **未配置任何认证时运行时完全开放**（fixtures/admin_denied.json 的对照步骤：无 token GET /api/runtime-tokens → 200）。**部署风险，T16 必须强制配置。**
- 公开路径（永不鉴权）：`/health`、`/oauth/callback*`、`GET /api/auth/session`、`POST /api/auth/logout`、`GET /api/files/*`、控制台壳。
- 运行时 token 打 admin 面 → `401 {error:{code:"unauthorized"}}`（admin API 错误形状 ≠ /v1 信封）。

### 3.2 Runtime Token（/api/runtime-tokens，admin scope）

- `POST` body `{name(必填), allowedActions?, blockedActions?, allowedProxies?, allowedConnections?}` → **`{token:"oct_<43 base64url>", record:{id,name,allowedActions,blockedActions,allowedProxies,allowedConnections,createdAt,lastUsedAt?}}`**；token 仅创建时返回一次（仅存 sha256）。
- Grant 字段**精确 camelCase 四个**：`allowedConnections / allowedActions / allowedProxies / blockedActions`。**无 `blockedProxies`**——出现即 `400 invalid_input "Token policy does not support proxy block rules."`（运行时证实）。
- `PUT /api/runtime-tokens/:id` **必须携带全部四个数组**（缺任一 → 400；运行时证实 `{"allowedConnections":[]}` → `400 "allowedActions must be an array of strings"`）；404 = `runtime_token_not_found`。
- `DELETE` → `{id, revoked:true}`。
- 限制：请求体 ≤256KB、单规则 ≤256B、每清单 ≤128 项。
- **空清单不对称（核心陷阱，运行时证实）**：
  - `allowedConnections: []` / 省略 = **允许全部连接**（调用未被 grant 层拦截，直达 provider）；
  - `allowedActions: []` = 允许全部动作；
  - `allowedProxies: []` = **拒绝全部代理**（与上两者相反！）。
  - 依据 fixtures/empty_grant.json + fixtures/proxy_denied.json；⇒ WeKnora 本地 `NewGrant` 必须拒绝空 grant（规格约束"无连接或无允许动作时不签发 Token"），撤销最后授权必须删 Token 而不是写 `allowedConnections=[]`（写空=放开，运行时已证）。
- 规则语法：动作 `*`/`svc.*`/精确 id；代理精确 service（允许 `*`）；连接为不透明稳定 id 精确匹配。

### 3.3 Action 调用（POST /v1/actions/{actionId}）

- Body：`{"input":{...}}`（无其他包装字段；input 缺省 {}）。
- Headers：`Authorization: Bearer <runtime token>`；可选 `x-oo-connector-alias: <alias>`（大小写不敏感）与 `Idempotency-Key`。别名优先级：body.connectionName > body.alias > header `x-oo-connector-alias` > query.connectionName > query.alias。
- **成功信封（200）**：`{"success":true,"message":"OK","data":<output??null>,"meta":{"executionId","actionId","auditPersisted":true|false}}`。
- **失败信封**：`{"success":false,"message",...,"data","errorCode","meta":{}}` —— 错误码字段是 **`errorCode`（不是 `code`）**；`executionId`/`auditPersisted` 在 **`meta`** 内（非顶层）。/v1 状态码映射：400 invalid_input、402 insufficient_credit、403 authorization_failed/**connection_not_allowed**、404 unknown_action/**connection_not_found**/unknown_service、409 oauth_token_expired/idempotency_*、429 rate_limited、500 internal_error/provider_error。
- 跨连接拒绝发生在凭据使用**之前**（403，meta 带 executionId 且 auditPersisted=true；fixtures/cross_connection.json）。
- **T18 真实 Provider 只读观察（2026-09-13，候选镜像 `4de6df4d…` + 真实 GitHub；记录不改写上文契约）**：受限 token 的动作允许层拒绝（动作不在 `allowedActions`）→ **400 `action_not_allowed`**（"not included in the local action allowlist"，属上文 "400 default" 家族，执行前拒绝、0 次 Provider 调用）；未授权别名 → 403 `connection_not_allowed`（与 fixtures/cross_connection.json 语义一致）。github 的 api_key 连接在 connect 时即对 api.github.com 做真实凭据校验（返回 `profile.accountId`）。宿主 fake-IP VPN（198.18.0.0/15 解析）会触发上游 SSRF 守卫按设计拒绝——按守卫提示加 `OOMOL_CONNECT_EGRESS_TRUSTED_HOSTS` 恢复（T01 先例）。完整记录：[open-connector-release.md](./open-connector-release.md) §6。

### 3.4 别名与 no_auth

- 省略/空别名 → 只解析 `"default"`；**不会回退**到其他命名连接（fixtures/default_alias.json：仅存 alias `primary` 时省略别名调用得到"配置默认凭据"类失败，而非使用 primary）。未知命名别名 → 404 connection_not_found（无任何回退）。
- no_auth provider（如 arxiv）按需合成虚拟连接（id `service:alias`，virtual:true，configured:true），**豁免 token 连接 grant**（受限 token 也能调，fixtures/no_auth.json）；删除后自动重建。
- 连接查找：admin `GET /v1/connections`（wire 字段 `alias` 即 connectionName，另含 providerAccountId）；`GET /v1/connections/by-id/:appId` 按稳定 id 查；**无按别名直查端点**（需列表+过滤）；外部账号 id 仅是响应字段。

### 3.5 Idempotency（协调者裁定 5 的时钟场景用上游测试证实）

- Header `Idempotency-Key`：trim 后非空、≤255 UTF-8 字节，否则 400；输入嵌套 ≤100。
- **窗口 24h 固定**（`idempotencyRetentionHours = 24`）；过期后同 key 重新执行（上游 `expires records 24 hours after the supplied time` 可控时钟测试通过；fixtures/expired_key.json）。
- Key 命名空间**全运行时**（keyHash=sha256(key)，无 token/连接前缀）；**指纹 = sha256({actionId, connectionName(归一化，default==省略), 规范化 input(键排序), runtimeTokenId})** —— 指纹含 runtimeTokenId 属**未文档化行为**（docs/runtime-api.md:210-213 未提）：两个不同 token 复用同一裸 key + 相同请求 → 409 idempotency_key_conflict 而非重放（上游测试 `returns an idempotency conflict when different stored tokens reuse one key`）。
- 重放：同指纹窗口内重放原始状态+响应（同 meta.executionId，fixtures/key_replay.json）；不同指纹 → 409 `idempotency_key_conflict`（fixtures/key_conflict.json）；并发重复 → 409 `idempotency_request_in_progress`（live 并发竞速捕获 + 上游确定性测试，fixtures/in_progress.json）。原始 key 不落盘（仅哈希）。

### 3.6 OAuth 关联（服务端状态机）

- 存储 client config：`PUT /api/oauth/configs/:service`（body 顶层 `clientId/clientSecret/requestedScopes/extra/secretExtra`；自定义 client 需 `OOMOL_CONNECT_ALLOWED_CUSTOM_OAUTH=<service|*>` + `OOMOL_CONNECT_ENCRYPTION_KEY`）。**注意：`POST /api/oauth/authorizations` 的 clientConfig 字段同样读 body 顶层（不是嵌套对象）**。
- 发起（admin）：`POST /api/oauth/authorizations {service, connectionName?, ...}` → `{authorizationUrl, state}`（state=UUID，TTL 15min，单次使用）。
- 发起（/v1）：`POST /v1/connections/:service/connect {returnUri?, ...}` → `data {authorizationUrl, stateHandle, connectionRequestId, status:"initiated", expiresAt(+10min)}`；轮询 `GET /v1/connection-requests/:id` → `{connectionRequestId, service, status: initiated|connected|failed|expired, appId, errorCode, errorMessage, expiresAt, createdAt, updatedAt}`。
- 回调：`GET /oauth/callback?state&code&error`（公开路径）——**关联仅靠 state**；回调 URL 不带别名/连接 id，目标在发起时冻结。未知 state → `400 invalid_oauth_state`；真 state + 假 code → state 被消费、provider token 交换失败 → 回调 400 provider_error 且连接请求 initiated→failed(provider_error)（fixtures/oauth_correlation.json）。
- **blocked-env 部分**：完整 connected（真 provider 授权码）在本环境不可得——已证机制（发起/轮询/回调拒绝/失败迁移），未编造端到端成功。

### 3.7 审计与 proxy

- `meta.auditPersisted`：runs.add 抛错时为 false，**动作结果不变**（成功仍是成功）——唯一触发是存储失败，无 env 开关；以上游注入失败单测证实（fixtures/audit_failure.json）。
- `POST /v1/proxy/:service`：token grant 必需；`allowedProxies=[]` → 403 proxy_not_allowed（空=拒绝全部）；无 executor → 501 proxy_not_supported。首期 WeKnora 不开放 proxy（规格约束）。

## 4. 证据案例表（12 必需场景）

| id | 结果 | 关键断言 | fixture |
| --- | --- | --- | --- |
| cross_connection | pass | 403 connection_not_allowed（凭据使用前） | fixtures/cross_connection.json |
| empty_grant | pass | 空 grant 放行（对比 403），provider 拒假 key | fixtures/empty_grant.json |
| default_alias | pass | 省略别名=default；无回退；未知别名 404 | fixtures/default_alias.json |
| no_auth | pass | 虚拟连接豁免 grant，受限 token 200 | fixtures/no_auth.json |
| admin_denied | pass | runtime token 打 admin 面 401；无认证=开放(对照) | fixtures/admin_denied.json |
| proxy_denied | pass | 403 proxy_not_allowed（空 allowedProxies） | fixtures/proxy_denied.json |
| oauth_correlation | pass | state 单点关联全链路（happy path blocked-env） | fixtures/oauth_correlation.json |
| key_replay | pass | 同 meta.executionId 重放 | fixtures/key_replay.json |
| key_conflict | pass | 409 idempotency_key_conflict | fixtures/key_conflict.json |
| in_progress | pass | 并发 409 idempotency_request_in_progress | fixtures/in_progress.json |
| expired_key | pass | 上游可控时钟 24h 过期测试通过 | fixtures/expired_key.json |
| audit_failure | pass | 注入 runs.add 失败：结果不变 auditPersisted=false | fixtures/audit_failure.json |

门禁输入：fixtures/contract-report.json（含 sha + image_digest + 12 case 指针）；`python3 scripts/open-connector/contract_gate.py <report>` → **exit 0**。

## 5. 与上游文档的差异（源码核对）

1. docs/runtime-api.md:210-213 幂等指纹描述缺 `runtimeTokenId`（源码 action-idempotency.ts:68 含）——跨 token 复用 key 会 409，集成方须知。
2. docs/runtime-api.md:313-342 admin 端点清单遗漏 /api/marketplace、/api/provider-preferences、/api/runtime-policy、/api/auth/*（存在但未列，非矛盾）。
3. 其余核对一致：信封字段、别名 header、默认连接名、空 grant 语义（含 allowedProxies 不对称）、24h、auditPersisted、token 一次性展示、admin token 用法。

## 6. 对 WeKnora 集成的直接约束（供 T02+ 消费）

- Token 签发前必须本地校验 grant 非空（upstream 空=放行）；撤销最后授权 → DELETE token，绝不 PUT `allowedConnections:[]`（=放开）。
- 消费 errorCode（非 code）、meta.executionId/meta.auditPersisted（非顶层）。
- 幂等 key 按“每空间+每连接”派生，避免全运行时命名空间跨租户碰撞；指纹含 runtimeTokenId → 换 token 重放会 409，本地需保持 token 稳定或容忍 409 语义。
- 默认别名仅 "default"、无回退：连接命名必须显式；no_auth provider 的虚拟连接不受 grant 保护（若需限制须在 WeKnora 侧白名单）。
- OAuth 关联只能靠 connectionRequestId 轮询（禁止"最近连接"猜测）；15min state TTL、10min request TTL 决定轮询窗口。
- 部署必须配置 ADMIN_TOKEN（否则完全开放，运行时已证）；首期禁 proxy。

## 7. 环境与复现

- 探针实例：pinned 源码 `node src/server/index.ts` @127.0.0.1:31701（一次性 sqlite /tmp/oc-t01-data-a，ADMIN_TOKEN/EGRESS_TRUSTED_HOSTS/ALLOWED_CUSTOM_OAUTH/ENCRYPTION_KEY 全为本地临时值，已清理）；对照实例 31702 无认证（已清理）。
- 上游测试：隔离 clone 内 `npx vitest run <file> -t <name>`（Node 26 host，vitest 4.1.9）。
- 镜像：本地 `docker build -f docker/Dockerfile -t oc-t01-open-connector:33dd4ad .`（已删除，digest 已记录）。
- host DNS 将部分公网 API 解析到私网地址 → guarded-fetch 拒绝；探针对 export.arxiv.org/api.crossref.org 加了 EGRESS_TRUSTED_HOSTS（仅一次性实例）。
