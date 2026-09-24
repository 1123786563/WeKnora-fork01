# Craft #107 T19 S3 Fix2 Task 1 — independent Spec and quality review

Reviewed 2026-09-24. Scope: the two owned service files and exact Fix2 Task 1 checkpoint, against the Fix2 plan, Fix1 review, S3 provider design, approved Craft web Artifact Spec, CONTEXT.md and ADR-0004. Read-only source review; no OCR or Docker probe was run.

## Findings

No new critical, high or medium finding in this Task 1 delta. The Fix1 F1 terminal-after-cancellation race is closed for the specified inspect-in-flight ordering. Fix1 F3's first-inspect false/zero limit is now explicitly covered, although it remains a deliberate liveness limit.

### R1 — Medium, previously known and still open — authoritative duration

- **Evidence / affected symbol:** `CraftDockerRestrictedExec.Wait` returns a terminal `CraftDockerOutputlessResult` with `DurationAvailable: false` and no measured duration (`internal/application/service/craft_docker_restricted_exec.go:152-154`). The pinned Docker inspect response lacks execution timestamps, as recorded in the provider design and Fix1 review.
- **Impact:** The complete restricted result contract, which calls for exit status and authoritative duration, is not yet satisfied. Zero duration must not be read as a measured value.
- **Smallest defensible correction:** Keep S3-R1 open until a separately designed and reviewed authoritative duration source exists; do not substitute polling elapsed time.

### R2 — Medium gate outside this checkpoint — coordinator-to-physical-start proof

- **Evidence / affected scope:** Fix2 changes only `craft_docker_restricted_exec.go` and its test. The provider design requires a disposable Docker response-loss test joining the durable claim to one physical `ExecStart` and one marker side effect; Fix1 Task 2 owns this proof.
- **Impact:** S3-R2 and full S3/T19 acceptance remain open. The Fix2 checkpoint is not evidence for enabling production routing.
- **Smallest defensible correction:** Complete and independently review Fix1 Task 2's real Docker proof before closing S3-R2 or routing chargeable commands.

## Scoped evidence

- Current owned-file SHA-256 values match the checkpoint: service `278bcd67627e0b5ca5a3f0dd74f999a619d04e3b2f068335980d899a4617a485`, test `4da6dcc328926f70175d0bcc8f0c9ebd4f60789164132f9144a671fd14829405`. Patch, archive and report hashes also match their manifest. The archive contains exactly the two owned files and their extracted hashes match current source. The Fix2 preimage hashes match the Fix1 postimage manifest; the task patch reverses cleanly against the current files. HEAD is the recorded `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- `Wait` rechecks `waitCtx.Err()` immediately after successful `Observe` and before accepting either terminal state (`craft_docker_restricted_exec.go:142-154`). The unknown branch carries the previously loaded exact durable receipt; the method performs no journal settlement or second provider start. stdout, stderr and duration remain marked unavailable.
- The new fake blocks the second inspect after a positive running observation and ignores cancellation. The test waits for that inspect to enter, then cancels or waits for deadline expiry before releasing terminal success or failure. Both cases assert `RemoteOperationUnknown`, exact receipt and retained `send_claimed_at` (`craft_docker_restricted_exec_test.go:257-317`). Channel synchronization fixes the event order rather than relying on timing alone. The report records RED failures with the post-Observe fence removed; this review did not mutate source to repeat RED.
- The first-inspect test returns an unknown observation with zero exit code, asserts no prior running evidence, and verifies the durable claim remains (`craft_docker_restricted_exec_test.go:321-338`). The adapter's separate false/zero test verifies the raw Docker mapping; the service test supplies its resulting unknown state. This accurately documents the liveness limit rather than treating false/zero as success.
- Independently reran `go test ./internal/application/service -run '^TestCraftDockerRestricted' -count=1` and the two new tests under `go test -race`; both passed. `gofmt -l` reported no owned files. `git apply --check --reverse` succeeded for the task patch.

## Verdict

**Scoped Spec compliance: PASS for Fix2 Task 1 / Fix1 F1 and F3.** The post-inspect decision respects the bounded Wait context in the deterministic late-terminal cases, and first false/zero stays unknown. S3-R1 duration and S3-R2 coordinator proof remain open; this is not full S3 or T19 acceptance.

**Scoped code quality: PASS.** The correction is a small fence at the terminal decision, with deterministic behavioral tests for cancellation and deadline and preserved receipt/claim assertions. No new blocking issue was found in the owned delta.
