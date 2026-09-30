package repository_test

// End-to-end delivery collaboration evidence (T23 #53). Everything runs
// through real HTTP handlers over a REAL fully-migrated sqlite database:
// the grants API, the delivery write/read surface, the A03 approve endpoint
// and the traceability read-back. GitHub is the only test double (an
// httptest server speaking the exact wire contract pinned by
// codedelivery/github_wire_test.go) — the provider is out of process and
// credential-gated in this environment (blocked-env); every authorization
// and attribution component under test is real. This is the AC3 evidence.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	deliveryToken   = "gho_testtoken"
	deliveryBaseSHA = "b000000000000000000000000000000000000000" // 40 hex chars
)

// githubStub speaks the minimal GitHub REST subset the delivery chain
// drives on the happy path (prepare: repo/branches/commits/trees/blobs;
// dispatch: ref create + PR + authenticated user). Response shapes mirror
// the emulator in codedelivery/github_wire_test.go.
type githubStub struct {
	srv *httptest.Server

	mu    sync.Mutex
	blobs map[string][]byte
	refs  map[string]string
	prs   []map[string]any
	prSeq int64
}

func newGitHubStub(t *testing.T) *githubStub {
	s := &githubStub{
		blobs: map[string][]byte{"blob-main": []byte("package main\n")},
		refs:  map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *githubStub) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *githubStub) serve(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer "+deliveryToken {
		s.writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "bad credentials"})
		return
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		s.writeJSON(w, http.StatusOK, map[string]any{"login": "octocat-remote"})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello":
		s.writeJSON(w, http.StatusOK, map[string]any{"default_branch": "main", "private": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/branches/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"protected": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/commits/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"tree": "tree-baseline"})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/trees/"):
		// The fixed baseline tree: one blob file, as the stub seeded it.
		s.writeJSON(w, http.StatusOK, map[string]any{"truncated": false, "tree": []map[string]any{
			{"path": "main.go", "sha": "blob-main", "type": "blob"},
		}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/blobs/"):
		sha := filepath.Base(path)
		content, ok := s.blobs[sha]
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such blob"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{
			"encoding": "base64", "content": base64.StdEncoding.EncodeToString(content),
		})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/ref/heads/"):
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/ref/heads/")
		sha, ok := s.refs[branch]
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such ref"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": sha}})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "blob-changed"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "tree-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "commit-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.refs[strings.TrimPrefix(body.Ref, "refs/heads/")] = body.SHA
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, map[string]any{"ref": body.Ref})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		s.mu.Lock()
		out := append([]map[string]any{}, s.prs...)
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, out)
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		s.mu.Lock()
		s.prSeq++
		pr := map[string]any{"number": s.prSeq, "html_url": fmt.Sprintf("https://github.com/octocat/hello/pull/%d", s.prSeq), "draft": true, "state": "open"}
		s.prs = append(s.prs, pr)
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, pr)
	default:
		s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected " + r.Method + " " + path})
	}
}

// deliveryCredentialSource adapts the real gorm rows onto the credential
// source + resolver the delivery chain needs (the same dual role the
// production MCPOAuthBindingStore plays; this adapter reads the SAME
// production connections table through bound parameters, so the e2e never
// stubs the authorization path. The token bytes are fixture data — the
// encrypted-token lifecycle belongs to the OAuth domain and is covered
// there).
type deliveryCredentialSource struct {
	db      *gorm.DB
	members map[string]bool
}

func newDeliveryCredentialSource(db *gorm.DB) *deliveryCredentialSource {
	return &deliveryCredentialSource{db: db, members: map[string]bool{"u1": true, "u2": true, "u3": true, "u4": true}}
}

// appConnAdapter 适配两侧 FindConnectionByID 返回类型差异。
type appConnAdapter struct{ base *deliveryCredentialSource }

func (a appConnAdapter) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	ident, err := a.base.FindConnectionByID(ctx, id)
	if err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: ident.ID, InstallationID: ident.InstallationID, Kind: ident.Kind,
		OwnerID: ident.OwnerID, State: ident.State, TenantID: ident.TenantID, AuthVersion: ident.AuthVersion,
	}, nil
}

