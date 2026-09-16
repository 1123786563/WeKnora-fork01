import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { Button, Dialog, Input, Select, Status } from '@weknora/ui';
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
  return <div className="relative" ref={rootRef}><button type="button" className="box-border flex w-full min-h-[2.45rem] cursor-pointer items-center justify-between gap-[0.55rem] rounded-control border border-line-control bg-surface px-[0.55rem] py-[0.35rem] text-left text-ink [font:inherit] [&>span:nth-child(2)]:flex-1" role="combobox" aria-haspopup="listbox" aria-expanded={open} aria-controls="wk-share-org-picker-list" onClick={() => setOpen((current) => !current)} onKeyDown={(event) => { if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && organizations.length > 0) { event.preventDefault(); setOpen(true); setActiveIndex((current) => (current + (event.key === 'ArrowDown' ? 1 : -1) + organizations.length) % organizations.length); } else if (event.key === 'Enter' && open) { event.preventDefault(); choose(activeIndex); } else if (event.key === 'Escape') setOpen(false); }}>{selected ? <><SpaceAvatar name={selected.name} avatar={(selected as Record<string, unknown>).avatar} size="small" /><span>{selected.name}</span></> : <span className="wk-muted text-muted">{placeholder}</span>}<span aria-hidden="true">⌄</span></button>{open ? <div id="wk-share-org-picker-list" className="absolute inset-x-0 top-[calc(100%_+_0.25rem)] z-30 grid max-h-[280px] overflow-y-auto rounded-[7px] border border-line bg-surface p-[0.25rem] shadow-[0_12px_30px_rgb(23_32_51_/_18%)]" role="listbox" aria-label={placeholder}>{organizations.map((organization, index) => { const row = organization as Record<string, unknown>; const role = row.is_owner === true ? t('organization.owner') : typeof row.my_role === 'string' ? t(`organization.role.${row.my_role}`) : ''; return <button type="button" role="option" aria-selected={organization.id === value} className={`flex w-full min-w-0 cursor-pointer items-center gap-[0.65rem] rounded-[7px] border-0 px-[0.65rem] py-[0.55rem] text-left ${index === activeIndex ? 'bg-accent-wash hover:bg-accent-wash' : organization.id === value ? 'bg-[#f1f4f9] hover:bg-[#f1f4f9]' : 'bg-transparent hover:bg-[#f1f4f9]'}`} key={organization.id} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}><SpaceAvatar name={organization.name} avatar={row.avatar} size="small" /><span className="grid min-w-0 flex-1 gap-[0.3rem]"><span className="flex items-center gap-2 [&>strong]:truncate"><strong>{organization.name}</strong>{role ? <span className="rounded-pill bg-[#eef1f6] px-[0.45rem] py-[0.15rem] text-[0.72rem] font-semibold whitespace-nowrap text-muted">{role}</span> : null}</span><span className="flex items-center gap-2 text-[0.75rem] font-normal text-muted [&_span]:inline-flex [&_span]:items-center [&_span]:gap-[0.2rem] [&_svg]:flex-none"><span><KbIcon name="user" size={13} />{orgValue(organization, 'member_count')}</span><span><KbIcon name="layers" size={13} />{orgValue(organization, 'share_count')}</span><span><KbIcon name="usergroup" size={13} />{orgValue(organization, 'agent_share_count')}</span></span></span></button>; })}</div> : null}</div>;
}

