import * as React from 'react';
import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import type { AgentConfiguration, InstalledSkill, ModelConfiguration, SandboxConfigRecord, SkillCatalog, SkillCatalogInstallation, SkillConfiguration, SkillFileContent, SkillInstallGuidanceState, WeKnoraClient } from '@weknora/api-client';
import { initialSkillTimelineState, installProgressPercent, reduceSkillTimelineFrame, type SkillInstallProgressEvent, type SkillTimelineState } from '@weknora/domain/sandbox/skill-install';
// TDesign 同构迁移（批次 2 收官）：列表域（section-header / loading / 空态 /
// skill-list 卡片网格 / chip + install 弹层面板 / add 卡）按 SkillSettings.vue
// 逐节点平移，组件换 tdesign-react（TTooltip/TLoading/TEmpty/TPopup/TButton +
// tdesign-icons-react）；Add 向导 / Install / Manage / Files 抽屉与删除确认弹层
// 沿用 React 表单栈 + Tailwind（批次先例：sandbox/parser/models 编辑器同口径，
// 扫描稳态不可达，待后续批次收编），packages/ui 旧栈 仅剩保留域使用。
// S6 抽屉收编：skills 四抽屉（添加/安装/管理/文件 + 删除确认）离开
// packages/ui 旧栈 表单栈（T15 硬前置），组件换 tdesign；Status/Card 走
// shared/wk-legacy（无 TDesign 对应，playbook §1 附行）。
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { AddIcon, DeleteIcon, FolderIcon, Icon as TIcon } from 'tdesign-icons-react';
import { Button as TButton, Checkbox as TCheckbox, Dialog as TDialog, Empty as TEmpty, Input as TInput, Loading as TLoading, Popup as TPopup, Select as TSelect, Switch as TSwitch, Textarea as TTextarea, Tooltip as TTooltip } from 'tdesign-react';
import { SettingDrawer } from './SettingDrawer.tsx';
import { renderChatMarkdown } from '../../../../packages/views/src/chat/markdown.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { pushSettingsToast } from './settings-toast.tsx';
import { providerLogo } from './providerLogos.ts';
import { navigate } from '../platform/navigation.ts';
import { observeUploadProgress } from '../platform/http.ts';
import {
  MAX_ENV_VALUE_BYTES, adminSkillEnvClearPayload, backendLabelKey, buildSkillFileTree, canClearAdminSkillEnv,
  canDeleteCatalog, classifySkillRegisterError, clearSubmittedSkillEnvDrafts, collectSkillDirPaths, compactSkillText,
  editedSkillEnvPayload, flattenSkillFileRows, installEntryTone, installErrorLines, installName, installsView,
  isInstallBusy, isMarkdownPath, isNamedSandboxBackend, isSafeSkillFilePath, isZipBundle, isValidEnvValueLength,
  liveCatalogInstalls, maxSkillBundleMB, sandboxPickRows, sandboxTargetLine, splitMarkdownFrontmatter,
} from '../configuration/management.ts';

type SettingsRole = 'viewer' | 'admin' | 'owner' | 'system-admin';

type Props = {
  client: WeKnoraClient;
  role: SettingsRole;
  initialSkills?: readonly SkillConfiguration[];
  initialCatalog?: readonly SkillCatalog[];
  initialSandboxConfigs?: readonly SandboxConfigRecord[];
};

// zh-CN fallbacks for keys the shared packages/i18n settings bundle has not migrated yet
// (they live under frontend/src/components/** in Vue, which the bulk port of
// frontend/src/views/settings/** did not cover). Once packages/i18n carries them the
// shared translations win automatically; until then the exact Vue zh-CN copy renders.
const VUE_SKILL_FALLBACK: Record<string, string> = {
  'common.copied': '已复制',
  'common.on': '开启',
  'common.confirmDelete': '确认删除',
  'settings.sandbox.skillFilesTitle': '文件',
  'settings.sandbox.skillFilesEmpty': '该技能还没有可查看的文件。',
  'settings.sandbox.skillFilesLoadFailed': '加载技能文件失败',
  'settings.sandbox.skillFilesFileLoadFailed': '无法读取该文件',
  'settings.sandbox.skillFilesBinary': '该文件是二进制内容，无法在线预览。',
  'settings.sandbox.skillFilesTruncated': '文件较大，仅显示前一部分。',
  'settings.sandbox.skillFilesSelectHint': '选择左侧文件以查看内容',
  'settings.sandbox.skillFilesPreview': '预览',
  'settings.sandbox.skillFilesSource': '源码',
  'settings.sandbox.backends.docker': 'Docker',
  'settings.sandbox.backends.cube': 'CubeSandbox',
  'settings.sandbox.backends.e2b': 'E2B',
  'settings.sandbox.skillLoadFailed': '加载技能列表失败',
  'settings.sandbox.skillToggleFailed': '更新技能状态失败',
  'settings.sandbox.skillDeleteAccepted': '已开始删除技能',
  'settings.sandbox.skillRetry': '重新安装',
  'settings.sandbox.skillRetryHint': '用已保存的安装包重试，无需重新上传',
  'settings.sandbox.skillRetryAccepted': '已开始重新安装',
  'settings.sandbox.skillRetryFailed': '重新安装失败',
  'settings.sandbox.skillStop': '停止安装',
  'settings.sandbox.skillStopHint': '中止当前安装，之后可以重试或卸载',
  'settings.sandbox.skillStopAccepted': '已停止',
  'settings.sandbox.skillStopFailed': '停止失败',
  'settings.sandbox.skillRemoveInProgress': '正在卸载',
  'settings.sandbox.skillRemoveWaiting': '已开始从镜像卸载，正在等待进度…',
  'settings.sandbox.skillRemoveDone': '已从沙箱卸载「{name}」。技能仍在目录里，可以稍后再装回去。',
  'settings.sandbox.skillTranscript': '查看安装过程',
  'settings.sandbox.skillTranscriptTitle': '安装过程',
  'settings.sandbox.skillUploadAccepted': '已开始安装技能',
  'settings.sandbox.skillDisableHint': '禁用后该技能对智能体不可见，文件仍保留在镜像中。变更将在会话下一次执行时生效。',
  'settings.sandbox.skillEnabled': '已启用技能',
  'settings.sandbox.skillDisabled': '已禁用技能',
  'settings.sandbox.skillEnv.toggle': '环境变量',
  'settings.sandbox.skillEnv.workspaceTitle': '空间共用值',
  'settings.sandbox.skillEnv.workspaceHint': '所有没有填写自己值的成员都会用这里的值。成员可以在「设置 → 沙箱密钥」里填自己的值。',
  'settings.sandbox.skillEnv.required': '必填',
  'settings.sandbox.skillEnv.isSet': '已设置',
  'settings.sandbox.skillEnv.notSet': '未设置',
  'settings.sandbox.skillEnv.placeholderSet': '已存值，填入新值即替换',
  'settings.sandbox.skillEnv.placeholderUnset': '填入值',
  'settings.sandbox.skillEnv.save': '保存',
  'settings.sandbox.skillEnv.saveSuccess': '空间共用值已保存',
  'settings.sandbox.skillEnv.saveFailed': '空间共用值保存失败。',
  'settings.sandbox.skillEnv.clear': '清除',
  'settings.sandbox.skillEnv.clearConfirm': '清除 {name} 的空间共用值？声明会保留，没有自己值的成员之后会缺少这个值。',
  'settings.sandbox.skillEnv.clearSuccess': '已清除空间共用值。',
  'settings.sandbox.skillEnv.valueTooLong': '单个值不能超过 {max} 字节。',
  'settings.skills.manageEnable': '启用',
  'settings.skills.manageUninstall': '从沙箱卸载',
  'settings.skills.manageUninstallConfirm': '确定从该沙箱卸载「{name}」？',
};

function interpolate(template: string, values?: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (match, name: string) => (values && values[name] !== undefined ? String(values[name]) : match));
}

/** t() that falls back to the byte-exact Vue zh-CN copy when packages/i18n has not migrated a key. */
function useSkillT(): (key: string, values?: Record<string, string | number>) => string {
  const locale = useAppLocale();
  const shared = useMemo(() => createTranslator(locale), [locale]);
  return useCallback((key: string, values?: Record<string, string | number>) => {
    const viaShared = shared(key, values);
    if (viaShared !== key) return viaShared;
    const fallback = VUE_SKILL_FALLBACK[key];
    return fallback === undefined ? key : interpolate(fallback, values);
  }, [shared]);
}

function errorText(cause: unknown, fallback: string): string {
  return cause instanceof Error && cause.message ? cause.message : fallback;
}

/** Vue utils/steerId.ts — getRandomValues also works on HTTP deployments. */
function makeSteerClientId(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6]! & 15) | 64;
  bytes[8] = (bytes[8]! & 63) | 128;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

interface DrawerWidthSpec {
  storageKey: string;
  defaultWidth: number;
  minWidth: number;
  maxWidth: number;
}

/** Vue drawer specs (SkillSettings.vue:132-134, 267-269, 316-318), storage keys byte-exact. */
const SKILL_DRAWER_SPECS = {
  add: { storageKey: 'setting-drawer:width:skill-catalog-add', defaultWidth: 680, minWidth: 560, maxWidth: 920 },
  install: { storageKey: 'setting-drawer:width:skill-catalog-install', defaultWidth: 560, minWidth: 480, maxWidth: 760 },
  manage: { storageKey: 'setting-drawer:width:skill-catalog-manage', defaultWidth: 680, minWidth: 560, maxWidth: 920 },
} satisfies Record<string, DrawerWidthSpec>;

/** Vue SettingDrawer.vue:162-166 clampWidth. */
function clampDrawerWidth(width: number, spec: DrawerWidthSpec): number {
  const viewport = typeof window === 'undefined' ? spec.maxWidth : window.innerWidth;
  const cap = Math.min(spec.maxWidth, viewport);
  const floor = Math.min(spec.minWidth, cap);
  return Math.max(floor, Math.min(cap, Math.round(width)));
}

function readStoredDrawerWidth(spec: DrawerWidthSpec): number {
  if (typeof window === 'undefined') return spec.defaultWidth;
  try {
    const raw = window.localStorage.getItem(spec.storageKey);
    const parsed = raw ? Number(raw) : Number.NaN;
    return Number.isFinite(parsed) ? clampDrawerWidth(parsed, spec) : spec.defaultWidth;
  } catch {
    return spec.defaultWidth;
  }
}

/**
 * Vue SettingDrawer drag-resize + persisted width (SettingDrawer.vue:150-248,
 * 499-540) adapted to the centered wk-dialog: the handle sits on the dialog's
 * left edge, drags clamp to [minWidth, min(maxWidth, viewport)] and persist to
 * the same per-title localStorage keys Vue uses.
 */
/* S6：抽屉换 tdesign Dialog（portal 到 body，脱离 DrawerShell 子树），宽度
   改经 context 下发到 dialogClassName/width props（原 [&_.wk-dialog] 后代
   选择器对 body portal 不成立）。 */
const SkillDrawerWidthContext = React.createContext<{ width: number; resizing: boolean }>({ width: 680, resizing: false });
function useSkillDrawerDialog(): { dialogClassName: string; width: string } {
  const { width, resizing } = React.useContext(SkillDrawerWidthContext);
  return { dialogClassName: 'wk-skill-drawer' + (resizing ? ' is-resizing' : ''), width: width + 'px' };
}

