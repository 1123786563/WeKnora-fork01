# Semantica A03 Model Gateway, Usage, and Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route Semantica model calls through a Go-authorized, budget-gated gateway that records physical usage and never retries an uncertain provider side effect blindly.

**Architecture:** Go persists an owner-tenant/KB model policy and issues a short-lived model-call capability only from a currently valid A01 scope. A consumer-owned `SemanticModelGateway` validates the capability, atomically claims an invocation, reserves budget before model resolution, marks dispatch immediately before provider I/O, resolves the tenant model through `ModelService`, and settles only trusted final usage through the existing `ExecutionGate`. Python sends only the capability and bounded messages/parameters to the internal route; it has no provider credentials or direct provider path.

**Tech Stack:** Go/Gin/GORM, existing A01 scope service, `ModelService`, `commercial.ExecutionGate`, commercial usage facts, Python `httpx`, PostgreSQL and SQLite versioned migrations, local controlled OpenAI-compatible provider for integration tests.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §§3, 5–6; `docs/adr/0002-semantica-independent-service.md`; `docs/plans/2026-09-20-semantica-rebaseline.md` §“A03 真实接缝与 RED/GREEN”; A03 in `docs/plans/2026-09-11-semantica-03-access-model.md` (subject to the rebaseline).

## Global Constraints

- Go remains authoritative for tenant, KB, model selection, funding, invocation identity, budget limits, deadline, price version, and raw usage; none is accepted from the Python model-call body.
- `SemanticModelWireRequest` contains exactly a short-lived capability token, messages, and bounded generation parameters; it never contains a tenant ID, model ID, run ID, call ID, funding, budget upper, deadline, provider URL, or provider credential.
- The semantic-service bearer identity, A01 scope signing key, and A03 model-capability signing key are distinct secrets. Missing/invalid signing material or missing price resolution fails closed; there is no direct-provider fallback.
- Budget denial occurs before `ModelService.GetChatModel` or provider I/O. A reserved call is released only while definitely not dispatched. Once provider I/O may have started, timeout/cancellation/malformed response becomes `unknown`, remains auditable, and is not automatically retried.
- Every successful physical call is recorded once with server-derived tenant/run/call/attempt/funding/model/price-version identity and provider-reported input/output token usage. BYOK calls retain raw usage but do not create model charges.
- The legacy environment `WEKNORA_USAGE_RATES` daily display aggregation is not a commercial rate source. A missing immutable `RateResolver` version must fail before provider dispatch; do not introduce arbitrary rates or billing values.
- Internal endpoints authenticate exact service bearer and audience before body decoding; provider credentials are resolved only by `ModelService` and must never enter logs or Python.
- No changes to existing Craft handler contracts, no public provider proxy, no model call from the UI, and no live-model or production-readiness claim without separately authorized credentials and evidence.

## Review Focus

- A forged, expired, replayed-under-changed-policy, wrong-owner, wrong-purpose, or caller-tampered capability must not reserve budget, resolve a model, or contact a provider.
- Concurrent requests with one invocation key and identical payload must yield one provider call and one settlement; the same key with different payload must conflict without dispatch.
- Budget denial must precede model resolution; a model-resolution failure before dispatch must release its hold; any possibly-dispatched outcome must remain `unknown` and protected for reconciliation.
- Provider-reported zero, missing, negative, overflowed, partial, or inconsistent token usage must never become an invented final zero-cost fact; platform and BYOK funding must be resolved by Go.
- Cancellation/deadline, output-token caps, error responses, and service shutdown must not leak provider secrets or accidentally release a possibly-spent reservation.

## Implementation Ruling: Owner-Scoped Model and Cost Policy

