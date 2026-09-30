# Craft #107 T19 S3 Fix1 Task 1 — independent Spec and quality review

Reviewed 2026-09-24. Scope: the four owned restricted Docker exec files at the Task 1 checkpoint, its pre/post manifests and task-local patch, Fix1 plan, initial S3 review, provider design, approved Craft web Artifact Spec, CONTEXT.md and ADR-0004. Read-only source review; no OCR or Docker probe was run.

## Findings

### F1 — Medium — terminal result can win after Wait cancellation or deadline

- **Evidence:** `CraftDockerRestrictedExec.Wait` checks `waitCtx.Err()` before `s.Observe`, but after `Observe` returns a terminal result it returns success without checking the context again (`internal/application/service/craft_docker_restricted_exec.go:139-151`). A cancellation or deadline while the inspect request is in flight can therefore yield `Succeeded`/`Failed` and `nil` if the inspect returns a result concurrently with cancellation. The added cancellation test cancels before entering `Wait` (`craft_docker_restricted_exec_test.go:182-189`); the deadline test only returns repeated `Running` observations (`:195-205`). Neither exercises this ordering.
- **Impact:** The caller may receive a successful terminal outcome after its Wait bound expired, contrary to the Fix1 plan and provider-design rule that cancellation/deadline during Wait remains unknown with the receipt and unresolved claim. The code does not itself release the hold, but a later consumer could treat the result as authoritative.
- **Smallest defensible correction:** Check `waitCtx.Err()` immediately after a successful `Observe` and before accepting a terminal state. Add a deterministic fake inspect that blocks until cancellation/deadline and then returns terminal despite that context; assert an unknown result and retained receipt/claim.

### F2 — Medium — authoritative execution duration remains unavailable (S3-R1 open)

- **Evidence:** `CraftDockerOutputlessResult` has `Duration` and `DurationAvailable`, but every successful `Wait` return sets `DurationAvailable: false` and leaves duration zero (`craft_docker_restricted_exec.go:149-151`). The Task 1 report correctly states that the pinned `ExecInspect` response lacks execution timestamps. The provider design's restricted result contract requires exit status **and duration**; the Fix1 plan explicitly says to leave R1 open if Docker cannot provide authoritative duration.
- **Impact:** This checkpoint cannot deliver the planned complete terminal outputless result. Consumers must not interpret zero duration as measured execution time or treat S3-R1 as closed.
- **Smallest defensible correction:** Keep R1 open until an authoritative duration source is designed and independently reviewed; preserve `DurationAvailable=false` in this checkpoint rather than fabricate elapsed polling time.

### F3 — Low — “fast terminal” test does not cover an exec that finishes before the first inspect

- **Evidence:** `TestCraftDockerRestrictedWaitReturnsFastTerminalState` injects `Running` on its first observation and terminal on its second (`craft_docker_restricted_exec_test.go:227-232`). The real adapter intentionally treats `Running=false, ExitCode=0` without a prior positive running observation as unknown (`internal/modules/execution/sandbox/docker_restricted_exec.go:140-151`), so an actual command completing before the first poll cannot pass this test's success path.
- **Impact:** The test name overstates coverage. Fast real commands may remain unknown through the Wait bound; this is a conservative safety choice under the approved positive-provenance rule, not permission to infer success from false/zero.
- **Smallest defensible correction:** Add an explicit first-observation terminal test asserting unknown and document the liveness limit. Do not weaken the positive-provenance rule without an authoritative alternate signal.

## Evidence and scope

- All four current owned-file SHA-256 values, the patch, postimage archive, and report match `2026-09-24-craft-107-t19-docker-s3-fix1-task1-checkpoint.json`. The preimage manifest matches the earlier S3 checkpoint for the two changed service files; the adapter files are unchanged. The patch contains only those two service files, and the postimage archive contains the four owned files. Its patch lines use ordinary diff prefixes, so a whitespace check on the patch text itself is not a source-file whitespace check.
- Independently reran `go test ./internal/modules/execution/sandbox -run '^TestDockerRestricted' -count=1` and `go test ./internal/application/service -run '^TestCraftDockerRestricted' -count=1`; both passed. The report records targeted race tests, gofmt and `git diff --check`; this review did not repeat the race suite or the prior real Docker probe.
- The post-claim `ctx.Err()` fence is present before permission consumption and provider start (`craft_docker_restricted_exec.go:89-105`). The new deterministic test records zero start callbacks and a retained `intent`/claim. The unavoidable cancellation race after the callback begins still resolves as unknown on provider error.
- `Wait` uses the claimed receipt and a positive duration bound; `Observe` reloads the durable receipt, inspects the same exec/container IDs, and retains positive running evidence in process memory. The adapter treats false/zero without that evidence and inspect errors as unknown. Unknown Wait paths do not settle the journal. stdout/stderr remain explicitly unavailable.
- S3-R2 remains assigned to Fix1 Task 2: this checkpoint has no coordinator-to-physical-start response-loss proof on disposable Docker. Production routing remains disabled. No full T19 acceptance is inferred.

## Verdict

**Scoped Spec compliance: FAIL / pending.** S3-R3's post-claim fence and most bounded same-receipt Wait behavior are implemented, but F1 violates the Wait cancellation/deadline rule and S3-R1 cannot close without authoritative duration. S3-R2 is outside this Task 1 checkpoint and remains open.

**Scoped code quality: FAIL pending F1.** Focused tests pass and the safety defaults are conservative, but the terminal-after-cancellation race needs a small source fix and behavioral test. F3 is a coverage/liveness disclosure, not a reason to accept false terminal evidence.
