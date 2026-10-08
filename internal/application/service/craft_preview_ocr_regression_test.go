package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// previewNoopVersions / previewNoopFiles satisfy the constructor's non-nil
// requirements without behavior.
type previewNoopVersions struct{ craft.VersionStore }

func (previewNoopVersions) Get(context.Context, craft.Scope, string) (craft.Version, error) {
	return craft.Version{}, craft.ErrNotFound
}

type previewNoopFiles struct{ interfaces.FileService }

// The OCR round regression set: shared docker client (no per-call dial),
// host default-port normalization, condition-only refusal texts (no
// "<nil>" tail), and the allowlist-first lookup order.

func TestCraftPreviewDockerClientIsSharedPerHost(t *testing.T) {
	craftPreviewDockerClients.mu.Lock()
	saved := craftPreviewDockerClients.clients
	craftPreviewDockerClients.clients = nil
	craftPreviewDockerClients.mu.Unlock()
	t.Cleanup(func() {
		craftPreviewDockerClients.mu.Lock()
		craftPreviewDockerClients.clients = saved
		craftPreviewDockerClients.mu.Unlock()
	})

	first, err := craftPreviewDockerClient("unix:///nonexistent/preview-a.sock")
	require.NoError(t, err)
	second, err := craftPreviewDockerClient("unix:///nonexistent/preview-a.sock")
	require.NoError(t, err)
	other, err := craftPreviewDockerClient("unix:///nonexistent/preview-b.sock")
	require.NoError(t, err)
	require.Same(t, first, second, "the same daemon host must reuse one cached client (no per-request dial)")
	require.NotSame(t, first, other, "a different daemon host gets its own client")
}

func TestCraftPreviewRefusalTextsCarryNoNilTail(t *testing.T) {
	// A condition-only refusal (binding nil while Get succeeded) must not
	// end with ": <nil>".
	store := &previewBindingStoreFake{}
	checker := CraftPreviewDockerNetworkChecker{
		Bindings: store, Loader: previewLoaderFake{}, Global: sandbox.DefaultConfig(),
		Inspector: previewInspectorFake{},
	}
	err := checker.CheckPreviewNoEgress(context.Background(), craft.Scope{TenantID: 1, SessionID: "s1"})
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.NotContains(t, err.Error(), "<nil>", "condition-only refusals must not carry a nil error tail")

	// A real error still surfaces its text.
	store.err = errors.New("redis down")
	err = checker.CheckPreviewNoEgress(context.Background(), craft.Scope{TenantID: 1, SessionID: "s1"})
	require.ErrorIs(t, err, craft.ErrUnsupported)
	require.Contains(t, err.Error(), "redis down")
}

func TestCraftPreviewAcceptsPreviewHostStripsDefaultPort(t *testing.T) {
	svc := NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
		AppOrigin:     "https://app.example.test",
		PreviewOrigin: "https://preview.example.test",
	})
	require.True(t, svc.AcceptsPreviewHost("preview.example.test"), "bare host matches as before")
	require.True(t, svc.AcceptsPreviewHost("preview.example.test:443"), "explicit https default port is normalized away (RFC 7230)")
	require.False(t, svc.AcceptsPreviewHost("preview.example.test:8443"), "non-default ports still refuse")
	require.False(t, svc.AcceptsPreviewHost("app.example.test"), "the app host never redeems preview tickets")

	// IPv6 literals keep their brackets after port stripping, so the
	// bracketed config form and the bracketed request form compare equal in
	// both directions.
	v6 := NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
		AppOrigin:     "https://app.example.test",
		PreviewOrigin: "https://[2001:db8::1]",
	})
	require.True(t, v6.AcceptsPreviewHost("[2001:db8::1]"), "bare bracketed IPv6 matches")
	require.True(t, v6.AcceptsPreviewHost("[2001:db8::1]:443"), "explicit default port on an IPv6 literal is normalized (brackets preserved)")
	require.False(t, v6.AcceptsPreviewHost("[2001:db8::2]"), "a different IPv6 literal refuses")

	v6Port := NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
		AppOrigin:     "https://app.example.test",
		PreviewOrigin: "https://[2001:db8::1]:443",
	})
	require.True(t, v6Port.AcceptsPreviewHost("[2001:db8::1]"), "config with explicit default port matches the bare bracketed form")
}

