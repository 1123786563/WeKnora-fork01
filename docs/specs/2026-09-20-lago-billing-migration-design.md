# Lago 计费平台迁移

日期：2026-09-20  
状态：已批准  
关联：[领域词汇](../../CONTEXT.md)、[ADR-0012](../adr/0012-lago-as-commercial-billing-authority.md)

## Problem Statement

WeKnora 需要让每个空间能够可靠地购买套餐和 Credits，并让模型、解析、沙箱及 Connector 等收费执行具备明确的权益、预算、用量、计价、账单、付款、退款和恢复语义。当前 OpenMeter 接入没有形成完整的商业计费平台：WeKnora 本地同时承担套餐、价格、订阅投影、Credits 批次、支付、退款、预算和最终计价，外部适配只覆盖少量 Grant／Settlement 调用，而且实际适配器与记录的 OpenMeter 契约存在方法及路径偏差。

从空间成员的视角，现状无法证明以下承诺能够在真实计费后端中端到端成立：付款只履约一次、套餐和充值额度按正确顺序消费、两个并发 Task 不会花同一份 Credits、异步计价不会提前释放额度、退款成功不会留下仍可消费的权益，以及计费系统故障后不会丢付款或用量。

从运营和工程视角，当前边界还会形成双账本风险：本地记录与外部计费系统都可能被理解成套餐、余额或消费权威。若直接把 OpenMeter 调用替换成 Lago 调用，这些歧义不会消失，反而会扩展到 Customer、Plan、Subscription、Entitlement、Wallet、Invoice、Payment 和 Credit Note。

本功能需要以 Lago Community 取代 OpenMeter，并重新确立商业权威、同步准入与异步计价之间的关系。迁移必须保留现有产品不变量，不得通过弱化 Credits 到期、预算、退款或 BYOK 规则来适配供应商能力。

## Solution

WeKnora 将自托管 Lago Community 作为商业计费权威。Lago 权威管理 Customer、不可变套餐版本、Subscription、Entitlement、Wallet、Credits、最终用量计价、客户账单、Payment 商业状态和 Credit Note。

WeKnora 保留空间身份与账单权限、面向用户的 Billing API、付款前报价、微信／支付宝渠道集成、渠道支付单、付款事实、渠道退款事实、Task Budget、调用前原子额度预占、不可变用量事实、退款锁定、Outbox、计费投影和异常恢复。WeKnora 的投影与协调状态不能独立发放、扣减或撤回 Credits，也不能形成第二个商业账本。

客户端始终调用 provider-neutral 的 WeKnora Billing API。Lago API、内部对象、凭据和状态枚举不会成为 Web 或移动端契约。WeKnora 通过一个深的 Commercial Platform module 隔离供应商：调用者只表达商业命令、读取商业快照和请求对账，不需要理解 Lago 的对象编排、Webhook、异步 worker 或恢复协议。

收费执行继续由 WeKnora 同步准入。每次真实调用前，系统按照 Lago 已发布价格的只读投影计算可信费用上界，并在 Task Budget 与空间保守可用余额中原子预占。实际用量进入 Lago 后，按 Task／结算批次和 Pricing Group 读取可关联的权威聚合计价结果，再结算或释放预占。Lago 接收事件不等于完成计价；没有可靠计价结果时，预占保持为结算待核对。

迁移只处理开发数据，不迁移 OpenMeter 测试数据，不长期双写。Lago 一旦开始承载真实商业数据，系统只允许前向修复，不能依靠切回 OpenMeter 回滚账本。

## User Stories

