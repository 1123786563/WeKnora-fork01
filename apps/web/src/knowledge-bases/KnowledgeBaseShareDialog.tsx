import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Status } from '@weknora/ui';

type Share = Record<string, unknown> & { id: string; organization_id?: string; organization_name?: string; permission?: string };
type Props = { client: WeKnoraClient; knowledgeBaseId: string; knowledgeBaseName: string; open: boolean; onClose: () => void; onChanged?: () => void };
function canShare(org: Organization): boolean { const row = org as Record<string, unknown>; return row.is_owner === true || row.my_role === 'admin' || row.my_role === 'editor'; }

export function KnowledgeBaseShareDialog({ client, knowledgeBaseId, knowledgeBaseName, open, onClose, onChanged }: Props) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [shares, setShares] = useState<Share[]>([]);
  const [organizationId, setOrganizationId] = useState('');
  const [permission, setPermission] = useState<'viewer' | 'editor'>('viewer');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [showShareList, setShowShareList] = useState(false);
  const busyRef = useRef(false);
  async function load() {
    setLoading(true); setError(null);
    try { const [orgs, currentShares] = await Promise.all([client.identity.organizations.list(), client.identity.organizations.knowledgeBaseShares.list(knowledgeBaseId)]); setOrganizations(orgs.items.filter(canShare)); setShares(currentShares.items as Share[]); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load knowledge-base shares'); }
    finally { setLoading(false); }
  }
  useEffect(() => { if (open) { setOrganizationId(''); setNotice(null); setShowShareList(false); void load(); } }, [open, knowledgeBaseId]);
  async function share(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!organizationId || busyRef.current) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.create(knowledgeBaseId, { organization_id: organizationId, permission }); setNotice('Knowledge base shared.'); await load(); onChanged?.(); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to share knowledge base'); } finally { busyRef.current = false; setBusy(false); } }
  async function unshare(item: Share) { if (busyRef.current || !window.confirm(`Remove sharing with ${item.organization_name ?? item.organization_id ?? 'organization'}?`)) return; busyRef.current = true; setBusy(true); setError(null); try { await client.identity.organizations.knowledgeBaseShares.remove(knowledgeBaseId, item.id); setNotice('Sharing removed.'); await load(); onChanged?.(); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to remove share'); } finally { busyRef.current = false; setBusy(false); } }
  const sharedIds = new Set(shares.map((item) => String(item.organization_id ?? '')));
  const available = organizations.filter((item) => !sharedIds.has(item.id));
  return <Dialog open={open} title={`Share ${knowledgeBaseName}`} onClose={onClose}>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{loading ? <Status>Loading shares</Status> : null}
    {showShareList ? <>
      <div className="wk-list-actions"><Button type="button" onClick={() => setShowShareList(false)}>Back</Button></div>
      <h3>Shared to organizations ({shares.length})</h3>
      {shares.length === 0 ? <Status>No shares.</Status> : <ul className="wk-list">{shares.map((item) => <li key={item.id}><span>{item.organization_name ?? item.organization_id}</span><span>{item.permission === 'editor' ? 'Editable' : 'Read-only'}</span><Button type="button" disabled={busy} onClick={() => void unshare(item)}>Remove</Button></li>)}</ul>}
    </> : <>
      <p className="wk-muted">Choose an organization and permission for this knowledge base.</p>
      <form className="wk-settings-editor" onSubmit={(event) => void share(event)}><label>Organization<select required value={organizationId} onChange={(event) => setOrganizationId(event.target.value)}><option value="">Select organization</option>{available.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}</select></label><label>Permission<select value={permission} onChange={(event) => setPermission(event.target.value as 'viewer' | 'editor')}><option value="viewer">Read-only</option><option value="editor">Editable</option></select></label><Button type="submit" loading={busy} disabled={!organizationId}>Confirm share</Button></form>
      {shares.length > 0 ? <Button type="button" onClick={() => setShowShareList(true)}>Shared to organizations ({shares.length})</Button> : null}
    </>}
  </Dialog>;
}
