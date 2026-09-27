package commercial

// #84 Task 1（异常付款事实持久化）：spec L127——错金额/错币种/部分付款的
// 外部资金事实必须被保留（commercial_payment_anomalies 表），但绝不修改
// Invoice 或扩大权益。本文件锁定表级行为：闭合分类、唯一键幂等落库、
// 未处置探测与运营处置（resolve 的版本守卫）。

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyPaymentAnomalyClosedKinds(t *testing.T) {
	if got := ClassifyPaymentAnomaly(9900, 5000, "CNY", "CNY"); got != PaymentAnomalyKindPartial {
		t.Fatalf("partial: got %s", got)
	}
	if got := ClassifyPaymentAnomaly(9900, 19900, "CNY", "CNY"); got != PaymentAnomalyKindAmount {
		t.Fatalf("amount: got %s", got)
	}
	if got := ClassifyPaymentAnomaly(9900, 9900, "CNY", "USD"); got != PaymentAnomalyKindCurrency {
		t.Fatalf("currency: got %s", got)
	}
}

func TestRecordPaymentAnomalyIdempotentOnUniqueKey(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	row := PaymentAnomalyRow{ID: "anom_1", TenantID: 7, OrderID: "ord_1", AttemptID: "mo_1",
		Provider: "wechat", Merchant: "mch", Transaction: "txn_a",
		Kind: PaymentAnomalyKindPartial, ExpectedAmountFen: 9900, ActualAmountFen: 5000,
		ExpectedCurrency: "CNY", ActualCurrency: "CNY"}
	if err := s.RecordPaymentAnomaly(ctx, row); err != nil {
		t.Fatal(err)
	}
	// 同 (provider, merchant, transaction) 不同主键 → 唯一冲突视为已记录 → nil，
	// 渠道重复通知不产生第二行（Review Focus 3）。
	row.ID = "anom_2"
	if err := s.RecordPaymentAnomaly(ctx, row); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&PaymentAnomalyRow{}).Count(&n)
	if n != 1 {
		t.Fatalf("want 1 row, got %d", n)
	}
	ok, err := s.HasUnresolvedPaymentAnomaly(ctx, "ord_1")
	if err != nil || !ok {
		t.Fatalf("want unresolved, got %v %v", ok, err)
	}
	// 空主键由 RecordPaymentAnomaly 内部生成（事务外落库时生成，避免回滚后
	// 重试换 ID——Task 1 契约）。
	if err := s.RecordPaymentAnomaly(ctx, PaymentAnomalyRow{TenantID: 7, OrderID: "ord_2",
		AttemptID: "mo_2", Provider: "wechat", Merchant: "mch", Transaction: "txn_b",
		Kind: PaymentAnomalyKindAmount, ExpectedAmountFen: 9900, ActualAmountFen: 9901,
		ExpectedCurrency: "CNY", ActualCurrency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	var gen PaymentAnomalyRow
	// "transaction" 是 SQL 关键字，裸列名在 SQLite 报语法错——带引号查询。
	if err := db.Where("\"transaction\" = ?", "txn_b").First(&gen).Error; err != nil {
		t.Fatal(err)
	}
	if gen.ID == "" || gen.State != PaymentAnomalyStateAwaiting {
		t.Fatalf("generated row must carry id and awaiting state, got %+v", gen)
	}
}

// TestPaymentAnomalyListAndResolveGuard：admin 全量读（created_at DESC）与
// resolve 的 state+version 守卫——正确 expected_version 翻 resolved（version+1、
// resolved_at 落位），过期/错误 version 与不存在分别给出两个可区分哨兵
// （Task 3 handler 的 409/404 由它们驱动）。
func TestPaymentAnomalyListAndResolveGuard(t *testing.T) {
	s, db := testOrderStore(t)
	ctx := context.Background()
	first := PaymentAnomalyRow{ID: "anom_r1", TenantID: 7, OrderID: "ord_r1", AttemptID: "mo_r1",
		Provider: "wechat", Merchant: "mch", Transaction: "txn_r1",
		Kind: PaymentAnomalyKindCurrency, ExpectedAmountFen: 9900, ActualAmountFen: 9900,
		ExpectedCurrency: "CNY", ActualCurrency: "USD"}
	second := first
	second.ID, second.OrderID, second.AttemptID, second.Transaction = "anom_r2", "ord_r2", "mo_r2", "txn_r2"
	for _, row := range []PaymentAnomalyRow{first, second} {
		if err := s.RecordPaymentAnomaly(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	// created_at 同秒粒度下 DESC 顺序不保证——列表断言只锁定集合与闭合字段。
	rows, err := s.ListPaymentAnomalies(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 anomalies, got %d err=%v", len(rows), err)
	}
	resolved, err := s.ResolvePaymentAnomaly(ctx, "anom_r1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != PaymentAnomalyStateResolved || resolved.Version != 2 || resolved.ResolvedAt == nil {
		t.Fatalf("resolve must flip state and bump version, got %+v", resolved)
	}
	if ok, _ := s.HasUnresolvedPaymentAnomaly(ctx, "ord_r1"); ok {
		t.Fatal("a resolved anomaly must leave the attention window")
	}
	// 重放同 version → 版本守卫拒绝（anomaly changed since read）。
	if _, err := s.ResolvePaymentAnomaly(ctx, "anom_r1", 1); !errors.Is(err, ErrPaymentAnomalyVersionConflict) {
		t.Fatalf("stale version must be refused with the conflict sentinel, got %v", err)
	}
	if _, err := s.ResolvePaymentAnomaly(ctx, "anom_missing", 1); !errors.Is(err, ErrPaymentAnomalyNotFound) {
		t.Fatalf("missing anomaly must answer not-found, got %v", err)
	}
	// 处置不改资金事实本体：仅 state/version/resolved_at 变化。
	var after PaymentAnomalyRow
	if err := db.Where("id = ?", "anom_r1").First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ActualCurrency != "USD" || after.ActualAmountFen != 9900 {
		t.Fatalf("resolve must never rewrite the retained fact, got %+v", after)
	}
}
