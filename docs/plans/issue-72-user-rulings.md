# Issue #72 用户裁决记录（spec/ADR owner 签发）

本文件记录 Lago 计费迁移实施期间由 spec/ADR owner（仓库所有者）作出的正式裁决。
每条含：决定、依据、错误代价。裁决一经记录对本分支全部实施活动有约束力。

---

## R-1 ｜ 2026-09-23 ｜ T02 付款激活通道 = 选项② 受支持 Provider

- **决定**：#81/#82 的 Payment 录入与 incomplete Subscription 激活通道采用 Lago 原生 Payment Provider 集成（Stripe TEST 已在 #74 实证：run_lab 九阶段全 pass，AC1-AC4 证据晋升 docs/migrations/lago/t02-payment-activation/）。
- **依据**：Lago Community manual payment 被 Premium 门控（403 源码实证），Premium 路径引入商业依赖违背 ADR-0012 自托管动机；Provider 路径保持 Lago 原生 Payment/Invoice 语义完整，spec 不动。
- **错误代价**：若 Provider 路径未来收紧（如 Stripe 白名单变更波及新渠道），#82 支付宝/#83 微信接入需重新评估，可能回到选项③改 spec。

## R-2 ｜ 2026-09-23 ｜ 75-a2 充值批次并发模型 = 选项 B 协调层承载

- **决定**：充值批次状态机由 WeKnora 协调层承载，Lago Wallet 交易仅在付款确认到账时刻幂等创建（幂等键绑定渠道支付单号），无产品级充值并发上限。ADR-0012 已追加修订记录（集成分支提交 552d98d12）。
- **依据**：#75/T03 的 422 实证是"实验直接并发创建 7 个活跃 Lago 钱包"触发供应商并发保护，非产品流真实约束；方案 A（≤5 并发闸门）是给产品加人为限制迁就实验路径。
- **错误代价**：若 Lago 单实例瞬时串行写入不足以支撑高峰到账（当前无证据），需重新评估。

## R-3 ｜ 2026-09-23 ｜ #81 spec L121 付款前 line-item 比对 = 选项 A 批准偏差实施

- **决定**：批准 #81 按「付款前以权威订阅面对象硬校验 Plan Version/币种/总额 + 无 charges 切片推导单行订阅费 + #82/#84 付款时 Invoice finalized 可见后完整 line-item 复核」实施，替代 spec L121 的付款前全项匹配。升级记录：workflow escalation dwfq-e9e3798d-1，主 Agent 裁决后经 spec owner 确认（本条即确认记录）。
- **依据**：pinned Lago v1.53.0 待付款 Invoice 对全部 API 不可见（源码实证 invoice.rb:100-101 INVISIBLE_STATUS 含 open、invoices_query.rb:122-129 显式 status 与 visible 集合求交、controller/GraphQL 均 .visible），spec L121 原文在当前版本上不可实现；替代链保留商业意图（金额不可篡改的付款前确认），防护实质等价、时序后移。
- **强制条件**：① t09 DECISION.md 完整留痕；② Ledger 记 Ruling 含错误代价；③ 付款时完整 line-item 复核必须是 #81 的显式 Produces 接口、后续 Issue Consumes 强制引用（防止付款前不比对演变为永远不比对）；④ 本偏差仅限此一条，不得外溢为其他 spec 条款先例。
- **错误代价**：若裁决错误且 line items 在付款前存在真实篡改面，代价是付款前防线弱化——由付款后复核与 #84 异常付款验收兜底暴露。
- **owner 权利**：本偏差全程可否决——全部成果在集成分支 codex/issue-72-lago（未合并未推送），叫停即回退。
