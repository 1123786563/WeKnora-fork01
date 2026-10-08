package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/workbench/voice"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestMobileVoiceSurfaceMountsNoApprovalEndpoint is the server-side structural
// evidence for Issue #57 AC2 (high-risk approvals can never be completed by
// voice): the mobile voice surface mounts EXACTLY the three W30 endpoints
// (authorize / stop+settle / transcribe proxy), and none of them is an
// approval or decision surface. Decision authority lives only behind the
// logged-in POST /workbench/interactions/:id/decisions lane; the voice grant
// token is a media-plane credential and carries no product authority.
func TestMobileVoiceSurfaceMountsNoApprovalEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	provider, err := voice.NewManagedProvider(voice.Config{})
	require.NoError(t, err)
	voiceHandler, err := handler.NewMobileVoiceHandler(routesVoiceStore{}, routesVoiceTranscriptions{}, provider, provider, routesVoiceGate{}, routesVoiceRates{})
	require.NoError(t, err)
	RegisterMobileVoiceRoutes(r.Group("/api/v1"), voiceHandler, g)
	routes := r.Routes()
	require.Len(t, routes, 3, "the voice surface mounts exactly the three W30 endpoints")
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Method+" "+route.Path] = true
		lower := strings.ToLower(route.Path)
		for _, forbidden := range []string{"decision", "approve", "interaction", "command", "budget"} {
			require.NotContains(t, lower, forbidden,
				"the voice surface must never mount an approval/decision-adjacent endpoint (AC2)")
		}
	}
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/sessions"])
	require.True(t, seen[http.MethodDelete+" /api/v1/mobile/voice/sessions/:id"])
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/transcriptions"])
}
