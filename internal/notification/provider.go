// Package notification contains the provider-neutral mobile push boundary.
package notification

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type PushPayload struct{ Title, Body, RunID, EventID string }
type PushReceipt struct{ ID, Status string }
type PushProvider interface {
	Send(context.Context, string, PushPayload) (PushReceipt, error)
}
type ProviderError struct {
	Code       string
	StatusCode int
	RetryAfter time.Duration
	Revoke     bool
	Retry      bool
	Err        error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("push provider %s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("push provider %s", e.Code)
}
func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
func ClassifyPushFailure(code string) (revoke, retry bool) {
	switch code {
	case "DeviceNotRegistered", "InvalidRegistration", "Unregistered", "BadDeviceToken":
		return true, false
	case "MessageRateExceeded", "TooManyRequests", "Unavailable", "UnknownTransport", "Timeout", "TransportError":
		return false, true
	case "MessageTooBig", "InvalidCredentials", "InvalidProviderToken", "MessageTooBigPermanent":
		return false, false
	default:
		return false, true
	}
}

var ErrMissingReceiptID = errors.New("push provider response missing receipt id")
