import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';

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
  const loadGeneration = useRef(0);
  const t = createTranslator(useAppLocale());
  async function load(): Promise<boolean> {
    const generation = ++loadGeneration.current;
    setLoading(true); setError(null);
    try {
      const [orgs, currentShares] = await Promise.all([client.identity.organizations.list(), client.identity.organizations.knowledgeBaseShares.list(knowledgeBaseId)]);
      if (generation !== loadGeneration.current) return false;
      setOrganizations(orgs.items.filter(canShare)); setShares(currentShares.items as Share[]);
      return true;
    } catch (cause) {
      if (generation !== loadGeneration.current) return false;
      setError(cause instanceof Error ? cause.message : t('organization.share.shareFailed')); return false;
    } finally { if (generation === loadGeneration.current) setLoading(false); }
  }
  useEffect(() => { if (open) { setOrganizationId(''); setNotice(null); setShowShareList(false); void load(); } }, [open, knowledgeBaseId]);
  async function share(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!organizationId || busyRef.current) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.create(knowledgeBaseId, { organization_id: organizationId, permission }); if (await load()) { setNotice(t('organization.share.shareSuccess')); onChanged?.(); } } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.share.shareFailed')); } finally { busyRef.current = false; setBusy(false); } }
  async function unshare(item: Share) { const name = item.organization_name ?? item.organization_id ?? 'organization'; if (busyRef.current || !window.confirm(t('organization.settings.removeShareConfirm', { name }))) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.remove(knowledgeBaseId, item.id); if (await load()) { setNotice(t('organization.share.unshareSuccess')); onChanged?.(); } } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.share.unshareFailed')); } finally { busyRef.current = false; setBusy(false); } }
  const sharedIds = new Set(shares.map((item) => String(item.organization_id ?? '')));
  const available = organizations.filter((item) => !sharedIds.has(item.id));
  return <Dialog open={open} title={`${t('organization.share.title')} · ${knowledgeBaseName}`} onClose={onClose}>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{loading ? <Status>{t('organization.share.loading')}</Status> : null}
    {loading ? null : showShareList ? <>
      <div className="wk-list-actions"><Button type="button" onClick={() => setShowShareList(false)}>{t('common.back')}</Button></div>
      <h3>{t('organization.share.sharedTo')} ({shares.length})</h3>
      {shares.length === 0 ? <Status>{t('organization.share.noShares')}</Status> : <ul className="wk-list">{shares.map((item) => <li key={item.id}><span>{item.organization_name ?? item.organization_id}</span><span>{item.permission === 'editor' ? t('organization.share.permissionEditable') : t('organization.share.permissionReadonly')}</span><Button type="button" disabled={busy} onClick={() => void unshare(item)}>{t('organization.share.unshareAction')}</Button></li>)}</ul>}
    </> : <>
      <p className="wk-muted">{t('organization.share.permissionTip')}</p>
      <form className="wk-settings-editor" onSubmit={(event) => void share(event)}><label>{t('organization.share.selectOrg')}<select required value={organizationId} onChange={(event) => setOrganizationId(event.target.value)}><option value="">{t('organization.share.selectOrgPlaceholder')}</option>{available.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}</select></label><label>{t('organization.share.permission')}<select value={permission} onChange={(event) => setPermission(event.target.value as 'viewer' | 'editor')}><option value="viewer">{t('organization.share.permissionReadonly')}</option><option value="editor">{t('organization.share.permissionEditable')}</option></select></label><Button type="submit" loading={busy} disabled={!organizationId}>{t('organization.share.title')}</Button></form>
      {shares.length > 0 ? <Button type="button" onClick={() => setShowShareList(true)}>{t('organization.share.sharedTo')} ({shares.length})</Button> : null}
    </>}
  </Dialog>;
}
