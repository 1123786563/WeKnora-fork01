# Ledger — Lago T07: Immutable Plan Version Publish (#79)

- Branch: `lago-79-plan-version-publish` (worktree `.worktrees/lago-79`)
- Plan: `docs/plans/2026-09-21-lago-t07-plan-version-publish.md` (approved)
- Ticket: #79 (4 acceptance criteria — all met, mapping below)

## Commits

| Task | Commit |
|---|---|
| 1 — payload + six-axis validation | 51860a10a |
| 2 — publications projection + trigger | 4c961c981 |
| 3 — seam command (fake + Lago) | bb9cde91e (+gofmt 230a358d6, 7e60134ce) |
| 4 — publish orchestration service | ba59fb643 |
| 5 — platform-gated admin API | 0b19125cc |
| 6 — real-stack evidence + docs | 9859b96c5 |

## Acceptance criteria → proof

| AC | Proof |
|---|---|
| 发布校验覆盖基础价格层级、CNY、Entitlement、Resource Quota、included Credits、费用上界 | `TestValidateForPublishValidatedSixAxes` (27 cases, table-driven) + `TestPublishValidationFailsClosedStaysDraft` (6 groups, zero seam calls) + `TestPlanAdminValidationFailureAnswers422Itemized` |
| 每个新版本获得独立 plan code，发布命令幂等 | `TestDeterministicPlanCodeAndCommandKey` + unique publication constraint (`TestRecordPublicationIdempotent`) + shared contract replay legs fake/stub-Lago + `TestPublishIdempotentAcrossReplays` + real-stack phases 1–2 (422 → read-back → exactly one plan) |
| 已发布版本不能原地修改，变更必须产生新版本 | `TestPublishedRowImmutableEveryLayer` (direct SQL UPDATE fails on the trigger) + `TestNewVersionNewCodeAndOldImmutable` + admin PATCH-published → 409 |
| 已有 Subscription 不因新版本发布而改变 | `TestPublishNeverTouchesSubscriptions` (byte-identical row) + real-stack phase 3 (subscription on v1 byte-stable across v2 publish) |

## Real-stack evidence

`deploy/lago/evidence/t07-run.txt` + `docs/migrations/lago/t07-plan-version-publish/README.md`.
`TestLagoPlanPublishIntegration` PASS on pinned v1.53.0 (project
`weknora-lago-79`, ports 48901/48902): publish+read-back (single-object GET
worked; list fallback unused), idempotent replay (exactly one plan),
subscription byte-stability, content-conflict rejection. T05 readiness
integration PASS on the same stack.

## Decisions / notes for the controller

- **Additive `PlanVersion.Name`** beyond the plan's enumerated additive
  fields: `DraftInput.Name` and the publish payload's mandatory `Name`
  need a durable home; `definition_json` is it. Backward-compatible decode.
- **Drafts store invalid commercial content** (identity-only draft input
  checks): the six axes gate at Validate/Publish — the itemized report is
  the product. Enforced by `TestPublishValidationFailsClosedStaysDraft`.
- **Admin wire omits `created`/`updated`** (no timestamp source on the
  catalog table); `published_at` rides the receipt block.
- **Migration numbers** 000179 (versioned) / 000100 (sqlite) — #78 owns
  000178/000099 per the waves doc; controller dedups at W3 integration.
- **One unrelated full-suite flake**: `TestLiveLockedBinaryTwoConsecutiveRoundsWithLocalMock`
  (internal/agent/opencode, live-binary timing) failed once under parallel
  full-suite load and passed standalone twice plus in the full re-sweep —
  no file of this ticket touches that package.
