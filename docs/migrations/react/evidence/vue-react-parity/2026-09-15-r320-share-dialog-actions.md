# R320 knowledge-base share dialog action parity

## Vue baseline

`frontend/src/components/ShareKnowledgeBaseDialog.vue` uses a 520px TDesign dialog whose title is `organization.share.title`. In the shared-list view, settings and unshare are text-button-free icon actions; their meaning is supplied by tooltip/icon semantics. The share form uses the Vue radio-button permission group and validates the organization before submission.

## React change

`apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx` now uses the shared dialog's Vue-sized 520px panel, the Vue-only share title, and icon-only settings/unshare actions with localized `aria-label` and `title`. The existing custom organization picker, permission radio group, loading/error states, stale-load guard, confirmation, and mutation lock remain intact. `kb-list-icons.tsx` adds the shared close icon used by the action.

## Verification

- Focused share-dialog tests: 13/13 passed.
- Web typecheck: passed.
- Web suite: 911/911 passed, with 0 failed/cancelled/skipped.
- `git diff --check`: passed.
- Existing jsdom interaction coverage verifies form validation, permission payload, keyboard picker, stale-load handling, duplicate-removal prevention, confirmation cancellation, and failure recovery.

## Boundary

This slice has source-level Vue comparison and jsdom behavior evidence. An authenticated post-edit Vue/React screenshot and real-provider share/unshare backend run remain open, so N005 stays `implementing`.
