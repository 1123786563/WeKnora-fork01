# t11 — payment settle trigger (#82)

The R-4 dual-track mechanism probe: WeKnora drives the Stripe Provider
gated settle charge (attach settle pm → PI update payment_method →
off-session confirm) and the Lago **built-in** webhook chain finalizes the
authority (payment succeeded → invoice finalized → subscription active),
exactly once.

- Verdict and source-chain citations: `DECISION.md`.
- Evidence (sanitized, per-phase JSON + environment + timeline):
  `t11-gated-3ds.json`, `t11-settle-probe-pa.json` (P-A),
  `t11-settle-trigger-pbce.json` (P-B/C/D/E), `t11-cleanup.json`,
  `t11-environment.json`.
- Lab code: `deploy/lago-lab/payment-settle-trigger/` (skeleton restored
  from the t10 lab at `ca04d7da7`, rewritten for the dual-track).
- Boundary: Alipay sandbox credentials remain unavailable (disclosed);
  this probe's channel leg is not under test — the Stripe settle rail and
  the Lago finalize chain are.