func (s *deliveryCredentialSource) FindConnectionByID(ctx context.Context, id string) (codedelivery.ConnectionIdentity, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", 1, id).First(&row).Error; err != nil {
		return codedelivery.ConnectionIdentity{}, err
	}
	return codedelivery.ConnectionIdentity{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID,
		State:   row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *deliveryCredentialSource) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(deliveryToken), nil
}

func (s *deliveryCredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *deliveryCredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

// Resolve satisfies appconnectorsvc.CredentialResolver: the delivery chain
// resolves the token AFTER the permission chain has passed.
func (s *deliveryCredentialSource) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(deliveryToken), nil
}

// deliveryCollabEnv is the fully-real HTTP rig: real migrated db, real
// handlers (task grants + delivery + A03 approve), real services.
type deliveryCollabEnv struct {
	db     *gorm.DB
	engine *gin.Engine
	wsRoot string
}

func newDeliveryCollabEnv(t *testing.T) *deliveryCollabEnv {
	t.Helper()
	db := openTaskGrantDB(t) // real full migrations/sqlite track + tenant/user/session fixtures

	// The task's admitted run: r1 on session s1, owner u1.
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Tenant GitHub App installation + u1's personal connection + the
	// tenant's space connection (both active, same installation).
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", TenantID: 1, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-space", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "mcp:conn-space:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)

	// Local workspace + GitHub HTTP double + real delivery chain.
	github := newGitHubStub(t)
	wsRoot := t.TempDir()
	workspace, err := codedelivery.NewLocalWorkspaceSource(wsRoot)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := newDeliveryCredentialSource(db)
	guard := appconnectorsvc.NewSubjectGuard(appConnAdapter{base: connections})
	factory := codedelivery.NewGitHubClientFactory(http.DefaultClient, github.srv.URL)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guardAdapter{g: guard},
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStoreAdapter{s: actionStore}, Runs: runsAdapter{r: runs},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcherAdapter{d: dispatcher}, dispatcherAdapter{d: dispatcher})
	svc := codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: lifecycleAdapter{s: actions}, ActionRows: actionStoreAdapter{s: actionStore},
		Connections: connections, Creds: connections,
		GitHub: factory, Workspace: workspace, Runs: runsAdapter{r: runs},
		Providers:  installStoreAdapter{s: appconnectorrepo.NewInstallationStore(db)},
		Dispatcher: dispatcher,
	})

	// Handlers + routes (the manual mount mirrors
	// RegisterWorkbenchDeliveryRoutes / the task-grant mounts in
	// task_collaboration_http_test.go and the ApproveAction mount in
	// routes_app_connectors.go).
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	v1.POST("/workbench/tasks/:task_id/grants", session.NewWorkbenchTaskGrantsHandler(grantsSvc).Grant)
	deliveryHandler := session.NewWorkbenchDeliveryHandler(runs, runs, svc)
	v1.GET("/workbench/executions/:run_id/delivery", deliveryHandler.GetDelivery)
	v1.POST("/workbench/executions/:run_id/baseline", deliveryHandler.MaterializeBaseline)
	v1.POST("/workbench/executions/:run_id/delivery", deliveryHandler.PrepareDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/dispatch", deliveryHandler.DispatchDelivery)
	actionHandler := handler.NewAppActionHandler(db)
	actionHandler.SetActionService(actions)
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	return &deliveryCollabEnv{db: db, engine: r, wsRoot: wsRoot}
}

