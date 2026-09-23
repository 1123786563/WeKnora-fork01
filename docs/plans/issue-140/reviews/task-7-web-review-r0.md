# T07 Web independent review R0

Code range: `85f950529..a9e4c69844adfc109cbc62d84f808102ca5a852e`. Date: 2026-09-24. Verdict: **Spec FAIL; code quality FAIL**. Focused reviewer rerun: 13 tests passed. Full suite/build evidence from implementer: 2314/2314 tests, typecheck and build passed. These tests did not cover the following behavior.

| Finding | Severity | Evidence and required correction |
| --- | --- | --- |
| T07-WEB-R0-F1 | high | `CareerPage.tsx` unknown-upload recovery matches `/sources` by `fileName`. An old same-name ready, failed or processing source can be mistaken for the current request. The public source has no request ID. Keep the attempt unresolved after source-list polling; use exact replay with retained File/request ID/expected revision to establish its outcome. Add same-name collision tests. |
| T07-WEB-R0-F2 | high | Forbidden cleanup clears state but does not increment the async epoch. A late upload or source response can repopulate private sources/proposals after same-scope authorization loss; recovery notices also write after awaits without epoch recheck. Invalidate the generation on forbidden, fence each awaited continuation, and test forbidden plus late response. |

Independent frontend validator's focused tests and typecheck also passed at the same code SHA. Validator separately identified the filename ambiguity and noted that live browser/responsive/keyboard/assistive-technology acceptance remains pending. The round-1 fix plan is `docs/plans/2026-09-24-issue-140-t07-web-review-fix-r1.md`. No integration or T07 verification occurs before the fix is independently reviewed.
