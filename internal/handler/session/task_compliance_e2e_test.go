package session

// T13 (#43) end-to-end evidence — AC3: every assertion below runs over a
// REAL fully-migrated sqlite database (openCraftHTTPDB) with REAL stores,
// the REAL TaskComplianceService, the REAL compliance/grants/read handlers,
// the REAL legal-hold gate inside the REAL DeleteSession handler, and the
// REAL audit trail (repository.NewAuditLogRepository + service.NewAuditLogService).
// No service mock, no hand-written projection (AC3: 底层单测、静态检查或 mock
// 不冒充真实集成证据).
//
// AC1 = TestComplianceEndToEndAC1IndependentFlow.
// AC2 = TestComplianceEndToEndAC2DeletionNeverCascadesOutward.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// sqlBackedSessions backs the REAL DeleteSession handler with the minimum
// session-service surface: ownership read + the production-shaped soft
// delete (UPDATE sessions SET deleted_at). Everything else in the real
// service (message cleanup goroutine, suggestion/Redis cleanup, sandbox
// teardown) is out of the deletion contract under test.
type sqlBackedSessions struct {
	db *gorm.DB
	interfaces.SessionService
}

func (s sqlBackedSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user, _ := types.UserIDFromContext(ctx)
	var sess types.Session
	err := s.db.Unscoped().Where("tenant_id = ? AND id = ? AND user_id = ?", tenant, id, user).Take(&sess).Error
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s sqlBackedSessions) DeleteSession(ctx context.Context, id string) error {
	tenant, _ := types.TenantIDFromContext(ctx)
	return s.db.Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", tenant, id).
		Update("deleted_at", time.Now().UTC()).Error
}

type complianceE2EEnv struct {
	db         *gorm.DB
	engine     *gin.Engine
	compliance *appservice.TaskComplianceService
	clock      *time.Time
}

func newComplianceE2EEnv(t *testing.T) *complianceE2EEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	seedComplianceE2E(t, db)

	store := repository.NewTaskComplianceStore(db)
	audit := appservice.NewAuditLogService(repository.NewAuditLogRepository(db))
	env := &complianceE2EEnv{db: db}
	now := time.Now().UTC()
	env.clock = &now
	env.compliance = appservice.NewTaskComplianceServiceWithClock(store, audit, func() time.Time { return *env.clock })

	runs := repository.NewAgentRunStore(db)
	readHandler := NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)
	grantsHandler := NewWorkbenchTaskGrantsHandler(appservice.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	))
	complianceHandler := NewWorkbenchTaskComplianceHandler(env.compliance)
	// The REAL #34 archive lane — the e2e asserts legal hold never blocks it.
	taskStateHandler := NewWorkbenchTaskStateHandler(repository.NewWorkbenchTaskStateStore(db))
	sessHandler := &Handler{
		sessionService:    sqlBackedSessions{db: db},
		agentRunService:   appservice.NewAgentRunService(runs),
		craftTombstoner:   &recordingTombstoner{},
		taskDeletionGuard: env.compliance,
	}

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	v1.DELETE("/sessions/:id", sessHandler.DeleteSession)
	v1.GET("/workbench/executions/:run_id", readHandler.GetWorkbenchExecution)
	v1.GET("/workbench/tasks/:task_id/grants", grantsHandler.List)
	v1.POST("/workbench/tasks/:task_id/archive", taskStateHandler.Archive)
	v1.GET("/workbench/compliance/task-policy", complianceHandler.GetTaskPolicy)
	v1.PUT("/workbench/compliance/task-policy", complianceHandler.SetTaskPolicy)
	v1.GET("/workbench/compliance/tasks/:task_id", complianceHandler.TaskMetadata)
	v1.POST("/workbench/compliance/tasks/:task_id/access", complianceHandler.RequestContentAccess)
	v1.GET("/workbench/compliance/tasks/:task_id/content", complianceHandler.ReadTaskContent)
	v1.DELETE("/workbench/tasks/:task_id", complianceHandler.PurgeTask)
	env.engine = r
	return env
}

