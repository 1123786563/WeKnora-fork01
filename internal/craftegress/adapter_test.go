package craftegress

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/sirupsen/logrus"
)

// mockGateway plays the Craft model gateway for adapter tests: it records the
// activity header of every forward and can be scripted to lose a response.
type mockGateway struct {
	mu        sync.Mutex
	activity  []string
	auth      []string
	bodies    []string
	failBody  bool // hang up after headers: response body never completes
	status    int
	body      string
	healthErr error
}

func (m *mockGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	var body strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		body.Write(buf[:n])
		if err != nil {
			break
		}
	}
	m.mu.Lock()
	m.activity = append(m.activity, r.Header.Get("X-Craft-Activity-ID"))
	m.auth = append(m.auth, auth)
	m.bodies = append(m.bodies, body.String())
	failBody, status, respBody := m.failBody, m.status, m.body
	m.mu.Unlock()
	if status == 0 {
		status = http.StatusOK
		respBody = `{"id":"chatcmpl-1","usage":{"prompt_tokens":1}}`
	}
	w.Header().Set("Content-Type", "application/json")
	if failBody {
		w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	}
	w.WriteHeader(status)
	if failBody {
		// Send a torn body, then drop the connection.
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte(respBody[:len(respBody)/2]))
		panic(http.ErrAbortHandler)
	}
	_, _ = w.Write([]byte(respBody))
}

func (m *mockGateway) activities() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.activity...)
}

func (m *mockGateway) auths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.auth...)
}

func newAdapterForTest(t *testing.T, gateway *httptest.Server, journalPath string) *CraftEgressAdapter {
	t.Helper()
	adapter, err := NewCraftEgressAdapter(CraftEgressAdapterConfig{
		GatewayBaseURL: gateway.URL + "/craft/model-gateway",
		Credential:     "craft-execution-credential-test",
		JournalPath:    journalPath,
		// The mock gateway is an httptest loopback listener; the production
		// dial-time private-IP refusal would correctly reject it.
		AllowPrivateTarget: true,
	})
	if err != nil {
		t.Fatalf("adapter construction failed: %v", err)
	}
	return adapter
}

func postChat(t *testing.T, adapter *CraftEgressAdapter, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)
	return rec
}

func journalLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("journal unreadable: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("journal line is not JSON: %v", err)
		}
		out = append(out, entry)
	}
	return out
}

