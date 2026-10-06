package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type craftSeedSnapshot struct {
	WorkspaceID    string               `json:"workspace_id"`
	State          craft.DraftHeadState `json:"state"`
	DraftRevision  int64                `json:"draft_revision"`
	SourceRunID    string               `json:"source_run_id"`
	ManifestDigest string               `json:"manifest_digest"`
}

func craftSeedAdmission(t *testing.T, runID, requestID, actor, prompt, knowledgeID string) agentruntime.Admission {
	t.Helper()
	assistantID := "assistant-" + requestID
	knowledgeIDs := []string{}
	if knowledgeID != "" {
		knowledgeIDs = []string{knowledgeID}
	}
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": prompt, "model_id": "model-1", "agent_config": json.RawMessage(`{}`),
		"craft_input_manifest":      []craft.Input{},
		"craft_knowledge_selection": map[string]any{"query": prompt, "knowledge_base_ids": knowledgeIDs},
	})
	require.NoError(t, err)
	user, err := json.Marshal(map[string]any{"role": "user", "content": prompt})
	require.NoError(t, err)
	assistant, err := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	require.NoError(t, err)
	return agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 1, RunID: runID}, SessionID: "s1", UserID: "u1", ActorUserID: actor,
		RequestID: requestID, AssistantMessageID: assistantID,
		RequestHash: craftRequestDigest(snapshot, assistantID), Snapshot: snapshot,
		UserMessage: user, AssistantMessage: assistant, Deadline: time.Now().Add(time.Hour),
	}
}

func craftRequestDigest(snapshot []byte, assistantID string) string {
	preimage := append(append([]byte(nil), snapshot...), []byte(assistantID)...)
	digest := sha256.Sum256(preimage)
	return hex.EncodeToString(digest[:])
}

