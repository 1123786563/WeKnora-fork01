# T19 E0 Fix1 Task 1 report

Date: 2026-09-24 (Asia/Shanghai)  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Plan: `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-fix1-plan.md`  
Checkpoint: `docs/plans/2026-09-24-craft-107-t19-e0-fix1-task1-checkpoint.json`

## Verdict

**DONE_WITH_CONCERNS — Fix1 static and synthetic evidence is passing; measured E0 remains BLOCKED/default-off.** One bounded disposable run was attempted and stopped during setup on an embedded-NUL error in the new process-probe command. That source defect was corrected after preserving the raw run artifact. The brief allows at most one live run, so no second Docker run was performed. No full post-fix live matrix or hostile-config process trace exists.

## Fixes

- Removed the client container's writable `/ledger` bind mount. The sink containers retain host-owned ledger mounts. Runtime topology records the client's mount inventory and a write attempt; the attempt must fail for topology to pass.
- The verifier now rejects any raw external sink event tagged with a denied or hostile-config case, checks the normal adapter POST count against raw adapter events, and requires raw successful HTTP controls for gateway/provider/proxy plus a successful proxy CONNECT. Negative verdicts no longer use manifest `ledger_delta` as the zero-traffic proof. Case ranges are rejected when beyond raw ledger lengths.
- Timeout handling sends SIGTERM to OpenCode processes, then requires those processes to be absent and host-owned listener counters stable for multiple samples before reporting quiescence. Original timeout stdout/stderr remains in the command record. Failure to prove quiescence is blocked.
- Hostile project/global cases now require a process log trace with the actual loaded config path, selected provider, and intended baseURL; the in-container config hash must also match the host fixture hash. Selected argv or a host file hash alone no longer passes.
- Synthetic mutation tests cover writable ledger mount, a raw tagged bypass hidden behind forged zero deltas, absent actual CONNECT control, forged positive adapter request count, out-of-bounds ranges, missing case PASS status, missing hostile process trace, a late post-SIGTERM event, and a still-live process.

## Verification

- `python3 -m py_compile docs/testing/craft/egress-probe/{run_v2.py,server_v2.py,assert_v2.py,test_assert_v2.py}` — passed after correcting the process-probe escape.
- RED evidence: the first Fix1 mutation run stopped at `client-can-mount-ledger: forged fixture unexpectedly passed`, confirming the verifier accepted the client-writable-ledger claim before the fix. The final mutation suite rejects that forgery.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed. The complete labelled synthetic schema passed; all eight original malformed evidence fixtures and seven Fix1 mutation fixtures were rejected as expected. Quiescence tests included a late counter increment before stability and blocked while a process remained alive.
- `git diff --check -- docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py` — passed.
- One bounded invocation of `python3 docs/testing/craft/egress-probe/run_v2.py` — **blocked**, exit 1 after approximately 75 seconds with `ValueError: embedded null byte`. Artifact: `docs/testing/craft/egress-probe/e0-fix2/craft-e0-20260923T200320Z-82947/`. The artifact records `client_mounts_ledger=false`, `client_ledger_write_denied=true`, `cleanup.resources_absent=true`, and cleanup exit 0. `docker ps -a`, `docker network ls`, and `docker volume ls` filtered by this run ID returned no resources. The embedded-NUL escape was corrected after this preserved run, and later source hardening added raw HTTP event counts, per-case ledger reconciliation, direct-listener case attribution, and exact control checks. The run therefore exercises the client ledger-mount topology but not the final runner/verifier source. It was not rerun under the one-run limit.
- The previous measured Fix2 run was not modified. Its manifest remains SHA256 `2218e83e6d145a959d67b766dbd54804729f14c1fa015da75a45522c7a631b10`.

## Limits

The sole Fix1 run did not cover the full matrix, actual timeout quiescence, or process-observed hostile config selection. The static source now requires those proofs, but no live proof was collected. The strict verifier on the earlier Fix2 measured artifact exits nonzero (expected; it predates the trust-boundary evidence). Keep E0 blocked/default-off and do not dispatch E1 until a separately authorized run satisfies the complete matrix and an independent review approves the raw-evidence boundary.

No production files, credentials, paid calls, host firewall settings, commit, or push were used. Existing T01/H2/T19 edits and all earlier fixture artifacts were preserved.
