package repository_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

const (
	managedDeliveryProbe = t25ProbeToken
	operatorShellProbe   = "operator-shell-value-keep-me"
)

type authorizationRecorder struct {
	http.RoundTripper
	mu     sync.Mutex
	values []string
}

func (r *authorizationRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.values = append(r.values, req.Header.Get("Authorization"))
	r.mu.Unlock()
	return r.RoundTripper.RoundTrip(req)
}

func (r *authorizationRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.values...)
}

func (r *authorizationRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = nil
}

type shellBoundaryHandle struct{ id string }

func (h *shellBoundaryHandle) ID() string                       { return h.id }
func (h *shellBoundaryHandle) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeE2B }
func (h *shellBoundaryHandle) Metadata() map[string]string      { return map[string]string{} }

type shellBoundaryClient struct {
	sandbox.RemoteSandboxClient
	mu      sync.Mutex
	creates []sandbox.RemoteCreateRequest
	execs   []sandbox.RemoteExecRequest
}

func (c *shellBoundaryClient) Provider() sandbox.RemoteProvider { return sandbox.SandboxTypeE2B }
func (c *shellBoundaryClient) Capabilities() sandbox.RemoteSandboxCapabilities {
	return sandbox.RemoteSandboxCapabilities{SupportsReconnect: true}
}
func (c *shellBoundaryClient) Health(context.Context) error { return nil }
func (c *shellBoundaryClient) Create(_ context.Context, req sandbox.RemoteCreateRequest) (sandbox.RemoteSandboxHandle, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.creates = append(c.creates, req)
	return &shellBoundaryHandle{id: "shell-boundary-1"}, nil
}
func (c *shellBoundaryClient) Connect(context.Context, sandbox.RemoteConnectRequest) (sandbox.RemoteSandboxHandle, error) {
	return &shellBoundaryHandle{id: "shell-boundary-1"}, nil
}
func (c *shellBoundaryClient) Exec(_ context.Context, _ sandbox.RemoteSandboxHandle, req sandbox.RemoteExecRequest) (*sandbox.RemoteExecResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execs = append(c.execs, req)
	return &sandbox.RemoteExecResult{Stdout: "shell-command-ok", ExitCode: 0}, nil
}
func (c *shellBoundaryClient) MakeDir(context.Context, sandbox.RemoteSandboxHandle, string) error {
	return nil
}
func (c *shellBoundaryClient) Delete(context.Context, string) error { return nil }
func (c *shellBoundaryClient) ListDir(context.Context, sandbox.RemoteSandboxHandle, string) ([]sandbox.RemoteDirEntry, error) {
	return nil, nil
}

