# T08 frontend validation — #126 role controls

Status: **DONE_WITH_CONCERNS**. This is a frontend-only validation of the assigned role projection. It does not attest server authorization, audit, data persistence, or listing/direct/preview/download revocation.

## Revision and integrity

- Source worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t08/WeKnora-fork01`
- `git rev-parse HEAD`: `dad8236768b1de88764c8780e017cc3615db1aa2`
- The UI files are unmodified in the current worktree and match the assigned checkpoint:
  - `packages/views/src/craft/access.tsx`: `1f4ef4abb918e7d7395505cc7705058bb057bb2bae0cf6384cbbf3e2a67f7f83`
  - `packages/views/src/craft/access.test.tsx`: `81743c6d700a876bc56fa71d669707fed0e1f6b9e20b9e0d751c48bc829bc0b7`
  - checkpoint manifest: `3bc31674ae9370bf51a675599ec6ed2f0e0a01421645b571a9a7ffe38796e3d1`
- `git status --short` contains backend fix files and reports only; no UI source/test edits.

## Commands and results

- `pnpm exec tsx --test packages/views/src/craft/access.test.tsx` — exit 0; 1 test passed, 0 failed. This was run on the hashes above.
- Reused same-checkpoint evidence from `docs/plans/2026-09-23-craft-107-t08-report.md`: `pnpm typecheck:web` — recorded exit 0. The report states the subsequent changes were backend-only; the UI files still match its recorded checkpoint hashes.
- No browser, responsive, screen-reader, or live API test was run. The component has no usage outside its own file/test in `packages/views/src`, so this checkpoint provides no rendered product route to exercise.

## Acceptance evidence and gaps

- **Owner grant/revoke projection:** present. Owner sees a labeled user ID input, role selector limited to Viewer/Collaborator, Add member submit button, and Revoke buttons for non-owner members. The existing focused test checks presence of these controls.
- **Collaborator/Viewer read-only projection:** implementation hides grant/revoke controls for both non-Owner roles. Existing test exercises Viewer only; Collaborator projection is not asserted.
- **Accessibility:** native `section`, list, form, labels, input, select, and buttons provide semantic controls and names. No explicit validation feedback or async status/error announcements exist; no keyboard or assistive-technology browser check was performed.
- **Loading/error/success/revocation states:** missing in the component. Promise results from grant/revoke are discarded; submission clears the user field immediately; no pending disablement, success confirmation, failure message, retry affordance, or revoke status is rendered. This leaves async failures silent and lets users repeat actions without feedback.
- **Responsive/browser behavior:** no component styles or browser test exist. Long user IDs have no wrapping/overflow constraint in this component, and narrow viewport layout was not verified. Actual appearance depends on its eventual host page/global styles.
- **Integration:** no application code imports or renders `CraftAccess` in the current view tree. This validates the isolated projection only, not that an Owner can reach it in the product.

The role-control visibility matches the assigned frontend projection, but interaction-state, collaborator-test, and integration/browser evidence remain gaps. Server-enforced authorization and audited mutation are backend acceptance items and are outside this frontend validation.