**Ruling:** The approved Semantica workflow has the KB administrator select the pilot KB and persist its model/budget policy before enablement. A03 therefore resolves the configured model and task budget under the source-owner tenant; the A01 requester identity remains the authorization and audit subject, not the billed tenant. Enabling the KB's Go-owned model policy is the owner's explicit consent to serve authorized shared readers through the proxy, subject to the current A01 read scope and owner budget. Credentials remain in the owner tenant and are never sent to Python. Platform-funded model usage debits owner Credits; BYOK provider charges remain the owner tenant's responsibility, and BYOK calls still consume owner-configured per-call/task token and invocation caps even though they do not settle platform model Credits. Model calls stay disabled by default; absent an approved immutable `RateResolver`, the gateway remains fail-closed. Cost if this ruling is wrong: a shared reader may consume the KB owner's platform Credits or BYOK provider quota; explicit owner-side enablement, finite caps, A01 live ACL checks, and the default-disabled/fail-closed gates limit that exposure. This is an implementation ruling inferred from the approved KB-owner setup/budget flow, not a production pricing approval; a later product ruling can replace it before promotion.

Evidence pointers: the spec requires a KB administrator to select the pilot KB and set its budget before enablement; A01 carries distinct source-owner scope and requester identity; native shared-KB retrieval resolves the source tenant's embedding model but does not define chat billing; shared custom Agents resolve an owner model under a separate AgentShare and have no production commercial-gate binding for classic QA. These neighboring paths support owner-scoped model resolution but are not treated as pricing evidence. See `internal/application/service/knowledgebase_search.go`, `internal/handler/session/qa.go`, `internal/agent/commercial_adapter.go`, and `CONTEXT.md`.

---

### Task 1: Persist and authorize the KB semantic model policy

**Files:**
- Create: `internal/application/repository/semantic_model_policy.go`
- Create: `internal/application/repository/semantic_model_policy_test.go`
- Create: `internal/application/service/semantic_model_policy.go`
- Create: `internal/application/service/semantic_model_policy_test.go`
- Modify: `internal/commercial/usage.go` and `internal/commercial/usage_test.go` to reject a fixed-point call charge that cannot fit the stored `int64` Credits amount
- Create: `internal/handler/semantic_model_policy.go`
- Create: `internal/handler/semantic_model_policy_test.go`
- Modify: `internal/router/routes_knowledge.go`
- Modify: `internal/container/container.go` for the policy service/handler/router dependency-injection provider path
- Modify: `internal/database/semantic_migration_test.go` and `internal/database/semantic_migration_pg_test.go` for the new formal migration head and policy-table up/down coverage
- Modify: `internal/types/knowledgebase.go` only if the existing KB policy route cannot own the DTO cleanly
- Create: PostgreSQL migration `migrations/versioned/000179_semantic_model_policy.up.sql` and `.down.sql`
- Create: SQLite migration `migrations/sqlite/000100_semantic_model_policy.up.sql` and `.down.sql`

**Interfaces:**
- `SemanticModelPolicy` is keyed by source-owner `(tenant_id, kb_id)` and contains owner `model_calls_enabled` (default false), selected owner-tenant chat `model_id`, immutable commercial `price_version`, maximum input/output tokens per call, maximum calls and total input/output tokens per task, optional positive task credit upper for platform-funded models, monotonically increasing `policy_version`, and update audit identity/time. The owner KB must exist and be active; the selected model must resolve within the owner tenant. Owner enablement is the required consent for shared requesters to call this model through the trusted proxy.
- Per-call `Upper` is derived in Go from the selected immutable `PriceVersionRates` and the persisted maximum input/output token envelope using the current commercial model-token dimension; neither policy clients nor Python may set it. `ExecutionGate.Finish` must use the exact same rate resolver/version. If rates are unavailable or the estimate is invalid, deny before dispatch.
- `SemanticModelPolicyService.Get(ctx, ownerScope)` returns the live, explicitly enabled policy or a not-configured/disabled error. `Put(ctx, ownerScope, input)` requires existing KB-write/manage permission, validates the owner model and finite per-call/task limits, resolves server-derived funding plus immutable price version, requires a positive task credit upper for platform funding, and writes a new version transactionally. It never accepts or edits price rates. The caller cannot write `policy_version`.
- Use the latest applied migration numbers from the integrated branch: PG 000178 and SQLite 000099 exist for A01, so the policy migration is PG 000179 / SQLite 000100 and the later invocation-ledger migration is PG 000180 / SQLite 000101.

- [ ] **Step 1: Write failing policy authorization and persistence tests**

