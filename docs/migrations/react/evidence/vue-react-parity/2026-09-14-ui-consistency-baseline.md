# React UI consistency baseline — 2026-09-14

## Purpose

This document records the read-only baseline for the planned React frontend consistency work. Vue remains the canonical visual and interaction reference. The first implementation scope is the platform sidebar, knowledge-base list, and settings modal shell.

## Repository state

- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
- Branch: `codex/react-multiclient`
- HEAD at baseline: `30700be991dc56d2935bc65f81024fb4d382c629`
- Parity inventory source recorded by the matrix: Vue `frontend/`, source commit `5cf093706ebecdfe8bc4eca80886e80c01805289`, task base `9b79558b6229d79d0ceebe22e1de4a439982c615`.

The worktree was already dirty before this baseline was written. These files are preserved and are outside this baseline task:

| State | Path | Observed change |
|---|---|---|
| modified | `apps/web/src/administration/AdministrationPage.tsx` | Tailwind utility layout/style adjustments |
| modified | `apps/web/src/data-sources/DataSourcesPage.tsx` | Tailwind utility layout/style adjustments |
| modified | `apps/web/src/styles.css` | 21-line removal from global styles |
| untracked | `docs/migrations/react/evidence/vue-react-parity/screenshots/docs-live-20260914/` | Existing screenshot evidence directory |
| untracked | `frontend/pnpm-lock.yaml` | Appeared after the Vue dev command reconciled frontend dependencies; preserve and review provenance before any cleanup |

The table is the initial snapshot taken before the Vue dev attempt. Since then,
the administration, data-source, and global-style edits have been committed by
the parallel migration work (`197ad6c5` and `5a1ee92c`); the current worktree
status must be read separately before each task.

No application code was changed for this baseline. The Vue and React dev servers
are currently reachable (`:5173` and `:5181`); Vue was started while attempting
the required fresh comparison and its dependency reconciliation created the
untracked frontend lockfile listed above. This is environment state, not a
claimed application change.

## Existing parity evidence and status

The authoritative inventory is `docs/migrations/react/vue-react-parity-matrix.md`; the running ledger is `docs/migrations/react/vue-react-parity-progress.md`. The matrix currently covers rows `R001–R056` and nested surfaces `N001–N033`, and explicitly requires state, locale, theme, role, viewport, and platform evidence before acceptance.

Relevant current rows:

- `R007` platform shell: review. Recent browser shell evidence exists, but Wails/native evidence remains open.
- `R008` settings modal: review. The shell was re-verified after recent changes; Wails/native evidence remains open.
- `R009` knowledge-base list: review. Rail, organization scope, card anatomy, upload, and share slices exist; final computed-style, real-backend, Wails/native evidence remains open.
- `N001–N005` cover the platform rail, shell interactions, KB list controls, upload and sharing surfaces. Several remain implementing/review pending same-condition browser and platform evidence.

Prior evidence is useful for locating completed slices, but must be rechecked against this HEAD whenever the cited component or global style changed. It does not replace a same-condition Vue/React comparison.

## Relevant implementation map

### Sidebar and platform shell

Canonical Vue sources:

- `frontend/src/views/platform/index.vue`
- `frontend/src/components/menu.vue`
- `frontend/src/components/UserMenu.vue`
- `frontend/src/components/TenantSelector.vue`
- `frontend/src/components/GlobalCommandPalette.vue`

React sources:

- `apps/web/src/platform/PlatformShell.tsx`
- `apps/web/src/platform/GlobalCommandPalette.tsx`
- `apps/web/src/platform/command-palette.ts`
- `apps/web/src/routes.tsx`
- `apps/web/src/main.tsx`
- `packages/i18n/src/menu.ts`
- `apps/desktop/src/platform/navigation.ts`

The baseline scope is the full shell state: expanded/collapsed rail, selected and hover states, grouped navigation, user area, tenant selector, command-palette entry, role/capability filtering, deep-link reload, and keyboard focus behavior.

### Knowledge-base list

Canonical Vue sources:

- `frontend/src/views/knowledge/KnowledgeBaseList.vue`
- `frontend/src/components/ListSpaceSidebar.vue`

React sources:

- `apps/web/src/App.tsx`
- `apps/web/src/knowledge-list.css`
- `apps/web/src/knowledge-bases/list.ts`
- `apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx`
- `apps/web/src/knowledge-bases/kb-list-icons.tsx`
- `apps/web/src/knowledge-bases/empty-kb-svg.ts`

The baseline scope is the list page anatomy: organization/space rail, expanded and collapsed rail states, page header and filters, three-column card density at desktop, card metadata and hover actions, pin/duplicate/settings/delete menu, empty/loading/error states, upload progress, share dialog, URL scope query, and responsive narrow-window behavior. `App.tsx` is a known concentration point; any later decomposition must preserve its route, permission, request, and URL behavior.

### Settings modal shell

Canonical Vue sources:

- `frontend/src/views/settings/Settings.vue`
- `frontend/src/config/settingsRoute.ts`

React sources:

- `apps/web/src/settings/SettingsPage.tsx`
- `apps/web/src/settings/settings-wrapper.css`
- `packages/views/src/settings/registry.ts`
- `packages/i18n/src/settings.ts`

