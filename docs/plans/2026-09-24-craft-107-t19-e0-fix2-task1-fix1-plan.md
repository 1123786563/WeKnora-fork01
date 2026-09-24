# Craft #107 T19 E0 Probe Infrastructure Fix 1 Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint, independent Spec/quality review. A passing test fixture is not E0 acceptance; measured E0 remains blocked until a complete live run independently verifies all cases.

**Goal:** Close four findings in `2026-09-24-craft-107-t19-e0-fix2-task1-review.md` so the disposable no-bypass probe can produce trustworthy evidence: protected client cannot alter sink logs, verifier derives claims from raw ledgers, timeout windows are quiescent, and hostile config selection has real process evidence.

**Sources:** Approved Craft Spec stories 34–35; T19 egress E0 root-cause plan and measured Fix2 report; independent Task1 review F1–F4.

## Global constraints

- Integration Worktree; own only `docs/testing/craft/egress-probe/{run_v2.py,server_v2.py,assert_v2.py,test_assert_v2.py}` and a new task report/checkpoint. No production code, paid calls, credentials, host firewall changes, commit/push or E1 dispatch.
- Keep raw measured runs immutable. A later script change requires a new bounded disposable run; never amend old manifest or claim coverage across runs.

## Task 1 — Independent raw-evidence boundary

**Role:** backend_implementer. **Consumes:** pinned image, isolated bridge and append-only sink/adapter events. **Produces:** a fail-closed runner and verifier with trustworthy per-case windows.

1. RED: client inspect with `/ledger` mount must fail; synthetic full-schema pass with forged zero ranges or `status=pass` against a bypass GET/CONNECT must fail; missing actual control HTTP/CONNECT, nonzero positive turn, missing manifest PASS, falsified case status/range must fail. Add timeout test where late child request arrives after SIGTERM; no zero-denial verdict may be recorded until termination and stable counters. Add hostile-config case without an effective-config/load trace; it must block.
2. GREEN: remove client `/ledger` mount, keep sink logs host-owned and prove client cannot write them. Tag raw events with monotonic sequence and exact case identity or use bounded start/end sequence under quiescence; independently recompute per-role counts, HTTP/CONNECT controls and each case result from raw events/process logs rather than manifest-supplied deltas/status. On timeout, stop or prove all relevant client processes dead, then wait for listener counters to settle; if not, mark blocked. Tie hostile project/global config to actual pinned OpenCode effective config/load trace and selected baseURL before and after restart, with file-hash comparison. Preserve cleanup and original timeout diagnostics.
3. Run syntax, full synthetic mutation suite, diff check. Save exact pre/post hashes and task-local patch. Then run at most one bounded disposable Docker matrix using the new code and verifier, retain raw append-only logs and cleanup inventory. Report every missing, timed-out or retried case as blocked, even when sink deltas are zero.

**Acceptance:** independent review PASS on infrastructure trust boundary and full raw-verifier reconstruction; a live E0 PASS only if the new run satisfies every required row and `assert_v2.py` exits 0 on that exact artifact. Observed OpenCode retries, missing hostile config proof or timeout keep E0 blocked/default-off.

## Review focus

Client cannot mutate or truncate independent ledgers; verifier does not trust manifest verdict/deltas/ranges; timeout cannot yield false zero; effective config evidence comes from the actual process, and synthetic mutation tests fail as expected.
