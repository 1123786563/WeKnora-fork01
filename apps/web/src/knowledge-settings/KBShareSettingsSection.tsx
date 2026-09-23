import { useCallback, useEffect, useRef, useState } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { SpaceAvatar } from '../organizations/SpaceAvatar.tsx';
import { KbIcon } from '../knowledge-bases/kb-list-icons.tsx';
import './knowledge-settings-u.css';

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
    <div className="wk-kss-1" data-kb-share-settings="">
      <div className="wk-kss-2">
        <div className="wk-kss-3">
          <span className="wk-kss-4">{t('organization.share.sharedTo')}</span>
          <span data-share-count="" className="wk-kss-5">{filteredShares.length}</span>
        </div>
        <div className="wk-kss-3">
          {/* Vue share-hint-trigger-btn: an icon-only trigger carrying the
              sharing-notes popover (hintTitle + tip1/tip2). */}
          <span className="wk-kss-6">
            <button type="button" aria-label={t('knowledgeEditor.share.hintTitle')} title={t('knowledgeEditor.share.hintTitle')} aria-expanded={hintOpen} onClick={() => setHintOpen((current) => !current)}>{/* R490 A7 (Vue t-icon name="info-circle"): the hint trigger is an SVG glyph — the old literal "ⓘ" character leaked into innerText. */}<KbIcon name="info-circle" size={16} /></button>
            {hintOpen ? <div role="note" data-share-hint="" className="wk-kss-7">
              <strong className="wk-kss-4">{t('knowledgeEditor.share.hintTitle')}</strong>
              <span>{t('knowledgeEditor.share.tip1')}</span>
              <span>{t('knowledgeEditor.share.tip2')}</span>
            </div> : null}
          </span>
          <input
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder={t('organization.share.searchPlaceholder')}
            aria-label={t('organization.share.searchPlaceholder')}
            className="wk-kss-8"
          />
          {canShare ? (
            <button
              type="button"
              aria-label={t('knowledgeEditor.share.addShare')}
              title={t('knowledgeEditor.share.addShare')}
              aria-expanded={addOpen}
              data-share-add-trigger=""
              onClick={() => { setAddOpen((current) => !current); setSelectedOrgId(''); setSelectedPermission('viewer'); }}
              className="wk-kss-9"
            >{/* R490 A7 (Vue t-button #icon + t-icon "add"): the square add trigger is an SVG glyph — the old literal "+" character leaked into innerText. */}<KbIcon name="add" size={16} /></button>
          ) : null}
        </div>
      </div>
      {error ? <p role="alert" data-share-error="" className="wk-kss-10" style={{ color: '#b42318' }}>{error}</p> : null}
      {notice ? <p role="status" data-share-notice="" className="wk-kss-10" style={{ color: '#067647' }}>{notice}</p> : null}
      {canShare && addOpen ? (
        // Vue share-add-popup: dialog title, org select, permission select,
        // the permission tip and the 取消/共享 footer pair.
        <div data-share-add-popup="" className="wk-kss-11">
          <strong className="wk-kss-12">{t('organization.share.addShareDialogTitle')}</strong>
          <label className="wk-kss-13">
            {t('organization.share.selectOrg')}
            <select data-share-add-org="" value={selectedOrgId} aria-label={t('organization.share.selectOrg')} onChange={(event) => setSelectedOrgId(event.target.value)}>
              <option value="">{t('organization.share.selectOrg')}</option>
              {availableOrganizations.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}
            </select>
          </label>
          <label className="wk-kss-13">
            {t('organization.share.permission')}
            <select data-share-add-permission="" value={selectedPermission} aria-label={t('organization.share.permission')} onChange={(event) => setSelectedPermission(event.target.value === 'editor' ? 'editor' : 'viewer')}>
              <option value="viewer">{t('organization.share.permissionReadonly')}</option>
              <option value="editor">{t('organization.share.permissionEditable')}</option>
            </select>
          </label>
          <p className="wk-kss-14">{t('organization.share.permissionTip')}</p>
          <div className="wk-kss-15">
            <button type="button" data-share-add-cancel="" onClick={() => setAddOpen(false)}>{t('common.cancel')}</button>
            <button type="button" data-share-add-confirm="" disabled={!selectedOrgId || submitting} onClick={() => void share()} aria-busy={submitting}>{t('knowledgeEditor.share.addShare')}</button>
          </div>
        </div>
      ) : null}
      {loading && shares.length === 0 ? <p className="wk-kss-16" data-share-loading="">{t('organization.share.loading')}</p> : null}
      {!loading && filteredShares.length === 0 ? <p className="wk-kss-16" data-share-empty="">{query ? t('organization.share.emptySearch', { q: searchQuery.trim() }) : t('organization.share.noShares')}</p> : null}
      {filteredShares.length > 0 ? (
        <div className="wk-kss-17">
          <table className="wk-kss-18">
            <thead>
              <tr className="wk-kss-19">
                <th className="wk-kss-20">{t('organization.share.columns.space')}</th>
                <th className="wk-kss-20">{t('organization.share.columns.permission')}</th>
                <th className="wk-kss-20">{t('organization.share.columns.sharedAt')}</th>
                {canShare ? <th className="wk-kss-20">{t('organization.share.columns.operations')}</th> : null}
              </tr>
            </thead>
            <tbody>
              {filteredShares.map((item) => (
                <tr key={item.id} className="wk-kss-21">
                  <td className="wk-kss-20">
                    <span className="wk-kss-3">
                      <SpaceAvatar name={orgName(item)} avatar={item.avatar} size="small" />
                      <span className="wk-kss-22">
                        <span className="wk-kss-23">{orgName(item)}</span>
                        {typeof item.shared_by_username === 'string' && item.shared_by_username ? <small className="wk-kss-24">{t('organization.share.sharedFrom')} {item.shared_by_username}</small> : null}
                      </span>
                    </span>
                  </td>
                  <td className="wk-kss-20">
                    {canShare ? (
                      <select data-share-permission="" value={String(item.permission ?? 'viewer')} aria-label={t('organization.share.permission')} onChange={(event) => void updatePermission(item, event.target.value === 'editor' ? 'editor' : 'viewer')}>
                        <option value="viewer">{t('organization.share.permissionReadonly')}</option>
                        <option value="editor">{t('organization.share.permissionEditable')}</option>
                      </select>
                    ) : (
                      <span className={`wk-kss-27 ${item.permission === 'editor' || item.permission === 'admin' ? 'wk-kss-28' : 'wk-kss-29'}`}>{permissionLabel(String(item.permission ?? ''), t)}</span>
                    )}
                  </td>
                  <td className="wk-kss-25">{formatShareDate(item.created_at, locale)}</td>
                  {canShare ? (
                    <td className="wk-kss-20">
                      {pendingUnshare?.id === item.id ? (
                        // Vue t-popconfirm: the unshare confirm inline before
                        // the DELETE fires.
                        <span className="wk-kss-3" data-share-unshare-confirm-row="">
                          <span className="wk-kss-26">{t('knowledgeEditor.share.unshareConfirm', { name: orgName(item) })}</span>
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
