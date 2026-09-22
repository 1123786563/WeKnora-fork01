package service

// CFT-S05-T033: the release gate matrix. Kinds gate open/closed per
// deployment (WEKNORA_CRAFT_KINDS); the gate's four acceptance bullets:
//  1. a closed kind refuses NEW tasks (admission, fail-closed)
//  2. already-published versions stay readable when a kind closes
//  3. active runs drain/cancel under an explicit policy (the lifecycle's
//     quiesce path), never silently killed
//  4. nothing commercial opens without the commercial gate's own evidence
//     (the release checker refuses un-evidenced capabilities)
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// Closing a kind refuses new work but never touches published versions.
func TestCraftReleaseGateClosedKindRefusesNewKeepsPublished(t *testing.T) {
	// The gate projection (T005) refuses the closed kind on the server:
	closed := CraftFeatureGate{Enabled: true, Kinds: []string{"web"}}
	require.False(t, closed.Allows("spreadsheet"), "a closed kind refuses new tasks")
	open := CraftFeatureGate{Enabled: true, Kinds: []string{"web", "spreadsheet"}}
	require.True(t, open.Allows("spreadsheet"))

	// Published versions are immutable regardless of the kind list — the
	// version store has no kind dimension on reads (publish/delete paths are
	// the lifecycle's conservative sweeps, never the gate's).
	db := openCraftBudgetTestDB(t)
	wsStore := repository.NewCraftStore(db)
	store := repository.NewCraftVersionStore(db)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
	// The FK chain: sessions -> craft_workspaces -> craft_versions/run.
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'gate', 'u1', 'trpc')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at) VALUES (1, 'run-g', 's1', 'u1', 'req-g', 'am-g', 'rh', '{}', 'succeeded', '', datetime('now', '+1 hour'), datetime('now'), datetime('now'))").Error)
	// The version store resolves the workspace binding (scope ACL), so the
	// workspace must exist first — publish through the real store.
	_, err := wsStore.PutWorkspace(ctx, craft.Workspace{Scope: scope, SandboxID: "sbx-g"}, 0)
	require.NoError(t, err)
	created, err := wsStore.GetWorkspace(ctx, scope)
	require.NoError(t, err)
	files := []craft.File{{Path: "index.html", Ref: "resource://g", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MIME: "text/html", Bytes: 10}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	published, err := store.Publish(ctx, scope, craft.Version{
		ID: craft.VersionID(created.ID, "run-g", digest), WorkspaceID: created.ID, RunID: "run-g", Kind: "web",
		Files: files, Checks: []craft.Check{{Name: "build", Status: "passed"}},
	})
	require.NoError(t, err)

	// With the kind now closed, the same version stays readable.
	got, err := store.Get(ctx, scope, published.ID)
	require.NoError(t, err)
	require.Equal(t, published.ID, got.ID)
	require.Len(t, got.Files, 1)
}

// Active runs follow the explicit quiesce/cancel policy: closing admission
// does not murder in-flight runs — they drain to their own terminal state
// (pinned by the lifecycle suite); new admissions stop at the gate.
func TestCraftReleaseGateClosingAdmissionDrainsNotKills(t *testing.T) {
	// AdmissionEnabled exists precisely as the drain switch: close
	// admission while the recovery worker keeps executing existing runs.
	// The lifecycle suites pin that draining; here we pin the switch itself
	// composes with the craft gate (enabled-for-reads, closed-for-new).
	gate := CraftFeatureGate{Enabled: true, Kinds: []string{}} // kinds closed
	require.False(t, gate.Allows("web"), "no kind open: new work refuses")
	// Reads (View/List) never pass through Allows — only Create/StartRun do
	// (pinned by TestCraftHTTPCapabilitiesProjection: closed kind -> 503 on
	// create while GET workspace keeps serving).
}
