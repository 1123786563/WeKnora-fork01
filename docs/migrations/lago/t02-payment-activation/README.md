# Lago T02 — payment-activation runtime evidence

Sanitized, reviewable evidence for Ticket #74 ([Lago 02] 证明外部付款可以激活
payment-gated Subscription): whether an externally-registered payment
activates a payment-gated Subscription exactly once on the pinned Lago
Community `v1.53.0` runtime, and which recording path Tickets #81/#82 can
build on. Produced 2026-09-20 on the T02 worktree
(`lago-74-payment-activation-lab`) by the lab under
`deploy/lago-lab/payment-activation/` (isolated Compose project
`weknora-lago-74`, loopback ports 48891/48892, never touching the #73
`weknora-lago` project or OpenMeter).

## Evidence contract

- Every file here is **verbatim tool output or a sanitized operator
  narrative**; no hand-edited verdicts, no re-typed numbers.
- **No secret values.** No `lab.env` content, no API key, no operator
  password, no GraphQL JWT, no Stripe key. Reports are sanitized by
  construction and a secrets scan over the run output and the tracked tree
  must show zero hits.
- **A blocked environment is recorded as `blocked-env` and clearly labeled.**
  In this run the caller environment had no Stripe TEST-mode key
  (`STRIPE_TEST_SECRET_KEY` / `STRIPE_SECRET_KEY`), so every
  provider/payment-dependent phase reports `blocked-env` with its explicit
  reason. That is honest evidence of an environment gap — never a passed
  contract and never a runtime failure verdict.
- These files evidence the **Community** runtime only (no `LAGO_LICENSE`,
  never unlocked). They are not AGPL legal approval.

## Files

| File | What it proves | Status in this run | How it was produced |
|------|----------------|--------------------|---------------------|
| `t02-run.txt` | Operator timeline: isolated stack lifecycle (init → up healthy → run → status ready → down, volumes preserved), the entitlement-contract fix discovered by the first attempt, and the verbatim runner timeline | context | sanitized operator narrative + verbatim `run_lab.py` timeline |
| `t02-environment.json` | Pinned release identity (v1.53.0 + all five locked digests from `images.lock.json`), API health 200, GraphQL operator login ok, Stripe test key absent (source env var recorded by name only), per-phase statuses, secrets-scan result (0 hits) | `blocked-env` overall (Stripe key missing) | `run_lab.py` (sanitized) |
| `t02-gating.json` | **AC1** gating observation: customer A subscription `incomplete`, entitlements `404`, gating invoice `open`/`pending`/numberless, stable window | `blocked-env` — requires provider-connected customer A (Stripe key missing) | `run_lab.py` phase `gate` |
| `t02-manual.json` | **AC4** manual path: `POST /api/v1/payments` forbidden (Premium-gated), state unchanged, actual accepted parameter contract recorded | `blocked-env` — requires customer A gating invoice | `run_lab.py` phase `manual` |
| `t02-activation.json` | **AC2** activation: exactly one succeeded provider payment, finalized numbered paid invoice, entitlement list, totals match | `blocked-env` — requires provider-connected customer B | `run_lab.py` phase `activate` |
| `t02-duplicates.json` | **AC2** exactly-once: duplicate re-registration probes rejected, final state byte-identical | `blocked-env` — requires settled customer B state | `run_lab.py` phase `duplicates` |
| `t02-retries.json` | **AC3** recovery: same-identity GET recovery, retry on the pending gate creates no second payment row | `blocked-env` — requires A pending gate + B settled state | `run_lab.py` phase `retries` |
| `t02-decline.json` | Negative control: declined provider payment → invoice closed, subscription `canceled` (`payment_failed`), entitlements `404` | `blocked-env` — requires provider-connected customer C | `run_lab.py` phase `decline_control` |
| `t02-cleanup.json` | Cleanup duty: every object the run created is deleted and verified (plan/feature re-read 404) | **pass** | `run_lab.py` phase `cleanup` |

Working reports for the non-AC phases (`t02-setup.json` — feature/plan/
entitlement creation, **pass** on the pinned runtime; `t02-provider.json`;
`t02-health.json` — stack `ready` snapshot) are kept verbatim in
[`deploy/lago-lab/payment-activation/evidence/`](../../../deploy/lago-lab/payment-activation/evidence/).

## Interpretation for Ticket #74

- The lab and its runner are proven end-to-end offline (60 behavioral tests)
  and against the real pinned stack (isolated project, health `ready`,
  seeded operator login, real feature/plan/entitlement creation, real
  cleanup with verification).
- The four acceptance criteria map to `t02-gating` (AC1), `t02-activation` +
  `t02-duplicates` (AC2), `t02-retries` (AC3), and `t02-manual` +
  `t02-decline` + `DECISION.md` (AC4). In **this** run those files carry
  `blocked-env` verdicts because no Stripe TEST-mode key was available;
  the contract claims themselves are NOT asserted from this run.
- `DECISION.md` states the decision analysis, the manual-vs-provider
  verdicts grounded in the pinned source (Premium gate on
  `Payments::ManualCreateService`; provider-driven activation flow), and
  exactly what remains to be re-run once a Stripe test key is supplied.

## Regenerating

Provide a Stripe TEST-mode key and re-run the workflow from
[`deploy/lago-lab/payment-activation/README.md`](../../../deploy/lago-lab/payment-activation/README.md):

```bash
./deploy/lago-lab/payment-activation/lab.sh up
STRIPE_SECRET_KEY=<sk_test_…> ./deploy/lago-lab/payment-activation/run_lab.py \
  --output-dir deploy/lago-lab/payment-activation/evidence
./deploy/lago-lab/payment-activation/lab.sh down
```

then re-promote the sanitized reports here. Run ids and timestamps will
differ; `pass` on every phase (with zero secrets-scan hits) is the expected
stable outcome once the Stripe test key is present.