// do injects the authenticated identity exactly like the collaboration e2e
// does (request context keys the auth middleware writes in lockstep).
func (e *deliveryCollabEnv) do(t *testing.T, method, path, body, userID string, tenant uint64, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// prepareDelivery drives the owner's real baseline→edit→prepare loop and
// returns the persisted delivery id / action id / digest.
func (e *deliveryCollabEnv) prepareDelivery(t *testing.T) (deliveryID, actionID, digest string) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The developer's edit on top of the fixed baseline (the bytes the
	// stub's CreateBlob acks).
	target := filepath.Join(e.wsRoot, "octocat/hello")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "main.go"), []byte("package main\n// changed\n"), 0o644))

	w = e.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"fix: greeting","pr_title":"WeKnora task s1"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var prepared struct {
		Data struct {
			Delivery struct {
				ID        string `json:"id"`
				ActionID  string `json:"action_id"`
				Digest    string `json:"digest"`
				Initiator string `json:"initiator"`
				State     string `json:"state"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prepared))
	require.Equal(t, "u1", prepared.Data.Delivery.Initiator, "发起者归因在 prepare 读回即在场")
	require.Equal(t, "prepared", prepared.Data.Delivery.State)
	return prepared.Data.Delivery.ID, prepared.Data.Delivery.ActionID, prepared.Data.Delivery.Digest
}

// approveThroughHTTP records u1's approval over the REAL A03 endpoint
// (digest + expected_version fence read from the persisted action row).
func (e *deliveryCollabEnv) approveThroughHTTP(t *testing.T, actionID, digest string) {
	t.Helper()
	var fence int64
	require.NoError(t, e.db.Raw(`SELECT fence FROM app_actions WHERE id = ?`, actionID).Scan(&fence).Error)
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest, fence)
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+actionID+"/approve", body, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	deliveryID, actionID, digest := env.prepareDelivery(t)

	// AC1 negative half: the SAME owner cannot deliver through the SPACE
	// connection — a space connection never substitutes for the owner's
	// personal one on the delivery face (structural personal-only in
	// codedelivery.authorize; the rejection happens before any GitHub call).
	w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-space","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"fix: greeting","pr_title":"WeKnora task s1"}`,
		"u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "code_delivery_forbidden", "AC1: 空间连接不能替代个人连接")

	// Approve over the real A03 endpoint, then dispatch through the owner's
	// personal connection.
	env.approveThroughHTTP(t, actionID, digest)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch",
		"", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The traceability read-back carries the FULL triple: initiator,
	// approver and the actual remote identity (the stub's GET /user).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"initiator":"u1"`, "发起成员（CONTEXT.md 代码平台连接）")
	require.Contains(t, w.Body.String(), `"approver":"u1"`, "批准成员")
	require.Contains(t, w.Body.String(), `"remote_login":"octocat-remote"`, "实际远端身份（provider GET /user）")
	require.Contains(t, w.Body.String(), `"pr_url":"https://github.com/octocat/hello/pull/1"`)
	require.NotContains(t, w.Body.String(), deliveryToken, "归因面永不携带凭据")
}

func TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	deliveryID, _, _ := env.prepareDelivery(t) // the owner's delivery exists first
	require.NotEmpty(t, deliveryID)

	// The owner shares the task: u3 becomes a collaborator.
	w := env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u3","role":"collaborator"}`, "u1", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The collaborator CAN read the delivery (the #42 granted-read face).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// ...but the write face stays owner-only: u3 cannot baseline-materialize
	// nor prepare a delivery through u1's personal connection. The strict
	// owner predicate answers a uniform 404 — a shared task never delegates
	// the owner's connection (CONTEXT.md 任务协作者). NOTE: the WRITE path's
	// miss is resolveOwnedRun's bare AbortWithStatus(404) (workbench_read.go:193-195,
	// no JSON body) — assert the status code only; the `run_not_found` JSON
	// envelope exists solely on the read face (resolveReadable).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`"}`,
		"u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+deliveryBaseSHA+`","commit_message":"x","pr_title":"y"}`,
		"u3", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)

	// A member without any grant cannot even read the delivery.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "u4", 1, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeliveryCollaborationEndToEndCrossTenantIsolated(t *testing.T) {
	env := newDeliveryCollabEnv(t)
	env.prepareDelivery(t)

	// Tenant 2's principal probes tenant 1's run: one uniform 404 on the
	// read face — the tenant scope is bound from the authenticated context,
	// and no cross-tenant probe learns anything.
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/delivery", "", "outsider", 2, types.TenantRoleContributor)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "run_not_found")
}

func (a appConnAdapter) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return a.base.LoadCredential(ctx, c)
}
func (a appConnAdapter) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return a.base.MemberActive(ctx, tenantID, userID)
}
func (a appConnAdapter) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return a.base.TryAcquireRefreshLease(ctx, c, leaseID, until)
}

// guardAdapter 把 appconnectorsvc.A02Guard 适配为 codedelivery.A02Guard（subject 类型名不同、字段同构）。
type guardAdapter struct{ g appconnectorsvc.A02Guard }

func (a guardAdapter) Check(ctx context.Context, s codedelivery.ActionSubject, actionID string, authVersion int64) error {
	return a.g.Check(ctx, appconnector.OCSubject{TenantID: s.TenantID, ActorID: s.ActorID}, actionID, authVersion)
}

// actionStoreAdapter 把 appconnectorrepo.ActionStore 适配为 codedelivery.ActionStoreSource。
type actionStoreAdapter struct{ s *appconnectorrepo.ActionStore }

func (a actionStoreAdapter) FindAction(ctx context.Context, id string) (codedelivery.ActionRecord, error) {
	r, err := a.s.FindAction(ctx, id)
	if err != nil {
		return codedelivery.ActionRecord{}, err
	}
	return codedelivery.ActionRecord{
		ID: r.ID, TenantID: r.TenantID, ActorID: r.ActorID, ConnectionID: r.ConnectionID,
		AppVersion: r.AppVersion, Target: r.Target, Risk: r.Risk, ArgsDigest: r.ArgsDigest,
		State: r.State, Fence: r.Fence, ArgsSnapshot: r.ArgsSnapshot, AuthVersion: r.AuthVersion,
		DigestVersion: int(r.DigestVersion), ProviderResult: r.ProviderResult,
	}, nil
}

// runsAdapter 把 AgentRunStore 适配为 codedelivery.RunReader。
type runsAdapter struct{ r *repository.AgentRunStore }

func (a runsAdapter) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (codedelivery.RunIdentity, error) {
	run, err := a.r.GetOwnedRun(ctx, tenantID, ownerID, runID)
	if err != nil {
		return codedelivery.RunIdentity{}, err
	}
	return codedelivery.RunIdentity{SessionID: run.SessionID}, nil
}

// dispatcherAdapter 把 codedelivery.DeliveryDispatcher 适配为 appconnectorsvc.ActionDispatcher。
type dispatcherAdapter struct {
	d *codedelivery.DeliveryDispatcher
}

func (a dispatcherAdapter) Dispatch(ctx context.Context, s appconnectorsvc.ActionSnapshot, reservationID string) (appconnectorsvc.DispatchOutcome, error) {
	out, err := a.d.Dispatch(ctx, codedelivery.ActionSnapshot{
		ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target,
		AuthVersion: s.AuthVersion, Args: s.Args,
	}, reservationID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{Status: out.Status, ProviderResult: out.ProviderResult}, nil
}
func (a dispatcherAdapter) QueryProvider(ctx context.Context, s appconnectorsvc.ActionSnapshot, executionID string) (appconnectorsvc.DispatchOutcome, error) {
	out, err := a.d.QueryProvider(ctx, codedelivery.ActionSnapshot{
		ID: s.ID, TenantID: s.TenantID, ActorID: s.ActorID, ConnectionID: s.ConnectionID, Target: s.Target,
		AuthVersion: s.AuthVersion, Args: s.Args,
	}, executionID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{Status: out.Status, ProviderResult: out.ProviderResult}, nil
}

// lifecycleAdapter 把 appconnectorsvc.ActionService 适配为 codedelivery.ActionLifecycle。
type lifecycleAdapter struct {
	s *appconnectorsvc.ActionService
}

func (a lifecycleAdapter) Prepare(ctx context.Context, in codedelivery.ActionInput) (string, error) {
	return a.s.Prepare(ctx, appconnector.Action{
		TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Target: in.Target, Risk: in.Risk, Args: in.Args, AuthVersion: in.AuthVersion,
	})
}
func (a lifecycleAdapter) Execute(ctx context.Context, id string) error { return a.s.Execute(ctx, id) }
func (a lifecycleAdapter) ResolveUnknown(ctx context.Context, id string) error {
	return a.s.ResolveUnknown(ctx, id)
}

// installStoreAdapter 把 appconnectorrepo.InstallationStore 适配为 codedelivery.ProviderSource。
type installStoreAdapter struct {
	s *appconnectorrepo.InstallationStore
}

func (a installStoreAdapter) GetInstallationByID(ctx context.Context, tenantID uint64, id string) (codedelivery.ProviderInstallation, error) {
	inst, err := a.s.GetInstallationByID(ctx, tenantID, id)
	if err != nil {
		return codedelivery.ProviderInstallation{}, err
	}
	return codedelivery.ProviderInstallation{AppID: inst.AppID}, nil
}
