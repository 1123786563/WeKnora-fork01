# T08 frontend validation — #126 Task access

**Status: DONE_WITH_CONCERNS.** The current frontend component and mocked route integration satisfy the reviewed role-control behaviors. This does not verify the full #126 acceptance for server authorization, private-by-default persistence, auditing, or revoked content access.

## Revision and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- `git rev-parse HEAD`: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Frontend files are modified/untracked in the shared worktree at this HEAD; the relevant source hashes inspected were:
  - `packages/views/src/craft/access.tsx`: `3d34c8b03a6c7f1a1ca7e1d9ecf0480ccbbee42a2f6126e9e5de507091398ae6`
  - `packages/views/src/craft/access.test.tsx`: `bb6544cdf9117b1cc7c508e3e19a17bf8e6bdcc04cd3b9cabfcaf54fb967df20`
  - `apps/web/src/features/craft/routes.tsx`: `7dcb395567f7c72912f2bf2bad599b7006b2f35df19eb267d0eea9a0ca58539c`
  - `apps/web/src/features/craft/routes.access.test.tsx`: `4bd3b227f516354dbb04aa50e212d60838e7828262cc2c0f5e920257eabd22b2`
  - `packages/api-client/src/craft/index.ts`: `ea59788611574d56e6e1637700812d14901de1324457653fead6fc1bf152802e`
  - `packages/api-client/src/craft/index.test.ts`: `00be8f493a1c3ee8fd19f424af1a1706cdb81b9df364ebc4b253a1f562ed9783`
- Authority reviewed: approved Craft Spec stories 28–31 and #126/T08 Task 8 plan/acceptance in `docs/plans/2026-09-23-craft-107-implementation.md` and `docs/plans/2026-09-23-craft-107-dag.md`; prior T08 UI review `docs/plans/2026-09-23-craft-107-t08-ui-fix-review.md`.
- Only this validation report was written. No source or test files were changed by validation.

## Commands and results

- `pnpm exec tsx --test packages/views/src/craft/access.test.tsx apps/web/src/features/craft/routes.access.test.tsx` — exit 0; 14 passed, 0 failed.
- `pnpm test:shared` — exit 0; 998 passed, 0 failed, 1 skipped. This includes `packages/api-client/src/craft/index.test.ts` and the Craft access component test.
- `pnpm typecheck:web` — exit 0.

These checks ran against the hashes above at the recorded HEAD. No browser or live service was exercised.

## Findings

- **Owner controls:** Owner sees a required User ID field, Viewer/Collaborator role selector, Add member button, and revoke controls for non-Owner members. Grant trims the submitted ID and awaits the callback before clearing it. Route integration calls the access API and refetches members after successful grant/revoke; tests assert this request sequence.
- **Collaborator and Viewer:** Both roles see member data without grant or revoke controls. Component and route DOM tests cover both roles. These controls are only a UI projection; API comments and component code identify the server as authorization authority.
- **Loading, empty, error, and success:** The route announces `Loading Task access…`, renders an alert on load failure, and withholds controls if the current user's role cannot be confirmed. An empty member list renders an empty list plus the Owner form (or no controls for an unconfirmed role); there is no dedicated empty-state message. Grant/revoke pending, success, and failure feedback is exposed via `role=status`/`role=alert`, and failures keep the draft/member present for retry. Route tests cover load failure, loading during delayed response, stale Task response suppression, mutation success, and component mutation errors.
- **Accessibility and responsive behavior:** Native labeled input/select and buttons are keyboard-focusable; revoke buttons have user-specific accessible names; status/error feedback uses live-region semantics. Rows and form wrap, IDs use `overflow-wrap:anywhere`, and the component sets shrinking width constraints. Tests inspect labels, announcements, and long-ID wrapping style. No actual keyboard traversal, screen reader, browser layout, or narrow viewport rendering was exercised.
- **Private default and backend authority:** The panel does not create grants on its own and reflects members returned by the API. The frontend route harness seeds explicit roles; it does not prove newly created Tasks persist only an Owner. No frontend evidence proves cross-tenant refusal, server-side role enforcement/audit, credential/source isolation, or revocation of listing/direct/preview/download access.

## Acceptance status and limits

The frontend role projection and interaction states are supported by passing component, route, API-client, and typecheck evidence. Full T08/#126 remains **not verified** until backend and authenticated product-path evidence establishes private-by-default state, server-enforced/audited grants, and effective revocation for list, direct read, preview, and download. Browser and assistive-technology behavior also remain unverified.
