import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import type { AuditLog, TenantInvitation, TenantMember, TenantRole, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Dialog, Input, Select, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';

type Role = 'viewer' | 'admin' | 'owner' | 'system-admin';
type Props = { client: WeKnoraClient; tenantId: number; role: Role; initialMembers?: { items: TenantMember[]; total: number } };

const roles: TenantRole[] = ['owner', 'admin', 'contributor', 'viewer'];
const INVITATION_TTL_DAYS = 7;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
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
type IconName = 'info' | 'history' | 'link' | 'user-add' | 'user-clear' | 'copy' | 'close' | 'search' | 'refresh';
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
const TBL = 'w-full border-collapse text-[0.8125rem]';
const TH = 'bg-[var(--wk-canvas,#f7f9fc)] py-2 px-3 text-left text-xs font-semibold whitespace-nowrap text-[var(--wk-muted,#66758b)]';
const TD = 'border-t border-[var(--wk-border,#dce3ed)] py-[0.55rem] px-3 align-middle text-[var(--wk-text,#172033)]';
const TR_HOVER = 'hover:bg-[rgb(46_109_230/4%)]';
// Vue t-pagination default: borderless numbers, radius 3px, 24px box; current =
// brand #07c05f fill + white text; hover = brand text on transparent.
const PAGER_BTN = 'inline-flex h-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent px-[0.3rem] py-0 font-normal text-[rgb(0_0_0/90%)] [font:inherit]';
// wk-tag base + tone variants (owner=primary green, admin=warning,
// contributor=success, viewer/expired mapping per invitationStatusTheme).
const WK_TAG_BASE = 'inline-flex items-center rounded-[4px] border text-xs font-semibold leading-[1.4] py-[0.05rem] px-[0.45rem] whitespace-nowrap ';
function wkTag(tone: 'primary' | 'success' | 'warning' | 'danger' | 'default'): string {
  switch (tone) {
    case 'primary': return WK_TAG_BASE + 'border-[rgb(46_140_90/35%)] bg-[rgb(46_140_90/12%)] text-[#1f7a4d]';
    case 'success': return WK_TAG_BASE + 'border-[rgb(46_140_90/30%)] bg-[rgb(46_140_90/10%)] text-[#1f7a4d]';
    case 'warning': return WK_TAG_BASE + 'border-[rgb(214_158_46/40%)] bg-[rgb(214_158_46/14%)] text-[#9a6b12]';
    case 'danger': return WK_TAG_BASE + 'border-[rgb(217_83_79/35%)] bg-[rgb(217_83_79/12%)] text-[#b3352f]';
    default: return WK_TAG_BASE + 'border-transparent bg-[var(--wk-canvas,#f7f9fc)] text-[var(--wk-muted,#66758b)]';
  }
}

function roleTagClass(role: TenantRole): string {
  return wkTag(role === 'owner' ? 'primary' : role === 'admin' ? 'warning' : role === 'contributor' ? 'success' : 'default');
}

function invitationStatusClass(status: TenantInvitation['status']): string {
  const tone = status === 'pending' ? 'primary' : status === 'accepted' ? 'success' : status === 'expired' ? 'danger' : status === 'declined' || status === 'revoked' ? 'warning' : 'default';
  return wkTag(tone) + ' status-tag';
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

  return <div className="data-table-shell__pager flex flex-wrap items-center justify-end gap-[0.4rem] border-t border-[var(--wk-border,#dce3ed)] box-border py-[0.4rem] px-[0.6rem] text-xs text-[var(--wk-muted,#66758b)] max-[720px]:justify-start">
    <span className="mr-auto">{tr('tenantMembersPanel.pager.total', { total })}</span>
    <Select className="w-auto!" value={pageSize} onChange={(event) => onPageSize(Number(event.target.value))}>
      {PAGE_SIZE_OPTIONS.map((size) => <option key={size} value={size}>{tr('tenantMembersPanel.pager.sizePerPage', { size })}</option>)}
    </Select>
    <button type="button" className={PAGER_BTN + ' disabled:text-[rgb(0_0_0/26%)] disabled:cursor-not-allowed disabled:opacity-50 enabled:hover:text-accent'} aria-label={tr('common.previous')} disabled={page <= 1} onClick={() => onPage(page - 1)}>‹</button>
    {pageWindow(page, maxPage).map((entry, index) => entry === 'ellipsis'
      ? <span key={'e' + index} className="wk-pager__ellipsis">…</span>
      : <button key={entry} type="button" className={PAGER_BTN + " aria-[current=page]:bg-accent aria-[current=page]:text-white [&:not([aria-current='page']):hover]:text-accent"} aria-current={entry === page ? 'page' : undefined} onClick={() => onPage(entry)}>{entry}</button>)}
    <button type="button" className={PAGER_BTN + ' disabled:text-[rgb(0_0_0/26%)] disabled:cursor-not-allowed disabled:opacity-50 enabled:hover:text-accent'} aria-label={tr('common.next')} disabled={page >= maxPage} onClick={() => onPage(page + 1)}>›</button>
    <span className="inline-flex items-center gap-[0.3rem]">
      {tr('tenantMembersPanel.pager.jumper')}
      <Input className="h-6 w-[2.6rem]! rounded-md! px-0! text-center!" type="text" inputMode="numeric" value={jump}
        onChange={(event) => setJump(event.target.value)}
        onBlur={commitJump}
        onKeyDown={(event) => { if (event.key === 'Enter') commitJump(); }} />
      /{maxPage} {tr('tenantMembersPanel.pager.pageUnit')}
    </span>
  </div>;
}

export function TenantMembersPanel({ client, tenantId, role, initialMembers }: Props) {
  const locale = useAppLocale();
  const tr = useMemo(() => createPanelTranslator(locale), [locale]);
  const canManage = role === 'owner' || role === 'admin';
  const canViewAudit = canManage || role === 'system-admin';
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
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : tr('tenantMember.errors.generic'));
    } finally { setLoading(false); }
  }

  useEffect(() => { if (initialMembers === undefined) void load(1, '', 20); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [client, tenantId]);

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

  async function loadAudit() {
    if (!canViewAudit || auditLoading) return;
    setAuditLoading(true); setAuditError(null);
    try { setAudit((await client.identity.tenants.auditLog.list(tenantId, { limit: 50 })).items); setAuditLoadedOnce(true); }
    catch (cause) { setAuditError(cause instanceof Error ? cause.message : tr('tenantMember.errors.generic')); }
    finally { setAuditLoading(false); }
  }

  function openAudit() {
    setAuditOpen((open) => {
      if (!open && !auditLoadedOnce) void loadAudit();
      return !open;
    });
  }

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

  function search(event: FormEvent<HTMLFormElement>) { event.preventDefault(); void load(1, query, pageSize); }
  function clearSearch() { setQuery(''); setPage(1); void load(1, '', pageSize); }

  function actorDisplayName(userId: string): string {
    const current = members.find((member) => member.user_id === userId);
    if (current?.username?.trim()) return current.username.trim();
    if (current?.email?.trim()) return current.email.trim();
    return userId || tr('tenantMember.audit.systemActor');
  }

  function auditActionLabel(action: string): string {
    const label = tr('tenantMember.audit.action.' + action);
    return label.startsWith('tenantMember.audit.action.') ? action : label;
  }

  function auditOutcomeLabel(outcome: string): string {
    const label = tr('tenantMember.audit.outcome.' + outcome);
    return label.startsWith('tenantMember.audit.outcome.') ? outcome : label;
  }

  const maxMembersPage = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));

  return <section className="flex w-full flex-col gap-4" data-testid="tenant-members-settings">
    <div className="mb-8 flex flex-col gap-1.5">
      <div className="section-header-row flex items-center justify-between">
        <div className="flex min-w-0 items-center gap-2">
          <h2 className="m-0! text-[20px] font-semibold leading-[25px] tracking-[-0.02em] text-[var(--wk-text,#172033)]">{tr('tenantMember.title')}</h2>
          <div className="relative inline-flex" ref={permissionsRef}>
            <button type="button" className="inline-flex h-6 w-6 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-[var(--wk-muted,#66758b)] hover:text-primary hover:[outline:none] focus-visible:text-primary focus-visible:outline-offset-2 focus-visible:[outline:var(--wk-focus-ring,3px_solid_rgb(46_109_230/35%))]" aria-label={tr('tenantMember.permissions.title')}
              title={tr('tenantMember.permissions.iconHint')} aria-expanded={permissionsOpen}
              onClick={() => setPermissionsOpen((open) => !open)}>
              <Icon name="info" size={16} />
            </button>
            {permissionsOpen ? <div className="absolute left-0 top-[calc(100%_+_0.375rem)] z-30 box-border w-[min(520px,calc(100vw_-_24px))] max-w-[min(520px,calc(100vw_-_24px))] max-h-[min(400px,65vh)] overflow-auto rounded-[10px] border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] p-[0.875rem] text-[var(--wk-text,#172033)] shadow-[0_12px_32px_rgb(23_32_51/16%)]" role="dialog" aria-label={tr('tenantMember.permissions.title')}>
              <div className="mb-2.5 flex flex-col gap-0.5">
                <span className="text-[0.8125rem] [font-weight:650]">{tr('tenantMember.permissions.title')}</span>
                <span className="text-xs text-[var(--wk-muted,#66758b)]">{tr('tenantMember.permissions.desc')}</span>
              </div>
              <div className="grid gap-2 grid-cols-[repeat(auto-fit,minmax(210px,1fr))]">
                {roleMatrixOrder.map((matrixRole) => <div key={matrixRole} className={'flex flex-col gap-1.5 rounded-card border p-2 px-2.5 ' + (matrixRole === 'owner' ? 'border-[rgb(46_109_230/45%)]' : 'border-[var(--wk-border,#dce3ed)]')}>
                  <div className="inline-flex items-center gap-1.5 text-[0.8125rem] [font-weight:650]">
                    <span>{tr('tenantMember.role.' + matrixRole)}</span>
                    {currentRole === matrixRole ? <span className="rounded-full bg-primary px-[0.4rem] py-[0.05rem] text-[0.6875rem] font-semibold text-white">{tr('common.me')}</span> : null}
                  </div>
                  <div className="flex flex-col gap-[0.2rem]">
                    {roleMatrix[matrixRole].map((perm) => <span key={perm.key} className={'text-xs leading-[1.35] ' + (perm.has ? 'text-[var(--wk-text,#172033)]' : 'text-[#a4b0c3]')}>
                      {perm.has ? '✓' : '✗'} {tr('tenantMember.permissions.' + perm.key)}
                    </span>)}
                  </div>
                </div>)}
              </div>
            </div> : null}
          </div>
        </div>
        {canViewAudit ? <Button type="button" variant="text" className="h-6 rounded-[3px]! px-[7px] py-0 text-[12px] text-[var(--wk-muted,#66758b)]! hover:text-primary!" onClick={openAudit}>
          <Icon name="history" size={16} /> {tr('tenantMember.audit.tabLabel')}
        </Button> : null}
      </div>
      <p className="m-0 text-[14px] leading-[1.5] text-[var(--wk-muted,#66758b)] opacity-75">
        {tr('tenantMember.sectionDescription')}
        <a className="ml-[0.375rem] inline-flex items-center gap-[0.2rem] text-primary no-underline hover:underline" href={RBAC_DOC_URL} target="_blank" rel="noopener noreferrer">
          {tr('tenantMember.learnRbacGuide')} <Icon name="link" size={12} />
        </a>
      </p>
    </div>

    <div className="flex flex-col gap-5">
      {canManage ? <div className="pending-invitations-section">
        <div className="mb-2 flex flex-wrap items-start justify-between gap-x-3 gap-y-[0.375rem]">
          <div className="inline-flex items-center gap-2">
            <span className="text-sm [font-weight:650] text-[var(--wk-text,#172033)]">{tr('tenantInvitation.pendingSectionTitle')}</span>
            <span className="members-list-count-badge inline-flex h-5 min-w-[1.375rem] items-center justify-center rounded-full border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-canvas,#f7f9fc)] px-[0.4rem] text-xs [font-weight:650] leading-none text-[var(--wk-text,#172033)]">{invitationsTotal}</span>
          </div>
          <span className="text-xs text-[var(--wk-muted,#66758b)]">{tr('tenantInvitation.pendingSectionDesc', { days: INVITATION_TTL_DAYS })}</span>
        </div>
        {invitationsLoading ? <div className="flex items-center gap-2"><Status>{tr('tenantMember.loading')}</Status></div>
          : invitationsError ? <div className="flex items-center gap-2"><Status tone="error">{invitationsError}</Status><Button type="button" onClick={() => void loadInvitations()}>{tr('tenantMember.retry')}</Button></div>
          : invitationsTotal === 0 ? <div className="rounded-[8px] border border-dashed border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-3 py-2.5 text-[0.8125rem] text-[var(--wk-muted,#66758b)]">{tr('tenantInvitation.pendingEmpty')}</div>
          : <div className="data-table-shell data-table-shell--with-footer pending-invitations-table overflow-hidden rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)]">
              <div className="overflow-x-auto">
                <table className={TBL}>
                  <thead><tr>
                    <th className={TH}>{tr('tenantInvitation.columns.invitee')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.role')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.inviter')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.expiresAt')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.status')}</th>
                    <th className={TH}>{tr('tenantInvitation.columns.operations')}</th>
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
                              <span className="inline-flex items-center gap-1 overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium text-[var(--wk-text,#172033)]"><Icon name="link" size={14} /> {tr('tenantInvitation.shareLink.cellTitle')}</span>
                              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[var(--wk-muted,#66758b)]">{(invitation.accepted_count ?? 0) > 0 ? tr('tenantInvitation.shareLink.cellAccepted', { count: invitation.accepted_count ?? 0 }) : tr('tenantInvitation.shareLink.cellEmpty')}</span>
                            </> : <>
                              <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium text-[var(--wk-text,#172033)]">{inviteePrimary(invitation)}</span>
                              {invitation.invitee_email && invitation.invitee_name ? <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[var(--wk-muted,#66758b)]">{invitation.invitee_email}</span> : null}
                            </>}
                          </div>
                        </td>
                        <td className={TD}><span className={roleTagClass(invitation.role)}>{tr('tenantMember.role.' + invitation.role)}</span></td>
                        <td className={TD}><span>{inviterPrimary(invitation)}</span></td>
                        <td className={TD}>{formatDate(invitation.expires_at, locale)}</td>
                        <td className={TD}><span className={invitationStatusClass(invitation.status)}>{statusLabel}</span></td>
                        <td className={TD}>
                          <div className="inline-flex items-center gap-1">
                            {invitation.status === 'pending' && invitation.invite_url ? <Button type="button" aria-label={tr('tenantInvitation.copyLink')} title={tr('tenantInvitation.copyLink')} onClick={() => void copyText(invitation.invite_url ?? '')}><Icon name="copy" /></Button> : null}
                            {invitation.status === 'pending' ? <span className="relative inline-flex">
                              <Button type="button" className="text-[#b3352f]! hover:border-[rgb(217_83_79/45%)]!" aria-label={tr('tenantInvitation.revoke.button')} title={tr('tenantInvitation.revoke.button')} onClick={() => setRevokeConfirmKey(revokeConfirmKey === invitation.id ? null : invitation.id)}><Icon name="close" /></Button>
                              {revokeConfirmKey === invitation.id ? <div className="absolute left-0 top-[calc(100%_+_0.3rem)] z-[25] box-border w-max max-w-[16rem] rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-[0.65rem] py-[0.55rem] text-xs text-[var(--wk-text,#172033)] shadow-[0_10px_28px_rgb(23_32_51/18%)]" role="alertdialog" aria-label={tr('tenantInvitation.revoke.button')}>
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

      <div className="members-list-wrap">
        <div className="members-list-header flex flex-wrap items-center justify-between gap-3 px-[0.125rem]">
          <div className="inline-flex min-w-0 items-center gap-2">
            <span className="members-list-title text-sm [font-weight:650] text-[var(--wk-text,#172033)]">{tr('tenantMember.listTitle')}</span>
            <span className="members-list-count-badge inline-flex h-5 min-w-[1.375rem] items-center justify-center rounded-full border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-canvas,#f7f9fc)] px-[0.4rem] text-xs [font-weight:650] leading-none text-[var(--wk-text,#172033)]">{total}</span>
          </div>
          <div className="m-0 inline-flex min-w-0 flex-[0_1_auto] items-center gap-2 max-[720px]:flex-wrap max-[720px]:w-full max-[720px]:justify-start">
            <form className="relative min-w-40 w-56 flex-[0_1_14rem] max-[720px]:min-w-0 max-[720px]:w-full max-[720px]:flex-[1_1_100%]" role="search" onSubmit={search}>
              <span className="pointer-events-none absolute left-[0.45rem] top-1/2 inline-flex -translate-y-1/2 items-center justify-center text-[var(--wk-muted,#66758b)]"><Icon name="search" /></span>
              <Input type="search" className="w-full rounded-md! px-[1.9rem]! py-[0.4rem]! text-[var(--wk-text,#172033)]! focus-visible:[outline:var(--wk-focus-ring,3px_solid_rgb(46_109_230/35%))] focus-visible:outline-offset-2" aria-label={tr('tenantMember.searchPlaceholder')} placeholder={tr('tenantMember.searchPlaceholder')} value={query} onChange={(event) => setQuery(event.target.value)} />
              {query ? <button type="button" className="absolute right-[0.35rem] top-1/2 inline-flex h-5 w-5 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-[var(--wk-muted,#66758b)] hover:text-[var(--wk-text,#172033)]" aria-label={tr('tenantMembersPanel.clearSearch')} onClick={clearSearch}><Icon name="close" size={12} /></button> : null}
            </form>
            {canManage ? <>
              <Button type="button" className="inline-flex h-8 w-8 items-center justify-center bg-[rgb(46_109_230/8%)]! border border-[rgb(46_109_230/45%)]! p-0! text-[#2e6de6]! hover:bg-[rgb(46_109_230/16%)]!"
                title={tr('tenantMember.add.button')} aria-label={tr('tenantMember.add.button')} onClick={openInvite}>
                <Icon name="user-add" size={16} />
              </Button>
              <Button type="button" className="inline-flex h-8 w-8 items-center justify-center bg-[var(--wk-surface,#fff)]! border border-[var(--wk-border,#dce3ed)]! p-0! text-[var(--wk-muted,#66758b)]! hover:text-[#2e6de6]!"
                title={tr('tenantInvitation.shareLink.button')} aria-label={tr('tenantInvitation.shareLink.button')} onClick={openShareLink}>
                <Icon name="link" size={16} />
              </Button>
            </> : null}
          </div>
        </div>
        {error ? <div className="flex items-center gap-2"><Status tone="error">{error}</Status><Button type="button" onClick={() => void load()}>{tr('tenantMember.retry')}</Button></div> : null}
        {notice ? <Status tone="success">{notice}</Status> : null}
        {loading && members.length === 0 ? <Status>{tr('tenantMember.loading')}</Status>
          : total === 0 ? <div className="py-2"><Status>{query.trim() ? tr('tenantMember.emptySearch', { q: query }) : tr('tenantMember.empty')}</Status></div>
          : <div className="data-table-shell data-table-shell--with-footer overflow-hidden rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)]">
              <div className="overflow-x-auto">
                <table className={TBL}>
                  <thead><tr>
                    <th className={TH}>{tr('tenantMember.columns.member')}</th>
                    <th className={TH}>{tr('tenantMember.columns.role')}</th>
                    <th className={TH}>{tr('tenantMember.columns.joinedAt')}</th>
                    <th className={TH}>{tr('tenantMember.columns.operations')}</th>
                  </tr></thead>
                  <tbody>
                    {members.map((member) => {
                      const isSelf = member.user_id === currentUserId;
                      return <tr key={member.user_id} className={TR_HOVER}>
                        <td className={TD}>
                          <div className="member-cell flex flex-col gap-[2px] min-w-0 py-[2px]">
                            <span className="overflow-hidden text-ellipsis whitespace-nowrap text-sm font-medium text-[var(--wk-text,#172033)]">{memberPrimary(member)}</span>
                            {memberSecondary(member) ? <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs leading-[1.35] text-[var(--wk-muted,#66758b)]">{memberSecondary(member)}</span> : null}
                          </div>
                        </td>
                        <td className={TD}>
                          <div className="role-cell inline-flex items-center">
                            {canManage && !isSelf ? <Select className="disabled:opacity-60 w-full [font:inherit]" aria-label={'Role for ' + member.username} value={member.role} disabled={busy}
                              onChange={(event) => void update(member, event.target.value as TenantRole)}>
                              {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
                            </Select>
                            : <span className={roleTagClass(member.role)}>{tr('tenantMember.role.' + member.role)}</span>}
                          </div>
                        </td>
                        <td className={TD}>{formatDate(member.joined_at, locale)}</td>
                        <td className={TD}>
                          {canManage && !isSelf ? <span className="relative inline-flex">
                            <Button type="button" className="text-[#b3352f]! hover:border-[rgb(217_83_79/45%)]!" aria-label={tr('tenantMember.remove.button')} title={tr('tenantMember.remove.button')}
                              onClick={() => setRemoveConfirmKey(removeConfirmKey === member.user_id ? null : member.user_id)}>
                              <Icon name="user-clear" />
                            </Button>
                            {removeConfirmKey === member.user_id ? <div className="absolute left-0 top-[calc(100%_+_0.3rem)] z-[25] box-border w-max max-w-[16rem] rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)] px-[0.65rem] py-[0.55rem] text-xs text-[var(--wk-text,#172033)] shadow-[0_10px_28px_rgb(23_32_51/18%)]" role="alertdialog" aria-label={tr('tenantMember.remove.button')}>
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

    {auditOpen && canViewAudit ? <Card role="region" aria-label={tr('tenantMember.audit.tabLabel')} className="flex flex-col gap-[0.625rem]">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-[var(--wk-muted,#66758b)]">{tr('tenantMember.audit.description')}</span>
        <Button type="button" disabled={auditLoading} onClick={() => void loadAudit()}>
          <Icon name="refresh" /> {tr('tenantMember.audit.refresh')}
        </Button>
      </div>
      {auditError ? <div className="flex items-center gap-2"><Status tone="error">{auditError}</Status><Button type="button" onClick={() => void loadAudit()}>{tr('tenantMember.retry')}</Button></div>
        : !auditLoading && audit.length === 0 ? <div className="py-2"><Status>{tr('tenantMember.audit.empty')}</Status></div>
        : <div className="data-table-shell overflow-hidden rounded-card border border-[var(--wk-border,#dce3ed)] bg-[var(--wk-surface,#fff)]">
            <div className="overflow-x-auto">
              <table className={TBL}>
                <thead><tr>
                  <th className={TH}>{tr('tenantMember.audit.columns.time')}</th>
                  <th className={TH}>{tr('tenantMember.audit.columns.actor')}</th>
                  <th className={TH}>{tr('tenantMember.audit.columns.action')}</th>
                  <th className={TH}>{tr('tenantMember.audit.columns.target')}</th>
                  <th className={TH}>{tr('tenantMember.audit.columns.path')}</th>
                  <th className={TH}>{tr('tenantMember.audit.columns.outcome')}</th>
                </tr></thead>
                <tbody>
                  {audit.map((entry) => <tr key={entry.id} className={TR_HOVER}>
                    <td className={TD}>{formatDate(entry.created_at, locale)}</td>
                    <td className={TD}>{entry.actor_user_id ? actorDisplayName(entry.actor_user_id) : tr('tenantMember.audit.systemActor')}</td>
                    <td className={TD}><span className={wkTag('default')}>{auditActionLabel(entry.action)}</span></td>
                    <td className={TD}>{entry.target_user_id ? actorDisplayName(entry.target_user_id) : entry.target_id ? `${entry.target_type}:${entry.target_id}` : '—'}</td>
                    <td className={TD}>{entry.request_path ? `${entry.request_method} ${entry.request_path}` : '—'}</td>
                    <td className={TD}><span className={wkTag(entry.outcome === 'denied' ? 'danger' : entry.outcome === 'success' ? 'success' : 'default')}>{auditOutcomeLabel(entry.outcome)}</span></td>
                  </tr>)}
                </tbody>
              </table>
            </div>
          </div>}
    </Card> : null}

    <Dialog open={inviteOpen} title={tr('tenantMember.add.dialogTitle')} onClose={() => setInviteOpen(false)} closeLabel={tr('common.close')}>
      <form className="flex flex-col gap-3" onSubmit={submitInvite}>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[var(--wk-text,#172033)]">{tr('tenantMember.add.emailLabel')}</span>
          <Input required type="email" className="w-full rounded-md! px-[0.55rem]! py-[0.45rem]! text-[var(--wk-text,#172033)]!" value={inviteEmail} placeholder={tr('tenantMember.add.emailPlaceholder').replace("{'@'}", '@')}
            onChange={(event) => setInviteEmail(event.target.value)} />
        </label>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[var(--wk-text,#172033)]">{tr('tenantMember.add.roleLabel')}</span>
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
        <p className="m-0 text-[0.8125rem] leading-[1.5] text-[var(--wk-muted,#66758b)]">{tr('tenantInvitation.shareLink.resultBody')}</p>
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
        <p className="m-0 text-[0.8125rem] leading-[1.5] text-[var(--wk-muted,#66758b)]">{tr('tenantInvitation.shareLink.description', { days: INVITATION_TTL_DAYS })}</p>
        <label className="flex! flex-col gap-[0.3rem]! text-[#27364d] font-semibold">
          <span className="text-[0.8125rem] font-semibold text-[var(--wk-text,#172033)]">{tr('tenantMember.add.roleLabel')}</span>
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
