# Baseline and canonical inventory evidence (2026-09-12)

Historical snapshot notice (2026-09-16): this file records the 2026-09-12 baseline, not the current route denominator. The current `route-parity.csv` has 59 data rows and no explicit `row_id` column; the 53/56 row counts and R001-R053/R001-R056 references below are retained as historical evidence. Current drift and missing Apps authority paths are recorded in `artifacts/route-inventory-parity-ledger-audit-20260916.md`.

## Workspace baseline

- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`.
- Branch: `codex/react-multiclient`.
- Task base / initial HEAD: `9b79558b6229d79d0ceebe22e1de4a439982c615`.
- Latest frontend source commit: `a62ad2dcae7befd68afe0454bf58f739cfdebdcb` (`fix: cache React embed assets through nginx`).
- Latest Vue router source commit / migration inventory source commit: `5cf093706ebecdfe8bc4eca80886e80c01805289` (`fix(chat): preserve steering continuity and composer focus`).
- Applicable repository instructions: no root `AGENTS.md` or `CLAUDE.md` found in this worktree; `cli/AGENTS.md` exists but is out of scope because Task 1 does not touch `cli/`.
- Ignored scratch: `.superpowers/sdd/.gitignore` contains `*`; SDD brief/ledger/report are not committed.

## Initial status and ownership

Parent context named active unrelated dirty files to preserve/not stage: `apps/mobile/expo-env.d.ts`, `apps/web/src/chat/ChatRoutePage.tsx`, `apps/web/src/chat/send-run.ts`, `apps/web/src/chat/send-run.test.ts`, `packages/api-client/src/identity/tenant.ts`. The first Task 1 status probe printed `M apps/mobile/expo-env.d.ts`, `M packages/api-client/src/identity/tenant.ts`, and untracked `docs/superpowers/*`; current `git status --short` later showed only this task's four documentation changes. The named files remain explicitly out-of-scope and unowned by this commit.

Files owned by this task:

- `docs/migrations/react/vue-react-parity-matrix.md`
- `docs/migrations/react/vue-react-parity-progress.md`
- `docs/migrations/react/evidence/vue-react-parity/2026-09-12-baseline-and-inventory.md`
- `docs/migrations/react/evidence/vue-react-parity/README.md`

## Existing local services observed

`lsof -nP -iTCP -sTCP:LISTEN` showed the relevant parity ports already in use:

- `node` on `[::1]:5181` (React dev server)
- `node` on `*:5180` (Vue dev server)
- `server` on `*:8080` (Go app backend)
- `node` on `*:18090` (mock OpenAI-compatible stream model)

No service was started, stopped, or reconfigured for this documentation task.

## Test account and data identifiers from existing evidence

- Test account: `parity-test@local.dev / Parity123456!` in the local isolated database.
- Temporary mobile input password used during simulator tests and restored afterward: `Parity12345678`.
- Test tenant: `10000`.
- Test KBs: `产品知识库`, `Parity KB Demo`, `parity-faq-kb`.
- Chat session screenshot matrix identifier: `80fd9cf5...`.
- iOS simulator evidence: iPhone 17 Pro simulator, iOS 26.5, UDID prefix `5EECD8BB`, native dev client `com.weknora.mobile`.
- Android simulator evidence: `test36-small`, `emulator-5554`, API host `10.0.2.2:8080`.
- Mock stream model: `KnowledgeQA mock-stream-model` via local server `:18090`.

## Inventory method and counts

Inputs read/reconciled:

- `docs/migrations/react/route-parity.csv` — 53 source rows.
- `docs/superpowers/plans/2026-09-10-react-migration-inventory.md` — static Vue/API/Swagger inventory.
- `docs/superpowers/specs/2026-09-10-react-multiclient-design.md` and `docs/superpowers/plans/2026-09-10-react-multiclient-migration.md` — architecture/plan constraints.
- `docs/migrations/react/vue-react-parity-matrix.md`, `docs/migrations/react/vue-react-parity-progress.md`, `docs/migrations/react/evidence/vue-react-parity/README.md`, `screenshot-matrix.md` — current migration state and evidence index.
- Source trees: Vue router, settings registry, view/component/API/i18n trees, React route resolver/render switch, all app/platform entries and shared package source trees.

Reconciled counts:

| Inventory source | Count |
|---|---:|
| route-parity.csv rows | 53 |
| canonical route/alias rows | 56 |
| canonical nested surface rows | 33 |
| canonical total rows | 89 |
| Vue `frontend/src/views` files | 171 |
| Vue `frontend/src/components` files | 94 |
| Vue `frontend/src/stores` files | 23 |
| Vue `frontend/src/composables` files | 27 |
| Vue `frontend/src/api` files | 33 |
| Vue `frontend/src/i18n` files | 21 |
| React `apps/web/src` files | 132 |
| React `apps/desktop/src` files | 9 |
| React `apps/embed/src` files | 7 |
| React `apps/mobile/app` files | 22 |
| React `apps/mobile/src` files | 66 |
| Shared `packages/api-client/src` files | 53 |
| Shared `packages/domain/src` files | 42 |
| Shared `packages/views/src` files | 32 |
| Shared `packages/i18n/src` files | 9 |
| Shared `packages/ui/src` files | 3 |
| Shared `packages/contracts/src` files | 2 |

Reconciliation decisions:

- Every `route-parity.csv` row received a stable row ID `R001`-`R053`.
- React-only route aliases `/creatChat`, `/platform/configuration`, and `/platform/administration` were added as `R054`-`R056` so they cannot be confused with Vue parity coverage. Nested tabs/dialogs/drawers/menus/previews/platform surfaces were added as `N001`-`N033`.
- Rows that previously said only `apps/web (routes.tsx) + packages/views` now name exact page, platform, API-client, domain, i18n, desktop, mobile, and embed files where applicable.
- Missing or placeholder React surfaces remain explicit `pending` rows: `R011`, `R024`, `R027`, `R031`, `R033`, `R038`, `R043`-`R046`.
- No Vue implementation file was modified.

## Commands and outputs

Commands planned for Task 1 validation:

```bash
pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts
pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts
```

Observed outputs:

- `python3` inventory sanity check: `missing_paths []`; `R row ids 56 R001 R056`; `N row ids 33 N001 N033`; `route parity rows 53`; `missing route ids []`; `duplicate ids []`.
- `pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts`: TAP `1..8`; `# tests 8`; `# pass 8`; `# fail 0`; duration `77.708459ms`.
- `pnpm --filter @weknora/mobile exec tsx --test src/features/knowledge/reference-parity.test.ts`: TAP `1..5`; `# tests 5`; `# pass 5`; `# fail 0`; duration `76.523209ms`.

Self-review result: row coverage is complete for the 53 `route-parity.csv` rows; React-only aliases and nested surfaces are explicit; all matrix file references exist; pending rows are intentionally preserved rather than accepted; no Vue implementation files were modified.

## Unresolved gaps preserved by this inventory

- `R011` command palette is only a query-preserving redirect in React; no visible `GlobalCommandPalette` equivalent is inventoried.
- `R024`, `R027`, `R031`, `R033`, `R038`, `R043`-`R046` are settings/configuration rows with exact files but pending exact Vue parity.
- Backend decisions still block final acceptance for organization `require_approval`, embed preview-token semantics, and chat approval cold-refresh persistence.
- Some file proxy and sandbox rows have unit coverage but still need final live no-permission/expired/WS interaction evidence.
- Mobile rows have native parity evidence for auth/KB basics, but each native feature family must keep platform deltas explicit.
