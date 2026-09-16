package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, db.Exec(`CREATE TABLE mobile_devices (
        tenant_id INTEGER NOT NULL, owner_id TEXT NOT NULL, device_id TEXT NOT NULL,
        environment TEXT NOT NULL, space_id TEXT NOT NULL DEFAULT '', platform TEXT NOT NULL,
        token_ciphertext TEXT NOT NULL, token_hash TEXT NOT NULL, revision INTEGER NOT NULL,
        scope_generation INTEGER NOT NULL DEFAULT 0, revoked_at DATETIME, last_seen_at DATETIME,
        created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
        PRIMARY KEY (tenant_id, owner_id, device_id, environment))`).Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX uq_mobile_handler_token ON mobile_devices(environment, token_hash) WHERE revoked_at IS NULL").Error)
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
	store := repository.NewMobileDeviceStore(openMobileHandlerDB(t), "dev")
	h := NewMobileDeviceHandlerWithSealer(store, "dev", func(value string) (string, error) { return "enc:" + value, nil })
	ctx := mobileRequest(http.MethodPut, "/api/v1/mobile/devices/d", 1, "u1", `{"token":"push-a","platform":"ios","scope_generation":1}`)
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
