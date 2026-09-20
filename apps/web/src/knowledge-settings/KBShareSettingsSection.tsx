import { useCallback, useEffect, useRef, useState } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { SpaceAvatar } from '../organizations/SpaceAvatar.tsx';
import { KbIcon } from '../knowledge-bases/kb-list-icons.tsx';

// R485 (Vue KBShareSettings): the shared-to management list embedded in the
// KB settings drawer — a count badge next to the "已共享到" header, a search
// box, an add-share popup (select shared space + permission + tip) and a table
// whose permission column is an inline select while the viewer can manage, with
// an unshare confirm step before the DELETE fires.

type Share = Record<string, unknown> & { id: string };

// Vue KBShareSettings availableOrganizations: organizations the current user
// can share into (owner/admin/editor) that are not already shared.
function canShareInto(org: Organization): boolean {
  const row = org as Record<string, unknown>;
  return row.is_owner === true || row.my_role === 'admin' || row.my_role === 'editor';
}

function permissionLabel(permission: string, t: (key: string) => string): string {
  return permission === 'editor' || permission === 'admin'
    ? t('organization.share.permissionEditable')
    : t('organization.share.permissionReadonly');
}

// Vue formatShareDate: invalid dates echo the raw string, missing dates '—'.
function formatShareDate(dateStr: unknown, locale: string): string {
  if (typeof dateStr !== 'string' || dateStr === '') return '—';
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;
  return date.toLocaleDateString(locale, { year: 'numeric', month: '2-digit', day: '2-digit' });
}

type Props = { client: WeKnoraClient; knowledgeBaseId: string; canShare?: boolean };

