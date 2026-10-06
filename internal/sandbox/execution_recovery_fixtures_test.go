package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

// fixtureRemoteClient serves a fixed provider inventory through List; every
// other RemoteSandboxClient method is unreachable from ObserveInstance.
type fixtureRemoteClient struct {
	RemoteSandboxClient
	provider  RemoteProvider
	inventory []RemoteSandboxSummary
}

func (f *fixtureRemoteClient) Provider() RemoteProvider { return f.provider }
func (f *fixtureRemoteClient) Capabilities() RemoteSandboxCapabilities {
	return RemoteSandboxCapabilities{SupportsListSandboxes: true, SupportsReconnect: true}
}
func (*fixtureRemoteClient) Health(context.Context) error { return nil }
func (f *fixtureRemoteClient) List(context.Context, RemoteListFilter) ([]RemoteSandboxSummary, error) {
	return f.inventory, nil
}

func fixtureManager(t *testing.T, provider RemoteProvider, inventory []RemoteSandboxSummary,
	binding *SessionSandboxBinding,
) *SessionBoundManager {
	t.Helper()
	store := NewMemorySessionSandboxBindingStore()
	if binding != nil {
		key := SessionSandboxKey{TenantID: binding.TenantID, SessionID: binding.SessionID}
		created, err := store.Create(context.Background(), key, *binding)
		require.NoError(t, err)
		require.True(t, created)
	}
	mgr, err := NewSessionBoundManager(SessionBoundManagerConfig{
		Client:          &fixtureRemoteClient{provider: provider, inventory: inventory},
		Store:           store,
		Checker:         fixtureChecker{},
		ConfigID:        "cfg-1",
		SkipHealthProbe: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Cleanup(context.Background()) })
	return mgr
}

func fixtureBinding() SessionSandboxBinding {
	return SessionSandboxBinding{
		Version: SessionSandboxBindingVersion, Provider: SandboxTypeDocker,
		TenantID: 1, SessionID: "s1", SandboxID: "sbx-1",
		TemplateID: "tpl", ConfigID: "cfg-1", Generation: "g1", CreatedAt: time.Now(),
	}
}

// TestObserveInstanceFixtureStates is the sandbox alive/lost/destroyed row:
// alive continues, a binding whose instance the provider no longer lists is
// unknown (parks), and a destroyed instance with a stale binding or a missing
// binding is missing (never fabricated into success).
func TestObserveInstanceFixtureStates(t *testing.T) {
	ctx := types.WithSandboxTenantID(context.Background(), 1)
	ref := ExecutionRef{
		TenantID: 1, SessionID: "s1", Provider: string(SandboxTypeDocker),
		ConfigID: "cfg-1", InstanceID: "sbx-1", Generation: "g1", TaskID: "call-1",
		WorkspaceID: "/w",
	}
	require.NoError(t, ref.Validate())

	t.Run("alive", func(t *testing.T) {
		inventory := []RemoteSandboxSummary{{
			ID: "sbx-1", State: RemoteStateRunning,
			Metadata: map[string]string{
				remoteMetadataTenantID: "1", remoteMetadataSessionID: "s1",
			},
		}}
		mgr := fixtureManager(t, SandboxTypeDocker, inventory, ptrBinding(fixtureBinding()))
		obs, err := mgr.ObserveInstance(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, "running", obs.State)
	})

	t.Run("lost-instance-unknown", func(t *testing.T) {
		// Binding exists but the provider list does not include the instance:
		// peek reports bound-unknown, which must surface as unknown so the
		// durable run parks instead of guessing.
		inventory := []RemoteSandboxSummary{}
		mgr := fixtureManager(t, SandboxTypeDocker, inventory, ptrBinding(fixtureBinding()))
		obs, err := mgr.ObserveInstance(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, "unknown", obs.State)
	})

	t.Run("destroyed-missing", func(t *testing.T) {
		// The instance id changed underneath the run (generation mismatch):
		// the binding no longer matches and the observation is missing.
		inventory := []RemoteSandboxSummary{{
			ID: "sbx-2", State: RemoteStateRunning,
			Metadata: map[string]string{
				remoteMetadataTenantID: "1", remoteMetadataSessionID: "s1",
			},
		}}
		binding := fixtureBinding()
		binding.Generation = "g2"
		binding.SandboxID = "sbx-2"
		mgr := fixtureManager(t, SandboxTypeDocker, inventory, ptrBinding(binding))
		obs, err := mgr.ObserveInstance(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, "missing", obs.State)
	})

	t.Run("no-binding-missing", func(t *testing.T) {
		inventory := []RemoteSandboxSummary{{
			ID: "sbx-1", State: RemoteStateRunning,
			Metadata: map[string]string{
				remoteMetadataTenantID: "1", remoteMetadataSessionID: "s1",
			},
		}}
		mgr := fixtureManager(t, SandboxTypeDocker, inventory, nil)
		obs, err := mgr.ObserveInstance(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, "missing", obs.State)
	})
}

func ptrBinding(b SessionSandboxBinding) *SessionSandboxBinding {
	result := b
	return &result
}

// fixtureChecker answers that the session row exists; ObserveInstance only
// needs existence, not any session content.
type fixtureChecker struct{}

func (fixtureChecker) SessionExists(context.Context, SessionSandboxKey) (bool, error) {
	return true, nil
}

// TestObserveInstanceNeverImportsInstanceLivenessAsTaskResult pins that a
// live instance never fabricates a task-level success: the observation
// carries no result payload even when the sandbox is running.
func TestObserveInstanceNeverImportsInstanceLivenessAsTaskResult(t *testing.T) {
	ctx := types.WithSandboxTenantID(context.Background(), 1)
	inventory := []RemoteSandboxSummary{{
		ID: "sbx-1", State: RemoteStateRunning,
		Metadata: map[string]string{
			remoteMetadataTenantID: "1", remoteMetadataSessionID: "s1",
		},
	}}
	mgr := fixtureManager(t, SandboxTypeDocker, inventory, ptrBinding(fixtureBinding()))
	obs, err := mgr.ObserveInstance(ctx, ExecutionRef{
		TenantID: 1, SessionID: "s1", Provider: string(SandboxTypeDocker),
		ConfigID: "cfg-1", InstanceID: "sbx-1", Generation: "g1", TaskID: "call-1",
		WorkspaceID: "/w",
	})
	require.NoError(t, err)
	require.False(t, CanImportObservation(obs), "instance liveness must never import as a task result")
}
