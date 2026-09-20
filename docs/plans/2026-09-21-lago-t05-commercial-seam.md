# Lago T05 — Commercial Platform Seam (Readiness Slice) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Freeze one deep provider-neutral Commercial Platform seam (commercial commands, commercial snapshots, reconciliation cursors) and ship the first product slice through it: one protected, read-only WeKnora Billing API operation that returns the billing authority's readiness/version snapshot. The old OpenMeter gateway keeps working untouched.

**Architecture:** The frozen Go port lives in `internal/commercial/platform.go` (package `commercial`), following the repo's existing port convention (`CommercialGateway` is declared in `internal/commercial/fulfillment.go` and implemented by `internal/infrastructure/openmeter`). Two adapters live behind it in a new `internal/infrastructure/commercialplatform/` package: a deterministic fake (tests/dev) and a Lago adapter that reads real Lago `/health` with server-side env config and fails closed. `internal/handler/commercial.go` gains a `PlatformReadiness` operation registered on the existing `/api/v1/commercial` group; `internal/container/container.go` selects the adapter from env. No per-object gateway wrappers are added — later tickets extend the seam by additive `CommandKind`/`SnapshotKind` values, not by new per-object methods.

**Tech Stack:** Go 1.26 (`github.com/Tencent/WeKnora`), gin, `net/http` + `httptest`, Python 3 unittest (probe fix), Docker Compose (optional real-Lago integration evidence only).

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — section "Commercial authority and module seam" (this ticket implements it verbatim); [ADR-0012](../../adr/0012-lago-as-commercial-billing-authority.md); ticket #77; blocked-by #73 (done, merged as `6d3ddc65`).

## Global Constraints

