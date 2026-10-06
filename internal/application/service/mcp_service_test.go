package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMCPRepo is a minimal in-memory implementation of
// interfaces.MCPServiceRepository for testing the service-layer logic
// without depending on the database.
type fakeMCPRepo struct {
	store map[string]*types.MCPService
}

func newFakeMCPRepo() *fakeMCPRepo {
	return &fakeMCPRepo{store: make(map[string]*types.MCPService)}
}

func (r *fakeMCPRepo) Create(_ context.Context, s *types.MCPService) error {
	r.store[s.ID] = cloneService(s)
	return nil
}

func (r *fakeMCPRepo) GetByID(_ context.Context, _ uint64, id string) (*types.MCPService, error) {
	s, ok := r.store[id]
	if !ok {
		return nil, nil
	}
	// Return a copy so service-layer mutations don't leak back into the
	// store, mirroring how GORM returns a fresh struct per query.
	return cloneService(s), nil
}

func cloneService(s *types.MCPService) *types.MCPService {
	cp := *s
	if s.AuthConfig != nil {
		ac := *s.AuthConfig
		if s.AuthConfig.CustomHeaders != nil {
			ac.CustomHeaders = make(map[string]string, len(s.AuthConfig.CustomHeaders))
			for k, v := range s.AuthConfig.CustomHeaders {
				ac.CustomHeaders[k] = v
			}
		}
		cp.AuthConfig = &ac
	}
	if s.AdvancedConfig != nil {
		adv := *s.AdvancedConfig
		cp.AdvancedConfig = &adv
	}
	return &cp
}

func (r *fakeMCPRepo) List(_ context.Context, _ uint64) ([]*types.MCPService, error) {
	out := make([]*types.MCPService, 0, len(r.store))
	for _, s := range r.store {
		out = append(out, cloneService(s))
	}
	return out, nil
}

func (r *fakeMCPRepo) ListEnabled(ctx context.Context, tenantID uint64) ([]*types.MCPService, error) {
	return r.List(ctx, tenantID)
}

func (r *fakeMCPRepo) ListByIDs(_ context.Context, _ uint64, ids []string) ([]*types.MCPService, error) {
	out := make([]*types.MCPService, 0, len(ids))
	for _, id := range ids {
		if s, ok := r.store[id]; ok {
			out = append(out, cloneService(s))
		}
	}
	return out, nil
}

func (r *fakeMCPRepo) Update(_ context.Context, s *types.MCPService) error {
	r.store[s.ID] = cloneService(s)
	return nil
}

func (r *fakeMCPRepo) Delete(_ context.Context, _ uint64, id string) error {
	delete(r.store, id)
	return nil
}

func seedService(t *testing.T, repo *fakeMCPRepo, apiKey, token string) string {
	t.Helper()
	s := &types.MCPService{
		ID:            "svc-test",
		TenantID:      1,
		Name:          "test",
		Enabled:       true,
		TransportType: types.MCPTransportSSE,
		AuthConfig: &types.MCPAuthConfig{
			APIKey: apiKey,
			Token:  token,
		},
	}
	require.NoError(t, repo.Create(context.Background(), s))
	return s.ID
}

// newTestService wires up a mcpServiceService with a fresh fake repo and a
// real (empty) MCPManager. CloseClient on an empty manager is a no-op.
func newTestService() (*mcpServiceService, *fakeMCPRepo) {
	repo := newFakeMCPRepo()
	svc := &mcpServiceService{
		mcpServiceRepo: repo,
		mcpManager:     mcp.NewMCPManager(nil),
		oauthRepo:      nil,
	}
	return svc, repo
}

