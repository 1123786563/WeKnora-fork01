# Issue #85 — Paid Credits Top-up Implementation Plan (R1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement tasks in order. Each task has one owner and an independent review/validation checkpoint.

**Goal:** Fulfill a channel-paid Credits purchase exactly once through Lago, expose its pending/processing state, and retain the credited batch source and expiry for twelve calendar months after the confirmed arrival.

**Architecture:** Preserve the frozen `CommercialPlatform` three-operation seam. The approved constraints currently have an unresolved feasibility conflict: R-2 prohibits a product-level top-up concurrency cap, while Lago T03 establishes a six-active-Wallet cap and per-Wallet (not per-transaction) expiry/consumption order. A coordinator registry or display overlay cannot itself prevent expired Credits from remaining spendable in Lago. Therefore the only ready work is contract/domain feasibility research; production design and implementation remain blocked until a supported Lago representation proves unlimited per-batch expiry and global consumption order, or an explicit ruling changes a constraint. Do not assume one Wallet per top-up or a pooled Wallet is acceptable.

**Tech stack:** Go, GORM, PostgreSQL and SQLite migrations, Lago Community v1.53.0, TypeScript contracts/API client, React BillingPage, existing Alipay/WeChat confirmation flow.

**Approved sources:** `docs/specs/2026-09-20-lago-billing-migration-design.md` §§Credits, purchase/payment and acceptance; `CONTEXT.md`; ADR-0012; the #72 DAG/inventory and user rulings; GitHub Issue #85 (authenticated read 2026-09-30 03:57 CST; OPEN, `ready-for-agent`, no comments/native children). Recovery/readiness source: `docs/plans/issue-72-parallel-lane-readiness-2026-09-30-refresh2.md`; field-by-field source map: `/tmp/issue72-85-acceptance-source-map.md`; migration boundary: `/tmp/issue72-85-migration-boundary.md`. Plan BASE: `e1a7f66c8680f25d9b61e651bbb4771c43c89991`. This is a conditional plan, not an implementation authorization for unmet dependency gates.

## Acceptance mapping

| #85 acceptance | Planned tasks / observable proof |
|---|---|
| Unpaid, failed, cancelled or mismatched payment creates no spendable Credits | Tasks 2, 5, 7: payment-winner validation before command; command call count and Lago balance unchanged; event remains pending/attention under existing policy. |
| Confirmed payment creates exactly one top-up with source and expiry | After feasibility ruling and contract pass: duplicate callback/outbox/concurrent fulfill; one registry row, one authority grant, one account batch; source `topup`; expiry exactly `effective_at.AddDate(1,0,0)` in UTC. |
| Delayed arrival is observable and recoverable | Tasks 5–7: `processing` projection while settlement is indeterminate; retry returns the same purchase and batch after authority confirmation; client parser and BillingPage render state. |
| Lost response recovers the original invoice/payment/idempotency identity | Tasks 1, 3–5: persist command identity before external dispatch; timeout after create is resolved by querying the same idempotency key / immutable payment reference; never mint a replacement order or second grant. |

## Global constraints

