# T19 normal-output Docker provider — independent Task 1 review

Date: 2026-09-24. Scope: Task 1 in `2026-09-24-craft-107-t19-normal-output-plan.md`, reviewed read-only against `2026-09-24-craft-107-t19-normal-output-architecture.md`, the approved Craft web Artifact Spec (stories 34–36 and Implementation/Testing Decisions), CONTEXT.md's Run and Task Budget definitions, ADR-0004, the Task 1 report/checkpoint/patch, and the two new provider files. No OCR run was invoked.

## Evidence and scope

- Integration Worktree HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Both owned files are absent at that commit. Current SHA-256 and sizes match the checkpoint exactly: `docker_normal_exec.go` = `b2a5c361f3e0bd137cc35447be99742cb3ed388acc319adbf371a67353fce694` / 18,442 bytes; `_test.go` = `3bcf711babdf88b23606c8e150544f92b0b788c5f43e6f1d928e8d24f0039152` / 22,538 bytes. The task-local patch names only these two files.
- Independent focused run: `go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` passed. The implementer reports focused race and disposable live Docker passes; I did not repeat the live probe. Its test creates a `NetworkMode: "none"` container, verifies the pinned image identity, uses a unique exact exec ID, checks separated stdout/stderr and one marker side effect, and registers forced removal plus absent-container inspection. The no-egress setting is directly visible in the test, but this probe does not itself attempt outbound connectivity or exercise live response loss.
- The provider's engine interface has `ExecCreate`, `ExecAttach`, and `ExecInspect` only; the sole physical start call is `ExecAttach` at line 223. No detached `ExecStart` call appears in the provider. Create fixes attached stdin/stdout/stderr and non-TTY options; attach errors do not retry, and a process-local `LoadOrStore` rejects another call on the same client. Receipt carries exact container/exec IDs and stdin hash/length, with exact-ID inspection before and after start. The input branch writes once and calls `CloseWrite`; the bounded parser separates non-TTY frames and records output through the sink before callbacks. Tests cover split frames, binary bytes, malformed stream type, quota, sink/callback failure, and attach-response loss.

## Findings

### F1 — High — Early clean stream EOF can be declared complete while the command is still running

**Evidence:** `demuxDockerExecStream` returns success on an `io.EOF` at a frame boundary (lines 469–471). `StartAttachedExecOnce` then sets `TransportComplete` before observing the process (lines 324–334), even if `ObserveAttachedExec` reports `ProcessRunning` (lines 365–366). The tests cover a terminal inspect after EOF, but no EOF-while-running case. **Impact:** A dropped hijacked read side that ends cleanly can be recorded as complete output although later bytes are unavailable. A later exact-ID terminal inspect would not reveal that missing output, risking the spec's false-success/version-promotion prohibition if Task 2 trusts this transport state. **Smallest correction:** Keep output partial/unknown when stream EOF precedes authoritative terminal observation, or establish a separately reviewable end-of-stream guarantee that distinguishes an early clean close; add an EOF-while-running behavior test.

### F2 — High — A blocked sink can outlive the bounded drain and race with returned outcome

**Evidence:** The stream goroutine calls the supplied sink synchronously via `writeStream` (lines 250–255 and 433–436). Cancellation closes the hijacked connection and waits at most `dockerExecDrainGrace` (lines 299–322), then reads `writer.bytesWritten`, `writer.truncated`, and `writer.err` (lines 324–335). A sink blocked in storage cannot be interrupted by closing the stream; it may continue and mutate those fields after the provider returns. **Impact:** The claimed bounded drain does not bound the worker, creates a data race on outcome fields, and permits a late output append/callback after an operation has been classified partial or unavailable. This is material for durable chunk order and recovery consistency. **Smallest correction:** Give the writer a synchronized, frozen snapshot on timeout and prevent any post-return callback; require a cancellation-aware sink contract or otherwise retain ownership until it exits. Add a blocking-sink cancellation test under `-race`.

### F3 — Medium — The supplied typed callback can panic the process

**Evidence:** `writeStream` invokes `w.callback` directly at lines 440–444, whereas the legacy callback is explicitly panic-wrapped at lines 446–462. No test covers a panic from the typed callback. **Impact:** A callback panic in the stream goroutine escapes the provider's typed partial/error result and can terminate the host process after the one-send claim, leaving the operation to recovery without its intended evidence. **Smallest correction:** Recover callback panics and return a typed partial error, as already done for the legacy callback; add a callback-panic test.

## Verdicts

**Spec compliance: conditional fail for Task 1.** The one-attach transport, inert receipt, exact-ID checks, stdin half-close, non-TTY demux, quota and explicit error states align with the Task 1 brief. F1 leaves a path to falsely complete output. The provider is correctly not production-routed; Task 2 must supply durable input/output and claim integration, and Task 3 must prove the joined build/version gate.

**Code quality: fail pending F1–F3.** The focused tests pass, and the live test is a useful single-start/cleanup proof. The clean-EOF and blocked-sink cases need behavioral coverage and correction before downstream code can safely consume `TransportComplete` or the returned byte counts.
