# Craft RunView OpenCode Client Protocol Report

Status: **client protocol adapter implemented; routing context is covered by package tests, but no OS isolation or Ticket completion is claimed.** No commit was created. Integration worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

## Scope

Changed only the assigned client and focused package test files:

- `internal/modules/agentruntime/agent/opencode/client.go`: added `WithDirectory`, strict canonical non-root absolute path validation, immutable client cloning, request header injection, and redirect policy checks for same-origin and retained directory binding.
- `internal/modules/agentruntime/agent/opencode/client_test.go`: added tests across all nine client request methods, invalid paths, immutable independent clones, legacy unbound behavior, same-origin redirect retention, header stripping, and cross-origin rejection.

No Craft runtime/executor, store, container, migrations, global transport, or UI files were changed by this task. Existing integration worktree edits were preserved.

## RED → GREEN evidence

Before implementing `WithDirectory`, ran:

```text
go test ./internal/modules/agentruntime/agent/opencode -run 'TestWithDirectory' -count=1
```

RED: compilation failed at the expected missing `Client.WithDirectory` method references in the new tests.

After implementation, ran:

```text
gofmt -w internal/modules/agentruntime/agent/opencode/client.go internal/modules/agentruntime/agent/opencode/client_test.go
go test ./internal/modules/agentruntime/agent/opencode -run 'TestWithDirectory' -count=1
go test ./internal/modules/agentruntime/agent/opencode -count=1
git diff --check
```

GREEN: targeted tests passed; full `internal/modules/agentruntime/agent/opencode` package passed; `git diff --check` passed. No test against the actual pinned OpenCode server was added or run in this bounded task.

## Protocol and isolation limits

`x-opencode-directory` is sent consistently on bound client requests, including `/event`. The pinned global `/event` stream is still global; sending the header does **not** make event delivery session- or RunView-scoped. A server-side event router/filter remains a separate requirement.

`WithDirectory` validates shape only. It does not prove the path exists, prevent symlink traversal, isolate filesystem access, or make the shared OpenCode serve process an OS sandbox. Only a trusted server-side caller may select the canonical directory after fresh Task authorization. Do not derive this header from model output or an HTTP request parameter. Per-Run containers/filesystem isolation and T01/T05 acceptance remain pending.

## Checkpoint identity

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged).
- Staged diff: empty at checkpoint.
- This task’s tracked unstaged scope: the two files below.
- The integration worktree was already non-clean at task start; additional unrelated paths appeared in the shared worktree during this task. All unrelated T01/T05/T08/integration changes were preserved; the report’s checkpoint status is the actual post-task `git status --short` in the shared worktree. No shared files were reset or stashed.
- SHA-256:

```text
828eb192fab0612fb0eb904dec45acc3c6cd9108da9b65a378b1781f5aa7b05f  internal/modules/agentruntime/agent/opencode/client.go
babef85a94b21167950200b309fc61f02e0d1fa85a4dad0d5a9be04fd29df61c  internal/modules/agentruntime/agent/opencode/client_test.go
```

The independent reviewer should inspect the two-file uncommitted diff against the unchanged HEAD and these full-content hashes. This task did not create a commit or a commit-based review package.
