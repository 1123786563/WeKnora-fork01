# T07 Web independent review R1

Code range: `a9e4c69844adfc109cbc62d84f808102ca5a852e..1b7f095a87780207956786ffd4d3d5cfabb1c42f`. Date: 2026-09-24.

The reviewer confirmed **R0-F1 and R0-F2 closed**: source-list polling no longer infers upload identity from filename; exact replay retains the original File/request ID/revision; forbidden advances the async epoch and late-response tests cover it. Focused 16/16 passed, and diff check passed. Independent validator also gave PASS for those assigned R1 findings with focused tests and typecheck passing.

The reviewer did **not** approve the full Web subtask: Spec FAIL and code quality FAIL on two new medium findings.

| Finding | Severity | Evidence and required correction |
| --- | --- | --- |
| T07-WEB-R1-F1 | medium | `CareerPage.tsx` treats definitive upload 400 `invalid_request` and 409 `idempotency_conflict` as unknown, retaining `uploadUnknown` and disabling a fresh file/request. Classify terminal rejection separately and allow deliberate new upload. |
| T07-WEB-R1-F2 | medium | A successful POST followed by failed GET `/sources` falls into the upload catch and rewrites a known success as unknown. Separate the follow-up read from POST outcome, retain source/receipt/proposals, and offer later refresh. |

Fix plan: `docs/plans/2026-09-24-issue-140-t07-web-review-fix-r2.md`. No integration or T07 verification occurs before re-review.