- **Readiness gate:** no implementation task starts until #75 expiry boundary evidence is recorded, #82 and #83 are accepted and their confirmed-payment/activation interfaces are integrated, and the plan has been rebased onto that exact integration checkpoint. Existing code presence or Issue closure is not acceptance evidence. #84 is a scheduling conflict only; serialize its shared order/contracts files after its owner releases them.
- **Isolation:** use a dedicated #85 worktree based on the verified integration HEAD. Do not write in shared root, `lago-int`, #84, or the current plan-only worktree. Do not run shared PostgreSQL tests while Craft #107 Fix21 owns the shared resource; use a verified isolated DSN or wait. Do not start/share Lago stacks or artifact directories owned by another lane.
- **Rulings and feasibility:** #75 R-2 assigns top-up coordination state to WeKnora and requires Lago Wallet creation only at confirmed arrival under an idempotency key tied to the channel payment order, with no product concurrency cap. Twelve-month expiry begins when the batch becomes effective. #87 R-3 excludes Lago Usage Charges from paid-plan purchases; top-up must not use a Usage Charge path. Current code converts fen to micro-Credits 1:1 (`TopUpCredits`), but no approved source located in this intake confirms it as product policy. The pending product-selection ruling gates all amount/quote/credit payloads. Separately, the no-cap rule is incompatible with simply creating one active Wallet per top-up under Lago's six-Wallet cap; pooling into a shared Wallet does not provide per-top-up expiry or global lot order. This is an architecture gate, not an implementation choice.
- **Authority and identity:** Lago is balance/transaction authority; no local spendable Credits before settled Lago confirmation. Use original quote, order, winning channel attempt, invoice, and stable command identity for all retries. Persist an indeterminate state before any external create; read/reconcile by stable identity after timeout. Never issue a new key to escape an unknown result.
- **Expiry:** capture a single immutable `effective_at` for the instant the batch becomes spendable; calculate `expires_at` using UTC calendar addition `AddDate(1,0,0)`; equality is expired. The Lago representation must use the same effective instant, including delayed settlement. Do not reset either value on replay. Preserve batch source/grant/expiry through account snapshots and closed-token API fields. If the authority cannot enforce expiry at this instant, implementation is blocked; a local display overlay is insufficient.
- **Security:** no provider vocabulary, external IDs, URLs, raw response/error text, or credentials cross the commercial API. Bound SQL values, tenant-scope every read/write, and use only server-side configured outbound Lago adapter.
- **Migrations:** at the observed integration checkpoint the migration heads were versioned 000189 and SQLite 000110. R8 reserves 000190/000111 and 000191/000112; #87 R16 reserves 000192–000194/000113–000115. Do not freeze #85 filenames now. Immediately before the migration Task brief, rebase, inspect both dialect heads and reserved files, then allocate next-free numbers and update this plan/ledger. Deliver both dialects.
- **Commit and review:** local task commits are authorized after RED/GREEN, validator and reviewer pass. Do not push, merge remotely, deploy, publish Lago Plans, or mutate GitHub Issues. Root records exact BASE/HEAD and task range.

## Interfaces and owned seams

- Current domain seam: `internal/modules/commercial/platform.go` has `SubmitCommand`, `ReadSnapshot`, `Reconcile`; only additive kinds/payloads are allowed. Current Lago adapter owns provider details in `internal/modules/commercial/commercialplatform/lago.go`; fake adapter contract belongs in `fake.go` and `contract_test.go`.
- Current payment boundary: `internal/modules/commercial/repository/commercial/order.go` stores immutable `OrderRow`, winning `PaymentAttemptRow`, and outbox identity; `service/commercial/purchase_fulfillment.go` validates the winning attempt before activation. #85 must consume that verified winner, not create a parallel payment authority.
- Current monthly registry: `repository/commercial/benefits.go:CreditBatchRow` is unique by `(tenant_id, period)`, and `migrations/versioned/000183...` / `migrations/sqlite/000104...` create it. #85 creates a separate `commercial_topup_batches` model/table; no widening/rekeying of monthly rows.
- Current UI contract: `packages/contracts/src/commercial.ts` already exposes purchase fulfillment `pending|processing|fulfilled|attention` and `CreditBatchView` source/granted/expiry. Extend additively only if #82/#83 exact response needs it. `packages/api-client/src/commercial.ts` has purchase/status/account operations. BillingPage is `apps/web/src/commercial/BillingPage.tsx`.
- Stable identity after upstream integration: `(tenant_id, provider, merchant, merchant_order_id, provider_transaction_id)` from the winning attempt, plus order and quote IDs. The top-up batch registry has a unique confirmed payment identity and a unique command key. If #82/#83 does not expose an immutable provider transaction identity through its winning-attempt lookup, stop and resolve the interface before implementation; do not infer identity from callback delivery.