1. As a 空间 Owner, I want each space to have an independent Billing Account, so that another space cannot share or consume its subscription or Credits.
2. As a 空间 Owner, I want to see a fixed Quote before payment, so that the product version, price, currency, entitlements, and validity period are clear.
3. As a 空间 Owner, I want the Quote to match the Lago Invoice exactly, so that I am never charged an amount different from what I approved.
4. As a 空间 Owner, I want to pay through WeKnora using WeChat Pay or Alipay, so that the billing-backend migration does not replace the familiar payment experience.
5. As a 空间 Owner, I want payment success and entitlement activation to be shown as separate states, so that I know when money has been received but fulfillment is still processing.
6. As a 空间 Owner, I want a repeated payment callback to be harmless, so that retries never grant a second subscription or duplicate Credits.
7. As a 空间 Owner, I want mismatched, partial, or wrong-currency payments to enter review without activating benefits, so that payment anomalies cannot expand entitlements.
8. As a 空间 Owner, I want multiple successful payment attempts to fulfill once and refund the excess, so that duplicate money receipt does not duplicate delivery.
9. As a 空间 Owner, I want paid top-up Credits to become spendable only after Lago confirms them, so that pending fulfillment is not mistaken for an available balance.
10. As a 空间 Owner, I want annual plans to issue included Credits monthly, so that annual payment does not grant a full year of consumption upfront.
11. As a 空间 Owner, I want plan upgrades to take effect immediately, so that newly purchased features are available after successful payment and activation.
12. As a 空间 Owner, I want an upgrade to grant only the prorated difference in current-period Credits, so that already issued or consumed Credits are not granted again.
13. As a 空间 Owner, I want downgrades to take effect at the end of the paid period, so that already purchased access is not removed early.
14. As a 空间 Owner, I want expiry without renewal to move the space to the Base Plan, so that existing data and unexpired top-up Credits remain available under reduced limits.
15. As a 空间 Owner, I want top-up Credits to remain independent from Plan entitlements and Resource Quotas, so that buying Credits cannot unlock unrelated features.
16. As a 账单管理员, I want to purchase, renew, top up, change plans, inspect invoices, and request refunds within my delegated role, so that ordinary space administrators do not automatically receive financial authority.
17. As a 账单管理员, I want every commercial command to be audited with actor, reason, idempotency identity, and external receipt, so that adjustments are explainable and recoverable.
18. As a 账单管理员, I want current balance, reserved Credits, refund-locked Credits, and reconciliation age shown separately, so that available funds are not confused with money already committed.
19. As a Task Owner, I want to set a finite Task Budget, so that a Task and all delegated work cannot consume unlimited Credits.
20. As a Task Owner, I want child Agents and Connectors to share the parent Task Budget, so that delegation cannot duplicate spending authority.
21. As a Task Owner, I want the system to reserve a conservative upper bound before a real paid call, so that concurrent calls cannot spend the same Credits.
22. As a Task Owner, I want an accepted Lago event to retain its reservation until rating is reconcilable, so that asynchronous ingestion is not mistaken for final settlement.
23. As a Task Owner, I want only the affected Task paused when its rating exceeds the allowed delay, so that unrelated Tasks with safe budgets can continue.
24. As a Task Owner, I want a Task paused whenever actual cost exceeds its reserved upper bound, so that an invalid admission-price projection is surfaced even when the space still has other Credits.
25. As a Task Owner, I want actual usage recorded after failure or cancellation, so that already incurred provider cost is not erased.
26. As a Task Owner, I want unknown call outcomes to keep their reservations, so that timeouts cannot make the same Credits available twice.
27. As a Task Owner using BYOK, I want model Credits excluded while platform parsing, sandbox, and Connector costs remain billable, so that the funding source is applied per service dimension.
28. As a 普通成员, I want an explicit “waiting for billing synchronization” state, so that a paused paid action is distinguishable from an application failure.
29. As a 普通成员, I want view, export, and cleanup operations to remain available during quota or billing outages, so that commercial controls do not trap existing data.
30. As a 普通成员, I want Resource Quota overage to block only growth, so that downgrade or expiry does not delete existing members, files, or other resources.
31. As a 财务运营人员, I want Customer Invoices to remain distinct from Chinese Tax Invoices, so that a Lago Invoice is not presented as a statutory tax document.
32. As a 财务运营人员, I want refund eligibility recalculated from authoritative Wallet, Invoice, Payment, and Credit Note state, so that consumed or previously refunded value cannot be refunded again.
33. As a 财务运营人员, I want refundable Credits locked before the channel refund starts, so that the same value cannot be consumed while money is being returned.
34. As a 财务运营人员, I want channel refund and Lago revocation to be separate recoverable states, so that a successful cash refund cannot leave spendable Credits.
35. As a 财务运营人员, I want duplicate refund commands to reuse a stable refund identity, so that retries cannot produce multiple payouts or multiple revocations.
36. As a 财务运营人员, I want finalized billing corrections represented by Credit Notes or later invoices, so that financial history is not silently rewritten.
37. As a 套餐运营人员, I want every published Plan Version to be immutable, so that changes never retroactively alter existing subscriptions.
38. As a 套餐运营人员, I want a new Lago plan code for every new Plan Version, so that old subscriptions keep their purchased commercial definition.
39. As a 套餐运营人员, I want higher product tiers to have non-decreasing daily base prices, so that Lago upgrade and downgrade behavior matches the product hierarchy.
40. As a 套餐运营人员, I want promotions represented separately from base-tier price, so that a temporary discount cannot invert the plan hierarchy.
41. As a 套餐运营人员, I want only pricing models with a trustworthy admission upper bound enabled initially, so that no configuration can bypass Task Budget enforcement.
42. As a 套餐运营人员, I want published Lago Plans editable only through audited WeKnora commands, so that Lago UI changes cannot silently alter customer billing.
43. As a 平台运营人员, I want Webhook signatures and unique keys verified, so that forged or duplicate notifications cannot mutate product state.
44. As a 平台运营人员, I want Webhooks treated as change notifications followed by authoritative reads, so that out-of-order delivery cannot make projections move backward.
45. As a 平台运营人员, I want periodic reconciliation to recover lost Webhooks and exhausted retries, so that projections eventually converge without relying on perfect delivery.
46. As a 平台运营人员, I want stale projections and negative Lago ongoing balance to block new paid work, so that data uncertainty fails closed.
47. As a 平台运营人员, I want payment callbacks and occurred usage persisted while Lago is unavailable, so that external facts are not lost during an outage.
48. As a 平台运营人员, I want pending commercial commands replayed using the same idempotency identity, so that response loss never creates a second remote object.
49. As a 平台运营人员, I want billing queues, dead jobs, event errors, activation delay, reconciliation age, and revocation failures monitored, so that an available API is not mistaken for a healthy billing system.
50. As a 平台运营人员, I want a recovery exercise before launch, so that Customer, Subscription, Wallet, Invoice, Payment, pending work, and projections can be reconciled after restore.
51. As a 安全审计人员, I want Lago credentials accessible only to approved server modules, so that browsers, mobile apps, Agents, logs, and Task content never receive billing secrets.
52. As a 安全审计人员, I want Usage Events to exclude prompts, outputs, knowledge content, and Connector parameters, so that Lago receives only the minimum data required for billing.
53. As a 合规负责人, I want Lago AGPL-3.0 usage and modifications reviewed before production, so that deployment and source-availability obligations are understood.
54. As a 合规负责人, I want transaction history retained separately from removable display data, so that space deletion can de-identify a customer without destroying required financial evidence.
55. As a WeKnora developer, I want one provider-neutral commercial seam, so that Lago object orchestration and recovery rules are local rather than spread across product callers.
56. As a WeKnora developer, I want the public Billing API independent of Lago fields and statuses, so that clients do not need another rewrite if the billing provider changes.
57. As a WeKnora developer, I want real Lago Community contract evidence, so that OpenAPI schemas and HTTP mocks are not mistaken for supported runtime behavior.
58. As a WeKnora developer, I want immutable Usage Facts correlated with Lago transaction identities and Settlement Batches, so that retries, corrections, and invoices can be audited end to end.
59. As a WeKnora developer, I want a content conflict under the same idempotency identity rejected, so that retry safety cannot become silent overwriting.
60. As a WeKnora operator, I want exact Lago release tags and staged bridge upgrades, so that migrations are reproducible and reversible at the infrastructure level.
61. As a WeKnora operator, I want PostgreSQL as the initial event store, so that the first production topology does not add Kafka and ClickHouse before measured need.
62. As a WeKnora operator, I want RPO no greater than five minutes and RTO no greater than sixty minutes, so that the billing authority has explicit recovery objectives.
63. As a WeKnora operator, I want OpenMeter removed after the rollback window closes, so that the system does not preserve an accidental long-term dual-write path.

