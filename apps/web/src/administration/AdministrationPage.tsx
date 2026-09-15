import { useEffect, useState } from 'react';
import type { AuditLog, RuntimeQueues, SystemAdminUser, SystemSetting, TenantInvitation, TenantMember, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Select, Status } from '@weknora/ui';
import { formatMessage } from '@weknora/i18n';
import { useAppLocale } from '../i18n.ts';
import { canManageTenant, canViewAudit, invitationIsOpen, isEditableMember, tenantRoleFromMemberships } from './summary.ts';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

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
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<'admin' | 'contributor' | 'viewer'>('viewer');
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentUserId, setCurrentUserId] = useState('');
  const [currentRole, setCurrentRole] = useState<string | undefined>();
  const identity = client.identity;
  const manageTenant = systemAdmin || canManageTenant(currentRole);

  async function load() {
    setLoading(true); setError(null);
    const results = await Promise.allSettled([identity.tenants.members.list(tenantId, { page: 1, pageSize: 100 }), client.auth.me()]);
    const memberResult = results[0];
    if (memberResult.status === 'fulfilled') setMembers(memberResult.value.items); else setError(errorText(memberResult.reason, t('mobileAdministration.loadFailed')));
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
      const adminResults = await Promise.allSettled([client.administration.admins.list({ limit: 100 }), client.administration.settings.list(), client.administration.runtime.queues()]);
      const adminResult = adminResults[0];
      const settingsResult = adminResults[1];
      const queueResult = adminResults[2];
      if (adminResult.status === 'fulfilled') setAdmins(adminResult.value.items); else setError((current) => current ?? errorText(adminResult.reason, t('mobileAdministration.loadFailed')));
      if (settingsResult.status === 'fulfilled') setSettings(settingsResult.value); else setError((current) => current ?? errorText(settingsResult.reason, t('mobileAdministration.loadFailed')));
      if (queueResult.status === 'fulfilled') setQueues(queueResult.value); else setError((current) => current ?? errorText(queueResult.reason, t('mobileAdministration.loadFailed')));
    }
    setLoading(false);
  }

  useEffect(() => { void load(); }, [client, tenantId, systemAdmin]);

  async function invite(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!manageTenant || !email.trim()) return; setSaving(true); setError(null);
    try { await identity.tenants.invitations.create(tenantId, { email: email.trim(), role }); setEmail(''); await load(); }
    catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToSend'))); }
    finally { setSaving(false); }
  }

  async function updateRole(member: TenantMember, nextRole: TenantMember['role']) { if (!manageTenant || !isEditableMember(member, currentUserId)) return; setError(null); try { await identity.tenants.members.updateRole(tenantId, member.user_id, nextRole); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToUpdateRole'))); } }
  async function removeMember(member: TenantMember) { if (!manageTenant || !isEditableMember(member, currentUserId) || !window.confirm(`${t('mobileAdministration.removeTitle')} ${t('mobileAdministration.removeMessage', { name: member.username })}`)) return; setError(null); try { await identity.tenants.members.remove(tenantId, member.user_id); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToRemove'))); } }
  async function revokeInvitation(invitation: TenantInvitation) { if (!manageTenant || !window.confirm(`${t('mobileAdministration.revoke')}: ${invitation.invitee_email ?? invitation.id}?`)) return; setError(null); try { await identity.tenants.invitations.revoke(tenantId, invitation.id); await load(); } catch (reason) { setError(errorText(reason, t('mobileAdministration.unableToRevoke'))); } }

  if (!manageTenant) return <main className="wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{t('mobileAdministration.title')}</h1><p className="wk-muted text-muted">{t('mobileAdministration.readOnly')}</p></div><Button type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</Button></header>{error ? <Status tone="error">{error}</Status> : null}<Card><h2 className="mt-0 mb-3">{t('mobileAdministration.members', { count: members.length })}</h2>{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list m-0 list-none p-0">{members.map((member) => <li key={member.user_id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{member.username}</strong><span className="font-mono text-[0.8rem] text-muted">{member.email} · {roleText(member.role)} · {member.status}</span></div></li>)}</ul>}</Card></main>;

  return <main className="wk-page max-w-[1180px]! mx-auto box-border px-[1.25rem] py-12"><header className="wk-header mb-6 flex items-start justify-between gap-4"><div><p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{systemAdmin ? t('settings.navGroups.systemAdministration') : t('mobileAdministration.title')}</h1><p className="wk-muted text-muted">{t('mobileAdministration.readOnly')}</p></div><Button type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</Button></header>{error ? <Status tone="error">{error}</Status> : null}<div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1"><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.members', { count: members.length })}</h2>{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list m-0 list-none p-0">{members.map((member) => <li key={member.user_id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{member.username}</strong><span className="font-mono text-[0.8rem] text-muted">{member.email} · {roleText(member.role)} · {member.status}</span></div><div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"><Select value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(event) => void updateRole(member, event.target.value as TenantMember['role'])} className="max-w-[9rem]"><option value="owner">{roleText('owner')}</option><option value="admin">{roleText('admin')}</option><option value="contributor">{roleText('contributor')}</option><option value="viewer">{roleText('viewer')}</option></Select><Button type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>{t('mobileAdministration.remove')}</Button></div></li>)}</ul>}</Card><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.invite')}</h2><form className="grid gap-[0.8rem]" onSubmit={invite}><label className="grid gap-[0.35rem] font-semibold">{t('mobileAdministration.inviteEmail')}<Input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label><label className="grid gap-[0.35rem] font-semibold">{t('mobileAdministration.role', { role: '' }).replace(/: $/, '')}<Select value={role} onChange={(event) => setRole(event.target.value as typeof role)}><option value="admin">{roleText('admin')}</option><option value="contributor">{roleText('contributor')}</option><option value="viewer">{roleText('viewer')}</option></Select></label><Button type="submit" loading={saving}>{t('mobileAdministration.sendInvitation')}</Button></form></Card><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.openInvitations')}</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>{t('mobileAdministration.noPendingInvitations')}</Status> : <ul className="wk-list m-0 list-none p-0">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span className="font-mono text-[0.8rem] text-muted">{roleText(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</span></div><Button type="button" onClick={() => void revokeInvitation(item)}>{t('mobileAdministration.revoke')}</Button></li>)}</ul>}</Card><Card><h2 className="mt-0 mb-3">{t('mobileAdministration.auditLog')}</h2>{audit.length === 0 ? <Status>{t('mobileAdministration.noAuditEntries')}</Status> : <ul className="wk-list m-0 list-none p-0">{audit.map((item) => <li key={item.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{item.action}</strong><span className="font-mono text-[0.8rem] text-muted">{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2 className="mt-0 mb-3">{t('settings.navGroups.systemAdministration')}</h2><ul className="wk-list m-0 list-none p-0">{admins.map((item, index) => <li key={String(item.id ?? index)} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><strong>{String(item.username ?? item.email ?? item.id)}</strong><span className="font-mono text-[0.8rem] text-muted">{item.is_active === false ? t('common.disabled') : t('mobileAdministration.status.active')}</span></li>)}</ul></Card><Card><h2 className="mt-0 mb-3">{t('settings.system')}</h2><ul className="wk-list m-0 list-none p-0">{settings.map((item, index) => <li key={String(item.id ?? index)} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><div className="wk-list-item-copy grid gap-[0.2rem] min-w-0"><strong>{String(item.key ?? t('common.name'))}</strong><span className="font-mono text-[0.8rem] text-muted">{item.is_secret === true ? t('common.disabled') : String(item.value ?? '')}</span></div></li>)}</ul><p className="wk-muted text-muted">{t('mobileAdministration.readOnly')}</p></Card><Card><h2 className="mt-0 mb-3">{t('settings.system')}</h2><Status>{queues ? `${t('common.success')}: ${queues.available === true ? t('common.yes') : t('common.no')}` : t('mobileAdministration.noMembers')}</Status></Card></> : null}</div></main>;
}