const INSTALLER_AGENT_ID = 'builtin-skill-installer';
const LAST_CHAT_MODEL_KEY = 'weknora_last_chat_model_id';
const SKILL_POLL_INTERVAL_MS = 2500;
/** Vue SKILL_ICON（frontend/src/types/mention.ts:4）——卡片徽章与抽屉头图标同名。 */
const SKILL_ICON = 'system-code';

/* Vue SandboxBackendBadge.vue（frontend/src/components/settings/）：同一枚后端
   徽章与 sandbox 面板共用，样式平移块在 settings.td.css（.sandbox-badge 家族）；
   有 mono logo（docker）时用 ::before mask，否则回落 TDesign glyph
   （cube→server、其余→cloud）。 */
function SandboxBackendBadge({ type, size = 'md' }: { type?: string; size?: 'xs' | 'sm' | 'md' }) {
  if (!type) return null;
  const logo = providerLogo('sandbox', type);
  const iconName = type === 'cube' ? 'server' : type === 'disabled' ? 'minus-circle' : 'cloud';
  const className = `sandbox-badge sandbox-badge--${type} sandbox-badge--${size}${logo?.mode === 'mono' ? ' sandbox-badge--mono' : ''}`;
  const style = logo?.mode === 'mono' ? { '--logo-url': `url("${logo.url}")` } as CSSProperties : undefined;
  return <span className={className} style={style} aria-hidden="true">{logo ? null : <TIcon name={iconName} />}</span>;
}

/**
 * Vue SkillSettings.vue:3-12 section-header：20px/600 标题行（标题 + 帮助图标，
 * 8px 间距）叠 14px secondary 描述，28px 底距；帮助是 t-tooltip（placement
 * right）包裹的 t-icon help-circle（16px、placeholder 色、cursor:help，hover
 * 移到 secondary，SkillSettings.vue:1250-1253），弹层内容限宽 340px、行高
 * 1.55（SkillSettings.vue:1269-1272 unscoped :global 块）。
 */
function SkillHelpTooltip({ content }: { content: string }) {
  return (
    <TTooltip content={content} placement="right" overlayClassName="skill-settings__help-tooltip">
      <TIcon name="help-circle" className="section-header__help" aria-label={content} />
    </TTooltip>
  );
}

function SkillSectionHeader({ helpContent }: { helpContent: string }) {
  const t = useSkillT();
  return (
    <div className="section-header">
      <div className="section-header__title-row">
        <h2>{t('settings.skills.title')}</h2>
        <SkillHelpTooltip content={helpContent} />
      </div>
      <p className="section-description">{t('settings.skills.description')}</p>
    </div>
  );
}

/**
 * Vue SettingDrawer header block (SettingDrawer.vue:291-356): leading icon
 * badge (32px, radius 9, brand 10% tint) + 15px/600 title + 12px subtitle,
 * rendered inside the shared Dialog h2.
 */
function readLastChatModelId(): string {
  try {
    return typeof window === 'undefined' ? '' : (window.localStorage.getItem(LAST_CHAT_MODEL_KEY) || '');
  } catch {
    return '';
  }
}

/** Vue installStatusText (SkillSettings.vue:652) as an i18n lookup. */
function installStatusKeys(status: string, enabled: boolean): string[] {
  if (status === 'installing') return ['settings.sandbox.skillStatusInstalling'];
  if (status === 'removing') return ['settings.sandbox.skillStatusRemoving'];
  if (status === 'failed') return ['settings.sandbox.skillStatusFailed'];
  if (status === 'ready') return enabled ? ['settings.sandbox.skillStatusReady'] : ['common.off'];
  return [];
}

/** Vue installChipStatus (SkillSettings.vue:661): ready+enabled+stale shows the outdated copy. */
function installChipStatusKeys(item: SkillCatalog, installation: SkillCatalogInstallation): string[] {
  if (installation.status === 'ready' && installation.enabled && item.bundleSha256 && installation.bundleSha256 && item.bundleSha256 !== installation.bundleSha256) {
    return ['settings.skills.installOutdated'];
  }
  if (installation.status === 'ready' && installation.enabled) return [];
  return installStatusKeys(installation.status, installation.enabled);
}

/** Vue installChipStatusIcon (SkillSettings.vue:669-674): the status glyph shown
 *  next to a single install entry inside the chip panel. */
function installChipStatusIcon(item: SkillCatalog, installation: SkillCatalogInstallation): string {
  if (installation.status === 'failed') return 'close-circle';
  if (installation.status === 'ready' && installation.enabled && item.bundleSha256 && installation.bundleSha256 && item.bundleSha256 !== installation.bundleSha256) return 'error-circle';
  if (installation.status === 'ready' && installation.enabled) return 'check-circle-filled';
  return '';
}

export function SkillSettingsPanel({ client, role, initialSkills, initialCatalog, initialSandboxConfigs }: Props) {
  // SkillSettings.vue is mounted for authenticated admin+ users; keep the
  // system-admin role on the catalog-management branch as well.
  const canEdit = role === 'admin' || role === 'owner' || role === 'system-admin';
  const t = useSkillT();
  if (!canEdit) {
    return <Card data-testid="skill-settings"><h3>{t('settings.skills.title')}</h3><p className="wk-muted">{t('settings.skills.description')}</p>{initialSkills && initialSkills.length > 0 ? <ul className="wk-list">{initialSkills.map((skill) => <li key={skill.id} className="wk-skill-viewer-row"><strong>{skill.name}</strong><span className="wk-skill-viewer-desc">{skill.description ?? t('settings.skills.emptyDesc')}</span></li>)}</ul> : <Status>{t('settings.skills.noInstalls')}</Status>}</Card>;
  }
  return <SkillCatalogSection client={client} initialCatalog={initialCatalog} initialSandboxConfigs={initialSandboxConfigs} />;
}

type InstallerModel = {
  modelId: string;
  models: readonly ModelConfiguration[];
  saving: boolean;
  onChange: (modelId: string) => void;
  /** Merges model_id into the builtin installer agent config (SkillSettings.vue:903-918). */
  persist: (modelId: string) => Promise<void>;
};

