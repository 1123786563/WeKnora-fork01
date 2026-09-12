package connectorcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testAdminSecret = "sec-test-admin-DO-NOT-LEAK"

type recordedRequest struct {
	method string
	path   string
	auth   string
	body   []byte
}

type adminHandlerFunc func(w http.ResponseWriter, r *http.Request, body []byte)

func newAdminTestServer(t *testing.T, h adminHandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		body, _ := io.ReadAll(r.Body)
		h(w, r, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

func testClientConfig(baseURL string, secret AdminSecret) AdminClientConfig {
	return AdminClientConfig{
		BaseURL:     baseURL,
		Allowlist:   []string{baseURL},
		AdminSecret: secret,
		Timeout:     2 * time.Second,
	}
}

func mustClient(t *testing.T, cfg AdminClientConfig) *RuntimeAdminClient {
	t.Helper()
	c, err := NewRuntimeAdminClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAdminClientRejectsUnlistedAndPublicAddresses(t *testing.T) {
	// With the DEFAULT (internal-only) allowlist, public URLs are rejected.
	if _, err := NewRuntimeAdminClient(AdminClientConfig{BaseURL: "https://api.github.com", AdminSecret: staticSecret("s")}); err == nil {
		t.Fatal("public url accepted by the default internal allowlist")
	}
	// With an explicit allowlist, only its exact members are accepted.
	allow := []string{"http://10.0.0.5:8080"}
	cases := []struct {
		name    string
		baseURL string
	}{
		{"unlisted internal", "http://10.0.0.9:8080"},
		{"different port", "http://10.0.0.5:8081"},
		{"scheme change", "https://10.0.0.5:8080"},
		{"userinfo", "http://user:pw@10.0.0.5:8080"},
		{"query", "http://10.0.0.5:8080?x=1"},
		{"fragment", "http://10.0.0.5:8080#f"},
		{"path prefix", "http://10.0.0.5:8080/oc"},
		{"not a url", "garbage"},
	}
	for _, tc := range cases {
		if _, err := NewRuntimeAdminClient(AdminClientConfig{BaseURL: tc.baseURL, Allowlist: allow, AdminSecret: staticSecret("s")}); err == nil {
			t.Fatalf("%s: address accepted: %s", tc.name, tc.baseURL)
		}
	}
	// The static default allowlist accepts its own entries (normalized).
	if _, err := NewRuntimeAdminClient(AdminClientConfig{BaseURL: "http://open-connector:8080/", AdminSecret: staticSecret("s")}); err != nil {
		t.Fatalf("default allowlist must accept the internal runtime address: %v", err)
	}
	// An explicit allowlist accepts its own member.
	if _, err := NewRuntimeAdminClient(AdminClientConfig{BaseURL: "http://10.0.0.5:8080", Allowlist: allow, AdminSecret: staticSecret("s")}); err != nil {
		t.Fatalf("allowlist member rejected: %v", err)
	}
}

func TestAdminClientCreateTokenSendsFrozenGrantBody(t *testing.T) {
	var got recordedRequest
	srv, count := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		got = recordedRequest{r.Method, r.URL.Path, r.Header.Get("Authorization"), body}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"token":"oct_MATERIAL","record":{"id":"rtt_1"}}`)
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	created, err := c.CreateRuntimeToken(context.Background(), CreateTokenRequest{
		Name: "weknora-7-conn1-v1", ExternalID: "ext-1", Actions: []string{"github.get_profile"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TokenRecordID != "rtt_1" || created.Token != "oct_MATERIAL" {
		t.Fatalf("created = %+v", created)
	}
	if atomic.LoadInt32(count) != 1 {
		t.Fatalf("requests = %d, want exactly 1", *count)
	}
	if got.method != "POST" || got.path != "/api/runtime-tokens" {
		t.Fatalf("wire = %s %s", got.method, got.path)
	}
	if got.auth != "Bearer "+testAdminSecret {
		t.Fatal("admin bearer missing")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	// Exactly the four frozen camelCase arrays plus name; NO blockedProxies.
	for _, key := range []string{"name", "allowedConnections", "allowedActions", "allowedProxies", "blockedActions"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("body missing %s: %s", key, got.body)
		}
	}
	if _, ok := body["blockedProxies"]; ok {
		t.Fatal("body must not carry blockedProxies")
	}
	var conns, actions []string
	json.Unmarshal(body["allowedConnections"], &conns)
	json.Unmarshal(body["allowedActions"], &actions)
	if len(conns) != 1 || conns[0] != "ext-1" {
		t.Fatalf("allowedConnections = %v", conns)
	}
	if len(actions) != 1 || actions[0] != "github.get_profile" {
		t.Fatalf("allowedActions = %v", actions)
	}
}

func TestAdminClientCreateTokenRejectsEmptyGrantBeforeHTTP(t *testing.T) {
	srv, count := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		t.Error("server must not be reached")
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	for name, req := range map[string]CreateTokenRequest{
		"no actions":       {Name: "n", ExternalID: "ext-1"},
		"blank connection": {Name: "n", ExternalID: "", Actions: []string{"a.b"}},
	} {
		_, err := c.CreateRuntimeToken(context.Background(), req)
		if err == nil {
			t.Fatalf("%s: empty grant accepted", name)
		}
		if !isPermanentError(err) {
			t.Fatalf("%s: must be a permanent rejection: %v", name, err)
		}
	}
	if atomic.LoadInt32(count) != 0 {
		t.Fatalf("empty grant must cost zero HTTP calls, got %d", *count)
	}
}

func TestAdminClientRevokeUsesDeleteNeverEmptyAllowlist(t *testing.T) {
	var method, path string
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"rtt_1","revoked":true}`)
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	if err := c.RevokeRuntimeToken(context.Background(), "rtt_1"); err != nil {
		t.Fatal(err)
	}
	if method != "DELETE" || path != "/api/runtime-tokens/rtt_1" {
		t.Fatalf("revoke wire = %s %s (must be DELETE)", method, path)
	}
	// 404 runtime_token_not_found is idempotent success.
	srv2, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"code":"runtime_token_not_found","message":"nope"}}`)
	})
	c2 := mustClient(t, testClientConfig(srv2.URL, staticSecret(testAdminSecret)))
	if err := c2.RevokeRuntimeToken(context.Background(), "rtt_gone"); err != nil {
		t.Fatalf("revoking a missing token must be idempotent: %v", err)
	}
}

func TestAdminClientNeverFollowsRedirects(t *testing.T) {
	var leaked int32
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		switch r.URL.Path {
		case "/api/runtime-tokens":
			w.Header().Set("Location", "/evil")
			w.WriteHeader(http.StatusFound)
		case "/evil":
			atomic.AddInt32(&leaked, 1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	_, err := c.CreateRuntimeToken(context.Background(), CreateTokenRequest{Name: "n", ExternalID: "e", Actions: []string{"a.b"}})
	var ae *AdminError
	if !errors.As(err, &ae) || ae.Status != http.StatusFound {
		t.Fatalf("redirect must surface as an error, got %v", err)
	}
	if atomic.LoadInt32(&leaked) != 0 {
		t.Fatal("client followed a redirect and leaked the Authorization header")
	}
}

func TestAdminClientSingleAttemptPerOperation(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":{"code":"internal_error"}}`)
	}))
	defer srv.Close()
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	err := c.RevokeRuntimeToken(context.Background(), "rtt_1")
	var ae *AdminError
	if !errors.As(err, &ae) || ae.Status != 500 || ae.Code != "internal_error" {
		t.Fatalf("expected typed 500 error, got %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("client retried: %d requests for one operation", hits)
	}
}

