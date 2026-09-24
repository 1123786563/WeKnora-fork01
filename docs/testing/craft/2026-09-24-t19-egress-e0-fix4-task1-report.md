# T19 E0 Evidence Fix4 Task 1 report

Date: 2026-09-24 (Asia/Shanghai)  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Plan: `docs/plans/2026-09-24-craft-107-t19-e0-evidence-fix4-plan.md`, Task 1  
Review: `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task1-review.md`

## Scope and status

Closed the reported malformed-second-request false accept in the assigned files `assert_v2.py`, `test_assert_v2.py`, and `test_source_owned_v2.py`. `server_v2.py`, `run_v2.py`, production code and measured artifacts were not modified. No Docker run or commit. E0 remains BLOCKED. The checkpoint is ready for independent review, but it does not release Fix3 Task 2.

The exact input preimage is under `docs/plans/2026-09-24-craft-107-t19-e0-fix4-task1-preimage/`. It reconstructs the Fix3 Task 1 final files from its verified preimage+patch, then captures the three Fix4-owned files; exact byte sizes and hashes are in `index.json`.

## RED evidence and implementation

Reproduced the independent reviewer’s exact mutation on the passing synthetic full-schema fixture: after direct connection `direct-c1` returned the expected GET, inserted a same-connection `parse_error` before close, then refreshed timestamps, sequence, final seal count, source cursor and raw hash index. The Fix3 verifier returned `PASS` with no errors. This is recorded by the test `fix4-expected-get-then-same-connection-parse-error` and was the RED result before the verifier change.

The verifier now rejects every direct-source `parse_error` and `request_error` as fail-closed observations. This applies regardless of whether they follow a completed expected request, occur on a control connection, fall in a negative phase, or appear outside an active phase. Existing ownership, ID/ordinal, terminal and close checks still run to report additional inconsistencies.

The full-fixture mutations cover expected GET then parse error, expected CONNECT then parse error, GET/CONNECT followed by a partial second request and `request_error`, parse error inside a negative window, and an error after the control connection has closed. `test_source_owned_v2.py` also drives the real local HTTP/1.1 recorder through an expected GET followed by malformed bytes on the same socket and confirms it retains request, response, parse error and close in order.

## Verification

- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed. Full synthetic artifact passes the common verifier; all existing Fix2/Fix3 mutations and six new Fix4 error variants are rejected.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — 15 passed.
- `python3 -m py_compile docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.
- Explicit trailing-whitespace scan — passed. `git diff --check` — passed.
- Patch reconstruction applied to the three captured preimages and reproduced all final file hashes exactly; details are in the checkpoint JSON.

Synthetic assertions verify this narrow stream-accounting seam only. They are not measured E0 evidence.

## Remaining gates

Fix2 review F2/F3 remain: verifier-owned paired pre/post control schedule, raw proof of client stop and helper peer/identity, and full raw command/config/topology/restart/cleanup reconstruction are still incomplete. The unchanged runner is incompatible with the strict evidence schema. OpenCode retry behavior and the measured BLOCKED cases remain unresolved. E0 stays BLOCKED; no live matrix, E1 release, or model-egress enablement follows from this checkpoint.

Exact pre/post hashes, patch reconstruction, tests, and remaining gates are recorded at `docs/plans/2026-09-24-craft-107-t19-e0-fix4-task1-checkpoint.json`.