func TestSameCraftAdmissionIntentCanonicalizesNestedJSON(t *testing.T) {
	stored := agentRunRow{Snapshot: `{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"craft_knowledge_selection":{"query":"Build","knowledge_base_ids":["kb-1","kb-2"]},"craft_workspace_seed":{"state":"empty"}}`}
	tests := []struct {
		name     string
		incoming string
		want     bool
	}{
		{
			name:     "object member order is not intent",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"knowledge_base_ids":["kb-1","kb-2"],"query":"Build"},"model_id":"model-1","query":"Build","version":1}`,
			want:     true,
		},
		{
			name:     "nested value change conflicts",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"knowledge_base_ids":["kb-1","kb-2"],"query":"Changed"},"model_id":"model-1","query":"Build","version":1}`,
			want:     false,
		},
		{
			name:     "array order remains intent",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"knowledge_base_ids":["kb-2","kb-1"],"query":"Build"},"model_id":"model-1","query":"Build","version":1}`,
			want:     false,
		},
		{
			name:     "integer precision remains exact",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"knowledge_base_ids":["kb-1","kb-2"],"query":"Build"},"model_id":"model-1","query":"Build","version":1,"large":9007199254740993}`,
			want:     false,
		},
		{
			name:     "duplicate nested keys fail closed",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"query":"Build","query":"Build","knowledge_base_ids":["kb-1","kb-2"]},"model_id":"model-1","query":"Build","version":1}`,
			want:     false,
		},
		{
			name:     "nested seed-like key is not excluded",
			incoming: `{"agent_config":{},"craft_knowledge_selection":{"knowledge_base_ids":["kb-1","kb-2"],"query":"Build"},"model_id":"model-1","query":"Build","version":1,"nested":{"craft_workspace_seed":"client-value"}}`,
			want:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, sameCraftAdmissionIntent(stored, json.RawMessage(tc.incoming)))
		})
	}
	t.Run("duplicate top-level keys fail closed", func(t *testing.T) {
		duplicate := agentRunRow{Snapshot: `{"version":1,"query":"Build","query":"Build","model_id":"model-1","agent_config":{},"craft_knowledge_selection":{"query":"Build","knowledge_base_ids":["kb-1","kb-2"]},"craft_workspace_seed":{"state":"empty"}}`}
		incoming := json.RawMessage(`{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"craft_knowledge_selection":{"query":"Build","knowledge_base_ids":["kb-1","kb-2"]}}`)
		require.False(t, sameCraftAdmissionIntent(duplicate, incoming))
	})
	t.Run("identical duplicate nested keys fail closed", func(t *testing.T) {
		duplicate := `{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"craft_knowledge_selection":{"query":"Build","query":"Build","knowledge_base_ids":["kb-1","kb-2"]}}`
		stored := agentRunRow{Snapshot: `{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"craft_knowledge_selection":{"query":"Build","query":"Build","knowledge_base_ids":["kb-1","kb-2"]},"craft_workspace_seed":{"state":"empty"}}`}
		require.False(t, sameCraftAdmissionIntent(stored, json.RawMessage(duplicate)))
	})
	t.Run("JSONB numeric normalization stays semantic", func(t *testing.T) {
		stored := agentRunRow{Snapshot: `{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"numeric":1000,"craft_workspace_seed":{"state":"empty"}}`}
		incoming := json.RawMessage(`{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"numeric":1e3}`)
		require.True(t, sameCraftAdmissionIntent(stored, incoming))
	})
	t.Run("large integer changes remain distinct", func(t *testing.T) {
		stored := agentRunRow{Snapshot: `{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"numeric":9007199254740992,"craft_workspace_seed":{"state":"empty"}}`}
		incoming := json.RawMessage(`{"version":1,"query":"Build","model_id":"model-1","agent_config":{},"numeric":9007199254740993}`)
		require.False(t, sameCraftAdmissionIntent(stored, incoming))
	})
}

func TestCraftAdmittedSnapshotDigestCoversFinalSnapshotFields(t *testing.T) {
	base := `{"query":"Build","model_id":"model-1","craft_input_manifest":[{"ref":"r","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}],"craft_knowledge_selection":{"knowledge_base_ids":["kb-1"]},"craft_workspace_seed":{"workspace_id":"ws-1","draft_revision":1}}`
	baseDigest, err := craftAdmittedSnapshotDigest([]byte(base))
	require.NoError(t, err)
	variants := map[string]string{
		"query":     strings.Replace(base, `"Build"`, `"Build changed"`, 1),
		"model":     strings.Replace(base, `"model-1"`, `"model-2"`, 1),
		"input":     strings.Replace(base, strings.Repeat("a", 64), strings.Repeat("b", 64), 1),
		"knowledge": strings.Replace(base, `"kb-1"`, `"kb-2"`, 1),
		"seed":      strings.Replace(base, `"draft_revision":1`, `"draft_revision":2`, 1),
	}
	for name, raw := range variants {
		t.Run(name, func(t *testing.T) {
			got, err := craftAdmittedSnapshotDigest([]byte(raw))
			require.NoError(t, err)
			require.NotEqual(t, baseDigest, got, "admitted snapshot digest must bind %s", name)
		})
	}
	reordered := `{"craft_workspace_seed":{"draft_revision":1,"workspace_id":"ws-1"},"craft_knowledge_selection":{"knowledge_base_ids":["kb-1"]},"craft_input_manifest":[{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","ref":"r"}],"model_id":"model-1","query":"Build"}`
	reorderedDigest, err := craftAdmittedSnapshotDigest([]byte(reordered))
	require.NoError(t, err)
	require.Equal(t, baseDigest, reorderedDigest, "object member order is not snapshot identity")
}

func TestCraftAdmissionSnapshotDigestPersistsAndFlowsThroughClaimFence(t *testing.T) {
	db := openRunTestDB(t)
	registerCraftWorkspace(t, db)
	store := NewAgentRunStore(db)
	run := admitCraftSeedRun(t, db, craftSeedAdmission(t, "craft-digest", "digest-request", "u1", "Build", "kb-1"))
	require.Equal(t, craftSnapshotDigestVersion, run.SnapshotDigestVersion)
	wantDigest, err := craftAdmittedSnapshotDigest(run.Snapshot)
	require.NoError(t, err)
	require.Equal(t, wantDigest, run.SnapshotDigest)
	var row agentRunRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", run.Key.TenantID, run.Key.RunID).Take(&row).Error)
	require.Equal(t, run.SnapshotDigestVersion, row.SnapshotDigestVersion)
	require.Equal(t, run.SnapshotDigest, row.SnapshotDigest)

	fence, err := store.ClaimDriver(context.Background(), run.Key, "platform", "digest-worker", time.Minute)
	require.NoError(t, err)
	require.Equal(t, run.SnapshotDigestVersion, fence.SnapshotDigestVersion)
	require.Equal(t, run.SnapshotDigest, fence.SnapshotDigest)
}

func TestCraftClaimFenceRejectsLegacyOrMutatedSnapshotIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *gorm.DB, agentruntime.Run)
	}{
		{
			name: "legacy digest absent",
			mutate: func(t *testing.T, db *gorm.DB, run agentruntime.Run) {
				require.NoError(t, db.Exec("UPDATE agent_runs SET snapshot_digest_version = NULL, snapshot_digest = NULL WHERE tenant_id = ? AND run_id = ?", run.Key.TenantID, run.Key.RunID).Error)
			},
		},
		{
			name: "stored snapshot changed after admission",
			mutate: func(t *testing.T, db *gorm.DB, run agentruntime.Run) {
				changed := strings.Replace(string(run.Snapshot), `"Build"`, `"Changed"`, 1)
				require.NoError(t, db.Exec("UPDATE agent_runs SET snapshot = ? WHERE tenant_id = ? AND run_id = ?", changed, run.Key.TenantID, run.Key.RunID).Error)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRunTestDB(t)
			registerCraftWorkspace(t, db)
			run := admitCraftSeedRun(t, db, craftSeedAdmission(t, "craft-digest", "digest-request", "u1", "Build", "kb-1"))
			tc.mutate(t, db, run)
			_, err := NewAgentRunStore(db).ClaimDriver(context.Background(), run.Key, "platform", "digest-worker", time.Minute)
			require.ErrorIs(t, err, agentruntime.ErrConflict)
			var row agentRunRow
			require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", run.Key.TenantID, run.Key.RunID).Take(&row).Error)
			require.Equal(t, "queued", row.Status, "failed identity verification rolls back the lease claim")
			require.Empty(t, row.LeaseOwner)
			require.Zero(t, row.Epoch)
		})
	}
}

