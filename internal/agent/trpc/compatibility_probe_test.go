package trpc

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"trpc.group/trpc-go/trpc-agent-go/graph"
	checkpointsqlite "trpc.group/trpc-go/trpc-agent-go/graph/checkpoint/sqlite"
)

func TestCheckpointProbeReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe.db")

	first, err := RunCheckpointProbe(context.Background(), path, true)
	require.NoError(t, err)
	require.False(t, first.Completed)
	require.Equal(t, 0, first.ToolCalls)

	next, err := RunCheckpointProbe(context.Background(), path, false)
	require.NoError(t, err)
	require.True(t, next.Completed)
	require.True(t, next.PendingRestored)
	require.Equal(t, 1, next.ToolCalls)
	require.Equal(t, 2, next.ModelCalls)
}

// A model's Done or runner.completion must not be mistaken for a completed
// graph: interruption also drains and closes the runner event stream.
func TestCheckpointProbeStreamTerminal(t *testing.T) {
	for _, interrupt := range []bool{true, false} {
		t.Run(map[bool]string{true: "interrupted", false: "completed"}[interrupt], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "terminal.db")
			result, err := RunCheckpointProbe(context.Background(), path, interrupt)
			require.NoError(t, err)
			require.Equal(t, !interrupt, result.Completed)
			if interrupt {
				require.Equal(t, 1, result.ModelCalls)
				require.Zero(t, result.ToolCalls)
			} else {
				require.Equal(t, 2, result.ModelCalls)
				require.Equal(t, 1, result.ToolCalls)
			}
		})
	}
}

func TestCheckpointProbeInterruptPayloadAndToolIDAreStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe.db")

	first, err := RunCheckpointProbe(context.Background(), path, true)
	require.NoError(t, err)
	require.False(t, first.Completed)

	interrupted := latestProbeCheckpoint(t, path)
	require.NotNil(t, interrupted.Checkpoint.InterruptState)
	payload, ok := interrupted.Checkpoint.InterruptState.InterruptValue.(map[string]any)
	require.True(t, ok)
	require.Equal(t, probeToolCallID, payload["tool_call_id"])
	require.Equal(t, probeToolName, payload["tool_name"])
	require.Equal(t, probeToolCallID, interrupted.Checkpoint.InterruptState.TaskID)

	next, err := RunCheckpointProbe(context.Background(), path, false)
	require.NoError(t, err)
	require.True(t, next.Completed)

	completed := latestProbeCheckpoint(t, path)
	require.Nil(t, completed.Checkpoint.InterruptState)
	require.Equal(t, probeToolCallID, completed.Checkpoint.ChannelValues[stateKeyToolCallID])
}

func TestSQLiteSaverPendingWritesSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writes.db")
	ctx := context.Background()
	config := graph.CreateCheckpointConfig("pending-lineage", "", "pending-ns")
	checkpoint := graph.NewCheckpoint(
		map[string]any{"plan": "persisted"},
		map[string]int64{"plan": 1},
		nil,
	)
	writes := []graph.PendingWrite{{
		TaskID:   "tool-task",
		Channel:  "tool-result",
		Value:    map[string]any{"ok": true},
		Sequence: 7,
	}}

	db := openProbeDB(t, path)
	saver, err := checkpointsqlite.NewSaver(db)
	require.NoError(t, err)
	storedConfig, err := saver.PutFull(ctx, graph.PutFullRequest{
		Config:        config,
		Checkpoint:    checkpoint,
		Metadata:      graph.NewCheckpointMetadata(graph.CheckpointSourceLoop, 1),
		NewVersions:   checkpoint.ChannelVersions,
		PendingWrites: writes,
	})
	require.NoError(t, err)
	require.NoError(t, saver.Close())

	db = openProbeDB(t, path)
	saver, err = checkpointsqlite.NewSaver(db)
	require.NoError(t, err)
	tuple, err := saver.GetTuple(ctx, storedConfig)
	require.NoError(t, err)
	require.Len(t, tuple.PendingWrites, 1)
	require.Equal(t, "tool-task", tuple.PendingWrites[0].TaskID)
	require.Equal(t, "tool-result", tuple.PendingWrites[0].Channel)
	require.EqualValues(t, 7, tuple.PendingWrites[0].Sequence)
	require.Equal(t, map[string]any{"ok": true}, tuple.PendingWrites[0].Value)
	require.NoError(t, saver.Close())
}

func latestProbeCheckpoint(t *testing.T, path string) *graph.CheckpointTuple {
	t.Helper()
	db := openProbeDB(t, path)
	saver, err := checkpointsqlite.NewSaver(db)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, saver.Close()) })
	tuple, err := graph.NewCheckpointManager(saver).Latest(
		context.Background(), probeLineageID, probeNamespace,
	)
	require.NoError(t, err)
	require.NotNil(t, tuple)
	return tuple
}

func openProbeDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	return db
}
