---
status: accepted
---

# Lago 作为商业计费权威，WeKnora 保留同步预算协调

WeKnora 采用自托管 Lago Community 替代 OpenMeter，并让 Lago 成为 Customer、不可变套餐版本、Subscription、Entitlement、Wallet、Credits、最终用量计价、Invoice、Payment 商业状态和 Credit Note 的权威。WeKnora 保留微信／支付宝渠道事实、Task Budget、调用前原子预占、不可变用量事实、Outbox、计费投影和恢复协调；这些本地状态不能形成第二个可独立改账的钱包。选择这一边界是为了使用 Lago 更完整的订阅与账单模型，同时保留 AI 执行在真实调用前必须同步阻止透支的产品不变量，因为 Lago 的异步 Event／Wallet 接口没有公开的原子 reservation 或逐事件计价完成回执。

## Considered Options

- 只把现有 OpenMeter Credits Gateway 替换为 Lago：范围较小，但无法实现已选择的套餐、订阅和账单统一目标。
- 将支付渠道和 Task Budget 也交给 Lago：Lago 没有已验证的微信／支付宝接入，也不提供所需的同步预占语义。
- 由 WeKnora 继续作为 Credits 与最终计价权威：会保留双账本，并使 Lago 退化为展示或记录系统。
- 维护 Lago fork 补充逐事件计价回执：首期运维、升级和 AGPL 合规成本过高；只有原生对象与协调层无法通过已批准能力门槛时才重新决策。

## Consequences

- 已发布套餐必须用新的 Lago plan code 表达不可变版本，禁止在 Lago UI 原地修改。
- WeKnora 必须按 Task／结算批次协调异步计价，并对价格模型施加“可计算可信上界”的首期限制。
- 外部支付激活、Credits 到期顺序、Pricing Group 规模和 Community 功能均是生产上线前的真实能力门槛。
- Lago 从 Apache-2.0 的 OpenMeter 切换为 AGPL-3.0，生产使用和任何修改都必须经过许可证审查。
- Lago 一旦承载真实商业数据，回退只能是前向修复，不能切回 OpenMeter 回滚账本。

## 修订记录

### 2026-09-29：收费维度归属与 R-3 Charges 边界（Issue #72/#87 用户裁决）

**裁决**：每个已发布 Plan Version 的显式 Billable Metric→计费维度映射，确定该收费维度对应的唯一计费订阅与不可变套餐版本。Base 与付费订阅对同一维度重叠，或多个订阅重复声明同一维度时，对受影响维度 fail closed，禁止新付费调用，直至配置消除歧义；其他存在唯一有效 owner 的维度可以继续。不得依据订阅新旧、Base/付费偏好或价格混合推断 owner。待处理、取消、未激活、陈旧、不可读或其他不可用的权威价格继续 fail closed。

**付款约束**：保留 `PurchaseService.ensureNoCharges`，首期付费套餐购买仅支持不含 Lago Usage Charge 的既有单行订阅费路径。只有另行批准的 R-3 相容契约明确金额、币种、购买金额中排除 Usage 行及付款时对 finalized Invoice 的完整比对后，才可重新考虑 Charges 路径。

**取舍与后果**：歧义会阻断该维度的新付费操作，降低可用性，但避免错选订阅、错价和不可审计的收费；明确映射后恢复该维度。此裁决关闭 #87 业务规则歧义，不代表代码已实现，也不解除 #86 集成/验收和认证 Lago v1.53 契约证据门槛。

**来源**：用户于 2026-09-29 在 Issue #72 执行对话中批准维度归属 fail closed，并选择继续排除 Charges。

### 2026-09-28：Base 与付费订阅分别定价（Issue #72/#87 用户裁决）

**裁决**：Base 订阅与付费订阅是独立的计费订阅，各自保留自己的价格；用户选择“Base 与付费订阅分别定价”不改变既有权益组合规则。一个收费维度不得把不同订阅的费率相加或混合。具体收费维度由哪条订阅计价、重复维度如何裁决，以及付费计划 Usage Charge 是否满足 R-3 Quote/Invoice 单行约束，仍须在实现前明确；无唯一有效价格时 fail closed。此裁决由用户在 Issue #72 执行中明确选择，并由 Spec 同日修订记录。

**取舍与后果**：这取代升级/降级/返回 Base 全部沿用同一外部 subscription identity 的既有要求；付费订阅内部版本转换语义仍维持原规则。后续实现必须为 billable dimension 提供明确的 owning subscription/Plan Version 解析，不能靠“最新订阅”或价格混合猜测。`PurchaseService.ensureNoCharges` 是否与新价格身份相容仍受已批准的 R-3 Invoice 约束限制，须经独立规格审查后再改；不得把“分别定价”扩大解释成新增收费行的批准。

### 2026-09-23：澄清充值批次（top-up）的钱包写入时机与并发模型（75-a2 裁决）

**背景**：Lago 实验栈（#75/T03）实测发现 Lago Community 对同时活跃的 wallet 批次存在并发保护——直接并发创建第 7 个活跃批次时返回 422（verdict a2 BLOCKED）。该限制若被理解为产品级充值并发上限，将迫使 WeKnora 设置人为闸门（活跃充值批次 ≤5）。

**裁决（spec/ADR owner 2026-09-23）**：采用协调层承载并发，不设产品级充值并发上限；本 ADR 的权威边界不变，仅澄清 Wallet 写入时机语义。

> **出处注记（2026-09-23 补录）**：本裁决经当日编排任务指令获用户确认（指令原文要点：『75-a2 已裁决为选项 B（充值批次状态机由 WeKnora 协调层承载、Lago Wallet 交易仅在到账时刻幂等创建）』）。此前 `docs/plans/issue-72-final-report.md` §9.3 曾记录本修订（提交 `552d98d12`）的裁决出处无法从过程文档溯源、效力待用户确认；该记录早于本确认到达，已被取代。本注记由 issue-72 架构师补录以闭合溯源缺口（DAG 审查 2026-09-23 high finding），未改动裁决内容本身。

**决策内容**：

- 充值批次（top-up batch）的完整状态机——创建、渠道下单、支付回执确认、失败与超时处理——由 WeKnora 协调层承载；与“WeKnora 保留微信／支付宝渠道事实”的既有边界一致，不新增本地账本能力。
- Lago Wallet 交易（Credits 到账）只在“付款确认到账”时刻创建，且必须幂等（幂等键绑定渠道支付单号），一次性完成；Lago 侧不存在长期“进行中”的充值批次。
- 因此 Lago Community 的活跃批次并发保护不构成产品级约束：正常产品流中 Lago 侧批次创建是到账时刻的瞬时动作。若未来出现瞬时批量到账（如营销批量发放），由协调层排队串行写入，不触达该限制。
- 权威边界不变：Lago 仍是 Credits 余额、到账事实与消费顺序的权威；WeKnora 协调层状态（渠道支付单、付款事实）是商业输入与恢复依据，不得独立发放或扣减 Credits。

**代价**：若实测发现 Lago 单实例瞬时串行写入能力不足以支撑高峰到账（当前无证据），需重新评估；协调层排队增加毫秒级到账确认延迟，非产品可感知。
