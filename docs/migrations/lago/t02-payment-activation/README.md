# Lago T02 — payment-activation runtime evidence

Sanitized, reviewable evidence for Ticket #74 ([Lago 02] 证明外部付款可以激活
payment-gated Subscription): whether an externally-registered payment
activates a payment-gated Subscription exactly once on the pinned Lago
Community `v1.53.0` runtime, and which recording path Tickets #81/#82 can
build on. Produced 2026-09-20 on the T02 worktree
(`lago-74-payment-activation-lab`) by the lab under
`deploy/lago-lab/payment-activation/` (isolated Compose project
`weknora-lago-74`, loopback ports 48891/48892, never touching the #73
`weknora-lago` project or OpenMeter). Re-run with a Stripe TEST-mode key
on 2026-09-23 (worktree `.worktrees/issue72-n74`); the files below are
that passing run (run id `5d06a277-c487-4e1b-a669-ae138032aed7`, all nine
phases `pass`).

## Evidence contract

- Every file here is **verbatim tool output or a sanitized operator
  narrative**; no hand-edited verdicts, no re-typed numbers.
- **No secret values.** No `lab.env` content, no API key, no operator
  password, no GraphQL JWT, no Stripe key. Reports are sanitized by
  construction and a secrets scan over the run output and the tracked tree
  must show zero hits.
- **A blocked environment is recorded as `blocked-env` and clearly labeled.**
  In the first attempt (2026-09-20) the caller environment had no Stripe
  TEST-mode key, so every provider/payment-dependent phase reported
  `blocked-env`; the 2026-09-23 re-run had the test key present and every
  phase reports a real verdict.
- These files evidence the **Community** runtime only (no `LAGO_LICENSE`,
  never unlocked). They are not AGPL legal approval.

## Files

| File | What it proves | Status in this run | How it was produced |
|------|----------------|--------------------|---------------------|
| `t02-run.txt` | Operator timeline: isolated stack lifecycle (init → up healthy → run → status ready → down, volumes preserved), the entitlement-contract fix discovered by the first attempt, and the verbatim runner timeline | context | sanitized operator narrative + verbatim `run_lab.py` timeline |
| `t02-environment.json` | Pinned release identity (v1.53.0 + all five locked digests from `images.lock.json`), API health 200, GraphQL operator login ok, Stripe test key present (source env var recorded by name only), per-phase statuses, secrets-scan result (0 hits) | `pass` (real run, Stripe TEST key present) | `run_lab.py` (sanitized) |
| `t02-gating.json` | **AC1** gating observation: customer A subscription `incomplete`, entitlements `404`, held across the window; the gating invoice stays API-invisible on v1.53.0 (`open`/`closed` are `INVISIBLE_STATUS`) — recorded as such | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `gate` |
| `t02-manual.json` | **AC4** manual path: `POST /api/v1/payments` forbidden (Premium-gated, HTTP 403), state unchanged, actual accepted parameter contract recorded | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `manual` |
| `t02-activation.json` | **AC2** activation: exactly one succeeded provider payment, finalized numbered paid invoice, entitlement list, totals match | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `activate` |
| `t02-duplicates.json` | **AC2** exactly-once: duplicate re-registration answered 200 with the SAME subscription (idempotent), `retry_payment` 405, manual 403; final state byte-identical | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `duplicates` |
| `t02-retries.json` | **AC3** recovery: same-identity GET recovery (same `lago_id`, one object); the gate retry creates no second payment row and the gate never activates | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `retries` |
| `t02-decline.json` | Negative control: a charge that cannot succeed never activates — with `timeout_hours: 0` the subscription stays `incomplete`, entitlements `404`, zero succeeded payments | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `decline_control` |
| `t02-cleanup.json` | Cleanup duty: every object the run created is deleted and verified (plan/feature re-read 404) | `pass` (real run, Stripe TEST key present) | `run_lab.py` phase `cleanup` |

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
  `t02-decline` + `DECISION.md` (AC4). In the re-run (Stripe TEST key
  present via caller environment) those files carry real runtime verdicts —
  every phase `pass` — so the AC1–AC4 contract claims ARE asserted from
  this run; the first attempt's `blocked-env` reports remain in git history
  as the honest environment-gap record.
- `DECISION.md` states the decision analysis, the manual-vs-provider
  verdicts grounded in the pinned source (Premium gate on
  `Payments::ManualCreateService`; provider-driven activation flow) and
  now confirmed at runtime (manual 403; provider path activating exactly
  once).

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
