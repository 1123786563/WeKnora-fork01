# R357 — Knowledge-base data-source create-step live parity

Date: 2026-09-15

## Baseline

Authenticated local Vue (`:5180`) and React (`:5181`) sessions opened the same
knowledge-base settings → 数据源 section in zh-CN. No data-source mutation was
submitted.

Vue `DataSourceSettings.vue` and its editor flow show a four-step create
wizard. The first visible step is `选择类型`, with connector cards; the
credential form is not shown until a connector is selected.

## Change

`apps/web/src/data-sources/DataSourcesPage.tsx` now tracks `createStep`:

- `openCreate()` opens `选择类型` with a close action;
- connector cards select the type and transition to the existing form;
- `openEdit()` still opens the existing edit form directly;
- no backend write occurs while selecting a type.

## Live evidence

React AX after opening 添加数据源 contains `选择类型`, `选择要同步的外部数据源类型`,
`关闭`, and keyboard-accessible connector buttons. Before this repair it
contained the full name/credentials/schedule form immediately. Vue shows the
same wizard entry semantics and does not show the form before type selection.

## Verification

- Browser: authenticated local Vue/React AX comparison.
- Static: `pnpm run typecheck:web` passed.
- Hygiene: `git diff --check` passed.
- Not established: Vue's curated connector subset and icons/descriptions,
  connector-specific credentials/range/sync steps, save validation/mutations,
  Wails/native, provider sandbox/live, and production acceptance.
