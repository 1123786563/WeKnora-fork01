package container

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// TestCraftSnapshotSourceLiveSmoke drives the local snapshot source's HTTP
// surface (Status/Messages/CreateSession — the exact calls Quiescent,
// ExportSessionData and RestoreGeneration issue) against a real pinned
// opencode serve. Env-gated like the C04 process matrix: set
// CRAFT_SNAPSHOT_LIVE_SMOKE=http://127.0.0.1:PORT of a running serve.
func TestCraftSnapshotSourceLiveSmoke(t *testing.T) {
	base := os.Getenv("CRAFT_SNAPSHOT_LIVE_SMOKE")
	if base == "" {
		t.Skip("live smoke not requested (set CRAFT_SNAPSHOT_LIVE_SMOKE to a running pinned serve URL)")
	}
	if _, err := url.Parse(base); err != nil {
		t.Fatalf("bad serve URL: %v", err)
	}
	client, err := opencode.NewClient(base, nil)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessionID, err := client.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	status, err := client.Status(ctx, sessionID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != "idle" {
		t.Fatalf("fresh session status = %q, want idle", status)
	}
	messages, err := client.Messages(ctx, sessionID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	// A session with no exchange has no restorable chain: the C05 rule that
	// files alone are never a complete snapshot.
	if len(messages) != 0 {
		t.Fatalf("fresh session already carries %d messages", len(messages))
	}
	if _, err := craft.SessionDigest(nil); err == nil {
		t.Fatal("empty chain accepted a digest")
	}
	// The missing-session read is the provider-unsupported path of
	// RestoreGeneration: it must answer an error, never an empty success.
	if _, err := client.Messages(ctx, "ses_does_not_exist"); err == nil {
		t.Fatal("missing session read succeeded")
	}
	fmt.Printf("[live-smoke] session=%s status=idle messages=0 ok\n", sessionID)
}
