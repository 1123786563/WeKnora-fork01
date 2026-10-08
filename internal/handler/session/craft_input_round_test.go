package session

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type craftT01API struct {
	accepted []service.CraftInputUpload
	action   string
}

func (f *craftT01API) AcceptInputRound(_ context.Context, _ craft.Scope, uploads []service.CraftInputUpload) ([]craft.Input, error) {
	f.accepted = uploads
	return []craft.Input{{Ref: "opaque://one", Name: uploads[0].Name, SHA256: uploads[0].SHA256, Bytes: int64(len(uploads[0].Content)), Recognition: &craft.InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_format"}}}, nil
}
func (f *craftT01API) DecideInput(_ context.Context, _ craft.Scope, _ string, action string) error {
	f.action = action
	return nil
}

func TestCraftInputRoundHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &craftT01API{}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u1"))
		c.Next()
	})
	RegisterCraftInputRoutes(r.Group("/sessions"), NewCraftInputHandler(fake))
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	f, err := w.CreateFormFile("files", "ledger.mystery")
	require.NoError(t, err)
	_, err = f.Write([]byte("opaque"))
	require.NoError(t, err)
	require.NoError(t, w.WriteField("sha256", "2e39488e5d0d9f50393f4fb9d355c4f5938e77ac9d0ed5b6243ec00b68d43270"))
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/sessions/s1/craft/input-rounds", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
	require.Len(t, fake.accepted, 1)
	require.Equal(t, "ledger.mystery", fake.accepted[0].Name)
	var projected map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &projected))
	require.Contains(t, resp.Body.String(), "unrecognized_format")

	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/sessions/s1/craft/inputs/decision", bytes.NewBufferString(`{"ref":"opaque://one","action":"cancel"}`)))
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Equal(t, "cancel", fake.action)
}
