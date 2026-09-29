import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { Organization, WeKnoraClient } from '@weknora/api-client';
import { Button, Empty, Input, MessagePlugin, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { SpaceAvatar } from '../organizations/SpaceAvatar.tsx';
import type { Translate } from './agent-editor.ts';

/**
 * React port of frontend/src/components/AgentShareSettings.vue (Vue baseline):
 * the「共享管理」section of the agent editor — share the agent to shared
 * spaces (viewer permission, Vue handleShare hardcodes 'viewer'), list the
 * spaces it has been shared to (with the sharing user + read-only tag) and
 * unshare after an inline confirm.
 *
 * DOM 类名按 Vue template 1:1 复刻（share-to-space-panel / share-panel-* /
 * share-space-cell …，样式平移在 agents.td.css，源
 * frontend/src/components/share-to-space-panel.less）。与 KBShareSettingsSection
 * 相同的移植取舍：Vue 的 body 弹层（t-popup 添加共享 / t-popconfirm 取消
 * 确认 / t-popup 提示气泡）在此渲染为编辑器内的内联面板。
 */
type Share = Record<string, unknown> & { id: string };

// Vue AgentShareSettings availableOrganizations: organizations the current
// user can share into (owner/admin/editor) that are not already shared.
function canShareInto(org: Organization): boolean {
  const row = org as Record<string, unknown>;
  return row.is_owner === true || row.my_role === 'admin' || row.my_role === 'editor';
}

// Vue formatShareDate: invalid dates echo the raw string, missing dates '—'.
function formatShareDate(dateStr: unknown, locale: string): string {
  if (typeof dateStr !== 'string' || dateStr === '') return '—';
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;
  return date.toLocaleDateString(locale, { year: 'numeric', month: '2-digit', day: '2-digit' });
}

export interface AgentShareSettingsProps {
  agentId: string;
  /** Vue passes the whole editor agent so the hint popover can check `agent?.config`. */
  agent?: Record<string, unknown> | null;
  client: WeKnoraClient;
  t: Translate;
  locale: string;
}

export function AgentShareSettings({ agentId, agent, client, t, locale }: AgentShareSettingsProps) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [shares, setShares] = useState<Share[]>([]);
  const [loadingShares, setLoadingShares] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedOrgId, setSelectedOrgId] = useState('');
  const [addPopupVisible, setAddPopupVisible] = useState(false);
  const [hintVisible, setHintVisible] = useState(false);
  const [pendingUnshare, setPendingUnshare] = useState<Share | null>(null);
  const generationRef = useRef(0);

  // Vue loadShares + loadOrganizations (watch(agentId, immediate) → both).
  const load = useCallback(async () => {
    const generation = ++generationRef.current;
    setLoadingShares(true);
    try {
      // Defensive namespace lookup: bare test fakes may not provide the
      // identity namespace (same posture as PlatformShell probes).
      const orgApi = (client as unknown as {
        identity?: { organizations?: { list?: (signal?: AbortSignal) => Promise<Organization[] | { items?: Organization[] }> } },
      }).identity?.organizations;
      const shareApi = (client as unknown as {
        identity?: { organizations?: { agentShares?: { list?: (agentId: string) => Promise<{ items?: Share[] } | Share[]> } } },
      }).identity?.organizations?.agentShares;
      const [orgs, sharePage] = await Promise.all([
        orgApi?.list ? orgApi.list() : Promise.resolve([] as Organization[]),
        shareApi?.list ? shareApi.list(agentId) : Promise.resolve({ items: [] as Share[] }),
      ]);
      if (generation !== generationRef.current) return;
      setOrganizations(((Array.isArray(orgs) ? orgs : orgs.items ?? []) as Organization[]));
      setShares(((Array.isArray(sharePage) ? sharePage : sharePage.items ?? []) as Share[]));
    } catch (cause) {
      if (generation !== generationRef.current) return;
      console.error('Failed to load agent shares:', cause);
      setShares([]);
    } finally {
      if (generation === generationRef.current) setLoadingShares(false);
    }
  }, [agentId, client]);

  useEffect(() => { void load(); }, [load]);

  // Vue availableOrganizations: not-yet-shared + owner/admin/editor role.
  const availableOrganizations = useMemo(() => {
    const sharedOrgIds = new Set(shares.map((share) => String(share.organization_id ?? '')));
    return organizations.filter((org) => !sharedOrgIds.has(org.id) && canShareInto(org));
  }, [organizations, shares]);

  // Vue filteredShares: haystack = organization_name + shared_by_username.
  const filteredShares = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return shares;
    return shares.filter((share) =>
      [String(share.organization_name ?? ''), String(share.shared_by_username ?? '')]
        .filter(Boolean).join(' ').toLowerCase().includes(query));
  }, [searchQuery, shares]);

  const orgForShare = (share: Share): Organization | undefined =>
    organizations.find((org) => org.id === String(share.organization_id ?? ''));

  // Vue loadShares mapping: fall back to the org list name, then the raw id.
  const orgName = (share: Share): string => {
    const name = String(share.organization_name ?? '');
    if (name) return name;
    const id = String(share.organization_id ?? '');
    return organizations.find((org) => org.id === id)?.name ?? id;
  };

  // Vue handleShare: viewer permission, success/failure toast, reload list.
  const handleShare = async () => {
    if (!selectedOrgId || submitting) return;
    setSubmitting(true);
    try {
      const shareApi = (client as unknown as {
        identity?: { organizations?: { agentShares?: { create?: (agentId: string, input: { organization_id: string; permission: 'viewer' }) => Promise<unknown> } } },
      }).identity?.organizations?.agentShares;
      if (!shareApi?.create) throw new Error(t('organization.share.shareFailed'));
      await shareApi.create(agentId, { organization_id: selectedOrgId, permission: 'viewer' });
      void MessagePlugin.success(t('organization.share.shareSuccess'));
      setSelectedOrgId('');
      setAddPopupVisible(false);
      await load();
    } catch (cause) {
      void MessagePlugin.error(cause instanceof Error ? cause.message : t('organization.share.shareFailed'));
    } finally {
      setSubmitting(false);
    }
  };

  // Vue handleUnshare: DELETE the share row, toast, reload.
  const handleUnshare = async (share: Share) => {
    setPendingUnshare(null);
    try {
      const shareApi = (client as unknown as {
        identity?: { organizations?: { agentShares?: { remove?: (agentId: string, shareId: string) => Promise<unknown> } } },
      }).identity?.organizations?.agentShares;
      if (!shareApi?.remove) throw new Error(t('organization.share.unshareFailed'));
      await shareApi.remove(agentId, share.id);
      void MessagePlugin.success(t('organization.share.unshareSuccess'));
      await load();
    } catch (cause) {
      void MessagePlugin.error(cause instanceof Error ? cause.message : t('organization.share.unshareFailed'));
    }
  };

  return (
    <div className="share-to-space-panel" data-agent-share-settings="">
      <div className="share-panel-header">
        <div className="share-panel-header-row">
          <div className="share-panel-titlewrap">
            <h2 className="share-panel-title">{t('organization.share.title')}</h2>
            {/* Vue t-popup hover 气泡 → 内联提示面板（类名对齐 share-hint-popover） */}
            <span className="share-hint-wrap">
              <button
                type="button"
                className="share-hint-trigger-btn"
                aria-label={t('agent.shareScope.title')}
                title={t('agent.shareScope.title')}
                aria-expanded={hintVisible}
                data-share-hint-trigger=""
                onClick={() => setHintVisible((current) => !current)}
              >
                <TIcon name="info-circle" size="16px" />
              </button>
              {hintVisible ? (
                <div className="share-hint-popover" role="note" data-share-hint="">
                  <p className="share-hint-title">{t('agent.shareScope.title')}</p>
                  <p className="share-hint-desc">{t('organization.share.agentShareDesc')}</p>
                  {agent?.config ? <p className="share-hint-desc">{t('agent.shareScope.desc')}</p> : null}
                </div>
              ) : null}
            </span>
          </div>
        </div>
        <p className="share-panel-desc">{t('organization.share.agentShareDesc')}</p>
      </div>

      <div className="share-panel-list-wrap">
        <div className="share-panel-list-header">
          <div className="share-panel-titlewrap">
            <span className="share-panel-list-title">{t('organization.share.sharedTo')}</span>
            <span className="share-panel-count-badge" data-share-count="">{filteredShares.length}</span>
          </div>
          <div className="share-panel-actions">
            <div className="share-panel-search">
              <Input
                size="small"
                value={searchQuery}
                clearable
                placeholder={t('organization.share.searchPlaceholder')}
                aria-label={t('organization.share.searchPlaceholder')}
                data-share-search=""
                prefixIcon={<TIcon name="search" />}
                onChange={(value) => setSearchQuery(String(value))}
              />
            </div>
            {/* Vue t-popup 添加共享弹层 → 内联面板（类名对齐 share-add-popup-inner） */}
            <span className="share-add-wrap">
              <Tooltip content={t('knowledgeEditor.share.addShare')} placement="top">
                <Button
                  theme="primary"
                  variant="outline"
                  shape="square"
                  size="small"
                  className="share-panel-add-btn"
                  aria-label={t('knowledgeEditor.share.addShare')}
                  aria-expanded={addPopupVisible}
                  data-share-add-trigger=""
                  onClick={() => { setAddPopupVisible((current) => !current); setSelectedOrgId(''); }}
                  icon={<TIcon name="add" />}
                />
              </Tooltip>
              {addPopupVisible ? (
                <div className="share-add-popup-inner" data-share-add-popup="">
                  <div className="member-invite-popup-title">{t('organization.share.addShareDialogTitle')}</div>
                  <div className="org-upgrade-fields">
                    <div className="org-upgrade-field org-upgrade-field--last">
                      <label className="org-upgrade-field-label" htmlFor="wk-agent-share-org">{t('organization.share.selectOrg')}</label>
                      <select
                        id="wk-agent-share-org"
                        className="share-org-select"
                        data-share-add-org=""
                        value={selectedOrgId}
                        disabled={submitting}
                        onChange={(event) => setSelectedOrgId(event.target.value)}
                      >
                        <option value="">{t('organization.share.selectOrg')}</option>
                        {availableOrganizations.map((org) => <option key={org.id} value={org.id}>{org.name}</option>)}
                      </select>
                    </div>
                  </div>
                  <div className="invite-popup-footer">
                    <Button variant="outline" disabled={submitting} data-share-add-cancel="" onClick={() => setAddPopupVisible(false)}>
                      {t('common.cancel')}
                    </Button>
                    <Button theme="primary" loading={submitting} disabled={!selectedOrgId} data-share-add-confirm="" onClick={() => void handleShare()}>
                      {t('knowledgeEditor.share.addShare')}
                    </Button>
                  </div>
                </div>
              ) : null}
            </span>
          </div>
        </div>

        {loadingShares && shares.length === 0 ? (
          <div className="share-panel-loading" data-share-loading="">
            <span>{t('organization.share.loading')}</span>
          </div>
        ) : null}
        {!loadingShares && filteredShares.length === 0 ? (
          <div className="share-panel-empty" data-share-empty="">
            {/* AGT-12 — Vue AgentShareSettings.vue:75-79 空表走 t-empty：
                组件自带默认标题行「暂无数据」(t-empty__title，随 tdesign
                locale) + 传入 description；原 React 只渲染了 description
                文本，缺「暂无数据」占位行。 */}
            <Empty
              className="share-panel-empty-text"
              description={searchQuery.trim() ? t('organization.share.emptySearch', { q: searchQuery.trim() }) : t('organization.share.noShares')}
            />
          </div>
        ) : null}
        {filteredShares.length > 0 ? (
          <div className="share-panel-table-shell">
            <table className="share-panel-table">
              <thead>
                <tr>
                  <th>{t('organization.share.columns.space')}</th>
                  <th>{t('organization.share.columns.permission')}</th>
                  <th>{t('organization.share.columns.sharedAt')}</th>
                  <th>{t('organization.share.columns.operations')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredShares.map((share) => (
                  <tr key={share.id}>
                    <td>
                      <div className="share-space-cell">
                        <span className="share-space-name">
                          <SpaceAvatar name={orgName(share)} avatar={orgForShare(share)?.avatar} size="small" />
                          <span className="share-space-name-text">{orgName(share)}</span>
                        </span>
                        {typeof share.shared_by_username === 'string' && share.shared_by_username ? (
                          <span className="share-space-meta">{t('organization.share.sharedFrom')} {share.shared_by_username}</span>
                        ) : null}
                      </div>
                    </td>
                    <td>
                      {/* Vue t-tag：智能体共享固定只读 */}
                      <span className="share-permission-tag">{t('organization.share.permissionReadonly')}</span>
                    </td>
                    <td>{formatShareDate(share.created_at, locale)}</td>
                    <td>
                      <div className="share-table-actions">
                        {pendingUnshare?.id === share.id ? (
                          // Vue t-popconfirm → 内联确认
                          <span className="share-unshare-confirm" data-share-unshare-confirm-row="">
                            <span className="share-unshare-confirm-text">{t('knowledgeEditor.share.unshareConfirm', { name: orgName(share) })}</span>
                            <Button size="small" theme="danger" variant="text" data-share-unshare-confirm="" onClick={() => void handleUnshare(share)}>
                              {t('common.confirm')}
                            </Button>
                            <Button size="small" variant="text" data-share-unshare-cancel="" onClick={() => setPendingUnshare(null)}>
                              {t('common.cancel')}
                            </Button>
                          </span>
                        ) : (
                          <Tooltip content={t('organization.share.unshareAction')} placement="top">
                            <Button
                              theme="danger"
                              shape="square"
                              variant="text"
                              size="small"
                              aria-label={t('organization.share.unshareAction')}
                              title={t('organization.share.unshareAction')}
                              data-share-unshare-trigger=""
                              onClick={() => setPendingUnshare(share)}
                              icon={<TIcon name="delete" />}
                            />
                          </Tooltip>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
    </div>
  );
}
