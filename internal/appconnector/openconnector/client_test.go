package openconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// capturedRequest records what the client actually put on the wire.
type capturedRequest struct {
	Method      string
	Path        string
	Body        string
	Auth        string
	IdemKey     string
	Alias       string
	ContentType string
}

// newCaptureServer starts an httptest server that records every request it
// receives (count + last snapshot) and delegates the response to respond.
func newCaptureServer(t *testing.T, respond http.HandlerFunc) (*httptest.Server, func() int, func() capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	var last capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls++
		last = capturedRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			Body:        string(body),
			Auth:        r.Header.Get("Authorization"),
			IdemKey:     r.Header.Get("Idempotency-Key"),
			Alias:       r.Header.Get("x-oo-connector-alias"),
			ContentType: r.Header.Get("Content-Type"),
		}
		mu.Unlock()
		respond(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return calls },
		func() capturedRequest { mu.Lock(); defer mu.Unlock(); return last }
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// validCall returns a fully valid call (alias set) for happy-path style tests.
func validCall() Call {
	return Call{
		Alias:    "primary",
		ActionID: "arxiv.get_paper",
		Key:      "wk-idx-1",
		Input:    json.RawMessage(`{"id":"2301.00001"}`),
	}
}

// TestClientRejectsAliasOmissionBeforeNetwork is the plan-pinned test: an
// omitted alias must fail validation with ZERO HTTP calls. Upstream's default
// alias is only ever "default" with no fallback (contract doc §3.4), so
// WeKnora never lets an empty alias reach the wire.
func TestClientRejectsAliasOmissionBeforeNetwork(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Execute(context.Background(), "scoped", Call{ActionID: "github.get_current_user", Key: "k", Input: json.RawMessage(`{}`)})
	if err == nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

// TestClientRejectsInvalidInputBeforeNetwork covers every pre-network
// validation from the task brief: blank alias, empty token, empty /
// whitespace-only / over-length (256-byte) idempotency key, malformed action
// id, and invalid (or missing) JSON input. All must fail with zero calls.
func TestClientRejectsInvalidInputBeforeNetwork(t *testing.T) {
	srv, calls, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"success":true,"message":"OK","data":null,"meta":{}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	good := validCall()
	badActionIDs := []string{"", "has space", "a/b", "a*b", "a?b", "a:b", "a;b"}
	for name, mutate := range map[string]func(*Call, *string){
		"alias blank":         func(c *Call, _ *string) { c.Alias = "   " },
		"alias omitted":       func(c *Call, _ *string) { c.Alias = "" },
		"token empty":         func(c *Call, _ *string) {},
		"key empty":           func(c *Call, _ *string) { c.Key = "" },
		"key whitespace":      func(c *Call, _ *string) { c.Key = "   " },
		"key 256 bytes":       func(c *Call, _ *string) { c.Key = strings.Repeat("k", 256) },
		"input nil":           func(c *Call, _ *string) { c.Input = nil },
		"input invalid json":  func(c *Call, _ *string) { c.Input = json.RawMessage(`{"id":`) },
		"input scalar string": func(c *Call, _ *string) { c.Input = json.RawMessage(`nope`) },
	} {
		t.Run(name, func(t *testing.T) {
			call := good
			token := "scoped"
			if name == "token empty" {
				token = ""
			}
			mutate(&call, &token)
			_, err := c.Execute(context.Background(), token, call)
			if err == nil {
				t.Fatalf("invalid call accepted (%s)", name)
			}
			if n := calls(); n != 0 {
				t.Fatalf("%s: network was hit %d time(s) before validation failed", name, n)
			}
		})
	}
	for _, id := range badActionIDs {
		t.Run("action id "+id, func(t *testing.T) {
			call := good
			call.ActionID = id
			_, err := c.Execute(context.Background(), "scoped", call)
			if err == nil {
				t.Fatalf("invalid action id %q accepted", id)
			}
			if n := calls(); n != 0 {
				t.Fatalf("action id %q: network was hit %d time(s)", id, n)
			}
		})
	}
}

// TestClientKeyByteBoundary pins the Idempotency-Key boundary: 255 bytes is
// the upstream maximum and must be accepted; 256 must be rejected locally.
func TestClientKeyByteBoundary(t *testing.T) {
	srv, calls, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"success":true,"message":"OK","data":null,"meta":{}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	call := validCall()
	call.Key = strings.Repeat("k", 255)
	if _, err := c.Execute(context.Background(), "scoped", call); err != nil {
		t.Fatalf("255-byte key rejected: %v", err)
	}
	if n := calls(); n != 1 {
		t.Fatalf("255-byte key: calls=%d, want 1", n)
	}
}

// TestNewClientRejectsUnsafeBaseURL covers base-URL validation: only http(s)
// roots without userinfo, query or fragment are accepted.
func TestNewClientRejectsUnsafeBaseURL(t *testing.T) {
	for _, base := range []string{
		"",
		"not a url",
		"ftp://127.0.0.1:1",
		"//no-scheme",
		"http://user:pass@127.0.0.1:1",
		"https://token@oc.internal",
		"http://127.0.0.1:1/?q=1",
		// A bare trailing "?" is a query separator that survives url.Parse
		// (ForceQuery=true, u.String() keeps it): accepting it would route
		// every action path into the rawQuery, so it must be rejected.
		"http://127.0.0.1:1/?",
		"http://127.0.0.1:1/#frag",
		// NOTE: only a bare trailing "#" is omitted — that one IS normalized
		// away by url.Parse (u.String() drops it), so it carries no fragment.
	} {
		if _, err := NewClient(base, http.DefaultClient); err == nil {
			t.Fatalf("unsafe base URL accepted: %q", base)
		}
	}
	for _, base := range []string{"http://127.0.0.1:31701", "https://oc.internal", "http://oc.internal/open-connector"} {
		if _, err := NewClient(base, http.DefaultClient); err != nil {
			t.Fatalf("valid base URL rejected: %q: %v", base, err)
		}
	}
	if _, err := NewClient("http://127.0.0.1:1", nil); err != nil {
		t.Fatalf("nil http client rejected: %v", err)
	}
}

// TestClientHappyPathDecodesEnvelope exercises a single POST against a
// fixture-shaped success envelope (mirrors fixtures/key_replay.json): body is
// exactly {"input":...}, headers are exactly the three required ones, and the
// decoded Result carries meta.executionId / meta.actionId /
// meta.auditPersisted (contract doc §3.3).
func TestClientHappyPathDecodesEnvelope(t *testing.T) {
	srv, calls, last := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"success":true,"message":"OK","data":{"found":true,"paper":{"id":"2301.00001v1"}},"meta":{"executionId":"0a1b2c3d-exec","actionId":"arxiv.get_paper","auditPersisted":true}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Execute(context.Background(), "oct_test_token", validCall())
	if err != nil {
		t.Fatalf("happy path failed: %v", err)
	}
	if n := calls(); n != 1 {
		t.Fatalf("calls=%d, want exactly 1 (no retry)", n)
	}
	got := last()
	if got.Method != http.MethodPost {
		t.Fatalf("method=%s, want POST", got.Method)
	}
	if got.Path != "/v1/actions/arxiv.get_paper" {
		t.Fatalf("path=%s, want /v1/actions/arxiv.get_paper", got.Path)
	}
	if got.Body != `{"input":{"id":"2301.00001"}}` {
		t.Fatalf("body=%s, want exactly %s with no other wrapper fields", got.Body, `{"input":{...}}`)
	}
	if got.Auth != "Bearer oct_test_token" {
		t.Fatalf("authorization header=%q", got.Auth)
	}
	if got.IdemKey != "wk-idx-1" {
		t.Fatalf("Idempotency-Key=%q", got.IdemKey)
	}
	if got.Alias != "primary" {
		t.Fatalf("x-oo-connector-alias=%q", got.Alias)
	}
	if got.ContentType != "application/json" {
		t.Fatalf("Content-Type=%q", got.ContentType)
	}
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("http status=%d", res.HTTPStatus)
	}
	if !res.Success {
		t.Fatalf("success=%v, want true", res.Success)
	}
	if res.Code != "" {
		t.Fatalf("code=%q, want empty on success envelope", res.Code)
	}
	if res.Message != "OK" {
		t.Fatalf("message=%q", res.Message)
	}
	if string(res.Data) != `{"found":true,"paper":{"id":"2301.00001v1"}}` {
		t.Fatalf("data=%s", res.Data)
	}
	if res.ExecutionID != "0a1b2c3d-exec" {
		t.Fatalf("meta.executionId not decoded: %q", res.ExecutionID)
	}
	if res.ActionID != "arxiv.get_paper" {
		t.Fatalf("meta.actionId not decoded: %q", res.ActionID)
	}
	if res.AuditPersisted == nil || !*res.AuditPersisted {
		t.Fatalf("meta.auditPersisted not decoded: %v", res.AuditPersisted)
	}
}

