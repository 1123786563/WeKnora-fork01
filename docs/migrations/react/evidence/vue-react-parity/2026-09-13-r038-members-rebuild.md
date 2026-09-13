# r038 — Members panel rebuild to Vue parity (react-multiclient)

- **Date:** 2026-09-13
- **Worktree:** `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient` (branch `codex/react-multiclient`)
- **Slice:** Settings → 成员管理 (TenantMembersPanel) parity rebuild
- **Baseline (authoritative, read-only):** `frontend/src/views/settings/TenantMembers.vue`
- **Gap evidence:** `docs/migrations/react/evidence/vue-react-parity/screenshots/accept-20260913/{vue,react}-settings-members.png`

## Problem (before)

The React panel rendered an English registry-leak subtitle
(`identity.tenants.members`), hardcoded English copy ("Invite colleagues and
manage tenant roles…", "Pending invitations (1)", "Refresh invitations",
"Invite member", "Revoke", "Search"), a raw invitation card
(`viewer · expires <ISO>`), an always-open invite form, sections out of order
(members before invitations), and no tables/pagination/header anatomy.

## Changed files (all owned by this slice)

| File | Change |
| --- | --- |
| `apps/web/src/settings/TenantMembersPanel.tsx` | Rebuilt: Vue section order, two data tables, pager footers, invite/share-link dialogs, permissions popover, audit region, inline popconfirms, i18n via shared keys + documented local fallbacks |
| `apps/web/src/settings/TenantMembersPanel.test.tsx` | Extended: 4 legacy tests kept (1 adapted for the dialog model), 5 new Vue-parity tests |
| `apps/web/src/settings/TenantMembersPanel.css` | Extended with Vue shell styles: section header, permissions popover, `data-table-shell` tables, `wk-tag` badges, `wk-pager` footer, popconfirm, dialog fields, audit region |

Not touched: `SettingsPage.tsx`, `ModelSettingsPanel*`, `packages/**`, any other panel.

## Ported anatomy → Vue citations

| Vue (frontend/src/views/settings/TenantMembers.vue) | React port |
| --- | --- |
| Header row: title + ⓘ permissions popover + 审计日志 text button — lines 8–52 (title 11, trigger 14–17, matrix 19–40, audit 47–50) | `.section-header-titlewrap` with `h2`, `.permissions-trigger-btn` popover (`tenantMember.permissions.*` matrix incl. 我 badge), `.header-audit-btn` |
| Subtitle + 了解 RBAC external link — lines 53–64 | `.section-description` + `a.doc-link` (same RBAC doc URL, `target=_blank`) |
| Pending invitations section header + count badge + hint — lines 77–90 | `.pending-invitations-header` (`tenantInvitation.pendingSectionTitle`, badge = server total, `pendingSectionDesc {days:7}`) |
| Invitations table (被邀请人/角色/邀请人/到期时间/状态/操作) — columns 863–872, share-link cell 108–127, role tag 128–132, status tag 137–143, copy 149–155, revoke popconfirm 160–173, pager 177–181 | `.pending-invitations-table` with 6 columns, share-link icon row (`cellTitle`/`cellAccepted`/`cellEmpty`), status badges (生效中 for active share links), copy + inline revoke popconfirm |
| Members list header: title + badge + search + add-member icon button + share-link icon button — lines 188–308 (search 195–199, invite popup 200–245, share-link popup 249–306) | `.members-list-header` with persistent search form, user-add icon button → invite Dialog, link icon button → share-link Dialog |
| Members table (姓名与邮箱 two-line / 角色 / 加入时间 / 操作) — columns 734–739, cells 329–367, pager 370–374 | `.members-list-wrap .data-table-shell` with 4 columns; owner/self rows show colored role tag (所有者 primary green), others a role `select`; remove icon button with inline popconfirm |
| Server-side pagination, 20/页 default, options [10,20,50,100] — lines 581–587, 607–608; maxPage clamp 821–827; reload on change 847–849, 947–949 | `load(page, q, pageSize)` server query + same clamp; `TablePager` footer on both tables |
| Pager footer (TDesign): 共 N 条数据 · page-size select · ‹ n › · 跳至 x /N 页 — rendered by `t-pagination` 178–180, 371–373 | `.data-table-shell__pager` with identical anatomy incl. jumper input (Enter commits, out-of-range clamps) |
| Invite dialog: 邮箱 + 角色 fields, 取消/发送邀请 — lines 200–245; sendInvitation 1331–1374 | Shared `Dialog` from the add-member button; `invitations.create`; success closes, shows `inviteSuccess`, reloads invitations |
| Share-link dialog: description + role select + 生成链接 → result + 复制邀请链接/关闭 — lines 249–306, 1275–1291 | Share-link Dialog with same two states and clipboard copy |
| Role change select (self/owner excluded) — 335–351, 1376–1417 | `updateRole` on select change; self + viewer rows get tags |
| Remove popconfirm (popconfirm instead of modal) — 353–367, removeRow 1422–1441 | Inline `.wk-popconfirm` (`confirmBody {name}` + 取消/移除), `members.remove` |
| Role permission matrix — 689–732 | Same `roleMatrix` data, ✓/✗ items, 我 badge for current role |
| Role tag themes (owner primary/admin warning/contributor success/viewer default) — 762–773 | `.wk-tag--primary/warning/success/default` |
| formatDate via Intl (numeric y/m/d + hh:mm) — 783–797 | Same `Intl.DateTimeFormat` call, locale-aware |
| Audit drawer (link target, Admin+ gated, lazy load, refresh) — 47–50, 383–511, canViewAudit 657–662, loadAuditLog 1137–1175, audit columns 983–1001 | 审计日志 button (canManage/system-admin) opens `.wk-audit-region` with description, 刷新, 6-column audit table (时间/操作人/事件/目标/请求/结果), empty/error states, lazy first-open load |
| Search debounce 320 ms — 851–859 | Kept React submit-driven search (interaction tests rely on submit; "persistent search" behavior preserved) |

