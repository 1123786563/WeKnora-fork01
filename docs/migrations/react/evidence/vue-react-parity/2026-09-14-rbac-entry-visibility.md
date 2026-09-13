# RBAC entry visibility — organizations nav (Vue menu.ts:72-81)

Date: 2026-09-14

## Slice

Vue sidebar parity for the organizations entry's role gate: React
`PlatformShell` rendered all four nav items unconditionally; Vue
(`frontend/src/stores/menu.ts:72-84`) hides `organizations` below
`hasRole('admin')` (viewer/contributor get no shared-space management
entry). This slice lands the TDD red→green for entry visibility.

## Semantics

- Vue source of truth (`menu.ts:64-84`): `visibleMenuArr` filters
  `organizations` unless `authStore.hasRole('admin')` — ROLE_LEVEL ordering
  viewer 10 / contributor 20 / admin 30 / owner 40, so owner and admin pass
  and viewer/contributor fail.
- React implementation (`PlatformShell.tsx` me handler) self-resolves the
  role with the R017 semantics already established in
  `OrganizationsPage.tsx:256-284`: selected tenant
  (`readReactPlatformState(...)?.tenantId`) first, falling back to the home
  tenant (`me.tenant.id`); membership rows matched via
  `tenant_id ?? tenantId`; owner/admin → visible, viewer/contributor →
  hidden; `me.user.can_access_all_tenants === true` (superuser) passes the
  gate.
- Fail-open policy (deliberate, per slice instruction): an unknown/absent
  role (no matching membership — e.g. embedded/test mounts) and a failed
  `auth/me` both keep the entry visible. UI rendering only — the server
  route guard remains the real authorization boundary (Vue `auth.ts`
  carries the same SECURITY note on its localStorage-derived role).

## Changes

- `apps/web/src/platform/PlatformShell.tsx` (nav visibility only):
  `canSeeOrganizations` state (initial `true` = fail-open while identity
  resolves); membership-role resolution inside the existing `auth/me`
  handler (no extra request); `visibleNavItems` memo filtering the
  `organizations` key between `navItems` and render.
- `apps/web/src/platform/platform-shell-org-subfilter.test.tsx` (+3 cases):
  viewer membership → entry hidden (other entries stay); admin membership →
  visible; viewer + `can_access_all_tenants: true` → visible. New
  `fakeClientWithMe` / `mountShellWithMe` helpers reuse the file's
  established act/settle harness; me payloads follow `AuthMe`
  (user/tenant/memberships).

## Verification

- Red first: before implementation the viewer case failed
  (`viewer must not see the organizations entry`; 1 fail / 6 pass) — admin
  and superuser cases pass trivially while the nav is unconditional.
- Green: `cd apps/web && npx tsx --test "src/platform/"*.test.tsx
  "src/platform/"*.test.ts` → **122/122 pass** (119 baseline + 3 new;
  shell-session-list / anatomy / org-subfilter existing cases intact),
  2.26s wall.
- `pnpm run typecheck:web` → 0 errors.

## Prior-hang root-cause conclusion

The Round N+8 41s hang did not reproduce. All three new cases follow the
existing mount-then-`settle(20)` harness with a collect-then-assert shape:
DOM queries run once after the final settle, assertions operate on plain
collected values, and no `await` sits between mount and assert. The
name-pattern-filtered run and the full 122-test suite both finish in
~1.6-2.3s under a `timeout` wrapper, so no act-loop/polling hang exists in
this test shape; the earlier hang is consistent with asserting across
`await` boundaries on live DOM rather than any production-code loop.

## Limits / handoff to coordinator

- **lite-mode hiding not implemented**: Vue also hides `logout` and
  `organizations` in lite mode (`liteHiddenPaths`). The React shell has no
  lite-mode signal wired (no `isLiteMode` equivalent), so the shell cannot
  gate on it yet — needs a coordinator decision on where lite mode lives in
  React before porting.
- **capability gating not implemented**: Vue (`menu.ts:79-81`) hides items
  whose `requiredCapability` is unsupported; React nav items carry no
  capability field and the shell does not consume `me.capabilities`.
- Only the nav item is filtered; the `/platform/organizations` route and
  the org sub-filter block are untouched (the server guard is the
  boundary). Concurrent agents' edits in the worktree (`styles.css`,
  `packages/views/src/integrations/page.tsx`, r012 screenshots) were left
  untouched.
- Not claimed: browser/live verification, Vue-side runtime check, Wails.
