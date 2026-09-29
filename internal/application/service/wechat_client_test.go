package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
)

func TestCode2SessionSuccess(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("appid") != "wx123" || q.Get("secret") != "sec" ||
			q.Get("js_code") != "CODE" || q.Get("grant_type") != "authorization_code" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"openid":"oABCDEF1234567890abcdef1234","session_key":"sk"}`))
	}))
	defer srv.Close()
	c := newWechatMPClient(&config.WechatMPConfig{AppID: "wx123", AppSecret: "sec"})
	c.baseURL = srv.URL
	openid, err := c.Code2Session(context.Background(), "CODE")
	if err != nil || openid != "oABCDEF1234567890abcdef1234" {
		t.Fatalf("got %q, %v", openid, err)
	}
}

func TestCode2SessionErrorCode(t *testing.T) {
	withOIDCSSRFWhitelist(t, "127.0.0.1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40029,"errmsg":"invalid code"}`))
	}))
	defer srv.Close()
	c := newWechatMPClient(&config.WechatMPConfig{AppID: "wx123", AppSecret: "sec"})
	c.baseURL = srv.URL
	if _, err := c.Code2Session(context.Background(), "CODE"); err == nil {
		t.Fatal("expected error for errcode 40029")
	}
}