export function SkillCatalogSection({ client, initialCatalog, initialSandboxConfigs }: { client: WeKnoraClient; initialCatalog?: readonly SkillCatalog[]; initialSandboxConfigs?: readonly SandboxConfigRecord[] }) {
  const t = useSkillT();
  const [loading, setLoading] = useState<boolean>(!initialCatalog || !initialSandboxConfigs);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [records, setRecords] = useState<SandboxConfigRecord[]>(initialSandboxConfigs ? [...initialSandboxConfigs] : []);
  const [catalog, setCatalog] = useState<SkillCatalog[]>(initialCatalog ? [...initialCatalog] : []);
  const [focusedCatalogId, setFocusedCatalogId] = useState('');
  const [deletingId, setDeletingId] = useState('');
  const [openPanelId, setOpenPanelId] = useState('');
  const [pendingDelete, setPendingDelete] = useState<SkillCatalog | null>(null);
  const [wizardOpen, setWizardOpen] = useState(false);
  const [installItem, setInstallItem] = useState<SkillCatalog | null>(null);
  const [installPreselectId, setInstallPreselectId] = useState('');
  const [manageTarget, setManageTarget] = useState<{ record: SandboxConfigRecord; skillId: string; catalogName: string } | null>(null);
  const [filesTarget, setFilesTarget] = useState<{ id: string; name: string } | null>(null);
  const focusTimer = useRef<number | null>(null);

  /** Vue MessagePlugin 语义经由 Settings 域 toast 总线呈现（R472 A2）。 */
  const onToast = useCallback((tone: 'success' | 'warning' | 'error', message: string) => {
    pushSettingsToast(message, tone);
  }, []);

  const skillConfigs = useMemo(() => records.filter((record) => isNamedSandboxBackend(record.sandbox_type)), [records]);

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    if (!silent) setLoadError(null);
    try {
      const [configResult, catalogResult] = await Promise.all([
        client.sandboxConfigurations.list(),
        client.configuration.skills.catalog.list(),
      ]);
      setRecords(configResult.items);
      setCatalog(catalogResult);
    } catch (cause) {
      if (!silent) {
        // Vue SkillSettings.vue:1163 — MessagePlugin.error(e?.message ||
        // t('settings.skills.loadFailed'))：后端原文优先、本地化兜底；
        // 列表区由中央空态 + 重试替代（R472 A2）。
        const message = errorText(cause, t('settings.skills.loadFailed'));
        setLoadError(message);
        pushSettingsToast(message);
      }
    } finally {
      if (!silent) setLoading(false);
    }
  }, [client, t]);

  useEffect(() => {
    if (initialCatalog && initialSandboxConfigs) return;
    void load();
  }, [initialCatalog, initialSandboxConfigs, load]);

  // Vue polls the catalog every 2.5s while any installation is installing/removing (SkillSettings.vue:1123-1145).
  const catalogBusy = catalog.some((item) => liveCatalogInstalls(item).some(isInstallBusy));
  useEffect(() => {
    if (!catalogBusy || typeof window === 'undefined') return undefined;
    const timer = window.setInterval(() => void load(true), SKILL_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [catalogBusy, load]);

  useEffect(() => () => { if (focusTimer.current != null && typeof window !== 'undefined') window.clearTimeout(focusTimer.current); }, []);

  // Installer agent + model selection shared by the add wizard and the install drawer (SkillSettings.vue:891-931).
  const [installerAgent, setInstallerAgent] = useState<AgentConfiguration | null>(null);
  const [installerModelId, setInstallerModelId] = useState('');
  const [models, setModels] = useState<readonly ModelConfiguration[]>([]);
  const [savingInstallerModel, setSavingInstallerModel] = useState(false);

  const loadInstallerModel = useCallback(async () => {
    try {
      const agent = await client.configuration.agents.get(INSTALLER_AGENT_ID);
      setInstallerAgent(agent);
      const configured = typeof agent.config?.model_id === 'string' ? (agent.config.model_id as string).trim() : '';
      setInstallerModelId(configured || readLastChatModelId());
    } catch {
      setInstallerAgent(null);
      setInstallerModelId(readLastChatModelId());
    }
  }, [client]);

  useEffect(() => {
    void loadInstallerModel();
    void client.configuration.models.list().then(setModels).catch(() => setModels([]));
  }, [client, loadInstallerModel]);

  const persistInstallerModel = useCallback(async (modelId: string) => {
    const id = modelId.trim();
    if (!id) throw new Error(t('settings.sandbox.skillInstallerModelRequired'));
    const current = installerAgent;
    const config = { ...(current?.config ?? {}), model_id: id };
    const updated = await client.configuration.agents.update(INSTALLER_AGENT_ID, {
      name: current?.name ?? '',
      description: current?.description ?? '',
      avatar: current?.avatar ?? '',
      config,
    });
    setInstallerAgent(updated);
    setInstallerModelId(id);
  }, [client, installerAgent, t]);

  const onInstallerModelChange = useCallback(async (modelId: string) => {
    if (!modelId) return;
    setInstallerModelId(modelId);
    setSavingInstallerModel(true);
    try {
      await persistInstallerModel(modelId);
    } catch (cause) {
      pushSettingsToast(errorText(cause, t('settings.sandbox.skillInstallerModelSaveFailed')), 'error');
    } finally {
      setSavingInstallerModel(false);
    }
  }, [persistInstallerModel, t]);

  const installer: InstallerModel = useMemo(() => ({
    modelId: installerModelId,
    models: models.filter((model) => model.type === 'KnowledgeQA'),
    saving: savingInstallerModel,
    onChange: (id: string) => void onInstallerModelChange(id),
    persist: (id: string) => persistInstallerModel(id),
  }), [installerModelId, models, savingInstallerModel, onInstallerModelChange, persistInstallerModel]);

  function revealCatalog(id: string) {
    setFocusedCatalogId(id);
    if (focusTimer.current != null && typeof window !== 'undefined') window.clearTimeout(focusTimer.current);
    focusTimer.current = typeof window === 'undefined' ? null : window.setTimeout(() => setFocusedCatalogId((current) => (current === id ? '' : current)), 2400);
  }

  function recordFor(id: string): SandboxConfigRecord | undefined {
    return records.find((record) => record.id === id);
  }

  function backendLabel(type: string | undefined): string {
    return type ? t(backendLabelKey(type)) : '';
  }

  function sandboxMetaLine(record: SandboxConfigRecord): string {
    const label = backendLabel(record.sandbox_type);
    const target = sandboxTargetLine(record);
    return target ? `${label} · ${target}` : label;
  }

  function installTooltip(item: SkillCatalog, installation: SkillCatalogInstallation): string {
    const statusKeys = installChipStatusKeys(item, installation);
    const statusText = statusKeys.length > 0 ? statusKeys.map((key) => t(key)).join(' ') : installStatusKeys(installation.status, installation.enabled).map((key) => t(key)).join(' ');
    return [installName(installation), installation.sandboxType ? backendLabel(installation.sandboxType) : '', statusText].filter(Boolean).join(' · ');
  }

  function installSummary(item: SkillCatalog, installs: readonly SkillCatalogInstallation[]): string {
    if (installs.length === 0) return t('settings.skills.installToSandbox');
    if (installs.length === 1) return t('settings.skills.installedOnName', { name: installName(installs[0]!) });
    return t('settings.skills.installedCount', { count: installs.length });
  }

  async function removeCatalog(item: SkillCatalog) {
    if (!canDeleteCatalog(item)) {
      pushSettingsToast(t('settings.skills.deleteCatalogBlocked'), 'warning');
      return;
    }
    setDeletingId(item.id);
    try {
      await client.configuration.skills.catalog.remove(item.id);
      pushSettingsToast(t('settings.skills.deleteSuccess'), 'success');
      await load(true);
    } catch (cause) {
      pushSettingsToast(errorText(cause, t('common.deleteFailed')), 'error');
    } finally {
      setDeletingId('');
    }
  }

  function openManage(item: SkillCatalog, installation: SkillCatalogInstallation) {
    const record = recordFor(installation.sandboxConfigId);
    if (!record) return;
    setOpenPanelId('');
    setManageTarget({ record, skillId: installation.skillId, catalogName: item.name });
  }

  function openInstall(item: SkillCatalog, preselectConfigId = '') {
    setOpenPanelId('');
    setInstallPreselectId(preselectConfigId);
    setInstallItem(item);
  }

  /** Vue setInstallPanel (SkillSettings.vue:747-753)：受控 t-popup 的回写。 */
  function setInstallPanel(id: string, visible: boolean) {
    if (visible) {
      setOpenPanelId(id);
      return;
    }
    setOpenPanelId((current) => (current === id ? '' : current));
  }

  /* Vue SkillSettings.vue:2-130 列表域逐节点平移：section-header →
     loading-container（t-loading）→ 空态（t-empty + hint + actions）/ skill-list
     卡片网格（badge t-icon system-code / 图标按钮 folder+delete / chip 三分支：
     label / 直达按钮 / t-popup 安装面板）→ skill-card--add 虚线卡。样式在
     settings.td.css skills §。 */
  const empty = catalog.length === 0;
  return <div className="skill-settings" data-testid="skill-settings">
    <SkillSectionHeader helpContent={t('settings.skills.helpTooltip')} />
    {loading ? <div className="loading-container"><TLoading text={t('common.loading')} /></div> : loadError ? (
      /* R472 A2 — Vue SkillSettings.vue 加载失败：Toast + 空态 + 重试；
         标题/说明保持渲染，列表区被空态替代（DOM 沿用 Vue .empty-state 形状）。 */
      <div className="empty-state" data-testid="settings-load-empty">
        <TEmpty description={loadError} />
        <div className="empty-actions">
          <TButton theme="primary" onClick={() => { void load(); }}>{t('common.retry')}</TButton>
        </div>
      </div>
    ) : empty ? (
      <div className="empty-state">
        <TEmpty description={t('settings.skills.emptyDesc')} />
        {skillConfigs.length === 0 ? <p className="empty-hint">{t('settings.skills.emptyNoSandboxHint')}</p> : null}
        <div className="empty-actions">
          <TButton theme="primary" onClick={() => { setWizardOpen(true); }}>{t('settings.skills.addSkill')}</TButton>
          {skillConfigs.length === 0
            ? <TButton theme="default" variant="outline" onClick={() => navigate('/platform/settings?section=sandbox')}>{t('settings.skills.goSandboxSettings')}</TButton>
            : null}
        </div>
      </div>
    ) : (
      <div className="skill-list">
        {catalog.map((item) => {
          const view = installsView(item, skillConfigs);
          const live = view.installs.length > 0;
          const chipTone = view.installs[0] ? installEntryTone(item, view.installs[0]) : null;
          const summary = installSummary(item, view.installs);
          const tooltipLines = [
            ...view.installs.map((installation) => installTooltip(item, installation)),
            ...view.available.map((config) => `${config.name} · ${t('settings.skills.installPanelAvailable')}`),
          ];
          const chipTooltip = tooltipLines.length === 0 ? t('settings.skills.installToSandbox') : tooltipLines.join('\n');
          // Vue chipClass (SkillSettings.vue:718-723)：idle/installed 轴 + 首装状态轴。
          const chipClass = [
            'skill-card__chip',
            live ? 'skill-card__chip--installed' : 'skill-card__chip--idle',
            chipTone ? `skill-card__entry--${chipTone}` : '',
          ].filter(Boolean).join(' ');
          return <article key={item.id} className={[
            'skill-card',
            focusedCatalogId === item.id ? 'skill-card--focused' : '',
            live ? 'skill-card--installed' : 'skill-card--idle',
          ].filter(Boolean).join(' ')}>
            <div className="skill-card__main">
              <div className="skill-card__body">
                <div className="skill-card__header">
                  <div className="skill-card__badge" aria-hidden="true">
                    <TIcon name={SKILL_ICON} size="14px" />
                  </div>
                  <div className="skill-card__heading">
                    <h3 className="skill-card__title" title={item.name}>{item.name}</h3>
                    {item.version ? <span className="skill-card__type">{item.version}</span> : null}
                  </div>
                  <div className="skill-card__actions">
                    <button type="button" className="skill-card__icon-btn" title={t('settings.sandbox.skillFiles')} aria-label={t('settings.sandbox.skillFiles')} onClick={() => setFilesTarget({ id: item.id, name: item.name })}>
                      <FolderIcon size="14px" />
                    </button>
                    {canDeleteCatalog(item)
                      ? <button type="button" className="skill-card__icon-btn skill-card__icon-btn--danger" disabled={deletingId === item.id} title={t('settings.skills.deleteCatalog')} aria-label={t('settings.skills.deleteCatalog')} onClick={() => setPendingDelete(item)}>
                        <DeleteIcon size="14px" />
                      </button>
                      : null}
                  </div>
                </div>
                {item.description ? <p className="skill-card__desc" title={item.description}>{compactSkillText(item.description)}</p> : null}
                <div className="skill-card__installs">
                  {view.installs.length === 0 && !view.canAdd ? <span className="skill-card__installs-label">{t('settings.skills.noInstalls')}</span>
                    : !view.needsPanel ? (
                      <button type="button" className={chipClass} disabled={Boolean(view.installs[0] && !recordFor(view.installs[0].sandboxConfigId))} title={chipTooltip} aria-label={summary}
                        onClick={() => { if (view.installs[0]) openManage(item, view.installs[0]); else if (view.canAdd) openInstall(item); }}>
                        {view.installs.some(isInstallBusy) ? <span className="skill-card__entry-dot" aria-hidden="true" /> : null}
                        <span className="skill-card__chip-text">{summary}</span>
                        <TIcon name="chevron-right" size="14px" className="skill-card__chip-go" />
                      </button>
                    ) : (
                      <TPopup
                        visible={openPanelId === item.id}
                        trigger="click"
                        placement="bottom-left"
                        attach="body"
                        destroyOnClose
                        overlayClassName="skill-install-panel-overlay"
                        overlayInnerStyle={{ padding: 0 }}
                        onVisibleChange={(visible: boolean) => setInstallPanel(item.id, visible)}
                        content={<div className="skill-install-panel">
                          {view.installs.length > 0 ? <>
                            <p className="skill-install-panel__group">{t('settings.skills.installPanelGroup')}</p>
                            {view.installs.map((installation) => (
                              <button key={installation.sandboxConfigId} type="button"
                                className={`skill-install-panel__item skill-card__entry--${installEntryTone(item, installation)}`}
                                disabled={!recordFor(installation.sandboxConfigId)} title={installTooltip(item, installation)}
                                onClick={() => openManage(item, installation)}>
                                <SandboxBackendBadge type={installation.sandboxType} size="xs" />
                                <span className="skill-install-panel__name">{installName(installation)}</span>
                                {isInstallBusy(installation)
                                  ? <span className="skill-card__entry-dot" aria-hidden="true" />
                                  : installChipStatusIcon(item, installation)
                                    ? <TIcon name={installChipStatusIcon(item, installation)} size="14px" className="skill-card__entry-status" />
                                    : null}
                              </button>
                            ))}
                          </> : null}
                          {view.available.length > 0 ? <>
                            {view.installs.length > 0 ? <div className="skill-install-panel__split" role="separator" /> : null}
                            <p className="skill-install-panel__group">{t('settings.skills.installPanelAvailable')}</p>
                            {view.available.map((config) => (
                              <button key={config.id} type="button" className="skill-install-panel__item skill-install-panel__item--available"
                                title={sandboxMetaLine(config)} onClick={() => openInstall(item, config.id)}>
                                <SandboxBackendBadge type={config.sandbox_type} size="xs" />
                                <span className="skill-install-panel__name">{config.name}</span>
                                <TIcon name="add" size="14px" className="skill-install-panel__add" />
                              </button>
                            ))}
                          </> : null}
                        </div>}
                      >
                        <button type="button" className={chipClass} title={chipTooltip} aria-label={summary} aria-expanded={openPanelId === item.id}>
                          {view.installs.some(isInstallBusy) ? <span className="skill-card__entry-dot" aria-hidden="true" /> : null}
                          <span className="skill-card__chip-text">{summary}</span>
                          <TIcon name="chevron-down" size="14px" className="skill-card__chip-go" />
                        </button>
                      </TPopup>
                    )}
                </div>
              </div>
            </div>
          </article>;
        })}
        <button type="button" className="skill-card skill-card--add" onClick={() => setWizardOpen(true)}>
          <span className="skill-card--add__icon" aria-hidden="true">
            <AddIcon />
          </span>
          <span className="skill-card--add__label">{t('settings.skills.addSkill')}</span>
        </button>
      </div>
    )}
    <AddSkillWizard client={client} open={wizardOpen} catalog={catalog} configs={skillConfigs} installer={installer} t={t}
      onClose={(registeredId) => { setWizardOpen(false); void load(true); if (registeredId) revealCatalog(registeredId); }}
      onCatalogChanged={() => void load(true)}
      onToast={onToast}
      onManage={(record, skillId, catalogName) => { setWizardOpen(false); setManageTarget({ record, skillId, catalogName }); }} />
    <InstallSkillDialog client={client} open={installItem !== null} item={installItem} configs={skillConfigs} preselectConfigId={installPreselectId} installer={installer} t={t}
      onClose={() => { setInstallItem(null); setInstallPreselectId(''); }}
      onCatalogChanged={() => void load(true)}
      onToast={onToast}
      onManage={(record, skillId, catalogName) => { setInstallItem(null); setManageTarget({ record, skillId, catalogName }); }} />
    <ManageSkillDialog client={client} open={manageTarget !== null} target={manageTarget} t={t}
      onClose={() => setManageTarget(null)}
      onChanged={() => void load(true)}
      onToast={onToast} />
    <CatalogFilesDialog client={client} open={filesTarget !== null} target={filesTarget} t={t} onClose={() => setFilesTarget(null)} />
    <TDialog footer={false} visible={pendingDelete !== null} header={t('common.confirmDelete')} onClose={() => setPendingDelete(null)}>
      {pendingDelete ? <>
        <p>{t('settings.skills.deleteCatalogConfirm', { name: pendingDelete.name })}</p>
        <div className="wk-list-actions">
          <TButton type="button" onClick={() => setPendingDelete(null)}>{t('common.cancel')}</TButton>
          <TButton type="button" loading={deletingId !== ''} onClick={() => { const item = pendingDelete; setPendingDelete(null); void removeCatalog(item); }}>{t('common.delete')}</TButton>
        </div>
      </> : null}
    </TDialog>
  </div>;
}

function skillRegisterErrorMessage(cause: unknown, t: (key: string, values?: Record<string, string | number>) => string, fromFile: boolean): string {
  const raw = cause instanceof Error ? cause.message : String(cause ?? '');
  const classified = classifySkillRegisterError(raw);
  if (classified) {
    if (classified.kind === 'bundleTooLarge') return t('settings.sandbox.skillBundleTooLarge', { size: maxSkillBundleMB() });
    if (classified.kind === 'bundleTooManyFiles') return t('settings.sandbox.skillBundleTooManyFiles', { count: classified.count ?? '' });
    return t('settings.sandbox.skillBundleTooManyZipEntries', { count: classified.count ?? '' });
  }
  if (raw) return raw;
  return fromFile ? t('settings.sandbox.skillUploadFailed') : t('settings.sandbox.skillSourceFailed');
}

function catalogInstallFailedCount(errors: Record<string, string> | undefined): number {
  return Object.keys(errors ?? {}).length;
}

/** Installs require a persisted installer model before the request leaves (SkillSettings.vue:933-939). */
async function ensureInstallerModel(installer: InstallerModel, targets: readonly string[], t: (key: string) => string): Promise<void> {
  if (targets.length === 0) return;
  if (!installer.modelId) throw new Error(t('settings.sandbox.skillInstallerModelRequired'));
  await installer.persist(installer.modelId);
}

function InstallerModelSelect({ installer, t }: { installer: InstallerModel; t: (key: string) => string }) {
  return <label className="wk-skill-label">{t('settings.sandbox.skillInstallerModel')}
    <TSelect className="wk-skill-sel-installer" value={installer.modelId} disabled={installer.saving}
      options={[{ value: '', label: t('settings.sandbox.skillInstallerModelRequired') }, ...installer.models.map((model) => ({ value: model.id, label: model.name }))]}
      onChange={(value) => installer.onChange(String(value))} />
  </label>;
}

function SandboxPickList({ client, item, configs, mode, sessionIds, targetIds, onToggle, onManage, t, metaLine }: {
  client: WeKnoraClient;
  item: SkillCatalog | null;
  configs: readonly SandboxConfigRecord[];
  mode: 'remaining' | 'all';
  sessionIds: readonly string[];
  targetIds: readonly string[];
  onToggle: (configId: string, checked: boolean) => void;
  onManage: (record: SandboxConfigRecord, installation: SkillCatalogInstallation) => void;
  t: (key: string, values?: Record<string, string | number>) => string;
  metaLine: (record: SandboxConfigRecord) => string;
}) {
  const rows = useMemo(() => sandboxPickRows(item, configs, mode, sessionIds), [configs, item, mode, sessionIds]);
  const [progressByConfig, setProgressByConfig] = useState<Record<string, SkillInstallProgressEvent | undefined>>({});

  // Vue's progressById fan-out follows every busy pick row, not only the
  // currently focused manage drawer (SandboxSkillsPanel.vue:1089-1174).
  useEffect(() => {
    const busyRows = rows.filter((row) => row.busy && row.install?.skillId);
    if (busyRows.length === 0) {
      setProgressByConfig((current) => Object.keys(current).length === 0 ? current : {});
      return undefined;
    }
    let active = true;
    const controllers = busyRows.map((row) => {
      const controller = new AbortController();
      void client.sandbox.skills.followInstallEvents(row.config.id, row.install!.skillId, (event) => {
        if (!active) return;
        setProgressByConfig((current) => ({ ...current, [row.config.id]: event.event }));
      }, controller.signal).catch(() => { /* status fallback remains visible */ });
      return controller;
    });
    return () => {
      active = false;
      controllers.forEach((controller) => controller.abort());
    };
  }, [client, rows]);
  if (rows.length === 0) return <p className="wk-muted">{t('settings.skills.noSandboxToInstall')}</p>;
  // Vue .sandbox-pick-list rows (SkillSettings.vue:1841-1953): 1px #e7e7e7
  // border, radius 10, 10px/12px padding; hover/checked tints are 40%/4-5%
  // brand mixes; busy rows take a 35% warning border.
  const pickRowBase = 'sandbox-pick-row';
  return <div className="sandbox-pick-list">
    {rows.map((row) => row.selectable ? (
      <label key={row.config.id} className={`sandbox-pick-row--selectable ${pickRowBase}`}>
        <TCheckbox checked={targetIds.includes(row.config.id)} onChange={(value) => onToggle(row.config.id, Boolean(value))} />
        <span className="sandbox-pick-row__main">
          <SandboxBackendBadge type={row.config.sandbox_type} size="sm" />
          <span className="sandbox-pick-row__text"><span className="sandbox-pick-row__name">{row.config.name}</span><span className="sandbox-pick-row__meta">{metaLine(row.config)}</span></span>
        </span>
      </label>
    ) : (
      <div key={row.config.id} className={`${pickRowBase}${row.busy ? ' is-busy' : ''}`}>
        <span className="sandbox-pick-row__main">
          <SandboxBackendBadge type={row.config.sandbox_type} size="sm" />
          <span className="sandbox-pick-row__text">
            <span className="sandbox-pick-row__name">{row.config.name}</span>
            <span className="sandbox-pick-row__meta">{row.install && isInstallBusy(row.install) ? installStatusKeys(row.install.status, row.install.enabled).map((key) => t(key)).join(' ') : row.ready ? t('settings.sandbox.skillStatusReady') : metaLine(row.config)}</span>
          </span>
        </span>
        {row.busy && row.install ? <div className="sandbox-pick-row__busy">
          <ProgressRing percent={installProgressPercent(progressByConfig[row.config.id], row.install.status)} />
          {progressByConfig[row.config.id] ? <span>{installProgressPercent(progressByConfig[row.config.id], row.install.status)}%</span> : null}
          <TButton type="button" variant="text" size="small" className="wk-skill-progress-link" onClick={() => onManage(row.config, row.install!)}>{t('settings.skills.viewInstallProgress')}</TButton>
        </div> : null}
      </div>
    ))}
  </div>;
}

/** Byte-level XHR progress -> percent, exactly the Vue api math (frontend/src/api/system/index.ts:1117)
 *  plus the KnowledgeBaseList.vue:1601 clamp. */
export function uploadPercentFromProgress(progress: { loaded: number; total: number }): number {
  if (!progress.total || progress.total <= 0) return 0;
  return Math.min(100, Math.max(0, Math.round((progress.loaded * 100) / progress.total)));
}

/** Upload slice of the add-skill drawer: percent text + small bar
 *  (SkillSettings.vue:208 t-progress + SandboxSkillsPanel.vue:64-72 skillUploading text). */
export function SkillUploadProgress({ percent, t }: {
  percent: number;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  return (
    <div className="skill-upload-progress" role="status">
      <span className="wk-skill-dropzone__selected">{t('settings.sandbox.skillUploading', { percent })}</span>
      <div
        className="skill-upload-progress__bar"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
      >
        <div className="skill-upload-progress__fill" style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

/** Registers the catalog zip with byte-level upload progress. The panel mints
 *  the blob: source itself so the web transport can key its progress observer
 *  (the api-client bridge passes a NativeFileSource through untouched). */
export async function registerSkillCatalogWithProgress(
  client: WeKnoraClient,
  file: File,
  onPercent: (percent: number) => void,
  signal?: AbortSignal,
): Promise<SkillCatalog> {
  const uri = URL.createObjectURL(file);
  const stopObserving = observeUploadProgress(uri, (progress) => onPercent(uploadPercentFromProgress(progress)));
  try {
    return await client.configuration.skills.catalog.register({
      file: { uri, name: file.name || 'file', type: file.type || 'application/zip', size: file.size },
      ...(signal === undefined ? {} : { signal }),
    });
  } finally {
    stopObserving();
  }
}
/** Two-step add drawer: register (source or zip) then pick sandboxes (SkillSettings.vue:132-265, 981-1060). */
function AddSkillWizard({ client, open, catalog, configs, installer, t, onClose, onCatalogChanged, onToast, onManage }: {
  client: WeKnoraClient;
  open: boolean;
  catalog: readonly SkillCatalog[];
  configs: readonly SandboxConfigRecord[];
  installer: InstallerModel;
  t: (key: string, values?: Record<string, string | number>) => string;
  onClose: (registeredCatalogId?: string) => void;
  onCatalogChanged: () => void;
  onToast: (tone: 'success' | 'warning' | 'error', message: string) => void;
  onManage: (record: SandboxConfigRecord, skillId: string, catalogName: string) => void;
}) {

  const [step, setStep] = useState(0);
  const [registeredId, setRegisteredId] = useState('');
  const [source, setSource] = useState('');
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadPercent, setUploadPercent] = useState(0);
  const [addingFromSource, setAddingFromSource] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [targetIds, setTargetIds] = useState<string[]>([]);
  const [sessionIds, setSessionIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) return;
    setStep(0); setRegisteredId(''); setSource(''); setPendingFile(null);
    setTargetIds([]); setSessionIds([]); setError(null); setUploading(false); setUploadPercent(0); setAddingFromSource(false);
  }, [open]);

  const addBusy = uploading || addingFromSource;
  const registered = registeredId ? catalog.find((item) => item.id === registeredId) ?? null : null;
  const registeredFallback = useRef<SkillCatalog | null>(null);
  const parsedCard = registered ?? registeredFallback.current;
  const pickItem = parsedCard;
  const defaultTargets = configs.length === 1 ? [configs[0]!.id] : [];
  const rows = sandboxPickRows(pickItem, configs, 'all', sessionIds);

  const primaryLoading = step === 0 ? addBusy : installing;
  const primaryDisabled = addBusy || installing
    || (step === 0 ? (registeredId ? false : !source.trim() && !pendingFile)
      : targetIds.length > 0 && !installer.modelId);
  const primaryText = step === 0 ? t('common.next') : targetIds.length > 0 ? t('settings.skills.installToSandbox') : t('settings.skills.addFinish');
  const stepDescription = step === 0 ? t('settings.skills.addStepRegisterDesc') : t('settings.skills.addStepInstallDesc');
  const steps = [t('settings.skills.addStepRegister'), t('settings.skills.addStepInstall')];

  function canJump(index: number): boolean {
    if (index === step) return false;
    return Boolean(registeredId) || index < step;
  }

  function setPick(configId: string, checked: boolean) {
    setTargetIds((current) => (checked ? [...new Set([...current, configId])] : current.filter((id) => id !== configId)));
  }

  function acceptFile(file: File | null) {
    if (addBusy || registeredId) return;
    if (!file) return;
    if (!isZipBundle(file)) {
      setError(t('settings.sandbox.skillUploadFailed'));
      return;
    }
    if (file.size > maxSkillBundleMB() * 1024 * 1024) {
      setError(t('settings.sandbox.skillBundleTooLarge', { size: maxSkillBundleMB() }));
      return;
    }
    setError(null);
    setPendingFile(file);
  }

  async function registerThenAdvance() {
    if (registeredId) {
      setTargetIds(defaultTargets);
      setStep(1);
      return;
    }
    const trimmed = source.trim();
    if (!pendingFile && !trimmed) return;
    try {
      let result: SkillCatalog;
      if (pendingFile) {
        setUploading(true);
        setUploadPercent(0);
        result = await registerSkillCatalogWithProgress(client, pendingFile, setUploadPercent);
      } else {
        setAddingFromSource(true);
        result = await client.configuration.skills.catalog.register({ source: trimmed });
      }
      registeredFallback.current = result;
      setRegisteredId(result.id);
      setTargetIds(defaultTargets);
      setStep(1);
      setSource('');
      setPendingFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      onToast('success', t('settings.skills.registerAccepted'));
      onCatalogChanged();
    } catch (cause) {
      setError(skillRegisterErrorMessage(cause, t, Boolean(pendingFile)));
    } finally {
      setUploading(false);
      setUploadPercent(0);
      setAddingFromSource(false);
    }
  }

  async function handlePrimary() {
    if (primaryLoading || primaryDisabled) return;
    if (step === 0) {
      await registerThenAdvance();
      return;
    }
    if (!registeredId) return;
    const targets = [...targetIds];
    if (targets.length === 0) {
      onClose(registeredId);
      return;
    }
    setInstalling(true);
    setError(null);
    try {
      await ensureInstallerModel(installer, targets, t);
      const result = await client.configuration.skills.catalog.install(registeredId, targets);
      const failed = catalogInstallFailedCount(result.errors);
      if (failed > 0) onToast('warning', t('settings.skills.installPartial', { failed }));
      else onToast('success', t('settings.skills.installAccepted'));
      setSessionIds((current) => [...new Set([...current, ...targets])]);
      onCatalogChanged();
      setTargetIds((current) => current.filter((id) => rows.some((row) => row.selectable && row.config.id === id)));
    } catch (cause) {
      setError(errorText(cause, t('settings.sandbox.skillUploadFailed')));
    } finally {
      setInstalling(false);
    }
  }

  /* Vue SkillSettings.vue:132-265 添加向导走 SettingDrawer：#header-extra
     步骤条、#footer-left 上一步、confirm 按主步语义（保存并下一步/安装）。 */
  return <SettingDrawer
    visible={open}
    title={t('settings.skills.addSkill')}
    description={stepDescription}
    icon={SKILL_ICON}
    width="680px" minWidth={560} maxWidth={920}
    storageKey={SKILL_DRAWER_SPECS.add.storageKey}
    confirmLoading={primaryLoading}
    confirmDisabled={primaryDisabled}
    confirmText={primaryText}
    headerExtra={<nav className="skill-add-steps" aria-label={t('settings.skills.addProgress')}>
      {steps.map((title, index) => {
        const clickable = canJump(index);
        /* Vue component :is（SkillSettings.vue:138-143）：可跳转才渲染 button，
           否则 div——避免浏览器 button 默认样式（边框/内边距/13.33px 字号）
           撑高步骤条（与 Vue 24px 行高差 6px，连带 body 整体下移）。 */
        const marker = <span className="skill-add-step__marker">{step > index ? <TIcon name="check" /> : index + 1}</span>;
        const titleSpan = <span className="skill-add-step__title">{title}</span>;
        const line = index < steps.length - 1 ? <span className="skill-add-step__line" aria-hidden="true" /> : null;
        const cls = `skill-add-step${step === index ? ' is-active' : ''}${step > index ? ' is-done' : ''}${clickable ? ' is-clickable' : ''}`;
        return clickable
          ? <button key={title} type="button" aria-current={step === index ? 'step' : undefined} className={cls}
              onClick={() => setStep(index)}>
              {marker}{titleSpan}{line}
            </button>
          : <div key={title} className={cls} aria-current={step === index ? 'step' : undefined}>
              {marker}{titleSpan}{line}
            </div>;
      })}
    </nav>}
    footerLeft={step > 0 ? <TButton variant="outline" onClick={() => setStep((current) => Math.max(0, current - 1))}>{t('settings.sandbox.back')}</TButton> : undefined}
    onConfirm={() => void handlePrimary()}
    onVisibleChange={(visible) => { if (!visible) onClose(registeredId || undefined); }}
  >
    {error ? <Status tone="error">{error}</Status> : null}
    {step > 0 && parsedCard ? <article className="skill-card parsed-skill">
      <div className="skill-card__main">
        <div className="skill-card__body">
          <div className="skill-card__header">
            <div className="skill-card__badge" aria-hidden="true"><TIcon name={SKILL_ICON} size="14px" /></div>
            <h3 className="skill-card__title" title={parsedCard.name}>{parsedCard.name}</h3>
            {parsedCard.version ? <span className="skill-card__type">{parsedCard.version}</span> : null}
          </div>
          {parsedCard.description ? <p className="skill-card__desc" title={parsedCard.description}>{compactSkillText(parsedCard.description)}</p> : null}
        </div>
      </div>
    </article> : null}
    {step === 0 ? <>
      <section className="setting-drawer__section">
        <h4 className="setting-drawer__section-title">{t('settings.sandbox.skillSourceSection')}</h4>
        <p className="installer-model-hint">{t('settings.sandbox.skillSourceSectionHint', { size: maxSkillBundleMB() })}</p>
        <TInput value={source} placeholder={t('settings.sandbox.skillSourcePlaceholder')} disabled={addBusy || Boolean(registeredId)} onChange={(value) => setSource(String(value))} />
      </section>
      <section className="setting-drawer__section">
        <h4 className="setting-drawer__section-title">{t('settings.sandbox.skillUploadSection')}</h4>
        <p className="installer-model-hint">{t('settings.sandbox.skillUploadSectionHint', { size: maxSkillBundleMB() })}</p>
        <input ref={fileInputRef} type="file" accept=".zip,application/zip" className="file-input-hidden" disabled={addBusy || Boolean(registeredId)} onChange={(event) => acceptFile(event.currentTarget.files?.[0] ?? null)} />
        <div className={'file-upload-area file-upload-area--large' + (pendingFile ? ' has-file' : '') + (addBusy || registeredId ? ' is-disabled' : '')}
          onClick={() => { if (!addBusy && !registeredId) fileInputRef.current?.click(); }}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => { event.preventDefault(); acceptFile(event.dataTransfer.files?.[0] ?? null); }}>
          <div className="file-upload-content">
            <div className="file-upload-icon-wrap" aria-hidden="true"><TIcon name="cloud-upload" size="32px" className="upload-icon" /></div>
            <div className="upload-text">
              {uploading ? <SkillUploadProgress percent={uploadPercent} t={t} /> : pendingFile ? <span className="upload-file-name">{t('settings.skills.addFileSelected', { name: pendingFile.name })}</span> : <>
                <span className="upload-primary-text">{t('settings.sandbox.skillUploadClick')}</span>
                <span className="upload-secondary-text">{t('settings.sandbox.skillUploadDrag')}</span>
              </>}
            </div>
          </div>
        </div>
        {pendingFile && !registeredId ? <TButton type="button" variant="text" size="small" disabled={addBusy} onClick={() => { setPendingFile(null); if (fileInputRef.current) fileInputRef.current.value = ''; }}>{t('settings.skills.addClearFile')}</TButton> : null}
      </section>
    </> : <>
      {configs.length > 0 ? <section className="setting-drawer__section">
        <h4 className="setting-drawer__section-title">{t('settings.skills.pickSandboxes')}</h4>
        <p className="installer-model-hint">{t('settings.skills.pickSandboxesHint')}</p>
        <SandboxPickList client={client} item={pickItem} configs={configs} mode="all" sessionIds={sessionIds} targetIds={targetIds} onToggle={setPick} t={t}
          metaLine={(record) => { const label = t(backendLabelKey(record.sandbox_type)); const target = sandboxTargetLine(record); return target ? `${label} · ${target}` : label; }}
          onManage={(record, installation) => { if (installation.skillId) onManage(record, installation.skillId, parsedCard?.name ?? ''); }} />
      </section> : <p className="installer-model-hint">{t('settings.skills.emptyNoSandboxHint')}</p>}
      {targetIds.length > 0 ? <section className="setting-drawer__section">
        <h4 className="setting-drawer__section-title">{t('settings.sandbox.skillInstallerModel')}</h4>
        <p className="installer-model-hint">{t('settings.sandbox.skillInstallerModelHint')}</p>
        <InstallerModelSelect installer={installer} t={t} />
      </section> : null}
    </>}
  </SettingDrawer>;
}

