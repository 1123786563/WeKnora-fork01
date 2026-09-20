# SP1 · Onyx 对齐清债接线（craft 模型网关 + Stop 路由 + 前端连接器入口 + 幽灵清理）设计

- 日期：2026-09-19
- 状态：待用户审阅
- 上游输入：[GAP-MATRIX.md](../../onyx-parity/GAP-MATRIX.md)（C-25、C-3、K-26、K-5）、[ROADMAP.md](../../onyx-parity/ROADMAP.md)（SP1）
- 参照系：Onyx `/Users/wuyongjun/trea/onyx`（craft_gateway 归属判定语义）；本设计不移植 Onyx 代码，只接线 WeKnora 已有实现

## 1. 背景与目标

WeKnora 的 craft 受控模型网关（O02）与验证式 Stop（R06）已实现且测试完备，但均未接入生产装配；confluence/dingtalk 连接器后端已注册但双前端无创建入口；`ConnectorMetadataRegistry` 存在 6 个"有元数据声明、无实现"的幽灵条目。SP1 把这四点全部接线/清债，不引入新产品形态。

**验收口径**：生产 `cmd/server` 启动后，四条能力全部端到端可用（网关凭据签发→HMAC 转发→预算记账；craft 运行中可 Stop 并收敛到终态；两套前端都能创建 confluence/dingtalk 数据源；`/datasource/types` 不再返回无实现的类型）。

## 2. 范围

**做**：
1. 模型网关生产挂载（含 Upstream resolver 生产实现、CraftBudgetService Provide、owner 列迁移、secret 配置、双认证面路由、装配门控）
2. craft Stop HTTP 路由 + executor 后置注入 + 前端 stop 入口
3. React/Vue 双端 confluence + dingtalk 创建入口（字段/引导/i18n）
4. 幽灵连接器声明删除（6 条 registry 条目）

**不做**（登记为已知边界）：
- G4 其余 4 张 AutoMigrate-only 表（orders/subscription/fulfillment/task_budget_extensions/recovery_audit/settlement 相关）的版本化迁移——不在网关调用链上，归商业线（release.go:19-22 的自述缺口维持）
- 商业账户初始化/充值流程
- Onyx 生态新功能（定时任务等，见 ROADMAP SP3+）
- `types.ConnectorType*` 常量不动（存量数据源行与未来 SP7-9 复用）

## 3. 任务 1：craft 模型网关生产挂载（矩阵 C-25）

### 3.1 Upstream resolver 生产实现（新增）

`internal/handler/craft_model_gateway.go:78` 定义 `CraftUpstreamResolver = func(ctx, model string) (CraftUpstreamTarget{BaseURL, Path, APIKey}, error)`，注释红线：只从服务端凭据存储读、fail-closed、禁环境变量。

实现：适配器放 `internal/container` 装配层（handler 定义 resolver 类型、container 组装依赖）——按 model id 从模型 repository 取 `ModelConfig`（`internal/types/model.go:125`；APIKey 落库 AES-GCM 加密 `:222-227`、读出解密 `:264-268`），拼出 `{BaseURL, Path, APIKey}`。模型不存在 / APIKey 为空 → 返回明确错误（网关转发面 fail-closed），**不回落 env、不回落 workspace**。

### 3.2 CraftBudgetService 进容器

`NewCraftBudgetService`（`internal/application/service/craft_budget.go:164`）当前无任何生产调用方。提供 dig Provider；构造参数 policy 从配置读取（CallUpper/TaskLimit 有合理默认）。商业语义分层由现有代码天然保证：
- 租户有 `commercial_budget_accounts` 行 → `Reserve` 完整 G4 预留
- 无账户行 → `ErrBudgetAccountMissing` → `mapReserveDenial` → 干净的预算拒绝（BUDGET_STOPPED 口径）
- **不引入任何 noOpBudget/非商业旁路**（用户决策 2026-09-19）

Recorder 复用已 Provide 的 `CraftUsageService`（`RecordPhysicalCall` 满足 `CraftPhysicalCallRecorder`）。

### 3.3 G4 owner 列迁移（新增）

