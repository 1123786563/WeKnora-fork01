# Craft #107 T19 S3 Task 1 — independent Spec and quality review

Reviewed 2026-09-24. Read-only review of the exact four-file S3 checkpoint, S3 plan/design, approved Craft web Artifact Spec (Task Budget and Run recovery), CONTEXT.md, ADR-0004, S2 Fix 1 review, task report, and tests. No OCR or new Docker call was run.

## Findings

### S3-R1 — Medium — bounded terminal outcome is absent

- **Evidence:** `DockerOutputlessExecClient` exposes create, start, and observe only (`internal/modules/execution/sandbox/docker_restricted_exec.go:45-49`). `CraftDockerRestrictedExec` similarly ends at `Observe` (`internal/application/service/craft_docker_restricted_exec.go:104-131`). There is no bounded wait or terminal outputless result with exit status/duration. The real Docker test implements its own polling loop (`docker_restricted_exec_test.go:145-160`). The S3 plan Task 1 step 2 and provider design §“Exact sequence and ownership” step 7 require a bounded wait; the design's restricted contract requires exit status/duration with stdout/stderr unavailable.
- **Impact:** A caller cannot use this restricted capability to await an authoritative command result under a deadline. It must invent polling and cancellation behavior, and this checkpoint does not satisfy the planned outputless result contract.
- **Smallest defensible correction:** Add a restricted `Wait` operation over the bound receipt, with a caller deadline, same-ID inspect, positive running provenance, explicit unavailable stdout/stderr, and exit status/duration. Return unknown on cancellation, inspect failure, or deadline without releasing the hold/fence. Test fast and slow completion plus cancellation.

### S3-R2 — Medium — physical one-send proof does not cross the durable coordinator on real Docker

- **Evidence:** The service fake counts one `startCalls` in a serial Start/Resume sequence (`craft_docker_restricted_exec_test.go:57-100`); it has no concurrent claimants, side-effect counter, bind failure, or after-claim cancellation case. The real Docker test invokes `CreateOutputlessExec` and `StartOutputlessExec` directly (`docker_restricted_exec_test.go:140-144`), without S2 Prepare/Bind/Claim. It checks one marker but not a lost response or replay across the durable claim. The S3 plan Task 1 step 1 and real-probe step 3 specifically require physical callback/side-effect proof under these cases; S2-R3 was left open for this seam.
- **Impact:** The recorded tests show the transport can start one command and the coordinator can supply a single permission in simple cases. They do not establish the claimed end-to-end invariant that racing/recovering application paths cause at most one physical `ExecStart`, including a daemon side effect followed by response loss. Thus S2-R3 cannot be closed by this checkpoint's evidence.
- **Smallest defensible correction:** Add a coordinator-level fake engine with concurrent Start/Resume, failed bind, crash after bind/claim, and a start callback that records a side effect before returning an error; assert one actual start and retained `intent`/hold/fence. Extend the disposable Docker probe through the coordinator or provide an equivalent exact-sequence integration fixture, including response-loss replay and cleanup evidence.

### S3-R3 — Medium — canceled context can still reach the physical start seam after claim

- **Evidence:** `claimAndStart` consumes the permission and calls `StartOutputlessExec(ctx, ...)` immediately after `op.Claim` (`craft_docker_restricted_exec.go:85-99`), without checking `ctx.Err()` after the durable claim. A cancellation between claim commit and this call therefore reaches the provider callback; a fake provider can send despite the canceled context. The provider design step 4 says a canceled context cannot reach `ExecStart`.
- **Impact:** Cancellation can cause an unnecessary chargeable send even though the caller has abandoned the operation. The durable claim still prevents a resend, so this is a cancellation contract defect rather than a duplicate-send finding.
- **Smallest defensible correction:** Check the context immediately after the claim and before consuming permission or invoking the provider; return an unknown result with the durable receipt and preserve the hold/fence. Add a deterministic test that cancels immediately after a successful claim and asserts zero physical start callbacks. The unavoidable cancellation race once the RPC begins must continue to classify as unknown.

## Evidence and limits

- Current SHA-256 values of all four owned files and the task-local patch match `2026-09-24-craft-107-t19-docker-s3-outputless-task1-checkpoint.json`; patch hash is `28ddc39b152f8e13c917c44af229a09e79e3b8da575611af5be4638e6866d760`. Base HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`. `git diff --check` on the four files passed.
- Independently reran `go test ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1` and `go test ./internal/application/service -run '^Test(CraftDockerRestricted|CraftDockerSendCoordinator(PreparesHeldIntentThenBindsExactReceipt|ClaimIsOneOwnerAndReplayCannotResend))' -count=1`; both passed. The task report records passing targeted race tests and a disposable Docker 29.4.0 marker-once probe; this reviewer did not repeat the real Docker call.
- The code correctly separates outputless opt-in, rejects stdin and output callbacks before hold/create, binds the exact receipt before claim, calls detached Start with a local deadline, treats start errors as unknown, and does not use attach or legacy Exec in this path. `Observe` checks the same exec/container ID and keeps false/zero and inspect errors unknown. The restricted capability remains outside production routing.
- This review does not assert full T19 acceptance: normal output/streaming, stdin, durable terminal reconciliation, other providers, and production wiring remain outside S3.

## Verdict

**Scoped Spec compliance: FAIL.** The bounded outputless wait/result is missing and the required physical one-send proof across the durable coordinator is incomplete. **Scoped code quality: FAIL pending R3 and the behavioral tests above.** The safe core sequence is present and the focused tests pass, but the post-claim cancellation gap and incomplete end-to-end evidence prevent approval of S3 as planned. Production routing remains correctly disabled.
