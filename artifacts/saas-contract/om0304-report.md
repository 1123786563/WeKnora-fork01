# OM-03 / OM-04 — Customer 与目录映射 + 付款后幂等发放 (runtime evidence)

- Checks: OM-03 (tenant→customer 映射唯一 / 目录版本重放), OM-04 (订单行重复请求 + 外部成功后本地崩溃 → 只有一份商业权益)
- Instance: local OpenMeter v1.0.0-beta.232 at http://127.0.0.1:48888 (OM-01 verified)
- Namespace: `om0304-20260912` (all write keys prefixed; instance runs implicit `default` namespace)
- Method: direct no-proxy urllib driver (same opener pattern as `scripts/saas/contract_inventory.py::_probe`), ThreadPoolExecutor+Barrier for the concurrent race. `scripts/saas/probe_case.py` was NOT used for these checks because the runner is sequential and cannot express the concurrent race or crash-reissue pattern; request/response evidence is equivalent and fully logged.
- Every claim below comes from an executed request recorded in `artifacts/saas-contract/om0304-request-log.json` (35 requests) and the per-check artifacts.

## Commands (all run from worktree root /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/saas-billing-connectors)

| # | Command | Exit code |
|---|---|---|
| 1 | `python3 artifacts/saas-contract/om0304-cases/om0304_experiment.py` (main experiment: 8-way concurrent customer race, retry replays, plan publish replay, order-line grant dup/crash, V2 contrast attempt 1) | 0 (driver printed section status; OM-03(a) analysis crashed in-driver on `sorted()` AFTER the 8 POSTs fired — crash recorded in `om03a-concurrent-crashrecord.json`, requests preserved in request log) |
| 2 | `python3 artifacts/saas-contract/om0304-cases/om03a_rebuild_concurrent_evidence.py` (rebuild OM-03(a) analysis from the request log + read-only uniqueness GETs; one-time artifact renames — NOT idempotent) | 0 |
| 3 | `python3 artifacts/saas-contract/om0304-cases/om04d_v2_grant_contrast_and_plan_version_uniqueness.py` (V2 contrast retry with valid effectiveAt — creates REAL grants, NOT idempotent; plan version-uniqueness follow-up via version-list endpoints) | 0 |

Honest recording: the first execution of the experiment (before run-2) crashed mid-driver on the same `sorted()` bug after creating customer key `om0304-20260912-cust1` (id `01M29V7MWK3KRK6WZVX51PJ1KM`) server-side without saving artifacts; run-2 therefore used key `om0304-20260912-cust1r2` for a clean race. The run-1 customer is recorded in `om03-concurrent-customer.json` (`prior_run1_customer`).

## OM-03 verdicts

| Sub-check | Verdict | Evidence (artifact) |
|---|---|---|
| (a) 并发创建同 tenant 映射 (N=8 同 key 并发 POST /api/v3/openmeter/customers) | **verified** — exactly 1×201 + 7×409, ONE distinct returned id `01M29VB1DAMEEEFAY2XFZAFEBC`; GET by id → 200 same id/key; list-credit-customers filtered client-side → exactly 1 customer with that key (真实外部 ULID 可查询) | `om03-concurrent-customer.json` |
| (b) 响应丢失重试 — customer 同 body 重复 POST | **verified (409 semantics, NOT idempotent-same-id)** — first 201 (`01M29VB1KFJNT4AGKDB70811DJ`), retry 409 Conflict, no id returned (`same_id_returned: false`) → 重试方必须以 key 查询取回原 id, 不能期望同 id 返回 | `om03-retry-replay.json` |
| (b) 响应丢失重试 — V3 grant body key 重复 | **verified** — first 201 (`01M29VB2CR8R3CXKNC5D499451`), retry 409; list-credit-grants filtered by key `om0304-20260912-grantA` → exactly 1 grant, same id | `om03-retry-replay.json` |
| (c) 版本发布重放 — create → publish → publish again | **verified with deviation** — publish#1 200 (`status: active, version: 1`); publish#2 = **400** (not 200/409): `only Plans in [draft scheduled] can be published/rescheduled, but it has active state`. Effect is still replay-safe: plans list filtered by key → exactly ONE plan, `version: 1, status: active`; no second version. Note: `GET /plans/{id}/versions` route does not exist (404); plan GET by key path also 404 (id-only) | `om03-plan-replay.json` |

