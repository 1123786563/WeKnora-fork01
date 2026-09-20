# Ledger — Lago T08: Base Plan Entitlements and Monthly Included Credits (#80)

- Branch: `lago-80-base-plan` (worktree `.worktrees/lago-80`)
- Plan: `docs/plans/2026-09-21-lago-t08-base-plan.md` (approved; Global Constraints binding)
- Ticket: #80 (4 acceptance criteria — all met, mapping below)

## Commits

| Task | Commit |
|---|---|
| 1 — seam surface (ensure_subscription, grant_included_credits, benefits kind, base-tier ladder) | 3fbb71585 |
| 2 — benefits projection + credit batch registry (000180/000101) | 05faec748 |
| 3 — seam commands + benefits snapshot on both adapters + shared contract legs | a74f44f1e |
| 4 — BenefitsService lazy chain + coordinator-owned monthly credits | 3da080c99 |
| 5 — account benefits section + atomic quota enforcement + container | 16cbaf8cc |
| 6 — real-stack evidence + docs | (this commit) |

## Acceptance criteria → proof

| AC | Proof |
|---|---|
| 重复初始化不创建并行 Subscription 或重复额度 | Task 3 shared contract legs (fake + stub Lago: replay = same receipt, count stays 1, different plan code = conflict) + Task 4 `TestEnsureBenefitsIdempotentReplay` / `TestEnsureBenefitsConcurrentExactlyOnce` (8-way race: 1 subscription, 1 wallet, total granted == exactly one month) + real-stack phase 3 (`deploy/lago/evidence/t08-run.txt`: exactly one subscription under the identity, exactly one active period wallet, balance unchanged) |
| Base Plan Entitlement 决定可用功能和资源上限 | Task 1 `TestBaseTierLadder` / `TestBaseTierPublishValidation` (base@0 bottom rung, zero-clause only there) + Task 4 `TestEnsureBenefitsHappyChain` (features/limits land in the projection; `members:5`, `storage_gb:10×2^30`, `concurrent_tasks:2`) + Task 5 `TestAccountBenefitsSection` (benefits section on GET /commercial/account) + real-stack phase 2 (real entitlements snapshot, plan active at price 0) |
| 达到上限后新增被阻止，查看、导出与清理仍可用 | Task 5 `TestMemberQuotaOverLimitBlocksAddOnly` (4xx quota_exceeded on add; list/update/remove open; re-add after removal passes) + `TestConcurrentAddsAtBoundary` (4 racing adds, exactly 1 lands) + `TestStorageQuotaBlocksCreateAtLimit` / `TestStorageQuotaReadPathsNeverCallGuard` (all three knowledge_create sites gated; read paths never consult the guard) + `TestAddMemberFailureReleasesReservation` (compensating decrement) + real-stack phase 6 (admits N, refuses N+1, allows −1) |
| 空间与其他 Tenant 的额度和配额完全隔离 | Derived identities everywhere (`ExternalCustomerID`, `ExternalSubscriptionID`, `MonthlyWalletName` — pure functions, no input can name another tenant's objects) + Task 4/5 store queries keyed by tenant + `TestCrossTenantBenefitsIsolation` (router level: no cross-identity leak, per-tenant batches) + real-stack phase 4 (A/B chains independent, A's balance unchanged, wallet lists do not cross) |

## Runtime verdicts recorded (evidence, not assumption)

1. **Zero-price plan ACCEPTED**: `POST /api/v1/plans` with `amount_cents: 0`
   answered 200 on pinned v1.53.0 — the pre-agreed 1-fen fallback is NOT
   activated; (base, v1) publishes at price 0.
2. **Lazy termination CONFIRMED**: a synthetic short-TTL wallet stayed
   ACTIVE past its expiry within the bounded observation window — the
   registry expiry overlay (pre-dispatch `expires_at` gate) is required,
   and the ≤2-transient-slots budget holds (monthly current + expired
   awaiting the `*:45` tick).
3. **E3 anti-pattern disproven on the real runtime**: a full replay left
   exactly one subscription, one wallet, unchanged balance; the registry
   UNIQUE (tenant, period) fired on the replay insert as designed.

## Design notes (in-plan decisions, executed)

- Monthly issuance is coordinator-owned: ONE short-TTL wallet per calendar
  month (`expiration_at` = exclusive period end), granted lazily on billing
  access through `grant_included_credits` with three-layer idempotency
  (registry unique → adapter read-before-create by deterministic
  name/metadata → never a blind re-POST). `wallet_limit_reached` is a
  bounded retry (≤ ~70 s) then closed `unreachable` (identity replay
  resolves on next access).
- The subscription is a STANDARD subscription without `activation_rules`
  (T02 decision); identity reads pass explicit `status[]` because the
  index defaults to active.
- `concurrent_tasks` is projected and exposed but its admission enforcement
  is explicitly deferred to #87/#88 (documented non-goal).
- Quota application is fail-open by construction: `hard_limit IS NULL`
  (unprojected dimension, outage, unwired stack) means every reserve
  passes; occupancy is re-synced BEFORE limits apply (ordering pinned by
  `TestApplyQuotasResyncBeforeLimit`).
- URL-based knowledge creates reserve the 1-byte floor (size unknown at
  admission) so an at-limit space refuses new growth; the counter re-syncs
  from `tenants.storage_used` at every projection refresh.
- Migration pair 000180 (versioned) / 000101 (sqlite) — self-contained;
  renumber mechanically if #81 lands first with a different number.

## Real-stack evidence

`deploy/lago/evidence/t08-run.txt` + `docs/migrations/lago/t08-base-plan/README.md`.
`TestLagoBasePlanIntegration` PASS on pinned v1.53.0 (project
`weknora-lago-80`, ports 48903/48904; volumes preserved on down).

## Verification

- `go build ./...` green at every commit; final sweep `go test ./...` fully
  green (seam, adapters, repository, service, router, handler, container,
  openmeter untouched).
- `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose
  deploy.lago.test_contract_probe` — 46 tests OK.
- Old OpenMeter gateway untouched (`internal/infrastructure/openmeter/**`
  and its registration unmodified).
