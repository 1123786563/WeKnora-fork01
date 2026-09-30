# T14 client-abort evidence fix: independent scoped review

Date: 2026-09-23. Read-only review of the two-file checkpoint in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Sources: approved Craft web-artifact Spec (`docs/specs/2026-09-23-craft-web-artifact-spec.md`), T14 #129 brief, abort-evidence fix plan/report, prior evidence-contract review, and the current preview fixture and runner. No source, test, or deployment files were edited; no OCR or live browser was run.

## Checkpoint and verdict

| File | Full-content SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `90a027f6bb120c00b0946fefd66cc454893d843f29f3dac07fa68ffb41b71f23` |
| `deploy/craft/render-boundary/test_probe.py` | `68f30c09810c58c5e71bc319ba54021eaad2df2ddffa0bb222d858fcb0bf5557` |

Both hashes match the implementation report; both files are uncommitted modifications atop the stated HEAD. The other concurrent T14 files are outside this checkpoint.

- **Scoped Spec compliance: FAIL (Medium finding below).** The single-failure defect from the prior review is fixed: `ERR_ABORTED`, `ERR_BLOCKED_BY_CLIENT`, and unknown errors remain unobserved. However, a client-aborted failure can coexist with an accepted failure for the same attempt ID and still receive a covered class. This violates the plan's unconditional “aborted/cancelled requests never enter `covered_classes`” condition.
- **Scoped code quality: FAIL (same finding).** The classifier is narrow and fail-closed for each individual failure, but aggregation loses contradictory causes. Tests cover one failure per attempt only.
- **Full T14 acceptance: NOT VERIFIED.** Synthetic Python and Node checks are not actual Chromium, Docker, authenticated preview, or broker proof. All 25 Document mechanism cells remain ambiguous and the runner rejects incomplete coverage.

## Medium — conflicting failures for one attempt still credit denial

**Evidence and affected symbol:** `validate_browser_evidence` stores an accepted network failure in `failed_ids` (`probe.py:215-217`) and an abort/unknown cause in `ambiguous_failure_causes` (`:218-219`). The final loop checks `failed_ids` first and skips the observation limit (`:238-252`). Both structures are keyed only by URL-derived attempt ID; `run()` discards CDP `requestId` when collecting `Network.loadingFailed` (`:509-516`). Two failure records for the same attempt URL therefore can describe distinct CDP requests without any consistency check.

**Reproduction:** I built the suite's 37-attempt synthetic evidence, retained its `fetch-public` `net::ERR_INTERNET_DISCONNECTED` failure, and appended another `fetch-public` failure with the same URL/type and `net::ERR_ABORTED`. `validate_browser_evidence` returned `fetch:public` in `covered_classes` and omitted `fetch-public` from `unobserved_browser_attempt_ids` (`covered True unobserved False`). This is a synthetic conflicting-record case, not an observed Chromium trace; the fixture normally issues one Fetch call per ID.

**Impact:** If one attempt URL has multiple CDP failures, an application/client abort can be hidden by a network-disconnection failure and the class may be reported as proven. The current Document limits still keep the entire runner nonzero, but this per-class false green matters when those limits are resolved.

**Smallest defensible correction:** Aggregate every CDP observation for an attempt and fail closed when any matching failure has a client-cancel, blocked, unknown, or missing-mechanism cause. Ideally retain and correlate CDP `requestId` across request and failure records so a distinct request cannot inherit another request's evidence. Add a synthetic dual-failure regression, in both record orders, that keeps `fetch-public` unobserved and reports the conflicting cause.

## Confirmed behavior and limits

`_classify_cdp_failure` (`probe.py:58-85`) credits only exact `net::ERR_INTERNET_DISCONNECTED` with empty/missing `blockedReason`; nonempty `blockedReason` takes precedence, and abort/client-block/cancel and unknown codes fail closed. The current preview's Fetch uses `AbortSignal.timeout(1000)` (`preview/app.js:163-172`), so excluding `ERR_ABORTED` directly addresses the earlier false credit. Exact target URL and resource-type checks, external response/WebSocket handshake rejection, and ambiguous Document handling remain (`probe.py:131-278`). An exact disconnection error is evidence of network unreachability in the runner's inspected `--network none` context, but the error string alone does not prove the container policy; no live Docker/Chromium observation was supplied.

I independently ran `python3 -m unittest -q` (22 passed), `node --test test_preview_fixture.mjs` (2 passed), and `git diff --check` on the two reviewed files (passed). Other valid infrastructure failure codes may be reported unobserved; this is an intentional conservative result pending a real trace. The release gate's browser evidence remains open.