/** Install-onto-sandboxes drawer opened from a catalog chip (SkillSettings.vue:267-314, 1074-1104). */
function InstallSkillDialog({ client, open, item, configs, preselectConfigId, installer, t, onClose, onCatalogChanged, onToast, onManage }: {
  client: WeKnoraClient;
  open: boolean;
  item: SkillCatalog | null;
  configs: readonly SandboxConfigRecord[];
  preselectConfigId: string;
  installer: InstallerModel;
  t: (key: string, values?: Record<string, string | number>) => string;
  onClose: () => void;
  onCatalogChanged: () => void;
  onToast: (tone: 'success' | 'warning' | 'error', message: string) => void;
  onManage: (record: SandboxConfigRecord, skillId: string, catalogName: string) => void;
}) {

  const [targetIds, setTargetIds] = useState<string[]>([]);
  const [sessionIds, setSessionIds] = useState<string[]>([]);
  const [installing, setInstalling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setSessionIds([]);
    setError(null);
    setInstalling(false);
    if (preselectConfigId && configs.some((config) => config.id === preselectConfigId)) setTargetIds([preselectConfigId]);
    else {
      const live = new Set((item?.installations ?? []).filter((installation) => installation.status !== 'removed')
        .filter((installation) => installation.status === 'installing' || installation.status === 'ready' || installation.status === 'removing')
        .map((installation) => installation.sandboxConfigId));
      const remaining = configs.filter((config) => !live.has(config.id));
      setTargetIds(remaining.length === 1 ? [remaining[0]!.id] : []);
    }
  }, [open, item, preselectConfigId, configs]);

  const rows = sandboxPickRows(item, configs, 'remaining', sessionIds);
  const confirmDisabled = installing || (targetIds.length > 0 && !installer.modelId);
  const confirmText = targetIds.length > 0 ? t('settings.skills.installToSandbox') : t('settings.skills.addFinish');
  const description = item ? t('settings.skills.installDrawerDesc', { name: item.name }) : t('settings.skills.installToSandboxDesc');

  async function confirm() {
    if (!item || targetIds.length === 0) {
      onClose();
      return;
    }
    setInstalling(true);
    setError(null);
    try {
      await ensureInstallerModel(installer, targetIds, t);
      const result = await client.configuration.skills.catalog.install(item.id, [...targetIds]);
      const failed = catalogInstallFailedCount(result.errors);
      if (failed > 0) onToast('warning', t('settings.skills.installPartial', { failed }));
      else onToast('success', t('settings.skills.installAccepted'));
      setSessionIds((current) => [...new Set([...current, ...targetIds])]);
      onCatalogChanged();
      setTargetIds((current) => current.filter((id) => rows.some((row) => row.selectable && row.config.id === id)));
    } catch (cause) {
      setError(errorText(cause, t('settings.sandbox.skillUploadFailed')));
    } finally {
      setInstalling(false);
    }
  }

  /* Vue SkillSettings.vue:267-314 安装抽屉：SettingDrawer + confirm 主按钮。 */
  return <SettingDrawer
    visible={open}
    title={t('settings.skills.installToSandbox')}
    description={description}
    icon={SKILL_ICON}
    width="560px" minWidth={480} maxWidth={760}
    storageKey={SKILL_DRAWER_SPECS.install.storageKey}
    confirmLoading={installing}
    confirmDisabled={confirmDisabled}
    confirmText={confirmText}
    onConfirm={() => void confirm()}
    onVisibleChange={(visible) => { if (!visible) onClose(); }}
  >
    {error ? <Status tone="error">{error}</Status> : null}
    <SandboxPickList client={client} item={item} configs={configs} mode="remaining" sessionIds={sessionIds} targetIds={targetIds}
      onToggle={(configId, checked) => setTargetIds((current) => (checked ? [...new Set([...current, configId])] : current.filter((id) => id !== configId)))} t={t}
      metaLine={(record) => { const label = t(backendLabelKey(record.sandbox_type)); const target = sandboxTargetLine(record); return target ? `${label} · ${target}` : label; }}
      onManage={(record, installation) => { if (installation.skillId && item) onManage(record, installation.skillId, item.name); }} />
    {targetIds.length > 0 ? <section className="setting-drawer__section">
      <h4 className="setting-drawer__section-title">{t('settings.sandbox.skillInstallerModel')}</h4>
      <p className="installer-model-hint">{t('settings.sandbox.skillInstallerModelHint')}</p>
      <InstallerModelSelect installer={installer} t={t} />
    </section> : null}
  </SettingDrawer>;
}

