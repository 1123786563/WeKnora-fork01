import { useEffect, useState } from 'react';
import type { Organization, OrganizationJoinRequest, OrganizationMember, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { organizationRoleLabel } from './summary.ts';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

export function OrganizationsPage({ client }: { client: WeKnoraClient }) {
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
  const organizationsApi = client.identity.organizations;

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

  return <main className="wk-page wk-organizations-page"><header className="wk-header"><div><p className="wk-eyebrow">Workspace organizations</p><h1>Organizations</h1><p className="wk-muted">Manage organization membership and shared-resource visibility through server-owned permissions.</p></div><Button type="button" onClick={() => void load()} disabled={loading}>Reload</Button></header>{error ? <Status tone="error">{error}</Status> : null}<div className="wk-organization-layout"><Card><h2>Organizations</h2>{loading ? <Status>Loading…</Status> : organizations.length === 0 ? <Status>No organizations returned.</Status> : <ul className="wk-list">{organizations.map((organization) => <li key={organization.id} className={selected?.id === organization.id ? 'is-selected' : ''}><button type="button" className="wk-organization-select" onClick={() => void select(organization)}><strong>{organization.name}</strong><span>{String(organization.member_count ?? 0)} members · {String(organization.share_count ?? 0)} KB shares · {String(organization.my_role ?? 'member')}</span></button></li>)}</ul>}</Card><Card><h2>Create organization</h2><form className="wk-admin-form" onSubmit={create}><label>Name<input required value={name} onChange={(event) => setName(event.target.value)} /></label><label>Description<textarea rows={3} value={description} onChange={(event) => setDescription(event.target.value)} /></label><Button type="submit" loading={saving}>Create organization</Button></form></Card>{selected ? <Card className="wk-organization-detail"><div className="wk-settings-panel-heading"><div><h2>{selected.name}</h2><p className="wk-muted">{selected.description || 'No description.'}</p></div><div className="wk-list-actions"><Button type="button" onClick={() => void leaveOrganization()}>Leave</Button><Button type="button" onClick={() => void deleteOrganization()}>Delete</Button></div></div><h3>Members</h3>{members.length === 0 ? <Status>No members returned.</Status> : <ul className="wk-list">{members.map((member) => <li key={member.id}><div className="wk-list-item-copy"><strong>{member.tenant_name ?? member.username}</strong><span>{member.email} · {organizationRoleLabel(member.role)}</span></div><div className="wk-list-actions"><select value={member.role} onChange={(event) => void updateMemberRole(member, event.target.value as 'admin' | 'editor' | 'viewer')}><option value="admin">Admin</option><option value="editor">Editor</option><option value="viewer">Viewer</option></select><Button type="button" onClick={() => void removeMember(member)}>Remove</Button></div></li>)}</ul>}<h3>Join requests</h3>{requests.filter((request) => request.status === 'pending').length === 0 ? <Status>No pending join requests.</Status> : <ul className="wk-list">{requests.filter((request) => request.status === 'pending').map((request) => <li key={request.id}><div className="wk-list-item-copy"><strong>{request.username}</strong><span>{String(request.requested_role)} · {request.created_at}</span></div><div className="wk-list-actions"><Button type="button" onClick={() => void reviewRequest(request, true)}>Approve</Button><Button type="button" onClick={() => void reviewRequest(request, false)}>Reject</Button></div></li>)}</ul>}<h3>Shared knowledge bases</h3>{sharedResources.length === 0 ? <Status>No shared knowledge bases returned.</Status> : <ul className="wk-list">{sharedResources.map((resource, index) => <li key={String(resource.share_id ?? resource.id ?? index)}><div className="wk-list-item-copy"><strong>{String(resource.knowledge_base_name ?? resource.name ?? resource.knowledge_base_id ?? 'Shared knowledge base')}</strong><span>{String(resource.permission ?? 'permission unavailable')} · source tenant {String(resource.source_tenant_id ?? 'unknown')}</span></div></li>)}</ul>}</Card> : null}</div></main>;
}
