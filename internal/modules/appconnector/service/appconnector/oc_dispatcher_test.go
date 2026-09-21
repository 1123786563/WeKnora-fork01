package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// T11: the open-connector HTTP dispatcher — pre-send fail-closed gates, the
// single POST with only approved args and the persisted key, the frozen T01
// result classification (unknown preserved), the validator registry, the
// auditPersisted=false warning and the 429 cooldown recording.
// ---------------------------------------------------------------------------

type stubOCTokens struct {
	token  string
	err    error
	calls  int32
	tenant uint64
	conn   string
	ver    int64
}

func (s *stubOCTokens) Token(ctx context.Context, tenant uint64, connectionID string, authVersion int64) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	s.tenant, s.conn, s.ver = tenant, connectionID, authVersion
	return s.token, s.err
}

type stubOCRecords struct {
	rec appconn.OCDispatchRecord
	err error
}

func (s *stubOCRecords) GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconn.OCDispatchRecord, error) {
	return s.rec, s.err
}

type stubOCBindings struct {
	b   appconn.OCBinding
	err error
}

func (s *stubOCBindings) GetBinding(ctx context.Context, tenant uint64, connection string) (appconn.OCBinding, error) {
	return s.b, s.err
}

type stubRetrySink struct {
	err        error
	calls      int32
	provider   string
	retryAfter time.Time
	observed   time.Time
}

func (s *stubRetrySink) NoteOCProviderRetryAfter(ctx context.Context, provider string, retryAfter, now time.Time) error {
	atomic.AddInt32(&s.calls, 1)
	s.provider, s.retryAfter, s.observed = provider, retryAfter, now
	return s.err
}

// ocDispSnap is one claimed open-connector action snapshot: the reviewed
// GitHub read action, dispatched with the persisted normalized args.
func ocDispSnap() ActionSnapshot {
	return ActionSnapshot{
		ID: "ocact-disp", TenantID: 7, ActorID: "alice", ConnectionID: "conn-oc",
		Version: "1.0.0", Target: "github.get_current_user", Risk: appconn.RiskRead,
		Digest: "dg-disp", State: appconn.ActionDispatched, Fence: 1,
		Args: json.RawMessage(`{}`), AuthVersion: 1, DigestVersion: 2,
		OC: &appconn.OCExecutionBinding{
			RuntimeID: "rt-1", Provider: "github", ExternalID: "ext-42",
			Alias: "alias-1", ActionID: "github.get_current_user",
			SchemaDigest: "sd-1", BindingVersion: 3,
		},
	}
}

func ocDispBinding() appconn.OCBinding {
	return appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 1, BindingVersion: 3,
		State: appconn.OCBindingActive,
	}
}

func ocDispRecord() appconn.OCDispatchRecord {
	return appconn.OCDispatchRecord{
		TenantID: 7, ActionID: "ocact-disp", RuntimeID: "rt-1",
		Key: "wk-oc-persisted-key-1", State: "dispatched", Fence: 1,
	}
}

// ocWireCapture is the last request the test server saw on the wire.
type ocWireCapture struct {
	Method, Path, Auth, Key, Alias, ContentType, Body string
}

// newOCDispServer starts a counting httptest server delegating to respond and
// returns the count accessor plus the last captured wire request. Both the
// handler's captures and the accessors share one mutex channel, so reads are
// race-free even when the responder aborts the connection mid-response.
func newOCDispServer(t *testing.T, respond http.HandlerFunc) (*httptest.Server, func() int, func() ocWireCapture) {
	t.Helper()
	var mu = make(chan struct{}, 1)
	mu <- struct{}{}
	var calls int
	var last ocWireCapture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		<-mu
		calls++
		last = ocWireCapture{
			Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"),
			Key: r.Header.Get("Idempotency-Key"), Alias: r.Header.Get("x-oo-connector-alias"),
			ContentType: r.Header.Get("Content-Type"), Body: string(raw),
		}
		mu <- struct{}{}
		respond(w, r)
	}))
	t.Cleanup(srv.Close)
	count := func() int {
		<-mu
		defer func() { mu <- struct{}{} }()
		return calls
	}
	capture := func() ocWireCapture {
		<-mu
		defer func() { mu <- struct{}{} }()
		return last
	}
	return srv, count, capture
}

