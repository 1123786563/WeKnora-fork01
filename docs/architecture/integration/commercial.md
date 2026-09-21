# Integration Brief — commercial（Pass A，任务 A2）

Integrator（IA1）对接摘要。事实源：`docs/architecture/moves/commercial.yaml`；
搬迁证据：`docs/architecture/evidence/commercial.md`；领域词汇：`CONTEXT.md` §5.13/§5.18。

## 模块布局（Pass A 后）

| 旧导入路径（别名仍在） | 新导入路径 | 包名 | 内容 |
|---|---|---|---|
| `internal/commercial` | `internal/modules/commercial` | `commercial` | 领域模型：套餐/订单/订阅/预算/配额/结算/履约/rollout/报价 |
| `internal/application/repository/commercial` | `internal/modules/commercial/repository/commercial` | `commercial` | 商业仓储（GORM）：账号、权益、计费账号、预算、订单、outbox、plan version、退款、订阅、usage facts |
| `internal/application/service/commercial` | `internal/modules/commercial/service/commercial` | `commercial` | 应用服务：benefits/billing account/plan version/order/fulfillment/recovery/refund/settlement/execution gate/quota guard |
| `internal/infrastructure/commercialplatform` | `internal/modules/commercial/commercialplatform` | `commercialplatform` | Lago 商业平台适配（provider=`lago`；未配置 env 为合法 blocked-env） |
| `internal/infrastructure/openmeter` | `internal/modules/commercial/openmeter` | `openmeter` | OpenMeter 网关（legacy 计量网关， removal 属 #105，不在本模块） |
| `internal/payment` | `internal/modules/commercial/payment` | `payment` | 支付渠道适配：alipay / wechat（`ProvidersFromEnv`） |
| `internal/usage` | `internal/modules/commercial/usage` | `usage` | 计价：`ModelRates` / `RatesFromEnv`（WEKNORA_USAGE_RATES） |

模块骨架文件（F 任务预置，非本次搬迁）：`internal/modules/commercial/{README.md,module.go,legacy/README.md}`。

## Billing / Payment providers

- 商业平台 seam：`domain.CommercialPlatform`（`internal/modules/commercial/platform.go:221`）；
  生产实现 `commercialplatform.NewPlatformFromEnv()`（config.go:62，provider=`lago`）。
- 支付渠道：`payment.ProvidersFromEnv()` → `map[string]payment.Provider`；空 map 合法
  （blocked-env，checkout 显式报未配置）；部分配置的渠道导致构造失败 = 启动失败。
- 执行闸/网关 seam：`domain.CommercialGateway`（platform.go:83）、`domain.ExecutionGate`
  （platform.go:118）；OpenMeter 生产实现 `openmeter.NewGatewayFromEnv()`。

## 路由（per-file 计数，含 callbacks）

- `internal/router/routes_commercial.go` — **20** 条路由注册；
  入口 `RegisterCommercialRoutes`（routes_commercial.go:19），由
  `internal/router/router.go:412` 挂载。含支付回调
  `callbacksGroup.POST("/:provider", callbacks.HandleProviderCallback)`
  （routes_commercial.go:116，`internal/handler/payment_callbacks.go` 的 handler）。
- `internal/router/routes_usage.go` — **3** 条路由注册；
  入口 `RegisterUsageRoutes`（routes_usage.go:21），由 `internal/router/router.go:380` 挂载。
- 两个入口均为禁改共享文件 `internal/router/router.go` 上的挂载点（见下文 switch 指令）。

## Workers（Redis + Lite asynq 任务类型）

manifest `integration_points.workers: []` —— commercial **没有 asynq 任务类型**。
唯一的后台任务是非 asynq 的履约 outbox drainer：`startCommercialFulfillment`
（`internal/container/container.go:2475`）以 `container.Invoke` 启动 goroutine，
向 `interfaces.ResourceCleaner` 注册 `"CommercialFulfillment"` 优雅停机；
它不占用任何 asynq server 池，Integrator 无需迁移任务类型。

## container.Invoke 生命周期挂点（全部在禁改文件 internal/container/container.go）

1. `container.go:881` — wire CommercialHandler + CommercialPlatform（inline `SetCommercialPlatform`）；
   provider 注册在 container.go:880（`commercialplatform.NewPlatformFromEnv`）。
