package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// SP3 (C-23): the /craft/scheduled-tasks HTTP matrix — the seven endpoints of
// spec §3 over the real service + repository + SQLite migrations, mounted
// through the real route registration, following the craft sessions harness
// above. The acting identity is per-request through the same X-Test-Identity
// closure (owner u1 / same-tenant viewer u2 / foreign tenant).

// scheduledHTTPEnv mounts the scheduled-task table on a test engine with the
// service's clock pinned at Monday 2026-09-21 12:00 UTC, so tickets and fires
// previews assert exact instants.
type scheduledHTTPEnv struct {
	db     *gorm.DB
	repo   repository.CraftScheduledTaskRepository
	svc    *service.CraftScheduledService
	engine *gin.Engine
}

func newScheduledHTTPEnv(t *testing.T) *scheduledHTTPEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	repo := repository.NewCraftScheduledTaskRepository(db)
	svc, err := service.NewCraftScheduledService(repo)
	require.NoError(t, err)
	pinned := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	svc.WithClock(func() time.Time { return pinned })
	env := &scheduledHTTPEnv{db: db, repo: repo, svc: svc, engine: gin.New()}
	env.engine.Use(middleware.ErrorHandler())
	env.engine.Use(func(c *gin.Context) {
		identity := c.GetHeader("X-Test-Identity")
		tenant := uint64(1)
		user, role := "u1", ""
		switch identity {
		case "viewer":
			user = "u2"
		case "admin":
			user, role = "u2", "admin"
		case "foreigntenant":
			tenant, user = 2, "u1"
		}
		c.Set(types.TenantIDContextKey.String(), tenant)
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		if role == "admin" {
			ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	group := env.engine.Group("/api/v1/craft/scheduled-tasks")
	RegisterCraftScheduledTaskRoutes(group, NewCraftScheduledHandler(svc))
	return env
}

func (env *scheduledHTTPEnv) do(t *testing.T, method, path, identity, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if identity != "" {
		req.Header.Set("X-Test-Identity", identity)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	return w
}

// scheduledJSON unpacks the standard envelope's data object.
func scheduledJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var parsed struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed), "body: %s", w.Body.String())
	require.True(t, parsed.Success)
	return parsed.Data
}

// scheduledList unpacks the data array + next_cursor envelope.
func scheduledList(t *testing.T, w *httptest.ResponseRecorder) ([]map[string]any, string) {
	t.Helper()
	var parsed struct {
		Success    bool             `json:"success"`
		Data       []map[string]any `json:"data"`
		NextCursor string           `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed), "body: %s", w.Body.String())
	require.True(t, parsed.Success)
	return parsed.Data, parsed.NextCursor
}

// scheduledTime parses one wire timestamp and compares by instant, so the
// storage driver's timezone rendering never decides the assertion.
func scheduledTime(t *testing.T, v any, want time.Time, label string) {
	t.Helper()
	raw, ok := v.(string)
	require.True(t, ok, "%s must be a timestamp string, got %v", label, v)
	got, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err, "%s: %q", label, raw)
	require.True(t, got.Equal(want), "%s: got %s want %s", label, got, want)
}

// scheduledCreate drives POST with an editor payload and returns the task id.
func (env *scheduledHTTPEnv) scheduledCreate(t *testing.T, name, mode, payload string, extra string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"prompt":"build the site","editor_mode":%q,"payload":%s`,
		name, mode, payload)
	if extra != "" {
		body += "," + extra
	}
	body += "}"
	w := env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks", "", body)
	require.Equal(t, http.StatusCreated, w.Code, "create body: %s", w.Body.String())
	return scheduledJSON(t, w)
}

