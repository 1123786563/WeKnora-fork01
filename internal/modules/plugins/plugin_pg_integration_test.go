//go:build integration

// T11（plan 05 Task 11 Step 5）：真 PostgreSQL 应用边界集成测试——成员
// 个人 OAuth 授权/隔离/撤销/过期。
//
// 环境契约（与 oc_integration_test.go / migration_pg_integration_test.go
// 同一约定）：
//
//	PLUGIN_TEST_DATABASE_URL  一次性 PostgreSQL DSN。
//	                          缺失 → t.Fatal("blocked-env: ...")，绝不 Skip 通过。
//
// pluginpg 基建（openPluginDB / newPluginStack）供 T13/T16-T19 复用：
// isolated schema + AutoMigrate 冻结模型 + 真实 repo/service/manager/
// oauthManager 组装 + plugintest 受控远端（完整 OAuth 端点集替身）。
package plugins_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// pluginpg 基建
// ---------------------------------------------------------------------------

// TestPluginPGEnvironment pins the environment contract: a missing
// PLUGIN_TEST_DATABASE_URL FAILS the suite (blocked-env), never skips.
func TestPluginPGEnvironment(t *testing.T) {
	if os.Getenv("PLUGIN_TEST_DATABASE_URL") == "" {
		t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")
	}
}

// openPluginDB opens the acceptance PostgreSQL, provisions an isolated
// per-run schema, AutoMigrates the FROZEN model set the plugin slice touches
// and registers the fixture cleanup (drop THIS schema only). The plan's
// frozen-model convention (T11 Produces): later tasks (T13/T16-T19) extend
// the model list only when their slice adds tables.
func openPluginDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("PLUGIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	schema := fmt.Sprintf("plugins_t11_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema %s: %v", schema, err)
		}
		sqlAdmin, _ := admin.DB()
		if sqlAdmin != nil {
			_ = sqlAdmin.Close()
		}
	})
	dsnSchema := dsn
	if p, err := url.Parse(dsn); err == nil && (p.Scheme == "postgres" || p.Scheme == "postgresql") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsnSchema = dsn + sep + "search_path=" + schema
	} else {
		// pgx keyword/value DSN: space-separated settings.
		dsnSchema = dsn + " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsnSchema), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open isolated schema: %v", err)
	}
	if err := db.AutoMigrate(
		&types.MCPService{},
		&types.MCPToolApproval{},
		&types.MCPOAuthToken{},
		&types.MCPOAuthClient{},
		&types.MCPMetadata{},
		&types.PluginPreview{},
		&types.PluginInstallation{},
		&types.TenantMember{},
	); err != nil {
		t.Fatalf("automigrate frozen models: %v", err)
	}
	return db
}

// pluginPGStack is the real application-boundary stack: real repositories,
// real services, real MCPManager/OAuthManager, controlled plugintest remote.
type pluginPGStack struct {
	db             *gorm.DB
	tenantID       uint64
	svc            interfaces.PluginService
	mcpServiceRepo interfaces.MCPServiceRepository
	oauthRepo      interfaces.MCPOAuthRepository
	manager        *internalmcp.MCPManager
	oauthManager   *internalmcp.OAuthManager
	remote         *plugintest.Server
	service        *types.MCPService // materialized service of the installation
	installationID string
}

// plugintestAuthToolSchema 与 plugintest 远端 search 工具的 schema 完全
// 一致（清单声明 digest 与 live 目录一致是预览核验的前提）。
const plugintestAuthToolSchema = `{"type":"object","properties":{},"additionalProperties":false}`