2. `container.go:892` — wire CommercialHandler + BillingAccountService（inline）。
3. `container.go:902` — wire CommercialHandler + PlanVersionService（inline）。
4. `container.go:916` — wire QuotaGuard → Commercial/TenantMember/Knowledge（inline）：
   `commercialsvc.NewQuotaGuard(db)` 同时注入 memberHandler 与 KnowledgeService
   （QuotaGuard 主所有权 commercial；触及 identity/knowledge，构造性 fail-open）。
5. `container.go:944` — `container.Invoke(startCommercialFulfillment)`（func 于 container.go:2475）。
6. `container.go:1015` — wire CommercialHandler + OrderService（inline）；
   providers provider 注册在其上一行（`payment.ProvidersFromEnv`）。

## 配置键（模块消费的 env）

- 商业平台：`WEKNORA_COMMERCIAL_PLATFORM_PROVIDER` / `_URL` / `_API_KEY` / `_RELEASE`
- 商业网关（OpenMeter）：`WEKNORA_COMMERCIAL_GATEWAY_FAMILY` / `_URL` / `_API_KEY`
- 支付渠道：`WEKNORA_ALIPAY_APP_ID`、`_SELLER_ID`、`_PUBLIC_KEY_PATH`、
  `_MERCHANT_KEY_PATH`、`_GATEWAY_URL`、`_NOTIFY_URL`、`_SIGN_TYPE`；
  `WEKNORA_WECHAT_APP_ID`、`_MCH_ID`、`_MCH_SERIAL`、`_MCH_KEY_PATH`、
  `_PLATFORM_CERTS`、`_NOTIFY_URL`、`_API_BASE_URL`、`_APIV3_KEY_PATH`
- 计价：`WEKNORA_USAGE_RATES`（JSON，由 container.go:2594 读取后传入 `usage.RatesFromEnv`）

## 禁改共享文件中对旧导入路径的引用清单（IA1 switch 指令）

Pass A 在全部 7 条旧路径留下零逻辑别名包，禁改文件保持旧导入即可编译。
**IA1（或 Pass B B-commercial）切换时，仅改 import 行，逐条对应：**

`internal/container/container.go`（唯一含旧路径引用的禁改文件；7 处）：

| 行 | 现状（旧路径） | 切换为 |
|---|---|---|
| 41 | `repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"` | `…/internal/modules/commercial/repository/commercial` |
| 56 | `commercialsvc "github.com/Tencent/WeKnora/internal/application/service/commercial"` | `…/internal/modules/commercial/service/commercial` |
| 62 | `domain "github.com/Tencent/WeKnora/internal/commercial"` | `…/internal/modules/commercial` |
| 92 | `commercialplatform "github.com/Tencent/WeKnora/internal/infrastructure/commercialplatform"` | `…/internal/modules/commercial/commercialplatform` |
| 94 | `ommeter "github.com/Tencent/WeKnora/internal/infrastructure/openmeter"` | `…/internal/modules/commercial/openmeter` |
| 103 | `"github.com/Tencent/WeKnora/internal/payment"` | `…/internal/modules/commercial/payment` |
| 111 | `"github.com/Tencent/WeKnora/internal/usage"` | `…/internal/modules/commercial/usage` |

`internal/router/router.go` / `task.go` / `sync_task.go`、`go.mod`、`go.sum`、`migrations/`：
**零处**旧路径引用，无需动作。

别名包（7 个，全部单文件 `alias.go`，头注释声明 Pass B `B-commercial` 删除）：
`internal/application/repository/commercial`、`internal/application/service/commercial`、
`internal/commercial`、`internal/infrastructure/commercialplatform`、
`internal/infrastructure/openmeter`、`internal/payment`、`internal/usage`。
别名面 = container.go 实际引用的符号（见 evidence 文档 alias surface 一节）；
切换后即可删除对应别名包。

## 横向遗留文件（Pass B B-commercial，不在本任务范围）

`internal/application/repository/{model_usage,user_usage}.go`、
`internal/application/service/{semantic_model_budget,usage_recorder}.go`、
`internal/handler/{commercial,commercial_task_budget,payment_callbacks,usage}.go`
—— 同域 `_test.go` 随主题文件留在横向包内（schema 规则 3）。