## Implementation Decisions

### Commercial authority and module seam

- Lago Community is the authority for Customer, published Plan Version, Subscription, Entitlement, Wallet, Credits, final rating, Customer Invoice, Payment commercial state, and Credit Note.
- WeKnora remains authoritative for Tenant identity, membership and billing roles, Quote, Channel Payment Order, Payment Fact, channel refund fact, Task Budget, Credit Reservation, Refund Lock, Usage Fact, and actual Resource Quota occupancy.
- The provider-neutral WeKnora Billing API is the primary behavior and acceptance seam. Clients never depend on Lago identifiers, credentials, URLs, field names, or status enums.
- The existing commercial gateway seam will be replaced rather than layered with a deep Commercial Platform module. Its interface has three cohesive operation families: submit typed commercial commands, read authoritative commercial snapshots, and reconcile from a durable cursor. Lago-specific object ordering, retries, Webhooks, workers, and recovery remain inside the Lago adapter.
- Commercial application callers and tests cross the same module interface. Internal adapters may exist for HTTP, persistence, clock, and payment providers, but they are not additional product-facing seams.
- A Tenant maps to one immutable Lago Customer. A WeKnora Deployment maps to one Lago Organization. Shared Organization remains a collaboration concept and never becomes a combined payer.
- The initial deployment supports one Billing Entity and CNY only. Monetary values use integer minor units and never pass through binary floating point.

