# SaaS 商业恢复手册（rollout 暂停期运维）

适用范围：商业链路（订单/履约/退款/结算/连接器）在灰度或回滚暂停期间的运营恢复。核心不变量：

1. **暂停只挡新消费**。`AllowDuringRollback` 仅放行 `payment_callback` / `refund_query` / `fulfillment_replay` / `settlement`；其余操作（`new_order`、`new_dispatch`、`connector_action`、`refund_create` 及一切未知写操作）一律拒绝。
2. **重放只认原始幂等键**。Replay 复用原 event_key，绝不生成新键，存储层 UNIQUE 兜底，重复投递不可能放大。
3. **未知写操作只允许核对**。unknown_action 类目只做查询/对账，绝不直接重派。
4. **不直接改余额**。恢复服务不触碰余额字段；结算数学一律走既有 budget/settlement 路径。
5. **已撤销连接不得因重放恢复**。对象状态校验发现 `revoked` 即拒绝（`recovery_connection_revoked`）。
6. **可回放的操作需平台运营能力**。能力检查未接线时 fail-closed（`recovery_operator_check_missing`）；每次重放（成功与失败）都写审计（执行人 + 结果）。

## 收费开关（回滚专用）

| 开关 | 配置键 | 环境变量 | 默认 |
| --- | --- | --- | --- |
| 新订单 | `commercial_new_orders` | `WEKNORA_COMMERCIAL_NEW_ORDERS` | 开 |
| 新派发 | `commercial_new_dispatch` | `WEKNORA_COMMERCIAL_NEW_DISPATCH` | 开 |
| 连接器新动作 | `connector_new_actions` | `WEKNORA_CONNECTOR_NEW_ACTIONS` | 开 |

默认全部**开启**（safe-on）：关闭是回滚动作，只能显式进行（配置写 `false` 或设环境变量）。开关只影响各自的“新”动作；回调/查询/结算照常。开关为全局默认值，按空间覆盖由存储层承载。

## 恢复队列类目（逐状态）

队列条目展示：类目、年龄、空间（tenant）、业务 ID、状态、尝试次数。

| 类目 | 查询路径 | 正常结果 | 异常结果 | 允许动作 |
| --- | --- | --- | --- | --- |
| `paid_unfulfilled`（已付未履约） | 按业务 ID 查订单 + outbox 事件（kind=payment_callback/fulfillment_replay，state≠sent） | 订单 paid、事件 pending → 重放 `fulfillment_replay` 后变 sent，权益到账 | 事件 dead 多次 → 检查渠道回调原始报文；订单未 paid → 转人工 | payment_callback / fulfillment_replay 重放 |
| `refund_unknown`（退款结果未知） | 用**原**退款键查渠道 `QueryRefund`（kind=refund_query，state=pending） | 查得成功/失败终态 → 走 C05 状态机收敛 | 渠道仍 unknown → 保持排队，禁止重新创建退款 | refund_query（查询/对账） |
| `revocation_pending`（撤销挂起） | 查退款行 state=revocation_pending | 精确积分撤销重试成功 → completed | 撤销再失败 → 保持挂起；**不得**再次打款 | 重试撤销（不重复支付） |
| `usage_unconfirmed`（用量未确认） | 查结算事件（kind=settlement，state≠sent）与预留行 | 渠道确认后结算落账，事件变 sent | 未确认 → 保留预留（`reconcile_needs_confirmation`），不得清零 | settlement 重放 |
| `balance_discrepancy`（余额差异） | 从源记录（订单/结算/预留）重算对账 | 差异可解释（在途）→ 关闭 | 真差异 → 冻结相关预留并升级，**禁止手改余额** | 只读对账；修复走专用结算路径 |
| `unknown_action`（未知动作） | 人工鉴定 kind 与 payload | 确认为已知类目后按其路径处理 | 无法鉴定 → 保持排队并升级 | **仅核对/查询，绝不重派** |

## 重放（Replay）规则

- 入口：`RecoveryService.Replay(ctx, operationID, actor)`。
- 前置：平台运营能力检查（未接线即拒绝）；对象版本 + 当前状态校验（无校验器即拒绝，fail-closed）。
- 已 sent 的事件拒绝重放（`recovery_already_delivered`）；不在放行表的操作拒绝重放（`recovery_query_only`）。
- 审计表 `commercial_recovery_audit` 记录 operation_id、kind、actor、result、detail；失败也留痕。

## 告警阈值与日志脱敏

- 指标：逐类目计数 + 年龄分位（P50/P90/P99/最大年龄），纯结构 `RecoveryCategoryMetrics`。
- 阈值：`RecoveryAlertThresholds{MaxCount, MaxAgeP90, MaxAge}`；至少一项为正（全零配置视为无效，拒绝），任一越界即告警。告警文案必须带业务 ID（订单号/退款号/事件键）与空间，便于定位。
- 日志：恢复相关日志经 `RedactForLog` —— credential 类键（token/secret/password/authorization/api_key/private/signature/session 等，含嵌套）值替换为 `[REDACTED]`，超长值截断（512 字符 + 截断标记）。**注意**：真实告警/指标端点未在本任务环境运行（blocked-env），以上为配置与代码级证据。
