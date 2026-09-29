# T14 ctnetlink fix4 validation

Status: **DONE_WITH_CONCERNS** (targeted checks pass; live tuple remains unproven)

## Scope and revision

- Assigned worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Helper SHA-256: `0f129de95e945ee9bf69ccab5f2493a49d9a3c1709f79a68ff646012972ed4c2`
- Test SHA-256: `8f3dd869a4044f3d130e91934b90e7a512bb78343176546264ba0ca9f57dc724`
- Scope: assigned local parser/query-path and barrier-adapter checks only. No source or test edits, Docker, discovery, or daemon operations performed.

## Evidence

Working directory for the following unit test commands:
`/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01/deploy/craft/render-boundary/policy-helper/tests`

1. `python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_accepts_matching_nonzero_local_port_id test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_mismatched_local_port_id test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_accepts_exact_ipv4_and_ipv6_established_flow test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_accepts_kernel_done_status_zero test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_nonzero_done_status test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_zero_error_ack test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_nonzero_error test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_bad_tuple_state_and_multiplicity test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_bad_transport_and_incomplete_dump test_controller_integration.PolicyTargetCounterUnitTests.test_ctnetlink_parser_rejects_malformed_attributes_and_limits`
   - Exit 0; 11 tests passed. Covers matching and mismatching nonzero local port IDs, fake socket `getsockname()` query behavior and close/error paths, parser selectors including exact-flow, sender/sequence/error/completion and malformed/bounds rejection.
2. `python3 -m unittest test_barrier_adapter.BarrierContractTests.test_webdriver_flow_requires_one_owned_established_loopback_tuple`
   - Exit 0; 1 test passed.
3. `python3 -m py_compile ../helper.py ../barrier_adapter.py test_controller_integration.py test_barrier_adapter.py`
   - Exit 0.
4. `git diff --no-index --check /dev/null ../helper.py` and `git diff --no-index --check /dev/null test_controller_integration.py`
   - Both produced no whitespace diagnostics. Each returns status 1 because no-index reports that the files differ from `/dev/null`; this is expected for these untracked checkpoint files, not a whitespace failure.

## Acceptance and limitations

The targeted fix4 parser and query-path acceptance cases pass. The barrier adapter selector and Python syntax checks pass. The raw diagnostic cited by the brief reports `recvmsg` sender PID 0 and only `NLMSG_DONE` with status 0, with no `CT_NEW` record. These local fixtures verify parser behavior and a fake socket exchange; they do **not** prove that the live conntrack dump contained the expected tuple. Exact live-flow acceptance therefore remains open for the separately assigned disposable runtime check.