func TestCraftAdmissionReplayRejectsCorruptPersistedSnapshotDigest(t *testing.T) {
	db := openRunTestDB(t)
	registerCraftWorkspace(t, db)
	store := NewAgentRunStore(db)
	in := craftSeedAdmission(t, "craft-replay-corrupt", "replay-corrupt", "u1", "Build", "kb-1")
	_, err := store.Admit(context.Background(), in)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"UPDATE agent_runs SET snapshot_digest = ? WHERE tenant_id = ? AND run_id = ?",
		strings.Repeat("f", 64), in.Key.TenantID, in.Key.RunID,
	).Error)

	_, err = store.Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict, "admission replay must detect persisted digest corruption before returning the Run")
}

func TestPersistUsageBindingPreservesExactIntegersAndRejectsDuplicateKeys(t *testing.T) {
	out, err := persistUsageBinding(json.RawMessage(`{"large":9007199254740993}`), agentruntime.Admission{})
	require.NoError(t, err)
	require.Contains(t, string(out), `"large":9007199254740993`)
	require.NotContains(t, string(out), `"large":9007199254740992`)

	_, err = persistUsageBinding(json.RawMessage(`{"nested":{"key":"first","key":"second"}}`), agentruntime.Admission{})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func craftSeedFromRun(t *testing.T, run agentruntime.Run) craftSeedSnapshot {
	t.Helper()
	var snapshot struct {
		CraftWorkspaceSeed *craftSeedSnapshot `json:"craft_workspace_seed"`
	}
	require.NoError(t, json.Unmarshal(run.Snapshot, &snapshot))
	require.NotNil(t, snapshot.CraftWorkspaceSeed)
	return *snapshot.CraftWorkspaceSeed
}

func registerCraftWorkspace(t *testing.T, db *gorm.DB) craft.Workspace {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id, tenant_id, kind) VALUES ('s1', 1, 'web')`).Error)
	return putCraftWorkspace(t, NewCraftStore(db))
}

func admitCraftSeedRun(t *testing.T, db *gorm.DB, in agentruntime.Admission) agentruntime.Run {
	t.Helper()
	run, err := NewAgentRunStore(db).Admit(context.Background(), in)
	require.NoError(t, err)
	return run
}

func failRunAndReleaseSlot(t *testing.T, db *gorm.DB, run agentruntime.Run) {
	t.Helper()
	store := NewAgentRunStore(db)
	fence, err := store.Claim(context.Background(), run.Key, "seed-test-worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, store.SetStatus(context.Background(), fence, "failed", "safe test failure"))
}

func terminalRunOnSession(t *testing.T, db *gorm.DB, sessionID, runID string) agentruntime.Fence {
	t.Helper()
	in := testAdmission()
	in.Key.RunID = runID
	in.SessionID = sessionID
	in.RequestID = "foreign-source-" + runID
	in.AssistantMessageID = "foreign-assistant-" + runID
	in.ActorUserID = "u1"
	runs := NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), in)
	require.NoError(t, err)
	fence, err := runs.Claim(context.Background(), in.Key, "foreign-source-worker", time.Minute)
	require.NoError(t, err)
	require.NoError(t, runs.SetStatus(context.Background(), fence, "failed", "terminal source"))
	return fence
}

func TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			d1 := terminalDraftRun(t, db, "seed-source-d1", "seed-call-d1")
			d2 := terminalDraftRun(t, db, "seed-source-d2", "seed-call-d2")
			workspace := registerCraftWorkspace(t, db)
			drafts := NewCraftDraftHeadStore(db)

			runA := admitCraftSeedRun(t, db, craftSeedAdmission(t, "craft-seed-a", "seed-a", "u1", "Build A", ""))
			seedA := craftSeedFromRun(t, runA)
			require.Equal(t, workspace.ID, seedA.WorkspaceID)
			require.Equal(t, craft.DraftHeadEmpty, seedA.State)
			require.Zero(t, seedA.DraftRevision)
			require.Empty(t, seedA.SourceRunID)
			require.Empty(t, seedA.ManifestDigest)
			failRunAndReleaseSlot(t, db, runA)

			filesD1 := []craft.File{sealedDraftFile("index.html", "object://seed/d1", "a", 8)}
			headD1, err := drafts.Advance(context.Background(), workspace.Scope, workspace.ID, 0, d1.RunID, filesD1)
			require.NoError(t, err)
			require.Equal(t, craft.DraftHeadSelected, headD1.State)

			admissionB := craftSeedAdmission(t, "craft-seed-b", "seed-b", "u1", "Build B", "kb-b")
			runB := admitCraftSeedRun(t, db, admissionB)
			seedB := craftSeedFromRun(t, runB)
			require.Equal(t, workspace.ID, seedB.WorkspaceID)
			require.Equal(t, craft.DraftHeadSelected, seedB.State)
			require.EqualValues(t, 1, seedB.DraftRevision)
			require.Equal(t, d1.RunID, seedB.SourceRunID)
			require.Equal(t, headD1.ManifestDigest, seedB.ManifestDigest)
			var runRow agentRunRow
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, runB.Key.RunID).Take(&runRow).Error)
			require.Equal(t, craftRequestDigest(runB.Snapshot, runB.AssistantMessageID), runRow.RequestHash,
				"the persisted request hash covers the repository-selected seed")
			var query string
			require.NoError(t, json.Unmarshal(runB.Snapshot, &struct {
				Query *string `json:"query"`
			}{Query: &query}))
			require.Equal(t, "Build B", query, "seed identity never enters the model query")
			failRunAndReleaseSlot(t, db, runB)

			filesD2 := []craft.File{sealedDraftFile("index.html", "object://seed/d2", "b", 9)}
			headD2, err := drafts.Advance(context.Background(), workspace.Scope, workspace.ID, 1, d2.RunID, filesD2)
			require.NoError(t, err)

			reopened := NewAgentRunStore(reopenRunDB(t, db))
			replay, err := reopened.Admit(context.Background(), admissionB)
			require.NoError(t, err)
			require.Equal(t, runB.Key, replay.Key)
			require.JSONEq(t, string(runB.Snapshot), string(replay.Snapshot), "same-key replay retains the original semantic snapshot after D2 and database reopen")
			require.Equal(t, seedB, craftSeedFromRun(t, replay))

			changedPrompt := craftSeedAdmission(t, "craft-seed-b", "seed-b", "u1", "Changed B", "kb-b")
			_, err = reopened.Admit(context.Background(), changedPrompt)
			require.ErrorIs(t, err, agentruntime.ErrConflict)
			changedInput := craftSeedAdmission(t, "craft-seed-b", "seed-b", "u1", "Build B", "kb-b")
			var changedFields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(changedInput.Snapshot, &changedFields))
			changedManifest, marshalErr := json.Marshal([]craft.Input{{
				Ref: "input-ref", Name: "brief.txt", SHA256: stringsRepeat("a", 64), Bytes: 1,
			}})
			require.NoError(t, marshalErr)
			changedFields["craft_input_manifest"] = changedManifest
			changedInput.Snapshot, marshalErr = json.Marshal(changedFields)
			require.NoError(t, marshalErr)
			_, err = reopened.Admit(context.Background(), changedInput)
			require.ErrorIs(t, err, agentruntime.ErrConflict)
			changedKnowledge := craftSeedAdmission(t, "craft-seed-b", "seed-b", "u1", "Build B", "kb-other")
			_, err = reopened.Admit(context.Background(), changedKnowledge)
			require.ErrorIs(t, err, agentruntime.ErrConflict)

			require.NoError(t, db.Exec(`INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u2','u2','u2@example.test','x',1)`).Error)
			require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u2','contributor','active')`).Error)
			changedActor := craftSeedAdmission(t, "craft-seed-b", "seed-b", "u2", "Build B", "kb-b")
			_, err = reopened.Admit(context.Background(), changedActor)
			require.ErrorIs(t, err, agentruntime.ErrConflict)

			fresh := admitCraftSeedRun(t, db, craftSeedAdmission(t, "craft-seed-c", "seed-c", "u1", "Build C", ""))
			seedC := craftSeedFromRun(t, fresh)
			require.EqualValues(t, 2, seedC.DraftRevision)
			require.Equal(t, d2.RunID, seedC.SourceRunID)
			require.Equal(t, headD2.ManifestDigest, seedC.ManifestDigest)
		})
	}
}

