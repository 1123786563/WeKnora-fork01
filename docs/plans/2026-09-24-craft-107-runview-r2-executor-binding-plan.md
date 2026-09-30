# T01 RunView R2: bind every OpenCode operation to one persisted Run

> **For Codex:** Execute this SDD task with RED→GREEN→REFACTOR, exact uncommitted Review Package and independent read-only review. It is a prerequisite; it does not enable production without the R5 assembly and Linux image gate.

**Sources:** approved Craft Spec #107/T01, `runview-runtime-adapter-plan.md`, reviewed R1 runtime/provider and inventory, pinned host-only protocol smoke. Integration Worktree original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and all owned-file pre-task content/hashes. No commits.

## Global Constraints

`taskId=sessionId` and Task workspace ownership remain; OpenCode session/home/directory are per **Run**. Only the authenticated persisted `task.Fence` plus Task scope may select a RunView. No fallback to `craft_workspaces.oc_session_id` for Craft Execute/Observe/Abort. Prompt message ID is persisted before submission; a failed pre-flight observation remains unknown, never a blind retry. `/event` is global in the local lock and cannot be trusted to be directory-filtered; a per-Run process is required at assembly, and event normalizer still filters session/message IDs. This task must not claim Linux OS isolation or Run A→B acceptance.

## Review Focus

Same persisted Run binding across Execute/Observe/Abort and retry; cross-Run/sibling session substitution; missing resolver fail closed; no old Task-level session fallback; exact message-ID preflight and one prompt; foreign event discarded; error paths preserve unknown not resubmit.

## Task 1 — per-call resolver contract and executor use

**Depends on:** R1 provider fix1 scoped PASS. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/modules/agentruntime/agent/opencode/executor.go`, focused executor tests and a new resolver contract file in that package if needed. Do not edit `client.go` unless a demonstrated pinned endpoint gap is reported first. **Consumes:** opaque resolved Run binding (directory-bound client and persisted OpenCode session ID), `craft.Task` scope/fence, existing store PrepareTask. **Produces:** `Execute`, `Observe` and `Abort` on exactly that client/session; no use of Task workspace's OpenCode session ID.

1. RED tests: two Runs under one Task resolve to different fake clients/sessions; missing resolver, mismatched Run binding or unbound session stops before Prompt/Events/Abort; retry with persisted message ID observes and does not submit again; Observe/Abort use the same resolved Run; foreign session/message event is discarded. Assert no Task-level session fallback even if old workspace field is populated.
2. Introduce a narrow resolver interface and explicit constructor/wiring contract. Retain `boundWorkspace` only for Task ownership verification. Pass the resolved client into snapshot, SSE, Prompt, Abort and settle paths; do not cache a previous Run's client on shared `Executor`. Fail closed when resolver absent or identity inconsistent.
3. Run focused and full `opencode` package tests, race focused checks where useful, gofmt and diff check. Save exact task-local patch, pre/post hashes, test output and report.

**Acceptance:** every Run operation resolves one persisted Run-owned session and client; old Task OpenCode session never authorizes Craft activity. **Failure handling:** if the existing `craft.Task` lacks the necessary authenticated Run identity, stop and request the exact upstream interface seam rather than using a prompt/client path.