export function KBShareSettingsSection({ client, knowledgeBaseId, canShare = false }: Props) {
  const locale = useAppLocale();
  const t = createTranslator(locale);
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [shares, setShares] = useState<Share[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [hintOpen, setHintOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [selectedOrgId, setSelectedOrgId] = useState('');
  const [selectedPermission, setSelectedPermission] = useState<'viewer' | 'editor'>('viewer');
  const [pendingUnshare, setPendingUnshare] = useState<Share | null>(null);
  const generationRef = useRef(0);

  const load = useCallback(async () => {
    const generation = ++generationRef.current;
    setLoading(true);
    try {
      const [orgs, currentShares] = await Promise.all([
        client.identity.organizations.list(),
        client.identity.organizations.knowledgeBaseShares.list(knowledgeBaseId),
      ]);
      if (generation !== generationRef.current) return;
      // Defensive page unwrap: tolerate both the paged contract ({items}) and
      // a bare array so a degraded listing cannot crash the drawer.
      setOrganizations((Array.isArray(orgs) ? orgs : orgs.items ?? []) as Organization[]);
      setShares((Array.isArray(currentShares) ? currentShares : currentShares.items ?? []) as Share[]);
    } catch (cause) {
      if (generation !== generationRef.current) return;
      setError(cause instanceof Error ? cause.message : t('organization.share.loading'));
    } finally {
      if (generation === generationRef.current) setLoading(false);
    }
  }, [client, knowledgeBaseId, t]);

  // t is a fresh closure every render (createTranslator), so the effect must
  // key on the identity inputs only — depending on `load` here would refetch
  // every render.
  useEffect(() => { void load(); }, [client, knowledgeBaseId]); // eslint-disable-line react-hooks/exhaustive-deps

  const sharedIds = new Set(shares.map((share) => String(share.organization_id ?? '')));
  const availableOrganizations = organizations.filter((org) => !sharedIds.has(org.id) && canShareInto(org));
  const query = searchQuery.trim().toLowerCase();
  const filteredShares = query
    ? shares.filter((share) => [String(share.organization_name ?? ''), String(share.shared_by_username ?? ''), permissionLabel(String(share.permission ?? ''), t)]
      .filter(Boolean).join(' ').toLowerCase().includes(query))
    : shares;

  const share = async () => {
    if (!selectedOrgId || submitting) return;
    setSubmitting(true); setError(null); setNotice(null);
    try {
      await client.identity.organizations.knowledgeBaseShares.create(knowledgeBaseId, { organization_id: selectedOrgId, permission: selectedPermission });
      setNotice(t('organization.share.shareSuccess'));
      setSelectedOrgId('');
      setSelectedPermission('viewer');
      setAddOpen(false);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('organization.share.shareFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  const updatePermission = async (target: Share, permission: 'viewer' | 'editor') => {
    if (String(target.permission ?? '') === permission) return;
    setError(null); setNotice(null);
    try {
      await client.identity.organizations.knowledgeBaseShares.updatePermission(knowledgeBaseId, target.id, permission);
      setNotice(t('organization.roleUpdated'));
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('organization.roleUpdateFailed'));
    }
  };

  const unshare = async (target: Share) => {
    setPendingUnshare(null);
    setError(null); setNotice(null);
    try {
      await client.identity.organizations.knowledgeBaseShares.remove(knowledgeBaseId, target.id);
      setNotice(t('organization.share.unshareSuccess'));
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('organization.share.unshareFailed'));
    }
  };

  const orgName = (share: Share): string => {
    const name = String(share.organization_name ?? '');
    if (name) return name;
    const id = String(share.organization_id ?? '');
    return organizations.find((org) => org.id === id)?.name ?? id;
  };

  return (
    <div className="grid gap-3" data-kb-share-settings="">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="font-semibold">{t('organization.share.sharedTo')}</span>
          <span data-share-count="" className="inline-flex min-w-[1.25rem] items-center justify-center rounded-pill bg-surface-muted px-[0.4rem] py-[0.05rem] text-xs text-muted">{filteredShares.length}</span>
        </div>
        <div className="flex items-center gap-2">
          {/* Vue share-hint-trigger-btn: an icon-only trigger carrying the
              sharing-notes popover (hintTitle + tip1/tip2). */}
          <span className="relative inline-flex">
            <button type="button" aria-label={t('knowledgeEditor.share.hintTitle')} title={t('knowledgeEditor.share.hintTitle')} aria-expanded={hintOpen} onClick={() => setHintOpen((current) => !current)}>{/* R490 A7 (Vue t-icon name="info-circle"): the hint trigger is an SVG glyph — the old literal "ⓘ" character leaked into innerText. */}<KbIcon name="info-circle" size={16} /></button>
            {hintOpen ? <div role="note" data-share-hint="" className="absolute right-0 top-[calc(100%+4px)] z-20 grid max-w-[380px] gap-1 rounded-control border border-line bg-surface p-3 text-xs shadow-[0_8px_20px_rgb(16_24_40/14%)]">
              <strong className="font-semibold">{t('knowledgeEditor.share.hintTitle')}</strong>
              <span>{t('knowledgeEditor.share.tip1')}</span>
              <span>{t('knowledgeEditor.share.tip2')}</span>
            </div> : null}
          </span>
          <input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder={t('organization.share.searchPlaceholder')}
            aria-label={t('organization.share.searchPlaceholder')}
            className="box-border min-h-[2rem] rounded-control border border-line-control bg-surface px-[0.55rem] py-[0.3rem] [font:inherit]"
          />
          {canShare ? (
            <button
              type="button"
              aria-label={t('knowledgeEditor.share.addShare')}
              title={t('knowledgeEditor.share.addShare')}
              aria-expanded={addOpen}
              data-share-add-trigger=""
              onClick={() => { setAddOpen((current) => !current); setSelectedOrgId(''); setSelectedPermission('viewer'); }}
              className="inline-flex h-7 w-7 items-center justify-center rounded-control border border-line-control text-sm"
            >{/* R490 A7 (Vue t-button #icon + t-icon "add"): the square add trigger is an SVG glyph — the old literal "+" character leaked into innerText. */}<KbIcon name="add" size={16} /></button>
          ) : null}
        </div>
      </div>
      {error ? <p role="alert" data-share-error="" className="m-0 text-sm" style={{ color: '#b42318' }}>{error}</p> : null}
      {notice ? <p role="status" data-share-notice="" className="m-0 text-sm" style={{ color: '#067647' }}>{notice}</p> : null}
      {canShare && addOpen ? (
        // Vue share-add-popup: dialog title, org select, permission select,
        // the permission tip and the 取消/共享 footer pair.
        <div data-share-add-popup="" className="grid gap-3 rounded-card border border-line bg-surface p-3">
          <strong className="text-sm font-semibold">{t('organization.share.addShareDialogTitle')}</strong>
          <label className="grid gap-[0.35rem] text-sm">
            {t('organization.share.selectOrg')}
            <select data-share-add-org="" value={selectedOrgId} aria-label={t('organization.share.selectOrg')} onChange={(event) => setSelectedOrgId(event.target.value)}>
              <option value="">{t('organization.share.selectOrg')}</option>
              {availableOrganizations.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}
            </select>
          </label>
          <label className="grid gap-[0.35rem] text-sm">
            {t('organization.share.permission')}
            <select data-share-add-permission="" value={selectedPermission} aria-label={t('organization.share.permission')} onChange={(event) => setSelectedPermission(event.target.value === 'editor' ? 'editor' : 'viewer')}>
              <option value="viewer">{t('organization.share.permissionReadonly')}</option>
              <option value="editor">{t('organization.share.permissionEditable')}</option>
            </select>
          </label>
          <p className="m-0 text-xs text-muted">{t('organization.share.permissionTip')}</p>
          <div className="flex items-center justify-end gap-2">
            <button type="button" data-share-add-cancel="" onClick={() => setAddOpen(false)}>{t('common.cancel')}</button>
            <button type="button" data-share-add-confirm="" disabled={!selectedOrgId || submitting} onClick={() => void share()} aria-busy={submitting}>{t('knowledgeEditor.share.addShare')}</button>
          </div>
        </div>
      ) : null}
      {loading && shares.length === 0 ? <p className="m-0 text-sm text-muted" data-share-loading="">{t('organization.share.loading')}</p> : null}
      {!loading && filteredShares.length === 0 ? <p className="m-0 text-sm text-muted" data-share-empty="">{query ? t('organization.share.emptySearch', { q: searchQuery.trim() }) : t('organization.share.noShares')}</p> : null}
      {filteredShares.length > 0 ? (
        <div className="overflow-auto rounded-card border border-line-soft">
          <table className="w-full min-w-[560px] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line-soft bg-surface-muted text-left text-xs text-muted">
                <th className="px-3 py-2">{t('organization.share.columns.space')}</th>
                <th className="px-3 py-2">{t('organization.share.columns.permission')}</th>
                <th className="px-3 py-2">{t('organization.share.columns.sharedAt')}</th>
                {canShare ? <th className="px-3 py-2">{t('organization.share.columns.operations')}</th> : null}
              </tr>
            </thead>
            <tbody>
              {filteredShares.map((item) => (
                <tr key={item.id} className="border-b border-line-soft last:border-0">
                  <td className="px-3 py-2">
                    <span className="flex items-center gap-2">
                      <SpaceAvatar name={orgName(item)} avatar={item.avatar} size="small" />
                      <span className="grid">
                        <span className="truncate">{orgName(item)}</span>
                        {typeof item.shared_by_username === 'string' && item.shared_by_username ? <small className="text-muted">{t('organization.share.sharedFrom')} {item.shared_by_username}</small> : null}
                      </span>
                    </span>
                  </td>
                  <td className="px-3 py-2">
                    {canShare ? (
                      <select data-share-permission="" value={String(item.permission ?? 'viewer')} aria-label={t('organization.share.permission')} onChange={(event) => void updatePermission(item, event.target.value === 'editor' ? 'editor' : 'viewer')}>
                        <option value="viewer">{t('organization.share.permissionReadonly')}</option>
                        <option value="editor">{t('organization.share.permissionEditable')}</option>
                      </select>
                    ) : (
                      <span className={`rounded-pill px-2 py-0.5 text-xs ${item.permission === 'editor' || item.permission === 'admin' ? 'bg-warning-wash text-warning-text' : 'bg-surface-muted text-muted'}`}>{permissionLabel(String(item.permission ?? ''), t)}</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-muted">{formatShareDate(item.created_at, locale)}</td>
                  {canShare ? (
                    <td className="px-3 py-2">
                      {pendingUnshare?.id === item.id ? (
                        // Vue t-popconfirm: the unshare confirm inline before
                        // the DELETE fires.
                        <span className="flex items-center gap-2" data-share-unshare-confirm-row="">
                          <span className="text-xs">{t('knowledgeEditor.share.unshareConfirm', { name: orgName(item) })}</span>
                          <button type="button" data-share-unshare-confirm="" onClick={() => void unshare(item)}>{t('common.confirm')}</button>
                          <button type="button" data-share-unshare-cancel="" onClick={() => setPendingUnshare(null)}>{t('common.cancel')}</button>
                        </span>
                      ) : (
                        <button type="button" aria-label={t('organization.share.unshareAction')} title={t('organization.share.unshareAction')} data-share-unshare-trigger="" onClick={() => setPendingUnshare(item)}>✕</button>
                      )}
                    </td>
                  ) : null}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}
