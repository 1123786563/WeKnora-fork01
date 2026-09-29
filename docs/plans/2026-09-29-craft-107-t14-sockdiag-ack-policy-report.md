# T14 bounded SOCK_DIAG and ACK policy implementation report

## Baseline checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Task-start source SHA-256: helper.py `0f129de95e945ee9bf69ccab5f2493a49d9a3c1709f79a68ff646012972ed4c2`; tests `8f3dd869a4044f3d130e91934b90e7a512bb78343176546264ba0ca9f57dc724`.
- At task start, 2,249 untracked files already existed across the worktree; the helper/test files were among them. They were not included as a bulk inventory in this report. Only the two owned source files and this report were changed by this task.

## Implementation checkpoint

Task-start source hashes from the brief (the prior uncommitted ctnetlink fix4 checkpoint):

- `helper.py`: `0f129de95e945ee9bf69ccab5f2493a49d9a3c1709f79a68ff646012972ed4c2`
- `tests/test_controller_integration.py`: `8f3dd869a4044f3d130e91934b90e7a512bb78343176546264ba0ca9f57dc724`

The prior ctnetlink implementation was replaced because the approved architecture ruling now selects bounded SOCK_DIAG socket/inode attestation and ACK-guarded exact tuple rules. The ctnetlink fix4 evidence/history and its prior hashes were preserved; no fix4 report was overwritten.

## Changes

- Replaced the NETLINK_NETFILTER query with a bounded `NETLINK_SOCK_DIAG` dump. The helper constructs a 56-byte `inet_diag_req_v2` for `SOCK_DIAG_BY_FAMILY`, `IPPROTO_TCP`, the requested address family, all-state dump mask, wildcard socket ID and `INET_DIAG_NOCOOKIE` values. It sends one dump request and uses the socket's bound local port ID to validate response `nlmsg_pid`.
- Added bounded framing and response checks: kernel source sockaddr PID 0, request sequence, local port ID, message type, multipart flag, family, address padding, attribute framing, completion status, truncation/interruption, timeout, datagram/aggregate byte, message and record limits. The socket closes in `finally` on success and failure.
- The parser returns proof only for exactly one record matching the host-attested client-to-driver tuple, TCP_ESTABLISHED state and client socket inode. It records the returned cookie as two UAPI `u32` words; the frozen seven-field contract has no host cookie to compare.
- Replaced the two `ct state established` conditions with exactly two directional full-tuple rules guarded by `tcp flags & (syn | ack) == ack`. No general accept or port-only rule was added.
- Updated the owned helper tests to remove obsolete ctnetlink parser/query fixtures and assertions, preserve the counter tests, and add SOCK_DIAG UAPI fixtures, IPv4/IPv6 proof, negative framing/evidence cases, fake query success/timeout with close assertions, and exact rule assertions.

## RED evidence

Before production edits, the four focused new selectors were run:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_attests_exact_ipv4_and_ipv6_client_records -v` — errored because `parse_sock_diag_dump` did not yet exist.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_rejects_bad_or_incomplete_dump_evidence -v` — errored because `parse_sock_diag_dump` did not yet exist.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_query_uses_tcp_dump_and_closes_socket -v` — errored because `require_attested_webdriver_socket` did not yet exist.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_webdriver_flow_rules_are_exact_ack_guarded_tuples -v` — failed both IPv4 and IPv6 subtests because the old rules omitted the required ACK guard.

## GREEN verification

All commands ran from `deploy/craft/render-boundary/policy-helper` except diff checks:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_attests_exact_ipv4_and_ipv6_client_records -v` — passed (IPv4 and IPv6 subtests).
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_parser_rejects_bad_or_incomplete_dump_evidence -v` — passed. It covers wrong source PID, sequence, header port ID, family, state, tuple and inode; malformed netlink framing/attributes; interruption; NLMSG_ERROR; nonzero completion; missing completion/socket; duplicate match; truncation; datagram/aggregate byte limits; and message-count limit.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_sock_diag_query_uses_tcp_dump_and_closes_socket -v` — passed. It verifies `AF_NETLINK/SOCK_DGRAM/protocol 4`, 72-byte request, type 20, request/dump flags, IPv4 family, TCP protocol, wildcard state mask, successful proof, timeout handling and close on both paths.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_webdriver_flow_rules_are_exact_ack_guarded_tuples -v` — passed exact IPv4/IPv6 rule-string assertions.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — 7 tests passed.
- `python3 -m unittest tests.test_barrier_adapter -v` — 18 tests passed.
- `python3 -m py_compile helper.py tests/test_controller_integration.py` — passed (exit 0, no output).
- `git diff --check` — passed (exit 0, no diagnostics). As both owned source files are untracked, `git diff --no-index --check /dev/null <file>` was also run separately for helper and tests; each returned exit 1 only because the file differs from `/dev/null`, with no whitespace diagnostics.

No Docker, unittest discovery, image/daemon operation, staging or commit was run. The exact privileged helper and disposable renderer acceptance remain for parent validation.

## Final checkpoint and limits

- Final `helper.py` SHA-256: `c3d40a6a1d5ba9ef21c15cbabe34d2964a282544586df83a3a1f0e224c248a6a`
- Final `tests/test_controller_integration.py` SHA-256: `5756944010c01f8bb0a6e2d701e72c938425db7ed63c9ae3268d8eb0c0584db6`
- HEAD remains `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. The index has no staged paths. Eight pre-existing tracked files remain modified outside task ownership and were not changed by this task. Final untracked count is 2,250: 2,249 pre-existing worktree files plus this new report. The helper and test files were pre-existing untracked paths whose hashes are captured above; all other pre-existing paths remain untouched. The report itself cannot include its own content hash; its current SHA-256 is reported to the parent separately.
- The target query evidence hash is `f5115d13bf173369fc84c35df5ff5aee3534961d81ca28c80cedceca19c59681`; it contains a successful protocol-4/type-20 bounded dump, exact record and terminating NLMSG_DONE in the following datagram. It does not record whether the probe created the socket with SOCK_DGRAM or SOCK_RAW. This implementation uses SOCK_DGRAM; the fake query verifies that choice and the saved target evidence verifies the matching wire protocol/request and response behavior, but does not independently prove the probe's socket type.
- SOCK_DIAG response messages contain no protocol field; TCP is pinned in the request and the fake query asserts `sdiag_protocol=6`. The architecture accepts ACK-guarded stateless exact tuples, not conntrack equivalence. This task does not establish live reuse, fresh-connection denial, external-egress denial or cleanup acceptance, and does not claim T14 completion.
