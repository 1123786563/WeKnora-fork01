# React route compatibility follow-up

Date: 2026-09-12

## Scope

This follow-up re-audited the React route resolver, guard, and initial Web
dispatcher against the registered Vue routes and the current React page
contracts. It covers:

- Safe decoding for knowledge-base, document, Wiki, FAQ, settings, and
  knowledge-base chat identifiers; malformed URI segments resolve to 404.
- Explicit `/platform/tenant` and `/platform/system/{settings,admins,queues}`
  compatibility mappings, with unknown system suffixes left as 404.
- Knowledge-base `tab`/`slug` query dispatch to documents, Wiki, and the
  explicit unsupported graph state.
- Legacy `knowledge-search?q=` mapping to `knowledge-bases?cmdk=`, and a
  fail-closed default for the development-only Markdown fixture.
- Knowledge-base chat context propagation into the stream request, Wiki slug
  selection, and organization invitation preview/join/request handling. A
  consumed invitation is removed from the browser URL only after the server
  mutation succeeds.

## Verification

All commands ran in the isolated `codex/react-multiclient` worktree:

| Check | Result |
|---|---|
| `pnpm test:web` | 0; 100/100 |
| `pnpm typecheck:web` | 0 |
| `pnpm build:web` | 0; 131 modules; existing `>500 kB` chunk warning remains non-fatal |
| `pnpm test:shared` | 0; 184/184 |
| `pnpm typecheck:shared` | 0 |
| `pnpm test:embed && pnpm typecheck:embed && pnpm build:embed` | 0; 3/3 |
| `pnpm test:desktop && pnpm typecheck:desktop && pnpm build:desktop-renderer` | 0; 2/2 |
| `node scripts/check-react-boundaries.mjs` | 0 |
| `git diff --check` | 0 |

This is static, Node test, and production-bundle evidence. It does not claim
full browser route coverage, native/Wails runtime coverage, or live
organization invitation mutation coverage. T04/T10/T11/T16 and T24 remain
`review` under the existing ledger gates.

## Runtime correction

The first live check of
`/platform/knowledge-bases/kb-route?tab=graph&slug=docs/start` still showed
Documents. The route resolver unit test passed, so the initial Web entry was
checked separately and found to pass only `window.location.pathname` to the
resolver. `apps/web/src/main.tsx` now passes pathname plus search. After a
fresh reload of the same connected Chrome tab, the page rendered `Knowledge
graph` and `selected slug: docs/start`. This correction was followed by a
fresh Web test/typecheck/build run: 100/100, 0, and 0 respectively.
