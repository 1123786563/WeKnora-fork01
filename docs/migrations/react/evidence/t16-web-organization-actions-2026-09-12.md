# T16 Web organization actions evidence (2026-09-12)

- The React Web organization detail now loads members, pending join requests,
  and organization knowledge-base shares through separate typed client seams.
- Pending requests expose explicit approve/reject actions. Leave and delete
  organization actions are confirmation-gated; every mutation waits for the
  server response, clears local detail only after success, and reloads the
  organization list. Member role/removal behavior remains server-authorized.
- Web static verification after this slice: `pnpm --filter @weknora/web test`
  94/94, `pnpm typecheck:web`, and `git diff --check` passed.

This is source and Node/SSR evidence. Real role-specific 403/409 behavior,
organization share mutation, browser interaction, and cross-tenant acceptance
remain open. T16 remains `review`.
