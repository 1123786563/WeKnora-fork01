// Octop M4 — skills market page (/platform/market).
//
// Skeleton follows ExpertsPage.tsx (props { client, role }, useState + load()
// + useEffect, formatMessage via the resolved locale, Tailwind class
// constants, fixed top-center role=status toast pill, right-side role=dialog
// aside); data comes from client.market.* (packages/api-client market.ts).
//
// Install-progress consumption mirrors the existing catalog-install flow in
// apps/web/src/settings/SkillSettingsPanel.tsx — REUSED, not reinvented:
// the market install (skill_market_service.go InstallMarketSkill) wraps
// RegisterCatalogFromArchive + InstallCatalogToConfigs, the exact pipeline
// the settings panel drives, and its 202 answer carries catalog_id plus the
// per-config skillIds (install_ids, in requested config order — the inverse
// of marketSkillInstallResult). Those (configId, skillId) pairs feed the SAME
// SSE subscription the panel uses per busy installation
// (client.sandbox.skills.followInstallEvents), while a 2.5s catalog-list
// poll (SKILL_POLL_INTERVAL_MS precedent) reconciles durable statuses —
// the panel's "stream closed early; the poll keeps the row fresh" fallback.
// Percent math is the shared domain helper installProgressPercent.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { MarketSkillSummary, SandboxConfigRecord, SkillCatalog, TenantPublishedSkill, WeKnoraClient } from '@weknora/api-client';
import { installProgressPercent, type SkillInstallProgressEvent } from '@weknora/domain/sandbox/skill-install';
import { formatMessage, isLocale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
import { createAgentMarketplaceApi } from '../agent-marketplace/agent-marketplace-api.ts';
import { TenantReleaseReview } from '../agent-marketplace/TenantReleaseReview.tsx';
import { backendLabelKey, compactSkillText, isNamedSandboxBackend, sandboxTargetLine } from '../configuration/management.ts';
import './market-u.css';

/** Matches WebTenantRole (platform/scope-runtime.ts); admin affordances gate on admin|owner like SkillSettingsPanel.canEdit. */
export type MarketRole = 'viewer' | 'contributor' | 'admin' | 'owner';

/** The registry's showcase kinds (packages/api-client market.ts rankingKinds). */
const RANKING_KINDS = ['hot', 'featured', 'newest', 'recommended', 'trending', 'paid'] as const;
type RankingKind = (typeof RANKING_KINDS)[number];

/** SkillSettingsPanel SKILL_POLL_INTERVAL_MS — the catalog-install poll cadence. */
const MARKET_POLL_INTERVAL_MS = 2500;
/** Debounce for the search box (GlobalCommandPalette live-search uses 350ms; a market query is a full listing round-trip, so the same order). */
const SEARCH_DEBOUNCE_MS = 300;

/* Tailwind v4 utility recipes shared across the page (ExpertsPage constants). */
const MK_PAGE = 'wk-page wk-mkt-mk-page';

const MK_HEADER = 'wk-mkt-mk-header';

const MK_TITLE = 'wk-mkt-mk-title';

const MK_SUBTITLE = 'wk-mkt-mk-subtitle';

const MK_TAB = 'wk-mkt-mk-tab aria-selected:border-accent aria-selected:bg-accent-wash aria-selected:text-accent';

const MK_GRID = 'wk-mkt-mk-grid';

const MK_CARD = 'wk-mkt-mk-card';

const MK_CARD_TITLE = 'wk-mkt-mk-card-title';

const MK_CARD_DESC = 'wk-mkt-mk-card-desc';

const MK_CHIP = 'wk-mkt-mk-chip';

const MK_STATE = 'wk-mkt-mk-state';

const MK_ERROR = 'wk-mkt-mk-error';

const MK_BTN_PRIMARY = 'wk-mkt-mk-btn-primary';

const MK_BTN_OUTLINE = 'wk-mkt-mk-btn-outline';

const MK_INPUT = 'wk-mkt-mk-input placeholder:text-[rgba(23,26,29,0.35)]';

const MK_SECTION_TITLE = 'wk-mkt-mk-section-title';

const MK_STALE = 'wk-mkt-mk-stale';

const MK_PROGRESS_TRACK = 'wk-mkt-mk-progress-track';


type ToastState = { tone: 'success' | 'warning' | 'error'; text: string } | null;

/** One per-sandbox-config row of an in-flight install (panel progressById row). */
interface InstallRowState {
  configId: string;
  configName: string;
  /** The skillId the 202 answer started on this config ('' when it never started). */
  skillId: string;
  /** Per-config failure carried by the 202 envelope's errors map. */
  startError?: string;
  /** True once the install-events stream reported its terminal frame (done/failed/detached). */
  terminal: boolean;
  /** Final durable outcome, reconciled from the terminal frame or the catalog-list poll. */
  status?: 'ready' | 'failed';
  /** Failure detail from the terminal frame (backend skill error). */
  errorText?: string;
}

/** One install started from the market or the tenant tab. */
interface InstallState {
  source: 'market' | 'tenant';
  catalogId: string;
  skillName: string;
  rows: InstallRowState[];
}

/** The card a drawer was opened for (before the 202 answer exists). */
type InstallTarget =
  | { kind: 'market'; skill: MarketSkillSummary }
  | { kind: 'tenant'; skill: TenantPublishedSkill };

interface MarketPageProps {
  client: WeKnoraClient;
  role: MarketRole;
}

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

/**
 * The 202 answer's install_ids are the started configs' skillIds in requested
 * config order (skill_market_service.go marketSkillInstallResult appends one
 * per configID that started); errors maps configId -> reason for the rest.
 * This is the inverse mapping: requested order, skipping failed configs.
 */
function marketInstallRows(requested: readonly string[], names: ReadonlyMap<string, string>, catalogId: string, installIds: readonly string[], errors: Record<string, string> | undefined): { catalogId: string; rows: InstallRowState[] } {
  let started = 0;
  const rows = requested.map((configId) => {
    const row: InstallRowState = {
      configId,
      configName: names.get(configId) ?? configId,
      skillId: '',
      terminal: false,
    };
    const failure = errors?.[configId];
    if (failure !== undefined) {
      row.startError = failure;
      return row;
    }
    row.skillId = installIds[started] ?? '';
    started += 1;
    return row;
  });
  return { catalogId, rows };
}

/** The tenant install answers the catalog-install shape directly: installs maps configId -> skillId. */
function tenantInstallRows(requested: readonly string[], names: ReadonlyMap<string, string>, installs: Record<string, string>, errors: Record<string, string> | undefined): InstallRowState[] {
  return requested.map((configId) => {
    const skillId = installs[configId] ?? '';
    return {
      configId,
      configName: names.get(configId) ?? configId,
      skillId,
      terminal: false,
      ...(errors?.[configId] === undefined ? {} : { startError: errors[configId] }),
    };
  });
}

export function MarketPage({ client, role }: MarketPageProps) {
  const locale = usePreferredLocale();
  const t = useCallback((key: string, values?: Record<string, string | number>): string =>
    formatMessage(isLocale(locale) ? locale : 'en-US', key, values), [locale]);
  const isAdmin = role === 'admin' || role === 'owner';
  const agentMarketplaceApi = useMemo(() => createAgentMarketplaceApi(client), [client]);

  const [tab, setTab] = useState<'market' | 'tenant'>('market');
  const [toast, setToast] = useState<ToastState>(null);

  // Market tab listing (search result or the selected ranking).
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [rankingKind, setRankingKind] = useState<RankingKind>('hot');
  const [listingEpoch, setListingEpoch] = useState(0);
  const [listing, setListing] = useState<{ results: MarketSkillSummary[]; stale: boolean } | null>(null);
  const [listingLoading, setListingLoading] = useState(true);
  const [listingError, setListingError] = useState('');

  // Tenant-internal tab (Viewer+ list; publish source for admins).
  const [tenantSkills, setTenantSkills] = useState<TenantPublishedSkill[] | null>(null);
  const [tenantLoading, setTenantLoading] = useState(false);
  const [tenantError, setTenantError] = useState('');
  const [workspaceCatalog, setWorkspaceCatalog] = useState<SkillCatalog[]>([]);
  const [tenantActionId, setTenantActionId] = useState('');

  // Install drawer: pick configs -> 202 -> per-config progress.
  const [installTarget, setInstallTarget] = useState<InstallTarget | null>(null);
  const [configs, setConfigs] = useState<SandboxConfigRecord[]>([]);
  const [configsLoading, setConfigsLoading] = useState(false);
  const [configsError, setConfigsError] = useState('');
  const [targetIds, setTargetIds] = useState<string[]>([]);
  const [installing, setInstalling] = useState(false);
  const [install, setInstall] = useState<InstallState | null>(null);
  // Live install-events frames per config (panel progressById/progressByConfig fan-out).
  const [progressByConfig, setProgressByConfig] = useState<Record<string, SkillInstallProgressEvent | undefined>>({});

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3000);
    return () => window.clearTimeout(timer);
  }, [toast]);

  // Debounce the search box.
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [query]);

  const activeQuery = debouncedQuery.trim();
  useEffect(() => {
    if (tab !== 'market') return undefined;
    let cancelled = false;
    setListingLoading(true);
    setListingError('');
    const request = activeQuery ? client.market.searchSkills(activeQuery) : client.market.rankings(rankingKind);
    void request.then((result) => {
      if (cancelled) return;
      setListing({ results: result.results, stale: result.stale });
    }).catch((cause: unknown) => {
      if (cancelled) return;
      setListing(null);
      setListingError(errorText(cause, t('market.loadFailed')));
    }).finally(() => {
      if (!cancelled) setListingLoading(false);
    });
    return () => { cancelled = true; };
  }, [tab, activeQuery, rankingKind, listingEpoch, client, t]);

  // Tenant-internal tab loads once per mount (Viewer+ listing; the workspace
  // catalog rides along for the admin publish affordances — the same loader the
  // SkillSettingsPanel uses, client.configuration.skills.catalog.list()).
  // tenantLoadedRef latches without re-running this effect (a state flag set in
  // its own effect body would cancel the first load via the cleanup);
  // tenantEpoch re-arms it for the retry button.
  const tenantLoadedRef = useRef(false);
  const [tenantEpoch, setTenantEpoch] = useState(0);
  useEffect(() => {
    if (tab !== 'tenant') return;
    tenantLoadedRef.current = true;
    let cancelled = false;
    setTenantLoading(true);
    setTenantError('');
    const load = async (): Promise<void> => {
      const [skillsResult, catalogResult] = await Promise.allSettled([
        client.market.tenantSkills(),
        isAdmin ? client.configuration.skills.catalog.list() : Promise.resolve([] as SkillCatalog[]),
      ]);
      if (cancelled) return;
      if (skillsResult.status === 'fulfilled') setTenantSkills(skillsResult.value.skills);
      else {
        setTenantSkills([]);
        setTenantError(errorText(skillsResult.reason, t('market.tenantLoadFailed')));
      }
      setWorkspaceCatalog(catalogResult.status === 'fulfilled' ? catalogResult.value : []);
      setTenantLoading(false);
    };
    void load().catch(() => {
      if (!cancelled) setTenantLoading(false);
    });
    return () => { cancelled = true; };
  }, [tab, tenantEpoch, isAdmin, client, t]);

  const reloadTenant = useCallback(async (): Promise<void> => {
    try {
      const [skillsResult, catalogResult] = await Promise.all([
        client.market.tenantSkills(),
        isAdmin ? client.configuration.skills.catalog.list() : Promise.resolve([] as SkillCatalog[]),
      ]);
      setTenantSkills(skillsResult.skills);
      setWorkspaceCatalog(catalogResult);
    } catch (cause) {
      setToast({ tone: 'error', text: errorText(cause, t('market.tenantLoadFailed')) });
    }
  }, [client, isAdmin, t]);

  // The drawer's sandbox-config loader — the same API the SkillSettingsPanel
  // uses (client.sandboxConfigurations.list()), filtered to named backends.
  // configsEpoch lets the error state retry the load.
  const [configsEpoch, setConfigsEpoch] = useState(0);
  useEffect(() => {
    if (!installTarget) return undefined;
    let cancelled = false;
    setConfigsLoading(true);
    setConfigsError('');
    setConfigs([]);
    void client.sandboxConfigurations.list().then((result) => {
      if (cancelled) return;
      const named = result.items.filter((record) => isNamedSandboxBackend(record.sandbox_type));
      setConfigs(named);
      // Default-select the single eligible config (AddSkillWizard defaultTargets precedent).
      setTargetIds(named.length === 1 ? [named[0]!.id] : []);
    }).catch((cause: unknown) => {
      if (cancelled) return;
      setConfigsError(errorText(cause, t('market.configsLoadFailed')));
    }).finally(() => {
      if (!cancelled) setConfigsLoading(false);
    });
    return () => { cancelled = true; };
  }, [installTarget, configsEpoch, client, t]);

  // Keep the live install readable from stable callbacks.
  const installRef = useRef<InstallState | null>(null);
  installRef.current = install;

  /**
   * Reconciles durable per-config statuses from the catalog list — the
   * panel's SKILL_POLL_INTERVAL_MS refresh: the market install registered
   * catalog_id, so its installations carry each targeted config's status.
   */
  const refreshStatuses = useCallback(async (): Promise<void> => {
    const current = installRef.current;
    if (!current || current.catalogId === '') return;
    try {
      const catalog = await client.configuration.skills.catalog.list();
      const item = catalog.find((entry) => entry.id === current.catalogId);
      if (!item) return;
      setInstall((state) => {
        if (!state || state.catalogId !== current.catalogId) return state;
        return {
          ...state,
          rows: state.rows.map((row) => {
            if (row.startError !== undefined || row.status !== undefined) return row;
            const installation = (item.installations ?? []).find((entry) => entry.sandboxConfigId === row.configId);
            if (installation === undefined || (installation.status !== 'ready' && installation.status !== 'failed')) return row;
            return { ...row, status: installation.status, ...(installation.status === 'failed' && installation.error ? { errorText: installation.error } : {}) };
          }),
        };
      });
    } catch {
      // Poll retries on the next tick; interim SSE frames keep the view warm.
    }
  }, [client]);

  // One install-events SSE per unresolved (configId, skillId) — the exact
  // subscription the panel fans out per busy installation. terminalSkillEvent
  // (sandbox_skill.go) carries the durable status on done/failed frames;
  // stage "detached" ends the stream without resolving the row, which stays
  // for the poll. pendingKey changes only when the unresolved set changes, so
  // progress frames never resubscribe.
  const pendingKey = install !== null
    ? install.rows
      .filter((row) => row.skillId !== '' && row.startError === undefined && row.status === undefined && !row.terminal)
      .map((row) => `${row.configId}\t${row.skillId}`)
      .join('\n')
    : '';
  useEffect(() => {
    if (pendingKey === '') return undefined;
    let active = true;
    const controllers = pendingKey.split('\n').map((entry) => {
      const separator = entry.indexOf('\t');
      const configId = entry.slice(0, separator);
      const skillId = entry.slice(separator + 1);
      const controller = new AbortController();
      void client.sandbox.skills.followInstallEvents(configId, skillId, (frame) => {
        if (!active) return;
        setProgressByConfig((current) => ({ ...current, [configId]: frame.event }));
        if (!frame.terminal) return;
        const failed = frame.event.stage === 'failed' || frame.event.status === 'failed';
        const detached = frame.event.stage === 'detached';
        setInstall((state) => {
          if (!state) return state;
          return {
            ...state,
            rows: state.rows.map((row) => row.configId !== configId ? row : {
              ...row,
              terminal: true,
              ...(failed ? { status: 'failed' as const, errorText: frame.event.log ?? frame.event.status } : {}),
              ...(!failed && frame.event.status === 'ready' ? { status: 'ready' as const } : {}),
            }),
          };
        });
        // done/failed with an off-enum status (or detached): the durable
        // statuses come from the catalog list, exactly like the panel.
        void refreshStatuses();
      }, controller.signal).catch(() => {
        // Stream closed early; the status poll keeps the row fresh.
      });
      return controller;
    });
    return () => {
      active = false;
      controllers.forEach((controller) => controller.abort());
    };
  }, [client, pendingKey, refreshStatuses]);

  const unresolvedCount = install !== null
    ? install.rows.filter((row) => row.startError === undefined && row.status === undefined).length
    : 0;
  // 2.5s catalog-list poll while anything is unresolved (panel catalogBusy poll).
  useEffect(() => {
    if (unresolvedCount === 0 || typeof window === 'undefined') return undefined;
    const timer = window.setInterval(() => void refreshStatuses(), MARKET_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [unresolvedCount, refreshStatuses]);

  function openInstall(target: InstallTarget): void {
    setProgressByConfig({});
    setInstall(null);
    setInstalling(false);
    setInstallTarget(target);
  }

  /**
   * Closing mid-progress drops the local view only — the install keeps
   * running server-side exactly like the panel's closed manage drawer; the
   * skills settings page owns ongoing-install tracking from there.
   */
  function closeDrawer(): void {
    setInstallTarget(null);
    setInstall(null);
    setProgressByConfig({});
  }

  async function startInstall(): Promise<void> {
    const target = installTarget;
    if (!target || installing || targetIds.length === 0) return;
    setInstalling(true);
    try {
      const names = new Map(configs.map((record) => [record.id, record.name]));
      const requested = [...targetIds];
      if (target.kind === 'market') {
        const result = await client.market.installSkill(target.skill.slug, requested);
        const { catalogId, rows } = marketInstallRows(requested, names, result.catalog_id, result.install_ids, result.errors);
        setInstall({ source: 'market', catalogId, skillName: target.skill.name || target.skill.slug, rows });
        const failed = Object.keys(result.errors ?? {}).length;
        if (failed > 0) setToast({ tone: 'warning', text: t('market.installPartial', { failed }) });
        else setToast({ tone: 'success', text: t('market.installAccepted') });
      } else {
        const result = await client.market.installTenantSkill(target.skill.catalog_id, requested);
        setInstall({
          source: 'tenant',
          catalogId: target.skill.catalog_id,
          skillName: target.skill.name,
          rows: tenantInstallRows(requested, names, result.installs, result.errors),
        });
        const failed = Object.keys(result.errors ?? {}).length;
        if (failed > 0) setToast({ tone: 'warning', text: t('market.installPartial', { failed }) });
        else setToast({ tone: 'success', text: t('market.installAccepted') });
      }
      setInstallTarget(null);
      void reloadTenant();
    } catch (cause) {
      setToast({ tone: 'error', text: t('market.installFailedMessage', { message: errorText(cause, t('market.installFailed')) }) });
    } finally {
      setInstalling(false);
    }
  }

  async function publishSkill(catalogId: string): Promise<void> {
    if (tenantActionId !== '') return;
    setTenantActionId(catalogId);
    try {
      await client.market.publishSkill(catalogId);
      setToast({ tone: 'success', text: t('market.publishAccepted') });
      await reloadTenant();
    } catch (cause) {
      setToast({ tone: 'error', text: t('market.publishFailedMessage', { message: errorText(cause, t('market.publishFailed')) }) });
    } finally {
      setTenantActionId('');
    }
  }

  async function unpublishSkill(catalogId: string): Promise<void> {
    if (tenantActionId !== '') return;
    setTenantActionId(catalogId);
    try {
      await client.market.unpublishSkill(catalogId);
      setToast({ tone: 'success', text: t('market.unpublishAccepted') });
      await reloadTenant();
    } catch (cause) {
      setToast({ tone: 'error', text: t('market.unpublishFailedMessage', { message: errorText(cause, t('market.unpublishFailed')) }) });
    } finally {
      setTenantActionId('');
    }
  }

  const drawerOpen = installTarget !== null || install !== null;
  const publishedIds = useMemo(() => new Set((tenantSkills ?? []).map((skill) => skill.catalog_id)), [tenantSkills]);
  const publishable = useMemo(
    () => workspaceCatalog.filter((entry) => !publishedIds.has(entry.id)).map((entry) => ({ id: entry.id, name: entry.name, version: entry.version ?? '', description: entry.description ?? '' })),
    [workspaceCatalog, publishedIds],
  );
  const okCount = install !== null ? install.rows.filter((row) => row.status === 'ready').length : 0;
  const failedCount = install !== null ? install.rows.filter((row) => row.status === 'failed' || row.startError !== undefined).length : 0;

  function configMetaLine(record: SandboxConfigRecord): string {
    const label = t(backendLabelKey(record.sandbox_type));
    const target = sandboxTargetLine(record);
    return target ? `${label} · ${target}` : label;
  }

  function rowPercent(row: InstallRowState): number {
    const status = row.status !== undefined ? row.status
      : row.startError !== undefined ? 'failed'
      : row.terminal ? 'ready'
      : 'installing';
    return installProgressPercent(progressByConfig[row.configId], status);
  }

  function skillCard(skill: MarketSkillSummary): React.ReactNode {
    return (
      <article key={skill.slug} data-market-card={skill.slug} className={MK_CARD}>
        <h3 className={MK_CARD_TITLE}>
          <span className="wk-mkt-1" title={skill.name || skill.slug}>{skill.name || skill.slug}</span>
          {skill.version ? <span className={MK_CHIP}>{skill.version}</span> : null}
        </h3>
        <p className={MK_CARD_DESC} title={skill.description}>{compactSkillText(skill.description)}</p>
        <div className="wk-mkt-2">
          <span className={MK_CHIP} title={skill.slug}>{skill.slug}</span>
          {isAdmin ? (
            <button
              type="button"
              className={MK_BTN_PRIMARY + ' wk-mkt-33'}
              data-market-install={skill.slug}
              disabled={drawerOpen}
              onClick={() => openInstall({ kind: 'market', skill })}
            >{t('market.install')}</button>
          ) : null}
        </div>
      </article>
    );
  }

  return (
    <main className={MK_PAGE}>
      <header className={MK_HEADER}>
        <div>
          <h2 className={MK_TITLE}>{t('market.title')}</h2>
          <p className={MK_SUBTITLE}>{t('market.subtitle')}</p>
        </div>
        <div role="tablist" aria-label={t('market.tabsLabel')} className="wk-mkt-3">
          <button type="button" role="tab" aria-selected={tab === 'market'} data-market-tab="market" className={MK_TAB} onClick={() => setTab('market')}>{t('market.tabMarket')}</button>
          <button type="button" role="tab" aria-selected={tab === 'tenant'} data-market-tab="tenant" className={MK_TAB} onClick={() => setTab('tenant')}>{t('market.tabTenant')}</button>
        </div>
      </header>

      {toast ? (
        <div
          role="status"
          className={'wk-mkt-34 ' + (toast.tone === 'success' ? 'wk-mkt-35' : toast.tone === 'warning' ? 'wk-mkt-36' : 'wk-mkt-37')}
        >{toast.text}</div>
      ) : null}

      {tab === 'market' ? (
        <>
          <div className="wk-mkt-4">
            <input
              type="search"
              className={MK_INPUT}
              aria-label={t('market.searchLabel')}
              placeholder={t('market.searchPlaceholder')}
              value={query}
              data-market-search
              onChange={(event) => setQuery(event.target.value)}
            />
            <div role="tablist" aria-label={t('market.rankingsLabel')} className="wk-mkt-5">
              {RANKING_KINDS.map((kind) => (
                <button
                  key={kind}
                  type="button"
                  role="tab"
                  aria-selected={activeQuery === '' && rankingKind === kind}
                  data-market-ranking={kind}
                  className={MK_TAB + ' wk-mkt-38'}
                  disabled={activeQuery !== ''}
                  onClick={() => setRankingKind(kind)}
                >{t(`market.rankings.${kind}`)}</button>
              ))}
            </div>
          </div>

          {listingLoading ? <div className={MK_STATE} role="status">{t('common.loading')}</div> : null}
          {!listingLoading && listingError ? (
            <div className={MK_ERROR} role="alert">
              <span>{listingError}</span>
              <button type="button" className={MK_BTN_OUTLINE} onClick={() => setListingEpoch((current) => current + 1)}>{t('common.retry')}</button>
            </div>
          ) : null}
          {!listingLoading && !listingError && listing ? (
            <>
              {listing.stale ? <div className={MK_STALE} role="status" data-testid="market-stale-banner">{t('market.staleBanner')}</div> : null}
              {listing.results.length === 0 ? <div className={MK_STATE}>{activeQuery !== '' ? t('market.noResults') : t('market.empty')}</div> : (
                <div className={MK_GRID}>{listing.results.map(skillCard)}</div>
              )}
            </>
          ) : null}
        </>
      ) : (
        <>
          <TenantReleaseReview api={agentMarketplaceApi} role={role} />
          {tenantLoading ? <div className={MK_STATE} role="status">{t('common.loading')}</div> : null}
          {!tenantLoading && tenantError ? (
            <div className={MK_ERROR} role="alert">
              <span>{tenantError}</span>
              <button type="button" className={MK_BTN_OUTLINE} onClick={() => { setTenantError(''); setTenantEpoch((current) => current + 1); }}>{t('common.retry')}</button>
            </div>
          ) : null}
          {!tenantLoading && !tenantError && tenantSkills !== null ? (
            <>
              <h3 className={MK_SECTION_TITLE}>{t('market.tenantTitle')}</h3>
              {tenantSkills.length === 0 ? <div className={MK_STATE}>{t('market.tenantEmpty')}</div> : (
                <div className={MK_GRID + ' wk-mkt-39'}>
                  {tenantSkills.map((skill) => (
                    <article key={skill.catalog_id} data-tenant-skill={skill.catalog_id} className={MK_CARD}>
                      <h3 className={MK_CARD_TITLE}>
                        <span className="wk-mkt-1" title={skill.name}>{skill.name}</span>
                        {skill.version ? <span className={MK_CHIP}>{skill.version}</span> : null}
                      </h3>
                      {skill.description ? <p className={MK_CARD_DESC} title={skill.description}>{compactSkillText(skill.description)}</p> : null}
                      <div className="wk-mkt-2">
                        {skill.installed ? <span className={MK_CHIP}>{t('market.tenantInstalledTag')}</span> : null}
                        {skill.publisher_name ? <span className={MK_CHIP}>{t('market.publisher', { name: skill.publisher_name })}</span> : null}
                        <span className="wk-mkt-6">
                          {isAdmin ? (
                            <>
                              <button type="button" className={MK_BTN_PRIMARY} data-tenant-install={skill.catalog_id} disabled={drawerOpen} onClick={() => openInstall({ kind: 'tenant', skill })}>{t('market.install')}</button>
                              <button type="button" className={MK_BTN_OUTLINE} data-tenant-unpublish={skill.catalog_id} disabled={tenantActionId === skill.catalog_id} onClick={() => void unpublishSkill(skill.catalog_id)}>{t('market.unpublish')}</button>
                            </>
                          ) : null}
                        </span>
                      </div>
                    </article>
                  ))}
                </div>
              )}
              {isAdmin ? (
                <section aria-label={t('market.publishSectionTitle')}>
                  <h3 className={MK_SECTION_TITLE}>{t('market.publishSectionTitle')}</h3>
                  <p className="wk-mkt-7">{t('market.publishSectionHint')}</p>
                  {publishable.length === 0 ? <div className={MK_STATE}>{t('market.publishSourceEmpty')}</div> : (
                    <ul className="wk-mkt-8">
                      {publishable.map((entry) => (
                        <li key={entry.id} data-tenant-publishable={entry.id} className="wk-mkt-9">
                          <span className="wk-mkt-10" title={entry.name}>{entry.name}</span>
                          {entry.version ? <span className={MK_CHIP}>{entry.version}</span> : null}
                          <button type="button" className={MK_BTN_OUTLINE + ' wk-mkt-33'} data-tenant-publish={entry.id} disabled={tenantActionId === entry.id} onClick={() => void publishSkill(entry.id)}>{t('market.publish')}</button>
                        </li>
                      ))}
                    </ul>
                  )}
                </section>
              ) : null}
            </>
          ) : null}
        </>
      )}

      {drawerOpen ? (
        <div
          className="wk-mkt-11"
          onClick={(event) => { if (event.target === event.currentTarget) closeDrawer(); }}
        >
          <aside
            className="wk-mkt-12"
            role="dialog"
            aria-label={t('market.installTitle')}
            data-market-drawer
          >
            <div className="wk-mkt-13">
              <h3 className="wk-mkt-14">
                <span className="wk-mkt-1">{installTarget ? (installTarget.kind === 'market' ? installTarget.skill.name || installTarget.skill.slug : installTarget.skill.name) : install?.skillName}</span>
                <span className="wk-mkt-15">{t('market.installTitle')}</span>
              </h3>
              <button
                type="button"
                className="wk-mkt-16"
                aria-label={t('common.cancel')}
                onClick={closeDrawer}
              >✕</button>
            </div>

            <div className="wk-mkt-17">
              {installTarget ? (
                <>
                  <p className="wk-mkt-18">{t('market.installDesc')}</p>
                  {configsLoading ? <div role="status">{t('common.loading')}</div> : null}
                  {configsError ? (
                    <div className="wk-mkt-19" role="alert">
                      <span>{configsError}</span>
                      <button type="button" className={MK_BTN_OUTLINE} onClick={() => setConfigsEpoch((current) => current + 1)}>{t('common.retry')}</button>
                    </div>
                  ) : null}
                  {!configsLoading && !configsError && configs.length === 0 ? <p className="wk-mkt-20">{t('market.installNoConfigs')}</p> : null}
                  {!configsLoading && !configsError && configs.length > 0 ? (
                    <ul className="wk-mkt-8">
                      {configs.map((record) => {
                        const checked = targetIds.includes(record.id);
                        return (
                          <li key={record.id}>
                            <label className={'wk-mkt-40 ' + (checked ? 'wk-mkt-41' : 'wk-mkt-42')}>
                              <input
                                type="checkbox"
                                data-market-config={record.id}
                                checked={checked}
                                onChange={(event) => setTargetIds((current) => event.target.checked ? [...new Set([...current, record.id])] : current.filter((id) => id !== record.id))}
                              />
                              <span className="wk-mkt-21">
                                <span className="wk-mkt-22" title={record.name}>{record.name}</span>
                                <span className="wk-mkt-23">{configMetaLine(record)}</span>
                              </span>
                            </label>
                          </li>
                        );
                      })}
                    </ul>
                  ) : null}
                </>
              ) : install !== null ? (
                <>
                  <h4 className="wk-mkt-24">{t('market.progressTitle')}</h4>
                  {unresolvedCount === 0 ? (
                    <div className="wk-mkt-25" data-testid="market-install-done" role="status">
                      <span className="wk-mkt-26">{failedCount > 0 ? t('market.doneWithFailures') : t('market.done')}</span>
                      <span className="wk-mkt-27">{t('market.doneSummary', { ok: okCount, failed: failedCount })}</span>
                    </div>
                  ) : null}
                  <ul className="wk-mkt-28">
                    {install.rows.map((row) => {
                      const percent = rowPercent(row);
                      const progress = progressByConfig[row.configId];
                      const failed = row.status === 'failed';
                      const ready = row.status === 'ready';
                      const failedToStart = row.startError !== undefined;
                      return (
                        <li key={row.configId} data-market-install-row={row.configId} className="wk-mkt-29">
                          <div className="wk-mkt-3">
                            <span className="wk-mkt-22" title={row.configName}>{row.configName}</span>
                            <span className={'wk-mkt-43 ' + (failed || failedToStart ? 'wk-mkt-44' : ready ? 'wk-mkt-45' : 'wk-mkt-46')}>
                              {failedToStart ? t('market.rowFailedToStart') : failed ? t('market.rowFailed') : ready ? t('market.rowReady') : `${percent}%`}
                            </span>
                          </div>
                          {!failedToStart ? (
                            <div className={MK_PROGRESS_TRACK} role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-label={row.configName}>
                              <div className={'wk-mkt-47 ' + (failed ? 'wk-mkt-48' : 'wk-mkt-49')} style={{ width: `${percent}%` }} />
                            </div>
                          ) : null}
                          {progress && !ready && !failed ? (
                            <p className="wk-mkt-30" title={progress.log ?? progress.stage}>{progress.log ?? progress.stage}</p>
                          ) : null}
                          {failedToStart ? <p className="wk-mkt-31" role="alert">{row.startError}</p> : null}
                          {failed && (row.errorText ?? '') !== '' ? <p className="wk-mkt-31" role="alert">{row.errorText}</p> : null}
                        </li>
                      );
                    })}
                  </ul>
                </>
              ) : null}
            </div>

            {installTarget ? (
              <div className="wk-mkt-32">
                <button type="button" className={MK_BTN_OUTLINE} onClick={closeDrawer}>{t('common.cancel')}</button>
                <button type="button" className={MK_BTN_PRIMARY} data-market-install-confirm disabled={installing || targetIds.length === 0} onClick={() => void startInstall()}>
                  {installing ? t('market.installing') : t('market.installConfirm')}
                </button>
              </div>
            ) : (
              <div className="wk-mkt-32">
                <button type="button" className={MK_BTN_PRIMARY} data-market-install-close onClick={closeDrawer}>{t('common.cancel')}</button>
              </div>
            )}
          </aside>
        </div>
      ) : null}
    </main>
  );
}
