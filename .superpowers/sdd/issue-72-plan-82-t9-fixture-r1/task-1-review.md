# Independent Task 1 review — Issue #82 T9 fixture identity

Reviewed BASE `8329b85d4dfff301d03f94406dfc829d87cb5b26` through HEAD `9d3df6f961142a6c5e0734d9eea4a283b34859d7`, using the supplied review package once, the Task 1 brief and report, approved Lago billing Spec, ADR-0012, `CONTEXT.md`, and current source/documentation. No Git commands, tests, OCR, Docker, database, network API, or live payment flow were run for this review.

## Verdicts

- **Spec Compliance: FAIL (scoped).** The tagged harness excludes historical succeeded IDs and captures only invoice-linked unsettled IDs, but an ambiguous post-settle success set is accepted rather than failing closed. Live T9 and AC4 sandbox acceptance remain incomplete as correctly disclosed.
- **Task Quality: FAIL (scoped).** The deterministic regression covers the primary historical-success case, but misses the ambiguous case and the ledger's checkpoint SHA does not match the supplied review package. The reported tests support compilation and primary selector behavior; they do not establish the live path.

## Strengths

- `lago_settlement_integration_test.go:1` retains the `lago_integration` build tag. The supplied package changes no production source or public interface.
- `preSettlePaymentIntentIDs` at lines 149–167 admits only nonempty IDs with nonempty `lago_invoice_id` and the three allowed unsettled statuses; the bounded wait at lines 352–367 reports HTTP status and observed IDs on expiry.
- `succeededPaymentIntent` at lines 169–187 requires a captured ID, succeeded status, and nonempty invoice metadata, and reports expected versus observed IDs when none matches. The test at lines 58–87 covers an unrelated historical success, an unlinked candidate, a matching candidate, and a no-match diagnostic.
- The #82 ledger at lines 246–251 preserves both the dedicated live T9 rerun and real Alipay sandbox evidence as incomplete.

## Findings

### F1 — Medium — Ambiguous post-settle success silently selects the first row

**Evidence / affected symbol:** `internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go:169-185`, especially the immediate `return intent, nil` at line 184. The pre-settle gate at lines 356–363 accepts any nonempty candidate set. If two captured, invoice-linked unsettled IDs both appear as succeeded in the post-settle list, the helper returns whichever appears first. The production settle path resolves a single `latestIntent` and applies same-invoice/ambiguity gates (`lago_settlement.go:151-169`), but the test selector retains neither the resolved target nor a uniqueness check. `TestPaymentIntentCandidateSelection` has only one eligible captured success (`:58-87`).

**Impact:** On a reused/shared customer or concurrent provider activity, the synthetic webhook can represent a different captured PI from the one selected by this settle command. The live test can then attribute an unrelated success to this settle, or hide a multiple-success anomaly. This violates the brief's same-selected-PI and fail-closed ambiguous-identity constraints even though it prevents the original arbitrary historical-success fallback.

**Smallest defensible correction:** Preserve enough pre-settle identity to prove the intended PI under the existing server disambiguation rule, then require that exact ID after settle; alternatively require exactly one eligible success transition and fail with candidate/observed IDs if more than one exists, while confirming it is the server-selected PI. Add a deterministic two-candidate/two-success regression and a two-candidate/one-success regression.

### F2 — Low — Durable checkpoint references an unrelated SHA

**Evidence / affected file:** `docs/plans/issue-72-execution-ledger.md:365` records checkpoint `4ac0c0af802d5154271d361813e48cba24a3f78f`, while the supplied BASE..HEAD review package lists `7e7975a72` as the test repair commit and `9d3df6f96` as the plan/report commit. The Task 1 report does not identify `4ac0c0af...` as its checkpoint.

**Impact:** Recovery or downstream integration following the ledger may select a different version than the reviewed code, weakening the evidence-to-checkpoint link.

**Smallest defensible correction:** Reconcile the ledger against the controller's actual task checkpoint and record the exact full SHA (and reviewed HEAD if different). Keep the review package and validation evidence bound to that same revision.

## ⚠️ Cannot verify from this review

- The same-checkpoint test results, formatting, and diff check are reported in `task-1-report.md:16-24`; I did not rerun them as instructed. No RED command result is recorded despite the brief's explicit RED step, so the stated TDD sequence is not independently evidenced.
- The dedicated T9 live stack and real Alipay sandbox were not run here. The #82 ledger correctly keeps both incomplete. A tagged compile-only pass and selector unit pass cannot prove the real settlement-to-webhook chain.
- The supplied diff package and ledger disagree on the checkpoint SHA (F2); no Git command was permitted to resolve which reference is authoritative.
