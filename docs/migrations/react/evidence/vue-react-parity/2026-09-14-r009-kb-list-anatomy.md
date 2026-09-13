# R009 KB list anatomy evidence — 2026-09-14

## Scope

- Vue authority: `frontend/src/views/knowledge/KnowledgeBaseList.vue` and `frontend/src/components/ListSpaceSidebar.vue`.
- React surface: `apps/web/src/App.tsx`, `apps/web/src/knowledge-list.css`, `apps/web/src/knowledge-bases/kb-list-icons.tsx`, and `apps/web/src/knowledge-bases/empty-kb-svg.ts`.
- Fixed test locale: `zh-CN`; test fixture uses an authenticated contributor in tenant `t-1`.

## Implemented parity

- Replaced the horizontal filter chrome with Vue's vertical all/favorites/recents/workspace rail and preserved scope counts/tooltips.
- Ported the compact responsive card grid, grouped/collapsible section headers, hover-only favorite/more controls, type/feature badges, origin chip, warning banner, skeletons, and empty illustration structure.
- Matched Vue's list failure behavior: the list request error renders the same empty-state fallback and does not expose the raw error text.
- Matched Vue's card menu action set exactly: pin, duplicate when permitted, settings/delete when manageable. React-only edit/share menu entries were removed; the existing editor/share components remain available to their owning flows.

## Verification

| Layer | Command/evidence | Result |
|---|---|---|
| Vue source | Vue template/style anchors around list rail, cards, section headers, menu, and empty states | reviewed |
| TDD focused | `node --import tsx --test apps/web/src/knowledge-bases/kb-list-anatomy.test.tsx` | 7/7 pass |
| Web regression | `pnpm run test:web` | 623/623 pass |
| Type/build | `pnpm run typecheck:web` and `pnpm run build:web` | exit 0 |
| Static hygiene | `git diff --check` | exit 0 |
| Visual artifacts | `evidence/vue-react-parity/screenshots/kb-list-anatomy/after-{vue,react}-kblist-{loaded,card-hover,more-menu,error-fallback}.png` | 1440x900 paired artifacts present |

## Not accepted yet

- Current run could not re-collect browser computed-style/interaction evidence because Chrome's remote-debugging permission prompt was pending and the local browser harness could not connect.
- No new real-backend, Wails, iOS, or Android evidence is claimed by this slice.
- The surrounding shell/logo still belongs to the existing shell parity rows and is not silently counted as R009 completion.
