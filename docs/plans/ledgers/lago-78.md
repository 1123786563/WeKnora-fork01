# Lago T06 — Tenant→Lago Customer Mapping: implementation ledger (#78)

Branch: `lago-78-lago-customer` (worktree `.worktrees/lago-78`).
Plan: `docs/plans/2026-09-21-lago-t06-lago-customer.md`.
Per-task progress ledger: `.superpowers/sdd/2026-09-21-lago-t06-lago-customer/progress.md`.

## Commits

| Task | Commit | Content |
|------|--------|---------|
| 1 | `be99ce5ad` | additive `ensure_customer` command kind, `account` snapshot kind, `AccountState`, `AccountSnapshot`, `Snapshot.Account`, `ExternalCustomerID` on the frozen seam |
| 2 | `9b7bcc10d` | both adapters implement `ensure_customer` (Lago read-before-create) + `account` snapshot; shared contract table extended (unknown-kind substitution + per-adapter W3 legs) |
| 3 | `17f0d8f48` | migrations `000178`/`000099`, `BillingAccountStore` (Bind precedent), `BillingAccountService` (lazy ensure, recovery-by-identity) |
| 4 | `01e54c04d` | `GET /api/v1/commercial/account` with lazy ensure, container wiring |
| 5 | (this commit) | tagged real-stack customer identity evidence + final verification |

## Evidence inventory

- **Seam purity/constants:** `internal/commercial/platform_test.go` (ExternalCustomerID pure-function/format/charset; frozen token values; Validate unchanged; AccountSnapshot round-trip, zero-value nil).
- **Adapter contract:** `internal/infrastructure/commercialplatform/lago_test.go` + `contract_test.go` — read-before-create request SEQUENCE (GET 404 → POST 200; GET 200 → NO POST), sentinel mapping (POST/GET 5xx + dropped connection → unreachable; 4xx → invalid response; missing config → unconfigured), payload guard (mismatched identity refused, zero requests), account truth (linked/absent/unreachable, zero-tenant refused), Bearer key only in the Authorization header, remaining kinds frozen. Fake: same-Key idempotent replay (original receipt), US-59 same-Key advisory name refresh, new-Key upsert (one customer), fault knob (persist-then-fail), account truth honest. Shared contract table runs BOTH adapters with identical legs.
- **Concurrency (AC1):** `internal/application/service/commercial/billing_account_test.go` `TestBillingAccountConcurrentEnsureCreatesExactlyOneCustomer` (8 barrier racers → all linked, ONE customer `weknora-tenant-101`, one row); store-level `TestBillingAccountEnsurePendingCompetingSameTenantDecideOneWinner`; PG twin `TestBillingAccountPGConcurrentEnsureDecidesOneRowPerTenant` (`//go:build commercial_integration`, env-gated on `SAAS_TEST_PG_DSN` — blocked-env skip verified, real-PG run pending operator DSN); real-stack replay probe in `lago_customer_integration_test.go`.
- **Isolation (AC2):** service `TestBillingAccountTenantIsolation` (two tenants, two rows/identities, store-level cross-identity typed conflict); API `TestCommercialAccountCrossTenantNeverLeaks`; derivation has no caller-naming input (Task 1 compile-level).
- **Identity stability (AC3):** Task 1 purity test; service rename/owner-transfer test (identity byte-identical, one customer, row unchanged); adapter fake US-59 tests; real-stack rename probe.
- **Recovery (AC4):** adapter stub recovery cases (GET-hit never POSTs — a lost create resolves by identity); service `TestBillingAccountRecoversByIdentityAfterLostResponse` (persist-then-fail → pending/unreachable, row retained → clear knob → snapshot linked → MarkLinked, NO second customer); real-stack identity replay probe.
- **Provider neutrality:** router `TestCommercialAccountFailsClosedAndNeverLeaks` (nil service, unreachable fault, leaky adapter, real adapter at marker origin — no `lago`/marker/path/`external_id`/`weknora-tenant-` substring) + exact closed envelope field sets.
- **Migrations:** `migrations/versioned/000178_commercial_billing_accounts.{up,down}.sql` + `migrations/sqlite/000099_commercial_billing_accounts.{up,down}.sql`; accepted by `internal/database` sqlite migration harness (head-derived).

## Runtime verdicts recorded (Task 5)

- Real-stack run (COMPOSE_PROJECT_NAME=weknora-lago-78, ports 48899/48900): `up` all-healthchecked healthy, `status --json` overall ready / release v1.53.0 (`deploy/lago/evidence/t06-status.json`); `TestLagoCustomerIntegration` PASS in 0.81s — ensure → linked → same-Key replay identity-stable → upsert probe → rename probe → lenient cleanup → post-cleanup absent. Transcript: `deploy/lago/evidence/t06-account.txt`.
- `GET /api/v1/customers/{external_id}` addressing (external-id path): exercised directly and through the adapter at runtime — answers 200/404 as mapped.
- Create-on-external_id upsert (#74 research claim): RAW POST probe answered HTTP 200 on pinned v1.53.0 — **UPSERT confirmed at runtime**. The product path never depends on it (adapter read-before-create never POSTs an existing identity).
- Dropped-response recovery is deliberately NOT simulated against the shared stack (orphan risk) — proven by fault injection (Tasks 2/3).
- PG twin: blocked-env this session (no `SAAS_TEST_PG_DSN`); skip semantics verified; SQLite constraint/serialization tests stand in.
- Final verification: `go build ./...` green; `go test ./... -count=1` green (one earlier sweep hit `TestProvidersFromEnvRejectsPartialAlipay` in `internal/payment` — a pre-existing order/count-dependent flake that reproduces on main with `-count=3` and is unrelated to this branch, which does not touch `internal/payment`); `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe` 46/46 OK.

## Deviations / notes

- Plan-internal tension resolved: Task 3's flow spec ("if row linked → return linked") vs the test sketch's "(name updated)" on re-ensure. Implemented the explicit flow spec (linked rows answer from the local projection without re-submitting — the right posture for a lazy-ensure-on-GET); the US-59 advisory-name refresh semantics live at the command layer and are proven by the adapter/fake tests.
