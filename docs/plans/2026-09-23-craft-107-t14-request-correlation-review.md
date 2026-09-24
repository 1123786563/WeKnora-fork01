# T14 request-correlation fix: independent scoped review

Date: 2026-09-23. Read-only review of the two Python files in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Sources: approved Craft web-artifact Spec, #129/T14 brief, request-correlation plan/report, previous conflicting-failure review, current preview fixture and release gate. No source/test edits, staging, OCR, Docker or live Chromium run.

## Exact checkpoint and verdict

| Reviewed file | Full-content SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `2cc6588f28e24940da54f953526f2ba56c05f1cd8ccdc1ed8d556d5fdbf3d36a` |
| `deploy/craft/render-boundary/test_probe.py` | `76d6dad68adda47222fec8b8cdd4057110b7eca40cda5d418768ce065e6add59` |

Both are uncommitted modifications and match the implementer report. The concurrently modified preview fixture and runner are outside this two-file checkpoint.

- **Scoped Spec compliance: PASS.** Every observed external request now needs its own matching denial failure by CDP request ID and exact URL/type before its attempt receives coverage. The known second-event, untyped-failure, mixed-abort, and mismatched-ID false greens are closed in the reviewed validator. An unobserved or ambiguous attempt remains an observation limit, and the existing runner exits nonzero on any such limit.
- **Scoped code quality: PASS.** The collector preserves `request_id` across `requestWillBeSent`, `loadingFailed`, `responseReceived`, WebSocket creation and handshake signals, and resolves failure/handshake URLs after collecting the whole log. Tests exercise reversed log order and the prior regressions. No new scoped finding requiring a correction was identified.
- **Full T14 browser/broker acceptance: NOT VERIFIED.** These are synthetic CDP records and browser-free fixture checks. No actual Chromium, container isolation, authenticated preview or broker proof exists. Document-class navigation/popup/anchor/form mechanisms remain 25 uncredited cells, so the release gate remains closed.

## Evidence

`collect_cdp_network_evidence` builds a request-ID-to-URL/type map before emitting signals (`probe.py:486-557`), allowing a reversed performance-log order to retain the proper failure and WebSocket handshake URL. Each emitted family carries `request_id` (`:528-555`). The validator checks exact page-attempt URL, target and mechanism for initiation (`:145-188`), then records every failure and its classification (`:190-240`). It joins each failure to the same request ID and exact URL (`:259-273`); every initiation must have a denial failure, and a conflicting, aborted, unknown, missing-type, missing-ID or unmatched record sets an observation limit before the attempt can enter `failed_ids` (`:274-292`). Any external response or WebSocket handshake rejects the proof (`:242-257`). Duplicate initiation IDs, even with the same URL, remain unobserved (`:183-188`), a defensible fail-closed choice until real CDP traces establish reuse semantics.

The new matrix checks a second Fetch request with no failure, missing ID, `ERR_ABORTED`, missing failure type, and duplicate ID (`test_probe.py:301-325`); additional tests cover a second approved request, unmatched failure ID, wrong URL/target/type, mixed abort in both orders, and conflicting failures for one ID (`:327-447`). Collector tests check all five event families and reversed log order (`:481-516`). The prior read-only counterexamples now return `fetch-public` unobserved rather than crediting `fetch:public`, consistent with these tests. Exact `net::ERR_INTERNET_DISCONNECTED` with no `blockedReason` remains the only credited failure class; other codes fail closed.

I independently ran `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test*.py' -q` (35 passed), `node --test deploy/craft/render-boundary/test_preview_fixture.mjs` (2 passed), Python `py_compile` for the three probe/test files (passed), and `git diff --check` on the two reviewed files (passed). The runner checks `unobserved_browser_attempt_ids` and `browser_observation_limits` before writing accepted evidence (`run-probe.sh:73-85`). A real browser may omit required CDP request IDs or emit failure codes other than the single accepted one; those cases will remain unobserved rather than be mistaken for network-policy proof. Live traces are required to establish whether this strict contract can close the full T14 matrix.
