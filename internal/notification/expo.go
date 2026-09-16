package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultExpoEndpoint = "https://exp.host/--/api/v2/push/send"

type ExpoProvider struct {
	endpoint, accessToken string
	client                *http.Client
}

func NewExpoProvider(endpoint, accessToken string) *ExpoProvider {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultExpoEndpoint
	}
	return &ExpoProvider{endpoint: strings.TrimSpace(endpoint), accessToken: strings.TrimSpace(accessToken), client: &http.Client{Timeout: 10 * time.Second}}
}
func NewExpoProviderWithClient(endpoint, accessToken string, client *http.Client) *ExpoProvider {
	p := NewExpoProvider(endpoint, accessToken)
	if client != nil {
		p.client = client
	}
	return p
}

type expoMessage struct {
	To    string            `json:"to"`
	Title string            `json:"title,omitempty"`
	Body  string            `json:"body,omitempty"`
	Data  map[string]string `json:"data,omitempty"`
}
type expoEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}
type expoTicket struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

func (p *ExpoProvider) Send(ctx context.Context, token string, payload PushPayload) (PushReceipt, error) {
	if p == nil || strings.TrimSpace(p.endpoint) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Revoke: false, Retry: false, Err: fmt.Errorf("expo endpoint is not configured")}
	}
	if strings.TrimSpace(token) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: fmt.Errorf("empty push token")}
	}
	body, err := json.Marshal(expoMessage{To: token, Title: payload.Title, Body: payload.Body, Data: map[string]string{"run_id": payload.RunID, "event_id": payload.EventID}})
	if err != nil {
		return PushReceipt{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return PushReceipt{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.accessToken)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, StatusCode: resp.StatusCode, Err: readErr}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return PushReceipt{}, &ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := "UnknownTransport"
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			code = "InvalidProviderToken"
		}
		revoke, retry := ClassifyPushFailure(code)
		return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("expo status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))}
	}
	var envelope expoEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, StatusCode: resp.StatusCode, Err: err}
	}
	ticket, err := firstTicket(envelope.Data)
	if err != nil {
		return PushReceipt{}, err
	}
	if strings.EqualFold(ticket.Status, "ok") && strings.TrimSpace(ticket.ID) != "" {
		return PushReceipt{ID: ticket.ID, Status: ticket.Status}, nil
	}
	code := strings.TrimSpace(ticket.Details.Error)
	if code == "" {
		code = "UnknownTransport"
	}
	revoke, retry := ClassifyPushFailure(code)
	if strings.EqualFold(ticket.Status, "ok") {
		return PushReceipt{}, &ProviderError{Code: "MissingReceipt", Retry: true, StatusCode: resp.StatusCode, Err: ErrMissingReceiptID}
	}
	return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("%s", strings.TrimSpace(ticket.Message))}
}

func firstTicket(raw json.RawMessage) (expoTicket, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return expoTicket{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: ErrMissingReceiptID}
	}
	var one expoTicket
	if raw[0] == '{' {
		if err := json.Unmarshal(raw, &one); err != nil {
			return expoTicket{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
		}
		return one, nil
	}
	var many []expoTicket
	if err := json.Unmarshal(raw, &many); err != nil || len(many) == 0 {
		if err == nil {
			err = ErrMissingReceiptID
		}
		return expoTicket{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	return many[0], nil
}
func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}