## Task DAG and dispatch order

All nodes are `pending`. Task 0 and Task 1 are read-only gates. Task 1 must prove joint feasibility for unlimited batches, individual expiry, global consumption order, and the effective-time boundary. Production Tasks 2–8 are blocked until (a) the architecture constraint is resolved without silently violating R-2, (b) the top-up amount/SKU and fen-to-Credits contract are approved, (c) #75/#82/#83 gates are verified and integrated, and (d) the integration worktree is pinned. No production task is presently `ready`.

### Task 0 — Dependency and interface readiness gate

**Dependencies:** none. **Owner:** researcher; **Validator:** backend_validator. **Owned files:** `docs/plans/issue-72-plan-85.md` and a readiness report under `.superpowers/sdd/issue-72-plan-85/` only.

**Consumes:** #75 exact-hash expiry verdict, #82/#83 accepted ledgers/review and integrated APIs; current #72 DAG/Ledger. **Produces:** a pinned integration BASE, verified winner lookup contract (quote/order/attempt/payment/invoice identity), current migration heads/reservations, released shared-file/resource ownership, and a ready/blocked ruling for Tasks 2–8.

- [ ] Verify each prerequisite from durable evidence and Git ancestry; do not infer from issue state.
- [ ] Confirm exact #82/#83 API and database fields return the immutable winning channel payment identity and original Lago Invoice/Payment relation.
- [ ] Confirm `lago-int` or a freshly designated integration worktree contains those reviewed checkpoints; record SHA and migration heads.
- [ ] Confirm #84 has released `order.go` and commercial contract files; confirm isolated PostgreSQL/Lago resources are available before their respective tasks.
- [ ] Expected result: PASS only when every gate is evidenced; otherwise mark dependent tasks blocked with the exact missing evidence and continue no dependent work.

### Task 1 — Confirm Lago top-up create, settle and unknown-result recovery contract

**Dependencies:** Task 0 readiness is not required for isolated read-only contract research; no production implementation. **Owner:** researcher; **Validator:** backend_validator. **Files:** `docs/migrations/lago/t15-topup-credits/contract.md`, redacted fixtures, contract tests only if using the existing owned isolated harness.

**Consumes:** pinned Lago v1.53.0 isolated runtime, #75 T03 verdict and R-2, approved Spec expiry/order requirements. **Produces:** a feasibility verdict and supported mapping among each paid order/payment, authority credit identity, coordinator batch, #86 budget lot, effective time, expiry and dispatch guard. Must establish unbounded top-up count under Lago's six-active-Wallet limit; per-batch spendability/expiry; globally correct monthly/top-up order; per-batch remaining/void semantics; and response-loss/idempotency recovery. Wallet create/readback semantics alone are not sufficient.

- [ ] Start a newly assigned isolated Lago stack only after port/volume/network ownership is verified; use synthetic tenant/order IDs and environment-only credentials.
- [ ] Probe creation with an explicit stable identity tied to the confirmed channel payment order; determine whether Lago itself enforces idempotency or whether adapter must query metadata and fail closed on ambiguity.
- [ ] Simulate response loss after server-side creation, query with the same identity, and assert recovery without a second grant.
- [ ] Probe whether more than six top-up batches can remain independently spendable and expirable without creating more than six active Wallets; test active and expired batches alongside monthly batches and verify actual usage is consumed by global `(expires_at, granted_at)` order.
- [ ] Probe batch-specific remaining balance/refund/void behavior where several top-ups share an authority object; assert local metadata cannot mask still-spendable expired authority balance.
- [ ] Probe delayed settlement and effective-time identity: define one observable instant at which Credits become spendable and prove the authority expiry is exactly twelve calendar months from that same instant, including response loss and delayed visibility.
- [ ] Record exact image digest, request/response shape with secrets removed, hashes and command results; stop stack and retain volumes per lab convention.
- [ ] Expected result: contract PASS only if all identity, capacity, per-batch expiry, mixed-order, void and effective-time properties are demonstrated on pinned Lago. If not, record the counterexample and keep production Tasks 2–8 blocked; return to domain/ADR owner. Do not treat a ledger overlay, short-TTL display, or fake adapter as proof of authority enforcement.

