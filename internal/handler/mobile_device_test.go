package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openMobileHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	up, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sqlite", "000058_mobile_devices.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	return db
}

func mobileRequest(method, path string, tenant uint64, owner string, body string) *gin.Context {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request.WithContext(context.WithValue(context.WithValue(request.Context(), types.TenantIDContextKey, tenant), types.UserIDContextKey, owner))
	return ctx
}

func TestMobileDeviceHandlerRequiresAuthAndScopesOwner(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	store := repository.NewMobileDeviceStore(openMobileHandlerDB(t), "dev")
	h := NewMobileDeviceHandlerWithSealer(store, "dev", func(value string) (string, error) { return "enc:" + value, nil })
	intent, err := encodeRegistrationIntent(mobileRegistrationIntent{Tenant: 1, Owner: "u1", Device: "d", Epoch: 1, Nonce: "test", Expiry: 9999999999})
	require.NoError(t, err)
	ctx := mobileRequest(http.MethodPut, "/api/v1/mobile/devices/d", 1, "u1", `{"token":"push-a","platform":"ios","registration_intent":"`+intent+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "d"}}
	h.Register(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())

	foreign := mobileRequest(http.MethodDelete, "/api/v1/mobile/devices/d?revision=1", 1, "u2", "")
	foreign.Params = gin.Params{{Key: "id", Value: "d"}}
	h.Revoke(foreign)
	require.Equal(t, http.StatusNotFound, foreign.Writer.Status())

	stale := mobileRequest(http.MethodDelete, "/api/v1/mobile/devices/d?revision=99", 1, "u1", "")
	stale.Params = gin.Params{{Key: "id", Value: "d"}}
	h.Revoke(stale)
	require.Equal(t, http.StatusConflict, stale.Writer.Status())
}

func TestMobileDeviceHandlerRejectsGuessedFutureEpoch(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	store := repository.NewMobileDeviceStore(openMobileHandlerDB(t), "dev")
	h := NewMobileDeviceHandlerWithSealer(store, "dev", func(value string) (string, error) { return "enc:" + value, nil })
	intent, err := encodeRegistrationIntent(mobileRegistrationIntent{Tenant: 1, Owner: "u1", Device: "d", Epoch: 1, Nonce: "test", Expiry: 9999999999})
	require.NoError(t, err)
	ctx := mobileRequest(http.MethodPut, "/api/v1/mobile/devices/d", 1, "u1", `{"token":"push-a","platform":"ios","registration_intent":"`+intent+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "d"}}
	h.Register(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())
	require.NoError(t, store.RevokeForTenant(context.Background(), 1, "u1", "d", 1))
	// A client cannot sign an epoch 999 intent; an opaque guessed value must
	// be rejected before the repository can reopen the revoked row.
	late := mobileRequest(http.MethodPut, "/api/v1/mobile/devices/d", 1, "u1", `{"token":"push-b","platform":"ios","scope_generation":999,"registration_intent":"epoch-999"}`)
	late.Params = gin.Params{{Key: "id", Value: "d"}}
	h.Register(late)
	require.Equal(t, http.StatusConflict, late.Writer.Status())
	rows, err := store.ListActiveForTenant(context.Background(), 1, "u1", "dev")
	require.NoError(t, err)
	require.Empty(t, rows)
}
