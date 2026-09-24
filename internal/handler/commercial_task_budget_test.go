package handler

// Budget-extension gate tests (T12 #42): raising a task budget admits the
// TASK OWNER or a billing-authorized caller — never a collaborator or
// bystander — and keeps the 404 TASK_BUDGET_NOT_FOUND contract for unknown
// runs. All on a real sqlite database through the real handler + route.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newBudgetGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// The owner predicate only reads agent_runs(tenant_id, run_id, owner_id).
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_budget_accounts (
		tenant_id INTEGER PRIMARY KEY, verified_micro BIGINT NOT NULL, unreflected_micro BIGINT NOT NULL DEFAULT 0,
		held_micro BIGINT NOT NULL DEFAULT 0, refund_locked_micro BIGINT NOT NULL DEFAULT 0,
		verified_until DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 0)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE commercial_task_budgets (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, limit_micro BIGINT NOT NULL,
		spent_micro BIGINT NOT NULL DEFAULT 0, held_micro BIGINT NOT NULL DEFAULT 0,
		deadline DATETIME NOT NULL, version BIGINT NOT NULL DEFAULT 1,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (7, 'r1', 'u1')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, verified_until, version) VALUES (7, 100000000, ?, 0)`,
		time.Now().Add(time.Hour).UTC()).Error)
	require.NoError(t, db.Exec(`INSERT INTO commercial_task_budgets
		(tenant_id, run_id, limit_micro, deadline, version) VALUES (7, 'r1', 1000, ?, 1)`,
		time.Now().Add(time.Hour).UTC()).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func postBudgetExtend(t *testing.T, db *gorm.DB, role, userID, runID, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCommercialHandler(db)
	r := gin.New()
	r.POST("/commercial/tasks/:id/budget/extend", h.ExtendTaskBudget)
	body := `{"additional_credits": 10, "idempotency_key": "` + idempotencyKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/commercial/tasks/"+runID+"/budget/extend", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	if role != "" {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	}
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExtendTaskBudgetAdmitsTaskOwner(t *testing.T) {
	db := newBudgetGateDB(t)
	// The task owner is a plain contributor: no tenant-owner role, no billing
	// grant. Before T12 this caller was 403'd by the route-level billing gate.
	w := postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code, "task owner may raise their own budget: %s", w.Body.String())

	// Idempotent replay of the same key stays a success.
	w = postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestExtendTaskBudgetRejectsCollaboratorAndBystander(t *testing.T) {
	db := newBudgetGateDB(t)
	// The owner applies an idempotency key FIRST, so the replay below hits an
	// ALREADY-APPLIED key: the role gate must still refuse the collaborator
	// (gate order: role before idempotency).
	w := postBudgetExtend(t, db, "contributor", "u1", "r1", "k-owner-1")
	require.Equal(t, http.StatusOK, w.Code, "owner applies the key first: %s", w.Body.String())

	// The collaborator (contributor role, not the run owner) replaying the
	// owner's applied key is still refused with the role-gate 403 — never the
	// idempotent 200 a legitimate replayer would see.
	w = postBudgetExtend(t, db, "contributor", "u2", "r1", "k-owner-1")
	require.Equal(t, http.StatusForbidden, w.Code, "collaborator replaying an applied key is refused")
	require.Contains(t, w.Body.String(), "BUDGET_FORBIDDEN")

	// A viewer member is refused too.
	w = postBudgetExtend(t, db, "viewer", "u3", "r1", "k-fresh-1")
	require.Equal(t, http.StatusForbidden, w.Code)

	// An admin without a billing grant is refused (administrative role alone
	// never grants purchase authority — unchanged semantics).
	w = postBudgetExtend(t, db, "admin", "u4", "r1", "k-fresh-2")
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestExtendTaskBudgetBillingGrantAndNotFoundKeepContract(t *testing.T) {
	db := newBudgetGateDB(t)
	// Billing authority (tenant owner role) keeps the existing admission.
	w := postBudgetExtend(t, db, "owner", "boss", "r1", "k-boss-1")
	require.Equal(t, http.StatusOK, w.Code, "billing-authorized caller keeps admission: %s", w.Body.String())

	// Unknown run keeps the explicit 404 TASK_BUDGET_NOT_FOUND (no 403 leak
	// of run existence to a non-owner).
	w = postBudgetExtend(t, db, "contributor", "u2", "missing", "k-fresh-3")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "TASK_BUDGET_NOT_FOUND")

	// Cross-tenant run is equally a 404.
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (8, 'r8', 'u1')`).Error)
	w = postBudgetExtend(t, db, "contributor", "u1", "r8", "k-fresh-4")
	require.Equal(t, http.StatusNotFound, w.Code)
}
