# R354 — Knowledge-base share management inline parity

## Scope

Authenticated edit-state comparison for the KB settings `共享管理` section,
using the local `Parity KB Demo` fixture with no existing shares.

## Vue baseline

`frontend/src/views/knowledge/settings/KBShareSettings.vue` renders the share
management surface inside the settings editor. Live Vue AX evidence showed:

- `共享到共享空间` title and explanatory text;
- `已共享到 0` count;
- `搜索共享空间…` field;
- `共享` action;
- empty state `暂无数据 尚未共享到任何共享空间`.

## React repair

React previously rendered only a section heading, description, and a button
that opened a separate dialog. The settings section now mounts the project
`KnowledgeBaseShareDialog` in inline mode. The shared component keeps the
Vue-shaped Tailwind/shadcn-compatible organization picker and permission radio
controls while adding the inline list surface, search filtering, empty state,
share creation, permission update, and unshare actions. The standalone dialog
entry remains available for other callers.

## Live evidence

React AX at
`http://localhost:5181/platform/knowledge-bases` after opening
`Parity KB Demo → 设置 → 共享管理` showed:

- `共享到共享空间` title and the same description;
- `搜索共享空间…`, `共享`, and `已共享到 (0)`;
- `尚未共享到任何共享空间` empty state.

Vue AX at
`http://localhost:5180/platform/knowledge-bases/22d38cb7-1fa5-48ed-8efc-be6f4f366640`
showed the matching `共享到共享空间`, `搜索共享空间…`, `共享`, count `0`,
and empty state. No share mutation was submitted.

## Validation boundary

- Authenticated browser/AX comparison: passed for the empty and inline states.
- Existing share-dialog tests: 13/13 passed.
- Organization/share provider success, permission-update failure, unshare
  confirmation, Wails, native, and production acceptance remain open.

