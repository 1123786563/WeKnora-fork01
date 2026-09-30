# Craft #107 T19 Normal Output Docker Exec Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support ordinary output-bearing Docker commands under a durable one-send claim without replaying an ambiguous start or falsely completing a build after output loss.

**Architecture:** Persist an inert exact ExecCreate receipt and one S2 claim, then use one hijacked `ExecAttach` as the sole physical start. Demultiplex into a durable bounded append/seal store; expose live output only by separately reading that store with a cursor. Direct caller callbacks are unsupported by the reviewed provider Fix3 contract. Inspect exact receipt for terminal state. Missing stream bytes remain typed partial/unavailable and block promotion.

**Tech Stack:** Go, pinned Moby client v0.5.1, S2 coordinator, SQLite/PostgreSQL, disposable Docker.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-architecture.md`; approved Craft web Artifact Spec #107/T19; S2/S3 reviewed interfaces.

## Global Constraints

- Budget admission/hold and Run fence precede Docker create. One durable claim precedes exactly one `/exec/{id}/start` POST; `ExecAttach` is itself a start. Never call detached `ExecStart` then `ExecAttach`, and never reattach after ambiguity.
- Normal command output and stdin require a distinct complete/partial/unavailable contract. Direct caller callbacks are rejected before Docker create; later live projection tails persisted chunks through a separate cursor API. No empty string may stand for lost output. Missing output blocks affected build promotion while the prior Version remains readable.
- Preserve exact tenant/Task/Run/activity/receipt scope, output quota and retention. Persist chunks before callback, and never log raw stdin/output.
- Existing restricted outputless path and production routing remain unchanged until all Tasks independently pass. No commit/push or shared stash; integration Worktree with exact pre/post checkpoints.

## Review Focus

- Any response loss after one physical start cannot trigger another attach/start.
- Non-TTY stdout/stderr frames, split headers, binary bytes and malformed frames are handled without fabricated completeness.
- Stdin is bounded and half-closed once; resume uses the original hash-bound input.
- Stream EOF is not process success; exact-ID inspect and durable positive start evidence are required.
- Output-store/callback failure, cancellation and daemon loss keep explicit partial/unknown state and the budget hold/fence.

---

### Task 1: Normal-output Docker provider transport

**Depends on:** S2 claim and S3 one-send reviews PASS. **Owner:** `backend_implementer`. **Validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** new `internal/modules/execution/sandbox/docker_normal_exec.go`, `_test.go` only. No S2/S3 service/repository edits.

**Consumes / produces:** Pinned Moby client, immutable Docker handle/receipt; produces an inert attached-capable ExecCreate, one `ExecAttach` transport start, bounded stdin half-close, non-TTY demux to a supplied chunk sink, exact-ID observe. It does not itself claim budget or advertise `RemoteExecInitiator`.

- [ ] Capture preimage and RED fake-client tests: options fixed at create, exactly one attach and zero detached start, wrong container/exec ID rejected before start, response-loss after side effect never internally retries, stdin once/half-close, split/malformed multiplex frames, stdout/stderr separation, output quota and callback/sink failure.
- [ ] Implement distinct provider interface/types and one start action. Bound context, memory and input/output bytes. Return typed stream completeness and exact receipt; transport ambiguity is unknown, never success. Keep exact inspect state separate from stream EOF.
- [ ] Run focused normal/race and compile tests; if feasible one disposable Docker transport probe with exact exec ID and marker once, preserving no-egress setup and cleanup. Save checkpoint/report/patch. Independent reviewer checks no accidental second start or unsupported full-result promise.

**Acceptance / failure handling:** Safe transport with no production route; provider-only tests do not close Task Budget or normal Craft completion. On SDK limitation, report exact API blocker without fallback to legacy `Exec`.

### Task 2: Durable output and application coordinator

**Depends on:** Task 1 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator` then independent `reviewer`.

**Owned files:** new application service/output-store files and focused tests; existing S2 coordinator/repository only under a separately recorded narrow interface change. No production routing file yet.

**Consumes / produces:** S2 Prepare/Bind/Claim and Task 1 provider, immutable stdin/output policy, operation-scoped chunk store; produces typed complete/partial/unavailable result, persisted chunks before callback, same-ID inspect/positive-start evidence and once-only charge resolution.

- [ ] RED claim races, bind failure, after-claim cancellation, response loss, chunk-store failure, live-reader reconnect cursor, stdin hash mismatch, stdout/stderr interleave, quota/truncation and terminal inspect ambiguity. A blocking live reader cannot block Docker transport or append/seal; a reader sees only committed chunks.
- [ ] Implement immutable input staging, output append sequence and after-claim one-start permission; never reattach on ResumeBound. Provide a separate bounded cursor reader for live projection after durable append, with no arbitrary callback invoked from the provider. Preserve hold/fence on unknown, separate process completion from output completeness and charge settlement.
- [ ] Run SQLite/PostgreSQL targeted/race tests and disposable Docker through coordinator; record one physical start, output bytes, response-loss classification, cleanup, checkpoint and independent review.

**Acceptance / failure handling:** No duplicate physical start or falsely complete output. Missing stream evidence cannot become ordinary `RemoteExecResult`.

### Task 3: Caller routing and joined budget/build proof

**Depends on:** Task 2 reviewed PASS plus T01 RunView and T19 E0 gates as applicable. **Owner:** backend specialist under a new bounded Brief; **validator:** backend validator + independent reviewer.

**Owned files:** central production DI/routing and focused caller tests allocated after Task 2. **Produces:** only marked Docker normal commands use the new operation; E2B/Cube/legacy paths remain explicit and fail closed where unsupported.

- [ ] RED normal build success, partial output/no promotion, old Version visibility, budget exhaustion, reconnect and no replay.
- [ ] Wire reviewed operation with typed caller contract and default-off gate; no mixed legacy path for marked Docker activities.
- [ ] Run joined real Docker and Task Budget verification, checkpoint, independent review and final OCR full-scope gate.