// TestCraftPreviewLookupAnswersOutsideAllowlistBeforeIO pins the reordered
// lookup: a capability-holder requesting a path outside the frozen manifest
// gets its 404 WITHOUT the access checker or the no-egress door ever being
// consulted (favicon.ico probes must not pay membership+binding+inspect
// round trips). A manifest path still runs both doors.
func TestCraftPreviewLookupAnswersOutsideAllowlistBeforeIO(t *testing.T) {
	access := &previewCountingAccess{allowed: true}
	previewCountingCheckerCalls = 0
	granted := NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
		AppOrigin:                  "https://app.example.test",
		PreviewOrigin:              "https://preview.example.test",
		AccessChecker:              access,
		NetworkChecker:             previewCountingChecker{},
		BrowserNavigationProtected: true,
	})
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	// Install a capability whose frozen manifest holds only index.html.
	granted.mu.Lock()
	granted.caps["capdigest"] = craftPreviewGrant{
		scope: scope, versionID: "v1", expiresAt: granted.config.Now().Add(time.Minute),
	}
	// The shared per-version allowlist cache backs the O(1) 404 door.
	granted.versionFiles["v1"] = map[string]struct{}{"index.html": {}}
	granted.mu.Unlock()

	_, _, _, err := granted.lookup(context.Background(), "capdigest", "favicon.ico")
	require.ErrorIs(t, err, craft.ErrNotFound)
	require.Zero(t, access.calls, "the allowlist 404 precedes the membership door")
	require.Zero(t, previewCountingCheckerCalls, "the allowlist 404 precedes the no-egress door")

	_, _, _, err = granted.lookup(context.Background(), "capdigest", "index.html")
	require.NotNil(t, err, "a manifest path proceeds past the allowlist into the doors and the version store")
	require.Positive(t, access.calls, "manifest paths still run the access door")
	require.Positive(t, previewCountingCheckerCalls, "manifest paths still run the no-egress door")
}

// previewBindingStoreFake returns a canned binding/error pair.
type previewBindingStoreFake struct {
	sandbox.SessionSandboxBindingStore
	binding *sandbox.SessionSandboxBinding
	err     error
}

func (f *previewBindingStoreFake) Get(context.Context, sandbox.SessionSandboxKey) (*sandbox.SessionSandboxBinding, error) {
	return f.binding, f.err
}

type previewLoaderFake struct {
	sandbox.TenantSandboxConfigLoader
}

type previewInspectorFake struct{}

func (previewInspectorFake) NetworkMode(context.Context, *sandbox.Config, string) (string, error) {
	return "none", nil
}

type previewCountingAccess struct {
	craft.TaskAccessChecker
	calls   int
	allowed bool
}

func (a *previewCountingAccess) CheckTaskAccess(context.Context, craft.Scope, craft.TaskAction) error {
	a.calls++
	if a.allowed {
		return nil
	}
	return craft.ErrForbidden
}

var previewCountingCheckerCalls int

type previewCountingChecker struct {
	CraftPreviewNetworkChecker
}

func (previewCountingChecker) CheckPreviewNoEgress(context.Context, craft.Scope) error {
	previewCountingCheckerCalls++
	return nil
}

// TestCraftPreviewOriginPortSpellingOverlapIsRefused is the round-3 OCR
// security regression: a preview origin that differs from the app origin
// only by default-port spelling (https://app:443 vs https://app) used to
// pass the raw-string distinctness check while trimDefaultPort made the
// hosts equal — putting the unauthenticated preview routes on the app
// origin. The constructor must refuse the shared hostname outright.
func TestCraftPreviewOriginPortSpellingOverlapIsRefused(t *testing.T) {
	require.Panics(t, func() {
		NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
			AppOrigin:     "https://app.example.test",
			PreviewOrigin: "https://app.example.test:443",
			AccessChecker: &previewCountingAccess{allowed: true},
		})
	}, "a preview origin sharing the app hostname must abort assembly")
	require.Panics(t, func() {
		NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
			AppOrigin:     "https://app.example.test:8443",
			PreviewOrigin: "https://app.example.test:443",
			AccessChecker: &previewCountingAccess{allowed: true},
		})
	}, "port-only separation shares Host-only cookies and must abort assembly")
}

// TestCraftPreviewAcceptsHostNeverServesTheAppHostname guards the serving
// side: even a legitimately distinct preview origin never answers a request
// whose host is the APP origin's hostname.
func TestCraftPreviewAcceptsHostNeverServesTheAppHostname(t *testing.T) {
	svc := NewCraftPreviewService(previewNoopVersions{}, previewNoopFiles{}, nil, CraftPreviewConfig{
		AppOrigin:     "https://app.example.test:443",
		PreviewOrigin: "https://preview.example.test",
		AccessChecker: &previewCountingAccess{allowed: true},
	})
	require.True(t, svc.Enabled())
	require.True(t, svc.AcceptsPreviewHost("preview.example.test"))
	require.False(t, svc.AcceptsPreviewHost("app.example.test"),
		"the app hostname must never reach unauthenticated preview routes")
	require.False(t, svc.AcceptsPreviewHost("app.example.test:443"))
}