事实链：`ReservationRow` 有 `Owner` 列（`internal/application/repository/commercial/budget.go:89`），但建表迁移 `migrations/versioned/000116_commercial_budgets.up.sql:24-35` 与 `migrations/sqlite/000036_commercial_budgets.up.sql` 均未建该列；`Reserve`（`budget_reservation.go:45` tryReserve）INSERT `ReservationRow` 时 GORM 会写 owner 列 → 生产 PG 报列不存在（错误的失败形态，污染错误语义）。

补两条迁移（取下一个可用版本号，PG 与 SQLite 编号体系各走各的）：
- PG：`ALTER TABLE commercial_reservations ADD COLUMN IF NOT EXISTS owner VARCHAR(...) NOT NULL DEFAULT ''`（与 GORM tag `not null;default:''` 对齐，含 down）
- SQLite：同语义

迁移后 Reserve/`VerifyLeaseCommit`/`TakeoverLease`（budget_lease.go:52 的 `SET owner = ?`）在生产库全部可用。Craft Go 测试套件补一条"迁移后的库上 Reserve→Dispatch 不报列错"。

### 3.4 路由挂载（两种认证面）

沿用两段式装配：container 构造 → 包级 `Register` 注入 handler 包私有变量 → router 非 nil 才挂（先例：`registerCraftUsageHTTPHandlers` `internal/container/craft_lifecycle.go:99-106` + `internal/router/routes_chat.go:146-158`）。

| 路由 | 方法 | 认证面 | 位置 |
|---|---|---|---|
| `/api/v1/craft/model-gateway/credentials` | POST | Auth 后（租户上下文 `appTenantScope`） | craft session 路由同区（routes_chat.go craft 组） |
| `/api/v1/craft/model-gateway/credentials/revoke` | POST | 同上 | 同上 |
| `/api/v1/craft/model-gateway/v1/chat/completions` | POST | **Auth 前**（cmg1 HMAC 自认证） | router.go pre-auth 区（先例 :216-236 craft preview/sandbox terminal） |
| `/api/v1/craft/model-gateway/v1/models` | GET | 同上 | 同上 |

### 3.5 装配门控与 secret 配置

- 新增 env `WEKNORA_CRAFT_GATEWAY_SECRET`（≥16 字节）；网关启用但 secret 缺失/过短 → 该装配 fail-closed 报错（复用 `NewCraftModelGateway` 自带校验 `craft_model_gateway.go:145-147`），不静默降级
- 挂载门控与 OC runtime 同族：`CRAFT_OPENCODE_BASE_URL` 未设（`internal/container/craft_runtime.go:87-100` fail-closed 语义）时网关不装配——Forward 无调用方，挂了只会增加暴露面
- 记账：`internal/craft/release.go` 的 Billing 轴自述缺口**不因 owner 列修复而翻转**（仍缺 4 张表迁移），收费发布门维持关闭；本任务解锁的是"非收费部署下网关可用且预算语义正确"

## 4. 任务 2：craft Stop 路由 + executor 注入（矩阵 C-3）

### 4.1 后端路由

- 扩展 `session.CraftInteractionAPI` 接口（`internal/handler/session/craft_interaction.go:35-38`）加 `Stop` 与 `DelegationStatus`（`*service.CraftControlService` 已实现，`internal/application/service/craft_control.go:401/510`，只改接口面与注册）
- 路由（挂 `RegisterCraftInteractionRoutes` 同级，遵守 gin 同树同名约定——POST 树 `:session_id`、GET 树 `:id`，见 craft.go:122-124 注释）：
  - `POST /api/v1/sessions/:session_id/craft/runs/:run_id/stop`：body 含 task_id（delegation）；响应 `{phase, result, note}` 保留诚实语义——stop 可能回 `stopping` 而非 canceled，**不折叠成布尔**（craft_control.go:494-503 语义）
  - `GET /api/v1/sessions/:session_id/craft/delegations/:task_id/status`：只读轮询端点（DelegationStatus 不写任何状态，:510-548）
- handler 复用 `craftScope`（craft.go:176-196）做 run 归属校验；与通用 `CancelAgentRun`（agent_run.go:272，仅写 cancel intent）的关系：craft Stop 是其验证式超集，两者并存不合并
- RBAC：沿用 craft 路由组的现有权限面，不新增角色

