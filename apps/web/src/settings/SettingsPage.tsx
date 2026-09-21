import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { isCapabilitySupported, type CapabilityMap } from '@weknora/domain';
import { integrationTabForSection, integrationSettingsQuery, selectSettingsQuery } from '@weknora/views/integrations/settings-route';
import { INTEGRATION_SECTIONS } from '@weknora/views/integrations/registry';
import { openContextualGuide } from '@weknora/views/guides/contextual-guides';
const IntegrationsRoutePage = lazy(() => import('../integrations/IntegrationsRoutePage.tsx').then((m) => ({ default: m.IntegrationsRoutePage })));
import type { WeKnoraClient } from '@weknora/api-client';
import type { SettingsRole } from '@weknora/views/settings/registry';
import { roleAtLeast, SETTINGS_SECTIONS, settingsSectionsForRole } from '@weknora/views/settings/registry';
import { Button, Status, Alert } from '@weknora/ui';
import { pushSettingsToast, SettingsToastHost } from './settings-toast.tsx';
import { modelFormatMessage } from './model-settings.ts';
import { navigate } from '../platform/navigation.ts';
import { profilePasswordPatch, settingsCloseMode, settingsSectionHeading, settingsSectionMeta, tenantEditState, tenantPatch } from './surface.ts';
const TenantDeleteZone = lazy(() => import('./TenantDeleteZone.tsx').then((m) => ({ default: m.TenantDeleteZone })));
const MemoryWorkspacePanel = lazy(() => import('./PersonalMemoryPanel.tsx').then((m) => ({ default: m.MemoryWorkspacePanel })));
const PersonalMemorySettingsPanel = lazy(() => import('./PersonalMemorySettingsPanel.tsx').then((m) => ({ default: m.PersonalMemorySettingsPanel })));
const ResourceSettingsPanel = lazy(() => import('./ResourceSettingsPanel.tsx').then((m) => ({ default: m.ResourceSettingsPanel })));
import type { SettingsModelOption } from './ConfigSettingsPanel.tsx';
const ConfigSettingsPanel = lazy(() => import('./ConfigSettingsPanel.tsx').then((m) => ({ default: m.ConfigSettingsPanel })));
const OllamaSettingsPanel = lazy(() => import('./OllamaSettingsPanel.tsx').then((m) => ({ default: m.OllamaSettingsPanel })));
const ParserEngineSettingsPanel = lazy(() => import('./ParserEngineSettingsPanel.tsx').then((m) => ({ default: m.ParserEngineSettingsPanel })));
const CloudSettingsPanel = lazy(() => import('./CloudSettingsPanel.tsx').then((m) => ({ default: m.CloudSettingsPanel })));
const EnvVarSettingsPanel = lazy(() => import('./EnvVarSettingsPanel.tsx').then((m) => ({ default: m.EnvVarSettingsPanel })));
import { LiveSectionsPanel, PortedSectionsPanel, readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
const McpSettingsPanel = lazy(() => import('./McpSettingsPanel.tsx').then((m) => ({ default: m.McpSettingsPanel })));
const ModelSettingsPanel = lazy(() => import('./ModelSettingsPanel.tsx').then((m) => ({ default: m.ModelSettingsPanel })));
const SandboxSettingsPanel = lazy(() => import('./SandboxSettingsPanel.tsx').then((m) => ({ default: m.SandboxSettingsPanel })));
const SkillSettingsPanel = lazy(() => import('./SkillSettingsPanel.tsx').then((m) => ({ default: m.SkillSettingsPanel })));
const TenantMembersPanel = lazy(() => import('./TenantMembersPanel.tsx').then((m) => ({ default: m.TenantMembersPanel })));
const GeneralPreferencesPanel = lazy(() => import('./GeneralPreferencesPanel.tsx').then((m) => ({ default: m.GeneralPreferencesPanel })));
// SP14 Task 4 — 会话偏好分区（默认对话模型，user-scope preferences 读写）。
const ChatPreferencesPanel = lazy(() => import('./ChatPreferencesPanel.tsx').then((m) => ({ default: m.ChatPreferencesPanel })));
const UsagePanel = lazy(() => import('./UsagePanel.tsx').then((m) => ({ default: m.UsagePanel })));
const QueryHistoryPanel = lazy(() => import('./QueryHistoryPanel.tsx').then((m) => ({ default: m.QueryHistoryPanel })));
const TenantInfoSection = lazy(() => import('./TenantUserProfileSections.tsx').then((m) => ({ default: m.TenantInfoSection })));
const UserProfileSection = lazy(() => import('./TenantUserProfileSections.tsx').then((m) => ({ default: m.UserProfileSection })));
const SystemInfoPanel = lazy(() => import('./SystemInfoPanel.tsx').then((m) => ({ default: m.SystemInfoPanel })));
const RuntimeQueuesPanel = lazy(() => import('./RuntimeQueuesPanel.tsx').then((m) => ({ default: m.RuntimeQueuesPanel })));
import { SystemGlobalSettingsPanel } from './SystemGlobalSettingsPanel.tsx';
const PlatformApiKeysPanel = lazy(() => import('./PlatformApiKeysPanel.tsx').then((m) => ({ default: m.PlatformApiKeysPanel })));
const SystemAuditLogPanel = lazy(() => import('./SystemAuditLogPanel.tsx').then((m) => ({ default: m.SystemAuditLogPanel })));
import './settings-wrapper.css';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

/*
 * R472 A2 — settings 分区错误态 UX 模式（对齐 .omc/state/r470/report-A3.md
 * 锚定的 Vue 模式；R481 A1 按 R480 浏览器锚定修订，证据
 * docs/migrations/react/evidence/vue-react-parity/2026-09-19-r480-settings-error-anchoring.md）：
 * - 'toast-keep'   models：Toast 本地化「加载模型列表失败」+ 界面保持
 *                  （ModelSettings.vue:488-490，骨架/默认态不清空）。
 * - 'toast-retry'  skills/mcp：Toast + 中央空态 + 重试按钮；面板自加载并
 *                  自行呈现错误（SkillSettings.vue:1161-1164、
 *                  McpSettings.vue:144-147），中央读取失败不再顶替内容区。
 * - 'banner-retry' members/parser/system/userprofile：浅红横幅透传后端原文 +
 *                  重试。R480 锚定：ParserEngineSettings.vue:14-21、
 *                  SystemInfo.vue:13-20、UserProfile.vue:14-21 均为
 *                  v-else-if="error" 的内容替换形态（重试文案 zh-CN 均为「重试」）。
 *                  members 的横幅由面板自加载渲染（标题在面板内部）；
 *                  parser/system/userprofile 的横幅在壳层渲染并替代内容
 *                  （标题由壳层 heading 保留）。
 * - 'silent'       storage/vectorstore/websearch/weknoracloud/ollama/retrieval：
 *                  R480 对称 500 拦截证实 Vue 完全静默降级——空列表/默认表单，
 *                  无横幅、无 toast、无重试（StorageBackendSettings.vue 渲染
 *                  空列表+添加按钮；retrieval-config boot 预取失败被静默吞掉）。
 *                  面板以 null payload 渲染；ollama/retrieval 的面板自身
 *                  错误态由面板负责（页级不出错误 UI）。
 * - 'inline'       其余分区维持裸 Status 行为（本轮未对齐范围）。
 * 共同点：错误态下分区标题保持渲染（R470 缺陷 4）。
 */
export type SettingsSectionErrorMode = 'inline' | 'toast-keep' | 'toast-retry' | 'banner-retry' | 'silent';

export function sectionErrorMode(key: string): SettingsSectionErrorMode {
  if (key === 'models') return 'toast-keep';
  if (key === 'skills' || key === 'mcp') return 'toast-retry';
  if (key === 'members' || key === 'parser' || key === 'system' || key === 'userprofile') return 'banner-retry';
  if (key === 'storage' || key === 'vectorstore' || key === 'websearch' || key === 'weknoracloud' || key === 'ollama' || key === 'retrieval') return 'silent';
  return 'inline';
}

const PARTIALLY_PORTED_SECTIONS = new Set(['models', 'members', 'mcp', 'sandbox', 'skills', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log']);
const SYSTEM_ADMIN_SECTIONS = new Set(['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log']);
// Keep the ordinary settings sections behind the same deployment gates as
// Settings.vue's SETTINGS_SECTION_CAPABILITY map. Integrations are described
// by their registry; these entries have no integration-tab counterpart.
const SETTINGS_SECTION_CAPABILITIES: Readonly<Record<string, string | undefined>> = {
  websearch: 'settings.websearch',
  vectorstore: 'settings.vectorstore',
  storage: 'settings.storage',
  sandbox: 'settings.sandbox',
  skills: 'settings.sandbox',
  envvars: 'settings.sandbox',
  mcp: 'settings.mcp',
};

export async function readSettingsSection(client: WeKnoraClient, key: string, tenantId: number): Promise<unknown> {
  switch (key) {
    case 'general': return client.settings.preferences.get();
    // SP14 Task 4 — the chat-preferences panel echoes the user preferences
    // payload (default_model) from /auth/me like the general section.
    case 'chat-preferences': return client.settings.preferences.get();
    case 'tenant': return client.settings.tenant.get();
    case 'userprofile': return client.settings.profile.get();
    // Vue OllamaSettings keeps the page mounted when either probe fails —
    // status degrades to 不可用 and the list to empty, never an inline error.
    case 'ollama': return Promise.all([
      client.settings.ollama.status().catch(() => ({ available: false })),
      client.settings.ollama.models().catch(() => []),
    ]).then(([status, models]) => ({ status, models }));
    // Vue ParserEngineSettings loads engines/config/wkc itself on mount.
    case 'parser': return Promise.resolve(null);
    // The usage panel self-fetches client.usage.my + commercial.summary for
    // its date window (parser precedent: no shell-level read needed).
    case 'usage': return Promise.resolve(null);
    // The query-history panel self-fetches the KV privacy config first
    // (disabled short-circuits the audit listing) and then
    // client.queryHistory.adminList for its own filters (usage precedent).
    case 'query-history': return Promise.resolve(null);
    case 'retrieval': return client.settings.retrieval.get();
    case 'memory': return client.settings.memory.workspace.get();
    // Vue mounts MemorySettings.vue (personal surface) under "mymemory"; the
    // panel fetches lists/counts client-side like the Vue source does.
    case 'mymemory': return client.settings.memory.personal.settings();
    case 'envvars': return client.settings.envVars.list();
    case 'storage': return Promise.all([client.settings.storage.backends.list(), client.settings.storage.legacy.status()]).then(([backends, legacy]) => ({ backends, legacy }));
    case 'vectorstore': return client.settings.vectorStores.list();
    case 'websearch': return client.settings.webSearch.providers.list();
    case 'chathistory': return Promise.all([client.settings.chatHistory.config.get(), client.settings.chatHistory.stats()]).then(([config, stats]) => ({ config, stats }));
    case 'system': return client.settings.system.info();
    case 'weknoracloud': return client.settings.weknoraCloud.status();
    case 'models': return client.configuration.models.list();
    case 'members': return client.identity.tenants.members.list(tenantId, { pageSize: 50 });
    case 'mcp': return client.configuration.mcp.list();
    case 'skills': return client.configuration.skills.list();
    case 'sandbox': return client.sandboxConfigurations.list();
    case 'system-global': return client.administration.settings.list();
    case 'runtime-queues': return client.administration.runtime.queues();
    case 'platform-api-keys': return client.administration.apiKeys.list();
    case 'system-audit-log': return client.administration.auditLog.list({ limit: 50 });
    default: throw new Error(`No read operation is registered for settings section: ${key}`);
  }
}

function requestedSection(search: string): string {
  const requested = integrationSettingsQuery(search).get('section');
  return requested && settingsSectionMeta(requested) ? requested : SETTINGS_SECTIONS[0]!.key;
}

// Vue keeps the model sub-tab in uiStore.settingsInitialSubSection
// (openSettings(section, subSection); ModelSettings.vue lines 329-337). The
// React shell has no pinia store, so the same value travels as the
// subsection query companion of the section query param.
function requestedSubSection(search: string): string | null {
  const value = new URLSearchParams(search).get("subsection");
  return value && value.trim() ? value.trim() : null;
}


export function SettingsPage({ client, tenantId, role = 'owner', capabilities = {}, liteMode = false }: { client: WeKnoraClient; tenantId: number; role?: SettingsRole; capabilities?: CapabilityMap; liteMode?: boolean }) {
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const [selectedKey, setSelectedKey] = useState(() => requestedSection(window.location.search));
  // Per-section payload cache: revisiting a seen section renders its cached
  // data instantly and refreshes silently. The previous single-slot payload
  // forced "old data → 加载中 → content" through three paints on every tab
  // switch, which read as a flicker.
  const [payloadCache, setPayloadCache] = useState<Record<string, unknown>>({});
  const payload = selectedKey in payloadCache ? payloadCache[selectedKey]! : null;
  const [models, setModels] = useState<readonly SettingsModelOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const modalRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const openerRef = useRef<HTMLElement | null>(null);
  const mountedRef = useRef(false);
  const loadGenerationRef = useRef(0);
  const section = settingsSectionMeta(selectedKey)!;
  const sectionSupported = (key: string) => {
    const integrationCapability = INTEGRATION_SECTIONS.find((item) => item.key === integrationTabForSection(key))?.capability;
    return isCapabilitySupported(capabilities, integrationCapability ?? SETTINGS_SECTION_CAPABILITIES[key], { liteMode });
  };
  const integrationTab = integrationTabForSection(selectedKey);
  const visibleSections = settingsSectionsForRole(role).filter((item) => sectionSupported(item.key));
  const roleDenied = !roleAtLeast(role, section.minRole);

  async function load(force = false) {
    const generation = ++loadGenerationRef.current;
    const isCurrentLoad = () => generation === loadGenerationRef.current;
    if (!sectionSupported(selectedKey) || integrationTab || roleDenied) { setError(null); setLoading(false); return; }
    // 首访（无缓存）与保存后的强制刷新才显示加载占位；普通切 tab 走静默刷新，
    // 已渲染的面板保持不动，数据到达后静默更新。
    const hasCache = selectedKey in payloadCache;
    if (!hasCache || force) setLoading(true);
    setError(null); setNotice(null);
    try {
      const next = await readSettingsSection(client, selectedKey, tenantId);
      if (!isCurrentLoad()) return;
      setPayloadCache((prev) => ({ ...prev, [selectedKey]: next }));
      if (selectedKey === 'retrieval' || selectedKey === 'chathistory') {
        try {
          const nextModels = await client.configuration.models.list();
          if (isCurrentLoad()) setModels(nextModels);
        } catch {
          if (isCurrentLoad()) setModels([]);
        }
      }
    }
    catch (reason) {
      if (!isCurrentLoad()) return;
      // 有缓存时静默保留旧数据；仅无缓存的失败才进入分区错误态。
      if (!hasCache) {
        const mode = sectionErrorMode(selectedKey);
        if (mode === 'toast-keep') {
          // Vue ModelSettings.vue:488-490 — Toast 本地化「加载模型列表失败」，
          // 面板保持渲染（默认空态），不透传后端原文、不清空骨架。
          setError(null);
          pushSettingsToast(modelFormatMessage(locale, 'model.editor.loadModelListFailed'));
        } else if (mode === 'toast-retry' || selectedKey === 'members' || mode === 'silent') {
          // skills/mcp/members 面板自加载并渲染各自的 Vue 对齐错误态
          // （toast+空态+重试 / 横幅+重试）；中央失败不得顶替内容区，
          // 面板以 undefined 初始数据自拉。
          // silent（R480 基线）：storage/vectorstore/websearch/weknoracloud/
          // ollama/retrieval 在 Vue 500 下完全静默降级（空列表/默认表单）——
          // 无横幅、无 toast、无重试；面板以 null payload 渲染。
          setError(null);
        } else if (mode === 'banner-retry') {
          // parser/system/userprofile（R480 锚定）：壳层横幅透传后端原文 +
          // 重试，替换内容区（Vue SystemInfo.vue:13-20、UserProfile.vue:14-21、
          // ParserEngineSettings.vue:14-21 的 t-alert theme=error 形态）。
          setError(errorText(reason, t('common.error')));
        } else {
          setError(errorText(reason, t('common.error')));
        }
      }
    }
    finally {
      if (isCurrentLoad()) setLoading(false);
    }
  }

  useEffect(() => { void load(); }, [client, selectedKey, role]);
  // Tenant-models contextual tour on the models settings entry (documentKb
  // variant): when the drawer opens on 模型配置 and the tenant has no models
  // configured yet, arm the tour. The Vue baseline only auto-arms it from
  // AgentList (agent variant); this settings-entry trigger gives the
  // documentKb variant its React entry point and fires once (dismissal
  // persists, the shell host applies the welcome-tour gate).
  useEffect(() => {
    if (selectedKey !== 'models' || integrationTab) return;
    if (Array.isArray(payload) && payload.length === 0) openContextualGuide('tenantModels');
  }, [integrationTab, payload, selectedKey]);
  useEffect(() => {
    const next = sectionSupported(selectedKey) ? selectedKey : 'general';
    if (next === selectedKey) return;
    if (next !== selectedKey) { setSelectedKey(next); setNotice(t('settings.capabilityUnavailable')); }
    const query = selectSettingsQuery(next, window.location.search);
    window.history.replaceState(null, '', '/platform/settings?' + query);
  }, [selectedKey, capabilities, liteMode]);
  useEffect(() => {
    // Vue normalizes the settings entry to the explicit default subsection;
    // preserve that deep-link/history contract for direct `/platform/settings`.
    if (new URLSearchParams(window.location.search).has('section')) return;
    window.history.replaceState(null, '', '/platform/settings?section=general');
  }, []);
  useEffect(() => {
    function onPopState() { setSelectedKey(requestedSection(window.location.search)); }
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') closeSettings();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
  useEffect(() => {
    if (mountedRef.current && document.activeElement instanceof HTMLElement) document.activeElement.blur();
    mountedRef.current = true;
  }, [selectedKey]);

  useEffect(() => {
    openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButtonRef.current?.focus();
    return () => {
      if (openerRef.current && document.contains(openerRef.current)) openerRef.current.focus();
      openerRef.current = null;
    };
  }, []);

  function handleDialogKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (event.key !== 'Tab' || !modalRef.current || !modalRef.current.contains(event.target as Node)) return;
    const focusable = Array.from(modalRef.current.querySelectorAll<HTMLElement>(
      'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
    ));
    if (focusable.length === 0) {
      event.preventDefault();
      modalRef.current.focus();
      return;
    }
    const currentIndex = focusable.indexOf(document.activeElement as HTMLElement);
    const nextIndex = event.shiftKey
      ? (currentIndex <= 0 ? focusable.length - 1 : currentIndex - 1)
      : (currentIndex === focusable.length - 1 ? 0 : currentIndex + 1);
    if (currentIndex === -1 || nextIndex !== currentIndex + (event.shiftKey ? -1 : 1)) {
      event.preventDefault();
      focusable[nextIndex]?.focus();
    }
  }

  // Settings.vue blurs the active control before closing the drawer. Keep the
  // same lifecycle for both the close button and Escape so focused controls do
  // not survive the route transition as detached elements.
  function closeSettings() {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    if (settingsCloseMode(window.location.search) === 'knowledge-bases') {
      navigate('/platform/knowledge-bases');
    } else {
      window.history.back();
    }
  }

  // Read once on mount; the tab then follows the drawer until the user
  // changes it (Vue consumes settingsInitialSubSection the same way).
  const initialSubSection = useState(() => requestedSubSection(window.location.search))[0];

  const select = useCallback((key: string) => {
    const next = settingsSectionMeta(key) ? key : SETTINGS_SECTIONS[0]!.key;
    setSelectedKey(next);
    // Section changes are state within the settings drawer, not new pages.
    // Keep one history entry for opening settings so Close returns to the
    // route that opened it even after several section changes.
    navigate('/platform/settings?' + selectSettingsQuery(next, window.location.search), 'replace');
  }, []);

  // Keep-alive: once a section has been opened its panel stays mounted and is
  // merely hidden on switch. Panels re-run their own requests on every
  // remount (skills need catalog + sandbox configs; memory/ollama fetch
  // internally), which read as a flicker on every revisit. Keeping them alive
  // also preserves in-panel form state; everything unmounts when the drawer
  // closes (the route leaves /platform/settings).
  const [visitedSections, setVisitedSections] = useState<string[]>(() => [selectedKey]);
  useEffect(() => {
    setVisitedSections((prev) => (prev.includes(selectedKey) ? prev : [...prev, selectedKey]));
  }, [selectedKey]);

  function renderSectionPanel(key: string): ReactNode {
    const section = settingsSectionMeta(key)!;
    const sectionPayload = key in payloadCache ? payloadCache[key]! : null;
    const sectionIntegrationTab = integrationTabForSection(key);
    const sectionRoleDenied = !roleAtLeast(role, section.minRole);
    const isActive = key === selectedKey;
    const sectionError = isActive ? error : null;
    const sectionLoading = isActive && loading;
    const sectionDenied = sectionRoleDenied
      ? <div data-testid="role-denied-panel"><Status tone="error">{t('settings.roleDenied.title')}</Status><p className="wk-muted text-muted">{t('settings.roleDenied.desc')}</p></div>
      : null;
    // SP14 Task 1 — general 分区传入 client：GeneralPreferencesPanel 顶部套餐
    // 卡片用它拉 client.commercial.summary()（失败静默隐藏整卡）。
    const generalPanel = key === 'general' ? <GeneralPreferencesPanel liteMode={liteMode} client={client} /> : null;
    // SP14 Task 4 — 会话偏好面板：壳层只递 preferences 回显载荷，保存走面板
    // 自己的 client.settings.preferences.update（无整页刷新）。
    const chatPreferencesPanel = key === 'chat-preferences' ? <ChatPreferencesPanel client={client} initialPreferences={sectionPayload} /> : null;
    const resourcePanel = key === 'storage' || key === 'vectorstore' || key === 'websearch'
      ? <ResourceSettingsPanel client={client} section={key} initialValue={sectionPayload} role={role} />
      : null;
    const configPanel = key === 'retrieval'
      ? <ConfigSettingsPanel client={client} section="retrieval" initialValue={sectionPayload} models={models} />
      : key === 'chathistory'
        ? <ConfigSettingsPanel
            client={client}
            section="chathistory"
            initialValue={((sectionPayload as Record<string, unknown> | null)?.config)}
            models={models}
            embeddingLocked={((sectionPayload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined)?.has_indexed_messages === true}
            stats={((sectionPayload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined) ?? null}
            onSaved={() => void load(true)}
          />
        : key === 'parser'
          ? <ParserEngineSettingsPanel client={client} />
          : null;
    const ollamaPanel = key === 'ollama' ? <OllamaSettingsPanel client={client} initialValue={sectionPayload} /> : null;
    const usagePanel = key === 'usage' ? <UsagePanel client={client} locale={locale} /> : null;
    const queryHistoryPanel = key === 'query-history' ? <QueryHistoryPanel client={client} locale={locale} role={role} /> : null;
    const cloudPanel = key === 'weknoracloud' ? <CloudSettingsPanel client={client} initialValue={sectionPayload} /> : null;
    const systemPanel = key === 'system' ? <SystemInfoPanel payload={sectionPayload} locale={locale} /> : null;
    // Vue Settings.vue: these two sections stay nav-visible but render no panel
    // content without the system-admin role (the content area is simply empty).
    const systemAdminOnlyPanelDenied = key === 'system-global' || key === 'runtime-queues' ? role !== 'system-admin' : false;
    const runtimeQueuesPanel = key === 'runtime-queues' && role === 'system-admin' ? <RuntimeQueuesPanel client={client} payload={sectionPayload as never} loading={sectionLoading} error={sectionError} /> : null;
    const systemGlobalPanel = key === 'system-global' && role === 'system-admin' ? <SystemGlobalSettingsPanel client={client} initialSettings={Array.isArray(sectionPayload) ? sectionPayload as never : []} /> : null;
    const platformApiKeysPanel = key === 'platform-api-keys' ? <PlatformApiKeysPanel client={client} initialKeys={Array.isArray(sectionPayload) ? sectionPayload as never : []} /> : null;
    const systemAuditPanel = key === 'system-audit-log' ? <SystemAuditLogPanel client={client} payload={sectionPayload} /> : null;
    const envVarPanel = key === 'envvars' ? <EnvVarSettingsPanel client={client} initialPayload={sectionPayload} onMutated={() => void load(true)} /> : null;
    const mcpPanel = key === 'mcp'
      ? <McpSettingsPanel client={client} role={role} initialServices={key in payloadCache ? (Array.isArray(sectionPayload) ? sectionPayload as never : []) : undefined} />
      : null;
    const modelPanel = key === 'models'
      ? <ModelSettingsPanel client={client} role={role} initialModels={Array.isArray(sectionPayload) ? sectionPayload as never : []} initialSubSection={initialSubSection ?? undefined} />
      : null;
    const sandboxPanel = key === 'sandbox'
      ? <SandboxSettingsPanel
          client={client}
          role={role}
          dockerBackendEnabled={isCapabilitySupported(capabilities, 'settings.sandbox.docker', { liteMode })}
          // SandboxSettings.vue openSession (341-344): row click opens the chat
          // session; full-page assign mirrors PlatformShell.openShellSession (204).
          onOpenSession={(sessionId) => { navigate(`/platform/chat/${encodeURIComponent(sessionId)}`); }}
        />
      : null;
    const skillPanel = key === 'skills'
      ? <SkillSettingsPanel client={client} role={role} initialSkills={Array.isArray(sectionPayload) ? sectionPayload as never : (((sectionPayload as { items?: unknown } | null)?.items ?? []) as never)} />
      : null;
    const membersPanel = key === 'members'
      ? <TenantMembersPanel client={client} tenantId={tenantId} role={role} initialMembers={key in payloadCache ? sectionPayload as never : undefined} />
      : null;
    const portedPanel = key === 'mcp' ? mcpPanel : key === 'models' ? modelPanel : key === 'sandbox' ? sandboxPanel : key === 'skills' ? skillPanel : key === 'members' ? membersPanel : key === 'runtime-queues' ? runtimeQueuesPanel : key === 'system-global' ? systemGlobalPanel : key === 'platform-api-keys' ? platformApiKeysPanel : key === 'system-audit-log' ? systemAuditPanel : PARTIALLY_PORTED_SECTIONS.has(key)
      ? (key === 'sandbox'
          ? <PortedSectionsPanel section={key} />
          : <LiveSectionsPanel client={client} section={key} payload={sectionPayload} />)
      : null;
    return (
      <div key={key} style={isActive
        ? { visibility: 'visible' }
        // Hidden sections stay mounted but are taken out of flow with
        // visibility+position instead of display:none — toggling display
        // would reset and replay the .wks-section fade-in animation on every
        // revisit, which read as a flicker.
        : { position: 'absolute', top: 0, left: 0, width: '100%', visibility: 'hidden', pointerEvents: 'none' }} className={`wks-content-wrapper${key === 'members' ? ' wks-content-wrapper--wide' : (SYSTEM_ADMIN_SECTIONS.has(key) || sectionIntegrationTab ? ' wks-content-wrapper--full' : '')}`}>
        {sectionIntegrationTab ? (sectionDenied ?? <IntegrationsRoutePage key={`${tenantId}:${sectionIntegrationTab}`} client={client} tenantId={String(tenantId)} activeTab={sectionIntegrationTab} embedded canEdit={roleAtLeast(role, 'admin')} onTabChange={(nextTab) => {
          // R490 C5 (R489 M3 D8 尾巴) — Vue integration tabs ARE settings
          // sections: the chrome/claw landing「打开 API 信息」button pushes
          // ?section=integration-api (ChromeExtensionLanding.vue openApiSettings
          // L119-122), moving the URL, the section and the sidebar highlight
          // together. Route tab switches through the section select so the
          // address bar tracks the visible section instead of going stale.
          select('integration-' + nextTab);
        }} />) : <div className="wk-settings-section wks-section">
          {/* Panels owning their full Vue section header render it themselves:
              general/models here, and members — TenantMembers.vue:8-65 renders
              the h2 + permissions popover + audit entry + section-description
              with the RBAC doc link, so a wrapper heading would duplicate it
              (previously it also leaked the registry apiDomain as the text);
              skills — SkillSettings.vue:3-11 renders the h2 + help-circle
              tooltip + section-description itself; envvars — R484 G4 D3,
              EnvVarSettings.vue:3-25 renders the h2 + help-circle hover popup
              (introPersonal/introRuntime blocks) + description itself;
              system-global — R491 agent I, SystemSettings.vue:35-57 renders
              the h2 + info-circle priority popup + description itself, so the
              wrapper heading duplicated it (R484 D3 envvars fix pattern).
              R484 G4 D7 — role-denied sections skip the wrapper heading too:
              Vue Settings.vue:88-94 renders ONLY the role-denied block when
              canSeeSection fails (the section component, and with it its
              h2/description, never mounts). */}
          {key !== 'general' && key !== 'models' && key !== 'members' && key !== 'memory' && key !== 'mymemory' && key !== 'mcp' && key !== 'skills' && key !== 'envvars' && key !== 'system-global' && !sectionRoleDenied ? (
            <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
              <div className="w-full">
                <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[normal]">{settingsSectionHeading(locale, key).title}</h2>
                <p className="wk-muted text-muted m-0">{settingsSectionHeading(locale, key).description}</p>
              </div>
            </div>
          ) : null}
          {/* R481 A1 — banner-retry（parser/system/userprofile）：壳层浅红横幅
              透传后端原文 + 重试按钮替代内容区（R480 锚定：Vue
              ParserEngineSettings.vue:14-21 / SystemInfo.vue:13-20 /
              UserProfile.vue:14-21 的 t-alert theme=error + 内嵌重试）；分区
              标题由上方 heading 保留。members 的横幅在 TenantMembersPanel
              自加载内渲染（页级 setError(null)）；storage 已按 R480 基线改判
              静默（StorageBackendSettings.vue 空列表+添加按钮，零错误 UI）。 */}
          {sectionDenied ?? (
          sectionError && sectionErrorMode(key) === 'banner-retry' ? (
            <div data-testid="settings-section-error-banner" role="alert" className="mb-1 flex flex-wrap items-center gap-2">
              <Alert tone="danger" className="min-w-0 flex-1">{sectionError}</Alert>
              <Button type="button" onClick={() => { void load(true); }}>{key === 'members' || key === 'storage' ? t('settings.storage.retry') : t('settings.parser.retry')}</Button>
            </div>
          ) : sectionError && sectionErrorMode(key) === 'inline' ? <Status tone="error">{sectionError}</Status> : sectionLoading ? <Status>{t('common.loading')}</Status> : <Suspense fallback={<Status>{t('common.loading')}</Status>}><>{isActive && notice ? <Status tone="success">{notice}</Status> : null}{generalPanel ?? chatPreferencesPanel ?? resourcePanel ?? configPanel ?? ollamaPanel ?? usagePanel ?? queryHistoryPanel ?? cloudPanel ?? envVarPanel ?? systemPanel ?? portedPanel ?? (key === 'tenant' ? <TenantInfoSection client={client} tenantId={tenantId} role={role} locale={locale} payload={sectionPayload} /> : key === 'userprofile' ? <UserProfileSection client={client} locale={locale} payload={sectionPayload} /> : key === 'memory' ? <div className="wk-settings-memory"><MemoryWorkspacePanel client={client} initialConfig={sectionPayload} canEdit={roleAtLeast(role, 'admin')} /></div> : key === 'mymemory' ? <PersonalMemorySettingsPanel client={client} initialSettings={sectionPayload} /> : null)}</></Suspense>)}
          {key === 'tenant' && role === 'owner' && !sectionDenied && !sectionError && !sectionLoading ? <TenantDeleteZone client={client} tenantId={tenantId} tenantName={tenantEditState(sectionPayload).name || String(tenantId)} onDeleted={() => { window.location.assign('/login'); }} /> : null}
        </div>}
      </div>
    );
  }
  return createPortal((
    <main className="wk-settings-drawer-root">
      {/* R472 A2 — settings 域错误 toast 宿主（对齐 Vue MessagePlugin 右上角
          浮动 + 3s 自动消失语义）。 */}
      <SettingsToastHost />
      <div className="wks-overlay">
        <div ref={modalRef} className="wks-modal" role="dialog" aria-modal="true" aria-label={t('general.settings')} onKeyDown={handleDialogKeyDown}>
          <button
            ref={closeButtonRef}
            type="button"
            className="wks-close focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/35"
            aria-label={t('general.close')}
            data-testid="settings-close"
            onClick={closeSettings}
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
          <div className="wks-container">
            <nav aria-label="Settings sections" className="wks-sidebar">
              <div className="wks-sidebar-header"><h2 className="wks-sidebar-title">{t('general.settings')}</h2></div>
              <div className="wks-nav">
                {settingsNavGroups(locale, visibleSections.map((item) => item.key)).map((group) => (
                  <div key={group.key}>
                    <div className="wks-nav-group-title">{group.label}</div>
                    {group.items.map((item) => (
                      <button
                        key={item.key}
                        type="button"
                        className={`wks-nav-item focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/35${item.key === selectedKey ? ' is-active' : ''}`}
                        aria-current={item.key === selectedKey ? 'page' : undefined}
                        onClick={() => select(item.key)}
                      >
                        <span className="wks-nav-icon">{item.icon}</span>
                        <span className="wks-nav-label">{item.label}</span>
                      </button>
                    ))}
                  </div>
                ))}
              </div>
            </nav>
            <section className="wks-content" aria-live="polite" style={{ position: 'relative' }}>
              {visitedSections.map((key) => renderSectionPanel(key))}
            </section>
          </div>
        </div>
      </div>
    </main>
  ), document.body);
}

// BEGIN settings nav grouping + inline lucide-style icons (ported from
// frontend/src/views/settings/Settings.vue navGroups/navItems).
import type { ReactNode } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';

// Ported from frontend/src/views/settings/Settings.vue navGroups (账户 / 空间 /
// 模型 / 发布集成 / 数据与扩展 / 系统管理 / 平台). Every React registry section
// must land in one of these groups; anything unknown falls into the fallback
// group at the bottom so role gating can still surface it.
// Localized section titles (settings.* keys exist in packages/i18n/src/settings.ts).


const NAV_GROUP_DEFS: ReadonlyArray<{ key: string; labelKey: string; sections: readonly string[] }> = [
  { key: 'account', labelKey: 'settings.navGroups.account', sections: ['general', 'chat-preferences', 'userprofile', 'mymemory', 'envvars', 'usage'] },
  { key: 'workspace', labelKey: 'settings.navGroups.workspace', sections: ['tenant', 'members', 'chathistory', 'memory'] },
  { key: 'models_runtime', labelKey: 'settings.navGroups.modelsRuntime', sections: ['models', 'ollama', 'weknoracloud'] },
  { key: 'integrations', labelKey: 'integrations.title', sections: INTEGRATION_SECTIONS.map((item) => `integration-${item.key}`) },
  { key: 'data_extensions', labelKey: 'settings.navGroups.dataExtensions', sections: ['vectorstore', 'parser', 'storage', 'sandbox', 'skills', 'websearch', 'mcp'] },
  { key: 'system_administration', labelKey: 'settings.navGroups.systemAdministration', sections: ['system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log'] },
  { key: 'platform', labelKey: 'settings.navGroups.platform', sections: ['system'] },
];

// Vue's current Settings.vue keeps retrieval available to its settings state
// model but does not expose it in navItems. Keep the React deep-link route
// readable for historical URLs while avoiding a navigation-only fallback item
// that has no Vue counterpart.
const NAV_HIDDEN_SECTIONS = new Set(['retrieval',
  // Round-22 parity (2026-09-19): SP12's 用量统计 section has no Vue
  // counterpart in Settings.vue navItems, so it joins retrieval as nav-hidden
  // while ?section=usage and the panel stay reachable for direct URLs.
  'usage',
  // SP13 query-history (2026-09-20): same ruling — no Vue navItems
  // counterpart, so the admin audit panel stays nav-hidden while
  // ?section=query-history keeps the deep link reachable.
  'query-history']);

// Vue label sources (Settings.vue navItems): every label is a settings.* i18n
// string auto-ported into packages/i18n/src/settings.ts; ollama/weknoracloud
// stay literal there too.
const SECTION_LABEL_KEYS: Record<string, string> = {
  general: 'general.title',
  'chat-preferences': 'chatPreferences.title',
  userprofile: 'userProfile.title',
  mymemory: 'memorySettings.title',
  envvars: 'envVarSettings.title',
  usage: 'settings.usage.title',
  tenant: 'settings.tenantInfo',
  members: 'tenantMember.title',
  chathistory: 'chatHistorySettings.title',
  memory: 'memoryWorkspaceSettings.title',
  retrieval: 'retrievalSettings.title',
  models: 'settings.modelManagement',
  websearch: 'settings.webSearchConfig',
  vectorstore: 'settings.vectorStoreEngine',
  parser: 'settings.parserEngine',
  storage: 'settings.storageEngine',
  sandbox: 'settings.sandbox.title',
  skills: 'settings.skills.title',
  mcp: 'settings.mcpService',
  system: 'settings.versionInfo',
  'system-global': 'settings.system',
  'runtime-queues': 'settings.taskQueue',
  'platform-api-keys': 'platformApiKeys.title',
  'system-audit-log': 'system.globalSettings.audit.tabLabel',
};

export interface SettingsNavItem {
  readonly key: string;
  readonly label: string;
  readonly icon: ReactNode;
}

export interface SettingsNavGroupView {
  readonly key: string;
  readonly label: string;
  readonly items: readonly SettingsNavItem[];
}

export function settingsSectionLabel(locale: Locale, key: string): string {
  const integrationTab = integrationTabForSection(key);
  if (integrationTab) return formatMessage(locale, `integrations.tabs.${integrationTab}`);
  const labelKey = SECTION_LABEL_KEYS[key];
  if (labelKey) return formatMessage(locale, labelKey);
  return settingsSectionMeta(key)?.title ?? key;
}

// Inline lucide-style stroke icons (24x24 viewBox), mirroring the t-icon names
// used by Settings.vue (setting / user / usergroup / key / chat / server / …).
function icon(paths: ReactNode): ReactNode {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths}
    </svg>
  );
}

const SECTION_ICONS: Record<string, ReactNode> = {
  general: icon(<><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></>),
  userprofile: icon(<><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></>),
  // SP14 Task 4 — 会话偏好：sliders（lucide sliders-horizontal 风格）。
  'chat-preferences': icon(<><line x1="21" y1="4" x2="14" y2="4" /><line x1="10" y1="4" x2="3" y2="4" /><line x1="21" y1="12" x2="12" y2="12" /><line x1="8" y1="12" x2="3" y2="12" /><line x1="21" y1="20" x2="16" y2="20" /><line x1="12" y1="20" x2="3" y2="20" /><line x1="14" y1="2" x2="14" y2="6" /><line x1="8" y1="10" x2="8" y2="14" /><line x1="16" y1="18" x2="16" y2="22" /></>),
  mymemory: icon(<path d="M19 21l-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2z" />),
  envvars: icon(<><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" /></>),
  usage: icon(<><path d="M3 3v18h18" /><path d="M7 15v-4M12 15V8M17 15v-7" /></>),
  tenant: icon(<><rect x="4" y="3" width="16" height="18" rx="2" /><path d="M9 8h.01M15 8h.01M9 12h.01M15 12h.01M9 16h.01M15 16h.01" /></>),
  members: icon(<><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></>),
  chathistory: icon(<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />),
  memory: icon(<><circle cx="5" cy="6" r="1.4" /><circle cx="5" cy="12" r="1.4" /><circle cx="5" cy="18" r="1.4" /><path d="M9 6h11M9 12h11M9 18h11" /></>),
  models: icon(<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9.5" y="9.5" width="5" height="5" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>),
  ollama: icon(<><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></>),
  weknoracloud: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <rect x="1.5" y="1.5" width="15" height="15" rx="3.5" stroke="currentColor" strokeWidth="1.2" />
      <path d="M4.5 5.5L6.5 12.5L9 7.5L11.5 12.5L13.5 5.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  ),
  vectorstore: icon(<><ellipse cx="12" cy="5" rx="8" ry="3" /><path d="M4 5v14c0 1.66 3.58 3 8 3s8-1.34 8-3V5" /><path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" /></>),
  parser: icon(<><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /><circle cx="11" cy="13" r="2.5" /><path d="M13 15l2.5 2.5" /></>),
  storage: icon(<path d="M17.5 19a4.5 4.5 0 0 0 .38-8.98 7 7 0 0 0-13.76 1.86A4 4 0 0 0 6 19z" />),
  sandbox: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <rect x="2.5" y="3" width="13" height="12" rx="2" stroke="currentColor" strokeWidth="1.2" />
      <path d="M2.5 6.5h13" stroke="currentColor" strokeWidth="1.2" />
      <path d="M5.5 10h4M5.5 12.5h2.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  ),
  skills: icon(<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />),
  websearch: (
    <svg width="17" height="17" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.2" />
      <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" strokeWidth="1.2" />
      <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" strokeWidth="1.2" />
      <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
      <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  ),
  mcp: icon(<><path d="M14.7 6.3a4.5 4.5 0 0 0 6 6l-7.4 7.4a2.1 2.1 0 0 1-3-3z" /><path d="M14.7 6.3l3-3 3 3-3 3" /></>),
  system: icon(<><circle cx="12" cy="12" r="10" /><path d="M12 16v-4M12 8h.01" /></>),
  'system-global': icon(<><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></>),
  'runtime-queues': icon(<><path d="M8 6h13M8 12h13M8 18h13" /><path d="M3 6h.01M3 12h.01M3 18h.01" /></>),
  'platform-api-keys': icon(<><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="M9.5 12l2 2 3.5-3.5" /></>),
  'system-audit-log': icon(<><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" /><path d="M12 7v5l3 3" /></>),
  // Integration nav icons mirror frontend/src/config/integrations.ts
  // INTEGRATION_PREVIEW_ITEMS (chat-message / code / secured / extension /
  // the claw lobster emoji).
  'integration-im': icon(<><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" /></>),
  'integration-embed': icon(<><path d="M16 18l6-6-6-6M8 6l-6 6 6 6" /></>),
  'integration-api': icon(<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />),
  'integration-cli': icon(<><path d="M4 17l6-6-6-6M12 19h8" /></>),
  'integration-chrome': icon(<><circle cx="12" cy="12" r="10" /><circle cx="12" cy="12" r="4" /><path d="M21.2 8H12M6.8 6.4L10 12M8 21.2L12.5 14" /></>),
  'integration-claw': <span className="nav-icon-emoji" aria-hidden="true">🦞</span>,
};

const FALLBACK_ICON = icon(<circle cx="12" cy="12" r="9" />);

export function settingsNavGroups(locale: Locale, visibleKeys: readonly string[]): SettingsNavGroupView[] {
  const navKeys = visibleKeys.filter((key) => !NAV_HIDDEN_SECTIONS.has(key));
  const labels = new Map(navKeys.map((key) => [key, settingsSectionLabel(locale, key)] as const));
  const groups: SettingsNavGroupView[] = NAV_GROUP_DEFS.map((def) => ({
    key: def.key,
    label: formatMessage(locale, def.labelKey),
    items: def.sections
      .filter((key) => labels.has(key))
      .map((key) => ({ key, label: labels.get(key)!, icon: SECTION_ICONS[key] ?? FALLBACK_ICON })),
  })).filter((group) => group.items.length > 0);
  // Unknown registry sections keep a fallback group so role-gated entries are
  // never silently dropped from the drawer navigation.
  const assigned = new Set(NAV_GROUP_DEFS.flatMap((def) => def.sections as readonly string[]));
  const unknown = navKeys.filter((key) => !assigned.has(key));
  if (unknown.length > 0) {
    groups.push({ key: 'other', label: formatMessage(locale, 'settings.navGroups.platform'), items: unknown.map((key) => ({ key, label: labels.get(key)!, icon: SECTION_ICONS[key] ?? FALLBACK_ICON })) });
  }
  return groups;
}