func TestAdapterMintsActivityIDBeforeForwardAndJournalsIt(t *testing.T) {
	gateway := &mockGateway{}
	server := httptest.NewServer(gateway)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter := newAdapterForTest(t, server, path)

	rec := postChat(t, adapter, `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	ids := gateway.activities()
	if len(ids) != 1 || ids[0] == "" {
		t.Fatalf("gateway must receive exactly one non-empty X-Craft-Activity-ID, got %v", ids)
	}
	if !validActivityID(ids[0]) {
		t.Fatalf("minted id %q violates the gateway charset contract", ids[0])
	}
	entries := journalLines(t, path)
	if len(entries) < 1 {
		t.Fatalf("at least the mint record expected, got %d", len(entries))
	}
	if entries[0]["attempt_id"] != ids[0] {
		t.Fatalf("journal attempt_id %v must equal forwarded header %q", entries[0]["attempt_id"], ids[0])
	}
	if entries[0]["state"] != "unresolved" {
		t.Fatalf("the mint record must be unresolved (persisted before the forward), got %v", entries[0]["state"])
	}
	// A fully delivered response resolves the attempt durably.
	terminal := false
	for _, entry := range journalLines(t, path) {
		if entry["attempt_id"] == ids[0] && entry["state"] == "resolved" {
			terminal = true
		}
	}
	if !terminal {
		t.Fatal("a completed forward must append a resolved record")
	}
	if got := gateway.auths(); len(got) != 1 || got[0] != "Bearer craft-execution-credential-test" {
		t.Fatalf("credential must be forwarded verbatim, got %v", got)
	}
}

func validActivityID(id string) bool {
	if id == "" || len(id) > 128 || id != strings.TrimSpace(id) {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("._:-", r):
		default:
			return false
		}
	}
	return true
}

func TestAdapterFailsClosedWhenJournalCannotPersist(t *testing.T) {
	gateway := &mockGateway{}
	server := httptest.NewServer(gateway)
	defer server.Close()
	// Journal path sits under a directory that is actually a file: replay and
	// append both fail, so no forward may ever leave the adapter.
	blockedDir := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter, err := NewCraftEgressAdapter(CraftEgressAdapterConfig{
		GatewayBaseURL: server.URL + "/craft/model-gateway",
		Credential:     "cred",
		JournalPath:    filepath.Join(blockedDir, "attempts.jsonl"),
	})
	if err == nil {
		adapter.Close()
		t.Fatal("an unpersistable journal must fail adapter construction")
	}
	if len(gateway.activities()) != 0 {
		t.Fatal("no forward may happen without a durable journal")
	}
}

func TestAdapterReusesActivityIDAfterUnknownAndMintsNewAfterResolution(t *testing.T) {
	gateway := &mockGateway{failBody: true, status: http.StatusBadGateway, body: `{"error":{"code":"ACTIVITY_UNRESOLVED"}}`}
	server := httptest.NewServer(gateway)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter := newAdapterForTest(t, server, path)
	body := `{"model":"m1","messages":[{"role":"user","content":"same logical call"}]}`

	first := postChat(t, adapter, body)
	if first.Code != http.StatusBadGateway {
		t.Fatalf("a lost response body must surface 502, got %d", first.Code)
	}
	second := postChat(t, adapter, body)
	if second.Code != http.StatusBadGateway {
		t.Fatalf("retry still loses the body, got %d", second.Code)
	}
	ids := gateway.activities()
	if len(ids) != 2 || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("retry after unknown must reuse the SAME activity id, got %v", ids)
	}

	// The transport recovers: the same logical call now completes.
	gateway.mu.Lock()
	gateway.failBody = false
	gateway.status = http.StatusOK
	gateway.mu.Unlock()
	third := postChat(t, adapter, body)
	if third.Code != http.StatusOK {
		t.Fatalf("recovered forward must succeed, got %d body=%s", third.Code, third.Body.String())
	}
	recovered := gateway.activities()
	if len(recovered) != 3 || recovered[2] != ids[0] {
		t.Fatalf("recovered retry must keep the same id until resolution, got %v", recovered)
	}

	// A deliberately new model attempt (same body, previous resolved) mints a new id.
	fourth := postChat(t, adapter, body)
	if fourth.Code != http.StatusOK {
		t.Fatalf("new attempt must succeed, got %d", fourth.Code)
	}
	final := gateway.activities()
	if len(final) != 4 || final[3] == final[2] {
		t.Fatalf("a resolved activity followed by a new request must mint a NEW id, got %v", final)
	}
}

func TestAdapterJournalSurvivesRestartAndReusesUnresolvedID(t *testing.T) {
	gateway := &mockGateway{failBody: true, status: http.StatusBadGateway, body: `{"error":{"code":"ACTIVITY_UNRESOLVED"}}`}
	server := httptest.NewServer(gateway)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	first := newAdapterForTest(t, server, path)
	if rec := postChat(t, first, `{"model":"m1","messages":[]}`); rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	first.Close()

	// Restart: a fresh adapter over the same journal must know the attempt.
	restarted := newAdapterForTest(t, server, path)
	defer restarted.Close()
	gateway.mu.Lock()
	gateway.failBody = false
	gateway.status = http.StatusOK
	gateway.mu.Unlock()
	rec := postChat(t, restarted, `{"model":"m1","messages":[]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("restart retry must complete, got %d body=%s", rec.Code, rec.Body.String())
	}
	ids := gateway.activities()
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatalf("restart must reuse the journaled unresolved id, got %v", ids)
	}
}

func TestAdapterPassesGatewayConflictThroughWithoutMinting(t *testing.T) {
	gateway := &mockGateway{status: http.StatusConflict, body: `{"error":{"code":"ACTIVITY_UNRESOLVED"}}`}
	server := httptest.NewServer(gateway)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter := newAdapterForTest(t, server, path)
	defer adapter.Close()
	body := `{"model":"m1","messages":[]}`

	first := postChat(t, adapter, body)
	if first.Code != http.StatusConflict {
		t.Fatalf("gateway 409 must pass through, got %d", first.Code)
	}
	second := postChat(t, adapter, body)
	if second.Code != http.StatusConflict {
		t.Fatalf("parked activity keeps answering 409, got %d", second.Code)
	}
	ids := gateway.activities()
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatalf("409 must not trigger a new id, got %v", ids)
	}
}

