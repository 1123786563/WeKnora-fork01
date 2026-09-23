import { useEffect, useRef, useState } from 'react';
import type { ApiKey, AuditLog, RuntimeQueues, SystemAdminUser, SystemSetting, TenantInvitation, TenantMember, WeKnoraClient } from '@weknora/api-client';
import { Button as TButton, Input as TInput, Select as TSelect } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { formatMessage } from '@weknora/i18n';
import { useAppLocale } from '../i18n.ts';
import { PlatformApiKeysPanel } from '../settings/PlatformApiKeysPanel.tsx';
import { RuntimeQueuesPanel } from '../settings/RuntimeQueuesPanel.tsx';
import { SystemAuditLogPanel } from '../settings/SystemAuditLogPanel.tsx';
import { SystemGlobalSettingsPanel } from '../settings/SystemGlobalSettingsPanel.tsx';
import { canManageTenant, canViewAudit, invitationIsOpen, isEditableMember, systemAdminPanelKeys, tenantRoleFromMemberships } from './summary.ts';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
type SystemAuditPayload = { items: Record<string, unknown>[]; nextCursor: number };
// Structural mirror of the identity TenantMemberPage (the api-client barrel
// does not export the named type; identity/tenant.ts remains the source).
type MembersPage = { items: TenantMember[]; total: number; page: number; pageSize: number };

