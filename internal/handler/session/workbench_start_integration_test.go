package session

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type integrationBudget struct {
	mu   sync.Mutex
	refs map[string]string
}

func (b *integrationBudget) Ensure(_ context.Context, tenant uint64, owner, requestID string, _ int64, _ time.Time) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.refs == nil {
		b.refs = map[string]string{}
	}
	key := fmt.Sprintf("%d/%s/%s", tenant, owner, requestID)
	if ref, ok := b.refs[key]; ok {
		return ref, nil
	}
	ref := "reservation/" + key
	b.refs[key] = ref
	return ref, nil
}
func (*integrationBudget) ReleaseUnstarted(context.Context, string) error { return nil }

func TestWorkbenchStartHTTPIntegrationAndIdentityIsolation(t *testing.T) {
	db := openWorkbenchHTTPDB(t)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), &integrationBudget{}, nil)
	h := NewWorkbenchStartHandler(coordinator)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/workbench/executions", withIdentity(1, "u1"), h.Start)
	r.GET("/api/v1/workbench/executions/requests/:request_id", withIdentity(1, "u1"), h.Lookup)
	r.GET("/api/v1/workbench/executions/requests/:request_id/other", withIdentity(1, "u2"), h.Lookup)
	r.GET("/api/v1/workbench/executions/requests/:request_id/tenant-two", withIdentity(2, "u1"), h.Lookup)

	body := `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"http-r1","text":"hello","budget_upper":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code)
	var accepted struct {
		Success bool `json:"success"`
		Data    struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &accepted))
	require.True(t, accepted.Success)
	require.NotEmpty(t, accepted.Data.RunID)

	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusAccepted, resp.Code)
	var replay struct {
		Data struct {
			RunID string `json:"run_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &replay))
	require.Equal(t, accepted.Data.RunID, replay.Data.RunID)

	conflictBody := `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"http-r1","text":"different","budget_upper":100}`
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(conflictBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusConflict, resp.Code)

	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/requests/http-r1", nil))
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"admitted"`)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/requests/http-r1/other", nil))
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"unknown"`)
	resp = httptest.NewRecorder()
	r.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/requests/http-r1/tenant-two", nil))
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"unknown"`)

	unauth := gin.New()
	unauth.GET("/lookup/:request_id", h.Lookup)
	resp = httptest.NewRecorder()
	unauth.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/lookup/http-r1", nil))
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

type failingGraphFreezer struct{ err error }

func (f failingGraphFreezer) FreezeAdmissionGraph(context.Context, uint64, string, workbenchservice.StartInput) (json.RawMessage, error) {
	return nil, f.err
}

// WB-GRAPH: a graph-resolution failure settles the admission as rejected —
// the first failure must surface as the same 409 its idempotent replay gets,
// with the freezer's short code intact in the body.
func TestWorkbenchStartGraphResolutionFailureMapsToConflict(t *testing.T) {
	db := openWorkbenchHTTPDB(t)
	coordinator := workbenchservice.NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), &integrationBudget{}, nil)
	coordinator.SetAdmissionGraphFreezer(failingGraphFreezer{
		err: errors.New("model_unresolved: chat model is not configured: please set model_id on agent a1"),
	})
	h := NewWorkbenchStartHandler(coordinator)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/workbench/executions", withIdentity(1, "u1"), h.Start)

	body := `{"session_id":"s1","agent_id":"a1","target_id":"platform","request_id":"http-graph-fail","text":"hello","budget_upper":100}`
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(resp, req)
	require.Equal(t, http.StatusConflict, resp.Code)
	require.Contains(t, resp.Body.String(), "model_unresolved")

	var runs int64
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", "http-graph-fail").Count(&runs).Error)
	require.Zero(t, runs, "a rejected admission leaves no Run row")
}

func withIdentity(tenant uint64, actor string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), tenant)
		c.Set(types.UserIDContextKey.String(), actor)
		c.Next()
	}
}

func openWorkbenchHTTPDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../"))
	dsn := "file:" + filepath.Join(t.TempDir(), "workbench-http.db") + "?_foreign_keys=on&_busy_timeout=10000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	m, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, m.Up())
	_, _ = m.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO tenants (id,name,business) VALUES (1,'t1','test')").Error)
	require.NoError(t, db.Exec("INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u1','u1','u1@test','x',1)").Error)
	// Admissions are actor-fenced: AgentRunStore.Admit rejects any actor that
	// is not an active member of the tenant. u1 must therefore be seeded as a
	// tenant member, mirroring the production identity chain.
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'u1','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)").Error)
	require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s1',1,'s1','u1','trpc')").Error)
	// Admission's security guard requires the snapshot's agent to be a live,
	// tenant-owned custom agent row (agent_security_guard.go); a1 without a
	// row fails closed as ErrAgentSecurityReleaseUnresolvable.
	require.NoError(t, db.Exec("INSERT INTO custom_agents (id,tenant_id,name) VALUES ('a1',1,'a1')").Error)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
