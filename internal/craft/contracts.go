// Package craft holds the stable business contracts and pure rules shared by
// the Craft workbench tasks: workspace binding, sub-execution delegation,
// immutable artifact versions and observation results. The package owns no
// HTTP, database or main-graph execution concerns.
package craft

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
)

// Sentinel errors returned by Craft ports. Callers classify with errors.Is;
// port errors and the recovery program's errors are mapped at assembly
// boundaries and must never be re-created with errors.New at call sites.
var (
	// ErrConflict reports an incompatible idempotent request or a revision
	// race, for example the same delegation call with different arguments.
	ErrConflict = errors.New("craft conflict")
	// ErrForbidden reports a scope or identity check failure: the resource
	// exists but the authenticated user, session or worker may not touch it.
	ErrForbidden = errors.New("craft forbidden")
	// ErrNotFound reports that no record exists in the requested scope.
	ErrNotFound = errors.New("craft not found")
	// ErrUnknown reports an execution outcome that could not be determined
	// and must stay pending on the main ToolCall instead of being finalized.
	ErrUnknown = errors.New("craft outcome unknown")
	// ErrInvalidInput reports malformed or incomplete request data.
	ErrInvalidInput = errors.New("craft invalid input")
	// ErrBusy reports that the workspace already has an active sub-execution
	// and serial modification rejects the new one.
	ErrBusy = errors.New("craft busy")
	// ErrUnsupported reports a capability the current engine does not offer.
	ErrUnsupported = errors.New("craft unsupported")
)

// Scope is the server-derived execution identity: authenticated tenant and
// user bound to one session. It never comes from client input.
type Scope struct {
	TenantID  uint64
	UserID    string
	SessionID string
}

// Workspace binds one session to one sandboxed OpenCode runtime so a second
// round on the same artwork reuses the same working directory. Revision
// supports compare-and-swap updates by the workspace service.
type Workspace struct {
	ID string
	Scope
	SandboxID, Generation, OpenCodeSessionID, RuntimeDigest string
	Revision                                                int64
}

// Input is one immutable upload referenced by a delegation prompt.
type Input struct {
	Ref, Name, SHA256, CitationID string
	Bytes                         int64
}

// Task is the durable delegation request executed by the OpenCode runtime
// under the main run's fence. PromptMessageID is assigned by PrepareTask.
type Task struct {
	ID, ToolCallID, Prompt, PromptMessageID, RequestHash string
	Scope
	Fence        runtime.Fence
	WorkspaceID  string
	Inputs       []Input
	SkillDigests []string
	Deadline     time.Time
}

// Check is one verification performed on a produced artifact. Status is
// passed, failed or not_run.
type Check struct {
	Name, Status, Detail string
}

// File is one artifact file handle inside a version or result.
type File struct {
	Path, Ref, SHA256, MIME string
	Bytes                   int64
}

// Version is an immutable, Run-linked snapshot of a workspace's files.
type Version struct {
	ID, WorkspaceID, RunID, Kind string
	Files                        []File
	Checks                       []Check
}

// Observation is the OpenCode session state projected by Observe.
type Observation struct {
	SessionID, PromptMessageID, AssistantParentID, Finish string
	Completed, Idle, PendingTool, Aborted                 bool
}

// Result is the terminal outcome of one delegated Task. Status is only
// succeeded, failed, canceled or unknown; unknown keeps the main ToolCall
// waiting for resolution.
type Result struct {
	TaskID, Status, Summary string
	Files                   []File
	Checks                  []Check
}

// Store persists workspace bindings, delegations and their results. All
// methods are scope-guarded and idempotent by logical identity.
type Store interface {
	GetWorkspace(context.Context, Scope) (Workspace, error)
	PutWorkspace(context.Context, Workspace, int64) (Workspace, error)
	PrepareTask(context.Context, Task) (Task, error)
	GetTask(context.Context, Scope, string) (Task, error)
	SaveResult(context.Context, runtime.Fence, Result) error
	GetResult(context.Context, Scope, string) (Result, error)
}

// Executor runs the delegated sub-execution inside the sandbox runtime.
type Executor interface {
	Execute(context.Context, Task) (Result, error)
	Observe(context.Context, Task) (Observation, error)
	Abort(context.Context, Task) error
}
