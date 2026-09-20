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

/* Tailwind v4 utility recipes shared across the page (AnalyticsPage constants). */
const XP_PAGE = 'wk-page box-border h-full overflow-y-auto px-[28px] pt-[24px] pb-[32px]';
const XP_HEADER = 'mb-[20px] flex flex-col gap-[12px]';
const XP_TITLE = 'm-0 text-[24px] font-semibold leading-[32px] text-[rgba(23,26,29,0.92)]';
const XP_SUBTITLE = 'm-0 text-[14px] font-normal leading-[20px] text-[rgba(23,26,29,0.6)]';
const XP_TAB = 'box-border inline-flex h-[32px] cursor-pointer items-center rounded-[6px] border border-[#e7e7ea] bg-surface px-[14px] font-[inherit] text-[13px] font-medium text-[rgba(23,26,29,0.75)] [transition:all_.2s_ease] hover:border-accent hover:text-accent aria-selected:border-accent aria-selected:bg-accent-wash aria-selected:text-accent';
const XP_GRID = 'grid grid-cols-1 gap-[16px] min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4';
const XP_CARD = 'box-border flex cursor-pointer flex-col gap-[10px] rounded-[10px] border border-[#e7e7ea] bg-surface px-[16px] py-[14px] text-left shadow-[0_1px_3px_rgba(0,0,0,0.04)] [transition:all_.2s_ease] hover:border-accent hover:shadow-[0_4px_14px_rgba(0,0,0,0.08)]';
const XP_CARD_STATIC = 'box-border flex flex-col gap-[10px] rounded-[10px] border border-[#e7e7ea] bg-surface px-[16px] py-[14px] text-left shadow-[0_1px_3px_rgba(0,0,0,0.04)]';
const XP_CARD_TITLE = 'm-0 flex items-center gap-[8px] text-[16px] font-semibold leading-[24px] text-[rgba(23,26,29,0.92)]';
const XP_CARD_DESC = 'm-0 line-clamp-2 text-[13px] font-normal leading-[19px] text-[rgba(23,26,29,0.6)]';
const XP_CHIP = 'inline-flex shrink-0 items-center rounded-[10px] bg-[rgba(127,127,127,0.1)] px-[8px] py-[2px] text-[11px] font-medium text-[rgba(23,26,29,0.75)]';
const XP_STATE = 'flex h-[240px] items-center justify-center text-[13px] text-[rgba(23,26,29,0.4)]';
const XP_ERROR = 'flex h-[240px] flex-col items-center justify-center gap-[10px] text-[13px] text-[#d54941]';
const XP_BTN_PRIMARY = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border-0 bg-accent px-[15px] font-[inherit] text-[14px] font-medium text-white shadow-[0_2px_8px_rgba(7,192,95,0.25)] [transition:all_.2s_ease] hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)] disabled:cursor-not-allowed disabled:opacity-55';
const XP_BTN_OUTLINE = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(7,192,95,0.5)] bg-surface px-[15px] font-[inherit] text-[14px] font-medium text-accent [transition:all_.2s_ease] hover:border-accent hover:bg-accent-wash disabled:cursor-not-allowed disabled:opacity-55';
const XP_BTN_DANGER = 'box-border inline-flex h-[32px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(213,73,65,0.5)] bg-surface px-[15px] font-[inherit] text-[14px] font-medium text-[#d54941] [transition:all_.2s_ease] hover:border-[#d54941] hover:bg-[rgba(213,73,65,0.06)] disabled:cursor-not-allowed disabled:opacity-55';
const XP_INPUT = 'box-border h-[32px] w-[200px] rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] font-[inherit] text-[13px] text-[rgba(23,26,29,0.92)] placeholder:text-[rgba(23,26,29,0.35)] focus:border-accent focus:outline-none';
const XP_TEXTAREA = 'box-border min-h-[64px] w-full resize-y rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] py-[8px] font-[inherit] text-[13px] leading-[19px] text-[rgba(23,26,29,0.92)] placeholder:text-[rgba(23,26,29,0.35)] focus:border-accent focus:outline-none';
const XP_SECTION_TITLE = 'm-0 mb-[8px] text-[13px] font-semibold uppercase tracking-[0.04em] text-[rgba(23,26,29,0.45)]';
const XP_STALE = 'mb-[16px] rounded-[8px] border border-[rgba(237,123,47,0.4)] bg-[rgba(237,123,47,0.08)] px-[14px] py-[10px] text-[13px] leading-[19px] text-[#b45309]';

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
          <span className="truncate" title={expert.name}>{expert.name}</span>
        </h3>
        {expert.description ? <p className={XP_CARD_DESC} title={expert.description}>{expert.description}</p> : null}
        <div className="mt-auto flex flex-wrap items-center gap-[6px]">
          {expert.installed ? <span className={XP_CHIP}>{t(locale, 'experts.market.installedTag')}</span> : null}
          {expert.publisher_name ? <span className={XP_CHIP}>{t(locale, 'market.publisher', { name: expert.publisher_name })}</span> : null}
          {formatDateTime(expert.created_at, locale) ? <span className={XP_CHIP}>{formatDateTime(expert.created_at, locale)}</span> : null}
          <span className="ml-auto flex items-center gap-[6px]">
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
        <div role="tablist" aria-label={t(locale, 'experts.tabsLabel')} className="flex items-center gap-[8px]">
          {EXPERTS_TABS.map(tabButton)}
        </div>
      </header>

      {toast ? (
        <div
          role="status"
          className={'fixed left-1/2 top-[24px] z-[3000] box-border flex max-w-[420px] -translate-x-1/2 items-center rounded-[8px] bg-[rgba(23,26,29,0.86)] px-[18px] py-[10px] text-[13px] shadow-[0_6px_20px_rgba(0,0,0,0.18)] ' + (toast.tone === 'success' ? 'text-[#7bf2b6]' : 'text-[#ffb4ae]')}
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
                    <span aria-hidden="true" className="inline-block h-[10px] w-[10px] shrink-0 rounded-full" style={{ background: dotColor(expert.color) }} />
                    <span className="truncate" title={expert.label}>{expert.label}</span>
                  </h3>
                  <p className={XP_CARD_DESC}>{expert.description}</p>
                  <div className="mt-auto flex flex-wrap items-center gap-[6px]">
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
                        <span className="truncate" title={skillset.name || skillset.slug}>{skillset.name || skillset.slug}</span>
                      </h3>
                      <p className={XP_CARD_DESC}>{skillset.description}</p>
                      <div className="mt-auto flex flex-wrap items-center gap-[6px]">
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
                <div className="mb-[16px] flex justify-end">
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
          className="fixed inset-0 z-[1000] flex justify-end bg-[rgba(0,0,0,0.4)]"
          onClick={(event) => { if (event.target === event.currentTarget) closeDetail(); }}
        >
          <aside
            className="flex h-full w-[420px] max-w-[90vw] flex-col bg-[var(--wk-bg,#fff)] shadow-[-4px_0_24px_rgba(0,0,0,0.12)]"
            role="dialog"
            aria-label={t(locale, 'experts.title')}
            data-expert-detail={detailId}
          >
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[rgba(127,127,127,0.2)] px-6 py-5">
              <h3 className="m-0 flex min-w-0 items-center gap-[8px] text-[18px] font-semibold">
                {detail ? (
                  <>
                    <span aria-hidden="true" className="inline-block h-[10px] w-[10px] shrink-0 rounded-full" style={{ background: dotColor(detail.color) }} />
                    <span className="truncate">{detail.label}</span>
                  </>
                ) : t(locale, 'experts.title')}
              </h3>
              <button
                type="button"
                className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border-none bg-[rgba(127,127,127,0.1)] text-[15px]"
                aria-label={t(locale, 'common.cancel')}
                onClick={closeDetail}
              >✕</button>
            </div>

            <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-6">
              {detailLoading ? <div role="status">{t(locale, 'common.loading')}</div> : null}
              {!detailLoading && detailError ? (
                <div className="flex flex-col items-start gap-[10px] text-[13px] text-[#d54941]" role="alert">
                  <span>{detailError}</span>
                  {detailId ? <button type="button" className={XP_BTN_OUTLINE} onClick={() => { const id = detailId; const summary = experts.find((row) => row.id === id); if (summary) void openDetail(summary); }}>{t(locale, 'common.retry')}</button> : null}
                </div>
              ) : null}
              {!detailLoading && !detailError && detail ? (
                <>
                  <p className="m-0 text-[14px] leading-[22px] text-[rgba(23,26,29,0.6)]">{detail.description}</p>
                  <div className="flex flex-wrap items-center gap-[6px]">
                    {detail.persona_mbti ? <span className={XP_CHIP}>{detail.persona_mbti}</span> : null}
                    {detail.skills.map((skill) => <span key={skill} className={XP_CHIP}>{skill}</span>)}
                  </div>

                  <section aria-label={t(locale, 'experts.detail.persona')}>
                    <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.persona')}</h4>
                    {/* renderChatMarkdown escapes raw HTML and allow-lists links */}
                    <div
                      className="wk-experts-persona text-[13px] leading-[21px] text-[rgba(23,26,29,0.85)] [&_a]:text-accent [&_a]:underline [&_h1]:mt-[12px] [&_h1]:mb-[6px] [&_h1]:text-[16px] [&_h1]:font-semibold [&_h2]:mt-[12px] [&_h2]:mb-[6px] [&_h2]:text-[15px] [&_h2]:font-semibold [&_h3]:mt-[12px] [&_h3]:mb-[6px] [&_h3]:text-[14px] [&_h3]:font-semibold [&_p]:m-0 [&_p]:mb-[8px] [&_ul]:m-0 [&_ul]:mb-[8px] [&_ul]:list-disc [&_ul]:pl-[18px] [&_ol]:m-0 [&_ol]:mb-[8px] [&_ol]:list-decimal [&_ol]:pl-[18px] [&_code]:rounded-[4px] [&_code]:bg-[rgba(127,127,127,0.1)] [&_code]:px-[4px] [&_pre]:overflow-x-auto [&_pre]:rounded-[6px] [&_pre]:bg-[rgba(127,127,127,0.08)] [&_pre]:p-[10px]"
                      data-expert-persona
                      dangerouslySetInnerHTML={{ __html: renderChatMarkdown(detail.persona_markdown) }}
                    />
                  </section>

                  {detail.quick_prompts.length > 0 ? (
                    <section aria-label={t(locale, 'experts.detail.quickPrompts')}>
                      <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.quickPrompts')}</h4>
                      <ul className="m-0 flex list-none flex-col gap-[8px] p-0">
                        {detail.quick_prompts.map((prompt) => (
                          <li key={prompt.title} className="rounded-[8px] border border-[#e7e7ea] px-[12px] py-[10px]" data-expert-prompt>
                            <div className="flex items-center gap-[8px]">
                              <span aria-hidden="true" className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ background: dotColor(prompt.color) }} />
                              <span className="text-[14px] font-semibold leading-[20px]">{prompt.title}</span>
                            </div>
                            {prompt.description ? <p className="m-0 mt-[4px] text-[12px] leading-[18px] text-[rgba(23,26,29,0.6)]">{prompt.description}</p> : null}
                          </li>
                        ))}
                      </ul>
                    </section>
                  ) : null}
                </>
              ) : null}
            </div>

            <div className="flex shrink-0 items-center justify-between gap-[10px] border-t border-[rgba(127,127,127,0.2)] bg-[var(--wk-bg,#fff)] px-6 py-4">
              <label className="flex min-w-0 flex-1 flex-col gap-[4px] text-[12px] leading-[16px] text-[rgba(23,26,29,0.6)]">
                {t(locale, 'experts.create.nameLabel')}
                <input
                  className={XP_INPUT + ' w-full'}
                  value={agentName}
                  placeholder={t(locale, 'experts.create.namePlaceholder')}
                  onChange={(event) => setAgentName(event.target.value)}
                />
              </label>
              <button
                type="button"
                className={XP_BTN_PRIMARY + ' shrink-0'}
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
          className="fixed inset-0 z-[1000] flex justify-end bg-[rgba(0,0,0,0.4)]"
          onClick={(event) => { if (event.target === event.currentTarget) closeRemoteDetail(); }}
        >
          <aside
            className="flex h-full w-[420px] max-w-[90vw] flex-col bg-[var(--wk-bg,#fff)] shadow-[-4px_0_24px_rgba(0,0,0,0.12)]"
            role="dialog"
            aria-label={t(locale, 'experts.tabRemote')}
            data-skillset-detail={remoteSlug}
          >
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[rgba(127,127,127,0.2)] px-6 py-5">
              <h3 className="m-0 flex min-w-0 items-center text-[18px] font-semibold">
                {remoteDetail ? <span className="truncate">{remoteDetail.name || remoteDetail.slug}</span> : t(locale, 'experts.tabRemote')}
              </h3>
              <button
                type="button"
                className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border-none bg-[rgba(127,127,127,0.1)] text-[15px]"
                aria-label={t(locale, 'common.cancel')}
                onClick={closeRemoteDetail}
              >✕</button>
            </div>

            <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-6">
              {remoteDetailLoading ? <div role="status">{t(locale, 'common.loading')}</div> : null}
              {!remoteDetailLoading && remoteDetailError ? (
                <div className="flex flex-col items-start gap-[10px] text-[13px] text-[#d54941]" role="alert">
                  <span>{remoteDetailError}</span>
                  <button type="button" className={XP_BTN_OUTLINE} onClick={() => setRemoteDetailEpoch((current) => current + 1)}>{t(locale, 'common.retry')}</button>
                </div>
              ) : null}
              {!remoteDetailLoading && !remoteDetailError && remoteDetail ? (
                <>
                  <p className="m-0 text-[14px] leading-[22px] text-[rgba(23,26,29,0.6)]">{remoteDetail.description}</p>
                  <section aria-label={t(locale, 'experts.detail.skills')}>
                    <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.detail.skills')}</h4>
                    <div className="flex flex-wrap items-center gap-[6px]">
                      {remoteDetail.installed ? <span className={XP_CHIP}>{t(locale, 'experts.market.installedTag')}</span> : null}
                      {remoteDetail.skill_slugs.map((slug) => <span key={slug} className={XP_CHIP} title={slug}>{slug}</span>)}
                    </div>
                  </section>
                </>
              ) : null}
            </div>

            <div className="flex shrink-0 items-center justify-between gap-[10px] border-t border-[rgba(127,127,127,0.2)] bg-[var(--wk-bg,#fff)] px-6 py-4">
              <label className="flex min-w-0 flex-1 flex-col gap-[4px] text-[12px] leading-[16px] text-[rgba(23,26,29,0.6)]">
                {t(locale, 'experts.create.nameLabel')}
                <input
                  className={XP_INPUT + ' w-full'}
                  value={agentName}
                  placeholder={t(locale, 'experts.create.namePlaceholder')}
                  onChange={(event) => setAgentName(event.target.value)}
                />
              </label>
              <button
                type="button"
                className={XP_BTN_PRIMARY + ' shrink-0'}
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
          className="fixed inset-0 z-[1100] flex items-start justify-center bg-[rgba(0,0,0,0.4)] pt-[12vh]"
          onClick={(event) => { if (event.target === event.currentTarget) setPublishOpen(false); }}
        >
          <div
            className="flex max-h-[76vh] w-[480px] max-w-[92vw] flex-col rounded-md bg-[var(--wk-bg,#fff)] shadow-[0_8px_32px_rgba(0,0,0,0.18)]"
            role="dialog"
            aria-label={t(locale, 'experts.tenant.publishTitle')}
            data-expert-publish-dialog
          >
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[rgba(127,127,127,0.2)] px-6 py-5">
              <h3 className="m-0 text-[18px] font-semibold">{t(locale, 'experts.tenant.publishTitle')}</h3>
              <button
                type="button"
                className="flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-md border-none bg-[rgba(127,127,127,0.1)] text-[15px]"
                aria-label={t(locale, 'common.cancel')}
                onClick={() => setPublishOpen(false)}
              >✕</button>
            </div>

            <div className="flex flex-1 flex-col gap-5 overflow-y-auto p-6">
              <p className="m-0 text-[13px] leading-[19px] text-[rgba(23,26,29,0.6)]">{t(locale, 'experts.tenant.publishDesc')}</p>
              <section aria-label={t(locale, 'experts.tenant.publishAgentLabel')}>
                <h4 className={XP_SECTION_TITLE}>{t(locale, 'experts.tenant.publishAgentLabel')}</h4>
                {agentsError !== '' ? <p className="m-0 text-[13px] text-[#d54941]" role="alert">{agentsError}</p> : null}
                {allAgents === null && agentsError === '' ? <div role="status">{t(locale, 'common.loading')}</div> : null}
                {allAgents !== null && ownAgents.length === 0 ? <p className="m-0 text-[13px] text-[rgba(23,26,29,0.6)]">{t(locale, 'experts.tenant.publishAgentEmpty')}</p> : null}
                <ul className="m-0 flex list-none flex-col gap-[8px] p-0">
                  {ownAgents.map((agent) => {
                    const selected = publishAgentId === agent.id;
                    return (
                      <li key={agent.id}>
                        <button
                          type="button"
                          data-publish-agent={agent.id}
                          aria-pressed={selected ? 'true' : 'false'}
                          className={'flex w-full cursor-pointer flex-col gap-[4px] rounded-[10px] border px-[12px] py-[10px] text-left font-[inherit] ' + (selected ? 'border-[rgba(7,192,95,0.5)] bg-accent-wash' : 'border-[#e7e7ea] bg-surface')}
                          onClick={() => setPublishAgentId(agent.id)}
                        >
                          <span className="truncate text-[14px] font-semibold text-[rgba(23,26,29,0.92)]" title={agent.name}>{agent.name}</span>
                          {agent.description ? <span className="m-0 line-clamp-2 text-[12px] leading-[18px] text-[rgba(23,26,29,0.6)]">{agent.description}</span> : null}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </section>
              <label className="flex flex-col gap-[4px] text-[12px] leading-[16px] text-[rgba(23,26,29,0.6)]">
                {t(locale, 'experts.tenant.publishNameLabel')}
                <input
                  className={XP_INPUT + ' w-full'}
                  value={publishName}
                  placeholder={t(locale, 'experts.tenant.publishNamePlaceholder')}
                  onChange={(event) => setPublishName(event.target.value)}
                />
              </label>
              <label className="flex flex-col gap-[4px] text-[12px] leading-[16px] text-[rgba(23,26,29,0.6)]">
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

            <div className="flex shrink-0 items-center justify-end gap-[10px] border-t border-[rgba(127,127,127,0.2)] px-6 py-4">
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
          className="fixed inset-0 z-[1100] flex items-start justify-center bg-[rgba(0,0,0,0.4)] pt-[40vh]"
          onClick={(event) => { if (event.target === event.currentTarget) setUnpublishTarget(null); }}
        >
          <div
            className="w-[400px] max-w-[90vw] rounded-md bg-[var(--wk-bg,#fff)] p-4 shadow-[0_8px_32px_rgba(0,0,0,0.18)]"
            role="alertdialog"
            aria-label={t(locale, 'experts.tenant.unpublishTitle')}
            data-expert-unpublish-dialog
          >
            <div className="mb-2 text-[16px] font-semibold leading-6">{t(locale, 'experts.tenant.unpublishTitle')}</div>
            <p className="m-0 mb-[18px] text-[14px] leading-[22px] text-[rgba(23,26,29,0.6)]">{t(locale, 'experts.tenant.unpublishMessage', { name: unpublishTarget.name })}</p>
            <div className="flex justify-end gap-[24px]">
              <button type="button" className="cursor-pointer border-none bg-transparent p-0 font-[inherit] text-[14px] text-inherit" onClick={() => setUnpublishTarget(null)}>{t(locale, 'common.cancel')}</button>
              <button type="button" className="cursor-pointer border-none bg-transparent p-0 font-[inherit] text-[14px] font-medium text-[#d54941]" data-expert-unpublish-confirm disabled={unpublishing} onClick={() => void unpublishConfirmed()}>
                {unpublishing ? t(locale, 'common.loading') : t(locale, 'experts.tenant.unpublish')}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </main>
  );
}
