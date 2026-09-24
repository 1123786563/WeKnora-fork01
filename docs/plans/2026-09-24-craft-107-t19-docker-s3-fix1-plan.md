# Craft #107 T19 Docker S3 Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close S3-R1, S3-R2, and S3-R3 from the independent review of the restricted outputless Docker start path.

**Architecture:** Keep the S2 durable prepare/bind/claim coordinator as the only authority to start. Add a bounded same-receipt terminal wait and a post-claim cancellation fence. Prove the physical send limit through the service/coordinator and a disposable real Docker execution, including response loss.

**Tech Stack:** Go, Moby Docker client, SQLite/PostgreSQL repository tests, disposable Docker engine.

**Spec:** `docs/specs/` Craft web Artifact approved Spec #107; `docs/plans/2026-09-24-craft-107-t19-docker-s3-provider-design.md`; `docs/plans/2026-09-24-craft-107-t19-docker-s3-outputless-task1-review.md`.

## Global Constraints

- T19 chargeable sends require one durable claim before the physical Docker `ExecStart`; ambiguous claimed outcomes retain hold/fence and never retry.
- This capability is outputless only: reject stdin, stdout, stderr, streaming, and callbacks before any hold or Docker call. Never present unavailable output as an empty successful stream.
- Same container and exec IDs must be checked on every observe/wait; false/zero inspect is not proof of completion.
- Keep normal production routing disabled. No commit, push, or release-image publication is authorized.
- Execute serially in the integration Worktree; preserve unrelated edits. Use an exact pre/post file checkpoint and task-local patch for review.

## Review Focus

- Cancellation immediately after the durable claim must not call the provider start callback.
- Cancellation or deadline during Wait must remain unknown with durable hold/fence.
- An inspect failure or never-observed running state must not become a successful zero exit.
- Racing/restarted Start and Resume with a lost response after physical side effect must cause at most one physical callback.
- Real Docker probe must traverse the coordinator and bind the same exec ID; direct adapter-only marker proof is insufficient.

---

### Task 1: Bounded outputless terminal result and cancellation fence

**Depends on:** S3 Task 1 checkpoint; no other S3 Fix task.

**Owner / validator:** `backend_implementer` / `backend_validator`; independent `reviewer` follows verification.

**Owned files:** `internal/modules/execution/sandbox/docker_restricted_exec.go`, `_test.go`; `internal/application/service/craft_docker_restricted_exec.go`, `_test.go`. No other business source or tests.

**Consumes / produces:** Existing exact Docker receipt and S2 `Claim`/`Observe` APIs; produces `Wait` on that same receipt with an explicit terminal outputless result (exit code and duration, stdout/stderr unavailable), and an unknown outcome for cancellation/deadline/inspect failure.

- [ ] Capture exact preimage hashes and write failing tests: immediate cancellation after a successful claim yields zero `StartOutputlessExec` calls and an unknown retained claim; fast terminal, slow terminal, canceled wait, deadline, inspect error, and false-zero observations.
- [ ] Run focused RED tests and save failure output. Implement the smallest post-claim `ctx.Err()` fence before provider start and the bounded same-ID wait. Require positive running provenance before accepting terminal exit; never clear hold/fence for unknown.
- [ ] Run focused normal and race tests and `git diff --check`; save exact command/output, checkpoint hashes, patch, and report. If Docker API cannot provide authoritative duration, record that limitation and leave R1 open rather than fabricate it.

**Acceptance / failure handling:** R1 and R3 observable cases pass. On a post-start cancellation race, classify unknown and do not resend. Independent reviewer must separately pass Spec and quality before Task 2 uses the checkpoint.

### Task 2: Coordinator-to-physical-start proof

**Depends on:** Task 1 independently reviewed.

**Owner / validator:** `backend_implementer` / `backend_validator`; independent `reviewer` follows verification.

**Owned files:** `internal/application/service/craft_docker_restricted_exec_test.go`; if necessary, a new focused integration test in the same service package. No production edits unless an independently reviewed defect is first recorded and replanned.

**Consumes / produces:** Task 1 outputless service path and S2 durable coordinator; produces deterministic tests and a disposable Docker probe/report proving at most one physical `ExecStart` side effect through Prepare → Create → Bind → Claim → Start.

- [ ] Capture exact Task 1 checkpoint hashes; write failing coordinator-level tests for concurrent Start/Resume, bind failure, restart after bind, restart after claim, and callback side effect followed by response loss. Assert one physical callback, exact receipt, and retained `intent`/hold/fence on ambiguous cases.
- [ ] Run the RED suite; adjust only test harness to make the ordering deterministic. Run normal/race GREEN suites. If a production defect appears, stop and create a bounded source-fix task rather than silently widen ownership.
- [ ] Run one bounded disposable Docker integration proof through the coordinator using the exact receipt; record daemon/client versions, container/exec IDs, one marker side effect, response-loss replay attempt, container cleanup and logs. Do not infer a one-send claim from a direct adapter call.
- [ ] Save report/checkpoint/patch; independent reviewer checks proof and closes S3-R2 only if the physical send callback and durable claim are joined by evidence.

**Acceptance / failure handling:** A crash/ambiguous send never retries physical start. Failure to run the disposable Docker probe leaves S3-R2 open; S3 and T19 remain unverified and production routing stays disabled.