// ocDispEnv builds a dispatcher over a REAL T02 client and the stub stores.
func ocDispEnv(t *testing.T, respond http.HandlerFunc) (*OCDispatcher, *stubOCTokens, func() int) {
	t.Helper()
	srv, calls, _ := newOCDispServer(t, respond)
	client, err := openconnector.NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	tokens := &stubOCTokens{token: "oct-scoped-token"}
	d := NewOCDispatcher(client,
		&stubOCBindings{b: ocDispBinding()},
		tokens,
		&stubOCRecords{rec: ocDispRecord()})
	return d, tokens, calls
}

func ocSuccessEnvelope(data string) string {
	return fmt.Sprintf(`{"success":true,"message":"OK","data":%s,"meta":{"executionId":"exec-77","actionId":"github.get_current_user","auditPersisted":true}}`, data)
}

// TestOCDispatcherSendsPersistedKeyAndApprovedArgsOnly pins the wire surface:
// exactly ONE POST to the action path, the Authorization token from the
// version-scoped token source, the PERSISTED operation-scoped key from the
// T10 dispatch record (never the connection id), the binding's alias header,
// and a body of {"input": <approved args>} and nothing else.
func TestOCDispatcherSendsPersistedKeyAndApprovedArgsOnly(t *testing.T) {
	srv, calls, last := newOCDispServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, ocSuccessEnvelope(`{"id":1,"login":"octocat"}`))
	})
	client, err := openconnector.NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	tokens := &stubOCTokens{token: "oct-scoped-token"}
	d := NewOCDispatcher(client, &stubOCBindings{b: ocDispBinding()}, tokens, &stubOCRecords{rec: ocDispRecord()})

	out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
	if derr != nil {
		t.Fatal(derr)
	}
	if out.Status != appconn.ActionSucceeded {
		t.Fatalf("status = %s (%s)", out.Status, out.ProviderResult)
	}
	if got := calls(); got != 1 {
		t.Fatalf("POSTs = %d, want exactly 1", got)
	}
	wire := last()
	if wire.Method != http.MethodPost {
		t.Fatalf("method = %s", wire.Method)
	}
	if wire.Path != "/v1/actions/github.get_current_user" {
		t.Fatalf("path = %s", wire.Path)
	}
	if wire.Auth != "Bearer oct-scoped-token" {
		t.Fatalf("authorization = %q", wire.Auth)
	}
	if wire.Key != "wk-oc-persisted-key-1" {
		t.Fatalf("idempotency key = %q, want the PERSISTED dispatch record key", wire.Key)
	}
	if strings.Contains(wire.Key, "conn-oc") {
		t.Fatal("connection id leaked onto the wire as the idempotency key")
	}
	if wire.Alias != "alias-1" {
		t.Fatalf("alias header = %q", wire.Alias)
	}
	if wire.Body != `{"input":{}}` {
		t.Fatalf("body = %q, want the approved normalized args only", wire.Body)
	}
	if tokens.calls != 1 || tokens.tenant != 7 || tokens.conn != "conn-oc" || tokens.ver != 1 {
		t.Fatalf("token source saw (%d, %d, %s, %d)", tokens.calls, tokens.tenant, tokens.conn, tokens.ver)
	}
}

// TestOCDispatcherDisconnectAfterBodyIsUnknownSinglePost is the ruling-9
// matrix case 1: the server receives the request body and THEN drops the
// connection. The provider outcome is unknown (the call may have executed)
// and exactly ONE POST was sent — the client never retries, and the
// dispatcher never wraps the call in a loop.
func TestOCDispatcherDisconnectAfterBodyIsUnknownSinglePost(t *testing.T) {
	d, _, calls := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)   // the provider DID receive the payload
		panic(http.ErrAbortHandler) // then the connection dies mid-response
	})

	out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
	if derr == nil {
		t.Fatalf("disconnect returned a result: %+v", out)
	}
	final := settleOutcome(out, derr)
	if final.Status != appconn.ActionUnknown {
		t.Fatalf("disconnect settled as %s, want unknown", final.Status)
	}
	if got := calls(); got != 1 {
		t.Fatalf("POSTs = %d, want exactly 1 (no retry after a disconnect)", got)
	}
}

