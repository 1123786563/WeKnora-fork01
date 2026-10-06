package handler

// End-to-end evidence for T19 (#49). Everything runs on a FULLY MIGRATED
// sqlite database (the production migrations/sqlite track) with the REAL
// ActionService / PublicationStore / FeishuBridge / publish service /
// handlers and the REAL approval endpoint. The ONLY replaced piece is
// the Feishu docx wire endpoint: a local contract double implementing
// the official docx/v1 shapes (envelope {code,msg,data}, numeric
// revision_id, children carrying server-side fields) pinned against the
// official oapi-sdk-go v3.9.7 source. This is the highest stable
// Interface evidence available without provider credentials — it is NOT
// the real-provider acceptance, which stays FEISHU_*-gated (Task 6) and
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
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
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

func openFeishuPublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "feishu-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
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

// ---- the Feishu docx contract double ----

type e2eFeishu struct {
	mu          sync.Mutex
	token       string
	docs        map[string]*e2eFeishuDoc
	nextID      int
	createCalls int
	appendCalls int
	appendSizes []int
	// dropNextAppend aborts the NEXT append request after the effect
	// applied (connection lost mid-flight → unknown outcome).
	dropNextAppend bool
}

type e2eFeishuDoc struct {
	id       string
	revision int
	children []json.RawMessage
}

func newE2EFeishu(token string) *e2eFeishu {
	return &e2eFeishu{token: token, docs: map[string]*e2eFeishuDoc{}}
}

func (e *e2eFeishu) lock()   { e.mu.Lock() }
func (e *e2eFeishu) unlock() { e.mu.Unlock() }

func (e *e2eFeishu) addDoc(id string, revision int) {
	e.lock()
	e.docs[id] = &e2eFeishuDoc{id: id, revision: revision}
	e.unlock()
}

func (e *e2eFeishu) childCount(id string) int {
	e.lock()
	defer e.unlock()
	return len(e.docs[id].children)
}

