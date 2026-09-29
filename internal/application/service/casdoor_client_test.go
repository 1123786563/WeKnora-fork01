package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

func newCasdoorTestServer(t *testing.T, ropcToken string, existingUsers map[string]bool) (*httptest.Server, *int, *int) {
	t.Helper()
	ropcCalls, addUserCalls := 0, 0
	mux := http.NewServeMux()
	// ROPC:管理员与普通用户共用同一端点
	mux.HandleFunc("/api/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		ropcCalls++
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.FormValue("username") == "svc_weknora" {
			_, _ = w.Write([]byte(`{"access_token":"admin-token","expires_in":"7200"}`))
			return
		}
		if r.FormValue("grant_type") != "password" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unsupported_grant_type"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"` + ropcToken + `"}`))
	})
	mux.HandleFunc("/api/add-user", func(w http.ResponseWriter, r *http.Request) {
		addUserCalls++
		_ = r.ParseForm()
		name := r.FormValue("name")
		if existingUsers[name] {
			_, _ = w.Write([]byte(`{"status":"error","msg":"user already exists"}`))
			return
		}
		existingUsers[name] = true
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/api/set-password", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id != "weknora/wx_abcd1234" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &ropcCalls, &addUserCalls
}

func TestCasdoorClientPasswordToken(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	srv, ropcCalls, _ := newCasdoorTestServer(t, "tok-1", map[string]bool{})
	c := newCasdoorClient(casdoorCfg(srv.URL), "web-client", "web-secret")
	tok, err := c.PasswordToken(context.Background(), "wx_abcd1234", "pw")
	if err != nil || tok != "tok-1" {
		t.Fatalf("got %q, %v", tok, err)
	}
	if *ropcCalls != 1 {
		t.Fatalf("expected 1 ropc call, got %d", *ropcCalls)
	}
}

func TestCasdoorClientEnsureUserIdempotent(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	srv, _, addUserCalls := newCasdoorTestServer(t, "tok-1", map[string]bool{"wx_abcd1234": true})
	c := newCasdoorClient(casdoorCfg(srv.URL), "web-client", "web-secret")
	if err := c.EnsureUser(context.Background(), "wx_abcd1234", "wx_abcd1234@wechat.local", "pw"); err != nil {
		t.Fatalf("existing user should fall through to set-password, got %v", err)
	}
	if err := c.EnsureUser(context.Background(), "wx_new", "wx_new@wechat.local", "pw"); err != nil {
		t.Fatalf("new user should be created, got %v", err)
	}
	if *addUserCalls != 2 {
		t.Fatalf("expected 2 add-user calls, got %d", *addUserCalls)
	}
}

func casdoorCfg(baseURL string) *config.CasdoorAdminConfig {
	return &config.CasdoorAdminConfig{
		BaseURL:       baseURL,
		OrgName:       "weknora",
		AdminUsername: "svc_weknora",
		AdminPassword: "svc-pw",
	}
}