// TestOCDispatcherSuccessMissingProviderIdentityIsUnknown is matrix case 2:
// a 200 success=true envelope whose data lacks the action's REQUIRED provider
// identity fields cannot prove which identity answered — unknown, never
// succeeded.
func TestOCDispatcherSuccessMissingProviderIdentityIsUnknown(t *testing.T) {
	for name, data := range map[string]string{
		"no login":      `{"id":1}`,
		"no id":         `{"login":"octocat"}`,
		"zero id":       `{"id":0,"login":"octocat"}`,
		"null data":     `null`,
		"string id":     `{"id":"1","login":"octocat"}`,
		"empty login":   `{"id":1,"login":"  "}`,
		"garbage types": `{"id":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			d, _, calls := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, ocSuccessEnvelope(data))
			})
			out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
			if derr != nil {
				t.Fatal(derr)
			}
			if out.Status != appconn.ActionUnknown {
				t.Fatalf("%s settled as %s, want unknown", name, out.Status)
			}
			if !strings.Contains(out.ProviderResult, "identity fields missing") {
				t.Fatalf("result = %q", out.ProviderResult)
			}
			if got := calls(); got != 1 {
				t.Fatalf("POSTs = %d", got)
			}
		})
	}
}

// TestOCDispatcherMalformedEnvelopeIsUnknown is matrix case 3: a response
// body that is not the frozen /v1 envelope is a protocol failure — the
// request DID reach the provider, so the outcome is unknown.
func TestOCDispatcherMalformedEnvelopeIsUnknown(t *testing.T) {
	d, _, _ := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html>gateway error page</html>")
	})
	out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
	if derr == nil {
		t.Fatalf("malformed envelope returned a result: %+v", out)
	}
	if !errors.Is(derr, openconnector.ErrMalformedEnvelope) {
		t.Fatalf("err = %v, want ErrMalformedEnvelope", derr)
	}
	if final := settleOutcome(out, derr); final.Status != appconn.ActionUnknown {
		t.Fatalf("malformed envelope settled as %s, want unknown", final.Status)
	}
}

// TestOCDispatcherPreExecutionRejectionIsFailed is matrix case 4: 400
// invalid_input and 403 authorization_failed / connection_not_allowed are
// the ONLY T01-proven pre-execution rejections (the cross-connection denial
// happens BEFORE credential use — contract §3.3, fixtures/cross_connection.
// json), so they are the only non-2xx outcomes that may settle as failed.
func TestOCDispatcherPreExecutionRejectionIsFailed(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		code   string
	}{
		"403 connection_not_allowed": {http.StatusForbidden,
			`{"success":false,"message":"connection refused by grant","data":null,"errorCode":"connection_not_allowed","meta":{"executionId":"exec-9"}}`, "connection_not_allowed"},
		"403 authorization_failed": {http.StatusForbidden,
			`{"success":false,"message":"authorization failed","data":null,"errorCode":"authorization_failed","meta":{}}`, "authorization_failed"},
		"400 invalid_input": {http.StatusBadRequest,
			`{"success":false,"message":"input rejected","data":null,"errorCode":"invalid_input","meta":{}}`, "invalid_input"},
	} {
		t.Run(name, func(t *testing.T) {
			d, _, _ := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
			if derr != nil {
				t.Fatal(derr)
			}
			if out.Status != appconn.ActionFailed {
				t.Fatalf("pre-execution rejection settled as %s (%s)", out.Status, out.ProviderResult)
			}
			if !strings.Contains(out.ProviderResult, "pre-execution rejection") ||
				!strings.Contains(out.ProviderResult, tc.code) {
				t.Fatalf("result = %q, want the T01 errorCode cited", out.ProviderResult)
			}
		})
	}
}

// TestOCDispatcherAuditFailureKeepsSuccessWithWarning is matrix case 5:
// meta.auditPersisted=false does NOT negate the action result (T01 §3.7,
// fixtures/audit_failure.json) — the success is retained and a LOCAL audit
// warning is recorded on the action row.
func TestOCDispatcherAuditFailureKeepsSuccessWithWarning(t *testing.T) {
	d, _, _ := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w,
			`{"success":true,"message":"OK","data":{"id":1,"login":"octocat"},"meta":{"executionId":"exec-audit","actionId":"github.get_current_user","auditPersisted":false}}`)
	})
	out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
	if derr != nil {
		t.Fatal(derr)
	}
	if out.Status != appconn.ActionSucceeded {
		t.Fatalf("audit failure negated the success: %s", out.Status)
	}
	if !strings.Contains(out.ProviderResult, "audit warning: meta.auditPersisted=false") {
		t.Fatalf("local audit warning missing: %q", out.ProviderResult)
	}
	if !strings.Contains(out.ProviderResult, "exec-audit") {
		t.Fatalf("execution id missing from the local record: %q", out.ProviderResult)
	}
}

// TestOCDispatcherUnprovableOutcomesStayUnknown: success=false whose
// side-effect freedom cannot be proven, every unmapped status (402/404/409,
// 5xx), and contradictory envelopes — all stay unknown: never a fabricated
// failure, never a re-send.
func TestOCDispatcherUnprovableOutcomesStayUnknown(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
	}{
		"200 success=false":       {http.StatusOK, `{"success":false,"message":"provider error","data":null,"errorCode":"provider_error","meta":{}}`},
		"402 insufficient_credit": {http.StatusPaymentRequired, `{"success":false,"message":"credit","data":null,"errorCode":"insufficient_credit","meta":{}}`},
		"404 unknown_action":      {http.StatusNotFound, `{"success":false,"message":"no action","data":null,"errorCode":"unknown_action","meta":{}}`},
		"409 idempotency":         {http.StatusConflict, `{"success":false,"message":"conflict","data":null,"errorCode":"idempotency_key_conflict","meta":{}}`},
		"500 internal_error":      {http.StatusInternalServerError, `{"success":false,"message":"boom","data":null,"errorCode":"internal_error","meta":{}}`},
		"503 unavailable":         {http.StatusServiceUnavailable, "service unavailable"},
		"403 without errorCode":   {http.StatusForbidden, `{"success":false,"message":"odd","data":null,"meta":{}}`},
	} {
		t.Run(name, func(t *testing.T) {
			d, _, _ := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
			// A non-JSON body (e.g. the 503 text page) surfaces as an executor
			// protocol error rather than a classified result — either way the
			// settled outcome must be unknown.
			if final := settleOutcome(out, derr); final.Status != appconn.ActionUnknown {
				t.Fatalf("%s settled as %s, want unknown", name, final.Status)
			}
		})
	}
}

// TestOCDispatcherThrottleRecordsCooldown treats T10-Q-5: on a 429 the
// provider cooldown is recorded durably, and the store reporting a RETAINED
// value (repoappconn.ErrOCDispatchConflict) is SUCCESS-with-keep — never a
// failure, and never a reason to change the unknown outcome.
func TestOCDispatcherThrottleRecordsCooldown(t *testing.T) {
	fixed := time.Unix(1700000000, 0).UTC()
	envelope := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"success":false,"message":"slow down","data":null,"errorCode":"rate_limited","meta":{}}`)
	}

	for name, sinkErr := range map[string]error{
		"recorded":      nil,
		"retained":      repoappconn.ErrOCDispatchConflict,
		"record failed": errors.New("db down"),
	} {
		t.Run(name, func(t *testing.T) {
			d, _, _ := ocDispEnv(t, envelope)
			sink := &stubRetrySink{err: sinkErr}
			d.UseOCRetryAfterSink(sink)
			d.now = func() time.Time { return fixed }

			out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
			if derr != nil {
				t.Fatal(derr)
			}
			if out.Status != appconn.ActionUnknown {
				t.Fatalf("%s: 429 settled as %s, want unknown", name, out.Status)
			}
			if sink.calls != 1 {
				t.Fatalf("%s: cooldown notes = %d, want 1", name, sink.calls)
			}
			if sink.provider != "github" {
				t.Fatalf("%s: provider = %q", name, sink.provider)
			}
			if !sink.retryAfter.Equal(fixed.Add(ocThrottleCooldown)) {
				t.Fatalf("%s: cooldown instant = %v", name, sink.retryAfter)
			}
			switch name {
			case "recorded":
				if !strings.Contains(out.ProviderResult, "cooldown recorded") {
					t.Fatalf("result = %q", out.ProviderResult)
				}
			case "retained":
				if !strings.Contains(out.ProviderResult, "cooldown retained") {
					t.Fatalf("retained sentinel not treated as success: %q", out.ProviderResult)
				}
			case "record failed":
				if !strings.Contains(out.ProviderResult, "cooldown record failed") {
					t.Fatalf("result = %q", out.ProviderResult)
				}
			}
		})
	}

	// Without a sink the 429 still settles as unknown (no panic, no note).
	t.Run("unwired", func(t *testing.T) {
		d, _, _ := ocDispEnv(t, envelope)
		out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
		if derr != nil {
			t.Fatal(derr)
		}
		if out.Status != appconn.ActionUnknown || !strings.Contains(out.ProviderResult, "rate limited") {
			t.Fatalf("unwired sink changed the outcome: %+v", out)
		}
	})
}

