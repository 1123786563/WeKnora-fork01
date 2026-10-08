package handler

// End-to-end evidence for T20 (#50). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PublicationStore / ConfluenceBridge /
// ConfluencePublishService / DBConfluenceScopeSource / handlers and the
// REAL approval endpoint. The ONLY replaced piece is the Confluence wire
// endpoint: a local contract double (e2eConfluence) implementing the
// official Cloud v2 page contract (GET/POST/PUT /wiki/api/v2/pages) with
// HTTP Basic auth and the monotonic version.number gate. This is the
// highest stable Interface evidence available without provider
// credentials — it is NOT the real-provider acceptance, which stays
// CONFLUENCE_*-gated (Task 9) and honestly SKIPs in this environment.

import (
	"context"
	"database/sql"
	"encoding/base64"
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

func openConfluencePublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "confluence-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
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

// ---- the contract double for the Confluence wire (Cloud v2, /wiki
// context path; NOT the real-provider acceptance) ----

type e2eConfluence struct {
	mu          sync.Mutex
	username    string
	secret      string
	pages       map[string]*e2eCfPage
	nextID      int
	postCalls   int
	putCalls    int
	dropNextPut bool
}

type e2eCfPage struct {
	id, spaceID, title, storage string
	version                     int
}

func newE2EConfluence(username, secret string) *e2eConfluence {
	return &e2eConfluence{username: username, secret: secret, pages: map[string]*e2eCfPage{}}
}

func (e *e2eConfluence) addPage(id, spaceID, title string, version int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pages[id] = &e2eCfPage{id: id, spaceID: spaceID, title: title, version: version}
}

// bump applies an EXTERNAL collaborator's edit between approval and
// publish (the AC1 window).
func (e *e2eConfluence) bump(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p, ok := e.pages[id]; ok {
		p.version++
	}
}

func e2eCfJSON(p *e2eCfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-26T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func (e *e2eConfluence) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(e.username+":"+e.secret))
		return r.Header.Get("Authorization") == want
	}
	mux.HandleFunc("/wiki/api/v2/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"errors":[{"title":"unauthorized"}]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// The double decodes the OFFICIAL Cloud v2 contract — camelCase
		// spaceId/parentId keys only (R5-F5: this e2e copy was the last
		// snake_case masking double; an isomorphic decode hides a
		// wire-contract regression exactly as it did on the real provider).
		var req struct {
			SpaceID  string `json:"spaceId"`
			ParentID string `json:"parentId"`
			Title    string `json:"title"`
			Body     struct {
				Value string `json:"value"`
			} `json:"body"`
		}
		_ = json.Unmarshal(body, &req)
		e.mu.Lock()
		_, ok := e.pages[req.ParentID]
		e.mu.Unlock()
		if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
			writeJSON(w, 400, `{"errors":[{"title":"invalid create request"}]}`)
			return
		}
		e.mu.Lock()
		e.nextID++
		e.postCalls++
		np := &e2eCfPage{id: fmt.Sprintf("cf-%d", e.nextID), spaceID: req.SpaceID, title: req.Title, storage: req.Body.Value, version: 1}
		e.pages[np.id] = np
		e.mu.Unlock()
		writeJSON(w, 200, e2eCfJSON(np, false))
	})
	mux.HandleFunc("/wiki/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"errors":[{"title":"unauthorized"}]}`)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/wiki/api/v2/pages/")
		e.mu.Lock()
		p, ok := e.pages[id]
		e.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			e.mu.Lock()
			e.mu.Unlock()
			writeJSON(w, 200, e2eCfJSON(p, true))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Title string `json:"title"`
				Body  struct {
					Value string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			e.mu.Lock()
			e.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := e.dropNextPut
			if drop {
				e.dropNextPut = false
			}
			e.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, e2eCfJSON(p, false))
		default:
			writeJSON(w, 405, `{"errors":[{"title":"method not allowed"}]}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- the full-stack environment ----

type confluenceE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eConfluence
}

func newConfluencePublishE2E(t *testing.T) *confluenceE2EEnv {
	t.Helper()
	db := openConfluencePublishE2EDB(t)
	// Tenants / users / membership (column sets verified against the
	// sqlite migration track — same seeds as the #48 E2E).
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	// Task = Session (ADR-0004) + an admitted run for the artifact version
	// binding.
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	// The confirmed artifact version + its real bytes on a REAL local file
	// service (the production content-reader path minus tenant backend
	// routing).
	baseDir := t.TempDir()
	artifactText := "first para.\n\nsecond para."
	digest := strings.Repeat("b", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte(artifactText), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', ?)`, digest, objectKey, len(artifactText)).Error)
	// The Confluence double FIRST: its loopback URL seeds the installation's
	// reviewed base_url (production scope resolution reads it back).
	fake := newE2EConfluence("user-a@test.example", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Workspace", 7)
	srv := fake.server(t)
	// Confluence installation + reviewed app version (write scope +
	// approved parents) + user-a's personal connection + its credential
	// row (the reviewed JSON credential shape, Basic username+secret).
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version, config_json)
		VALUES ('inst-cf', 7, 'confluence', 'v1', 'active', 1, ?)`, fmt.Sprintf(`{"base_url":%q,"edition":"cloud"}`, srv.URL+"/wiki")).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('confluence', 'v1', '{"scopes":["write_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-cf', 'inst-cf', 'personal', 'user-a', 'mcp_oauth_token:confluence', 'active', 1)`).Error)
	// mcp_oauth_tokens.service_id is a FOREIGN KEY to mcp_services(id) —
	// the service row must exist first.
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('confluence', 7, 'confluence', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-cf', 7, 'user-a', 'web_user', 'user-a', 'confluence', '{"username":"user-a@test.example","secret":"secret_cf_token"}', 'bearer', '2099-01-01 00:00:00')`).Error)

	// The production composition: REAL DBConfluenceScopeSource (base_url +
	// schema above) and REAL credential token source; only the outbound
	// POLICY is the injected loopback-authorized variant — the documented
	// HTTPPolicy test hook (http_policy.go:63-66) that lets the public-
	// address dial gate accept 127.0.0.0/8 for the httptest double.
	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: "/wiki/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBConfluenceScopeSource(db)
	bridge := publish.NewConfluenceBridge(scopeSrc, &e2ePolicyProvider{pol: pol},
		publish.NewConfluenceCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))))
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &e2eLocalContent{svc: file.NewLocalFileService(baseDir, "")}
	svc := publish.NewConfluencePublishService(publishActions, store, pubs, repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)

	publishHandler := NewAppConfluencePublishHandler(db)
	publishHandler.SetConfluencePublishService(svc)
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
	// Mirror of RegisterAppConfluencePublishRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	g := v1.Group("/apps/confluence-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormConfluencePublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishConfluenceAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileConfluenceAction)
		g.GET("/actions/:id", publishHandler.GetConfluencePublication)
	}
	return &confluenceE2EEnv{engine: engine, db: db, fake: fake}
}

