package repository_test

// T25 (#55) recovery evidence over the real HTTP handler surface with an
// explicit test identity context (production route guard/read-gate wiring is
// pinned separately in internal/router/workbench_delivery_registration_test.go), a REAL
// fully-migrated sqlite database, the real A03 approval chain, and a
// GitHub stub speaking the wire contract pinned by
// codedelivery/github_wire_test.go. The stub is the only double; the
// provider itself is credential-gated (blocked-env, Task 7 covers the
// real-provider loop). This file mirrors delivery_collaboration_http_test.go.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/middleware"
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

// The credential probe: a FAKE token (never a usable credential). Every
// assertion below proves this byte string exists ONLY in the outbound
// Authorization header — nowhere in responses, workspace files or the
// delivery ledger (spec: "does not ... expose remote credentials to Shell").
const t25ProbeToken = "ghp_T25PROBE_7f3a9c1e"
const t25BaseSHA = "b000000000000000000000000000000000000000"

// recoveryGitHubStub: the wire contract of githubStub (T23) plus call
// counters, deterministic PR failure injection (422 → partial `pushed`),
// transport-failure injection (connection abort → `unknown`), and
// head-filtered PR listing for the unknown resolver.
type recoveryGitHubStub struct {
	srv *httptest.Server

	mu              sync.Mutex
	blobs           map[string][]byte
	refs            map[string]string
	prs             []map[string]any
	prSeq           int64
	calls           map[string]int
	failPRCreations int  // >0: every PR creation answers 422 (definite failure)
	failPRTransport bool // persist a PR, then abort before its response reaches the client
}

func newRecoveryGitHubStub(t *testing.T) *recoveryGitHubStub {
	s := &recoveryGitHubStub{
		blobs: map[string][]byte{"blob-main": []byte("package main\n")},
		refs:  map[string]string{},
		calls: map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *recoveryGitHubStub) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *recoveryGitHubStub) count(key string) {
	s.mu.Lock()
	s.calls[key]++
	s.mu.Unlock()
}

func (s *recoveryGitHubStub) snapshotCalls() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int{}
	for k, v := range s.calls {
		out[k] = v
	}
	return out
}

func (s *recoveryGitHubStub) snapshotWrites() map[string]int {
	calls := s.snapshotCalls()
	writes := map[string]int{}
	for key, count := range calls {
		if strings.HasPrefix(key, "POST ") || strings.HasPrefix(key, "PATCH ") {
			writes[key] = count
		}
	}
	return writes
}

func (s *recoveryGitHubStub) serve(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer "+t25ProbeToken {
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
		s.mu.Lock()
		sha, ok := s.refs[branch]
		s.mu.Unlock()
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such ref"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": sha}})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		s.count("POST /git/blobs")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "blob-changed"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		s.count("POST /git/trees")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "tree-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		s.count("POST /git/commits")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "commit-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		s.count("POST /git/refs")
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.refs[strings.TrimPrefix(body.Ref, "refs/heads/")] = body.SHA
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, map[string]any{"ref": body.Ref})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/repos/octocat/hello/git/refs/"):
		s.count("PATCH /git/refs")
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": "commit-delivery"}})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		head := r.URL.Query().Get("head")
		s.mu.Lock()
		out := []map[string]any{}
		for _, pr := range s.prs {
			if head == "" || pr["head_ref"] == strings.TrimPrefix(head, "octocat:") {
				out = append(out, pr)
			}
		}
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, out)
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		s.count("POST /pulls")
		s.mu.Lock()
		failCreations := s.failPRCreations
		failTransport := s.failPRTransport
		s.mu.Unlock()
		if failCreations > 0 {
			s.mu.Lock()
			s.failPRCreations--
			s.mu.Unlock()
			s.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "Validation Failed"})
			return
		}
		s.mu.Lock()
		s.prSeq++
		pr := map[string]any{"number": s.prSeq, "html_url": fmt.Sprintf("https://github.com/octocat/hello/pull/%d", s.prSeq), "draft": true, "state": "open", "head_ref": "weknora/task/s1"}
		s.prs = append(s.prs, pr)
		s.mu.Unlock()
		if failTransport {
			// GitHub committed the PR, but the response was lost. This is
			// genuinely unknown to the caller; resolution must find this PR.
			panic(http.ErrAbortHandler)
		}
		s.writeJSON(w, http.StatusCreated, pr)
	default:
		s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected " + r.Method + " " + path})
	}
}

