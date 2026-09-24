# R5 Effect Task3a Task1 independent admitted-coordinator review

## Scope and checkpoint

Reviewed the approved Craft #107 R5 effect constraints, Task3a plan/report/checkpoint, the Task2a/2b authority seams, the current legacy provider interface, and only `craft_runview_runtime.go` plus its new admitted test. The task patch SHA-256 is `280ddde0dcfe772e0e9c3f1e53a9236f98f3d09150c688ab27f961c15f17742d`; archive hashes are `84caccc199c357a904e94f2b05fccdd1e6c3678e5edecd8b0e693ca7e5eff010` (pre) and `a0759b6590b761704a87bc6a5dbdd90ffb815bf813780ec8bd275d1173edcc7a` (post). I verified the two preimage manifest entries, applied the patch in a temporary tree, and compared both outputs byte-for-byte with the post manifest and current integration files. No concurrent files are included.

The checkpoint reports focused `TestCraftRunViewAdmitted`, race, container compile and diff checks passing at the postimage. I inspected the tests but did not duplicate the controller's Go run or invoke a physical provider. All transition-first and claim-first tests here use fakes; Task2b's repository lock tests supply separate DB evidence.

## Findings

### R5E-3A-1 — Medium — pre-send observation failure permanently consumes a claim

**Evidence:** `internal/container/craft_runview_runtime.go:286-315,337-358` calls `BeginEffect` before the read-only `ObserveContainer` / `ObserveContainerState`. If either observation fails or returns a mismatched identity, the coordinator finishes a newly granted claim as `unknown` even though `CreateGenerationContainer` or `StartGenerationContainer` has not been called. `FinishEffect` makes `unknown` unresolved and immutable for that claim; a later exact replay receives `maySend=false` and cannot retry the missing operation. The tests inject provider mutation failures, but no pre-send observation failure (`craft_runview_admitted_runtime_test.go:102-180,341-382`).

**Impact:** A transient inspection failure or stale observation can fence the Run and session slot indefinitely with an “unknown external effect” when no external mutation occurred. This turns a recoverable read error into a false uncertain-send state and blocks the intended generation.

**Smallest correction:** Perform pure observation and its identity checks before granting the one-shot send claim, then claim immediately before mutation. If an observation after claiming is indispensable, add an authority outcome that durably proves no send and permits a later claim; do not label that case an unknown send. Test the failing read and exact replay with the real authority state machine.

### R5E-3A-2 — Medium — DB-only session marker failure strands an unused OpenCode claim

**Evidence:** `craft_runview_runtime.go:203-219` claims `RunViewEffectOpenCodeCreate` before calling the legacy store's `BeginSessionCreate` compatibility marker. If that DB-only call fails, it returns immediately with a pending effect and no `CreateOpenCodeSession` call. On exact replay, `BeginEffect` returns the same claim with `maySend=false`; once the marker succeeds, a still-empty inventory returns unresolved at line 219 and the actual create can never be sent. The fake marker never fails, so the focused tests do not cover this boundary.

**Impact:** A transient marker write failure permanently fences normal session creation despite no OpenCode request having been attempted. This also widens the interval between claim and external mutation beyond the plan's immediate-before-send contract.

**Smallest correction:** Persist and validate the DB-only compatibility marker before the OpenCode effect claim, then claim immediately before `CreateOpenCodeSession`; add a marker-failure/replay test with a stateful authority fake or repository-backed authority.

## Positive evidence and verdicts

The admitted path passes the original `craft.Task` unchanged to `AllocateAdmitted` and each `BeginEffect`, checks local Fence/digest consistency, verifies claim key/generation/kind/token, and uses separate claims for Docker create, Docker start and OpenCode create. Mutations only occur on `maySend=true`; a used claim remains observe-only. Observed container and session identities are checked before successful receipts. The current combined provider does not implement the split interface and is rejected before any provider call. The report correctly identifies `EnsurePrivateNetwork` as a separate unclaimed network-create effect blocker; no production DI or physical provider path was enabled.

- **Scoped Spec compliance: FAIL.** The claimed mutation stages and legacy-provider refusal match the approved boundary, but findings R5E-3A-1/2 let pre-send read or bookkeeping failures consume one-shot authority as if an external send might have happened. The required recoverable exact generation is then stuck without an external attempt.
- **Code quality: CHANGES REQUIRED.** The exact patch and fake-provider tests establish the happy path and post-send unknown/no-resend behavior. The missing pre-send failure/replay tests conceal the two availability defects above.

Real split Moby/OpenCode adapters, network provisioning authority, central assembly, joined Run transition races, R4 runtime quiescence and physical evidence remain downstream gates. This review does not release R5 effects or production routing.