func TestUpdateMCPService_RespectsScalarFieldPresence(t *testing.T) {
	tests := []struct {
		name            string
		update          *types.MCPService
		updateFields    map[string]bool
		wantName        string
		wantDescription string
		wantEnabled     bool
	}{
		{
			name:            "description only",
			update:          &types.MCPService{Description: "after"},
			updateFields:    map[string]bool{"description": true},
			wantName:        "test",
			wantDescription: "after",
			wantEnabled:     true,
		},
		{
			name:            "name only",
			update:          &types.MCPService{Name: "renamed"},
			updateFields:    map[string]bool{"name": true},
			wantName:        "renamed",
			wantDescription: "before",
			wantEnabled:     true,
		},
		{
			name:            "explicit empty description",
			update:          &types.MCPService{Description: ""},
			updateFields:    map[string]bool{"description": true},
			wantName:        "test",
			wantDescription: "",
			wantEnabled:     true,
		},
		{
			name:            "explicit disable",
			update:          &types.MCPService{Enabled: false},
			updateFields:    map[string]bool{"enabled": true},
			wantName:        "test",
			wantDescription: "before",
			wantEnabled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, repo := newTestService()
			id := seedService(t, repo, "stored-api", "stored-token")
			repo.store[id].Description = "before"

			tt.update.ID = id
			tt.update.TenantID = 1
			require.NoError(t, svc.UpdateMCPService(ctx, tt.update, tt.updateFields))

			got := repo.store[id]
			assert.Equal(t, tt.wantName, got.Name)
			assert.Equal(t, tt.wantDescription, got.Description)
			assert.Equal(t, tt.wantEnabled, got.Enabled)
		})
	}
}

func TestUpdateMCPService_AppliesNonScalarUpdateWithoutName(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")
	// Use resolvable example.com paths: subdomains like before.example.com fail
	// SSRF DNS checks because they do not resolve to a public IP.
	beforeURL := "https://example.com/before"
	repo.store[id].Description = "before"
	repo.store[id].URL = &beforeURL

	afterURL := "https://example.com/after"
	update := &types.MCPService{
		ID:       id,
		TenantID: 1,
		URL:      &afterURL,
	}
	require.NoError(t, svc.UpdateMCPService(ctx, update, nil))

	got := repo.store[id]
	require.NotNil(t, got.URL)
	assert.Equal(t, afterURL, *got.URL)
	assert.Equal(t, "test", got.Name)
	assert.Equal(t, "before", got.Description)
	assert.True(t, got.Enabled)
}

// ---- UpdateMCPService: must not touch APIKey/Token even when caller sends them ----

// The handler now strips api_key/token from the main PUT body, but defense
// in depth: if a future caller (CLI / test / misconfigured proxy) still
// passes auth_config with secret fields, UpdateMCPService must NOT clobber
// the stored credentials. Credentials live behind the dedicated subresource.
func TestUpdateMCPService_DoesNotTouchSecretsEvenIfPassed(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")

	upd := &types.MCPService{
		ID:            id,
		TenantID:      1,
		Name:          "renamed",
		Enabled:       true,
		TransportType: types.MCPTransportSSE,
		// Hostile body: tries to overwrite both secrets via main PUT.
		AuthConfig: &types.MCPAuthConfig{
			APIKey: "should-not-overwrite",
			Token:  "should-not-overwrite-either",
		},
	}
	require.NoError(t, svc.UpdateMCPService(ctx, upd, map[string]bool{
		"name":    true,
		"enabled": true,
	}))

	got := repo.store[id]
	assert.Equal(t, "stored-api", got.AuthConfig.APIKey,
		"main PUT must not overwrite stored APIKey under any circumstance")
	assert.Equal(t, "stored-token", got.AuthConfig.Token,
		"main PUT must not overwrite stored Token under any circumstance")
	assert.Equal(t, "renamed", got.Name, "non-secret field updates still apply")
}