### Task 2 — Add typed top-up command and neutral authority receipt

**Dependencies:** Tasks 0 and 1, approved product amount/SKU ruling. **Owner:** backend_implementer; **Validator:** backend_validator; **Reviewer:** reviewer. **Owned files:** `internal/modules/commercial/platform.go`, new `topup_command.go` and tests, `commercialplatform/lago.go`, `fake.go`, `contract_test.go`, Lago adapter tests.

**Consumes:** Task 1 feasible authority contract, exact #82/#83 winning-attempt identity, and durable product price/amount/Credits/currency/quote ruling. **Produces:** additive `CommandKindCreateTopUp` with typed payload containing tenant, approved amount/credit conversion, source order/payment identity, and an effective-time/expiry representation supported by Task 1; receipt/readback reports `pending|confirmed` without provider fields. Preserve all existing signatures.

- [ ] RED: contract tests reject empty tenant/order/key, nonpositive approved amount, invalid currency/source, inconsistent expiry, cross-tenant identity and unsupported duplicate/mismatched key; fake tracks pending and settled transactions separately. Do not freeze amount-to-Credits conversion until the product ruling is recorded.
- [ ] RED: Lago HTTP tests prove the command sends a deterministic operation to the pinned v1.53 endpoint, preserves bounded product errors, and `ReadSnapshot` (or typed command receipt semantics) can recover by the same command key after a response-loss fixture.
- [ ] GREEN: implement the additive command payload/kind and adapter mapping. Command key is derived once from the confirmed immutable payment order; on an existing key return its original receipt, on indeterminate lookup return processing, and never create a replacement identity.
- [ ] REFACTOR: shared fake/Lago contract suite; reject any state that cannot prove settlement; ensure provider IDs stay seam-internal.
- [ ] Verify: targeted commercial platform tests and `go test ./internal/modules/commercial/...`; expected PASS, no network runtime unless the isolated lab harness is explicitly invoked.
- **Failure handling:** unknown Lago outcome remains processing/retryable; definitive mismatch becomes attention; neither case grants locally.

### Task 3 — Add separate durable top-up batch registry and dual-dialect migrations

**Dependencies:** Task 2 and approved product ruling. **Owner:** backend_implementer; **Validator:** backend_validator; **Reviewer:** reviewer. **Owned files:** `internal/modules/commercial/repository/commercial/topup_batch.go` and tests; newly allocated paired SQL migrations; migration runner tests. Recompute migration filenames immediately before dispatch.

**Consumes:** Task 2 stable command and payment identities. **Produces:** tenant-scoped unique registry keyed by `(tenant_id, provider, merchant, merchant_order_id, provider_transaction_id)` and unique command identity. Row fields: order/quote references, amount, currency, source, immutable `effective_at`, `expires_at`, authority receipt identity internally, closed `pending|confirmed|expired|attention` state, created/updated timestamps. Do not store a writable balance.

- [ ] RED: PostgreSQL and SQLite migration tests assert both tables/indexes, duplicate payment/command rejection, tenant isolation, and multiple top-ups for one tenant in the same month.
- [ ] RED: store tests prove `Begin` persists pending identity before outbound create; reentry returns the original row; conflicting amount/order/payment identity fails closed; confirmation is compare-and-set and fixes effective/expiry exactly once.
- [ ] GREEN: implement transaction-safe insert/readback and confirm transition. On unknown uniqueness races, load the row by its immutable unique identity and compare every immutable field.
- [ ] REFACTOR: parameter-bound, tenant-scoped accessors; no auto-migrated production-only hidden schema; include migration runner on both DBs.
- [ ] Verify: repository package tests, both migration streams and dialect-specific concurrency test on an isolated PostgreSQL DSN; expected all pass without skips.
- **Failure handling:** do not choose IDs at plan time; do not dispatch while shared PostgreSQL owner is active absent a distinct isolated DSN.