func TestGatewayOutcomeDefinitivePolicy(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "non-5xx response", status: http.StatusOK, body: `{"ok":true}`, want: true},
		{name: "known not-started envelope remains uncertain", status: http.StatusBadGateway, body: `{"success":false,"error":{"code":"UPSTREAM_ERROR","message":"the activity never started"}}`, want: false},
		{name: "full appFail terminal envelope cannot prove origin", status: http.StatusBadGateway, body: `{"success":false,"error":{"code":"UPSTREAM_ERROR","message":"the upstream request failed"}}`, want: false},
		{name: "missing success field is not terminal proof", status: http.StatusBadGateway, body: `{"error":{"code":"UPSTREAM_ERROR","message":"failed"}}`, want: false},
		{name: "success true lookalike is not terminal proof", status: http.StatusBadGateway, body: `{"success":true,"error":{"code":"UPSTREAM_ERROR","message":"failed"}}`, want: false},
		{name: "missing message is not terminal proof", status: http.StatusBadGateway, body: `{"success":false,"error":{"code":"UPSTREAM_ERROR"}}`, want: false},
		{name: "empty message is not terminal proof", status: http.StatusBadGateway, body: `{"success":false,"error":{"code":"UPSTREAM_ERROR","message":""}}`, want: false},
		{name: "unresolved envelope stays parked", status: http.StatusBadGateway, body: `{"error":{"code":"ACTIVITY_UNRESOLVED"}}`, want: false},
		{name: "bare 409 is a definitive upstream response", status: http.StatusConflict, body: `{"error":{"code":"OTHER"}}`, want: true},
		{name: "bare 502 stays parked", status: http.StatusBadGateway, body: `gateway error`, want: false},
		{name: "503 cannot prove gateway outcome", status: http.StatusServiceUnavailable, body: `{"error":"upstream failure"}`, want: false},
		{name: "504 proxy JSON cannot prove gateway outcome", status: http.StatusGatewayTimeout, body: `{"error":"upstream failure"}`, want: false},
		{name: "UPSTREAM_ERROR at another status remains uncertain", status: http.StatusServiceUnavailable, body: `{"error":{"code":"UPSTREAM_ERROR"}}`, want: false},
		{name: "non-envelope 502 JSON is uncertain", status: http.StatusBadGateway, body: `{"error":"proxy timeout"}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gatewayOutcomeIsDefinitive(tt.status, []byte(tt.body)); got != tt.want {
				t.Fatalf("gatewayOutcomeIsDefinitive(%d, %q) = %v, want %v", tt.status, tt.body, got, tt.want)
			}
		})
	}
}

func TestAdapterComplete502EnvelopeKeepsAttemptParked(t *testing.T) {
	for _, tt := range []struct {
		name         string
		responseBody string
	}{
		{name: "actual gateway appFail shape", responseBody: `{"success":false,"error":{"code":"UPSTREAM_ERROR","message":"the activity never started"}}`},
		{name: "missing success", responseBody: `{"error":{"code":"UPSTREAM_ERROR","message":"proxy error"}}`},
		{name: "success true", responseBody: `{"success":true,"error":{"code":"UPSTREAM_ERROR","message":"proxy error"}}`},
		{name: "missing message", responseBody: `{"success":false,"error":{"code":"UPSTREAM_ERROR"}}`},
		{name: "empty message", responseBody: `{"success":false,"error":{"code":"UPSTREAM_ERROR","message":""}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &mockGateway{status: http.StatusBadGateway, body: tt.responseBody}
			server := httptest.NewServer(gateway)
			defer server.Close()
			adapter := newAdapterForTest(t, server, filepath.Join(t.TempDir(), "attempts.jsonl"))
			defer adapter.Close()
			body := `{"model":"m1","messages":[{"role":"user","content":"envelope shape"}]}`
			if got := postChat(t, adapter, body); got.Code != http.StatusBadGateway {
				t.Fatalf("first response must pass through as 502, got %d", got.Code)
			}
			gateway.mu.Lock()
			gateway.status = http.StatusOK
			gateway.mu.Unlock()
			if got := postChat(t, adapter, body); got.Code != http.StatusOK {
				t.Fatalf("retry must complete, got %d", got.Code)
			}
			ids := gateway.activities()
			if len(ids) != 2 {
				t.Fatalf("expected two physical sends, got %v", ids)
			}
			same := ids[0] == ids[1]
			if !same {
				t.Fatalf("complete 502 response must keep retry on the same attempt id; ids=%v", ids)
			}
		})
	}
}

func TestAdapterProxy5xxJSONKeepsAttemptParked(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
	}{
		{status: http.StatusBadGateway, body: `{"error":"proxy timeout"}`},
		{status: http.StatusBadGateway, body: `{"error":{"code":"UPSTREAM_ERROR"}}`},
		{status: http.StatusGatewayTimeout, body: `{"error":"upstream timed out"}`},
	} {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			gateway := &mockGateway{status: tt.status, body: tt.body}
			server := httptest.NewServer(gateway)
			defer server.Close()
			adapter := newAdapterForTest(t, server, filepath.Join(t.TempDir(), "attempts.jsonl"))
			defer adapter.Close()
			body := `{"model":"m1","messages":[{"role":"user","content":"proxy error"}]}`
			if got := postChat(t, adapter, body); got.Code != tt.status {
				t.Fatalf("proxy response must pass through, got %d want %d", got.Code, tt.status)
			}
			gateway.mu.Lock()
			gateway.status = http.StatusOK
			gateway.mu.Unlock()
			if got := postChat(t, adapter, body); got.Code != http.StatusOK {
				t.Fatalf("retry must complete, got %d", got.Code)
			}
			ids := gateway.activities()
			if len(ids) != 2 || ids[0] != ids[1] {
				t.Fatalf("proxy %d JSON response must keep retry on the same attempt id, got %v", tt.status, ids)
			}
		})
	}
}

