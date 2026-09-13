import { useEffect, useState, type FormEvent } from 'react';
import type { TenantInvitation, TenantMember, TenantRole, WeKnoraClient, AuditLog } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import './TenantMembersPanel.css';

type Role = 'viewer' | 'admin' | 'owner' | 'system-admin';
type Props = { client: WeKnoraClient; tenantId: number; role: Role; initialMembers?: { items: TenantMember[]; total: number } };
const roles: TenantRole[] = ['owner', 'admin', 'contributor', 'viewer'];

export function TenantMembersPanel({ client, tenantId, role, initialMembers }: Props) {
  const t = createTranslator(useAppLocale());
  const canManage = role === 'owner' || role === 'admin';
  const [members, setMembers] = useState(initialMembers?.items ?? []);
  const [total, setTotal] = useState(initialMembers?.total ?? 0);
  const [query, setQuery] = useState('');
  const [email, setEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<TenantRole>('contributor');
  const [shareLinkRole, setShareLinkRole] = useState<TenantRole>('contributor');
  const [shareLink, setShareLink] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(initialMembers === undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [invitations, setInvitations] = useState<TenantInvitation[]>([]);
  const [audit, setAudit] = useState<AuditLog[]>([]);
  const [showAudit, setShowAudit] = useState(false);
  const [currentUserId, setCurrentUserId] = useState('');

  async function load(nextPage = page, nextQuery = query) {
    setLoading(true); setError(null);
    try { const result = await client.identity.tenants.members.list(tenantId, { q: nextQuery.trim() || undefined, page: nextPage, pageSize: 50 }); setMembers(result.items); setTotal(result.total); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load tenant members'); }
    finally { setLoading(false); }
  }
  useEffect(() => { if (initialMembers === undefined) void load(1, ''); }, [client, tenantId]);
  useEffect(() => { void client.auth.me().then((result) => setCurrentUserId(String(result.user?.id ?? ''))).catch(() => setCurrentUserId('')); }, [client]);
  async function loadInvitations() { if (!canManage) return; try { setInvitations((await client.identity.tenants.invitations.listTenant(tenantId, { page: 1, pageSize: 50 })).items.filter((item) => item.status === 'pending')); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load invitations'); } }
  async function loadAudit() { try { setAudit((await client.identity.tenants.auditLog.list(tenantId, { limit: 50 })).items); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load audit log'); } }
  useEffect(() => { if (canManage) void loadInvitations(); }, [client, tenantId, canManage]);
  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!canManage || !email.trim() || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.identity.tenants.invitations.create(tenantId, { email: email.trim(), role: inviteRole }); setEmail(''); setNotice('Invitation sent.'); await load(); await loadInvitations(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to invite member'); }
    finally { setBusy(false); }
  }
  async function update(member: TenantMember, nextRole: TenantRole) {
    if (!canManage || busy || member.role === nextRole) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.identity.tenants.members.updateRole(tenantId, member.user_id, nextRole); setNotice('Member role updated.'); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to update member role'); }
    finally { setBusy(false); }
  }
  async function remove(member: TenantMember) {
    if (!canManage || busy || !window.confirm(`Remove ${member.username || member.email}?`)) return;
    setBusy(true); setError(null); setNotice(null);
    try { await client.identity.tenants.members.remove(tenantId, member.user_id); setNotice('Member removed.'); await load(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to remove member'); }
    finally { setBusy(false); }
  }
  function search(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setPage(1); void load(1, query); }
  function clearSearch() { setQuery(''); setPage(1); void load(1, ''); }
  async function revoke(invitation: TenantInvitation) { if (!canManage || busy || !window.confirm(`Revoke invitation for ${invitation.invitee_email ?? invitation.invitee_user_id}?`)) return; setBusy(true); try { await client.identity.tenants.invitations.revoke(tenantId, invitation.id); setNotice('Invitation revoked.'); await loadInvitations(); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to revoke invitation'); } finally { setBusy(false); } }
  async function createShareLink() {
    if (!canManage || busy) return;
    setBusy(true); setError(null); setNotice(null); setShareLink(null);
    try {
      const invitation = await client.identity.tenants.invitations.createInviteLink(tenantId, { role: shareLinkRole });
      if (!invitation.invite_url) throw new Error('The server did not return an invite URL.');
      setShareLink(invitation.invite_url);
      try { await navigator.clipboard?.writeText(invitation.invite_url); setNotice('Share link created and copied.'); }
      catch { setNotice('Share link created. Copy it from the field below.'); }
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to create share link'); }
    finally { setBusy(false); }
  }

  return <section className="wk-tenant-members" data-testid="tenant-members-settings">
    <div className="members-list-header">
      <div className="members-list-titlewrap"><span className="members-list-title">{t('tenantMember.listTitle')}</span><span className="members-list-count-badge">{total}</span></div>
      <form className="members-list-actions" role="search" onSubmit={search}>
        <div className="members-list-search"><input type="search" aria-label={t('tenantMember.searchPlaceholder')} placeholder={t('tenantMember.searchPlaceholder')} value={query} onChange={(event) => setQuery(event.target.value)} /></div>
        {query ? <Button type="button" aria-label="Clear search" onClick={clearSearch}>Clear</Button> : null}
        <Button type="submit" disabled={loading}>Search</Button>
      </form>
    </div>
    <p className="wk-muted">Invite colleagues and manage tenant roles. Server permissions remain authoritative.</p>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}
    {canManage ? <div className="wk-settings-panel-heading"><h4>Pending invitations ({invitations.length})</h4><Button type="button" onClick={() => void loadInvitations()}>Refresh invitations</Button></div> : null}
    {canManage && invitations.length > 0 ? <Card><ul className="wk-list">{invitations.map((invitation) => <li key={invitation.id}><div className="wk-list-item-copy"><strong>{invitation.invitee_name ?? invitation.invitee_email ?? invitation.invitee_user_id}</strong><span>{invitation.role} · expires {invitation.expires_at}</span></div><Button type="button" disabled={busy} onClick={() => void revoke(invitation)}>Revoke</Button></li>)}</ul></Card> : null}
    {canManage ? <form className="wk-settings-editor" onSubmit={(event) => void invite(event)}><h4>Invite member</h4><label>Email<input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="invitee@example.com" /></label><label>Role<select value={inviteRole} onChange={(event) => setInviteRole(event.target.value as TenantRole)}>{roles.filter((item) => item !== 'owner').map((item) => <option key={item} value={item}>{item}</option>)}</select></label><Button type="submit" loading={busy}>Send invitation</Button></form> : null}
    {canManage ? <Card><h4>Create share link</h4><div className="wk-list-actions"><label>Role<select value={shareLinkRole} onChange={(event) => setShareLinkRole(event.target.value as TenantRole)}>{roles.filter((item) => item !== 'owner').map((item) => <option key={item} value={item}>{item}</option>)}</select></label><Button type="button" disabled={busy} onClick={() => void createShareLink()}>Create link</Button></div>{shareLink ? <input aria-label="Workspace invite link" readOnly value={shareLink} onFocus={(event) => event.currentTarget.select()} /> : null}</Card> : null}
    <Card><h4>Role permissions</h4><ul className="wk-list">{roles.map((item) => <li key={item}><strong>{item}</strong><span>{item === 'owner' ? 'Full workspace control' : item === 'admin' ? 'Manage members and settings' : item === 'contributor' ? 'Edit workspace content' : 'View workspace content'}</span></li>)}</ul></Card>
    {loading ? <Status>Loading members…</Status> : members.length === 0 ? <Status>{query ? `No members found for “${query}”.` : 'No members configured.'}</Status> : <Card><p className="wk-muted">{total} member(s)</p><ul className="wk-list">{members.map((member) => { const isSelf = member.user_id === currentUserId; return <li key={member.user_id}><div className="wk-list-item-copy"><strong>{member.username}</strong><span>{member.email} · {member.status}{isSelf ? ' · current user' : ''}</span></div><div className="wk-list-actions">{canManage ? <select aria-label={`Role for ${member.username}`} value={member.role} disabled={busy || isSelf} onChange={(event) => void update(member, event.target.value as TenantRole)}>{roles.map((item) => <option key={item} value={item}>{item}</option>)}</select> : <span>{member.role}</span>}{canManage ? <Button type="button" disabled={busy || member.role === 'owner' || isSelf} onClick={() => void remove(member)}>Remove</Button> : null}</div></li>; })}</ul></Card>}
    {total > 50 ? <div className="wk-list-actions"><Button type="button" disabled={page <= 1 || loading} onClick={() => { const next = page - 1; setPage(next); void load(next); }}>Previous</Button><span>Page {page}</span><Button type="button" disabled={page * 50 >= total || loading} onClick={() => { const next = page + 1; setPage(next); void load(next); }}>Next</Button></div> : null}
    {canManage ? <div className="wk-settings-panel-heading"><Button type="button" onClick={() => { setShowAudit((current) => !current); if (!showAudit) void loadAudit(); }}>{showAudit ? 'Hide audit log' : 'Open audit log'}</Button></div> : null}
    {showAudit ? <Card role="region" aria-label="Audit log"><h4>Audit log</h4>{audit.length === 0 ? <Status>No audit events returned.</Status> : <ul className="wk-list">{audit.map((entry) => <li key={entry.id}><div className="wk-list-item-copy"><strong>{entry.action}</strong><span>{entry.outcome} · {entry.created_at}</span><small>{entry.request_method} {entry.request_path}</small></div></li>)}</ul>}</Card> : null}
  </section>;
}
