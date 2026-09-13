package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// openSemanticInternalDB opens a REAL migrated SQLite business database.
func openSemanticInternalDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dbPath := filepath.Join(t.TempDir(), "semantic-internal.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

func newSemanticInternalFixture(t *testing.T) (*gin.Engine, *service.SemanticScopeService, *semanticInternalSeed) {
	t.Helper()
	db := openSemanticInternalDB(t)
	repo := apprepo.NewSemanticControlRepository(db)
	_, err := repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: 7, KBID: "kb-x", DocumentID: "doc-1", Payload: []byte("{}"),
	}, func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)
	svc := service.NewSemanticScopeService(repo, service.SemanticScopeConfig{
		Audience: "weknora-semantic", TTL: time.Minute,
		IssuingKey: []byte("handler-test-key-0123456789abcdef0123"), Now: func() time.Time { return time.Now().UTC() },
	})
	engine := gin.New()
	RegisterSemanticInternalRoutes(engine.Group(""), "resolve-token", svc)
	return engine, svc, &semanticInternalSeed{repo: repo}
}

type semanticInternalSeed struct {
	repo *apprepo.SemanticControlRepository
}

func resolveScope(t *testing.T, engine *gin.Engine, token, scopeRef string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{"scope_ref": scopeRef})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/internal/semantic/scope/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Semantic-Internal-Token", token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestSemanticInternalRejectsMissingIdentity(t *testing.T) {
	engine, svc, _ := newSemanticInternalFixture(t)
	scope, err := svc.Issue(context.Background(), "member-a",
		types.SemanticScopeKey{TenantID: 7, KBID: "kb-x"}, "reason")
	require.NoError(t, err)

	w := resolveScope(t, engine, "", scope.ScopeRef)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	w = resolveScope(t, engine, "wrong-token", scope.ScopeRef)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSemanticInternalResolvesSnapshot(t *testing.T) {
	engine, svc, _ := newSemanticInternalFixture(t)
	scope, err := svc.Issue(context.Background(), "member-a",
		types.SemanticScopeKey{TenantID: 7, KBID: "kb-x"}, "reason")
	require.NoError(t, err)

	w := resolveScope(t, engine, "resolve-token", scope.ScopeRef)
	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		AllowedDocumentIDs []string          `json:"allowed_document_ids"`
		MaxSourceRevision  map[string]uint64 `json:"max_source_revision"`
		DenyRevisions      map[string]uint64 `json:"deny_revisions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Contains(t, payload.AllowedDocumentIDs, "doc-1")
	require.Equal(t, uint64(1), payload.MaxSourceRevision["doc-1"])
}

func TestSemanticInternalChangedScopeIsConflict(t *testing.T) {
	engine, svc, seed := newSemanticInternalFixture(t)
	scope, err := svc.Issue(context.Background(), "member-a",
		types.SemanticScopeKey{TenantID: 7, KBID: "kb-x"}, "reason")
	require.NoError(t, err)
	// Bump the epoch via a real deletion transaction.
	_, err = seed.repo.WithSemanticMutation(context.Background(), types.SemanticMutation{
		TenantID: 7, KBID: "kb-x", DocumentID: "doc-1",
		ExpectedRevision: 1, Deleted: true, Payload: []byte("{}"),
	}, func(tx *gorm.DB) error { return nil })
	require.NoError(t, err)

	w := resolveScope(t, engine, "resolve-token", scope.ScopeRef)
	require.Equal(t, http.StatusConflict, w.Code, "changed authorization must be a recognizable conflict")
}

func TestSemanticInternalInvalidScopeIsForbidden(t *testing.T) {
	engine, _, _ := newSemanticInternalFixture(t)
	w := resolveScope(t, engine, "resolve-token", "forged.reference")
	require.Equal(t, http.StatusForbidden, w.Code)
}
