# R017 organizations RBAC + shell sub-filter evidence — 2026-09-13

## Scope

- Vue authority: `frontend/src/views/organization/OrganizationList.vue` (RBAC gating), `frontend/src/components/ListSpaceSidebar.vue` (mode="organization" sub-filter), `frontend/src/stores/auth.ts` (`hasRole` / `canAccessAllTenants`), `frontend/src/stores/menu.ts` (organizations entry gate), `frontend/src/views/organization/OrganizationSettingsModal.vue` (isAdmin composition).
- React surface (owned by this slice): `apps/web/src/organizations/OrganizationsPage.tsx`, `apps/web/src/platform/PlatformShell.tsx` (organizations sub-filter block only), plus the two test files.
- Fixed test locale: `zh-CN`.

## Vue semantics conclusion (verified against source)

1. **menu.vue has NO organizations submenu.** The organizations nav entry in `menu.vue` renders only a pending-join-requests badge (`orgStore.totalPendingJoinRequestCount`, menu.vue:93-96). The 全部/我创建的/我加入的 sub-filter with count badges lives in the **page-level** `ListSpaceSidebar mode="organization"` rail (`OrganizationList.vue:3-4`), driven by `spaceSelection: 'all' | 'created' | 'joined'` with counts `countAll/countCreated/countJoined` (collapsed-strip tooltips format `"name (count)"`, ListSpaceSidebar.vue:237-239).
2. **RBAC write gating** (`OrganizationList.vue:499-504`): `canManageOrg = authStore.hasRole('admin') || authStore.canAccessAllTenants`. `hasRole` orders tenant roles viewer(10) < contributor(20) < admin(30) < owner(40) (`auth.ts:173-181`), so **owner and admin** keep the write affordances; **contributor and viewer** get them disabled. `canAccessAllTenants` (cross-tenant superuser) passes independently.
3. **Per-operation matrix** (OrganizationList.vue):
   - Join button (header + empty state): `disabled={!canManageOrg}`, tooltip swaps to `organization.rbac.needTenantAdminTip`.
   - Create button (header + empty state): same gate, same tooltip swap.
   - Card menu 删除: `v-if="org.is_owner && canManageOrg"` (hidden otherwise).
   - Card menu 退出 (leave): `v-if="!org.is_owner"` — any role, no tenant-role check.
   - Card menu 设置 (edit/settings entry) and card click: **not gated at list level**; inside `OrganizationSettingsModal.vue` the form fields require `isAdmin = (my_role === 'admin' || is_owner) && hasTenantAdmin` (modal lines 931-949). The React settings modal interior is a separate parity item (see below).

## Implemented parity

- `OrganizationsPage` accepts `role?: 'owner' | 'admin' | 'contributor' | 'viewer'` (same pattern as SettingsPage's `scopeRuntime.role()` prop). When absent, the page self-resolves from `auth/me`: active-tenant membership role (selected tenant, falling back to the home tenant like Vue's `currentTenantRole`) plus `user.can_access_all_tenants`. Identity failure keeps the legacy permissive UI (server remains the authorization boundary).
- Header/empty-state 加入/创建 buttons are disabled with the rbac tooltip for contributor/viewer; owner/admin unaffected.
- Card more-menu shows 删除 only for owned spaces **and** canManageOrg; leave stays available for joined spaces at any role; the settings entry and card click stay ungated (Vue list-level parity).
- `?scope=` convention: `readScopeFromUrl`/`writeScopeToUrl` mirror the KB list (App.tsx) — values `all|created|joined`, `all` removes the param; rail clicks sync the URL (`history.replaceState`), deep links preselect the rail.
- Rail buttons carry Vue's collapsed-strip count tooltips (`"全部 (n)"` / `"我创建的 (n)"` / `"我加入的 (n)"`).
- `PlatformShell` renders an organizations sub-filter block on `/platform/organizations` (expanded sidebar): links 全部 → `/platform/organizations`, 我创建的 → `?scope=created`, 我加入的 → `?scope=joined`, active state from `?scope=`. It reuses the KB quick-filter classes (identical visual language, no shell.css growth) and is distinguished by `aria-label="共享空间"` (`menu.organizations`).

## Verification

| Layer | Command/evidence | Result |
|---|---|---|
| Vue source | menu.vue / ListSpaceSidebar.vue / OrganizationList.vue / auth.ts / menu.ts anchors cited above | reviewed |
| TDD red | new tests written first: 6 RBAC/scope page tests + 4 shell sub-filter tests | red confirmed (6 fail + 4 fail) |
| TDD green (organizations) | `cd apps/web && npx tsx --test src/organizations/*.test.*` | 26/26 pass (baseline 19 + 7 new) |
| Platform regression | `cd apps/web && npx tsx --test src/platform/*.test.*` | 119/119 pass (baseline 115 intact + 4 new shell tests) |
| Typecheck | `pnpm run typecheck:web` | exit 1 — all 7 errors in `src/documents/**` (another agent's in-flight files: KnowledgeDocumentsPage.tsx, tags.ts, tags.test.ts); zero errors in organizations/** or platform/PlatformShell.tsx |
| Static hygiene | `git diff --check` (own files) | clean |

## Not accepted yet / registered leftovers

- `apps/web/src/main.tsx` is a shared mount file outside this slice's write scope: the one-line wiring `<OrganizationsPage … role={scopeRuntime.role()} />` should be added by the coordinator (the page already gates correctly via its auth/me fallback, so behavior is live today; the prop just removes a redundant `me()` request).
- Vue's `menu.ts:76` hides the organizations **nav entry** for tenant roles < admin and lite edition; the React shell nav still shows the entry unconditionally (capability `organizations` is route-guarded; role gating of the entry itself is a separate parity row).
- The React settings modal interior (form fields disabled unless org-admin × tenant-admin, `showTenantRoleHint`) is modal-level parity not covered by this row's page-level scope.
- menu.vue's pending-join-requests badge on the organizations nav entry (`totalPendingJoinRequestCount`) is not ported to the shell nav yet.
- `typecheck:web` cannot go green inside this slice while the documents agent's files are mid-edit; re-run after their integration.