func TestUpdateMCPService_CustomHeadersPreserveOnNil(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")
	repo.store[id].AuthConfig.CustomHeaders = map[string]string{"X-Tenant": "acme"}

	upd := &types.MCPService{
		ID:            id,
		TenantID:      1,
		Name:          "test",
		Enabled:       true,
		TransportType: types.MCPTransportSSE,
		// AuthConfig present but CustomHeaders nil → preserve.
		AuthConfig: &types.MCPAuthConfig{},
	}
	require.NoError(t, svc.UpdateMCPService(ctx, upd, nil))

	got := repo.store[id]
	assert.Equal(t, "acme", got.AuthConfig.CustomHeaders["X-Tenant"],
		"nil CustomHeaders in request must preserve existing headers")
}

func TestUpdateMCPService_CustomHeadersReplaceOnNonNil(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")
	repo.store[id].AuthConfig.CustomHeaders = map[string]string{"X-Tenant": "acme"}

	upd := &types.MCPService{
		ID:            id,
		TenantID:      1,
		Name:          "test",
		Enabled:       true,
		TransportType: types.MCPTransportSSE,
		AuthConfig: &types.MCPAuthConfig{
			CustomHeaders: map[string]string{"X-Replaced": "yes"},
		},
	}
	require.NoError(t, svc.UpdateMCPService(ctx, upd, nil))

	got := repo.store[id]
	assert.Equal(t, map[string]string{"X-Replaced": "yes"}, got.AuthConfig.CustomHeaders,
		"non-nil CustomHeaders must replace the stored map")
}

// ---- UpdateMCPCredentials: write path ----

func TestUpdateMCPCredentials_WritesAPIKey(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "", "")

	newKey := "fresh-api-key"
	got, err := svc.UpdateMCPCredentials(ctx, 1, id, &newKey, nil)
	require.NoError(t, err)
	require.NotNil(t, got.AuthConfig)
	assert.Equal(t, "fresh-api-key", got.AuthConfig.APIKey)
	assert.Empty(t, got.AuthConfig.Token, "untouched field stays untouched")

	stored := repo.store[id]
	assert.Equal(t, "fresh-api-key", stored.AuthConfig.APIKey, "persisted")
}

func TestUpdateMCPCredentials_NilPointerIsNoop(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")

	got, err := svc.UpdateMCPCredentials(ctx, 1, id, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "stored-api", got.AuthConfig.APIKey)
	assert.Equal(t, "stored-token", got.AuthConfig.Token)
}

func TestUpdateMCPCredentials_EmptyStringIsNoop(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")

	empty := ""
	got, err := svc.UpdateMCPCredentials(ctx, 1, id, &empty, &empty)
	require.NoError(t, err)
	assert.Equal(t, "stored-api", got.AuthConfig.APIKey,
		"empty string is treated as no-op; clearing goes through ClearMCPCredential")
	assert.Equal(t, "stored-token", got.AuthConfig.Token)
}

func TestUpdateMCPCredentials_ReplacesExisting(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "old-api", "old-token")

	newKey, newTok := "new-api", "new-tok"
	got, err := svc.UpdateMCPCredentials(ctx, 1, id, &newKey, &newTok)
	require.NoError(t, err)
	assert.Equal(t, "new-api", got.AuthConfig.APIKey)
	assert.Equal(t, "new-tok", got.AuthConfig.Token)
}

func TestUpdateMCPCredentials_RejectsBuiltin(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "")
	repo.store[id].IsBuiltin = true

	newKey := "anything"
	_, err := svc.UpdateMCPCredentials(ctx, 1, id, &newKey, nil)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "builtin")
}

func TestUpdateMCPCredentials_ServiceNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService()
	newKey := "x"
	_, err := svc.UpdateMCPCredentials(ctx, 1, "nope", &newKey, nil)
	require.Error(t, err)
}

// ---- ClearMCPCredential ----

func TestClearMCPCredential_ClearsAPIKey(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")

	require.NoError(t, svc.ClearMCPCredential(ctx, 1, id, "api_key"))
	stored := repo.store[id]
	assert.Empty(t, stored.AuthConfig.APIKey)
	assert.Equal(t, "stored-token", stored.AuthConfig.Token, "other field untouched")
}