// TestOCDispatcherPreSendRejectionsFailClosedZeroCalls: every pre-send gate
// (wiring, missing claim record, snapshot/binding drift in any field, token
// failure, unusable call inputs) rejects with ErrDispatchNotStarted, settles
// as failed, and costs ZERO HTTP calls.
func TestOCDispatcherPreSendRejectionsFailClosedZeroCalls(t *testing.T) {
	snapFor := func(mutate func(*ActionSnapshot)) ActionSnapshot {
		s := ocDispSnap()
		if mutate != nil {
			mutate(&s)
		}
		return s
	}
	bindingFor := func(mutate func(*appconn.OCBinding)) appconn.OCBinding {
		b := ocDispBinding()
		if mutate != nil {
			mutate(&b)
		}
		return b
	}
	recordFor := func(mutate func(*appconn.OCDispatchRecord)) appconn.OCDispatchRecord {
		rec := ocDispRecord()
		if mutate != nil {
			mutate(&rec)
		}
		return rec
	}
	for name, tc := range map[string]struct {
		snap    ActionSnapshot
		binding appconn.OCBinding
		rec     appconn.OCDispatchRecord
		tokens  *stubOCTokens
	}{
		"native snapshot":        {snapFor(func(s *ActionSnapshot) { s.OC = nil }), ocDispBinding(), ocDispRecord(), &stubOCTokens{token: "t"}},
		"records error":          {snapFor(nil), ocDispBinding(), ocDispRecord(), &stubOCTokens{token: "t", err: errors.New("row missing")}},
		"record without key":     {snapFor(nil), ocDispBinding(), recordFor(func(r *appconn.OCDispatchRecord) { r.Key = "" }), &stubOCTokens{token: "t"}},
		"binding lookup error":   {snapFor(nil), appconn.OCBinding{}, ocDispRecord(), &stubOCTokens{token: "t"}},
		"binding revoked":        {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.State = appconn.OCBindingRevoked }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"runtime drifted":        {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.RuntimeID = "rt-2" }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"provider drifted":       {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.Provider = "notion" }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"external id drifted":    {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.ExternalID = "ext-other" }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"alias drifted":          {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.Alias = "alias-2" }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"binding gen drifted":    {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.BindingVersion = 4 }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"auth gen drifted":       {snapFor(nil), bindingFor(func(b *appconn.OCBinding) { b.AuthVersion = 2 }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"claim pinned elsewhere": {snapFor(nil), ocDispBinding(), recordFor(func(r *appconn.OCDispatchRecord) { r.RuntimeID = "rt-9" }), &stubOCTokens{token: "t"}},
		"token error":            {snapFor(nil), ocDispBinding(), ocDispRecord(), &stubOCTokens{token: "t", err: errors.New("mint failed")}},
		"token empty":            {snapFor(nil), ocDispBinding(), ocDispRecord(), &stubOCTokens{}},
		"alias empty":            {snapFor(func(s *ActionSnapshot) { s.OC.Alias = "" }), bindingFor(func(b *appconn.OCBinding) { b.Alias = "" }), ocDispRecord(), &stubOCTokens{token: "t"}},
		"args not json":          {snapFor(func(s *ActionSnapshot) { s.Args = json.RawMessage("nope") }), ocDispBinding(), ocDispRecord(), &stubOCTokens{token: "t"}},
	} {
		t.Run(name, func(t *testing.T) {
			srv, calls, _ := newOCDispServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, ocSuccessEnvelope(`{"id":1,"login":"octocat"}`))
			})
			client, err := openconnector.NewClient(srv.URL, srv.Client())
			if err != nil {
				t.Fatal(err)
			}
			records := &stubOCRecords{rec: tc.rec}
			if name == "records error" {
				records.err = errors.New("not found")
			}
			bindings := &stubOCBindings{b: tc.binding}
			if name == "binding lookup error" {
				bindings.err = errors.New("not found")
			}
			d := NewOCDispatcher(client, bindings, tc.tokens, records)

			out, derr := d.Dispatch(context.Background(), tc.snap, "conn-oc")
			if derr == nil || !errors.Is(derr, ErrDispatchNotStarted) {
				t.Fatalf("%s: err = %v, want ErrDispatchNotStarted", name, derr)
			}
			if final := settleOutcome(out, derr); final.Status != appconn.ActionFailed {
				t.Fatalf("%s: settled as %s, want failed (pre-send proof)", name, final.Status)
			}
			if got := calls(); got != 0 {
				t.Fatalf("%s: HTTP calls = %d, want 0", name, got)
			}
		})
	}

	// Incomplete dispatcher wiring fails closed the same way.
	t.Run("nil deps", func(t *testing.T) {
		d := NewOCDispatcher(nil, nil, nil, nil)
		out, derr := d.Dispatch(context.Background(), ocDispSnap(), "conn-oc")
		if derr == nil || !errors.Is(derr, ErrDispatchNotStarted) {
			t.Fatalf("err = %v, want ErrDispatchNotStarted", derr)
		}
		if final := settleOutcome(out, derr); final.Status != appconn.ActionFailed {
			t.Fatalf("settled as %s, want failed", final.Status)
		}
	})
}

