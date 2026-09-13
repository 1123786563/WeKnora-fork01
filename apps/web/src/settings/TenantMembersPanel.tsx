import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import type { AuditLog, TenantInvitation, TenantMember, TenantRole, WeKnoraClient } from '@weknora/api-client';
import type { Locale } from '@weknora/i18n';
import { Button, Card, Dialog, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import './TenantMembersPanel.css';

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
  },
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

// Port of TenantMembers.vue roleTagTheme (owner=primary green, admin=warning,
// contributor=success, viewer=default) and invitationStatusTheme.
function roleTagClass(role: TenantRole): string {
  switch (role) {
    case 'owner': return 'wk-tag wk-tag--primary role-tag';
    case 'admin': return 'wk-tag wk-tag--warning role-tag';
    case 'contributor': return 'wk-tag wk-tag--success role-tag';
    default: return 'wk-tag wk-tag--default role-tag';
  }
}

function invitationStatusClass(status: TenantInvitation['status']): string {
  switch (status) {
    case 'pending': return 'wk-tag wk-tag--primary status-tag';
    case 'accepted': return 'wk-tag wk-tag--success status-tag';
    case 'declined':
    case 'revoked': return 'wk-tag wk-tag--warning status-tag';
    case 'expired': return 'wk-tag wk-tag--danger status-tag';
    default: return 'wk-tag wk-tag--default status-tag';
  }
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

  return <div className="data-table-shell__pager wk-pager">
    <span className="wk-pager__total">{tr('tenantMembersPanel.pager.total', { total })}</span>
    <select className="wk-pager__size" value={pageSize} onChange={(event) => onPageSize(Number(event.target.value))}>
      {PAGE_SIZE_OPTIONS.map((size) => <option key={size} value={size}>{tr('tenantMembersPanel.pager.sizePerPage', { size })}</option>)}
    </select>
    <button type="button" className="wk-pager__prev" aria-label={tr('common.previous')} disabled={page <= 1} onClick={() => onPage(page - 1)}>‹</button>
    {pageWindow(page, maxPage).map((entry, index) => entry === 'ellipsis'
      ? <span key={'e' + index} className="wk-pager__ellipsis">…</span>
      : <button key={entry} type="button" className="wk-pager__page" aria-current={entry === page ? 'page' : undefined} onClick={() => onPage(entry)}>{entry}</button>)}
    <button type="button" className="wk-pager__next" aria-label={tr('common.next')} disabled={page >= maxPage} onClick={() => onPage(page + 1)}>›</button>
    <span className="wk-pager__jumper">
      {tr('tenantMembersPanel.pager.jumper')}
      <input className="wk-pager__jumper-input" type="text" inputMode="numeric" value={jump}
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

  return <section className="wk-tenant-members" data-testid="tenant-members-settings">
    <div className="section-header">
      <div className="section-header-row">
        <div className="section-header-titlewrap">
          <h2>{tr('tenantMember.title')}</h2>
          <div className="permissions-anchor" ref={permissionsRef}>
            <button type="button" className="permissions-trigger-btn" aria-label={tr('tenantMember.permissions.title')}
              title={tr('tenantMember.permissions.iconHint')} aria-expanded={permissionsOpen}
              onClick={() => setPermissionsOpen((open) => !open)}>
              <Icon name="info" size={16} />
            </button>
            {permissionsOpen ? <div className="permissions-compact permissions-compact--popover" role="dialog" aria-label={tr('tenantMember.permissions.title')}>
              <div className="permissions-compact-header">
                <span className="permissions-compact-title">{tr('tenantMember.permissions.title')}</span>
                <span className="permissions-compact-desc">{tr('tenantMember.permissions.desc')}</span>
              </div>
              <div className="permissions-compact-grid">
                {roleMatrixOrder.map((matrixRole) => <div key={matrixRole} className={'perm-role-block perm-role-block--' + matrixRole}>
                  <div className="perm-role-tag">
                    <span>{tr('tenantMember.role.' + matrixRole)}</span>
                    {currentRole === matrixRole ? <span className="me-badge">{tr('common.me')}</span> : null}
                  </div>
                  <div className="perm-items">
                    {roleMatrix[matrixRole].map((perm) => <span key={perm.key} className={'perm-item ' + (perm.has ? 'has' : 'no')}>
                      {perm.has ? '✓' : '✗'} {tr('tenantMember.permissions.' + perm.key)}
                    </span>)}
                  </div>
                </div>)}
              </div>
            </div> : null}
          </div>
          {canViewAudit ? <Button type="button" className="header-audit-btn" onClick={openAudit}>
            <Icon name="history" /> {tr('tenantMember.audit.tabLabel')}
          </Button> : null}
        </div>
      </div>
      <p className="section-description">
        {tr('tenantMember.sectionDescription')}
        <a className="doc-link" href={RBAC_DOC_URL} target="_blank" rel="noopener noreferrer">
          {tr('tenantMember.learnRbacGuide')} <Icon name="link" size={12} />
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
        {invitationsLoading ? <div className="loading-inline"><Status>{tr('tenantMember.loading')}</Status></div>
          : invitationsError ? <div className="error-inline"><Status tone="error">{invitationsError}</Status><Button type="button" onClick={() => void loadInvitations()}>{tr('tenantMember.retry')}</Button></div>
          : invitationsTotal === 0 ? <div className="pending-invitations-empty">{tr('tenantInvitation.pendingEmpty')}</div>
          : <div className="data-table-shell data-table-shell--with-footer pending-invitations-table">
              <div className="data-table-shell__scroll">
                <table className="wk-data-table">
                  <thead><tr>
                    <th>{tr('tenantInvitation.columns.invitee')}</th>
                    <th>{tr('tenantInvitation.columns.role')}</th>
                    <th>{tr('tenantInvitation.columns.inviter')}</th>
                    <th>{tr('tenantInvitation.columns.expiresAt')}</th>
                    <th>{tr('tenantInvitation.columns.status')}</th>
                    <th>{tr('tenantInvitation.columns.operations')}</th>
                  </tr></thead>
                  <tbody>
                    {invitations.map((invitation) => {
                      const isShareLink = invitation.is_share_link === true;
                      const statusLabel = isShareLink && invitation.status === 'pending'
                        ? tr('tenantInvitation.status.shareLinkActive')
                        : tr('tenantInvitation.status.' + invitation.status);
                      return <tr key={invitation.id}>
                        <td>
                          <div className="member-cell">
                            {isShareLink ? <>
                              <span className="member-name share-link-title"><Icon name="link" size={14} /> {tr('tenantInvitation.shareLink.cellTitle')}</span>
                              <span className="member-email">{(invitation.accepted_count ?? 0) > 0 ? tr('tenantInvitation.shareLink.cellAccepted', { count: invitation.accepted_count ?? 0 }) : tr('tenantInvitation.shareLink.cellEmpty')}</span>
                            </> : <>
                              <span className="member-name">{inviteePrimary(invitation)}</span>
                              {invitation.invitee_email && invitation.invitee_name ? <span className="member-email">{invitation.invitee_email}</span> : null}
                            </>}
                          </div>
                        </td>
                        <td><span className={roleTagClass(invitation.role)}>{tr('tenantMember.role.' + invitation.role)}</span></td>
                        <td><span>{inviterPrimary(invitation)}</span></td>
                        <td>{formatDate(invitation.expires_at, locale)}</td>
                        <td><span className={invitationStatusClass(invitation.status)}>{statusLabel}</span></td>
                        <td>
                          <div className="table-actions">
                            {invitation.status === 'pending' && invitation.invite_url ? <Button type="button" aria-label={tr('tenantInvitation.copyLink')} title={tr('tenantInvitation.copyLink')} onClick={() => void copyText(invitation.invite_url ?? '')}><Icon name="copy" /></Button> : null}
                            {invitation.status === 'pending' ? <span className="popconfirm-anchor">
                              <Button type="button" className="danger" aria-label={tr('tenantInvitation.revoke.button')} title={tr('tenantInvitation.revoke.button')} onClick={() => setRevokeConfirmKey(revokeConfirmKey === invitation.id ? null : invitation.id)}><Icon name="close" /></Button>
                              {revokeConfirmKey === invitation.id ? <div className="wk-popconfirm" role="alertdialog" aria-label={tr('tenantInvitation.revoke.button')}>
                                <p>{isShareLink ? tr('tenantInvitation.shareLink.revokeConfirm') : tr('tenantInvitation.revoke.confirmBody', { email: invitation.invitee_email || invitation.invitee_user_id })}</p>
                                <div className="wk-popconfirm__actions">
                                  <Button type="button" onClick={() => setRevokeConfirmKey(null)}>{tr('common.cancel')}</Button>
                                  <Button type="button" className="wk-popconfirm__confirm danger" disabled={busy} onClick={() => void revoke(invitation)}>{tr('tenantInvitation.revoke.confirm')}</Button>
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
        <div className="members-list-header">
          <div className="members-list-titlewrap">
            <span className="members-list-title">{tr('tenantMember.listTitle')}</span>
            <span className="members-list-count-badge">{total}</span>
          </div>
          <div className="members-list-actions">
            <form className="members-list-search" role="search" onSubmit={search}>
              <span className="members-list-search-icon"><Icon name="search" /></span>
              <input type="search" aria-label={tr('tenantMember.searchPlaceholder')} placeholder={tr('tenantMember.searchPlaceholder')} value={query} onChange={(event) => setQuery(event.target.value)} />
              {query ? <button type="button" className="members-list-search-clear" aria-label="Clear search" onClick={clearSearch}><Icon name="close" size={12} /></button> : null}
            </form>
            {canManage ? <>
              <Button type="button" className="members-list-add-btn members-list-add-btn--invite"
                title={tr('tenantMember.add.button')} aria-label={tr('tenantMember.add.button')} onClick={openInvite}>
                <Icon name="user-add" size={16} />
              </Button>
              <Button type="button" className="members-list-add-btn members-list-add-btn--link"
                title={tr('tenantInvitation.shareLink.button')} aria-label={tr('tenantInvitation.shareLink.button')} onClick={openShareLink}>
                <Icon name="link" size={16} />
              </Button>
            </> : null}
          </div>
        </div>
        {error ? <div className="error-inline"><Status tone="error">{error}</Status><Button type="button" onClick={() => void load()}>{tr('tenantMember.retry')}</Button></div> : null}
        {notice ? <Status tone="success">{notice}</Status> : null}
        {loading && members.length === 0 ? <Status>{tr('tenantMember.loading')}</Status>
          : total === 0 ? <div className="empty-state"><Status>{query.trim() ? tr('tenantMember.emptySearch', { q: query }) : tr('tenantMember.empty')}</Status></div>
          : <div className="data-table-shell data-table-shell--with-footer">
              <div className="data-table-shell__scroll">
                <table className="wk-data-table">
                  <thead><tr>
                    <th>{tr('tenantMember.columns.member')}</th>
                    <th>{tr('tenantMember.columns.role')}</th>
                    <th>{tr('tenantMember.columns.joinedAt')}</th>
                    <th>{tr('tenantMember.columns.operations')}</th>
                  </tr></thead>
                  <tbody>
                    {members.map((member) => {
                      const isSelf = member.user_id === currentUserId;
                      return <tr key={member.user_id}>
                        <td>
                          <div className="member-cell">
                            <span className="member-name">{memberPrimary(member)}</span>
                            {memberSecondary(member) ? <span className="member-email">{memberSecondary(member)}</span> : null}
                          </div>
                        </td>
                        <td>
                          <div className="role-cell">
                            {canManage && !isSelf ? <select className="member-role-select" aria-label={'Role for ' + member.username} value={member.role} disabled={busy}
                              onChange={(event) => void update(member, event.target.value as TenantRole)}>
                              {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
                            </select>
                            : <span className={roleTagClass(member.role)}>{tr('tenantMember.role.' + member.role)}</span>}
                          </div>
                        </td>
                        <td>{formatDate(member.joined_at, locale)}</td>
                        <td>
                          {canManage && !isSelf ? <span className="popconfirm-anchor">
                            <Button type="button" className="danger" aria-label={tr('tenantMember.remove.button')} title={tr('tenantMember.remove.button')}
                              onClick={() => setRemoveConfirmKey(removeConfirmKey === member.user_id ? null : member.user_id)}>
                              <Icon name="user-clear" />
                            </Button>
                            {removeConfirmKey === member.user_id ? <div className="wk-popconfirm" role="alertdialog" aria-label={tr('tenantMember.remove.button')}>
                              <p>{tr('tenantMember.remove.confirmBody', { name: member.username || member.email })}</p>
                              <div className="wk-popconfirm__actions">
                                <Button type="button" onClick={() => setRemoveConfirmKey(null)}>{tr('common.cancel')}</Button>
                                <Button type="button" className="wk-popconfirm__confirm danger" disabled={busy} onClick={() => void remove(member)}>{tr('tenantMember.remove.confirm')}</Button>
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

    {auditOpen && canViewAudit ? <Card role="region" aria-label={tr('tenantMember.audit.tabLabel')} className="wk-audit-region">
      <div className="audit-header">
        <span className="audit-desc">{tr('tenantMember.audit.description')}</span>
        <Button type="button" className="audit-refresh-btn" disabled={auditLoading} onClick={() => void loadAudit()}>
          <Icon name="refresh" /> {tr('tenantMember.audit.refresh')}
        </Button>
      </div>
      {auditError ? <div className="error-inline"><Status tone="error">{auditError}</Status><Button type="button" onClick={() => void loadAudit()}>{tr('tenantMember.retry')}</Button></div>
        : !auditLoading && audit.length === 0 ? <div className="empty-state"><Status>{tr('tenantMember.audit.empty')}</Status></div>
        : <div className="data-table-shell">
            <div className="data-table-shell__scroll">
              <table className="wk-data-table wk-data-table--audit">
                <thead><tr>
                  <th>{tr('tenantMember.audit.columns.time')}</th>
                  <th>{tr('tenantMember.audit.columns.actor')}</th>
                  <th>{tr('tenantMember.audit.columns.action')}</th>
                  <th>{tr('tenantMember.audit.columns.target')}</th>
                  <th>{tr('tenantMember.audit.columns.path')}</th>
                  <th>{tr('tenantMember.audit.columns.outcome')}</th>
                </tr></thead>
                <tbody>
                  {audit.map((entry) => <tr key={entry.id}>
                    <td>{formatDate(entry.created_at, locale)}</td>
                    <td>{entry.actor_user_id ? actorDisplayName(entry.actor_user_id) : tr('tenantMember.audit.systemActor')}</td>
                    <td><span className="wk-tag wk-tag--default">{auditActionLabel(entry.action)}</span></td>
                    <td>{entry.target_user_id ? actorDisplayName(entry.target_user_id) : entry.target_id ? `${entry.target_type}:${entry.target_id}` : '—'}</td>
                    <td>{entry.request_path ? `${entry.request_method} ${entry.request_path}` : '—'}</td>
                    <td><span className={'wk-tag ' + (entry.outcome === 'denied' ? 'wk-tag--danger' : entry.outcome === 'success' ? 'wk-tag--success' : 'wk-tag--default')}>{auditOutcomeLabel(entry.outcome)}</span></td>
                  </tr>)}
                </tbody>
              </table>
            </div>
          </div>}
    </Card> : null}

    <Dialog open={inviteOpen} title={tr('tenantMember.add.dialogTitle')} onClose={() => setInviteOpen(false)} closeLabel={tr('common.close')}>
      <form className="member-invite-form" onSubmit={submitInvite}>
        <label className="wk-field">
          <span className="wk-field__label">{tr('tenantMember.add.emailLabel')}</span>
          <input required type="email" value={inviteEmail} placeholder={tr('tenantMember.add.emailPlaceholder').replace("{'@'}", '@')}
            onChange={(event) => setInviteEmail(event.target.value)} />
        </label>
        <label className="wk-field">
          <span className="wk-field__label">{tr('tenantMember.add.roleLabel')}</span>
          <select value={inviteRole} onChange={(event) => setInviteRole(event.target.value as TenantRole)}>
            {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
          </select>
        </label>
        <div className="invite-popup-footer">
          <Button type="button" disabled={busy} onClick={() => setInviteOpen(false)}>{tr('common.cancel')}</Button>
          <Button type="submit" loading={busy}>{tr('tenantInvitation.inviteSubmit')}</Button>
        </div>
      </form>
    </Dialog>

    <Dialog open={shareLinkOpen} title={shareLink ? tr('tenantInvitation.shareLink.resultTitle') : tr('tenantInvitation.shareLink.dialogTitle')} onClose={() => setShareLinkOpen(false)} closeLabel={tr('common.close')}>
      {shareLink ? <div className="share-link-result">
        <p className="invite-confirm-body">{tr('tenantInvitation.shareLink.resultBody')}</p>
        <div className="share-link-row">
          <input className="share-link-row__input" readOnly aria-label={tr('tenantInvitation.shareLink.resultTitle')} value={absoluteInviteURL(shareLink.invite_url ?? '')}
            onFocus={(event) => event.currentTarget.select()} />
          <Button type="button" onClick={() => void copyText(shareLink.invite_url ?? '')}>
            <Icon name="copy" /> {tr('tenantInvitation.copyLink')}
          </Button>
        </div>
        <div className="invite-popup-footer">
          <Button type="button" onClick={() => setShareLinkOpen(false)}>{tr('common.close')}</Button>
        </div>
      </div> : <div className="member-invite-form">
        <p className="invite-confirm-body">{tr('tenantInvitation.shareLink.description', { days: INVITATION_TTL_DAYS })}</p>
        <label className="wk-field">
          <span className="wk-field__label">{tr('tenantMember.add.roleLabel')}</span>
          <select value={shareLinkRole} onChange={(event) => setShareLinkRole(event.target.value as TenantRole)}>
            {roles.map((item) => <option key={item} value={item}>{tr('tenantMember.role.' + item)}</option>)}
          </select>
        </label>
        <div className="invite-popup-footer">
          <Button type="button" disabled={busy} onClick={() => setShareLinkOpen(false)}>{tr('common.cancel')}</Button>
          <Button type="button" loading={busy} onClick={() => void submitShareLink()}>{tr('tenantInvitation.shareLink.generate')}</Button>
        </div>
      </div>}
    </Dialog>
  </section>;
}
