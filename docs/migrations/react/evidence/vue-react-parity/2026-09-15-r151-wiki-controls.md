# R151 Wiki shared controls

- Scope: `apps/web/src/wiki/WikiPage.tsx`.
- Change: Wiki search, title, slug, summary, and page-content fields now use shared `Input`/`Textarea`, preserving read/write permission gating, localized labels, controlled state, and save behavior.
- Validation: direct Wiki suite passed 5/5; prior Web typecheck and full regression remain green.
- Boundary: protected Wiki CRUD/conflict runtime and same-session Vue visual comparison remain open.
