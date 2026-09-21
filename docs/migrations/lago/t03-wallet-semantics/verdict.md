# T03 verdict — Wallet 批次到期、消费顺序与撤回语义（Lago Community v1.53.0）

Ticket #75 · Wave 2 · 实验室：`deploy/lago-lab/wallet-semantics/`（独立
Compose 项目 `weknora-lago-75`，回环端口 48893/48894，digest 锁定镜像）。
所有结论来自 E1–E4 对 pinned 运行时的真实运行证据
（`evidence/e{1,2,3,4}-*.json`，一次性 `run-all` 生成）与 pinned 源码引证
（`source-notes.md`）。判定词汇：`PASS`（原生即可保持不变量）、
`PASS-WITH-COORDINATION`（不变量可保持，但依赖 WeKnora 协调层的明确义务，
逐条列出）、`BLOCKED`（pinned 运行时无法保持该不变量，迁移在该点上被阻断）。

## 判定总表

| # | 不变量（验收标准） | 实验 | 证据 | 判定 |
|---|---|---|---|---|
| a1 | 套餐内额度按月到期且不结转 | E1 | `evidence/e1-expiry.json` | **PASS-WITH-COORDINATION** |
| a2 | 充值批次按到账日起十二个月到期 | E1（cap 探针） | `evidence/e1-expiry.json` | **BLOCKED**（六活动钱包硬上限） |
| b | 多个有效批次按最早到期、同到期最早发放消费 | E2 | `evidence/e2-order.json` | **PASS-WITH-COORDINATION** |
| c | 并发消费、到期和重试不重复扣减、不产生负可退余额 | E3（+E1 竞态窗口） | `evidence/e3-concurrency.json` | **PASS-WITH-COORDINATION** |
| d | void 与 refund 只撤回未消费部分 | E4 | `evidence/e4-withdraw.json` | **PASS-WITH-COORDINATION** |

任一不变量 BLOCKED ⇒ 按 ticket 要求，Lago 原生对象**不能**在不引入协调层
重设计的情况下承载已批准的 Credits 模型；阻断点与可行路径见下文逐条分析。

---

## a1 — 套餐内额度按月到期且不结转：PASS-WITH-COORDINATION

**断言内容**：短 TTL 钱包（模拟月度批次）到期后由时钟终止（`*:45` 小时
`TerminateWalletsJob`）；终止后的余额不可被任何后续发票消费；已终止钱包
拒绝新发放（`wallet_is_terminated`）；余额不会通过新发放复活；充值批次钱包
跨过月度边界仍可消费。

**运行时观察（E1）**：

- 月度钱包在 `expiration_at` 通过后被终止（实测距到期约 1652 s——时钟
  小时粒度，见竞态窗口）；`status: active → terminated`，其余额 60.00
  原样搁浅、无任何结转，终止后发票对它的扣减为 0。
- 对已终止钱包再次发放被 422 拒绝，余额不变。
- 十二个月充值钱包跨过月度边界后被正常消费（5000 cents），状态保持
  active。
- interval 规则钱包（不设过期）余额跨边界保持 3000 —— 区间规则语义上
  就是"同一持久钱包滚动累加"，无法表达按月清零（映射负面结论）。

**协调层义务（必须，缺一不可）**：

1. **竞态窗口（E1 预注册 FAIL 项，`expired_wallet_not_consumable`）**：
   终止是惰性的——`expiration_at` 已过但时钟未跑的窗口内（实测最长
   ~65 分钟）钱包仍是 `active` 且**可被消费**（E1 实测从已过期未终止的
   钱包扣掉了 40.00）。协调层必须在 WeKnora 侧按自己的批次注册表在
   派发前拒绝已过期批次，绝不能依赖 Lago 的惰性终止。
2. **初始额度异步入账**：wallet 创建自带的 granted/paid 初始额度经
   after-commit job 异步结算；创建后立即消费会跳过最新（最高优先级）
   钱包而先消费更晚到期的批次（E2 首跑实测）。协调层必须在发放后等待
   余额可见（settle-wait）再放行消费。
3. 每个月度批次一个短 TTL 钱包（无每交易过期字段；区间规则在 Community
   不可用，见已知限制）。

