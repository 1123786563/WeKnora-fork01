package middleware

import (
	"net/http"
	"testing"
)

func TestIsNoAuthAPI(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/v1/auth/login", true},
		{http.MethodPost, "/api/v1/auth/oidc/exchange", true},
		{http.MethodPost, "/api/v1/auth/mobile/exchange", true},
		{http.MethodGet, "/api/v1/auth/mobile/exchange", false},
		{http.MethodPost, "/api/v1/auth/mobile/exchange/", false},
		{http.MethodGet, "/api/v1/knowledge-bases", false},
		{http.MethodGet, "/api/v1/files/presigned", true},
		{http.MethodHead, "/api/v1/files/presigned", true},
		{http.MethodPost, "/api/v1/commercial/callbacks/alipay", true},
		{http.MethodPost, "/api/v1/commercial/other", false},
	}
	for _, tt := range tests {
		if got := isNoAuthAPI(tt.path, tt.method); got != tt.want {
			t.Errorf("isNoAuthAPI(%s %s) = %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}
