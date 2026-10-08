package router

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/career"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerEvaluationRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
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

func TestCareerSourceImportRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/opportunities/import-url"])
	require.True(t, paths["GET /api/v1/career/opportunities/:opportunityId/observations"])
	require.True(t, paths["POST /api/v1/career/opportunities/import"])
}

func TestCareerApplicationRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/applications"])
	require.True(t, paths["GET /api/v1/career/applications/receipt"])
	require.True(t, paths["GET /api/v1/career/applications/:applicationId"])
	require.True(t, paths["POST /api/v1/career/applications/link/reconcile"])
}

func TestCareerSearchRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/searches"])
	require.True(t, paths["GET /api/v1/career/searches/receipt"])
	require.True(t, paths["GET /api/v1/career/searches/:searchId"])
}

func TestCareerSearchRuleRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/rules"])
	require.True(t, paths["GET /api/v1/career/rules"])
	require.True(t, paths["GET /api/v1/career/rules/receipt"])
	require.True(t, paths["GET /api/v1/career/rules/:ruleId"])
}

func TestCareerMaterialRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/materials"])
	require.True(t, paths["POST /api/v1/career/materials/confirm"])
	require.True(t, paths["GET /api/v1/career/materials/receipt"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId/versions"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId/versions/:versionId"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId/versions/:versionId/compare"])
}

func TestCareerMaterialExportRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/materials/:materialId/exports"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId/exports"])
	require.True(t, paths["POST /api/v1/career/materials/:materialId/exports/:exportId/signed-url"])
	require.True(t, paths["GET /api/v1/career/materials/:materialId/exports/:exportId/download"])
	require.True(t, paths["DELETE /api/v1/career/materials/:materialId/exports/:exportId"])
}

func TestCareerProgressRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/applications/:applicationId/progress"])
	require.True(t, paths["POST /api/v1/career/applications/:applicationId/progress/correct"])
	require.True(t, paths["GET /api/v1/career/applications/:applicationId/progress"])
	require.True(t, paths["GET /api/v1/career/progress/receipt"])
}

func TestCareerSubmissionRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/applications/:applicationId/submissions"])
	require.True(t, paths["GET /api/v1/career/applications/:applicationId/submissions"])
	require.True(t, paths["GET /api/v1/career/submissions/receipt"])
}

func TestCareerPreparationRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/applications/:applicationId/preparations"])
	require.True(t, paths["GET /api/v1/career/applications/:applicationId/preparations"])
	require.True(t, paths["GET /api/v1/career/preparations/receipt"])
}

func TestCareerReminderRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/reminders"])
	require.True(t, paths["GET /api/v1/career/reminders"])
	require.True(t, paths["GET /api/v1/career/reminders/receipt"])
}

func TestCareerExportDeletionRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/exports"])
	require.True(t, paths["GET /api/v1/career/exports/receipt"])
	require.True(t, paths["GET /api/v1/career/deletions/boundary"])
	require.True(t, paths["POST /api/v1/career/deletions"])
	require.True(t, paths["GET /api/v1/career/deletions/receipt"])
}

func TestCareerUsageEstimateRouteIsRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["GET /api/v1/career/usage/estimate"])
}

func TestCareerReconciliationRoutesAreRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	handler, err := career.NewHandler(db, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	engine := gin.New()
	RegisterCareerRoutes(engine.Group("/api/v1"), handler)
	paths := make(map[string]bool)
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["POST /api/v1/career/opportunities/reconcile"])
	require.True(t, paths["GET /api/v1/career/opportunities/:opportunityId/status"])
	require.True(t, paths["GET /api/v1/career/opportunities/:opportunityId/reconciliations"])
	require.True(t, paths["GET /api/v1/career/reconciliations/receipt"])
	require.True(t, paths["GET /api/v1/career/coverage"])
}
