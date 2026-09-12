# T08 Web knowledge graph follow-up

Date: 2026-09-12

## Scope

This slice replaces the React graph placeholder with a bounded, server-backed
Wiki graph view. The shared API client now exposes the documented
`GET /api/v1/knowledgebase/{kb_id}/wiki/graph` contract, strictly validates
nodes, edges, and metadata, and encodes overview/ego query parameters. The
Web page supports overview loading, title/slug and page-type filtering, bounded
ego expansion, explicit loading/error/empty states, keyboard-accessible SVG
nodes, and a readable node list fallback.

## TDD evidence

- The graph utility test first failed because `apps/web/src/knowledge/graph.ts`
  did not exist.
- The API-client graph tests first failed because `createWikiPagesApi` did not
  expose a `graph` method.
- After the implementation, the focused graph tests passed 2/2 and the focused
  Wiki API tests passed 6/6.
- A follow-up RED test caught that changing the type filter after ego expansion
  could lose the active center; `7cdece5` centralizes query construction and
  preserves the current mode/center. The focused graph suite now passes 3/3.

## Verification

All commands ran in the isolated `codex/react-multiclient` worktree:

| Check | Result |
|---|---|
| `pnpm test:web` | 0; 102/102 |
| `pnpm typecheck:web` | 0 |
| `pnpm test:shared` | 0; 186/186 |
| `pnpm typecheck:shared` | 0 |
| `pnpm build:web` | 0; 132 modules; existing `>500 kB` warning remains non-fatal |
| `node scripts/check-react-boundaries.mjs` | 0 |
| `git diff --check` | 0 |

The shared Web renderer also passed the adjacent Embed and desktop checks:
`pnpm test:embed` 3/3, `pnpm typecheck:embed` 0, `pnpm build:embed` 68
modules, `pnpm test:desktop` 2/2, `pnpm typecheck:desktop` 0, and
`pnpm build:desktop-renderer` 133 modules. The desktop build has the same
existing non-fatal `>500 kB` warning.

This is static, Node-test, and production-bundle evidence. It does not claim
full browser graph interaction, graph performance at large knowledge-base
size, role/tenant negative coverage, or production backend acceptance. T08
remains `review` pending those gates and the broader FAQ/Wiki parity checks.
