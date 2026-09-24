# T19 normal-output provider Fix1 — independent Task 1 review

Date: 2026-09-24. Read-only review of the Fix1 plan, prior Task 1 review, Fix1 report/checkpoint/patch, the two owned provider files, approved Craft web Artifact Spec (stories 34–36 and implementation/testing decisions), CONTEXT.md, ADR-0004, and the normal-output architecture brief. No OCR run, source change, test edit, or commit was made.

## Scope and verification

- Integration Worktree HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Current SHA-256 matches the Fix1 checkpoint: `docker_normal_exec.go` `40f1c21f53f39260f46515f33632aafa66aa2e74b7de96c69dfd063d285c0981` (20,662 bytes) and `_test.go` `7cd44c1686f033b58f4d0eafa986e353ee67f42ee013dc71273723df9a67b027` (28,120 bytes). The task-local patch changes only these two files from the prior checkpoint hashes. Neither file is in HEAD; both remain untracked in the integration Worktree.
- Independently ran `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1`: PASS. Independently reran the live test at the matching current hashes with `DOCKER_HOST=unix:///var/run/docker.sock CRAFT_T19_NORMAL_OUTPUT_DOCKER_TEST=1 go test -v ./internal/modules/execution/sandbox -run '^TestDockerNormalExecRealPinnedNoEgressTransport$' -count=1`: PASS. It used pinned Linux/arm64 image `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, disposable container `ef784ee8ef9a717f27740a3dedd5272ce94e176f6466565a4b0a430f14c76314`, exact exec `13006c740a632397ed666062d33c510d0cb686b72d7c9354221e3b85386a4dd4`, one marker, separated output, terminal exit zero, and verified container removal. The test configures `NetworkMode: "none"`; it does not probe outbound connectivity.
- Prior F1 is closed for the reviewed path: `StartAttachedExecOnce` changes clean EOF to partial with `ErrDockerNormalExecNotTerminal` when exact-ID observation remains Running/Unknown (lines 341–367); the Running test asserts one attach and zero detached starts. Prior F2's blocked **sink** case is protected by synchronized freeze/snapshot followed by a contractually linearizable `Seal` (lines 437–519); the race test releases the sink after provider return and checks no append or callback. Prior F3 is closed for typed callback panics via `callDockerNormalTypedCallback` (lines 483–489, 521–528) and a subprocess test. The engine interface still has only create, attach, and inspect; `ExecAttach` at line 231 is the sole start call, and the lost-response test asserts no retry.

## Findings

### F1 — High — A blocked callback defeats bounded cancellation

**Evidence / affected symbols:** `dockerNormalExecBoundedWriter.writeStream` holds `w.mu` while invoking both caller callbacks (lines 475–499). On cancellation, `StartAttachedExecOnce` waits only `dockerExecDrainGrace` for the stream (lines 307–320), but then `freezeAndSeal` unconditionally waits for that same mutex (lines 332 and 508–512). A callback that waits indefinitely therefore keeps the stream worker inside `writeStream` and prevents the provider from returning after its advertised drain bound. The new blocked-sink test (lines 394–461) does not enter either callback before cancellation.

**Impact:** A stalled live projection can hold a claimed Docker operation and its budget/Run recovery path indefinitely despite cancellation or deadline. The report's assertion that callbacks “must return promptly” is not an enforceable property of the current function callback API. The lock does prevent callbacks *after* a returned result, but achieves that by allowing the return to stall.

**Smallest defensible correction:** Remove arbitrary callback execution from the lock-protected freeze path and provide a bounded cancellation/termination contract for callbacks, or reject callback forms that cannot meet it. Add a behavioral test that enters a blocking typed callback, cancels, and checks bounded return, a frozen result, and no callback work after return under `-race`. Apply the same rule to the legacy callback.

### F2 — Medium — Cancellation after stream drain can still return a complete result

**Evidence / affected symbol:** `StartAttachedExecOnce` records `waitErr` only when the loop selects `startCtx.Done()` (lines 293–311). After the stream and input cases finish, it derives `TransportComplete` from `waitErr == nil` (lines 332–340) without checking `startCtx.Err()`. If cancellation arrives after the last receive and before or during observation, `observationContext` deliberately uses a fresh context (lines 407–412), and a terminal inspect can leave `TransportComplete` with a nil error (lines 341–368).

**Impact:** The Fix1 plan explicitly requires cancellation/deadline to remain partial or unavailable. A completion racing with cancellation can be reported as a normal complete result, weakening the conservative recovery and false-success gate.

**Smallest defensible correction:** Check `startCtx.Err()` before declaring complete and again before final return; if set, return partial with that error. Add a deterministic test that cancels after the stream finishes and before terminal inspection completes.

## Verdicts and limits

**Spec compliance: conditional fail.** The original clean-EOF false-completeness, blocked conforming sink late-unblock, and typed callback panic cases are corrected at the reviewed hashes. F2 still violates the Fix1 cancellation rule. The provider remains unrouted; durable S2 claim, real append-store implementation, joined build/version gate, and broader spec acceptance are outside this Task 1 checkpoint.

**Code quality: fail pending F1–F2.** The focused race suite and final-hash live Docker probe pass, but neither exercises the callback cancellation path or the stream-drain/cancellation race. The `Append`/`Seal` interface rejects a plain function sink at compile time and documents a linearizable, prompt `Seal`; the provider cannot prove that an arbitrary implementation honors it. The blocked-sink test supplies a conforming test sink, while normal/live tests use a no-op `Seal` adapter, so production routing must use a separately verified durable sink implementation.