func TestAgentRunCraftSeedAdmissionRejectsUnresolvedHeadWithoutTransition(t *testing.T) {
	db := openRunTestDB(t)
	workspace := registerCraftWorkspace(t, db)
	require.NoError(t, db.Exec("DELETE FROM craft_workspace_draft_heads WHERE workspace_id = ?", workspace.ID).Error)
	store := NewAgentRunStore(db)
	in := craftSeedAdmission(t, "craft-missing-head", "missing-head", "u1", "Build", "")
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO craft_session_requests
		(tenant_id,user_id,purpose,request_id,request_hash,session_id,admission_run_id,admission_token,admission_state,lease_expires_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, 1, "u1", "input_admission", "selected-input", "intent", "s1", in.Key.RunID, "claim-token", "claimed", now.Add(time.Minute), now).Error)
	in.InputClaims = []agentruntime.InputAdmissionClaim{{DecisionKey: "selected-input", RunID: in.Key.RunID, Token: "claim-token"}}
	var versionsBefore int64
	require.NoError(t, db.Table("craft_versions").Where("tenant_id = ? AND workspace_id = ?", 1, workspace.ID).Count(&versionsBefore).Error)

	_, err := store.Admit(context.Background(), in)
	require.ErrorIs(t, err, craft.ErrDraftHeadUnresolved)
	var runs, versionsAfter int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
	require.Zero(t, runs)
	require.NoError(t, db.Table("craft_versions").Where("tenant_id = ? AND workspace_id = ?", 1, workspace.ID).Count(&versionsAfter).Error)
	require.Equal(t, versionsBefore, versionsAfter)
	var slot sql.NullString
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, "s1").Scan(&slot).Error)
	require.False(t, slot.Valid && slot.String != "")
	var claimState string
	require.NoError(t, db.Table("craft_session_requests").Select("admission_state").Where(
		"tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ?", 1, "u1", "input_admission", "selected-input",
	).Scan(&claimState).Error)
	require.Equal(t, "claimed", claimState, "head failure cannot admit the input claim")
}