// t25CredentialSource: same dual role as the T23 rig (connection reader +
// credential resolver), but LoadCredential/Resolve hand out the PROBE token
// so the containment assertions observe the real dispatch path.
type t25CredentialSource struct {
	db      *gorm.DB
	members map[string]bool
}

func (s *t25CredentialSource) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", 1, id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *t25CredentialSource) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(t25ProbeToken), nil
}

func (s *t25CredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *t25CredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (s *t25CredentialSource) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(t25ProbeToken), nil
}

// recoveryEnv mirrors newDeliveryCollabEnv WITH the Providers wiring (the
// regression Task 1 fixed) and the counting/injecting stub.
type recoveryEnv struct {
	db        *gorm.DB
	engine    *gin.Engine
	wsRoot    string
	github    *recoveryGitHubStub
	responses []recoveryHTTPResponse
	logs      bytes.Buffer
}

type recoveryHTTPResponse struct {
	status  int
	body    string
	headers http.Header
}

func newRecoveryEnv(t *testing.T) *recoveryEnv {
	return newRecoveryEnvWithHTTPClient(t, http.DefaultClient, false)
}

func newRecoveryEnvWithHTTPClient(t *testing.T, httpClient *http.Client, managedCredential bool) *recoveryEnv {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", TenantID: 1, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	if !managedCredential {
		require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)
	}

	github := newRecoveryGitHubStub(t)
	wsRoot := t.TempDir()
	workspace, err := codedelivery.NewLocalWorkspaceSource(wsRoot)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	var connections codedelivery.ConnectionReader
	var credentialSource appconnectorsvc.ConnectionCredentialSource
	var creds appconnectorsvc.CredentialResolver
	if managedCredential {
		require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES (?, ?, ?, ?)`,
			"github", 1, "github", types.MCPTransportHTTPStreamable).Error)
		bindingStore := repository.NewMCPOAuthBindingStore(db)
		require.NoError(t, bindingStore.IssueBindingState(context.Background(), appconnector.OAuthBinding{
			State: "recovery-fixture-oauth-state", InstallationID: "inst-gh", ActorID: "u1", TenantID: 1,
			ExpiresAt: time.Now().Add(time.Minute),
		}, "github"))
		conn, err := bindingStore.CompleteBinding(context.Background(), 1, "recovery-fixture-oauth-state", "u1", &types.MCPOAuthToken{
			AccessToken: t25ProbeToken, TokenType: "Bearer",
		})
		require.NoError(t, err)
		// The existing HTTP flow references conn-gh, so preserve that fixture id
		// while keeping the production credential reference and token row.
		require.NoError(t, db.Model(&appconnectorrepo.ConnectionRow{}).Where("tenant_id = ? AND id = ?", 1, conn.ID).Update("id", "conn-gh").Error)
		connections = bindingStore
		credentialSource = bindingStore
		creds = appconnectorsvc.NewCredentialResolver(bindingStore)
	} else {
		fake := &t25CredentialSource{db: db, members: map[string]bool{"u1": true}}
		connections = fake
		credentialSource = fake
		creds = fake
	}
	guard := appconnectorsvc.NewSubjectGuard(credentialSource)
	factory := codedelivery.NewGitHubClientFactory(httpClient, github.srv.URL)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: creds, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: creds,
		GitHub: factory, Providers: appconnectorrepo.NewInstallationStore(db),
		Workspace: workspace, Runs: runs, Dispatcher: dispatcher,
	})

	deliveryHandler := session.NewWorkbenchDeliveryHandler(runs, runs, svc)
	actionHandler := handler.NewAppActionHandler(db)
	actionHandler.SetActionService(actions)
	// The router-package contract test exercises production route registration,
	// role guards and the read gate. This harness retains the real HTTP logger
	// so the credential test can inspect emitted request/response logs.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Logger())
	v1 := r.Group("/api/v1")
	v1.GET("/workbench/executions/:run_id/delivery", deliveryHandler.GetDelivery)
	v1.POST("/workbench/executions/:run_id/baseline", deliveryHandler.MaterializeBaseline)
	v1.POST("/workbench/executions/:run_id/delivery", deliveryHandler.PrepareDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/dispatch", deliveryHandler.DispatchDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/resolve", deliveryHandler.ResolveDeliveryUnknown)
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	return &recoveryEnv{db: db, engine: r, wsRoot: wsRoot, github: github}
}

func (e *recoveryEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	e.responses = append(e.responses, recoveryHTTPResponse{
		status: w.Code, body: w.Body.String(), headers: w.Result().Header.Clone(),
	})
	return w
}

// seedApprovedDelivery drives the owner's real baseline→edit→prepare→approve
// loop and returns the delivery id. Request shapes mirror the T23 rig exactly
// (delivery_collaboration_http_test.go:289-326): repo is the STRING
// "owner/name" (deliveryBaselineInput.Repo is a string, workbench_delivery.go:52),
// baseline answers 201, and approve carries the action's CURRENT fence as
// expected_version.
func (e *recoveryEnv) seedApprovedDelivery(t *testing.T) string {
	t.Helper()
	w := e.do(t, "POST", "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+t25BaseSHA+`"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	target := filepath.Join(e.wsRoot, "octocat/hello")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "main.go"), []byte("package main\n\nfunc main() { println(\"t25\") }\n"), 0o644))
	w = e.do(t, "POST", "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+t25BaseSHA+`","commit_message":"fix: t25 recovery","pr_title":"WeKnora t25 recovery"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var prepared struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prepared))
	require.Equal(t, "u1", prepared.Data.Delivery.Initiator)
	require.Equal(t, "prepared", prepared.Data.Delivery.State)
	// Approve over the REAL A03 endpoint with the persisted fence — same as
	// the T23 approveThroughHTTP (delivery_collaboration_http_test.go:323-332).
	var fence int64
	require.NoError(t, e.db.Raw(`SELECT fence FROM app_actions WHERE id = ?`, prepared.Data.Delivery.ActionID).Scan(&fence).Error)
	w = e.do(t, "POST", "/api/v1/apps/actions/"+prepared.Data.Delivery.ActionID+"/approve",
		fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, prepared.Data.Delivery.Digest, fence))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return prepared.Data.Delivery.ID
}

