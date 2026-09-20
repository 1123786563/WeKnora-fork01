# Lago T08 — Base Plan entitlements and monthly included credits (#80)

Evidence index for the Base-Plan ensure chain (seed → customer →
subscription → monthly credits → projection → quota enforcement), the t02–t07
convention.

## Verdicts recorded

| Verdict | Evidence |
|---|---|
| `amount_cents: 0` plan create ACCEPTED on pinned v1.53.0 | `deploy/lago/evidence/t08-run.txt` phase 1 (200 on the probe POST; the 1-fen fallback stayed unused) |
| Standard subscription without `activation_rules` reaches `active` on the free tier | phase 2 (T02 decision verified end-to-end) |
| No native grant idempotency (E3) — the three-layer identity prevents balance doubling on the real runtime | phase 3 (exactly one subscription, exactly one period wallet, unchanged balance after replay) |
| Expired wallets terminate LAZILY (E1) on the real runtime | phase 5 (short-TTL wallet stayed active past expiry; the registry expiry overlay is required) |
| Quota CAS admits N / refuses N+1 / allows −1 | phase 6 (sqlite-backed in the lab env; the lab wires no WeKnora PostgreSQL — honest sqlite-only note) |

## Artifacts

- Operator timeline: `deploy/lago/evidence/t08-run.txt`
- Tagged integration test: `internal/infrastructure/commercialplatform/lago_benefits_integration_test.go` (`//go:build lago_integration`, env-gated on `LAGO_INTEGRATION_*`)
- Shared contract (fake + stub-backed Lago, identical legs): `internal/infrastructure/commercialplatform/contract_test.go`
- Service chain tests: `internal/application/service/commercial/benefits_test.go`
- Enforcement tests: `internal/handler/commercial_benefits_test.go`, `internal/application/service/knowledge_quota_guard_test.go`, `internal/router/commercial_benefits_route_test.go`

## Stack isolation

`COMPOSE_PROJECT_NAME=weknora-lago-80`, ports 48903/48904, loopback only;
volumes preserved on `down` (T01 convention). No secret is committed — the
operator key lived at `/tmp/lago80-api-key` (mode 600) for the run only and
was deleted after.
