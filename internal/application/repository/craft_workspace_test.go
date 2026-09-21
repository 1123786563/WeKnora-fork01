package repository

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openCraftDB applies the real SQLite migrations (000041_craft included) and
// seeds the tenant/user/session fixtures the craft foreign keys require. A
// subtest named "postgres" runs the same assertions against an isolated
// PostgreSQL schema when TRPC_TEST_POSTGRES_DSN is set; without it the
// PostgreSQL acceptance stays recorded as NOT VERIFIED.
func openCraftDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openRunTestDB(t)
}

func craftTestScope() craft.Scope {
	return craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}
}

func craftToolPlan(callID string) agentruntime.ToolPlan {
	return agentruntime.ToolPlan{
		CallID: callID, Name: "craft_delegate", Identity: "craft",
		ArgsHash: "args-" + callID, Args: json.RawMessage(`{"kind":"craft"}`),
	}
}

// seedCraftRun admits, claims and journal-plans a real run so delegation rows
// satisfy the agent_runs and agent_tool_calls foreign keys through the
// production APIs instead of raw inserts.
func seedCraftRun(t *testing.T, db *gorm.DB, runID, callID string) agentruntime.Fence {
	t.Helper()
	ctx := context.Background()
	runs := NewAgentRunStore(db)
	_, err := runs.Admit(ctx, agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          "s1",
		UserID:             "u1",
		RequestID:          "craft-" + runID,
		AssistantMessageID: "craft-a-" + runID,
		RequestHash:        "craft-h-" + runID,
		Snapshot:           json.RawMessage(`{"version":1,"craft":true}`),
		UserMessage:        json.RawMessage(`{"role":"user","content":"make a site"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := runs.Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: runID}, "craft-worker", time.Hour)
	require.NoError(t, err)
	_, err = runs.EnsureToolPlan(ctx, fence, craftToolPlan(callID))
	require.NoError(t, err)
	return fence
}

func putCraftWorkspace(t *testing.T, store craft.Store) craft.Workspace {
	t.Helper()
	ws, err := store.PutWorkspace(context.Background(), craft.Workspace{
		Scope:             craftTestScope(),
		SandboxID:         "sbx-1",
		Generation:        "0",
		OpenCodeSessionID: "oc-1",
		RuntimeDigest:     "sha256:runtime",
	}, 0)
	require.NoError(t, err)
	return ws
}

// TestCraftWorkspaceConcurrentCreateProducesSingleBinding: two racing
// first-creates of the same (tenant, session) binding produce exactly one
// durable workspace; every loser observes ErrConflict.
func TestCraftWorkspaceConcurrentCreateProducesSingleBinding(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftStore(db)
			scope := craftTestScope()
			results := make([]craft.Workspace, 2)
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i := range results {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = store.PutWorkspace(context.Background(), craft.Workspace{
						Scope: scope, SandboxID: "sbx-race", Generation: "0",
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
			require.Equal(t, 1, winners, "exactly one racing creator may claim the binding")
			var bound int64
			require.NoError(t, db.Table("craft_workspaces").
				Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).
				Count(&bound).Error)
			require.EqualValues(t, 1, bound)
			got, err := store.GetWorkspace(context.Background(), scope)
			require.NoError(t, err)
			require.Equal(t, "sbx-race", got.SandboxID)
			require.EqualValues(t, 1, got.Revision)
		})
	}
}

// TestCraftWorkspaceRevisionCASAndReopen: expected=0 insert, revision CAS
// update, stale/conflict/missing guards and durability across a database
// reopen.
func TestCraftWorkspaceRevisionCASAndReopen(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftStore(db)
			ctx := context.Background()
			scope := craftTestScope()
			created, err := store.PutWorkspace(ctx, craft.Workspace{
				ID: "ws-craft-1", Scope: scope, SandboxID: "sbx-1",
			}, 0)
			require.NoError(t, err)
			require.EqualValues(t, 1, created.Revision)

			// A first-create retry against the existing binding conflicts;
			// revision updates must go through the CAS path below.
			_, err = store.PutWorkspace(ctx, craft.Workspace{
				ID: "ws-craft-1", Scope: scope, SandboxID: "sbx-stale",
			}, 0)
			require.ErrorIs(t, err, craft.ErrConflict)

			_, err = store.PutWorkspace(ctx, craft.Workspace{
				ID: "ws-craft-1", Scope: scope, SandboxID: "sbx-stale",
			}, 7)
			require.ErrorIs(t, err, craft.ErrConflict)

			updated, err := store.PutWorkspace(ctx, craft.Workspace{
				ID: "ws-craft-1", Scope: scope, SandboxID: "sbx-2", Generation: "2",
				OpenCodeSessionID: "oc-9", RuntimeDigest: "sha256:d2",
			}, 1)
			require.NoError(t, err)
			require.EqualValues(t, 2, updated.Revision)
			require.Equal(t, "sbx-2", updated.SandboxID)

			_, err = store.PutWorkspace(ctx, craft.Workspace{ID: "ws-404", Scope: scope}, 3)
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = store.PutWorkspace(ctx, craft.Workspace{Scope: craft.Scope{TenantID: 1}}, 0)
			require.ErrorIs(t, err, craft.ErrInvalidInput)

			reopened := NewCraftStore(reopenRunDB(t, db))
			got, err := reopened.GetWorkspace(ctx, scope)
			require.NoError(t, err)
			require.EqualValues(t, 2, got.Revision)
			require.Equal(t, "oc-9", got.OpenCodeSessionID)
			require.Equal(t, "sha256:d2", got.RuntimeDigest)

			_, err = reopened.GetWorkspace(ctx, craft.Scope{TenantID: 1, UserID: "u1", SessionID: "missing"})
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = reopened.GetWorkspace(ctx, craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"})
			require.ErrorIs(t, err, craft.ErrForbidden)
		})
	}
}

// TestCraftPrepareTaskIdempotencyAndConflicts: the same tool call with the
// same request replays to the same id/promptMessageID, a different request on
// the same call conflicts, and scope, tool call and fence guards hold.
func TestCraftPrepareTaskIdempotencyAndConflicts(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftStore(db)
			ctx := context.Background()
			fence := seedCraftRun(t, db, "r-craft-1", "c1")
			scope := craftTestScope()
			ws := putCraftWorkspace(t, store)

			task := craft.Task{
				ToolCallID: "c1", Prompt: "build the landing page", RequestHash: "rh-1",
				Scope: scope, Fence: fence, WorkspaceID: ws.ID,
				Inputs:       []craft.Input{{Ref: "resource://in/brief", Name: "brief.md", SHA256: "sha-in", Bytes: 12}},
				SkillDigests: []string{"sha256:skill"},
				Deadline:     time.Now().Add(30 * time.Minute).UTC().Truncate(time.Microsecond),
			}
			first, err := store.PrepareTask(ctx, task)
			require.NoError(t, err)
			require.NotEmpty(t, first.ID)
			require.NotEmpty(t, first.PromptMessageID)

			again, err := store.PrepareTask(ctx, task)
			require.NoError(t, err)
			require.Equal(t, first.ID, again.ID)
			require.Equal(t, first.PromptMessageID, again.PromptMessageID)

			conflict := task
			conflict.RequestHash, conflict.Prompt = "rh-2", "a different prompt"
			_, err = store.PrepareTask(ctx, conflict)
			require.ErrorIs(t, err, craft.ErrConflict)

			wrongUser := task
			wrongUser.Scope.UserID = "u2"
			_, err = store.PrepareTask(ctx, wrongUser)
			require.ErrorIs(t, err, craft.ErrForbidden)

			missingCall := task
			missingCall.ToolCallID = "c404"
			_, err = store.PrepareTask(ctx, missingCall)
			require.ErrorIs(t, err, craft.ErrNotFound)

			missingWorkspace := task
			missingWorkspace.WorkspaceID = "ws-404"
			_, err = store.PrepareTask(ctx, missingWorkspace)
			require.ErrorIs(t, err, craft.ErrNotFound)

			// A superseded epoch may not prepare new delegations.
			_, err = NewAgentRunStore(db).EnsureToolPlan(ctx, fence, craftToolPlan("c2"))
			require.NoError(t, err)
			require.NoError(t, db.Exec("UPDATE agent_runs SET epoch = epoch + 1 WHERE tenant_id = 1 AND run_id = 'r-craft-1'").Error)
			stale := task
			stale.ToolCallID = "c2"
			_, err = store.PrepareTask(ctx, stale)
			require.ErrorIs(t, err, craft.ErrConflict)

			gotTask, err := store.GetTask(ctx, scope, first.ID)
			require.NoError(t, err)
			require.Equal(t, "c1", gotTask.ToolCallID)
			require.Equal(t, task.Prompt, gotTask.Prompt)
			require.Equal(t, first.PromptMessageID, gotTask.PromptMessageID)
			require.Equal(t, task.Inputs, gotTask.Inputs)

			_, err = store.GetTask(ctx, craft.Scope{TenantID: 2, UserID: "u1", SessionID: "s1"}, first.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)
			_, err = store.GetTask(ctx, craft.Scope{TenantID: 1, UserID: "u2", SessionID: "s1"}, first.ID)
			require.ErrorIs(t, err, craft.ErrForbidden)

			_, err = store.GetResult(ctx, scope, first.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}

// TestCraftSaveResultFenceAndIdempotency: results commit only under the live
// run fence, identical replays are idempotent, different results for a
// finished delegation are rejected, and an expired epoch cannot write.
func TestCraftSaveResultFenceAndIdempotency(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			store := NewCraftStore(db)
			ctx := context.Background()
			fence := seedCraftRun(t, db, "r-craft-1", "c1")
			scope := craftTestScope()
			ws := putCraftWorkspace(t, store)
			task, err := store.PrepareTask(ctx, craft.Task{
				ToolCallID: "c1", Prompt: "build", RequestHash: "rh-1",
				Scope: scope, Fence: fence, WorkspaceID: ws.ID,
			})
			require.NoError(t, err)

			result := craft.Result{
				TaskID: task.ID, Status: "succeeded", Summary: "done",
				Files:  []craft.File{{Path: "index.html", Ref: "resource://out/index.html", SHA256: "sha-out", MIME: "text/html", Bytes: 42}},
				Checks: []craft.Check{{Name: "build", Status: "passed"}},
			}
			require.NoError(t, store.SaveResult(ctx, fence, result))
			require.NoError(t, store.SaveResult(ctx, fence, result))

			different := result
			different.Summary = "changed"
			err = store.SaveResult(ctx, fence, different)
			require.ErrorIs(t, err, craft.ErrConflict)

			err = store.SaveResult(ctx, fence, craft.Result{TaskID: task.ID, Status: "exploded"})
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			err = store.SaveResult(ctx, fence, craft.Result{Status: "succeeded"})
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			err = store.SaveResult(ctx, fence, craft.Result{TaskID: "ws-404", Status: "succeeded"})
			require.ErrorIs(t, err, craft.ErrNotFound)

			got, err := store.GetResult(ctx, scope, task.ID)
			require.NoError(t, err)
			require.Equal(t, "succeeded", got.Status)
			require.Equal(t, result.Files, got.Files)
			require.Equal(t, result.Checks, got.Checks)
			var status string
			require.NoError(t, db.Raw("SELECT status FROM craft_delegations WHERE id = ?", task.ID).Scan(&status).Error)
			require.Equal(t, "succeeded", status)

			// A delegation written by a worker whose epoch has been
			// superseded must not be finalized by that worker.
			_, err = NewAgentRunStore(db).EnsureToolPlan(ctx, fence, craftToolPlan("c2"))
			require.NoError(t, err)
			task2, err := store.PrepareTask(ctx, craft.Task{
				ToolCallID: "c2", Prompt: "refine", RequestHash: "rh-2",
				Scope: scope, Fence: fence, WorkspaceID: ws.ID,
			})
			require.NoError(t, err)
			require.NoError(t, db.Exec("UPDATE agent_runs SET epoch = epoch + 1 WHERE tenant_id = 1 AND run_id = 'r-craft-1'").Error)
			err = store.SaveResult(ctx, fence, craft.Result{TaskID: task2.ID, Status: "succeeded", Summary: "late"})
			require.ErrorIs(t, err, craft.ErrConflict)
			_, err = store.GetResult(ctx, scope, task2.ID)
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}
