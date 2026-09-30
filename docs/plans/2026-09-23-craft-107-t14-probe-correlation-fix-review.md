# T14 probe correlation fix independent review

Date: 2026-09-23. Read-only review of the two-file scoped checkpoint in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Sources: approved #107/#129 Spec, T14 render boundary and render-proof-fix reviews, correlation fix plan and implementation report. Concurrent frontend `preview/app.js` and fixture changes were outside this review and untouched. No OCR, staging, or code edits.

## Exact checkpoint and verdict

| File | Full-content SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `7fd72f1cd1f868378abd955041bc3fd2ec851005c3c2d1cc2beab2e556f4e982` |
| `deploy/craft/render-boundary/test_probe.py` | `26717bc275fdeecc67b2381a51b41458baddbcd8fb62cff47af479b3d50e1383` |

Both hashes match the implementation report. The files are modified but uncommitted. `python3 -m unittest test_probe.py` passed independently (14 tests), as did `python3 -m py_compile probe.py test_probe.py` and `git diff --check` for the two files. The report records RED on all three new regression cases before the validator change; I did not reproduce that historical state.

- **Scoped Spec compliance: PASS.** Page attempt URLs and every mapped CDP request/failure must now match the expected scheme, host, port, path and attempt ID (`probe.py:41-55, 87-106, 118-144, 157-188`). CDP type must match `Fetch`, `WebSocket`, or `Document`; an event on a private URL with `fetch-public` cannot credit the public attempt. Wrong types and wrong failed-request destinations raise errors rather than being discarded (`test_probe.py:191-213`).
- **Scoped code quality: PASS.** The `Document` CDP type cannot distinguish navigation, popup, anchor and form, so these events are tracked as ambiguous and excluded from `covered_classes` (`probe.py:145-150, 207-263`; `test_probe.py:215-233`). The synthetic matrix reports those attempt IDs under `unobserved_browser_attempt_ids` and `ambiguous_cdp_mechanism`. A Fetch/WebSocket event with no matching loading failure retains an explicit page-error observation limit. Successful external responses/handshakes still fail validation (`probe.py:192-205`). No scoped blocking finding remains.
- **Full T14 browser/broker gate: NOT VERIFIED / BLOCKED.** There is no actual Chromium egress run, screenshot or authenticated broker attestation for this checkpoint. Synthetic validator tests cannot establish effective browser denial. `run-probe.sh:73-90` records the coverage summary but does not require `unobserved_browser_attempt_ids` to be empty, so an exit-zero probe must not be read as proof that all 37 mechanism/target cells have Chromium-confirmed coverage. In particular, the document mechanisms remain unconfirmed by CDP type alone. This is an explicit end-to-end gate, outside the two-file correlation fix; a later proof must add distinguishable mechanism evidence or report the cells as unverified.

## Review detail

The URL matcher compares parsed origin and path and rejects embedded credentials/fragments (`probe.py:46-55`). The attempt query must identify exactly one known ID (`:41-43, 91-99, 122, 161`). Both the page and CDP loops reject a known external attempt routed to the preview origin (`:124-127, 163-166`), and mapped external requests/failures with unsupported schemes or wrong destinations fail (`:128-144, 167-188`). This closes the prior Medium wrong-target/wrong-mechanism finding. Extra non-attempt query parameters are tolerated; the plan's required scheme, host, port, path and attempt ID still match.

The browser collection retains request resource type for `Network.loadingFailed` where available (`probe.py:469-495`) and records WebSocket creation/handshake telemetry (`:505-512`). It does not create independent proof of how a `Document` request was initiated; the validator's explicit ambiguity is the correct result for that data. Concurrent frontend fixture changes need their own review and cannot be inferred from this checkpoint.