func (e *confluenceE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

func (e *confluenceE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *confluenceE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	// Route through the EXISTING approval endpoint: approval authority
	// stays on the frozen ApproveAction predicate.
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestConfluencePublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external baseline read through the PRODUCTION scope source) → approve
// on the EXISTING endpoint → publish → receipt with external id +
// version; the publication row lands 'published'.
func TestConfluencePublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newConfluencePublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "parent-1", plan["destination"])
	require.Equal(t, "7", plan["expected_external_version"], "AC1 baseline: the plan read the parent page's current version")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan["action_id"].(string)+"/publish", "")
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
	require.Equal(t, "1", out.Data.Publication.ExternalVersion, "the receipt must save the version the publish produced")
	require.Equal(t, "ver-1", out.Data.Publication.ArtifactVersionID)

	// The receipt is durable and queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/confluence-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published page exists remotely with the derived storage body.
	env.fake.mu.Lock()
	created := env.fake.pages[out.Data.Publication.ExternalID]
	env.fake.mu.Unlock()
	require.NotNil(t, created)
	require.Equal(t, "<p>first para.</p>\n<p>second para.</p>", created.storage, "two paragraphs derived from the artifact text")
}

// TestConfluencePublishEndToEndUpdateConflict: AC1 — an external edit
// between plan formation and publish is detected at execute time and the
// publish is refused with 409 and ZERO write requests; the update is only
// reachable through a page this connection published before.
func TestConfluencePublishEndToEndUpdateConflict(t *testing.T) {
	env := newConfluencePublishE2E(t)
	// First publish to mint a receipt-backed target page.
	plan1 := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
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
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	require.Equal(t, "update", plan2["mode"])
	require.Equal(t, "1", plan2["expected_external_version"])
	env.approve(t, plan2)

	// External collaborator edits between approval and publish: version 1 → 2.
	env.fake.mu.Lock()
	putsBefore := env.fake.putCalls
	env.fake.mu.Unlock()
	env.fake.bump(pageID)

	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_VERSION_CONFLICT")
	env.fake.mu.Lock()
	putsAfter := env.fake.putCalls
	env.fake.mu.Unlock()
	require.Equal(t, putsBefore, putsAfter, "AC1: conflict leaves ZERO page writes")
}

// TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst: AC2 — a lost
// write reply parks the action unknown; a second publish is structurally
// refused; reconcile reads the REMOTE first and settles the receipt, with
// no re-dispatch ever.
func TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newConfluencePublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
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

	// The update plan; the PUT's reply is lost after the effect applied.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	env.approve(t, plan2)
	env.fake.mu.Lock()
	env.fake.dropNextPut = true
	putsBefore := env.fake.putCalls
	env.fake.mu.Unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store (claim only from
	// authorized); the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.mu.Lock()
	require.Equal(t, putsBefore+1, env.fake.putCalls, "the dropped put counted once; NO re-dispatch happened")
	putsAfterUnknown := env.fake.putCalls
	env.fake.mu.Unlock()

	// Reconcile: provider query FIRST — the effect applied remotely, so
	// the query settles success and the receipt lands published.
	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.mu.Lock()
	require.Equal(t, putsAfterUnknown, env.fake.putCalls, "reconcile must not re-send")
	env.fake.mu.Unlock()
}
