# Craft #107 T19 S3 Fix1 Task 2 — independent Spec and quality review

Reviewed 2026-09-24 in the integration Worktree. Scope: Task 2's test-only coordinator-to-physical-start proof in `internal/application/service/craft_docker_restricted_exec_test.go`, its exact checkpoint, patch and report. Sources include the approved Craft web Artifact Spec, `CONTEXT.md`, ADR-0004, the S3 provider design and Fix1 plan, the S2 Fix1 review, and the S3 Fix2 Task 1 review. This review did not run OCR or repeat the disposable Docker probe.

## Findings

No new critical, high or medium defect found in this Task 2 delta.

### Previously known limit — S3-R1 remains open

**Severity:** Medium acceptance gate outside Task 2. **Evidence / affected symbol:** `CraftDockerRestrictedExec.Wait` returns `DurationAvailable: false`; Docker exec inspect supplies no authoritative command duration. The Fix1 plan and S3 Fix2 review expressly leave this open. **Impact:** A complete outputless terminal result and full S3/T19 acceptance cannot be claimed. **Smallest defensible correction:** Design and independently review an authoritative duration source; keep the field unavailable until then. Do not infer duration from polling time.

## Task 2 evidence

- The current test SHA-256 is `3b210e80c962a4b46fa956358ea250defedf1f69219a5a7bc70e491b3f9f4565` (36,277 bytes). Patch `7b2371fce65d60ebc5e36e19f8de476c51c0f87c57121f9784d8046874bdb5d2`, postimage archive `7d4f410d38e881ae513a12de498b6b8458226395ac4801f00c4bcc895b7a5980`, and report `ca70f8540bedf5d39347469a8da996cb4d34e3ee42229a07908f29b07711aac5` match the checkpoint. The archive contains only the owned test and extracts to its current hash. The preimage hash `4da6dcc328926f70175d0bcc8f0c9ebd4f60789164132f9144a671fd14829405` matches the reviewed S3 Fix2 Task 1 postimage; `git apply --check --reverse` succeeds for this task's 340-line test-only patch. No production source changed in this checkpoint.
- The deterministic bind-failure test advances the Run revision after inert create, then asserts zero physical start callbacks, unbound/unclaimed `intent`, and a dispatched hold. Reconstructed coordinator tests cover a bound unclaimed receipt resuming with the same IDs and no new create, and a previously claimed receipt refusing a second start while retaining its receipt, claim and hold.
- The concurrent Start/ResumeBound test blocks the first callback after the provider-side fake effect, verifies that callback observed the exact durable claimed receipt, races both replay entry points, then returns a simulated lost response. It asserts one callback/effect, `RemoteOperationUnknown`, the exact receipt, and the retained `intent`, `send_claimed_at`, and dispatched reservation. Its channels establish the relevant ordering without relying on a sleep.
- The opt-in real Docker test calls service `Start`, which executes Prepare → inert Create → exact Bind → Claim → provider `ExecStart`. Inside the provider wrapper, before `ExecStart`, it reads the durable coordinator projection and verifies the same container/exec IDs are claimed. The wrapper delegates one detached physical start, then deliberately loses the successful response. A `ResumeBound` replay is rejected before another provider callback. `Wait` observes the same bound receipt to terminal success, and a read-only container archive returns exactly `marker\n` from a unique marker path. The recorded Docker run reports client API 1.54, server 29.4.0/API 1.54, one callback and marker effect, empty container logs, successful forced removal, and a later `docker inspect` returning no such object. This is a controlled post-acceptance response-loss simulation; it does not independently exercise an actual network deadline or establish authoritative duration.
- Independently ran the four focused coordinator fake tests in normal and race modes; both passed. `git diff --check` produced no output. The real Docker command and cleanup evidence were assessed from the exact-hashed Task 2 report and test code, not rerun.

## Verdict

**Scoped Spec compliance: PASS for Task 2 / S3-R2.** The disposable physical-start proof joins the exact durable claim to one Docker `ExecStart` callback and one marker effect through the service, including simulated response-loss replay. The reconstruction, bind-failure and concurrent replay cases cover the assigned recovery boundaries. **Full S3/T19: pending** S3-R1 authoritative duration and other broader acceptance gates; production routing remains disabled.

**Scoped code quality: PASS.** The delta is confined to the owned test file, uses deterministic barriers for the replay race, verifies durable state rather than token count alone, and keeps the Docker test opt-in with bounded context, `network=none`, and deferred container removal. The independent normal/race reruns passed. No source correction is required by this Task 2 review.