- **Isolation:** all work on branch `lago-77-commercial-seam` in `.worktrees/lago-77`. Any real-Lago run uses its own stack: `COMPOSE_PROJECT_NAME=weknora-lago-77`, `LAGO_API_PORT=48897`, `LAGO_FRONT_PORT=48898`. Never reuse OpenMeter or another ticket's Compose project, volumes, or ports.
- **Interface freeze (W3 prerequisite):** after Task 2 lands, the three method signatures in `internal/commercial/platform.go` are frozen for #78/#79. Additive growth only (new `CommandKind`/`SnapshotKind` constants, new typed payloads, new optional struct fields); changing a signature or adding a per-object method requires an ADR amendment and a W3 re-plan, not a silent edit.
- **One deep seam, no per-object wrappers:** the interface exposes exactly three operation families — `SubmitCommand` (typed commercial commands), `ReadSnapshot` (authoritative commercial snapshots; readiness is the first kind), `Reconcile` (cursor-based reconciliation). No `GetCustomer`/`GetWallet`/`GetInvoice`-style methods, ever (ADR-0012, spec "Commercial authority and module seam").
- **Provider neutrality:** the public Billing API response contains only WeKnora product vocabulary and a closed token set. No Lago URLs, object shapes, external IDs, raw status enums, response bodies, or error strings — including when the adapter fails. Provider correlation identities may exist inside the seam (`CommandReceipt.ExternalID`) but never cross the Billing API. The readiness snapshot's `release` value is deployment-pinned config (the #73 lock fact), never text parsed from a provider response.
- **Secrets:** Lago credentials come only from server-side env (`WEKNORA_COMMERCIAL_PLATFORM_API_KEY` and friends, following `internal/payment/providers_env.go` and `internal/infrastructure/openmeter/commercial.go` precedent). Nothing committed, logged, or echoed in responses.
- **Fail closed:** the Lago adapter never fabricates readiness. Missing config → typed `ErrPlatformUnconfigured`; unreachable/timeout/non-2xx → `ErrPlatformUnreachable`; unsupported command kinds / snapshot kinds / reconcile streams → `ErrPlatformUnsupported`. Command and reconcile families are frozen but NOT enabled in T05 — both adapters fail closed on them.
- **Old gateway untouched:** `internal/infrastructure/openmeter/**` and its container registration (`container.go` line ~212, `ommeter.NewGatewayFromEnv, dig.As(new(domain.CommercialGateway))`) are not modified. Both seams coexist until the #105 removal ticket. The new env family is `WEKNORA_COMMERCIAL_PLATFORM_*`, distinct from the old `WEKNORA_COMMERCIAL_GATEWAY_*`.
- **No duplication of the commercial domain:** `internal/commercial/` quote/order/budget/settlement/refund/usage types are reused as-is where needed and otherwise not touched. Readiness is platform operational state, not a commercial domain object; only the port file is added to `internal/commercial/`. No DB migration is needed (read-only slice; the waves doc reserved segment 000060+ for #77 if ever required — unused here).
- **Trunk green:** `go build ./...` and the Go test suite must pass at every commit; Python probe tests stay green. Real-Lago integration is separately tagged and optional — Docker unavailability is recorded as blocked-env, never as a failure or a fake pass.

## Review Focus

- **Provider leakage in the API response:** `GET /api/v1/commercial/platform/readiness` must answer with the closed envelope only. Test with an adapter whose base URL contains a marker string and force every failure class; assert the marker, any `lago` path/URL substring, and any raw error text are absent from the body.
- **Per-object wrapper creep:** reviewer checks `internal/commercial/platform.go` against the freeze — exactly three methods, kinds-not-methods growth; rejects any PR (here or in W3) that adds a shallow per-object wrapper.
- **Fake/Lago contract divergence:** both adapters must run the SAME contract suite (one table in `internal/infrastructure/commercialplatform/contract_test.go`); a behavior only one adapter has is a defect.
- **Auth gap:** the endpoint must sit inside the existing `RequireExplicitCommercialCapability` + `RequireManageBillingForWrites` group; tests prove anonymous is rejected at the auth layer and an API key without the explicit commercial capability gets 403 (GET bypasses the billing-role gate by design — it is a read).
- **Old-gateway regression:** old OpenMeter tests and the container boot path stay green; the new provider registration must not disturb the Invoke-ordering trap documented around `container.go` line ~207 (register near `handler.NewCommercialHandler`, not near the craft runtime).

---

### Task 1: Carryover — contract probe lenient cleanup after a failed create (#73 deferred fix)

**Files:**
- Modify: `deploy/lago/contract_probe.py`
- Test: `deploy/lago/test_contract_probe.py`

**Interfaces:**
- Consumes: the existing `run_probe(base_url, api_key)` report shape (`health`/`create`/`cleanup` step records, `pass | fail | blocked-env`).
- Produces: after ANY create-step failure (`rejected`/`error`/`malformed`), the probe still attempts a lenient `DELETE /api/v1/customers/{requested_external_id}`; HTTP 404 counts as a successful cleanup (`outcome: "absent"`), so no synthetic customer can be orphaned when the server persisted it but the response was lost or unreadable.

- [ ] **Step 1: Write failing behavior tests (RED)**

Extend the `http.server` fixture in `test_contract_probe.py`:

```python
def test_create_transport_failure_still_deletes_requested_id(server):
    # Server persists the customer, then drops the connection (indeterminate outcome).
    server.persist_then_drop_create = True
    result = run_probe(server.url, api_key="secret-for-test-only")
    assert result.status == "fail"
    assert server.deleted_external_ids == [server.last_requested_external_id]

def test_create_malformed_response_still_deletes_requested_id(server):
    server.create_customer_response = "not-json{"
    result = run_probe(server.url, api_key="secret-for-test-only")
    assert result.status == "fail"
    assert server.deleted_external_ids == [server.last_requested_external_id]

def test_lenient_cleanup_treats_404_as_absent_not_failed(server):
    server.fail_create_with_status = 404          # nothing was created
    server.delete_responds_404 = True             # lenient delete also 404s
    result = run_probe(server.url, api_key="secret-for-test-only")
    assert result.status == "fail"                # the create itself failed
    assert json.loads(result.serialized())["cleanup"]["outcome"] == "absent"
```

Also assert: a `health`-step failure (before any create was attempted) still performs NO delete; the API key never appears in `result.serialized()`; the success path still deletes the echoed `created_external_id`.

- [ ] **Step 2: Verify RED**

Run: `python3 -m unittest deploy.lago.test_contract_probe -v`

Expected: the new cases fail — today `cleanup` only runs when `created_external_id is not None`, so a failed create (transport/malformed) attempts no delete and 404 cleanups are classified `failed`.

- [ ] **Step 3: Implement the lenient cleanup**

In `run_probe`, track whether the create step was ATTEMPTED. In the `finally`, delete `created_external_id` when present (unchanged); otherwise, if create was attempted and failed, delete the REQUESTED `external_id`. Teach `_delete_customer` to classify HTTP 404 as `{"outcome": "absent", ...}` (a provable miss is a clean end state). Keep `blocked-env` (missing key) and health-failure paths delete-free. Sanitization rules unchanged.

- [ ] **Step 4: Verify GREEN**

Run: `python3 -m unittest deploy.lago.test_contract_probe deploy.lago.test_health deploy.lago.test_compose -v`

Expected: all pass — the #73 suite plus the carryover cases.

- [ ] **Step 5: Commit**

`fix(lago): probe lenient-cleans the requested customer after a failed create (#73 carryover)`

### Task 2: Freeze the seam port and ship the fake adapter

**Files:**
- Create: `internal/commercial/platform.go`
- Create: `internal/infrastructure/commercialplatform/fake.go`
- Create: `internal/infrastructure/commercialplatform/contract_test.go`
- Test: `internal/commercial/platform_test.go`, `internal/infrastructure/commercialplatform/contract_test.go`

**Interfaces (THE FREEZE — #78/#79 depend on these exact signatures):**

```go
// internal/commercial/platform.go — package commercial
type CommandKind string  // T05 defines none; W3 adds constants additively (e.g. ensure_customer, publish_plan_version)
type Command struct { Kind CommandKind; Key string; Actor string; Reason string; Payload any } // Key = idempotency identity
func (c Command) Validate() error // rejects empty Kind/Key; payload typing lands with W3 kinds

type SnapshotKind string
const SnapshotKindReadiness SnapshotKind = "readiness"
type SnapshotQuery struct { Kind SnapshotKind; TenantID uint64 } // TenantID 0 = platform-wide (readiness)

type ReadinessState string
const ( ReadinessReady ReadinessState = "ready"; ReadinessDegraded ReadinessState = "degraded"; ReadinessUnavailable ReadinessState = "unavailable" )
type ReadinessSnapshot struct { State ReadinessState; Release string; CheckedAt time.Time; Reason string } // Reason = closed token; Release = deployment-pinned config, never provider response text
type Snapshot struct { Kind SnapshotKind; Readiness *ReadinessSnapshot } // later kinds add sections additively

type ReconciliationCursor struct { Stream string; Value string } // opaque durable token
type CommercialChange struct { Kind string; TenantID uint64; ExternalID string; At time.Time }
type ReconciliationPage struct { Changes []CommercialChange; Next ReconciliationCursor; HasMore bool }

type CommandReceipt struct { Key string; ExternalID string; RecordedAt time.Time } // seam-internal correlation; never crosses the Billing API

var ( ErrPlatformUnsupported; ErrPlatformUnconfigured; ErrPlatformUnreachable; ErrPlatformInvalidResponse ) // sentinel errors, provider-neutral

type CommercialPlatform interface {
	SubmitCommand(ctx context.Context, cmd Command) (CommandReceipt, error)
	ReadSnapshot(ctx context.Context, query SnapshotQuery) (Snapshot, error)
	Reconcile(ctx context.Context, from ReconciliationCursor) (ReconciliationPage, error)
}
```

Fake adapter: `NewFakeAdapter() *FakeAdapter` with `SetReadiness(ReadinessSnapshot)`; `SubmitCommand`/`Reconcile` and unknown snapshot kinds return `ErrPlatformUnsupported`.

- [ ] **Step 1: Write failing tests (RED)**

`internal/commercial/platform_test.go`: `Command.Validate` rejects empty Kind/Key and accepts a well-formed command; readiness state constants are the exact three product states. `internal/infrastructure/commercialplatform/contract_test.go`: one shared table-driven suite `runPlatformContract(t, name string, p commercial.CommercialPlatform, wantReady func() bool)` asserting — readiness snapshot has `Kind == readiness`, a state from the closed enum, non-zero `CheckedAt`, and empty `Reason` when ready; unknown snapshot kind, any `SubmitCommand`, any `Reconcile` all fail closed with `errors.Is(..., commercial.ErrPlatformUnsupported)`. Register only the fake leg in this task.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/commercial/ ./internal/infrastructure/commercialplatform/`

Expected: compile failure / failing assertions — neither package exists yet.

- [ ] **Step 3: Implement the port and the fake**

Write `platform.go` exactly as frozen above (doc comments citing ADR-0012 and the no-per-object-wrapper rule) and `fake.go` (deterministic, mutex-free reads are fine for tests; `SetReadiness` stores, `ReadSnapshot` copies back verbatim). No provider concepts appear in the port file — no "lago" identifier, no HTTP types.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/commercial/ ./internal/infrastructure/commercialplatform/ -v`

Expected: contract suite passes for the fake; `Command.Validate` passes.

- [ ] **Step 5: Commit**

`feat(commercial): freeze CommercialPlatform seam port and fake adapter (#77)`

### Task 3: Lago adapter behind the same contract

**Files:**
- Create: `internal/infrastructure/commercialplatform/config.go`
- Create: `internal/infrastructure/commercialplatform/lago.go`
- Create: `internal/infrastructure/commercialplatform/lago_test.go`
- Modify: `internal/infrastructure/commercialplatform/contract_test.go`

**Interfaces:**
- Consumes: the frozen `commercial.CommercialPlatform`; env `WEKNORA_COMMERCIAL_PLATFORM_PROVIDER` (`""`/`lago` = Lago adapter, `fake` = fake adapter, anything else = construction error), `WEKNORA_COMMERCIAL_PLATFORM_URL`, `WEKNORA_COMMERCIAL_PLATFORM_API_KEY`, `WEKNORA_COMMERCIAL_PLATFORM_RELEASE` (pinned release string, e.g. `v1.53.0`, surfaced as the snapshot's `Release`).
- Produces: `Config`/`ConfigFromEnv()`/`NewPlatformFromEnv() (commercial.CommercialPlatform, error)` (adapter selection, `providers_env.go` pattern); `NewLagoAdapter(Config) *LagoAdapter` — construction succeeds unconfigured (blocked-env stays legal, openmeter precedent), calls fail fast.

- [ ] **Step 1: Write failing tests (RED)**

`lago_test.go` with an `httptest.Server` Lago stub: `GET /health` 200 → snapshot `State: ready`, `Release` equal to the configured string, request went to `<base>/health` (the #73 probe fact: `/health` is an unauthenticated liveness signal; version identity is deployment config, NOT parsed from the response). `/health` 500, refused connection, and expired context → `errors.Is(err, commercial.ErrPlatformUnreachable)` — never a fabricated `ready`. Missing `URL` or `APIKey` → `ErrPlatformUnconfigured`. API key never appears in any error string. Add the Lago-adapter leg (stub-backed) to the shared contract table — acceptance criterion "fake 与 Lago adapter 通过同一接口契约" is proven here.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/infrastructure/commercialplatform/`

Expected: `lago.go`/`config.go` do not exist; tests fail to compile.

- [ ] **Step 3: Implement the adapter and selection**

`LagoAdapter` holds `Config` + `*http.Client` (timeout default 5s, context honored). `ReadSnapshot(readiness)` → `configured()` check, then `GET {BaseURL}/health` with a short deadline; 2xx → `ready` snapshot with configured `Release` and `CheckedAt`; transport/timeout/5xx → wrapped `ErrPlatformUnreachable`; 4xx → `ErrPlatformInvalidResponse` (a definitive wrong answer, not a retry). `SubmitCommand`/`Reconcile`/unknown kinds → `ErrPlatformUnsupported`. No response-body parsing, no URL or status text inside errors returned to callers beyond the typed sentinel. `NewPlatformFromEnv` selects fake/lago; unknown provider values return an error at startup (no silent fallback).

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/infrastructure/commercialplatform/ -v`

Expected: lago tests and the two-adapter contract table pass.

- [ ] **Step 5: Commit**

`feat(commercialplatform): Lago adapter behind the frozen seam contract (#77)`

### Task 4: Protected readiness endpoint, route registration, container wiring

**Files:**
- Modify: `internal/handler/commercial.go`
- Modify: `internal/router/routes_commercial.go`
- Modify: `internal/container/container.go`
- Create: `internal/router/commercial_platform_route_test.go`
- Test: `internal/router/commercial_platform_route_test.go`

**Interfaces:**
- Consumes: `commercial.CommercialPlatform` (Task 2/3), the existing commercial route group middlewares, the repo `{success:true,data:...}` envelope.
- Produces: `CommercialHandler.SetCommercialPlatform(p commercial.CommercialPlatform)`; `GET /api/v1/commercial/platform/readiness` → `200 {"success":true,"data":{"state":"ready|degraded|unavailable","release":"v1.53.0","checked_at":"...","reason":""}}`. `reason` is a closed token (`unconfigured|unreachable|invalid_response|unsupported`), empty when `ready`; `release` omitted when unknown.

- [ ] **Step 1: Write failing router tests (RED)**

Follow the `commercial_scope_test.go` harness (gin engine + `RegisterCommercialRoutes(v1, h)` + injected auth middleware). Cases: (1) authenticated caller with a ready fake → 200 envelope, exact closed field set, `state == "ready"`; (2) fake returning `degraded`/`unavailable`/`ErrPlatformUnreachable` → 200 with the mapped token; (3) nil platform → 200 `state:"unavailable", reason:"unconfigured"` (fail closed, endpoint honest); (4) anonymous request blocked by the auth middleware; API key without the explicit commercial capability → 403 (existing `RequireExplicitCommercialCapability`); (5) **provider-leak test**: Lago adapter configured with base URL `http://lago-marker-48897.invalid` and a fake whose error path carries Lago-ish text — force `unreachable` and assert the response body contains neither the marker, nor `/health`, nor any `lago` substring; (6) GET is not billing-write-gated: a plain member (read) passes the write gate by design.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/router/ -run TestCommercialPlatform -v`

Expected: route and handler method do not exist; tests fail.

- [ ] **Step 3: Implement handler, route, wiring**

`PlatformReadiness`: nil platform or `ErrPlatformUnconfigured` → `unconfigured`; `ErrPlatformUnreachable` → `unreachable`; `ErrPlatformInvalidResponse` → `invalid_response`; `ErrPlatformUnsupported`/other → `unsupported`; snapshot success → state/release/checked_at (release omitted when empty). Never echo `err.Error()`. Register `commercialGroup.GET("/platform/readiness", commercialHandler.PlatformReadiness)` inside the existing guarded group (gates apply; GET skips the billing-role gate by the middleware's existing read-method branch). In `container.go`, next to `handler.NewCommercialHandler` (~line 762): `must(container.Provide(commercialplatform.NewPlatformFromEnv, dig.As(new(domain.CommercialPlatform))))` and an `Invoke` calling `SetCommercialPlatform`. Add the ordering comment (kept away from the pre-craft-Invoke provider block on purpose). OpenMeter registration untouched.

- [ ] **Step 4: Verify GREEN + regression**

Run: `go test ./internal/router/ ./internal/handler/ ./internal/container/ ./internal/commercial/ ./internal/infrastructure/... -v` and `go build ./...`

Expected: new tests pass; the old commercial/router/openmeter suites stay green (old gateway path unaffected).

- [ ] **Step 5: Commit**

`feat(billing): protected provider-neutral platform readiness endpoint (#77)`

### Task 5: ADR-0014, real-Lago integration evidence, final verification

**Files:**
- Create: `docs/adr/0014-commercial-platform-single-deep-seam.md`
- Create: `internal/infrastructure/commercialplatform/lago_integration_test.go`
- Create: `deploy/lago/evidence/` note for T05 evidence (if the stack runs)

**Interfaces:**
- Consumes: Tasks 2–4; the #73 operator lifecycle (`./deploy/lago/lago.sh init|up|status|down`) with the T05 stack env.
- Produces: the recorded seam decision + optional tagged integration proof against real Lago `v1.53.0`.

- [ ] **Step 1: Write the ADR (decision record)**

ADR-0014, status accepted: one deep Commercial Platform seam with three families; port frozen at `internal/commercial/platform.go`, adapters at `internal/infrastructure/commercialplatform/` (port-in-domain follows the `CommercialGateway` convention and keeps handler→domain dependency direction); growth is additive kinds, never per-object wrappers; old OpenMeter gateway coexists unchanged until #105; adapter selection via `WEKNORA_COMMERCIAL_PLATFORM_*` env, fail closed. Considered options: per-object wrapper gateway; interface living inside the infrastructure package; extending the legacy `CommercialGateway` (all rejected — creep / inverted dependency / layers a second seam).

- [ ] **Step 2: Write the tagged integration test (RED without env, GREEN with stack)**

`lago_integration_test.go` with `//go:build lago_integration`, env-gated on `LAGO_INTEGRATION_BASE_URL` + `LAGO_INTEGRATION_API_KEY` (skip otherwise): build the adapter, read readiness, assert `state == ready` and `Release ==` the `release` in `deploy/lago/images.lock.json` (`v1.53.0`). Without the build tag the normal suite never runs it (unit tests stay Docker-free).

- [ ] **Step 3: Run the real stack and collect evidence**

Run: `COMPOSE_PROJECT_NAME=weknora-lago-77 LAGO_API_PORT=48897 LAGO_FRONT_PORT=48898 ./deploy/lago/lago.sh init && COMPOSE_PROJECT_NAME=weknora-lago-77 LAGO_API_PORT=48897 LAGO_FRONT_PORT=48898 ./deploy/lago/lago.sh up`

Run: `COMPOSE_PROJECT_NAME=weknora-lago-77 LAGO_API_PORT=48897 LAGO_FRONT_PORT=48898 ./deploy/lago/lago.sh status --json`

Run: `LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48897 LAGO_INTEGRATION_API_KEY=... go test -tags lago_integration ./internal/infrastructure/commercialplatform/ -run TestLagoAdapterIntegration -v`

Run: `COMPOSE_PROJECT_NAME=weknora-lago-77 ./deploy/lago/lago.sh down`

Expected: adapter reports `ready` with release `v1.53.0` against the real pinned stack. If Docker or an operator API key is unavailable, record `blocked-env` in the ledger/evidence note and rely on the httptest contract evidence — do not claim the real-Lago pass.

- [ ] **Step 4: Final verification sweep**

Run: `go build ./... && go test ./...` and `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe -v`

Expected: trunk green — old gateway tests, commercial domain tests, seam tests, router tests, and the probe suite all pass.

- [ ] **Step 5: Commit and update the ledger**

`docs(lago): ADR-0014 seam decision and T05 integration evidence (#77)`; tick the ticket's acceptance boxes with pointers to the tests/evidence that prove each.

## Plan Self-Review

- **Spec coverage / acceptance mapping:** (1) "调用者只看到 provider-neutral 商业状态" → Task 4 closed-envelope handler + provider-leak test (`commercial_platform_route_test.go`); (2) "fake adapter 与 Lago adapter 通过同一接口契约" → Task 2/3 shared `contract_test.go` table; (3) "旧 Gateway 路径仍可运行，主干保持绿色" → Task 4 Step 4 regression run + Task 5 Step 4 full `go test ./...`, openmeter registration untouched; (4) "接口集中商业命令、商业快照和游标对账，不增加逐对象浅封装" → Task 2 frozen three-family port + Review Focus check + ADR-0014. The ticket's "ONE protected read-only Billing API operation" is Task 4; the #73 probe carryover is Task 1.
- **Interface freeze:** signatures, file path, kind-growth rule, and the ADR gate are stated in Global Constraints and Task 2; W3 (#78/#79) can be planned against `internal/commercial/platform.go` byte-for-byte.
- **Execution detail scan:** every task names files, commands, and expected results; secrets stay in env; the only Docker dependency is the optional tagged run; no migration is needed; container placement avoids the documented Invoke-ordering trap.
- **Interface consistency:** Task 2's port is consumed by Task 3 (adapters), Task 4 (handler + wiring), and Task 5 (integration test); the fake adapter is shared by contract and router tests.
- **TDD:** each task writes failing behavioral tests first (probe cleanup, port/fake contract, adapter failures, route protection/leak), then implements, then verifies GREEN before a focused commit.
- **Known risks:** `Payload any` defers payload typing to W3 (acceptable — freeze covers the method signatures and envelope, and no command kinds are enabled yet); real-Lago evidence may be `blocked-env` without Docker credentials (explicitly non-blocking); the readiness `degraded` state is only producible by the fake in T05 — later tickets add richer checks, and the closed enum already reserves it.