### 4.2 executor 后置注入（破 provider 环）

生产装配 `internal/container/craft_interaction.go:62` 构造 `NewCraftControlService(runs, store, nil, ...)` 第三参 executor 故意传 nil（注释 :42-46：避免 provider 环，R06 停在 recorded-intent 阶段）。现状：即使挂路由，Stop 永远回 "cancel intent recorded; no executor available"（craft_control.go:476-479），无法收敛 canceled。

修法用同文件现成先例（`wireCraftInteractionRegistrar` 在 container.go:554 Invoke 阶段给 runtime 后装 interaction emitter）：给 `CraftControlService` 加 `SetExecutor(executor)` 后置 setter（并发安全：装配期单写、运行期只读，用 atomic 或锁；实施时按服务现有并发风格选），OC runtime 装配完成后回调注入。executor 为 nil 时 Stop 保持现有诚实降级（"no executor available"），**不假装成功**。

### 4.3 前端

- `packages/api-client/src/craft/index.ts`：补 `stopCraftRun(sessionID, runID, taskID)` 与 `getCraftDelegationStatus(sessionID, taskID)`（该文件现有 create/list/get/inputs/runs/versions/preview/download 全集里没有任何 stop/cancel）
- 工作台（`apps/web/src/features/craft/`）：运行中状态显示停止按钮 → 调 stop → 若 phase=stopping 则轮询 status 至终态（间隔与现有轮询一致）；用户取消订阅不得重复触发 stop（`packages/core/src/craft/controller.ts:365` 现有"subscription stops — no cancel call"语义保持，按钮是显式用户动作）
- i18n：停止按钮/确认文案/停止中状态文案，中英双语

## 5. 任务 3：前端 confluence + dingtalk 创建入口（矩阵 K-26）

后端已注册（`internal/container/container.go:2052-2056`），纯前端三件套 × 两端。

### 5.1 字段定义（对照后端 parseConfig）

| 源 | 字段（*必填） | 后端依据 |
|---|---|---|
| confluence | `edition`（下拉 server/cloud，默认 server）、`base_url`*、`username`*、`api_token`*（cloud 时）/ `password`*（server 时） | `internal/datasource/connector/confluence/types.go:40-78`（credentials 回退 settings；密钥按 edition 二取一） |
| dingtalk | `client_id`*、`client_secret`*、`operator_id`* | `internal/datasource/connector/dingtalk/client.go:54-61`（三者全必填） |

`operator_id` 含用户身份，按 secret 字段掩码处理。confluence 密钥字段 UI 按 edition 值切换显示（单字段呈现，后端只取对应一个）。

### 5.2 React（apps/web）

- `apps/web/src/data-sources/DataSourcesPage.tsx:15` `VUE_CREATE_CONNECTOR_ORDER` 追加 `'confluence','dingtalk'`。注意 `createTypes = ORDER ∩ /datasource/types`（:298-301），服务端已注册故交集自动成立；**只改 ORDER 即够**（幽灵清理后服务端返回集更准确，交集语义不变）
- `apps/web/src/data-sources/form.ts:142` `VUE_CREDENTIAL_FIELDS` 加两源字段条目（形态 `{key, label(i18n key), placeholder?, secret?, optional?, hint?}`）；`:160` `VUE_CONNECTOR_GUIDES` 加 docUrl/权限页/requiredPermissions；credentials 沿用 `key = value` 行协议（form.ts:57-71 parseCredentialLines），`edition` 作为普通凭据字段即可，无需 rss/gitlab 式特判
- i18n `packages/i18n/src/generated/dataSource.ts`：`dataSource.connector.confluence/dingtalk` 与 `connectorDesc.*` 名称/描述键（缺失会显裸 key）；字段 label 键 `dataSource.field.*` 同步

### 5.3 Vue（frontend/）

- `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:499-633` `connectorDefs` 加两源条目（`{type, available:true, docUrl, permissionDocUrl, permissionPageUrl, requiredPermissions, fields:[...]}`，labelKey 用 `datasource.field.*` 前缀——注意与 React 的 `dataSource.` 命名空间不同）
- Vue i18n 同步补键

