package bootstrap

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

func TestAsynqWorkerRegistryRejectsDuplicate(t *testing.T) {
	r := NewAsynqWorkerRegistry(asynq.NewServeMux())
	require.NoError(t, r.Register("task:x", func(context.Context, *asynq.Task) error { return nil }))
	dup := r.Register("task:x", func(context.Context, *asynq.Task) error { return nil })
	require.ErrorContains(t, dup, "already registered")
}

func TestAsynqWorkerRegistryKeepsFirstHandler(t *testing.T) {
	mux := asynq.NewServeMux()
	r := NewAsynqWorkerRegistry(mux)

	first := make(chan struct{}, 1)
	require.NoError(t, r.Register("task:keep", func(context.Context, *asynq.Task) error {
		first <- struct{}{}
		return nil
	}))
	dup := r.Register("task:keep", func(context.Context, *asynq.Task) error { return nil })
	require.ErrorContains(t, dup, "already registered")

	// The rejected duplicate must not have displaced the first handler on the mux.
	require.NoError(t, mux.ProcessTask(context.Background(), asynq.NewTask("task:keep", nil)))
	select {
	case <-first:
	default:
		t.Fatal("first handler was not invoked; the duplicate registration displaced it")
	}
}
