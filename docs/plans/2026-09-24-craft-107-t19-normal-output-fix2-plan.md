# Craft #107 T19 Normal Output Provider Fix 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two remaining cancellation findings in the independently reviewed normal-output provider Fix 1 checkpoint.

**Architecture:** Keep one physical Docker attach and a frozen returned outcome. Decouple caller projection callbacks from the lock required to seal result state, and ensure cancellation remains partial even when it arrives after stream drain. Do not promise bounded termination for arbitrary blocking function callbacks without an enforceable interface contract.

**Tech Stack:** Go, pinned Moby client, focused deterministic and race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-fix1-task1-review.md`; `docs/plans/2026-09-24-craft-107-t19-normal-output-architecture.md`; approved Craft Spec #107 stories 34–36.

## Global Constraints

- Exactly one `ExecAttach` after a durable claim; no detached Start, second attach, or legacy Exec fallback.
- No false complete after cancellation, deadline, unknown process state, partial stream, sink failure or callback failure.
- Bounded provider return and no caller callback or durable output append after return must both hold for every accepted callback/sink form. If an arbitrary function callback cannot satisfy both, explicitly reject it before Docker create/start or replace it with a verifiable cancellable/sealable contract. Never silently discard accepted output.
- No production routing, application coordinator edits, commit or push. Record preimage, exact postimage hash, focused test output and an incremental review package.

## Review Focus

- A typed and legacy callback blocked after entry cannot hold the seal mutex across cancellation; a late release cannot perform work after return.
- A deterministic cancellation delivered after stream drain but before terminal observation is always partial with the cancellation cause.
- Prior Fix 1 protections for clean EOF while Running, blocked conforming sink, callback panic, one attach, and real Docker terminal behavior remain intact.

---

### Task 1: Bound callback cancellation and close the late-cancel race

**Depends on:** Normal-output Fix 1 Task 1 independent FAIL; no other Ticket. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `internal/modules/execution/sandbox/docker_normal_exec.go`, `internal/modules/execution/sandbox/docker_normal_exec_test.go` only.

**Consumes / produces:** Existing `StartAttachedExecOnce`, callback/sink API and exact-ID inspection; produces enforceable callback lifetime and cancellation-consistent frozen `DockerNormalExecResult`.

- [ ] Capture current two-file hashes and prior review evidence. RED: enter a blocking typed callback and a blocking legacy callback; cancel, require bounded provider return and frozen result, release callback later, assert no accepted callback work/output after return under `-race`. RED: cancel exactly after stream/input drain and before terminal inspect resolves; assert partial and cancellation cause.
- [ ] Rework writer/callback synchronization or accepted callback contract so provider freeze does not wait indefinitely for arbitrary caller function code. Explicitly reject unsupported callback forms before physical start if they cannot meet both lifetime guarantees; cover that rejection in tests. Preserve a conforming way to project live output when possible.
- [ ] Check `startCtx.Err()` after drain and before final completeness/return, including an inspection race. Preserve exact one-attach observation, sealed sink and no retry.
- [ ] Run `gofmt`, focused `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1`, compile/package checks, and real Docker probe only if transport changed. Save report, pre/post hashes, test evidence and incremental patch for independent Spec and quality review.

**Acceptance / failure handling:** Both findings F1 High and F2 Medium closed at reviewed exact hashes without regressing previously closed findings. If callback semantics cannot be made enforceable inside these owned files, stop with a precise API/interface block and proposed Task 2 dependency; do not claim T19 normal output ready.
