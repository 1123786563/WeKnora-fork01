import { useEffect, useState } from 'react';
import type { AuditLog, RuntimeQueues, SystemAdminUser, SystemSetting, TenantInvitation, TenantMember, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { invitationIsOpen, roleLabel } from './summary.ts';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

export function AdministrationPage({ client, tenantId, systemAdmin = false }: { client: WeKnoraClient; tenantId: number; systemAdmin?: boolean }) {
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
  const identity = client.identity;

  async function load() {
    setLoading(true); setError(null);
    const results = await Promise.allSettled([identity.tenants.members.list(tenantId, { page: 1, pageSize: 100 }), identity.tenants.invitations.listTenant(tenantId, { page: 1, pageSize: 100 }), identity.tenants.auditLog.list(tenantId, { limit: 50 })]);
    const memberResult = results[0];
    const invitationResult = results[1];
    const auditResult = results[2];
    if (memberResult.status === 'fulfilled') setMembers(memberResult.value.items); else setError(errorText(memberResult.reason, 'Unable to load members'));
    if (invitationResult.status === 'fulfilled') setInvitations(invitationResult.value.items); else setError((current) => current ?? errorText(invitationResult.reason, 'Unable to load invitations'));
    if (auditResult.status === 'fulfilled') setAudit(auditResult.value.items); else setError((current) => current ?? errorText(auditResult.reason, 'Unable to load audit log'));
    if (systemAdmin) {
      const adminResults = await Promise.allSettled([client.administration.admins.list({ limit: 100 }), client.administration.settings.list(), client.administration.runtime.queues()]);
      const adminResult = adminResults[0];
      const settingsResult = adminResults[1];
      const queueResult = adminResults[2];
      if (adminResult.status === 'fulfilled') setAdmins(adminResult.value.items); else setError((current) => current ?? errorText(adminResult.reason, 'Unable to load system administrators'));
      if (settingsResult.status === 'fulfilled') setSettings(settingsResult.value); else setError((current) => current ?? errorText(settingsResult.reason, 'Unable to load system settings'));
      if (queueResult.status === 'fulfilled') setQueues(queueResult.value); else setError((current) => current ?? errorText(queueResult.reason, 'Unable to load runtime queues'));
    }
    setLoading(false);
  }

  useEffect(() => { void load(); }, [client, tenantId, systemAdmin]);

  async function invite(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!email.trim()) return; setSaving(true); setError(null);
    try { await identity.tenants.invitations.create(tenantId, { email: email.trim(), role }); setEmail(''); await load(); }
    catch (reason) { setError(errorText(reason, 'Unable to send invitation; no local row was added.')); }
    finally { setSaving(false); }
  }

  async function updateRole(member: TenantMember, nextRole: TenantMember['role']) { setError(null); try { await identity.tenants.members.updateRole(tenantId, member.user_id, nextRole); await load(); } catch (reason) { setError(errorText(reason, 'Unable to update member role.')); } }
  async function removeMember(member: TenantMember) { if (!window.confirm(`Remove ${member.username}?`)) return; setError(null); try { await identity.tenants.members.remove(tenantId, member.user_id); await load(); } catch (reason) { setError(errorText(reason, 'Unable to remove member; the server state was kept.')); } }
  async function revokeInvitation(invitation: TenantInvitation) { if (!window.confirm(`Revoke invitation ${invitation.invitee_email ?? invitation.id}?`)) return; setError(null); try { await identity.tenants.invitations.revoke(tenantId, invitation.id); await load(); } catch (reason) { setError(errorText(reason, 'Unable to revoke invitation.')); } }

  return <main className="wk-page wk-administration-page"><header className="wk-header"><div><p className="wk-eyebrow">Workspace administration · tenant {tenantId}</p><h1>{systemAdmin ? 'Administration' : 'Members and audit'}</h1><p className="wk-muted">Server-confirmed membership and audit state. Failed mutations never remove local rows optimistically.</p></div><Button type="button" onClick={() => void load()} disabled={loading}>Reload</Button></header>{error ? <Status tone="error">{error}</Status> : null}<div className="wk-admin-grid"><Card><h2>Members</h2>{loading ? <Status>Loading…</Status> : members.length === 0 ? <Status>No members returned.</Status> : <ul className="wk-list">{members.map((member) => <li key={member.user_id}><div className="wk-list-item-copy"><strong>{member.username}</strong><span>{member.email} · {roleLabel(member.role)} · {member.status}</span></div><div className="wk-list-actions"><select value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(event) => void updateRole(member, event.target.value as TenantMember['role'])}><option value="owner">Owner</option><option value="admin">Admin</option><option value="contributor">Contributor</option><option value="viewer">Viewer</option></select><Button type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>Remove</Button></div></li>)}</ul>}</Card><Card><h2>Invite member</h2><form className="wk-admin-form" onSubmit={invite}><label>Email<input type="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label><label>Role<select value={role} onChange={(event) => setRole(event.target.value as typeof role)}><option value="admin">Admin</option><option value="contributor">Contributor</option><option value="viewer">Viewer</option></select></label><Button type="submit" loading={saving}>Send invitation</Button></form></Card><Card><h2>Open invitations</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>No pending invitations.</Status> : <ul className="wk-list">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id}><div className="wk-list-item-copy"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span>{roleLabel(item.role)} · expires {item.expires_at}</span></div><Button type="button" onClick={() => void revokeInvitation(item)}>Revoke</Button></li>)}</ul>}</Card><Card><h2>Audit log</h2>{audit.length === 0 ? <Status>No audit entries returned.</Status> : <ul className="wk-list">{audit.map((item) => <li key={item.id}><div className="wk-list-item-copy"><strong>{item.action}</strong><span>{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2>System administrators</h2><ul className="wk-list">{admins.map((item, index) => <li key={String(item.id ?? index)}><strong>{String(item.username ?? item.email ?? item.id)}</strong><span>{item.is_active === false ? 'inactive' : 'active'}</span></li>)}</ul></Card><Card><h2>System settings</h2><ul className="wk-list">{settings.map((item, index) => <li key={String(item.id ?? index)}><div className="wk-list-item-copy"><strong>{String(item.key ?? 'setting')}</strong><span>{item.is_secret === true ? 'secret' : String(item.value ?? '')}</span></div></li>)}</ul><p className="wk-muted">System settings are listed here; writes remain behind the explicit system-admin configuration flow.</p></Card><Card><h2>Runtime queues</h2><Status>{queues ? `Runtime endpoint available: ${queues.available === true ? 'yes' : 'no'}` : 'No queue state returned.'}</Status></Card></> : null}</div></main>;
}
