// Package boottest holds whole-container boot smoke tests in their own test
// binary: BuildContainer mutates process-global state (SSRF policy, engine
// registry), so running it beside the other container tests corrupts them.
package boottest

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/container"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"
)

// TestBuildContainerBootsLite assembles the REAL BuildContainer headlessly
// (temp SQLite + Lite mode, no Redis, no external services). The 2026-09-30
// merge batch twice broke server boot purely by dig call ordering —
// provideAgentSecurity eager-Invoked before its late-registered providers,
// and the knowledge module provided after the task servers that resolve it —
// and no package test noticed because none ever ran BuildContainer.
func TestBuildContainerBootsLite(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	// config.yaml and migrations/sqlite resolve relative to the repo root,
	// exactly like `go run ./cmd/server`.
	t.Chdir(root)

	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "boot.db"))
	t.Setenv("REDIS_ADDR", "") // Lite: sync task executor, no asynq
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	t.Setenv("AUTO_MIGRATE", "true")

	c := container.BuildContainer(dig.New())

	// provideAgentSecurity runs three eager Invokes; at the regressed call
	// position the session handler was not registered yet, so its dig.In
	// optional resolved nil and the claims wiring silently no-opped.
	var session *sessionhandler.Handler
	require.NoError(t, c.Invoke(func(h *sessionhandler.Handler) { session = h }))
	require.NotNil(t, session)
	claims := reflect.ValueOf(session).Elem().FieldByName("agentChatTurnClaimStore")
	require.False(t, claims.IsNil(), "wireAgentSecuritySessionClaims must observe a live session handler")

	var cleaner interfaces.ResourceCleaner
	require.NoError(t, c.Invoke(func(cl interfaces.ResourceCleaner) { cleaner = cl }))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, err := range cleaner.Cleanup(ctx) {
		t.Logf("cleanup: %v", err)
	}
}
