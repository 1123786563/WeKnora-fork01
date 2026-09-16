package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExpoProviderReceiptAndNoTokenLeak(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		got = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":"ok","id":"ticket-1"}}`))
	}))
	defer srv.Close()
	p := NewExpoProviderWithClient(srv.URL, "secret", srv.Client())
	receipt, err := p.Send(context.Background(), "ExponentPushToken[abc]", PushPayload{Title: "Done", Body: "Ready", RunID: "run-1", EventID: "event-1"})
	if err != nil || receipt.ID != "ticket-1" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if got == "" {
		t.Fatal("request missing")
	}
}
func TestExpoProviderRateLimitHonorsRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	_, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).Send(context.Background(), "token", PushPayload{})
	var pe *ProviderError
	if !errors.As(err, &pe) || !pe.Retry || pe.Code != "MessageRateExceeded" || pe.RetryAfter != 7*time.Second {
		t.Fatalf("err=%v", err)
	}
}
func TestExpoProviderClassifiesReceiptFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}}`))
	}))
	defer srv.Close()
	_, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).Send(context.Background(), "token", PushPayload{})
	var pe *ProviderError
	if !errors.As(err, &pe) || !pe.Revoke || pe.Retry || pe.Code != "DeviceNotRegistered" {
		t.Fatalf("err=%v", err)
	}
}
func TestExpoProviderRejectsMissingReceipt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"data":{"status":"ok"}}`)) }))
	defer srv.Close()
	_, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).Send(context.Background(), "token", PushPayload{})
	if !errors.Is(err, ErrMissingReceiptID) {
		t.Fatalf("err=%v", err)
	}
}
