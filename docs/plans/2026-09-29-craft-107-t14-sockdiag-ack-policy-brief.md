# Task Brief — T14 bounded SOCK_DIAG + ACK-guarded exact flow

**Issue:** #129 / T14 (Craft #107), live Issue snapshot in the integration Worktree. Approved product Spec: `docs/specs/2026-09-23-craft-web-artifact-spec.md`, stories 12 and 14. Related authority: `CONTEXT.md`, `docs/adr/0004-task-is-session.md`, T00 contract and `docs/plans/2026-09-29-craft-107-t14-conntrack-vs-sockdiag-architecture.md`.

**Detailed plan:** `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-plan.md`.

**Predecessor:** T00 verified/integrated. T14 has only an implementation checkpoint, not acceptance. ctnetlink dump successfully completes with no CT_NEW for an established loopback flow. An unprivileged `docker exec` in the pinned target renderer namespace proved a SOCK_DIAG bounded dump returns the exact client tuple, TCP_ESTABLISHED, inode and cookie; the inode matches `/proc/1/fd/3`. It did not invoke the privileged helper; that remains to be proved live. The response does not encode protocol, so the request must pin TCP. Evidence `docs/testing/craft/t14/2026-09-29-sockdiag-dump-probe/query.json` SHA-256 `f5115d13bf173369fc84c35df4ff5aee3534961d81ca28c80cedceca19c59681`; same path copied in integration Worktree. A wrong-form exact SOCK_DIAG query returns `NLMSG_ERROR -ENOENT`; do not use it.

**Architecture ruling:** use only a bounded `SOCK_DIAG_BY_FAMILY` dump to identify exactly one ESTABLISHED client record matching the host-attested seven fields and `client_socket_inode`. Replace `ct state established` with exactly two fully specified directional tuples guarded by `tcp flags & (syn | ack) == ack`. This is an explicit change from the old implementation invariant, not equivalence to conntrack. The ACK condition is only a packet header test; nft does not bind to inode/cookie. No generic or broad allow is permitted. Preserve no-egress/default-drop behavior and policy lifetime with renderer. Live canaries are mandatory before releasing T15.

**Owner role:** `backend_implementer`. **Validator:** `backend_validator`. **Independent Reviewer:** parent assigns after checkpoint. **Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, branch/worktree HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Existing ctnetlink fix4 hashes are helper `0f129de95e945ee9bf69ccab5f2493a49d9a3c1709f79a68ff646012972ed4c2`, test `8f3dd869a4044f3d130e91934b90e7a512bb78343176546264ba0ca9f57dc724`; preserve history/evidence, do not overwrite its report.

**Write ownership:**
- `deploy/craft/render-boundary/policy-helper/helper.py`
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-report.md`

Do not edit controller.py, barrier_adapter.py, shared contracts, issue/DAG/ledger, or other tickets. No commits, staging, push, stash, Issue writes, Docker commands, image/daemon changes. One Docker resource is reserved for parent-run disposable exact-flow acceptance after Review/validation.

**Consumes:** seven-field flow JSON, target raw query evidence, saved architecture ruling.

**Produces:** validated SOCK_DIAG proof (one tuple/state/inode match), exactly two ACK-set/SYN-clear full-tuple nft exceptions, failure-safe tests, exact-hash implementation report/review package.

**Required RED/GREEN:** add tests first; prove failures and test socket closure. Then implement. Test IPv4/IPv6; malformed response, wrong sender/sequence/port ID, absent/ambiguous tuple, wrong tuple/state/inode, truncation, interrupted dump, error, timeout, limits. Rule tests prove exactly two complete tuple rules and exact flag mask with no conntrack/broad rules.

**Checks (no Docker):** focused SOCK_DIAG unit/query tests; `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v`; `python3 -m unittest tests.test_barrier_adapter -v`; `python3 -m py_compile helper.py tests/test_controller_integration.py`; `git diff --check`. Do not run unittest discovery because class setup builds images.

**Report:** record baseline and final hashes, HEAD, staged/unstaged state, complete untracked file list/content hashes, RED and GREEN command outputs, limitations and exact changes. Do not claim T14 acceptance; only parent can run live checks. Keep current ctnetlink fix4 checkpoint untouched until new implementation and tests complete, then report if its code has been replaced and why.
