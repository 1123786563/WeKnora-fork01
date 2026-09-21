//go:build lago_integration

// Tagged real-Lago integration evidence (T05, #77). The build tag keeps
// this out of every normal suite run — unit tests stay Docker-free. The
// test is env-gated on LAGO_INTEGRATION_BASE_URL + LAGO_INTEGRATION_API_KEY
// (operator-owned secrets; never committed) and skips otherwise, so a
// missing stack records blocked-env instead of failing or faking a pass.
package commercialplatform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// lockedRelease reads the deployment-pinned release identity from
// deploy/lago/images.lock.json — the SAME lock the operator stack boots
// from — so the assertion compares the adapter answer against the pin, not
// against a hardcoded string drifting from the deployment.
func lockedRelease(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "deploy", "lago", "images.lock.json")
	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read image lock: %v", err)
	}
	var lock struct {
		Release string `json:"release"`
	}
	if err := json.Unmarshal(blob, &lock); err != nil || lock.Release == "" {
		t.Fatalf("parse image lock release: %v (%s)", err, path)
	}
	return lock.Release
}

// TestLagoAdapterIntegration: against the REAL pinned Lago stack, the
// adapter reads a ready readiness snapshot whose Release equals the lock's
// release (v1.53.0). Version identity is deployment config — never text
// parsed from the provider response.
func TestLagoAdapterIntegration(t *testing.T) {
	baseURL := os.Getenv("LAGO_INTEGRATION_BASE_URL")
	apiKey := os.Getenv("LAGO_INTEGRATION_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("blocked-env: LAGO_INTEGRATION_BASE_URL / LAGO_INTEGRATION_API_KEY not set (operator stack not provided)")
	}
	release := lockedRelease(t)
	p := NewLagoAdapter(Config{
		Provider: ProviderLago,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Release:  release,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	snap, err := p.ReadSnapshot(ctx, commercial.SnapshotQuery{Kind: commercial.SnapshotKindReadiness})
	if err != nil {
		t.Fatalf("readiness against the real stack: %v", err)
	}
	if snap.Readiness == nil || snap.Readiness.State != commercial.ReadinessReady {
		t.Fatalf("real stack must answer ready, got %+v", snap.Readiness)
	}
	if snap.Readiness.Release != release {
		t.Fatalf("release = %q, want the locked %q", snap.Readiness.Release, release)
	}
	if snap.Readiness.CheckedAt.IsZero() || time.Since(snap.Readiness.CheckedAt) > time.Minute {
		t.Fatalf("CheckedAt must be the check time, got %v", snap.Readiness.CheckedAt)
	}
	if snap.Readiness.Reason != "" {
		t.Fatalf("ready snapshot must carry an empty reason, got %q", snap.Readiness.Reason)
	}
}