### Catalog, subscriptions, and entitlements

- Every published Plan Version receives a distinct Lago plan code. A published version is immutable; price, Charge, Billable Metric, Entitlement, Resource Quota, and included Credits changes require a new version.
- Base subscription price is monotonic with the product tier. Discounts and promotions do not modify the tier ordering.
- Subscription continuity uses the same external subscription identity across upgrade, downgrade, and transition to the Base Plan.
- Upgrade becomes effective immediately after successful payment and Lago activation. WeKnora calculates only the approved prorated included-Credits difference and records it as an idempotent granted Wallet transaction in Lago.
- Downgrade becomes effective at the paid-period boundary. It does not reclaim already issued current-period Credits.
- Annual subscriptions issue included Credits monthly. Expiry without renewal moves to the Base Plan while preserving existing data and unexpired top-up Credits.
- Lago Entitlement defines purchased features and quota limits. WeKnora enforces current occupancy atomically and preserves view, export, and cleanup when growth is blocked.
- Top-up Credits change Wallet balance only; they do not change Entitlement or Resource Quota.

### Quotes, payments, and fulfillment

- WeKnora creates a short-lived Quote from an immutable Plan Version. Because Premium Invoice Preview is out of scope, initial pricing is limited to models that WeKnora can reproduce deterministically for pre-payment display.
- Before a Channel Payment Order is created, the Quote and actual Lago Invoice must match in Plan Version, currency, total, and line items. A mismatch aborts the purchase and creates no channel payment request.
- Initial subscriptions are pay-in-advance, have no trial, and use a payment activation rule. They remain incomplete until the gating payment succeeds.
- WeKnora owns WeChat Pay and Alipay request creation, callback verification, query, close, and refund behavior. A verified channel result produces an immutable Payment Fact.
- A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment.
- If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally.
- Paid Wallet top-up is not spendable until Lago confirms the Wallet transaction. Response loss is recovered with the original invoice, channel, and idempotency identities.
- Duplicate, mismatched, partial, wrong-currency, and multiple-success payment cases retain their external facts without automatically changing Invoice amount or delivered benefits.

### Credits and Task admission

- Included Credits expire at the end of their monthly period and do not roll over. Each top-up batch expires twelve calendar months after becoming effective.
- Credits consume globally by earliest expiry, then earliest grant time. Lago Wallet and transaction priorities may implement this order only after real runtime verification; inability to preserve this invariant blocks migration.
- Task Budget is a spending ceiling for a Task and all delegated work. It is not the Tenant Wallet balance.
- Before every paid external action, WeKnora atomically reserves a conservative upper bound against both Task Budget and the conservative Tenant available balance.
- The admission-price projection is a read-only projection of the same immutable Lago pricing version. Missing, stale, or non-bounded pricing blocks dispatch.
- Initial Charge models are limited to fixed unit, deterministic package, or explicitly capped models for which a trustworthy upper bound exists.
- Payment success, Lago fulfillment, and WeKnora paid-work admission remain independent states.

### Usage, rating, and reconciliation

