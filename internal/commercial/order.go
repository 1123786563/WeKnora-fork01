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

// ErrPaymentMismatch reports a payment fact that does not match its order:
// a different order, tenant, amount, or currency, or a non-succeeded state.
var ErrPaymentMismatch = errors.New("payment_mismatch")

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
