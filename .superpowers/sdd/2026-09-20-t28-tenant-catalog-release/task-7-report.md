# Task 7 Report: Provide the Tenant Author and Reviewer Web workflow

Status: DONE
Branch: `marketplace/t28-tenant-catalog-release`

## Implementation

- Added `createAgentMarketplaceApi`, which adapts `createAgentVersionsApi` and `createTenantReleaseApi` over the authenticated `WeKnoraClient.request` transport.
- Added an Agent editor Author panel for freezing saved Agent configurations, choosing one of the frozen versions, completing the required release Manifest metadata, and submitting the selected immutable version. It also exposes optional non-use cases, capability requirements, data categories, external side effects, and change notes.
- Added a Tenant Release review queue to the Tenant Market tab. Queue loading and decision controls are gated to Tenant Admin and Owner. The panel shows the fixed Manifest, Dependency Lock, license and bundle digest before a reviewer selects an explicit decision.
- Approval sends the selected submission's `bundle_digest` as `expected_digest` and displays the immutable Release returned in the atomic review response. Rejection and change requests require a trimmed non-empty reason. The UI offers only `approved`, `rejected`, and `changes_requested`.
- No Adoption, Variant, or upgrade flow was added.

## TDD Evidence

- **RED:** After writing both component tests, `pnpm --filter @weknora/web test` failed to load the two new files because `agent-marketplace-api.ts` was missing; the existing 2,232 tests passed.
- **GREEN, focused:** `pnpm --filter @weknora/web exec node --import tsx --test src/agent-marketplace/AgentVersionActions.test.tsx src/agent-marketplace/TenantReleaseReview.test.tsx` — 4 passed, 0 failed.
- **GREEN, full Web suite:** `pnpm --filter @weknora/web test` — 2,236 passed, 0 failed.
- **TypeScript:** `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed (exit 0).
- `git diff --check` — passed.

## Changed Files

- `apps/web/src/agent-marketplace/agent-marketplace-api.ts`
- `apps/web/src/agent-marketplace/AgentVersionActions.tsx`
- `apps/web/src/agent-marketplace/AgentVersionActions.test.tsx`
- `apps/web/src/agent-marketplace/TenantReleaseReview.tsx`
- `apps/web/src/agent-marketplace/TenantReleaseReview.test.tsx`
- `apps/web/src/agents/AgentEditorModal.tsx`
- `apps/web/src/market/MarketPage.tsx`
- `.superpowers/sdd/2026-09-20-t28-tenant-catalog-release/task-7-report.md`

## Assumptions and Remaining Risk

- The version freeze API snapshots the persisted Agent configuration. Authors should save editor changes before freezing; the panel labels the action as freezing the saved configuration.
- Task 5's production dependency resolver currently fails closed for Agent versions that reference dependencies without stable distributable versions and licenses. This UI does not invent or bypass those dependencies.
- New panel copy is English-only; this task's allowed file list did not include the shared locale catalogs.
