import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
// S6 抽屉收编：@weknora/ui 离栈（T15 硬前置）。隐藏 required select 保留原生
// <select>（tdesign Select 无原生表单校验语义，该节点仅为 required 兜底）。
import { Button as TButton, Dialog as TDialog, Input as TInput } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { KbIcon } from './kb-list-icons.tsx';
import { SpaceAvatar } from '../organizations/SpaceAvatar.tsx';

type Share = Record<string, unknown> & { id: string; organization_id?: string; organization_name?: string; permission?: string };
type Props = { client: WeKnoraClient; knowledgeBaseId: string; knowledgeBaseName: string; open: boolean; onClose: () => void; onChanged?: () => void; inline?: boolean };
function canShare(org: Organization): boolean { const row = org as Record<string, unknown>; return row.is_owner === true || row.my_role === 'admin' || row.my_role === 'editor'; }
function orgValue(org: Organization, key: string): number { const value = (org as Record<string, unknown>)[key]; return typeof value === 'number' ? value : 0; }
function initials(name: string): string { return name.trim().split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase() || '?'; }

function OrganizationPicker({ organizations, value, onChange, placeholder, t }: { organizations: Organization[]; value: string; onChange: (value: string) => void; placeholder: string; t: (key: string, vars?: Record<string, string | number>) => string }) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const rootRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') { setOpen(false); } };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => { document.removeEventListener('pointerdown', onPointerDown); document.removeEventListener('keydown', onKeyDown); };
  }, [open]);
  useEffect(() => {
    const index = organizations.findIndex((organization) => organization.id === value);
    setActiveIndex(index >= 0 ? index : 0);
  }, [organizations, value]);
  const selected = organizations.find((organization) => organization.id === value);
  const choose = (index: number) => { const organization = organizations[index]; if (!organization) return; onChange(organization.id); setActiveIndex(index); setOpen(false); };
  return <div className="wk-kb-share-picker" ref={rootRef}><button type="button" className="wk-kb-share-picker-trigger" role="combobox" aria-haspopup="listbox" aria-expanded={open} aria-controls="wk-share-org-picker-list" onClick={() => setOpen((current) => !current)} onKeyDown={(event) => { if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && organizations.length > 0) { event.preventDefault(); setOpen(true); setActiveIndex((current) => (current + (event.key === 'ArrowDown' ? 1 : -1) + organizations.length) % organizations.length); } else if (event.key === 'Enter' && open) { event.preventDefault(); choose(activeIndex); } else if (event.key === 'Escape') setOpen(false); }}>{selected ? <><SpaceAvatar name={selected.name} avatar={(selected as Record<string, unknown>).avatar} size="small" /><span>{selected.name}</span></> : <span className="wk-kb-share-picker-placeholder">{placeholder}</span>}<span aria-hidden="true">⌄</span></button>{open ? <div id="wk-share-org-picker-list" className="wk-kb-share-picker-list" role="listbox" aria-label={placeholder}>{organizations.map((organization, index) => { const row = organization as Record<string, unknown>; const role = row.is_owner === true ? t('organization.owner') : typeof row.my_role === 'string' ? t(`organization.role.${row.my_role}`) : ''; return <button type="button" role="option" aria-selected={organization.id === value} className={`wk-kb-share-picker-option${index === activeIndex ? ' is-active' : ''}${organization.id === value ? ' is-selected' : ''}`} key={organization.id} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}><SpaceAvatar name={organization.name} avatar={row.avatar} size="small" /><span className="wk-kb-share-option-main"><span className="wk-kb-share-option-name"><strong>{organization.name}</strong>{role ? <span className="wk-kb-share-option-role">{role}</span> : null}</span><span className="wk-kb-share-option-stats"><span><KbIcon name="user" size={13} />{orgValue(organization, 'member_count')}</span><span><KbIcon name="layers" size={13} />{orgValue(organization, 'share_count')}</span><span><KbIcon name="usergroup" size={13} />{orgValue(organization, 'agent_share_count')}</span></span></span></button>; })}</div> : null}</div>;
}

