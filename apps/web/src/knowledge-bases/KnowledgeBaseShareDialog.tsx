import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Status } from '@weknora/ui';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { KbIcon } from './kb-list-icons.tsx';

type Share = Record<string, unknown> & { id: string; organization_id?: string; organization_name?: string; permission?: string };
type Props = { client: WeKnoraClient; knowledgeBaseId: string; knowledgeBaseName: string; open: boolean; onClose: () => void; onChanged?: () => void };
function canShare(org: Organization): boolean { const row = org as Record<string, unknown>; return row.is_owner === true || row.my_role === 'admin' || row.my_role === 'editor'; }
function orgValue(org: Organization, key: string): number { const value = (org as Record<string, unknown>)[key]; return typeof value === 'number' ? value : 0; }
function initials(name: string): string { return name.trim().split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase() || '?'; }

function OrganizationPicker({ organizations, value, onChange, placeholder, t }: { organizations: Organization[]; value: string; onChange: (value: string) => void; placeholder: string; t: (key: string, vars?: Record<string, string | number>) => string }) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') { setOpen(false); } };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('pointerdown', onPointerDown); document.removeEventListener('keydown', onKeyDown); };
  }, [open]);
  const selected = organizations.find((organization) => organization.id === value);
  return <div className="wk-share-org-picker" ref={rootRef}><button type="button" className="wk-share-org-picker-trigger" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen((current) => !current)}>{selected ? <><span className="wk-share-org-avatar" aria-hidden="true">{initials(selected.name)}</span><span>{selected.name}</span></> : <span className="wk-muted">{placeholder}</span>}<span aria-hidden="true">⌄</span></button>{open ? <div className="wk-share-org-picker-menu" role="listbox" aria-label={placeholder}>{organizations.map((organization) => { const row = organization as Record<string, unknown>; const role = row.is_owner === true ? t('organization.owner') : typeof row.my_role === 'string' ? t(`organization.role.${row.my_role}`) : ''; return <button type="button" role="option" aria-selected={organization.id === value} className={`wk-share-org-option${organization.id === value ? ' is-selected' : ''}`} key={organization.id} onClick={() => { onChange(organization.id); setOpen(false); }}><span className="wk-share-org-avatar" aria-hidden="true">{initials(organization.name)}</span><span className="wk-share-org-body"><span className="wk-share-org-header"><strong>{organization.name}</strong>{role ? <span className="wk-share-role">{role}</span> : null}</span><span className="wk-share-org-meta"><span><KbIcon name="user" size={13} />{orgValue(organization, 'member_count')}</span><span><KbIcon name="layers" size={13} />{orgValue(organization, 'share_count')}</span><span><KbIcon name="usergroup" size={13} />{orgValue(organization, 'agent_share_count')}</span></span></span></button>; })}</div> : null}</div>;
}

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
  function goToOrganizationSettings(organizationId: string) {
    const url = new URL('/platform/organizations', window.location.origin);
    url.searchParams.set('orgId', organizationId);
    window.history.pushState({}, '', `${url.pathname}${url.search}`);
    onClose();
  }
  const sharedIds = new Set(shares.map((item) => String(item.organization_id ?? '')));
  const available = organizations.filter((item) => !sharedIds.has(item.id));
  return <Dialog open={open} title={`${t('organization.share.title')} · ${knowledgeBaseName}`} onClose={onClose}>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{loading ? <Status>{t('organization.share.loading')}</Status> : null}
    {loading ? null : showShareList ? <>
      <div className="wk-list-actions"><Button type="button" onClick={() => setShowShareList(false)}>{t('common.back')}</Button></div>
      <h3>{t('organization.share.sharedTo')} ({shares.length})</h3>
      {shares.length === 0 ? <Status>{t('organization.share.noShares')}</Status> : <ul className="wk-share-items">{shares.map((item) => <li className="wk-share-item" key={item.id}><div className="wk-share-item-info"><span className="wk-share-item-avatar" aria-hidden="true">{initials(item.organization_name ?? item.organization_id ?? '')}</span><span className="wk-share-org-name">{item.organization_name ?? item.organization_id}</span><span className={`wk-share-permission wk-share-permission--${item.permission === 'editor' ? 'editor' : 'viewer'}`}>{item.permission === 'editor' ? t('organization.share.permissionEditable') : t('organization.share.permissionReadonly')}</span></div><div className="wk-share-item-actions"><Button type="button" aria-label={t('organization.settings.editTitle')} title={t('organization.settings.editTitle')} onClick={() => goToOrganizationSettings(String(item.organization_id ?? ''))}><KbIcon name="settings" size={14} /></Button><Button type="button" disabled={busy} aria-label={t('organization.share.unshareAction')} onClick={() => void unshare(item)}>{t('organization.share.unshareAction')}</Button></div></li>)}</ul>}
    </> : <>
      <form className="wk-share-form" onSubmit={(event) => void share(event)}><label>{t('organization.share.selectOrg')}<OrganizationPicker organizations={available} value={organizationId} onChange={setOrganizationId} placeholder={t('organization.share.selectOrgPlaceholder')} t={t} /><select className="wk-share-org-native-select" required value={organizationId} onChange={(event) => setOrganizationId(event.target.value)} aria-hidden="true" tabIndex={-1}><option value="">{t('organization.share.selectOrgPlaceholder')}</option>{available.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}</select></label><label>{t('organization.share.permission')}<select value={permission} onChange={(event) => setPermission(event.target.value as 'viewer' | 'editor')}><option value="viewer">{t('organization.share.permissionReadonly')}</option><option value="editor">{t('organization.share.permissionEditable')}</option></select></label><div className="wk-share-permission-tip"><KbIcon name="info-circle" size={14} /><span>{t('organization.share.permissionTip')}</span></div><div className="wk-share-form-actions"><Button type="button" onClick={onClose}>{t('common.cancel')}</Button><Button type="submit" loading={busy} disabled={!organizationId}>{t('common.confirm')}</Button></div></form>
      {shares.length > 0 ? <Button type="button" onClick={() => setShowShareList(true)}>{t('organization.share.sharedTo')} ({shares.length})</Button> : null}
    </>}
  </Dialog>;
}