### 5.4 编辑模式

编辑已有 confluence/dingtalk 数据源（API 建的）时类型下拉用服务端全集（DataSourcesPage.tsx:327），已能出现，本任务不改动编辑面。

## 6. 任务 4：幽灵连接器声明删除（矩阵 K-5）

**决策（2026-09-19）：直接删除 6 条 registry 条目，保留 `types.ConnectorType*` 常量。**

- 删除 `internal/datasource/connector.go:175-312` 中 github(:240)/google_drive(:248)/onedrive(:256)/web_crawler(:272)/slack(:280)/imap(:288) 六个条目（dingtalk :264 是实装，保留）
- 不给 `ConnectorMetadata` 加 experimental 字段：真实现时（SP7-9）元数据（AuthType/Capabilities）大概率重写，标记只是延迟同一决策
- 影响面（已核实）：`ListAvailableConnectors`（:314-331）唯一生产消费方是 `GET /api/v1/datasource/types`（routes_infra.go:301）；React 创建向导被 ORDER 交集天然过滤不受影响；Vue 不消费该 API；现有测试（connector_test.go:10、ima/connector_test.go:504）不锚定这 6 条；**唯一可见变化 = React 编辑模式类型下拉不再出现假选项**（预期修复）
- 测试：更新 `/datasource/types` 返回集断言（11 条）

## 7. 错误处理与安全

- 网关 Forward 失败路径全部已有实现与测试（凭据过期 401/预算拒绝 BUDGET_STOPPED 402-403/上游错误透传语义），本设计不改行为，只装配
- pre-auth 路由暴露面收敛：Forward/ListModels 仅凭 cmg1 HMAC 凭据（≤15min TTL、绑定 tenant/run/delegation/grant/model）鉴权，handler 内部不再依赖任何用户上下文；挂载前在 router 测试断言"未启用网关时这两条路径 404"
- Stop 的保序语义（cancel intent 先于 abort、完成竞态保留原结果）已有测试锚定（craft_control_test.go:502-570），装配层不得破坏
- 迁移可逆：owner 列 ALTER 带 down

## 8. 测试与验收清单

**后端**
1. 容器装配测试：secret 缺失/过短 → 网关装配报错（fail-closed）；`CRAFT_OPENCODE_BASE_URL` 未设 → 网关 handler 未注册、pre-auth 路径 404
2. 生产 router 端到端冒烟：IssueCredential → Forward（真 cmg1 HMAC + mock 上游 + 迁移后 SQLite/PG 库）→ usage 记账出现
3. owner 列迁移：Craft Go 迁移测试套件双库通过；Reserve→Dispatch 无列错误
4. Stop：handler 归属校验（跨租户 404）、phase 语义（abort 未确认时 stopping 不折叠）、SetExecutor 注入后可收敛 canceled、未注入时诚实降级
5. `/datasource/types` 返回集 = 11 实装源

**前端**
6. React：form.ts 两源字段渲染（edition 切换密钥字段）、ORDER 交集出现两源卡片、i18n 键存在性
7. Vue：connectorDefs 两源条目渲染、i18n 键存在性
8. craft 工作台：停止按钮→stop→stopping 轮询→终态 的交互测试

**端到端（源码启动栈）**
9. Vue/React 各创建一个 confluence（server 版可选 mock 端点）与 dingtalk 数据源走通"测试连接"
10. craft 运行中点停止 → 会话收敛终态（真 OC runtime 栈）

## 9. 决策记录

| 决策 | 结论 | 时间 |
|---|---|---|
| G4 owner 列缺失处理 | 补 PG+SQLite ALTER 迁移（不走非商业旁路；无商业账户租户由 ErrBudgetAccountMissing 自然拒绝） | 2026-09-19 |
| 幽灵连接器 6 条 | 直接删 registry 条目，保留类型常量，不加 experimental 字段 | 2026-09-19 |
| release.go Billing 轴 | 维持 NOT satisfiable（4 张 AutoMigrate-only 表仍缺，归商业线），本 SP 不翻转 | 2026-09-19 |
| Stop 与通用 CancelAgentRun | 并存不合并（Stop 是验证式超集） | 2026-09-19 |
