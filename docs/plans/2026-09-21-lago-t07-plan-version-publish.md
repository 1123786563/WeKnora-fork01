# Lago T07 — Immutable Plan Version Publish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 套餐运营人员 can draft a plan, validate the commercial constraints, and publish a new **immutable Plan Version** through the frozen Commercial Platform seam to Lago (`publish_plan_version` command), with idempotent publish, durable receipts readable from the 管理后台, and zero impact on existing subscriptions. (Ticket #79, Wave 3; CONTEXT.md term: 套餐版本.)

**Architecture:** The frozen port `internal/commercial/platform.go` grows ONLY additively (T05 Task 2 freeze): a new `CommandKind` constant `publish_plan_version` plus a typed provider-neutral payload live in a NEW file `internal/commercial/plan_command.go` (the frozen file itself is not edited) — no signature changes, no per-object wrappers. Draft→validate→publish lifecycle extends the existing catalog domain (`internal/commercial/catalog.go` already defines `PlanVersion`, lifecycle states and `ValidateForPublish`; `commercial_plan_catalog` already stores `(plan_key, version)` rows) with a publish-validation entrypoint covering the six acceptance axes, and a NEW append-only projection table `commercial_plan_publications` (migration `000179` versioned / `000100` sqlite) mapping each published version to its command key, external plan code and receipt. A new `PlanVersionService` (`internal/application/service/commercial/planversion.go`) orchestrates validate → `SubmitCommand` → record receipt → flip state; the coordinator (not Lago) owns idempotency identity: `Command.Key = publish_plan_version:<plan_key>:<version>`, deterministic Lago plan code `weknora-<slug>-v<n>`. The Lago adapter (`internal/infrastructure/commercialplatform/lago.go`) implements the command as `POST /api/v1/plans` (v1.53.0 contract proven by the #74/#76 labs); a 422 already-exists is resolved by read-back verify (`GET /api/v1/plans/{code}` + field compare — the #76 lab proved 422 bodies cannot distinguish replay from conflict). Admin endpoints hang directly on the v1 group at `/api/v1/admin/plans/*` behind a NEW platform-scope grant `plan_publish` (exact `RequirePlatformRefundReviewer` precedent) — space roles never admit catalog operations. The external plan code never crosses the admin API (mapped via the local table). Old OpenMeter gateway untouched.

**Tech Stack:** Go 1.26 (`github.com/Tencent/WeKnora`), gin, gorm (SQLite tests + PostgreSQL versioned migrations), `net/http` + `httptest`, Docker Compose (`deploy/lago/lago.sh`) for the optional tagged real-stack evidence.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — "Catalog, subscriptions, and entitlements" (every published Plan Version a distinct Lago plan code; published version immutable; base price monotonic with tier), "Commercial authority and module seam" (CNY-only single Billing Entity, integer minor units), "Credits and Task admission" (initial Charge models limited to fixed unit / deterministic package / explicitly capped — the computable-upper-bound restriction); user stories 37–42 (套餐运营人员), 48/59 (idempotent replay, content conflict rejected); behavior matrix 3 (immutable catalog: new Plan Version creates a new external plan; existing subscriptions remain on their purchased version) and 21 (permissions). [ADR-0012](../adr/0012-lago-as-commercial-billing-authority.md), [ADR-0014](../adr/0014-commercial-platform-single-deep-seam.md). Lab facts: `docs/migrations/lago/t02-payment-activation/` (plan payload shape, pay_in_advance mandatory, entitlement attach map form), `docs/migrations/lago/t03-wallet-semantics/source-notes.md` (plan-level `pay_in_advance` boolean required), `docs/migrations/lago/t04-pricing-group/` (`billable_metric_id` reference form, `sum_agg`, decimal-string `properties.amount` ×100, 422-indistinguishable → read-back compare).

## Global Constraints

- **Isolation:** all work on branch `lago-79-plan-version-publish` in `.worktrees/lago-79`. Real-Lago evidence uses its own stack: `COMPOSE_PROJECT_NAME=weknora-lago-79`, `LAGO_API_PORT=48901`, `LAGO_FRONT_PORT=48902`. Never reuse another ticket's Compose project, volumes, or ports; `lago.sh down` preserves volumes per T01 convention.
- **Seam freeze (binding, from T05 Task 2):** the three method signatures in `internal/commercial/platform.go` are FROZEN. Growth is additive only: the `publish_plan_version` `CommandKind` constant and the `PublishPlanVersionPayload` type go in a NEW file `internal/commercial/plan_command.go`; `platform.go` is not edited. No `GetPlan`/`CreatePlan`-style per-object wrapper may appear anywhere (ADR-0014). `CommandReceipt.ExternalID` carries the Lago plan code inside the seam and NEVER crosses the admin API.
- **Provider neutrality:** admin API responses contain only WeKnora product vocabulary and closed tokens. The external/Lago plan code, lago ids, URLs, status enums and raw error text never appear in a response — the local `commercial_plan_publications` row is the mapping. Error mapping reuses the T05 closed tokens; `err.Error()` is never echoed. **Decision (documented in code):** the Lago plan code does NOT leak; callers address versions by `(plan_key, version)` and receive `receipt:{received, published_at}` plus WeKnora's own command key.
- **CNY-only first slice, integer minor units:** the payload currency is the closed token `"CNY"`; all money is `int64` fen end-to-end. The Lago adapter converts fen to Lago's decimal-string `properties.amount` by exact integer→string formatting (fen/100), never binary float (spec: "Monetary values use integer minor units and never pass through binary floating point"; T04 lab ×100 fact).
- **Computable cost upper bound = closed charge-model set:** publish validation accepts only `fixed_unit` and `package` charge models (explicit integer fen amount; package_size > 0; free_units >= 0). Graduated, percentage, minimum-commitment and commitment-spend models are rejected fail-closed (spec out-of-scope list). A draft carrying no charges is valid in T07 (usage pricing dimensions arrive with #87/#88); the rule still binds any charge that IS present.
- **Validation runs BEFORE publish, in WeKnora domain code:** `internal/commercial` pure functions, table-driven; publish refuses (stays draft) on any failed axis. The six axes: 基础价格层级 (tier price ladder + non-inversion vs currently-published neighbors), CNY, Entitlement, Resource Quota, included Credits, 费用上界 (charge-model whitelist).
- **Immutability:** a published row can never be edited in place — service-layer guard (existing `SaveDefinition` rejects published rows; `Publish` is idempotent no-op on already-published) plus a DB trigger in the migration rejecting `definition_json`/`external_id` changes on published rows (both PostgreSQL and SQLite dialects). Any commercial change = new `(plan_key, version)` row = new Lago plan code.
- **Existing subscriptions untouched:** the publish path performs NO writes to `commercial_subscriptions` and no Lago subscription calls; proven by a service-layer assertion test (Task 4) and by real-stack evidence: a subscription pinned to v1 survives a v2 publish byte-identical (Task 6).
- **Secrets:** Lago credentials only from the existing `WEKNORA_COMMERCIAL_PLATFORM_*` env family (T05 `config.go`); nothing committed, logged, or echoed. Integration-test keys only via `LAGO_INTEGRATION_*` env.
- **Migration number:** versioned `000179_commercial_plan_publications` + sqlite `000100_commercial_plan_publications` (next free pair is 000178/000099; #78 [Lago Customer] is expected to take 000178/000099 but has NO plan in the repo yet — **controller dedups at W3 integration**, waves doc assigns migration numbers; if #78 lands first with a different number, renumber mechanically before merge, the migration is self-contained).
- **Admin capability gating:** plan publish/validate/draft endpoints are platform-scope only — a platform API key (`scope.IsPlatform()`) or a human carrying `commercial_grants(tenant_id=0, capability='plan_publish')`. A space owner/Admin/billing-grantee is REJECTED (403), exactly the `RequirePlatformRefundReviewer` precedent: catalog operations are cross-space and must never be reachable through tenant authority.
- **Old gateway untouched:** `internal/infrastructure/openmeter/**` and its registration are not modified; both seams coexist until #105.
- **Trunk green:** `go build ./...` and the Go test suite pass at every commit. Real-Lago integration is separately tagged (`//go:build lago_integration`) and env-gated; Docker unavailability is recorded as `blocked-env`, never a failure or a fake pass.

## Review Focus

- **Mutable published version:** any path that updates `definition_json`/`external_id` of a published row (service, repository, raw SQL, gorm `Updates`) is a defect. Reviewer checks the DB trigger exists in BOTH migration dialects and the repository tests prove it (direct UPDATE on a published row must fail), plus that `Publish` on a published row is an idempotent no-op returning the recorded receipt without a second seam call.
- **Non-idempotent publish / double plan:** a retry (response loss, timeout, operator double-click) must never create a second Lago plan or a second publication row. Identity is coordinator-owned: same `Command.Key` → same plan code → Lago 422 → read-back verify → same receipt. Reviewer hunts for any path that regenerates the plan code or mints a new Key on retry, and for the content-conflict hole: same Key with different payload must be rejected (`ErrPlatformInvalidResponse`-mapped `publish_conflict`), verified by read-back compare (#76 lab: 422 bodies are indistinguishable).
- **Validation gap on the upper bound:** a graduated/percentage/minimum-commitment model, a float money conversion, a negative package_size, or a charge with no explicit fen amount slipping through publish is a defect. Table-driven tests must enumerate the rejected model tokens and the accepted two; the ladder and neighbor-inversion rules must fail closed on unknown tier keys.
- **Lago plan code leakage in the admin API:** every admin response is asserted (router test) to contain neither the `weknora-…-v<n>` external code nor any `lago` substring, including error paths (adapter configured with a marker URL, forced unreachable/conflict).
- **Subscription side-effect:** publish must not touch subscriptions. Service test asserts `commercial_subscriptions` (and Lago subscription endpoints via the adapter stub's recorded paths) see zero writes/calls during draft→validate→publish; real-stack evidence re-reads a v1 subscription after v2 publish.
- **(Secondary) seam-freeze creep:** exactly one new kind + one payload type; no new methods on `CommercialPlatform`; the shared contract table still runs identical legs for fake and Lago adapters.

---

### Task 1: Domain — publish command payload + the six-axis publish validation (table-driven)

**Files:**
- Create: `internal/commercial/plan_command.go`
- Create: `internal/commercial/plan_command_test.go`
- Modify (additive fields only): `internal/commercial/catalog.go`

**Interfaces:**
- Produces (all in package `commercial`, provider-neutral — no Lago vocabulary):

```go
// plan_command.go
const CommandKindPublishPlanVersion CommandKind = "publish_plan_version"

// Closed first-slice charge models (spec: computable-upper-bound restriction).
const (
    ChargeModelFixedUnit = "fixed_unit" // Lago "standard" (adapter maps)
    ChargeModelPackage   = "package"    // Lago "package" (adapter maps)
)

// PlanCharge is one usage-pricing component of a plan version. AmountFen is
// integer fen per unit (fixed_unit) or per package (package). Optional in
// T07 — a plan version without charges prices only its base fee.
type PlanCharge struct {
    Dimension    string // product service-dimension code; the adapter resolves it to the provider metric
    Model        string // ChargeModelFixedUnit | ChargeModelPackage
    AmountFen    int64
    PackageUnits int64 // package model only: units per package, > 0
    FreeUnits    int64 // >= 0
}

// PublishPlanVersionPayload is the typed W3 payload for the frozen seam.
type PublishPlanVersionPayload struct {
    PlanKey             string
    Version             int64
    PlanCode            string // deterministic weknora-<slug>-v<n>; the adapter echoes it as ExternalID
    Name                string
    Interval            string // closed token "monthly" in T07
    AmountFen           int64  // base subscription price, CNY fen, > 0
    Currency            string // closed token "CNY"
    PayInAdvance        bool   // always true in T07 (spec: initial subscriptions pay-in-advance)
    IncludedCreditsMicro int64 // monthly included credits, > 0
    Features            map[string]bool // entitlements
    Limits              map[string]int64 // resource quotas (absent = unlimited, 0 = hard zero)
    Charges             []PlanCharge
}
func (p PublishPlanVersionPayload) Validate() error // structural: closed tokens, slug form, fen/units bounds, no unknown charge model

// DeterministicPlanCode(planKey string, version int64) string  → "weknora-<slug>-v<n>"
// PublishCommandKey(planKey string, version int64) string      → "publish_plan_version:<plan_key>:<version>"
```

- Produces (catalog.go, additive — existing `ValidateForPublish` untouched for the legacy path): `PlanVersion` gains `Currency string` and `Charges []PlanCharge` (optional, `nil` valid; `definition_json` decodes backward-compatibly), plus:

```go
// PriceLadder is the closed, ascending tier order of the first slice.
type TierPrice struct { TierKey string; PriceFen int64 }
type PriceLadder []TierPrice // invariant: ascending, unique tier keys
func DefaultPriceLadder() PriceLadder            // the first-slice constant (values defined with the tests)
func (l PriceLadder) Validate() error            // ascending + unique

// PublishValidationContext feeds the cross-version rules.
type PublishValidationContext struct {
    Ladder            PriceLadder
    PublishedPrices   map[string]int64 // tierKey → price_fen of its currently-published version
}

// ValidateForPublishValidated enforces the SIX acceptance axes and returns
// an itemized, closed-token error per failed axis:
//   axis: base_price_tier (ladder membership + no inversion vs PublishedPrices neighbors)
//         currency_cny, entitlements, resource_quota, included_credits, cost_upper_bound
func (p PlanVersion) ValidateForPublishValidated(ctx PublishValidationContext) error
```

- [ ] **Step 1: Write failing tests (RED)**

`plan_command_test.go`, table-driven:
1. `TestPayloadValidate` table: rejects — empty PlanKey/PlanCode, PlanKey with chars outside `[a-z0-9-]`, Version <= 0, PlanCode not equal to `DeterministicPlanCode(PlanKey, Version)` (determinism is structural), AmountFen <= 0, Currency != "CNY", Interval != "monthly", PayInAdvance false, IncludedCreditsMicro <= 0, charge model outside the closed set (`"graduated"`, `"percentage"`, `"minimum_commitment"` each rejected), package charge with PackageUnits <= 0, AmountFen <= 0 or FreeUnits < 0; accepts — a fully-formed payload, and a payload with `Charges == nil`.
2. `TestDeterministicPlanCodeAndCommandKey`: `weknora-pro-v3` / `weknora-pro-max-v12` forms; same inputs → same outputs (pure).
3. `TestValidateForPublishValidatedSixAxes` — one table case per axis and per escape: (a) price off the ladder → `base_price_tier` error; (b) price that would invert tier ordering vs a published neighbor (`PublishedPrices` contains a higher tier at a lower price) → `base_price_tier` error; (c) legitimate on-ladder price change accepted; (d) non-CNY currency → `currency_cny`; (e) entitlement key not in the closed first-slice feature set (or empty key / non-bool impossible in Go — use unknown key) → `entitlements`; (f) limit key outside the closed dimension set or negative value → `resource_quota` (zero-vs-absent both valid, both asserted); (g) IncludedCreditsMicro <= 0 → `included_credits`; (h) each forbidden charge model and a float-shaped amount string (not representable — amount arrives as int64; the case is a charge whose fen-to-decimal round-trip would not be exact — assert the helper `FenToDecimalString` round-trips exactly for the accepted set) → `cost_upper_bound`; (i) `PriceLadder.Validate` rejects descending/duplicate ladders; (j) unknown TierKey fails closed.
4. `TestPlanVersionJSONRoundTripAddsFields`: a pre-T07 `definition_json` (no currency/charges) decodes with `Currency == ""` treated as invalid for publish (must be set explicitly) — no silent default.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/commercial/ -run 'TestPayload|TestDeterministic|TestValidateForPublishValidated|TestPriceLadder|TestPlanVersionJSON' -v`

Expected: compile failure — `plan_command.go` does not exist.

- [ ] **Step 3: Implement**

Write `plan_command.go` exactly as the interface above (doc comments citing ADR-0014 additive-kind rule and the spec sections). Add `Currency`/`Charges` to `PlanVersion` in `catalog.go` with a comment marking them additive T07 fields. `FenToDecimalString(fen int64) string` does exact integer→`"yuan.fen"` formatting (reuse the `formatFixed` precedent in `amount.go`; no float). The closed feature-key and quota-dimension sets are small exported slices (`PublishedFeatureKeys`, `QuotaDimensionKeys`) — first-slice constants, values fixed alongside the tests; unknown keys fail closed.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/commercial/ -v`

Expected: new tests pass; existing `catalog`/`quote`/`order` domain tests stay green (additive fields only).

- [ ] **Step 5: Commit**

`feat(commercial): publish_plan_version payload and six-axis publish validation (#79)`

### Task 2: Local persistence — publications projection migration, repository, immutability trigger

**Files:**
- Create: `migrations/versioned/000179_commercial_plan_publications.up.sql` / `.down.sql`
- Create: `migrations/sqlite/000100_commercial_plan_publications.up.sql` / `.down.sql`
- Create: `internal/application/repository/commercial/planversion.go`
- Create: `internal/application/repository/commercial/planversion_test.go`

**Interfaces:**
- Consumes: `PlanRow`/`CatalogStore` (existing `catalog.go` — SaveDefinition/Publish stay as the legacy local path; NOT modified except additive reads), domain types from Task 1.
- Produces:

```go
// PublicationRow is the append-only publish projection: the immutable map
// from a plan version to its seam command identity and receipt.
type PublicationRow struct {
    CommandKey  string    `gorm:"primaryKey;column:command_key"` // publish_plan_version:<key>:<version>
    PlanKey     string    `gorm:"column:plan_key;uniqueIndex:uq_plan_publication_version"`
    Version     int64     `gorm:"column:version;uniqueIndex:uq_plan_publication_version"`
    PlanCode    string    `gorm:"column:plan_code;uniqueIndex"` // seam-internal; never crosses the admin API
    ReceiptJSON string    `gorm:"column:receipt_json;not null"`
    PublishedBy string    `gorm:"column:published_by;not null default ''"`
    PublishedAt time.Time `gorm:"column:published_at;not null"`
}
func (PublicationRow) TableName() string { return "commercial_plan_publications" }

type PlanVersionStore struct{ db *gorm.DB }
func NewPlanVersionStore(db *gorm.DB) *PlanVersionStore

func (s *PlanVersionStore) EnsureSchema(ctx context.Context) error // portable CREATE TABLE IF NOT EXISTS (NewCommercialHandler precedent) + the immutability trigger (CREATE TRIGGER IF NOT EXISTS equivalent per dialect) so tests/dev deployments are safe without the migration
func (s *PlanVersionStore) NextVersion(ctx context.Context, planKey string) (int64, error)      // max(version)+1; races resolved by the PK + caller retry
func (s *PlanVersionStore) CreateDraft(ctx context.Context, row repocommercial.PlanRow) error   // state=draft only
func (s *PlanVersionStore) UpdateDraft(ctx context.Context, planKey string, version int64, definitionJSON string) error // rejects non-draft (incl. published → ErrPublishedPlanImmutable)
func (s *PlanVersionStore) SetPublishing(ctx context.Context, planKey string, version int64) error // draft|publishing → publishing (retry-safe)
func (s *PlanVersionStore) RecordPublication(ctx context.Context, row PublicationRow, planKey string, version int64) error // ONE transaction: insert-or-verify publication + flip catalog row publishing→published
func (s *PlanVersionStore) GetPublication(ctx context.Context, planKey string, version int64) (PublicationRow, error) // ErrPublicationNotFound
func (s *PlanVersionStore) ListVersions(ctx context.Context) ([]VersionView, error) // catalog rows LEFT JOIN publications, ordered
func (s *PlanVersionStore) GetVersion(ctx context.Context, planKey string, version int64) (VersionView, error)
```

- The versioned (PostgreSQL) migration creates `commercial_plan_publications` (columns as above, `TIMESTAMPTZ`, comments in the 000175 style) plus a trigger `trg_commercial_plan_catalog_published_immutable` raising `'published_plan_immutable'` on any UPDATE of `definition_json`/`external_id` when `OLD.state = 'published'`; the sqlite migration mirrors both with `RAISE(ABORT, …)` syntax. Down drops the trigger then the table.

- [ ] **Step 1: Write failing repository tests (RED)**

Sqlite in-memory gorm (the `order_test.go` harness pattern; `EnsureSchema` bootstraps). Cases:
1. `TestNextVersionMonotonic`: seeded rows v1..v3 → next = 4; empty key → 1.
2. `TestDraftLifecycle`: CreateDraft (state draft) → UpdateDraft mutates → SetPublishing → RecordPublication flips the catalog row to `published` and inserts the publication atomically (assert both in one read-back).
3. `TestPublishedRowImmutableEveryLayer`: after RecordPublication — (a) `UpdateDraft` returns `ErrPublishedPlanImmutable`; (b) a DIRECT `db.Exec("UPDATE commercial_plan_catalog SET definition_json = … WHERE …")` on the published row FAILS with the trigger error (prove the DB constraint, not just the service guard); (c) `SaveDefinition` (legacy path) also rejects.
4. `TestRecordPublicationIdempotent`: replaying the same `CommandKey` verifies-equals and returns without error and without a second row (unique constraint + compare-on-conflict → `ErrPublicationConflict` when the stored publication differs).
5. `TestListAndGetVersionsJoinReceipts`: published version carries receipt presence + `published_at`; draft carries neither.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/repository/commercial/ -run TestPlanVersion -v`

Expected: compile failure — package file does not exist.

- [ ] **Step 3: Implement**

Write the four migration files (versioned 000179 / sqlite 000100 — numbers reserved per Global Constraints) and `planversion.go` with `EnsureSchema` (portable DDL: `CREATE TABLE IF NOT EXISTS`, trigger created per-dialect — guard with a dialect check like `db.Dialector.Name()`; PostgreSQL trigger needs `CREATE OR REPLACE FUNCTION` + `DROP TRIGGER IF EXISTS` + `CREATE TRIGGER`). `RecordPublication` runs in one `Transaction`: insert the publication (on PK conflict read back and compare — mismatch → `ErrPublicationConflict`), then guarded UPDATE `state='publishing' → 'published'` (RowsAffected 0 + already published → idempotent success).

- [ ] **Step 4: Verify GREEN + migration sanity**

Run: `go test ./internal/application/repository/commercial/ -v && bash -c 'ls migrations/versioned/000179* migrations/sqlite/000100*'`

Expected: repository suite green (existing catalog/order/subscription tests untouched and green); the four migration files exist with paired `.down.sql`.

- [ ] **Step 5: Commit**

`feat(commercial): plan version publications projection with immutability trigger (#79)`

### Task 3: Seam command — fake + Lago adapters and the shared contract leg

**Files:**
- Modify: `internal/infrastructure/commercialplatform/fake.go`
- Modify: `internal/infrastructure/commercialplatform/lago.go`
- Modify: `internal/infrastructure/commercialplatform/contract_test.go`
- Create: `internal/infrastructure/commercialplatform/lago_plan_test.go`

**Interfaces:**
- Consumes: Task 1 `CommandKindPublishPlanVersion` + `PublishPlanVersionPayload`; frozen `SubmitCommand` signature (unchanged).
- Produces:
  - Fake: `SubmitCommand(publish_plan_version)` validates `Command.Validate` + payload, records under `Key`, returns `CommandReceipt{Key, ExternalID: payload.PlanCode, RecordedAt}`. Replay same Key + byte-equal payload → the SAME receipt, no second record; same Key + different payload → wrapped `ErrPlatformInvalidResponse` (`publish_command_conflict` — spec story 59). Any other kind still `ErrPlatformUnsupported`. Add a mutex around command state (T05 fake was read-only; concurrent publish tests need it). Test hook `FakeAdapter.Commands() []Command`.
  - Lago adapter `SubmitCommand` for the kind: `configured()` → validate payload → `POST {base}/api/v1/plans` with `{"plan":{code, name, interval, amount_cents, amount_currency, pay_in_advance: true, trial_period: 0.0, charges: […]}}`; charges (when present) resolved per Dimension via `GET {base}/api/v1/billable_metrics/{dimension}` → `billable_metric_id` (T04 fact: id-reference, not code; 404 → wrapped `ErrPlatformInvalidResponse`, pricing dimension not provisioned — fail closed); charge `properties.amount` = `FenToDecimalString(AmountFen)` (T04 ×100 fact); `charge_model` mapped `fixed_unit→"standard"`, `package→"package"` with `package_size`/`free_units`. Features: ensure each key exists (`POST /api/v1/features {feature:{code,name}}` treating already-exists as success) then attach `POST /api/v1/plans/{code}/entitlements` with `{"entitlements": {key: {}}}` — the MAP form (T02 fact: an array makes `PlanEntitlementsUpdateService` 500). Outcomes: 2xx → receipt; **422 → read-back verify** `GET {base}/api/v1/plans/{code}`, compare `(amount_cents, amount_currency, interval, pay_in_advance)` — equal → receipt (idempotent replay), different → `ErrPlatformInvalidResponse` (`publish_conflict`); other 4xx → `ErrPlatformInvalidResponse`; 5xx/transport/timeout → `ErrPlatformUnreachable`. No URL/status/body text in errors; API key only in the `Authorization` header.

- [ ] **Step 1: Write failing tests (RED)**

`lago_plan_test.go` — `planStub` httptest server (the `healthStub` pattern) recording method+path+body, scripted responses: (1) happy path → receipt `ExternalID == payload.PlanCode`, request body carries `pay_in_advance: true`, `trial_period: 0`, integer `amount_cents`, CNY, and the `Authorization` header; (2) replay: stub answers the second POST with 422 + a GET that returns the SAME fields → one receipt, and the test asserts the adapter issued the read-back GET (T04 indistinguishable-422 fact); (3) conflict: 422 + read-back with different `amount_cents` → `errors.Is(err, ErrPlatformInvalidResponse)`; (4) other-4xx → `ErrPlatformInvalidResponse`; 500/refused → `ErrPlatformUnreachable`; (5) unconfigured → `ErrPlatformUnconfigured`; (6) charge resolution: `GET /api/v1/billable_metrics/{dim}` 200 → payload embeds `billable_metric_id` and decimal-string `properties.amount` (`AmountFen 7` → `"0.07"`); metric 404 → invalid-response fail-closed; (7) feature ensure + entitlement attach use the map form (stub asserts the body shape is an object, not an array); (8) no `lago` URL/path substring in any error string.
`contract_test.go`: update the T05 unsupported-command leg to use a never-enabled kind (keep `ensure_customer` — still unsupported in both adapters) and ADD a shared `runPublishContract(t, name, p, prime …)` leg run identically for fake and stub-backed Lago: publish → receipt with ExternalID == PlanCode; replay same Key → same receipt and NO second create (each adapter exposes a way to observe — fake `Commands()`, stub POST count); same Key different payload → `ErrPlatformInvalidResponse`; unknown kinds unsupported.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: compile/runtime failures — `SubmitCommand` still returns `ErrPlatformUnsupported` for the publish kind.

- [ ] **Step 3: Implement**

Extend both adapters per the interfaces. Shared helpers stay unexported; provider vocabulary (`plan`, `charges`, `billable_metric_id`) lives ONLY in `lago.go`. The fake stays deterministic; replay comparison is on the encoded payload (`reflect.DeepEqual` on the decoded payload).

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/infrastructure/commercialplatform/ ./internal/commercial/ -v`

Expected: the two-adapter contract table (readiness + publish legs) passes; T05 tests stay green.

- [ ] **Step 5: Commit**

`feat(commercialplatform): publish_plan_version command on fake and Lago adapters (#79)`

### Task 4: Publish orchestration service — validate, command, receipt, subscription non-impact

**Files:**
- Create: `internal/application/service/commercial/planversion.go`
- Create: `internal/application/service/commercial/planversion_test.go`

**Interfaces:**
- Consumes: `PlanVersionStore` (Task 2), `CatalogStore` reads, `commercial.CommercialPlatform` (Task 3), `DefaultPriceLadder`, `ValidateForPublishValidated`.
- Produces:

```go
type PlanVersionService struct { versions *repocommercial.PlanVersionStore; catalog *repocommercial.CatalogStore; platform commercial.CommercialPlatform }
func NewPlanVersionService(db *gorm.DB, platform commercial.CommercialPlatform) (*PlanVersionService, error) // nil platform legal (blocked-env): draft/validate/list work; publish fails closed

type DraftInput struct { PlanKey, Name string; AmountFen int64; IncludedCreditsMicro int64; Features map[string]bool; Limits map[string]int64; Currency string; Charges []commercial.PlanCharge }
func (s *PlanVersionService) CreateDraft(ctx, actor string, in DraftInput) (VersionView, error) // allocates NextVersion, PlanCode = DeterministicPlanCode, state draft
func (s *PlanVersionService) UpdateDraft(ctx, planKey string, version int64, in DraftInput) (VersionView, error)
func (s *PlanVersionService) Validate(ctx, planKey string, version int64) (ValidationReport, error) // itemized per-axis report; NO state change
func (s *PlanVersionService) Publish(ctx, actor, reason, planKey string, version int64) (PublishResult, error)
```

- `Publish` algorithm (one place, fail-closed): load row → if already `published` AND a publication exists → return the recorded receipt (200-idempotent, NO seam call) → decode + `ValidateForPublishValidated` (context: ladder + currently-published prices) → invalid stays `draft`, returns the report → `SetPublishing` → `platform.SubmitCommand(Command{Kind: publish_plan_version, Key: PublishCommandKey(…), Actor, Reason, Payload})` → receipt → `RecordPublication` (command key, PlanCode from the payload, receipt JSON, actor) → `PublishResult{VersionView, receipt received:true}`. Adapter error mapping: `ErrPlatformUnconfigured` → row stays publishing, closed error `platform_unconfigured`; `ErrPlatformUnreachable` → stays publishing, `platform_unreachable` (retry with the SAME key is the recovery — spec story 48); `ErrPlatformInvalidResponse` → stays publishing, `publish_conflict` (operator attention; a code exists with different content). `PublishResult` and `ValidationReport` carry closed tokens only — never the plan code, never provider text.

- [ ] **Step 1: Write failing service tests (RED)**

Sqlite harness (fake adapter or a scripted stub platform). Cases:
1. `TestPublishHappyPath`: draft → validate (all axes pass) → publish → catalog row `published`, publication row present with `PlanCode == weknora-<key>-v1`, receipt recorded.
2. `TestPublishValidationFailsClosedStaysDraft`: one failing axis per AC group (off-ladder price, non-CNY, unknown feature, negative limit, zero credits, graduated charge) → publish returns the itemized report, state stays `draft`, ZERO `SubmitCommand` calls (platform spy counts).
3. `TestPublishIdempotentAcrossReplays`: (a) after a successful publish, a second `Publish` returns the SAME receipt with no seam call; (b) response-loss simulation: platform returns `ErrPlatformUnreachable` on first call → row `publishing` → second `Publish` replays the SAME `Command.Key` (spy asserts byte-equal Key and payload) → success → exactly one publication row; (c) `ErrPlatformInvalidResponse` → closed `publish_conflict`, row stays publishing.
4. `TestNewVersionNewCodeAndOldImmutable`: publish v1, draft+publish v2 → distinct plan codes (`-v1`/`-v2`), version numbers strictly increasing; `UpdateDraft` on published v1 → `ErrPublishedPlanImmutable`.
5. `TestPublishNeverTouchesSubscriptions`: seed one `commercial_subscriptions` row; run the full draft→validate→publish; assert the row is byte-identical (all columns) and no other subscription rows exist. (The real-stack re-proof is Task 6.)
6. `TestNilPlatformFailsClosed`: draft/validate/list work; publish → closed `platform_unconfigured`, stays publishing-safe (no state flip, no fabricated receipt).
7. `TestConcurrentCreateDraftVersionsMonotonic`: two goroutines creating drafts for the same key → distinct, gap-free-enough monotonic versions (PK retry inside `NextVersion`).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/service/commercial/ -run TestPlanVersion -v`

Expected: compile failure.

- [ ] **Step 3: Implement**

Follow the algorithm exactly; all persistence through the store's transactional units (the `OrderService` no-direct-db-handle precedent). The platform spy in tests is a local stub implementing the frozen interface.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/application/service/commercial/ ./internal/application/repository/commercial/ -v`

Expected: new suite green; order/refund/lifecycle suites untouched and green.

- [ ] **Step 5: Commit**

`feat(commercial): plan version publish orchestration with idempotent seam command (#79)`

### Task 5: 管理后台 admin API — platform-scope gate, provider-neutral responses, wiring

**Files:**
- Modify: `internal/handler/commercial.go`
- Modify: `internal/router/routes_commercial.go`
- Modify: `internal/container/container.go`
- Create: `internal/router/commercial_plan_admin_route_test.go`

**Interfaces:**
- Consumes: `PlanVersionService` (Task 4); `commercial_grants` platform-scope rows; the T05 route-group conventions.
- Produces (all hung DIRECTLY on the v1 parent group — NOT the tenant commercial group — behind `RequirePlatformPlanPublisher()`, the exact `RequirePlatformRefundReviewer` mirror: platform API key `scope.IsPlatform()` OR `commercial_grants(tenant_id=0, capability='plan_publish')`; everything else 403):

```
POST   /api/v1/admin/plans/drafts                       CreatePlanDraft
PATCH  /api/v1/admin/plans/drafts/:key/:version         UpdatePlanDraft
POST   /api/v1/admin/plans/drafts/:key/:version/validate  ValidatePlanDraft
POST   /api/v1/admin/plans/drafts/:key/:version/publish   PublishPlanVersion (Actor from principal, Reason from body)
GET    /api/v1/admin/plans/versions                     ListPlanVersions
GET    /api/v1/admin/plans/versions/:key/:version       GetPlanVersion
```

- Handler surface: `const PlatformPlanPublisherCapability = "plan_publish"`; `SetPlanVersionService(*commercialsvc.PlanVersionService)`; wire in `container.go` next to the T05 platform Invoke block (~line 776): `Provide(commercialsvc.NewPlanVersionService)` + `Invoke` calling `SetPlanVersionService` — away from the pre-craft Invoke-ordering trap. Response fields (closed set): `plan_key, version, state, name, amount_fen (digit string, quoteWire precedent), currency ("CNY"), included_credits_micro, features, limits, created/updated, validation (itemized per-axis array), receipt {received bool, published_at, command_key}` — the external plan code and every Lago identifier absent BY DESIGN (mapping lives in `commercial_plan_publications`). Errors: closed tokens (`platform_unconfigured` → 503, `platform_unreachable` → 503 retryable, `publish_conflict` → 409, `published_plan_immutable` → 409, `plan_version_not_found` → 404, validation failure → 422 with the itemized report).

- [ ] **Step 1: Write failing router tests (RED)**

`commercial_plan_admin_route_test.go` — the `commercial_platform_route_test.go` harness (gin engine + `RegisterCommercialRoutes` + injected auth). Cases:
1. `TestPlanAdminRequiresPlatformPublisher`: space admin with tenant billing grant → 403 on every endpoint; tenant owner → 403; anonymous → 401 (auth layer); platform-scope `plan_publish` grant (tenant_id=0) → admitted; platform API key (`scope.IsPlatform()`) → admitted.
2. `TestPlanAdminLifecycleOverFake`: fake adapter; create draft (201/200 envelope `{success:true,data:…}`) → validate (itemized axes) → publish → version row published; list + get return the receipt block (`received:true, published_at, command_key`) and the state.
3. `TestPlanAdminProviderNeutrality`: fake platform whose errors carry marker text + a Lago-configured marker base URL; force unconfigured/unreachable/conflict paths → assert the response bodies contain NO `weknora-…-v` external plan code in any RECEIPT-free field, no `lago` substring, no marker, no URL, no raw error text (the T05 leak-test pattern).
4. `TestPlanAdminImmutabilitySurface`: PATCH a published version → 409 `published_plan_immutable`; PATCH a draft → 200; publish replay (double POST) → one receipt, no error.
5. `TestPlanAdminNilServiceFailsClosed`: no service wired → draft endpoints 503 closed error, never a fabricated success.
6. `TestPlanAdminNotOnTenantGroup`: the tenant-scoped `/api/v1/commercial/plans` (member view) keeps listing published versions only and is unaffected by admin drafts (regression).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/router/ -run TestPlanAdmin -v`

Expected: routes/handlers do not exist.

- [ ] **Step 3: Implement**

Handlers follow the `PlatformReadiness` closed-envelope discipline; `err.Error()` never echoed; wire the container block with the ordering comment. Keep `RegisterCommercialRoutes`' existing signature additive-only (the new endpoints are registered inside it on the parent group, mirroring the refund-review route).

- [ ] **Step 4: Verify GREEN + regression**

Run: `go test ./internal/router/ ./internal/handler/ ./internal/container/ ./internal/application/... ./internal/commercial/ ./internal/infrastructure/... -v && go build ./...`

Expected: all green; T05 readiness tests and the openmeter suite untouched.

- [ ] **Step 5: Commit**

`feat(billing): platform-gated admin plan version API behind the seam (#79)`

### Task 6: Real-stack integration evidence + final verification

**Files:**
- Create: `internal/infrastructure/commercialplatform/lago_plan_integration_test.go` (`//go:build lago_integration`)
- Create: `deploy/lago/evidence/t07-run.txt` (operator timeline, if the stack runs)
- Create: `docs/migrations/lago/t07-plan-version-publish/README.md` (evidence index, the t02–t04 convention)

**Interfaces:**
- Consumes: Tasks 3–5; the #73 operator lifecycle `./deploy/lago/lago.sh init|up|status|down`; the ticket-isolated stack env.
- Produces: real pinned-Lago `v1.53.0` proof for all four acceptance criteria, or honest `blocked-env`.

- [ ] **Step 1: Write the tagged integration test (RED without env, GREEN with stack)**

`lago_plan_integration_test.go`, `//go:build lago_integration`, env-gated on `LAGO_INTEGRATION_BASE_URL` + `LAGO_INTEGRATION_API_KEY` (skip → `blocked-env` log). Phases in ONE test (uuid-prefixed synthetic keys, the lab isolation convention):
1. **Publish v1** through `NewLagoAdapter` + `SubmitCommand` (payload with `PlanKey "t07-<uuid>"`, CNY, one `fixed_unit` charge optional-skip if no metric is provisioned — default: no charges): receipt returned; `GET /api/v1/plans/{code}` read-back confirms `amount_cents`/`amount_currency`/`interval`/`pay_in_advance` (assert `lockedRelease`-style pin only where applicable — plan fields come from the created object, that is a read-back of OUR write, allowed).
2. **Idempotent replay**: a SECOND `SubmitCommand` with the SAME `Command.Key` and byte-equal payload → success receipt, and `GET /api/v1/plans?per_page=100` shows EXACTLY ONE plan with that code (no double plan).
3. **Subscription non-impact**: create a Lago customer (`external_id t07-<uuid>-cust`) + subscription on plan v1 (`POST /api/v1/subscriptions`, `external_id` per T03 fact: own `external_id` mandatory) → publish **v2** through the adapter → re-read the subscription: `plan_code` still the v1 code, `external_id` unchanged, `status` unchanged, `lago_id` unchanged.
4. **Conflict detection**: `SubmitCommand` same Key with a DIFFERENT `AmountFen` → `ErrPlatformInvalidResponse` (read-back compare caught the content conflict).

- [ ] **Step 2: Verify the tag gates it**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: the integration test does not run (no tag); the normal suite stays Docker-free and green.

- [ ] **Step 3: Run the isolated stack and collect evidence**

```
COMPOSE_PROJECT_NAME=weknora-lago-79 LAGO_API_PORT=48901 LAGO_FRONT_PORT=48902 ./deploy/lago/lago.sh init
COMPOSE_PROJECT_NAME=weknora-lago-79 LAGO_API_PORT=48901 LAGO_FRONT_PORT=48902 ./deploy/lago/lago.sh up
COMPOSE_PROJECT_NAME=weknora-lago-79 LAGO_API_PORT=48901 LAGO_FRONT_PORT=48902 ./deploy/lago/lago.sh status --json
LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48901 LAGO_INTEGRATION_API_KEY=… go test -tags lago_integration ./internal/infrastructure/commercialplatform/ -run TestLagoPlanPublishIntegration -v
COMPOSE_PROJECT_NAME=weknora-lago-79 ./deploy/lago/lago.sh down
```

Expected: all four phases pass against `v1.53.0` (check `deploy/lago/images.lock.json`). Record the timeline in `deploy/lago/evidence/t07-run.txt` and the artifact index in `docs/migrations/lago/t07-plan-version-publish/README.md`. If Docker or an operator key is unavailable: record `blocked-env` (never a fake pass) and rely on the stub-backed contract evidence; state this honestly in the ledger.

- [ ] **Step 4: Final verification sweep**

Run: `go build ./... && go test ./...` and `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe -v`

Expected: trunk green — seam, adapters, repository, service, router suites plus all pre-existing commercial/openmeter suites.

- [ ] **Step 5: Commit, ledger, ticket**

`docs(lago): T07 plan version publish real-stack evidence (#79)`; write the SDD ledger (`docs/plans/ledgers/lago-79.md`, per the waves convention); tick the ticket's four acceptance boxes with pointers:

| AC | Proof |
|---|---|
| 发布校验覆盖基础价格层级、CNY、Entitlement、Resource Quota、included Credits、费用上界 | Task 1 table-driven six-axis tests + Task 4 `TestPublishValidationFailsClosedStaysDraft` (zero seam calls) + Task 5 validate endpoint test |
| 每个新版本获得独立 plan code，发布命令幂等 | Task 1 `DeterministicPlanCode` + Task 2 unique publication + Task 3 contract replay leg (fake + stub Lago) + Task 4 replay/response-loss tests + Task 6 phase 1–2 (real 422 read-back, exactly one plan) |
| 已发布版本不能原地修改 | Task 2 trigger test (direct SQL UPDATE fails) + Task 4 immutability test + Task 5 PATCH-published 409 |
| 已有 Subscription 不因新版本发布而改变 | Task 4 `TestPublishNeverTouchesSubscriptions` + Task 6 phase 3 (real subscription byte-stable across v2 publish) |

## Plan Self-Review

- **Spec coverage / acceptance mapping:** every ticket AC maps to at least one unit/contract test AND one real-stack phase (table above). Spec "Catalog, subscriptions, and entitlements" decisions implemented verbatim: distinct plan code per version (deterministic `weknora-<slug>-v<n>`), immutability (service + DB trigger + idempotent no-op), tier price monotonicity (ladder + neighbor non-inversion validation), CNY-only integer-fen (closed token + `FenToDecimalString` exactness), computable-upper-bound charge whitelist (spec "Credits and Task admission" first-slice list). User story 42 (published Lago Plans editable only through audited WeKnora commands): Actor/Reason ride the Command and land in the publication row; Lago-UI direct mutation is out of scope (documented, spec out-of-scope "Direct operator mutation of published Lago Plans" is a prohibition on OUR operators, upheld by the immutability trigger).
- **Seam freeze respected:** one new kind constant + one payload type in a NEW file; `platform.go` unedited; three method signatures untouched; no per-object wrappers; `ExternalID` (plan code) stays seam-internal — the Review Focus and the Task 5 leak test police this. ADR-0014 needs no amendment.
- **TDD:** every task is RED (failing behavioral tests first) → verify-RED command → implement → verify-GREEN command → focused commit. Tests assert business invariants (one plan per version, immutable rows, itemized validation, byte-stable subscription), not HTTP trivia; the concurrency case proves distinct versions, the idempotency cases prove exactly-one-plan.
- **Interface consistency:** Task 1's payload/types feed Task 3 (adapters) and Task 4 (service validation); Task 2's store feeds Task 4; Task 4's service feeds Task 5's handlers; Task 6 exercises Tasks 3–4 against the real stack. The fake adapter remains the shared test double for contract, service and router tests.
- **Execution-detail scan:** every task names files, commands, expected results; migration numbers reserved with the dedup note (#78 presumed 000178/000099 — no #78 plan exists in the repo); the only Docker dependency is the optional tagged run; secrets env-only; container wiring avoids the documented Invoke-ordering trap; old gateway untouched.
- **Known risks:** (1) Lago plan `GET /api/v1/plans/{code}` single-object read-back is source-inferred (plans are path-addressed by code — T02's `DELETE /api/v1/plans/{code}` proves the addressing; the list endpoint is the lab-proven fallback — the integration test uses whichever the run answers, recorded honestly). (2) Entitlement feature ensure/attach adds adapter surface; if the map-form attach misbehaves on the pinned release the integration phase records it and the fallback is publishing plans without entitlement attach in T07 (entitlement runtime enforcement is WeKnora-local per spec; Lago attach completes in a follow-up) — the plan states this branch explicitly. (3) `PriceLadder` tier values are first-slice constants; a later ticket may externalize them — validation is already injection-based. (4) Real-Lago evidence may be `blocked-env` (honest, non-blocking). (5) #78 may land with a different migration number — renumbering is mechanical and self-contained.