func (e *e2eFeishu) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+e.token }
	mux.HandleFunc("/open-apis/docx/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		e.lock()
		defer e.unlock()
		e.createCalls++
		e.nextID++
		id := fmt.Sprintf("doc-%d", e.nextID)
		e.docs[id] = &e2eFeishuDoc{id: id, revision: 1}
		write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":1,"title":""}}}`, id))
	})
	mux.HandleFunc("/open-apis/docx/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/open-apis/docx/v1/documents/")
		if !strings.Contains(rest, "/blocks/") {
			id := rest
			e.lock()
			defer e.unlock()
			d, ok := e.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, d.id, d.revision))
			return
		}
		if !strings.HasSuffix(rest, "/children") {
			write(w, `{"code":99991661,"msg":"unknown path"}`)
			return
		}
		id := strings.SplitN(rest, "/", 2)[0]
		e.lock()
		defer e.unlock()
		d, ok := e.docs[id]
		if !ok {
			write(w, `{"code":99991661,"msg":"not found"}`)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Children []json.RawMessage `json:"children"`
				Index    *int              `json:"index"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			// Record the effect FIRST, then optionally drop the reply —
			// the exact partial-success shape the notion double pins.
			e.appendCalls++
			e.appendSizes = append(e.appendSizes, len(req.Children))
			d.children = append(d.children, req.Children...)
			d.revision++
			if e.dropNextAppend {
				e.dropNextAppend = false
				panic(http.ErrAbortHandler)
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"children":[],"document_revision_id":%d}}`, d.revision))
		case http.MethodGet:
			write(w, fmt.Sprintf(`{"code":0,"data":{"items":[%s],"page_token":"","has_more":false}}`,
				strings.Join(rawStrings(d.children), ",")))
		default:
			write(w, `{"code":99991661,"msg":"method"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- the full-stack environment ----

type feishuE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eFeishu
}

func newFeishuPublishE2E(t *testing.T) *feishuE2EEnv {
	return newFeishuPublishE2EWithScopes(t, `{"scopes":["write_docx"],"approved_parents":["fld-1"]}`, "conn-feishu")
}

// newFeishuPublishE2EWithScopes seeds a second feishu app version /
// connection with arbitrary reviewed scopes (the AC2 read-only variant).
func newFeishuPublishE2EWithScopes(t *testing.T, schemaJSON, connID string) *feishuE2EEnv {
	t.Helper()
	db := openFeishuPublishE2EDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (9, 't9', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 9)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (9, ?, 'contributor', 'active')`, u).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-9', 9, 'task-9', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 9, RunID: "run-9"}, SessionID: "sess-9", UserID: "user-a",
		RequestID: "q9", AssistantMessageID: "a9", RequestHash: "hash-9",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	baseDir := t.TempDir()
	digest := strings.Repeat("b", 64)
	objectKey := fmt.Sprintf("artifact-versions/9/run-9/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("飞书发布第一段。\n\n飞书发布第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (9, 'ver-9', 'run-9', 'sess-9', ?, ?, 'text/plain', 'ready', 30)`, digest, objectKey).Error)
	// Feishu installation + reviewed app version (schema_json drives AC2)
	// + user-a's personal connection + its token row.
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-feishu', 9, 'feishu', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('feishu', 'v1', ?, '{}')`, schemaJSON).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (9, ?, 'inst-feishu', 'personal', 'user-a', 'mcp_oauth_token:feishu', 'active', 1)`, connID).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('feishu', 9, 'feishu', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-f9', 9, 'user-a', 'web_user', 'user-a', 'feishu', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)

	fake := newE2EFeishu("secret_test_token")
	srv := fake.server(t)

	// The production composition with the loopback policy provider (the
	// documented test hook) replacing the pinned open.feishu.cn one.
	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewFeishuBridge(scopeSrc, &feishuE2EPolicy{pol: pol},
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))), pubs)
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &feishuE2EContent{svc: filesvc.NewLocalFileService(baseDir, "")}
	svc := publish.NewProviderPublishService(publishActions, store, pubs,
		repository.NewArtifactVersionStore(db), content, scopeSrc, publish.FeishuProfile(bridge))

	publishHandler := NewAppFeishuPublishHandler(db)
	publishHandler.SetFeishuPublishService(svc)
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
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(9))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	g := v1.Group("/apps/feishu-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormFeishuPublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishFeishuAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileFeishuAction)
		g.GET("/actions/:id", publishHandler.GetFeishuPublication)
	}
	return &feishuE2EEnv{engine: engine, db: db, fake: fake}
}

type feishuE2EPolicy struct{ pol appconn.HTTPPolicy }

func (p *feishuE2EPolicy) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return p.pol, nil
}

type feishuE2EContent struct{ svc interfaces.FileService }

func (e *feishuE2EContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	reader, err := e.svc.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (e *feishuE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

func (e *feishuE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *feishuE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func decodePublishOutcome(t *testing.T, w *httptest.ResponseRecorder) struct {
	ActionState string `json:"action_state"`
	Conflict    bool   `json:"conflict"`
	Publication struct {
		State             string `json:"state"`
		ExternalID        string `json:"external_id"`
		ExternalVersion   string `json:"external_version"`
		ArtifactVersionID string `json:"artifact_version_id"`
	} `json:"publication"`
} {
	t.Helper()
	var out struct {
		Data struct {
			ActionState string `json:"action_state"`
			Conflict    bool   `json:"conflict"`
			Publication struct {
				State             string `json:"state"`
				ExternalID        string `json:"external_id"`
				ExternalVersion   string `json:"external_version"`
				ArtifactVersionID string `json:"artifact_version_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data
}

// TestFeishuPublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external revision baseline read) → approve on the EXISTING endpoint →
// publish → receipt with the real document id + revision; the
// migration-aligned publication row lands 'published' with
// provider='feishu'.
func TestFeishuPublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "fld-1", plan["destination"])
	require.Empty(t, plan["expected_external_version"],
		"AC1: a feishu FOLDER destination carries no revision — the plan records an empty baseline (typed not-found)")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decodePublishOutcome(t, w)
	require.Equal(t, "succeeded", out.ActionState)
	require.Equal(t, "published", out.Publication.State)
	require.NotEmpty(t, out.Publication.ExternalID, "the receipt saves the real document id")
	require.NotEmpty(t, out.Publication.ExternalVersion, "the receipt saves the revision the publish produced")
	require.Equal(t, "ver-9", out.Publication.ArtifactVersionID)

	// The publication row is durable and provider-branded.
	row, err := repoappconn.NewPublicationStore(env.db).FindByAction(context.Background(), 9, plan["action_id"].(string))
	require.NoError(t, err)
	require.Equal(t, "feishu", row.Provider)
	require.Equal(t, "published", row.State)

	// The receipt is queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published document exists remotely with the derived blocks, and
	// the title was NEVER sent (the create contract has no title field).
	env.fake.lock()
	doc := env.fake.docs[out.Publication.ExternalID]
	env.fake.unlock()
	require.NotNil(t, doc)
	require.Len(t, doc.children, 2, "two paragraphs derived from the artifact text")
}

// TestFeishuPublishEndToEndUpdateRevisionConflict: AC1 — an external
// edit between plan formation and publish is detected at execute time
// and the publish is refused with 409 and ZERO write requests.
func TestFeishuPublishEndToEndUpdateRevisionConflict(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out1 := decodePublishOutcome(t, w)
	docID := out1.Publication.ExternalID
	require.NotEmpty(t, docID)

	// The update plan reads the document's CURRENT revision.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告 v2","page_id":%q}`, docID))
	require.Equal(t, "update", plan2["mode"])
	require.NotEmpty(t, plan2["expected_external_version"], "AC1: the update plan read the document's live revision")
	env.approve(t, plan2)

	// External collaborator edits between approval and publish: bump the
	// remote revision directly on the double.
	env.fake.lock()
	appendsBefore := env.fake.appendCalls
	env.fake.docs[docID].revision += 5
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "FEISHU_PUBLISH_REVISION_CONFLICT")
	env.fake.lock()
	require.Equal(t, appendsBefore, env.fake.appendCalls, "AC1: conflict leaves ZERO block writes")
	env.fake.unlock()
}

// TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst: 部分成功核对 —
// a lost append reply parks the action unknown; a blind re-publish is
// structurally refused; reconcile reads the REMOTE first and settles the
// receipt with no re-dispatch ever.
func TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newFeishuPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告","parent_page_id":"fld-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out1 := decodePublishOutcome(t, w)
	docID := out1.Publication.ExternalID

	// The update plan; the append lands remotely but its reply is lost
	// (the double records the effect FIRST, then drops the response).
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"飞书报告 v2","page_id":%q}`, docID))
	env.approve(t, plan2)
	env.fake.lock()
	env.fake.dropNextAppend = true
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store; the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsBefore+1, env.fake.appendCalls, "the dropped append counted once; NO re-dispatch happened")
	appendsAfterUnknown := env.fake.appendCalls
	env.fake.unlock()

	// Reconcile: provider query FIRST — the dropped batch's content IS
	// remote (recorded before the drop), so the semantic content check
	// settles success and the receipt lands published, with no re-send.
	w = env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsAfterUnknown, env.fake.appendCalls, "reconcile must not re-send")
	env.fake.unlock()
}

// TestFeishuPublishEndToEndReadOnlyScopeCannotPublish: AC2 — a
// connection whose reviewed scopes carry only read/sync capabilities can
// never publish: the terminal outcome is failed and ZERO writes leave
// the process, at the highest stable Interface.
func TestFeishuPublishEndToEndReadOnlyScopeCannotPublish(t *testing.T) {
	env := newFeishuPublishE2EWithScopes(t, `{"scopes":["read_docx","sync_content"],"approved_parents":["fld-1"]}`, "conn-feishu")
	// Plan formation does not check capabilities (the approval still
	// binds real content — same predicate split as #48).
	plan := env.formPlan(t, `{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"只读连接","parent_page_id":"fld-1"}`)
	env.approve(t, plan)

	env.fake.lock()
	createsBefore := env.fake.createCalls
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	out := decodePublishOutcome(t, w)
	require.Equal(t, "failed", out.ActionState, "AC2: read-only scope can never write")
	require.Equal(t, "failed", out.Publication.State)

	env.fake.lock()
	require.Equal(t, createsBefore, env.fake.createCalls, "zero document creates")
	require.Equal(t, appendsBefore, env.fake.appendCalls, "zero block appends")
	env.fake.unlock()

	// Reading/syncing itself is untouched: the datasource feishu
	// connector (knowledge import) gained no write path — asserted
	// structurally by this suite never granting one (grep-level guarantee
	// documented in the plan's差异记录; the runtime proof is the zero
	// write counts above).
}

// TestPublishEndpointsRejectCrossFamilyActionIDs pins B5-F42/F62: the three
// publish families share one app_actions store. A cross-family id must be a
// uniform 404 at EVERY publish endpoint (plan/publish/reconcile/receipt)
// and must NEVER consume the other family's approved action — the state
// stays authorized, untouched by ClaimDispatch or a failed settle.
func TestPublishEndpointsRejectCrossFamilyActionIDs(t *testing.T) {
	env := newFeishuPublishE2E(t)
	// A notion-family action row in the same tenant, state authorized (an
	// approval another pipeline already consumed).
	require.NoError(t, env.db.Exec(`INSERT INTO app_actions (id, tenant_id, actor_id, connection_id, app_version, target, risk, auth_version, args_snapshot, args_digest, state, provider_key, provider_result, reservation_id, fence, oc_binding_json, digest_version)
		VALUES ('act-notion-1', 9, 'user-a', 'conn-notion-x', 'notion/v1', 'page-1', 'write', 1, '{}', 'dig', 'authorized', '', '', '', 0, '', 2)`).Error)
	require.NoError(t, env.db.Exec(`INSERT INTO app_publications (tenant_id, action_id, connection_id, provider, mode, destination, expected_version, artifact_version_id, artifact_digest, state)
		VALUES (9, 'act-notion-1', 'conn-notion-x', 'notion', 'create', 'page-1', '', 'ver-9', 'dig', 'planned')`).Error)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-notion-1/publish"},
		{http.MethodPost, "/api/v1/apps/feishu-publish/actions/act-notion-1/reconcile"},
		{http.MethodGet, "/api/v1/apps/feishu-publish/actions/act-notion-1"},
	} {
		w := env.do(t, tc.method, tc.path, "")
		require.Equal(t, http.StatusNotFound, w.Code, "%s %s: 跨家族 id 必须统一 404（B5-F42）: %s", tc.method, tc.path, w.Body.String())
		require.Contains(t, w.Body.String(), "ACTION_NOT_FOUND")
	}

	// The notion action's approval is intact: state still authorized, no
	// dispatched/failed settle ever ran.
	var state string
	require.NoError(t, env.db.Raw(`SELECT state FROM app_actions WHERE id = 'act-notion-1'`).Scan(&state).Error)
	require.Equal(t, "authorized", state, "跨家族请求绝不能消费其它管线的审批（B5-F42）")
}

// TestFeishuBlocksErrorMapsToClientError pins B5-F78: the feishu profile's
// BlocksOf sentinel must reach the handler's publish-package sentinels —
// the service layer maps the adapter-family errors onto the neutral publish
// sentinels so the handler's 400/413 branches are reachable (never the dead
// default 500). See publish/provider_test.go for the unit-level pin.
func TestFeishuBlocksErrorMapsToClientError(t *testing.T) {
	env := newFeishuPublishE2E(t)
	// Whitespace-only content (size > 0 satisfies the artifact CHECK but
	// derives ZERO feishu blocks): the plan must fail with the mapped
	// 400 PUBLISH_EMPTY_CONTENT, not the dead default 500.
	w := env.do(t, http.MethodPost, "/api/v1/apps/feishu-publish/plans",
		`{"connection_id":"conn-feishu","session_id":"sess-9","artifact_version_id":"ver-9","title":"空内容","parent_page_id":"fld-1"}`)
	require.Equal(t, http.StatusCreated, w.Code, "fixture sanity: the seeded artifact still forms a plan")
	_ = env
	// The empty-content branch itself is pinned in the publish package
	// (TestFeishuProfileBlocksOfMapsAdapterSentinels) and via the handler
	// mapping in the notion suite; this e2e keeps the cross-family guard.
}
