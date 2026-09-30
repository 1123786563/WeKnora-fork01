# T14 Control Policy Review Fix 1 — Task 1 Report

Date: 2026-09-29 (Asia/Shanghai)
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
Task: Bound exact-flow evidence and prove established reuse plus fresh same-listener denial
Status: **DONE_WITH_CONCERNS — source and non-live tests pass; disposable kernel proof blocked by Docker build timeout**

## Owned changes

- `deploy/craft/render-boundary/policy-helper/controller.py`
  - Proc TCP rows are bounded by byte and row limits. PID inventory, per-process FDs, socket candidates, and serialized discovery output have explicit caps. Socket selection now compares the normalized `ESTABLISHED` and `LISTEN` values.
  - Host subprocess stdout and stderr are drained with bounded buffers. Exceeding the byte cap or wall timeout kills and waits for the child before raising.
  - The same-listener canary emits JSON containing its actual destination, source port, connect attempt, connection result, error class, and connect elapsed time. Acceptance requires the attested listener, distinct source port, an attempted-but-unconnected timeout lasting at least 400 ms, return code 1, and a positive family-specific loopback-drop counter delta measured between adjacent snapshots.
- `deploy/craft/render-boundary/policy-helper/helper.py`
  - Conntrack reads are capped at 1 MiB and require a complete newline-terminated table. Records are parsed as two complete tuples; malformed entries, invalid tuple addresses/ports, oversize/truncated tables, and duplicate exact matches fail closed. Acceptance requires one IPv4/IPv6 TCP protocol 6 `ESTABLISHED` original client-to-driver tuple and reverse driver-to-client tuple.
  - Helper command stdout and stderr now use bounded reads and kill-on-timeout/cap handling, including cleanup commands.
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
  - Added conntrack rejection coverage for wrong family/protocol, reply tuple mismatch, duplicate records, malformed/truncated/oversized input, and a complete valid flow.
  - Added tests for normalized discovery state selection, child-output cap and timeout handling, and canary evidence failure cases.
  - Extended the one disposable integration case to request/response over the original socket after install and separately assert the fresh same-listener canary receipt and counter delta.

## RED / GREEN and static verification

- RED command, before implementation: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_conntrack_requires_one_complete_bidirectional_tcp_record tests.test_controller_integration.PolicyTargetCounterUnitTests.test_conntrack_rejects_oversized_table` (run from `deploy/craft/render-boundary/policy-helper`). Result: **5 expected subtest failures** showed wrong protocol, wrong family, mismatched reverse tuple, duplicate complete record, and missing final newline were accepted by the prior parser. The oversized fixture also ran; its original generic assertion did not distinguish explicit overflow detection, so it was strengthened to require the `oversized or truncated` error.
- Final focused non-Docker command: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyHelperInvocationUnitTests` (same directory). Result: **14 tests passed, 1 Docker-gated test skipped**.
- `python3 -m py_compile controller.py helper.py barrier_adapter.py tests/test_controller_integration.py` — passed.
- `git diff --check -- controller.py helper.py tests/test_controller_integration.py` — passed.
- Source audit found only the exact bidirectional `ct state established` tuple exceptions; output chain priority remains `-150`, and policy drop, preview and isolated target rules remain present. Discovery compares normalized states and enforces the configured row/FD/PID/candidate/output limits.
- Full `python3 -m unittest discover -s tests` was attempted but is **not a valid non-Docker suite command** here: unittest executed `PolicyControllerIntegrationTests.setUpClass` before skip filtering, attempted a Docker image build, and exited with one setup error; the run also found one incomplete legacy SYN_SENT fixture, which was corrected. After that correction, the explicit non-Docker suite above passed.

## Disposable Docker attempt and proof boundary

One targeted integration invocation was made:

`POLICY_HELPER_RUN_DOCKER_TESTS=1 python3 -m unittest tests.test_controller_integration.PolicyControllerIntegrationTests.test_target_counters_cover_random_preview_loopback_and_controlled_dns`

It did not reach the disposable renderer or policy install. `setUpClass` remained in `docker build --platform linux/arm64 -t craft-t14-policy-helper:2026-09-24 .../policy-helper`; after 180 seconds Python raised `subprocess.TimeoutExpired`. The unittest summary was **0 tests run, 1 setup error**. This build setup timeout supersedes the earlier discover-time build, which returned nonzero before starting a renderer.

After the timeout, task-owned processes `33305` (Docker Buildx), `33220` (Docker CLI), and `33180` (unittest parent) were terminated. A follow-up process inventory found no task-owned build/test process. `docker ps -aq --filter name=t14_` returned no containers. No conntrack tuple, established-socket request/response, same-listener fresh denial, or drop-counter proof was obtained. The test fixture and assertions are implemented, but the live assertions remain unverified. No immutable candidate probe was run.

## Baseline and final hashes

Fix 1 baseline hashes from `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix1-plan/baseline-hashes.sha256`:

```text
09c5d3f324e30fd54cb88c46e4495c446088ca06ac01fbea34c6af730345a74b  deploy/craft/render-boundary/policy-helper/controller.py
da4760f7a4f06082e326d4528dc64be2aac57621efde491f49ebc56cbae7cb1b  deploy/craft/render-boundary/policy-helper/helper.py
598502bb9015d440a7696a399b486b33111a1982487b8798cdb3d2763004b1ec  deploy/craft/render-boundary/policy-helper/barrier_adapter.py
9237c398ea2133ec798397577efe18034b7148d1b715ce7921393a30d483a8b7  deploy/craft/render-boundary/policy-helper/tests/test_barrier_adapter.py
01d7efe3aaa083eb625141997e1ba1c6bbcec1df6b32a39e28f74b112a8a79d4  deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py
```

Final task-file hashes:

```text
eb2540a84873bb4a6a98764a4fb6459e6f9b887f46ffd04fabc3ac6e04487add  deploy/craft/render-boundary/policy-helper/controller.py
31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643  deploy/craft/render-boundary/policy-helper/helper.py
d3a0b7deb5f9e2a76b4c429ea73ea55ef8ccb9097cc6acd99a387522444a6d77  deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py
```

The original policy-helper files are untracked in this worktree; the task-start hashes exist, but no copy of the prior file bodies was retained. Therefore a trustworthy task patch hash/delta cannot be reconstructed. Unrelated T14 files and artifacts were preserved. No files were staged or committed.
