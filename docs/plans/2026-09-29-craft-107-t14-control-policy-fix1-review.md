# T14 Control Policy Fix 1 — Independent Review

Date: 2026-09-29 (Asia/Shanghai)
Reviewer: `/root/t14_fix1_reviewer` (read only source review)
Scope: `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix1-plan/review-package-02` and the corresponding live files in this worktree.
Fact sources: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, `CONTEXT.md`, T14 policy plan and Fix 1 plan, and the original independent review.

## Verdict

- **Spec compliance: FAIL / T14 acceptance unverified.** The implementation has not demonstrated the required real namespace behavior. The single targeted disposable integration attempt timed out after 180 seconds in the policy-helper Docker image build, before a renderer, policy install, conntrack assertion, old socket reuse, or fresh same-listener denial; zero tests ran. The immutable T14 attempt matrix was not run. The generated canary syntax defect below would independently prevent a successful install if the image build succeeded.
- **Code quality: FAIL.** One high severity executable-code defect remains in Fix 1. The source-only repairs to normalization, bounded discovery, and conntrack matching are directionally correct, but they do not clear this defect or substitute for live proof.
- No Docker command, OCR run, implementation edit, or test-source edit was made in this review.

## Package integrity

All five `manifest.json` entries match both `before/` and `after/` SHA-256 values; each live file also matches its `after_sha256`. The `fix1.patch` SHA-256 is `2d4de297fd075c976525c358f6b7b59ce8d5919b207ae34bbc572e60c0da7048`; the package archive SHA-256 is `ad1dff6304fedfbd9bd477c72738a2d33948f874c5bfdd27eb29c5480dd83067`. The reviewed controller, helper, and integration-test final hashes are `eb2540a84873bb4a6a98764a4fb6459e6f9b887f46ffd04fabc3ac6e04487add`, `31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643`, and `d3a0b7deb5f9e2a76b4c429ea73ea55ef8ccb9097cc6acd99a387522444a6d77`.

## Findings

### High — generated same-listener canary is invalid Python

**Evidence:** `deploy/craft/render-boundary/policy-helper/controller.py`, `Controller.run_mandatory_canary`, the `canary_code` construction around lines 346–356, concatenates `out={...};` immediately with `if local=={source}:...`. Python does not permit a compound `if` statement after a semicolon. I evaluated the exact AST expression that creates `canary_code` with `family=4`, `address='127.0.0.1'`, `port=9515`, and `source=49152`; `compile(code, ..., 'exec')` returned `SyntaxError: invalid syntax`, line 1, offset 333. The offending substring is `... 'connect_attempted':False};if local==49152:...`. The current unit test calls only `same_listener_canary_passed` with fabricated evidence; it never compiles or runs the emitted script.

**Impact:** `docker exec python3 -c` exits before any connect attempt. The same-listener canary cannot pass, so `install` fails and invokes failure cleanup. This blocks T14 policy acceptance independently of the Docker image-build timeout.

**Smallest correction:** Put a newline before the generated `if`, or express the branch as a valid simple statement, then add a unit test that compiles the generated IPv4 and IPv6 programs and executes the relevant no-connect and failed-connect branches in a controlled fixture. Re-run the disposable namespace proof before any immutable T14 probe.

### Medium — reuse proof assertion races the signal handler

**Evidence:** `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, `test_target_counters_cover_random_preview_loopback_and_controlled_dns`, calls `docker kill --signal USR1` and immediately invokes `docker exec cat /tmp/reuse-proof` (around lines 487–489). Docker's signal command acknowledges delivery; the renderer's Python handler writes the file after delivery. There is no synchronization that the handler completed before `cat`.

**Impact:** The test can report a false failure even when the established socket successfully exchanged `reuse`/`reuse-ok`, obscuring the live policy result when Docker becomes available.

**Smallest correction:** Poll for the marker with a short explicit deadline, fail if the renderer exits, and assert its exact bytes. Preserve the same socket and do not weaken the policy or timeout/resource limits.

## Original finding disposition

1. **High, normalized socket states:** Code fixed. The discovery filter now compares `ESTABLISHED` and `LISTEN`. Its unit test inspects source text; the disposable fixture would exercise actual socket records, but did not run. Runtime closure remains pending.
2. **High, incomplete conntrack proof:** Code fixed on inspected paths. The parser bounds the table at 1 MiB, requires a complete final newline, validates two original/reply address and port tuples, TCP protocol 6, family and `ESTABLISHED`, and rejects duplicate exact matches. Unit cases cover invalid and one valid record. Actual kernel table behavior remains unverified.
3. **Medium, canary without actual connect:** **Open.** The new predicate requires structured attempt, destination, distinct source port, timeout and counter delta, but the emitted canary cannot execute due to the high finding above.
4. **Medium, unbounded discovery/output:** Code fixed on inspected paths. Proc row, FD, PID, candidate and serialized JSON limits and bounded child stdout/stderr with termination are present; focused tests cover child cap and timeout. No live namespace proof.
5. **High, missing separate reuse/denial behavior:** The disposable test now contains both assertions, but the Docker build timeout prevented execution. The marker read also has the race described above. **Acceptance remains unverified.**

The generated nft OUTPUT chain retains `priority -150`, `policy drop`, exact bidirectional established-flow rules, preview-only allows, and loopback/isolated-target drops. These static observations do not establish live conntrack hook, counter, or WebDriver behavior.

## Verification boundary

The implementer reported 14 focused non-Docker tests passed, 1 Docker-gated test skipped, `py_compile` passed, and `git diff --check` passed. This review independently verified hashes, read the patch/full relevant source, and compiled the emitted canary expression, reproducing the syntax error. The Docker setup timeout and zero executed integration tests are recorded in `docs/plans/2026-09-29-craft-107-t14-control-policy-fix1-report.md`. No live T14 acceptance evidence is available for this revision.
