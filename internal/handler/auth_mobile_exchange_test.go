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
	require.NoError(t, store.Put(context.Background(), repository.MobileExchange{CodeHash: repository.HashMobileExchangeValue("code"), StateHash: repository.HashMobileExchangeValue("state"), RedirectURI: "weknora://auth-return", Challenge: challenge, Subject: "u1", ExpiresAt: time.Now().Add(time.Minute)}))
	h := NewAuthHandler(&config.Config{}, &mobileExchangeUserService{}, nil, nil, nil)
	h.SetMobileExchangeStore(store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/mobile/exchange", h.MobileOIDCExchange)
	body := func(verifier string) *bytes.Reader {
		b, _ := json.Marshal(map[string]string{"code": "code", "state": "state", "redirect_uri": "weknora://auth-return", "code_verifier": verifier})
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