Test that an owner KB manager can persist/read and explicitly enable a valid model policy; policy defaults disabled. Another tenant, a viewer, a missing/deleted KB, a model outside the owner tenant, an unknown price version, an unresolved immutable rate version, or an unbounded/invalid per-call/per-task cap is rejected without writes. Platform-funded policy requires a positive task credit upper; BYOK policy still requires finite per-task call/token caps. Replacing/disabling policy must increment `policy_version` and preserve the previous version for audit. A caller context whose requester tenant differs from the KB owner must still be switched to owner execution scope before either model or funding/pricing resolution. The exact maximum representable Credits charge must be accepted and a mathematically larger charge must return an error rather than wrap/truncate. Use formal PostgreSQL/SQLite migrations, not GORM-only schema creation.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/repository ./internal/application/service ./internal/handler ./internal/router ./internal/commercial -run '^(TestSemanticModelPolicy|TestPriceVersionRatesRejectsChargeOverflow)$' -count=1`

Expected: tests fail because no persisted Semantica model policy or management route exists.

- [ ] **Step 3: Implement the policy storage and authorized management route**

Store only policy values and the immutable rate-version reference, never rate edits or provider secrets. Reuse existing `OwnedKBOrAdmin`, `KBAccessWrite`, and owner-scope permission checks. Reject unknown request fields and trailing JSON. Write using the shared transaction and make policy versions monotonic under concurrent updates. The current production `RateResolver` defaults to unavailable; do not substitute legacy environment display rates or guess pricing. Until an approved resolver is wired, policy creation and model dispatch must fail closed.

- [ ] **Step 4: Run GREEN**

Run the Task 1 command again and both migration up/down/up tests. Expected: only authorized owner-side changes persist, policy version advances once, stale/missing model or rate versions deny without changing the previous policy, the owner execution context reaches all owner model/pricing lookups, and unrepresentable Charges fail closed.

### Task 2: Add model capability issuance and the durable invocation ledger

**Files:**
- Create: `internal/types/semantic_model.go`
- Create: `internal/types/interfaces/semantic_model.go`
- Create: `internal/application/service/semantic_model_capability.go`
- Create: `internal/application/service/semantic_model_capability_test.go`
- Create: `internal/application/repository/semantic_model_invocation.go`
- Create: `internal/application/repository/semantic_model_invocation_test.go`
- Create: PostgreSQL migration `migrations/versioned/000180_semantic_model_invocations.up.sql` and `.down.sql`
- Create: SQLite migration `migrations/sqlite/000101_semantic_model_invocations.up.sql` and `.down.sql`
- Modify: `internal/config/config.go`, `internal/config/semantic_test.go`, and `internal/runtime/startup.go` for a separate sensitive `SEMANTIC_MODEL_SIGNING_KEY`
- Modify: `internal/container/container.go`

**Interfaces:**
- `SemanticModelWireRequest` is `{capability_token, messages, parameters}` only. Each message has a validated role and bounded content. Parameters contain temperature and max output tokens only.
- `SemanticModelCapability` is Go-only and binds the resolved owner scope, A01 subject/requester/purpose/scope hash/expiry, model-policy version, owner model/billing tenant, model ID, model funding, task run ID, call ID, per-call and per-task limits, per-call upper for platform-funded models, price version, and deadline.
- `SemanticModelCapabilityIssuer.Issue(ctx, scopeRef, invocationKey)` verifies the live A01 scope with `Resolve`, loads the active owner-KB policy, derives run/call identity in Go, obtains the versioned commercial price/rate estimate, persists/reuses the task budget, and signs a short-TTL HS256 token. Python cannot request another model, funding source, upper, deadline, owner, or tenant.
- Implement this issuer in the new `internal/application/service/semantic_model_capability.go`; inject the A01 scope resolver, active `SemanticModelPolicyService`, the same `SemanticModelPricingResolver`/immutable rates used by policy validation and final settlement, and the invocation repository. Re-read owner policy version and price version at issuance and invocation; stale capability fails before quota/budget changes.
- Funding is derived by a server-side `SemanticModelFundingResolver` from the model's trusted credential provenance. Never infer it from a client field, URL, or mere presence of an API-key string; if source/funding cannot be established, reject the call.
- Derive `runID` as `semantic-run:` plus SHA-256 of the exact A01 `scope_ref`, and derive `callID` as a namespaced SHA-256 of `scope_ref` plus `invocationKey`; for platform funding call `BudgetStore.EnsureTaskBudget(ctx, ownerTenant, runID, policy.TaskUpper, scopeExpiry)` idempotently. For BYOK, do not reserve or settle platform model Credits, but atomically enforce the same owner policy's max calls and aggregate token caps in the invocation ledger. The invocation key is only a stable deduplication input inside that authenticated scope, never an authorization field.
- `SemanticModelInvocationStore.EnsureRun(ctx, capability)` idempotently persists the signed owner task limits and policy/scope version; replay with different limits or policy binding conflicts. `Claim(ctx, capability, requestHash)` atomically checks/claims one owner-run call and reserves its maximum input/output token envelope against `max_calls_per_task` and aggregate token caps before provider dispatch. Same call ID plus same request hash returns the prior completed result; a changed hash conflicts. `Complete` replaces the reserved token envelope with observed usage and persists the result; `FailBeforeDispatch` returns the unspent call/token quota and is terminal for that call ID; `MarkUnknown` retains the full envelope/quota for reconciliation. Unknown and completed calls cannot be replayed to the provider under that call ID.

- [ ] **Step 1: Write failing capability and concurrency tests**

Test expired/wrong-audience/tampered tokens, mismatched A01 scope, deleted owner, revoked scope, disabled/stale policy, and unknown rate version; assert no ledger or platform-budget change on rejection. Race two identical claims and assert one quota slot and invocation claim. Test max-call and per-task input/output reservation limits at and over the boundary. Reuse the key with a different payload hash and assert conflict. Verify pre-dispatch failure releases its quota slot, unknown outcome retains its full token envelope, and completed response replays without a second dispatch.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/repository ./internal/application/service ./internal/config -run '^TestSemanticModel(Capability|Invocation)' -count=1`

