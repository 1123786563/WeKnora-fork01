# R16 Task 0 parent Lago charge configuration audit

Date: 2026-09-30 (Asia/Shanghai)
Audited checkout: `86b6e7ac0e921f7e1d4fd328ce29be3aa12dc6ce`
Audit input SHA-256: `8945da633656d2f4734b37e5f02eb8e6aa4832c8e3323544d21431a8b1a911c1`

## Verdict and scope

The committed checkout inspected at the exact audited commit contains no authoritative captured parent Plan charge configuration for the R16 admission contract. The static evidence consists of synthetic T04 experiment objects and historical parent-looking readbacks with empty `charges` arrays. This is a historical committed-checkout conclusion only; it does not assert what any current/live parent Plan contains.

The source audit was read from `/tmp/issue72-task0-parent-charge-config-audit-20260930.md`; its SHA-256 was independently checked before archiving. The source reports using `git show`/`git grep` against committed files only. This archive does not inspect live Lago, services, secrets, or dirty owner-controlled files.

## Synthetic T04 evidence (not parent configuration)

[`docs/migrations/lago/t04-pricing-group/README.md`](../migrations/lago/t04-pricing-group/README.md) identifies T04 as a synthetic per-run/per-task experiment. It documents the fixture's zero-price CNY base Plan, standard charge `7` fen/unit, package charge `1500` fen/package, package size `100`, and `free_units: 0`. The corresponding fixture code is [`deploy/lago-lab/pricing-group/fixture.py`](../../deploy/lago-lab/pricing-group/fixture.py), with constants `STANDARD_AMOUNT_CENTS=7`, `PACKAGE_AMOUNT_CENTS=1500`, `PACKAGE_SIZE=100`, and `FREE_UNITS=0`.

[`docs/migrations/lago/t04-pricing-group/t04-state.json`](../migrations/lago/t04-pricing-group/t04-state.json) records synthetic customer external ID `weknora-t04-8ed2a126-05ba-441e-bc04-2a39e99226f7` (Lago ID `b0d533d4-9b1d-4144-9aed-16b8365f9f33`), corresponding synthetic Plan code `weknora-t04-8ed2a126-05ba-441e-bc04-2a39e99226f7-plan` (Lago ID `6c88f43d-89f9-4ca6-8228-1efe3d284449`), and metric codes ending `-model-units` and `-tool-calls`. The model metric Lago ID is `1e3cc8fb-87ae-46cd-85fb-f9f3d94978f8`, attached charge ID `8fcaacec-d63c-4b07-abf8-5038efcba953`; the tool-call metric Lago ID is `6a65cbe5-c77a-4c9f-b493-ffb49b96dfb8`, attached charge ID `d76c6693-b8fa-42df-891d-48bfe328b58f`. The run's dedicated Task subscriptions use the same `weknora-t04-…-task-<k>` identity pattern. These IDs and values belong to the fixture; they do not identify or configure the parent Plan.

The T04 real-stack notes establish Lago v1.53.0 behavior and the fixture's charge encoding only. They do not establish parent Plan charge models, rates, package/free-unit values, or metric-to-product-dimension ownership.

## Parent-looking historical readbacks

At the audited commit, these snapshots have empty charges:

| Committed path | Observed identity and root amount | Observed charges |
| --- | --- | --- |
| [`docs/plans/issue-72-flow-evidence-82/lago-plan-after-publish.txt`](issue-72-flow-evidence-82/lago-plan-after-publish.txt) | `weknora-pro-v1`, Lago ID `06c3afdd-7482-4498-a6ff-8a4c566d2f65`, `amount_cents: 9900` CNY | `[]` |
| [`docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-04-lago-plan-readback.json`](issue-72-flow-evidence-82/r5b-flowcheck/api-04-lago-plan-readback.json) | `weknora-pro-v1`, Lago ID `50aa612f-0a70-4f34-a8ed-23c7224ff459`, `amount_cents: 9900` CNY | `[]` |
| [`docs/plans/issue-72-flow-evidence-84/api-03-lago-plans.json`](issue-72-flow-evidence-84/api-03-lago-plans.json) | `weknora-pro-v2`, Lago ID `b2c826cc-8fa8-4215-baa4-65dc4abcc816`, `amount_cents: 9900` CNY; `weknora-base-v1`, Lago ID `1ff58cf4-8e5e-4100-9f9d-2b0f4f6cf3ba`, `amount_cents: 0` CNY | both `[]` |

These historical readbacks show only that the captured objects had no charges in those snapshots. The root Plan amounts are not charge prices and do not prove any current parent configuration.

## Evidence still required by R16 Task 0

[`docs/plans/issue-72-plan-87-r16.md`](issue-72-plan-87-r16.md) Task 0 remains open. It requires a fresh, isolated, pinned Lago Community v1.53.0 capture that identifies the exact parent publication/version identity and sanitized readback, including each charge's billable metric identity and intended metric-to-dimension mapping, charge model, amount, package size, free units, filters, currency, and pricing-unit fields/paths. It also requires measured parent behavior at quantities `1`, `99`, `100`, and `101`, with exact charge totals, publication receipt/read-after-publish identity and timestamp. The 1/99/100/101 expected values in the plan are conditional test expectations, not observed parent behavior.

Until that evidence exists, amount/package/free-unit/pricing-unit facts and parent metric-to-dimension mapping are unknown. R16 Task 0 is incomplete; #87 is not ready for implementation. The separate #86 integration/acceptance gate remains open. The approved R-6 rule remains explicit published Billable Metric→dimension ownership with dimension-scoped fail-closed behavior for overlap or ambiguity. R-3 remains unchanged: continue excluding Usage Charges from paid purchases and keep `PurchaseService.ensureNoCharges`.

## Prior final OCR result-record review

The independent review archived at [`issue-72-live-tree-and-agent-refresh-2026-09-30-final-ocr-record-review.md`](issue-72-live-tree-and-agent-refresh-2026-09-30-final-ocr-record-review.md) covers range `3a3ec8820150aefb1edf913c0fe775c53722efc2..59ad12e0f6b445186894a4a9cd8ee8422a39dddf`. Its scoped verdict is Spec compliance PASS, document quality PASS, no finding. It also observes the OCR output says `Review skipped: no items were selected.`; therefore the outer OCR gate is incomplete, not passed.
