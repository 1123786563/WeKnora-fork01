# T14 preview attempt fixture independent review

Date: 2026-09-23. Read-only review of `preview/app.js` and `test_preview_fixture.mjs` in the T14 Worktree at HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Sources: approved #107/#129, fixture plan/report, prior T14 render-proof review, and the current Python `probe.py` validator. Concurrent Python/runner changes were outside the two-file checkpoint; no source/test edits, staging, OCR or real Chromium run.

## Checkpoint and verdict

Both live full-content SHA-256 values match the report: `deploy/craft/render-boundary/preview/app.js` `d6cc139c8b18bb94ae959dbb309b4c97a9d4e47e070a4b98be238fcd9b758cca`; `deploy/craft/render-boundary/test_preview_fixture.mjs` `f5009e860a9f0bbcafe95ecf3c6ccdda98eb4e2518562c21f7ad48a22e459ee3`. The app is modified and the test untracked; neither is committed.

- **Scoped fixture command matrix: PASS.** Six target URLs match the Python matrix, and the page issues six each of navigation, fetch, WebSocket, popup, anchor and form plus the direct `window.location` public attempt (`app.js:9-16, 90-107, 113-196`). Rows have unique expected IDs, mechanism/target labels and exact `/craft-probe?attempt=<ID>` URL construction (`:18-61`). The test checks all 37 rows and actual API invocations under a fake DOM (`test_preview_fixture.mjs:136-180`).
- **Scoped Spec compliance against the current runner contract: FAIL.** The fixture intentionally reports `outcome` rather than fabricating `denied`; the current validator requires every row to contain `denied === true`, so a real runner using this fixture fails before it can assess CDP coverage. See High finding.
- **Scoped code quality: PASS with the contract blocker.** Each command is guarded independently; synchronous exceptions and asynchronous promise/event results update the same row without suppressing later attempts (`app.js:24-61, 164-193`). The local asset and click/key checks remain exercised. The tests use a fake DOM and do not establish Chromium's popup, navigation, iframe, form or network behavior.
- **Full T14 browser/broker gate: NOT VERIFIED.** No live Chromium denial evidence, screenshot from this fixture, or authenticated render broker attestation exists. The separately reviewed Python correlation fix excludes ambiguous `Document` classes from `covered_classes`.

## Finding

### High — page telemetry and Python denial contract cannot interoperate

**Evidence / affected symbols:** `runAttempt` constructs each row with `attempted: false` and `outcome: {kind: "pending"}`; later callbacks change `outcome` only (`preview/app.js:38-61, 64-70, 98-105, 157-183`). No path sets `denied`. The test explicitly asserts every row lacks a `denied` property (`test_preview_fixture.mjs:161-162`). In contrast, `validate_browser_evidence` raises `ValueError` unless `attempt.get("denied") is True` for each row (`probe.py:87-106`), and `run-probe.sh:73` invokes that validator on the captured fixture output. The fixture report itself acknowledges the mismatch. `form.submit()` also does not emit a normal `submit` event, so the handler at `app.js:157` does not supply a denial signal for form attempts; iframe load/error alone cannot prove egress denial.

**Impact:** The 37 commands can run, but the real probe cannot produce a successful validated evidence artifact with the current app/validator pair. Passing two fake DOM tests must not be presented as a passing browser boundary test. Automatically setting `denied=true` when an API was invoked or a page event fired would create a false green.

**Smallest defensible correction:** Define one evidence contract across fixture, CDP collector and validator. Keep page rows as command/outcome facts; derive denial only from correlated browser failure or an explicitly bounded, truthful observation limit, using the target/type checks already added to the Python validator. Add a cross-language contract test that feeds the fixture's actual telemetry shape to the validator, plus a real Chromium run for final proof. Popup-blocked rows must remain `attempted=false` and unverified rather than being counted as requests.

## Verification and limits

I independently ran `node --test deploy/craft/render-boundary/test_preview_fixture.mjs` (2 passed), `node --check deploy/craft/render-boundary/preview/app.js`, and `git diff --check` for the two files; all passed. The report records a RED run against the original app (15 versus 37 rows), which I did not rerun. The harness checks URL construction and command calls, not browser egress, CDP event origin, actual popup creation, or denial. Popup attempts are initiated from the Selenium click handler before `startEgressAttempts`, matching the runner's click → start order, but six real popups from one gesture are not established by this fake window implementation (`test_preview_fixture.mjs:92-96`).