Expected: fail because no A03 capability signing or idempotency ledger exists.

- [ ] **Step 3: Implement signed capability and atomic invocation claim**

Use HS256 only and a new secret distinct from both the A01 scope signer and internal service bearer. Hash messages and parameters for invocation identity; never persist capability tokens or provider credentials. Persist owner run-quota limits and invocation result under the owner tenant; only expose results through the authenticated internal route. Make token expiry no later than both A01 scope expiry and owner-policy deadline.

- [ ] **Step 4: Run GREEN**

Run the Task 2 command and paired migration up/down/up tests. Expected: invalid authority is rejected before any stateful budget/model work; concurrent identical retries converge on one invocation, and changed payload cannot reuse an identity.

### Task 3: Build the Go `SemanticModelGateway` over commercial and model seams

**Files:**
- Create: `internal/application/service/semantic_model.go`
- Create: `internal/application/service/semantic_model_budget.go`
- Create: `internal/application/service/semantic_model_test.go`
- Modify: `internal/types/interfaces/semantic_model.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- `SemanticModelGateway.Invoke(ctx, wire) (SemanticModelResult, error)` accepts only the Python wire request.
- `SemanticModelResult` returns text, prompt/completion token counts, optional provider request ID, and resolved model version; fields unavailable from a provider remain absent, never synthesized.
- `SemanticScopeResolver` exposes `Resolve(ctx, scopeRef) (service.SemanticScopeSnapshot, error)`; the interface belongs to the consumer service package because A01 intentionally exports its concrete service without adding this unrelated method to shared `types/interfaces`.
- `SemanticModelGateway` uses an injected `interfaces.ModelService`, that narrow A01 resolver, a semantic-budget adapter backed by the existing commercial `BudgetStore` and `ExecutionGate.Finish`, the same immutable `RateResolver` used by final settlement, policy service, and invocation store. It does not import Craft handler/service types or change the generic `ExecutionGate.Begin` contract.
- The semantic-budget adapter performs `BudgetStore.Reserve`, `MarkReservationDispatched`, and `ReleaseReservation` as distinct transitions for platform funding. Model lookup, platform budget rows, and usage facts use the same source-owner tenant; requester identity stays in A01 scope and invocation audit fields. For BYOK, the usage fact is recorded under the owner with `FundingBYOK`, no platform model Credits are settled, and owner policy token/call caps remain mandatory.
- The dispatch order is: validate capability/owner policy/A01 scope and request bounds → atomically claim invocation and reserve owner task call/token quota → for platform funding reserve an unstarted owner-tenant hold → resolve the owner-tenant `GetChatModel` → persist dispatched state and mark any platform reservation dispatched immediately before `chat.Chat` → invoke with cancellation and max-output cap → record provider-observed usage under the owner → finish a platform reservation exactly once with the exact returned ID. BYOK makes no platform model-credit reservation but still atomically consumes the owner policy's per-task call/token quota. On known pre-dispatch failure, `BudgetStore.ReleaseReservation` releases the still-held platform reservation and the invocation ledger returns its call/token quota; after the dispatched boundary, store `unknown` and retain the full quota plus any platform hold for reconciliation.

- [ ] **Step 1: Write failing gateway ordering and accounting tests**

Required RED cases: platform budget denial performs zero `GetChatModel` calls and zero provider calls; owner policy disabled/ACL invalid/quota exhausted are denied before model resolution; model resolution failure releases only an undispatched platform hold and call/token quota; a successful platform call finishes exactly once with the owner reservation; token usage becomes one owner-tenant final `commercial.UsageFact` with server-derived funding and rate version; BYOK raw usage is retained, consumes call/token quotas, and creates no platform model-credit settlement; missing/partial usage is `unknown`, not a zero fact, and retains the full token quota; cancellation after dispatch is unknown; completed identical replay returns stored bytes with no second `Begin`/model call/`Finish`.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/application/service ./internal/application/service/commercial -run '^TestSemanticModel' -count=1`

