# T08 Web UI repair independent validation

- Status: `DONE_WITH_CONCERNS`
- Revision: `3fd9c19369c45cef5be104a11f67f1cce02165eb`
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t08-web/WeKnora-fork01`
- Scope: assigned UI repair only. No source/test edits; validation report is the only file created.

## Commands and results

- `git rev-parse HEAD` — PASS; exact assigned SHA.
- `git status --short` — PASS; clean before report creation.
- `git show --stat --oneline --no-renames 3fd9c19369c45cef5be104a11f67f1cce02165eb` — PASS; repair touches Opportunity UI/test and CSS only.
- `wc -l apps/web/src/career/opportunity.css` — PASS; final file is 71 lines.
- `git diff --check 3fd9c19369c45cef5be104a11f67f1cce02165eb^ 3fd9c19369c45cef5be104a11f67f1cce02165eb` — PASS.
- `pnpm --filter @weknora/web exec tsx --test src/career/OpportunityPage.test.tsx src/routes.test.ts src/router.test.tsx src/chat/chat-route-page-send.test.ts` — PASS, 36 tests, 0 failures. Covers saved intent locked against repeat submission, explicit new-draft reset followed by fresh request ID/input, opportunity path, chat slot, and protected route. Existing CJS `import.meta` warnings arose in unrelated chat test imports.
- `pnpm typecheck:web` — PASS.
- `pnpm test:web` — PASS, 2329 tests, 0 failures/cancellations.
- `pnpm build:web` — PASS; Vite transformed 6932 modules and built successfully in 16.83s. Existing warnings include PostCSS `@import` order, invalid `calc()` whitespace, and large chunks.

## Findings

- Success state retains the receipt and evidence link, disables the saved-intent button, and offers an explicit “开始新草稿” action. That action clears JD/source metadata/receipt/attempt and returns the panel to editable idle state. The new submission is asserted to carry a different request ID and the exact new text.
- Final `opportunity.css` is 71 lines and contains only opportunity import/evidence selectors and the opportunity mobile breakpoint. The 467 removed lines were unrelated global, settings, and integration rules that had been appended to this component stylesheet; full Web tests and production build passed after removal.
- Scope risk: this CSS deletion is large (467 lines). Tests/build show no functional/build break, but no screenshot or visual regression run was performed to prove that every former global selector has equivalent styling elsewhere. Review the cascade separately if visual parity evidence is required.
- No acceptance failure found in this UI repair. Browser E2E/live API validation remains outside the evidence produced here.
