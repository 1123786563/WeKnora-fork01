package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func appDeviceHandler(t *testing.T, enterpriseAppID string) *MobileDeviceHandler {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	store := repository.NewMobileDeviceStore(openMobileHandlerDB(t), "dev")
	return NewMobileDeviceHandlerWithSealer(store, "dev", func(value string) (string, error) { return "enc:" + value, nil }).
		WithMobileAppPolicy(MobileAppPolicy{EnterpriseAppID: enterpriseAppID})
}

func appGinContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request.WithContext(context.WithValue(context.WithValue(request.Context(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1"))
	return ctx, recorder
}

func issueAppIntent(t *testing.T, h *MobileDeviceHandler, device, appID string) string {
	t.Helper()
	ctx, recorder := appGinContext(http.MethodPost, "/api/v1/mobile/devices/"+device+"/registration-intent", `{"app_id":"`+appID+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: device}}
	h.IssueIntent(ctx)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Data struct {
			RegistrationIntent string `json:"registration_intent"`
			AppID              string `json:"app_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, appID, response.Data.AppID)
	return response.Data.RegistrationIntent
}

func registerAppDevice(t *testing.T, h *MobileDeviceHandler, device, appID, token, intent string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, recorder := appGinContext(http.MethodPut, "/api/v1/mobile/devices/"+device,
		`{"token":"`+token+`","platform":"ios","app_id":"`+appID+`","registration_intent":"`+intent+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: device}}
	h.Register(ctx)
	return recorder
}

func TestRegisterAppIDBindsIntentAndIsolates(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	officialIntent := issueAppIntent(t, h, "shared", "official")
	enterpriseIntent := issueAppIntent(t, h, "shared", "enterprise:acme")

	require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", "official", "tok-official", officialIntent).Code)
	require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", "enterprise:acme", "tok-enterprise", enterpriseIntent).Code)

	ctx, listRecorder := appGinContext(http.MethodGet, "/api/v1/mobile/devices", "")
	h.List(ctx)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var listed struct {
		Data []struct {
			DeviceID string `json:"device_id"`
			AppID    string `json:"app_id"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 2, "one row per app for the same physical device")
	apps := map[string]int64{}
	for _, row := range listed.Data {
		apps[row.AppID] = row.Revision
	}
	require.EqualValues(t, 1, apps["official"])
	require.EqualValues(t, 1, apps["enterprise:acme"], "the enterprise bind must not take over the official row")
}

func TestRegisterRejectsUndeclaredEnterpriseApp(t *testing.T) {
	h := appDeviceHandler(t, "") // 未声明企业 App：仅 official

	ctx, recorder := appGinContext(http.MethodPost, "/api/v1/mobile/devices/d/registration-intent", `{"app_id":"enterprise:acme"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "d"}}
	h.IssueIntent(ctx)
	require.Equal(t, http.StatusBadRequest, recorder.Code, "undeclared enterprise intents fail closed")

	require.Equal(t, http.StatusBadRequest,
		registerAppDevice(t, h, "d", "enterprise:acme", "t", "x").Code,
		"undeclared enterprise registrations fail closed")
	require.Equal(t, http.StatusBadRequest,
		registerAppDevice(t, h, "d", "Enterprise:ACME", "t", "x").Code,
		"malformed app ids are rejected")
}

func TestOfficialIntentCannotBindEnterpriseRow(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	officialIntent := issueAppIntent(t, h, "shared", "official")
	require.Equal(t, http.StatusConflict,
		registerAppDevice(t, h, "shared", "enterprise:acme", "tok", officialIntent).Code,
		"an official intent must not bind an enterprise registration")
}

func TestPresenceScopedByApp(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	for _, appID := range []string{"official", "enterprise:acme"} {
		intent := issueAppIntent(t, h, "shared", appID)
		require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", appID, "tok-"+appID, intent).Code)
	}
	putCtx, putRecorder := appGinContext(http.MethodPut, "/api/v1/mobile/devices/shared/presence?app_id=enterprise:acme", "")
	putCtx.Params = gin.Params{{Key: "id", Value: "shared"}}
	h.PutPresence(putCtx)
	require.Equal(t, http.StatusOK, putRecorder.Code, putRecorder.Body.String())

	// official 行不受企业 presence 影响，且仍可按 app 独立读取。
	getCtx, getRecorder := appGinContext(http.MethodGet, "/api/v1/mobile/devices/shared/presence?app_id=official", "")
	getCtx.Params = gin.Params{{Key: "id", Value: "shared"}}
	h.GetPresence(getCtx)
	require.Equal(t, http.StatusOK, getRecorder.Code, getRecorder.Body.String())
}