function isSkillBusy(skill: InstalledSkill): boolean {
  return skill.status === 'installing' || skill.status === 'removing';
}

/** The row is 'installing' before its locators exist; afterwards they gate the run view (SandboxSkillsPanel.vue:846-849). */
function hasTranscript(skill: InstalledSkill): boolean {
  if (skill.status === 'installing') return true;
  return Boolean(skill.installSessionId && skill.installMessageId);
}

const REMOVE_STAGE_KEYS: Record<string, string> = {
  accepted: 'settings.sandbox.skillRemoveStage.accepted',
  sandbox_ready: 'settings.sandbox.skillRemoveStage.sandbox_ready',
  removed: 'settings.sandbox.skillRemoveStage.removed',
  done: 'settings.sandbox.skillRemoveStage.done',
  failed: 'settings.sandbox.skillRemoveStage.failed',
};

/** Vue progressStageText (SandboxSkillsPanel.vue:1026-1042). */
function removeStageText(progress: SkillInstallProgressEvent | undefined, skill: InstalledSkill, t: TimelineT): string {
  const key = progress?.stage ? REMOVE_STAGE_KEYS[progress.stage] : undefined;
  if (key) return t(key);
  if (skill.status === 'removing') return t('settings.sandbox.skillRemoveWaiting');
  return progress?.log ?? '';
}

