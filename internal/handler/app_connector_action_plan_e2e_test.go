package handler

// End-to-end evidence for T21 (#51). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PlanStore / NotionPublishService / NotionBridge
// / handlers and the REAL approval machinery. The ONLY replaced piece is
// the Notion wire endpoint (the #48 contract double) — this is the
// highest stable Interface evidence available without provider
// credentials; it is NOT the real-provider acceptance, which stays
// NOTION_TOKEN-gated in notion_publish_real_test.go (single-action leg)
// and is blocked-env for the plan-level loop in this environment.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/plan"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestAppActionPlansTablesExistAfterMigrations: the plan tables must be
// created by the PRODUCTION migrations (columns aligned with the Go rows
// — the workbench-notifications precedent).
func TestAppActionPlansTablesExistAfterMigrations(t *testing.T) {
	db := openNotionPublishE2EDB(t)
	require.True(t, db.Migrator().HasTable("app_action_plans"),
		"app_action_plans must be created by the production migrations")
	for _, column := range []string{"tenant_id", "id", "actor_id", "digest", "state", "excluded_json", "approved_by", "approved_at"} {
		require.True(t, db.Migrator().HasColumn("app_action_plans", column),
			"app_action_plans.%s must exist (aligned with ActionPlanRow)", column)
	}
	require.True(t, db.Migrator().HasTable("app_action_plan_items"),
		"app_action_plan_items must be created by the production migrations")
	for _, column := range []string{"tenant_id", "plan_id", "seq", "action_id"} {
		require.True(t, db.Migrator().HasColumn("app_action_plan_items", column),
			"app_action_plan_items.%s must exist (aligned with ActionPlanItemRow)", column)
	}
}

// ---- the full-stack environment ----

type actionPlanE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eNotion
}