func TestAgentRunCraftSeedAdmissionRejectsCallerSelectedSeed(t *testing.T) {
	db := openRunTestDB(t)
	workspace := registerCraftWorkspace(t, db)
	in := craftSeedAdmission(t, "craft-caller-seed", "caller-seed", "u1", "Build", "")
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(in.Snapshot, &fields))
	seed, err := json.Marshal(craftSeedSnapshot{WorkspaceID: workspace.ID, State: craft.DraftHeadEmpty})
	require.NoError(t, err)
	fields["craft_workspace_seed"] = seed
	in.Snapshot, err = json.Marshal(fields)
	require.NoError(t, err)

	_, err = NewAgentRunStore(db).Admit(context.Background(), in)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
	require.Zero(t, runs)
	var slot sql.NullString
	require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, "s1").Scan(&slot).Error)
	require.False(t, slot.Valid && slot.String != "")
}

func TestAgentRunCraftSeedAdmissionRequiresMarkerForRegisteredCraftSession(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, map[string]json.RawMessage)
	}{
		{name: "missing", mutate: func(_ *testing.T, fields map[string]json.RawMessage) {
			delete(fields, "craft_input_manifest")
		}},
		{name: "null", mutate: func(_ *testing.T, fields map[string]json.RawMessage) {
			fields["craft_input_manifest"] = json.RawMessage(`null`)
		}},
		{name: "malformed", mutate: func(_ *testing.T, fields map[string]json.RawMessage) {
			fields["craft_input_manifest"] = json.RawMessage(`"not-an-array"`)
		}},
		{name: "incomplete", mutate: func(_ *testing.T, fields map[string]json.RawMessage) {
			fields["craft_input_manifest"] = json.RawMessage(`[{}]`)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := openRunTestDB(t)
			registerCraftWorkspace(t, db)
			in := craftSeedAdmission(t, "craft-marker-"+tc.name, "marker-"+tc.name, "u1", "Build", "")
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(in.Snapshot, &fields))
			tc.mutate(t, fields)
			encoded, err := json.Marshal(fields)
			require.NoError(t, err)
			in.Snapshot = encoded

			now := time.Now()
			decisionKey := "marker-input-" + tc.name
			token := "marker-token-" + tc.name
			require.NoError(t, db.Exec(`INSERT INTO craft_session_requests
				(tenant_id,user_id,purpose,request_id,request_hash,session_id,admission_run_id,admission_token,admission_state,lease_expires_at,created_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)`, 1, "u1", "input_admission", decisionKey, "intent", "s1", in.Key.RunID, token, "claimed", now.Add(time.Minute), now).Error)
			in.InputClaims = []agentruntime.InputAdmissionClaim{{DecisionKey: decisionKey, RunID: in.Key.RunID, Token: token}}

			_, err = NewAgentRunStore(db).Admit(context.Background(), in)
			require.ErrorIs(t, err, agentruntime.ErrConflict)
			var runs int64
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
			require.Zero(t, runs)
			var slot sql.NullString
			require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, "s1").Scan(&slot).Error)
			require.False(t, slot.Valid && slot.String != "")
			var claimState string
			require.NoError(t, db.Table("craft_session_requests").Select("admission_state").Where(
				"tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ?", 1, "u1", "input_admission", decisionKey,
			).Scan(&claimState).Error)
			require.Equal(t, "claimed", claimState)
		})
	}
}