## a2 — 充值批次十二个月到期：BLOCKED

**阻断证据（E1 cap 探针）**：v1.53.0 的过期只存在于钱包级
（`wallets.expiration_at`），因此**每个批次一个到期 ⇒ 每个批次一个钱包**。
而 `Wallets::ValidateService::MAXIMUM_WALLETS_PER_CUSTOMER = 6` 是硬编码
运行时上限（`maximum_wallets_per_customer` 覆写在 v1.53.0 无任何写入方）。
E1 实测：第 7 个并发 active 钱包创建被 422 `wallet_limit_reached` 拒绝；
终止一个后可再创建（slot 释放）。

**推论**：一个客户持有 1 个月度套餐钱包 + 6 笔未过期充值批次 = 7 个
active 钱包 ⇒ 超限。任何持有超过 ~5 笔并发未过期充值批次的客户都会撞
上限。这是 spec 四个供应商阻断项（"Credits-lot expiry and deterministic
consumption order"）之外的**新实测阻断项**。可选出路（超出本 ticket，
须回到设计阶段）：

- 限制并发充值批次数 ≤ 5 并产品化表达（弱化产品能力）；
- 或将"十二个月到期"从 Lago 原生对象移入 WeKnora 协调层（批次注册表
  记到期，Lago 钱包只按协调层指令 void/终止）——此时 Lago 不再是该
  不变量的权威，需 ADR 修订商业权威边界。

## b — 消费顺序（最早到期、同到期最早发放）：PASS-WITH-COORDINATION

**断言内容**：E2 用三个钱包（创建顺序 C,B,A；优先级按到期秩编码
A=1,B=2,C=3，到期秩与创建顺序相反）串行消费恰好抽干单个钱包，逐笔观察
扣减落在哪个钱包。

**运行时观察（E2，5/5 pass）**：

- 扣减序列 A(50.00) → B(70.00) → C(40.00 部分)：严格
  `priority ASC, created_at ASC`（`Wallet.in_application_order`），
  优先级（到期编码）压倒创建顺序；无任何原生按到期排序。
- 同钱包三批次（G1 granted → P1 purchased → G2 granted 按创建时间）按
  G1 → G2 → P1 抽干：同优先级时 **granted 先于 purchased**（压倒
  created_at），同为 granted 时按 created_at。
- `remaining_amount_cents` 全程单调不增且永不为负。

**协调层义务**：到期秩必须编码进 `priority`（1..50），同优先级靠
`created_at`（= 发放时间）决胜即可精确表达"最早到期、同到期最早发放"；
但 granted-before-purchased 意味着**同一钱包内**充值（purchased）批次总
排在套餐（granted）批次之后——批次一钱包一到期（a2 已 BLOCKED）时该
规则不构成额外偏差；若未来合批，则同到期下"最早发放"不可精确表达。
并发批次 ≤ 6 时优先级空间无碰撞。消费前必须完成 a1 的 settle-wait。

## c — 并发、到期与重试：无重复扣减、无负可退余额：PASS-WITH-COORDINATION

**运行时观察（E3，6/6 pass；E1 补充）**：

- 6 线程并发终态化（合计需求 300.00 对 100.00 余额）：总扣减恰好
  100.00，钱包余额到 0 为止，`remaining_amount_cents` 最小值 0，
  永不为负（`Customers::LockService scope: prepaid_credit` 串行化 +
  DB 检查约束 `remaining_amount_cents_non_negative`）。
- void 与终态化对同一余额竞争：二者互斥（实测 void 赢：100.00 全额
  void、消费 0；反向亦满足），drawn + voided ≤ 发放额。
- 到期边界前后各消费一次：合计 80.00 ≤ 100.00，无双重扣减（竞态窗口
  本身见 a1）。
- **重试语义（预注册负面，实测确认）**：钱包/钱包交易 API **无外部
  幂等标识**——同一 `POST /api/v1/wallet_transactions` 载荷重放会铸造
  第二笔 settled 入账、余额翻倍；恢复路径 = `metadata` 键查询索引
  （实测按 `weknora_t03_batch` 检索出两笔）。对照组：事件按
  `transaction_id` 幂等（重放 422 `value_already_exist`）。