// TestClientAuditPersistedNilWhenAbsent: meta.auditPersisted is a pointer —
// nil exactly when the envelope omits it (never fabricated true/false).
func TestClientAuditPersistedNilWhenAbsent(t *testing.T) {
	srv, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, `{"success":true,"message":"OK","data":null,"meta":{"executionId":"exec-1"}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Execute(context.Background(), "scoped", validCall())
	if err != nil {
		t.Fatal(err)
	}
	if res.AuditPersisted != nil {
		t.Fatalf("auditPersisted=%v, want nil when absent", *res.AuditPersisted)
	}
	if res.ExecutionID != "exec-1" {
		t.Fatalf("executionId=%q", res.ExecutionID)
	}
}

// TestClientFailureEnvelopeStaysRaw: a success=false envelope (fixture-shaped
// after fixtures/cross_connection.json and the §3.3 status mapping) must come
// back as raw data with nil error — the error code field is errorCode (NOT
// code), executionId/auditPersisted sit inside meta, and business-result
// classification is deliberately left to T11.
func TestClientFailureEnvelopeStaysRaw(t *testing.T) {
	srv, calls, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "internal.fail") {
			writeJSON(w, http.StatusInternalServerError, `{"success":false,"message":"boom","data":null,"errorCode":"internal_error","meta":{}}`)
			return
		}
		// 403 cross-connection shape
		writeJSON(w, http.StatusForbidden, `{"success":false,"message":"de6bbc9d-26c2 connection is not granted to this runtime token.","data":null,"errorCode":"connection_not_allowed","meta":{"executionId":"de6bbc9d-exec","actionId":"dune.get_execution_status","auditPersisted":true}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	call := validCall()
	call.ActionID = "dune.get_execution_status"
	res, err := c.Execute(context.Background(), "scoped", call)
	if err != nil {
		t.Fatalf("non-2xx business failure must not be a transport error: %v", err)
	}
	if res.HTTPStatus != http.StatusForbidden {
		t.Fatalf("http status=%d", res.HTTPStatus)
	}
	if res.Success {
		t.Fatalf("success=%v, want false raw from envelope", res.Success)
	}
	if res.Code != "connection_not_allowed" {
		t.Fatalf("errorCode not mapped from errorCode field: %q", res.Code)
	}
	if res.ExecutionID != "de6bbc9d-exec" {
		t.Fatalf("meta.executionId not decoded: %q", res.ExecutionID)
	}
	if res.AuditPersisted == nil || !*res.AuditPersisted {
		t.Fatalf("meta.auditPersisted not decoded: %v", res.AuditPersisted)
	}
	// 5xx envelope: still a single raw call, no retry, no classification.
	call2 := validCall()
	call2.ActionID = "internal.fail"
	res2, err := c.Execute(context.Background(), "scoped", call2)
	if err != nil {
		t.Fatalf("5xx envelope must not be a transport error: %v", err)
	}
	if res2.HTTPStatus != http.StatusInternalServerError || res2.Code != "internal_error" || res2.Success {
		t.Fatalf("5xx raw result mishandled: %+v", res2)
	}
	if n := calls(); n != 2 {
		t.Fatalf("calls=%d, want 2 total (one per Execute, no retry)", n)
	}
}

// TestClientDoesNotFollowRedirect: a 302 must not be followed — assert a
// single request, no second call, and the 302 result returned raw.
func TestClientDoesNotFollowRedirect(t *testing.T) {
	srv, calls, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/actions/arxiv.get_paper" {
			w.Header().Set("Location", "/elsewhere")
			writeJSON(w, http.StatusFound, `{"success":false,"message":"moved","data":null,"errorCode":"internal_error","meta":{}}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"success":true,"message":"OK","data":{"leaked":true},"meta":{}}`)
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Execute(context.Background(), "scoped", validCall())
	if err != nil {
		t.Fatalf("302 handling returned transport error: %v", err)
	}
	if n := calls(); n != 1 {
		t.Fatalf("redirect followed: calls=%d, want 1", n)
	}
	if res.HTTPStatus != http.StatusFound {
		t.Fatalf("http status=%d, want 302 raw", res.HTTPStatus)
	}
	if string(res.Data) != `null` || res.Success {
		t.Fatalf("302 body mishandled: %+v", res)
	}
}

// envelopeOfTotalSize builds a valid success envelope whose total serialized
// length is exactly total bytes (padding lives inside the data string).
func envelopeOfTotalSize(total int) []byte {
	prefix := []byte(`{"success":true,"message":"OK","data":"`)
	suffix := []byte(`","meta":{"executionId":"cap","actionId":"cap.act","auditPersisted":false}}`)
	pad := total - len(prefix) - len(suffix)
	if pad < 0 {
		panic("requested envelope smaller than its skeleton")
	}
	body := append([]byte{}, prefix...)
	body = append(body, bytes.Repeat([]byte("a"), pad)...)
	return append(body, suffix...)
}

// TestClientResponseCap: a body of exactly MaxResponseBytes (2 MiB) is
// accepted; one byte more is rejected with an error and no classification.
func TestClientResponseCap(t *testing.T) {
	srv, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelopeOfTotalSize(MaxResponseBytes))
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Execute(context.Background(), "scoped", validCall())
	if err != nil {
		t.Fatalf("2 MiB body must be accepted: %v", err)
	}
	if !res.Success || res.ExecutionID != "cap" {
		t.Fatalf("boundary body not decoded: %+v", res)
	}

	srv2, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(envelopeOfTotalSize(MaxResponseBytes + 1))
	})
	c2, err := NewClient(srv2.URL, srv2.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c2.Execute(context.Background(), "scoped", validCall()); err == nil {
		t.Fatal("2 MiB + 1 byte body must be rejected")
	}
}

