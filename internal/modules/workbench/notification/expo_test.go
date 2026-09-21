package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestExpoProviderBatchReturnsPerItemPartialResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var messages []map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&messages))
		require.Len(t, messages, 2)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"status":"ok","id":"ticket-1"},{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}]}`)
	}))
	defer srv.Close()
	results, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).SendBatch(context.Background(), []PushBatchItem{
		{ID: "delivery-1", Token: "token-1", Payload: PushPayload{Body: "one"}},
		{ID: "delivery-2", Token: "token-2", Payload: PushPayload{Body: "two"}},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "ticket-1", results[0].Receipt.ID)
	var providerErr *ProviderError
	require.ErrorAs(t, results[1].Err, &providerErr)
	require.Equal(t, "DeviceNotRegistered", providerErr.Code)
	require.False(t, providerErr.Retry)
}

func TestExpoProviderBatchReportsEmptyTokenWithoutSubmittingEmptyMessage(t *testing.T) {
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var messages []map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&messages))
		require.Len(t, messages, 1)
		require.Equal(t, "token-valid", messages[0]["to"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"status":"ok","id":"ticket-good"}]}`)
	}))
	defer srv.Close()
	results, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).SendBatch(context.Background(), []PushBatchItem{
		{ID: "empty", Token: "  "},
		{ID: "good", Token: "token-valid"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
	require.Len(t, results, 2)
	var providerErr *ProviderError
	require.ErrorAs(t, results[0].Err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.False(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)
	require.Equal(t, "ticket-good", results[1].Receipt.ID)
}

func TestExpoProviderRejectsMalformedEndpointAsConfigurationError(t *testing.T) {
	provider := NewExpoProvider("://bad", "")
	_, err := provider.Send(context.Background(), "token", PushPayload{})
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
	require.False(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)
	_, err = provider.SendBatch(context.Background(), []PushBatchItem{{ID: "d1", Token: "token"}})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
}

func TestExpoProviderBatchPausesCredentialFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, code: "InvalidCredentials"},
		{name: "forbidden", status: http.StatusForbidden, code: "InvalidProviderToken"},
		{name: "structured", status: http.StatusBadRequest, body: `{"errors":[{"code":"InvalidCredentials"}]}`, code: "InvalidCredentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if tc.body != "" {
					_, _ = io.WriteString(w, tc.body)
				}
			}))
			defer srv.Close()
			_, err := NewExpoProviderWithClient(srv.URL, "", srv.Client()).SendBatch(context.Background(), []PushBatchItem{
				{ID: "d1", Token: "token-1"}, {ID: "d2", Token: "token-2"},
			})
			var providerErr *ProviderError
			require.ErrorAs(t, err, &providerErr)
			require.Equal(t, tc.code, providerErr.Code)
			require.False(t, providerErr.Revoke)
			require.False(t, providerErr.Retry)
		})
	}
}