func TestAdminClientErrorsCarryNoSecrets(t *testing.T) {
	var errStrings []string

	// transport failure
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()
	c1 := mustClient(t, testClientConfig(closed.URL, staticSecret(testAdminSecret)))
	_, err := c1.CreateRuntimeToken(context.Background(), CreateTokenRequest{Name: "n", ExternalID: "e", Actions: []string{"a.b"}})
	errStrings = append(errStrings, err.Error())

	// server 500 + 404 + malformed 2xx + oversize
	modes := []struct {
		status int
		body   string
	}{{500, `{"error":{"code":"internal_error"}}`}, {404, `{"error":{"code":"runtime_token_not_found"}}`}, {200, "not-json"}, {200, strings.Repeat("x", AdminMaxResponseBytes+2)}}
	for _, m := range modes {
		srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
			w.WriteHeader(m.status)
			io.WriteString(w, m.body)
		})
		c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
		_, err := c.CreateRuntimeToken(context.Background(), CreateTokenRequest{Name: "n", ExternalID: "e", Actions: []string{"a.b"}})
		if err == nil {
			t.Fatalf("mode %d must fail", m.status)
		}
		errStrings = append(errStrings, err.Error())
	}

	for _, s := range errStrings {
		if strings.Contains(s, testAdminSecret) || strings.Contains(s, "oct_MATERIAL") {
			t.Fatalf("error leaks secret material: %q", s)
		}
	}
}

func TestAdminClientSecretUnavailableFailsClosed(t *testing.T) {
	srv, count := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		t.Error("server must not be reached")
	})
	// API-process-style environment: the admin secret mount does not exist.
	missing := FileAdminSecretSource(filepath.Join(t.TempDir(), "absent", "connector-admin-token"))
	c := mustClient(t, testClientConfig(srv.URL, missing))
	ctx := context.Background()
	if _, err := c.CreateRuntimeToken(ctx, CreateTokenRequest{Name: "n", ExternalID: "e", Actions: []string{"a.b"}}); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("create: %v", err)
	}
	if err := c.RevokeRuntimeToken(ctx, "rtt_1"); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := c.StartAuthorization(ctx, "github", "alias-1"); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("authorize: %v", err)
	}
	if atomic.LoadInt32(count) != 0 {
		t.Fatalf("fail-closed client performed %d HTTP calls", *count)
	}
}

func TestAdminClientFileSecretSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "connector-admin-token")
	if err := os.WriteFile(path, []byte("  sec-file-value \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := FileAdminSecretSource(path)
	v, err := src(context.Background())
	if err != nil || v != "sec-file-value" {
		t.Fatalf("file source = %q, %v", v, err)
	}
	// missing mount
	if _, err := FileAdminSecretSource(filepath.Join(dir, "nope"))(context.Background()); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("missing mount must fail closed: %v", err)
	}
	// empty file
	empty := filepath.Join(dir, "empty")
	os.WriteFile(empty, []byte("  "), 0o600)
	if _, err := FileAdminSecretSource(empty)(context.Background()); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("empty mount must fail closed: %v", err)
	}
	// world-readable file
	world := filepath.Join(dir, "world")
	os.WriteFile(world, []byte("sec"), 0o644)
	if _, err := FileAdminSecretSource(world)(context.Background()); !errors.Is(err, ErrAdminSecretUnavailable) {
		t.Fatalf("insecure permissions must fail closed: %v", err)
	}
}

func TestAdminClientStartAuthorizationAndLookup(t *testing.T) {
	var method, path string
	var body []byte
	srv, _ := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, raw []byte) {
		method, path = r.Method, r.URL.Path
		body = raw
		if r.URL.Path == "/api/oauth/authorizations" {
			io.WriteString(w, `{"authorizationUrl":"https://ext/auth","state":"st-9"}`)
			return
		}
		io.WriteString(w, `{"success":true,"message":"OK","data":{"id":"ext-1","alias":"alias-1","providerAccountId":"acct-9","service":"github","status":"active"},"meta":{}}`)
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	ctx := context.Background()
	start, err := c.StartAuthorization(ctx, "github", "alias-1")
	if err != nil || start.URL != "https://ext/auth" || start.State != "st-9" {
		t.Fatalf("start = %+v, %v", start, err)
	}
	if method != "POST" || path != "/api/oauth/authorizations" {
		t.Fatalf("authorize wire = %s %s", method, path)
	}
	var sent map[string]string
	json.Unmarshal(body, &sent)
	if sent["service"] != "github" || sent["connectionName"] != "alias-1" {
		t.Fatalf("authorize body = %v", sent)
	}

	conn, err := c.LookupRuntimeConnection(ctx, "ext-1")
	if err != nil || conn.Alias != "alias-1" || conn.ProviderAccountID != "acct-9" || conn.Provider != "github" {
		t.Fatalf("lookup = %+v, %v", conn, err)
	}
	if method != "GET" || path != "/v1/connections/by-id/ext-1" {
		t.Fatalf("lookup wire = %s %s", method, path)
	}
	// blank arguments are permanent rejections before HTTP
	if _, err := c.StartAuthorization(ctx, "", "alias"); !isPermanentError(err) {
		t.Fatalf("blank service: %v", err)
	}
	if _, err := c.LookupRuntimeConnection(ctx, " "); !isPermanentError(err) {
		t.Fatalf("blank id: %v", err)
	}
}

func TestAdminClientDeleteConnectionUnpinnedFailsClosed(t *testing.T) {
	srv, count := newAdminTestServer(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		t.Error("server must not be reached")
	})
	c := mustClient(t, testClientConfig(srv.URL, staticSecret(testAdminSecret)))
	if err := c.DeleteRuntimeConnection(context.Background(), "ext-1"); !errors.Is(err, ErrUnpinnedEndpoint) {
		t.Fatalf("delete_connection must fail closed: %v", err)
	}
	if atomic.LoadInt32(count) != 0 {
		t.Fatal("unpinned endpoint performed HTTP")
	}
}

func TestFileSecretSinkStoresMaterialUnderHashedRef(t *testing.T) {
	dir := t.TempDir()
	sink, err := NewFileSecretSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := sink.PutSecret(ctx, "oc/runtime-token/7/conn1/1", "oct_A"); err != nil {
		t.Fatal(err)
	}
	if err := sink.PutSecret(ctx, "oc/runtime-token/7/conn1/1", "oct_B"); err != nil {
		t.Fatal(err)
	}
	// hostile ref must not traverse out of the dir
	if err := sink.PutSecret(ctx, "../../etc/passwd", "x"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 secret files (overwrite + hostile ref), got %d", len(entries))
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("secret file %s too open: %v", e.Name(), info.Mode())
		}
		if strings.Contains(e.Name(), "runtime-token") || strings.Contains(e.Name(), "..") {
			t.Fatalf("secret filename leaks the ref: %s", e.Name())
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) == "oct_A" {
			t.Fatal("re-put must overwrite, not duplicate")
		}
	}
}

func TestParseAllowlist(t *testing.T) {
	if got := ParseAllowlist(" a , ,http://x:1 ,, "); len(got) != 2 || got[0] != "a" || got[1] != "http://x:1" {
		t.Fatalf("ParseAllowlist = %v", got)
	}
	if got := ParseAllowlist("  "); got != nil {
		t.Fatalf("empty list = %v", got)
	}
}
