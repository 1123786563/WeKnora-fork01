package commercial

import "testing"

func TestPaymentCannotChangeOrderAmount(t *testing.T) {
	order := Order{ID: "o1", TenantID: 7, Amount: 100, Currency: "CNY"}
	fact := PaymentFact{OrderID: "o1", TenantID: 7, Amount: 99, Currency: "CNY", State: "succeeded"}
	if ValidatePayment(order, fact) == nil {
		t.Fatal("amount mismatch accepted")
	}
	fact.Amount = 100
	fact.TenantID = 8
	if ValidatePayment(order, fact) == nil {
		t.Fatal("space mismatch accepted")
	}
}

func TestPaymentCurrencyMismatchRejected(t *testing.T) {
	order := Order{ID: "o1", TenantID: 7, Amount: 100, Currency: "CNY"}
	fact := PaymentFact{OrderID: "o1", TenantID: 7, Amount: 100, Currency: "USD", State: "succeeded"}
	if ValidatePayment(order, fact) == nil {
		t.Fatal("currency mismatch accepted")
	}
}

func TestPaymentNonSucceededStateRejected(t *testing.T) {
	order := Order{ID: "o1", TenantID: 7, Amount: 100, Currency: "CNY"}
	fact := PaymentFact{OrderID: "o1", TenantID: 7, Amount: 100, Currency: "CNY", State: "failed"}
	if ValidatePayment(order, fact) == nil {
		t.Fatal("non-succeeded state accepted")
	}
}

func TestPaymentWrongOrderRejected(t *testing.T) {
	order := Order{ID: "o1", TenantID: 7, Amount: 100, Currency: "CNY"}
	fact := PaymentFact{OrderID: "o2", TenantID: 7, Amount: 100, Currency: "CNY", State: "succeeded"}
	if ValidatePayment(order, fact) == nil {
		t.Fatal("wrong order accepted")
	}
}

func TestPaymentMatchingFactValidated(t *testing.T) {
	order := Order{ID: "o1", TenantID: 7, Amount: 100, Currency: "CNY", State: OrderStatePending, Version: 1}
	fact := PaymentFact{
		OrderID: "o1", TenantID: 7, AttemptID: "m1", Provider: "wechat", Merchant: "wxm",
		Transaction: "t1", Amount: 100, Currency: "CNY", State: "succeeded",
	}
	if err := ValidatePayment(order, fact); err != nil {
		t.Fatalf("matching fact rejected: %v", err)
	}
}