// TestOCDispatcherWriteActionWithoutPublishedValidatorSucceeds: the frozen
// registry contains ONLY reviewed actions with confirmed identity fields —
// github.get_current_user. A write-shaped action has NO published validator
// until T18 confirms its fields, so its clean success envelope stands as
// success (there is nothing to confirm), while the reviewed read action is
// still held to id+login.
func TestOCDispatcherWriteActionWithoutPublishedValidatorSucceeds(t *testing.T) {
	if _, ok := ocResultValidators["github.get_current_user"]; !ok {
		t.Fatal("registry lost the reviewed github.get_current_user validator")
	}
	d, _, _ := ocDispEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w,
			`{"success":true,"message":"OK","data":{"number":7},"meta":{"executionId":"exec-w","actionId":"github.create_issue","auditPersisted":true}}`)
	})
	snap := ocDispSnap()
	snap.OC = &appconn.OCExecutionBinding{
		RuntimeID: "rt-1", Provider: "github", ExternalID: "ext-42",
		Alias: "alias-1", ActionID: "github.create_issue",
		SchemaDigest: "sd-1", BindingVersion: 3,
	}
	out, derr := d.Dispatch(context.Background(), snap, "conn-oc")
	if derr != nil {
		t.Fatal(derr)
	}
	if out.Status != appconn.ActionSucceeded {
		t.Fatalf("unpublished write action settled as %s, want succeeded", out.Status)
	}
}