func (e *recoveryEnv) dispatchState(t *testing.T, deliveryID string) (int, codedelivery.DeliveryView) {
	t.Helper()
	w := e.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch", "")
	var out struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w.Code, out.Data.Delivery
}

func (e *recoveryEnv) replaceManagedTokenWithWrongScope(t *testing.T, tenant uint64, owner, service string) {
	t.Helper()
	require.NoError(t, e.db.Where("tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ?", 1, types.PrincipalWebUser, "u1", "github").Delete(&types.MCPOAuthToken{}).Error)
	if tenant != 1 {
		require.NoError(t, e.db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh-wrong", TenantID: tenant, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	}
	var memberCount int64
	require.NoError(t, e.db.Model(&types.TenantMember{}).Where("tenant_id = ? AND user_id = ?", tenant, owner).Count(&memberCount).Error)
	if memberCount == 0 {
		require.NoError(t, e.db.Exec(`INSERT INTO tenant_members (user_id, tenant_id, role, status, joined_at) VALUES (?, ?, ?, ?, ?)`, owner, tenant, types.TenantRoleContributor, types.TenantMemberStatusActive, time.Now()).Error)
	}
	require.NoError(t, e.db.Exec(`INSERT OR IGNORE INTO mcp_services (id, tenant_id, name, transport_type) VALUES (?, ?, ?, ?)`, service, tenant, service, types.MCPTransportHTTPStreamable).Error)
	installationID := "inst-gh"
	if tenant != 1 { installationID = "inst-gh-wrong" }
	bs := repository.NewMCPOAuthBindingStore(e.db)
	require.NoError(t, bs.IssueBindingState(context.Background(), appconnector.OAuthBinding{State: "wrong-scope-state", InstallationID: installationID, ActorID: owner, TenantID: tenant, ExpiresAt: time.Now().Add(time.Minute)}, service))
	_, err := bs.CompleteBinding(context.Background(), tenant, "wrong-scope-state", owner, &types.MCPOAuthToken{AccessToken: t25ProbeToken, TokenType: "Bearer"})
	require.NoError(t, err)
}

// AC1 e2e：推送成功 + PR 确定性失败 = pushed；恢复只补 PR 恰一次；
// delivered 之后的重复派发被状态机拒绝（409）且零远端副作用。
func TestT25PartialPushPRFailureRecoversExactlyOnceOverHTTP(t *testing.T) {
	env := newRecoveryEnv(t)
	env.github.mu.Lock()
	env.github.failPRCreations = 1
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	code, view := env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "pushed", view.State, "push-success + definite PR failure = partial completion")
	require.NotEmpty(t, view.CommitSHA, "the pushed commit is already on the ledger")

	pushed := env.github.snapshotCalls()
	for _, key := range []string{"POST /git/blobs", "POST /git/trees", "POST /git/commits", "POST /git/refs"} {
		require.Equal(t, 1, pushed[key], "%s must have happened exactly once during the push half", key)
	}
	require.Equal(t, 1, pushed["POST /pulls"], "the PR creation failed exactly once")

	// The recovery: same approval, PR-only.
	code, view = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "delivered", view.State)
	require.NotZero(t, view.PRNumber)
	require.NotEmpty(t, view.PRURL)

	recovered := env.github.snapshotCalls()
	require.Equal(t, pushed["POST /git/blobs"], recovered["POST /git/blobs"], "recovery must not re-send blobs")
	require.Equal(t, pushed["POST /git/trees"], recovered["POST /git/trees"], "recovery must not re-send trees")
	require.Equal(t, pushed["POST /git/commits"], recovered["POST /git/commits"], "recovery must not re-send commits")
	require.Equal(t, pushed["POST /git/refs"], recovered["POST /git/refs"], "recovery must not re-push the branch")
	require.Equal(t, pushed["POST /pulls"]+1, recovered["POST /pulls"], "recovery retries ONLY the PR creation")

	// Duplicate dispatch after delivery: refused by the state machine, zero
	// new remote side effects (double-tap / network-retry re-entry).
	code, _ = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusConflict, code)
	final := env.github.snapshotCalls()
	require.Equal(t, recovered, final, "the rejected duplicate dispatch must not touch the provider")

	w := env.do(t, "GET", "/api/v1/workbench/executions/r1/delivery", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"state":"delivered"`)
}

// AC1 e2e（unknown half）：GitHub 已创建 PR 但响应丢失 → unknown；resolve
// 只读远端分支和 PR 事实，直接收敛为 delivered，不再重放任何写操作。
func TestT25UnknownResolvesFromRemoteFactsOverHTTP(t *testing.T) {
	env := newRecoveryEnv(t)
	env.github.mu.Lock()
	env.github.failPRTransport = true
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	code, view := env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "unknown", view.State, "an unobservable PR transport failure settles unknown, never a replay")
	beforeResolve := env.github.snapshotWrites()

	w := env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/resolve", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resolved struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resolved))
	require.Equal(t, "delivered", resolved.Data.Delivery.State, "remote facts include the PR committed before its response was lost")
	require.NotZero(t, resolved.Data.Delivery.PRNumber)
	require.Equal(t, beforeResolve, env.github.snapshotWrites(), "resolution must reconcile remote facts without issuing any provider writes")
}

// AC2/凭据隔离 e2e（spec :140 "does not ... expose remote credentials to
// Shell"）：在此 delivery path 中探针 token 只用于 GitHub Authorization；
// 它不出现在 HTTP response、共享工作区、交付台账、连接/Action 行、环境或日志。
// 该 server-side delivery path 不启动 Shell / RemoteSandboxClient，见 Task 2 Ruling。
func TestT25CredentialsNeverLeaveTheDispatchBoundary(t *testing.T) {
	// This test is intentionally not parallel: SetOutput is package-global.
	// The logger's documented default destination is os.Stdout; restore it
	// after the captured production request/application log stream.
	env := newRecoveryEnv(t)
	logger.SetOutput(&env.logs)
	defer logger.SetOutput(os.Stdout)
	env.github.mu.Lock()
	env.github.failPRTransport = true
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	// e.do records every HTTP response, including baseline materialization,
	// delivery preparation and the real A03 approval response from seeding.
	code, dispatched := env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "unknown", dispatched.State, "the provider committed the PR but its response was lost")
	code, _ = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusConflict, code, "an unknown delivery cannot be blindly dispatched again")
	read := env.do(t, "GET", "/api/v1/workbench/executions/r1/delivery", "")
	require.Equal(t, http.StatusOK, read.Code)
	var readView struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(read.Body.Bytes(), &readView))
	require.Equal(t, "unknown", readView.Data.Delivery.State)
	resolved := env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/resolve", "")
	require.Equal(t, http.StatusOK, resolved.Code)
	var resolvedView struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resolved.Body.Bytes(), &resolvedView))
	require.Equal(t, "delivered", resolvedView.Data.Delivery.State, "remote facts reconcile the committed PR")
	require.GreaterOrEqual(t, len(env.responses), 7, "credential scan must cover baseline, prepare, approval and later delivery responses")

	for i, response := range env.responses {
		require.NotContains(t, response.body, t25ProbeToken, "response %d body must not carry credential material", i)
		for key, values := range response.headers {
			require.NotContains(t, key, t25ProbeToken, "response %d header name must not carry credential material", i)
			for _, value := range values {
				require.NotContains(t, value, t25ProbeToken, "response %d header %q must not carry credential material", i, key)
			}
		}
	}
	// The task workspace is the file surface shared with task execution; no
	// dispatched file may embed the probe. This flow does not launch a Shell.
	require.NoError(t, filepath.WalkDir(env.wsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		require.NotContains(t, string(content), t25ProbeToken, "workspace file %s must not embed the credential (Shell isolation)", path)
		return nil
	}))
	// The delivery ledger: no string column may embed the probe.
	var rows []deliveryrepo.DeliveryRow
	require.NoError(t, env.db.Where("tenant_id = ?", 1).Find(&rows).Error)
	for _, row := range rows {
		for _, field := range []string{row.ID, row.TaskID, row.RunID, row.OwnerID, row.ActionID,
			row.ConnectionID, row.Repo, row.BaselineSHA, row.Branch, row.CommitSHA, row.PRURL,
			row.RemoteLogin, row.State, row.Failure} {
			require.NotContains(t, field, t25ProbeToken, "the delivery ledger must not carry credential material")
		}
	}
	var actions []appconnectorrepo.ActionRow
	require.NoError(t, env.db.Find(&actions).Error)
	for _, row := range actions {
		assertNoProbeInStringFields(t, "app_actions", row)
	}
	var connections []appconnectorrepo.ConnectionRow
	require.NoError(t, env.db.Find(&connections).Error)
	for _, row := range connections {
		assertNoProbeInStringFields(t, "connections", row)
	}
	for _, entry := range os.Environ() {
		require.NotContains(t, entry, t25ProbeToken, "credential resolution must not add the probe to the inherited environment used by shell launchers")
	}
	logOutput := env.logs.String()
	require.Contains(t, logOutput, "method=POST", "credential scan requires evidence that production request logs were captured")
	require.Contains(t, logOutput, "/apps/actions/", "captured logs must include the A03 approval request")
	require.NotContains(t, logOutput, t25ProbeToken, "request/response and application logs must not contain the credential")
	// Positive control: the probe DID leave as the Authorization header —
	// every provider call carried it (otherwise the stub 401s and the run
	// above would not have reached delivered/pushed states).
	require.NotZero(t, env.github.snapshotCalls()["POST /git/blobs"])
}

func assertNoProbeInStringFields(t *testing.T, table string, row any) {
	t.Helper()
	value := reflect.ValueOf(row)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	require.Equal(t, reflect.Struct, value.Kind())
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Kind() == reflect.String {
			require.NotContains(t, field.String(), t25ProbeToken, "%s.%s must not persist credential material", table, value.Type().Field(i).Name)
		}
	}
}
