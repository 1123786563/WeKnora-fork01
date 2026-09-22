import { lazy, Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { isCapabilitySupported, type CapabilityMap } from '@weknora/domain';
import { integrationTabForSection, integrationSettingsQuery, selectSettingsQuery } from '@weknora/views/integrations/settings-route';
import { INTEGRATION_SECTIONS } from '@weknora/views/integrations/registry';
import { openContextualGuide } from '@weknora/views/guides/contextual-guides';
const IntegrationsRoutePage = lazy(() => import('../integrations/IntegrationsRoutePage.tsx').then((m) => ({ default: m.IntegrationsRoutePage })));
import type { WeKnoraClient } from '@weknora/api-client';
import type { SettingsRole } from '@weknora/views/settings/registry';
import { roleAtLeast, SETTINGS_SECTIONS, settingsSectionsForRole } from '@weknora/views/settings/registry';
import { Alert as TAlert, Button as TButton } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
// TDesign 同构迁移（T12a）：图标走 tdesign-icons-react 本地 sprite（= Vue 端
// `Icon as TIcon`，与 Vue t-icon 同源同字形，台账 #10）。
import { Icon as TIcon } from 'tdesign-icons-react';
import { pushSettingsToast, SettingsToastHost } from './settings-toast.tsx';
import { modelFormatMessage } from './model-settings.ts';
import { navigate } from '../platform/navigation.ts';
import { profilePasswordPatch, settingsCloseMode, settingsSectionHeading, settingsSectionMeta, tenantPatch } from './surface.ts';
const MemoryWorkspacePanel = lazy(() => import('./PersonalMemoryPanel.tsx').then((m) => ({ default: m.MemoryWorkspacePanel })));
const PersonalMemorySettingsPanel = lazy(() => import('./PersonalMemorySettingsPanel.tsx').then((m) => ({ default: m.PersonalMemorySettingsPanel })));
const ResourceSettingsPanel = lazy(() => import('./ResourceSettingsPanel.tsx').then((m) => ({ default: m.ResourceSettingsPanel })));
import type { SettingsModelOption } from './ConfigSettingsPanel.tsx';
const ConfigSettingsPanel = lazy(() => import('./ConfigSettingsPanel.tsx').then((m) => ({ default: m.ConfigSettingsPanel })));
// T12b：chathistory 分区平移 ChatHistorySettings.vue（自带 section-header，直挂 .section）。
const ChatHistorySettingsPanel = lazy(() => import('./ChatHistorySettingsPanel.tsx').then((m) => ({ default: m.ChatHistorySettingsPanel })));
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
import './settings.td.css';

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
 *
 * T12a（TDesign 同构迁移）：各 section 面板按 Vue SFC 自持 loading/error
 * 态（UserProfile.vue/TenantInfo.vue 等各自 fetch）迁移时，把对应键加进
 * SELF_ERROR_SECTIONS 并同步锚定测试（settings-error-ux.test.tsx）。
 */
export type SettingsSectionErrorMode = 'inline' | 'toast-keep' | 'toast-retry' | 'banner-retry' | 'silent';

export function sectionErrorMode(key: string): SettingsSectionErrorMode {
  if (key === 'models') return 'toast-keep';
  if (key === 'skills' || key === 'mcp') return 'toast-retry';
  if (key === 'members' || key === 'parser' || key === 'system' || key === 'userprofile') return 'banner-retry';
  if (key === 'storage' || key === 'vectorstore' || key === 'websearch' || key === 'weknoracloud' || key === 'ollama' || key === 'retrieval') return 'silent';
  return 'inline';
}

// Sections whose panels own their Vue loading/error states (fetch inside the
// panel like the Vue SFC does); the shell keeps its silent refresh behavior.
// T12a 各 section 提交时按 Vue SFC 实况增删（userprofile/tenant/mymemory/
// envvars 待各自面板提交时改为自持错误态并同步锚定测试）。
const SELF_ERROR_SECTIONS = new Set(['members']);

// Panels migrated to the Vue DOM (T12a): they render their own section-header
// and their styles live in settings.td.css — they mount directly inside the
// shell .section container like Vue Settings.vue does (no wk-settings-section
// wrapper, no shell heading). Integration sections have been self-headered
// since R490 (handled separately via sectionIntegrationTab).
// T12c：system 面板自持 section-header 与 loading/error 态（SystemInfo.vue
// loading-inline/error-inline），userprofile 同款自持先例。
const SELF_HEADER_SECTIONS = new Set<string>(['general', 'userprofile', 'envvars', 'tenant', 'mymemory', 'chathistory', 'memory', 'ollama', 'weknoracloud', 'models', 'parser', 'sandbox', 'skills', 'system']);

// S1 评审回收：system-global/runtime-queues/platform-api-keys/system-audit-log
// 四个死条目已删——T12c 后它们在 portedPanel 三元链上有显式分支，永不落入
// PARTIALLY_PORTED 兜底路径。
const PARTIALLY_PORTED_SECTIONS = new Set(['models', 'members', 'mcp', 'sandbox', 'skills']);
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
        } else if (mode === 'toast-retry' || SELF_ERROR_SECTIONS.has(selectedKey) || mode === 'silent') {
          // skills/mcp/members 面板自加载并渲染各自的 Vue 对齐错误态
          // （toast+空态+重试 / 横幅+重试）；中央失败不得顶替内容区，
          // 面板以 undefined 初始数据自拉。
          // silent（R480 基线）：storage/vectorstore/websearch/weknoracloud/
          // ollama/retrieval 在 Vue 500 下完全静默降级（空列表/默认表单）——
          // 无横幅、无 toast、无重试；面板以 null payload 渲染。
          setError(null);
        } else if (mode === 'banner-retry') {
          // parser/system（R480 锚定）：壳层横幅透传后端原文 +
          // 重试，替换内容区（Vue SystemInfo.vue:13-20、
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

  // Vue drawer never paints a focus ring on the programmatically-focused
  // close button when the overlay opens (the scan screenshot compares that
  // exact frame). Keep the focus for a11y + tests, but only reveal
  // :focus-visible outlines once real keyboard navigation happens.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Tab') document.documentElement.classList.add('wk-kbd-nav');
    };
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.documentElement.classList.remove('wk-kbd-nav');
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
  // merely hidden on switch (Vue Settings.vue uses v-if per section — panels
  // unmount on switch; the keep-alive here is the React-side flicker guard.
  // Hidden sections go display:none (v-show semantics) so the CSS fadeIn
  // animation replays on revisit like a Vue remount).
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
    // Vue Settings.vue:88-94 — role-denied renders only the role-denied
    // block (class "section role-denied" directly inside content-wrapper, no
    // panel component mounts).
    if (sectionRoleDenied) {
      return (
        <div key={key} className="section role-denied" style={isActive ? undefined : { display: 'none' }} data-testid="role-denied-panel">
          <div className="role-denied-icon"><TIcon name="lock-on" size="48px" /></div>
          <div className="role-denied-title">{t('settings.roleDenied.title')}</div>
          <div className="role-denied-desc">{t('settings.roleDenied.desc')}</div>
        </div>
      );
    }
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
      : key === 'parser'
        ? <ParserEngineSettingsPanel client={client} />
        : null;
    // Vue ChatHistorySettings.vue 自持 section-header 与统计区（T12b 平移）。
    const chatHistoryPanel = key === 'chathistory'
      ? <ChatHistorySettingsPanel
          client={client}
          initialValue={((sectionPayload as Record<string, unknown> | null)?.config)}
          models={models}
          embeddingLocked={((sectionPayload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined)?.has_indexed_messages === true}
          stats={((sectionPayload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined) ?? null}
          onSaved={() => void load(true)}
        />
      : null;
    const ollamaPanel = key === 'ollama' ? <OllamaSettingsPanel client={client} initialValue={sectionPayload} /> : null;
    const usagePanel = key === 'usage' ? <UsagePanel client={client} locale={locale} /> : null;
    const queryHistoryPanel = key === 'query-history' ? <QueryHistoryPanel client={client} locale={locale} role={role} /> : null;
    const cloudPanel = key === 'weknoracloud' ? <CloudSettingsPanel client={client} initialValue={sectionPayload} /> : null;
    const systemPanel = key === 'system' ? <SystemInfoPanel payload={sectionPayload} locale={locale} error={sectionError} loading={sectionLoading} onRetry={() => { void load(true); }} /> : null;
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
    // Vue Settings.vue 渲染结构：content-wrapper > div.section > 面板根元素。
    // 已迁面板自带 section-header + 平移样式，直挂 .section（T12a 各面板提交
    // 时把 key 加进 SELF_HEADER_SECTIONS）；未迁面板仍由 wk-settings-section
    // 壳层 wrapper + heading 承载（settings-wrapper.css 旧栈样式，待各自批次
    // 迁移时收编）。integration 面板从 R490 起即自持 header，无壳层 wrapper。
    if (SELF_HEADER_SECTIONS.has(key) || sectionIntegrationTab) {
      const content = sectionIntegrationTab
        ? <IntegrationsRoutePage key={`${tenantId}:${sectionIntegrationTab}`} client={client} tenantId={String(tenantId)} activeTab={sectionIntegrationTab} embedded canEdit={roleAtLeast(role, 'admin')} onTabChange={(nextTab) => {
            // R490 C5 (R489 M3 D8 尾巴) — Vue integration tabs ARE settings
            // sections: the chrome/claw landing「打开 API 信息」button pushes
            // ?section=integration-api (ChromeExtensionLanding.vue openApiSettings
            // L119-122), moving the URL, the section and the sidebar highlight
            // together. Route tab switches through the section select so the
            // address bar tracks the visible section instead of going stale.
            select('integration-' + nextTab);
          }} />
        : <Suspense fallback={<Status>{t('common.loading')}</Status>}>{generalPanel ?? chatPreferencesPanel ?? resourcePanel ?? configPanel ?? chatHistoryPanel ?? ollamaPanel ?? usagePanel ?? queryHistoryPanel ?? cloudPanel ?? envVarPanel ?? systemPanel ?? portedPanel ?? (key === 'tenant' ? <TenantInfoSection client={client} tenantId={tenantId} role={role} locale={locale} payload={sectionPayload} error={sectionError} loading={sectionLoading} onRetry={() => { void load(true); }} /> : key === 'userprofile' ? <UserProfileSection client={client} locale={locale} payload={sectionPayload} error={sectionError} loading={sectionLoading} onRetry={() => { void load(true); }} /> : key === 'memory' ? <MemoryWorkspacePanel client={client} initialConfig={sectionPayload} canEdit={roleAtLeast(role, 'admin')} /> : key === 'mymemory' ? <PersonalMemorySettingsPanel client={client} initialSettings={sectionPayload} /> : null)}</Suspense>;
      return (
        <div key={key} className="section" style={isActive ? undefined : { display: 'none' }}>
          {content}
        </div>
      );
    }
    return (
      <div key={key} className="section" style={isActive ? undefined : { display: 'none' }}>
        <div className="wk-settings-section wks-section">
          {/* Panels owning their full Vue section header render it themselves:
              general/models here, and members — TenantMembers.vue:8-65 renders
              the h2 + permissions popover + audit entry + section-description
              with the RBAC doc link, so a wrapper heading would duplicate it;
              memory/mymemory — MemoryWorkspaceSettings.vue / MemorySettings.vue
              render the h2 + hint trigger + description themselves;
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
          {key !== 'general' && key !== 'models' && key !== 'members' && key !== 'memory' && key !== 'mymemory' && key !== 'mcp' && key !== 'skills' && key !== 'envvars' && key !== 'system-global' && key !== 'weknoracloud' && key !== 'system-audit-log' ? (
            /* Vue panels own their section-header (TenantInfo.vue:682-697 et
               al.): 20px/600 h2 with an 8px gap, 14px/1.5 secondary
               description, then a bare 32px margin — no divider line. */
            <div className="wk-settings-panel-heading flex items-start justify-between gap-4 mb-8 max-[720px]:flex-col">
              <div className="w-full">
                <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[normal]">{settingsSectionHeading(locale, key).title}</h2>
                <p className="m-0 text-[14px] leading-[21px] text-[rgba(0,0,0,0.6)]">{settingsSectionHeading(locale, key).description}</p>
              </div>
            </div>
          ) : null}
          {systemAdminOnlyPanelDenied ? null : (
          sectionError && sectionErrorMode(key) === 'banner-retry' ? (
            <div data-testid="settings-section-error-banner" role="alert" className="mb-1 flex flex-wrap items-center gap-2">
              <TAlert theme="error" className="min-w-0 flex-1" message={sectionError} />
              <TButton type="button" onClick={() => { void load(true); }}>{key === 'members' || key === 'storage' ? t('settings.storage.retry') : t('settings.parser.retry')}</TButton>
            </div>
          ) : sectionError && sectionErrorMode(key) === 'inline' ? <Status tone="error">{sectionError}</Status> : sectionLoading ? <Status>{t('common.loading')}</Status> : <Suspense fallback={<Status>{t('common.loading')}</Status>}><>{isActive && notice ? <Status tone="success">{notice}</Status> : null}{generalPanel ?? chatPreferencesPanel ?? resourcePanel ?? configPanel ?? chatHistoryPanel ?? ollamaPanel ?? usagePanel ?? queryHistoryPanel ?? cloudPanel ?? envVarPanel ?? systemPanel ?? portedPanel ?? (key === 'tenant' ? <TenantInfoSection client={client} tenantId={tenantId} role={role} locale={locale} payload={sectionPayload} error={sectionError} loading={sectionLoading} onRetry={() => { void load(true); }} /> : key === 'userprofile' ? <UserProfileSection client={client} locale={locale} payload={sectionPayload} error={sectionError} loading={sectionLoading} onRetry={() => { void load(true); }} /> : key === 'memory' ? <MemoryWorkspacePanel client={client} initialConfig={sectionPayload} canEdit={roleAtLeast(role, 'admin')} /> : key === 'mymemory' ? <PersonalMemorySettingsPanel client={client} initialSettings={sectionPayload} /> : null)}</></Suspense>)}
        </div>
      </div>
    );
  }

  const wrapperModifier = selectedKey === 'members'
    ? 'content-wrapper--wide wks-content-wrapper--wide'
    : SYSTEM_ADMIN_SECTIONS.has(selectedKey) || integrationTab
      ? 'content-wrapper--full wks-content-wrapper--full'
      : '';

  return createPortal((
    <div className="wk-settings-drawer-root">
      {/* R472 A2 — settings 域错误 toast 宿主（对齐 Vue MessagePlugin 右上角
          浮动 + 3s 自动消失语义）。 */}
      <SettingsToastHost />
      {/* Vue Settings.vue 壳层 DOM（Teleport to body → createPortal）：
          settings-overlay > settings-modal > close-btn + settings-container
          (settings-sidebar[sidebar-header + settings-nav] + settings-content
          > content-wrapper > section)。样式平移在 settings.td.css §1。 */}
      <div className="settings-overlay">
        <div ref={modalRef} className="settings-modal wks-modal" role="dialog" aria-modal="true" aria-label={t('general.settings')} onKeyDown={handleDialogKeyDown}>
          <button
            ref={closeButtonRef}
            type="button"
            className="close-btn"
            aria-label={t('general.close')}
            data-testid="settings-close"
            onClick={closeSettings}
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
          <div className="settings-container">
            <div className="settings-sidebar">
              <div className="sidebar-header"><h2 className="sidebar-title">{t('general.settings')}</h2></div>
              <div className="settings-nav">
                {settingsNavGroups(locale, visibleSections.map((item) => item.key)).map((group) => (
                  <div key={group.key}>
                    <div className="nav-group-title">{group.label}</div>
                    {group.items.map((item) => (
                      <div
                        key={item.key}
                        className={'nav-item' + (item.key === selectedKey ? ' active' : '')}
                        role="button"
                        tabIndex={0}
                        aria-current={item.key === selectedKey ? 'page' : undefined}
                        data-settings-nav={item.key}
                        onClick={() => select(item.key)}
                        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); select(item.key); } }}
                      >
                        <SettingsNavIcon itemKey={item.key} />
                        <span className="nav-label">{item.label}</span>
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            </div>
            <div className="settings-content">
              <div className={'content-wrapper wks-content-wrapper' + (wrapperModifier ? ' ' + wrapperModifier : '')}>
                {visitedSections.map((key) => renderSectionPanel(key))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  ), document.body);
}

/* ==== Settings nav（Settings.vue navGroups/navItems 平移） ==== */

// BEGIN settings nav grouping + tdesign sprite icons (ported from
// frontend/src/views/settings/Settings.vue navGroups/navItems).
import { formatMessage, type Locale } from '@weknora/i18n';

// Ported from frontend/src/views/settings/Settings.vue navGroups (账户 / 空间 /
// 模型 / 发布集成 / 数据与扩展 / 系统管理 / 平台). Every React registry section
// must land in one of these groups; anything unknown falls into the fallback
// group at the bottom so role gating can still surface it.
// Localized section titles (settings.* keys exist in packages/i18n/src/settings.ts).

const NAV_GROUP_DEFS: ReadonlyArray<{ key: string; labelKey: string; sections: readonly string[] }> = [
  { key: 'account', labelKey: 'settings.navGroups.account', sections: ['general', 'userprofile', 'mymemory', 'envvars', 'usage'] },
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
  // R-parity (2026-09-21): SP14's 会话偏好 has no Vue Settings.vue navItems
  // counterpart (Vue navGroups account = general/userprofile/mymemory/envvars),
  // so it joins retrieval as nav-hidden while ?section=chat-preferences and
  // the panel stay reachable for direct URLs.
  'chat-preferences',
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

// Vue Settings.vue navItems icon names（t-icon sprite 名）；websearch /
// weknoracloud / sandbox 为自定义 svg，claw 为 emoji（见 SettingsNavIcon）。
const NAV_ICON_NAMES: Record<string, string> = {
  general: 'setting',
  userprofile: 'user',
  mymemory: 'bookmark',
  envvars: 'key',
  tenant: 'user-circle',
  members: 'usergroup',
  chathistory: 'chat',
  memory: 'bulletpoint',
  models: 'control-platform',
  ollama: 'server',
  vectorstore: 'data-base',
  parser: 'file-search',
  storage: 'cloud',
  // SKILL_ICON（frontend/src/types/mention.ts:4）
  skills: 'system-code',
  mcp: 'tools',
  system: 'info-circle',
  'system-global': 'server',
  'runtime-queues': 'queue',
  'platform-api-keys': 'secured',
  'system-audit-log': 'history',
  'integration-im': 'chat-message',
  'integration-embed': 'code',
  'integration-api': 'secured',
  'integration-cli': 'code',
  'integration-chrome': 'extension',
};

// Settings.vue navItems: claw 的 icon 是 emoji（INTEGRATION_PREVIEW_ITEMS）。
const NAV_EMOJI_ICONS: Record<string, string> = { 'integration-claw': '🦞' };

// Vue Settings.vue navItems 自定义 svg 图标逐字复刻（websearch / weknoracloud
// / sandbox——避免与 Ollama / 系统设置共用 server 图标）。
function SettingsNavIcon({ itemKey }: { itemKey: string }) {
  if (itemKey === 'websearch') {
    return (
      <svg width="17" height="17" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg" className="nav-icon">
        <circle cx="9" cy="9" r="7" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
        <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
      </svg>
    );
  }
  if (itemKey === 'weknoracloud') {
    return (
      <svg width="17" height="17" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg" className="nav-icon">
        <rect x="1.5" y="1.5" width="15" height="15" rx="3.5" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <path d="M4.5 5.5L6.5 12.5L9 7.5L11.5 12.5L13.5 5.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      </svg>
    );
  }
  if (itemKey === 'sandbox') {
    return (
      <svg width="17" height="17" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg" className="nav-icon">
        <rect x="2.5" y="3" width="13" height="12" rx="2" stroke="currentColor" strokeWidth="1.2" fill="none" />
        <path d="M2.5 6.5h13" stroke="currentColor" strokeWidth="1.2" />
        <path d="M5.5 10h4M5.5 12.5h2.5" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
      </svg>
    );
  }
  const emoji = NAV_EMOJI_ICONS[itemKey];
  if (emoji) return <span className="nav-icon nav-icon-emoji">{emoji}</span>;
  return <TIcon name={NAV_ICON_NAMES[itemKey] ?? 'setting'} className="nav-icon" />;
}

export function settingsNavGroups(locale: Locale, visibleKeys: readonly string[]): SettingsNavGroupView[] {
  const navKeys = visibleKeys.filter((key) => !NAV_HIDDEN_SECTIONS.has(key));
  const labels = new Map(navKeys.map((key) => [key, settingsSectionLabel(locale, key)] as const));
  const groups: SettingsNavGroupView[] = NAV_GROUP_DEFS.map((def) => ({
    key: def.key,
    label: formatMessage(locale, def.labelKey),
    items: def.sections
      .filter((key) => labels.has(key))
      .map((key) => ({ key, label: labels.get(key)! })),
  })).filter((group) => group.items.length > 0);
  // Unknown registry sections keep a fallback group so role-gated entries are
  // never silently dropped from the drawer navigation.
  const assigned = new Set(NAV_GROUP_DEFS.flatMap((def) => def.sections as readonly string[]));
  const unknown = navKeys.filter((key) => !assigned.has(key));
  if (unknown.length > 0) {
    groups.push({ key: 'other', label: formatMessage(locale, 'settings.navGroups.platform'), items: unknown.map((key) => ({ key, label: labels.get(key)! })) });
  }
  return groups;
}