// TestClientRejectsMalformedJSONBody: a non-JSON body is a protocol failure.
func TestClientRejectsMalformedJSONBody(t *testing.T) {
	srv, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not-json{{")
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Execute(context.Background(), "scoped", validCall())
	if err == nil {
		t.Fatal("malformed JSON response must be an error")
	}
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("http status should still be captured, got %d", res.HTTPStatus)
	}
}

// TestClientContextCancelMidRequest: cancelling the caller context while the
// server is still handling the request must surface context.Canceled.
func TestClientContextCancelMidRequest(t *testing.T) {
	srv, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(30*time.Millisecond, cancel)
	defer timer.Stop()
	res, err := c.Execute(ctx, "scoped", validCall())
	if err == nil {
		t.Fatal("cancelled context must surface an error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
	if res.HTTPStatus != 0 {
		t.Fatalf("no response should be captured on cancel, got status %d", res.HTTPStatus)
	}
}

// TestClientErrorsCarryNoSecrets scans every client error path used in these
// tests for credential material: token, idempotency key and raw input must
// never appear in an error message.
func TestClientErrorsCarryNoSecrets(t *testing.T) {
	const token = "oct_SECRET_TOKEN"
	srv, _, _ := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "SECRET_RESPONSE_BODY")
	})
	c, err := NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	call := Call{Alias: "primary", ActionID: "arxiv.get_paper", Key: "SECRET_KEY", Input: json.RawMessage(`{"secret":"SECRET_INPUT"}`)}
	_, execErr := c.Execute(context.Background(), token, call)
	_, invalidErr := c.Execute(context.Background(), token, Call{Alias: "primary", ActionID: "bad id", Key: "SECRET_KEY", Input: json.RawMessage(`{}`)})
	for name, err := range map[string]error{"malformed-response": execErr, "invalid-call": invalidErr} {
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		msg := err.Error()
		for _, secret := range []string{"SECRET_TOKEN", "SECRET_KEY", "SECRET_INPUT", "SECRET_RESPONSE_BODY"} {
			if strings.Contains(msg, secret) {
				t.Fatalf("%s error leaks %q: %s", name, secret, msg)
			}
		}
	}
}
