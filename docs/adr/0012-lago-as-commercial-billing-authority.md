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