// newPluginStack assembles the full stack and lands ONE confirmed
// installation (admin preview → ConfirmInstallation) of the plugintest
// plugin with a personal-auth search tool and OAuth enabled for two members.
func newPluginStack(t *testing.T, db *gorm.DB, tenantID uint64) *pluginPGStack {
	t.Helper()

	remote := plugintest.New()
	remote.EnableOAuth(map[string]string{"user-a": "pass-a", "user-b": "pass-b"})
	remote.SetTools([]plugintest.Tool{{
		Name: "search", Description: "personal search", ReadOnly: true,
		RequiresPersonalAuth: true, Scopes: []string{"read:demo"},
		InputSchema: plugintestAuthToolSchema,
		Call: func(member string) (string, error) {
			if member == "" {
				return "", fmt.Errorf("no authenticated member on call")
			}
			return "data-for:" + member, nil
		},
	}})
	remote.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	oauthRepo := repository.NewMCPOAuthRepository(db)
	manager := internalmcp.NewMCPManager(oauthRepo)
	t.Cleanup(manager.Shutdown)
	mcpSvcService := service.NewMCPServiceService(mcpRepo, manager, oauthRepo)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)

	// Lister over the real manager, mirroring container.
	// NewPluginMCPEndpointLister's nonce-exclusive client per verification
	// (a local seam here: internal/modules 不得互相 import 容器装配)。
	lister := func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		var nonce [4]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, fmt.Errorf("generate verification nonce: %w", err)
		}
		sum := sha256.Sum256([]byte(endpointURL))
		serviceID := "plugin-verify-" + hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString(nonce[:])
		verify := &types.MCPService{
			ID: serviceID, TenantID: 0, Name: "plugin-verify", Enabled: true,
			TransportType: types.MCPTransportType(transportType), URL: &endpointURL,
		}
		client, err := manager.GetOrCreateClient(ctx, verify)
		if err != nil {
			_ = manager.CloseClient(verify.ID)
			return nil, err
		}
		defer func() {
			_ = client.Disconnect()
			_ = manager.CloseClient(verify.ID)
		}()
		return client.ListTools(ctx)
	}
	closer := service.MCPClientCloser(func(serviceID string) { _ = manager.CloseClient(serviceID) })
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, lister, closer, oauthRepo)
	oauthManager := internalmcp.NewOAuthManager(oauthRepo, mcpRepo, nil)

	ctx := context.Background()
	preview, err := svc.PreviewFromManifest(ctx, tenantID, "admin-1", remote.ManifestURL())
	require.NoError(t, err)
	result, err := svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.NotEmpty(t, result.ServiceID)
	serviceRow, err := mcpRepo.GetByID(ctx, tenantID, result.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, serviceRow)

	return &pluginPGStack{
		db:             db,
		tenantID:       tenantID,
		svc:            svc,
		mcpServiceRepo: mcpRepo,
		oauthRepo:      oauthRepo,
		manager:        manager,
		oauthManager:   oauthManager,
		remote:         remote,
		service:        serviceRow,
		installationID: result.InstallationID,
	}
}

// authorizeMember drives the REAL member OAuth flow against the plugintest
// remote: StartAuthorization (discovery + dynamic registration + PKCE) →
// member submits credentials at /authorize → one-time code →
// CompleteAuthorization (code exchange persisted per principal).
func (st *pluginPGStack) authorizeMember(t *testing.T, principal types.Principal, username, password string) {
	t.Helper()
	ctx := context.Background()
	authURL, _, err := st.oauthManager.StartAuthorization(ctx, st.service, st.tenantID, principal,
		"http://127.0.0.1:1/cb", "/")
	require.NoError(t, err)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	state := u.Query().Get("state")
	require.NotEmpty(t, state)
	require.NotEmpty(t, u.Query().Get("code_challenge"), "PKCE code_challenge required")

	// 真实流程里浏览器先 GET 授权页（登记 state），再提交凭据。
	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	page, err := noRedirect.Get(u.String())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, page.StatusCode, "authorize page must render")

	form := url.Values{"state": {state}, "username": {username}, "password": {password}}
	resp, err := noRedirect.PostForm(u.String(), form)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode, "member consent must 302 back with a code")
	location, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	code := location.Query().Get("code")
	require.NotEmpty(t, code)
	require.Equal(t, state, location.Query().Get("state"))

	_, _, err = st.oauthManager.CompleteAuthorization(ctx, state, code)
	require.NoError(t, err)
}

// memberContext builds the per-principal context the manager derives the
// OAuth connection key from (cacheKey: serviceID + principal).
func (st *pluginPGStack) memberContext(principal types.Principal) context.Context {
	ctx := types.WithPrincipal(context.Background(), principal)
	return context.WithValue(ctx, types.TenantIDContextKey, st.tenantID)
}

// callSearchTool invokes the search tool as one principal and returns the
// tool's text result.
func (st *pluginPGStack) callSearchTool(t *testing.T, principal types.Principal) (string, error) {
	t.Helper()
	client, err := st.manager.GetOrCreateClient(st.memberContext(principal), st.service)
	if err != nil {
		return "", err
	}
	res, err := client.CallTool(st.memberContext(principal), "search", map[string]interface{}{})
	if err != nil {
		return "", err
	}
	require.False(t, res.IsError, "unexpected tool error result")
	require.NotEmpty(t, res.Content)
	return res.Content[0].Text, nil
}

func memberPrincipal(id string) types.Principal {
	return types.Principal{Type: types.PrincipalWebUser, ID: id}
}

// ---------------------------------------------------------------------------
// 场景：两成员凭据隔离 + 撤销不影响他人 + 跨空间 not found
// ---------------------------------------------------------------------------

