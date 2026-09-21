# Lago T02 — Payment-Activation Lab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Prove on the pinned Lago Community `v1.53.0` runtime whether an externally-registered payment can activate a payment-gated Subscription exactly once, produce reviewable evidence for every acceptance criterion of Ticket #74, and decide the purchase-flow recording path (manual Payment via API vs Lago-supported external Payment Provider) for Tickets #81/#82.

**Architecture:** A self-contained lab under `deploy/lago-lab/payment-activation/` runs its **own** instance of the #73 pinned stack (reusing `deploy/lago/compose.yaml` and `deploy/lago/images.lock.json` read-only, via Compose project `weknora-lago-74`, API port `48891`, frontend port `48892`). A stdlib-only Python experiment library drives real REST/GraphQL calls through experiment phases (gating observation, manual-payment attempt, provider-path activation, duplicate/retry probes, decline control, cleanup); each phase writes a sanitized report. Final evidence and the decision document live in `docs/migrations/lago/t02-payment-activation/`. No product code changes; no changes to `deploy/lago/`.

**Tech Stack:** Python 3 standard library (`unittest`, `http.server` fixtures), Bash, Docker Compose v2, Lago Community `v1.53.0` (digest-locked by #73), Stripe **test mode** API as the Lago-supported external Payment Provider.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — sections *Quotes, payments, and fulfillment* (activation rule; manual Payment must not be force-bypassed) and *Testing Decisions* (contract tests record pinned release, request, response, final objects, known limitation; schema inspection alone is not a pass). Parent and acceptance source: GitHub Issues #72 and #74; environment base: Ticket #73.

## Pinned-release findings (research, verified against the v1.53.0 source)

The application source is the `api` submodule of `getlago/lago` at tag `v1.53.0`, commit `591ae9005110346f1c6034ec72ea9046625668cf` (`getlago/lago-api`). These facts drive the experiment design; the lab must still **verify them on the runtime** and record any deviation:

1. **Payment-gated subscriptions exist in Community v1.53.0.** `Subscription::STATUSES` includes `incomplete` (`app/models/subscription.rb`); a subscription created with `activation_rules: [{type: "payment", timeout_hours: N}]` (`POST /api/v1/subscriptions`, param permitted in `app/controllers/api/v1/subscriptions_controller.rb`) is gated by `Subscriptions::ActivateService` → `mark_as_incomplete!`, billed pay-in-advance with `skip_charges: true`, leaving an **`open` (unfinalized, numberless) invoice** and emitting `subscription.incomplete`. `timeout_hours: 0` means "never expires"; the clock expires gated subscriptions hourly at :20 (`clock.rb`, `ExpireIncompleteSubscriptionsJob`) — too slow for runtime observation; document from source.
2. **Manual Payment is Premium-gated in Community.** `Payments::ManualCreateService#check_preconditions` returns `forbidden_failure!` unless `License.premium?` (`app/services/payments/manual_create_service.rb`); `License.premium?` requires a server-verified `LAGO_LICENSE` (`config/initializers/license.rb`, `lib/lago_utils/lago_utils/license.rb`). Both `POST /api/v1/payments` and GraphQL `CreatePayment` hit this service. The REST contract accepts only `invoice_id`, `amount_cents`, `reference`, `paid_at` — **no `external_id`, no `status` field** (always recorded `succeeded`); this differs from the field list the ticket draft assumed, and the evidence must record the actual contract.
3. **A gated subscription cannot be created without a chargeable provider customer.** `ActivationRules::Payment::ValidateService` rejects the create when the customer has no linked payment provider or no resolvable payment method (`no_linked_payment_provider` / `no_default_payment_method`), and rejects `payment_method_type: manual` outright. So every experiment customer is provider-connected **before** the subscription is created; the "before payment" window is observed between subscription creation and payment settlement.
4. **Activation is driven by invoice `payment_status`, via the provider payment flow.** A gated invoice auto-triggers `Invoices::Payments::CreateService` (`app/services/invoices/subscription_service.rb`), which creates a provider Payment and a confirmed, off-session PaymentIntent (`app/services/payment_providers/stripe/payments/create_service.rb`). The Stripe success result is **synchronous** — no webhook needed (loopback is unreachable from Stripe anyway). Succeeded/failed payment status flows through `Invoices::UpdateService#handle_payment_gated_activation` → `Payment::ResolveJob` → rule `satisfied` → invoice **finalized** → subscription **active** (`app/services/subscriptions/activation_rules/payment/resolve_service.rb`, `resolve_subscription_status_service.rb`); failed → invoice closed → subscription **canceled** (`cancellation_reason: payment_failed`).
5. **Exactly-once is enforced by state machines and a partial unique index.** At most one provider payment in `pending/processing` per payable (`app/models/payment.rb` index); `ResolveService` no-ops unless the subscription is still `incomplete` and the invoice still `open`; `POST /api/v1/invoices/{id}/retry_payment` returns `not_allowed (invalid_status)` once `payment_succeeded?`; re-creating a subscription with the same `external_id` returns `422 value_already_exist` (active) or `subscription_incomplete` (incomplete); the PaymentIntent carries `idempotency_key: "payment-#{payment.id}"`.
6. **A stable pre-payment window exists via a 3DS-challenge test card.** For gated payments, `authentication_required` is retriable (`retriable_authentication_failure?` → `payment.gated_subscription_activation?`), so the invoice stays `payment_status: pending` and the subscription stays `incomplete` indefinitely — this is how the lab holds a gated subscription open for the manual-payment attempt and retry probes.
7. **Entitlements are active-subscription-only at the API.** `GET /api/v1/subscriptions/{external_id}/entitlements` filters `status: :active` by default (`app/controllers/api/v1/subscriptions/entitlements_controller.rb`) → `404` while incomplete, entitlement list after activation.
8. **Provider configuration is GraphQL + user JWT, not Premium-gated, and Stripe payment-method types are allowlisted.** `AddStripePaymentProvider` etc. require a logged-in user (the seeded operator account works). `PaymentProviderCustomers::StripeCustomer::PAYMENT_METHODS` = card, sepa_debit, us_bank_account, bacs_debit, link, boleto, crypto, customer_balance — **no `wechat_pay`/`alipay`**, and there is no generic/custom provider in Community (`app/models/payment_providers/` = stripe, gocardless, adyen, cashfree, flutterwave, moneyhash). Lago `PaymentMethod` records are imported when a provider customer is connected with an existing `provider_customer_id` (`FetchDefaultPaymentMethodJob` reads the Stripe customer's default payment method).

**Consequence for the decision:** the manual path is Community-blocked (403, finding 2); the provider path works (finding 4) but only for provider-driven charges — Community v1.53.0 has **no supported entry point to register an externally completed WeChat/Alipay channel payment against a gated invoice** (finding 8 + finding 2). The decision document must state this blocker explicitly with evidence and frame the options for #81/#82 (see Task 4).

## Global Constraints

- **Isolation:** the lab stack is its own Compose project `weknora-lago-74` with `LAGO_API_PORT=48891`, `LAGO_FRONT_PORT=48892` (loopback only). It must never start, stop, or read state of the `weknora-lago` (#73) or OpenMeter projects, and never touch their volumes, `.env`, or data. All lab objects (Compose volumes, external ids) are project/run-scoped.
- **`deploy/lago/` is read-only** for this ticket: reuse `compose.yaml`, `images.lock.json`, `health.py`, and `README.md` as-is (import `health.py` functions or invoke `docker compose` directly with `--env-file`; do not edit them). If a change there turns out to be required, do NOT make it — record it as a cross-ticket note in the decision document.
- **Stripe test mode only.** The runner refuses any `STRIPE_SECRET_KEY` that does not start with `sk_test_` or `rk_test_`. No live keys, no real charges. Stripe-side objects (test customer, attached test payment method) are throwaway; the Lago-side cleanup duty still applies to everything the lab creates in Lago.
- **Secrets hygiene:** `LAGO_API_KEY` comes from the lab stack's own seed (the lab generates `LAGO_ORG_API_KEY` itself in its git-ignored `lab.env`, exactly like #73's seed flow); `STRIPE_SECRET_KEY` comes from the caller environment. No secret value may be committed, logged, or written to any evidence file; the GraphQL login JWT and operator password are equally secret. Evidence files are sanitized by construction and verified by a secrets scan before commit.
- **Synthetic isolated objects:** every external id and code is prefixed `weknora-t02-<run-uuid>` (customer, plan, feature, subscription). No personal data. **Cleanup is mandatory**: the runner always attempts to delete what it created (subscription → customer → plan → feature), reports cleanup failures distinctly, and never reports overall `pass` with unreported cleanup failures.
- **Community boundary:** `LAGO_LICENSE` stays empty. Any capability that turns out Premium-gated (expected: manual payments) is recorded as evidence, never a reason to set a license or to force state locally (spec: *may not force the Subscription active locally*).
- **Unit tests must not require Docker, Stripe, or network.** All offline tests use in-process `http.server` fixtures and local files. The real-stack run (Task 3) is the only Docker-dependent step and may be executed off-peak if machine resources are tight; everything else (Tasks 1, 2, 4) must be completable without Docker.
- **Honesty of outcomes:** every phase result is `pass | fail | blocked-env`. Missing Docker, missing API key, missing Stripe test key (provider phases only), or unreachable `api.stripe.com` from the Lago API container is `blocked-env` evidence — never a mock pass, and never presented as a passed contract (spec: *an OpenAPI schema … or a healthy Lago API process is not sufficient completion evidence*).
- **Out of scope:** no product/Billing API code, no WeKnora adapter, no OpenMeter changes, no Lago forks, no Premium feature usage, no AGPL legal conclusions (evidence only, as in T01).

## Review Focus

- **Stack collision:** the lab stack accidentally controls or collides with the #73 `weknora-lago` project, OpenMeter, or another worktree — test project name, ports, and volume prefix invariants; `lab.sh` must scope every `docker compose` call with `-p weknora-lago-74` and the lab env file.
- **Secret leakage into evidence or logs:** API key, Stripe key, JWT, or operator password appearing in phase reports, run logs, or committed evidence — test redaction on every serialization path plus a pre-commit grep scan over the tracked evidence.
- **Verdict inflation:** a phase marked `pass` from an HTTP 200 or a schema match without final authoritative state (subscription status, invoice status/payment_status, payment count, entitlement list) — tests must assert classification on full outcome tuples, and `blocked-env` must be distinct from `fail`.
- **Exactly-once illusion:** duplicate/retry probes that only check the second call's error code without proving the first activation was not duplicated — tests must assert final state (single active subscription, single succeeded payment, stable `total_paid_amount_cents`) after all duplicate attempts.
- **Misread Premium gate:** a 403 from `POST /api/v1/payments` treated as a product bug, or any temptation to bypass via `LAGO_LICENSE`, GraphQL internals, or direct DB writes — the plan treats the 403 as the expected Community evidence.
- **Abandoned lab objects:** cleanup skipped or silently failing after an assertion error — test `finally`-path cleanup and distinct cleanup-failure reporting.

---

### Task 1: Isolated lab stack lifecycle and redaction-safe clients

**Files:**
- Create: `deploy/lago-lab/payment-activation/lab.sh`
- Create: `deploy/lago-lab/payment-activation/lab.env.example`
- Create: `deploy/lago-lab/payment-activation/.gitignore` (ignore `lab.env`, run transients; keep scripts, fixtures, and sanitized evidence)
- Create: `deploy/lago-lab/payment-activation/clients.py`
- Create: `deploy/lago-lab/payment-activation/test_lab.py`

**Interfaces:**
- `lab.sh init|up|down|status|config` — same operator contract as #73's `lago.sh`, but drives `docker compose -f <repo>/deploy/lago/compose.yaml --env-file <lab>/lab.env -p weknora-lago-74`. `init` generates the full secret set plus seed values (`LAGO_CREATE_ORG=true`, `LAGO_ORG_USER_EMAIL/PASSWORD/NAME`, random `LAGO_ORG_API_KEY`) into git-ignored `lab.env` (mode 600, refuse overwrite); `up` validates secrets differ from Lago sample defaults and port/URL consistency, then `up -d --wait`; `status` builds the health snapshot by importing `deploy/lago/health.py`'s pure functions over its **own** `docker compose … ps` rows and `http://127.0.0.1:48891/health`; `config` prints resolved Compose config redacted.
- `clients.py` — stdlib-only: `LagoRestClient` (bearer auth, JSON, never follows redirects off-origin), `lago_graphql_login(url, email, password) -> jwt`, `StripeTestClient` (create customer, attach payment method, set default payment method; refuses non-test keys). All take origins/keys as explicit arguments; no secret is ever included in `repr`/serialized output (test-enforced redaction helper `sanitize(obj)`).

- [ ] **Step 1: Write failing behavior tests**

Offline `unittest` cases (in-process `http.server` fixtures, temp dirs; no Docker/network):

```python
def test_lab_env_pins_isolation_values():
    env = generate_lab_env(tmp_path)
    assert env["COMPOSE_PROJECT_NAME"] == "weknora-lago-74"
    assert env["LAGO_API_PORT"] == "48891" and env["LAGO_FRONT_PORT"] == "48892"
    assert env["LAGO_API_URL"] == "http://127.0.0.1:48891"

def test_stripe_client_refuses_live_keys():
    with pytest_style_assert_raises(RefusedLiveKey):
        StripeTestClient(api_key="sk_live_x", base_url=server.url)
```

Also test: `init` refuses to overwrite, writes mode 600, and every generated secret differs from the official Lago sample defaults; REST client sends `Authorization: Bearer` only to the configured origin and never to a redirect target; GraphQL login extracts the JWT from `loginUser` and never echoes it in exceptions; `sanitize` strips known secret shapes; lab.sh's compose invocation (asserted as a unit over its constructed argv) always carries `-p weknora-lago-74` and `--env-file` pointing inside the lab directory.

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Expected: tests fail because the lab modules do not exist.

- [ ] **Step 3: Implement the lab lifecycle and clients**

Implement `lab.sh` and `clients.py` per the interfaces above. Mirror #73's secret generation and validation logic (do not duplicate `health.py`; import it). `status` must run `docker compose -f … -p weknora-lago-74 ps --format json` from the lab directory so it observes only the lab project.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Expected: all Task 1 tests pass without Docker or network.

- [ ] **Step 5: Commit the lab foundation slice**

Commit only the lab lifecycle, clients, and tests.

### Task 2: Experiment phase library against the pinned contracts

**Files:**
- Create: `deploy/lago-lab/payment-activation/fixtures.py` (plan/feature JSON templates, pay-in-advance monthly, positive amount, plan-attached entitlement)
- Create: `deploy/lago-lab/payment-activation/phases.py`
- Create: `deploy/lago-lab/payment-activation/test_phases.py`

**Interfaces:**
- Consumes: Task 1 clients; run context `{run_id, prefix, lago_url, api_key, stripe_key, graphql_jwt}`; external ids `weknora-t02-<run-uuid>[-a|-b|-c]` (customers A/B/C).
- Produces: pure-ish phase functions, each returning a report dict `{phase, expected, observed, status: pass|fail|blocked-env, evidence}`:
  - `phase_setup(ctx)` — create Feature, Plan (`interval: monthly`, `pay_in_advance: true`, positive `amount_cents`), plan entitlement; record created codes.
  - `phase_provider_setup(ctx)` — GraphQL `AddStripePaymentProvider` (test key); Stripe test customers; attach payment methods (A: `pm_card_authenticationRequired`, B: `pm_card_visa`, C: `pm_card_visa_chargeDeclined`) and set each as Stripe default; create Lago customers with `payment_provider: "stripe"`, `provider_customer_id`, `provider_payment_methods: ["card"]`; poll `GET /api/v1/customers/{id}/payment_methods` until each customer has a Lago default payment method (bounded polling with timeout). `blocked-env` when Stripe key missing or Stripe API unreachable.
  - `phase_gate(ctx)` — **AC1**: for customer A create subscription with `activation_rules: [{type: "payment", timeout_hours: 0}]`; immediately observe and record: subscription `status: incomplete`; `GET /api/v1/subscriptions/{external_id}/entitlements` → `404`; gating invoice `status: open` (no number), `payment_status: pending`; payments list has at most one non-succeeded payment. Then confirm the stable window: after the auth-challenge auto-charge attempt, subscription is **still** `incomplete` and invoice still `pending`.
  - `phase_manual(ctx)` — **AC4 (manual path)**: `POST /api/v1/payments` against customer A's open gating invoice with `{invoice_id, amount_cents, reference, paid_at}`; record HTTP status and error body. Expected on Community: forbidden. Assert invoice and subscription state unchanged after the attempt; record the actual accepted/required parameter contract observed from the error/validation response.
  - `phase_activate(ctx)` — **AC2**: for customer B create the same gated subscription; poll until (bounded): subscription `active`; entitlements `200` with the plan feature; invoice `finalized` with a number and `payment_status: succeeded`; `GET /api/v1/payments?external_customer_id=…` shows **exactly one** `succeeded` provider payment with a `provider_payment_id`; `total_paid_amount_cents == invoice total_amount_cents`.
  - `phase_duplicates(ctx)` — **AC2**: against the settled customer B state: (1) re-`POST /api/v1/subscriptions` with the same `external_id` (expect `422 value_already_exist`); (2) `POST /api/v1/invoices/{id}/retry_payment` (expect not-allowed / invalid status); (3) re-attempt the manual payment POST (expect forbidden again); then re-read final authoritative state: still one subscription, still `active`, still exactly one succeeded payment, invoice unchanged.
  - `phase_retries(ctx)` — **AC3**: (1) response-loss on create: re-POST customer B's subscription with the same `external_id`, then recover via `GET /api/v1/subscriptions?external_id=…` → same Lago subscription id and status (same commercial identity, no second object); (2) retry on a live pending gate: `POST /api/v1/invoices/{id}/retry_payment` on customer A's pending gated invoice (expect a re-attempt that keeps the invoice pending and does **not** create a second pending/succeeded payment row); (3) final payments count for A unchanged. Record all request/response pairs.
  - `phase_decline_control(ctx)` — **negative control for AC2/AC4**: customer C gated subscription with the declined card → poll until subscription `canceled` with `cancellation_reason: payment_failed`, invoice closed, entitlements `404`. Proves activation happens only on a verified provider success.
  - `phase_cleanup(ctx)` — always executed last (and from `finally`): terminate/delete subscriptions, delete customers, delete plan, delete feature (dependency order); delete Stripe test customers best-effort; record per-object cleanup results distinctly.

- [ ] **Step 1: Write failing phase tests**

Drive every phase against scripted `http.server` fixtures that replay the pinned v1.53.0 contract shapes above. Assert at least:

```python
def test_gate_phase_requires_incomplete_and_entitlement_404(server, ctx):
    report = phase_gate(ctx)
    assert report.observed["subscription_status"] == "incomplete"
    assert report.observed["entitlements_status"] == 404
    assert report.status == "pass"

def test_activate_phase_fails_without_exactly_one_succeeded_payment(server, ctx):
    server.payments_for_customer = [succeeded, succeeded]  # duplicate injection
    assert phase_activate(ctx).status == "fail"
```

Also test: manual phase records forbidden without mutating state; duplicates phase asserts **final state** after the duplicate attempts, not just error codes; retries phase proves same-identity recovery via GET; decline control requires `canceled` + `payment_failed`; cleanup runs and reports after a mid-phase exception; every report survives `sanitize` with zero secret occurrences (inject canary secrets); missing Stripe key → provider phases report `blocked-env` while gate/manual phases still classify normally; polling timeout produces `fail` with the last observed state attached.

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Expected: phase tests fail because `phases.py`/`fixtures.py` do not exist.

- [ ] **Step 3: Implement the phases**

Before coding each call, cross-check the operation and field names against the pinned runtime behavior documented in the *Pinned-release findings* section and, where ambiguous, the official docs for the pinned release (getlago.com/docs); when the live runtime disagrees with either, the runtime wins and the deviation is recorded in the phase report (`contract_notes`). Keep all polling bounded and configurable; never retry a `POST` whose outcome is unknown except through the documented same-identity recovery (GET by `external_id`), mirroring the spec's indeterminate-outcome rule.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Expected: all offline tests pass with zero network access.

- [ ] **Step 5: Commit the phase library slice**

Commit only `phases.py`, `fixtures.py`, and `test_phases.py`.

### Task 3: Runner, real-stack execution, and the sanitized evidence bundle

**Files:**
- Create: `deploy/lago-lab/payment-activation/run_lab.py`
- Create: `deploy/lago-lab/payment-activation/README.md` (operator walkthrough: prerequisites incl. Stripe test key, commands, blocked-env meanings, evidence contract)
- Create: `deploy/lago-lab/payment-activation/evidence/` (verbatim per-phase working reports produced by the real run; sanitized by construction)
- Create: `docs/migrations/lago/t02-payment-activation/README.md` (final evidence contract + file table, in the style of `deploy/lago/evidence/README.md`)
- Create (from the real run, final copies): `docs/migrations/lago/t02-payment-activation/t02-environment.json`, `t02-gating.json`, `t02-manual.json`, `t02-activation.json`, `t02-duplicates.json`, `t02-retries.json`, `t02-decline.json`, `t02-cleanup.json`, `t02-run.txt`

**Interfaces:**
- `run_lab.py --output-dir <dir>`: reads `lab.env` for Lago origin/key/JWT inputs and caller-env `STRIPE_SECRET_KEY`; executes phases in order (setup → provider_setup → gate → manual → activate → duplicates → retries → decline_control → cleanup), always running cleanup; writes one sanitized JSON report per phase plus a sanitized run timeline; prints an overall verdict that is `pass` only when every executed phase passed and cleanup succeeded; exits non-zero on `fail`, exit 2 on `blocked-env`.

- [ ] **Step 1: Implement the runner and evidence contract**

Wire the Task 2 phases into the ordered runner with per-phase output files, timestamps, pinned release identity from `deploy/lago/images.lock.json`, and a secrets scan (grep the output tree for every secret value known to the run; zero hits required before any file is promotable to `docs/`).

- [ ] **Step 2: Run all offline checks**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Run: `./deploy/lago-lab/payment-activation/lab.sh init && ./deploy/lago-lab/payment-activation/lab.sh config`

Expected: all tests pass; the lab Compose config resolves without pulling or creating anything, shows project `weknora-lago-74`, ports 48891/48892 loopback-only, and no secret values.

- [ ] **Step 3: Run the real experiment (may be scheduled off-peak)**

Run (repo root):

```bash
./deploy/lago-lab/payment-activation/lab.sh up
LAGO_ORG_USER_EMAIL/PASSWORD sourced by the runner from lab.env \
STRIPE_SECRET_KEY=<operator test key> \
  ./deploy/lago-lab/payment-activation/run_lab.py --output-dir deploy/lago-lab/payment-activation/evidence
./deploy/lago-lab/payment-activation/lab.sh status --json
./deploy/lago-lab/payment-activation/lab.sh down
```

Expected per acceptance criterion on the pinned Community runtime:
- **AC1** (`t02-gating.json`): customer A subscription observed `incomplete`; entitlements endpoint `404`; gating invoice `open`/`payment_status: pending` and still pending after the auth-challenge auto-charge — payment is registered by nobody and nothing is usable.
- **AC2** (`t02-activation.json`, `t02-duplicates.json`): customer B subscription transitions to `active` exactly once — one succeeded provider payment, one finalized paid invoice, one entitlement list; every duplicate re-registration attempt (subscription re-POST `422`, `retry_payment` not-allowed, manual re-POST forbidden) leaves that state byte-identical.
- **AC3** (`t02-retries.json`): response-loss recovery via the same `external_id` returns the same Lago subscription (no second commercial object); retry on the pending gate re-attempts without creating a second payment row; recovery never depends on a new identity.
- **AC4** (`t02-manual.json`, `t02-decline.json`): manual Payment registration is **forbidden** on Community (Premium-gated) with state unchanged; the decline control shows a failed provider payment closing the invoice and canceling the subscription (`payment_failed`) — activation is success-gated, and the decision document names the supported path or blocker.
- If Docker is unavailable, the API key cannot be seeded, or the Stripe test key is missing/Stripe is unreachable from the API container (preflight with `docker compose … exec api curl -sf https://api.stripe.com/v1/…`), the affected phases record `blocked-env` and nothing is claimed as passed.

- [ ] **Step 4: Promote and review the evidence**

Copy the sanitized phase reports into `docs/migrations/lago/t02-payment-activation/`, write `t02-run.txt` (sanitized operator timeline), complete the evidence README table, then re-run the secrets scan over the tracked tree (`grep -r` for every live secret value; zero hits). Commit the runner, README, and evidence.

### Task 4: Decision document and cross-ticket notes

**Files:**
- Create: `docs/migrations/lago/t02-payment-activation/DECISION.md`
- Update (if the real run surfaced anything): `deploy/lago-lab/payment-activation/README.md`

**Interfaces:**
- Consumes: all Task 3 evidence files and the *Pinned-release findings*.
- Produces: the Ticket #74 decision — which recording path Tickets #81/#82 build on, with blocker evidence.

- [ ] **Step 1: Write the decision document**

Structure (evidence-linked, no unverifiable claims):
1. **Question:** manual Payment via API vs Lago-supported external Payment Provider for the WeKnora purchase flow.
2. **Manual path verdict:** Community v1.53.0 returns forbidden (`t02-manual.json`; source: `Payments::ManualCreateService` premium gate). Note from source that Premium manual payments would flow through invoice `payment_status` into the same activation rule resolution — i.e., the mechanism exists but is license-gated. State the cost: Premium license (out of scope per spec).
3. **Provider path verdict:** proven end-to-end on Community (`t02-activation.json`): gated subscription → provider-driven charge → rule satisfied → invoice finalized → subscription active, exactly once (`t02-duplicates.json`), with success-gated decline behavior (`t02-decline.json`) and same-identity recovery (`t02-retries.json`).
4. **Blocker statement for WeChat/Alipay:** Community has no supported API to record an externally completed channel payment against a gated invoice — manual is Premium-gated; the provider set is fixed (stripe/gocardless/adyen/cashfree/flutterwave/moneyhash); Stripe payment-method types are allowlisted without `wechat_pay`/`alipay`; forging provider webhooks or writing local state is excluded by spec. Record the source citations.
5. **Options for #81/#82** (each with evidence links and what must be escalated to spec/ADR owners): (a) acquire Premium and use manual Payment recording driven by WeKnora-verified channel facts; (b) adopt a Lago-supported provider as the actual charging rail where feasible and keep WeKnora channels only where the provider cannot serve them (requires product decision on CN payments); (c) propose a spec/ADR change if neither is acceptable. The lab recommends (record which, and why) and explicitly marks anything unproven.
6. **Cross-ticket notes:** any requested changes to `deploy/lago/` (expected: none), any deviation between the pinned runtime and the documented API contract observed during the run, and the rule-timeout (`timeout_hours` + hourly clock) semantics documented from source but not observed at runtime.

- [ ] **Step 2: Final verification pass**

Run: `python3 -m unittest discover -s deploy/lago-lab/payment-activation -p 'test_*.py' -v`

Run: secrets scan over `git ls-files` of both new directories; verify every acceptance criterion maps to a committed evidence file referenced from `DECISION.md`; verify the shared assets are untouched — `git status --short deploy/lago deploy/openmeter` must be empty.

- [ ] **Step 3: Commit the decision slice**

Commit `DECISION.md` and documentation updates.

## Plan Self-Review

- **Spec coverage:** the four #74 acceptance criteria map one-to-one to experiment phases and evidence files (AC1→gate, AC2→activate+duplicates, AC3→retries, AC4→manual+decline+DECISION). The spec's four vendor blockers named for the pinned release include "external payment releasing the activation rule" — this lab resolves exactly that one. Spec invariants respected: no local force-active, no Premium bypass, no dual-write, CNY/integer-cents pricing deferred to later tickets.
- **Research-grounded design:** every phase's expected outcome is grounded in the v1.53.0 source findings (activation-rule engine, premium gate on manual payments, synchronous provider success, exactly-once guards, active-only entitlements). Where runtime may deviate from source or docs, phases record `contract_notes` instead of silently adapting.
- **Execution detail scan:** each task names files, commands, and expected results; unit tests run without Docker/Stripe; the only Docker/Stripe-dependent step is Task 3's real run, explicitly allowed to run off-peak; blocked-env paths are defined for every external dependency.
- **Interface consistency:** Task 1 exports `lab.sh` + `clients.py` consumed by Task 2; Task 2 exports phase report dicts consumed by Task 3's runner; Task 3 exports the evidence bundle consumed by Task 4's decision.
- **Review focus:** the six failure cases above are each covered by named tests (isolation invariants, redaction + secrets scan, verdict classification, final-state assertions on duplicates, premium-gate handling, cleanup-in-`finally`).
- **TDD:** Tasks 1–2 are strict RED → GREEN → commit; Tasks 3–4 are scripted real-runtime verification and documentation, with the offline test suite and secrets scan as their gates.
- **Known risks:** (1) requires an operator-provided Stripe test key and outbound `api.stripe.com` from the Lago API container — preflighted and `blocked-env`-safe; (2) the stable-incomplete window depends on the 3DS-challenge card behavior (finding 6) — if the runtime instead fails the payment, the gate phase degrades to observing `canceled(payment_failed)` and the plan's AC1 evidence falls back to the pre-charge observation window between subscription creation and the first payment attempt; (3) full `timeout_hours` expiry cannot be observed in a lab session (hourly clock) — documented from source, not runtime.
