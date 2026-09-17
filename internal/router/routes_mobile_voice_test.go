package router

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/voice"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Stubs satisfying the handler seams for a route-declaration test.
type routesVoiceStore struct{}

func (routesVoiceStore) CreateVoiceSession(context.Context, repository.VoiceSessionRow) (repository.VoiceSessionRow, error) {
	return repository.VoiceSessionRow{}, nil
}
func (routesVoiceStore) GetOwnedVoiceSession(context.Context, uint64, string, string) (repository.VoiceSessionRow, error) {
	return repository.VoiceSessionRow{}, nil
}
func (routesVoiceStore) MarkVoiceSessionUnknown(context.Context, uint64, string) error { return nil }
func (routesVoiceStore) PendingUnknownVoiceSession(context.Context, uint64, string) (bool, error) {
	return false, nil
}
func (routesVoiceStore) RecordVoiceSessionUsage(context.Context, uint64, string, int64) error { return nil }
func (routesVoiceStore) CloseVoiceSessionWithUsage(context.Context, uint64, string, int64, time.Time) (bool, error) {
	return true, nil
}

type routesVoiceGate struct{}

func (routesVoiceGate) Begin(context.Context, domain.BudgetRequest) (domain.Reservation, error) {
	return domain.Reservation{}, nil
}
func (routesVoiceGate) Finish(context.Context, string, domain.UsageFact) error { return nil }

type routesVoiceRates struct{}

func (routesVoiceRates) VoiceAdmission(context.Context, int64) (string, domain.Credits, error) {
	return "v1", 1, nil
}

// TestRegisterMobileVoiceRoutesDeclaresTheThreeEndpoints: the W30 surface
// mounts exactly the authorized session creation, its stop/settle path and
// the server-proxied transcription. A nil handler mounts nothing, and the
// handler constructor fails closed on incomplete wiring.
func TestRegisterMobileVoiceRoutesDeclaresTheThreeEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := &rbacGuards{cfg: &config.Config{}}
	h, err := handler.NewMobileVoiceHandler(nil, nil, nil, nil, nil)
	require.Error(t, err, "incomplete wiring must fail closed")
	require.Nil(t, h)
	RegisterMobileVoiceRoutes(r.Group("/api/v1"), nil, g)
	require.Empty(t, r.Routes(), "a nil handler must mount nothing")

	provider, err := voice.NewManagedProvider(voice.Config{})
	require.NoError(t, err)
	voiceHandler, err := handler.NewMobileVoiceHandler(routesVoiceStore{}, provider, provider, routesVoiceGate{}, routesVoiceRates{})
	require.NoError(t, err)
	r2 := gin.New()
	RegisterMobileVoiceRoutes(r2.Group("/api/v1"), voiceHandler, g)
	seen := map[string]bool{}
	for _, route := range r2.Routes() {
		seen[route.Method+" "+route.Path] = true
	}
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/sessions"])
	require.True(t, seen[http.MethodDelete+" /api/v1/mobile/voice/sessions/:id"])
	require.True(t, seen[http.MethodPost+" /api/v1/mobile/voice/transcriptions"])
}
