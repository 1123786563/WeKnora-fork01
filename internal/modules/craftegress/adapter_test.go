package craftegress

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
	w.WriteHeader(status)
	if failBody {
		// Panic with http.ErrAbortHandler drops the connection mid-response.
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
	gateway := &mockGateway{failBody: true}
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
	gateway := &mockGateway{failBody: true}
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