Section order preserved: header → 待接受的邀请 → 空间成员 (asserted by index test).

## i18n verification (packages/i18n untouched)

All visible copy resolves through `@weknora/i18n` keys first (`tenantMember.*`,
`tenantInvitation.*`, `common.*` — 95 settings-domain keys verified present in
all five locales). The following keys are **missing from packages/i18n** and are
temporarily served by byte-exact local fallbacks in
`TenantMembersPanel.tsx` (zh-CN + en-US ported from
`frontend/src/i18n/locales/{zh-CN,en-US}.ts`; ja/ko/ru fall back to en-US until
backfill). **Do not ship locales without backfilling these:**

1. `tenantInvitation.status.pending / accepted / declined / revoked / expired`
   (only `status.shareLinkActive` exists) — 待接受 / 已接受 / 已拒绝 / 已撤销 / 已过期
2. `tenantInvitation.copied` / `tenantInvitation.copyFailed` — 已复制到剪贴板 / 复制失败，请手动选中文本
3. `tenantMember.permissions.manageMembers / manageTenantConfig / manageInfra / createOwnKB / readAll` — 管理成员 / 修改空间配置 / 配置模型 / 向量库 / IM 通道 / 创建并编辑自己的知识库和智能体 / 查看空间内容
4. `tenantMember.audit.action.rbac.*` (10 keys) and `tenantMember.audit.outcome.success / denied`
5. Pager copy (TDesign locale text, no project key exists): 共 {total} 条数据 / {size} 条/页 / 跳至 / 页 — suggest new keys `tenantMember.pager.total|sizePerPage|jumper|pageUnit`

Known quirk: `tenantMember.add.emailPlaceholder` stores the vue-i18n literal
`invitee{'@'}example.com`; the panel unescapes `{'@'}` → `@` at render time.

## TDD

- **RED** (new parity tests vs old panel): 2 pass / 7 fail — old panel had
  English strings, no tables, no pagers, no dialog.
- **GREEN** after rebuild: **9 pass / 0 fail**:

    pnpm --filter @weknora/web exec tsx --test src/settings/TenantMembersPanel.test.tsx
    ✔ tenant members keeps viewer state read-only
    ✔ tenant members exposes manager search, invite and role controls  (adapted, see below)
    ✔ member list header and localized search stay mounted during initial loading
    ✔ search remains mounted and clearing reloads the unfiltered member list
    ✔ zh-CN panel anatomy mirrors the Vue baseline: header row, two tables, pagers, no English
    ✔ permissions popover and audit drawer open from the header row
    ✔ invite dialog opens from the add-member button and sends the invitation
    ✔ pending invitation rows expose status badges and an inline revoke confirm
    ✔ members pager drives server-side pagination like the Vue table

- `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` → 0 errors
  (whole project clean at time of writing; owned files were verified clean via
  the scoped grep even while sibling panels had transient errors).

### Preserved behaviors (earlier rounds)

Server-backed member count badge (mount→load→badge update), persistent search
header across loading states, submit + clear-search reload semantics
(`q: undefined` on clear), per-member role select (`Role for <name>` a11y
label), viewer read-only gating — all legacy tests pass unmodified except one
documented adaptation:

- *Adaptation (test 2 + one assertion in test 4):* the old always-open invite
  form is the defect being fixed, so its static strings "Invite member" /
  "Send invitation" no longer exist outside the dialog. Test 2 now asserts the
  manager entry points via shared-key labels (`Add Member` icon button,
  `Role for Alice` select, `Remove` control); dialog title/CTA are covered by
  the new invite-dialog test (`邀请成员` / `发送邀请`).

### Hardcoded a11y labels kept for legacy-test stability

- `aria-label="Clear search"` on the search clear icon (legacy test asserts it).
- `aria-label="Role for <username>"` on role selects.

These are invisible attributes, not rendered copy; zh-CN visible text is fully
localized.

## Implementation notes / deliberate deviations

- `window.confirm` (old panel) replaced by Vue-style inline popconfirms for
  revoke/remove (Vue 160–173, 354–366).
- Invite surface is a centered modal (shared `Dialog` primitive) instead of
  Vue's anchored `t-popup`; Vue's two-step confirm depends on the auth store's
  `autoAcceptInvitation` flag, which the React client does not expose — the
  React dialog sends in one step (form → API).
- Audit log renders as an inline region with the Vue table columns; Vue's
  resizable SettingDrawer (1120 px), expandable detail rows/JSON, and
  IntersectionObserver infinite scroll are not ported (fixed `limit: 50`).
- Pager page-number windowing: all numbers when ≤ 7 pages, else 1 … current±1 … N.

## Remaining gaps

1. Missing i18n keys above must be backfilled into `packages/i18n/src/settings.ts`
   (plus ja/ko/ru translations); the panel's local fallbacks can then be dropped.
2. Audit drawer parity: resizable drawer shell, expandable rows (操作人 ID/目标
   ID/原始详情 JSON), cursor infinite scroll.
3. Vue select-option icons (crown/shield/edit/browse) — native `<select>`
   renders text-only options.
4. Debounced (320 ms) search-as-you-type is not ported; search stays
   submit-driven to preserve the persistent-search interaction contract.
5. Popconfirm focus/Escape behavior should be verified in a real browser pass
   (jsdom coverage asserts wiring only).
