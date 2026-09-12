package payment

import (
	"context"
	"errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/commercial"
)

// AttemptState is the provider-side attempt/refund state vocabulary. It is
// a DISTINCT type from the refund lifecycle states (commercial.RefundState)
// and from the channel refund outcome vocabulary
// (commercial.RefundChannelState): the compiler rejects mixing a payment
// attempt state into a refund transition or vice versa.
type AttemptState string

// Attempt/result states shared by every Provider implementation. The domain
// acceptance state is StateSucceeded (commercial.ValidatePayment);
// StateUnknown means a channel request timed out, so the caller must
// reconcile by querying the ORIGINAL identifier instead of re-keying the
// order.
const (
	StatePending   AttemptState = "pending"
	StateSucceeded AttemptState = "succeeded"
	StateClosed    AttemptState = "closed"
	StateUnknown   AttemptState = "unknown"
)

// String keeps the raw wire value available where the persistence layer or
// logs need the plain string.
func (s AttemptState) String() string { return string(s) }

// ErrUnknownState reports that a channel request timed out: the remote
// order may or may not exist, so the caller must Query the original
// identifier and must never retry under a fresh one.
var ErrUnknownState = errors.New("unknown_provider_state")

// ErrInvalidRequest reports a locally invalid provider request (empty
// identifiers or a non-positive amount) before anything is sent.
var ErrInvalidRequest = errors.New("invalid_provider_request")

// OrderRequest describes one payable order handed to a provider.
// MerchantOrderID is the merchant-side attempt identifier that callback
// reconciliation keys on.
type OrderRequest struct {
	OrderID         string
	MerchantOrderID string
	AmountFen       int64
	Currency        string
}

// AttemptResult reports the provider-side state of one attempt. ProviderID
// is the provider-visible identifier of the ORIGINAL request (for WeChat
// the out_trade_no); CheckoutURL carries the customer-facing payment link
// or code URL when the channel returns one.
type AttemptResult struct {
	State       AttemptState
	ProviderID  string
	CheckoutURL string
}

// RefundRequest refunds a previously captured attempt; ProviderID again
// references the original order identifier on the channel.
type RefundRequest struct {
	RefundID   string
	ProviderID string
	AmountFen  int64
}

// RefundResult reports the provider-side refund state keyed by RefundID.
type RefundResult struct {
	State      AttemptState
	ProviderID string
}

// Provider is one payment channel adapter. Verify authenticates an
// asynchronous callback (raw header map + raw body, each read exactly once
// by the caller) and returns the payment fact WITHOUT local identity:
// TenantID and OrderID stay zero because a callback payload is untrusted;
// the caller must resolve them from the local order registry.
type Provider interface {
	Create(context.Context, OrderRequest) (AttemptResult, error)
	Query(context.Context, string) (AttemptResult, error)
	Close(context.Context, string) error
	Verify(context.Context, http.Header, []byte) (commercial.PaymentFact, error)
	Refund(context.Context, RefundRequest) (RefundResult, error)
	QueryRefund(context.Context, string) (RefundResult, error)
}
