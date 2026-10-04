package commercial

import "errors"

// Order lifecycle states. A new order starts pending; confirming a payment
// fact moves it to paid; delivering the fulfillment outbox event moves it to
// fulfilled. Readers must distinguish paid from fulfilled so benefits are
// never re-granted and never exposed before delivery.
const (
	OrderStatePending   = "pending"
	OrderStatePaid      = "paid"
	OrderStateFulfilled = "fulfilled"
)

// Order kinds select the fulfillment policy. A purchase order settles as
// credit top-up; an upgrade order switches the subscription plan (keeping
// the paid interval) and grants only the prorated monthly-credit delta.
const (
	OrderKindPurchase = "purchase"
	OrderKindUpgrade  = "upgrade"
)

// ErrPaymentMismatch reports a payment fact that does not match its order:
// a different order, tenant, amount, or currency, or a non-succeeded state.
var ErrPaymentMismatch = errors.New("payment_mismatch")

// ErrPaymentNotSucceeded (#84, spec L127 classification) reports a
// VERIFIED-SIGNATURE fact whose channel state is not succeeded (e.g.
// trade_status=TRADE_CLOSED): it is neither a fulfillment nor an abnormal
// collection — nothing is persisted, the transaction rolls back whole, and
// the callback face keeps its non-2xx answer so the channel finishes its
// retry policy. It is a SEPARATE classification from ErrPaymentMismatch
// (which retains the fund fact as an anomaly and answers terminally):
// folding the two would misread "right amount, closed state" as a false
// amount mismatch.
var ErrPaymentNotSucceeded = errors.New("payment_not_succeeded")

// Order awaits exactly one successful payment before fulfillment.
type Order struct {
	ID       string
	TenantID uint64
	Amount   CNYFen
	Currency string
	State    string
	Version  int64
}

// PaymentFact records a provider-confirmed payment attempt result. AttemptID
// carries the merchant-side attempt identifier (merchant_order_id).
type PaymentFact struct {
	TenantID    uint64
	OrderID     string
	AttemptID   string
	Provider    string
	Merchant    string
	Transaction string
	Amount      CNYFen
	Currency    string
	State       string
	// RefundID is non-empty only for a verified REFUND.* notification (#97):
	// it carries the refund's own stable out_refund_no while AttemptID
	// keeps the ORIGINAL payment out_trade_no. A refund notification is a
	// re-read trigger only — it must never reach ConfirmPayment.
	RefundID string
}

// ValidatePayment rejects any fact that would change what the order charged:
// the order, tenant, amount, and currency must all agree and the payment must
// have succeeded.
func ValidatePayment(o Order, f PaymentFact) error {
	if o.ID != f.OrderID || o.TenantID != f.TenantID || o.Amount != f.Amount ||
		o.Currency != f.Currency || f.State != "succeeded" {
		return ErrPaymentMismatch
	}
	return nil
}