func TestClearMCPCredential_ClearsToken(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "stored-token")

	require.NoError(t, svc.ClearMCPCredential(ctx, 1, id, "token"))
	stored := repo.store[id]
	assert.Equal(t, "stored-api", stored.AuthConfig.APIKey)
	assert.Empty(t, stored.AuthConfig.Token)
}

func TestClearMCPCredential_IdempotentOnEmpty(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "") // token already empty

	require.NoError(t, svc.ClearMCPCredential(ctx, 1, id, "token"),
		"clearing already-empty field must not error")
	stored := repo.store[id]
	assert.Equal(t, "stored-api", stored.AuthConfig.APIKey)
	assert.Empty(t, stored.AuthConfig.Token)
}

func TestClearMCPCredential_UnknownFieldErrors(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "")

	err := svc.ClearMCPCredential(ctx, 1, id, "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestClearMCPCredential_RejectsBuiltin(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "stored-api", "")
	repo.store[id].IsBuiltin = true

	err := svc.ClearMCPCredential(ctx, 1, id, "api_key")
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "builtin")
}

// ---- Service-layer Get/List: returns RAW entity (DTO handles redaction) ----

// After the credential-subresource refactor, the service-layer Get/List
// return the entity unmodified. Handlers MUST convert via
// dto.NewMCPServiceResponse, but the credentials handler depends on the
// unredacted form to derive metadata (configured: bool).
func TestGetMCPServiceByID_ReturnsRawCredentials(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	id := seedService(t, repo, "real-api", "real-token")

	got, err := svc.GetMCPServiceByID(ctx, 1, id)
	require.NoError(t, err)
	require.NotNil(t, got.AuthConfig)
	assert.Equal(t, "real-api", got.AuthConfig.APIKey,
		"service layer returns raw credentials; redaction is the DTO's job")
	assert.Equal(t, "real-token", got.AuthConfig.Token)
}

func TestListMCPServices_ReturnsRawCredentials(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestService()
	seedService(t, repo, "real-api", "real-token")

	got, err := svc.ListMCPServices(ctx, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "real-api", got[0].AuthConfig.APIKey)
	assert.Equal(t, "real-token", got[0].AuthConfig.Token)
}

// seedPluginManagedService 落一条插件物化形态的服务行（PluginInstallationID
// 指向安装行）——GetMCPServiceResources（Viewer+）与 TestMCPService（Admin+）
// 对这类行必须与 GetMCPServiceTools（OCR R1 F07）同口径拒绝：实时远端
// ListResources/连通测试绕过已接受快照边界，未接受能力/漂移后目录可经
// 此二面泄露（B+A 裁决 #4）。
func seedPluginManagedService(t *testing.T, repo *fakeMCPRepo) string {
	t.Helper()
	installationID := "11111111-2222-4333-8444-555555555555"
	endpoint := "http://127.0.0.1:1/mcp"
	s := &types.MCPService{
		ID:                   "svc-plugin-managed",
		TenantID:             1,
		Name:                 "plugin:com.example.jira-todo",
		Enabled:              true,
		TransportType:        types.MCPTransportSSE,
		URL:                  &endpoint,
		PluginInstallationID: &installationID,
	}
	require.NoError(t, repo.Create(context.Background(), s))
	return s.ID
}

func TestGetMCPServiceResourcesRejectsPluginManagedRow(t *testing.T) {
	svc, repo := newTestService()
	id := seedPluginManagedService(t, repo)

	_, err := svc.GetMCPServiceResources(context.Background(), 1, id)
	require.ErrorIs(t, err, ErrPluginManagedService,
		"plugin-materialized rows must not serve live remote resources through the generic Viewer+ endpoint — the accepted snapshot is the only directory boundary")
}