- Every physical paid attempt produces an immutable Usage Fact with Tenant, Task, Run, call, attempt, revision, service dimension, funding source, occurrence time, status, and raw quantity.
- Every physical attempt and revision maps to a globally stable Lago transaction identity. A retry reuses the same identity and timestamp; a real retry or correction uses a new identity. Same identity with different content is a conflict.
- Lago event acceptance is durable-ingestion evidence only. It is not rating or settlement evidence.
- Usage is grouped by Task or a bounded time window into a Settlement Batch with its own Pricing Group. The batch is reconciled only when Lago current usage can be associated with that group and yields a determinate authoritative amount.
- Reservations remain held while rating is unavailable. Other Tasks may continue only when their own budgets and the conservative Tenant balance remain safe.
- A batch unresolved for five minutes pauses new paid actions for its Task. At fifteen minutes or on an event-processing error, it creates an operational alert.
- A confirmed actual amount greater than the pre-dispatch reservation always pauses the Task and requires reconciliation, even if the Tenant still has unrelated Wallet headroom. This rule was validated by the logic prototype and prevents an invalid admission upper bound from being silently accepted.
- Lago negative ongoing balance blocks all new paid actions for the affected Tenant. Already occurred usage remains recorded and billable.
- Usage Facts are never overwritten. Before invoice finalization, only a verified metric-specific compensating event may adjust usage. After finalization, corrections use a Credit Note or a later Invoice.

### Refunds

- Refund approval first creates a Refund Lock over the still-refundable benefit.
- Refund eligibility is recalculated from authoritative Lago Wallet, Invoice, Payment, and Credit Note state immediately before approval.
- Channel refund uses a stable refund identity. A channel-success result moves to revocation pending; it does not complete the refund.
- Lago then voids remaining Credits, adjusts Subscription, or creates a Credit Note as appropriate. The Refund Lock is released only after channel refund and Lago revocation are both confirmed.
- Channel success followed by Lago failure retains the lock and recovers forward. It never repays the channel a second time.

### Local persistence and state projection

- WeKnora retains Plan drafts, publish commands, Quote, Channel Payment Order, Payment Fact, channel refund fact, Task Budget, Credit Reservation, Refund Lock, Usage Fact, Lago event correlation, Settlement Batch, Outbox, Inbox, Webhook deduplication, reconciliation cursor, and audit records.
- Customer, Subscription, Entitlement, Wallet, Invoice, Payment, and Credit Note local records are rebuildable Billing Projections only.
- Existing local Order, Subscription, Credit Lot, rating, and balance tables must be migrated to these new meanings or retired. No code path may continue to mutate them as an independent commercial authority.
- Webhooks are signed change notifications. Consumers verify HMAC or JWT, deduplicate by unique key and business object version, then re-read the authoritative Lago object. Out-of-order notifications cannot move a projection backward.
- WeKnora-to-Lago commands use a durable Outbox and stable idempotency identity. Timeout and server error are indeterminate outcomes that must be queried before replay.

### Public product states

- WeKnora maps provider state into stable product states: awaiting payment, paid awaiting activation, active, fulfillment processing, waiting for billing synchronization, insufficient Credits, refund review, channel refund pending, revocation pending, and operator attention.
- The public Billing API never exposes Lago credentials, internal URLs, Lago Organization identifiers, or raw status enums.
- During a Lago outage, WeKnora stops new purchases, plan changes, top-ups, and paid dispatch. It continues to durably accept verified payment callbacks, already occurred usage, Outbox work, view, export, and cleanup.

### Security, privacy, licensing, and lifecycle

- Lago API credentials live only in server-side secret management. Browser, mobile, Agent context, logs, and generated artifacts cannot access them.
- Network policy allows only approved WeKnora server modules and controlled operations access to Lago.
- Usage metadata contains billing dimensions and correlation identities only. It excludes prompts, outputs, knowledge content, Connector parameters, and payment secrets.
- Space deletion stops new commerce, revokes spendable benefits, de-identifies removable presentation data, and retains legally required transaction history. Customer identities are never reused.
- Lago AGPL-3.0 use and any source modification require legal review before production. Premium feature controls cannot be bypassed.
- Local and CI environments use a pinned Lago Compose release. Production uses Helm with independently managed PostgreSQL, Sidekiq Redis, cache Redis, and object storage; API and major worker queues scale separately.
- PostgreSQL is the initial Lago event store. ClickHouse and Kafka require measured need and a separately approved migration.
- Production targets RPO no greater than five minutes and RTO no greater than sixty minutes. A staging restore and commercial reconciliation exercise is required before release.
- Lago upgrades pin an exact release, follow release notes and mandatory bridge releases, create a recovery point, and replay fixed commercial fixtures in staging.

### Migration and removal