export function AdministrationPage({ client, tenantId, systemAdmin = false }: { client: WeKnoraClient; tenantId: number; systemAdmin?: boolean }) {
  const locale = useAppLocale();
  const t = (key: string, values?: Record<string, string | number>) => formatMessage(locale, key, values);
  const roleText = (role: string) => role === 'owner' ? t('tenantMember.role.owner') : t(`mobileAdministration.role.${role}`);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [audit, setAudit] = useState<AuditLog[]>([]);
  const [admins, setAdmins] = useState<SystemAdminUser[]>([]);
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [queues, setQueues] = useState<RuntimeQueues | null>(null);
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [systemAudit, setSystemAudit] = useState<SystemAuditPayload | null>(null);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<'admin' | 'contributor' | 'viewer'>('contributor');
  // Vue contract (TenantMembers.vue): the invite popup is two-step — the form
  // branch only previews, the confirm branch fires the API.
  const [inviteStep, setInviteStep] = useState<'form' | 'confirm'>('form');
  const [memberSearch, setMemberSearch] = useState('');
  const [appliedMemberSearch, setAppliedMemberSearch] = useState('');
  // Vue members pagination (TenantMembers.vue membersPage/membersPageSize):
  // default page size 20, options 10/20/50/100, server-side paging.
  const [membersPage, setMembersPage] = useState(1);
  const [membersPageSize, setMembersPageSize] = useState(20);
  const [membersTotal, setMembersTotal] = useState(0);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentUserId, setCurrentUserId] = useState('');
  const [currentRole, setCurrentRole] = useState<string | undefined>();
  const identity = client.identity;
  const manageTenant = systemAdmin || canManageTenant(currentRole);

  async function load() {
    setLoading(true); setError(null);
    const results = await Promise.allSettled([identity.tenants.members.list(tenantId, { page: membersPage, pageSize: membersPageSize, q: appliedMemberSearch || undefined }), client.auth.me()]);
    const memberResult = results[0];
    if (memberResult.status === 'fulfilled') {
      try { await applyMembersPage(membersPage, membersPageSize, appliedMemberSearch || undefined, memberResult.value); }
      catch (reason) { setError(errorText(reason, t('mobileAdministration.loadFailed'))); }
    } else setError(errorText(memberResult.reason, t('mobileAdministration.loadFailed')));
    const authResult = results[1];
    let nextManageTenant = systemAdmin;
    let nextViewAudit = systemAdmin;
    if (authResult.status === 'fulfilled') {
      const nextUserId = String(authResult.value.user?.id ?? '');
      const nextRole = tenantRoleFromMemberships(authResult.value.memberships, tenantId) ?? (memberResult.status === 'fulfilled' ? memberResult.value.items.find((member) => member.user_id === nextUserId)?.role : undefined);
      setCurrentUserId(nextUserId); setCurrentRole(nextRole);
      nextManageTenant ||= canManageTenant(nextRole); nextViewAudit ||= canViewAudit(nextRole);
    } else setError((current) => current ?? errorText(authResult.reason, t('mobileAdministration.loadFailed')));
    if (nextManageTenant) {
      try { setInvitations((await identity.tenants.invitations.listTenant(tenantId, { page: 1, pageSize: 100 })).items); }
      catch (reason) { setError((current) => current ?? errorText(reason, t('mobileAdministration.loadFailed'))); }
    } else setInvitations([]);
    if (nextViewAudit) {
      try { setAudit((await identity.tenants.auditLog.list(tenantId, { limit: 50 })).items); }
      catch (reason) { setError((current) => current ?? errorText(reason, t('mobileAdministration.loadFailed'))); }
    } else setAudit([]);
    if (systemAdmin) {
      const adminResults = await Promise.allSettled([client.administration.admins.list({ limit: 100 }), client.administration.settings.list(), client.administration.runtime.queues(), client.administration.apiKeys.list(), client.administration.auditLog.list({ limit: 50 })]);
      const adminResult = adminResults[0];
      const settingsResult = adminResults[1];
      const queueResult = adminResults[2];
      const apiKeysResult = adminResults[3];
      const auditResult = adminResults[4];
      if (adminResult.status === 'fulfilled') setAdmins(adminResult.value.items); else setError((current) => current ?? errorText(adminResult.reason, t('mobileAdministration.loadFailed')));
      if (settingsResult.status === 'fulfilled') setSettings(settingsResult.value); else setError((current) => current ?? errorText(settingsResult.reason, t('mobileAdministration.loadFailed')));
      if (queueResult.status === 'fulfilled') setQueues(queueResult.value); else setError((current) => current ?? errorText(queueResult.reason, t('mobileAdministration.loadFailed')));
      if (apiKeysResult.status === 'fulfilled') setApiKeys(apiKeysResult.value); else setError((current) => current ?? errorText(apiKeysResult.reason, t('mobileAdministration.loadFailed')));
      if (auditResult.status === 'fulfilled') setSystemAudit(auditResult.value); else setError((current) => current ?? errorText(auditResult.reason, t('mobileAdministration.loadFailed')));
    }
    setLoading(false);
  }

  useEffect(() => { void load(); }, [client, tenantId, systemAdmin]);

  // Vue contract (frontend/src/views/settings/TenantMembers.vue): the member
  // search input is debounced 320ms before it is applied as the server-side
  // `q` filter, and applying a new filter resets pagination to page 1.
  const searchDebounceInitRef = useRef(true);
  useEffect(() => {
    if (searchDebounceInitRef.current) { searchDebounceInitRef.current = false; return; }
    const timer = window.setTimeout(() => setAppliedMemberSearch(memberSearch.trim()), 320);
    return () => window.clearTimeout(timer);
  }, [memberSearch]);
  const appliedSearchRef = useRef('');
  useEffect(() => {
    if (appliedMemberSearch === appliedSearchRef.current) return;
    appliedSearchRef.current = appliedMemberSearch;
    setMembersPage(1);
    void reloadMembers(1, membersPageSize);
  }, [appliedMemberSearch, membersPageSize, client, tenantId]);

  // Vue contract (TenantMembers.vue loadMembers): when the requested page
  // overshoots the last page (a removal shrank the roster), re-request the
  // clamped last page instead of rendering an empty list.
  async function applyMembersPage(page: number, pageSize: number, q: string | undefined, prefetched?: MembersPage) {
    const data = prefetched ?? await identity.tenants.members.list(tenantId, { page, pageSize, q });
    const maxPage = Math.max(1, Math.ceil(data.total / Math.max(1, data.pageSize || pageSize)));
    if (page > maxPage) {
      setMembersPage(maxPage);
      const retry = await identity.tenants.members.list(tenantId, { page: maxPage, pageSize, q });
      setMembers(retry.items); setMembersTotal(retry.total); return;
    }
    setMembers(data.items); setMembersTotal(data.total);
  }

  async function reloadMembers(page: number, pageSize: number) {
    try { await applyMembersPage(page, pageSize, appliedMemberSearch || undefined); }
    catch (reason) { setError(errorText(reason, t('mobileAdministration.loadFailed'))); }
  }

  // Pager handlers: t-pagination fires @change for both page and page-size
  // edits and each change reloads the current view server-side.
  function onMembersPage(nextPage: number) { setMembersPage(nextPage); void reloadMembers(nextPage, membersPageSize); }
  function onMembersPageSize(nextSize: number) { setMembersPageSize(nextSize); void reloadMembers(membersPage, nextSize); }

  function onInviteFormSubmit(event: React.FormEvent<HTMLFormElement>) {
    // Vue submitAdd form branch: validate, then swap to the in-place confirm
    // summary; the API only fires from the confirm step.
    event.preventDefault(); if (!manageTenant || !email.trim()) return; setInviteStep('confirm');
  }

  async function sendInvitation() {
    if (!manageTenant || !email.trim()) return; setSaving(true); setError(null);
    try { await identity.tenants.invitations.create(tenantId, { email: email.trim(), role }); setEmail(''); setInviteStep('form'); await load(); }
    catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToSend'))); }
    finally { setSaving(false); }
  }

  async function updateRole(member: TenantMember, nextRole: TenantMember['role']) { if (!manageTenant || !isEditableMember(member, currentUserId)) return; setError(null); try { await identity.tenants.members.updateRole(tenantId, member.user_id, nextRole); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToUpdateRole'))); } }
  async function removeMember(member: TenantMember) { if (!manageTenant || !isEditableMember(member, currentUserId) || !window.confirm(`${t('mobileAdministration.removeTitle')} ${t('mobileAdministration.removeMessage', { name: member.username })}`)) return; setError(null); try { await identity.tenants.members.remove(tenantId, member.user_id); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToRemove'))); } }
  async function revokeInvitation(invitation: TenantInvitation) { if (!manageTenant || !window.confirm(`${t('mobileAdministration.revoke')}: ${invitation.invitee_email ?? invitation.id}?`)) return; setError(null); try { await identity.tenants.invitations.revoke(tenantId, invitation.id); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToRevoke'))); } }

  if (!manageTenant) return <main className="wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t('mobileAdministration.title')}</h1><p className="wk-muted text-muted">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<Card><h2 className="mt-0 mb-3">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="mb-3 w-full" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list m-0 list-none p-0">{members.map((member) => <li key={member.user_id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{member.username}</strong><span className="font-mono text-[0.8rem] text-muted">{member.email} · {roleText(member.role)} · {member.status}</span></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card></main>;

  return <main className="wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{systemAdmin ? t('settings.navGroups.systemAdministration') : t('mobileAdministration.title')}</h1><p className="wk-muted text-muted">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1"><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="mb-3 w-full" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list m-0 list-none p-0">{members.map((member) => <li key={member.user_id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{member.username}</strong><span className="font-mono text-[0.8rem] text-muted">{member.email} · {roleText(member.role)} · {member.status}</span></div><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><TSelect value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(value) => void updateRole(member, String(value) as TenantMember['role'])} className="max-w-[9rem]" options={[{ value: 'owner', label: roleText('owner') }, { value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /><TButton type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>{t('mobileAdministration.remove')}</TButton></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card><Card><h2 className="mt-0 mb-3">{inviteStep === 'confirm' ? t('mobileAdministration.confirmInviteTitle') : t('mobileAdministration.invite')}</h2>{inviteStep === 'confirm' ? <div className="grid gap-[0.8rem]"><p className="wk-muted text-muted m-0">{t('mobileAdministration.confirmInviteBody', { email: email.trim(), role: roleText(role) })}</p><div className="flex items-center justify-end gap-[0.5rem]"><TButton type="button" disabled={saving} onClick={() => setInviteStep('form')}>{t('mobileAdministration.back')}</TButton><TButton type="button" loading={saving} onClick={() => void sendInvitation()}>{t('mobileAdministration.confirmSend')}</TButton></div></div> : <form className="grid gap-[0.8rem]" onSubmit={onInviteFormSubmit}><label className="grid gap-[0.35rem] font-semibold">{t('mobileAdministration.inviteEmail')}<TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} /></label><label className="grid gap-[0.35rem] font-semibold">{t('mobileAdministration.role', { role: '' }).replace(/: $/, '')}<TSelect value={role} onChange={(value) => setRole(String(value) as typeof role)} options={[{ value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /></label><TButton type="submit" loading={saving}>{t('mobileAdministration.sendInvitation')}</TButton></form>}</Card><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.openInvitations')}</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>{t('mobileAdministration.noPendingInvitations')}</Status> : <ul className="wk-list m-0 list-none p-0">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span className="font-mono text-[0.8rem] text-muted">{roleText(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</span></div><TButton type="button" onClick={() => void revokeInvitation(item)}>{t('mobileAdministration.revoke')}</TButton></li>)}</ul>}</Card><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.auditLog')}</h2>{audit.length === 0 ? <Status>{t('mobileAdministration.noAuditEntries')}</Status> : <ul className="wk-list m-0 list-none p-0">{audit.map((item) => <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{item.action}</strong><span className="font-mono text-[0.8rem] text-muted">{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2 className="mt-0 mb-3">{t('settings.navGroups.systemAdministration')}</h2><ul className="wk-list m-0 list-none p-0">{admins.map((item, index) => <li key={String(item.id ?? index)} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><strong>{String(item.username ?? item.email ?? item.id)}</strong><span className="font-mono text-[0.8rem] text-muted">{item.is_active === false ? t('common.disabled') : t('mobileAdministration.status.active')}</span></li>)}</ul></Card><div data-testid="system-administration-panels" className="col-span-2 grid gap-6 max-[720px]:col-span-1">{systemAdminPanelKeys(systemAdmin).map((panel) => panel === 'system-global' ? <SystemGlobalSettingsPanel key={panel} client={client} initialSettings={settings} /> : panel === 'runtime-queues' ? <RuntimeQueuesPanel key={panel} client={client} payload={queues} loading={loading} /> : panel === 'platform-api-keys' ? <PlatformApiKeysPanel key={panel} client={client} initialKeys={apiKeys} /> : <SystemAuditLogPanel key={panel} client={client} payload={systemAudit} />)}</div></> : null}</div></main>;
}

// Vue members pagination surface (TenantMembers.vue t-pagination: show-jumper,
// show-page-number, show-page-size, options 10/20/50/100). Page-number window
// mirrors the React TablePager already shipped for TenantMembersPanel.
const MEMBERS_PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const PAGER_BTN = 'inline-flex h-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent px-[0.3rem] py-0 font-normal text-[rgb(0_0_0/90%)] [font:inherit]';
const PAGER_BTN_DISABLED = ' disabled:text-[rgb(0_0_0/26%)] disabled:cursor-not-allowed disabled:opacity-50 enabled:hover:text-accent';

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

function MembersPager({ total, page, pageSize, onPage, onPageSize, t }: {
  total: number; page: number; pageSize: number;
  onPage: (page: number) => void; onPageSize: (size: number) => void;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const maxPage = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
  const [jump, setJump] = useState(String(page));
  useEffect(() => { setJump(String(page)); }, [page, pageSize]);

  function commitJump() {
    const next = Math.min(maxPage, Math.max(1, Number.parseInt(jump, 10) || page));
    if (next !== page) onPage(next);
    else setJump(String(page));
  }

  return <div className="mt-3 flex flex-wrap items-center justify-end gap-[0.4rem] border-t border-line-soft pt-3 text-xs text-muted max-[720px]:justify-start">
    <span className="mr-auto">{t('mobileAdministration.pager.total', { total })}</span>
    <TSelect className="wk-admin-page-size w-auto!" value={pageSize} onChange={(value) => onPageSize(Number(String(value)))} options={MEMBERS_PAGE_SIZE_OPTIONS.map((size) => ({ value: size, label: t('mobileAdministration.pager.sizePerPage', { size }) }))} />
    <button type="button" className={PAGER_BTN + PAGER_BTN_DISABLED} aria-label={t('common.previous')} disabled={page <= 1} onClick={() => onPage(page - 1)}>‹</button>
    {pageWindow(page, maxPage).map((entry, index) => entry === 'ellipsis'
      ? <span key={'e' + index} className="px-1">…</span>
      : <button key={entry} type="button" className={PAGER_BTN + " aria-[current=page]:bg-accent aria-[current=page]:text-white [&:not([aria-current='page']):hover]:text-accent"} aria-current={entry === page ? 'page' : undefined} onClick={() => onPage(entry)}>{entry}</button>)}
    <button type="button" className={PAGER_BTN + PAGER_BTN_DISABLED} aria-label={t('common.next')} disabled={page >= maxPage} onClick={() => onPage(page + 1)}>›</button>
    <span className="inline-flex items-center gap-[0.3rem]">
      {t('mobileAdministration.pager.jumper')}
      <TInput className="wk-admin-pager-jump h-6 w-[2.6rem]! rounded-md! px-0! text-center!" type="text" value={jump}
        onChange={(value) => setJump(String(value))}
        onBlur={commitJump}
        onKeydown={(_, context) => { if (context.e.key === 'Enter') commitJump(); }} />
      /{maxPage} {t('mobileAdministration.pager.pageUnit')}
    </span>
  </div>;
}
