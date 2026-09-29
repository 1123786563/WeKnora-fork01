package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

// casdoorFakeServer 复刻 casbin/casdoor 真实契约,消除自证式 mock:
//
//   - /api/* 管理端点一律要求 Authorization: Bearer(管理 token 由 ROPC
//     用 weknora/svc_weknora 真实换取),匿名/错误 token 返回
//     HTTP 200 + {"status":"error","msg":"Unauthorized operation"};
//   - /api/add-user 只认 JSON body(json.Unmarshal),不认 form 字段;
//   - /api/set-password 只读 userOwner/userName/newPassword 表单字段,
//     ?id= 参数无效;
//   - /api/get-user 以 status ok + 用户字段报告存在性;
//   - /api/login/oauth/access_token(ROPC)接受 form 参数(上游现状)。
type casdoorFakeServer struct {
	srv *httptest.Server

	mu            sync.Mutex
	userToken     string // ROPC 发给普通用户的 token
	ropcCalls     int
	addUserCalls  int
	getUserCalls  int
	setPwdCalls   int
	addUserAuth   string
	addUserCT     string
	addUserBody   map[string]string
	setPwdAuth    string
	setPwdForm    map[string]string
	setPwdQuery   string
	existingUsers map[string]string // name -> password
	// forceAddUserError 非空时,对不存在用户的 add-user 也失败,
	// 用于验证原始错误不被吞掉。
	forceAddUserError string
}

func newCasdoorTestServer(t *testing.T, existingUsers map[string]string) *casdoorFakeServer {
	t.Helper()
	f := &casdoorFakeServer{userToken: "tok-1", existingUsers: existingUsers}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/oauth/access_token", f.handleROPC)
	mux.HandleFunc("/api/add-user", f.handleAddUser)
	mux.HandleFunc("/api/get-user", f.handleGetUser)
	mux.HandleFunc("/api/set-password", f.handleSetPassword)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *casdoorFakeServer) handleROPC(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.ropcCalls++
	f.mu.Unlock()
	_ = r.ParseForm()
	w.Header().Set("Content-Type", "application/json")
	if r.FormValue("grant_type") != "password" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"unsupported_grant_type"}`))
		return
	}
	// 用户名按 org/name 拼接(PasswordToken 发送 weknora/svc_weknora)
	if r.FormValue("username") == "weknora/svc_weknora" && r.FormValue("password") == "svc-pw" {
		_, _ = w.Write([]byte(`{"access_token":"admin-token","expires_in":"7200"}`))
		return
	}
	_, _ = w.Write([]byte(`{"access_token":"` + f.userToken + `"}`))
}

// requireAdmin 复刻 ApiFilter:匿名/无效凭证的管理调用返回 200 + 错误体。
func (f *casdoorFakeServer) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") == "Bearer admin-token" {
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"error","msg":"Unauthorized operation"}`))
	return false
}

func (f *casdoorFakeServer) handleAddUser(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.addUserCalls++
	w.Header().Set("Content-Type", "application/json")
	if !f.requireAdmin(w, r) {
		return
	}
	f.addUserAuth = r.Header.Get("Authorization")
	f.addUserCT = r.Header.Get("Content-Type")
	if !strings.HasPrefix(f.addUserCT, "application/json") {
		_, _ = w.Write([]byte(`{"status":"error","msg":"add-user requires a JSON body"}`))
		return
	}
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		_, _ = w.Write([]byte(`{"status":"error","msg":"invalid JSON body"}`))
		return
	}
	f.addUserBody = payload
	for _, field := range []string{"owner", "name", "email", "password"} {
		if payload[field] == "" {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"error","msg":"missing field %s"}`, field)))
			return
		}
	}
	if payload["owner"] != "weknora" {
		_, _ = w.Write([]byte(`{"status":"error","msg":"user's owner is not allowed"}`))
		return
	}
	if _, ok := f.existingUsers[payload["name"]]; ok {
		_, _ = w.Write([]byte(`{"status":"error","msg":"user already exists"}`))
		return
	}
	if f.forceAddUserError != "" {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"error","msg":%q}`, f.forceAddUserError)))
		return
	}
	f.existingUsers[payload["name"]] = payload["password"]
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ok","name":%q,"data":"Affected 1 rows"}`, payload["name"])))
}