// TestExecuteWithOCDispatcherEndToEnd wires the REAL dispatcher into Execute
// over the real stores: the durable claim mints the key, the dispatcher
// sends exactly one POST carrying that key, and the action settles.
func TestExecuteWithOCDispatcherEndToEnd(t *testing.T) {
	svc, gate, store, db := newOCExecuteEnvDispatcher(t)
	ctx := context.Background()
	ocSeedAuthorizedOCAction(t, db, "act-e2e")

	if err := svc.Execute(ctx, "act-e2e"); err != nil {
		t.Fatal(err)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-e2e").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionSucceeded {
		t.Fatalf("final state = %s", row.State)
	}
	rec, err := store.GetOCDispatch(ctx, 7, "act-e2e")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Key == "" || rec.ReservationID == "" {
		t.Fatalf("record = %+v", rec)
	}
	if begins, finishes, _ := gate.stats(); begins != 1 || finishes != 1 {
		t.Fatalf("begins = %d finishes = %d, want 1/1", begins, finishes)
	}
}

// TestExecuteWithOCDispatcherUnknownParksForProviderQuery: through the REAL
// dispatcher, a mid-response disconnect parks the action in unknown (never
// failed, never re-queued) and the reservation is NOT settled — only the
// provider query path may resolve it (T12).
func TestExecuteWithOCDispatcherUnknownParksForProviderQuery(t *testing.T) {
	svc, gate, _, db := newOCExecuteEnvDispatcherDisconnect(t)
	ctx := context.Background()
	ocSeedAuthorizedOCAction(t, db, "act-unk")

	err := svc.Execute(ctx, "act-unk")
	if !errors.Is(err, ErrDispatchUnknown) {
		t.Fatalf("err = %v, want ErrDispatchUnknown", err)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-unk").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionUnknown {
		t.Fatalf("state = %s, want unknown", row.State)
	}
	if err := svc.Execute(ctx, "act-unk"); !errors.Is(err, ErrActionState) {
		t.Fatalf("unknown re-queued: %v", err)
	}
	if _, finishes, _ := gate.stats(); finishes != 0 {
		t.Fatalf("finishes = %d, want 0 (unknown keeps the hold for recovery)", finishes)
	}
}

