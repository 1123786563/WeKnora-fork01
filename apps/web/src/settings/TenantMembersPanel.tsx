import { Fragment, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import type { AuditLog, TenantInvitation, TenantMember, TenantRole, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Dialog, Input, Select, Status } from '@weknora/ui';
// T12a：可见面直译 TenantMembers.vue 的 t-tag / t-pagination / t-popup /
// t-button / t-icon；表格暂保留原生实现（偏离项见 task-12a 报告）。
import { Icon as TIcon } from 'tdesign-icons-react';
import { Button as TButton, Input as TInput, Pagination, Popup as TPopup, Tag } from 'tdesign-react';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { TenantAuditDrawer } from './TenantAuditDrawer.tsx';
import { EmptyState } from './EmptyState.tsx';
import { auditDateParts } from './SystemAuditLogPanel.tsx';

type Role = 'viewer' | 'admin' | 'owner' | 'system-admin';
type Props = { client: WeKnoraClient; tenantId: number; role: Role; initialMembers?: { items: TenantMember[]; total: number } };

const roles: TenantRole[] = ['owner', 'admin', 'contributor', 'viewer'];
const INVITATION_TTL_DAYS = 7;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
// TenantMembers.vue:627 — audit cursor pages are 50 rows.
const AUDIT_PAGE_SIZE = 50;
// TenantMembers.vue:858 — member search debounce window.
const SEARCH_DEBOUNCE_MS = 320;
const RBAC_DOC_URL = 'https://github.com/Tencent/WeKnora/blob/main/docs/RBAC%E8%AF%B4%E6%98%8E.md';

// ---- Local fallbacks for keys missing from @weknora/i18n (reported upstream; do
// ---- not edit packages/i18n from the panel). zh-CN values are byte-exact ports
// ---- of frontend/src/i18n/locales/zh-CN.ts; en-US of locales/en-US.ts. Pager
// ---- copy mirrors the TDesign locale the Vue table renders by default.
type Fallbacks = Record<string, string>;
// ja-JP / ko-KR / ru-RU intentionally fall through to the en-US fallbacks
// until the missing keys are backfilled into @weknora/i18n (reported upstream).
const LOCAL_FALLBACKS: Partial<Record<Locale, Fallbacks>> = {
  'zh-CN': {
    'tenantInvitation.status.pending': '待接受',
    'tenantInvitation.status.accepted': '已接受',
    'tenantInvitation.status.declined': '已拒绝',
    'tenantInvitation.status.revoked': '已撤销',
    'tenantInvitation.status.expired': '已过期',
    'tenantInvitation.copied': '已复制到剪贴板',
    'tenantInvitation.copyFailed': '复制失败，请手动选中文本',
    'tenantMember.permissions.manageMembers': '管理成员',
    'tenantMember.permissions.manageTenantConfig': '修改空间配置',
    'tenantMember.permissions.manageInfra': '配置模型 / 向量库 / IM 通道',
    'tenantMember.permissions.createOwnKB': '创建并编辑自己的知识库和智能体',
    'tenantMember.permissions.readAll': '查看空间内容',
    'tenantMember.audit.action.rbac.member_added': '新增成员',
    'tenantMember.audit.action.rbac.member_removed': '移除成员',
    'tenantMember.audit.action.rbac.member_role_changed': '角色变更',
    'tenantMember.audit.action.rbac.member_left': '成员退出',
    'tenantMember.audit.action.rbac.access_denied': '访问被拒',
    'tenantMember.audit.action.rbac.invitation_sent': '发出邀请',
    'tenantMember.audit.action.rbac.invitation_accepted': '接受邀请',
    'tenantMember.audit.action.rbac.invitation_declined': '拒绝邀请',
    'tenantMember.audit.action.rbac.invitation_revoked': '撤销邀请',
    'tenantMember.audit.action.rbac.invitation_expired': '邀请过期',
    'tenantMember.audit.outcome.success': '成功',
    'tenantMember.audit.outcome.denied': '拒绝',
    'tenantMembersPanel.pager.total': '共 {total} 条数据',
    'tenantMembersPanel.pager.sizePerPage': '{size} 条/页',
    'tenantMembersPanel.pager.jumper': '跳至',
    'tenantMembersPanel.pager.pageUnit': '页',
    'tenantMembersPanel.clearSearch': '清除搜索',
  },
  'en-US': {
    'tenantInvitation.status.pending': 'Pending',
    'tenantInvitation.status.accepted': 'Accepted',
    'tenantInvitation.status.declined': 'Declined',
    'tenantInvitation.status.revoked': 'Revoked',
    'tenantInvitation.status.expired': 'Expired',
    'tenantInvitation.copied': 'Copied to clipboard',
    'tenantInvitation.copyFailed': 'Copy failed. Check your browser clipboard permission.',
    'tenantMember.permissions.manageMembers': 'Manage Members',
    'tenantMember.permissions.manageTenantConfig': 'Edit workspace settings',
    'tenantMember.permissions.manageInfra': 'Configure models / vector stores / IM channels',
    'tenantMember.permissions.createOwnKB': 'Create and edit own KBs and agents',
    'tenantMember.permissions.readAll': 'Read workspace content',
    'tenantMember.audit.action.rbac.member_added': 'Member added',
    'tenantMember.audit.action.rbac.member_removed': 'Member removed',
    'tenantMember.audit.action.rbac.member_role_changed': 'Role changed',
    'tenantMember.audit.action.rbac.member_left': 'Member left',
    'tenantMember.audit.action.rbac.access_denied': 'Access denied',
    'tenantMember.audit.action.rbac.invitation_sent': 'Invitation sent',
    'tenantMember.audit.action.rbac.invitation_accepted': 'Invitation accepted',
    'tenantMember.audit.action.rbac.invitation_declined': 'Invitation declined',
    'tenantMember.audit.action.rbac.invitation_revoked': 'Invitation revoked',
    'tenantMember.audit.action.rbac.invitation_expired': 'Invitation expired',
    'tenantMember.audit.outcome.success': 'Success',
    'tenantMember.audit.outcome.denied': 'Denied',
    'tenantMembersPanel.pager.total': 'Total {total} items',
    'tenantMembersPanel.pager.sizePerPage': '{size} / page',
    'tenantMembersPanel.pager.jumper': 'Go to',
    'tenantMembersPanel.pager.pageUnit': 'page',
    'tenantMembersPanel.clearSearch': 'Clear search',
  },
  'ja-JP': { 'tenantMembersPanel.clearSearch': '検索をクリア' },
  'ko-KR': { 'tenantMembersPanel.clearSearch': '검색 지우기' },
  'ru-RU': { 'tenantMembersPanel.clearSearch': 'Очистить поиск' },
};

function interpolate(template: string, values?: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values?.[name] ?? `{${name}}`));
}

// Shared key first (@weknora/i18n), byte-exact local fallback second, raw key last.
function createPanelTranslator(locale: Locale) {
  const t = createTranslator(locale);
  return (key: string, values?: Record<string, string | number>): string => {
    const rendered = t(key, values);
    if (rendered !== key) return rendered;
    const template = LOCAL_FALLBACKS[locale]?.[key] ?? LOCAL_FALLBACKS['en-US']?.[key];
    return template === undefined ? key : interpolate(template, values);
  };
}

type Translate = ReturnType<typeof createPanelTranslator>;

