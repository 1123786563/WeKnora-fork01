
─── internal/ocr-review-supplement/commercial_order_review.go:1087-1089 ───
[test · medium] 覆盖缺口（对照 Issue #84 验收描述）："one order" 分支（tenant 112）没有 seed 任何异常行，也没有断言 attention
投影——该分支只证明了查询次数为 1。若 ListOrders 在小结果集上把 attention 恒置 false、或在批量投影边界（1 行 vs 40 行走不同分支）有缺陷，本用例无法发现。建议给
count_single_order 也挂一条 anomaly 并在 "one order" 分支断言 got[0].PaymentAttention == true（wantQueries 仍为
1），与四十行分支形成对称覆盖。

  	if err := db.Create(&repocommercial.OrderRow{ID: "count_single_order", TenantID: 112, QuoteID: "count_single_quote", Kind: "purchase", AmountFen: 100, Currency: "CNY", State: domain.OrderStatePaid, Version: 1}).Error; err != nil {
+ 		t.Fatal(err)
+ 	}
+ 	if err := repocommercial.NewOrderStore(db).RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{TenantID: 112, OrderID: "count_single_order", AttemptID: "a", Provider: "wechat", Merchant: "m", Transaction: "t", Kind: repocommercial.PaymentAnomalyKindAmount, ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
  		t.Fatal(err)
  	}
+ 	// 并在 "one order" 分支断言：got[0].PaymentAttention == true 且 anomaly-table SELECT 计数仍为 1


─── internal/ocr-review-supplement/commercial_order_review.go:1190-1193 ───
[test · medium] 覆盖缺口（对照 Issue #84 验收描述）："重复 checkout 重放 × 异常处置前后 attention true→false"
的组合契约没有被任何单一用例锁定——本用例验证了 attention true→false 的转换，但两次重放都没有断言 CheckoutURL
被原样重发；TestCheckoutURLPersistsAndReplaysVerbatim 只验证 URL 重发、完全没有异常/attention
维度。CurrentPayablePendingOrderView 返回
OrderView（service/commercial/order.go:357），重放视图应同时携带链接与标志。建议在两次读取处同时断言 v.CheckoutURL ==
order.CheckoutURL（处置前 attention=true 且链接在，处置后 attention=false 且链接仍在）。

  	v, err = svc.CurrentPayablePendingOrderView(ctx, 101)
  	if err != nil || v.PaymentAttention {
  		t.Fatalf("resolved view=%+v err=%v", v, err)
+ 	}
+ 	if v.CheckoutURL != order.CheckoutURL {
+ 		t.Fatalf("replayed checkout link must survive anomaly resolution, got %q want %q", v.CheckoutURL, order.CheckoutURL)
  	}


─── internal/ocr-review-supplement/commercial_order_review.go:657-661 ───
[test · medium] Count 的返回错误被忽略（共三处调用）：错币种用例中 nFulfill 的 `!= 0` 守卫在查询失败（如 AutoMigrate 未完成、SQL
错误）时保持零值、核心负向断言假绿；另一用例中 nFulfill 的 `!= 1` 与 nOver 断言在查询失败时会以误导性的 "got 0" 假失败掩盖真实原因。同包已有检查错误并 t.Fatal
的 countOutbox 帮助函数（order_close_test.go:161），本文件其他用例（如
TestRecoverOrderStatusUnknownCollectionFaceIsRetryable）已在用，建议统一替换以消除假绿路径并保证失败可诊断。