func TestMCPServiceTestRejectsPluginManagedRow(t *testing.T) {
	svc, repo := newTestService()
	id := seedPluginManagedService(t, repo)

	_, err := svc.TestMCPService(context.Background(), 1, id)
	require.ErrorIs(t, err, ErrPluginManagedService,
		"a live connectivity test against the plugin endpoint would enumerate unaccepted tools/resources in its Admin+ result payload")
}

// ---- TestMCPService: tools/list failure must not report success (#3880) ----

// newFakeMCPUpstream starts an HTTP-Streamable MCP endpoint speaking raw
// JSON-RPC over single-shot application/json responses. It reproduces the
// upstream repro from #3880: initialize answers a healthy handshake while
// tools/list answers a -32603 JSON-RPC error. An empty toolsListError serves
// a one-tool directory instead (success path).
func newFakeMCPUpstream(t *testing.T, toolsListError string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(req.ID) == 0 {
			// Notifications (notifications/initialized) carry no response body.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		reply := func(payload map[string]any) {
			payload["jsonrpc"] = "2.0"
			payload["id"] = req.ID
			_ = json.NewEncoder(w).Encode(payload)
		}
		switch req.Method {
		case "initialize":
			reply(map[string]any{
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
					"serverInfo":      map[string]any{"name": "fake-echo", "version": "9.9.9"},
				},
			})
		case "tools/list":
			if toolsListError != "" {
				reply(map[string]any{
					"error": map[string]any{"code": -32603, "message": toolsListError},
				})
				return
			}
			reply(map[string]any{
				"result": map[string]any{
					"tools": []any{map[string]any{
						"name":        "echo",
						"description": "Echo a message",
						"inputSchema": map[string]any{"type": "object"},
					}},
				},
			})
		case "resources/list":
			reply(map[string]any{
				"result": map[string]any{"resources": []any{}},
			})
		default:
			reply(map[string]any{
				"error": map[string]any{"code": -32601, "message": "method not found: " + req.Method},
			})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func seedHTTPStreamableService(t *testing.T, repo *fakeMCPRepo, url string) string {
	t.Helper()
	s := &types.MCPService{
		ID:            "svc-http",
		TenantID:      1,
		Name:          "http-streamable",
		Enabled:       true,
		URL:           &url,
		TransportType: types.MCPTransportHTTPStreamable,
	}
	require.NoError(t, repo.Create(context.Background(), s))
	return s.ID
}

// A server whose initialize succeeds but whose tools/list answers -32603 used
// to be reported as {"success": true, "tools": []} — presenting a broken
// upstream as a healthy service and misleading troubleshooting. The test must
// answer success=false and keep the original error in the message.
func TestMCPServiceTest_ToolsListFailureIsNotSuccess(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)

	upstream := newFakeMCPUpstream(t, "upstream tool registry exploded")
	svc, repo := newTestService()
	id := seedHTTPStreamableService(t, repo, upstream.URL)

	result, err := svc.TestMCPService(context.Background(), 1, id)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success,
		"a failed tools/list must never be reported as a successful connection")
	assert.Contains(t, result.Message, "failed to list tools")
	assert.Contains(t, result.Message, "upstream tool registry exploded",
		"the original upstream error must stay in the message for troubleshooting")
	assert.False(t, result.OAuthRequired)
}

func TestMCPServiceTest_SuccessIncludesTools(t *testing.T) {
	utils.SetSSRFWhitelistFromRaw("127.0.0.1")
	t.Cleanup(utils.ResetSSRFWhitelistForTest)

	upstream := newFakeMCPUpstream(t, "")
	svc, repo := newTestService()
	id := seedHTTPStreamableService(t, repo, upstream.URL)

	result, err := svc.TestMCPService(context.Background(), 1, id)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "Connected successfully")
	assert.Contains(t, result.Message, "fake-echo")
	require.Len(t, result.Tools, 1)
	assert.Equal(t, "echo", result.Tools[0].Name)
}
