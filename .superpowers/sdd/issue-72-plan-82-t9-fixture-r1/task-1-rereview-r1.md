# Task 1 scoped re-review — fix round 1

Reviewed fix base `9d3df6f961142a6c5e0734d9eea4a283b34859d7` through HEAD `ac343f22e480acebff733e058efe3e46e4d0924c` against the refreshed Task 1 brief, appended implementer report, supplied diff package (read once), and the prior task review. This is a read-only review of the fix; no Git commands, tests, OCR, network, Docker, database, or payment flow were run.

## Prior findings

### F1 Medium — ADDRESSED

Prior finding: `succeededPaymentIntent` returned the first eligible row if multiple captured PaymentIntents appeared succeeded, so it could produce a webhook for a PI other than the server-selected latest intent.

`preSettlePaymentIntentCandidates` now keeps each invoice-linked unsettled PI's ID, invoice ID, and positive integral creation time (`lago_settlement_integration_test.go:165-198`). `expectedPaymentIntentID` selects the unique greatest creation time and rejects a latest-time tie (`:200-218`). The live harness resolves this target before `SubmitCommand` (`:390-421`), then `succeededPaymentIntent` accepts only that exact ID with succeeded status and nonempty invoice metadata (`:220-239`, call at `:438-447`). The regression covers two candidates in reverse chronological list order, two succeeded rows with the newer selected, older-only success failing with expected/observed IDs, and a tied latest timestamp failing before target selection (`:58-105`). This matches the requested repair and removes the prior first-eligible-row behavior.

### F2 Low — ADDRESSED

Prior finding: `docs/plans/issue-72-execution-ledger.md` had the incorrect checkpoint `4ac0c0af...`.

The ledger now records implementation checkpoint `7e7975a7262dcea43f0fbef42df54438b39af836`, plan record `9d3df6f961142a6c5e0734d9eea4a283b34859d7`, and review record `fac64ec95f3a7af2343069ef016060cda4dc5337` separately (`docs/plans/issue-72-execution-ledger.md:365`). These agree with the supplied package and refreshed brief.

## New breakage

None found in the fix diff. The test retains `lago_integration`; no production source or public interface changed. Missing/invalid/tied candidate timestamps now fail before the charge-bearing settle command, while an older-only post-settle success fails without synthesizing a webhook.

## Out-of-scope observations and evidence limits

- The implementer reports RED build failure for the not-yet-implemented helpers, then focused tagged PASS, target package PASS, tagged compile-only PASS, gofmt PASS, and `git diff --check` PASS in `task-1-report.md`. I did not rerun them under the review constraints.
- The added `InvoiceID` is captured but the post-settle helper checks only that invoice metadata remains nonempty. Comparing invoice values was not part of the requested F1 correction; the same PI ID is the controlling identity here. A separate review of metadata mutation or provider response integrity would need its own scope.
- The dedicated live T9 rerun and AC4 real Alipay sandbox evidence remain incomplete. This fix review does not establish those acceptance gates.

## Verdict

**Spec Compliance: PASS for the scoped fixture repair. Task Quality: PASS for fix round 1.** Both prior findings are addressed, with no new breakage identified from the supplied diff. The verdict is limited to this test fixture and recorded checks; live T9 and AC4 acceptance remain open.
