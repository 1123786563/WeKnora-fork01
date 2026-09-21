package repository

// CFT-S02-T016: the workspace guard matrix over the REAL store — one
// serialized writer per workspace, revision CAS, fence/generation staleness
// and read paths that never hold the write slot. (Placed beside the store:
// every assertion is store behavior; the service assembly consumes it.)
import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func guardWorkspace(t *testing.T, store craft.Store) craft.Workspace {
	t.Helper()
	created, err := store.PutWorkspace(context.Background(), craft.Workspace{
		Scope: craftTestScope(), SandboxID: "sbx-guard", Generation: "0",
	}, 0)
	require.NoError(t, err)
	return created
}

// Concurrent writes accept exactly ONE winner; the loser gets a conflict
// and the binding stays single.
func TestCraftWorkspaceGuardSingleWriter(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	scope := craftTestScope()
	const writers = 4
	results := make([]craft.Workspace, writers)
	errs := make([]error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = store.PutWorkspace(context.Background(), craft.Workspace{
				Scope: scope, SandboxID: "sbx-race-guard", Generation: "0",
			}, 0)
		}(i)
	}
	wg.Wait()
	winners := 0
	for i, err := range errs {
		if err == nil {
			winners++
			require.NotEmpty(t, results[i].ID)
			continue
		}
		require.ErrorIs(t, err, craft.ErrConflict)
	}
	require.Equal(t, 1, winners)
}

// A stale revision fails the CAS and leaves NO side effects behind — no
// workspace mutation and no version rows (nothing was "written").
func TestCraftWorkspaceGuardStaleRevisionWritesNothing(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	ctx := context.Background()
	ws := guardWorkspace(t, store)
	versions := NewCraftVersionStore(db)

	_, err := store.PutWorkspace(ctx, craft.Workspace{
		ID: ws.ID, Scope: ws.Scope, SandboxID: "sbx-stale-write", Generation: "stale",
	}, ws.Revision+5)
	require.ErrorIs(t, err, craft.ErrConflict)

	var versionRows int64
	require.NoError(t, db.Table("craft_versions").Where("workspace_id = ?", ws.ID).Count(&versionRows).Error)
	require.Zero(t, versionRows, "a failed CAS leaves no version behind")

	kept, err := store.GetWorkspace(ctx, ws.Scope)
	require.NoError(t, err)
	require.Equal(t, "sbx-guard", kept.SandboxID, "the workspace row is untouched")
	require.EqualValues(t, 1, kept.Revision)
	_ = versions
}

// A superseded fence (the old worker's epoch) can neither prepare a
// delegation nor record a result — so it can never reach a publish.
func TestCraftWorkspaceGuardStaleFenceCannotPublish(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	ctx := context.Background()
	fence := seedCraftRun(t, db, "r-guard-1", "c-guard")
	ws := guardWorkspace(t, store)

	// Prepare once with the live fence, then bump the run's epoch: the SAME
	// worker identity is now stale.
	_, err := store.PrepareTask(ctx, craft.Task{
		ToolCallID: "c-guard", Prompt: "guard", RequestHash: "rh-guard",
		Scope: craftTestScope(), Fence: fence, WorkspaceID: ws.ID,
		Deadline: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_runs SET epoch = epoch + 1 WHERE tenant_id = ? AND run_id = ?",
		fence.TenantID, fence.RunID).Error)

	// A NEW tool call under the stale fence is refused at the plan boundary
	// already (lease lost) — and preparing through the store conflicts too.
	_, planErr := NewAgentRunStore(db).EnsureToolPlan(ctx, fence, craftToolPlan("c-guard-2"))
	require.Error(t, planErr, "a stale fence cannot even plan a tool call (lease lost)")
	_, err = store.PrepareTask(ctx, craft.Task{
		ToolCallID: "c-guard-2", Prompt: "guard-2", RequestHash: "rh-guard-2",
		Scope: craftTestScope(), Fence: fence, WorkspaceID: ws.ID,
		Deadline: time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
	})
	require.ErrorIs(t, err, craft.ErrConflict, "a stale worker cannot prepare (thus cannot publish)")

	// And it cannot record a result either.
	err = store.SaveResult(ctx, fence, craft.Result{TaskID: "dlg-guard", Status: "succeeded"})
	require.Error(t, err, "a stale fence cannot store a result")

	var versionRows int64
	require.NoError(t, db.Table("craft_versions").Where("workspace_id = ?", ws.ID).Count(&versionRows).Error)
	require.Zero(t, versionRows, "nothing was published")
}

// Reading an old version never holds the write slot: a subsequent CAS
// write on the same workspace still succeeds.
func TestCraftWorkspaceGuardReadDoesNotHoldWriteSlot(t *testing.T) {
	db := openCraftDB(t)
	store := NewCraftStore(db)
	versions := NewCraftVersionStore(db)
	ctx := context.Background()
	ws := guardWorkspace(t, store)
	digest, idErr := craft.ManifestDigest([]craft.File{{Path: "index.html", Ref: "resource://g", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MIME: "text/html", Bytes: 10}})
	require.NoError(t, idErr)
	published, err := versions.Publish(ctx, ws.Scope, craft.Version{
		ID: craft.VersionID(ws.ID, "r-guard-r", digest), WorkspaceID: ws.ID, RunID: "r-guard-r", Kind: "web",
		Files:  []craft.File{{Path: "index.html", Ref: "resource://g", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MIME: "text/html", Bytes: 10}}, //nolint:lll // 预存长测试数据行
		Checks: []craft.Check{{Name: "build", Status: "passed"}},
	})
	require.NoError(t, err)

	// View the old version — a pure read.
	_, err = versions.Get(ctx, ws.Scope, published.ID)
	require.NoError(t, err)

	// The write slot is free: a correct-revision CAS still succeeds.
	updated, err := store.PutWorkspace(ctx, craft.Workspace{
		ID: ws.ID, Scope: ws.Scope, SandboxID: "sbx-guard-2", Generation: "1",
	}, 1)
	require.NoError(t, err, "a read must never hold the workspace write slot")
	require.EqualValues(t, 2, updated.Revision)
}