func TestMemberConnectionsIsolatedAndRevocable(t *testing.T) {
	db := openPluginDB(t)
	tenantID := uint64(1)
	st := newPluginStack(t, db, tenantID)
	ctx := context.Background()

	memberA := memberPrincipal("user-a")
	memberB := memberPrincipal("user-b")

	// 1) 未授权：A 与 B 的 connections/me 均为 unauthorized。
	for _, member := range []types.Principal{memberA, memberB} {
		status, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, member)
		require.NoError(t, err)
		require.Equal(t, types.PluginConnectionUnauthorized, status.State)
		require.Equal(t, "/api/v1/mcp-services/"+st.service.ID+"/oauth/authorize-url", status.AuthorizeURLPath)
	}

	// 2) B 未授权调用工具 → OAuth 授权引导错误（无 token 时是
	// *OAuthReauthorizationRequiredError——OAuthRequired 引导家族）。
	_, err := st.callSearchTool(t, memberB)
	require.Error(t, err)
	var reauth *internalmcp.OAuthReauthorizationRequiredError
	require.ErrorAs(t, err, &reauth, "unauthorized member must be guided to authorize")

	// 3) A 走真实 OAuth 流 → authorized；调用只命中 A 的数据。
	st.authorizeMember(t, memberA, "user-a", "pass-a")
	statusA, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, statusA.State)
	require.True(t, statusA.Authorized)
	out, err := st.callSearchTool(t, memberA)
	require.NoError(t, err)
	require.Equal(t, "data-for:user-a", out, "member A must only hit A's data")

	// B 仍未授权（凭据不混用：A 的授权不外溢）。
	statusB, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberB)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionUnauthorized, statusB.State)

	// 4) B 完成授权 → B 只命中 B 的数据，A 仍是 A 的。
	st.authorizeMember(t, memberB, "user-b", "pass-b")
	out, err = st.callSearchTool(t, memberB)
	require.NoError(t, err)
	require.Equal(t, "data-for:user-b", out)
	out, err = st.callSearchTool(t, memberA)
	require.NoError(t, err)
	require.Equal(t, "data-for:user-a", out)

	// 5) A 撤销（DELETE token 语义经 oauthManager.Revoke + 连接回收）→
	// A unauthorized，B 仍 authorized 且数据不受影响。
	require.NoError(t, st.oauthManager.Revoke(ctx, tenantID, memberA, st.service.ID))
	require.NoError(t, st.manager.CloseClient(st.service.ID))
	statusA, err = st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionUnauthorized, statusA.State)
	require.False(t, statusA.Authorized)
	statusB, err = st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberB)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, statusB.State)
	out, err = st.callSearchTool(t, memberB)
	require.NoError(t, err)
	require.Equal(t, "data-for:user-b", out)

	// 6) 空间 2（租户 2）成员查空间 1 的安装 → not found；空间 2 另装同
	// 插件，两空间 token 互不可见（(tenant, principal, service) 存储键）。
	_, err = st.svc.GetMyConnectionStatus(ctx, 2, st.installationID, memberPrincipal("user-x"))
	require.ErrorIs(t, err, service.ErrInstallationNotFound)

	st2 := newPluginStack(t, db, 2)
	// 空间 2 用同名成员 principal 授权（替身凭据表属服务侧，与空间 1 的
	// user-a 授权互不相干——存储键含 tenant 与各自物化 service_id）。
	st2.authorizeMember(t, memberPrincipal("user-a"), "user-a", "pass-a")
	statusX2, err := st2.svc.GetMyConnectionStatus(ctx, 2, st2.installationID, memberPrincipal("user-a"))
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, statusX2.State)
	// 空间 2 的授权不改变空间 1 的任何视图：A 在空间 1 仍是撤销后的
	// unauthorized，B 仍 authorized。
	statusA1, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionUnauthorized, statusA1.State)
	statusB1, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberB)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, statusB1.State)
}

// ---------------------------------------------------------------------------
// 场景：token 过期 → expired 且调用引导重授权
// ---------------------------------------------------------------------------

func TestExpiredTokenGuidesReauthorization(t *testing.T) {
	db := openPluginDB(t)
	tenantID := uint64(1)
	st := newPluginStack(t, db, tenantID)
	ctx := context.Background()

	memberA := memberPrincipal("user-a")
	st.authorizeMember(t, memberA, "user-a", "pass-a")
	status, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionAuthorized, status.State)

	// 直接把 A 的 token 置为过去且无 refresh（参数绑定，不用拼接 SQL）。
	require.NoError(t, db.Exec(
		"UPDATE mcp_oauth_tokens SET expires_at = ?, refresh_token = '' WHERE tenant_id = ? AND service_id = ?",
		time.Now().Add(-time.Minute), tenantID, st.service.ID,
	).Error)

	// connections/me → expired，DTO 引导重授权。
	status, err = st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionExpired, status.State)
	require.False(t, status.Authorized)
	require.Equal(t, "/api/v1/mcp-services/"+st.service.ID+"/oauth/authorize-url", status.AuthorizeURLPath)

	// 调用工具 → 引导重授权错误（ensureFresh：过期且无 refresh）。
	_, err = st.callSearchTool(t, memberA)
	require.Error(t, err)
	var reauth *internalmcp.OAuthReauthorizationRequiredError
	require.ErrorAs(t, err, &reauth, "expired token without refresh must guide re-authorization")
}
