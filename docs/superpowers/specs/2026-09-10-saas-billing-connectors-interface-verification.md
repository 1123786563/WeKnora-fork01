# WeKnora SaaS 与 Connector 接口验证清单

日期：2026-09-10。关联：[完整规格](2026-09-10-saas-billing-connectors-design.md)。状态：检查项与通过标准已整理；仅完成固定版本 schema 静态核验和本地只读连接探测，未完成接口联调。

## 1. 状态、证据与使用规则

- `schema`：固定提交声明该方法／路径／operationId；不保证本地启用或语义符合产品。
- `doc`：官方正文或可读官方索引支持所述事实；不等于本版运行结果。
- `pending`：尚未执行，不能写成通过。
- `blocked-env`：已尝试，本地服务／账号／凭据等前置条件缺失。
- `pass` / `fail`：实际执行并保存可复核输入、结果与断言后才能使用。

本轮没有任何业务接口项目标记为 pass。历史 Docker 冒烟见 [部署说明](../../../deploy/openmeter/README.md)，仅证明当时测试事件重复投递后聚合为 1。

本轮本地直连四个 GET 均 connection refused。第一次通过环境默认代理的请求得到 502，随后禁用回环代理直连仍拒绝连接；因此结论是当前未获得服务响应，不能据此断言路径错误。未启动／重启容器，未写 Customer、Grant、订单或外部应用。

机器可读的版本、schema SHA-256、operationId 行号和探测结果见 [接口目录证据](evidence/2026-09-10-saas-billing-interface-inventory.json)。禁止保存真实密钥、付款用户信息或完整凭据到证据目录。

执行记录统一包含：检查 ID、提交和镜像 digest、配置摘要、时间、隔离空间／测试对象 ID、脱敏请求与响应、幂等键、前后状态、实际断言、清理方式和限制。外部结果不明的记录不能通过“删本地记录重测”掩盖。

## 1A. 2026-09-11 单元／集成证据状态汇总（O03 收口）

本节按[实施台账](../plans/saas-billing-connectors-progress.md)的真实证据，记录各检查当前可主张的最高层级；不改变各检查的最终通过标准（门槛仍以 runtime 联验为准），provider/browser 级一律保持 blocked-env/pending，**没有任何 pending/blocked-env 被改为 pass**。逐项证据（任务／提交／命令／结果）见[验收收口文档](../plans/saas-billing-connectors-acceptance.md)。

- OM-01..OM-10：blocked-env（无 OpenMeter 服务／商户凭据；V03 gate f84b98b 持续拒绝 selected-model.json）。
- COM-01..COM-07：unit/integration 层 pass（F02 bb7e8c6、F03 2f4462a、C01 19e64ef、C04 1e7de9a、C05 d22aad3、F04 af899ce；SQLite／域层定向测试）。
- WX-02、ALI-02：unit 层 pass（C02 e269415 TestWechat 9/9、C03 187570b TestAlipay 14/14，回调验签／解密／重放）；WX-01/03/04、ALI-01/03/04 pending（无真实商户配置，且无下单／查单／退款链路证据）；WX-05、ALI-05 blocked-env。
- BUD-01..BUD-08、USE-01..USE-06：integration/unit 层 pass（U01 5449a1c、U02 0cfa73e、U03 3ef3742、U04 a137c79、U05 0a38092、F01 5a1111e；SQLite + -race）。
- CON-01..CON-08：integration/unit 层 pass（A01 a798e77、A02 1b5f149、A03 02cc527、A04 21bb6cd）。
- FS-01..FS-03、NO-01..NO-03：integration 层 pass（httptest 桩：A05 0888fac TestFeishu 9/9、A06 1cfc957 TestNotion 12/12）；FS-04、NO-04 blocked-env（真实执行需用户指定目标与凭据）。
- SYNC-01..SYNC-05：integration 层 pass（A07 0e30173，SQLite + 桩；真实飞书／Notion 端点 blocked-env）；OPS-01/OPS-03（O01 bf5ab4d）、OPS-02（O02 4e99dc9）integration 层 pass；OPS-04 unit 层 pass（W02 7adec10 纯函数），浏览器层未运行。
- AC-01..AC-20：blocked-env（browser 层产品联验）。

