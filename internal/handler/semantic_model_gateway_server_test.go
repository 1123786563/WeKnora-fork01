package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// countingProvider wraps the controlled provider and exposes the durable
// invocation count over HTTP for the Python fixture.
type countingProvider struct{ calls int }

func (p *countingProvider) Invoke(_ context.Context, _ types.SemanticModelRequest) (types.SemanticModelResult, error) {
	p.calls++
	return types.SemanticModelResult{
		Text: fmt.Sprintf("answer-%d", p.calls), InputTokens: 10, OutputTokens: 20,
		ProviderRequestID: fmt.Sprintf("pr-%d", p.calls), Status: "completed",
	}, nil
}

// TestSemanticModelGatewayServer runs the REAL internal handler stack for
// the Python fixture: compiled with `go test -c`, started with
// -test.run TestSemanticModelGatewayServer, address in SEMANTIC_TEST_MODEL_ADDR.
func TestSemanticModelGatewayServer(t *testing.T) {
	addr := os.Getenv("SEMANTIC_TEST_MODEL_ADDR")
	if addr == "" {
		t.Skip("server mode only")
	}
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "model-gateway.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	provider := &countingProvider{}
	modelService := service.NewSemanticModelService(db, provider)
	if err := modelService.SetBudgetForTest(context.Background(), "budget-tight", 40); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	RegisterSemanticModelInternalRoutes(engine.Group(""), "model-test-token", modelService)
	engine.GET("/internal/semantic/model/count", func(c *gin.Context) {
		var count int64
		_ = db.Raw("SELECT COUNT(*) FROM semantic_invocations WHERE invocation_id = ?", c.Query("invocation_id")).Scan(&count).Error
		c.JSON(http.StatusOK, map[string]int64{"count": count})
	})
	if err := http.ListenAndServe(addr, engine); err != nil {
		t.Fatal(err)
	}
}

var _ = json.Marshal