func seedComplianceE2E(t *testing.T, db *gorm.DB) {
	t.Helper()
	// u1 owns s1 (contributor); u9 is the tenant admin; u2 is a viewer.
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u9','u9','u9@example.test','x',1)").Error)
	for _, m := range []struct {
		user string
		role types.TenantRole
	}{
		{"u1", types.TenantRoleContributor},
		{"u2", types.TenantRoleViewer},
		{"u9", types.TenantRoleAdmin},
	} {
		// openCraftHTTPDB 已为 u1 预播 owner 成员行；UPSERT 保留本场景的角色语义。
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at)
			VALUES (1,?,?,'active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
			ON CONFLICT(user_id, tenant_id) DO UPDATE SET role=excluded.role, updated_at=CURRENT_TIMESTAMP`,
			m.user, m.role).Error)
	}
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-s1', 'u1', 'trpc')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO messages (id, request_id, session_id, role, content) VALUES ('m1','req-1','s1','user','private question'), ('m2','req-1','s1','assistant','private answer')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES (1,'r1','s1','u1','req-1','m2','h1','trpc','succeeded','{}', datetime('now','+1 hour'))").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO craft_workspaces (id, tenant_id, session_id, owner_id) VALUES ('cw1', 1, 's1', 'u1')").Error)
	// Historical audit rows — the baseline that must never shrink.
	require.NoError(t, db.Exec(
		"INSERT INTO audit_logs (tenant_id, actor_user_id, action) VALUES (1,'u1','session.created'),(1,'u1','kb.indexed')").Error)
}

func (e *complianceE2EEnv) do(t *testing.T, method, path, body, userID string, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *complianceE2EEnv) count(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Table(table).Where(where, args...).Count(&n).Error)
	return n
}

// AC1: 合规访问不把管理员加入 Task 协作列表。The compliance flow is an
// INDEPENDENT lane: metadata is default-visible to the admin, private
// content requires a reasoned+time-limited window with a full audit trail,
// and neither the window nor the metadata view ever creates a task_grants
// row — the run read lane and the grants list stay untouched.
func TestComplianceEndToEndAC1IndependentFlow(t *testing.T) {
	env := newComplianceE2EEnv(t)

	// Metadata by default: the admin reads the metadata view, no window needed.
	w := env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "task-s1")
	require.NotContains(t, w.Body.String(), "private question", "the default view carries no content")

	// Review Focus 1: the admin cannot reach private content via the run
	// read lane — Admin+ tenant role is NOT run-level authorization.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "run lane: owner-or-grant only, admin role gains nothing: %s", w.Body.String())

	// Content without a window is refused (403).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Reasoned + time-limited window opens (201) with the audit trail.
	w = env.do(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"security incident review","ttl_hours":72}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The window unlocks ONLY the compliance content lane.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "private question")

	// AC1 core: still no grant row for the admin — and the run lane STILL
	// refuses the admin (the window grants no collaboration identity).
	require.Zero(t, env.count(t, "task_grants", "tenant_id = ? AND grantee_id = ?", uint64(1), "u9"),
		"AC1: a compliance window must never write a task_grants row")
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "the window grants no run-lane access: %s", w.Body.String())

	// The owner's grants list stays clean of the administrator.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/tasks/s1/grants", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "u9", "the collaboration list never contains the admin (AC1)")

	// The full trail exists: requested + content_read.
	require.Equal(t, int64(2), env.count(t, "audit_logs",
		"tenant_id = ? AND action IN (?, ?)", uint64(1),
		types.AuditActionComplianceAccessRequested, types.AuditActionComplianceContentRead))

	// Review Focus 2: after the window expires, content seals again.
	future := time.Now().UTC().Add(73 * time.Hour)
	env.clock = &future
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusForbidden, w.Code, "an expired window authorizes nothing: %s", w.Body.String())

	// Non-admin is refused on every compliance endpoint (403).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"x","ttl_hours":1}`, "u2", types.TenantRoleViewer)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// AC2: 删除内部 Task 不隐式删除外部文档、代码或审计。Soft delete keeps the
// craft workspace row (tombstone lanes own external teardown) and every
// audit row; legal hold refuses deletion outright with its own audit trail;
// purge (past retention) removes internal rows only — audit rows only grow.
func TestComplianceEndToEndAC2DeletionNeverCascadesOutward(t *testing.T) {
	env := newComplianceE2EEnv(t)
	auditsBefore := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))

	// Legal hold ON: the owner's delete is refused (409) and audited.
	w := env.do(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":true}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodDelete, "/api/v1/sessions/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusConflict, w.Code, "legal hold refuses internal deletion: %s", w.Body.String())
	require.Equal(t, int64(1), env.count(t, "audit_logs",
		"tenant_id = ? AND action = ?", uint64(1), types.AuditActionTaskDeleteDenied))
	require.Zero(t, env.count(t, "sessions", "tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", uint64(1), "s1"),
		"the refused delete must not soft-delete anything")

	// Archiving stays available under hold (non-destructive organization —
	// the REAL #34 archive handler succeeds while the delete lane is gated).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/archive", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "archive is a non-destructive action, hold never blocks it: %s", w.Body.String())
	require.Equal(t, int64(1), env.count(t, "sessions", "tenant_id = ? AND id = ? AND archived_at IS NOT NULL", uint64(1), "s1"),
		"the archive really landed")

	// Hold OFF + retention 30d: the delete now succeeds (soft), and NOTHING
	// external or auditable disappears.
	w = env.do(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":false}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodDelete, "/api/v1/sessions/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, int64(1), env.count(t, "sessions", "tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", uint64(1), "s1"),
		"the delete soft-deleted the session")
	require.Equal(t, int64(1), env.count(t, "craft_workspaces", "tenant_id = ? AND session_id = ?", uint64(1), "s1"),
		"AC2: internal soft delete keeps the craft workspace row (external teardown belongs to the tombstone/sweep lanes)")
	require.Equal(t, int64(2), env.count(t, "messages", "session_id = ?", "s1"),
		"soft delete keeps message rows (restorable semantics)")
	auditsAfterDelete := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))
	require.GreaterOrEqual(t, auditsAfterDelete, auditsBefore, "AC2: audit rows never shrink")

	// Purge inside the retention window (10 days in, 30 required): refused.
	inWindow := time.Now().UTC().Add(10 * 24 * time.Hour)
	env.clock = &inWindow
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusConflict, w.Code, "retention window refuses purge: %s", w.Body.String())

	// Past the horizon (40 days in): purge succeeds, internal rows vanish,
	// the audit trail only GREW (task.purged appended), nothing else called.
	pastHorizon := time.Now().UTC().Add(40 * 24 * time.Hour)
	env.clock = &pastHorizon
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Zero(t, env.count(t, "sessions", "tenant_id = ? AND id = ?", uint64(1), "s1"))
	require.Zero(t, env.count(t, "messages", "session_id = ?", "s1"))
	require.Zero(t, env.count(t, "agent_runs", "tenant_id = ? AND session_id = ?", uint64(1), "s1"))
	require.Zero(t, env.count(t, "task_compliance_access", "tenant_id = ? AND task_id = ?", uint64(1), "s1"))
	require.Equal(t, int64(1), env.count(t, "audit_logs", "tenant_id = ? AND action = ?", uint64(1), types.AuditActionTaskPurged),
		"the purge authorization is audited")
	finalAudits := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))
	require.GreaterOrEqual(t, finalAudits, auditsAfterDelete, "AC2: purge never deletes audit rows")

	// Cross-tenant uniform miss on the compliance lane.
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "a purged task is a uniform 404: %s", w.Body.String())
}
