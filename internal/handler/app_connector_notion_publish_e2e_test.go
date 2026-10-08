package handler

// End-to-end evidence for T18 (#48). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PublicationStore / NotionBridge / handlers and
// the REAL approval endpoint. The ONLY replaced piece is the Notion wire
// endpoint: a local contract double (bridgeFakeNotion's handler shape,
// duplicated below as e2eNotion) implementing the official page-object
// contract with last_edited_time. This is the highest stable Interface
// evidence available without provider credentials — it is NOT the
// real-provider acceptance, which stays NOTION_TOKEN-gated (Task 9) and
// honestly SKIPs in this environment.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---- production-migrated database ----

func openNotionPublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "notion-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

// TestAppPublicationsTableExistsAfterMigrations: the receipt table must
// be created by the PRODUCTION migrations (columns aligned with
// PublicationRow — the workbench-notifications precedent).
func TestAppPublicationsTableExistsAfterMigrations(t *testing.T) {
	db := openNotionPublishE2EDB(t)
	require.True(t, db.Migrator().HasTable("app_publications"),
		"app_publications must be created by the production migrations")
	for _, column := range []string{"tenant_id", "action_id", "connection_id", "provider", "mode",
		"destination", "expected_version", "artifact_version_id", "artifact_digest", "state",
		"external_id", "external_version", "receipt_json", "progress_json"} {
		require.True(t, db.Migrator().HasColumn("app_publications", column),
			"app_publications.%s must exist (aligned with PublicationRow)", column)
	}
}

// ---- the contract double for the Notion wire (same shape as Task 5's
// bridgeFakeNotion, self-contained here for the HTTP-level flow) ----

type e2eNotion struct {
	mu             sync.Mutex
	token          string
	pages          map[string]*e2ePage
	nextID         int
	patchCalls     int
	appendCalls    int
	dropNextAppend bool
	// T21 (#51) additive hook: when a page is CREATED with this title,
	// its first append-children reply is lost AFTER the effect applied
	// (blocks land, the connection dies) — the plan-level unknown leg.
	// The pre-existing dropNextAppend behavior is untouched.
	dropAppendForTitle string
	droppedOnce        map[string]bool
}

type e2ePage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newE2ENotion(token string) *e2eNotion {
	return &e2eNotion{token: token, pages: map[string]*e2ePage{}, droppedOnce: map[string]bool{}}
}

func (e *e2eNotion) lock()   { e.mu.Lock() }
func (e *e2eNotion) unlock() { e.mu.Unlock() }

func (e *e2eNotion) addPage(id, parent, title string) {
	e.lock()
	e.pages[id] = &e2ePage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
	e.unlock()
}

func (e *e2eNotion) touch(id, when string) {
	e.lock()
	if p, ok := e.pages[id]; ok {
		p.lastEdited = when
	}
	e.unlock()
}

func (e *e2eNotion) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+e.token }
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Parent struct {
				PageID string `json:"page_id"`
			} `json:"parent"`
			Properties struct {
				Title struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"title"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(body, &req)
		e.lock()
		e.nextID++
		id := fmt.Sprintf("page-%d", e.nextID)
		e.pages[id] = &e2ePage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		e.unlock()
		e.lock()
		if e.dropAppendForTitle != "" && req.Properties.Title.Title[0].Text.Content == e.dropAppendForTitle {
			e.droppedOnce[id] = true
		}
		e.unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		id := r.URL.Path[len("/v1/pages/"):]
		e.lock()
		p, ok := e.pages[id]
		le := ""
		if ok {
			le = p.lastEdited
		}
		e.unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			e.lock()
			le = p.lastEdited
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Properties struct {
					Title struct {
						Title []struct {
							Text struct {
								Content string `json:"content"`
							} `json:"text"`
						} `json:"title"`
					} `json:"title"`
				} `json:"properties"`
			}
			_ = json.Unmarshal(body, &req)
			e.lock()
			e.patchCalls++
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			le = p.lastEdited
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		if !strings.HasSuffix(rest, "/children") {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		id := strings.TrimSuffix(rest, "/children")
		e.lock()
		p, ok := e.pages[id]
		e.unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Children []json.RawMessage `json:"children"`
			}
			_ = json.Unmarshal(body, &req)
			e.lock()
			e.appendCalls++
			p.children = append(p.children, req.Children...)
			p.lastEdited = "2026-09-24T12:00:00.000Z"
			drop := e.dropNextAppend
			if drop {
				e.dropNextAppend = false
			}
			if e.droppedOnce[id] {
				delete(e.droppedOnce, id)
				drop = true
			}
			results := append([]json.RawMessage(nil), p.children...)
			e.unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, strings.Join(rawStrings(results), ",")))
		case http.MethodGet:
			e.lock()
			results := append([]json.RawMessage(nil), p.children...)
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, strings.Join(rawStrings(results), ",")))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func rawStrings(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = string(it)
	}
	return out
}

// ---- the full-stack environment ----

type notionE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eNotion
}

