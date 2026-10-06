package service

// T16 (#134) — the durable Workspace writer lease as ONE journey at the
// highest service seam owned by this lane (CraftWorkspaceService over the
// real migrated database):
//
//  1. two concurrent writing Runs on one Workspace → exactly ONE acquisition
//     and ONE stable conflict that keeps naming the winning Run;
//  2. the read-only surface (workspace read, version list/get, lease
//     projection) stays available while the lease is held and never creates
//     or mutates a lease row; a non-holder cannot release the lease;
//  3. an UNKNOWN outcome retains the fence — release is refused and a third
//     Run still cannot break in, even under the verified/stop basis;
//  4. only after the holder's authoritative terminal observation does the
//     lease release (verified completion) and a waiting Run acquires;
//  5. the second successful edit publishes a NEW immutable version without
//     mutating the old one, and read-only traffic after full release creates
//     no lease.
//
// Preview/version/download HTTP handlers sit above these seams; their data
// paths are exercised here through the same stores they read.
import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

func TestCraftT16Journey(t *testing.T) {
	db := openCraftSessionDB(t)
	ctx := context.Background()
	const taskID = "s-t16" // Task ID == Session ID
	scope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: taskID}
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, 1, 't16', 'u1', 'trpc')`,
		taskID).Error)

	store, ok := repository.NewCraftStore(db).(*repository.CraftStore)
	require.True(t, ok, "the concrete lease store is assembled from the migrated database")
	workspace, err := store.PutWorkspace(ctx, craft.Workspace{
		Scope: scope, SandboxID: "sbx-t16", Generation: "0",
		OpenCodeSessionID: "oc-t16", RuntimeDigest: pinnedCraftRuntimeDigest,
	}, 0)
	require.NoError(t, err)

	svc, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:    store,
		Bindings: sandbox.NewMemorySessionSandboxBindingStore(),
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) {
			return false, nil
		},
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			return nil, errors.New("dial is not part of the T16 journey")
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
		WriterLeases:  store,
	})
	require.NoError(t, err)

	runs := repository.NewAgentRunStore(db)
	// admit establishes a Run through the real admission API: it also claims
	// the session's single run slot, which is exactly the pre-T16 status quo.
	admit := func(runID string) {
		t.Helper()
		_, err := runs.Admit(ctx, agentruntime.Admission{
			Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
			SessionID:          taskID,
			UserID:             "u1",
			RequestID:          "craft-" + runID,
			AssistantMessageID: "craft-a-" + runID,
			RequestHash:        "craft-h-" + runID,
			Snapshot:           json.RawMessage(`{"version":1,"craft":true}`),
			UserMessage:        json.RawMessage(`{"role":"user","content":"t16"}`),
			AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
			Deadline:           time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
	}
	// admitDurable establishes a second durable Run row for the same Task
	// without going through the session-slot admission path: the concurrent
	// writer of the lease race (a second worker, a cross-process dispatch)
	// is durable while the session slot is held by the first arrival.
	admitDurable := func(runID string) {
		t.Helper()
		require.NoError(t, db.Exec(
			`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id,
			   assistant_message_id, request_hash, status, snapshot, deadline)
			 VALUES (1, ?, ?, 'u1', ?, ?, ?, 'queued', ?, ?)`,
			runID, taskID, "craft-"+runID, "craft-a-"+runID, "craft-h-"+runID,
			`{"version":1,"craft":true}`, time.Now().Add(time.Hour).UTC(),
		).Error)
	}
	// terminalize simulates the authoritative terminal observation of a Run:
	// the durable run row is succeeded and the session slot is free.
	terminalize := func(runID string) {
		t.Helper()
		require.NoError(t, db.Exec(
			`UPDATE agent_runs SET status = 'succeeded' WHERE tenant_id = 1 AND run_id = ?`, runID).Error)
		require.NoError(t, db.Exec(
			`UPDATE sessions SET active_agent_run_id = NULL WHERE tenant_id = 1 AND id = ?`, taskID).Error)
	}
	leaseRow := func() (string, int64) {
		t.Helper()
		var row struct {
			RunID    string `gorm:"column:run_id"`
			Revision int64  `gorm:"column:revision"`
		}
		require.NoError(t, db.Raw(
			`SELECT run_id, revision FROM craft_workspace_writer_leases WHERE workspace_id = ?`,
			workspace.ID).Scan(&row).Error)
		return row.RunID, row.Revision
	}

	// ------------------------------------------------------------------
	// Phase 1 — two concurrent writers: exactly one acquisition, one stable
	// conflict, both persisted in the database bound to Task, Workspace, Run
	// and the draft-head revision.
	// ------------------------------------------------------------------
	admit("run-a")
	admitDurable("run-b")
	acquisitions := make([]craft.WriterAcquisition, 2)
	acquireErrs := make([]error, 2)
	var wg sync.WaitGroup
	for i, runID := range []string{"run-a", "run-b"} {
		wg.Add(1)
		go func(i int, runID string) {
			defer wg.Done()
			acquisitions[i], acquireErrs[i] = svc.AcquireWriter(ctx, scope, runID)
		}(i, runID)
	}
	wg.Wait()

	winners, conflicts := 0, 0
	winnerRun, holderRun := "", ""
	for i, acq := range acquisitions {
		require.NoError(t, acquireErrs[i], "acquisition itself must not fail; conflict is an outcome")
		switch acq.Outcome.Status {
		case craft.WriterAcquired:
			winners++
			require.Equal(t, workspace.ID, acq.Outcome.WorkspaceID)
			require.NotNil(t, acq.Lease, "an acquisition answers the durable lease")
			require.Equal(t, workspace.ID, acq.Lease.WorkspaceID, "the lease binds the Workspace")
			require.Equal(t, taskID, acq.Lease.TaskID, "the lease binds the Task (session)")
			require.Contains(t, []string{"run-a", "run-b"}, acq.Lease.RunID, "the lease binds the acquiring Run")
			require.Zero(t, acq.Lease.Revision, "the lease binds the draft-head revision at acquisition")
			winnerRun = acq.Lease.RunID
		case craft.WriterConflict:
			conflicts++
			require.Equal(t, workspace.ID, acq.Outcome.WorkspaceID)
			require.NotNil(t, acq.Holder, "a conflict names the current holder")
			holderRun = acq.Holder.RunID
		default:
			t.Fatalf("unexpected acquisition status %q", acq.Outcome.Status)
		}
	}
	require.Equal(t, 1, winners, "exactly one concurrent writer acquires")
	require.Equal(t, 1, conflicts, "exactly one concurrent writer conflicts")
	require.Equal(t, winnerRun, holderRun, "the conflict names the winner as its holder")

	persistedRun, persistedRevision := leaseRow()
	require.Equal(t, winnerRun, persistedRun, "the lease is database-backed")
	require.Zero(t, persistedRevision)

	loserRun := "run-a"
	if winnerRun == "run-a" {
		loserRun = "run-b"
	}
	// The conflict is STABLE: retrying the loser keeps conflicting with the
	// same holder, and the winner's retry is idempotent.
	retry, err := svc.AcquireWriter(ctx, scope, loserRun)
	require.NoError(t, err)
	require.Equal(t, craft.WriterConflict, retry.Outcome.Status)
	require.Equal(t, winnerRun, retry.Holder.RunID)
	again, err := svc.AcquireWriter(ctx, scope, winnerRun)
	require.NoError(t, err)
	require.Equal(t, craft.WriterAcquired, again.Outcome.Status)
	require.Equal(t, winnerRun, again.Lease.RunID)
	require.Zero(t, again.Lease.Revision)

	// ------------------------------------------------------------------
	// Phase 2 — read-only traffic never acquires the writer lease, stays
	// available while a writer holds it, and a non-holder cannot release it.
	// ------------------------------------------------------------------
	versions := repository.NewCraftVersionStore(db)
	listed, err := versions.List(ctx, scope)
	require.NoError(t, err, "version listing stays available under a held lease")
	require.Empty(t, listed)
	gotWorkspace, err := store.GetWorkspace(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, workspace.ID, gotWorkspace.ID)
	held, err := svc.WriterLease(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, winnerRun, held.RunID, "the lease projection reads the durable row")
	require.Equal(t, taskID, held.TaskID)
	require.Equal(t, workspace.ID, held.WorkspaceID)

	err = svc.ReleaseWriter(ctx, scope, loserRun, craft.WriterReleaseVerifiedCompletion)
	require.ErrorIs(t, err, craft.ErrForbidden, "only the holding Run may release the lease")
	runID, rev := leaseRow()
	require.Equal(t, winnerRun, runID, "a refused release leaves the lease intact")
	require.Zero(t, rev)

	// ------------------------------------------------------------------
	// Phase 3 — an unknown outcome retains the fence: release is refused and
	// no third Run can break in — not even via the verified-completion basis
	// while the holder has no authoritative terminal observation.
	// ------------------------------------------------------------------
	err = svc.ReleaseWriter(ctx, scope, winnerRun, craft.WriterReleaseUnknown)
	require.ErrorIs(t, err, craft.ErrBusy, "unknown outcome must retain the writer fence")
	runID, _ = leaseRow()
	require.Equal(t, winnerRun, runID, "the fence survives the unknown outcome")

	err = svc.ReleaseWriter(ctx, scope, winnerRun, craft.WriterReleaseVerifiedCompletion)
	require.ErrorIs(t, err, craft.ErrBusy, "a live holder without a verified terminal observation cannot release")

	admitDurable("run-c")
	breakIn, err := svc.AcquireWriter(ctx, scope, "run-c")
	require.NoError(t, err)
	require.Equal(t, craft.WriterConflict, breakIn.Outcome.Status, "an unknown-held fence cannot admit a conflicting writer")
	require.Equal(t, winnerRun, breakIn.Holder.RunID)

	// ------------------------------------------------------------------
	// Phase 4 — verified completion releases the fence and the waiting Run
	// acquires against the same Workspace.
	// ------------------------------------------------------------------
	terminalize(winnerRun)
	require.NoError(t, svc.ReleaseWriter(ctx, scope, winnerRun, craft.WriterReleaseVerifiedCompletion))
	runID, _ = leaseRow()
	require.Empty(t, runID, "the lease row is gone after verified completion")

	next, err := svc.AcquireWriter(ctx, scope, "run-c")
	require.NoError(t, err)
	require.Equal(t, craft.WriterAcquired, next.Outcome.Status)
	require.Equal(t, "run-c", next.Lease.RunID)
	require.Equal(t, taskID, next.Lease.TaskID)
	require.Zero(t, next.Lease.Revision, "the revision fence still binds the current draft head")

	// ------------------------------------------------------------------
	// Phase 5 — the second successful edit publishes a NEW immutable version
	// without mutating the old one; after every lease is released, read-only
	// traffic creates no lease.
	// ------------------------------------------------------------------
	publish := func(runID string, expectedRevision int64, content string, shaSeed byte) craft.Version {
		t.Helper()
		files := []craft.File{{
			Path: "index.html", Ref: "resource://out/" + runID + "-index.html",
			SHA256: strings.Repeat(string(rune('a'+shaSeed)), 64),
			MIME:   "text/html", Bytes: int64(len(content)),
		}}
		head, err := repository.NewCraftDraftHeadStore(db).Advance(ctx, scope, workspace.ID, expectedRevision, runID, files)
		require.NoError(t, err, "a terminal Run advances the workspace draft")
		digest, err := craft.ManifestDigest(files)
		require.NoError(t, err)
		require.Equal(t, digest, head.ManifestDigest)
		published, err := versions.Publish(ctx, scope, craft.Version{
			WorkspaceID: workspace.ID, RunID: runID, Kind: craft.KindWeb, Files: files,
			Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed}},
		})
		require.NoError(t, err)
		require.Equal(t, craft.VersionID(workspace.ID, runID, digest), published.ID)
		return published
	}

	terminalize("run-c")
	v1 := publish("run-c", 0, "<h1>v1</h1>", 0)
	require.NoError(t, svc.ReleaseWriter(ctx, scope, "run-c", craft.WriterReleaseVerifiedCompletion))

	admitDurable("run-d")
	second, err := svc.AcquireWriter(ctx, scope, "run-d")
	require.NoError(t, err)
	require.Equal(t, craft.WriterAcquired, second.Outcome.Status)
	require.Equal(t, int64(1), second.Lease.Revision, "the second edit's lease fences the advanced draft revision")
	terminalize("run-d")
	v2 := publish("run-d", 1, "<h1>v2</h1>", 1)
	require.NoError(t, svc.ReleaseWriter(ctx, scope, "run-d", craft.WriterReleaseVerifiedCompletion))

	all, err := versions.List(ctx, scope)
	require.NoError(t, err)
	require.Len(t, all, 2, "two successful edits publish two versions")
	require.NotEqual(t, v1.ID, v2.ID)

	old, err := versions.Get(ctx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.Files, old.Files, "the first successful version is never mutated by the second edit")
	require.Equal(t, v1.Checks, old.Checks)
	replayed, err := versions.Publish(ctx, scope, craft.Version{
		WorkspaceID: workspace.ID, RunID: "run-c", Kind: craft.KindWeb, Files: v1.Files,
		Checks: v1.Checks,
	})
	require.NoError(t, err)
	require.Equal(t, v1.ID, replayed.ID, "replaying the first publish adopts the stored version")
	stillAll, err := versions.List(ctx, scope)
	require.NoError(t, err)
	require.Len(t, stillAll, 2, "the replay created no third version")

	runID, _ = leaseRow()
	require.Empty(t, runID, "after verified releases no lease remains")
	_, err = versions.List(ctx, scope)
	require.NoError(t, err, "read-only traffic after full release stays available")
	runID, _ = leaseRow()
	require.Empty(t, runID, "read-only traffic never acquires the writer lease")
}
