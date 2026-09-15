# R313 platform navigation icon geometry

## Scope

- Route: `/platform/knowledge-bases`
- Surface: expanded platform shell navigation rail
- Fixture: authenticated `paritytester` / `Parity KB Demo`, tenant `10000`, `zh-CN`, light theme
- Viewport: Chrome side-by-side capture at approximately `1355x720`

## Vue baseline

`frontend/src/components/menu.vue` renders the navigation icons from the fixed assets `prefixIcon.svg`, `zhishiku.svg`, `agent.svg`, and `organization.svg`. Their source viewBox is `20 20`, with the knowledge-base, agent, and organization geometry distinct from generic Lucide chat/book/bot/users icons.

## React change

`apps/web/src/platform/PlatformShell.tsx` now renders the Vue asset path geometry inline while retaining the existing React navigation links, active-state classes, keyboard behavior, and Tailwind/shadcn shell composition. Multi-path icons are supported for the filled Vue shapes.

## Runtime comparison

Both `http://localhost:5173/platform/knowledge-bases` and `http://localhost:5181/platform/knowledge-bases` loaded the same authenticated fixture. The page title, warning banner, scope rail, KB card, shell widths, and session region matched the existing baseline. The React navigation icons now use the corresponding Vue geometry; no route or business-state change was introduced.

## Verification

- Focused shell tests: `30/30` passed.
- Web suite: `911/911` passed, with 0 failed/cancelled/skipped.
- `pnpm typecheck:web`: passed.
- Browser evidence: live paired Vue/React observation in the parity browser session; no persistent screenshot was written because the repository screenshot directory contains concurrent untracked artifacts.

## Remaining boundary

This is a protected authenticated `zh-CN` light-theme browser slice. Other locales, dark theme, responsive widths, role downgrade, Wails, native mobile, and protected non-empty backend state remain governed by the canonical matrix and are not marked accepted by this slice alone.