func TestAgentRunGenericAdmissionDoesNotRequireCraftSeed(t *testing.T) {
	db := openRunTestDB(t)
	in := testAdmission()
	run, err := NewAgentRunStore(db).Admit(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, in.Snapshot, run.Snapshot)
	require.NotContains(t, string(run.Snapshot), "craft_workspace_seed")
}

func TestAgentRunCraftSeedAdmissionRejectsWorkspaceOwnerOrSourceMismatch(t *testing.T) {
	t.Run("Workspace owner mismatch", func(t *testing.T) {
		db := openRunTestDB(t)
		workspace := registerCraftWorkspace(t, db)
		require.NoError(t, db.Exec(`INSERT INTO users (id,username,email,password_hash,tenant_id) VALUES ('u2','u2','u2@example.test','x',1)`).Error)
		require.NoError(t, db.Exec("UPDATE craft_workspaces SET owner_id = 'u2' WHERE id = ?", workspace.ID).Error)
		in := craftSeedAdmission(t, "craft-owner-mismatch", "owner-mismatch", "u1", "Build", "")
		_, err := NewAgentRunStore(db).Admit(context.Background(), in)
		require.ErrorIs(t, err, craft.ErrForbidden)
		var runs int64
		require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
		require.Zero(t, runs)
	})

	t.Run("foreign source Run session", func(t *testing.T) {
		db := openRunTestDB(t)
		foreign := terminalRunOnSession(t, db, "s2", "foreign-seed-source")
		workspace := registerCraftWorkspace(t, db)
		// D0 immutable-origin (SQLite000119/PG000198) forbids rewriting a
		// sealed revision, and CraftDraftHeadStore.Advance itself rejects a
		// foreign-session source Run. Construct the corrupted-but-coherent
		// state at birth instead: revision 1 and its files are inserted
		// directly with the foreign source so the immutable row never has to
		// be updated, and the mutable head pointer names the same revision.
		files := []craft.File{sealedDraftFile("index.html", "object://seed/foreign", "e", 12)}
		digest, err := craft.ManifestDigest(files)
		require.NoError(t, err)
		require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_revisions
			(workspace_id, revision, tenant_id, source_run_id, manifest_digest, created_at)
			VALUES (?, 1, ?, ?, ?, CURRENT_TIMESTAMP)`,
			workspace.ID, workspace.Scope.TenantID, foreign.RunID, digest).Error)
		require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_files
			(workspace_id, revision, path, object_ref, sha256, bytes, mime) VALUES (?, 1, ?, ?, ?, ?, ?)`,
			workspace.ID, files[0].Path, files[0].Ref, files[0].SHA256, files[0].Bytes, files[0].MIME).Error)
		require.NoError(t, db.Table("craft_workspace_draft_heads").Where("workspace_id = ?", workspace.ID).
			Updates(map[string]any{"revision": 1, "state": string(craft.DraftHeadSelected),
				"source_run_id": foreign.RunID, "manifest_digest": digest}).Error)
		in := craftSeedAdmission(t, "craft-foreign-source", "foreign-source", "u1", "Build", "")
		_, err = NewAgentRunStore(db).Admit(context.Background(), in)
		require.Error(t, err)
		var runs int64
		require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
		require.Zero(t, runs)
	})
}