func (f *casdoorFakeServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getUserCalls++
	w.Header().Set("Content-Type", "application/json")
	if !f.requireAdmin(w, r) {
		return
	}
	id := r.URL.Query().Get("id")
	if !strings.HasPrefix(id, "weknora/") {
		_, _ = w.Write([]byte(`{"status":"error","msg":"invalid id"}`))
		return
	}
	name := strings.TrimPrefix(id, "weknora/")
	pwd, ok := f.existingUsers[name]
	if !ok {
		_, _ = w.Write([]byte(`{"status":"error","msg":"user not found"}`))
		return
	}
	// 对齐真实 get-user 响应形状:status ok + 用户字段(顶层与 data 各一份)
	resp := map[string]any{
		"status": "ok",
		"sub":    "",
		"name":   name,
		"data":   map[string]string{"owner": "weknora", "name": name, "password": pwd},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *casdoorFakeServer) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setPwdCalls++
	w.Header().Set("Content-Type", "application/json")
	if !f.requireAdmin(w, r) {
		return
	}
	f.setPwdAuth = r.Header.Get("Authorization")
	f.setPwdQuery = r.URL.RawQuery
	_ = r.ParseForm()
	f.setPwdForm = map[string]string{
		"userOwner":   r.FormValue("userOwner"),
		"userName":    r.FormValue("userName"),
		"newPassword": r.FormValue("newPassword"),
	}
	for _, field := range []string{"userOwner", "userName", "newPassword"} {
		if f.setPwdForm[field] == "" {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"error","msg":"missing form field %s"}`, field)))
			return
		}
	}
	if f.setPwdForm["userOwner"] != "weknora" {
		_, _ = w.Write([]byte(`{"status":"error","msg":"user's owner is not allowed"}`))
		return
	}
	if _, ok := f.existingUsers[f.setPwdForm["userName"]]; !ok {
		_, _ = w.Write([]byte(`{"status":"error","msg":"user not found"}`))
		return
	}
	f.existingUsers[f.setPwdForm["userName"]] = f.setPwdForm["newPassword"]
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (f *casdoorFakeServer) snapshot() (addUserCalls, getUserCalls, setPwdCalls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addUserCalls, f.getUserCalls, f.setPwdCalls
}

func TestCasdoorClientPasswordToken(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	f := newCasdoorTestServer(t, nil)
	c := newCasdoorClient(casdoorCfg(f.srv.URL), "web-client", "web-secret")
	tok, err := c.PasswordToken(context.Background(), "wx_abcd1234", "pw")
	if err != nil || tok != "tok-1" {
		t.Fatalf("got %q, %v", tok, err)
	}
	f.mu.Lock()
	n := f.ropcCalls
	f.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 ropc call, got %d", n)
	}
}

func TestCasdoorClientEnsureUserCreatesUser(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	f := newCasdoorTestServer(t, map[string]string{})
	c := newCasdoorClient(casdoorCfg(f.srv.URL), "web-client", "web-secret")
	if err := c.EnsureUser(context.Background(), "wx_new", "wx_new@wechat.local", "pw"); err != nil {
		t.Fatalf("new user should be created, got %v", err)
	}
	addUser, getUser, setPwd := f.snapshot()
	if addUser != 1 || getUser != 0 || setPwd != 0 {
		t.Fatalf("calls add-user=%d get-user=%d set-password=%d, want 1/0/0", addUser, getUser, setPwd)
	}
	f.mu.Lock()
	auth, ct, body := f.addUserAuth, f.addUserCT, f.addUserBody
	f.mu.Unlock()
	if auth != "Bearer admin-token" {
		t.Errorf("add-user Authorization = %q, want Bearer admin-token", auth)
	}
	if ct != "application/json" {
		t.Errorf("add-user Content-Type = %q, want application/json", ct)
	}
	want := map[string]string{
		"owner":       "weknora",
		"name":        "wx_new",
		"displayName": "wx_new",
		"email":       "wx_new@wechat.local",
		"password":    "pw",
		"type":        "normal-user",
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("add-user JSON body = %v, want %v", body, want)
	}
}

func TestCasdoorClientEnsureUserExistingResetsPassword(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	f := newCasdoorTestServer(t, map[string]string{"wx_abcd1234": "old-pw"})
	c := newCasdoorClient(casdoorCfg(f.srv.URL), "web-client", "web-secret")
	if err := c.EnsureUser(context.Background(), "wx_abcd1234", "wx_abcd1234@wechat.local", "pw"); err != nil {
		t.Fatalf("existing user should fall through to set-password, got %v", err)
	}
	addUser, getUser, setPwd := f.snapshot()
	if addUser != 1 || getUser != 1 || setPwd != 1 {
		t.Fatalf("calls add-user=%d get-user=%d set-password=%d, want 1/1/1", addUser, getUser, setPwd)
	}
	f.mu.Lock()
	auth, form, query, pwd := f.setPwdAuth, f.setPwdForm, f.setPwdQuery, f.existingUsers["wx_abcd1234"]
	f.mu.Unlock()
	if auth != "Bearer admin-token" {
		t.Errorf("set-password Authorization = %q, want Bearer admin-token", auth)
	}
	wantForm := map[string]string{
		"userOwner":   "weknora",
		"userName":    "wx_abcd1234",
		"newPassword": "pw",
	}
	if !reflect.DeepEqual(form, wantForm) {
		t.Errorf("set-password form = %v, want %v", form, wantForm)
	}
	if query != "" {
		t.Errorf("set-password carried query %q, upstream controller ignores ?id=", query)
	}
	if pwd != "pw" {
		t.Errorf("stored password = %q, want reset to %q", pwd, "pw")
	}
}

func TestCasdoorClientEnsureUserPropagatesAddUserError(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	f := newCasdoorTestServer(t, map[string]string{})
	f.mu.Lock()
	f.forceAddUserError = "quota exceeded"
	f.mu.Unlock()
	c := newCasdoorClient(casdoorCfg(f.srv.URL), "web-client", "web-secret")
	err := c.EnsureUser(context.Background(), "wx_new", "wx_new@wechat.local", "pw")
	if err == nil {
		t.Fatal("add-user failure with no existing user must surface, got nil")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("want original add-user error preserved, got %v", err)
	}
	addUser, getUser, setPwd := f.snapshot()
	if addUser != 1 || getUser != 1 || setPwd != 0 {
		t.Fatalf("calls add-user=%d get-user=%d set-password=%d, want 1/1/0 (no silent degradation)", addUser, getUser, setPwd)
	}
}

func TestCasdoorClientSetPasswordFields(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")
	f := newCasdoorTestServer(t, map[string]string{"wx_abcd1234": "old-pw"})
	c := newCasdoorClient(casdoorCfg(f.srv.URL), "web-client", "web-secret")
	if err := c.SetPassword(context.Background(), "wx_abcd1234", "pw"); err != nil {
		t.Fatalf("set-password: %v", err)
	}
	f.mu.Lock()
	auth, form, query := f.setPwdAuth, f.setPwdForm, f.setPwdQuery
	f.mu.Unlock()
	if auth != "Bearer admin-token" {
		t.Errorf("set-password Authorization = %q, want Bearer admin-token", auth)
	}
	wantForm := map[string]string{
		"userOwner":   "weknora",
		"userName":    "wx_abcd1234",
		"newPassword": "pw",
	}
	if !reflect.DeepEqual(form, wantForm) {
		t.Errorf("set-password form = %v, want %v", form, wantForm)
	}
	if query != "" {
		t.Errorf("set-password carried query %q, upstream controller ignores ?id=", query)
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