**协调层义务**：发放幂等必须由协调层自有（Outbox 稳定幂等标识 +
metadata 相关键作为事后恢复路径），照抄事件的 transaction_id 模式；
超时重试前先按 metadata 查询再补发。

## d — void 与 refund 只撤回未消费部分：PASS-WITH-COORDINATION

**运行时观察（E4，8/8 pass）**：

- 部分_void（20.00 ≤ 余 60.00）：成功，只递减该入账的
  `remaining_amount_cents` 与钱包余额，已消费历史（outbound invoiced）
  原样保留。
- 恰好等于余量：批次清零、余额清零。
- 超余量（批次层面）：422 `exceeds_remaining_transaction_amount`，
  无任何变更；超过钱包总余额更早被 422 `insufficient_credits` 拦截。
- `void_remaining`（不给金额）对已抽干批次：422
  `no_remaining_amount`（校验层拒绝，非静默 no-op），余额不为负。
- 退款模型（消费 40.00 后整体 void 余 60.00）：批次余量 0、钱包余额
  0、无可消费残留。
- **purchased 批次同样可按余量 void**：入账在 pending 期间
  `remaining_amount_cents` 为 nil，但结算（付款确认）时被赋值
  （`SettleService`），此后与 granted 完全同语义。
- **Credit Note 在 Community 被溢价门禁**：`POST /api/v1/credit_notes`
  实测 403 `feature_unavailable`。Community 上的退款路径收敛为
  "钱包 void（撤回未消费部分）+ 渠道侧资金退款"，且 credit note 不会
  给钱包加回余额（`RecreditService` 仅对 voided invoice 触发）。
- **paid credits 需付款确认才可消费**（修正研究结论）：无支付 provider
  时 `paid_credits` POST 只产生 pending 入账、余额为 0；`PUT
  /api/v1/invoices/:id {payment_status: succeeded}` → `PrepaidCreditJob`
  → 结算入账。这正是 spec 的 Payment-Fact → Lago 履约接缝，WeKnora
  必须显式驱动。

**协调层义务**：退款锁定期间先 void 剩余（锁定 = 协调层状态），资金
退款在渠道侧完成；不得依赖 credit note（Community 不可用）；充值退款
必须先完成付款确认结算再 void。

---

## 已知限制台账（源码引证见 `source-notes.md`）

| 限制 | 影响 |
|---|---|
| 无每交易过期（仅 `wallets.expiration_at`） | 批次一钱包 → 撞六钱包上限（a2 BLOCKED 的根因） |
| 同优先级 granted 先于 purchased | 同钱包合批时"同到期最早发放"不可精确表达 |
| 钱包创建初始额度 after-commit 异步结算 | 创建后立即消费跳过最新钱包（a1/b 协调义务） |
| 惰性终止（小时钟，`*:45`） | 最长 ~65 分钟已过期仍可消费（a1 协调义务） |
| 无 `refunded` 交易状态；credit note 溢价门禁（403） | 退款=void+渠道资金，无 Lago 原生退款状态机 |
| 钱包交易无幂等标识 | 发放重试重复入账（c 协调义务） |
| interval 规则溢价门禁且创建路径 500 | Community 无法用区间规则表达任何周期发放 |
| 一次性发票（`POST /api/v1/invoices`）不消费钱包 | 消费触发必须走 pay-in-advance 事件路径 |
| 阈值规则钱包可透支为负（ongoing 分配） | WeKnora 不得使用 threshold 规则 |
| paid credits 结算依赖付款确认 | 履约必须由 Payment Fact 驱动（与 spec 一致） |

## 结论

Lago Community v1.53.0 的钱包对象可以承载：月度不结转（带协调义务）、
确定性消费顺序（优先级编码）、并发安全、余量精确 void。**不能**承载：
超过 ~5 笔并发未过期批次的逐批次十二个月到期（六活动钱包硬上限，
无覆盖写入路径）——该点按 ticket 要求**明确阻断迁移**，需回到设计阶段
决定产品上限或把批次到期权威移入协调层（ADR 修订）。此外惰性终止竞态
窗口与发放幂等是必须由协调层承担的硬义务，spec 已预见协调层角色，不构
成额外阻断。