func TestAgentRunCraftSeedAdmissionRollsBackWriterAndInputClaim(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			source := terminalDraftRun(t, db, "seed-rollback-source", "seed-rollback-call")
			workspace := registerCraftWorkspace(t, db)
			_, err := NewCraftDraftHeadStore(db).Advance(context.Background(), workspace.Scope, workspace.ID, 0, source.RunID,
				[]craft.File{sealedDraftFile("index.html", "object://seed/rollback", "f", 13)})
			require.NoError(t, err)
			in := craftSeedAdmission(t, source.RunID, "seed-rollback-new-request", "u1", "Build", "")
			now := time.Now()
			require.NoError(t, db.Exec(`INSERT INTO craft_session_requests
				(tenant_id,user_id,purpose,request_id,request_hash,session_id,admission_run_id,admission_token,admission_state,lease_expires_at,created_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?)`, 1, "u1", "input_admission", "rollback-input", "intent", "s1", in.Key.RunID, "rollback-token", "claimed", now.Add(time.Minute), now).Error)
			in.InputClaims = []agentruntime.InputAdmissionClaim{{DecisionKey: "rollback-input", RunID: in.Key.RunID, Token: "rollback-token"}}

			_, err = NewAgentRunStore(db).Admit(context.Background(), in)
			require.ErrorIs(t, err, agentruntime.ErrConflict, "the duplicate run primary key fails after seed choice and slot reservation")
			var slot sql.NullString
			require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, "s1").Scan(&slot).Error)
			require.False(t, slot.Valid && slot.String != "", "the failed Run insert rolls back the writer reservation")
			var claimState string
			require.NoError(t, db.Table("craft_session_requests").Select("admission_state").Where(
				"tenant_id = ? AND user_id = ? AND purpose = ? AND request_id = ?", 1, "u1", "input_admission", "rollback-input",
			).Scan(&claimState).Error)
			require.Equal(t, "claimed", claimState, "a failed Run insert cannot mark the T01 claim admitted")
			var newRows int64
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND request_id = ?", 1, in.RequestID).Count(&newRows).Error)
			require.Zero(t, newRows)
		})
	}
}

