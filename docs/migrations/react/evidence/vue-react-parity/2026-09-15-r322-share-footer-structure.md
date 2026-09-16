# R322 share dialog footer structure

Vue `ShareKnowledgeBaseDialog.vue` keeps the `sharedTo` action in the bottom `.share-actions` row, before the flexible spacer and cancel/confirm actions. React previously rendered that action outside the form, creating a separate block and changing the footer rhythm.

React now renders the action inside the form footer with a flexible spacer, matching Vue's left/right grouping while preserving submit validation and the existing loading/disabled behavior.

Verification:

- Focused share-dialog tests: 13/13 passed, including a DOM assertion that the shared-list action is inside the form footer.
- Web suite: 911/911 passed, with 0 failed/cancelled/skipped.
- Web typecheck and `git diff --check`: passed.

Authenticated paired screenshots and real backend share/unshare execution remain open; N005 is not marked accepted.