Expected: fail because the semantic gateway and its consumer-owned release-before-dispatch adapter do not exist.

- [ ] **Step 3: Implement the consumer-owned gateway and narrow pre-dispatch gate seam**

Keep the generic `ExecutionGate.Begin` contract and all existing callers unchanged. Use the semantic-budget adapter's unstarted reservation transitions so a proven pre-send failure can be released before dispatch and mark it dispatched only at the irreversible outbound boundary. Do not release after timeout/cancellation once provider I/O may have begun. Resolve the model only after budget admission. Validate provider response usage as non-negative, bounded integers; absent usage or malformed provider completion becomes unknown and is never settled as a fabricated zero.

- [ ] **Step 4: Run GREEN**

Run the Task 3 command. Expected: accounting and dispatch counts match every ordering assertion, unknown reservations remain protected, and no non-A03 gate consumer changes behavior.

### Task 4: Add the service-authenticated HTTP capability and invoke routes

**Files:**
- Create: `internal/handler/semantic_model_internal.go`
- Create: `internal/handler/semantic_model_internal_test.go`
- Modify: `internal/router/routes_infra.go`
- Modify: `internal/router/router.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- `POST /api/v1/internal/semantic/model-capabilities` accepts `scope_ref` and an opaque invocation idempotency key; it returns only the short-lived capability and policy-bounded request limits.
- `POST /api/v1/internal/semantic/models/invoke` accepts only `SemanticModelWireRequest` and returns `SemanticModelResult` or a non-leaking error classification.
- Both routes verify exact service Bearer and configured audience before strict JSON decoding. No ordinary user auth or API key is an alternate path. Capability issue validates a live A01 scope and policy; invocation validates the capability again.

- [ ] **Step 1: Write failing route tests**

Test missing/wrong service token, wrong audience, unknown JSON fields, trailing JSON, invalid capability, payload conflict, model error, gate denial, cancellation, and valid controlled invocation. Assert unauthorized requests never call capability/gateway services and responses/log capture never contain either signing key, service token, API key, or base URL.

- [ ] **Step 2: Confirm RED**

Run: `go test ./internal/handler ./internal/router -run '^TestSemanticModelInternal' -count=1`

Expected: fail because these routes do not exist.

- [ ] **Step 3: Implement routes and non-leaking failure mapping**

Authenticate before decode, set request-derived deadlines/cancellation, cap request body bytes and message count/length, and never echo provider errors or credential-bearing model configuration. Enable routes only when semantic service identity, scope signer, model signer, and rate resolver are all configured.

- [ ] **Step 4: Run GREEN**

Run the Task 4 command. Expected: only the authenticated semantic service can issue/invoke and every malformed or unauthorized request fails before model or budget dispatch.

### Task 5: Add the Python adapter and controlled-provider integration proof

**Files:**
- Create: `semantic/semantic_service/model_gateway.py`
- Create: `semantic/tests/test_model_gateway.py`
- Modify: `semantic/tests/conftest.py` only for isolated Go route and local provider fixtures
- Create/modify: `internal/application/service/semantic_model_integration_test.go` for the real Go gateway path

**Interfaces:**
- Python `ModelGateway.invoke(scope_ref, invocation_key, messages, parameters)` calls capability issuance, then the invoke endpoint. It has only the semantic-service bearer and never reads provider API credentials or provider URL.
- A retry reuses the same capability and identical canonical request. A true new provider attempt uses a new invocation key and is linked to the same parent task by Go.
- The controlled-provider proof uses an actual Go `ModelService`/chat adapter pointed at a loopback test HTTP server with deterministic usage headers/body; it asserts capability fields, provider request, raw usage row, settlement, replay, cancellation and credential isolation. It is labeled `controlled-provider`, not `live-model`.

- [ ] **Step 1: Write failing Python and controlled-provider tests**

Python tests assert exact request fields, no tenant/model/upper/deadline/provider credential, no direct provider dependency, and one gateway response on an identical retry. Go integration tests assert one provider request, one raw final usage fact, one settlement, and no second provider request for the replayed invocation.

- [ ] **Step 2: Confirm RED**

Run: `uv run --locked --project semantic python -m pytest semantic/tests/test_model_gateway.py -q` and `go test ./internal/application/service -run '^TestSemanticModelControlledProvider' -count=1`.

Expected: fail because Python adapter and controlled Go model-proxy route are absent.

- [ ] **Step 3: Implement the Python-only internal adapter**

Use bounded connect/read timeouts and propagate caller cancellation. Retry only a replay-safe identical request with its same capability; never retry a `unknown` or timeout response with a new invocation key automatically.

- [ ] **Step 4: Run GREEN**

Run the Task 5 commands. Expected: all cases pass using only loopback controlled-provider fixtures; no real model credentials, external provider, or production endpoint is touched.

### Task 6: Final verification and evidence

- [ ] **Step 1: Run focused gates**

Run:

```sh
go test ./internal/application/repository ./internal/application/service ./internal/handler ./internal/router ./internal/config ./internal/container -run 'TestSemanticModel|TestSemantic.*Model' -count=1
uv run --locked --project semantic python -m pytest semantic/tests/test_model_gateway.py -q
go test ./internal/application/service ./internal/application/service/commercial -count=1
```

- [ ] **Step 2: Audit boundaries and migrations**

Use `rg` to prove only the new A03 gateway calls `GetChatModel` for the Semantica model path; no Python provider implementation, API key, BaseURL, service token, or signing key crosses into model-call payloads/logs. Run PostgreSQL and SQLite migration up/down/up against isolated temporary databases. Confirm existing non-Semantica model paths and Craft tests remain unchanged.

- [ ] **Step 3: Record evidence and review**

Update `docs/plans/semantica/progress.md` with exact command, exit, environment, migration/runtime layers, review findings, and explicit limits. Keep A03 `in_progress` if the production immutable rate resolver, authorized pricing configuration, real provider, or any required evidence layer is absent. Run `git diff --check`, independent read-only review, and commit the A03 implementation separately from plan changes.

## Completion Boundary

A03 can prove a controlled, Go-mediated, budget-gated model invocation and durable usage/unknown state. It does not prove current live provider prices, credentials, upstream service availability, Semantica answer quality, A02 authorized-subgraph correctness, Q03 model reasoning, Q04 user-delivery authorization, or production enablement. Missing price resolution, model configuration, budget account, or credentials must remain an explicit fail-closed condition.