// TestCraftScheduledHTTPCRUDWalk walks the recipe lifecycle over HTTP: create
// (compiled cron, armed ticket, no identity leakage), newest-first list,
// detail with the next-3-fires preview, partial update incl. the payload:null
// normalization (Task 2 review Minor-3), input rejections, and the soft
// delete's idempotent 204 with the post-delete invisibility.
func TestCraftScheduledHTTPCRUDWalk(t *testing.T) {
	env := newScheduledHTTPEnv(t)

	// Create compiles the interval editor and arms the first ticket (12:30).
	first := env.scheduledCreate(t, "interval task", "interval", `{"every_minutes":30}`, "")
	require.NotEmpty(t, first["id"])
	require.Equal(t, "*/30 * * * *", first["cron_expression"])
	require.Equal(t, "interval", first["editor_mode"])
	require.Equal(t, "active", first["status"])
	require.Equal(t, "build the site", first["prompt"])
	scheduledTime(t, first["next_run_at"], time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC), "first ticket")
	// Identity fields never ride the wire, and no run was requested.
	require.NotContains(t, first, "tenant_id")
	require.NotContains(t, first, "owner_id")
	require.NotContains(t, first, "run_id")
	require.NotContains(t, first, "deleted_at")

	// A second task (daily editor) makes the list order observable.
	second := env.scheduledCreate(t, "daily task", "daily", `{"at":"09:30"}`, "")
	require.Equal(t, "30 9 * * *", second["cron_expression"])

	w := env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks", "", "")
	require.Equal(t, http.StatusOK, w.Code, "list: %s", w.Body.String())
	rows, _ := scheduledList(t, w)
	require.Len(t, rows, 2)
	require.Equal(t, second["id"], rows[0]["id"], "newest first")
	require.Equal(t, first["id"], rows[1]["id"])

	// Detail previews the next three fires (daily 09:30 from Mon 12:00 →
	// Tue/Wed/Thu).
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+second["id"].(string), "", "")
	require.Equal(t, http.StatusOK, w.Code, "detail: %s", w.Body.String())
	detail := scheduledJSON(t, w)
	fires, ok := detail["next_fires"].([]any)
	require.True(t, ok, "next_fires array: %v", detail["next_fires"])
	require.Len(t, fires, 3)
	scheduledTime(t, fires[0], time.Date(2026, 9, 22, 9, 30, 0, 0, time.UTC), "fire 1")
	scheduledTime(t, fires[2], time.Date(2026, 9, 24, 9, 30, 0, 0, time.UTC), "fire 3")

	// Partial update: name/prompt keep the schedule (idempotent recompute).
	taskID := first["id"].(string)
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "",
		`{"name":"renamed","prompt":"rebuild it"}`)
	require.Equal(t, http.StatusOK, w.Code, "patch: %s", w.Body.String())
	patched := scheduledJSON(t, w)
	require.Equal(t, "renamed", patched["name"])
	require.Equal(t, "rebuild it", patched["prompt"])
	require.Equal(t, "*/30 * * * *", patched["cron_expression"])
	scheduledTime(t, patched["next_run_at"], time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC),
		"unchanged schedule keeps its ticket")

	// Rescheduling arrives as the mode+payload pair and recompiles the cron.
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "",
		`{"editor_mode":"advanced","payload":{"cron":"*/5 * * * *"}}`)
	require.Equal(t, http.StatusOK, w.Code, "reschedule: %s", w.Body.String())
	rescheduled := scheduledJSON(t, w)
	require.Equal(t, "advanced", rescheduled["editor_mode"])
	require.Equal(t, "*/5 * * * *", rescheduled["cron_expression"])
	scheduledTime(t, rescheduled["next_run_at"], time.Date(2026, 9, 21, 12, 5, 0, 0, time.UTC),
		"reschedule recomputes the ticket")

	// The pair rule: a lone mode is a 400, and an explicit payload:null is
	// normalized to ABSENT (Minor-3) — not a malformed payload.
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "",
		`{"editor_mode":"daily"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, "lone mode: %s", w.Body.String())
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "", `{"payload":null}`)
	require.Equal(t, http.StatusOK, w.Code, "payload null alone: %s", w.Body.String())
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "",
		`{"editor_mode":"daily","payload":null}`)
	require.Equal(t, http.StatusBadRequest, w.Code, "mode + null payload: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "pair")

	// Pausing parks the ticket; resuming re-arms it.
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "", `{"status":"paused"}`)
	require.Equal(t, http.StatusOK, w.Code, "pause: %s", w.Body.String())
	paused := scheduledJSON(t, w)
	require.Equal(t, "paused", paused["status"])
	require.Contains(t, paused, "next_run_at")
	require.Nil(t, paused["next_run_at"])
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "", `{"status":"active"}`)
	require.Equal(t, http.StatusOK, w.Code, "resume: %s", w.Body.String())
	resumed := scheduledJSON(t, w)
	scheduledTime(t, resumed["next_run_at"], time.Date(2026, 9, 21, 12, 5, 0, 0, time.UTC),
		"resume re-arms the ticket")

	// Input rejections: malformed bodies, bad cron, oversized name and
	// client-supplied identity fields never reach the service.
	for _, body := range []string{
		`{"name":"x","prompt":"p","editor_mode":"advanced","payload":{"cron":"61 * * * *"}}`,
		`{"name":"x","prompt":"p","editor_mode":"advanced","payload":{"cron":"0 0 30 2 *"}}`,
		`{"name":"x","prompt":"p","editor_mode":"weird","payload":{}}`,
		`{"name":"  ","prompt":"p","editor_mode":"advanced","payload":{"cron":"*/5 * * * *"}}`,
		"{\"name\":\"" + strings.Repeat("n", 129) + "\",\"prompt\":\"p\",\"editor_mode\":\"advanced\",\"payload\":{\"cron\":\"*/5 * * * *\"}}",
		`{"name":"x","prompt":"p","editor_mode":"advanced","payload":{"cron":"*/5 * * * *"},"tenant_id":2}`,
		`{"name":"x","prompt":"p","editor_mode":"advanced","payload":{"cron":"*/5 * * * *"},"owner_id":"u2"}`,
	} {
		w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks", "", body)
		require.Equal(t, http.StatusBadRequest, w.Code, "create reject %s: %s", body, w.Body.String())
	}
	w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks", "", `{}`)
	require.Equal(t, http.StatusBadRequest, w.Code, "empty-ish body")

	// Soft delete: 204 with no body, idempotent on repeat, and the recipe is
	// invisible afterwards (GET/PATCH 404) while its sibling stays.
	w = env.do(t, http.MethodDelete, "/api/v1/craft/scheduled-tasks/"+taskID, "", "")
	require.Equal(t, http.StatusNoContent, w.Code)
	require.Empty(t, w.Body.String())
	w = env.do(t, http.MethodDelete, "/api/v1/craft/scheduled-tasks/"+taskID, "", "")
	require.Equal(t, http.StatusNoContent, w.Code, "second delete")
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID, "", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "", `{"name":"zombie"}`)
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks", "", "")
	rows, _ = scheduledList(t, w)
	require.Len(t, rows, 1, "the sibling survives")
}

// TestCraftScheduledHTTPRunNowAndRunImmediately pins both manual-fire
// entrances: run_immediately on create appends the queued manual run without
// touching the ticket, and run-now answers 202 {run_id} — on a PAUSED recipe
// too, still without arming the ticket.
func TestCraftScheduledHTTPRunNowAndRunImmediately(t *testing.T) {
	env := newScheduledHTTPEnv(t)

	// run_immediately=true: the response carries the run id and the ticket
	// stays the schedule's own first fire.
	created := env.scheduledCreate(t, "immediate", "interval", `{"every_minutes":30}`,
		`"run_immediately":true`)
	runID, ok := created["run_id"].(string)
	require.True(t, ok, "run_id in create response: %v", created)
	require.NotEmpty(t, runID)
	scheduledTime(t, created["next_run_at"], time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC),
		"manual fire does not move the ticket")

	taskID := created["id"].(string)
	w := env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID+"/runs", "", "")
	require.Equal(t, http.StatusOK, w.Code, "runs: %s", w.Body.String())
	runs, _ := scheduledList(t, w)
	require.Len(t, runs, 1)
	require.Equal(t, runID, runs[0]["id"])
	require.Equal(t, "queued", runs[0]["status"])
	require.Equal(t, "manual_run_now", runs[0]["trigger_source"])

	// Without the flag no run id rides the response and no run row exists.
	plain := env.scheduledCreate(t, "plain", "daily", `{"at":"09:30"}`, "")
	require.NotContains(t, plain, "run_id")
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+plain["id"].(string)+"/runs", "", "")
	plainRuns, _ := scheduledList(t, w)
	require.Len(t, plainRuns, 0)

	// run-now on the active task: 202 with a fresh run id.
	w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks/"+taskID+"/run-now", "", "")
	require.Equal(t, http.StatusAccepted, w.Code, "run-now: %s", w.Body.String())
	manual := scheduledJSON(t, w)
	require.NotEmpty(t, manual["run_id"])
	require.NotEqual(t, runID, manual["run_id"])

	// run-now on a PAUSED recipe is allowed and never arms the ticket.
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "", `{"status":"paused"}`)
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks/"+taskID+"/run-now", "", "")
	require.Equal(t, http.StatusAccepted, w.Code, "paused run-now: %s", w.Body.String())
	require.NotEmpty(t, scheduledJSON(t, w)["run_id"])
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID, "", "")
	require.Nil(t, scheduledJSON(t, w)["next_run_at"], "paused stays unarmed")

	// The run ledger lists newest-first (both manual runs).
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID+"/runs", "", "")
	runs, _ = scheduledList(t, w)
	require.Len(t, runs, 3)

	// Wrong owner and missing tasks are the same 404.
	for _, identity := range []string{"viewer", "foreigntenant"} {
		w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks/"+taskID+"/run-now", identity, "")
		require.Equal(t, http.StatusNotFound, w.Code, "%s run-now: %s", identity, w.Body.String())
	}
	w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks/missing/run-now", "", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestCraftScheduledHTTPRunsPagination pins the keyset page: newest-first,
// the before cursor walk, and the limit/before validation edges (spec §3:
// limit 1..100 default 50, before = started_at ISO).
func TestCraftScheduledHTTPRunsPagination(t *testing.T) {
	env := newScheduledHTTPEnv(t)
	created := env.scheduledCreate(t, "paged", "interval", `{"every_minutes":30}`, "")
	taskID := created["id"].(string)

	// Seed five terminal runs with distinct fire instants.
	ctx := context.Background()
	for i := 4; i >= 0; i-- {
		at := time.Date(2026, 9, 21, 12, i, 0, 0, time.UTC)
		run := &types.CraftScheduledTaskRun{
			TaskID: taskID, Status: types.CraftScheduledRunStatusSucceeded,
			TriggerSource: types.CraftScheduledTriggerScheduled, StartedAt: &at,
		}
		if i == 2 {
			run.Status = types.CraftScheduledRunStatusFailed
			ec, detail := "stuck", "worker lease expired"
			run.ErrorClass, run.ErrorDetail = &ec, &detail
		}
		require.NoError(t, env.repo.InsertRun(ctx, run))
	}

	base := "/api/v1/craft/scheduled-tasks/" + taskID + "/runs"
	w := env.do(t, http.MethodGet, base, "", "")
	require.Equal(t, http.StatusOK, w.Code)
	runs, next := scheduledList(t, w)
	require.Len(t, runs, 5, "default limit 50 keeps the whole history")
	require.Empty(t, next)
	scheduledTime(t, runs[0]["started_at"], time.Date(2026, 9, 21, 12, 4, 0, 0, time.UTC), "newest first")
	require.Equal(t, "failed", runs[2]["status"])
	require.Equal(t, "stuck", runs[2]["error_class"])

	// limit=2 pages strictly before the last seen started_at.
	w = env.do(t, http.MethodGet, base+"?limit=2", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	page1, next1 := scheduledList(t, w)
	require.Len(t, page1, 2)
	scheduledTime(t, page1[0]["started_at"], time.Date(2026, 9, 21, 12, 4, 0, 0, time.UTC), "page1[0]")
	require.NotEmpty(t, next1)
	w = env.do(t, http.MethodGet, base+"?limit=2&before="+next1, "", "")
	require.Equal(t, http.StatusOK, w.Code, "page2: %s", w.Body.String())
	page2, next2 := scheduledList(t, w)
	require.Len(t, page2, 2)
	scheduledTime(t, page2[0]["started_at"], time.Date(2026, 9, 21, 12, 2, 0, 0, time.UTC), "page2[0]")
	scheduledTime(t, page2[1]["started_at"], time.Date(2026, 9, 21, 12, 1, 0, 0, time.UTC), "page2[1]")
	w = env.do(t, http.MethodGet, base+"?limit=2&before="+next2, "", "")
	require.Equal(t, http.StatusOK, w.Code)
	page3, next3 := scheduledList(t, w)
	require.Len(t, page3, 1)
	scheduledTime(t, page3[0]["started_at"], time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), "page3[0]")
	require.Empty(t, next3)

	// Boundary limits are accepted, out-of-band and malformed are 400s. An
	// EMPTY limit/before value reads as unset (the list-route precedent) and
	// falls back to the defaults.
	w = env.do(t, http.MethodGet, base+"?limit=1", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodGet, base+"?limit=100", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodGet, base+"?limit=", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodGet, base+"?before=", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	for _, query := range []string{
		"limit=0", "limit=101", "limit=-1", "limit=abc",
		"before=not-a-time", "before=2026-09-21",
	} {
		w = env.do(t, http.MethodGet, base+"?"+query, "", "")
		require.Equal(t, http.StatusBadRequest, w.Code, "query %q: %s", query, w.Body.String())
	}

	// Ownership: a foreign owner's run history reads as 404.
	w = env.do(t, http.MethodGet, base, "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodGet, base, "foreigntenant", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

// TestCraftScheduledHTTPOwnershipAndFailClosed pins the scoping and mounting
// contracts: a same-tenant viewer never sees another owner's recipes (the
// idempotent delete is its one 204 no-op), a foreign tenant sees nothing, a
// nil service answers 503 everywhere, an unregistered surface mounts no
// route, and a request without an authenticated identity answers 401.
func TestCraftScheduledHTTPOwnershipAndFailClosed(t *testing.T) {
	env := newScheduledHTTPEnv(t)
	created := env.scheduledCreate(t, "mine", "interval", `{"every_minutes":30}`, "")
	taskID := created["id"].(string)

	// Same-tenant viewer: invisible reads, refused writes — except the
	// idempotent delete, whose no-op success IS the desired end state.
	w := env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID, "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPatch, "/api/v1/craft/scheduled-tasks/"+taskID, "viewer", `{"name":"x"}`)
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/craft/scheduled-tasks/"+taskID+"/run-now", "viewer", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks", "viewer", "")
	require.Equal(t, http.StatusOK, w.Code)
	viewerRows, _ := scheduledList(t, w)
	require.Len(t, viewerRows, 0)
	w = env.do(t, http.MethodDelete, "/api/v1/craft/scheduled-tasks/"+taskID, "viewer", "")
	require.Equal(t, http.StatusNoContent, w.Code, "foreign delete is a no-op success")
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID, "", "")
	require.Equal(t, http.StatusOK, w.Code, "the no-op delete left the recipe live")

	// A foreign tenant sees an empty list and 404s everywhere else.
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks", "foreigntenant", "")
	require.Equal(t, http.StatusOK, w.Code)
	foreignRows, _ := scheduledList(t, w)
	require.Len(t, foreignRows, 0)
	w = env.do(t, http.MethodGet, "/api/v1/craft/scheduled-tasks/"+taskID, "foreigntenant", "")
	require.Equal(t, http.StatusNotFound, w.Code)

	// A nil service answers 503 on every endpoint without touching anything.
	nilHandler := NewCraftScheduledHandler(nil)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	nilGroup := engine.Group("/api/v1/craft/scheduled-tasks")
	RegisterCraftScheduledTaskRoutes(nilGroup, nilHandler)
	for method, path := range map[string]string{
		http.MethodGet:    "/api/v1/craft/scheduled-tasks",
		http.MethodPost:   "/api/v1/craft/scheduled-tasks",
		http.MethodPatch:  "/api/v1/craft/scheduled-tasks/t1",
		http.MethodDelete: "/api/v1/craft/scheduled-tasks/t1",
	} {
		req := httptest.NewRequest(method, path, nil)
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "%s %s", method, path)
	}
	for method, path := range map[string]string{
		http.MethodPost: "/api/v1/craft/scheduled-tasks/t1/run-now",
		http.MethodGet:  "/api/v1/craft/scheduled-tasks/t1/runs",
	} {
		req := httptest.NewRequest(method, path, nil)
		resp := httptest.NewRecorder()
		engine.ServeHTTP(resp, req)
		require.Equal(t, http.StatusServiceUnavailable, resp.Code, "%s %s", method, path)
	}

	// Without a handler the surface does not exist at all (fail-closed).
	bare := gin.New()
	RegisterCraftScheduledTaskRoutes(bare.Group("/api/v1/craft/scheduled-tasks"), nil)
	w = httptest.NewRecorder()
	bare.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/craft/scheduled-tasks", nil))
	require.Equal(t, http.StatusNotFound, w.Code)

	// Without an authenticated tenant+user the surface answers 401.
	noIdentity := gin.New()
	noIdentity.Use(middleware.ErrorHandler())
	RegisterCraftScheduledTaskRoutes(
		noIdentity.Group("/api/v1/craft/scheduled-tasks"), NewCraftScheduledHandler(env.svc))
	w = httptest.NewRecorder()
	noIdentity.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/craft/scheduled-tasks", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code, "unauthenticated list: %s", w.Body.String())
}
