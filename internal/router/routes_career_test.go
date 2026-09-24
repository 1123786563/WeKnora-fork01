package router

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/career"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerEvaluationRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/evaluations"])
	require.True(t, paths["GET /api/v1/career/evaluations/receipt"])
	require.True(t, paths["GET /api/v1/career/evaluations/:evaluationId"])
}
