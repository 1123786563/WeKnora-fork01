package service

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
)

// Shared fakes for the fork/snapshot/checkpointer/rewind tests (moved out of
// the dropped per-file suites when the upstream test set was pruned).

type fakeSnapshotDeleter struct {
	deleted []string
	err     error
}

func (f *fakeSnapshotDeleter) DeleteSnapshot(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return f.err
}

type fakeForkSessionSnapshotDeleter struct {
	fakeSnapshotDeleter
	forkDeleted []string
	forkErr     error
	lastTenant  uint64
	lastConfig  string
}

func (f *fakeForkSessionSnapshotDeleter) DeleteForkSnapshot(
	_ context.Context, tenantID uint64, sandboxConfigID, snapshotID string,
) error {
	f.lastTenant = tenantID
	f.lastConfig = sandboxConfigID
	f.forkDeleted = append(f.forkDeleted, snapshotID)
	return f.forkErr
}

type fakeShellRunner struct {
	calls   []string
	result  *sandbox.ExecuteResult
	err     error
	timeout time.Duration
	workDir string
}

func (f *fakeShellRunner) ExecShellCommand(
	_ context.Context, _ string, command, workDir string,
	timeout time.Duration, _ map[string]string,
) (*sandbox.ExecuteResult, error) {
	f.calls = append(f.calls, command)
	f.timeout = timeout
	f.workDir = workDir
	return f.result, f.err
}