### Task 4 — Payment-confirmed, exactly-once fulfillment and lost-response recovery

**Dependencies:** Tasks 2 and 3, approved product/quote contract, plus integrated #82/#83 winner lookup. **Owner:** backend_implementer; **Validator:** backend_validator; **Reviewer:** reviewer. **Owned files:** `internal/modules/commercial/service/commercial/topup_fulfillment.go` and tests; narrow integration to `purchase_fulfillment.go`/`fulfillment.go` only if code ownership is released; `container.go` only if adapter wiring is needed; the exact purchase/status handler source and tests discovered and named in Task 0 are also owned here for the backend `processing|fulfilled|attention` projection.

**Consumes:** immutable paid order + verified winning attempt and quote; Task 2 command; Task 3 registry. **Produces:** a coordinator that first validates the exact winner and quote, creates/loads pending batch state, then issues/reconciles the same Lago command; transitions to confirmed only after Lago settlement; retry resumes original invoice/payment/key.

- [ ] RED: unpaid, failed, cancelled, amount/currency/quote mismatch never submits top-up and never changes authority balance; event stays pending or moves to existing attention policy as appropriate.
- [ ] RED: duplicate callback, concurrent drainers and replay produce one registry row, one command key, one Lago transaction and one Credits delta.
- [ ] RED: simulated timeout after Lago creates transaction but before caller receives response; next retry reads original command/payment identity, settles same batch and creates no duplicate.
- [ ] RED: pending settlement does not update spendable balance; API returns processing; settled replay returns fulfilled with source and expiry.
- [ ] RED: handler projection tests prove provider IDs, URLs and raw errors never appear in purchase/status/account responses.
- [ ] GREEN: wire top-up order line to the new coordinator only for payment-confirmed lines; retain plan activation and legacy OpenMeter behavior only where still required by other verified paths. Remove #85's production OpenMeter binding only after all affected commercial fulfillment kinds have an explicit Lago route; do not silently route unsupported benefit kinds.
- [ ] REFACTOR: keep database transactions outside external calls; persist intent before call; use compare-and-set transitions and bounded retry/backoff; preserve the existing purchase fulfillment event's drain isolation.
- [ ] Verify: focused fulfillment and handler projection tests, full commercial package suite, PostgreSQL duplicate/concurrency tests; exact current-hash tagged Lago lost-response test passes with zero skip.
- **Failure handling:** malformed quote/winner → attention with no grant; unavailable authority/unknown outcome → processing and pending outbox; settled receipt with registry write failure → replay same identity and reconcile.

### Task 5 — Confirmed batch enters account Credits breakdown

**Dependencies:** Task 4. **Owner:** backend_implementer; **Validator:** backend_validator; **Reviewer:** reviewer. **Owned files:** `internal/modules/commercial/service/commercial/benefits.go` and tests, account query/wire projection files explicitly identified in brief, `commercial/topup_fulfillment.go` only via stable interface.

**Consumes:** confirmed registry row plus settled Lago snapshot. **Produces:** account batch with source `topup`, granted instant and exact expiry; spendable total is derived from Lago confirmed balance, and expired rows project zero at `expires_at <= now`.

- [ ] RED: pending/unsettled row is absent from spendable batches; confirmed row appears once with source/granted/expiry; expired equality returns zero and cannot be reactivated by a later stale snapshot.
- [ ] RED: two monthly top-ups retain separate source/expiry rows; a tenant cannot read another tenant's batch or authority wallet reference.
- [ ] GREEN: fold authority snapshot and registry metadata under existing account projection semantics; do not create local credits or balance adjustment.
- [ ] Verify: service/API projection tests, contracts parser tests, existing account/BillingPage regression suite.
- **Failure handling:** if Lago balance and confirmed batch registry disagree, fail closed for spendability and surface processing/attention through existing closed state; no synthetic repair.