function PermissionRadio({ value, onChange, t }: { value: 'viewer' | 'editor'; onChange: (value: 'viewer' | 'editor') => void; t: (key: string) => string }) {
  return <div className="inline-flex w-fit overflow-hidden rounded-[3px] border border-line-neutral" role="radiogroup" aria-label={t('organization.share.permission')}>
    {(['viewer', 'editor'] as const).map((item) => <button key={item} type="button" role="radio" aria-checked={value === item} className={`min-h-8 cursor-pointer border-y-0 border-l-0 border-r border-line-neutral px-4 py-0 [font:inherit] last:border-r-0 ${value === item ? 'bg-accent text-white' : 'bg-surface text-[rgba(0,0,0,0.9)] hover:text-accent focus-visible:text-accent focus-visible:outline-none'}`} onClick={() => onChange(item)}>{item === 'viewer' ? t('organization.share.permissionReadonly') : t('organization.share.permissionEditable')}</button>)}
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
  async function share(event: FormEvent<HTMLFormElement>) { event.preventDefault(); if (!organizationId || busyRef.current) return; busyRef.current = true; setBusy(true); setError(null); setNotice(null); try { await client.identity.organizations.knowledgeBaseShares.create(knowledgeBaseId, { organization_id: organizationId, permission }); if (await load()) { setOrganizationId(''); setPermission('viewer'); setNotice(t('organization.share.shareSuccess')); onChanged?.(); } } catch (cause) { setError(cause instanceof Error ? cause.message : t('organization.share.shareFailed')); } finally { busyRef.current = false; setBusy(false); } }
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
  const content = <>
    {error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}{loading ? <Status>{t('organization.share.loading')}</Status> : null}
    {loading ? null : showShareList ? <>
      <div className="mb-3 flex items-center gap-2">{inline ? <><Input value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder={t('organization.share.searchPlaceholder')} aria-label={t('organization.share.searchPlaceholder')} /><Button type="button" onClick={() => setShowShareList(false)}>{t('knowledgeEditor.share.addShare')}</Button></> : null}<Button type="button" variant="text" onClick={() => setShowShareList(false)}><KbIcon name="chevron-left" size={14} />{t('common.back')}</Button></div>
      <h3>{t('organization.share.sharedTo')} ({filteredShares.length})</h3>
      {filteredShares.length === 0 ? <Status>{searchQuery.trim() ? t('organization.share.emptySearch', { q: searchQuery.trim() }) : t('organization.share.noShares')}</Status> : <ul className="m-0 grid list-none max-h-[280px] gap-[0.6rem] overflow-y-auto p-0">{filteredShares.map((item) => <li className="flex items-center justify-between gap-3 rounded-card border border-line-soft bg-canvas p-3 [transition:background_0.2s,border-color_0.2s] hover:border-[#d8e0ec] hover:bg-[#f1f4f9] max-[640px]:flex-col max-[640px]:items-start" key={item.id}><div className="flex min-w-0 items-center gap-2"><SpaceAvatar name={item.organization_name ?? item.organization_id ?? ''} avatar={(item as Record<string, unknown>).avatar} size="small" /><span className="truncate">{item.organization_name ?? item.organization_id}</span><span className={`rounded-pill px-2 py-0.5 text-xs ${item.permission === 'editor' ? 'bg-warning-wash text-warning-text' : 'bg-surface-muted text-muted'}`}>{item.permission === 'editor' ? t('organization.share.permissionEditable') : t('organization.share.permissionReadonly')}</span></div><div className="flex flex-none items-center gap-2 max-[640px]:self-end"><Button type="button" aria-label={t('organization.settings.editTitle')} title={t('organization.settings.editTitle')} onClick={() => goToOrganizationSettings(String(item.organization_id ?? ''))}><KbIcon name="settings" size={14} /></Button><Button type="button" disabled={busy} aria-label={t('organization.share.unshareAction')} title={t('organization.share.unshareAction')} onClick={() => void unshare(item)}><KbIcon name="close" size={14} /></Button></div></li>)}</ul>}
    </> : <>
      <form className="grid gap-[0.9rem] pt-[0.5rem]" onSubmit={(event) => void share(event)}><label className="grid gap-[0.4rem] font-semibold">{t('organization.share.selectOrg')}<OrganizationPicker organizations={available} value={organizationId} onChange={setOrganizationId} placeholder={t('organization.share.selectOrgPlaceholder')} t={t} /><Select className="absolute box-border h-px w-full min-h-[2.45rem] rounded-control border border-line-control bg-surface px-[0.65rem] py-[0.55rem] text-ink [font:inherit] opacity-0 pointer-events-none" required value={organizationId} onChange={(event) => setOrganizationId(event.target.value)} aria-hidden="true" tabIndex={-1}><option value="">{t('organization.share.selectOrgPlaceholder')}</option>{available.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}</Select></label><label className="grid gap-[0.4rem] font-semibold">{t('organization.share.permission')}<PermissionRadio value={permission} onChange={setPermission} t={t} /></label><div className="flex items-start gap-[0.45rem] rounded-control bg-[#f4f6fa] px-[0.75rem] py-[0.7rem] text-[0.8rem] font-normal leading-[1.5] text-muted"><KbIcon name="info-circle" size={14} /><span>{t('organization.share.permissionTip')}</span></div><div className="mt-[0.35rem] flex items-center gap-[0.6rem] border-t border-line-soft pt-4">{shares.length > 0 ? <Button type="button" onClick={() => setShowShareList(true)}>{t('organization.share.sharedTo')} ({shares.length})</Button> : null}<span className="flex-1" aria-hidden="true" /><Button type="button" onClick={onClose}>{t('common.cancel')}</Button><Button type="submit" loading={busy} disabled={!organizationId}>{t('common.confirm')}</Button></div></form>
    </>}
  </>;
  return inline ? <section className="grid gap-3" aria-label={t('organization.share.title')}><h3 className="m-0 text-[20px] font-semibold leading-7 text-ink">{t('organization.share.title')}</h3><p className="m-0 text-sm leading-[22px] text-muted">{t('knowledgeEditor.share.description')}</p>{content}</section> : <Dialog open={open} title={t('organization.share.title')} className="w-[520px]! max-w-[calc(100vw-2rem)]" onClose={onClose}>{content}</Dialog>;
}
