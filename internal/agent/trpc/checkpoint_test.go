package trpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func checkpointDB(t *testing.T) (*gorm.DB, *repository.AgentRunStore, agentruntime.Fence) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "checkpoint.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	// Minimal admission fixtures; the checkpoint/journal tables are the exact production migration.
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id TEXT, tenant_id INTEGER, user_id TEXT, deleted_at DATETIME);
		INSERT INTO sessions VALUES ('s1',1,'u1',NULL); INSERT INTO sessions VALUES ('s2',1,'u1',NULL);`).Error)
	raw, err := os.ReadFile("../../../migrations/sqlite/000014_agent_runs.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(raw)).Error)
	for _, run := range []struct{ id, session string }{{"r1", "s1"}, {"r2", "s2"}} {
		require.NoError(t, db.Exec(`INSERT INTO agent_runs
			(tenant_id,run_id,session_id,owner_id,request_id,assistant_message_id,request_hash,snapshot,deadline)
			VALUES (1,?,?,'u1',?,'a1','hash','{}',?)`, run.id, run.session, run.id, time.Now().Add(time.Hour)).Error)
	}
	store := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: "r1"}
	fence, err := store.Claim(context.Background(), key, "worker", time.Minute)
	require.NoError(t, err)
	return db, store, fence
}

func TestCheckpointPersistsFullStateWritesAndMetadataAcrossHandles(t *testing.T) {
	db, store, fence := checkpointDB(t)
	require.NoError(t, db.Exec(`INSERT INTO agent_tool_calls
		(tenant_id,run_id,call_id,call_seq,tool_name,tool_identity,args_hash,args,status,result)
		VALUES (1,'r1','c1',1,'lookup','lookup','hash','{}','succeeded','"done"'),
		(1,'r1','c2',2,'lookup','lookup','hash','{}','planned',NULL)`).Error)
	saver := NewCheckpointSaver(store, fence)
	state := State{
		Version: 1, PendingCallIDs: []string{"c1", "c2"}, NextCallIndex: 1, AppliedCallIDs: map[string]bool{"c1": true},
		InputCursor: 9007199254740993, ModelAttemptID: "m1",
		Messages: []model.Message{model.NewAssistantMessage("planned")},
	}
	cp := graph.NewCheckpoint(map[string]any{StateKey: state}, map[string]int64{StateKey: 3}, nil)
	cp.NextNodes = []string{"tools"}
	cp.InterruptState = &graph.InterruptState{
		NodeID: "tools", TaskID: "c2",
		InterruptValue: map[string]any{"reason": "unknown"},
	}
	metadata := graph.NewCheckpointMetadata(graph.CheckpointSourceInterrupt, 3)
	metadata.Extra["model_attempt_id"] = "m1"
	config, err := saver.PutFull(context.Background(), graph.PutFullRequest{
		Config:     CheckpointConfig(fence.RunKey, ""),
		Checkpoint: cp, Metadata: metadata, NewVersions: cp.ChannelVersions,
		PendingWrites: []graph.PendingWrite{{TaskID: "c1", Channel: "tool-result", Value: "done", Sequence: 4}},
	})
	require.NoError(t, err)
	require.NoError(t, saver.PutWrites(context.Background(), graph.PutWritesRequest{
		Config: config, TaskID: "c2", TaskPath: "tools",
		Writes: []graph.PendingWrite{{Channel: "tool-result", Value: "pending", Sequence: 5}},
	}))
	require.NoError(t, saver.Close())
	reopened := NewCheckpointSaver(repository.NewAgentRunStore(db), fence)
	tuple, err := reopened.GetTuple(context.Background(), config)
	require.NoError(t, err)
	require.Equal(t, state, tuple.Checkpoint.ChannelValues[StateKey])
	require.Equal(t, "c2", tuple.Checkpoint.InterruptState.TaskID)
	require.Equal(t, metadata, tuple.Metadata)
	require.Len(t, tuple.PendingWrites, 2)
	require.Equal(t, "c2", tuple.PendingWrites[1].TaskID)
	require.Equal(t, int64(5), tuple.PendingWrites[1].Sequence)
	require.Equal(t, []string{"tools"}, tuple.Checkpoint.NextNodes)
}

func TestCheckpointFencesVersionsIsolationAndMissingToolLogs(t *testing.T) {
	db, store, fence := checkpointDB(t)
	saver := NewCheckpointSaver(store, fence)
	ctx := context.Background()
	cp := graph.NewCheckpoint(map[string]any{StateKey: State{Version: 1}}, nil, nil)
	config, err := saver.Put(ctx, graph.PutRequest{Config: CheckpointConfig(fence.RunKey, ""), Checkpoint: cp})
	require.NoError(t, err)
	_, err = saver.Get(ctx, graph.CreateCheckpointConfig("r2", cp.ID, CheckpointNamespace(fence.RunKey)))
	require.Error(t, err)
	_, err = saver.Get(ctx, graph.CreateCheckpointConfig("r1", cp.ID, "graph-v2"))
	require.Error(t, err)
	otherFence, err := store.Claim(ctx, agentruntime.RunKey{TenantID: 1, RunID: "r2"}, "worker", time.Minute)
	require.NoError(t, err)
	other, err := NewCheckpointSaver(store, otherFence).Get(ctx, CheckpointConfig(otherFence.RunKey, ""))
	require.NoError(t, err)
	require.Nil(t, other)
	bad := graph.NewCheckpoint(map[string]any{StateKey: State{Version: 2}}, nil, nil)
	_, err = saver.Put(ctx, graph.PutRequest{Config: config, Checkpoint: bad})
	require.Error(t, err)
	missingState := State{Version: 1, PendingCallIDs: []string{"absent"}}
	missing := graph.NewCheckpoint(map[string]any{StateKey: missingState}, nil, nil)
	missingConfig, err := saver.Put(ctx, graph.PutRequest{Config: config, Checkpoint: missing})
	require.NoError(t, err)
	_, err = saver.Get(ctx, missingConfig)
	require.ErrorContains(t, err, "tool")
	require.NoError(t, db.Exec(`UPDATE agent_runs SET epoch=epoch+1 WHERE run_id='r1'`).Error)
	_, err = saver.Put(ctx, graph.PutRequest{Config: config, Checkpoint: cp})
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	err = saver.PutWrites(ctx, graph.PutWritesRequest{
		Config: config, TaskID: "old",
		Writes: []graph.PendingWrite{{Channel: "x", Value: 1}},
	})
	require.ErrorIs(t, err, agentruntime.ErrLeaseLost)
	require.ErrorIs(t, saver.DeleteLineage(ctx, "r1"), agentruntime.ErrLeaseLost)
}

func TestCheckpointListParentAndDelete(t *testing.T) {
	_, store, fence := checkpointDB(t)
	saver := NewCheckpointSaver(store, fence)
	ctx := context.Background()
	first := graph.NewCheckpoint(map[string]any{StateKey: State{Version: 1}}, nil, nil)
	config, err := saver.Put(ctx, graph.PutRequest{
		Config: CheckpointConfig(fence.RunKey, ""), Checkpoint: first,
		Metadata: graph.NewCheckpointMetadata("input", -1),
	})
	require.NoError(t, err)
	next := graph.NewCheckpoint(map[string]any{StateKey: State{Version: 1}}, nil, nil)
	second, err := saver.Put(ctx, graph.PutRequest{
		Config: config, Checkpoint: next,
		Metadata: graph.NewCheckpointMetadata("loop", 0),
	})
	require.NoError(t, err)
	loaded, err := saver.GetTuple(ctx, second)
	require.NoError(t, err)
	require.Equal(t, config, loaded.ParentConfig)
	all, err := saver.List(ctx, CheckpointConfig(fence.RunKey, ""), &graph.CheckpointFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, next.ID, all[0].Checkpoint.ID)
	older, err := saver.List(ctx, CheckpointConfig(fence.RunKey, ""), &graph.CheckpointFilter{Before: second})
	require.NoError(t, err)
	require.Len(t, older, 1)
	require.Equal(t, first.ID, older[0].Checkpoint.ID)
	require.NoError(t, saver.DeleteLineage(ctx, "r1"))
	all, err = saver.List(ctx, CheckpointConfig(fence.RunKey, ""), nil)
	require.NoError(t, err)
	require.Empty(t, all)
}

func TestCheckpointRejectsPersistedUnknownEnvelope(t *testing.T) {
	db, store, fence := checkpointDB(t)
	saver := NewCheckpointSaver(store, fence)
	cp := graph.NewCheckpoint(map[string]any{StateKey: State{Version: 1}}, nil, nil)
	req := graph.PutRequest{Config: CheckpointConfig(fence.RunKey, ""), Checkpoint: cp}
	config, err := saver.Put(context.Background(), req)
	require.NoError(t, err)
	var raw string
	require.NoError(t, db.Table("agent_run_checkpoints").Select("state").Scan(&raw).Error)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &envelope))
	envelope["graph_version"] = json.RawMessage(`"future"`)
	changed, err := json.Marshal(envelope)
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_run_checkpoints SET state=?", string(changed)).Error)
	_, err = saver.Get(context.Background(), config)
	require.ErrorContains(t, err, "version")
}