// newOCExecuteEnvDispatcher is newOCExecuteEnv (oc_limiter_test.go) with the
// stub dispatcher replaced by the REAL OCDispatcher over an httptest server
// answering a clean success envelope.
func newOCExecuteEnvDispatcher(t *testing.T) (*ActionService, *stableGate, *repoappconn.OCStore, *gorm.DB) {
	t.Helper()
	svc, gate, store, db, _ := newOCExecuteEnvWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w,
			`{"success":true,"message":"OK","data":{"ok":true},"meta":{"executionId":"exec-e2e","actionId":"send_message","auditPersisted":true}}`)
	})
	return svc, gate, store, db
}

func newOCExecuteEnvDispatcherDisconnect(t *testing.T) (*ActionService, *stableGate, *repoappconn.OCStore, *gorm.DB) {
	t.Helper()
	svc, gate, store, db, _ := newOCExecuteEnvWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		panic(http.ErrAbortHandler)
	})
	return svc, gate, store, db
}

// newOCExecuteEnvWithServer builds the full OC Execute stack (real stores,
// real claim, real gate) with the REAL dispatcher over the given responder.
func newOCExecuteEnvWithServer(t *testing.T, respond http.HandlerFunc) (*ActionService, *stableGate, *repoappconn.OCStore, *gorm.DB, *OCDispatcher) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(respond))
	t.Cleanup(srv.Close)
	client, err := openconnector.NewClient(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	db := ocLimiterDB(t)
	inst := repoappconn.NewInstallationStore(db)
	store := repoappconn.NewOCStore(db)
	ctx := context.Background()
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: "app-oc", Version: "1.0.0", State: appconn.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{
		ID: "conn-oc", InstallationID: "inst-7", Kind: appconn.ConnectionKindSpace,
		OwnerID: "owner", CredentialRef: "cred/x", State: appconn.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBinding(ctx, appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
	disp := NewOCDispatcher(client, store, &stubOCTokens{token: "oct-scoped-token"}, store)
	gate := &stableGate{}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, nil)
	svc.UseOCDispatchClaims(store)
	return svc, gate, store, db, disp
}