## OM-04 verdicts

| Sub-check | Verdict | Evidence (artifact) |
|---|---|---|
| (a) 单订单行重复请求 (grant key `om0304-orderline-1`, amount "50" USD, issued twice identical) | **verified** — first 201 (`01M29VB2VS4YCAP26QFDQRG9XX`); duplicate → **409** with DB-level proof: `duplicate key value violates unique constraint "chargecreditpurchase_namespace_customer_id_key" (SQLSTATE 23505)`; balance after duplicate = live **50** / settled **50** (NOT 100) | `om04-orderline.json` |
| (b) 外部成功后本地崩溃 (response ignored, re-issue same key) | **verified** — reissue → 409; balance still live/settled **50**; transactions ledger: total 1 entry, exactly ONE `type: funded` amount "50" with `available_balance.before 0 → after 50`, id == grant id | `om04-orderline.json` |
| (c) 可唯一查询机制 | **verified** — list-credit-grants filtered by key `om0304-orderline-1` → exactly 1 grant (`matching_count: 1`, one distinct id, status active); balance arithmetic end-to-end 0 → 50 consistent across balance + transactions ledgers | `om04-orderline.json` |
| (d) V2 对比 (无 dedup key) | **verified (negative)** — identical body POSTed twice → **TWO 201 with distinct ids** (`01M29VDKCVD155EQCVYAYVW7S8`, `01M29VDKE67Q5A5B84ECVH1VQW`), entitlement grants 1 → 3. V2 双发真实发生 → V2 不得作为发放族。 First attempt with effectiveAt 03:00Z 400'd (`grant effective date ... before the current usage period 03:17 UTC`) — recorded in request log `om04d-*` | `om04-v2-contrast.json` |

## Key new findings (beyond om02-differences.md)

1. **Dedup is enforced at the DATABASE layer for V3 grants**: the 409 detail exposes `chargecreditpurchase_namespace_customer_id_key` unique constraint — grant body `key` is unique **per customer**, so the same key CAN exist on two different customers (adapter must scope dedup keys per customer, or use globally-unique order-line keys).
2. **Customer duplicate POST returns 409 WITHOUT the existing id** — response-lost retry on customer creation must recover the id via GET/list by key, not from the 409 body.
3. **Plan publish replay is rejected by a state machine (400), not idempotent-200/409** — catalog replay cannot create a second published version; deviation from the acceptance's expected "200 or 409" but with the required outcome (one target version).
4. **V3 has no GET-by-key path for customers or plans** (404); queryability is id-path + list-filter — the adapter must persist the ULID at creation time.
5. **V2 double-issuance re-confirmed with minimal amounts** (1+1): no dedup field exists at all; a V2 issuance adapter needs its own external dedup ledger (per OM-02 recommendation).
6. V2 grant creation validates `effectiveAt` against the entitlement's CURRENT usage period start (400 if before) — relevant to any V2 metering write path.

## Artifacts

- `om03-concurrent-customer.json` — (a) race + uniqueness queries
- `om03-retry-replay.json` — (b) customer + grant response-lost retries
- `om03-plan-replay.json` — (c) publish replay + version uniqueness follow-up
- `om04-orderline.json` — (a)(b)(c) order-line dedup/crash/query + balance arithmetic
- `om04-v2-contrast.json` — (d) V2 double-issuance
- `om0304-request-log.json` — all 35 executed requests with status + full bodies
- `om03a-concurrent-crashrecord.json` — honest record of the in-driver analysis crash
- Scripts: `om0304-cases/om0304_experiment.py`, `om03a_rebuild_concurrent_evidence.py` (read-only rebuild), `om04d_v2_grant_contrast_and_plan_version_uniqueness.py` (writes real grants; run only against the disposable evidence instance)

## Not verified / gaps

- Concurrency race run only once per key (run-1 crashed in analysis; run-2 clean). 8-way race showed a single winner; no repeated race distribution testing.
- OM-03(c) acceptance wording expected second publish "idempotent 200 or 409" — observed 400 state rejection instead (same net effect: one version). Flagged as deviation, not failure.
- V3 grant dedup is per-customer-namespace; cross-customer key reuse NOT tested (inferred from constraint name only).
