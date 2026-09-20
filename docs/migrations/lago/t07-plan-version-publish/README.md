# Lago T07 — Immutable Plan Version Publish (ticket #79)

Wave 3 product slice: 套餐运营人员 drafts a plan in WeKnora, validates the
six commercial acceptance axes, and publishes a new **immutable Plan
Version** through the frozen Commercial Platform seam
(`publish_plan_version` command) to Lago — with coordinator-owned
idempotency, durable receipts readable from the 管理后台, and zero impact on
existing subscriptions.

Plan: `docs/plans/2026-09-21-lago-t07-plan-version-publish.md` (approved).
Parent spec: `docs/specs/2026-09-20-lago-billing-migration-design.md`
("Catalog, subscriptions, and entitlements"; "Commercial authority and
module seam"; "Credits and Task admission"; user stories 37–42, 48, 59;
behavior matrix 3 and 21). ADR-0012, ADR-0014.

## What landed

| Piece | Where |
|---|---|
| `publish_plan_version` command kind + typed payload + six-axis publish validation | `internal/commercial/plan_command.go`, `internal/commercial/catalog.go` (additive `Currency`/`Name`/`Charges` fields, `PriceLadder`, closed feature/quota key sets) |
| Append-only publish projection + DB immutability trigger | migrations `000179_commercial_plan_publications` (versioned/PostgreSQL) and `000100_...` (sqlite); `internal/application/repository/commercial/planversion.go` |
| Seam command on both adapters (fake + Lago) | `internal/infrastructure/commercialplatform/fake.go`, `lago.go`; shared contract legs in `contract_test.go` |
| Publish orchestration (idempotent, fail-closed, subscription non-impact) | `internal/application/service/commercial/planversion.go` |
| Platform-gated admin API (`/api/v1/admin/plans/*`, `plan_publish` grant) | `internal/handler/commercial.go`, `internal/router/routes_commercial.go`, `internal/container/container.go` |
| Real-stack evidence | `deploy/lago/evidence/t07-run.txt` (timeline below) |

## Acceptance-criteria mapping

| Ticket AC | Proof |
|---|---|
| 发布校验覆盖基础价格层级、CNY、Entitlement、Resource Quota、included Credits、费用上界 | Task 1 table-driven six-axis tests (`TestValidateForPublishValidatedSixAxes`, 27 cases) + Task 4 `TestPublishValidationFailsClosedStaysDraft` (six groups, ZERO seam calls) + Task 5 `TestPlanAdminValidationFailureAnswers422Itemized` (422 itemized endpoint answer) |
| 每个新版本获得独立 plan code，发布命令幂等 | `DeterministicPlanCode` purity test + unique publication constraint + shared contract replay legs (fake `Commands()` + stub-Lago accepted-create count) + Task 4 `TestPublishIdempotentAcrossReplays` (no-op replay / response-loss same-key retry / closed conflict) + real-stack phases 1–2 (422 → read-back verify → exactly one plan) |
| 已发布版本不能原地修改，变更必须产生新版本 | Repository trigger test (direct SQL UPDATE fails with `published_plan_immutable`, both store guard and legacy `SaveDefinition` too) + Task 4 `TestNewVersionNewCodeAndOldImmutable` + Task 5 PATCH-published → 409 |
| 已有 Subscription 不因新版本发布而改变 | Task 4 `TestPublishNeverTouchesSubscriptions` (seeded row byte-identical, no other rows) + real-stack phase 3 (subscription on v1 byte-stable across v2 publish: plan_code / external_id / status / lago_id) |

## Real-stack run (pinned v1.53.0, project `weknora-lago-79`, ports 48901/48902)

Evidence timeline: [`deploy/lago/evidence/t07-run.txt`](../../../deploy/lago/evidence/t07-run.txt).

`TestLagoPlanPublishIntegration` (tag `lago_integration`, env-gated):
**PASS** — publish+read-back (single-object `GET /api/v1/plans/{code}`
answered on v1.53.0; the list-endpoint fallback stayed unused), idempotent
replay with exactly one plan, subscription byte-stability across v2
publish, content-conflict rejection. `TestLagoAdapterIntegration` (T05
readiness against the same stack): PASS.

## Decisions recorded during execution

- `PlanVersion` gained an additive `Name` field beyond the plan's
  enumerated `Currency`/`Charges`: `DraftInput.Name` and the publish
  payload's mandatory `Name` must survive the draft→publish round-trip, and
  `definition_json` is their only home. Additive-only, backward-compatible
  decode (pre-T07 JSON leaves it empty).
- Drafts deliberately store commercially-invalid content: the six axes gate
  at Validate/Publish (fail closed there), never silently at draft time —
  this is what makes the itemized report meaningful.
- The admin version wire omits `created`/`updated` (the catalog table has
  no timestamps to source them from); `published_at` rides the receipt
  block instead.
- Plan-code scheme `weknora-<slug>-v<n>`, CNY-only integer fen, the
  `fixed_unit`/`package` charge whitelist, and the map-form entitlement
  attach all follow the pre-ruled plan decisions and the t02–t04 lab facts.
- Migration numbers 000179 (versioned) / 000100 (sqlite) per the waves doc
  (#78 is expected to take 000178/000099; controller dedups at W3
  integration — renumbering is mechanical if needed).