The baseline scope is the modal container rather than every settings panel: overlay, width/height, close behavior, left navigation grouping, selected item state, content scroll boundary, section deep links, query preservation, role/capability filtering, loading/error/denied states, focus restoration, Escape, and nested dropdown/dialog behavior. Individual panels are downstream work and should not be reformatted globally until the shell contract is stable.

## Baseline comparison protocol for implementation

Use the same authenticated account, tenant, role, data fixture, locale, theme, and viewport on both applications. Start with `1440×900`, then check `1280×800` and `390×844` where applicable. Record Vue and React screenshots, computed dimensions/spacing/colors for disputed elements, and the trigger/result sequence for behavior differences. Required states are normal, loading, empty, error, no-permission, disabled, editing, submitting, success, and failure whenever the surface supports them.

For each discrepancy, record the matrix row, Vue source anchor, React component/style owner, reproduction steps, expected result, actual result, severity, and evidence path. Do not classify a difference caused by role, tenant, data, theme, or viewport as a UI defect until the conditions are equalized.

## Concrete discrepancy register carried into this run

The local browser harness could not connect because Chrome is waiting for the
remote-debugging permission, so the entries below are carried from evidence
captured against this branch and are explicitly marked `reverify-required`.
They are candidate implementation inputs, not current acceptance claims.

| ID | Matrix | Vue source | React owner | Expected → observed | Evidence / severity |
|---|---|---|---|---|---|
| B-001 | N001/R007 | `frontend/src/components/menu.vue` `.submenu` | `apps/web/src/platform/PlatformShell.tsx`, `packages/views/src/chat/session-sidebar.tsx`, `apps/web/src/platform/shell.css` | Sessions are visible under the platform nav on every protected page → prior React implementation kept the list inside chat routes and could duplicate the sidebar | `2026-09-13-shell-session-list.md`; P1 until reverified |
| B-002 | R009/N005 | `frontend/src/views/knowledge/KnowledgeBaseList.vue`, `frontend/src/components/ListSpaceSidebar.vue` | `apps/web/src/App.tsx`, `apps/web/src/knowledge-bases/list.ts`, `apps/web/src/knowledge-bases/upload-progress.ts`, `apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx`, `packages/domain/src/knowledge/list.ts`, `packages/api-client/src/client.ts` | Vertical scope rail, compact card grid, Vue action set and equivalent empty/error states → historical React slice required parity recheck after shared CSS changes | `2026-09-14-r009-kb-list-anatomy.md`; P1 until reverified |
| B-003 | R008/N014 | `frontend/src/views/settings/Settings.vue`, `frontend/src/config/settingsRoute.ts` | `apps/web/src/settings/SettingsPage.tsx`, `apps/web/src/settings/settings-wrapper.css`, `packages/views/src/settings/registry.ts`, `packages/api-client/src/settings/index.ts`, `packages/domain/src/settings/surface.ts` | Settings section spacing, TDesign green selected controls, 32px selects, overlay and nested-layer behavior → prior React slice had old section margins and blue selected states | `2026-09-13-settings-visual-polish.md`; P1 until reverified |
| B-004 | N002/R007 | `frontend/src/components/UserMenu.vue`, `frontend/src/components/TenantSelector.vue` | `apps/web/src/platform/PlatformShell.tsx`, `apps/web/src/platform/scope-runtime.ts`, `apps/web/src/platform/adapters.ts`, `apps/web/src/platform/legacy-session.ts`, `apps/web/src/platform/credentials.ts` | User/tenant actions remain available and role-gated after scope changes → must be checked with owner and viewer fixtures | shell evidence and matrix N002; P2 |

The React dev server is reachable on `:5181`; the Vue dev server was restarted
for a live check and served `/login`, but is not currently running after the
check (`:5173` is unavailable). The Vue browser tab is unauthenticated while
the React tab is authenticated. A fresh same-condition screenshot/computed-style
pass therefore still requires an authenticated Vue session and a running Vue
server. T01 remains `blocked-env` until that pass is collected.

## 2026-09-15 public auth recheck

The unauthenticated login and registration routes were opened concurrently at
Vue `:5173` and React `:5181` with the same locale and viewport. The React
registration card now computes to `480px` wide, matching Vue's `.form-card`
(`480px`); its heading computes to `24px`/`600`, and its return action is a
green `500`-weight link with hover underline. The four registration labels
expose the same required markers (`*用户名`, `*邮箱`, `*密码`, `*确认密码`).
The login card's `box-border` fix and green create-account outline were also
rechecked in the browser. These findings are limited to public auth routes;
protected route, role, real-backend, Wails, and native evidence remains open.

## Known blockers and boundaries

- Protected-page live browser comparison was not run in this baseline; the public auth recheck above is a later, separately scoped result.
- The discrepancy register above is historical evidence carried forward and is not claimed as a current result; it exists so implementation can begin from concrete owners and expected behavior while the browser gate is resolved.
- Wails/native acceptance is still open for the relevant rows.
- Real-backend upload/share/settings scenarios require suitable authenticated fixtures and must be recorded separately from static or mocked rendering evidence.
- Existing dirty files and the untracked screenshot directory must remain untouched while implementing this scope.
