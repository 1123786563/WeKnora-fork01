# Apps Vue authority resolution (2026-09-16)

The four Apps Vue sources referenced by `route-parity.csv` are not present in
the current checkout, but they are present in repository history at the Vue
implementation commit `9b0c11c4` (`feat(web): add tenant app connections and
action approval views`). This resolves the source-location finding; it does
not by itself prove visual, permission, or mutation parity.

Verified with:

```text
git cat-file -e 9b0c11c4:frontend/src/views/apps/AppsView.vue
git cat-file -e 9b0c11c4:frontend/src/views/apps/ConnectionsView.vue
git cat-file -e 9b0c11c4:frontend/src/views/apps/AuthorizationView.vue
git cat-file -e 9b0c11c4:frontend/src/views/apps/ActionView.vue
git show 9b0c11c4:frontend/src/router/index.ts
```

| Route | Vue authority at `9b0c11c4` | React implementation |
|---|---|---|
| `/platform/apps` | `frontend/src/views/apps/AppsView.vue` | `apps/web/src/apps/AppsPages.tsx` |
| `/platform/apps/connections` | `frontend/src/views/apps/ConnectionsView.vue` | `apps/web/src/apps/AppsPages.tsx` |
| `/platform/apps/authorization/:id` | `frontend/src/views/apps/AuthorizationView.vue` | `apps/web/src/apps/AppsPages.tsx` |
| `/platform/apps/actions/:id` | `frontend/src/views/apps/ActionView.vue` | `apps/web/src/apps/AppsPages.tsx` |

Evidence layer: static source/history only.
