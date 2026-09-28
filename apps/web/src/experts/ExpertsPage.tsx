// Octop M2 + M4 — expert-template sources (/platform/experts).
//
// Skeleton follows AnalyticsPage.tsx (props { client }, useState + load()
// + useEffect, formatMessage via the resolved locale, Tailwind class constants);
// the builtin tab keeps the M2 shape (Task 6 client.experts.* endpoints,
// AgentDetailDrawer-style fixed right-side aside, persona_markdown through the
// shared chat markdown boundary, OrganizationsPage fixed top-center
// role=status toast pill, navigation through the platform navigation sink).
//
// Octop M4 adds two tabs (binding spec: three tabs with ?tab= URL state):
// - 远程市场 — the remote SkillHub skillset (expert) index
//   (client.market.skillsets/skillset/installSkillset, packages/api-client
//   market.ts). A cached answer served after a failed refresh answers 200
//   with stale=true, rendered as the MarketPage stale banner. "创建 Agent"
//   reuses the M2 create-agent flow verbatim: pending_skills toast branch +
//   deferred /platform/agents?edit=<id> deep link.
// - 租户内发布 — the tenant-internal expert market (client.market.
//   tenantExperts/publishAgentAsExpert/unpublishExpert/installTenantExpert).
//   Publish is OwnedAgentOrAdmin server-side; the client gate follows the
//   AgentsPage viewer pattern (client.auth.me memberships → isAdmin; the
//   picker lists only the viewer's own agents, created_by === userId). The
//   picker reuses the agents module's list glue (agents/api.ts loadAgents —
//   the same GET /api/v1/agents the agents surface reads). Unpublish is
//   admin-only (tenant_expert_market.go guard) behind an alertdialog confirm.
//
// Tab state lives in ?tab= (builtin|remote|tenant; default and invalid →
// builtin) read once on mount and written with history.replaceState — the
// AgentsPage ?scope= pattern, so a reload keeps the tab and the router keeps
// the page component mounted.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type {
  ExpertDetail,
  ExpertSummary,
  InstantiateResult,
  MarketSkillset,
  MarketSkillsetDetail,
  TenantPublishedExpert,
} from '@weknora/api-client';
import { formatMessage, isLocale } from '@weknora/i18n';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { usePreferredLocale } from '../locale.ts';
import { navigate } from '../platform/navigation.ts';
import { loadAgents } from '../agents/api.ts';
import type { Agent } from '../agents/state.ts';
import '../chat/views-chat-u.css';
import './experts-u.css';

/* Tailwind v4 utility recipes shared across the page (AnalyticsPage constants). */
const XP_PAGE = 'wk-page wk-exp-xp-page';

const XP_HEADER = 'wk-exp-xp-header';

const XP_TITLE = 'wk-exp-xp-title';

const XP_SUBTITLE = 'wk-exp-xp-subtitle';

const XP_TAB = 'wk-exp-xp-tab';

const XP_GRID = 'wk-exp-xp-grid';

const XP_CARD = 'wk-exp-xp-card';

const XP_CARD_STATIC = 'wk-exp-xp-card-static';

const XP_CARD_TITLE = 'wk-exp-xp-card-title';

const XP_CARD_DESC = 'wk-exp-xp-card-desc';

const XP_CHIP = 'wk-exp-xp-chip';

const XP_STATE = 'wk-exp-xp-state';

const XP_ERROR = 'wk-exp-xp-error';

const XP_BTN_PRIMARY = 'wk-exp-xp-btn-primary';

const XP_BTN_OUTLINE = 'wk-exp-xp-btn-outline';

const XP_BTN_DANGER = 'wk-exp-xp-btn-danger';

const XP_INPUT = 'wk-exp-xp-input';

const XP_TEXTAREA = 'wk-exp-xp-textarea';

const XP_SECTION_TITLE = 'wk-exp-xp-section-title';

const XP_STALE = 'wk-exp-xp-stale';


/** expert.color is server-controlled; only plain hex literals reach the style
 * attribute so a hostile catalog value cannot smuggle CSS into the DOM. */
const FALLBACK_DOT_COLOR = 'rgba(127,127,127,0.35)';
function dotColor(color: string): string {
  return /^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(color) ? color : FALLBACK_DOT_COLOR;
}

function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

/** OrganizationsPage toast pill (fixed top-center, auto-dismiss, role=status). */
type ToastState = { tone: 'success' | 'error'; text: string } | null;

/** The success toast stays perceivable for a beat before the editor deep link
 * takes the page away — the experts page owns its toast, so the navigation is
 * deferred past the toast paint instead of flashing it for zero frames. */
const NAVIGATE_DELAY_MS = 600;

