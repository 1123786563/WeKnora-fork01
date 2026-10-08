# Lago T09 — Quote → gated Invoice + incomplete Subscription (#81)

Evidence index for the payment-gated purchase chain (frozen Quote →
provider-binding ensure → `create_purchase_subscription` → quote-vs-authority
match gate → awaiting-payment product state), the t02–t08 convention.

## Verdicts recorded

| Verdict | Evidence |
|---|---|
| Gated create requires a synced default payment method (NEW fact F11): binding a provider alone answers 422 `no_default_payment_method` | `deploy/lago/evidence/t09-run.txt` §2 probe 3; the adapter now attaches + promotes a configured PM token during the binding ensure and polls the authority's import (`lago_purchase.go waitForPaymentMethodSync`) |
| The provider CLONES shared `pm_card_*` ids on attach; the default update must reference the customer-scoped id | t02 phases.py precedent, re-proven in the t09 run (`providerAttachDefaultPaymentMethod`) |
| After the gated create the authority may advance incomplete→active within seconds when the provider auto-collects successfully (F9 full path) | run §3 step d (purchase status answered `active` under a collectable test PM); the awaiting-payment snapshot IS the creation-instant truth — asserted in the tagged integration test Phase 2 |
| The gating invoice is PRORATED (first period), NOT `plan_amount_cents` (9900 → 2540 for a mid-period create) | run §4 (DB authoritative read: invoice fee 2540 vs plan 9900). This STRENGTHENS decision D2/option A: a pre-payment invoice-total comparison was never readable AND would not equal the plan price anyway; the payment-time full re-check (#82/#84 via `PurchaseSnapshot.InvoiceFees`) is the only complete guard |
| Quote-vs-authority mismatch aborts with ZERO channel payment requests (AC2) | run §3 step i (drifted quote → HTTP 409 `invoice_quote_mismatch`, orders list empty) |
| Command retry creates NO second subscription/invoice (AC4) | run §3 steps f/g/h (retry over fresh quotes; `subscriptions` count for the purchase identity = 1; subscription-type invoice count unchanged 2→2 across the retry) |
| Replay NEVER re-POSTs (F7 deferred-terminate risk) | stub test `TestLagoCreatePurchaseReplayNeverReposts` (1 POST, 1 customer POST across the replay) + integration Phase 4 |
| Concurrent plan change is a definitive conflict; entitlements stay CLOSED while gated (AC3/AC4) | tagged integration Phases 3/6 (404 entitlements while gated; different plan → `ErrPlatformInvalidResponse`, count stays 1) |

## Artifacts

- Operator timeline: `deploy/lago/evidence/t09-run.txt`
- Tagged integration test: `internal/commercial/commercialplatform/lago_purchase_integration_test.go`
  (`//go:build lago_integration`, env-gated on `LAGO_INTEGRATION_*`; skips honestly without a stack)
- Seam additive types: `internal/commercial/purchase_command.go`
- Adapter implementation + tests: `internal/commercial/commercialplatform/lago_purchase.go`,
  `lago_purchase_test.go`; shared contract legs in `contract_test.go`
- Service: `internal/commercial/service/commercial/purchase.go` (+ tests)
- HTTP surface: `internal/handler/commercial.go` (Purchase/PurchaseStatus),
  `internal/router/routes_commercial.go` (POST /commercial/purchases, GET /commercial/purchase),
  route tests `internal/router/commercial_purchase_route_test.go`
- Decision record (D1/D2/D3 + spec L121 deviation): `docs/migrations/lago/t09-quote-invoice/DECISION.md`

## Environment notes

- Stack: the operator's running `weknora-lago-*` compose project (pinned
  v1.53.0), API 127.0.0.1:48889, loopback only. Credentials were read from the
  running containers / `~/.zcode/issue72-stripe.env` at run time
  (environment ONLY; nothing committed, logged or echoed — the S3 red-line
  grep in the run record covers the committed artifacts).
- WeKnora backend for the API-chain evidence ran on sqlite
  (`DB_DRIVER=sqlite`) because the #80 benefits EnsureSchema still emits a
  SQLite-only `AUTOINCREMENT` DDL on the PostgreSQL path — a pre-existing
  defect recorded in the ledger for a follow-up ticket, not a #81 scope item.
- Channel payment adapters (wechat/alipay) have no credentials in this
  environment; checkout answers the closed 503 `payment_provider_unconfigured`
  AFTER the gated subscription and the match gate both succeeded — the honest
  blocked-env posture. Order-level idempotency (AC2/AC4 channel assertion
  `creates == 1/0`) is covered by the stub-provider unit tests.