func TestAgentRunCraftSeedAdmissionRejectsCorruptOrForeignHead(t *testing.T) {
	for _, tc := range []struct {
		name string
		// advance selects the shared valid Advance setup. Cases that must
		// construct their immutable revision rows directly (because D0
		// immutable-origin forbids post-hoc rewrites) set advance=false.
		advance bool
		mutate  func(*testing.T, *gorm.DB, craft.Workspace, agentruntime.Fence)
	}{
		{name: "head digest mismatch", advance: true, mutate: func(t *testing.T, db *gorm.DB, workspace craft.Workspace, source agentruntime.Fence) {
			require.NoError(t, db.Exec("UPDATE craft_workspace_draft_heads SET manifest_digest = ? WHERE workspace_id = ?", stringsRepeat("0", 64), workspace.ID).Error)
		}},
		{name: "selected files removed", advance: false, mutate: func(t *testing.T, db *gorm.DB, workspace craft.Workspace, source agentruntime.Fence) {
			// D0 immutable-origin (SQLite000119/PG000198) forbids deleting a
			// sealed revision's files. Construct the corrupt state at birth
			// instead: the immutable revision 1 row is inserted directly with
			// no file rows, and the mutable head names it.
			files := []craft.File{sealedDraftFile("index.html", "object://seed/corrupt", "c", 10)}
			digest, digestErr := craft.ManifestDigest(files)
			require.NoError(t, digestErr)
			require.NoError(t, db.Exec(`INSERT INTO craft_workspace_draft_revisions
				(workspace_id, revision, tenant_id, source_run_id, manifest_digest, created_at)
				VALUES (?, 1, ?, ?, ?, CURRENT_TIMESTAMP)`,
				workspace.ID, workspace.Scope.TenantID, source.RunID, digest).Error)
			require.NoError(t, db.Table("craft_workspace_draft_heads").Where("workspace_id = ?", workspace.ID).
				Updates(map[string]any{"revision": 1, "state": string(craft.DraftHeadSelected),
					"source_run_id": source.RunID, "manifest_digest": digest}).Error)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRunTestDB(t)
			source := terminalDraftRun(t, db, "seed-corrupt-source", "seed-corrupt-call")
			workspace := registerCraftWorkspace(t, db)
			if tc.advance {
				_, err := NewCraftDraftHeadStore(db).Advance(context.Background(), workspace.Scope, workspace.ID, 0, source.RunID,
					[]craft.File{sealedDraftFile("index.html", "object://seed/corrupt", "c", 10)})
				require.NoError(t, err)
			}
			tc.mutate(t, db, workspace, source)
			in := craftSeedAdmission(t, "craft-corrupt-head", "corrupt-head", "u1", "Build", "")
			_, err := NewAgentRunStore(db).Admit(context.Background(), in)
			require.Error(t, err)
			var runs int64
			require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 1, in.Key.RunID).Count(&runs).Error)
			require.Zero(t, runs)
			var slot sql.NullString
			require.NoError(t, db.Table("sessions").Select("active_agent_run_id").Where("tenant_id = ? AND id = ?", 1, "s1").Scan(&slot).Error)
			require.False(t, slot.Valid && slot.String != "")
		})
	}
}

func TestAgentRunCraftSeedAdmissionSerializesAgainstDraftAdvance(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			source := terminalDraftRun(t, db, "seed-race-source", "seed-race-call")
			workspace := registerCraftWorkspace(t, db)
			admission := craftSeedAdmission(t, "craft-seed-race", "seed-race", "u1", "Build", "")
			start := make(chan struct{})
			var wg sync.WaitGroup
			var admitted agentruntime.Run
			var admitErr, advanceErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				admitted, admitErr = NewAgentRunStore(db).Admit(context.Background(), admission)
			}()
			go func() {
				defer wg.Done()
				<-start
				_, advanceErr = NewCraftDraftHeadStore(db).Advance(context.Background(), workspace.Scope, workspace.ID, 0, source.RunID,
					[]craft.File{sealedDraftFile("index.html", "object://seed/race", "d", 11)})
			}()
			close(start)
			wg.Wait()
			require.NoError(t, admitErr)
			if advanceErr == nil {
				require.Equal(t, craft.DraftHeadSelected, craftSeedFromRun(t, admitted).State)
				require.EqualValues(t, 1, craftSeedFromRun(t, admitted).DraftRevision)
			} else {
				require.ErrorIs(t, advanceErr, craft.ErrBusy)
				require.Equal(t, craft.DraftHeadEmpty, craftSeedFromRun(t, admitted).State)
				require.Zero(t, craftSeedFromRun(t, admitted).DraftRevision)
			}
			head, err := NewCraftDraftHeadStore(db).Read(context.Background(), workspace.Scope, workspace.ID)
			require.NoError(t, err)
			if head.State == craft.DraftHeadEmpty {
				require.ErrorIs(t, advanceErr, craft.ErrBusy)
				require.Equal(t, craft.DraftHeadEmpty, craftSeedFromRun(t, admitted).State)
			} else {
				require.NoError(t, advanceErr)
				require.Equal(t, head.ManifestDigest, craftSeedFromRun(t, admitted).ManifestDigest)
			}
		})
	}
}

func stringsRepeat(char string, count int) string { return strings.Repeat(char, count) }