func TestManagedDeliveryCredentialNeverEntersGeneralShell(t *testing.T) {
	// Observe requests emitted by the real Delivery factory/dispatcher. The
	// GitHub stub still validates the fake credential on every wire request.
	auth := &authorizationRecorder{RoundTripper: http.DefaultTransport}
	delivery := newRecoveryEnvWithHTTPClient(t, &http.Client{Transport: auth}, true)
	store := repository.NewMCPOAuthBindingStore(delivery.db)
	var storedToken types.MCPOAuthToken
	require.NoError(t, delivery.db.Where("tenant_id = ? AND principal_id = ? AND service_id = ?", 1, "u1", "github").First(&storedToken).Error)
	actualToken := storedToken.AccessToken
	require.Equal(t, managedDeliveryProbe, actualToken)
	conn, err := store.FindConnectionByID(context.Background(), "conn-gh")
	require.NoError(t, err)
	require.Equal(t, uint64(1), conn.TenantID)
	require.Equal(t, "u1", conn.OwnerID)
	require.Equal(t, repository.CredentialRefPrefix+"github", conn.CredentialRef)
	resolved, err := appconnectorsvc.NewCredentialResolver(store).Resolve(context.Background(), "conn-gh", conn.AuthVersion)
	require.NoError(t, err)
	require.Equal(t, actualToken, string(resolved))
	// Store a distinct operator-provided Shell variable in the real tenant
	// configuration repository. This value must survive effective config and
	// provider create unchanged.
	sandboxRepo := repository.NewTenantSandboxConfigRepository(delivery.db)
	tenantConfig := &types.TenantSandboxConfig{
		SandboxType: "e2b",
		EnvVars:     map[string]string{"OPERATOR_SHELL_VALUE": operatorShellProbe},
		E2B:         &types.E2BSandboxConfig{APIKey: "shell-test-key", TemplateID: "shell-test-template"},
	}
	stored := &types.TenantSandboxConfigEntity{
		ID: "cfg-shell-boundary", TenantID: 1, Name: "shell-boundary", SandboxType: "e2b", Config: tenantConfig,
	}
	require.NoError(t, sandboxRepo.Create(context.Background(), stored))

	client := &shellBoundaryClient{}
	var resolvedShellConfig *sandbox.Config
	resolver, err := sandbox.NewTenantSandboxResolver(sandbox.TenantSandboxResolverDeps{
		GlobalConfig: sandbox.DefaultConfig(),
		Loader:       appservice.NewTenantSandboxConfigLoader(sandboxRepo),
		Store:        sandbox.NewMemorySessionSandboxBindingStore(),
		Checker:      sandbox.PermissiveSessionExistenceChecker{},
		RemoteClientFactory: func(cfg *sandbox.Config) (sandbox.RemoteSandboxClient, error) {
			resolvedShellConfig = cfg
			return client, nil
		},
	})
	require.NoError(t, err)
	manager, err := resolver.Resolve(context.Background(), 1, "cfg-shell-boundary")
	require.NoError(t, err)
	bound, ok := manager.(*sandbox.SessionBoundManager)
	require.True(t, ok)

	// Dispatch the persisted managed credential while the same tenant's Shell
	// configuration and execution path are active in this test.
	deliveryID := delivery.seedApprovedDelivery(t)
	status, state := delivery.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "delivered", state.State)
	require.NotEmpty(t, auth.snapshot())
	for _, value := range auth.snapshot() {
		require.Equal(t, "Bearer "+actualToken, value)
	}

	shellCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	result, err := bound.ExecShellCommandWithOptions(shellCtx, "shell-session-1", "printf shell-command-ok", sandbox.ShellExecOptions{WorkDir: "/workspace"})
	require.NoError(t, err)
	require.Equal(t, "shell-command-ok", result.Stdout)

	// Inspect persisted/effective config, provider create and exec requests,
	// and command output as complete serialized observations.
	persisted, err := sandboxRepo.GetByID(context.Background(), 1, "cfg-shell-boundary")
	require.NoError(t, err)
	observation, err := json.Marshal(map[string]any{
		"persisted_config": persisted.Config,
		"resolved_config":  resolvedShellConfig,
		"creates":          client.creates,
		"execs":            client.execs,
		"output":           result.Stdout,
	})
	require.NoError(t, err)
	require.NotContains(t, string(observation), actualToken)
	require.Contains(t, string(observation), operatorShellProbe)
	require.Len(t, client.creates, 1)
	require.Equal(t, operatorShellProbe, client.creates[0].EnvVars["OPERATOR_SHELL_VALUE"])
	require.Len(t, client.execs, 2, "session workspace bootstrap and requested command both cross the Shell exec boundary")
	for _, request := range client.execs {
		require.NotContains(t, request.Env, managedDeliveryProbe)
	}
}

func TestManagedCredentialWrongScopeCannotDispatch(t *testing.T) {
	cases := []struct {
		name    string
		tenant  uint64
		owner   string
		service string
	}{
		{name: "tenant", tenant: 2, owner: "u1", service: "github"},
		{name: "owner", tenant: 1, owner: "u2", service: "github"},
		{name: "service", tenant: 1, owner: "u1", service: "other-service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authorizationRecorder{RoundTripper: http.DefaultTransport}
			delivery := newRecoveryEnvWithHTTPClient(t, &http.Client{Transport: auth}, true)
			id := delivery.seedApprovedDelivery(t)
			delivery.replaceManagedTokenWithWrongScope(t, tc.tenant, tc.owner, tc.service)
			auth.reset()
			status, state := delivery.dispatchState(t, id)
			require.NotEqual(t, "delivered", state.State)
			require.NotEqual(t, http.StatusOK, status)
			require.Empty(t, auth.snapshot(), "scope mismatch must fail before an outbound request with Authorization")
			require.Empty(t, delivery.github.snapshotWrites(), "scope mismatch must produce no provider write")
		})
	}
}
