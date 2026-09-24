# T14 conflicting-failure fix: independent scoped review

Date: 2026-09-23. Read-only review of the `probe.py` / `test_probe.py` checkpoint in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`. Sources: approved Craft web-artifact Spec, T14 #129 brief, conflicting-failure plan/report, previous abort-evidence review, current probe runner and fixture. HEAD is `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; reviewed files are uncommitted modifications. No source/test edits, staging, OCR, Docker or live Chromium run.

## Exact checkpoint and verdict

| Reviewed file | Full-content SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `bbd0513168ed0905d47c7b998f6d9814530cbd06360a07b71ad763e465c0f8fd` |
| `deploy/craft/render-boundary/test_probe.py` | `98f2748136e4e55868a086b527d46cba41d6031b2d0682e1874a90f568d46059` |

Both hashes match the implementation report. Concurrent `preview/app.js`, `run-probe.sh`, and their tests are outside this checkpoint.

- **Prior mixed-signal finding: resolved for failures with valid resource types.** An accepted disconnection plus an abort, in either order and under the same or different request IDs, now leaves the attempt unobserved (`probe.py:223-252`; `test_probe.py:314-356`). Repeated approved records for the same request ID deduplicate; multiple unidentified failures conservatively remain unobserved (`:225-245`; tests `:335-366`). Unknown errors and nonempty `blockedReason` remain noncreditable.
- **Scoped Spec compliance: FAIL on the Medium finding below.** The plan calls for every mapped CDP request and failure to be accounted for. An accepted failure still overrides a second request without a matching failure or a second failure lacking mechanism evidence.
- **Scoped code quality: FAIL on the same finding.** Failure classification now aggregates all typed failures, but event-to-failure correlation and precedence for untyped failures are incomplete. The four new tests do not exercise these combinations.
- **Full T14 browser/broker acceptance: NOT VERIFIED.** There is no real Chromium/Docker or authenticated broker proof. All 25 Document mechanism cells remain deliberately ambiguous and the runner rejects incomplete evidence.

## Medium — accepted failure masks other mapped CDP signals

**Evidence / affected symbol:** `validate_browser_evidence` records missing failure resource types in `missing_failure_mechanism` (`probe.py:206-210`) but, if another typed failure for the same attempt is accepted, inserts the attempt into `failed_ids` (`:223-247`). The final loop skips all limits for any `failed_ids` member before checking missing type or missing failure (`:269-285`). Likewise, `external_events` is populated but not correlated with the failure request IDs (`:134-172, :223-252`). Runtime `Network.requestWillBeSent` output does not retain `requestId` for HTTP Fetch events (`:526-539`), whereas `Network.loadingFailed` now does (`:540-548`); thus the validator cannot establish that every observed request failed.

**Read-only reproduction:** Starting from `BrowserProofCoverageTests.make_evidence()`, I assigned the accepted `fetch-public` failure `request_id='request-a'`. Case A appended a second matching `fetch-public` failure with `request_id='request-b'` and empty `type`. Case B appended a second matching `Fetch` request event with `request_id='request-b'` but no second failure. In both cases `validate_browser_evidence` returned `fetch:public` covered and `fetch-public` absent from unobserved (`covered=True, unobserved=False`). These are synthetic traces; no real Chromium duplicate request was observed.

**Impact:** A known request with no denial evidence, or a failure whose mechanism is unproven, can be hidden by a separate disconnection for the same attempt URL. The per-class proof can be falsely green. Present Document limits still keep the full release gate nonzero, but this defect remains material to the eventual gate.

**Smallest defensible correction:** Retain `requestId` for each HTTP `Network.requestWillBeSent` event and use one normalized field name for HTTP and WebSocket events. For each attempt, require every distinct observed request ID to have a matching accepted failure; treat unmatched, unidentified, missing-type, and conflicting records as an explicit observation limit. Give those limits precedence over any accepted failure. Add both reproduction cases to the Python suite, plus a repeated approved-only control.

## Confirmed checks and limits

I independently ran `python3 -m unittest -q` (26 passed), `python3 -m py_compile probe.py test_probe.py test_probe_release_gate.py` (passed), `node --test test_preview_fixture.mjs` (2 passed), and `git diff --check` on the reviewed files (passed). Existing exact URL/target/type validation and external response/WebSocket handshake rejection remain in place (`probe.py:140-267`). The `net::ERR_INTERNET_DISCONNECTED` classification is conservative only in conjunction with the runner's inspected `--network none` context; this review did not execute that context. Multiple failure rows without request IDs now fail closed, which may create an unobserved result for duplicate logging but is safer than false coverage until a live trace establishes semantics.