// ---- Inline icons (Vue uses t-icons; the web client ships no icon package). ----
type IconName = 'info' | 'history' | 'link' | 'user-add' | 'user-clear' | 'copy' | 'close' | 'search' | 'refresh' | 'chevron-down' | 'chevron-left' | 'chevron-right';
function Icon({ name, size = 14 }: { name: IconName; size?: number }) {
  const common = { width: size, height: size, viewBox: '0 0 16 16', 'aria-hidden': true as const, focusable: false as const };
  switch (name) {
    case 'info':
      return <svg {...common}><circle cx="8" cy="8" r="6.6" fill="none" stroke="currentColor" strokeWidth="1.3" /><rect x="7.3" y="7" width="1.4" height="4.4" rx="0.7" fill="currentColor" /><circle cx="8" cy="4.8" r="0.9" fill="currentColor" /></svg>;
    case 'history':
      return <svg {...common}><path d="M8 3.2a4.8 4.8 0 1 1-4.7 5.8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /><path d="M3.2 3v3h3" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /><path d="M8 5.4V8l2 1.4" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>;
    case 'link':
      return <svg {...common}><path d="M6.5 9.5 9.5 6.5" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /><path d="M7.5 4.9l1.4-1.4a2.7 2.7 0 0 1 3.8 3.8L11.3 8.7M8.5 11.1l-1.4 1.4a2.7 2.7 0 0 1-3.8-3.8L4.7 7.3" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>;
    case 'user-add':
      return <svg {...common}><circle cx="6.4" cy="5.6" r="2.6" fill="none" stroke="currentColor" strokeWidth="1.3" /><path d="M1.9 13.4a4.5 4.5 0 0 1 9 0" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /><path d="M12.4 5.2v4M10.4 7.2h4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>;
    case 'user-clear':
      return <svg {...common}><circle cx="6.4" cy="5.6" r="2.6" fill="none" stroke="currentColor" strokeWidth="1.3" /><path d="M1.9 13.4a4.5 4.5 0 0 1 9 0" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /><path d="M10.6 10.6l4 4M14.6 10.6l-4 4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>;
    case 'copy':
      return <svg {...common}><rect x="5.4" y="5.4" width="8" height="8" rx="1.2" fill="none" stroke="currentColor" strokeWidth="1.3" /><path d="M10.6 3.4v-.2A1.2 1.2 0 0 0 9.4 2H3.6a1.2 1.2 0 0 0-1.2 1.2v5.8a1.2 1.2 0 0 0 1.2 1.2h.2" fill="none" stroke="currentColor" strokeWidth="1.3" /></svg>;
    case 'close':
      return <svg {...common}><path d="M3.6 3.6l8.8 8.8M12.4 3.6l-8.8 8.8" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" /></svg>;
    case 'search':
      return <svg {...common}><circle cx="7" cy="7" r="4.2" fill="none" stroke="currentColor" strokeWidth="1.3" /><path d="M10.2 10.2 14 14" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /></svg>;
    case 'refresh':
      return <svg {...common}><path d="M13.2 8a5.2 5.2 0 1 1-1.6-3.8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" /><path d="M13.4 2.6v2.8h-2.8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>;
    case 'chevron-down':
      return <svg {...common}><path d="M3.6 6.2 8 10.4l4.4-4.2" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>;
    case 'chevron-left':
      return <svg {...common}><path d="M9.8 3.6 5.6 8l4.2 4.4" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>;
    case 'chevron-right':
      return <svg {...common}><path d="M6.2 3.6 10.4 8l-4.2 4.4" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" /></svg>;
  }
}

