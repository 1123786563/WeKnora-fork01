---
status: accepted
date: 2026-09-21
---

# Commercial Platform 采用单条深 seam（命令、快照、游标对账），禁止逐对象浅封装

WeKnora 通过一条冻结的 provider-neutral `CommercialPlatform` 接口访问商业计费权威（Lago，ADR-0012）：接口只暴露三个操作族——`SubmitCommand`（带类型载荷的商业命令，`Command.Key` 为幂等身份）、`ReadSnapshot`（权威商业快照，readiness 是第一个快照种类）、`Reconcile`（基于不透明游标的对账）。端口定义在领域包 `internal/commercial/platform.go`（沿用 `CommercialGateway` 声明于 `internal/commercial/fulfillment.go`、由 infrastructure 实现的既有惯例，保持 handler→domain 的依赖方向）；适配器位于 `internal/infrastructure/commercialplatform/`（确定性 fake + Lago adapter，二者运行同一份契约测试表）。接口只允许加法演化：新增 `CommandKind`/`SnapshotKind` 常量、新增类型化载荷、新增可选结构字段；修改方法签名或新增 `GetCustomer`/`GetWallet`/`GetInvoice` 式逐对象浅封装方法，必须走 ADR 修订并重新规划 W3，不允许静默修改。旧 OpenMeter Gateway（`internal/infrastructure/openmeter`）与其容器注册保持原样共存，直到 #105 移除；新环境变量族 `WEKNORA_COMMERCIAL_PLATFORM_*` 与旧 `WEKNORA_COMMERCIAL_GATEWAY_*` 相互独立。两个适配器均 fail closed：缺配置返回 `ErrPlatformUnconfigured`、不可达/超时/5xx 返回 `ErrPlatformUnreachable`、确定性错误应答返回 `ErrPlatformInvalidResponse`、未启用的命令/快照/对账种类返回 `ErrPlatformUnsupported`，绝不伪造 ready；错误与 Billing API 应答只含 WeKnora 产品词汇与封闭 token 集，供应商关联身份（`CommandReceipt.ExternalID`）留在 seam 内部。

## Considered Options

- 逐对象浅封装 Gateway（每个 Lago 对象一个 GetXxx/SetXxx 方法）：表面直观，但每个新对象都要改接口并重新冻结契约，方法数随对象数线性膨胀，最终把 Lago 的对象形状泄漏进调用方——拒绝（蠕变）。
- 接口定义在 infrastructure 包内（如 `commercialplatform.CommercialPlatform`）：适配器实现方便，但 handler 将依赖 infrastructure 包，依赖方向倒置，且领域侧（订单/对账 worker）无法在不引基础设施包的情况下编程 against 端口——拒绝。
- 扩展既有 `CommercialGateway`（benefit 网关接口）：看似少一个接口，实际上在同一接口上叠加第二个 seam 的语义（命令/快照/对账 vs benefit apply/find/revoke/settle），旧 fulfillment worker 与新计费迁移互相绑架对方的演化节奏，且 #105 移除 OpenMeter 时无法整段摘除——拒绝（叠加出第二条 seam）。

## Consequences

- 端口签名在 #77（T05）落地后对 W3（#78/#79）冻结：readiness 是唯一启用的快照种类，命令与对账族冻结但未启用，两个适配器对它们 fail closed。
- readiness 的 `Release` 是部署锁定配置（`WEKNORA_COMMERCIAL_PLATFORM_RELEASE`，即 #73 的镜像锁定事实），永不解析供应商响应文本；`/health` 仅作为无凭据的存活性信号。
- `Payload any` 将载荷类型化推迟到 W3 的具体命令种类：冻结覆盖方法签名与信封形状，T05 没有任何命令种类被启用。
- 后续每个 Lago 纵向切片（customer、plan、wallet、invoice……）都表现为新增 kind 常量 + 类型化载荷 + 对应快照 section，评审以"是否出现逐对象方法"为一票否决项。
- 未知 `WEKNORA_COMMERCIAL_PLATFORM_PROVIDER` 值在启动时构造失败，不静默回退；未配置环境保持合法（blocked-env，沿用 openmeter 惯例），`GET /api/v1/commercial/platform/readiness` 如实回答 unavailable/unconfigured。