### Task 6 — Purchase processing projection and API client contract

**Dependencies:** Task 4. **Owner:** frontend_implementer; **Frontend validator:** frontend_validator; **Reviewer:** reviewer. **Owned files:** `packages/contracts/src/commercial.ts` and tests; `packages/api-client/src/commercial.ts` and tests. Backend handler projection tests remain owned and verified by Task 4's backend validator; this task consumes that API shape.

**Consumes:** persisted pending/confirmed batch state and existing `PurchaseView` fulfillment states. **Produces:** stable `processing` while Lago settlement/readback is unknown, `fulfilled` only after settled balance/receipt, and batch source/expiry in the account response; public responses contain no provider IDs/URL/error.

- [ ] RED: lost response and delayed authority visibility parses/render processing without invented balance; retry/status returns the same purchase; confirmed response parses exact batch face; forbidden provider fields do not appear.
- [ ] GREEN: add only necessary closed product fields; reuse existing fulfillment state tokens wherever possible.
- [ ] Verify: the owning frontend validator runs contract and API client tests using the repository's defined package scripts; expected PASS. Backend handler tests are verified in Task 4 before this task starts.
- **Failure handling:** reject malformed response at parser boundary; keep valid order and processing status visible.

### Task 7A — Server-side top-up catalog/quote and checkout contract

**Dependencies:** approved product-selection and amount/Credits/currency/quote matching ruling; Tasks 0, 4, 6; #84 must release shared order files. **Owner:** backend_implementer; **Validator:** backend_validator; **Reviewer:** reviewer. **Owned files:** dedicated top-up catalog/quote backend seam and its tests, backend routing/handler files explicitly listed in the task brief. No frontend files.

**Consumes:** approved product-selection and purchase accounting rulings, existing WeChat/Alipay checkout links. **Produces:** server-priced top-up selection and quote; confirmed quote then uses existing channel purchase pipeline. Paid-plan Quote/Invoice never contains Lago Usage Charges; top-up matching and invoice treatment must be explicitly compatible with the approved R-3 contract.

- [ ] RED: non-admin cannot list/quote/create top-up; unsupported amount/SKU, zero/overflow, currency mismatch and stale quote reject before payment; retry reuses same quote/order.
- [ ] GREEN: implement the approved shape; server owns amount and Credits derivation; use existing channel path with no client-supplied Credits authority.
- [ ] Verify: backend route/service tests and R-3 payment matching regression. Do not invoke provider payments.
- **Failure handling:** until the ruling is recorded in durable Spec/rulings, this task stays blocked; no SKU, amount, or Invoice matching rule is invented.

### Task 7B — Admin Billing entry and purchase progress UI

**Dependencies:** Task 7A reviewed/integrated and Task 6 reviewed/integrated; #84 releases `BillingPage.tsx`/contract changes. **Owner:** frontend_implementer; **Validator:** frontend_validator; **Reviewer:** reviewer. **Owned files:** `apps/web/src/commercial/BillingPage.tsx` and adjacent component tests only.

**Consumes:** approved catalog/quote API and contracts, existing WeChat/Alipay checkout links. **Produces:** admin-only product selection, confirmed quote display, provider choice, pending payment and fulfillment processing, confirmed source/expiry and retry/status presentation.

- [ ] RED: non-admin cannot access purchase controls; desktop/narrow layout shows selected amount and Credits before checkout, then pending/processing/fulfilled/attention states without invented balance.
- [ ] GREEN: connect only to the server-priced quote and existing channel payment flow; no client-supplied Credits amount.
- [ ] Verify: frontend component/accessibility checks at narrow/wide viewports and checkout-flow regression; no provider payment is triggered.
- **Failure handling:** malformed/unavailable API retains a recoverable state; no client-side quote fallback.