function absoluteInviteURL(raw: string): string {
  if (!raw) return '';
  if (/^https?:\/\//i.test(raw)) return raw;
  const origin = typeof window !== 'undefined' && window.location ? window.location.origin : '';
  return raw.startsWith('/') ? origin + raw : origin + '/' + raw;
}

function formatDate(value: string | undefined, locale: string): string {
  if (!value) return '—';
  try {
    return new Intl.DateTimeFormat(locale, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
  } catch {
    return value;
  }
}

// Tailwind recipes migrated 1:1 from TenantMembersPanel.css (file deleted).
/* Vue t-table 默认 table-layout: fixed —— 列宽严格按 width 生效，成员列吃掉余量 */
// Vue t-table 基准字号 14px/22px（headless 实测 tbody fs14 lh22）
const TBL = 'w-full table-fixed border-collapse text-sm leading-[22px]';
// Vue th：白底、13px/22px、色 rgba(0,0,0,0.4)、左右 padding 16px
const TH = 'box-border bg-white py-2 px-4 text-left text-[13px] leading-[22px] font-semibold whitespace-nowrap text-[rgba(0,0,0,0.4)]';
const TD = 'box-border border-t border-[var(--wk-border,#dce3ed)] px-4 align-middle text-[rgba(0,0,0,0.9)]';
const TR_HOVER = 'hover:bg-[rgb(46_109_230/4%)]';
// Audit table header (TenantMembers.vue:1745-1780 .audit-table-shell): the
// drawer's scroll area is the scroll container, so thead pins to its top.
const AUDIT_TH = TH + ' sticky top-0 z-[2] [box-shadow:inset_0_-1px_0_var(--wk-border,#dce3ed)]';
// TenantMembers.vue:983-1001 auditColumns widths (target/path wrap instead of clip).
const AUDIT_COLUMNS: Array<{ key: string; label: string; width?: number; minWidth?: number; align?: 'center' }> = [
  { key: 'created_at', label: 'tenantMember.audit.columns.time', width: 120 },
  { key: 'actor', label: 'tenantMember.audit.columns.actor', width: 180 },
  { key: 'action', label: 'tenantMember.audit.columns.action', width: 130 },
  { key: 'target', label: 'tenantMember.audit.columns.target', minWidth: 200 },
  { key: 'request_path', label: 'tenantMember.audit.columns.path', minWidth: 160 },
  { key: 'outcome', label: 'tenantMember.audit.columns.outcome', width: 80, align: 'center' },
];
// Vue t-pagination default: borderless numbers, radius 3px, 24px box; current =
// brand #07c05f fill + white text; hover = brand text on transparent.
const PAGER_BTN = 'inline-flex h-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent px-1 py-0 font-normal text-[rgb(0_0_0/90%)] [font:inherit]';
// wk-tag base + tone variants — Vue t-tag default variant="dark": solid theme
// color fill + white text (owner=primary #07c05f, admin=warning, contributor/
// accepted=success #00a870), small size = height 20px, radius 3px, padding 0 7px.
// Vue t-tag size-s：padding 0 4px + 1px 透明边框，lh 20px（实测）；box-border
// 使 h-5=20px 含边框（全局无 border-box reset）；相对 baseline 悬挂，
// +0.3px 使文字 Range 与 Vue t-tag 文字 y 对齐（邀请行 -0.19 / 成员行 +0.41）。
const WK_TAG_BASE = 'relative top-[0.3px] inline-flex box-border h-5 items-center rounded-[3px] border border-solid border-transparent text-xs font-normal leading-[20px] px-[4px] whitespace-nowrap align-middle ';
function wkTag(tone: 'primary' | 'success' | 'warning' | 'danger' | 'default'): string {
  switch (tone) {
    case 'primary': return WK_TAG_BASE + 'bg-[#07c05f] text-white';
    case 'success': return WK_TAG_BASE + 'bg-[#00a870] text-white';
    case 'warning': return WK_TAG_BASE + 'bg-[#ed7b2f] text-white';
    case 'danger': return WK_TAG_BASE + 'bg-[#e34d59] text-white';
    default: return WK_TAG_BASE + 'bg-[var(--wk-canvas,#f3f3f3)] text-[var(--wk-text,rgba(0,0,0,0.9))]';
  }
}

// T12a：t-tag theme 映射（Vue t-tag 默认 variant=dark，映射保持不变）。
function roleTagTone(role: TenantRole): 'primary' | 'warning' | 'success' | 'default' {
  return role === 'owner' ? 'primary' : role === 'admin' ? 'warning' : role === 'contributor' ? 'success' : 'default';
}

function invitationStatusTone(status: TenantInvitation['status']): 'primary' | 'success' | 'danger' | 'warning' | 'default' {
  return status === 'pending' ? 'primary' : status === 'accepted' ? 'success' : status === 'expired' ? 'danger' : status === 'declined' || status === 'revoked' ? 'warning' : 'default';
}

// Static role-permission matrix, kept aligned with TenantMembers.vue roleMatrix.
type RolePerm = { key: string; has: boolean };
const roleMatrixOrder: TenantRole[] = ['owner', 'admin', 'contributor', 'viewer'];
const roleMatrix: Record<TenantRole, RolePerm[]> = {
  owner: [
    { key: 'manageMembers', has: true },
    { key: 'manageTenantConfig', has: true },
    { key: 'manageInfra', has: true },
    { key: 'createOwnKB', has: true },
    { key: 'readAll', has: true },
  ],
  admin: [
    { key: 'manageMembers', has: false },
    { key: 'manageTenantConfig', has: false },
    { key: 'manageInfra', has: true },
    { key: 'createOwnKB', has: true },
    { key: 'readAll', has: true },
  ],
  contributor: [
    { key: 'manageMembers', has: false },
    { key: 'manageTenantConfig', has: false },
    { key: 'manageInfra', has: false },
    { key: 'createOwnKB', has: true },
    { key: 'readAll', has: true },
  ],
  viewer: [
    { key: 'manageMembers', has: false },
    { key: 'manageTenantConfig', has: false },
    { key: 'manageInfra', has: false },
    { key: 'createOwnKB', has: false },
    { key: 'readAll', has: true },
  ],
};

function memberPrimary(row: TenantMember): string {
  return row.username?.trim() || row.email?.trim() || '—';
}

function memberSecondary(row: TenantMember): string {
  const name = row.username?.trim();
  const mail = row.email?.trim();
  return name && mail ? mail : '';
}

function inviteePrimary(row: TenantInvitation): string {
  return row.invitee_name?.trim() || row.invitee_email?.trim() || row.invitee_user_id;
}

function inviterPrimary(row: TenantInvitation): string {
  return row.inviter_name?.trim() || row.inviter_email?.trim() || row.invited_by || '—';
}

function pageWindow(current: number, max: number): Array<number | 'ellipsis'> {
  if (max <= 7) return Array.from({ length: max }, (_unused, index) => index + 1);
  const pages = new Set<number>([1, max, current - 1, current, current + 1]);
  const inner = [...pages].filter((page) => page >= 1 && page <= max).sort((a, b) => a - b);
  const out: Array<number | 'ellipsis'> = [];
  let previous = 0;
  for (const page of inner) {
    if (previous && page - previous > 1) out.push('ellipsis');
    out.push(page);
    previous = page;
  }
  return out;
}

function TablePager({ total, page, pageSize, onPage, onPageSize, tr }: {
  total: number; page: number; pageSize: number;
  onPage: (page: number) => void; onPageSize: (size: number) => void; tr: Translate;
}) {
  const maxPage = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
  const [jump, setJump] = useState(String(page));
  useEffect(() => { setJump(String(page)); }, [page, pageSize]);

  function commitJump() {
    const next = Math.min(maxPage, Math.max(1, Number.parseInt(jump, 10) || page));
    if (next !== page) onPage(next);
    else setJump(String(page));
  }

  // Vue t-pagination 实测：内容行高 24px、上下 padding 10px（border-t +
  // 灰底区 294~337），邀请表 pager 透出 shell 灰底、成员表白底。
  // T12a：tdesign Pagination 直译 Vue t-pagination（size=small +
  // show-jumper show-page-number show-page-size，页脚壳样式走 settings.td.css §7）。
  return <div className="data-table-shell__pager">
    <Pagination
      total={total}
      current={page}
      pageSize={pageSize}
      size="small"
      showJumper
      showPageNumber
      showPageSize
      pageSizeOptions={PAGE_SIZE_OPTIONS.map((size) => ({ label: tr('tenantMembersPanel.pager.sizePerPage', { size }), value: size }))}
      onChange={(pageInfo: { current: number; previous: number; pageSize: number }) => {
        if (pageInfo.pageSize !== pageSize) onPageSize(pageInfo.pageSize);
        if (pageInfo.current !== page) onPage(pageInfo.current);
      }}
    />
  </div>;
}

export function TenantMembersPanel({ client, tenantId, role, initialMembers }: Props) {
  const locale = useAppLocale();
  const tr = useMemo(() => createPanelTranslator(locale), [locale]);
  // SP14 router folds a system admin to 'system-admin'; Vue derives the
  // members role from the tenant membership (owner), so a system admin must
  // keep the manage surface (invites / 待接受的邀请 table) here too.
  const canManage = role === 'owner' || role === 'admin' || role === 'system-admin';
  const canViewAudit = canManage;
  const currentRole = role === 'system-admin' ? '' : (role as TenantRole);

  const [members, setMembers] = useState(initialMembers?.items ?? []);
  const [total, setTotal] = useState(initialMembers?.total ?? 0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(initialMembers === undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [currentUserId, setCurrentUserId] = useState('');

  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [invitationsTotal, setInvitationsTotal] = useState(0);
  const [invitationsPage, setInvitationsPage] = useState(1);
  const [invitationsPageSize, setInvitationsPageSize] = useState(20);
  const [invitationsLoading, setInvitationsLoading] = useState(false);
  const [invitationsError, setInvitationsError] = useState<string | null>(null);

  const [permissionsOpen, setPermissionsOpen] = useState(false);
  const [auditOpen, setAuditOpen] = useState(false);
  const [audit, setAudit] = useState<AuditLog[]>([]);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditError, setAuditError] = useState<string | null>(null);
  const [auditLoadedOnce, setAuditLoadedOnce] = useState(false);
  // Cursor pagination (TenantMembers.vue:618-627): backend pages by
  // descending id via after_id; next_cursor === 0 stops loading.
  const auditCursorRef = useRef(0);
  const [auditHasMore, setAuditHasMore] = useState(true);
  const auditHasMoreRef = useRef(true);
  const auditLoadingRef = useRef(false);
  // Expanded-row state stays ephemeral so reopening the drawer starts
  // collapsed (TenantMembers.vue:1113-1116).
  const [auditExpandedKeys, setAuditExpandedKeys] = useState<number[]>([]);
  // Drawer scroll root + bottom sentinel for IntersectionObserver-driven
  // infinite scroll (TenantMembers.vue:629-632, 1177-1197).
  const auditScrollRef = useRef<HTMLDivElement | null>(null);
  const auditSentinelRef = useRef<HTMLDivElement | null>(null);
  const auditObserverRef = useRef<IntersectionObserver | null>(null);
  // Display names seen across paginated member payloads, so audit rows can
  // resolve user ids that are not on the current member page
  // (TenantMembers.vue:589-590, 799-803, 1064-1074).
  const memberDisplayRef = useRef<Record<string, { username?: string; email?: string }>>({});
  const permissionsRef = useRef<HTMLDivElement | null>(null);

  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<TenantRole>('contributor');
  const [shareLinkOpen, setShareLinkOpen] = useState(false);
  const [shareLinkRole, setShareLinkRole] = useState<TenantRole>('contributor');
  const [shareLink, setShareLink] = useState<TenantInvitation | null>(null);

  const [revokeConfirmKey, setRevokeConfirmKey] = useState<number | null>(null);
  const [removeConfirmKey, setRemoveConfirmKey] = useState<string | null>(null);

  useEffect(() => { void client.auth.me().then((result) => setCurrentUserId(String(result.user?.id ?? ''))).catch(() => setCurrentUserId('')); }, [client]);

  async function load(nextPage = page, nextQuery = query, nextPageSize = pageSize) {
    setLoading(true); setError(null);
    try {
      const result = await client.identity.tenants.members.list(tenantId, { q: nextQuery.trim() || undefined, page: nextPage, pageSize: nextPageSize });
      const maxPage = Math.max(1, Math.ceil(result.total / Math.max(1, nextPageSize)));
      if (nextPage > maxPage) { setLoading(false); await load(maxPage, nextQuery, nextPageSize); return; }
      setMembers(result.items); setTotal(result.total);
      setPage(nextPage); setPageSize(nextPageSize);
      rememberMembersForAudit(result.items);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : tr('tenantMember.errors.generic'));
    } finally { setLoading(false); }
  }

  // Search debounce (TenantMembers.vue:576-579, 851-859): typing schedules a
  // single server query 320ms later with the trimmed input, back on page 1.
  // appliedQueryRef dedupes against already-applied terms so the explicit
  // submit/clear paths don't double-fire when they reset `query`.
  const appliedQueryRef = useRef('');
  const searchTimerRef = useRef<number | undefined>(undefined);
  useEffect(() => {
    const next = query.trim();
    if (next === appliedQueryRef.current) return;
    window.clearTimeout(searchTimerRef.current);
    searchTimerRef.current = window.setTimeout(() => {
      appliedQueryRef.current = next;
      void load(1, next, pageSize);
    }, SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(searchTimerRef.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query]);

  // Tenant switch resets the search state like TenantMembers.vue:1447-1465
  // (query, debounce timer, applied term, page and page size).
  useEffect(() => {
    window.clearTimeout(searchTimerRef.current);
    appliedQueryRef.current = '';
    setQuery('');
    if (initialMembers === undefined) void load(1, '', 20); // eslint-disable-line react-hooks/exhaustive-deps
  }, [client, tenantId]);

  async function loadInvitations(nextPage = invitationsPage, nextPageSize = invitationsPageSize) {
    if (!canManage) return;
    setInvitationsLoading(true); setInvitationsError(null);
    try {
      const result = await client.identity.tenants.invitations.listTenant(tenantId, { page: nextPage, pageSize: nextPageSize });
      setInvitations(result.items); setInvitationsTotal(result.total);
      setInvitationsPage(nextPage); setInvitationsPageSize(nextPageSize);
    } catch (cause) {
      setInvitationsError(cause instanceof Error ? cause.message : tr('tenantInvitation.errors.generic'));
    } finally { setInvitationsLoading(false); }
  }

  useEffect(() => { if (canManage) void loadInvitations(1, 20); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [client, tenantId, canManage]);

  const loadAuditRef = useRef<(reset: boolean) => Promise<void>>(async () => {});

  // TenantMembers.vue:1134-1175. reset=true (first open / refresh button)
  // drops the list and starts from the top; false appends the next page.
  async function loadAudit(reset: boolean) {
    if (!canViewAudit || auditLoadingRef.current) return;
    if (!reset && !auditHasMoreRef.current) return;
    auditLoadingRef.current = true;
    setAuditLoading(true); setAuditError(null);
    try {
      const result = await client.identity.tenants.auditLog.list(tenantId, { afterId: reset ? undefined : (auditCursorRef.current || undefined), limit: AUDIT_PAGE_SIZE });
      const rows = result.items;
      setAudit((current) => reset ? rows : [...current, ...rows]);
      auditCursorRef.current = result.nextCursor || 0;
      // next_cursor === 0 means "empty page OR smallest id reached" — stop.
      const hasMore = !!result.nextCursor && rows.length > 0;
      auditHasMoreRef.current = hasMore;
      setAuditHasMore(hasMore);
      setAuditLoadedOnce(true);
    } catch (cause) {
      const status = (cause as { status?: number } | null)?.status;
      setAuditError(status === 403
        ? tr('tenantMember.audit.forbidden')
        : cause instanceof Error ? cause.message : tr('tenantMember.errors.generic'));
    } finally {
      auditLoadingRef.current = false;
      setAuditLoading(false);
    }
  }
  loadAuditRef.current = loadAudit;

  // TenantMembers.vue:1177-1197: sentinel observer bound to the drawer's
  // scroll area, firing the next cursor page ~100px before the bottom.
  function detachAuditInfiniteScroll() {
    auditObserverRef.current?.disconnect();
    auditObserverRef.current = null;
  }

  function attachAuditInfiniteScroll() {
    detachAuditInfiniteScroll();
    if (typeof IntersectionObserver === 'undefined') return;
    const root = auditScrollRef.current;
    const sentinel = auditSentinelRef.current;
    if (!root || !sentinel) return;
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return;
      if (!auditHasMoreRef.current || auditLoadingRef.current) return;
      void loadAuditRef.current(false);
    }, { root, rootMargin: '100px 0px', threshold: 0 });
    observer.observe(sentinel);
    auditObserverRef.current = observer;
  }

  // Drawer lifecycle mirrors TenantMembers.vue:1199-1243: lazy-load on first
  // open only (refresh stays explicit), attach the observer after the drawer
  // content mounts, detach on close / error, and clean up on unmount.
  function openAuditDrawer() {
    setAuditOpen(true);
    if (!auditLoadedOnce) void loadAudit(true);
  }

  useEffect(() => {
    if (!auditOpen || auditError) {
      detachAuditInfiniteScroll();
      return;
    }
    // jsdom (node --test) ships no rAF; fall back to a task like FAQPage/McpToolsDirectory.
    const schedule = typeof window.requestAnimationFrame === 'function'
      ? window.requestAnimationFrame.bind(window)
      : (callback: FrameRequestCallback) => window.setTimeout(() => callback(0), 0);
    const frame = schedule(() => attachAuditInfiniteScroll());
    return () => { typeof window.cancelAnimationFrame === 'function' ? window.cancelAnimationFrame(frame) : window.clearTimeout(frame); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auditOpen, auditError]);

  useEffect(() => () => detachAuditInfiniteScroll(), []);

  // Close the permissions popover on outside click, like the Vue hover popup.
  useEffect(() => {
    if (!permissionsOpen) return;
    function onPointerDown(event: MouseEvent) {
      if (permissionsRef.current && event.target instanceof Node && !permissionsRef.current.contains(event.target)) setPermissionsOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) { if (event.key === 'Escape') setPermissionsOpen(false); }
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('mousedown', onPointerDown); document.removeEventListener('keydown', onKeyDown); };
  }, [permissionsOpen]);

  function openInvite() { setInviteEmail(''); setInviteRole('contributor'); setInviteOpen(true); }
  function openShareLink() { setShareLinkRole('contributor'); setShareLink(null); setShareLinkOpen(true); }

  async function submitInvite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canManage || !inviteEmail.trim() || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.tenants.invitations.create(tenantId, { email: inviteEmail.trim(), role: inviteRole });
      setInviteOpen(false); setInviteEmail('');
      setNotice(tr('tenantInvitation.inviteSuccess'));
      await loadInvitations(1, invitationsPageSize);
    } catch (cause) { setError(cause instanceof Error ? cause.message : tr('tenantInvitation.errors.generic')); }
    finally { setBusy(false); }
  }

  async function submitShareLink() {
    if (!canManage || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const invitation = await client.identity.tenants.invitations.createInviteLink(tenantId, { role: shareLinkRole });
      setShareLink(invitation);
      await loadInvitations(1, invitationsPageSize);
    } catch (cause) { setError(cause instanceof Error ? cause.message : tr('tenantInvitation.errors.generic')); }
    finally { setBusy(false); }
  }

  async function copyText(raw: string) {
    try {
      await navigator.clipboard?.writeText(absoluteInviteURL(raw));
      setNotice(tr('tenantInvitation.copied'));
    } catch { setNotice(tr('tenantInvitation.copyFailed')); }
  }

  async function revoke(invitation: TenantInvitation) {
    if (!canManage || busy) return;
    setRevokeConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.tenants.invitations.revoke(tenantId, invitation.id);
      setNotice(tr('tenantInvitation.revoke.success'));
      await loadInvitations();
    } catch (cause) { setError(cause instanceof Error ? cause.message : tr('tenantInvitation.errors.generic')); }
    finally { setBusy(false); }
  }

  async function update(member: TenantMember, nextRole: TenantRole) {
    if (!canManage || busy || member.role === nextRole) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.tenants.members.updateRole(tenantId, member.user_id, nextRole);
      setNotice(tr('tenantMember.roleChange.success'));
      await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : tr('tenantMember.errors.generic')); }
    finally { setBusy(false); }
  }

  async function remove(member: TenantMember) {
    if (!canManage || busy) return;
    setRemoveConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.identity.tenants.members.remove(tenantId, member.user_id);
      setNotice(tr('tenantMember.remove.success'));
      await load();
    } catch (cause) { setError(cause instanceof Error ? cause.message : tr('tenantMember.errors.generic')); }
    finally { setBusy(false); }
  }

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    window.clearTimeout(searchTimerRef.current);
    const next = query.trim();
    appliedQueryRef.current = next;
    void load(1, next, pageSize);
  }
  function clearSearch() {
    window.clearTimeout(searchTimerRef.current);
    appliedQueryRef.current = '';
    setQuery(''); setPage(1); void load(1, '', pageSize);
  }

  // TenantMembers.vue:799-803 — remember member display fields from every
  // paginated payload so the audit table can resolve ids off the current page.
  function rememberMembersForAudit(rows: TenantMember[]) {
    for (const row of rows) memberDisplayRef.current[row.user_id] = { username: row.username, email: row.email };
  }

  function actorDisplayName(userId: string): string {
    const current = members.find((member) => member.user_id === userId);
    if (current?.username?.trim()) return current.username.trim();
    if (current?.email?.trim()) return current.email.trim();
    const memo = memberDisplayRef.current[userId];
    if (memo?.username?.trim()) return memo.username.trim();
    if (memo?.email?.trim()) return memo.email.trim();
    return userId;
  }

  // ---- Audit row helpers (TenantMembers.vue:1081-1132) ----

  function auditTargetSubject(entry: AuditLog): string {
    if (entry.target_user_id) return actorDisplayName(entry.target_user_id);
    if (entry.target_id) return entry.target_type ? `${entry.target_type}:${entry.target_id}` : entry.target_id;
    return '';
  }

  function auditTargetDiff(entry: AuditLog): string {
    const details = entry.details && typeof entry.details === 'object' ? entry.details as Record<string, unknown> : null;
    if (!details) return '';
    if (entry.action === 'rbac.member_role_changed' && details.old_role && details.new_role) return `${String(details.old_role)} → ${String(details.new_role)}`;
    if (entry.action === 'rbac.access_denied' && typeof details.required_role === 'string') return tr('tenantMember.audit.requiredRole', { role: details.required_role });
    if ((entry.action === 'rbac.invitation_sent' || entry.action === 'rbac.invitation_revoked') && typeof details.role === 'string') return details.role;
    return '';
  }

  function auditDetailsJSON(entry: AuditLog): string {
    if (entry.details === null || entry.details === undefined) return '{}';
    if (typeof entry.details === 'string') return entry.details;
    try { return JSON.stringify(entry.details, null, 2); } catch { return String(entry.details); }
  }

  function auditActionLabel(action: string): string {
    const label = tr('tenantMember.audit.action.' + action);
    return label.startsWith('tenantMember.audit.action.') ? action : label;
  }

  function auditOutcomeLabel(outcome: string): string {
    const label = tr('tenantMember.audit.outcome.' + outcome);
    return label.startsWith('tenantMember.audit.outcome.') ? outcome : label;
  }

  function auditActionTone(action: string): 'success' | 'warning' | 'danger' | 'default' {
    switch (action) {
      case 'rbac.access_denied': return 'danger';
      case 'rbac.member_added': return 'success';
      case 'rbac.member_removed':
      case 'rbac.member_left':
      case 'rbac.member_role_changed': return 'warning';
      default: return 'default';
    }
  }

  function toggleAuditExpand(id: number) {
    setAuditExpandedKeys((keys) => keys.includes(id) ? keys.filter((key) => key !== id) : [...keys, id]);
  }

  const maxMembersPage = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));

  return <section className="tenant-members" data-testid="tenant-members-settings">
    <div className="section-header">
      <div className="section-header-row">
        <div className="section-header-titlewrap">
          <h2>{tr('tenantMember.title')}</h2>
          <TPopup
            placement="bottom-left"
            trigger="hover"
            overlayClassName="permissions-popup-overlay"
            content={(
              <div className="permissions-compact permissions-compact--popover" role="dialog" aria-label={tr('tenantMember.permissions.title')}>
                <div className="permissions-compact-header">
                  <span className="permissions-compact-title">{tr('tenantMember.permissions.title')}</span>
                  <span className="permissions-compact-desc">{tr('tenantMember.permissions.desc')}</span>
                </div>
                <div className="permissions-compact-grid">
                  {roleMatrixOrder.map((matrixRole) => (
                    <div key={matrixRole} className={'perm-role-block ' + matrixRole + (currentRole === matrixRole ? ' is-me' : '')}>
                      <div className="perm-role-tag">
                        <span>{tr('tenantMember.role.' + matrixRole)}</span>
                        {currentRole === matrixRole ? <span className="me-badge">{tr('common.me')}</span> : null}
                      </div>
                      <div className="perm-items">
                        {roleMatrix[matrixRole].map((perm) => <span key={perm.key} className={'perm-item ' + (perm.has ? 'has' : 'no')}>{tr('tenantMember.permissions.' + perm.key)}</span>)}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          >
            <button type="button" className="permissions-trigger-btn" aria-label={tr('tenantMember.permissions.title')} title={tr('tenantMember.permissions.iconHint')}>
              <TIcon name="info-circle" size="16px" />
            </button>
          </TPopup>
          {canViewAudit ? (
            <TButton variant="text" size="small" className="header-audit-btn" icon={<TIcon name="history" />} onClick={openAuditDrawer}>
              {tr('tenantMember.audit.tabLabel')}
            </TButton>
          ) : null}
        </div>
      </div>
      <p className="section-description">
        {tr('tenantMember.sectionDescription')}{' '}
        <a className="doc-link" href={RBAC_DOC_URL} target="_blank" rel="noopener noreferrer">
          {tr('tenantMember.learnRbacGuide')} <TIcon name="link" className="link-icon" />
        </a>
      </p>
    </div>
    <div className="members-tab-layout">
      {canManage ? <div className="pending-invitations-section">
        <div className="pending-invitations-header">
          <div className="pending-invitations-titlewrap">
            <span className="pending-invitations-title">{tr('tenantInvitation.pendingSectionTitle')}</span>
            <span className="members-list-count-badge">{invitationsTotal}</span>
          </div>
          <span className="pending-invitations-desc">{tr('tenantInvitation.pendingSectionDesc', { days: INVITATION_TTL_DAYS })}</span>
        </div>
        {invitationsLoading ? <div className="flex items-center gap-2"><Status>{tr('tenantMember.loading')}</Status></div>
          : invitationsError ? <div className="flex items-center gap-2"><Status tone="error">{invitationsError}</Status><Button type="button" onClick={() => void loadInvitations()}>{tr('tenantMember.retry')}</Button></div>
          : invitationsTotal === 0 ? <div className="rounded-[8px] border border-dashed border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-3 py-2.5 text-[0.8125rem] text-[rgba(0,0,0,0.6)]">{tr('tenantInvitation.pendingEmpty')}</div>
          : <div className="data-table-shell data-table-shell--with-footer pending-invitations-table overflow-hidden rounded-card">
              <div className="overflow-x-auto">
                <table className={TBL}>
                  <thead><tr>
                    <th className={TH}>{tr('tenantInvitation.columns.invitee')}</th>
                    <th className={TH} style={{ width: 110 }}>{tr('tenantInvitation.columns.role')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.inviter')}</th>
                    <th className={TH} style={{ width: 160 }}>{tr('tenantInvitation.columns.expiresAt')}</th>
                    <th className={TH} style={{ width: 100 }}>{tr('tenantInvitation.columns.status')}</th>
                    <th className={TH} style={{ width: 120 }}>{tr('tenantInvitation.columns.operations')}</th>
                  </tr></thead>
                  <tbody>
                    {invitations.map((invitation) => {
                      const isShareLink = invitation.is_share_link === true;
                      const statusLabel = isShareLink && invitation.status === 'pending'
                        ? tr('tenantInvitation.status.shareLinkActive')
                        : tr('tenantInvitation.status.' + invitation.status);
                      return <tr key={invitation.id} className={TR_HOVER}>
                        <td className={TD}>
                          <div className="member-cell flex flex-col gap-[2px] min-w-0 py-[2px]">
                            {isShareLink ? <>
                              <span className="inline-flex items-center gap-1 overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium leading-[22px] text-[rgba(0,0,0,0.9)]"><TIcon name="link" size="14px" /> {tr('tenantInvitation.shareLink.cellTitle')}</span>
                              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[rgba(0,0,0,0.6)]">{(invitation.accepted_count ?? 0) > 0 ? tr('tenantInvitation.shareLink.cellAccepted', { count: invitation.accepted_count ?? 0 }) : tr('tenantInvitation.shareLink.cellEmpty')}</span>
                            </> : <>
                              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium leading-[22px] text-[rgba(0,0,0,0.9)]">{inviteePrimary(invitation)}</span>
                              {invitation.invitee_email && invitation.invitee_name ? <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[rgba(0,0,0,0.6)]">{invitation.invitee_email}</span> : null}
                            </>}
                          </div>
                        </td>
                        <td className={TD}><Tag theme={roleTagTone(invitation.role)} size="small">{tr('tenantMember.role.' + invitation.role)}</Tag></td>
                        <td className={TD}><span>{inviterPrimary(invitation)}</span></td>
                        <td className={TD}>{formatDate(invitation.expires_at, locale)}</td>
                        <td className={TD}><Tag theme={invitationStatusTone(invitation.status)} size="small">{statusLabel}</Tag></td>
                        <td className={TD}>
                          <div className="inline-flex items-center gap-0">
                            {invitation.status === 'pending' && invitation.invite_url ? <TButton shape="square" variant="text" size="small" aria-label={tr('tenantInvitation.copyLink')} title={tr('tenantInvitation.copyLink')} onClick={() => void copyText(invitation.invite_url ?? '')} icon={<TIcon name="copy" />} /> : null}
                            {invitation.status === 'pending' ? <span className="relative inline-flex">
                              <TButton theme="danger" shape="square" variant="text" size="small" aria-label={tr('tenantInvitation.revoke.button')} title={tr('tenantInvitation.revoke.button')} onClick={() => setRevokeConfirmKey(revokeConfirmKey === invitation.id ? null : invitation.id)} icon={<TIcon name="close" />} />
                              {revokeConfirmKey === invitation.id ? <div className="absolute left-0 top-[calc(100%_+_0.3rem)] z-[25] box-border w-max max-w-[16rem] rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-[0.65rem] py-[0.55rem] text-xs text-[rgba(0,0,0,0.9)] shadow-[0_10px_28px_rgb(23_32_51/18%)]" role="alertdialog" aria-label={tr('tenantInvitation.revoke.button')}>
                                <p className="m-0 mb-[0.45rem]">{isShareLink ? tr('tenantInvitation.shareLink.revokeConfirm') : tr('tenantInvitation.revoke.confirmBody', { email: invitation.invitee_email || invitation.invitee_user_id })}</p>
                                <div className="flex justify-end gap-[0.4rem]">
                                  <Button type="button" onClick={() => setRevokeConfirmKey(null)}>{tr('common.cancel')}</Button>
                                  <Button type="button" className="text-[#b3352f]! hover:border-[rgb(217_83_79/45%)]!" disabled={busy} onClick={() => void revoke(invitation)}>{tr('tenantInvitation.revoke.confirm')}</Button>
                                </div>
                              </div> : null}
                            </span> : null}
                          </div>
                        </td>
                      </tr>;
                    })}
                  </tbody>
                </table>
              </div>
              <TablePager total={invitationsTotal} page={invitationsPage} pageSize={invitationsPageSize}
                onPage={(next) => void loadInvitations(next)} onPageSize={(size) => void loadInvitations(1, size)} tr={tr} />
            </div>}
      </div> : null}

      {/* mt 4.3px：邀请 shell 底（gap-5=20px 上邻）到列表头 y363.3 的 Vue 实测链距 */}
      <div className="members-list-wrap">
        <div className="members-list-header">
          <div className="inline-flex min-w-0 items-center gap-2">
            <span className="members-list-title text-sm font-semibold leading-[normal] text-[rgba(0,0,0,0.9)]">{tr('tenantMember.listTitle')}</span>
            <span className="members-list-count-badge inline-flex h-5 min-w-[1.375rem] items-center justify-center rounded-full bg-[#f3f3f3] px-[7px] text-xs font-semibold leading-none text-[rgba(0,0,0,0.9)]">{total}</span>
          </div>
          <div className="m-0 inline-flex min-w-0 flex-[0_1_auto] items-center gap-2 max-[720px]:flex-wrap max-[720px]:w-full max-[720px]:justify-start">
            {/* flex：input 换成 flex item，避免 inline-block baseline descent 把
                form 撑到 25.5px（Vue 列表头行高 24px） */}
            {/* T12a：t-input size=small + prefix-icon/clearable、t-button
                outline square small（TenantMembers.vue :195-256 直译）。 */}
            <form className="members-list-search" role="search" onSubmit={search}>
              <TInput size="small" clearable aria-label={tr('tenantMember.searchPlaceholder')} placeholder={tr('tenantMember.searchPlaceholder')} value={query} onChange={(value) => setQuery(String(value ?? ''))} prefixIcon={<TIcon name="search" />} />
            </form>
            {canManage ? <>
              <TButton theme="primary" variant="outline" shape="square" size="small" className="members-list-add-btn"
                title={tr('tenantMember.add.button')} aria-label={tr('tenantMember.add.button')} onClick={openInvite} icon={<TIcon name="user-add" />} />
              <TButton theme="default" variant="outline" shape="square" size="small" className="members-list-add-btn"
                title={tr('tenantInvitation.shareLink.button')} aria-label={tr('tenantInvitation.shareLink.button')} onClick={openShareLink} icon={<TIcon name="link" />} />
            </> : null}
          </div>
        </div>
        {/* R472 A2 — Vue TenantMembers.vue:313-318 t-alert theme=error + retry：
            浅红横幅透传后端原文 + 重试按钮（load() 重发同请求）。 */}
        {error ? <div data-testid="tenant-members-error" role="alert" className="flex items-center gap-2"><Status tone="error">{error}</Status><Button type="button" onClick={() => void load()}>{tr('tenantMember.retry')}</Button></div> : null}
        {notice ? <Status tone="success">{notice}</Status> : null}
        {loading && members.length === 0 ? <Status>{tr('tenantMember.loading')}</Status>
          : total === 0 ? <div className="py-2"><Status>{query.trim() ? tr('tenantMember.emptySearch', { q: query }) : tr('tenantMember.empty')}</Status></div>
          : <div className="data-table-shell data-table-shell--with-footer overflow-hidden rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)]">
              <div className="overflow-x-auto">
                <table className={TBL}>
                  <thead><tr>
                    <th className={TH}>{tr('tenantMember.columns.member')}</th>
                    <th className={TH} style={{ width: 128 }}>{tr('tenantMember.columns.role')}</th>
                    <th className={TH} style={{ width: 154 }}>{tr('tenantMember.columns.joinedAt')}</th>
                    <th className={TH} style={{ width: 88 }}>{tr('tenantMember.columns.operations')}</th>
                  </tr></thead>
                  <tbody>
                    {members.map((member) => {
                      const isSelf = member.user_id === currentUserId;
                      return <tr key={member.user_id} className={TR_HOVER + (isSelf ? ' bg-[#f3f3f3]' : '')}>
                        <td className={TD}>
                          <div className="member-cell flex flex-col gap-[2px] min-w-0 py-[2px]">
                            <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium leading-[22px] text-[rgba(0,0,0,0.9)]">{memberPrimary(member)}</span>
                            {memberSecondary(member) ? <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[rgba(0,0,0,0.6)]">{memberSecondary(member)}</span> : null}
                          </div>
                        </td>
                        <td className={TD}>
                          <div className="role-cell inline-flex items-center">
                            {canManage && !isSelf ? <Select className="disabled:opacity-60 w-full [font:inherit]" aria-label={'Role for ' + member.username} value={member.role} disabled={busy}
                              onChange={(event) => void update(member, event.target.value as TenantRole)}>
                              {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
                            </Select>
                            : <Tag theme={roleTagTone(member.role)} size="small">{tr('tenantMember.role.' + member.role)}</Tag>}
                          </div>
                        </td>
                        <td className={TD}>{formatDate(member.joined_at, locale)}</td>
                        <td className={TD}>
                          {canManage && !isSelf ? <span className="relative inline-flex">
                            <Button type="button" className="h-6! w-6! min-h-6! min-w-6! bg-transparent! border-transparent! shadow-none! p-0! text-[#e34d59]! hover:bg-[rgba(227,77,89,0.08)]!" aria-label={tr('tenantMember.remove.button')} title={tr('tenantMember.remove.button')}
                              onClick={() => setRemoveConfirmKey(removeConfirmKey === member.user_id ? null : member.user_id)}>
                              <Icon name="user-clear" />
                            </Button>
                            {removeConfirmKey === member.user_id ? <div className="absolute left-0 top-[calc(100%_+_0.3rem)] z-[25] box-border w-max max-w-[16rem] rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-[0.65rem] py-[0.55rem] text-xs text-[rgba(0,0,0,0.9)] shadow-[0_10px_28px_rgb(23_32_51/18%)]" role="alertdialog" aria-label={tr('tenantMember.remove.button')}>
                              <p className="m-0 mb-[0.45rem]">{tr('tenantMember.remove.confirmBody', { name: member.username || member.email })}</p>
                              <div className="flex justify-end gap-[0.4rem]">
                                <Button type="button" onClick={() => setRemoveConfirmKey(null)}>{tr('common.cancel')}</Button>
                                <Button type="button" className="text-[#b3352f]! hover:border-[rgb(217_83_79/45%)]!" disabled={busy} onClick={() => void remove(member)}>{tr('tenantMember.remove.confirm')}</Button>
                              </div>
                            </div> : null}
                          </span> : null}
                        </td>
                      </tr>;
                    })}
                  </tbody>
                </table>
              </div>
              <TablePager total={total} page={page} pageSize={pageSize}
                onPage={(next) => void load(next)} onPageSize={(size) => void load(1, query, size)} tr={tr} />
            </div>}
        {loading && members.length > 0 ? <Status>{tr('tenantMember.loading')}</Status> : null}
      </div>
    </div>

    {/* Audit drawer — port of TenantMembers.vue:380-511 (SettingDrawer with
        width="1120px" min 720 max 1600, storage-key tenant-members-audit). */}
    <TenantAuditDrawer
      open={auditOpen && canViewAudit}
      title={tr('tenantMember.audit.tabLabel')}
      onClose={() => setAuditOpen(false)}
      width={1120}
      minWidth={720}
      maxWidth={1600}
      storageKey="setting-drawer:width:tenant-members-audit"
    >
      <div className="flex min-h-0 w-full flex-1 flex-col gap-3.5">
        <div className="flex items-center justify-between gap-3 rounded-card bg-[var(--wk-canvas,#f7f9fc)] px-4 py-3">
          <span className="min-w-0 flex-1 text-[13px] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.description')}</span>
          <Button type="button" variant="text" size="small" className="shrink-0" loading={auditLoading} disabled={auditLoading} onClick={() => void loadAudit(true)}>
            <Icon name="refresh" /> {tr('tenantMember.audit.refresh')}
          </Button>
        </div>
        {auditError ? <div className="flex flex-1 flex-col items-start"><div className="flex items-center gap-2"><Status tone="error">{auditError}</Status><Button type="button" onClick={() => void loadAudit(true)}>{tr('tenantMember.retry')}</Button></div></div>
          : !auditLoading && audit.length === 0 ? <div className="flex flex-1 flex-col items-center justify-center px-3 py-6"><EmptyState description={tr('tenantMember.audit.empty')} /></div>
          : <div ref={auditScrollRef} className="audit-scroll-area min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
              <div className="overflow-hidden rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)]">
                <table className={TBL}>
                  <thead><tr>
                    <th className={AUDIT_TH + ' w-9'} aria-label={tr('tenantMember.audit.expanded.details')} />
                    {AUDIT_COLUMNS.map((column) => <th key={column.key} className={AUDIT_TH + (column.align === 'center' ? ' text-center' : '')} style={column.width ? { width: column.width } : { minWidth: column.minWidth }}>{tr(column.label)}</th>)}
                  </tr></thead>
                  <tbody>
                    {audit.map((entry) => {
                      const expanded = auditExpandedKeys.includes(entry.id);
                      const subject = auditTargetSubject(entry);
                      const diff = auditTargetDiff(entry);
                      const parts = auditDateParts(entry.created_at, locale);
                      return <Fragment key={entry.id}>
                        <tr className={TR_HOVER + ' cursor-pointer'} aria-expanded={expanded} onClick={() => toggleAuditExpand(entry.id)}>
                          <td className={TD + ' pr-0'}>
                            <span className={'inline-flex text-[rgba(0,0,0,0.6)] transition-transform ' + (expanded ? 'rotate-180' : '')}><Icon name="chevron-down" /></span>
                          </td>
                          <td className={TD}>
                            <div className="flex flex-col gap-[2px] leading-[1.3]">
                              <span className="text-xs text-[rgba(0,0,0,0.6)]">{parts.date}</span>
                              <span className="text-[13px] font-medium text-[rgba(0,0,0,0.9)] [font-variant-numeric:tabular-nums]">{parts.time}</span>
                            </div>
                          </td>
                          <td className={TD}>
                            <div className="flex min-w-0 flex-col gap-[2px] leading-[1.3]">
                              <span className="overflow-hidden text-[13px] font-medium text-ellipsis whitespace-nowrap text-[rgba(0,0,0,0.9)]">
                                {entry.actor_user_id ? actorDisplayName(entry.actor_user_id) : tr('tenantMember.audit.systemActor')}
                              </span>
                              {entry.actor_role ? <span className="text-xs text-[rgba(0,0,0,0.6)]">{tr('tenantMember.role.' + entry.actor_role)}</span> : null}
                            </div>
                          </td>
                          <td className={TD}><Tag theme={auditActionTone(entry.action)} size="small">{auditActionLabel(entry.action)}</Tag></td>
                          <td className={TD}>
                            <div className="flex min-w-0 flex-col gap-1 py-[2px] leading-[1.35]">
                              {subject ? <span className="break-all text-[13px] text-[rgba(0,0,0,0.9)]">{subject}</span> : null}
                              {diff ? <span className="break-all font-mono text-xs leading-[1.4] text-[rgba(0,0,0,0.6)]">{diff}</span> : null}
                              {!subject && !diff ? <span className="text-[rgba(0,0,0,0.4)]">—</span> : null}
                            </div>
                          </td>
                          <td className={TD}>
                            {entry.request_path ? <span className="break-all font-mono text-xs text-[rgba(0,0,0,0.6)]">
                              {entry.request_method ? <span className="mr-1 inline-block font-semibold text-[rgba(0,0,0,0.9)]">{entry.request_method}</span> : null}
                              {entry.request_path}
                            </span> : <span className="text-[rgba(0,0,0,0.4)]">—</span>}
                          </td>
                          <td className={TD + ' text-center'}><Tag theme={entry.outcome === 'denied' ? 'danger' : entry.outcome === 'success' ? 'success' : 'default'} size="small">{auditOutcomeLabel(entry.outcome)}</Tag></td>
                        </tr>
                        {expanded ? <tr className="audit-expanded-row">
                          <td colSpan={AUDIT_COLUMNS.length + 1} className="border-t border-[var(--wk-border,#dce3ed)] p-0! align-top!">
                            <div className="flex flex-col gap-3 bg-[var(--wk-canvas,#f7f9fc)] px-4 py-3">
                              <div className="grid gap-x-[18px] gap-y-2.5 [grid-template-columns:repeat(auto-fill,minmax(220px,1fr))]">
                                <div className="flex min-w-0 flex-col gap-[2px]">
                                  <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.expanded.actorId')}</span>
                                  <span className="break-all font-mono text-xs text-[rgba(0,0,0,0.9)]">{entry.actor_user_id || '—'}</span>
                                </div>
                                {entry.target_user_id ? <div className="flex min-w-0 flex-col gap-[2px]">
                                  <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.expanded.targetUserId')}</span>
                                  <span className="break-all font-mono text-xs text-[rgba(0,0,0,0.9)]">{entry.target_user_id}</span>
                                </div> : null}
                                {entry.target_type ? <div className="flex min-w-0 flex-col gap-[2px]">
                                  <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.expanded.targetType')}</span>
                                  <span className="break-all font-mono text-xs text-[rgba(0,0,0,0.9)]">{entry.target_type}</span>
                                </div> : null}
                                {entry.target_id ? <div className="flex min-w-0 flex-col gap-[2px]">
                                  <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.expanded.targetId')}</span>
                                  <span className="break-all font-mono text-xs text-[rgba(0,0,0,0.9)]">{entry.target_id}</span>
                                </div> : null}
                              </div>
                              <div className="flex flex-col gap-1">
                                <span className="text-[11px] font-semibold uppercase tracking-[0.04em] text-[rgba(0,0,0,0.6)]">{tr('tenantMember.audit.expanded.details')}</span>
                                <pre className="m-0 max-h-[280px] overflow-auto whitespace-pre-wrap break-all rounded-[6px] border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-3 py-2.5 font-mono text-xs leading-[1.55] text-[rgba(0,0,0,0.9)]">{auditDetailsJSON(entry)}</pre>
                              </div>
                            </div>
                          </td>
                        </tr> : null}
                      </Fragment>;
                    })}
                  </tbody>
                </table>
              </div>
              {/* 触底 sentinel：IntersectionObserver root 指向 .audit-scroll-area（TenantMembers.vue:497-498） */}
              <div ref={auditSentinelRef} className="audit-load-sentinel pointer-events-none h-px w-full" aria-hidden="true" />
              {auditLoading && audit.length > 0 ? <div className="audit-loading-more flex items-center justify-center gap-2.5 p-3 text-xs text-[rgba(0,0,0,0.6)]"><Status>{tr('tenantMember.loading')}</Status></div> : null}
              {!auditHasMore && audit.length > 0 && !auditLoading ? <p className="audit-end-hint m-0 py-2 pb-3.5 text-center text-xs text-[rgba(0,0,0,0.4)]">{tr('tenantMember.audit.end')}</p> : null}
            </div>}
      </div>
    </TenantAuditDrawer>

    <Dialog open={inviteOpen} title={tr('tenantMember.add.dialogTitle')} onClose={() => setInviteOpen(false)} closeLabel={tr('common.close')}>
      <form className="flex flex-col gap-3" onSubmit={submitInvite}>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[rgba(0,0,0,0.9)]">{tr('tenantMember.add.emailLabel')}</span>
          <Input required type="email" className="w-full rounded-md! px-[0.55rem]! py-[0.45rem]! text-[rgba(0,0,0,0.9)]!" value={inviteEmail} placeholder={tr('tenantMember.add.emailPlaceholder')}
            onChange={(event) => setInviteEmail(event.target.value)} />
        </label>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[rgba(0,0,0,0.9)]">{tr('tenantMember.add.roleLabel')}</span>
          <Select className="w-full [font:inherit]" value={inviteRole} onChange={(event) => setInviteRole(event.target.value as TenantRole)}>
            {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
          </Select>
        </label>
        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" disabled={busy} onClick={() => setInviteOpen(false)}>{tr('common.cancel')}</Button>
          <Button type="submit" loading={busy}>{tr('tenantInvitation.inviteSubmit')}</Button>
        </div>
      </form>
    </Dialog>

    <Dialog open={shareLinkOpen} title={shareLink ? tr('tenantInvitation.shareLink.resultTitle') : tr('tenantInvitation.shareLink.dialogTitle')} onClose={() => setShareLinkOpen(false)} closeLabel={tr('common.close')}>
      {shareLink ? <div className="flex flex-col gap-3">
        <p className="m-0 text-[0.8125rem] leading-[1.5] text-[rgba(0,0,0,0.6)]">{tr('tenantInvitation.shareLink.resultBody')}</p>
        <div className="flex items-center gap-2">
          <Input className="min-w-0 flex-[1_1_auto] rounded-md! px-[0.55rem]! py-[0.45rem]! text-[0.8125rem]! read-only:text-muted" readOnly aria-label={tr('tenantInvitation.shareLink.resultTitle')} value={absoluteInviteURL(shareLink.invite_url ?? '')}
            onFocus={(event) => event.currentTarget.select()} />
          <Button type="button" onClick={() => void copyText(shareLink.invite_url ?? '')}>
            <Icon name="copy" /> {tr('tenantInvitation.copyLink')}
          </Button>
        </div>
        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" onClick={() => setShareLinkOpen(false)}>{tr('common.close')}</Button>
        </div>
      </div> : <div className="flex flex-col gap-3">
        <p className="m-0 text-[0.8125rem] leading-[1.5] text-[rgba(0,0,0,0.6)]">{tr('tenantInvitation.shareLink.description', { days: INVITATION_TTL_DAYS })}</p>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[rgba(0,0,0,0.9)]">{tr('tenantMember.add.roleLabel')}</span>
          <Select className="w-full [font:inherit]" value={shareLinkRole} onChange={(event) => setShareLinkRole(event.target.value as TenantRole)}>
            {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
          </Select>
        </label>
        <div className="mt-1 flex justify-end gap-2">
          <Button type="button" disabled={busy} onClick={() => setShareLinkOpen(false)}>{tr('common.cancel')}</Button>
          <Button type="button" loading={busy} onClick={() => void submitShareLink()}>{tr('tenantInvitation.shareLink.generate')}</Button>
        </div>
      </div>}
    </Dialog>
  </section>;
}
