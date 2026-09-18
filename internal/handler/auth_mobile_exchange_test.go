package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mobileExchangeUserService struct{ interfaces.UserService }

func (s *mobileExchangeUserService) GetUserByID(context.Context, string) (*types.User, error) {
	return &types.User{ID: "u1", TenantID: 1}, nil
}
func (s *mobileExchangeUserService) GenerateTokens(context.Context, *types.User) (string, string, error) {
	return "access", "refresh", nil
}
func (s *mobileExchangeUserService) BuildLoginMemberships(context.Context, *types.User, *types.Tenant) []types.Membership {
	return []types.Membership{}
}

func TestMobileOIDCExchangeHTTPMatrix(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.MobileExchange{}))
	store := repository.NewMobileExchangeStore(db)
	verifier := "verifier-123"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	require.NoError(t, store.Put(context.Background(), repository.MobileExchange{CodeHash: repository.HashMobileExchangeValue("code"), StateHash: repository.HashMobileExchangeValue("state"), RedirectURI: "weknora://oidc", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(time.Minute)}))
	h := NewAuthHandler(&config.Config{}, &mobileExchangeUserService{}, nil, nil, nil)
	h.SetMobileExchangeStore(store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/mobile/exchange", h.MobileOIDCExchange)
	body := func(verifier string) *bytes.Reader {
		b, _ := json.Marshal(map[string]string{"code": "code", "state": "state", "redirect_uri": "weknora://oidc", "code_verifier": verifier})
		return bytes.NewReader(b)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/mobile/exchange", body(verifier))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "access")
	req = httptest.NewRequest(http.MethodPost, "/auth/mobile/exchange", body(verifier))
	req.Header.Set("content-type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMobileOIDCExchangeRejectsBindingExpiryAndURLCredentials(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.MobileExchange{}))
	store := repository.NewMobileExchangeStore(db)
	verifier := "v"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	for _, row := range []repository.MobileExchange{
		{CodeHash: repository.HashMobileExchangeValue("wrong-verifier"), StateHash: repository.HashMobileExchangeValue("s1"), RedirectURI: "weknora://oidc", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(time.Minute)},
		{CodeHash: repository.HashMobileExchangeValue("wrong-state"), StateHash: repository.HashMobileExchangeValue("s2"), RedirectURI: "weknora://oidc", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(time.Minute)},
		{CodeHash: repository.HashMobileExchangeValue("wrong-redirect"), StateHash: repository.HashMobileExchangeValue("s3"), RedirectURI: "weknora://oidc", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(time.Minute)},
		{CodeHash: repository.HashMobileExchangeValue("expired"), StateHash: repository.HashMobileExchangeValue("s4"), RedirectURI: "weknora://oidc", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(-time.Hour)},
	} {
		require.NoError(t, store.Put(context.Background(), row))
	}
	h := NewAuthHandler(&config.Config{}, &mobileExchangeUserService{}, nil, nil, nil)
	h.SetMobileExchangeStore(store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/mobile/exchange", h.MobileOIDCExchange)
	request := func(code, state, redirect, v string, token bool) int {
		values := map[string]string{"code": code, "state": state, "redirect_uri": redirect, "code_verifier": v}
		b, _ := json.Marshal(values)
		path := "/auth/mobile/exchange"
		if token {
			path += "?access_token=leak"
		}
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusUnauthorized, request("wrong-verifier", "s1", "weknora://oidc", "bad", false))
	require.Equal(t, http.StatusUnauthorized, request("wrong-state", "bad", "weknora://oidc", verifier, false))
	require.Equal(t, http.StatusUnauthorized, request("wrong-redirect", "s3", "weknora://other", verifier, false))
	require.Equal(t, http.StatusUnauthorized, request("expired", "s4", "weknora://oidc", verifier, false))
	require.Equal(t, http.StatusBadRequest, request("wrong-state", "s2", "weknora://oidc", verifier, true))
	for _, key := range []string{"token", "id_token", "refresh_token", "access_token"} {
		b, _ := json.Marshal(map[string]string{"code": "wrong-state", "state": "s2", "redirect_uri": "weknora://oidc", "code_verifier": verifier})
		req := httptest.NewRequest(http.MethodPost, "/auth/mobile/exchange?"+key+"=x&"+key+"=y", bytes.NewReader(b))
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code, key)
	}
}
