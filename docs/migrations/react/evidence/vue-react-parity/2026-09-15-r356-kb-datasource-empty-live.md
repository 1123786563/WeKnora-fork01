# R356 — Knowledge-base data-source empty surface live parity

Date: 2026-09-15

## Scope

Authenticated local fixture `Parity KB Demo` (`22d38cb7-1fa5-48ed-8efc-be6f4f366640`), same Chrome viewport and zh-CN locale. Vue ran at `http://localhost:5180`; React ran at `http://localhost:5181`. The fixture returned zero configured data sources.

## Vue baseline

`frontend/src/views/knowledge/settings/DataSourceSettings.vue` renders:

- `数据源管理` and `配置外部数据源，自动同步内容到知识库` in the section body;
- no action button in the section header;
- for a manager with an empty list, a dashed `添加数据源` card inside the data-source grid;
- no `暂无数据源` status in this manager-visible empty branch.

The source confirms the grid/card tokens: 320px minimum columns, 12px gap, 10px radius, 1px dashed border, 68px minimum height, and brand hover/focus treatment.

## React result

`apps/web/src/data-sources/DataSourcesPage.tsx` now:

- removes the header-level add button;
- renders an inline Tailwind dashed add card for the successful empty manager state;
- keeps the existing shadcn-compatible `Button`, `Card`, loading, forbidden, editor, logs, resource, and API flows intact;
- applies the Vue-derived card spacing, border, radius, hover background, brand icon tile, and focus outline.

Live AX comparison:

| Surface | Vue | React |
| --- | --- | --- |
| Section title | 数据源管理 | 数据源管理 |
| Description | 配置外部数据源，自动同步内容到知识库 | 配置外部数据源，自动同步内容到知识库 |
| Empty manager action | Inline dashed 添加数据源 card | Inline dashed 添加数据源 card |
| Header action | None | None |
| Empty status copy | None in manager branch | None |

## Verification

- Browser: authenticated Vue/React AX snapshots on the local backend.
- Web regression: `pnpm run test:web`, 963/963 passed.
- Static: `pnpm run typecheck:web` passed.
- Hygiene: `git diff --check` passed.
- Not established by this round: non-empty connector card visual/action parity, connector-specific editor flows, provider sandbox/live behavior, Wails renderer, native mobile, and production acceptance.