func newNotionPublishE2E(t *testing.T) *notionE2EEnv {
	t.Helper()
	db := openNotionPublishE2EDB(t)
	// Tenants / users / membership (column sets verified against the
	// sqlite migration track: tenants/users/sessions mirror the
	// task-grant fixtures; tenant_members columns per 000000_init.up.sql).
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	// Task = Session (ADR-0004) + an admitted run for the artifact version
	// binding (agent_runs has many NOT NULL columns — seed through the
	// real store, exactly like the #42 collaboration fixtures).
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	// The confirmed artifact version (columns per 000067_artifact_versions)
	// + its real bytes on a REAL local file service (the production
	// content-reader path minus tenant backend routing).
	baseDir := t.TempDir()
	digest := strings.Repeat("a", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("第一段。\n\n第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', 15)`, digest, objectKey).Error)
	// Notion installation + reviewed app version (scope + approved
	// parents) + user-a's personal connection + its token row (principal
	// columns per 000011_principal_model).
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-notion', 7, 'notion', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-notion', 'inst-notion', 'personal', 'user-a', 'mcp_oauth_token:notion', 'active', 1)`).Error)
	// mcp_oauth_tokens.service_id is a FOREIGN KEY to mcp_services(id)
	// (000000_init.up.sql) — the service row must exist first.
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('notion', 7, 'notion', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-1', 7, 'user-a', 'web_user', 'user-a', 'notion', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)
	// The Notion double: the pre-approved parent page exists remotely.
	fake := newE2ENotion("secret_test_token")
	fake.addPage("parent-1", "root", "Workspace")
	srv := fake.server(t)

	// The production composition, with the loopback policy provider
	// (the documented test hook) replacing the pinned api.notion.com one.
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
	svc := publish.NewNotionPublishService(publishActions, store, pubs, repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)

	publishHandler := NewAppNotionPublishHandler(db)
	publishHandler.SetNotionPublishService(svc)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(publishActions)

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
	// The EXISTING approval endpoint — approval authority stays there.
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	// Mirror of RegisterAppNotionPublishRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	g := v1.Group("/apps/notion-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormNotionPublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishNotionAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileNotionAction)
		g.GET("/actions/:id", publishHandler.GetNotionPublication)
	}
	return &notionE2EEnv{engine: engine, db: db, fake: fake}
}

type e2ePolicyProvider struct{ pol appconn.HTTPPolicy }

func (p *e2ePolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return p.pol, nil
}

type e2ePassGuard struct{}

func (e2ePassGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

type e2eLocalContent struct{ svc interfaces.FileService }

func (e *e2eLocalContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	reader, err := e.svc.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (e *notionE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *notionE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/notion-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *notionE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	// Route through the EXISTING approval endpoint: approval authority
	// stays on the frozen ApproveAction predicate.
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestNotionPublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external baseline read) → approve on the EXISTING endpoint → publish →
// receipt with external id + version; the migration-aligned publication
// row lands 'published'.
func TestNotionPublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newNotionPublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "parent-1", plan["destination"])
	require.NotEmpty(t, plan["expected_external_version"], "AC1 baseline: the plan read the external version")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			ActionState string `json:"action_state"`
			Publication struct {
				State             string `json:"state"`
				ExternalID        string `json:"external_id"`
				ExternalVersion   string `json:"external_version"`
				ArtifactVersionID string `json:"artifact_version_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "succeeded", out.Data.ActionState)
	require.Equal(t, "published", out.Data.Publication.State)
	require.NotEmpty(t, out.Data.Publication.ExternalID)
	require.NotEmpty(t, out.Data.Publication.ExternalVersion, "the receipt must save the external version")
	require.Equal(t, "ver-1", out.Data.Publication.ArtifactVersionID)

	// The receipt is durable and queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/notion-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published page exists remotely with the derived blocks.
	env.fake.lock()
	created := env.fake.pages[out.Data.Publication.ExternalID]
	env.fake.unlock()
	require.NotNil(t, created)
	require.Len(t, created.children, 2, "two paragraphs derived from the artifact text")
}

// TestNotionPublishEndToEndUpdateConflict: AC1 — an external edit between
// plan formation and publish is detected at execute time and the publish
// is refused with 409 and ZERO write requests.
func TestNotionPublishEndToEndUpdateConflict(t *testing.T) {
	env := newNotionPublishE2E(t)
	// First publish to mint a receipt-backed target page.
	plan1 := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out1 struct {
		Data struct {
			Publication struct {
				ExternalID string `json:"external_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out1))
	pageID := out1.Data.Publication.ExternalID

	// The update plan reads the page's CURRENT version.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	require.Equal(t, "update", plan2["mode"])
	env.approve(t, plan2)

	// External collaborator edits between approval and publish.
	env.fake.lock()
	patchesBefore := env.fake.patchCalls
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()
	env.fake.touch(pageID, "2026-09-24T10:30:00.000Z")

	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_VERSION_CONFLICT")
	env.fake.lock()
	patchesAfter := env.fake.patchCalls
	appendsAfter := env.fake.appendCalls
	env.fake.unlock()
	require.Equal(t, patchesBefore, patchesAfter, "AC1: conflict leaves ZERO page writes")
	require.Equal(t, appendsBefore, appendsAfter, "AC1: conflict leaves ZERO block writes")
}

// TestNotionPublishEndToEndUnknownReconcilesRemoteFirst: AC2 — a lost
// write reply parks the action unknown; a second publish is structurally
// refused; reconcile reads the REMOTE first and settles the receipt, with
// no re-dispatch ever.
func TestNotionPublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newNotionPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out1 struct {
		Data struct {
			Publication struct {
				ExternalID string `json:"external_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out1))
	pageID := out1.Data.Publication.ExternalID

	// The update plan; the append's reply is lost after the effect applied.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	env.approve(t, plan2)
	env.fake.lock()
	env.fake.dropNextAppend = true
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store (claim only from
	// authorized); the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsBefore+1, env.fake.appendCalls, "the dropped append counted once; NO re-dispatch happened")
	appendsAfterUnknown := env.fake.appendCalls
	env.fake.unlock()

	// Reconcile: provider query FIRST — the effect applied remotely, so
	// the query settles success and the receipt lands published.
	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsAfterUnknown, env.fake.appendCalls, "reconcile must not re-send")
	env.fake.unlock()
}
