package router

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

// The Lite executor must satisfy the shared worker composition contract.
var _ bootstrap.TaskHandlerRegistry = (*SyncTaskExecutor)(nil)

func TestSyncTaskExecutorRegisterDuplicate(t *testing.T) {
	executor := NewSyncTaskExecutor()

	first := make(chan string, 1)
	require.NoError(t, executor.Register("test:duplicate", func(context.Context, *asynq.Task) error {
		first <- "first"
		return nil
	}))

	second := make(chan string, 1)
	err := executor.Register("test:duplicate", func(context.Context, *asynq.Task) error {
		second <- "second"
		return nil
	})
	require.ErrorContains(t, err, "already registered")

	// The first handler must survive the rejected duplicate: enqueueing the
	// task dispatches to the original registration, never the duplicate.
	task := asynq.NewTask("test:duplicate", nil)
	info, err := executor.Enqueue(task)
	require.NoError(t, err)
	require.NotEmpty(t, info.ID)

	select {
	case got := <-first:
		require.Equal(t, "first", got)
	case got := <-second:
		t.Fatalf("duplicate handler displaced the first registration (ran %q)", got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sync task")
	}
}
