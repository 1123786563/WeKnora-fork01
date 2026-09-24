# T10 — settlement-trigger contract probe evidence (#82)

Produced by `deploy/lago-lab/payment-trigger/run_lab.py` against the isolated
pinned Lago Community v1.53.0 lab stack (`weknora-lago-82`, API :48895 — see
`deploy/lago-lab/payment-trigger/lab.sh`) with a Stripe TEST-mode key
injected via the environment only. Every JSON was sanitized in-run
(Bearer/`sk_test`/JWT shapes redacted by `clients.sanitize`; the post-run
secret scan reported 0 hits).

Verdict and analysis: `DECISION.md` in this directory.

| File | Content |
|---|---|
| `t10-environment.json` | run identity, pinned release identity (`v1.53.0` + image digests), health/login status, Stripe key source (name only), phase statuses, secrets-scan summary |
| `t10-gated-3ds.json` | provider registration + customer A (3DS challenge card) + gated create (`activation_rules payment/timeout_hours=0`) → incomplete with the default pm imported (F7/F8 re-proof) — PASS |
| `t10-payment-probe-p1.json` | P1: Lago payments index empty across the stuck window (F5 falsified — `visible_payable_condition`); P1': the unsettled Stripe intent with `metadata.lago_invoice_id` found — PASS on the provider-side locator |
| `t10-trigger-p2.json` | P2: intent cancel 200; settle pm attach/default 200; second-pm import NOT triggered (same-pcid and fresh-pcid upserts); `retry_payment` 404 `invoice_not_found` (F3 narrowed — `invoices.visible` filter) — FAIL |
| `t10-retry-dup-p3.json` | P3 duplicate-retry guard — blocked (requires P2 activation) |
| `t10-paid-retry-p4.json` | P4 paid-invoice 405 guard — blocked (requires P2 activation) |
| `t10-cleanup.json` | every lab object deleted — PASS |

Reproduce:

```bash
cd deploy/lago-lab/payment-trigger
set -a && source ~/.zcode/issue72-stripe.env && set +a
./lab.sh init && ./lab.sh up   # cold start takes minutes; ./lab.sh status must be all healthy
python3 run_lab.py             # writes evidence/t10-*.json
```

Credential discipline: the Stripe key exists ONLY in the caller environment
(`sk_test`/`rk_test` prefixes enforced at client construction; live keys
refused); no key material appears in this directory — verified by the
runner's own secret scan AND by the repo-wide red-line grep in Task 10.
