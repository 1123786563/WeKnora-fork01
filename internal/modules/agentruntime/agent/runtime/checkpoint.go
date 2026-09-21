package runtime

import "context"

// CheckpointStore extends RunStore with the complete durable saver operations.
// UpdateCheckpoint runs the mutation under the same transactional fence as the
// write. The callback must be pure: it receives detached data and may run again
// after a caller retry. nextSeq is reserved by that transaction for new records.
type CheckpointStore interface {
	RunStore
	UpdateCheckpoint(context.Context, Fence, string, string,
		func(current *CheckpointRecord, nextSeq int64) (*CheckpointRecord, error)) error
	ListCheckpoints(context.Context, RunKey, string) ([]CheckpointRecord, error)
	DeleteCheckpoints(context.Context, Fence, string) error
	ValidateCheckpointCalls(context.Context, RunKey, []string, map[string]bool) error
}
