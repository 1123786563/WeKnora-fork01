package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// The T15 central wiring regressions: the session View's default-seat
// selection for web sessions honors the injected selector, falls back to
// the legacy newest-version rule for selector errors, ignores the selector
// for non-web kinds, and an unwired (nil) selector keeps the recorded
// legacy behavior.

func newSessionViewEnvWithSelector(t *testing.T, kind string, selector DefaultVersionSelector) (*CraftSessionService, string) {
	t.Helper()
	env := newCraftSessionEnv(t, openGate)
	// A REAL craft session of the requested kind, created through the
	// production Create path (sessions row + registration + workspace).
	// View's read path is the strict owner route (access unwired in this
	// env), so the caller must be the creating owner.
	ctx := craftCtx(1, "u1", "")
	ws, err := env.svc.Create(ctx, ownerScope(1, "u1", ""),
		craft.CreateRequest{RequestID: "t15-" + kind, Title: "T15 接线", Kind: kind})
	require.NoError(t, err)

	svc, err := NewCraftSessionService(CraftSessionConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(env.db),
		TemporaryDocs: env.docs, Files: &fakeCraftFiles{blobs: map[string][]byte{}},
		Models: env.models, Gate: openGate, DefaultVersionSelector: selector,
	})
	require.NoError(t, err)
	return svc, ws.SessionID
}

func TestCraftSessionViewUsesWebDefaultVersionSelectorForWebKind(t *testing.T) {
	pinned := craft.Version{ID: "v-pinned", Kind: craft.KindWeb}
	calls := 0
	svc, sessionID := newSessionViewEnvWithSelector(t, craft.KindWeb, func(context.Context, craft.Scope) (craft.Version, bool, error) {
		calls++
		return pinned, true, nil
	})
	view, err := svc.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID})
	require.NoError(t, err)
	require.Equal(t, 1, calls, "the web view must consult the T15 selector exactly once")
	require.NotNil(t, view.CurrentVersion)
	require.Equal(t, "v-pinned", view.CurrentVersion.ID, "the four-check-selected version takes the default seat")
}

func TestCraftSessionViewFallsBackOnSelectorErrorOrEmpty(t *testing.T) {
	// Selector error → the read still succeeds (degraded to legacy rule).
	failing, failingSession := newSessionViewEnvWithSelector(t, craft.KindWeb, func(context.Context, craft.Scope) (craft.Version, bool, error) {
		return craft.Version{}, false, context.Canceled
	})
	view, err := failing.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: failingSession})
	require.NoError(t, err, "a selector error degrades to the legacy rule, never fails the read")

	// No qualifying version (ok=false) → CurrentVersion stays nil: no
	// unverified version sneaks into the default seat.
	none, noneSession := newSessionViewEnvWithSelector(t, craft.KindWeb, func(context.Context, craft.Scope) (craft.Version, bool, error) {
		return craft.Version{}, false, nil
	})
	view, err = none.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: noneSession})
	require.NoError(t, err)
	require.Nil(t, view.CurrentVersion, "without a four-check version the seat stays empty")
}

func TestCraftSessionViewIgnoresSelectorForNonWebKind(t *testing.T) {
	calls := 0
	svc, sessionID := newSessionViewEnvWithSelector(t, craft.KindDocument, func(context.Context, craft.Scope) (craft.Version, bool, error) {
		calls++
		return craft.Version{ID: "v-should-not-win"}, true, nil
	})
	view, err := svc.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID})
	require.NoError(t, err)
	require.Zero(t, calls, "non-web kinds never consult the web selector")
	// The legacy rule governs: an empty history keeps an empty seat.
	require.Nil(t, view.CurrentVersion)
}

func TestCraftSessionViewNilSelectorKeepsLegacyBehavior(t *testing.T) {
	svc, sessionID := newSessionViewEnvWithSelector(t, craft.KindWeb, nil)
	view, err := svc.View(craftCtx(1, "u1", ""), craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID})
	require.NoError(t, err)
	require.Nil(t, view.CurrentVersion, "an empty history keeps an empty seat under the unwired selector")
}
