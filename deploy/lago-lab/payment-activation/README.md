# Lago T02 — payment-activation lab

Self-contained experiment lab for Ticket #74 ([Lago 02] 证明外部付款可以激活
payment-gated Subscription): it runs **its own** instance of the #73-pinned
Lago Community `v1.53.0` stack and drives a real experiment that answers
whether an externally-registered payment activates a payment-gated
Subscription exactly once, and which recording path (manual Payment vs
Lago-supported external Payment Provider) Tickets #81/#82 can build on.

The lab is fully isolated: Compose project `weknora-lago-74`, loopback-only
ports 48891 (API) / 48892 (frontend), own volumes and secrets. It never
starts, stops, or reads state of the `weknora-lago` (#73) or OpenMeter
projects, and it treats `deploy/lago/` as read-only (it reuses
`compose.yaml`, `images.lock.json`, and `health.py` from there as-is).

## Requirements

- Docker Engine with Compose v2.20+ (`start_interval` healthchecks).
- `python3` (stdlib only) and `openssl` on the PATH.
- Disk/time budget as in `deploy/lago/README.md` (same five pinned images;
  a cold `up` takes minutes; gotenberg is amd64-emulated on Apple Silicon).
- A Stripe **test-mode** key (`sk_test_…`/`rk_test_…`) for the provider
  phases. Live keys are refused outright; without a key the provider
  phases record `blocked-env` (see below) and nothing is faked.

## Operator walkthrough

All commands run from the repository root:

```bash
./deploy/lago-lab/payment-activation/lab.sh init
#    1. generates lab.env (random secrets + one-shot seed values, mode 600,
#       refuses overwrite). The seed (LAGO_CREATE_ORG=true, generated
#       LAGO_ORG_USER_EMAIL/PASSWORD/NAME/API_KEY) mints the operator
#       account + organization API key on the first `up`.
./deploy/lago-lab/payment-activation/lab.sh up
#    2. validates secrets (never Lago sample defaults, never a license flag,
#       port/URL consistency), then docker compose up -d --wait on the
#       isolated project (expect several minutes on a cold start).
./deploy/lago-lab/payment-activation/lab.sh status
#    3. health snapshot JSON for THIS project only.
STRIPE_SECRET_KEY=<sk_test_…> \
  ./deploy/lago-lab/payment-activation/run_lab.py \
    --output-dir deploy/lago-lab/payment-activation/evidence
#    4. the real experiment (phase order: setup, provider_setup, gate,
#       manual, activate, duplicates, retries, decline_control, cleanup —
#       cleanup always runs). One sanitized JSON report per phase plus
#       t02-environment.json and t02-run.txt. Exit 0 pass / 1 fail /
#       2 blocked-env.
./deploy/lago-lab/payment-activation/lab.sh down
#    5. stop the lab stack (volumes preserved; wipe with
#       docker compose -p weknora-lago-74 down -v if desired).
./deploy/lago-lab/payment-activation/lab.sh config
#       resolved Compose config with secrets redacted, anytime.
```

`run_lab.py` reads the Lago origin, the seeded API key, and the operator
login from `lab.env` (git-ignored); it takes the Stripe test key only from
the caller environment (`STRIPE_TEST_SECRET_KEY` or `STRIPE_SECRET_KEY`).
The GraphQL login JWT never appears in any output. Before finishing it
scans every written file for the secret values known to the run and
requires zero hits after scrubbing; only then may evidence be promoted to
`docs/migrations/lago/t02-payment-activation/`.

## What each phase proves

| Phase | Acceptance criterion | Expected on Community v1.53.0 |
|-------|----------------------|-------------------------------|
| `setup` | — | feature + pay-in-advance monthly plan + plan entitlement created |
| `provider_setup` | — | Stripe provider registered via GraphQL; customers A (3DS-challenge card), B (succeeding card), C (declined card) connected with default payment methods |
| `gate` | **AC1** | customer A subscription `incomplete`, entitlements endpoint `404`, gating invoice `open`/`pending`/numberless, at most one non-succeeded payment, stable across the auth-challenge window |
| `manual` | **AC4** | `POST /api/v1/payments` **forbidden** (Premium-gated), state unchanged; records the actual accepted parameter contract (`invoice_id`, `amount_cents`, `reference`, `paid_at` — no `external_id`, no `status`) |
| `activate` | **AC2** | customer B subscription becomes `active` exactly once: one succeeded provider payment, one finalized numbered paid invoice, entitlement list, totals match |
| `duplicates` | **AC2** | re-registration probes (subscription re-POST, `retry_payment`, manual re-POST) all rejected and final state byte-identical |
| `retries` | **AC3** | response-loss recovery via the same `external_id` (no second commercial object); retry on the live pending gate creates no second payment row |
| `decline_control` | AC2/AC4 negative | declined provider payment → invoice closed, subscription `canceled` with `cancellation_reason: payment_failed`, entitlements `404` |
| `cleanup` | — | everything the run created is deleted; failures reported distinctly |

## `blocked-env` meanings

A phase reports `status: "blocked-env"` when the environment cannot
exercise the contract — this is honest evidence of a gap, never a pass:

- `provider_setup`: no `STRIPE_TEST_SECRET_KEY`/`STRIPE_SECRET_KEY` in the
  caller environment, a non-test key was refused, the operator GraphQL JWT
  could not be obtained, or Stripe is unreachable.
- Any gated phase: its required predecessor objects do not exist because an
  earlier phase was blocked (e.g. no provider-connected customer).
- Transport-level unreachability of the Lago API.
- Stripe reachability from the Lago API container (recorded in
  `t02-environment.json` under `stripe.api_container_reachability`) — when
  the container cannot reach `api.stripe.com`, provider settlement cannot
  happen and the affected phases degrade to `fail` with last observed
  state; treat that combination as an environment gap, not a contract
  verdict.

Fix the environment (start the stack, provide a test key) and re-run;
never commit a blocked run as a passed contract.

## Evidence contract

- Files under `evidence/` are verbatim tool output from the real run,
  sanitized by construction (`clients.sanitize` + the runner's secrets
  scan). No API key, Stripe key, JWT, or password may ever appear.
- Synthetic objects are prefixed `weknora-t02-<run-uuid>`; no personal data.
- The final reviewable evidence bundle promoted to
  `docs/migrations/lago/t02-payment-activation/` is described by the README
  in that directory.
