# T11 — Settle-trigger contract probe (payment_settle_trigger lab)

**Question:** does the R-4 α dual-track activation chain hold on the pinned
Lago Community v1.53.0 — channel-collected payment + WeKnora driving the
Stripe Provider gated settle charge → Lago **built-in** webhook finalize →
active?

**Verdict: PASS** — all five criteria (P-A..P-E) verified on the pinned
stack (run `b132b9df-8ced-4240-b535-5f414bd8dfad`, 2026-09-26, exit 0;
evidence in this directory, sanitized, secrets scan 0 hits).

| # | Criterion | Observed |
|---|---|---|
| P-A | the stuck gating PI is locatable provider-side: unsettled status + `metadata.lago_invoice_id` | `GET /v1/payment_intents?customer=` → 1 unsettled intent, **`intent_status: requires_payment_method`** (t10's only observed shape, F5 — confirmed again), `lago_invoice_id: 7050f348-…` |
| P-B | settle pm attachable + settable as customer default | attach 200, default 200 (`settle_pm_attached: true`) |
| P-C | **the unverified link of D2' step (iv)**: stuck PI accepts `update payment_method` + `confirm` → succeeded | update 200, confirm 200, **`pi_status_after_confirm: "succeeded"`** (off-session synchronous charge on a `requires_payment_method` intent) |
| P-D | built-in receive chain accepts a real `payment_intent.succeeded` (body = PI read back from Stripe; real-secret HMAC) | `POST /webhooks/stripe/{org_id}?code={provider}` → **200**, `inbound_webhooks` count 1→2 |
| P-E | the webhook chain finalizes the authority; re-delivery is a no-op | subscription **active**, invoice **finalized/numbered `WEK-ED5E-006-001`/`payment_status: succeeded`**, payments **exactly 1 succeeded** (the stuck `requires_action` row stays as history), entitlements 200; same-event re-delivery → **byte-identical four objects** (`redelivery_noop: true`) |

## Source chain (pinned `getlago/lago-api@591ae900`)

- `app/services/payment_providers/stripe/base_service.rb:35-47` —
  `webhook_endpoint_shared_params`/`webhook_endpoint_destination`:
  `Stripe::WebhookEndpoint.create(url: LAGO_API_URL/webhooks/stripe/{org}?code={code})`.
- `app/services/payment_providers/stripe/register_webhook_service.rb` —
  stores the Stripe-returned `webhook_id`/`webhook_secret` on the provider
  (settings accessors, `app/models/payment_providers/base_provider.rb:34`).
- `config/routes.rb:67` + `app/controllers/webhooks_controller.rb:5-16` —
  `POST /webhooks/stripe/:organization_id`, provider lookup by the
  `?code=` query param, `head(:bad_request)` on failure.
- `app/services/inbound_webhooks/create_service.rb` → `validate_payload_service.rb`
  → `app/services/payment_providers/stripe/validate_incoming_webhook_service.rb`
  — `Stripe::Webhook::Signature.verify_header(payload, signature,
  provider.webhook_secret, tolerance: 300)`; `InboundWebhooks::ProcessJob`
  then runs the built-in handling chain.
- `app/services/payment_providers/stripe_service.rb:37-39` — provider
  create enqueues `RegisterWebhookJob` **only for new providers**; job
  failures are swallowed (rescue AuthenticationError/PermissionError).

## Boundary finding (F10 assumption falsified; ruling recorded)

The plan's F10 assumed the provider's `webhook_secret` is psql-readable
(`payment_providers.webhook_secret`). Observed on the local stack:

1. There is **no `webhook_secret` column** — it is a settings-jsonb
   accessor (`BaseProvider.settings_accessors :webhook_secret`).
2. On a local stack it is **empty**: `RegisterWebhookJob` fails with
   `Stripe::InvalidRequestError: "Invalid URL: URL must be publicly
   accessible…"` (worker log evidence, run 2) — Stripe refuses to
   register a loopback `LAGO_API_URL`, so no secret ever lands. This is a
   **local-topology boundary, not a mechanism failure** (a public Lago URL
   registers fine in production).

**Ruling (t11):** the P-D transport-leg stand-in (D8) extends to minting the
secret: the harness creates a Stripe webhook endpoint via the **same Stripe
API** `RegisterWebhookService` uses (public-format placeholder URL that
never receives pushes — the harness delivers), stores the **Stripe-generated
secret** through the Lago **model layer** (`update!(webhook_secret:)`,
never SQL), and signs the real PI event with it. Event bodies remain real
Stripe read-backs; signature verification and the whole receive/process
chain remain 100% Lago built-in. The Stripe endpoint is deleted in the
cleanup phase. Product code gains no such path (GC-3 unchanged).

## Backing for the plan

- **D2' endorsed.** All five algorithm steps hold, including the previously
  unverified step (iv): a stuck `requires_payment_method` gating PI accepts
  `payment_method` update + off-session `confirm` → `succeeded`, and the
  built-in webhook chain finalizes the authority exactly once.
- **P-A observed status word:** `requires_payment_method` (both unsettled
  shapes accepted by the locator predicate; `requires_action` remains
  accepted defensively — Lago's payment row for the 3DS window reads
  `requires_action` while the PI reads `requires_payment_method`, F7).
- Task 5's Stripe-rail implementation, Task 9's integration test, and Task
  10's `deliver_stripe_webhook.py` reuse this lab's shapes
  (`fixtures.sign_stripe_event` / `build_pi_succeeded_event`, the `?code=`
  query param, the model-layer secret read).

## Running

```bash
cd deploy/lago-lab/payment-settle-trigger
./lab.sh init && ./lab.sh up          # isolated stack weknora-lago-t11 (:48895/:48896)
set -a; source ~/.zcode/issue72-stripe.env; set +a
python3 run_lab.py                    # exit 0 = all phases pass
python3 -m unittest test_phases -v    # pure-function pins (no stack)
```

Cleanup deletes the Lago subscription/customer/plan and the Stripe
customer + minted webhook endpoint created by the run.