准入命令：`python3 scripts/saas/release_gate.py --report artifacts/saas-acceptance/report.json` 当前退出码 1（诚实预期；该命令缺项／blocked-env／skip／mock 替代真实提供方均退出 1，功能门槛可 `--without` 逐能力排除但始终列出未完成范围）。未完成范围清单见验收收口文档。

## 2. 官方 OpenMeter 固定版本接口目录

来源为官方提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`，对应发布版本 `v1.0.0-beta.232`：

- [api/openapi.yaml](https://github.com/openmeterio/openmeter/blob/887e0cac903ccd06e74d61ed23c651651d10c7a9/api/openapi.yaml)，SHA-256 `927d82ffe64709d1527f705f9af1c192b3c53596b152894bfad30a3d1d26e5f1`。
- [api/v3/openapi.yaml](https://github.com/openmeterio/openmeter/blob/887e0cac903ccd06e74d61ed23c651651d10c7a9/api/v3/openapi.yaml)，SHA-256 `5ae852be96ca3102ff5878d2656d7ef20835e9aa4d0ae46b934d2b96aaedd7f1`。

V3 schema 的本地 server 前缀为 `/api/v3`，paths 以 `/openmeter` 开始；下表已拼接本地完整路径，不能误用为单独 `/openmeter/...`。这两套商业模型的关联需 OM-02 验证，不可因同源而直接混用。

| 家族 | 方法与本地路径 | operationId | 证据状态 |
| --- | --- | --- | --- |
| 官方 V1/V2 | `POST /api/v1/apps/custom-invoicing/{invoiceId}/payment/status` | `appCustomInvoicingUpdatePaymentStatus` | schema |
| 官方 V1/V2 | `POST /api/v1/customers` | `createCustomer` | schema |
| 官方 V1/V2 | `GET /api/v1/customers/{customerIdOrKey}` | `getCustomer` | schema |
| 官方 V1/V2 | `POST /api/v1/events` | `ingestEvents` | schema |
| 官方 V1/V2 | `DELETE /api/v1/grants/{grantId}` | `voidGrant` | schema |
| 官方 V1/V2 | `GET /api/v1/meters/{meterIdOrSlug}/query` | `queryMeter` | schema |
| 官方 V1/V2 | `POST /api/v1/plans` | `createPlan` | schema |
| 官方 V1/V2 | `POST /api/v1/plans/{planId}/publish` | `publishPlan` | schema |
| 官方 V1/V2 | `POST /api/v1/subscriptions` | `createSubscription` | schema |
| 官方 V1/V2 | `POST /api/v1/subscriptions/{subscriptionId}/cancel` | `cancelSubscription` | schema |
| 官方 V1/V2 | `POST /api/v1/subscriptions/{subscriptionId}/change` | `changeSubscription` | schema |
| 官方 V1/V2 | `POST /api/v2/customers/{customerIdOrKey}/entitlements` | `createCustomerEntitlementV2` | schema |
| 官方 V1/V2 | `GET /api/v2/customers/{customerIdOrKey}/entitlements/{entitlementIdOrFeatureKey}/grants` | `listCustomerEntitlementGrantsV2` | schema |
| 官方 V1/V2 | `POST /api/v2/customers/{customerIdOrKey}/entitlements/{entitlementIdOrFeatureKey}/grants` | `createCustomerEntitlementGrantV2` | schema |
| 官方 V1/V2 | `GET /api/v2/customers/{customerIdOrKey}/entitlements/{entitlementIdOrFeatureKey}/history` | `getCustomerEntitlementHistoryV2` | schema |
| 官方 V1/V2 | `POST /api/v2/customers/{customerIdOrKey}/entitlements/{entitlementIdOrFeatureKey}/reset` | `resetCustomerEntitlementUsageV2` | schema |
| 官方 V1/V2 | `GET /api/v2/customers/{customerIdOrKey}/entitlements/{entitlementIdOrFeatureKey}/value` | `getCustomerEntitlementValueV2` | schema |
| 官方 V3 | `POST /api/v3/openmeter/customers` | `create-customer` | schema |
| 官方 V3 | `GET /api/v3/openmeter/customers/{customerId}` | `get-customer` | schema |
| 官方 V3 | `POST /api/v3/openmeter/customers/{customerId}/credits/adjustments` | `create-credit-adjustment` | schema |
| 官方 V3 | `GET /api/v3/openmeter/customers/{customerId}/credits/balance` | `get-customer-credit-balance` | schema |
| 官方 V3 | `POST /api/v3/openmeter/customers/{customerId}/credits/grants` | `create-credit-grant` | schema |
| 官方 V3 | `GET /api/v3/openmeter/customers/{customerId}/credits/grants` | `list-credit-grants` | schema |
| 官方 V3 | `GET /api/v3/openmeter/customers/{customerId}/credits/grants/{creditGrantId}` | `get-credit-grant` | schema |
| 官方 V3 | `POST /api/v3/openmeter/customers/{customerId}/credits/grants/{creditGrantId}/settlement/external` | `update-credit-grant-external-settlement` | schema |
| 官方 V3 | `POST /api/v3/openmeter/customers/{customerId}/credits/grants/{creditGrantId}/void` | `void-credit-grant` | schema |
| 官方 V3 | `GET /api/v3/openmeter/customers/{customerId}/credits/transactions` | `list-credit-transactions` | schema |
| 官方 V3 | `POST /api/v3/openmeter/events` | `ingest-metering-events` | schema |
| 官方 V3 | `POST /api/v3/openmeter/governance/query` | `query-governance-access` | schema |
| 官方 V3 | `POST /api/v3/openmeter/plans` | `create-plan` | schema |
| 官方 V3 | `POST /api/v3/openmeter/plans/{planId}/publish` | `publish-plan` | schema |
| 官方 V3 | `POST /api/v3/openmeter/subscriptions` | `create-subscription` | schema |
| 官方 V3 | `POST /api/v3/openmeter/subscriptions/{subscriptionId}/cancel` | `cancel-subscription` | schema |
| 官方 V3 | `POST /api/v3/openmeter/subscriptions/{subscriptionId}/change` | `change-subscription` | schema |

上述目录刻意只列当前设计相关操作。特别注意：V2 Entitlement Grant 的作废仍由固定 schema 中 `DELETE /api/v1/grants/{grantId}` 声明；V3 Credit Grant 是另一条 void 操作。路径存在不证明作废可以替代部分退款。

未在两份 schema 的路径／operationId 中定位到显式命名为 reserve/reservation 的端点，不将这个搜索结果扩大为“官方一定没有预占能力”。需要在 OM-10/BUD 检查中验证能力或采用经证明的平台协调方案。

## 3. 门槛顺序

| 门槛 | 必须满足 | 未满足时的边界 |
| --- | --- | --- |
| G0 环境与契约 | OM-01、OM-02，确定实际启用版本和唯一商业模型 | 不编写假定某套 Credits 语义的生产适配。 |
| G1 商业正确性 | OM-03 至 OM-09、COM-01 至 COM-07 | 不开放实际商品购买／充值／退款。 |
| G2 严格消费 | OM-10、BUD-01 至 BUD-08、USE-01 至 USE-06 | 不开放依赖严格预付保证的收费执行；可做无扣费影子记录。 |
| G3 外部渠道 | WX-01 至 WX-05、ALI-01 至 ALI-05 | 哪个渠道未通过，哪个渠道不能上线；模拟不替代真实接入。 |
| G4 Connector | CON-01 至 CON-08、FS-01 至 FS-04、NO-01 至 NO-04、SYNC-01 至 SYNC-05 | 不开放未验证的动作／同步；范围可按已通过能力控制。 |
| G5 产品联验 | 完整规格 AC-01 至 AC-20、OPS-01 至 OPS-04 | 不宣布全链路或生产验收完成。 |

金额、套餐、退款和保留等上线配置必须完成发布校验；未设置的配置不能被“测试通过”默认补齐。

## 4. OpenMeter 能力实验

当前 OM-01 至 OM-10 均为 blocked-env：本轮没有可用 OpenMeter 服务或商户凭据，未发送业务请求。候选模型记录在 [selected-model.json](../../../deploy/openmeter/contract-cases/selected-model.json)，仅作阻断记录；`scripts/saas/gate.py` 会拒绝它，直到获得 OM-01 至 OM-09 的真实运行证据。时间精度、结算确认和退款链路保持 unverified，不据此开放适配器。

| ID | 目标与准备 | 实验步骤及通过标准 | 关联 |
| --- | --- | --- | --- |
| OM-01 | 固定版本与可用性 | 核对运行版本／镜像 digest；直连只读查询。保存配置摘要和响应；四个 GET 的响应只能证明对应路径可达。 | G0 |
| OM-02 | 唯一商业模型 | 分别在隔离 Customer 下验证 V2 metered entitlement 与 V3 Credits 的发放、消费、余额、到期和订阅关联；列出差异，选择一套权威并写适配决策。禁止同笔充值双发。 | 规格 3.1 |
| OM-03 | Customer 与目录 | 并发创建同 tenant 映射、响应丢失重试、版本发布重放；最终一映射、一个目标版本，可查询真实外部 ID。 | B01/B21 |
| OM-04 | 付款后幂等发放 | 单个订单行重复请求、外部成功后本地崩溃；只能有一份商业权益。若外部无幂等／可唯一查询机制，必须给出并验证无重复策略，否则 fail。 | B02/B15 |
| OM-05 | 月度 reset 与充值保留 | 两种来源批次、跨两个月、先消耗后 reset；月额度失效、充值只保留剩余、不恢复已用值；自动发放与调度不重复。 | B03/B04 |
| OM-06 | 到期顺序与时间边界 | 同／异到期、同创建时间、月末、闰年、分钟内生效／reset、迟报；实际 burn-down 与产品顺序一致，确认时间与数值精度。V2 的文档结论不能直接套 V3。 | B04/AC-06/08 |
| OM-07 | 套餐变更 | 升级、连续升级、延迟履约、已有未来续费、降级和到期；外部订阅与产品权益区间一致，不额外收费或重发月额度。 | B05/B16/B17 |
| OM-08 | 部分退款与撤回 | 消费部分额度后退剩余的一部分；并发使用、未知消费、已过期、整批 void、撤回响应丢失分别测；可精确减少可退权益并保留其他批次。 | B18/B19 |
| OM-09 | 订单与账单衔接 | 比较 External Invoicing 及选定模型的充值结算路径；支付状态只确认一次，Invoice／订单／Grant 映射可回溯，不双收或双发。 | 规格 7.2 |
| OM-10 | 准入与消费交接 | 查清是否有满足需求的原子接口；若无，验证平台协调器的批次投影、水位、过期处理、并发、退款锁定及远端变化检测。不能仅“余额够就放行”。 | B22/B23/G2 |

参考文档：[Grant](https://openmeter.io/docs/billing/entitlements/grant) 支持核验到期、排序、reset 与分钟粒度；[External Invoicing](https://openmeter.io/docs/integrations/external-invoicing/overview) 支持核验外部支付状态同步方向。这些只作为实验输入。

## 5. 商业命令、权限与幂等

以下为产品逻辑接口验证，不是已实现路由；状态均 pending。

| ID | 接口／场景 | 必须通过的断言 |
| --- | --- | --- |
| COM-01 | BillingAccount 与权限 | 服务端绑定 tenant；Owner／账单管理员允许商业命令，未授权 Admin 拒绝；跨空间对象 ID、停用成员、撤权后重试均拒绝。 |
| COM-02 | Catalog.Publish / Quote | 版本不可追溯修改；无基础档／非法限额／无法表达精度不得发布；报价旧版本或过期返回明确冲突。 |
| COM-03 | Order / PaymentAttempt | 重复命令只建一订单；切换支付渠道不重复履约；多次成功付款识别多收款，不能重复发权益。 |
| COM-04 | paid → fulfilled | 回调持久化后工作进程崩溃、Outbox 重放、外部返回丢失均可恢复；用户界面不把 paid 显示为已到账。 |
| COM-05 | 退款申请与批准 | 申请与审批权限分离；审核时重新核算已用／预占／已退款；两个并发退款不能锁定同一份额度。 |
| COM-06 | 退款结果未知 | 稳定退款键查询、已失败解锁、unknown 保持锁定、成功后撤权失败可恢复；没有重复出款。 |
| COM-07 | 周期与资源限制 | 续费、升级差额、降级、超限写入并发、基础档保留数据与未到期充值均满足规格；充值不修改资源上限。 |

## 6. 微信支付独立检查

当前为 pending，尚未取得真实商户配置。候选官方入口：[Native 指引](https://pay.wechatpay.cn/doc/v3/merchant/4012791891)、[支付成功通知](https://pay.wechatpay.cn/doc/v3/merchant/4012791882)。具体客户端支付产品在 WX-01 固定后填写真实方法／路径，不把 Native 视为全部客户端的已批准方案。

| ID | 接口组 | 验证动作与通过标准 |
| --- | --- | --- |
| WX-01 | 产品、下单、关闭 | 确认商户／应用与客户端支付产品；固定版本、金额单位、notify_url、超时和关单；下单响应丢失查原单，不能新键重下。 |
| WX-02 | 支付回调 | 验证原始请求签名与证书／公钥轮换、按接口要求解密；校验商户、应用、订单、金额、币种、成功状态；伪造／重放不重复履约。 |
| WX-03 | 查单与竞态 | 漏通知、乱序、关单后晚成功、多个 attempt 成功；以可信结果入账一次，多收款进入人工处理。 |
| WX-04 | 申请退款／查询／通知 | 全额与部分、相同退款键重试、渠道受理未完成、未知结果、成功但撤权失败；不把受理当退款成功。 |
| WX-05 | 真实受控联验 | 指定测试商户／订单额度并获授权后，核验真实付款→权益→部分使用→可退部分→原路退款，保留脱敏渠道结果和对账。 |

## 7. 支付宝独立检查

当前为 pending。官方 [查单入口](https://opendocs.alipay.com/apis/api_1/alipay.trade.query) 与 [退款入口](https://opendocs.alipay.com/apis/api_1/alipay.trade.refund) 本轮无可读正文，方法名只是待核对候选，签名、字段和状态必须按实际选定支付产品逐项确认。

| ID | 接口组 | 验证动作与通过标准 |
| --- | --- | --- |
| ALI-01 | 下单、关闭与产品 | 确认网页／移动等支付产品、app 与商户身份、金额格式、SDK/API 版本；整数分与渠道金额转换无误差，不使用浮点。 |
| ALI-02 | 异步通知与验签 | 按官方规则验证签名、应用、卖家、订单、金额和成功状态；同步返回页不确认付款。重复及伪造通知不改变权益。 |
| ALI-03 | alipay.trade.query 候选查单 | 核实 API 与实际成功／关闭／等待状态映射；漏通知、乱序和关闭竞态不丢付款。不得复用微信状态解释。 |
| ALI-04 | alipay.trade.refund 与退款查询 | 核实部分退款标识及查询方式；稳定键重试、受理与完成、unknown 和撤权补偿分别验证。 |
| ALI-05 | 真实受控联验 | 与 WX-05 同样独立走完实际链路并核对渠道记录；微信成功不能代替该项。 |

## 8. 预算与用量

全部 pending。并发测试不仅检查 HTTP 响应，还检查最终实际派发数、总获批额度、最终远端消费与释放记录。

| ID | 场景 | 必须通过的断言 |
| --- | --- | --- |
| BUD-01 | 同空间并发预占 | 余额有限，多个进程同时申请；总获准额度不超过可验证下界，不能两个任务花同一份余额。 |
| BUD-02 | 父子预算 | 多子 Runtime 及 Connector 同时消费，同一父预算不能复制；真正超额停止新增派发。 |
| BUD-03 | 追加 | 有权限、在预算政策范围、余额足够才追加；重复追加键不放大上限；写操作审批仍独立生效。 |
| BUD-04 | TTL 与崩溃 | 未派发占用可释放；已派发未知不因租约过期释放；恢复者 fence 掉旧工作进程。 |
| BUD-05 | 结算交接窗口 | 模拟本地 usage 落地后远端聚合延迟，余额多次查询／重启后不能重复准入同一笔待确认消费；外部确认后只移交一次。 |
| BUD-06 | 到期与退款交叉 | 占用跨有效期、退款同时锁定、迟报、使用区间跨月；不延长充值有效期、不把已用再退。 |
| BUD-07 | 外部不可用／数据陈旧 | 无法建立安全下界时拒绝新收费操作；已经发生的使用保留并最终结算，不因为停机漏计。 |
| BUD-08 | 可执行费用上界 | 模型输出、沙箱时长、解析数量、重试次数等限制确实作用到所有出站路径；不受控路径不能宣称硬预算保证。 |
| USE-01 | 资金来源 | 平台模型／BYOK／混合任务逐调用判定；客户端伪造 BYOK 无效，失败不自动切收费模型。 |
| USE-02 | 主子去重 | 主 tRPC 与 OpenCode 同一子调用摘要不二次消费；新真实重试有独立 attempt。 |
| USE-03 | 流式与重复 | partial/final/cumulative、重复 Outbox、不同传输事件 ID 对应同一调用均不重复汇总。 |
| USE-04 | 缺失与修正 | 超时缺失保留 unknown，后续更正有版本关联；负数冲正或补偿必须走已验证接口。 |
| USE-05 | 定点与价格 | 固定价格版本、单位转换与舍入，Token meter 与 Credits 消费不双扣；边界数量不溢出。 |
| USE-06 | 失败／取消与实际成本 | 已实际计费使用保留，未用占用释放；超出预估进入异常核对，不无授权扩大预算。 |

## 9. 安装、连接、MCP 与 HTTP

全部 pending。

| ID | 接口／场景 | 必须通过的断言 |
| --- | --- | --- |
| CON-01 | Installation | 同 App 多空间安装独立；版本升级权限增加需重新授权，停用后新执行拒绝。 |
| CON-02 | Connection 类型 | 同安装多连接；个人本人／代理任务可用，其他成员不可用；团队同步仅空间连接。 |
| CON-03 | OAuth | state、回调、一次性、空间／成员绑定及提供方所需 PKCE；跨空间替换与重放失败。 |
| CON-04 | 凭据生命周期 | refresh 竞态、密钥轮换、撤销与成员离开正确；日志、提示词、工具结果无原始凭据。 |
| CON-05 | 审批绑定 | 改参数、目标、连接、版本使旧批准无效；预算批准不替代外部写批准。 |
| CON-06 | 预授权 | 目标、动作、期限、次数、金额边界并发原子生效；对外发送不因一般写授权放行。 |
| CON-07 | MCP | 固定协议／传输／工具 schema；外部只读提示不可信；工具列表变化不得扩大授权，stdio 进程隔离。 |
| CON-08 | 受控 HTTP | 方法／目标／认证头受控，重定向及解析后内部地址拒绝；模型不能指定任意出站目标。 |

## 10. 飞书发送消息

官方 [发送消息入口](https://open.feishu.cn/document/server-docs/im-v1/message/create) 本轮为无可读正文的动态页；当前均 pending，不能宣称已核验具体 UUID 窗口或字段。

| ID | 检查 | 通过标准 |
| --- | --- | --- |
| FS-01 | API 与连接权限 | 填入官方方法／完整路径、API 版本、令牌类型、发送 scope、目标会话准入条件和消息类型；读同步凭据不得假定有发送权限。 |
| FS-02 | 参数与审批 | 审批的连接、会话、类型和内容与实际请求一致；发送前撤权则拒绝；默认每次发送审批。 |
| FS-03 | 幂等与未知结果 | 核验提供方幂等字段、窗口和冲突行为；断线重启与超时不重复发；超出可靠范围进入 unknown 核对。 |
| FS-04 | 受控真实执行 | 用户指定测试会话与内容后发送，保存实际成功标识和目标核对；清理也按外部权限执行，不默认撤回消息。 |

## 11. Notion 创建页面

[官方创建页面入口](https://developers.notion.com/reference/post-page) 的官方索引说明写入目标需要内容插入能力；本轮完整正文抓取失败，以下均 pending。

| ID | 检查 | 通过标准 |
| --- | --- | --- |
| NO-01 | API 与父页面授权 | 固定 Notion-Version、创建接口、父页面类型、内容写入能力、分享／授权范围、内容大小限制。首期父页面，不隐式扩展所有数据库操作。 |
| NO-02 | 创建与预授权 | 指定父页面可审批或有限预授权；不同父页面、内容变化、权限撤销无法复用旧授权。 |
| NO-03 | 部分完成与未知 | 创建成功但后续内容追加失败时显示已创建页面和剩余步骤；未证明幂等时不重建；核实查询能否可靠定位未知创建结果。 |
| NO-04 | 受控真实执行 | 用户指定测试父页面和内容后执行，保存实际 page ID、父页面与内容核对；不以 mock 结果宣布真实可用。 |

## 12. 同步与运营恢复

全部 pending。

| ID | 场景 | 必须通过的断言 |
| --- | --- | --- |
| SYNC-01 | 飞书全量与增量 | 源资源选择、导入内容和游标可恢复；增量不重复建条目，结果关联空间。 |
| SYNC-02 | Notion 全量与增量 | 独立验证适配语义、分页和权限变化；不以飞书测试代替。 |
| SYNC-03 | checkpoint 崩溃 | 内容持久化前／后、checkpoint 前／后分别中断；不漏文档，重复处理可追踪且不虚构未发生的费用。 |
| SYNC-04 | 租约与暂停 | 老工作进程不能覆盖新游标；套餐、预算、连接或配额暂停有具体原因，恢复不清空进度。 |
| SYNC-05 | 删除与迁移 | 源删除、权限拒绝、暂时错误分开；默认保留策略明确，旧数据源不误删，连接归属不明要求重新授权。 |
| OPS-01 | 对账队列 | paid 未发放、退款撤权失败、用量待确认、余额差异和 unknown 写操作均可定位与幂等恢复。 |
| OPS-02 | 数据与审计保留 | 注销停新消费但不丢历史付款和结算；未定保留期不启动自动删数据；敏感字段不入审计。 |
| OPS-03 | 开关与回退 | 停新购买／收费／Action 后，回调、查单、退款、历史结算恢复仍运行；不重复投递消费。 |
| OPS-04 | 用户界面与通知 | 正确区分 paid/fulfilled、占用/可用、退款处理中、等待预算、等待写审批、unknown；告警关联业务对象而非暴露凭据。 |

## 13. 验证收口模板

每个项目更新状态时填写：

| 字段 | 必填内容 |
| --- | --- |
| check_id / status | 本文 ID，pass/fail/blocked-env 等真实状态。 |
| version / environment | 产品提交、外部 schema/SDK 版本、镜像 digest、配置版本、隔离对象。 |
| request / fault_point | 脱敏请求或脚本引用、注入故障点与并发方式。 |
| expected / observed | 通过标准与实际结果，不能只写“请求成功”。 |
| evidence | 响应、最终账本／外部对象、用量／授权／Outbox 关联与断言。 |
| limits / next_action | 未覆盖范围、失败原因、下一步及是否阻止对应发布门槛。 |

执行顺序：先 G0，再在同一模型下验证 G1/G2；渠道和 Connector 可按独立环境准备，但不提前宣称联验完成。实施计划必须引用这些 ID，禁止用旧本地 OpenMeter fork 的测试或一个静态 schema 替代实际证据。