/** The three source tabs (binding spec: builtin|remote|tenant, default builtin). */
type ExpertsTab = 'builtin' | 'remote' | 'tenant';
const EXPERTS_TABS: ReadonlyArray<{ key: ExpertsTab; labelKey: string }> = [
  { key: 'builtin', labelKey: 'experts.tabBuiltin' },
  { key: 'remote', labelKey: 'experts.tabRemote' },
  { key: 'tenant', labelKey: 'experts.tabTenant' },
];

/** ?tab= is read once on mount (deep link survives reload); writes go through
 * history.replaceState (AgentsPage ?scope= pattern). The default tab keeps the
 * URL clean, and an invalid value falls back to builtin. */
function readTabFromUrl(): ExpertsTab {
  const value = new URLSearchParams(window.location.search).get('tab')?.trim();
  return value === 'remote' || value === 'tenant' ? value : 'builtin';
}

function writeTabToUrl(tab: ExpertsTab): void {
  const url = new URL(window.location.href);
  if (tab === 'builtin') url.searchParams.delete('tab');
  else url.searchParams.set('tab', tab);
  window.history.replaceState({}, '', url);
}

/** AgentsPage membershipRoleOf: the active tenant's membership role from the
 * /auth/me memberships list (scope-runtime.ts membershipRole precedent). */
function membershipRoleOf(memberships: unknown, tenantId: string | null): string {
  if (!Array.isArray(memberships)) return 'viewer';
  for (const item of memberships) {
    if (!item || typeof item !== 'object') continue;
    const row = item as Record<string, unknown>;
    const id = row.tenant_id ?? row.tenantId;
    const role = row.role;
    if (tenantId !== null && String(id) === tenantId && typeof role === 'string') return role;
  }
  return 'viewer';
}