// newActionPlanE2E assembles the SAME production composition as
// newNotionPublishE2E plus the plan service and routes. The seeding is
// deliberately duplicated (not refactored out of the frozen #48 fixture)
// so this file stays merge-isolated.
func newActionPlanE2E(t *testing.T) *actionPlanE2EEnv {
	t.Helper()
	db := openNotionPublishE2EDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	baseDir := t.TempDir()
	digest := strings.Repeat("a", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("第一段。\n\n第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', 15)`, digest, objectKey).Error)
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-notion', 7, 'notion', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-notion', 'inst-notion', 'personal', 'user-a', 'mcp_oauth_token:notion', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('notion', 7, 'notion', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-1', 7, 'user-a', 'web_user', 'user-a', 'notion', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)
	fake := newE2ENotion("secret_test_token")
	fake.addPage("parent-1", "root", "Workspace")
	srv := fake.server(t)

	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewNotionBridge(scopeSrc, &e2ePolicyProvider{pol: pol},
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))), pubs)
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &e2eLocalContent{svc: file.NewLocalFileService(baseDir, "")}
	publishSvc := publish.NewNotionPublishService(publishActions, store, pubs,
		repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)
	planSvc := plan.NewService(repoappconn.NewPlanStore(db), store, publishActions, publishSvc)

	publishHandler := NewAppNotionPublishHandler(db)
	publishHandler.SetNotionPublishService(publishSvc)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(publishActions)
	planHandler := NewAppActionPlanHandler(db)
	planHandler.SetActionPlanService(planSvc)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		role := types.TenantRoleAdmin
		if c.GetHeader("X-Test-Role") != "" {
			role = types.TenantRole(c.GetHeader("X-Test-Role"))
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	v1.GET("/apps/actions/:id", actionHandler.GetAction)
	gp := v1.Group("/apps/notion-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		gp.POST("/actions/:id/reconcile", publishHandler.ReconcileNotionAction)
		gp.GET("/actions/:id", publishHandler.GetNotionPublication)
	}
	g := v1.Group("/apps/action-plans", planHandler.RequireActionCapabilityForWrites())
	{
		g.POST("", planHandler.FormActionPlan)
		g.POST("/:id/approve", planHandler.ApproveActionPlan)
		g.POST("/:id/execute", planHandler.ExecuteActionPlan)
		g.GET("/:id", planHandler.GetActionPlan)
	}
	return &actionPlanE2EEnv{engine: engine, db: db, fake: fake}
}

func (e *actionPlanE2EEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func planCreateItem(title string) string {
	return fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":%q,"parent_page_id":"parent-1"}`, title)
}

func planUpdateItem(title, pageID string) string {
	return fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":%q,"page_id":%q}`, title, pageID)
}

type formedPlan struct {
	id, digest string
}

func (e *actionPlanE2EEnv) formPlanItems(t *testing.T, items ...string) formedPlan {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans", `{"items":[`+strings.Join(items, ",")+`]}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data struct {
			ID     string `json:"id"`
			Digest string `json:"digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return formedPlan{id: parsed.Data.ID, digest: parsed.Data.Digest}
}

func (e *actionPlanE2EEnv) formPlan(t *testing.T, titles ...string) formedPlan {
	t.Helper()
	items := make([]string, 0, len(titles))
	for _, title := range titles {
		items = append(items, planCreateItem(title))
	}
	return e.formPlanItems(t, items...)
}

func (e *actionPlanE2EEnv) approve(t *testing.T, p formedPlan) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func (e *actionPlanE2EEnv) approveExcluding(t *testing.T, p formedPlan, seqs []int) {
	t.Helper()
	raw, _ := json.Marshal(seqs)
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q,"exclude_seqs":%s}`, p.digest, raw))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

type e2eItemOutcome struct {
	Seq         int    `json:"seq"`
	ActionID    string `json:"action_id"`
	Disposition string `json:"disposition"`
	ActionState string `json:"action_state"`
	Conflict    bool   `json:"conflict"`
	Publication struct {
		State           string `json:"state"`
		ExternalID      string `json:"external_id"`
		ExternalVersion string `json:"external_version"`
	} `json:"publication"`
}

func (e *actionPlanE2EEnv) execute(t *testing.T, p formedPlan) []e2eItemOutcome {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			Items []e2eItemOutcome `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data.Items
}

// publishOne creates exactly one page through a ONE-item plan and
// returns its external id — a receipt-backed update target minted
// through the plan seam itself.
func (e *actionPlanE2EEnv) publishOne(t *testing.T, title string) string {
	t.Helper()
	p := e.formPlan(t, title)
	e.approve(t, p)
	items := e.execute(t, p)
	require.Len(t, items, 1)
	require.Equal(t, "succeeded", items[0].ActionState, fmt.Sprintf("%+v", items))
	require.NotEmpty(t, items[0].Publication.ExternalID)
	return items[0].Publication.ExternalID
}

// TestActionPlanEndToEndApproveExecutePerItemResults: the whole-plan
// chain — form a THREE-item plan through the real publish seam, approve
// the WHOLE plan once, execute, and read per-item receipts. Each item is
// an independent page creation and records its own result.
func TestActionPlanEndToEndApproveExecutePerItemResults(t *testing.T) {
	env := newActionPlanE2E(t)
	p := env.formPlan(t, "Report A", "Report B", "Report C")

	w := env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+p.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, p.digest))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	out := env.execute(t, p)
	require.Len(t, out, 3)
	for i, it := range out {
		require.Equal(t, i+1, it.Seq)
		require.Equal(t, "executed", it.Disposition, "item %d: %+v", i+1, it)
		require.Equal(t, "succeeded", it.ActionState)
		require.Equal(t, "published", it.Publication.State)
		require.NotEmpty(t, it.Publication.ExternalID)
		require.NotEmpty(t, it.Publication.ExternalVersion, "each receipt saves the external version")
	}
	// Three REAL pages exist remotely with the derived blocks.
	env.fake.lock()
	require.Len(t, env.fake.pages, 4, "parent + three created pages")
	env.fake.unlock()
	// The durable projection agrees.
	w = env.do(t, http.MethodGet, "/api/v1/apps/action-plans/"+p.id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`)
	require.Contains(t, w.Body.String(), `"state":"published"`)
}

// TestActionPlanEndToEndContentChangeInvalidatesOldApproval (AC1): a
// plan whose content changed (different items → different digest) can
// never be approved or executed with the OLD plan's digest — the old
// approval authorizes only its own frozen content, and every refusal
// happens BEFORE any dispatch.
func TestActionPlanEndToEndContentChangeInvalidatesOldApproval(t *testing.T) {
	env := newActionPlanE2E(t)
	oldPlan := env.formPlan(t, "Report A", "Report B")
	newPlan := env.formPlan(t, "Report A v2", "Report B v2")
	require.NotEqual(t, oldPlan.digest, newPlan.digest, "计划内容变化必须产生新 plan digest")

	w := env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/approve",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	w = env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	// The new content can never ride the old plan's approval either:
	// approve the OLD plan, then present ITS digest against the NEW
	// plan — the same refusal.
	env.approve(t, oldPlan)
	w = env.do(t, http.MethodPost, "/api/v1/apps/action-plans/"+newPlan.id+"/execute",
		fmt.Sprintf(`{"digest":%q}`, oldPlan.digest))
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_PLAN_DIGEST_MISMATCH")

	// Zero dispatches happened anywhere (fail closed before any send).
	env.fake.lock()
	require.Len(t, env.fake.pages, 1, "parent only — no page was ever created")
	env.fake.unlock()
}

// TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly (AC2): a
// three-item ordered plan [create-ok, update-stale, create-unknown]:
// pass 1 records executed/succeeded + executed/failed(version conflict,
// zero writes) + executed/unknown (blocks landed, reply lost); pass 2
// (resume, same digest) re-dispatches NOTHING; the unknown item resolves
// through the provider query; pass 3 reads skipped_succeeded for both
// recovered items. External write counts prove the recovery never
// repeats a confirmed action.
func TestActionPlanEndToEndPartialSuccessResumesUnfinishedOnly(t *testing.T) {
	env := newActionPlanE2E(t)
	seed := env.publishOne(t, "Seed")

	// The plan's update item binds the CURRENT external version at
	// formation; the collaborator edit comes AFTER approval.
	items := []string{planCreateItem("Doc A"), planUpdateItem("Seed v2", seed), planCreateItem("Doc Ghost")}
	p := env.formPlanItems(t, items...)
	env.fake.dropAppendForTitle = "Doc Ghost"
	env.approve(t, p)
	env.fake.touch(seed, "2026-09-24T10:30:00.000Z")

	// Pass 1: partial success — succeeded + failed(conflict) + unknown.
	out := env.execute(t, p)
	require.Len(t, out, 3)
	require.Equal(t, "executed", out[0].Disposition)
	require.Equal(t, "succeeded", out[0].ActionState)
	require.Equal(t, "executed", out[1].Disposition)
	require.Equal(t, "failed", out[1].ActionState)
	require.True(t, out[1].Conflict, "the stale update must be a definitive version conflict: %+v", out[1])
	require.Equal(t, "executed", out[2].Disposition)
	require.Equal(t, "unknown", out[2].ActionState)

	env.fake.lock()
	pagesAfterPass1 := len(env.fake.pages)    // parent + Seed + Doc A + Doc Ghost
	appendsAfterPass1 := env.fake.appendCalls // Seed + Doc A + Doc Ghost(applied)
	patchesAfterPass1 := env.fake.patchCalls  // 0: the conflict refused BEFORE any write
	env.fake.unlock()
	require.Equal(t, 4, pagesAfterPass1, "Seed, Doc A and Doc Ghost must all exist")
	require.Equal(t, 0, patchesAfterPass1, "the conflicted update must write NOTHING")

	// Pass 2 (resume, same digest): confirmed outcomes never re-sent.
	out2 := env.execute(t, p)
	require.Len(t, out2, 3)
	require.Equal(t, "skipped_succeeded", out2[0].Disposition)
	require.Equal(t, "settled", out2[1].Disposition)
	require.Equal(t, "failed", out2[1].ActionState)
	require.Equal(t, "settled", out2[2].Disposition)
	require.Equal(t, "unknown", out2[2].ActionState)
	env.fake.lock()
	require.Equal(t, appendsAfterPass1, env.fake.appendCalls, "AC2: resume re-dispatched nothing")
	require.Equal(t, pagesAfterPass1, len(env.fake.pages), "AC2: no duplicate page")
	env.fake.unlock()

	// Reconcile the unknown item through the EXISTING per-action endpoint
	// (provider query first): the effect HAD applied remotely → succeeded.
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+out[2].ActionID+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`)
	require.Contains(t, w.Body.String(), `"state":"published"`)

	// Pass 3: both recovered legs read as confirmed outcomes.
	out3 := env.execute(t, p)
	require.Len(t, out3, 3)
	require.Equal(t, "skipped_succeeded", out3[0].Disposition)
	require.Equal(t, "settled", out3[1].Disposition)
	require.Equal(t, "skipped_succeeded", out3[2].Disposition)
	env.fake.lock()
	require.Equal(t, appendsAfterPass1, env.fake.appendCalls, "reconcile + resume never re-send blocks")
	require.Equal(t, 0, env.fake.patchCalls, "the failed conflict update is NEVER re-dispatched")
	env.fake.unlock()
}

// TestActionPlanEndToEndExcludeItem (Owner 排除单项): the approval
// excludes item 2; execution runs 1 and 3; the excluded item's action
// stays awaiting_approval — queryable through the frozen single-action
// GET, never dispatched by the plan.
func TestActionPlanEndToEndExcludeItem(t *testing.T) {
	env := newActionPlanE2E(t)
	p := env.formPlan(t, "Doc A", "Doc B", "Doc C")
	env.approveExcluding(t, p, []int{2})
	out := env.execute(t, p)
	require.Len(t, out, 3)
	require.Equal(t, "executed", out[0].Disposition)
	require.Equal(t, "excluded", out[1].Disposition)
	require.Equal(t, "executed", out[2].Disposition)
	// The excluded action stayed untouched.
	w := env.do(t, http.MethodGet, "/api/v1/apps/actions/"+out[1].ActionID, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"awaiting_approval"`)
	env.fake.lock()
	require.Len(t, env.fake.pages, 3, "parent + two created pages — the excluded item never dispatched")
	env.fake.unlock()
	// The plan GET projects the exclusion.
	w = env.do(t, http.MethodGet, "/api/v1/apps/action-plans/"+p.id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"excluded":[2]`)
}
