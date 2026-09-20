# Lago T08 — Base Plan Entitlements and Monthly Included Credits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 新空间通过同一条 ensure 链（#78 ensure_customer → 本票新增 ensure_subscription）进入 **Base Plan**（基础档），在 Billing 页面看到生效权益（Entitlement）、Resource Quota 与套餐内 Credits 月度额度；资源增长路径按配额原子阻断（超限状态 = 阻新增、保留查看/导出/清理），且与其他 Tenant 的额度、配额完全隔离。(Ticket #80, Wave 4; CONTEXT.md terms: 基础档 / 套餐内额度 / 超限状态 / 资源配额.)

**Architecture:** The frozen port `internal/commercial/platform.go` is NOT edited; growth is additive in a NEW file `internal/commercial/subscription_command.go` (exact `plan_command.go` precedent): two new CommandKinds — `ensure_subscription` (idempotent-by-identity standard subscription on the Base Plan version, **no activation_rules**: T02/#74 decision — payment-gated subscriptions need provider+payment-method in Community, and the free Base Plan must never gate on payment) and `grant_included_credits` (coordinator-owned monthly issuance) — plus one new SnapshotKind `benefits` with an additive `Snapshot.Benefits` section. **Monthly included-credits design (binding, from the #75/T03 verdict):** Lago Community has NO native recurring wallet issuance (interval-rule creation is Premium-gated AND 500s, `source-notes.md` "Interval rules"), so the **coordinator owns monthly issuance**: one **short-TTL wallet per calendar month** (`expiration_at` = month end — verdict a1 obligation 3 prescribes exactly this form; a rolling wallet + coordinator void-then-regrant would be non-atomic across the month boundary and entangle included and future top-up batches in one wallet), granted lazily at billing access via `grant_included_credits` with deterministic identity `grant_included_credits:<ext-customer>:<YYYY-MM>` + deterministic wallet name + metadata, because Lago wallet/wallet-transaction APIs carry **no external idempotency** (E3: a replayed grant POST doubles the balance — recovery is by metadata query, never blind re-create). The local **batch registry** `commercial_credit_batches` (unique per `(tenant_id, period)`) anchors the two remaining verdict obligations: pre-dispatch expiry rejection (lazy termination leaves an expired wallet consumable up to ~65 min — E1) and wallet-cap accounting (hard cap 6 active wallets per customer: monthly cadence occupies ≤ 2 transient slots — current month + the just-expired one awaiting the `*:45` clock — leaving ≥ 4 for future top-ups; the a2 twelve-month-top-up BLOCKED escalation stays a design-phase decision, untouched here). Settle-wait (a1 obligation 2: wallet-create credits settle via an after-commit job, so immediate consumption skips the newest wallet) is a bounded balance poll inside the adapter. A new `BenefitsService` (`internal/application/service/commercial/benefits.go`) runs the chain **lazily on billing access** (the documented #78 trigger): seed/publish the built-in Base Plan `(base, v1)` idempotently through the existing `PlanVersionService` (#79 flow — ladder gains a zero-priced `base` rung) → `EnsureBillingAccount` (#78) → `ensure_subscription` → `grant_included_credits` for the current period → write the **benefits projection** (`commercial_tenant_benefits`, migration `000180` versioned / `000101` sqlite) → apply quota `hard_limit`s onto the existing `commercial_resource_counters` (CAS mechanism already in `CommercialHandler.ReserveResource`) and re-sync `used` from observed occupancy. Enforcement wires at the real growth paths: `TenantMemberHandler.AddMember` (+1 `members` CAS with compensating decrement) and the three `knowledge_create.go` storage sites (bytes under dimension `storage_gb`); 超限状态 keeps read/export/clean open by construction (delta ≤ 0 always passes; those paths never touch counters). `GET /commercial/account` grows an additive `benefits` section (plan/features/limits/credits breakdown) in closed WeKnora vocabulary — external plan codes, subscription ids, wallet ids and every Lago token stay seam-internal. `concurrent_tasks` is projected and exposed but its admission enforcement lands with the usage-admission tickets (#87/#88) — documented non-goal. Old OpenMeter gateway untouched.

**Tech Stack:** Go 1.26 (`github.com/Tencent/WeKnora`), gin, gorm (SQLite tests + PostgreSQL versioned migrations), `net/http` + `httptest`, Docker Compose (`deploy/lago/lago.sh`) for the tagged real-stack evidence on pinned Lago Community `v1.53.0`.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — "Catalog, subscriptions, and entitlements" (subscription continuity on one external identity; Lago Entitlement defines features/quotas while WeKnora enforces occupancy atomically and preserves view/export/cleanup when growth is blocked; expiry moves to Base Plan preserving data and unexpired top-ups), "Credits and Task admission" (included Credits expire at monthly period end, no rollover; consumption earliest-expiry — priority encoding per verdict), "Local persistence and state projection" (Subscription/Entitlement/Wallet local records are rebuildable Billing Projections), "Public product states"; user stories 1 (per-space isolation), 14 (Base Plan transition), 15 (top-ups independent of quotas — respected by not consuming extra wallet slots), 29/30 (view/export/cleanup under outage; overage blocks only growth); behavior matrix 2 (tenant isolation), 17 (plan lifecycle Base-Plan leg), 21 (permissions). [ADR-0012](../adr/0012-lago-as-commercial-billing-authority.md), [ADR-0014](../adr/0014-commercial-platform-single-deep-seam.md). Lab verdicts (binding): `docs/migrations/lago/t03-wallet-semantics/verdict.md` (a1 monthly PASS-WITH-COORDINATION with three obligations; E3 no-grant-idempotency; 6-wallet cap; consumption order `priority ASC, created_at ASC`) + `source-notes.md` (interval rules Premium-gated/500; one-off invoices never draw wallets; hourly `*:45` lazy termination); `docs/migrations/lago/t02-payment-activation/DECISION.md` (payment gating needs a supported provider — Base Plan must be a standard non-gated subscription; subscription index defaults to `status: active` — identity reads must pass explicit statuses; `POST /api/v1/subscriptions` requires its own `external_id`).

## Global Constraints

- **Isolation:** all work on branch `lago-80-base-plan` in `.worktrees/lago-80`. Real-Lago evidence uses its own stack: `COMPOSE_PROJECT_NAME=weknora-lago-80`, `LAGO_API_PORT=48903`, `LAGO_FRONT_PORT=48904`. Never reuse another ticket's Compose project, volumes, or ports; `lago.sh down` preserves volumes per T01 convention.
- **Seam freeze (binding, T05 Task 2 / ADR-0014):** the three method signatures in `internal/commercial/platform.go` are FROZEN and the file is NOT edited. Growth is additive only: the new kind constants (`ensure_subscription`, `grant_included_credits`), their typed payloads, `SnapshotKindBenefits` + `BenefitsSnapshot` + the additive `Snapshot.Benefits` field live in a NEW file `internal/commercial/subscription_command.go`… with one narrow exception: the `Snapshot` struct gains the `Benefits` pointer field, which requires appending one field to `platform.go` — the exact pattern #78 used for `Snapshot.Account` (additive field + comment, no signature change); everything else stays out of the frozen file. No per-object wrapper methods (`GetSubscription`/`GetWallet` style) may appear anywhere.
- **Provider neutrality:** Billing API responses carry only WeKnora product vocabulary and closed tokens. Lago plan codes, subscription external ids, wallet `lago_id`s, URLs, status enums and raw error text never cross the API — the local projection/publications tables are the mapping. Errors reuse the T05 closed sentinel mapping (`unconfigured|unreachable|invalid_response|unsupported` / product tokens); `err.Error()` is never echoed. Adapter identities (`ExternalCustomerID`, plan codes, wallet names) are WeKnora-derived deterministic strings — they live inside the seam.
- **Deterministic identities (one per tenant, forever):** `ExternalCustomerID(t)` (#78, reused); NEW `ExternalSubscriptionID(t) = "weknora-tenant-<id>-sub"` — subscription continuity identity (spec: same identity across upgrade/downgrade/Base-Plan transition), defined once in the seam file and used by service, both adapters and tests; wallet name `"weknora-tenant-<id>-<YYYY-MM>"` + metadata `{weknora_tenant, weknora_period}` — the E3 recovery-by-metadata obligation.
- **Monthly issuance is coordinator-owned (T03 verdict, binding):** no Lago recurring rules exist in Community (create 500s); every month is ONE short-TTL wallet (`expiration_at` = period end, UTC), granted through the `grant_included_credits` command whose Key is deterministic per `(tenant, period)`. Grant idempotency has three layers, mirroring #78: local registry unique `(tenant_id, period)` → adapter read-before-create (list customer wallets, match name/metadata) → never a blind second create. Credits units stay integer end-to-end: `CreditsMicro` must be cent-aligned (`% 10_000 == 0`, validated); the adapter converts micro → Lago decimal string exactly (`micro/10^6`, e.g. `9_900_000 → "9.9"`, `rate_amount "1"`) and back (`balance cents × 10_4`); no binary float.
- **Wallet-cap-aware design:** monthly cadence occupies ≤ 2 transient wallet slots per customer (current + expired-awaiting-termination ≤ ~65 min — E1/source-notes `*:45` clock). A `wallet_limit_reached` (422) answer on monthly grant is a bounded retry (backoff across the hourly termination tick, ≤ ~70 s inside one command), then a closed `unreachable`-class indeterminate (the grant replays safely by identity next access). The registry records every batch so future top-up tickets can budget the remaining ≥ 4 slots; T08 performs NO top-up grants.
- **Pre-dispatch expiry rejection (registry obligation):** any balance surfaced by the projection treats a batch whose `expires_at ≤ now` as unavailable (zero available) even while Lago still reports the wallet active — the lazy-termination window never leaks spendable-looking credits into the Billing API. (Consumption-side dispatch enforcement lands with #87/#88; the registry + snapshot overlay anchor it now.)
- **Base Plan seeding via the #79 flow, lazy and idempotent:** `SeedBasePlan` runs inside the ensure chain, not at process start (boot must never block on a Lago outage). It is a no-op once `(base, v1)` is published; otherwise `CreateDraft` + `Publish` through `PlanVersionService` with actor `system:base-plan-seed`. The tier ladder grows additively: `DefaultPriceLadder()` gains the bottom rung `{TierKey: "base", PriceFen: 0}`; `PriceLadder.Validate` and `validateBasePriceTier` gain a narrow zero-clause (zero is a valid price point ONLY for the `base` rung; every other rung stays `> 0`; strict ascending otherwise). `PublishPlanVersionPayload.Validate` moves from `AmountFen > 0` to `AmountFen >= 0` with the six-axis `base_price_tier` check rejecting zero for any non-`base` tier — paid-tier invariants are tightened or preserved, never weakened. Real-stack Task 6 verifies Lago accepts `amount_cents: 0` on `POST /api/v1/plans` (pinned v1.53.0); if the runtime rejects zero, the documented fallback is seeding at `1` fen with a code comment + ledger note (product-visible impact: a 0.01 CNY monthly invoice on a free space) — decided by evidence, not assumption.
- **Subscription form (T02 decision, binding):** the Base Plan subscription is a STANDARD subscription — `POST /api/v1/subscriptions {subscription: {external_customer_id, plan_code, name?, external_id: ExternalSubscriptionID(t)}}` with NO `activation_rules` (never payment-gated). Identity reads pass explicit statuses (`GET /api/v1/subscriptions?external_id=…&status[]=active&status[]=incomplete&status[]=canceled&status[]=terminated…`) because the index defaults to `active` (T02 cross-ticket note). A create answering 422 `value_already_exist`/`subscription_incomplete` resolves by identity re-read + plan-code compare — never a second create; a held subscription on a DIFFERENT plan code is a definitive conflict (`ErrPlatformInvalidResponse`): the upgrade path is a later ticket, and this seam must never silently mint a parallel subscription.
- **Enforcement posture:** quota guards bite only where a projection with limits exists — `hard_limit IS NULL` (blocked-env, unprojected dimension) means unlimited, so older deployments and Lago outages degrade to open, never to lockout (spec: outage keeps view/export/cleanup; a pending projection blocks only what a closed product state explicitly gates). Guards use the existing conditional-UPDATE CAS (`ReserveResource` mechanism) so two concurrent adds at the boundary admit exactly the allowed number; every increment has a compensating decrement on caller failure, and `delta <= 0` always passes (超限状态 cleanup). The projection refresh re-syncs `used` from observed occupancy (member count, `tenants.storage_used`) before applying limits — counters cannot silently drift into permanent over-block. Units: `members` = persons, `storage_gb` counter tracks BYTES with `hard_limit = G × 2^30` (documented in code), `concurrent_tasks` = tasks (exposed only).
- **Migration numbers:** versioned `000180_commercial_tenant_benefits` + sqlite `000101_commercial_tenant_benefits` (next free pair after 000179/000100, confirmed free in this worktree; #81 has no plan in the repo — if it lands first with a different number, renumber mechanically before merge, the migration is self-contained).
- **Secrets:** Lago credentials only from the existing `WEKNORA_COMMERCIAL_PLATFORM_*` env family; nothing committed, logged, or echoed; no secret-shaped literals in tests (fake/stub keys are obviously-fake markers like the T05/T07 stubs). Integration-test inputs only via `LAGO_INTEGRATION_*` env.
- **Old gateway untouched:** `internal/infrastructure/openmeter/**` and its registration are not modified; both seams coexist until #105.
- **Trunk green:** `go build ./...` and the Go test suite pass at every commit. Real-Lago integration is separately tagged (`//go:build lago_integration`) and env-gated; Docker unavailability is recorded as `blocked-env`, never a failure or a fake pass.

## Review Focus

- **Parallel double subscription:** two concurrent ensures (or a retry after response loss) must yield EXACTLY ONE subscription for the tenant's identity — reviewer hunts any path that regenerates `ExternalSubscriptionID`, mints a fresh Key on retry, or treats Lago's `value_already_exist` 422 as a failure instead of resolving by identity re-read + plan-code compare. A held subscription on a different plan code must hard-conflict, not fork a second one.
- **Duplicate monthly grant:** the T03 E3 fact (no wallet idempotency — a replayed grant doubles the balance) makes this the highest-severity hole. Reviewer checks all three layers: registry unique `(tenant_id, period)` under concurrency, adapter read-before-create by deterministic name/metadata, and settle-wait timeout classified as indeterminate (replay resolves; never a blind re-POST). The fake must model balance so the contract proves the total granted micro is exactly one month's amount.
- **Entitlement/provider leakage:** every Billing API response (including error paths and the benefits section) asserted to contain no `weknora-…` external plan/subscription code, no `lago` substring, no URL, no raw error text; the plan/subscription/wallet mapping lives only in the projection/publications tables; closed token sets everywhere (`subscription_state ∈ {active, pending}`, reason tokens as in #78).
- **Quota bypass path:** reviewer walks EVERY member-add and storage-growth entry (including the batch/upload variants — `knowledge_create.go` has three sites) and the member-remove/knowledge-delete decrements; a growth path that skips the CAS, a guard that fails OPEN on a projection with a hard limit, a counter that can go negative, or a refresh that applies limits BEFORE re-syncing occupancy (permanent over-block) are all defects. The concurrency test must prove two racing adds against one remaining slot admit exactly one.
- **Cross-tenant contamination:** every store row, snapshot query and guard is keyed by the authenticated tenant only; identities are DERIVED (`ExternalCustomerID(t)`, `ExternalSubscriptionID(t)`) so no input can name another tenant's objects. Tests prove tenant A's ensure/grant never creates or reads tenant B's subscription/wallet, and A's billing answer never includes B's balances or limits.
- **(Secondary) expired-batch spendability:** an expired-but-lazy-terminated wallet must surface as zero available credits in the projection/API overlay (the ~65-min window), and the seam-freeze creep check: exactly two new kinds + one snapshot kind + additive payload types; the shared contract table still runs identical legs for both adapters.

---

### Task 1: Domain — subscription/credits command payloads, benefits snapshot kind, base tier ladder (table-driven)

**Files:**
- Create: `internal/commercial/subscription_command.go`
- Create: `internal/commercial/subscription_command_test.go`
- Modify (one additive field + narrow zero-clause): `internal/commercial/platform.go` (Snapshot struct only), `internal/commercial/catalog.go` (ladder), `internal/commercial/plan_command.go` (AmountFen bound)

**Interfaces:**
- Produces (all provider-neutral, package `commercial`):

```go
// subscription_command.go — the T08 additive seam surface (#80, ADR-0014
// additive-kind rule). Two command kinds + one snapshot kind; the frozen
// file gains only the Snapshot.Benefits field (the Snapshot.Account precedent).

const CommandKindEnsureSubscription CommandKind = "ensure_subscription"

// ExternalSubscriptionID is THE deterministic WeKnora→authority subscription
// identity (subscription continuity: one identity per tenant across upgrade,
// downgrade and Base-Plan transition). Pure function of tenantID.
func ExternalSubscriptionID(tenantID uint64) string // "weknora-tenant-<id>-sub"

type EnsureSubscriptionPayload struct {
    TenantID                uint64
    ExternalCustomerID      string // must equal ExternalCustomerID(TenantID)
    ExternalSubscriptionID  string // must equal ExternalSubscriptionID(TenantID)
    PlanCode                string // the Base Plan version's deterministic code (seam-internal)
}
// BasePlanKey / base-tier vocabulary
const BasePlanKey = "base"

const CommandKindGrantIncludedCredits CommandKind = "grant_included_credits"
const MonthlyWalletPriority = 1 // earliest-expiry class consumed first (E2: priority ASC)

type GrantIncludedCreditsPayload struct {
    TenantID           uint64
    ExternalCustomerID string // derived-equality enforced
    Period             string // "YYYY-MM", UTC
    CreditsMicro       int64  // > 0 and cent-aligned (CreditsMicro % 10_000 == 0)
    ExpiresAt          time.Time // exclusive period end, > grant time
}
func (p GrantIncludedCreditsPayload) Validate() error
func GrantCreditsCommandKey(externalCustomerID, period string) string // "grant_included_credits:<ext>:<period>"
func EnsureSubscriptionCommandKey(externalSubscriptionID string) string

// Credits unit helpers — exact integer mapping, never float.
func MicroToDecimalString(micro int64) string // 9_900_000 -> "9.9"; requires cent alignment
func CentsToMicro(cents int64) int64          // × 10_4

const SnapshotKindBenefits SnapshotKind = "benefits"

// SubscriptionState closed set (authority truth, NOT the API envelope).
const (
    SubscriptionStateActive  = "active"
    SubscriptionStatePending = "pending" // absent/incomplete/unconfirmed — closed product mapping decides
)

type CreditBatchSnapshot struct {
    Period      string
    BalanceMicro int64
    ExpiresAt   time.Time
}
type BenefitsSnapshot struct {
    TenantID          uint64
    SubscriptionState string // closed set above
    PlanCode          string // seam-internal; the service maps it via commercial_plan_publications
    Features          map[string]bool
    BalanceMicro      int64 // total across active wallets, already expiry-overlaid? NO — raw authority balance; the service overlays registry expiry
    Batches           []CreditBatchSnapshot
    CheckedAt         time.Time
}
// Snapshot gains: Benefits *BenefitsSnapshot (additive, nil unless kind == benefits)
```

- Produces (catalog.go, additive + narrow zero-clause): `DefaultPriceLadder()` gains the bottom rung `{TierKey: "base", PriceFen: 0}`; `PriceLadder.Validate` accepts `PriceFen == 0` ONLY at index 0 and keeps strictly-ascending for the rest; `validateBasePriceTier` accepts `price == 0` iff the plan key is the `base` rung (non-base tiers keep `price > 0` + ladder-point + non-inversion rules unchanged); `plan_command.go` `Validate` moves to `AmountFen >= 0` (zero legality decided by the six-axis check, not structurally).

- [ ] **Step 1: Write failing tests (RED)**

`subscription_command_test.go`, table-driven:
1. `TestEnsureSubscriptionPayloadValidate`: rejects — TenantID 0, mismatched `ExternalCustomerID`, mismatched `ExternalSubscriptionID`, empty PlanCode; accepts a fully-formed base-plan payload.
2. `TestExternalSubscriptionIDDeterministic`: pure, stable form `weknora-tenant-42-sub`; distinct tenants distinct ids.
3. `TestGrantPayloadValidate`: rejects — non-cent-aligned `CreditsMicro` (e.g. `1_234_567`), `<= 0`, malformed Period (`"2026-13"`, `"2026-9"`, `"abc"`), `ExpiresAt` not after now / not matching Period end; accepts a cent-aligned monthly payload.
4. `TestCreditsUnitHelpers`: `MicroToDecimalString` exact table (`9_900_000 → "9.9"`, `100_000 → "0.1"`, `1_000_000 → "1"`), rejects non-cent-aligned input (error, not rounding); `CentsToMicro` exact; round-trip property for the cent-aligned set.
5. `TestCommandKeys`: forms + determinism.
6. `TestBaseTierLadder` (catalog): `DefaultPriceLadder` contains `base@0` at index 0 followed by strictly ascending positive rungs; a custom ladder with zero at a non-zero index, or two zeros, is rejected; paid tiers priced 0 fail `ValidateForPublishValidated` with `base_price_tier`; `base` priced 0 passes all six axes (CNY, closed features/limits, positive credits); a `base` version priced 1900 (a ladder point but not its rung) fails.
7. `TestPublishPayloadZeroAmount`: payload `AmountFen 0` structurally valid; the six-axis check is the gate (covered in 6).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/commercial/ -run 'TestEnsureSubscription|TestGrantPayload|TestCreditsUnit|TestCommandKeys|TestBaseTier|TestPublishPayloadZero' -v`

Expected: compile failure — `subscription_command.go` does not exist.

- [ ] **Step 3: Implement**

Write `subscription_command.go` exactly as the interface (doc comments citing the T03 verdict obligations and ADR-0014). Append the `Snapshot.Benefits` field with the additive comment (the `Account` precedent text). Ladder + validation zero-clause edits stay surgical; T07 tests must stay green unchanged except where the ladder membership assertion legitimately enumerates rungs (update the enumeration, not the rule).

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/commercial/ -v`

Expected: new tests pass; existing catalog/plan_command/platform suites green (additive only).

- [ ] **Step 5: Commit**

`feat(commercial): ensure_subscription and grant_included_credits seam surface with benefits snapshot kind (#80)`

### Task 2: Local persistence — benefits projection + credit batch registry (migration 000180/000101)

**Files:**
- Create: `migrations/versioned/000180_commercial_tenant_benefits.up.sql` / `.down.sql`
- Create: `migrations/sqlite/000101_commercial_tenant_benefits.up.sql` / `.down.sql`
- Create: `internal/application/repository/commercial/benefits.go`
- Create: `internal/application/repository/commercial/benefits_test.go`

**Interfaces:**
- Consumes: Task 1 domain types; the `EnsureSchema` portable-DDL precedent (`planversion.go`), the 000179 comment style.
- Produces:

```go
// BenefitsRow is the rebuildable per-tenant billing projection: the plan the
// space is on, its closed entitlement/quota snapshot, and the freshness time.
// External identities (plan_code, external_subscription_id) are seam-internal
// columns — the Billing API maps through commercial_plan_publications.
type BenefitsRow struct {
    TenantID               uint64 `gorm:"primaryKey;column:tenant_id"`
    ExternalCustomerID     string `gorm:"column:external_customer_id;not null"`
    ExternalSubscriptionID string `gorm:"column:external_subscription_id;not null"`
    SubscriptionState      string `gorm:"column:subscription_state;not null"` // active|pending
    PlanCode               string `gorm:"column:plan_code;not null;default ''"`
    PlanKey                string `gorm:"column:plan_key;not null;default ''"`   // resolved via publications at write time
    PlanVersion            int64  `gorm:"column:plan_version;not null;default 0"`
    FeaturesJSON           string `gorm:"column:features_json;not null;default '{}'"`
    LimitsJSON             string `gorm:"column:limits_json;not null;default '{}'"`
    CreditsBalanceMicro    int64  `gorm:"column:credits_balance_micro;not null;default 0"`
    ProjectedAt            time.Time `gorm:"column:projected_at;not null"`
}
func (BenefitsRow) TableName() string { return "commercial_tenant_benefits" }

// CreditBatchRow is the coordinator's monthly batch registry (T03 verdict
// obligations: pre-dispatch expiry rejection, grant idempotency, wallet-cap
// accounting). One row per (tenant, period) — the unique constraint IS the
// first idempotency layer.
type CreditBatchRow struct {
    ID             int64  `gorm:"primaryKey;autoIncrement"`
    TenantID       uint64 `gorm:"column:tenant_id;uniqueIndex:uq_credit_batch_tenant_period"`
    Period         string `gorm:"column:period;uniqueIndex:uq_credit_batch_tenant_period"`
    CommandKey     string `gorm:"column:command_key;not null"` // grant_included_credits:<ext>:<period>
    WalletRef      string `gorm:"column:wallet_ref;not null;default ''"` // seam-internal wallet lago_id once known
    GrantedMicro   int64  `gorm:"column:granted_micro;not null"`
    ExpiresAt      time.Time `gorm:"column:expires_at;not null"`
    State          string `gorm:"column:state;not null"` // granted|expired (state is advisory; expiry is decided by ExpiresAt)
    CreatedAt      time.Time `gorm:"column:created_at;not null"`
}
func (CreditBatchRow) TableName() string { return "commercial_credit_batches" }

type BenefitsStore struct{ db *gorm.DB }
func NewBenefitsStore(db *gorm.DB) *BenefitsStore
func (s *BenefitsStore) EnsureSchema(ctx context.Context) error // portable CREATE TABLE IF NOT EXISTS for both tables
func (s *BenefitsStore) UpsertProjection(ctx context.Context, row BenefitsRow) error
func (s *BenefitsStore) GetProjection(ctx context.Context, tenantID uint64) (BenefitsRow, error) // ErrProjectionNotFound
// EnsureBatch is the concurrency-safe single-grant gate: inserts (tenant,
// period) once; on the unique race it returns the EXISTING row so the caller
// replays instead of re-granting.
func (s *BenefitsStore) EnsureBatch(ctx context.Context, row CreditBatchRow) (CreditBatchRow, bool /*created*/, error)
func (s *BenefitsStore) GetBatch(ctx context.Context, tenantID uint64, period string) (CreditBatchRow, error)
func (s *BenefitsStore) MarkBatchWallet(ctx context.Context, id int64, walletRef string) error
func (s *BenefitsStore) ActiveBatches(ctx context.Context, tenantID uint64, now time.Time) ([]CreditBatchRow, error) // expires_at > now — the pre-dispatch registry read
func (s *BenefitsStore) SetBatchExpired(ctx context.Context, id int64) error
```

- The versioned (PostgreSQL) migration creates BOTH tables (`TIMESTAMPTZ`, comments in the 000179 style; `commercial_credit_batches` notes the T03 wallet-cap ledger role); the sqlite migration mirrors with portable types. Down drops both tables.

- [ ] **Step 1: Write failing repository tests (RED)**

Sqlite in-memory gorm (`planversion_test.go` harness; `EnsureSchema` bootstraps). Cases:
1. `TestProjectionUpsertRoundTrip`: insert → read back byte-equal; upsert overwrites with fresh `ProjectedAt` (one row per tenant).
2. `TestEnsureBatchExactlyOnce`: first `EnsureBatch` creates; a second with the same `(tenant, period)` returns the EXISTING row (`created=false`) and does NOT insert; different periods coexist.
3. `TestEnsureBatchConcurrentSingleRow`: two goroutines racing `EnsureBatch` for the same `(tenant, period)` → exactly one row (unique constraint path exercised, not serialized away).
4. `TestActiveBatchesExpiryBoundary`: batches at `expires_at = now`, `now+1h`, `now-1min` → only the future one is active (the pre-dispatch registry semantics; boundary `== now` is expired).
5. `TestMarkBatchWalletIdempotent`: set wallet ref; re-set with the SAME ref is a no-op success; a DIFFERENT ref on a non-empty column is an error (identity must never be silently rewritten).
6. `TestSchemaBootstrapsWithoutMigration`: fresh DB + `EnsureSchema` → both tables usable (dev/blocked-env posture).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/repository/commercial/ -run TestBenefits -v`

Expected: compile failure — the file does not exist.

- [ ] **Step 3: Implement**

Four migration files (000180/000101, paired `.down.sql`) + `benefits.go`. `EnsureBatch` uses `INSERT … ON CONFLICT DO NOTHING`-equivalent portable GORM (create; on duplicate-key error read back) so the DB decides under concurrency. No direct `db.Exec` outside the store.

- [ ] **Step 4: Verify GREEN + migration sanity**

Run: `go test ./internal/application/repository/commercial/ -v && bash -c 'ls migrations/versioned/000180* migrations/sqlite/000101*'`

Expected: repository suite green (existing suites untouched); four migration files exist with paired downs.

- [ ] **Step 5: Commit**

`feat(commercial): tenant benefits projection and credit batch registry (#80)`

### Task 3: Seam commands + benefits snapshot — fake and Lago adapters, shared contract legs

**Files:**
- Modify: `internal/infrastructure/commercialplatform/fake.go`
- Modify: `internal/infrastructure/commercialplatform/lago.go`
- Modify: `internal/infrastructure/commercialplatform/contract_test.go`
- Create: `internal/infrastructure/commercialplatform/lago_subscription_test.go`

**Interfaces:**
- Consumes: Task 1 kinds/payloads; frozen `SubmitCommand`/`ReadSnapshot` signatures.
- Produces:
  - **Fake** grows a real in-memory authority: `subscriptions map[extSubID]fakeSubscription{ExternalID, ExternalCustomerID, PlanCode, State}` (upsert by identity — never a second entry) and `wallets []fakeWallet{Name, Customer, BalanceCents, ExpiresAt, Terminated}` with clock injection (`SetNow(func() time.Time)`, default real) so expiry tests are deterministic. `ensure_subscription`: payload guard → upsert (an existing entry with a DIFFERENT PlanCode answers `ErrPlatformInvalidResponse` conflict — the fake models the no-parallel-subscription rule); replay answers the same identity. `grant_included_credits`: find wallet by `(customer, deterministic name)` → exists = replay (receipt, no balance change); absent = create with `granted_credits` settled after a simulated one-tick lag (the a1 settle-wait fact) and `expiration_at = payload.ExpiresAt`. `ReadSnapshot(benefits)`: derives honestly from the stores — subscription state/plan code, feature map passed through a primed `SetBasePlanFeatures`, wallet balances summed with ALREADY-TERMINATED wallets excluded but not-yet-terminated expired ones included (the raw authority truth — the coordinator overlays expiry). Test hooks: `Wallets()`, `Subscriptions()`, `FailSubmitsWith` now also applies to the new kinds (persisted-but-response-lost: state applies, injected error answers).
  - **Lago adapter**: `ensureSubscription` = configured → payload guard → identity read with explicit statuses (`GET /api/v1/subscriptions?external_id=…` + `status[]` list — T02 fact: index defaults to active) → found & plan code equal = receipt; found & different plan code = `ErrPlatformInvalidResponse` (subscription conflict); not found → `POST /api/v1/subscriptions` (external_customer_id, plan_code, external_id, NO activation_rules) → 2xx receipt echoing `ExternalSubscriptionID`; 422 → identity re-read (found+equal = replay receipt; still absent = `ErrPlatformInvalidResponse`); other 4xx invalid-response; 5xx/transport unreachable. `grantIncludedCredits` = configured → payload guard → read-before-create: `GET /api/v1/customers/{ext}/wallets` (paged) matching deterministic wallet name (fallback metadata) → found = replay receipt; absent → `POST /api/v1/wallets` `{wallet: {external_customer_id, name, currency: "CNY", granted_credits: MicroToDecimalString(micro), rate_amount: "1", expiration_at, metadata: {weknora_tenant, weknora_period}}}` (2xx → capture `lago_id`) → **settle-wait**: poll `GET /api/v1/wallets/{lago_id}` until `balance_cents ≥ granted cents` (bounded ~10 s / 500 ms ticks — the a1 after-commit settlement fact); `wallet_limit_reached` 422 → bounded retry across the termination tick (≤ ~70 s total), then `ErrPlatformUnreachable` (indeterminate — replay resolves by identity); other outcomes per the shared mapping. `ReadSnapshot(benefits)` = subscription identity read (statuses explicit) + entitlements read (`GET /api/v1/customers/{ext}/entitlements`, features keyed by code — T02 fact) + wallets read (balances summed, `status=active` filter) → `BenefitsSnapshot{raw authority truth}`. All new provider vocabulary (`wallets`, `entitlements`, `granted_credits`, `status[]`) lives ONLY in `lago.go`; receipts echo WeKnora-derived identities (`ExternalID` = ext subscription id / deterministic wallet name), never a provider `lago_id` — the wallet `lago_id` returns inside the seam via a typed `GrantReceipt`… NO: the frozen `CommandReceipt` is unchanged — the wallet `lago_id` is persisted by the SERVICE from the snapshot/batch lookup (the registry's `WalletRef` fills on the post-grant projection refresh), keeping the receipt shape frozen.
  - **Shared contract** (both adapters, identical legs — `runSubscriptionContract`, `runGrantContract`, `runBenefitsContract` added to the shared table): ensure_subscription is idempotent by identity (replay = same receipt, `Subscriptions()`-equivalent count stays 1); grant replays never double the balance (balance observable on both adapters — fake `Wallets()`, stub wallet GET); same-Key-different-content = `ErrPlatformInvalidResponse`; benefits snapshot answers the closed truth for the ensured tenant and honest absence (`pending` subscription state / zero balance) for an untouched one; unknown kinds still fail closed.

- [ ] **Step 1: Write failing tests (RED)**

`lago_subscription_test.go` — httptest stubs (`newSubscriptionsStub`, `newWalletsStub`, the `newPlanStub` pattern) recording method+path+query+body: (1) ensure happy path — request carries `external_id`, `plan_code`, `external_customer_id` and NO `activation_rules` key (assert body); receipt `ExternalID == ext sub id`; (2) replay — index shows the subscription → ZERO POSTs (count asserted); (3) 422 `value_already_exist` → re-read equal = receipt, still-absent = invalid-response; (4) different-plan-code conflict → `ErrPlatformInvalidResponse`; (5) index called with explicit `status[]` query params (the T02 default-active trap); (6) grant happy path — wallets list empty → wallet POST carries `granted_credits` decimal string, `rate_amount "1"`, `expiration_at`, metadata; settle-poll GET answered with the credited balance → receipt; (7) grant replay — wallets list shows the deterministic name → NO second POST; (8) `wallet_limit_reached` then success after retry window (stub scripted) → eventual receipt; exhausted → unreachable; (9) benefits snapshot — stubs answer active subscription + entitlements map + two wallets (one terminated) → snapshot carries plan code, closed state, feature map, summed active balance, per-batch entries; (10) no `lago` URL/path substring in any error string; authorization header present, credential absent from bodies/logs.
`contract_test.go`: register the three new shared legs for fake + stub-backed Lago (the `runPublishContract` extension pattern), including the untouched-tenant honesty leg.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: failures — both adapters answer `ErrPlatformUnsupported` for the new kinds.

- [ ] **Step 3: Implement**

Extend both adapters per the interfaces; the fake stays deterministic (mutex already present). Keep the wallet `lago_id` out of receipts; `do()` reuse for all new calls; timeouts via the existing ctx pattern (`publishRequestTimeout` precedent — add `subscriptionRequestTimeout` ≈ 15 s and `walletSettleTimeout` ≈ 10 s constants).

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/infrastructure/commercialplatform/ ./internal/commercial/ -v`

Expected: the shared contract table (readiness + account + publish + subscription + grant + benefits legs) passes for both adapters.

- [ ] **Step 5: Commit**

`feat(commercialplatform): ensure_subscription, grant_included_credits and benefits snapshot on both adapters (#80)`

### Task 4: BenefitsService — the lazy ensure chain, monthly issuance, projection refresh

**Files:**
- Create: `internal/application/service/commercial/benefits.go`
- Create: `internal/application/service/commercial/benefits_test.go`

**Interfaces:**
- Consumes: `BillingAccountService` (#78), `PlanVersionService` (#79), `BenefitsStore` (Task 2), the seam kinds (Tasks 1/3).
- Produces:

```go
// BenefitsService owns the T08 lazy chain, run on billing access (the #78
// documented trigger; no startup dependency, no background clock — credits
// become spendable only with #87/#88 admission, which can add the clock):
//
//   EnsureBenefits
//     ├─ SeedBasePlan            (idempotent #79 publish of (base, v1); no-op once published)
//     ├─ EnsureBillingAccount    (#78 — the SAME customer ensure; linked row answers fast)
//     ├─ ensure_subscription     (seam; idempotent by ExternalSubscriptionID)
//     ├─ EnsureMonthlyCredits    (registry-gated grant for the current UTC period)
//     └─ RefreshProjection       (benefits snapshot → plan identity via publications → limits/features → counters)
type BenefitsService struct {
    accounts  *BillingAccountService
    plans     *PlanVersionService
    store     *repocommercial.BenefitsStore
    platform  commercial.CommercialPlatform // nil legal: projection stays pending, nothing fabricated
    catalogs  *repocommercial.CatalogStore
    now       func() time.Time // injectable clock
}
func NewBenefitsService(db *gorm.DB, accounts *BillingAccountService, plans *PlanVersionService, platform commercial.CommercialPlatform) (*BenefitsService, error)

// BasePlanSeed constants: PlanKey "base", Version 1, the built-in definition
// (features {api_access:true, advanced_models:false, priority_support:false},
// limits {members:5, storage_gb:10, concurrent_tasks:2}, IncludedCreditsMicro
// cent-aligned first-slice constant fixed with the tests) — values are
// first-slice constants; externalizing them is a later product ticket.
const (
    BasePlanSeedVersion            int64 = 1
    BasePlanSeedIncludedCreditsMicro int64 = 1_000_000 // 1 credit/month, cent-aligned
    BasePlanSeedActor                    = "system:base-plan-seed"
)

func (s *BenefitsService) SeedBasePlan(ctx context.Context) (seeded bool, err error)
func (s *BenefitsService) EnsureBenefits(ctx context.Context, tenantID uint64, displayName, actor string) (BenefitsStatus, error)
// EnsureMonthlyCredits: period = UTC "YYYY-MM" of now(); EnsureBatch gate →
// skip if the row exists (created=false); else submit grant_included_credits
// under GrantCreditsCommandKey → MarkBatchWallet on the later refresh →
// registry row records GrantedMicro/ExpiresAt.
func (s *BenefitsService) EnsureMonthlyCredits(ctx context.Context, tenantID uint64) (CreditBatchRow, error)
// RefreshProjection: ReadSnapshot(benefits) → resolve PlanCode→(plan_key,
// version) via commercial_plan_publications (missing publication = closed
// pending reason, projection keeps last-known plan) → UpsertProjection →
// ApplyQuotas (below).
func (s *BenefitsService) RefreshProjection(ctx context.Context, tenantID uint64) (BenefitsRow, error)
// ApplyQuotas: re-sync counters.used from observed occupancy (member count;
// tenants.storage_used) THEN set hard_limit per projected limits (members→N;
// storage_gb→G×2^30 bytes; concurrent_tasks→C; absent dimension = NULL).
// Ordering is binding: occupancy first, limits second — 超限状态 must reflect
// real overage, never counter drift.
func (s *BenefitsService) ApplyQuotas(ctx context.Context, tenantID uint64, limits map[string]int64) error

// BenefitsStatus is the closed product answer served by the Billing API:
type BenefitsStatus struct {
    Account  BillingAccountStatus // the #78 envelope, unchanged
    Plan     *PlanView            // nil while pending — never a fabricated plan
    Credits  *CreditsView         // included balance (registry-expiry-overlaid), batch breakdown
    Reason   string               // closed tokens (""|unconfigured|unreachable|invalid_response|unsupported)
}
type PlanView struct { Key string; Version int64; State string; Features map[string]bool; Limits map[string]int64 }
type CreditsView struct { BalanceMicro int64; Batches []BatchView } // BatchView{Period, BalanceMicro, ExpiresAt}; expired batches report 0
```

- Failure posture (the #78 doctrine): every platform failure is a STATE (pending + closed reason), never a caller error; the chain is resumable at every step (linked account row / published seed / subscription identity / registry row are all idempotent checkpoints). Only DB errors return errors.

- [ ] **Step 1: Write failing service tests (RED)**

Sqlite harness + the fake adapter (Task 3) as the authority. Cases:
1. `TestEnsureBenefitsHappyChain`: fresh tenant → account linked, base plan seeded (exactly one `(base,1)` publication), subscription created (fake holds exactly one), current-period batch granted (fake wallet count 1, balance == one month's micro), projection row written (plan key `base`, features/limits from the definition), quotas applied (counter `members.hard_limit == 5`, `storage_gb.hard_limit == 10×2^30`).
2. `TestEnsureBenefitsIdempotentReplay`: run `EnsureBenefits` three times → fake subscription count stays 1, wallet count stays 1 (ONE balance, not tripled — the E3 anti-pattern proven absent), one batch row, one publication; the third run answers the same plan view.
3. `TestEnsureBenefitsConcurrentExactlyOnce`: 8 goroutines racing `EnsureBenefits` on one tenant → exactly 1 subscription, 1 wallet, 1 batch row, 1 publication; total granted micro == exactly one month (the spec's "prove the total granted amount" concurrency rule).
4. `TestMonthlyGrantNewPeriod`: advance the injected clock across a month boundary → next ensure creates a SECOND wallet for the new period (old batch stays in registry, surfaced as expired/zero in `CreditsView` once past `ExpiresAt` — the lazy-termination overlay), fake wallet count 2 (the ≤2-transient-slots budget), balance view = new period only.
5. `TestExpiredBatchSurfacesZero`: clock past `expires_at` while the fake wallet is still "active" (lazy termination window simulated) → `CreditsView.BalanceMicro` excludes it (registry overlay wins over raw authority balance).
6. `TestGrantWalletCapRetry`: fake scripted `wallet_limit_reached` for the first attempts → service surfaces closed `unreachable` reason (state pending), NO second batch row minted; clearing the block and re-ensuring completes the grant by identity (no duplicate).
7. `TestPlatformFailureIsPendingState`: `FailSubmitsWith` at each chain stage → `BenefitsStatus{Account pending|Reason closed-token}`, no fabricated plan/credits, no error; clearing the fault and re-running completes the chain (resumable).
8. `TestNilPlatformFailsClosed`: nil seam → account row pending/unconfigured posture, projection absent, zero panics.
9. `TestSeedBasePlanIdempotent`: seed twice → one publication, second is a no-op; a pre-published `(base,1)` (operator-published variant definition) is respected, NOT re-published (publish-once immutability).
10. `TestApplyQuotasResyncBeforeLimit`: seed counters `members.used = 7` (drift), projection limits `members: 5` → refresh sets `used = actual member count` first, then `hard_limit 5` → 超限状态 derives from real occupancy; a tenant with 6 real members and limit 5 is over-limit (block add) but removal still passes.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/service/commercial/ -run TestBenefits -v`

Expected: compile failure.

- [ ] **Step 3: Implement**

Follow the chain exactly; persistence only through `BenefitsStore` transactional units + the existing #78/#79 services (no direct db handles beyond store construction, the `PlanVersionService` precedent). `EnsureMonthlyCredits` computes the period from the injected clock (UTC) and derives `ExpiresAt` as the exclusive period end. Registry `EnsureBatch` is the concurrency gate; the grant submits ONLY when it created the row.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/application/service/commercial/ -v`

Expected: new suite green; #78/#79 service suites untouched and green.

- [ ] **Step 5: Commit**

`feat(commercial): benefits service with lazy base-plan chain and coordinator-owned monthly credits (#80)`

### Task 5: Billing API extension + enforcement wiring + 超限 semantics + container

**Files:**
- Modify: `internal/handler/commercial.go` (benefits section on AccountStatus; guard wiring points)
- Modify: `internal/handler/tenant_member.go` (AddMember/RemoveMember guard)
- Modify: `internal/application/service/knowledge_create.go` (three storage sites — injected optional `ResourceQuotaGuard`)
- Modify: `internal/router/routes_commercial.go` (no new route needed — additive response fields; only if the guard needs a health surface)
- Modify: `internal/container/container.go` (BenefitsService provide/invoke, guard injections)
- Create: `internal/router/commercial_benefits_route_test.go`
- Create: `internal/handler/commercial_benefits_test.go`

**Interfaces:**
- Consumes: `BenefitsService` (Task 4); `ReserveResource` CAS + `commercial_resource_counters` (existing); `types.NewStorageQuotaExceededError` conventions.
- Produces:
  - `GET /api/v1/commercial/account` grows an ADDITIVE `benefits` object (absent while pending — no breaking change for existing clients): `benefits: {plan: {key, version, state}, features: {…}, limits: {…}, credits: {balance_micro, batches: [{period, balance_micro, expires_at}]}}` — closed vocabulary; `state ∈ {active, pending}`; amounts as digit strings where the wire convention requires (the `quoteWire` precedent). `AccountStatus` now calls `EnsureBenefits` (the lazy trigger stays this GET). Nil service → the #78 fail-closed envelope PLUS `benefits` absent (documented, never 500 for wiring gaps).
  - `ResourceQuotaGuard` (small interface, defined in `internal/application/service` or `internal/commercial`):

```go
// ResourceQuotaGuard is the commercial growth gate. nil (not wired) means no
// commercial limits exist — the call passes (blocked-env posture). A guard
// answers ErrQuotaExceeded-class errors ONLY for growth that a projected
// hard_limit refuses; delta <= 0 always passes (超限状态 cleanup).
type ResourceQuotaGuard interface {
    ReserveGrowth(ctx context.Context, tenantID uint64, dimension string, delta int64) (release func(), err error)
}
```

  - `TenantMemberHandler.AddMember`: before the member insert, `ReserveGrowth(tenant, "members", +1)`; on failure → the existing 4xx error shape with closed token `quota_exceeded`; on insert failure → the release/compensating decrement runs. `RemoveMember`: best-effort `ReserveGrowth(tenant, "members", -1)` (always allowed; keeps the counter honest).
  - `knowledge_create.go` three sites: alongside the existing local `StorageQuota` check, `ReserveGrowth(tenant, "storage_gb", +fileSizeBytes)`; creation failure releases; the knowledge delete/cleanup path decrements best-effort. Read/export paths untouched (they never call the guard — the 超限状态 read/export guarantee is by construction and pinned by test).
  - Container: `Provide(commercialsvc.NewBenefitsService)` + `Invoke` wiring `SetBenefitsService` on the commercial handler AND constructing the guard from the service; inject the guard into the member handler and knowledge service ONLY when the commercial stack is wired (nil otherwise — the fail-open degraded posture), next to the T06/T07 blocks (~line 787+), away from the pre-craft Invoke-ordering trap.

- [ ] **Step 1: Write failing tests (RED)**

`commercial_benefits_route_test.go` (gin + `RegisterCommercialRoutes` + injected auth, the T07 harness) and `commercial_benefits_test.go`:
1. `TestAccountBenefitsSection`: tenant with a fake-backed projection → `data.benefits.plan.key == "base"`, features/limits echoed, `credits.balance_micro == one month`, batches listed with period+expires_at; a pending tenant → `benefits` absent (or `state:"pending"` — assert the chosen closed shape), reason token present.
2. `TestAccountBenefitsProviderNeutrality`: fake platform whose errors carry marker text + marker plan/wallet ids on the seam → force unconfigured/unreachable/conflict → response contains no `weknora-…-v`/`weknora-tenant-…-sub` external identities, no `lago` substring, no marker, no URL, no raw error (the T05/T07 leak-test pattern).
3. `TestMemberQuotaOverLimitBlocksAddOnly`: project limits `members: 2` with 2 real members → `AddMember` → 4xx `quota_exceeded`; `ListMembers`, member update, `RemoveMember` (and re-add after removal) still succeed (超限状态: block-add, keep read/clean).
4. `TestStorageQuotaBlocksCreateKeepsReadExportClean`: limits `storage_gb: 1`, occupancy at limit → knowledge create with bytes → quota error; existing knowledge read/export/delete paths succeed (they must not call the guard — assert via a counting guard); delete decrements (subsequent create passes).
5. `TestConcurrentAddsAtBoundary`: limit with exactly 1 slot left, 4 parallel `AddMember` → exactly 1 succeeds, 3 quota errors, member count == limit (the CAS atomicity proof; SQLite single-writer harness or Postgres-gated like `account_pg_test.go`).
6. `TestGuardNotWiredFailsOpen`: nil guard → adds/creates pass (older deployments unaffected).
7. `TestCrossTenantIsolation`: tenants A and B both ensured; A's account answer contains only A's balances/limits; a request authenticated as B never observes A's plan/credits (router-level assertion); guard reserves on B's counters only (counter rows keyed per tenant asserted).
8. `TestTenantIsolationOnSeam` (service-level, if not already in Task 4): A's ensure creates exactly one subscription/wallet for A's derived identity; B's fake store shows nothing new.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/router/ ./internal/handler/ -run 'TestAccountBenefits|TestMemberQuota|TestStorageQuota|TestConcurrentAdds|TestGuardNotWired|TestCrossTenant' -v`

Expected: handlers/wiring do not exist.

- [ ] **Step 3: Implement**

Handlers follow the closed-envelope discipline; `err.Error()` never echoed; the guard implementation wraps the counter CAS (the `ReserveResource` conditional UPDATE moved behind the interface or reused via a small store method — keep the SQL in ONE place). Container wiring additive-only.

- [ ] **Step 4: Verify GREEN + regression**

Run: `go test ./internal/router/ ./internal/handler/ ./internal/container/ ./internal/application/... ./internal/commercial/ ./internal/infrastructure/... -v && go build ./...`

Expected: all green; #78/#79 endpoint tests, member/knowledge legacy tests and the openmeter suite untouched.

- [ ] **Step 5: Commit**

`feat(billing): base-plan benefits on the account endpoint and atomic quota enforcement at growth paths (#80)`

### Task 6: Real-stack integration evidence + final verification

**Files:**
- Create: `internal/infrastructure/commercialplatform/lago_benefits_integration_test.go` (`//go:build lago_integration`)
- Create: `deploy/lago/evidence/t08-run.txt` (operator timeline, if the stack runs)
- Create: `docs/migrations/lago/t08-base-plan/README.md` (evidence index, the t02–t07 convention)

**Interfaces:**
- Consumes: Tasks 1–5; the #73 operator lifecycle `./deploy/lago/lago.sh init|up|status|down`; the ticket-isolated stack env.
- Produces: real pinned-Lago `v1.53.0` proof for all four acceptance criteria, or honest `blocked-env`.

- [ ] **Step 1: Write the tagged integration test (RED without env, GREEN with stack)**

`lago_benefits_integration_test.go`, `//go:build lago_integration`, env-gated on `LAGO_INTEGRATION_BASE_URL` + `LAGO_INTEGRATION_API_KEY` (skip → `blocked-env` log). uuid-prefixed tenants (the lab isolation convention). Phases in ONE test:
1. **Zero-price plan acceptance probe:** `POST /api/v1/plans` with `amount_cents: 0` (payload otherwise per the T07 contract) → record accept/reject. If rejected: the fallback (seed at `1` fen) activates and the phase re-runs — evidence decides, per Global Constraints.
2. **Full chain for tenant A** through the real adapter + `BenefitsService`: seed → customer → subscription (assert created WITHOUT activation rules and reaches `active`) → monthly grant (wallet created, settle-wait completes, balance == one month) → benefits snapshot (plan code, entitlements incl. base features, balance) → projection quotas applied.
3. **Idempotent replay:** a second full `EnsureBenefits` for A → `GET /api/v1/subscriptions?external_id=…` (explicit statuses) shows EXACTLY ONE subscription; the customer's wallet list shows EXACTLY ONE wallet for the period; balance unchanged (no doubled grant — the E3 anti-pattern disproven on the real runtime).
4. **Cross-tenant isolation:** run the chain for tenant B; A's snapshot/balance unchanged; B's identities are B-derived; the wallet lists do not cross.
5. **Wallet-slot measurement:** record A's active wallet count (expect 1) and, with a synthetic short-TTL second wallet (minutes-scale `expiration_at`, the lab technique), observe the lazy-termination delay after expiry (bounded observation ~2 min; the ~65 min ceiling is cited from E1, not re-measured) — evidence for the ≤2-transient-slots budget and the registry overlay decision.
6. **Quota end-to-end (service level against the real DB):** apply `members: N` limits with N at the boundary → `ReserveGrowth` admits exactly N, refuses N+1, allows −1 (超限状态) — the CAS against real PostgreSQL (or honest sqlite-only note if PG is not wired in the lab env).

- [ ] **Step 2: Verify the tag gates it**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: the integration test does not run (no tag); the normal suite stays Docker-free and green.

- [ ] **Step 3: Run the isolated stack and collect evidence**

```
COMPOSE_PROJECT_NAME=weknora-lago-80 LAGO_API_PORT=48903 LAGO_FRONT_PORT=48904 ./deploy/lago/lago.sh init
COMPOSE_PROJECT_NAME=weknora-lago-80 LAGO_API_PORT=48903 LAGO_FRONT_PORT=48904 ./deploy/lago/lago.sh up
COMPOSE_PROJECT_NAME=weknora-lago-80 LAGO_API_PORT=48903 LAGO_FRONT_PORT=48904 ./deploy/lago/lago.sh status --json
LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48903 LAGO_INTEGRATION_API_KEY=… go test -tags lago_integration ./internal/infrastructure/commercialplatform/ -run TestLagoBasePlanIntegration -v
COMPOSE_PROJECT_NAME=weknora-lago-80 ./deploy/lago/lago.sh down
```

Expected: all phases pass against `v1.53.0` (check `deploy/lago/images.lock.json`); phase 1's zero-price verdict recorded either way. Record the timeline in `deploy/lago/evidence/t08-run.txt` and the artifact index in `docs/migrations/lago/t08-base-plan/README.md`. If Docker or an operator key is unavailable: record `blocked-env` (never a fake pass) and rely on the stub-backed contract evidence; state this honestly in the ledger.

- [ ] **Step 4: Final verification sweep**

Run: `go build ./... && go test ./...` and `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe -v`

Expected: trunk green — seam, adapters, repository, service, router, handler suites plus all pre-existing commercial/openmeter suites.

- [ ] **Step 5: Commit, ledger, ticket**

`docs(lago): T08 base plan real-stack evidence (#80)`; write the SDD ledger (`docs/plans/ledgers/lago-80.md`, per the waves convention); tick the ticket's four acceptance boxes with pointers:

| AC | Proof |
|---|---|
| 重复初始化不创建并行 Subscription 或重复额度 | Task 3 contract replay legs (fake + stub) + Task 4 `TestEnsureBenefitsIdempotentReplay`/`TestEnsureBenefitsConcurrentExactlyOnce` + Task 6 phase 3 (exactly one subscription, exactly one period wallet, unchanged balance) |
| Base Plan Entitlement 决定可用功能和资源上限 | Task 1 base-tier ladder/validation + Task 4 projection (`TestEnsureBenefitsHappyChain` asserts features/limits land) + Task 5 benefits section test + Task 6 phase 2 (real entitlements snapshot) |
| 达到上限后新增被阻止，查看、导出与清理仍可用 | Task 5 `TestMemberQuotaOverLimitBlocksAddOnly` + `TestStorageQuotaBlocksCreateKeepsReadExportClean` + `TestConcurrentAddsAtBoundary` + Task 6 phase 6 |
| 空间与其他 Tenant 的额度和配额完全隔离 | Task 4 derived-identity chain + Task 5 `TestCrossTenantIsolation` (router + counters) + Task 6 phase 4 (real A/B isolation) |

## Plan Self-Review

- **Spec coverage / acceptance mapping:** every ticket AC maps to at least one unit/contract test AND one real-stack phase (table above). Spec decisions implemented verbatim: subscription continuity via ONE deterministic external subscription identity; Lago Entitlement defines features/quotas while WeKnora enforces occupancy atomically (CAS) and preserves view/export/cleanup when blocked (guards never sit on read/export/clean paths — pinned by test); included Credits monthly non-carryover (short-TTL wallet per period, `expiration_at` = period end); expiry-to-Base-Plan data preservation (over-limit keeps existing resources — removal/decrement always passes); per-space isolation (derived identities, per-tenant registry/counters/projection, router-level A/B test). User stories 1/14/15/29/30 covered; story 15 respected structurally (top-ups are a later ticket and the monthly cadence leaves ≥ 4 wallet slots).
- **Verdict fidelity (#75/T03):** the three a1 coordinator obligations are each anchored: (1) pre-dispatch expiry rejection → registry `ActiveBatches`/`expires_at` + the projection's expiry overlay (Task 2/4/5, tested with the lazy-termination window simulated); (2) settle-wait → bounded balance poll in the adapter (Task 3, stub-tested); (3) one short-TTL wallet per monthly batch → the chosen issuance design, justified against the rolling-wallet alternative (non-atomic month rollover, included/top-up entanglement in one wallet, no clock-terminated guarantee) and the 6-wallet cap (≤2 transient monthly slots; `wallet_limit_reached` handled with bounded retry then indeterminate-replay). The E3 no-idempotency fact drives the three-layer grant identity (registry unique → read-before-create by deterministic name/metadata → never blind re-POST), and the b consumption-order fact is pre-encoded via `MonthlyWalletPriority` for the later admission tickets. The a2 twelve-month BLOCKED escalation is explicitly NOT touched (design-phase decision, cited not solved).
- **Seam freeze respected:** exactly two new kind constants + one snapshot kind + typed payloads in a NEW file; `platform.go` gains only the additive `Snapshot.Benefits` field (the #78 `Snapshot.Account` precedent); three method signatures untouched; no per-object wrappers; receipts echo WeKnora-derived identities only (the wallet `lago_id` deliberately stays OUT of `CommandReceipt`, reaching the registry via the snapshot refresh). ADR-0014 needs no amendment.
- **TDD:** every task is RED (failing behavioral tests first) → verify-RED command → implement → verify-GREEN command → focused commit. Tests assert business invariants (exactly-one subscription, exactly-one-month granted total under 8-way concurrency, expired batches surface zero, exactly-one add at the boundary, A never sees B), not HTTP trivia.
- **Interface consistency:** Task 1 types feed Tasks 3/4; Task 2's store feeds Task 4; Task 4's service feeds Task 5's endpoints and guards; Task 6 exercises the whole chain on the real stack. The fake adapter remains the shared test double across contract, service and router tests.
- **Execution-detail scan:** every task names files, commands, expected results; migration numbers 000180/000101 confirmed free and self-contained (renumber-note included); the only Docker dependency is the optional tagged run; secrets env-only with no secret-shaped literals; container wiring additive and away from the Invoke-ordering trap; old gateway untouched; `concurrent_tasks` enforcement deferral is an explicit documented non-goal with the projection/limit surface delivered.
- **Known risks:** (1) Lago `amount_cents: 0` plan acceptance is unverified on v1.53.0 — Task 6 phase 1 decides with evidence and the 1-fen fallback is pre-agreed (product impact documented); (2) the entitlements read (`GET /api/v1/customers/{id}/entitlements`) is source-inferred for this use; if the runtime shape differs, the adapter falls back to features-from-publication (the projection already carries the definition) — the branch is stated and cheap; (3) the wallet read-back-by-name recovery relies on the customer wallet list carrying `name` — metadata is the documented fallback filter (E3 proved metadata query works on the transaction index; the wallet index shape is verified in Task 6 phase 3); (4) real-Lago evidence may be `blocked-env` (honest, non-blocking); (5) storage counter maintenance is best-effort with drift corrected at every projection refresh — permanent drift cannot over-block because limits apply AFTER the occupancy re-sync (ordering pinned by test); (6) #81 may land with a different migration number — renumbering is mechanical.
