# T14 ctnetlink Task 1 Report

## Scope and baseline

Implemented only Task 1 from `2026-09-29-craft-107-t14-ctnetlink-plan.md` in the T14 Worktree. Owned files were `deploy/craft/render-boundary/policy-helper/helper.py` and `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; no files were staged or committed.

Starting SHA-256 values matched the Brief exactly:

- `helper.py`: `31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643`
- `tests/test_controller_integration.py`: `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`

## Changes

- Replaced `/proc/net/nf_conntrack` and `/proc/net/ip_conntrack` reads with a stdlib `NETLINK_NETFILTER` `IPCTNL_MSG_CT_GET` dump sent from the helper's current network namespace.
- Added strict raw netlink framing and nested ctnetlink tuple parsing for IPv4/IPv6, TCP protocol and TCP `ESTABLISHED` state. A proof is returned only for one exact original/reply tuple pair.
- Bound socket wait to 2 seconds, datagrams to 64 KiB, total dump bytes to 1 MiB, records to 4,096, and messages to 8,192. Validate kernel sender PID, sequence, response framing, `NLMSG_ERROR`, `NLMSG_DONE`, dump interruption, truncation and deadline. Fail closed on missing/duplicate/malformed evidence.
- Replaced old procfs parser tests with synthetic raw ctnetlink fixtures covering positive IPv4/IPv6 and negative family/protocol/state/tuple, duplicate, malformed attributes, sender/sequence, error/interruption, missing completion, truncation and resource limits.

## Verification

RED evidence:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` before implementation — 4 new test errors because the ctnetlink fixture/parser interface did not exist.

Passing checks (run from `deploy/craft/render-boundary/policy-helper`):

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — 8 tests passed.
- `python3 -m unittest tests.test_barrier_adapter -v` — 18 tests passed.
- `python3 -m unittest tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyHelperInvocationUnitTests -v` — 9 tests passed, 1 Docker-gated test skipped.
- `python3 -m py_compile helper.py controller.py barrier_adapter.py` — passed.
- `git diff --check -- deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` — passed.

## Docker run incident and remaining verification

A requested non-Docker `python3 -m unittest discover -s tests -v` was not safe for this repository: unittest discovery ran the Docker integration class-level `setUpClass`, which executes `docker build` before any test-method skip can apply. The run then remained at `PolicyControllerIntegrationTests.test_cleanup_recovers_partial_install_without_controller_state`; I stopped the unittest process. It did not enter `start_renderer`, so no disposable renderer/helper test container was created. The shared helper image tag may have been rebuilt from the current helper source; I did not inspect or alter the shared Docker daemon afterward. Parent should account for this possible image-tag side effect while reserving the Docker slot.

No disposable exact-flow test was run. Parent must retain a Docker slot and perform/coordinate the one exact policy flow test from Plan Task 1 Step 5 before considering the behavior live-verified. The integration workspace was not modified. The 37-cell browser matrix remains out of scope for this task.

## Final hashes and limitations

- `helper.py`: `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`
- `tests/test_controller_integration.py`: `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`

The owned source files were already untracked T14 outputs before this task. Their preimage hashes are recorded above, but I did not preserve source preimage copies before editing, so an exact task-only patch hash cannot be reconstructed in this report. No other file was intentionally modified. Runtime ctnetlink behavior and disposable-container cleanup remain unverified pending the coordinated Docker run.