- 	var nFulfill int64
- 	db.Model(&repocommercial.OutboxEvent{}).Where("kind = ?", repocommercial.OutboxKindFulfill).Count(&nFulfill)
- 	if nFulfill != 0 {
- 		t.Fatalf("a wrong-currency collection must not mint a fulfill right, got %d", nFulfill)
+ 	if n := countOutbox(t, db, repocommercial.OutboxKindFulfill); n != 0 {
+ 		t.Fatalf("a wrong-currency collection must not mint a fulfill right, got %d", n)
  	}


─── internal/ocr-review-supplement/commercial_order_review.go:88-90 ───
[test · low]
三处建库（newOrderTestEnv、TestOpenOrderPersistsChannelFailurePastCallerCancellation、newRealWechatObservat
ionEnv）都用 t.Name() 作为 cache=shared 内存库的键，且从不关闭 *sql.DB。共享缓存内存库只要有任一连接存活就持续存在——连接池未 Close
时，同名库的重复执行（go test -count=2、重试）会读到上一轮残留行（本文件大量固定 ID 如 order_batch_%02d、count_order_%02d 会直接撞 UNIQUE
约束），AutoMigrate 也不会清空既有数据。单次运行内 t.Name() 唯一所以日常 CI 不受影响，但建议在拿到 *sql.DB 后注册 t.Cleanup 关闭（三处均适用）。

  	if s, err := db.DB(); err == nil {
  		s.SetMaxOpenConns(1)
+ 		t.Cleanup(func() { _ = s.Close() })
  	}


─── internal/ocr-review-supplement/commercial_order_review.go:319-321 ───
[maintainability · low] 此处置复了建库样板但 AutoMigrate 漏掉
PaymentAnomalyRow，而另两处（newOrderTestEnv、newRealWechatObservationEnv）都包含它。OrderService
的读路径（RecoverOrderStatus/ListOrders）已依赖 commercial_payment_anomalies 表，这个降级的样板一旦被复制到需要异常断言的用例，会以 "no
such table" 失败且难以定位。建议补齐该表保持三处一致，或提取共享的建库 helper 消除重复。

  	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
  		&repocommercial.OutboxEvent{}, &repocommercial.PlanRow{}, &repocommercial.QuoteRow{},
- 		&repocommercial.Subscription{}); err != nil {
+ 		&repocommercial.Subscription{}, &repocommercial.PaymentAnomalyRow{}); err != nil {



──────── Project Summary ────────

# 项目级评审摘要

## Top Issues

1. **测试核心断言存在“假绿”通道 —— `Count` 错误被忽略（3 处调用）**
   `internal/ocr-review-supplement/commercial_order_review.go` 中三处调用 `Count` 均未检查返回错误。错币种用例里 `nFulfill != 0` 守卫在查询失败（AutoMigrate 未完成、SQL 错误）时因零值直接短路，核心负向断言假绿；另一用例的 `nFulfill != 1` 与 `nOver` 断言同样会在查询失败时以零值通过。这类失败会静默掩盖真实回归。

2. **建库样板复制粘贴漂移 —— AutoMigrate 漏掉 `PaymentAnomalyRow`**
   第三处建库样板的 AutoMigrate 缺少 `PaymentAnomalyRow`，而 `newOrderTestEnv`、`newRealWechatObservationEnv` 两处均包含。`OrderService` 读路径（`RecoverOrderStatus` / `ListOrders`）已依赖 `commercial_payment_anomalies` 表，这个降级环境一旦触发相关查询，会产出误导性的错误或行为。

3. **`cache=shared` 内存库以 `t.Name()` 为键且从不关闭 `*sql.DB`（3 处）**
   `newOrderTestEnv`、`TestOpenOrderPersistsChannelFailurePastCallerCancellation`、`newRealWechatObservationEnv` 三处共享缓存内存库只要有任一连接存活就持续存在，且连接从不关闭。存在跨用例状态泄漏与连接资源累积风险，可能导致测试间相互污染或顺序依赖。

4. **验收契约覆盖缺口：tenant 112 "one order" 分支只验证了查询次数**
   对照 Issue #84 验收描述，该分支没有 seed 任何异常行，也没有断言 attention 投影——只证明了查询次数为 1。若 `ListOrders` 在小结果集上把 attention 恒置 false、或在批量投影边界处出错，现有用例无法捕获。

5. **验收契约覆盖缺口：重放 × attention 翻转 × `CheckoutURL` 保留的组合契约未被任何单一用例锁定**
   现有用例验证了 attention true→false 的转换，但两次重放都没有断言 `CheckoutURL` 被原样保留。重复 checkout 的幂等契约与异常处置的交互目前处于“部分覆盖”状态，各半张拼图不在同一个用例里。

## Module Hotspots

- `internal/ocr-review-supplement/commercial_order_review.go`：全部 5 条评论集中于此单文件。其中三个环境构建点（`newOrderTestEnv`、`TestOpenOrderPersistsChannelFailurePastCallerCancellation`、`newRealWechatObservationEnv`）是问题密度最高的位置——同时涉及样板漂移、DB 泄漏、Count 错误忽略三类问题，建议作为整改进场的第一站。

## Cross-Cutting Concerns

- **错误处理被系统性忽略**：3 处 `Count` 返回错误均未检查，且后果均为断言假绿而非显式失败（`internal/ocr-review-supplement/commercial_order_review.go`）。这是最危险的一类忽略模式——测试会“安静地通过”。
- **测试环境搭建复制粘贴而非单一来源**：3 处建库样板已出现 AutoMigrate 集合不一致（缺 `PaymentAnomalyRow`），说明环境构造没有共享 helper，漂移会随用例增加继续扩大。
- **测试资源生命周期无管理**：共享缓存内存库连接从不关闭，缺少 `defer db.Close()` 约定。
- **断言停留在“证明实现做了什么”而非“锁定契约”**：attention 投影、`CheckoutURL` 幂等保留等 Issue #84 明确的验收语义没有对应断言，现有断言主要覆盖查询次数与计数。

## Quick Wins

- **封装 `mustCount(t, ...) helper`**：内部检查 `Count` 错误并 `t.Fatalf`，一次性修复 3 处假绿风险，且杜绝后续复制该模式。
- **合并三处建库样改为单一 `newOrderTestEnv` 入口**：AutoMigrate 集合单点维护（补回 `PaymentAnomalyRow`），并在 helper 内统一 `t.Cleanup(func(){ db.Close() })` 关闭连接——低工作量同时消除两条 Top Issue。
- **为 tenant 112 分支补 seed 异常行 + attention 投影断言**：只需在现有用例中追加数据与断言，即可闭合 Issue #84 的该条验收项。
- **在重放用例中追加 `CheckoutURL` 原样保留断言**：两次重放处各加一行比较即可锁定幂等契约，无需新用例。
