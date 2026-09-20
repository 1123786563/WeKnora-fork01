# Lago T06 — Tenant→Lago Customer Mapping (Billing Account) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** When a space first touches billing, WeKnora idempotently establishes the space's Billing Account — the immutable mapping from Tenant to exactly one Lago Customer — and exposes an understandable, provider-neutral account status through the Billing API. Concurrent initialization of the same space yields exactly one Customer; rename and owner transfer never change identity; lost responses recover through the original identity without creating a second Customer.

**Architecture:** The mapping is a WeKnora-owned Billing Projection in a NEW local table `commercial_billing_accounts` (migration `000178` versioned / `000099` sqlite). Customer creation flows through the frozen `CommercialPlatform` seam (ADR-0014) as the first enabled command kind `ensure_customer`, whose `Command.Key` is the idempotency identity `ensure_customer:<external_customer_id>`, where `external_customer_id` is a pure deterministic function of the Tenant ID (`weknora-tenant-<id>`, defined once in `internal/commercial`). The Lago adapter implements the command with read-before-create (`GET /api/v1/customers/{external_id}` then `POST /api/v1/customers` only when absent — the #73-proven payload `{customer:{external_id,name}}`), so a replay after a lost response resolves by identity and never issues a second create. Account status reads ride a NEW additive snapshot kind `account` (authority truth: `linked|absent`). A new `BillingAccountService` owns the lazy ensure-on-first-billing-access flow (DB unique constraint + `INSERT … ON CONFLICT DO NOTHING` following the `AccountStore.Bind` precedent), and `GET /api/v1/commercial/account` serves the closed product envelope `linked|pending` + closed reason tokens — never a Lago ID, URL, or raw error.

**Tech Stack:** Go 1.26 (`github.com/Tencent/WeKnora`), gin, gorm (SQLite in tests, PostgreSQL via tagged twin), `net/http` + `httptest`, golang-migrate versioned SQL twins, Python 3 unittest (regression), Docker Compose (optional real-Lago integration evidence, `//go:build lago_integration`).

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — "Commercial authority and module seam" (Tenant→one immutable Lago Customer; WeKnora Deployment→one Lago Organization), "Local persistence and state projection" (Customer local records are rebuildable Billing Projections; commands use stable idempotency identity; timeout/server error are indeterminate outcomes queried before replay), "Public product states" (no Lago identifiers in the Billing API); [ADR-0012](../adr/0012-lago-as-commercial-billing-authority.md); [ADR-0014](../adr/0014-commercial-platform-single-deep-seam.md) (additive-only seam growth; W3 extension contract in `docs/plans/2026-09-21-lago-t05-commercial-seam.md` Task 2); ticket #78; blocked-by #77 (merged as `6a651c300`).

## Global Constraints

- **Isolation:** all work on branch `lago-78-lago-customer` in `.worktrees/lago-78`. Any real-Lago run uses its own stack: `COMPOSE_PROJECT_NAME=weknora-lago-78`, `LAGO_API_PORT=48899`, `LAGO_FRONT_PORT=48900` (via `./deploy/lago/lago.sh`, read-only reuse of the #73 compose assets). Never reuse OpenMeter or another ticket's Compose project, volumes, or ports.
- **Seam freeze (additive only):** `internal/commercial/platform.go` method signatures and file location are frozen. This ticket adds ONLY: constants `CommandKindEnsureCustomer` / `SnapshotKindAccount`, typed payload `EnsureCustomerPayload`, optional `Snapshot.Account *AccountSnapshot` section field, closed `AccountState` enum, and the pure derivation `ExternalCustomerID(tenantID uint64) string`. NO signature changes, NO `Validate()` behavior change, NO per-object wrapper method (`GetCustomer`-style) — a violation is an ADR-0014 amendment, not an edit here. The shared contract table in `internal/infrastructure/commercialplatform/contract_test.go` is a test file and IS extended (see Task 2): the "any submit fails closed" case must switch its example kind from `ensure_customer` to a genuinely unknown literal once `ensure_customer` is enabled.
- **Deterministic identity:** `external_customer_id = "weknora-tenant-" + decimal tenantID`. It is derived from NOTHING else — never name, owner, or creation time — so rename (`UpdateTenant`) and owner transfer cannot move it. One derivation function, defined in `internal/commercial`, used by the service (Key/payload), both adapters (REST addressing), and tests. Uniqueness inside one Lago Organization is guaranteed by Tenant uniqueness (spec: one Deployment = one Organization); the `weknora-` prefix keeps lab/operator objects distinguishable.
- **Idempotency & concurrency:** exactly one Customer per space is enforced by three layers: (1) local DB — `tenant_id` PRIMARY KEY + `external_customer_id` UNIQUE with `INSERT … ON CONFLICT DO NOTHING` then re-read (the `AccountStore.Bind` precedent, `internal/application/repository/commercial/account.go`); (2) command — `Command.Key` stable across retries; (3) authority — deterministic `external_id` + adapter read-before-create, so a replay after ANY failure resolves by identity (Lago create-on-external_id upsert semantics per #74 research HELP but are not load-bearing — the adapter never POSTs when a GET finds the customer, so even non-upsert behavior cannot duplicate).
- **Recovery semantics:** `ErrPlatformUnreachable` (timeout, transport, 5xx) is an indeterminate outcome — the local row stays `pending`, nothing is rolled back, and the next ensure re-reads by identity before any create. Only a confirmed receipt or an authority `linked` snapshot moves the row to `linked`. A `pending` account never blocks non-commercial space functions (spec "Public product states").
- **Ensure-command semantics (US-59 note):** `ensure_customer` is an ensure/upsert command, not a create-with-content command: replay under the same Key with a different display name (space renamed) legitimately updates advisory metadata; identity is immutable. Content-conflict rejection (US-59) applies to create-semantics commands in later tickets, not to ensure.
- **Tenant isolation:** tenant scope comes exclusively from the authenticated context (`commercialTenantScope`); no tenant path parameter exists on any endpoint. Every store query is `WHERE tenant_id = ?`; the external id is derived from the authenticated tenant only, so no caller can address another space's Customer. The Billing API response carries no provider correlation identity (`CommandReceipt.ExternalID` stays inside the seam).
- **Provider neutrality:** `GET /api/v1/commercial/account` answers the closed envelope only — `state ∈ {linked, pending}`, `reason ∈ {"", unconfigured, unreachable, invalid_response, unsupported}` (empty iff linked), `ensured_at` RFC3339 (omitted when never ensured). No Lago URL, path, external id, `lago_id`, status enum, or `err.Error()` text ever crosses; adapter error strings carry only the sentinel + short closed description.
- **Secrets:** only the existing server-side `WEKNORA_COMMERCIAL_PLATFORM_*` env family (T05). No new secret, nothing committed, logged, or echoed.
- **Migration numbers (allocated, controller dedupe):** this ticket owns `migrations/versioned/000178_commercial_billing_accounts.*` (PostgreSQL) and `migrations/sqlite/000099_commercial_billing_accounts.*` (SQLite twin, same name suffix — the two tracks number independently; current maxima are 000177/000098). #79 must take `000179`/`000100`. If #78 lands after #79, renumber to the next free pair at integration time (the waves doc's historical three-lane collision at 000058 is the reason this is written down).
- **Trigger point choice (documented):** lazy ensure-on-first-billing-access, triggered by `GET /api/v1/commercial/account`. Rationale: (1) `tenantService.CreateTenant` (`internal/application/service/tenant.go`) must not depend on billing-authority availability — a Lago outage would otherwise break or asynchronously complicate space creation, contradicting the spec's outage posture; (2) spaces that never use billing never allocate authority objects; (3) the idempotent ensure makes adding more entry points (e.g. `Summary`) a one-line adoption in later tickets. Eager creation at tenant-create time is explicitly rejected for this slice.
- **Legacy coexistence:** the OpenMeter-era `commercial_accounts` table and `AccountStore` (`migration.go` / `saas-migrate`) are NOT touched — the new projection is a separate table with separate semantics (rebuildable projection, not the O02 migration binding). Old gateway registration (`container.go` line ~213) untouched until #105.
- **Trunk green:** `go build ./...` and the Go test suite pass at every commit; Python `deploy.lago` suites stay green. Real-Lago integration is separately tagged (`lago_integration`) and env-gated — Docker unavailability records `blocked-env`, never a failure or fake pass.

## Review Focus

- **Concurrent double-create:** two requests race the first billing access for one space — prove exactly one Customer identity is ever submitted (fake records distinct external ids: count must be 1), the local table holds exactly one row, and under real PostgreSQL the constraint decides the winner (tagged twin following `account_pg_test.go`). The adapter's read-before-create must also be proven at the stub level: GET-hit never issues a POST.
- **Cross-tenant leak:** authenticated as space A with space B's row injected — A's answer reflects only A; the response body never carries B's mapping, any external id, or any provider ref; the derivation function has no input through which a caller can name another tenant.
- **Unstable identity on rename/owner transfer:** re-ensure after `UpdateTenant` (new name) and after an owner change — external id byte-identical, exactly one Customer in the authority (upsert, not a second create), local mapping row unchanged.
- **Lost response → second Customer:** fault-injected fake creates the customer then fails the submit with `ErrPlatformUnreachable` (the #73 probe's persisted-but-response-lost case) — the retry recovers by identity (snapshot `linked` or re-submit resolving to the same customer), never a second object; local row goes `pending → linked`.
- **Provider leakage in the API:** force every failure class (nil platform, unconfigured, unreachable with marker-laden adapter text, invalid response) — the account envelope contains only the closed fields; marker/`lago`/`external_id` substring assertions on every body.
- **Seam-freeze creep:** reviewer checks `internal/commercial/platform.go` diff against ADR-0014 — additive constants/types/field only; rejects any new method or signature edit; confirms the contract-table change is a test-file extension, not a port change.

---

### Task 1: Seam extension — `ensure_customer` command kind, `account` snapshot kind, deterministic identity

**Files:**
- Modify: `internal/commercial/platform.go`
- Test: `internal/commercial/platform_test.go`

**Interfaces (all additive to the frozen port — no signature changes):**

```go
// internal/commercial/platform.go — additions
const CommandKindEnsureCustomer CommandKind = "ensure_customer"

// EnsureCustomerPayload is the typed payload of ensure_customer. TenantID is
// the WeKnora space; ExternalCustomerID is the deterministic immutable
// identity (ExternalCustomerID(t)); DisplayName is ADVISORY metadata (a
// rename updates it, never identity).
type EnsureCustomerPayload struct {
    TenantID            uint64
    ExternalCustomerID  string
    DisplayName         string
}

const SnapshotKindAccount SnapshotKind = "account"

// AccountState is the closed authority-truth enum for the account snapshot.
const (
    AccountStateLinked AccountState = "linked" // authority holds this tenant's customer
    AccountStateAbsent AccountState = "absent" // authority definitively holds none
)
type AccountState string

// AccountSnapshot is the authority-side account section (NOT the API state).
type AccountSnapshot struct {
    TenantID   uint64
    State      AccountState
    CheckedAt  time.Time
}

// Snapshot gains: Account *AccountSnapshot  // additive optional section

// ExternalCustomerID is THE deterministic WeKnora→authority customer
// identity: a pure function of the tenant ID only (no name, owner, or time).
// Defined once here; service, adapters, and tests all call it.
func ExternalCustomerID(tenantID uint64) string // "weknora-tenant-<decimal id>"
```

- [ ] **Step 1: Write failing tests (RED)**

`internal/commercial/platform_test.go` additions: (1) `ExternalCustomerID` is a pure function — same input ⇒ same output, format `weknora-tenant-<id>`, distinct tenants ⇒ distinct ids, charset is `[a-z0-9-]` only (the #73 probe-proven Lago external-id class); the function signature takes exactly one `uint64` (rename/owner cannot influence it — compile-enforced). (2) The new constants have the exact frozen token values (`"ensure_customer"`, `"account"`, `"linked"`, `"absent"`). (3) A `Command{Kind: CommandKindEnsureCustomer, Key: "ensure_customer:weknora-tenant-1", Payload: EnsureCustomerPayload{…}}` passes `Validate()` unchanged (Validate still only checks Kind/Key — do NOT extend it). (4) `Snapshot{Kind: SnapshotKindAccount, Account: &AccountSnapshot{…}}` round-trips the section field (zero-value `Snapshot` keeps `Account == nil` — readiness-only consumers are unaffected).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/commercial/ -run 'EnsureCustomer|AccountSnapshot|ExternalCustomerID' -v`

Expected: compile failure — the constants, types, and function do not exist.

- [ ] **Step 3: Implement the additive port growth**

Add exactly the declarations above to `internal/commercial/platform.go`, doc comments citing ADR-0014 (additive-kind rule) and the identity rule (stable across rename/owner transfer). Do not touch `Command`, `Command.Validate`, `SnapshotQuery`, the interface, or the sentinel errors. Adapters still fail closed on the new kinds after this task — the shared contract table's "any submit fails closed" case (`contract_test.go` uses the literal `CommandKind("ensure_customer")`) still passes as-is; it is rewritten in Task 2 when the kinds are enabled.

- [ ] **Step 4: Verify GREEN + regression**

Run: `go test ./internal/commercial/ ./internal/infrastructure/commercialplatform/ -v`

Expected: new tests pass; the T05 contract table and adapter suites stay green (both adapters still answer `ErrPlatformUnsupported` for the new kinds).

- [ ] **Step 5: Commit**

`feat(commercial): additive ensure_customer command and account snapshot kinds on the frozen seam (#78)`

### Task 2: Both adapters implement `ensure_customer` + `account` behind the shared contract table

**Files:**
- Modify: `internal/infrastructure/commercialplatform/fake.go`
- Modify: `internal/infrastructure/commercialplatform/lago.go`
- Modify: `internal/infrastructure/commercialplatform/contract_test.go`
- Test: `internal/infrastructure/commercialplatform/lago_test.go`

**Interfaces:**
- Consumes: Task 1 kinds + `ExternalCustomerID`; the #73 runtime-proven contract `POST /api/v1/customers` with `{"customer":{"external_id","name"}}` (echoed `customer.external_id`; API-key auth header) and `DELETE /api/v1/customers/{external_id}`.
- Produces: `SubmitCommand(ensure_customer)` → `CommandReceipt{Key: cmd.Key, ExternalID: <external_customer_id>, RecordedAt}` on both adapters; `ReadSnapshot(account, TenantID≠0)` → `Snapshot.Account.State ∈ {linked, absent}`; unchanged fail-closed behavior for every other kind, for `SubmitCommand` with an empty/`TenantID==0` payload guard rejected by the adapter as `ErrPlatformUnsupported`, and for `Reconcile`.
- Lago adapter command algorithm (read-before-create — the retry-safety core): validate payload (non-zero TenantID, non-empty ExternalCustomerID, equality with `ExternalCustomerID(payload.TenantID)` — a mismatched identity is refused `ErrPlatformUnsupported`, never silently forwarded); `GET {base}/api/v1/customers/{external_id}` → 200: receipt (no POST); 404: `POST {base}/api/v1/customers` with `{customer:{external_id, name: DisplayName}}` → 2xx: receipt; 404-on-GET and non-2xx-on-POST map to the existing sentinels (5xx/transport → `ErrPlatformUnreachable` — indeterminate, caller retries by identity; 4xx → `ErrPlatformInvalidResponse`). No response-body parsing, no URL/status text in errors, no credential in errors. `account` snapshot: same GET → 200 `linked`, 404 `absent`, sentinels otherwise; `TenantID==0` or an id inconsistent with the query's TenantID → `ErrPlatformUnsupported`.
- Fake adapter: records customers keyed by external id (mutex — it now mutates); `SubmitCommand(ensure_customer)` is idempotent per `Key` (replay returns the ORIGINAL receipt with unchanged `RecordedAt`); re-submit under a different Key with the same external id updates the single stored customer (upsert — never a second entry); exposes test knobs `FailSubmitsWith(error)` (fault injection for the recovery tests) and `Customers()` for identity assertions; `account` snapshot derives from stored customers (`linked`/`absent`); unprimed/absent stays honest — never fabricates `linked`.

- [ ] **Step 1: Write failing tests (RED)**

Extend `lago_test.go` with a `customersStub` (httptest, records method+path+body per request): (1) read-before-create — first submit issues `GET /api/v1/customers/weknora-tenant-42` (404) then `POST /api/v1/customers` (200), receipt `ExternalID == "weknora-tenant-42"`; (2) identity-present — GET 200 ⇒ receipt with NO POST issued (assert recorded request sequence); (3) POST 5xx / dropped connection ⇒ `errors.Is(err, commercial.ErrPlatformUnreachable)`; POST 404/422 ⇒ `ErrPlatformInvalidResponse`; (4) missing config ⇒ `ErrPlatformUnconfigured`; (5) payload guard — empty external id, or external id ≠ `ExternalCustomerID(tenantID)` ⇒ `ErrPlatformUnsupported` and the stub received zero requests; (6) `account` snapshot: 200 ⇒ `linked`, 404 ⇒ `absent`, transport ⇒ unreachable; (7) the API key appears ONLY in the `Authorization` header, never in any error string; (8) `Reconcile` and unknown command kinds still `ErrPlatformUnsupported`. Fake-side tests: idempotent replay (same receipt, one stored customer), upsert under new Key (one customer, name updated), fault knob surfaces the injected sentinel, `account` reflects store truth. Rewrite the shared table: the "any submit fails closed" case switches its kind to `CommandKind("no_such_kind")`; add per-adapter `ensure_customer` happy-path + `account` snapshot legs to `runPlatformContract` so BOTH adapters prove the identical contract.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: new cases fail — neither adapter implements the kinds; the rewritten table case fails against unimplemented behavior.

- [ ] **Step 3: Implement both adapters**

`lago.go`: add the command branch and snapshot branch per the algorithm above (one small `customers` HTTP helper; keep the existing closed error style). `fake.go`: add the customer store, idempotency map, and knobs. Keep T05 readiness behavior byte-identical.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/infrastructure/commercialplatform/ ./internal/commercial/ -v`

Expected: the extended shared contract table passes for BOTH the fake and the stub-backed Lago adapter; all T05 readiness/leak tests unchanged and green.

- [ ] **Step 5: Commit**

`feat(commercialplatform): ensure_customer and account snapshot on both adapters, shared contract extended (#78)`

### Task 3: Billing Account projection — migration, store, and the idempotent ensure service

**Files:**
- Create: `migrations/versioned/000178_commercial_billing_accounts.up.sql` / `.down.sql`
- Create: `migrations/sqlite/000099_commercial_billing_accounts.up.sql` / `.down.sql`
- Create: `internal/application/repository/commercial/billing_account.go`
- Create: `internal/application/repository/commercial/billing_account_test.go`
- Create: `internal/application/repository/commercial/billing_account_pg_test.go` (`//go:build commercial_integration`)
- Create: `internal/application/service/commercial/billing_account.go`
- Test: `internal/application/service/commercial/billing_account_test.go`

**Interfaces:**

```sql
-- 000178 (PG) / 000099 (sqlite twin): rebuildable Billing Projection
CREATE TABLE commercial_billing_accounts (
    tenant_id              BIGINT/INTEGER PRIMARY KEY,          -- one row per space
    external_customer_id   TEXT NOT NULL UNIQUE,                -- deterministic identity
    provider_customer_ref  TEXT NOT NULL DEFAULT '',            -- seam-internal receipt identity; NEVER crosses the API
    state                  TEXT NOT NULL DEFAULT 'pending',     -- pending | linked (projection-local)
    ensured_at             TIMESTAMPTZ/DATETIME NULL,           -- receipt/snapshot confirmed at
    created_at             / NOT NULL DEFAULT NOW()/CURRENT_TIMESTAMP,
    updated_at             / NOT NULL DEFAULT NOW()/CURRENT_TIMESTAMP
);
-- down: DROP TABLE IF EXISTS commercial_billing_accounts;
```

```go
// repository: BillingAccountStore (AccountStore.Bind precedent)
type BillingAccount struct { TenantID uint64; ExternalCustomerID string; ProviderCustomerRef string; State string; EnsuredAt *time.Time; ... }
func (s *BillingAccountStore) EnsurePending(ctx, a BillingAccount) (BillingAccount, error) // INSERT … ON CONFLICT(tenant_id) DO NOTHING + re-read; conflict on external_customer_id across tenants ⇒ typed ErrBillingAccountConflict
func (s *BillingAccountStore) MarkLinked(ctx, tenantID uint64, ref string, at time.Time) (BillingAccount, error) // idempotent update pending→linked (or linked→linked refresh)
func (s *BillingAccountStore) Get(ctx, tenantID uint64) (BillingAccount, error) // always WHERE tenant_id = ?

// service: BillingAccountService
type BillingAccountStatus struct { State string /* linked|pending */; Reason string /* closed token */; EnsuredAt *time.Time }
func NewBillingAccountService(db *gorm.DB, platform commercial.CommercialPlatform, opts...) (*BillingAccountService, error)
func (s *BillingAccountService) EnsureBillingAccount(ctx context.Context, tenantID uint64, displayName, actor string) (BillingAccountStatus, error)
```

`EnsureBillingAccount` flow (lazy ensure, recovery-safe): derive `extID := commercial.ExternalCustomerID(tenantID)`; `EnsurePending` (DB decides the single row under concurrency); if row `linked` → return `linked`. Else recovery-by-identity first: `ReadSnapshot(account, tenantID)` → `linked` ⇒ `MarkLinked` with the snapshot check time (the lost-response fast path — no submit at all) and return. Else `SubmitCommand(Command{Kind: ensure_customer, Key: "ensure_customer:"+extID, Actor: actor, Reason: "first_billing_access", Payload: EnsureCustomerPayload{TenantID, extID, DisplayName}})` → receipt ⇒ `MarkLinked(ref=receipt.ExternalID)`; `ErrPlatformUnconfigured/Unreachable/InvalidResponse/Unsupported` ⇒ row stays `pending`, return `BillingAccountStatus{State: pending, Reason: <closed token>}` (never an error to the caller for platform failures — the outage is the state, matching the readiness endpoint's posture). DB errors return error. The service takes tenantID from the caller's authenticated scope only; no API through which one tenant names another.

- [ ] **Step 1: Write failing tests (RED)**

Store tests (`billing_account_test.go`, `testAccountStore` harness — shared-cache SQLite, `SetMaxOpenConns(1)`, AutoMigrate the model): idempotent `EnsurePending`; competing same-tenant rows leave exactly one; two tenants CANNOT share one external id (typed conflict); `MarkLinked` pending→linked and linked→linked idempotent; `Get` scoped by tenant. PG twin (`billing_account_pg_test.go`, `//go:build commercial_integration`, `account_pg_test.go` pattern with a start-channel barrier): concurrent `EnsurePending` + `MarkLinked` races decide exactly one winner per tenant under real PostgreSQL — replacing the constraint must fail this test. Service tests (`billing_account_test.go` in the service package, fake adapter + SQLite): (1) first ensure → `linked`, one submit recorded, local row linked; (2) **AC1 concurrency** — barrier-started goroutines (8×) `EnsureBillingAccount(101, …)` ⇒ all return `linked`, fake `Customers()` has exactly ONE entry, its external id is `weknora-tenant-101`, one local row; (3) **AC3 identity stability** — ensure with name "Old", `UpdateTenant`-style rename to "New", re-ensure ⇒ same external id, fake still one customer (name updated), row unchanged apart from timestamps; the derivation ignores owner entirely (assert `ExternalCustomerID` unchanged for any owner — compile-level: no owner parameter exists); (4) **AC4 recovery** — fake `FailSubmitsWith(ErrPlatformUnreachable)` after persisting the create ⇒ first ensure returns `pending/unreachable`; clear the knob, re-ensure ⇒ snapshot read finds the customer, `MarkLinked`, NO second customer, status `linked`; (5) platform unconfigured ⇒ `pending/unconfigured`, no submit, row stays pending; (6) **AC2 isolation** — tenants 201/202 ensured concurrently ⇒ two rows, two distinct external ids, each `Get` returns its own; a store-level attempt to bind tenant 202 onto 201's external id ⇒ typed conflict.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/application/repository/commercial/ ./internal/application/service/commercial/ -run 'BillingAccount' -v`

Expected: compile failure — files do not exist.

- [ ] **Step 3: Implement migration, store, service**

Write the four migration files (PG twin + SQLite twin, header comments naming the projection semantics and the migration-number allocation; SQLite uses INTEGER/DATETIME/CURRENT_TIMESTAMP per the 000176↔000097 precedent). Store follows `AccountStore.Bind` exactly (ON CONFLICT DO NOTHING + re-read + conflict normalization; no external call inside a transaction). Service per the flow above; every platform failure maps to a closed reason token; log lines carry tenant id only — never the payload or provider text.

- [ ] **Step 4: Verify GREEN + migration check**

Run: `go test ./internal/application/repository/commercial/ ./internal/application/service/commercial/ -v && go test ./internal/database/ -v`

Expected: all new tests pass; existing commercial repo/service suites stay green; the SQLite migration test harness (migration_sqlite_versioned_schema_test.go) accepts the new twin.

- [ ] **Step 5: Commit**

`feat(commercial): billing account projection with idempotent tenant→customer ensure service (#78)`

### Task 4: Billing API — `GET /api/v1/commercial/account` with lazy ensure, wiring

**Files:**
- Modify: `internal/handler/commercial.go`
- Modify: `internal/router/routes_commercial.go`
- Modify: `internal/container/container.go`
- Create: `internal/router/commercial_account_route_test.go`
- Test: `internal/router/commercial_account_route_test.go`

**Interfaces:**
- Consumes: `BillingAccountService` (Task 3), the existing commercial route group guards, the `{success:true,data:…}` envelope, the `commercial_platform_route_test.go` harness (`authAs`/`decodeEnvelope`/`wantString`).
- Produces: `CommercialHandler.SetBillingAccountService(*commercialsvc.BillingAccountService)`; `GET /api/v1/commercial/account` → `200 {"success":true,"data":{"state":"linked","ensured_at":"…","reason":""}}` or `{"state":"pending","reason":"unconfigured|unreachable|invalid_response|unsupported"}` (`ensured_at` omitted when never ensured). Nil service ⇒ honest `pending/unconfigured` (fail closed, endpoint never 500s for wiring gaps). Container: `must(container.Provide(commercialsvc.NewBillingAccountService))` + an `Invoke` calling `SetBillingAccountService`, placed immediately after the T05 platform Invoke (~line 777) with the ordering-trap comment preserved (away from the pre-craft Invoke block).

- [ ] **Step 1: Write failing router tests (RED)**

Cases (`commercial_account_route_test.go`, sqlite in-memory engine + fake adapter): (1) authenticated member GET, unlinked space ⇒ 200 `linked` + `ensured_at`, fake holds exactly one customer `weknora-tenant-101`, local row created (lazy ensure proven at the API seam); (2) repeat GET ⇒ still one customer (idempotent at the boundary); (3) **AC2 at the API** — engine with two tenants authed in turn, tenant A's body never contains `weknora-tenant-102` and vice versa; (4) fail-closed ladder — nil service ⇒ `pending/unconfigured`; unconfigured fake ⇒ same; fake `FailSubmitsWith(ErrPlatformUnreachable)` ⇒ `pending/unreachable`; **provider-leak**: a marker-laden platform (`BaseURL: http://lago-marker-48899.invalid` real adapter, plus the leakyPlatform pattern) on every failure class ⇒ body contains no `lago`, no marker, no `/api/v1/customers`, no `external_id` substring, no `weknora-tenant-` (identity stays internal), and `err.Error()` is never echoed; (5) auth — anonymous 401 at the auth layer with the platform never read; full-access API key without the explicit commercial capability ⇒ 403; plain member GET passes the write gate by design (read); (6) response field set is EXACTLY the closed envelope (no extra fields, `ensured_at` omitted when pending-never-ensured).

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/router/ -run TestCommercialAccount -v`

Expected: route and handler method do not exist; tests fail.

- [ ] **Step 3: Implement handler, route, wiring**

`AccountStatus` handler: tenant from `commercialTenantScope` (403 `missing_tenant_scope` when absent); nil service ⇒ closed `pending/unconfigured`; `EnsureBillingAccount(ctx, tenant, tenantDisplayName, actor=userID)`; DB error ⇒ 500 with a generic error string (no provider text); success ⇒ closed envelope. DisplayName: read the tenant name via the handler's `db` (`SELECT name FROM tenants WHERE id = ?`) — advisory only; a missing name degrades to `fmt.Sprintf("WeKnora Space %d", tenantID)`, never an error. Register `commercialGroup.GET("/account", commercialHandler.AccountStatus)` beside `/platform/readiness`. Container wiring per the interface notes.

- [ ] **Step 4: Verify GREEN + regression**

Run: `go test ./internal/router/ ./internal/handler/ ./internal/container/ ./internal/commercial/ ./internal/infrastructure/... ./internal/application/... -v && go build ./...`

Expected: new tests pass; T05 readiness/scope suites, old commercial suites, and openmeter paths stay green; container boot unaffected.

- [ ] **Step 5: Commit**

`feat(billing): provider-neutral account status endpoint with lazy customer ensure (#78)`

### Task 5: Real-Lago integration evidence, ledger, final verification

**Files:**
- Create: `internal/infrastructure/commercialplatform/lago_customer_integration_test.go` (`//go:build lago_integration`)
- Create: `deploy/lago/evidence/t06-account.txt` (run transcript, if the stack runs)
- Modify: ticket #78 acceptance checkboxes + `docs/plans/ledgers/lago-78.md` (implementation ledger)

**Interfaces:**
- Consumes: Tasks 1–4; the #73 operator lifecycle `./deploy/lago/lago.sh init|up|status|down` with the #78 stack env; `LAGO_INTEGRATION_BASE_URL` + `LAGO_INTEGRATION_API_KEY` (operator-owned, never committed); `LAGO_INTEGRATION_TENANT_ID` (default `780001`, a lab-only tenant id that cannot collide with product spaces in a shared org).
- Produces: tagged proof against real pinned Lago `v1.53.0` that ensure_customer is idempotent-by-identity on the real authority, plus the ticket AC mapping.

- [ ] **Step 1: Write the tagged integration test**

`lago_customer_integration_test.go` (env-gated skip ⇒ `blocked-env`, never a fake pass): (1) `SubmitCommand(ensure_customer, Key="ensure_customer:"+ext, ext=ExternalCustomerID(LAGO_INTEGRATION_TENANT_ID))` ⇒ receipt with the requested identity; (2) `ReadSnapshot(account, tenant)` ⇒ `linked`; (3) replay the SAME Key ⇒ receipt identity unchanged; re-submit under a NEW Key with the same external id (the upsert probe — #74 research says create is upsert-on-external_id; this records the RUNTIME verdict either way) ⇒ still one customer: `ReadSnapshot(account)` ⇒ `linked` and a direct `GET /api/v1/customers/{ext}` shows one object with the expected external_id; (4) rename probe: re-ensure with a different display name ⇒ snapshot still `linked`, identity unchanged; (5) lenient cleanup `DELETE /api/v1/customers/{ext}` (404 counts as absent — #73 carryover rule); (6) a dropped-response simulation is NOT possible against the shared stack without orphans — the recovery path is proven by the Task 2/3 fault-injection tests, and this test documents that split in a comment.

- [ ] **Step 2: Run the real stack and collect evidence**

Run: `COMPOSE_PROJECT_NAME=weknora-lago-78 LAGO_API_PORT=48899 LAGO_FRONT_PORT=48900 ./deploy/lago/lago.sh init && COMPOSE_PROJECT_NAME=weknora-lago-78 LAGO_API_PORT=48899 LAGO_FRONT_PORT=48900 ./deploy/lago/lago.sh up`

Run: `COMPOSE_PROJECT_NAME=weknora-lago-78 LAGO_API_PORT=48899 LAGO_FRONT_PORT=48900 ./deploy/lago/lago.sh status --json`

Run: `LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48899 LAGO_INTEGRATION_API_KEY=<operator key> go test -tags lago_integration ./internal/infrastructure/commercialplatform/ -run TestLagoCustomerIntegration -v`

Run: `COMPOSE_PROJECT_NAME=weknora-lago-78 ./deploy/lago/lago.sh down`

Expected: ensure → linked → replay/identity-stable → cleanup all pass against the pinned `v1.53.0`. If Docker or an operator key is unavailable: record `blocked-env` in the evidence note and ledger, rely on the stub contract + fault-injection evidence, and claim nothing unproven.

- [ ] **Step 3: Final verification sweep**

Run: `go build ./... && go test ./...` and `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe -v`

Expected: trunk green — T05 seam suites, both adapter legs of the shared contract, projection/store/service/router suites, old gateway and commercial domain suites, and the deploy suites all pass.

- [ ] **Step 4: Commit, tick acceptance boxes, update ledger**

Commit: `test(lago): T06 real-stack customer identity evidence and final verification (#78)`. Tick the ticket's four acceptance boxes with pointers: AC1 → Task 3 Step 1 service concurrency test + Task 3 PG twin + Task 5 replay probe; AC2 → Task 3 isolation tests + Task 4 router cross-tenant case; AC3 → Task 1 derivation test + Task 3 rename test + Task 5 rename probe; AC4 → Task 2 stub recovery cases + Task 3 fault-injection recovery test + Task 5 identity-replay probe. Update `docs/plans/ledgers/lago-78.md` with the evidence inventory.

## Plan Self-Review

- **Spec coverage / acceptance mapping:** AC1 (并发只创建一个) → deterministic identity + three idempotency layers (Global Constraints), proven by Task 3 concurrency tests, the tagged PG twin, and Task 5's real-stack replay; AC2 (租户隔离) → context-derived scope + derivation purity + `WHERE tenant_id` stores + Task 4 API-level cross-tenant test; AC3 (身份稳定) → single pure derivation function (compile-enforced inputs), Task 3 rename test, Task 5 rename probe; AC4 (失败恢复) → adapter read-before-create (Task 2) + service snapshot-recovery fast path + pending-state retention (Task 3) + fault-injection test simulating persisted-but-lost responses. Spec sections implemented: Tenant↔Customer immutability, rebuildable Billing Projection with a real migration, command idempotency identity (`Command.Key`), indeterminate-outcome-queried-before-replay, closed product states with no provider leakage.
- **Seam freeze respected:** every port change is an additive constant/type/optional-field — exactly the W3 extension contract T05 Task 2 reserved ("W3 adds CommandKind constants additively (e.g. ensure_customer)"); no signature, `Validate`, sentinel, or per-object-method change; the shared contract table change is a test-file extension with an explicit unknown-kind substitution so the fail-closed default stays asserted.
- **Execution detail scan:** every task names files, commands, and expected outcomes; migration numbers allocated (000178/000099) with the #79 dedupe note; secrets stay in the existing env family; the only Docker dependency is the optional tagged run (blocked-env is legal); container placement honors the documented Invoke-ordering trap; the legacy `commercial_accounts`/OpenMeter paths are untouched (verified by regression sweeps).
- **Interface consistency:** Task 1's kinds/payload/derivation are consumed by Task 2 (adapters), Task 3 (service Key/payload/recovery snapshot), Task 4 (handler envelope), and Task 5 (integration probes); the fake's fault knobs introduced in Task 2 are the same ones Task 3's recovery tests drive; the router harness reuses the T05 helpers.
- **TDD:** each task writes failing behavioral tests first (port purity/constants, stub-recorded request sequences, store/constraint races, service invariants over a real SQLite DB, router closed-envelope/leak/auth cases), then implements, then verifies GREEN before a focused commit.
- **Known risks:** (1) Lago create-on-external_id upsert is research-carried (#74) and runtime-unproven until Task 5 — mitigated by design (read-before-create makes correctness independent of upsert) and Task 5 records the runtime verdict either way; (2) `GET /api/v1/customers/{external_id}` by external id is source/standard-contract knowledge not yet runtime-proven in-repo — Task 5 Step 1(3) exercises it directly, and a 404-mapping surprise would surface there, not in production; (3) SQLite shared-cache serializes writers (`SetMaxOpenConns(1)` harness note), so the true concurrency verdict rests on the PG twin — recorded as commercial_integration-tagged, blocked-env-legal; (4) lazy-ensure on a GET is a deliberate, documented side effect (idempotent, authority-failure-safe) — if review prefers an explicit POST later, the service seam already supports adding it additively; (5) `degraded`-style partial states are out of scope for the account slice — `pending` + closed reason tokens cover every failure class this ticket can observe.
