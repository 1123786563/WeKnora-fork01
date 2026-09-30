# T14 SOCK_DIAG + ACK Policy Validation

**Result: DONE_WITH_CONCERNS — assigned non-Docker checks pass; live acceptance remains pending.**

## Scope and revision

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Helper SHA-256: `c3d40a6a1d5ba9ef21c15cbabe34d2964a282544586df83a3a1f0e224c248a6a`
- Test SHA-256: `5756944010c01f8bb0a6e2d701e72c938425db7ed63c9ae3268d8eb0c0584db6`
- Target probe evidence SHA-256: `f5115d13bf173369fc84c35df4ff5aee3534961d81ca28c80cedceca19c59681`
- All three hashes matched the assigned values before checks. The policy helper/test remain at those hashes after validation.

## Commands and results

Commands below ran from `deploy/craft/render-boundary/policy-helper` unless specified otherwise.

| Command | Result |
| --- | --- |
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_attests_exact_ipv4_and_ipv6_client_records -v` | PASS, 1 test |
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_rejects_bad_or_incomplete_dump_evidence -v` | PASS, 1 test |
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_query_uses_tcp_dump_and_closes_socket -v` | PASS, 1 test |
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_webdriver_flow_rules_are_exact_ack_guarded_tuples -v` | PASS, 1 test |
| `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` | PASS, 7 tests |
| `python3 -m unittest tests.test_barrier_adapter -v` | PASS, 18 tests |
| `python3 -m py_compile deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` | PASS, exit 0 |
| `git diff --check` | PASS, exit 0 |

No Docker command or unittest discovery was run. No production or test source was modified.

## Acceptance assessment

The checked unit evidence covers IPv4/IPv6 SOCK_DIAG parsing, malformed/incomplete dump rejection, bounded TCP dump query and socket closure, and the exact ACK-guarded tuple rule shape. The assigned non-Docker validation commands all pass.

The following Task 1 acceptance remains unverified and is parent-owned: execution of the privileged live helper in the pinned disposable renderer; same-PID-1-socket marker success and exception counter delta; fresh-connection and same-source-port reuse denial canaries with loopback-drop deltas; preview/external-egress negative canaries; and correlated image, namespace, flow/inode, rules, counters, and cleanup receipts. The saved target probe establishes a bounded raw SOCK_DIAG dump and matching PID 1 inode, but does not invoke the privileged helper or prove nft policy behavior. Therefore this report does not mark T14 accepted or verified.

## Risks and limitations

The architecture ruling explicitly treats the nft ACK mask as a packet-header test, not conntrack equivalence or an inode/socket-bound policy. Exact-tuple reuse and forged ACK-bearing packets remain residual risks described in the ruling; unit tests cannot resolve them. Live canaries and lifecycle/cleanup evidence are still required before acceptance. This validation does not assess authentication/authorization, migrations, or database consistency because this task changes a renderer network policy helper and has no such backend surfaces.

The worktree contained unrelated modified renderer files and numerous untracked T14 evidence/planning files at validation start; these were left untouched. `git diff --check` covered the current tracked diff and reported no whitespace errors.