function PermissionRadio({ value, onChange, t }: { value: 'viewer' | 'editor'; onChange: (value: 'viewer' | 'editor') => void; t: (key: string) => string }) {
  return <div className="wk-kb-share-perm" role="radiogroup" aria-label={t('organization.share.permission')}>
    {(['viewer', 'editor'] as const).map((item) => <button key={item} type="button" role="radio" aria-checked={value === item} className={`wk-kb-share-perm-btn${value === item ? ' is-selected' : ''}`} onClick={() => onChange(item)}>{item === 'viewer' ? t('organization.share.permissionReadonly') : t('organization.share.permissionEditable')}</button>)}
  </div>;
}

export function KnowledgeBaseShareDialog({ client, knowledgeBaseId, knowledgeBaseName, open, onClose, onChanged, inline = false }: Props) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [shares, setShares] = useState<Share[]>([]);
  const [organizationId, setOrganizationId] = useState('');
  const [permission, setPermission] = useState<'viewer' | 'editor'>('viewer');
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [showShareList, setShowShareList] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
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
  useEffect(() => { if (open) { setOrganizationId(''); setPermission('viewer'); setNotice(null); setSearchQuery(''); setShowShareList(inline); void load(); } }, [open, knowledgeBaseId, inline]);
  // R462: the event is optional because the inline branch confirms through a
  // plain button onClick (Vue KBShareSettings has no <form> at all); the dialog
  // branch still submits through the form onSubmit.
  async function share(event?: FormEvent<HTMLFormElement>) { event?.preventDefault(); if (!organizationId || busyRef.current) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.create(knowledgeBaseId, { organization_id: organizationId, permission }); if (await load()) { setOrganizationId(''); setPermission('viewer'); setNotice(t('organization.share.shareSuccess')); onChanged?.(); } } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.share.shareFailed')); } finally { busyRef.current = false; setBusy(false); } }
  async function unshare(item: Share) { if (busyRef.current) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.remove(knowledgeBaseId, item.id); if (await load()) { setNotice(t('organization.share.unshareSuccess')); onChanged?.(); } } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.share.unshareFailed')); } finally { busyRef.current = false; setBusy(false); } }
  function goToOrganizationSettings(organizationId: string) {
    const url = new URL('/platform/organizations', window.location.origin);
    url.searchParams.set('orgId', organizationId);
    window.history.pushState({}, '', `${url.pathname}${url.search}`);
    onClose();
  }
  const sharedIds = new Set(shares.map((item) => String(item.organization_id ?? '')));
  const available = organizations.filter((item) => !sharedIds.has(item.id));
  const filteredShares = shares.filter((item) => !searchQuery.trim() || `${item.organization_name ?? ''} ${item.organization_id ?? ''} ${item.permission ?? ''}`.toLowerCase().includes(searchQuery.trim().toLowerCase()));
  // R462: inline mounts live inside the editor save <form> (App.tsx share
  // section), so a nested <form> here breaks HTML parsing/hydration. Vue
  // KBShareSettings — the inline counterpart — renders no form at all and
  // confirms via a t-button @click with :disabled/:loading guards, so the
  // inline branch wraps the fields in a plain div and confirms through
  // onClick. The dialog branch keeps its own <form> submit (Vue
  // ShareKnowledgeBaseDialog layout) including the hidden required select.
  const shareFormFields = <>
    <label className="wk-kb-share-label">{t('organization.share.selectOrg')}<OrganizationPicker organizations={available} value={organizationId} onChange={setOrganizationId} placeholder={t('organization.share.selectOrgPlaceholder')} t={t} />{inline ? null : <select className="wk-kb-share-hidden-select" required value={organizationId} onChange={(event) => setOrganizationId(event.target.value)} aria-hidden="true" tabIndex={-1}><option value="">{t('organization.share.selectOrgPlaceholder')}</option>{available.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}</select>}</label>
    <label className="wk-kb-share-label">{t('organization.share.permission')}<PermissionRadio value={permission} onChange={setPermission} t={t} /></label>
    <div className="wk-kb-share-tip"><KbIcon name="info-circle" size={14} /><span>{t('organization.share.permissionTip')}</span></div>
  </>;
  const shareFormActions = <div className="wk-kb-share-actions">{shares.length > 0 ? <TButton type="button" onClick={() => setShowShareList(true)}>{t('organization.share.sharedTo')} ({shares.length})</TButton> : null}<span className="wk-kb-share-spacer" aria-hidden="true" /><TButton type="button" onClick={onClose}>{t('common.cancel')}</TButton>{inline ? <TButton type="button" loading={busy} disabled={!organizationId} onClick={() => void share()}>{t('common.confirm')}</TButton> : <TButton type="submit" loading={busy} disabled={!organizationId}>{t('common.confirm')}</TButton>}</div>;
  const shareFormView = inline
    ? <div className="wk-kb-share-form">{shareFormFields}{shareFormActions}</div>
    : <form className="wk-kb-share-form" onSubmit={(event) => void share(event)}>{shareFormFields}{shareFormActions}</form>;
  const content = <>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{loading ? <Status>{t('organization.share.loading')}</Status> : null}
    {loading ? null : showShareList ? <>
      <div className="wk-kb-share-list-head">{inline ? <><TInput value={searchQuery} onChange={(value) => setSearchQuery(String(value))} placeholder={t('organization.share.searchPlaceholder')} aria-label={t('organization.share.searchPlaceholder')} /><TButton type="button" onClick={() => setShowShareList(false)}>{t('knowledgeEditor.share.addShare')}</TButton></> : null}<TButton type="button" variant="text" onClick={() => setShowShareList(false)}><KbIcon name="chevron-left" size={14} />{t('common.back')}</TButton></div>
      <h3>{t('organization.share.sharedTo')} ({filteredShares.length})</h3>
      {filteredShares.length === 0 ? <Status>{searchQuery.trim() ? t('organization.share.emptySearch', { q: searchQuery.trim() }) : t('organization.share.noShares')}</Status> : <ul className="wk-kb-share-list">{filteredShares.map((item) => <li className="wk-kb-share-item" key={item.id}><div className="wk-kb-share-item-main"><SpaceAvatar name={item.organization_name ?? item.organization_id ?? ''} avatar={(item as Record<string, unknown>).avatar} size="small" /><span className="wk-kb-share-item-name">{item.organization_name ?? item.organization_id}</span><span className={`wk-kb-share-item-pill${item.permission === 'editor' ? ' is-editor' : ''}`}>{item.permission === 'editor' ? t('organization.share.permissionEditable') : t('organization.share.permissionReadonly')}</span></div><div className="wk-kb-share-item-actions"><TButton type="button" aria-label={t('organization.settings.editTitle')} title={t('organization.settings.editTitle')} onClick={() => goToOrganizationSettings(String(item.organization_id ?? ''))}><KbIcon name="settings" size={14} /></TButton><TButton type="button" disabled={busy} aria-label={t('organization.share.unshareAction')} title={t('organization.share.unshareAction')} onClick={() => void unshare(item)}><KbIcon name="close" size={14} /></TButton></div></li>)}</ul>}
    </> : shareFormView}
  </>;
  return inline ? <section className="wk-kb-share-inline" aria-label={t('organization.share.title')}><h3>{t('organization.share.title')}</h3><p>{t('knowledgeEditor.share.description')}</p>{content}</section> : <TDialog footer={false} visible={open} header={t('organization.share.title')} dialogClassName="wk-kb-share-dialog" onClose={onClose}>{content}</TDialog>;
}