type TimelineT = (key: string, values?: Record<string, string | number>) => string;

/**
 * Compact install run console ported from SkillInstallTimeline.vue: a live
 * transcript SSE tail (transcript endpoint, chat-shaped frames), the durable
 * history fallback for finished runs, and the guidance composer that either
 * steers the live run or reinstalls with guidance.
 */
function SkillInstallTimeline({ client, configId, skillId, sessionId, messageId, live, canRetry, onRestarted, t }: {
  client: WeKnoraClient;
  configId: string;
  skillId: string;
  /** The durable rows behind the run, used when the event log has aged out. */
  sessionId: string;
  messageId: string;
  /** True while the skill is still installing; locators land after the sandbox is up. */
  live: boolean;
  canRetry: boolean;
  onRestarted: () => void;
  t: TimelineT;
}) {
  const [timeline, setTimeline] = useState<SkillTimelineState>(initialSkillTimelineState);
  const [loading, setLoading] = useState(false);
  const [guidance, setGuidance] = useState<SkillInstallGuidanceState>({ accepting: false, messages: [] });
  const [guidanceText, setGuidanceText] = useState('');
  const [guidanceError, setGuidanceError] = useState('');
  const [sendingGuidance, setSendingGuidance] = useState(false);
  // openRun: stop()/a newer open() bump this so an in-flight live loop cannot
  // keep following after the props changed (SkillInstallTimeline.vue:152-156).
  const runRef = useRef(0);
  const guidanceEpochRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const propsRef = useRef({ client, configId, skillId, sessionId, messageId, live });
  propsRef.current = { client, configId, skillId, sessionId, messageId, live };
  // Keeps the same id after an uncertain response so Retry cannot enqueue twice (SkillInstallTimeline.vue:82-83).
  const pendingSendRef = useRef<{ messageId: string; content: string; id: string } | undefined>(undefined);

  const wait = useCallback((ms: number) => new Promise<void>((resolve) => { window.setTimeout(resolve, ms); }), []);

  /** Durable message rows replayed through the same reducer; resolves with how many frames landed. */
  const loadPersisted = useCallback(async (run: number): Promise<number> => {
    const current = propsRef.current;
    try {
      const rows = await current.client.sessions.messages(current.sessionId, { limit: 100 });
      if (run !== runRef.current) return 0;
      let next = initialSkillTimelineState();
      for (const row of rows as unknown[]) next = reduceSkillTimelineFrame(next, row);
      setTimeline(next);
      return next.frames;
    } catch {
      return 0;
    }
  }, []);

  /** One transcript SSE attempt; resolves to whether the endpoint served content (404 → throw). */
  const follow = useCallback(async (run: number): Promise<boolean> => {
    const current = propsRef.current;
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      return await current.client.sandbox.skills.followTranscript(current.configId, current.skillId, (frame) => {
        if (run !== runRef.current) return;
        setTimeline((state) => reduceSkillTimelineFrame(state, frame));
      }, controller.signal);
    } finally {
      if (abortRef.current === controller) abortRef.current = null;
    }
  }, []);

  // The run view (SkillInstallTimeline.vue open(), lines 268-319).
  useEffect(() => {
    const run = ++runRef.current;
    setTimeline(initialSkillTimelineState());
    setLoading(false);
    if (!configId || !skillId) return;
    const stale = () => run !== runRef.current;
    const followOnce = async () => { try { return await follow(run); } catch { return false; } };
    void (async () => {
      try {
        if (!live) {
          // A finished install replays durable rows without animating; a one-shot
          // transcript replay covers maintenance sessions with no durable rows.
          setLoading(true);
          if (sessionId) {
            const frames = await loadPersisted(run);
            if (!stale() && frames === 0 && messageId) await followOnce();
          }
          return;
        }
        // Locators land after the installer sandbox is up; the parent's poll
        // refreshes the row, and the key change remounts this timeline.
        if (!sessionId || !messageId) return;
        setLoading(true);
        for (;;) {
          if (stale() || !propsRef.current.live) return;
          const served = await followOnce();
          if (stale() || !propsRef.current.live || served) return;
          if (propsRef.current.sessionId && propsRef.current.messageId) {
            const frames = await loadPersisted(run);
            if (stale() || frames > 0) return;
          }
          if (stale() || !propsRef.current.live) return;
          await wait(1000);
        }
      } finally {
        if (!stale()) setLoading(false);
      }
    })();
    return () => {
      runRef.current += 1;
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [configId, skillId, sessionId, messageId, live, follow, loadPersisted, wait]);

  // Guidance state polls every 1.5s while the run is live (SkillInstallTimeline.vue:85-96, 134-144).
  useEffect(() => {
    const epoch = ++guidanceEpochRef.current;
    setGuidance({ accepting: false, messages: [] });
    if (!configId || !skillId) return;
    let timer: number | undefined;
    const refresh = async () => {
      try {
        const state = await client.sandbox.skills.guidance(configId, skillId);
        if (epoch !== guidanceEpochRef.current) return;
        setGuidance(state);
      } catch {
        if (epoch === guidanceEpochRef.current) setGuidance((current) => ({ ...current, accepting: false }));
      }
      if (epoch === guidanceEpochRef.current && propsRef.current.live) {
        timer = window.setTimeout(() => void refresh(), 1500);
      }
    };
    void refresh();
    return () => {
      guidanceEpochRef.current += 1;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [client, configId, skillId, live]);

  async function sendGuidance() {
    const content = guidanceText.trim();
    if (!content || sendingGuidance || (live ? !guidance.accepting : !canRetry)) return;
    setSendingGuidance(true);
    setGuidanceError('');
    try {
      if (live) {
        if (!pendingSendRef.current || pendingSendRef.current.messageId !== messageId || pendingSendRef.current.content !== content) {
          pendingSendRef.current = { messageId, content, id: makeSteerClientId() };
        }
        const steerId = pendingSendRef.current.id;
        await client.sandbox.skills.steer(configId, skillId, { expectedMessageId: messageId, steerId, content });
        // A failed refresh after a successful POST must not turn Retry into a duplicate send.
        setGuidance((current) => current.messages.some((item) => item.id === steerId)
          ? current
          : { ...current, messages: [...current.messages, { id: steerId, content, status: 'pending' }] });
        pendingSendRef.current = undefined;
      } else {
        await client.configuration.skills.installed.reinstall(configId, skillId, content);
        onRestarted();
      }
      setGuidanceText('');
    } catch (cause) {
      setGuidanceError(errorText(cause, t('settings.sandbox.skillGuidance.failed')));
    } finally {
      setSendingGuidance(false);
    }
  }

  /* S6 Tailwind 收编：时间线/引导 composer utilities → settings.td.css
     .skill-timeline__* 家族（抽屉 body 内，portal DOM，unscoped）。 */
  return <section className="skill-timeline" aria-busy={loading}>
    <div className="skill-timeline__scroll">
      {loading && timeline.frames === 0 ? <Status>{t('common.loading')}</Status>
        : timeline.frames === 0 ? <p className="skill-timeline__empty">{live ? t('settings.sandbox.skillTranscriptWaiting') : t('settings.sandbox.skillTranscriptEmpty')}</p>
        : <>
          {timeline.prompt ? <pre className="skill-timeline__prompt">{timeline.prompt}</pre> : null}
          {timeline.thinking ? <div className="skill-timeline__thinking">{timeline.thinking}</div> : null}
          {timeline.toolCalls.map((call) => (
            <div key={call.id} className="skill-timeline__call">
              <span className={'skill-timeline__call-dot' + (call.status === 'completed' ? ' is-completed' : call.status === 'failed' ? ' is-failed' : '')} aria-hidden="true" />
              <span className="skill-timeline__call-name">{call.name ?? call.id}</span>
              {typeof call.result === 'string' && call.result ? <span className="skill-timeline__call-result">{call.result}</span> : null}
            </div>
          ))}
          {timeline.answer ? <div className="skill-timeline__answer markdown-content" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(timeline.answer) }} /> : null}
          {timeline.error ? <p className="skill-timeline__error" role="alert">{timeline.error}</p> : null}
        </>}
      {guidance.messages.map((item) => (
        <div key={item.id} className="skill-timeline__guidance">
          <span>{t(`settings.sandbox.skillGuidance.${item.status}`)}</span>
          <p>{item.content}</p>
        </div>
      ))}
    </div>
    {live || canRetry ? <div className="skill-timeline__composer">
      <TTextarea value={guidanceText} maxLength={10000} rows={2} disabled={sendingGuidance}
        className="skill-timeline__composer-input"
        placeholder={t('settings.sandbox.skillGuidance.placeholder')}
        onChange={(value) => setGuidanceText(String(value))} />
      {guidanceError ? <p role="alert" className="skill-timeline__composer-error">{guidanceError}</p> : null}
      <div className="skill-timeline__composer-actions">
        <span>{live && !guidance.accepting ? t('settings.sandbox.skillGuidance.unavailable') : ''}</span>
        <TButton type="button" loading={sendingGuidance} disabled={!guidanceText.trim() || (live && !guidance.accepting)}
          onClick={() => void sendGuidance()}>
          {t(live ? 'settings.sandbox.skillGuidance.send' : 'settings.sandbox.skillGuidance.retry')}
        </TButton>
      </div>
    </div> : null}
  </section>;
}

/** Focused install management drawer (SandboxSkillsPanel.vue focus mode, 92-262 + 1374-1458). */
function ManageSkillDialog({ client, open, target, t, onClose, onChanged, onToast }: {
  client: WeKnoraClient;
  open: boolean;
  target: { record: SandboxConfigRecord; skillId: string; catalogName: string } | null;
  t: (key: string, values?: Record<string, string | number>) => string;
  onClose: () => void;
  onChanged: () => void;
  onToast: (tone: 'success' | 'warning' | 'error', message: string) => void;
}) {

  const [skill, setSkill] = useState<InstalledSkill | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [toggling, setToggling] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [uninstalling, setUninstalling] = useState(false);
  const [uninstallDone, setUninstallDone] = useState(false);
  const [pendingUninstall, setPendingUninstall] = useState(false);
  const [envDrafts, setEnvDrafts] = useState<Record<string, string>>({});
  const [envSaving, setEnvSaving] = useState(false);
  const [pendingClearEnv, setPendingClearEnv] = useState('');
  // Live install/removal progress from the install-events SSE (SandboxSkillsPanel.vue progressById).
  const [progress, setProgress] = useState<SkillInstallProgressEvent | undefined>(undefined);
  // Every open re-reads the run from the top (SandboxSkillsPanel.vue:874-879 transcriptEpoch).
  const [transcriptEpoch, setTranscriptEpoch] = useState(0);

  const configId = target?.record.id ?? '';
  const skillId = target?.skillId ?? '';

  const load = useCallback(async (silent = false) => {
    if (!configId || !skillId) return;
    if (!silent) setLoading(true);
    try {
      setSkill(await client.configuration.skills.installed.get(configId, skillId));
      setError(null);
    } catch (cause) {
      if (uninstalling) {
        setUninstallDone(true);
        return;
      }
      if (!silent) setError(errorText(cause, t('settings.sandbox.skillLoadFailed')));
    } finally {
      if (!silent) setLoading(false);
    }
  }, [client, configId, skillId, t, uninstalling]);

  useEffect(() => {
    if (!open) return;
    setUninstallDone(false);
    setUninstalling(false);
    setPendingUninstall(false);
    setPendingClearEnv('');
    setEnvDrafts({});
    setError(null);
    setProgress(undefined);
    setTranscriptEpoch((current) => current + 1);
    void load();
  }, [open, load]);

  const busy = skill ? isSkillBusy(skill) : false;
  useEffect(() => {
    if (!open || !busy || typeof window === 'undefined') return undefined;
    const timer = window.setInterval(() => void load(true), SKILL_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [open, busy, load]);

  // Keep load/onChanged out of the follow effect so a background refresh does
  // not tear the SSE connection down (Vue keeps one follow per busy skill).
  const loadRef = useRef(load);
  loadRef.current = load;
  const onChangedRef = useRef(onChanged);
  onChangedRef.current = onChanged;

  // followBusySkills/followProgress (SandboxSkillsPanel.vue:1089-1174): one
  // install-events stream per busy skill; the backend always terminates it, so
  // the terminal frame drives the refresh exactly like Vue's done branch.
  useEffect(() => {
    if (!open || !busy || !configId || !skillId) return undefined;
    let active = true;
    const controller = new AbortController();
    void client.sandbox.skills.followInstallEvents(configId, skillId, (frame) => {
      if (!active) return;
      setProgress(frame.event);
      if (frame.terminal) {
        void loadRef.current(true);
        onChangedRef.current();
      }
    }, controller.signal).catch(() => {
      // Stream closed early; the 2.5s status poll keeps the row fresh.
    });
    return () => {
      active = false;
      controller.abort();
    };
  }, [client, open, busy, configId, skillId]);

  // Vue watches the list until a removing row disappears, then shows the done note (SandboxSkillsPanel.vue:823-834).
  useEffect(() => {
    if (uninstallDone || !uninstalling || !skill) return;
    if (skill.status === 'removing') return;
    if (skill.status === 'ready' || skill.status === 'failed') {
      setUninstallDone(true);
      setUninstalling(false);
      onToast('success', t('settings.sandbox.skillRemoveDone', { name: skill.name }));
      onChanged();
    }
  }, [skill, uninstallDone, uninstalling, onChanged, onToast, t]);

  async function toggleEnabled(enabled: boolean) {
    if (!skill || busy) return;
    setToggling(true);
    try {
      const updated = await client.configuration.skills.installed.update(configId, skillId, { enabled });
      setSkill(updated);
      onToast('success', enabled ? t('settings.sandbox.skillEnabled') : t('settings.sandbox.skillDisabled'));
    } catch (cause) {
      onToast('error', errorText(cause, t('settings.sandbox.skillToggleFailed')));
    } finally {
      setToggling(false);
    }
  }

  async function retry() {
    if (!skill) return;
    setRetrying(true);
    setProgress(undefined);
    try {
      await client.configuration.skills.installed.reinstall(configId, skillId);
      onToast('success', t('settings.sandbox.skillRetryAccepted'));
      await load(true);
    } catch (cause) {
      onToast('error', errorText(cause, t('settings.sandbox.skillRetryFailed')));
    } finally {
      setRetrying(false);
    }
  }

  async function stop() {
    if (!skill) return;
    setStopping(true);
    setProgress(undefined);
    try {
      setSkill(await client.configuration.skills.installed.stop(configId, skillId));
      onToast('success', t('settings.sandbox.skillStopAccepted'));
      await load(true);
      onChanged();
    } catch (cause) {
      onToast('error', errorText(cause, t('settings.sandbox.skillStopFailed')));
    } finally {
      setStopping(false);
    }
  }

  async function uninstall() {
    if (!skill || busy) return;
    setPendingUninstall(false);
    setUninstalling(true);
    setProgress(undefined);
    try {
      await client.configuration.skills.installed.remove(configId, skillId);
      onToast('success', t('settings.sandbox.skillDeleteAccepted'));
      setSkill({ ...skill, status: 'removing' });
      await load(true);
    } catch (cause) {
      setUninstalling(false);
      onToast('error', errorText(cause, t('common.deleteFailed')));
    }
  }

  function envPayload(): Record<string, string> {
    return editedSkillEnvPayload((skill?.envs ?? []).map((env) => env.name), envDrafts);
  }

  async function submitEnvs(envs: Record<string, string>, successKey: string) {
    if (!skill || envSaving) return;
    setEnvSaving(true);
    try {
      const updated = await client.configuration.skills.installed.update(configId, skillId, { envs });
      setSkill(updated);
      setEnvDrafts((current) => clearSubmittedSkillEnvDrafts(current, envs));
      onToast('success', t(successKey));
    } catch (cause) {
      onToast('error', errorText(cause, t('settings.sandbox.skillEnv.saveFailed')));
    } finally {
      setEnvSaving(false);
    }
  }

  async function saveEnvs() {
    if (!skill || busy) return;
    const envs = envPayload();
    if (Object.keys(envs).length === 0) return;
    if (Object.values(envs).some((value) => !isValidEnvValueLength(value))) {
      onToast('error', t('settings.sandbox.skillEnv.valueTooLong', { max: MAX_ENV_VALUE_BYTES }));
      return;
    }
    await submitEnvs(envs, 'settings.sandbox.skillEnv.saveSuccess');
  }

  const errorLines = installErrorLines(skill?.error);
  /* Vue SkillSettings.vue:316-321 管理抽屉：SettingDrawer hideFooter + z-index 2600。 */
  return <SettingDrawer
    visible={open}
    title={target?.catalogName ?? ''}
    description={target ? t('settings.skills.manageDrawerDesc', { name: target.record.name }) : undefined}
    icon={SKILL_ICON}
    width="680px" minWidth={560} maxWidth={920}
    storageKey={SKILL_DRAWER_SPECS.manage.storageKey}
    hideFooter
    zIndex={2600}
    onVisibleChange={(visible) => { if (!visible) onClose(); }}
  >
    {loading ? <Status>{t('common.loading')}</Status> : null}
    {error ? <Status tone="error">{error}</Status> : null}
    {skill ? uninstallDone ? <div className="skill-manage__done">
      <span aria-hidden="true">✓</span>
      <p>{t('settings.sandbox.skillRemoveDone', { name: skill.name })}</p>
    </div> : busy && skill.status === 'removing' ? <section className="skill-manage__section">
      <div className="skill-manage__section-head">
        <h4>{t('settings.sandbox.skillRemoveInProgress')}</h4>
        <ProgressRing percent={installProgressPercent(progress, skill.status)} />
      </div>
      <p className="skill-manage__stage">{removeStageText(progress, skill, t)}</p>
    </section> : <>
      <div className="skill-manage__row">
        <div className="skill-manage__row-copy">
          <label>{t('settings.skills.manageEnable')}</label>
          <p className="wk-muted skill-manage__hint">{t('settings.sandbox.skillDisableHint')}</p>
          <p className="wk-muted skill-manage__hint">{installStatusKeys(skill.status, skill.enabled).map((key) => t(key)).join(' · ')}</p>
        </div>
        <div className="skill-manage__controls">
          <label className="skill-manage__switch"><TSwitch className="wk-skill-switch" aria-label={t('settings.skills.manageEnable')} value={skill.enabled} disabled={busy} onChange={(checked) => void toggleEnabled(Boolean(checked))} /> {toggling ? '…' : ''}</label>
          {skill.status === 'failed' ? <TButton type="button" loading={retrying} disabled={busy} title={t('settings.sandbox.skillRetryHint')} onClick={() => void retry()}>{t('settings.sandbox.skillRetry')}</TButton> : null}
          {skill.status === 'installing' ? <TButton type="button" loading={stopping} disabled={busy} title={t('settings.sandbox.skillStopHint')} onClick={() => void stop()}>{t('settings.sandbox.skillStop')}</TButton> : null}
          {skill.status !== 'installing' ? <TButton type="button" loading={uninstalling} disabled={busy} title={t('settings.skills.manageUninstallConfirm', { name: skill.name })} onClick={() => setPendingUninstall(true)}>{t('settings.skills.manageUninstall')}</TButton> : null}
        </div>
      </div>
      {pendingUninstall ? <div className="wk-list-actions">
        <span className="skill-manage__uninstall-copy">{t('settings.skills.manageUninstallConfirm', { name: skill.name })}</span>
        <TButton type="button" onClick={() => setPendingUninstall(false)}>{t('common.cancel')}</TButton>
        <TButton type="button" loading={uninstalling} onClick={() => void uninstall()}>{t('common.delete')}</TButton>
      </div> : null}
      {errorLines.length > 0 ? <ul className="skill-manage__error-list">{errorLines.map((line, index) => <li key={index}>{line}</li>)}</ul> : null}
      {(skill.envs ?? []).length > 0 ? <section className="skill-manage__section">
        <h4>{t('settings.sandbox.skillEnv.toggle')}</h4>
        <p className="wk-muted">{t('settings.sandbox.skillEnv.workspaceHint')}</p>
        <div className="skill-manage__env-list">
          {(skill.envs ?? []).map((env) => <div key={env.name} className="skill-manage__env">
            <div className="skill-manage__env-head">
              <code className="skill-manage__env-name">{env.name}</code>
              {env.required ? <span className="skill-manage__env-badge skill-manage__env-badge--required">{t('settings.sandbox.skillEnv.required')}</span> : null}
              <span className={'skill-manage__env-badge ' + (env.isSet ? 'skill-manage__env-badge--set' : 'skill-manage__env-badge--unset')}>{env.isSet ? t('settings.sandbox.skillEnv.isSet') : t('settings.sandbox.skillEnv.notSet')}</span>
              {env.description ? <span className="skill-manage__env-desc">{env.description}</span> : null}
            </div>
            <div className="skill-manage__env-row">
              <TInput type="password" autocomplete="new-password" spellCheck={false} aria-label={env.name}
                placeholder={env.isSet ? t('settings.sandbox.skillEnv.placeholderSet') : t('settings.sandbox.skillEnv.placeholderUnset')}
                value={envDrafts[env.name] ?? ''} disabled={busy || envSaving}
                onChange={(value) => setEnvDrafts((current) => ({ ...current, [env.name]: String(value) }))} />
              {canClearAdminSkillEnv(env) ? <>
                <TButton type="button" disabled={envSaving} onClick={() => setPendingClearEnv(env.name)}>{t('settings.sandbox.skillEnv.clear')}</TButton>
                {pendingClearEnv === env.name ? <span>
                  {t('settings.sandbox.skillEnv.clearConfirm', { name: env.name })}
                  <TButton type="button" onClick={() => setPendingClearEnv('')}>{t('common.cancel')}</TButton>
                  <TButton type="button" loading={envSaving} onClick={() => { const name = env.name; setPendingClearEnv(''); void submitEnvs(adminSkillEnvClearPayload(name), 'settings.sandbox.skillEnv.clearSuccess'); }}>{t('common.delete')}</TButton>
                </span> : null}
              </> : null}
            </div>
          </div>)}
        </div>
        <div className="wk-list-actions">
          <TButton type="button" loading={envSaving} disabled={Object.keys(envPayload()).length === 0 || busy} onClick={() => void saveEnvs()}>{t('settings.sandbox.skillEnv.save')}</TButton>
        </div>
      </section> : null}
      {hasTranscript(skill) ? <section className="skill-manage__section skill-manage__section--transcript">
        <div className="skill-manage__section-head">
          <h4>{t('settings.sandbox.skillTranscriptTitle')}</h4>
          {skill.status === 'installing' ? <ProgressRing percent={installProgressPercent(progress, skill.status)} /> : null}
        </div>
        <SkillInstallTimeline
          key={`${skill.id}-${skill.installSessionId ?? ''}-${transcriptEpoch}`}
          client={client}
          configId={configId}
          skillId={skillId}
          sessionId={skill.installSessionId ?? ''}
          messageId={skill.installMessageId ?? ''}
          live={skill.status === 'installing'}
          canRetry={skill.status === 'ready' || skill.status === 'failed'}
          onRestarted={() => { setProgress(undefined); void load(true); onChangedRef.current(); }}
          t={t}
        />
      </section> : null}
    </> : !loading && !error ? <Status>{t('common.loading')}</Status> : null}
  </SettingDrawer>;
}

/** Vue focus-drawer progress ring (SandboxSkillsPanel.vue:239-248 t-progress circle + percent). */
function ProgressRing({ percent }: { percent: number }) {
  const circumference = 2 * Math.PI * 7;
  const clamped = Math.max(0, Math.min(100, percent));
  return <div className="skill-manage__progress">
    <svg viewBox="0 0 18 18" width="18" height="18" aria-hidden="true">
      <circle className="skill-manage__progress-track" cx="9" cy="9" r="7" fill="none" strokeWidth="2" />
      <circle className="skill-manage__progress-value" cx="9" cy="9" r="7" fill="none" strokeWidth="2" strokeLinecap="round"
        strokeDasharray={`${(clamped / 100) * circumference} ${circumference}`} transform="rotate(-90 9 9)" />
    </svg>
    <span>{clamped}%</span>
  </div>;
}

/** Catalog file browser drawer (SkillFilesDrawer.vue + SkillFilesPanel.vue). */
function CatalogFilesDialog({ client, open, target, t, onClose }: {
  client: WeKnoraClient;
  open: boolean;
  target: { id: string; name: string } | null;
  t: (key: string, values?: Record<string, string | number>) => string;
  onClose: () => void;
}) {
  const drawerDialog = useSkillDrawerDialog();
  const [nodes, setNodes] = useState<ReturnType<typeof buildSkillFileTree>>([]);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [selectedPath, setSelectedPath] = useState('');
  const [file, setFile] = useState<SkillFileContent | null>(null);
  const [listLoading, setListLoading] = useState(false);
  const [fileLoading, setFileLoading] = useState(false);
  const [listError, setListError] = useState('');
  const [fileError, setFileError] = useState('');
  const [markdownSource, setMarkdownSource] = useState(false);
  const [copied, setCopied] = useState(false);
  const catalogId = target?.id ?? '';

  const selectFile = useCallback(async (path: string) => {
    if (!catalogId || !isSafeSkillFilePath(path)) return;
    setSelectedPath(path);
    setFileLoading(true);
    setFileError('');
    setFile(null);
    setCopied(false);
    setMarkdownSource(false);
    try {
      setFile(await client.configuration.skills.catalog.file(catalogId, path));
    } catch (cause) {
      setFileError(errorText(cause, t('settings.sandbox.skillFilesFileLoadFailed')));
    } finally {
      setFileLoading(false);
    }
  }, [catalogId, client, t]);

  useEffect(() => {
    if (!open || !catalogId) return;
    let cancelled = false;
    setListLoading(true);
    setListError('');
    setNodes([]);
    setSelectedPath('');
    setFile(null);
    void client.configuration.skills.catalog.files(catalogId).then((files) => {
      if (cancelled) return;
      const tree = buildSkillFileTree(files);
      setNodes(tree);
      setExpanded(new Set(collectSkillDirPaths(tree)));
      const initial = files.some((entry) => entry.path === 'SKILL.md') ? 'SKILL.md' : files[0]?.path;
      if (initial) void selectFile(initial);
    }).catch((cause: unknown) => {
      if (!cancelled) setListError(errorText(cause, t('settings.sandbox.skillFilesLoadFailed')));
    }).finally(() => { if (!cancelled) setListLoading(false); });
    return () => { cancelled = true; };
  }, [open, catalogId, client, t, selectFile]);

  const rows = flattenSkillFileRows(nodes, expanded);
  const markdown = selectedPath ? isMarkdownPath(selectedPath) : false;
  const frontmatter = markdown && file?.encoding === 'utf-8' && file.content != null ? splitMarkdownFrontmatter(file.content) : null;
  const imageSrc = file && file.encoding === 'base64' && file.content && file.mediaType ? `data:${file.mediaType};base64,${file.content}` : '';

  async function copyContent() {
    if (!file?.content || file.encoding !== 'utf-8') return;
    try {
      await navigator.clipboard.writeText(file.content);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return <TDialog footer={false} visible={open} header={target?.name ?? ''} onClose={onClose} {...drawerDialog}>
    <p className="wk-muted">{t('settings.sandbox.skillFilesTitle')}</p>
    {/* S6 Tailwind 收编：文件浏览器 utilities → settings.td.css .skill-files-* 家族。 */}
    <div className="skill-files-layout">
      <aside className="skill-files-tree">
        {listError ? <p className="wk-muted">{listError}</p>
          : listLoading ? <Status>{t('common.loading')}</Status>
          : rows.length === 0 ? <p className="wk-muted">{t('settings.sandbox.skillFilesEmpty')}</p>
          : <ul className="skill-files-list">
            {rows.map((row) => (
              <li key={row.path}>
                <button type="button" className={'skill-files-item' + (selectedPath === row.path ? ' is-selected' : '') + (row.isDir ? ' is-dir' : '')} title={row.path}
                  onClick={() => { if (row.isDir) setExpanded((current) => { const next = new Set(current); if (next.has(row.path)) next.delete(row.path); else next.add(row.path); return next; }); else void selectFile(row.path); }}>
                  <span className="skill-files-panel__indent" style={{ width: `${row.depth * 12}px` }} />
                  <span aria-hidden="true">{row.isDir ? (expanded.has(row.path) ? '▾' : '▸') : '·'}</span>
                  <span className="skill-files-item__name">{row.name}</span>
                </button>
              </li>
            ))}
          </ul>}
      </aside>
      <section className="skill-files-detail">
        {selectedPath ? <div className="skill-files-head">
          <span className="skill-files-path" title={selectedPath}>{selectedPath}</span>
          {markdown && file?.encoding === 'utf-8' ? <TButton type="button" onClick={() => setMarkdownSource((current) => !current)}>{markdownSource ? t('settings.sandbox.skillFilesPreview') : t('settings.sandbox.skillFilesSource')}</TButton> : null}
          {file?.content && file.encoding === 'utf-8' ? <TButton type="button" onClick={() => void copyContent()}>{copied ? t('common.copied') : t('common.copy')}</TButton> : null}
        </div> : null}
        <div className="skill-files-body">
          {fileLoading ? <Status>{t('common.loading')}</Status>
            : fileError ? <Status tone="error">{fileError}</Status>
            : !selectedPath ? <p className="wk-muted">{t('settings.sandbox.skillFilesSelectHint')}</p>
            : <>
              {file?.truncated ? <Status tone="warning">{t('settings.sandbox.skillFilesTruncated')}</Status> : null}
              {imageSrc ? <img className="skill-files-image" src={imageSrc} alt={selectedPath} />
                : markdown && file?.encoding === 'utf-8' && file.content != null && !markdownSource ? <>
                  {frontmatter && frontmatter.fields.length > 0 ? <dl className="skill-files-panel__meta">
                    {frontmatter.fields.map((field) => <div key={field.key} className="skill-files-meta-row"><dt>{field.key}</dt><dd className={field.code ? 'is-code' : undefined}>{field.value}</dd></div>)}
                  </dl> : null}
                  {/* Vue renders the markdown body as HTML (SkillFilesPanel.vue:87-91, 476-478); the shared renderer escapes raw HTML and allow-lists links. */}
                  <div className="skill-files-panel__markdown markdown-content" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(frontmatter?.body ?? file.content) }} />
                </>
                : file?.encoding === 'utf-8' && file.content != null ? <pre className="skill-files-code"><code>{file.content}</code></pre>
                : <p className="wk-muted">{t('settings.sandbox.skillFilesBinary')}</p>}
            </>}
        </div>
      </section>
    </div>
  </TDialog>;
}