/** TenantUserProfileSections.formatDateTime precedent for created_at chips. */
function formatDateTime(value: string, locale: string): string {
  if (value === '') return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat(isLocale(locale) ? locale : 'zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date);
}

/** The viewer facets the tenant tab gates on (AgentsPage AgentViewer pattern). */
interface ExpertsViewer {
  ready: boolean;
  userId: string;
  isAdmin: boolean;
}

interface ExpertsPageProps {
  client: WeKnoraClient;
}

export function ExpertsPage({ client }: ExpertsPageProps) {
  const locale = usePreferredLocale();
  const [tab, setTab] = useState<ExpertsTab>(() => (typeof window === 'undefined' ? 'builtin' : readTabFromUrl()));
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [experts, setExperts] = useState<ExpertSummary[]>([]);
  // Detail drawer state: summaryId drives the fetch, detail the rendered body.
  const [detailId, setDetailId] = useState('');
  const [detail, setDetail] = useState<ExpertDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');
  const [agentName, setAgentName] = useState('');
  const [instantiating, setInstantiating] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const navigateTimer = useRef<number | null>(null);

  // Remote-market tab (client.market.skillsets index; lazy per activation).
  const [skillsets, setSkillsets] = useState<{ items: MarketSkillset[]; stale: boolean } | null>(null);
  const [remoteLoading, setRemoteLoading] = useState(false);
  const [remoteError, setRemoteError] = useState('');
  const [remoteEpoch, setRemoteEpoch] = useState(0);
  // Remote skillset drawer (slug drives the fetch — detail carries name_en &c).
  const [remoteSlug, setRemoteSlug] = useState('');
  const [remoteDetail, setRemoteDetail] = useState<MarketSkillsetDetail | null>(null);
  const [remoteDetailLoading, setRemoteDetailLoading] = useState(false);
  const [remoteDetailError, setRemoteDetailError] = useState('');
  const [remoteDetailEpoch, setRemoteDetailEpoch] = useState(0);

  // Tenant-internal tab (client.market.tenantExperts list; lazy per activation).
  const [tenantExperts, setTenantExperts] = useState<TenantPublishedExpert[] | null>(null);
  const [tenantLoading, setTenantLoading] = useState(false);
  const [tenantError, setTenantError] = useState('');
  const [tenantEpoch, setTenantEpoch] = useState(0);
  const [installingTenantId, setInstallingTenantId] = useState('');
  // Publish-from-agent picker (agents/api.ts loadAgents + viewer ownership).
  const [viewer, setViewer] = useState<ExpertsViewer>({ ready: false, userId: '', isAdmin: false });
  const [allAgents, setAllAgents] = useState<Agent[] | null>(null);
  const [agentsError, setAgentsError] = useState('');
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishAgentId, setPublishAgentId] = useState('');
  const [publishName, setPublishName] = useState('');
  const [publishDescription, setPublishDescription] = useState('');
  const [publishing, setPublishing] = useState(false);
  const [unpublishTarget, setUnpublishTarget] = useState<TenantPublishedExpert | null>(null);
  const [unpublishing, setUnpublishing] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setListError('');
    try {
      setExperts(await client.experts.list());
    } catch (reason) {
      setExperts([]);
      setListError(errorText(reason, t(locale, 'common.error')));
    } finally {
      setLoading(false);
    }
  }, [client, locale]);

  useEffect(() => {
    void load();
  }, [load]);

  // Viewer hydrate (AgentsPage pattern): the tenant tab's publish/unpublish
  // gates wait on the authoritative membership result; a failed identity
  // lookup must not accidentally grant the admin affordances.
  useEffect(() => {
    let active = true;
    void client.auth.me().then((me) => {
      if (!active) return;
      const rawTenantId = me.tenant && (typeof me.tenant.id === 'string' || typeof me.tenant.id === 'number') ? String(me.tenant.id) : null;
      const role = membershipRoleOf(me.memberships, rawTenantId);
      setViewer({
        ready: true,
        userId: typeof me.user?.id === 'string' ? me.user.id : '',
        isAdmin: role === 'owner' || role === 'admin' || me.user?.is_system_admin === true,
      });
    }).catch(() => {
      if (active) setViewer({ ready: true, userId: '', isAdmin: false });
    });
    return () => { active = false; };
  }, [client]);

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3000);
    return () => window.clearTimeout(timer);
  }, [toast]);

  // Deferred editor deep link (see NAVIGATE_DELAY_MS) must not fire after the
  // page unmounted for an unrelated reason.
  useEffect(() => () => { if (navigateTimer.current !== null) window.clearTimeout(navigateTimer.current); }, []);

  // Remote skillset index loads when the remote tab activates (MarketPage
  // tenant-tab pattern: epoch-armed retry; the installed flags refresh on
  // every activation).
  useEffect(() => {
    if (tab !== 'remote') return undefined;
    let cancelled = false;
    setRemoteLoading(true);
    setRemoteError('');
    void client.market.skillsets().then((result) => {
      if (cancelled) return;
      setSkillsets({ items: result.skillsets, stale: result.stale });
    }).catch((reason: unknown) => {
      if (cancelled) return;
      setSkillsets(null);
      setRemoteError(errorText(reason, t(locale, 'experts.market.loadFailed')));
    }).finally(() => {
      if (!cancelled) setRemoteLoading(false);
    });
    return () => { cancelled = true; };
  }, [tab, remoteEpoch, client, locale]);

  // The tenant tab rides the experts list and the agents glue in one pass —
  // the picker needs the viewer's own agents (loadAgents is the agents
  // module's list function, the same GET /api/v1/agents the agents page uses).
  useEffect(() => {
    if (tab !== 'tenant') return undefined;
    let cancelled = false;
    setTenantLoading(true);
    setTenantError('');
    const load = async (): Promise<void> => {
      const [expertsResult, agentsResult] = await Promise.allSettled([
        client.market.tenantExperts(),
        loadAgents(client),
      ]);
      if (cancelled) return;
      if (expertsResult.status === 'fulfilled') setTenantExperts(expertsResult.value.experts);
      else {
        setTenantExperts([]);
        setTenantError(errorText(expertsResult.reason, t(locale, 'experts.tenant.loadFailed')));
      }
      if (agentsResult.status === 'rejected') {
        setAllAgents([]);
        setAgentsError(errorText(agentsResult.reason, t(locale, 'experts.tenant.agentsLoadFailed')));
      } else if (agentsResult.value.status === 'error') {
        setAllAgents([]);
        setAgentsError(agentsResult.value.message);
      } else {
        setAllAgents(agentsResult.value.items);
        setAgentsError('');
      }
      setTenantLoading(false);
    };
    void load().catch(() => { if (!cancelled) setTenantLoading(false); });
    return () => { cancelled = true; };
  }, [tab, tenantEpoch, client, locale]);

  const reloadTenant = useCallback(async (): Promise<void> => {
    try {
      const [expertsResult, agentsResult] = await Promise.allSettled([
        client.market.tenantExperts(),
        loadAgents(client),
      ]);
      if (expertsResult.status === 'fulfilled') setTenantExperts(expertsResult.value.experts);
      if (agentsResult.status === 'fulfilled' && agentsResult.value.status === 'success') setAllAgents(agentsResult.value.items);
    } catch {
      // The toast surfaces action failures; a silent refresh miss is fine.
    }
  }, [client]);

  // Publish is OwnedAgentOrAdmin server-side: the picker lists only the
  // viewer's own agents (classifyAgent 'mine' rule), and the entry shows for
  // admins (who may publish any agent) or members who own at least one.
  const ownAgents = useMemo(
    () => (allAgents ?? []).filter((agent) => viewer.userId !== '' && !agent.is_builtin && agent.created_by === viewer.userId),
    [allAgents, viewer.userId],
  );
  const canPublish = viewer.ready && (viewer.isAdmin || ownAgents.length > 0);

  const openDetail = useCallback(async (expert: ExpertSummary) => {
    setDetailId(expert.id);
    setDetail(null);
    setDetailError('');
    setDetailLoading(true);
    setAgentName(expert.label);
    try {
      setDetail(await client.experts.get(expert.id));
    } catch (reason) {
      setDetailError(errorText(reason, t(locale, 'experts.detail.loadFailed')));
    } finally {
      setDetailLoading(false);
    }
  }, [client, locale]);

  function closeDetail(): void {
    setDetailId('');
    setDetail(null);
    setDetailError('');
  }

  function selectTab(next: ExpertsTab): void {
    // Only one surface's drawer may be open at a time — the overlay blocks
    // mouse tab clicks, but a keyboard-driven tab switch must not stack two
    // role=dialog asides.
    closeDetail();
    closeRemoteDetail();
    setTab(next);
    writeTabToUrl(next);
  }

  function openRemoteDetail(skillset: MarketSkillset): void {
    setRemoteSlug(skillset.slug);
    setRemoteDetail(null);
    setRemoteDetailError('');
    setAgentName(skillset.name || skillset.slug);
  }

  function closeRemoteDetail(): void {
    setRemoteSlug('');
    setRemoteDetail(null);
    setRemoteDetailError('');
  }

  // The remote drawer fetch runs off the slug so the retry button can re-arm
  // it (remoteDetailEpoch) without a card reference.
  useEffect(() => {
    if (remoteSlug === '') return undefined;
    let cancelled = false;
    setRemoteDetailLoading(true);
    setRemoteDetailError('');
    void client.market.skillset(remoteSlug).then((result) => {
      if (cancelled) return;
      setRemoteDetail(result);
    }).catch((reason: unknown) => {
      if (cancelled) return;
      setRemoteDetailError(errorText(reason, t(locale, 'experts.market.detail.loadFailed')));
    }).finally(() => {
      if (!cancelled) setRemoteDetailLoading(false);
    });
    return () => { cancelled = true; };
  }, [remoteSlug, remoteDetailEpoch, client, locale]);

  /**
   * The M2 create-agent flow, shared by all three sources: pending_skills
   * toast branch + deferred /platform/agents?edit=<id> deep link
   * (apps/web/src/agents/route.ts buildAgentPath contract).
   */
  function finishAgentCreation(result: InstantiateResult): void {
    setToast({
      tone: 'success',
      text: result.pending_skills.length > 0
        ? t(locale, 'experts.createdPending', { count: result.pending_skills.length })
        : t(locale, 'experts.created'),
    });
    const target = `/platform/agents?edit=${encodeURIComponent(result.agent.id)}`;
    navigateTimer.current = window.setTimeout(() => {
      navigateTimer.current = null;
      navigate(target);
    }, NAVIGATE_DELAY_MS);
  }

  function createFailedToast(reason: unknown): void {
    const message = errorText(reason, '');
    setToast({ tone: 'error', text: message !== '' ? t(locale, 'experts.createFailedMessage', { message }) : t(locale, 'experts.createFailed') });
  }

  async function instantiate(): Promise<void> {
    if (!detail || instantiating) return;
    setInstantiating(true);
    try {
      const name = agentName.trim() || detail.label;
      const result = await client.experts.instantiate(detail.id, { agentName: name });
      setInstantiating(false);
      finishAgentCreation(result);
    } catch (reason) {
      setInstantiating(false);
      createFailedToast(reason);
    }
  }

  async function installSkillset(): Promise<void> {
    if (!remoteDetail || instantiating) return;
    setInstantiating(true);
    try {
      const name = agentName.trim() || remoteDetail.name || remoteDetail.slug;
      const result = await client.market.installSkillset(remoteDetail.slug, { agentName: name });
      setInstantiating(false);
      finishAgentCreation(result);
    } catch (reason) {
      setInstantiating(false);
      createFailedToast(reason);
    }
  }

  async function installTenant(expert: TenantPublishedExpert): Promise<void> {
    if (installingTenantId !== '') return;
    setInstallingTenantId(expert.id);
    try {
      const result = await client.market.installTenantExpert(expert.id);
      setInstallingTenantId('');
      finishAgentCreation(result);
    } catch (reason) {
      setInstallingTenantId('');
      createFailedToast(reason);
    }
  }

  function openPublish(): void {
    setPublishAgentId(ownAgents[0]?.id ?? '');
    setPublishName('');
    setPublishDescription('');
    setPublishOpen(true);
  }

  async function publish(): Promise<void> {
    if (publishing || publishAgentId === '') return;
    setPublishing(true);
    try {
      await client.market.publishAgentAsExpert(publishAgentId, {
        ...(publishName.trim() === '' ? {} : { name: publishName.trim() }),
        ...(publishDescription.trim() === '' ? {} : { description: publishDescription.trim() }),
      });
      setPublishing(false);
      setPublishOpen(false);
      setToast({ tone: 'success', text: t(locale, 'experts.tenant.publishAccepted') });
      await reloadTenant();
    } catch (reason) {
      setPublishing(false);
      const message = errorText(reason, '');
      setToast({ tone: 'error', text: message !== '' ? t(locale, 'experts.tenant.publishFailedMessage', { message }) : t(locale, 'experts.tenant.publishFailed') });
    }
  }

  async function unpublishConfirmed(): Promise<void> {
    if (!unpublishTarget || unpublishing) return;
    setUnpublishing(true);
    try {
      await client.market.unpublishExpert(unpublishTarget.id);
      setUnpublishing(false);
      setUnpublishTarget(null);
      setToast({ tone: 'success', text: t(locale, 'experts.tenant.unpublishAccepted') });
      await reloadTenant();
    } catch (reason) {
      setUnpublishing(false);
      const message = errorText(reason, '');
      setToast({ tone: 'error', text: message !== '' ? t(locale, 'experts.tenant.unpublishFailedMessage', { message }) : t(locale, 'experts.tenant.unpublishFailed') });
    }
  }

  function tabButton(entry: { key: ExpertsTab; labelKey: string }): React.ReactNode {
    return (
      <button
        key={entry.key}
        type="button"
        role="tab"
        aria-selected={tab === entry.key}
        data-experts-tab={entry.key}
        className={XP_TAB}
        onClick={() => selectTab(entry.key)}
      >{t(locale, entry.labelKey)}</button>
    );
  }

  function tenantCard(expert: TenantPublishedExpert): React.ReactNode {
    const busy = installingTenantId !== '';
    return (
      <article key={expert.id} data-tenant-expert={expert.id} className={XP_CARD_STATIC}>
        <h3 className={XP_CARD_TITLE}>
          <span className="wk-exp-1" title={expert.name}>{expert.name}</span>
        </h3>
        {expert.description ? <p className={XP_CARD_DESC} title={expert.description}>{expert.description}</p> : null}
        <div className="wk-exp-2">
          {expert.installed ? <span className={XP_CHIP}>{t(locale, 'experts.market.installedTag')}</span> : null}
          {expert.publisher_name ? <span className={XP_CHIP}>{t(locale, 'market.publisher', { name: expert.publisher_name })}</span> : null}
          {formatDateTime(expert.created_at, locale) ? <span className={XP_CHIP}>{formatDateTime(expert.created_at, locale)}</span> : null}
          <span className="wk-exp-3">
            <button
              type="button"
              className={XP_BTN_PRIMARY}
              data-tenant-install={expert.id}
              disabled={busy}
              onClick={() => void installTenant(expert)}
            >{installingTenantId === expert.id ? t(locale, 'experts.create.creating') : t(locale, 'experts.createAgent')}</button>
            {viewer.isAdmin ? (
              <button
                type="button"
                className={XP_BTN_DANGER}
                data-tenant-unpublish={expert.id}
                disabled={unpublishing}
                onClick={() => setUnpublishTarget(expert)}
              >{t(locale, 'experts.tenant.unpublish')}</button>
            ) : null}
          </span>
        </div>
      </article>
    );
  }

  return (
    <main className={XP_PAGE}>
      <header className={XP_HEADER}>
        <div>
          <h2 className={XP_TITLE}>{t(locale, 'experts.title')}</h2>
          <p className={XP_SUBTITLE}>{t(locale, 'experts.subtitle')}</p>
        </div>
        <div role="tablist" aria-label={t(locale, 'experts.tabsLabel')} className="wk-exp-4">
          {EXPERTS_TABS.map(tabButton)}
        </div>
      </header>

      {toast ? (
        <div
          role="status"
          className={'wk-exp-42 ' + (toast.tone === 'success' ? 'wk-exp-43' : 'wk-exp-44')}
        >{toast.text}</div>
      ) : null}

      {tab === 'builtin' ? (
        <>
          {loading ? <div className={XP_STATE} role="status">{t(locale, 'common.loading')}</div> : null}
          {!loading && listError ? (
            <div className={XP_ERROR} role="alert">
              <span>{listError}</span>
              <button type="button" className={XP_BTN_OUTLINE} onClick={() => void load()}>{t(locale, 'common.retry')}</button>
            </div>
          ) : null}
          {!loading && !listError && experts.length === 0 ? <div className={XP_STATE}>{t(locale, 'experts.empty')}</div> : null}
          {!loading && !listError && experts.length > 0 ? (
            <div className={XP_GRID}>
              {experts.map((expert) => (
                <article
                  key={expert.id}
                  data-expert-id={expert.id}
                  className={XP_CARD}
                  onClick={() => void openDetail(expert)}
                >
                  <h3 className={XP_CARD_TITLE}>
                    <span aria-hidden="true" className="wk-exp-5" style={{ background: dotColor(expert.color) }} />
                    <span className="wk-exp-1" title={expert.label}>{expert.label}</span>
                  </h3>
                  <p className={XP_CARD_DESC}>{expert.description}</p>
                  <div className="wk-exp-2">
                    <span className={XP_CHIP}>{t(locale, 'experts.skillCount', { count: expert.skills.length })}</span>
                    {expert.persona_mbti ? <span className={XP_CHIP}>{expert.persona_mbti}</span> : null}
                  </div>
                </article>
              ))}
            </div>
          ) : null}
        </>
      ) : null}

      {tab === 'remote' ? (
        <>
          {remoteLoading ? <div className={XP_STATE} role="status">{t(locale, 'common.loading')}</div> : null}
          {!remoteLoading && remoteError ? (
            <div className={XP_ERROR} role="alert">
              <span>{remoteError}</span>
              <button type="button" className={XP_BTN_OUTLINE} onClick={() => setRemoteEpoch((current) => current + 1)}>{t(locale, 'common.retry')}</button>
            </div>
          ) : null}
          {!remoteLoading && !remoteError && skillsets !== null ? (
            <>
              {skillsets.stale ? <div className={XP_STALE} role="status" data-testid="experts-stale-banner">{t(locale, 'market.staleBanner')}</div> : null}
              {skillsets.items.length === 0 ? <div className={XP_STATE}>{t(locale, 'experts.market.empty')}</div> : (
                <div className={XP_GRID}>
                  {skillsets.items.map((skillset) => (
                    <article
                      key={skillset.slug}
                      data-skillset-slug={skillset.slug}
                      className={XP_CARD}
                      onClick={() => openRemoteDetail(skillset)}
                    >
                      <h3 className={XP_CARD_TITLE}>
                        <span className="wk-exp-1" title={skillset.name || skillset.slug}>{skillset.name || skillset.slug}</span>
                      </h3>
                      <p className={XP_CARD_DESC}>{skillset.description}</p>
                      <div className="wk-exp-2">
                        <span className={XP_CHIP}>{t(locale, 'experts.skillCount', { count: skillset.skill_slugs.length })}</span>
                        {skillset.installed ? <span className={XP_CHIP}>{t(locale, 'experts.market.installedTag')}</span> : null}
                      </div>
                    </article>
                  ))}
                </div>
              )}
            </>
          ) : null}
        </>
      ) : null}

      {tab === 'tenant' ? (
        <>
          {tenantLoading ? <div className={XP_STATE} role="status">{t(locale, 'common.loading')}</div> : null}
          {!tenantLoading && tenantError ? (
            <div className={XP_ERROR} role="alert">
              <span>{tenantError}</span>
              <button type="button" className={XP_BTN_OUTLINE} onClick={() => setTenantEpoch((current) => current + 1)}>{t(locale, 'common.retry')}</button>
            </div>
          ) : null}
          {!tenantLoading && !tenantError && tenantExperts !== null ? (
            <>
              {canPublish ? (
                <div className="wk-exp-6">
                  <button type="button" className={XP_BTN_PRIMARY} data-expert-publish-open onClick={openPublish}>{t(locale, 'experts.tenant.publishOpen')}</button>
                </div>
              ) : null}
              {tenantExperts.length === 0 ? <div className={XP_STATE}>{t(locale, 'experts.tenant.empty')}</div> : (
                <div className={XP_GRID}>{tenantExperts.map(tenantCard)}</div>
              )}
            </>
          ) : null}
        </>
      ) : null}

      {detailId !== '' ? (
        <div
          className="wk-exp-7"
          onClick={(event) => { if (event.target === event.currentTarget) closeDetail(); }}
        >
          <aside
            className="wk-exp-8"
            role="dialog"
            aria-label={t(locale, 'experts.title')}
            data-expert-detail={detailId}
          >
            <div className="wk-exp-9">
              <h3 className="wk-exp-10">
                {detail ? (
                  <>
                    <span aria-hidden="true" className="wk-exp-5" style={{ background: dotColor(detail.color) }} />
                    <span className="wk-exp-1">{detail.label}</span>
                  </>
                ) : t(locale, 'experts.title')}
              </h3>
              <button
                type="button"
                className="wk-exp-11"
                aria-label={t(locale, 'common.cancel')}
                onClick={closeDetail}
              >✕</button>
            </div>

            <div className="wk-exp-12">
              {detailLoading ? <div role="status">{t(locale, 'common.loading')}</div> : null}
              {!detailLoading && detailError ? (
                <div className="wk-exp-13" role="alert">
                  <span>{detailError}</span>
                  {detailId ? <button type="button" className={XP_BTN_OUTLINE} onClick={() => { const id = detailId; const summary = experts.find((row) => row.id === id); if (summary) void openDetail(summary); }}>{t(locale, 'common.retry')}</button> : null}
                </div>
              ) : null}
              {!detailLoading && !detailError && detail ? (
                <>
                  <p className="wk-exp-14">{detail.description}</p>
                  <div className="wk-exp-15">
                    {detail.persona_mbti ? <span className={XP_CHIP}>{detail.persona_mbti}</span> : null}
                    {detail.skills.map((skill) => <span key={skill} className={XP_CHIP}>{skill}</span>)}
                  </div>

                  <section aria-label={t(locale, 'experts.detail.persona')}>
                    <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.persona')}</h4>
                    {/* renderChatMarkdown escapes raw HTML and allow-lists links */}
                    <div
                      className="wk-experts-persona wk-exp-16"
                      data-expert-persona
                      dangerouslySetInnerHTML={{ __html: renderChatMarkdown(detail.persona_markdown) }}
                    />
                  </section>

                  {detail.quick_prompts.length > 0 ? (
                    <section aria-label={t(locale, 'experts.detail.quickPrompts')}>
                      <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.quickPrompts')}</h4>
                      <ul className="wk-exp-17">
                        {detail.quick_prompts.map((prompt) => (
                          <li key={prompt.title} className="wk-exp-18" data-expert-prompt>
                            <div className="wk-exp-4">
                              <span aria-hidden="true" className="wk-exp-19" style={{ background: dotColor(prompt.color) }} />
                              <span className="wk-exp-20">{prompt.title}</span>
                            </div>
                            {prompt.description ? <p className="wk-exp-21">{prompt.description}</p> : null}
                          </li>
                        ))}
                      </ul>
                    </section>
                  ) : null}
                </>
              ) : null}
            </div>

            <div className="wk-exp-22">
              <label className="wk-exp-23">
                {t(locale, 'experts.create.nameLabel')}
                <input
                  className={XP_INPUT + ' wk-exp-45'}
                  value={agentName}
                  placeholder={t(locale, 'experts.create.namePlaceholder')}
                  onChange={(event) => setAgentName(event.target.value)}
                />
              </label>
              <button
                type="button"
                className={XP_BTN_PRIMARY + ' wk-exp-46'}
                disabled={!detail || detailError !== '' || instantiating}
                data-expert-instantiate={detailId}
                onClick={() => void instantiate()}
              >{instantiating ? t(locale, 'experts.create.creating') : t(locale, 'experts.createAgent')}</button>
            </div>
          </aside>
        </div>
      ) : null}

      {remoteSlug !== '' ? (
        <div
          className="wk-exp-7"
          onClick={(event) => { if (event.target === event.currentTarget) closeRemoteDetail(); }}
        >
          <aside
            className="wk-exp-8"
            role="dialog"
            aria-label={t(locale, 'experts.tabRemote')}
            data-skillset-detail={remoteSlug}
          >
            <div className="wk-exp-9">
              <h3 className="wk-exp-24">
                {remoteDetail ? <span className="wk-exp-1">{remoteDetail.name || remoteDetail.slug}</span> : t(locale, 'experts.tabRemote')}
              </h3>
              <button
                type="button"
                className="wk-exp-11"
                aria-label={t(locale, 'common.cancel')}
                onClick={closeRemoteDetail}
              >✕</button>
            </div>

            <div className="wk-exp-12">
              {remoteDetailLoading ? <div role="status">{t(locale, 'common.loading')}</div> : null}
              {!remoteDetailLoading && remoteDetailError ? (
                <div className="wk-exp-13" role="alert">
                  <span>{remoteDetailError}</span>
                  <button type="button" className={XP_BTN_OUTLINE} onClick={() => setRemoteDetailEpoch((current) => current + 1)}>{t(locale, 'common.retry')}</button>
                </div>
              ) : null}
              {!remoteDetailLoading && !remoteDetailError && remoteDetail ? (
                <>
                  <p className="wk-exp-14">{remoteDetail.description}</p>
                  <section aria-label={t(locale, 'experts.detail.skills')}>
                    <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.skills')}</h4>
                    <div className="wk-exp-15">
                      {remoteDetail.installed ? <span className={XP_CHIP}>{t(locale, 'experts.market.installedTag')}</span> : null}
                      {remoteDetail.skill_slugs.map((slug) => <span key={slug} className={XP_CHIP} title={slug}>{slug}</span>)}
                    </div>
                  </section>
                </>
              ) : null}
            </div>

            <div className="wk-exp-22">
              <label className="wk-exp-23">
                {t(locale, 'experts.create.nameLabel')}
                <input
                  className={XP_INPUT + ' wk-exp-45'}
                  value={agentName}
                  placeholder={t(locale, 'experts.create.namePlaceholder')}
                  onChange={(event) => setAgentName(event.target.value)}
                />
              </label>
              <button
                type="button"
                className={XP_BTN_PRIMARY + ' wk-exp-46'}
                disabled={!remoteDetail || remoteDetailError !== '' || instantiating}
                data-skillset-install={remoteSlug}
                onClick={() => void installSkillset()}
              >{instantiating ? t(locale, 'experts.create.creating') : t(locale, 'experts.createAgent')}</button>
            </div>
          </aside>
        </div>
      ) : null}

      {publishOpen ? (
        <div
          className="wk-exp-25"
          onClick={(event) => { if (event.target === event.currentTarget) setPublishOpen(false); }}
        >
          <div
            className="wk-exp-26"
            role="dialog"
            aria-label={t(locale, 'experts.tenant.publishTitle')}
            data-expert-publish-dialog
          >
            <div className="wk-exp-9">
              <h3 className="wk-exp-27">{t(locale, 'experts.tenant.publishTitle')}</h3>
              <button
                type="button"
                className="wk-exp-11"
                aria-label={t(locale, 'common.cancel')}
                onClick={() => setPublishOpen(false)}
              >✕</button>
            </div>

            <div className="wk-exp-12">
              <p className="wk-exp-28">{t(locale, 'experts.tenant.publishDesc')}</p>
              <section aria-label={t(locale, 'experts.tenant.publishAgentLabel')}>
                <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.tenant.publishAgentLabel')}</h4>
                {agentsError !== '' ? <p className="wk-exp-29" role="alert">{agentsError}</p> : null}
                {allAgents === null && agentsError === '' ? <div role="status">{t(locale, 'common.loading')}</div> : null}
                {allAgents !== null && ownAgents.length === 0 ? <p className="wk-exp-30">{t(locale, 'experts.tenant.publishAgentEmpty')}</p> : null}
                <ul className="wk-exp-17">
                  {ownAgents.map((agent) => {
                    const selected = publishAgentId === agent.id;
                    return (
                      <li key={agent.id}>
                        <button
                          type="button"
                          data-publish-agent={agent.id}
                          aria-pressed={selected ? 'true' : 'false'}
                          className={'wk-exp-47 ' + (selected ? 'wk-exp-48' : 'wk-exp-49')}
                          onClick={() => setPublishAgentId(agent.id)}
                        >
                          <span className="wk-exp-31" title={agent.name}>{agent.name}</span>
                          {agent.description ? <span className="wk-exp-32">{agent.description}</span> : null}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </section>
              <label className="wk-exp-33">
                {t(locale, 'experts.tenant.publishNameLabel')}
                <input
                  className={XP_INPUT + ' wk-exp-45'}
                  value={publishName}
                  placeholder={t(locale, 'experts.tenant.publishNamePlaceholder')}
                  onChange={(event) => setPublishName(event.target.value)}
                />
              </label>
              <label className="wk-exp-33">
                {t(locale, 'experts.tenant.publishDescriptionLabel')}
                <textarea
                  className={XP_TEXTAREA}
                  rows={3}
                  value={publishDescription}
                  placeholder={t(locale, 'experts.tenant.publishDescriptionPlaceholder')}
                  onChange={(event) => setPublishDescription(event.target.value)}
                />
              </label>
            </div>

            <div className="wk-exp-34">
              <button type="button" className={XP_BTN_OUTLINE} onClick={() => setPublishOpen(false)}>{t(locale, 'common.cancel')}</button>
              <button
                type="button"
                className={XP_BTN_PRIMARY}
                data-expert-publish-confirm
                disabled={publishing || publishAgentId === ''}
                onClick={() => void publish()}
              >{publishing ? t(locale, 'experts.tenant.publishing') : t(locale, 'experts.tenant.publishConfirm')}</button>
            </div>
          </div>
        </div>
      ) : null}

      {unpublishTarget ? (
        <div
          className="wk-exp-35"
          onClick={(event) => { if (event.target === event.currentTarget) setUnpublishTarget(null); }}
        >
          <div
            className="wk-exp-36"
            role="alertdialog"
            aria-label={t(locale, 'experts.tenant.unpublishTitle')}
            data-expert-unpublish-dialog
          >
            <div className="wk-exp-37">{t(locale, 'experts.tenant.unpublishTitle')}</div>
            <p className="wk-exp-38">{t(locale, 'experts.tenant.unpublishMessage', { name: unpublishTarget.name })}</p>
            <div className="wk-exp-39">
              <button type="button" className="wk-exp-40" onClick={() => setUnpublishTarget(null)}>{t(locale, 'common.cancel')}</button>
              <button type="button" className="wk-exp-41" data-expert-unpublish-confirm disabled={unpublishing} onClick={() => void unpublishConfirmed()}>
                {unpublishing ? t(locale, 'common.loading') : t(locale, 'experts.tenant.unpublish')}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}