func TestAdapterTornBodyResolutionIsStatusSensitive(t *testing.T) {
	for _, tt := range []struct {
		status       int
		wantReissued bool
	}{
		{status: http.StatusOK, wantReissued: true},
		{status: http.StatusConflict, wantReissued: false},
		{status: http.StatusBadGateway, wantReissued: false},
		{status: http.StatusServiceUnavailable, wantReissued: false},
		{status: http.StatusGatewayTimeout, wantReissued: false},
	} {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			gateway := &mockGateway{failBody: true, status: tt.status, body: `{"error":{"code":"UPSTREAM_ERROR"}}`}
			server := httptest.NewServer(gateway)
			defer server.Close()
			adapter := newAdapterForTest(t, server, filepath.Join(t.TempDir(), "attempts.jsonl"))
			defer adapter.Close()
			body := `{"model":"m1","messages":[{"role":"user","content":"torn response"}]}`
			first := postChat(t, adapter, body)
			if first.Code != http.StatusBadGateway {
				t.Fatalf("torn response must surface 502, got %d", first.Code)
			}
			gateway.mu.Lock()
			gateway.failBody = false
			gateway.status = http.StatusOK
			gateway.mu.Unlock()
			if got := postChat(t, adapter, body); got.Code != http.StatusOK {
				t.Fatalf("retry must complete, got %d", got.Code)
			}
			ids := gateway.activities()
			if len(ids) != 2 {
				t.Fatalf("expected two physical attempts, got %v", ids)
			}
			same := ids[0] == ids[1]
			if same == tt.wantReissued {
				t.Fatalf("status %d: retry ID reuse = %v, want reissued=%v; ids=%v", tt.status, same, tt.wantReissued, ids)
			}
		})
	}
}

func TestAdapterLogsTornBodyResolutionFailure(t *testing.T) {
	var adapter *CraftEgressAdapter
	var logs bytes.Buffer
	log := logrus.New()
	log.SetOutput(&logs)
	log.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})
	log.SetLevel(logrus.ErrorLevel)

	var attemptID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptID = r.Header.Get(craftModelActivityHeader)
		// Mint has already been persisted before forwarding. Closing the journal
		// here makes the subsequent Resolve append fail deterministically.
		if err := adapter.journal.file.Close(); err != nil {
			t.Errorf("close journal file to induce Resolve failure: %v", err)
		}
		body := `{"ok":true}`
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte(body[:len(body)/2]))
		panic(http.ErrAbortHandler)
	}))
	defer server.Close()

	adapter = newAdapterForTest(t, server, filepath.Join(t.TempDir(), "attempts.jsonl"))
	defer adapter.Close()
	requestBody := `{"model":"m1","messages":[{"role":"user","content":"torn resolve failure"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), types.LoggerContextKey, logrus.NewEntry(log)))
	recorder := httptest.NewRecorder()
	adapter.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("torn response must remain HTTP 502, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if attemptID == "" {
		t.Fatal("gateway must receive an attempt ID")
	}
	for _, want := range []string{attemptID, "definitive=true", "gateway_status=200", "error="} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("journal resolution failure log missing %q: %s", want, logs.String())
		}
	}
}

func TestAdapterJournalNeverContainsCredentialMaterial(t *testing.T) {
	gateway := &mockGateway{}
	server := httptest.NewServer(gateway)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter := newAdapterForTest(t, server, path)
	if rec := postChat(t, adapter, `{"model":"m1","messages":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	adapter.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "craft-execution-credential-test") {
		t.Fatal("the durable journal must never contain credential material")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("journal must be owner-only, got %v", info.Mode().Perm())
	}
}

func TestAdapterRejectsUnsafeTargets(t *testing.T) {
	if _, err := NewCraftEgressAdapter(CraftEgressAdapterConfig{
		GatewayBaseURL: "ftp://gateway.internal/craft",
		Credential:     "cred",
		JournalPath:    filepath.Join(t.TempDir(), "a.jsonl"),
	}); err == nil {
		t.Fatal("non-http(s) gateway schemes must be rejected at construction")
	}
	gateway := &mockGateway{}
	server := httptest.NewServer(gateway)
	defer server.Close()
	adapter := newAdapterForTest(t, server, filepath.Join(t.TempDir(), "a.jsonl"))
	defer adapter.Close()
	req := httptest.NewRequest(http.MethodPost, "/../admin/secret", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	adapter.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("path traversal must be refused, got %d", rec.Code)
	}
	if len(gateway.activities()) != 0 {
		t.Fatal("no forward may happen for a refused path")
	}
}
