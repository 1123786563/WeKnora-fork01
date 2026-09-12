import { useEffect, useState } from 'react';
import type { Organization, OrganizationJoinRequest, OrganizationMember, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { formatMessage, isLocale, supportedLocales } from '@weknora/i18n';
import { clampApplicationNote, inviteJoinMode, requestedRoleOf } from './join.ts';
import { buildInviteLink, copyText, sharedResourceRow } from './settings-actions.ts';
import { organizationRoleLabel } from './summary.ts';

function currentLocale(): string {
  const candidate = navigator.language;
  if (isLocale(candidate)) return candidate;
  const base = candidate.split('-')[0];
  return supportedLocales.find((locale) => locale.split('-')[0] === base) ?? 'en-US';
}
function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

export function OrganizationsPage({ client, inviteCode }: { client: WeKnoraClient; inviteCode?: string }) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [selected, setSelected] = useState<Organization | null>(null);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [requests, setRequests] = useState<OrganizationJoinRequest[]>([]);
  const [sharedResources, setSharedResources] = useState<Array<Record<string, unknown>>>([]);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [invitePreview, setInvitePreview] = useState<Record<string, unknown> | null>(null);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteAction, setInviteAction] = useState<'idle' | 'joining' | 'requesting'>('idle');
  const [activeInviteCode, setActiveInviteCode] = useState(inviteCode);
  const [requestRole, setRequestRole] = useState<'admin' | 'editor' | 'viewer'>('viewer');
  const [requestNote, setRequestNote] = useState('');
  const [inviteLink, setInviteLink] = useState('');
  const [notice, setNotice] = useState('');
  const [upgradeRole, setUpgradeRole] = useState<'admin' | 'editor' | 'viewer'>('editor');
  const [upgradeNote, setUpgradeNote] = useState('');
  const locale = currentLocale();
  const organizationsApi = client.identity.organizations;

  function clearInviteFromUrl() {
    const url = new URL(window.location.href);
    url.searchParams.delete('invite_code');
    window.history.replaceState({}, document.title, `${url.pathname}${url.search}${url.hash}`);
  }

  useEffect(() => { setActiveInviteCode(inviteCode); }, [inviteCode]);

  useEffect(() => {
    if (!activeInviteCode) { setInvitePreview(null); return; }
    let active = true;
    setInviteLoading(true);
    void organizationsApi.preview(activeInviteCode).then((preview) => {
      if (active) setInvitePreview(preview);
    }).catch((reason) => {
      if (active) setError(errorText(reason, 'Unable to preview this organization invitation'));
    }).finally(() => { if (active) setInviteLoading(false); });
    return () => { active = false; };
  }, [activeInviteCode, organizationsApi]);

  async function load() {
    setLoading(true); setError(null);
    try { const result = await organizationsApi.list(); setOrganizations(result.items); if (selected) { const next = result.items.find((item) => item.id === selected.id); if (next) setSelected(next); } }
    catch (reason) { setError(errorText(reason, 'Unable to load organizations')); }
    finally { setLoading(false); }
  }
  async function select(organization: Organization) {
    setSelected(organization); setError(null);
    const result = await Promise.allSettled([organizationsApi.members.list(organization.id), organizationsApi.joinRequests.list(organization.id), organizationsApi.knowledgeBaseShares.listForOrganization(organization.id)]);
    const membersResult = result[0];
    const requestsResult = result[1];
    const sharesResult = result[2];
    if (membersResult.status === 'fulfilled') setMembers(membersResult.value.items); else setError(errorText(membersResult.reason, 'Unable to load organization members'));
    if (requestsResult.status === 'fulfilled') setRequests(requestsResult.value.items); else setError((current) => current ?? errorText(requestsResult.reason, 'Unable to load join requests'));
    if (sharesResult.status === 'fulfilled') setSharedResources(sharesResult.value.items); else setError((current) => current ?? errorText(sharesResult.reason, 'Unable to load shared resources'));
  }
  useEffect(() => { void load(); }, [client]);

  async function create(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!name.trim()) return; setSaving(true); setError(null);
    try { const created = await organizationsApi.create({ name: name.trim(), description }); setName(''); setDescription(''); await load(); await select(created); }
    catch (reason) { setError(errorText(reason, 'Unable to create organization; no local row was added.')); }
    finally { setSaving(false); }
  }
  async function updateMemberRole(member: OrganizationMember, nextRole: 'admin' | 'editor' | 'viewer') { if (!selected) return; setError(null); try { await organizationsApi.members.updateRole(selected.id, member.tenant_id, { role: nextRole }); await select(selected); } catch (reason) { setError(errorText(reason, 'Unable to update organization member role.')); } }
  async function removeMember(member: OrganizationMember) { if (!selected || !window.confirm(`Remove ${member.tenant_name ?? member.username}?`)) return; setError(null); try { await organizationsApi.members.remove(selected.id, member.tenant_id); await select(selected); } catch (reason) { setError(errorText(reason, 'Unable to remove organization member.')); } }
  async function reviewRequest(request: OrganizationJoinRequest, approved: boolean) {
    if (!selected || !window.confirm(`${approved ? 'Approve' : 'Reject'} ${request.username}'s join request?`)) return;
    setError(null);
    try { await organizationsApi.joinRequests.review(selected.id, request.id, { approved, role: request.requested_role as 'admin' | 'editor' | 'viewer' }); await select(selected); }
    catch (reason) { setError(errorText(reason, 'Unable to review join request; the server state was kept.')); }
  }
  async function leaveOrganization() {
    if (!selected || !window.confirm(`Leave ${selected.name}?`)) return;
    setError(null);
    try { await organizationsApi.leave(selected.id); setSelected(null); setMembers([]); setRequests([]); setSharedResources([]); await load(); }
    catch (reason) { setError(errorText(reason, 'Unable to leave organization; the server state was kept.')); }
  }
  async function deleteOrganization() {
    if (!selected || !window.confirm(`Delete ${selected.name}? This cannot be undone.`)) return;
    setError(null);
    try { await organizationsApi.remove(selected.id); setSelected(null); setMembers([]); setRequests([]); setSharedResources([]); await load(); }
    catch (reason) { setError(errorText(reason, 'Unable to delete organization; the server state was kept.')); }
  }

  async function acceptInvite() {
    if (!activeInviteCode) return;
    setInviteAction('joining'); setError(null);
    try { const joined = await organizationsApi.join({ invite_code: activeInviteCode }); setInvitePreview(null); setActiveInviteCode(undefined); clearInviteFromUrl(); await load(); await select(joined); }
    catch (reason) { setError(errorText(reason, 'Unable to join organization; no local row was added.')); }
    finally { setInviteAction('idle'); }
  }

  async function requestInvite() {
    if (!activeInviteCode) return;
    setInviteAction('requesting'); setError(null);
    try {
      await organizationsApi.submitJoinRequest({ invite_code: activeInviteCode, role: requestRole, ...(requestNote.trim() ? { message: clampApplicationNote(requestNote) } : {}) });
      setInvitePreview(null); setActiveInviteCode(undefined); setRequestNote('');
      clearInviteFromUrl();
    }
    catch (reason) { setError(errorText(reason, 'Unable to submit organization join request.')); }
    finally { setInviteAction('idle'); }
  }

  async function generateInviteLink() {
    if (!selected) return;
    setError(null); setNotice('');
    try {
      const { inviteCode } = await organizationsApi.generateInviteCode(selected.id);
      const link = buildInviteLink(inviteCode, { origin: window.location.origin, pathname: window.location.pathname, search: window.location.search });
      setInviteLink(link);
      const copied = await copyText(link);
      setNotice(copied ? 'Invite link copied to the clipboard.' : '');
    } catch (reason) { setError(errorText(reason, 'Unable to generate an invite link.')); }
  }

  async function unshareKnowledgeBase(row: ReturnType<typeof sharedResourceRow>) {
    if (!selected) return;
    if (!window.confirm(`Unshare "${row.name}" from ${selected.name}?`)) return;
    setError(null);
    try { await organizationsApi.knowledgeBaseShares.remove(row.knowledgeBaseId, row.shareId); await select(selected); }
    catch (reason) { setError(errorText(reason, 'Unable to remove the knowledge-base share.')); }
  }

  async function submitUpgradeRequest(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    setError(null); setNotice('');
    try {
      await organizationsApi.requestRoleUpgrade(selected.id, { requested_role: upgradeRole, ...(upgradeNote.trim() ? { message: clampApplicationNote(upgradeNote) } : {}) });
      setUpgradeNote('');
      setNotice('Role-upgrade request submitted.');
    } catch (reason) { setError(errorText(reason, 'Unable to submit the role-upgrade request.')); }
  }

  const inviteName = typeof invitePreview?.name === 'string' ? invitePreview.name : 'Organization invitation';
  const inviteDescription = typeof invitePreview?.description === 'string' ? invitePreview.description : 'Review this invitation before joining.';
  const joinMode = inviteJoinMode(invitePreview as Record<string, unknown> | null);

  return <main className="wk-page wk-organizations-page"><header className="wk-header"><div><p className="wk-eyebrow">Workspace organizations</p><h1>Organizations</h1><p className="wk-muted">Manage organization membership and shared-resource visibility through server-owned permissions.</p></div><Button type="button" onClick={() => void load()} disabled={loading}>Reload</Button></header>{error ? <Status tone="error">{error}</Status> : null}{activeInviteCode ? <Card><h2>{inviteName}</h2>{inviteLoading ? <Status>Checking invitation…</Status> : <><p className="wk-muted">{inviteDescription}</p>{joinMode === 'member' ? <Status tone="success">{t(locale, 'organization.invite.alreadyMember')}</Status> : <><p className="wk-muted">{t(locale, 'organization.invite.approvalLabel')}: {joinMode === 'request' ? t(locale, 'organization.invite.needApproval') : t(locale, 'organization.invite.noApproval')}</p>{joinMode === 'request' ? <div className="wk-admin-form"><label>{t(locale, 'organization.invite.requestRole')}<select value={requestRole} onChange={(event) => setRequestRole(event.target.value as 'admin' | 'editor' | 'viewer')}><option value="viewer">{t(locale, 'organization.role.viewer')}</option><option value="editor">{t(locale, 'organization.role.editor')}</option><option value="admin">{t(locale, 'organization.role.admin')}</option></select></label><label>{t(locale, 'organization.invite.applicationNote')}<textarea rows={3} maxLength={500} value={requestNote} onChange={(event) => setRequestNote(clampApplicationNote(event.target.value))} /></label></div> : null}<div className="wk-list-actions">{joinMode === 'join' ? <Button type="button" loading={inviteAction === 'joining'} onClick={() => void acceptInvite()}>{t(locale, 'organization.invite.primaryJoin')}</Button> : <Button type="button" loading={inviteAction === 'requesting'} onClick={() => void requestInvite()}>{t(locale, 'organization.invite.submitRequest')}</Button>}</div></>}</>}</Card> : null}<div className="wk-organization-layout"><Card><h2>Organizations</h2>{loading ? <Status>Loading…</Status> : organizations.length === 0 ? <Status>No organizations returned.</Status> : <ul className="wk-list">{organizations.map((organization) => <li key={organization.id} className={selected?.id === organization.id ? 'is-selected' : ''}><button type="button" className="wk-organization-select" onClick={() => void select(organization)}><strong>{organization.name}</strong><span>{String(organization.member_count ?? 0)} members · {String(organization.share_count ?? 0)} KB shares · {String(organization.my_role ?? 'member')}</span></button></li>)}</ul>}</Card><Card><h2>Create organization</h2><form className="wk-admin-form" onSubmit={create}><label>Name<input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>Description<textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} /></label><Button type="submit" loading={saving}>Create organization</Button></form></Card>{selected ? <Card className="wk-organization-detail"><div className="wk-settings-panel-heading"><div><h2>{selected.name}</h2><p className="wk-muted">{selected.description || 'No description.'}</p></div><div className="wk-list-actions"><Button type="button" onClick={() => void leaveOrganization()}>Leave</Button><Button type="button" onClick={() => void deleteOrganization()}>Delete</Button></div></div><h3>Members</h3>{members.length === 0 ? <Status>No members returned.</Status> : <ul className="wk-list">{members.map((member) => <li key={member.id}><div className="wk-list-item-copy"><strong>{member.tenant_name ?? member.username}</strong><span>{member.email} · {organizationRoleLabel(member.role)}</span></div><div className="wk-list-actions"><select value={member.role} onChange={(event) => void updateMemberRole(member, event.target.value as 'admin' | 'editor' | 'viewer')}><option value="admin">Admin</option><option value="editor">Editor</option><option value="viewer">Viewer</option></select><Button type="button" onClick={() => void removeMember(member)}>Remove</Button></div></li>)}</ul>}<h3>Join requests</h3>{requests.filter((request) => request.status === 'pending').length === 0 ? <Status>No pending join requests.</Status> : <ul className="wk-list">{requests.filter((request) => request.status === 'pending').map((request) => <li key={request.id}><div className="wk-list-item-copy"><strong>{request.username}</strong><span>{String(request.requested_role)} · {request.created_at}</span></div><div className="wk-list-actions"><Button type="button" onClick={() => void reviewRequest(request, true)}>Approve</Button><Button type="button" onClick={() => void reviewRequest(request, false)}>Reject</Button></div></li>)}</ul>}<h3>Shared knowledge bases</h3>{sharedResources.length === 0 ? <Status>No shared knowledge bases returned.</Status> : <ul className="wk-list">{sharedResources.map((resource, index) => { const row = sharedResourceRow(resource); return <li key={row.shareId || index}><div className="wk-list-item-copy"><strong>{row.name}</strong><span>{row.permission || 'permission unavailable'} · source tenant {String((resource as Record<string, unknown>).source_tenant_id ?? 'unknown')}</span></div>{row.canUnshare ? <div className="wk-list-actions"><Button type="button" onClick={() => void unshareKnowledgeBase(row)}>Unshare</Button></div> : null}</li>; })}</ul>}<h3>Invite link</h3><div className="wk-list-actions"><Button type="button" onClick={() => void generateInviteLink()}>Generate invite link</Button>{inviteLink ? <><code>{inviteLink}</code><Button type="button" onClick={() => { void copyText(inviteLink).then((copied) => setNotice(copied ? 'Invite link copied.' : '')); }}>Copy</Button></> : null}</div><p className="wk-muted">Anyone opening this link can preview the organization and join or request to join.</p><h3>Request role upgrade</h3><form className="wk-admin-form" onSubmit={submitUpgradeRequest}><label>Requested role<select value={upgradeRole} onChange={(event) => setUpgradeRole(event.target.value as 'admin' | 'editor' | 'viewer')}><option value="viewer">Viewer</option><option value="editor">Editor</option><option value="admin">Admin</option></select></label><label>Message<textarea rows={2} maxLength={500} value={upgradeNote} onChange={(event) => setUpgradeNote(clampApplicationNote(event.target.value))} /></label><Button type="submit">Submit request</Button></form></Card> : null}</div></main>;
}