### Task 8 — Issue-level acceptance evidence collation

**Dependencies:** Tasks 0–7B verified and integrated. **Owner:** researcher (evidence collation only); **Validator:** backend_validator for backend/runtime matrix and frontend_validator for UI matrix; **Reviewer:** reviewer. **Owned files:** `docs/plans/issue-72-ledger-85.md`, `docs/migrations/lago/t15-topup-credits/` exact-hash evidence, parent Ledger/DAG update. Validators do not edit product code or tests.

**Consumes:** exact integration HEAD, all task Review Packages/reports, pinned Lago project and isolated PostgreSQL. **Produces:** four-AC mapping with live/fake evidence distinction, verification matrix, accepted issue-level ledger only if every criterion has current-hash proof.

- [ ] Run offline Go, migration, contracts, API client, frontend type/lint/test/build checks specified by repository scripts; preserve exact commands/output and hashes.
- [ ] Run pinned Lago v1.53 isolated end-to-end checkout/payment confirmation, delayed settlement, lost response, exact retry, duplicate callback and final account Credits readback with synthetic identity; confirm unpaid negative path and no Usage Charge rows.
- [ ] Verify expiry payload is exactly arrival + 12 calendar months and test expiry-boundary dispatch rejection; short-TTL boundary proof is distinguished from actual twelve-month elapsed runtime.
- [ ] Run independent full-source review and OCR over the exact supported range/workspace; zero-selected Markdown-only runs do not count as OCR pass. Reconcile exclusions separately under repository policy.
- [ ] Collate the two validators' independently produced evidence without modifying their source; reviewer checks completeness against all four ACs. Promote #85 to verified only if all gates pass at the same final hash; otherwise preserve exact blocked evidence and do not claim completion.

## Review focus

1. Provider command is idempotent after response loss and never mints a new payment/order/invoice identity.
2. `processing` never contributes to spendable Credits; confirmation requires settled Lago truth.
3. One payment identity cannot create two batches across simultaneous callbacks, service restarts, or repeated outbox drains.
4. Twelve-month expiry uses calendar arithmetic, is immutable after arrival, and equality is expired.
5. Multiple purchases in one tenant/month work without colliding with monthly included batches.
6. Provider IDs/raw errors/URLs never escape API/contracts; tenant filtering applies at every layer.
7. The selected top-up amount/catalog is server-authoritative and separate from paid-plan Charges/Quote-Invoice semantics.

## Dispatch state and unresolved inputs

- DAG: `T0 -> T1 -> T2 -> T3 -> T4 -> (T5, T6) -> T7A -> T7B -> T8`; T5/T6 are serial if they share contracts/API paths. T2–T4 and T7A also await durable product amount/quote ruling.
- Current status: T0/T1 pending feasibility research; T2–T8 blocked by Lago batch-semantics feasibility, product amount/quote ruling and #75/#82/#83 acceptance/integration. Shared PostgreSQL is held by Craft #107 Fix21 at the captured refresh; verify again before any DB task.
- Pending user ruling: top-up product choice and conversion. Recommended simplest first release is administrator-entered CNY amount with an explicitly approved fen-to-Credits conversion and server-defined limits; alternatively a fixed SKU catalog requires each amount/Credits value. This decision gates every production command, persisted amount, quote, invoice-match and UI contract.
- Pending architecture ruling: maintain R-2's no-product-cap rule and block implementation if Task 1 cannot prove per-batch expiry/order under Lago's capacity; or explicitly amend the product concurrency policy. The coordinator alone cannot make provider balance authority enforce expiry/order.
- Migration numbers and task BASE are deliberately reallocated/re-pinned in Task 0 at actual dispatch. This plan must be amended and independently reviewed if any API, amount-selection, or upstream identity assumption changes.