- A separate Lago environment is created; OpenMeter databases, Kafka, ClickHouse, Redis, and test data are not reused or migrated.
- Synthetic shadow comparison may be used before cutover, but shadow Lago cannot change real benefits.
- There is no long-term dual-write mode.
- The default adapter changes only after Community capability, behavior, failure, performance, security, and recovery gates pass.
- The rollback window exists only before Lago carries real commercial data. After that point, failures are repaired forward.
- Once the rollback window closes, OpenMeter adapters, deployment assets, image locks, dedicated probes, environment variables, and write paths are removed. Historical evidence remains explicitly marked as deprecated.

## Testing Decisions

### Testing seams

- The primary acceptance seam is the provider-neutral WeKnora Billing API. Tests drive user-visible commands and assert product states, balances, entitlements, invoices, pauses, refunds, and audit outcomes without referring to Lago response shapes.
- The supplier seam is the Commercial Platform module. A deterministic fake adapter supports application behavior tests; the real Lago adapter runs the same contract against a pinned Lago Community environment.
- Task admission and reconciliation are tested through commercial application behavior with a real transactional database for concurrency. Tests do not reach into private repository steps to prove business outcomes.
- Provider callback verification remains tested through the payment-provider seam because signature, merchant identity, amount, currency, query, close, and refund semantics differ by channel.

### What makes a good test

- A test asserts an observable business invariant: one fulfillment, one spend, one refund, one state transition, one retained reservation, or one rejected unauthorized action.
- A test uses stable business identities and checks final authoritative and projected state, not only an HTTP status or mocked method call.
- A failure-injection test proves behavior after response loss, duplicate delivery, reordering, process crash, queue delay, or dependency outage.
- A concurrency test proves the total granted or reserved amount, not merely that each request returned a plausible response.
- A contract test records the pinned Lago release, request, response, final Lago objects, and known limitation. Schema inspection alone is not a pass.
- A UI/API test asserts stable WeKnora product states and never snapshots raw Lago fields.

### Behavior and contract matrix

1. **Community and license:** target Customer, Plan, Subscription, Entitlement, Wallet, Invoice, Payment, Credit Note, Webhook, and current-usage capabilities run in Community; AGPL review is complete.
2. **Tenant isolation:** a Tenant can observe and mutate only its mapped Customer, Subscription, Wallet, Invoice, Payment, events, and benefits.
3. **Immutable catalog:** a new Plan Version creates a new external plan; existing subscriptions remain on their purchased version.
4. **Quote match:** Quote and Invoice totals, currency, and line items match before a Channel Payment Order exists; mismatch produces no payment request.
5. **External payment activation:** a verified WeChat Pay or Alipay result reaches Lago through a supported integration and moves an incomplete subscription to active exactly once.
6. **Top-up fulfillment:** unpaid Credits are unavailable; successful payment credits exactly once; response loss recovers without a second Wallet transaction.
7. **Payment anomalies:** duplicate, partial, mismatched, wrong-currency, late, and multiple-success payments never duplicate benefits.
8. **Credits ordering:** monthly expiry, twelve-month top-up expiry, earliest-expiry consumption, grant-time tie-break, concurrent use, void, and refund satisfy the approved invariants.
9. **Usage identity:** duplicate delivery is idempotent; new attempts and revisions remain separate; conflicting content under one identity is rejected.
10. **Settlement Batch:** Task/Pricing Group current usage produces the expected amount and never masquerades as a per-event rating receipt.
11. **Latency and scale:** event-to-reconcilable-batch p95 is at most sixty seconds; five- and fifteen-minute behavior works; peak ingestion, billing-cycle load, and Pricing Group cardinality pass.
12. **Concurrent admission:** racing workers cannot reserve more than Tenant available balance or Task Budget; parent and delegated work share one ceiling.
13. **Reservation lifecycle:** event acceptance retains reservation; determinate rating captures actual and releases excess; unknown result keeps the hold.
14. **Upper-bound violation:** actual rating above reservation pauses the Task even when Tenant Wallet remains positive.
15. **BYOK:** model dimension is not charged, while other billable service dimensions remain charged.
16. **Correction:** supported pre-invoice compensation adjusts the intended metric; finalized-invoice corrections use Credit Note or later Invoice.
17. **Plan lifecycle:** immediate upgrade, prorated included-Credits grant, period-end downgrade, annual monthly grant, and Base Plan transition behave as specified.
18. **Refund:** lock, review, channel payout, Lago revocation, recovery, and replay cannot over-refund or leave refunded Credits spendable.
19. **Webhook and reconciliation:** signature, duplicate, out-of-order, lost delivery, exhausted retry, and periodic reread converge to authoritative state.
20. **Fault injection:** API timeout, worker crash, queue backlog, response loss, restart, and partial dependency outage lose neither Payment Facts nor Usage Facts and duplicate no benefit.
21. **Permissions:** Owner and delegated Billing Admin can perform approved commands; ordinary Admin, removed member, API key without commercial capability, and cross-Tenant identifiers are rejected.
22. **Privacy and secrets:** Lago credentials and payment secrets do not appear in client payloads, logs, Usage metadata, Task context, or artifacts.
23. **Backup and restore:** staging restore reconciles Customer, Subscription, Wallet, Invoice, Payment, pending work, and Billing Projections within RPO/RTO.
24. **OpenMeter removal:** after rollback closure, no runtime configuration, write path, worker, or adapter can reach OpenMeter.

