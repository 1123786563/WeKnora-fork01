package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// GetSystemInfo must advertise whether the swagger docs route is mounted so
// the frontend only renders its "OpenAPI 文档" entries when the backend really
// serves /swagger/index.html. The router mounts /swagger/*any exactly when
// gin.Mode() != gin.ReleaseMode (internal/router/router.go), so the response
// field must mirror that condition per mode.
func TestGetSystemInfoReportsSwaggerEnabled(t *testing.T) {
	originalMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(originalMode) })

	cases := []struct {
		name string
		mode string
		want bool
	}{
		{name: "test mode enables swagger", mode: gin.TestMode, want: true},
		{name: "debug mode enables swagger", mode: gin.DebugMode, want: true},
		{name: "release mode disables swagger", mode: gin.ReleaseMode, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(tc.mode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", "/system/info", nil)

			// Zero-value handler: every field GetSystemInfo touches (cfg,
			// neo4jDriver, env vars, cached migration state) is nil- or
			// default-safe, so the swagger flag is the only assertion that
			// depends on the gin mode.
			(&SystemHandler{}).GetSystemInfo(ctx)

			if recorder.Code != 200 {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			var body struct {
				Code int                   `json:"code"`
				Data GetSystemInfoResponse `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
			}
			if body.Code != 0 {
				t.Fatalf("code = %d, want 0", body.Code)
			}
			if body.Data.SwaggerEnabled != tc.want {
				t.Fatalf("swagger_enabled = %v, want %v (gin mode %q)", body.Data.SwaggerEnabled, tc.want, tc.mode)
			}
		})
	}
}