### Prior art in the repository

- Existing commercial domain and repository tests already exercise immutable catalog behavior, idempotent payment confirmation, fulfillment recovery, refund locks, usage revisions, Task reservation, and SQLite/PostgreSQL concurrency.
- Existing payment-provider tests verify WeChat Pay and Alipay signatures, merchant identity, exact money conversion, callback replay, query, close, and refund behavior.
- Existing OpenMeter contract probes demonstrate the required evidence shape—pinned release, real endpoint behavior, captured result, and explicit blocked status—but their OpenMeter assumptions are not reusable as Lago passes.
- Existing guarded-route and product-state tests provide prior art for keeping provider objects behind stable WeKnora API and UI states.

### Completion gate

The feature is complete only when the entire behavior and contract matrix passes against the pinned Lago Community environment; required unit, integration, concurrency, contract, end-to-end, static, performance, security, and restore checks pass; AGPL review is accepted; independent Spec Compliance and code review have no unresolved blockers; the rollback window is closed; and OpenMeter runtime paths are removed.

Mock success, an OpenAPI schema, an event-ingestion response, or a healthy Lago API process is not sufficient completion evidence.

## Out of Scope

- Migration of OpenMeter development or test data.
- Lago Cloud.
- Lago Premium features, including Premium Invoice Preview or Subscription Overrides.
- Chinese statutory Tax Invoice issuance, red-letter correction, or delivery.
- Multiple Billing Entities, multiple currencies, and cross-currency Wallet behavior.
- Complex pricing without a trustworthy pre-dispatch upper bound, including graduated, percentage, minimum-commitment, and commitment-spend models.
- Automatic recurring payment collection.
- Direct browser or mobile access to Lago.
- A Lago-provided per-event rating-complete receipt.
- ClickHouse and Kafka as the initial Lago event store.
- Direct operator mutation of published Lago Plans.
- A maintained Lago fork or custom rated-event callback.
- Weakening Credits expiry, ordering, budget, refund, or BYOK invariants to fit Lago.
- Long-term OpenMeter/Lago dual write.
- Treating a rollback to OpenMeter as a data-recovery strategy after Lago receives real commercial data.

## Further Notes

- The accepted architectural rationale is recorded in ADR-0012: Lago owns the commercial ledger while WeKnora retains synchronous Task admission and external Chinese payment-channel facts.
- The project glossary is authoritative for Tenant, Shared Organization, Billing Account, Billing Entity, Customer Invoice, Channel Payment Order, Quote, Payment Fact, Fulfillment, Usage Fact, Settlement Batch, Plan Version, Base Plan, Task Budget, Credit Reservation, Reconciliation Pending, Refund Lock, Resource Quota, Platform Model, and BYOK.
- The throwaway logic prototype validated the separation between Payment, activation, reservation, rating, and revocation. Its key newly surfaced verdict is that actual cost above the reserved upper bound pauses the Task even when unrelated Tenant balance remains positive. The prototype is evidence for the decision, not production code.
- Four vendor capabilities remain explicit implementation blockers until proven on the pinned Community release: external payment releasing the activation rule; pay-after-success Wallet top-up; Credits-lot expiry and deterministic consumption order; and Task/Pricing Group rating latency and cardinality.
- Lago event acceptance can be queried but does not expose a public per-event rated status. Settlement therefore remains Task／batch-scoped unless a future, separately approved ADR changes the design.
- The initial operational objectives are event-to-reconcilable-batch p95 at most sixty seconds, Task pause after five minutes, operator alert after fifteen minutes, RPO at most five minutes, and RTO at most sixty minutes.
