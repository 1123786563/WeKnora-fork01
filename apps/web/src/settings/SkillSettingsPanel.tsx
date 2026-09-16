import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { AgentConfiguration, InstalledSkill, ModelConfiguration, SandboxConfigRecord, SkillCatalog, SkillCatalogInstallation, SkillConfiguration, SkillFileContent, SkillInstallGuidanceState, WeKnoraClient } from '@weknora/api-client';
import { initialSkillTimelineState, installProgressPercent, reduceSkillTimelineFrame, type SkillInstallProgressEvent, type SkillTimelineState } from '@weknora/domain/sandbox/skill-install';
import { Button, Card, Checkbox, Dialog, Input, Select, Status, Switch, Textarea, Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@weknora/ui';
import { renderChatMarkdown } from '../../../../packages/views/src/chat/markdown.ts';
import { createTranslator, useAppLocale } from '../i18n.ts';
import { EmptyState } from './EmptyState.tsx';
import { observeUploadProgress } from '../platform/http.ts';
import './skill-settings.css';
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
function DrawerShell({ open, spec, children }: { open: boolean; spec: DrawerWidthSpec; children: React.ReactNode }) {
  const [width, setWidth] = useState(spec.defaultWidth);
  const [resizing, setResizing] = useState(false);
  const widthRef = useRef(width);
  widthRef.current = width;

  useEffect(() => {
    if (!open) return;
    setWidth(readStoredDrawerWidth(spec));
    const onWindowResize = () => setWidth((current) => clampDrawerWidth(current, spec));
    window.addEventListener('resize', onWindowResize);
    return () => window.removeEventListener('resize', onWindowResize);
  }, [open, spec]);

  function onHandleDown(event: React.MouseEvent) {
    event.preventDefault();
    const start = { x: event.clientX, width: widthRef.current };
    setResizing(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    const onMove = (move: MouseEvent) => setWidth(clampDrawerWidth(start.width + (start.x - move.clientX), spec));
    const onUp = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      setResizing(false);
      try {
        window.localStorage.setItem(spec.storageKey, String(clampDrawerWidth(widthRef.current, spec)));
      } catch {
        // localStorage can throw in private mode / quota errors.
      }
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  return <div className="contents [&_.wk-dialog]:w-[min(var(--skill-drawer-width,680px),100%)]! [&[data-resizing]_.wk-dialog]:[transition:none] [&[data-resizing]_.wk-dialog]:select-none" style={{ '--skill-drawer-width': `${width}px` } as React.CSSProperties} data-resizing={resizing || undefined}>
    {children}
    {open ? <div className="group/resize fixed top-0 bottom-0 left-[calc((100vw_-_var(--skill-drawer-width,680px))_/_2_-_6px)] z-[2600] w-3 cursor-col-resize" role="presentation" onMouseDown={onHandleDown}>
      <div className={`mx-auto h-full w-0.5 ${resizing ? 'bg-primary' : 'bg-transparent group-hover/resize:bg-primary'}`} />
    </div> : null}
  </div>;
}

const INSTALLER_AGENT_ID = 'builtin-skill-installer';
const LAST_CHAT_MODEL_KEY = 'weknora_last_chat_model_id';
const SKILL_POLL_INTERVAL_MS = 2500;

/* ---- Vue baseline glyphs (TDesign currentColor icons, SkillSettings.vue:44-59,196-198) ---- */
function GlyphIcon({ children, size = 14 }: { children: React.ReactNode; size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={{ display: 'block' }}>{children}</svg>;
}
/** t-icon "system-code" (SKILL_ICON, frontend/src/types/mention.ts:4). */
function SkillGlyph({ size }: { size?: number }) {
  return <GlyphIcon size={size}><path d="m8.5 8-4.5 4 4.5 4" /><path d="m15.5 8 4.5 4-4.5 4" /><path d="M13.5 5 10.5 19" /></GlyphIcon>;
}
function FolderGlyph({ size }: { size?: number }) {
  return <GlyphIcon size={size}><path d="M3.5 6.5A1.5 1.5 0 0 1 5 5h4.2l1.8 2H19a1.5 1.5 0 0 1 1.5 1.5v9A1.5 1.5 0 0 1 19 19H5a1.5 1.5 0 0 1-1.5-1.5z" /></GlyphIcon>;
}
function DeleteGlyph({ size }: { size?: number }) {
  return <GlyphIcon size={size}><path d="M4 7h16M9.5 7V4.5h5V7m-8.5 0 .8 12.5h11.4L19 7" /><path d="M10 11v5.5M14 11v5.5" /></GlyphIcon>;
}
function CloudUploadGlyph({ size }: { size?: number }) {
  return <GlyphIcon size={size}><path d="M7.5 17.5a4.2 4.2 0 0 1-.9-8.3 5.6 5.6 0 0 1 11-.1 4 4 0 0 1-.3 8.2" /><path d="M12 12.5V20" /><path d="m8.8 15.2 3.2-3.2 3.2 3.2" /></GlyphIcon>;
}
function CloudGlyph() {
  return <GlyphIcon><path d="M7 17.5a4 4 0 0 1-.9-7.9 5.4 5.4 0 0 1 10.6-.1 3.9 3.9 0 0 1-.2 7.9" /></GlyphIcon>;
}
function ServerGlyph() {
  return <GlyphIcon><rect x="4" y="5" width="16" height="6" rx="1.2" /><rect x="4" y="13" width="16" height="6" rx="1.2" /><path d="M7.5 8h.01M7.5 16h.01" /></GlyphIcon>;
}
function MinusCircleGlyph() {
  return <GlyphIcon><circle cx="12" cy="12" r="8.5" /><path d="M8.5 12h7" /></GlyphIcon>;
}

/**
 * Vue SkillSettings.vue:5-9 header help: a t-icon "help-circle" wrapped in a
 * t-tooltip (placement right). Icon is 16px, --td-text-color-placeholder with
 * cursor:help and a hover shift to --td-text-color-secondary
 * (SkillSettings.vue:1244-1253); the popup content caps at 340px width with
 * line-height 1.55 (SkillSettings.vue:1268-1271).
 */
function SkillHelpTooltip({ content }: { content: string }) {
  return (
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            aria-label={content}
            className="m-0 inline-flex cursor-help border-0 bg-transparent p-0 text-[rgba(0_0_0_/.4)] transition-colors duration-150 hover:text-[rgba(0_0_0_/.6)] focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={{ display: 'block' }}>
              <circle cx="12" cy="12" r="10" />
              <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
              <path d="M12 17h.01" />
            </svg>
          </button>
        </TooltipTrigger>
        <TooltipContent side="right" className="max-w-[340px] leading-[1.55]">{content}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

/** Vue SkillSettings.vue:3-11 .section-header: 20px/600 title row (title +
 * help icon, 8px gap) over a 14px secondary description, 28px bottom margin. */
function SkillSectionHeader({ helpContent }: { helpContent: string }) {
  const t = useSkillT();
  return (
    <header className="mb-7">
      <div className="mb-2 flex items-center gap-2">
        <h2 className="m-0 text-[20px] font-semibold leading-[normal] text-[rgba(0_0_0_/.9)]">{t('settings.skills.title')}</h2>
        <SkillHelpTooltip content={helpContent} />
      </div>
      <p className="m-0 text-sm leading-[1.6] text-[rgba(0_0_0_/.6)]">{t('settings.skills.description')}</p>
    </header>
  );
}

/** Vue SandboxBackendBadge.vue — square icon badge, exact tones/sizes. */
const SANDBOX_BADGE_TONES: Record<string, string> = {
  e2b: 'bg-[rgb(98_53_187/10%)] text-[#6235bb]',
  docker: 'bg-[rgb(29_99_237/10%)] text-[#1d63ed]',
};
function SandboxBadge({ type, size }: { type?: string; size: 'xs' | 'sm' }) {
  if (!type) return null;
  const dims = size === 'xs'
    ? 'h-4 w-4 rounded-[4px] [&_svg]:h-2.5 [&_svg]:w-2.5'
    : 'h-[26px] w-[26px] rounded-[7px] [&_svg]:h-3.5 [&_svg]:w-3.5';
  return <span aria-hidden="true" className={`inline-flex shrink-0 items-center justify-center ${dims} ${SANDBOX_BADGE_TONES[type] ?? 'bg-[rgb(0_82_217/10%)] text-[#0052d9]'}`}>
    {type === 'cube' ? <ServerGlyph /> : type === 'disabled' ? <MinusCircleGlyph /> : <CloudGlyph />}
  </span>;
}

/**
 * Vue SettingDrawer header block (SettingDrawer.vue:291-356): leading icon
 * badge (32px, radius 9, brand 10% tint) + 15px/600 title + 12px subtitle,
 * rendered inside the shared Dialog h2.
 */
function DrawerTitle({ icon, title, subtitle }: { icon: React.ReactNode; title: string; subtitle?: string }) {
  return <span className="flex min-w-0 items-center gap-2.5">
    <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[9px] bg-[rgb(7_192_95/10%)] text-[#07c05f]" aria-hidden="true">{icon}</span>
    <span className="flex min-w-0 flex-1 flex-col gap-px text-left">
      <span className="overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold leading-[1.4] text-[rgba(0_0_0_/.9)]">{title}</span>
      {subtitle ? <span className="text-[12px] leading-[1.45] text-[rgba(0_0_0_/.6)]">{subtitle}</span> : null}
    </span>
  </span>;
}

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

export function SkillSettingsPanel({ client, role, initialSkills, initialCatalog, initialSandboxConfigs }: Props) {
  // SkillSettings.vue is mounted for authenticated admin+ users; keep the
  // system-admin role on the catalog-management branch as well.
  const canEdit = role === 'admin' || role === 'owner' || role === 'system-admin';
  const t = useSkillT();
  if (!canEdit) {
    return <Card data-testid="skill-settings"><h3>{t('settings.skills.title')}</h3><p className="wk-muted text-muted">{t('settings.skills.description')}</p>{initialSkills && initialSkills.length > 0 ? <ul className="wk-list m-0 list-none p-0">{initialSkills.map((skill) => <li key={skill.id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]"><strong>{skill.name}</strong><span className="font-mono text-[0.8rem] text-muted">{skill.description ?? t('settings.skills.emptyDesc')}</span></li>)}</ul> : <Status>{t('settings.skills.noInstalls')}</Status>}</Card>;
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
  const [toast, setToast] = useState<{ tone: 'success' | 'warning' | 'error'; message: string } | null>(null);
  const focusTimer = useRef<number | null>(null);

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
      if (!silent) setLoadError(errorText(cause, t('settings.skills.loadFailed')));
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
      setToast({ tone: 'error', message: errorText(cause, t('settings.sandbox.skillInstallerModelSaveFailed')) });
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
      setToast({ tone: 'warning', message: t('settings.skills.deleteCatalogBlocked') });
      return;
    }
    setDeletingId(item.id);
    try {
      await client.configuration.skills.catalog.remove(item.id);
      setToast({ tone: 'success', message: t('settings.skills.deleteSuccess') });
      await load(true);
    } catch (cause) {
      setToast({ tone: 'error', message: errorText(cause, t('common.deleteFailed')) });
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

  const empty = catalog.length === 0;
  return <section data-testid="skill-settings" className="grid gap-3">
    <SkillSectionHeader helpContent={t('settings.skills.helpTooltip')} />
    {toast ? <Status tone={toast.tone}>{toast.message}</Status> : null}
    {loadError ? <Status tone="error">{loadError}</Status> : null}
    {loading ? <Status>{t('common.loading')}</Status> : empty ? (
      <div className="flex flex-col items-center justify-center px-4 py-20 text-center">
        <EmptyState
          description={t('settings.skills.emptyDesc')}
          hint={skillConfigs.length === 0 ? t('settings.skills.emptyNoSandboxHint') : undefined}
        >
          <Button type="button" variant="primary" onClick={() => { setWizardOpen(true); }}>{t('settings.skills.addSkill')}</Button>
          {skillConfigs.length === 0
            ? <Button type="button" onClick={() => { if (typeof window !== 'undefined') window.location.assign('/platform/settings?section=sandbox'); }}>{t('settings.skills.goSandboxSettings')}</Button>
            : null}
        </EmptyState>
      </div>
    ) : (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] items-stretch gap-2.5">
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
          // Vue: border/box-shadow brand + --td-brand-color-focus 20% mix (SkillSettings.vue:1334-1337).
          const cardTone = focusedCatalogId === item.id ? 'border-[#07c05f] shadow-[0_0_0_2px_rgb(7_192_95/20%)]' : 'border-[#e7e7e7]';
          const chipColorCls = chipTone === 'off' ? 'text-[rgba(0_0_0_/.4)]' : live ? 'text-[rgba(0_0_0_/.6)]' : 'text-[#07c05f]';
          const chipBgCls = chipTone === 'stale' ? 'bg-[rgb(237_123_47/10%)]' : chipTone === 'failed' ? 'bg-[rgb(227_77_89/10%)]' : live ? 'bg-[#f3f3f3]' : 'bg-[rgb(7_192_95/10%)]';
          const chipHoverCls = live ? 'enabled:hover:text-[rgba(0_0_0_/.9)] enabled:hover:bg-[#f3f3f3]' : 'enabled:hover:text-[#07c05f] enabled:hover:bg-[rgb(7_192_95/16%)]';
          const chipCls = `group/chip inline-flex items-center gap-1 min-w-0 max-w-full m-0 px-1.5 py-0.5 border-0 rounded-control [font:inherit] text-xs leading-[18px] text-left cursor-pointer focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2 disabled:cursor-default disabled:opacity-60 ${chipColorCls} ${chipBgCls} ${chipHoverCls}`;
          const chipGoCls = live ? 'shrink-0 text-[rgba(0_0_0_/.4)] group-hover/chip:text-current' : 'shrink-0 text-[#07c05f]';
          const entryStatusCls = chipTone === 'ready' ? 'text-[#00a870]' : chipTone === 'stale' ? 'text-[#ed7b2f]' : chipTone === 'failed' ? 'text-[#e34d59]' : '';
          return <article key={item.id} className={`relative flex flex-col p-0 overflow-hidden rounded-[10px] bg-white transition-[border-color,box-shadow] duration-[180ms] min-w-0 h-full border ${cardTone}${focusedCatalogId === item.id ? ' skill-card--focused' : ''}${live ? ' skill-card--installed' : ' skill-card--idle'}`}>
            <div className="flex items-stretch p-3 min-w-0 flex-1"><div className="flex-1 min-w-0 flex flex-col gap-2">
              <div className="flex items-center gap-2.5 min-w-0 min-h-[28px]">
                <span className={`shrink-0 w-[26px] h-[26px] rounded-[7px] flex items-center justify-center ${live ? 'bg-[rgb(7_192_95/12%)] text-[#07c05f]' : 'bg-[#f3f3f3] text-[rgba(0_0_0_/.6)]'}`} aria-hidden="true"><SkillGlyph size={14} /></span>
                <div className="flex-1 min-w-0 flex items-baseline gap-1.5">
                  <h3 className="flex-[0_1_auto] min-w-0 m-0 text-sm font-semibold leading-5 text-[rgba(0_0_0_/.9)] overflow-hidden text-ellipsis whitespace-nowrap" title={item.name}>{item.name}</h3>
                  {item.version ? <span className="shrink-0 text-[11px] font-medium leading-[18px] text-[rgba(0_0_0_/.4)]">{item.version}</span> : null}
                </div>
                <div className="shrink-0 flex items-center gap-0.5">
                  <button type="button" className="shrink-0 inline-flex items-center justify-center w-6 h-6 m-0 p-0 border-0 rounded-control bg-none text-[rgba(0_0_0_/.4)] cursor-pointer focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2 enabled:hover:text-[rgba(0_0_0_/.9)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40" title={t('settings.sandbox.skillFiles')} aria-label={t('settings.sandbox.skillFiles')} onClick={() => setFilesTarget({ id: item.id, name: item.name })}><FolderGlyph size={14} /></button>
                  {canDeleteCatalog(item)
                    ? <button type="button" className="shrink-0 inline-flex items-center justify-center w-6 h-6 m-0 p-0 border-0 rounded-control bg-none text-[rgba(0_0_0_/.4)] cursor-pointer focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2 enabled:hover:text-[#e34d59] enabled:hover:bg-[rgb(227_77_89/8%)] disabled:cursor-not-allowed disabled:opacity-40" disabled={deletingId === item.id} title={t('settings.skills.deleteCatalog')} aria-label={t('settings.skills.deleteCatalog')} onClick={() => setPendingDelete(item)}><DeleteGlyph size={14} /></button>
                    : null}
                </div>
              </div>
              {item.description ? <p className="line-clamp-2 m-0 overflow-hidden text-xs leading-[1.5] text-[rgba(0_0_0_/.6)] [overflow-wrap:anywhere]" title={item.description}>{compactSkillText(item.description)}</p> : null}
              <div className="flex items-center min-w-0 mt-auto">
                {view.installs.length === 0 && !view.canAdd ? <span className="text-xs leading-[18px] text-[rgba(0_0_0_/.4)]">{t('settings.skills.noInstalls')}</span>
                  : !view.needsPanel ? (
                    <button type="button" className={`skill-card__chip ${live ? 'skill-card__chip--installed' : 'skill-card__chip--idle'}${chipTone ? ` skill-card__entry--${chipTone}` : ''} ${chipCls}`} disabled={Boolean(view.installs[0] && !recordFor(view.installs[0].sandboxConfigId))} title={chipTooltip} aria-label={summary}
                      onClick={() => { if (view.installs[0]) openManage(item, view.installs[0]); else if (view.canAdd) openInstall(item); }}>
                      {view.installs.some(isInstallBusy) ? <span className={`skill-card__entry-dot w-1.5 h-1.5 rounded-full ${chipTone === 'busy' ? 'bg-[#ed7b2f]' : 'bg-current'} animate-[skill-chip-dot_1.2s_ease-in-out_infinite]`} aria-hidden="true" /> : null}
                      <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{summary}</span>
                      <span className={chipGoCls} aria-hidden="true"><GlyphIcon size={14}><path d="m9 6 6 6-6 6" /></GlyphIcon></span>
                    </button>
                  ) : (
                    <div className="relative inline-flex min-w-0">
                      <button type="button" className={`skill-card__chip ${live ? 'skill-card__chip--installed' : 'skill-card__chip--idle'}${chipTone ? ` skill-card__entry--${chipTone}` : ''} ${chipCls}`} title={chipTooltip} aria-label={summary} aria-expanded={openPanelId === item.id}
                        onClick={() => setOpenPanelId((current) => (current === item.id ? '' : item.id))}>
                        {view.installs.some(isInstallBusy) ? <span className={`skill-card__entry-dot w-1.5 h-1.5 rounded-full ${chipTone === 'busy' ? 'bg-[#ed7b2f]' : 'bg-current'} animate-[skill-chip-dot_1.2s_ease-in-out_infinite]`} aria-hidden="true" /> : null}
                        <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{summary}</span>
                        <span className={chipGoCls} aria-hidden="true"><GlyphIcon size={14}><path d="m6 9 6 6 6-6" /></GlyphIcon></span>
                      </button>
                      {openPanelId === item.id ? (
                        <div className="absolute top-[calc(100%_+_6px)] left-0 z-[3050] flex flex-col w-60 max-w-[calc(100vw_-_32px)] max-h-[min(360px,70vh)] overflow-y-auto py-1 bg-white border border-[#e7e7e7] rounded-[6px] shadow-[0px_3px_14px_2px_rgba(0,0,0,.05),0px_8px_10px_1px_rgba(0,0,0,.06),0px_5px_5px_-3px_rgba(0,0,0,.1)]" data-testid={`skill-install-panel-${item.id}`}>
                          {view.installs.length > 0 ? <>
                            <p className="m-0 pt-1.5 px-3 pb-1 text-xs leading-5 text-[rgba(0_0_0_/.4)]">{t('settings.skills.installPanelGroup')}</p>
                            {view.installs.map((installation) => (
                              <button key={installation.sandboxConfigId} type="button" className={`skill-card__entry--${installEntryTone(item, installation)} flex items-center gap-2 m-0 h-8 w-full shrink-0 px-3 py-0 border-0 bg-none [font:inherit] text-[13px] leading-[22px] text-left cursor-pointer focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2 enabled:hover:bg-[#f3f3f3] disabled:cursor-default disabled:opacity-50 text-[rgba(0_0_0_/.9)]`} disabled={!recordFor(installation.sandboxConfigId)} title={installTooltip(item, installation)}
                                onClick={() => openManage(item, installation)}>
                                <SandboxBadge type={installation.sandboxType} size="xs" />
                                <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{installName(installation)}</span>
                                {isInstallBusy(installation) ? <span className="skill-card__entry-dot w-1.5 h-1.5 rounded-full bg-[#ed7b2f] animate-[skill-chip-dot_1.2s_ease-in-out_infinite]" aria-hidden="true" /> : <span className={`shrink-0 ${installEntryTone(item, installation) === 'ready' ? 'text-[#00a870]' : installEntryTone(item, installation) === 'stale' ? 'text-[#ed7b2f]' : installEntryTone(item, installation) === 'failed' ? 'text-[#e34d59]' : ''}`} aria-hidden="true">{installation.status === 'failed' ? '✕' : installation.status === 'ready' ? '✓' : '!'}</span>}
                              </button>
                            ))}
                          </> : null}
                          {view.available.length > 0 ? <>
                            {view.installs.length > 0 ? <div className="h-px mx-0 my-1 bg-[#e7e7e7]" role="separator" /> : null}
                            <p className="m-0 pt-1.5 px-3 pb-1 text-xs leading-5 text-[rgba(0_0_0_/.4)]">{t('settings.skills.installPanelAvailable')}</p>
                            {view.available.map((config) => (
                              <button key={config.id} type="button" className="group/avail flex items-center gap-2 m-0 h-8 w-full shrink-0 px-3 py-0 border-0 bg-none text-[rgba(0_0_0_/.6)] [font:inherit] text-[13px] leading-[22px] text-left cursor-pointer focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2 enabled:hover:bg-[#f3f3f3] disabled:cursor-default disabled:opacity-50" title={sandboxMetaLine(config)}
                                onClick={() => openInstall(item, config.id)}>
                                <SandboxBadge type={config.sandbox_type} size="xs" />
                                <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{config.name}</span>
                                <span className="shrink-0 text-[rgba(0_0_0_/.4)] group-hover/avail:text-[#07c05f]" aria-hidden="true"><GlyphIcon size={14}><path d="M12 5v14M5 12h14" /></GlyphIcon></span>
                              </button>
                            ))}
                          </> : null}
                        </div>
                      ) : null}
                    </div>
                  )}
              </div>
            </div></div>
          </article>;
        })}
        <button type="button" className="skill-card--add group relative flex flex-col items-center justify-center gap-1.5 p-3 overflow-hidden h-full min-h-[88px] min-w-0 border border-dashed border-[#e7e7e7] rounded-[10px] bg-transparent text-[rgba(0_0_0_/.4)] cursor-pointer [font:inherit] text-center transition-[border-color,box-shadow] duration-[180ms] hover:text-[#07c05f] hover:border-[#07c05f] hover:bg-[rgb(7_192_95/6%)] hover:shadow-none focus-visible:text-[#07c05f] focus-visible:border-[#07c05f] focus-visible:bg-[rgb(7_192_95/6%)] focus-visible:shadow-none focus-visible:outline-2 focus-visible:outline-[#07c05f] focus-visible:-outline-offset-2" onClick={() => setWizardOpen(true)}>
          <span className="flex items-center justify-center w-8 h-8 rounded-card bg-[#f3f3f3] text-[rgba(0_0_0_/.6)] group-hover:bg-[rgb(7_192_95/10%)] group-hover:text-[#07c05f] group-focus-visible:bg-[rgb(7_192_95/10%)] group-focus-visible:text-[#07c05f]" aria-hidden="true"><GlyphIcon size={18}><path d="M12 5v14M5 12h14" /></GlyphIcon></span>
          <span className="text-[13px] font-medium leading-[1.4]">{t('settings.skills.addSkill')}</span>
        </button>
      </div>
    )}
    <AddSkillWizard client={client} open={wizardOpen} catalog={catalog} configs={skillConfigs} installer={installer} t={t}
      onClose={(registeredId) => { setWizardOpen(false); void load(true); if (registeredId) revealCatalog(registeredId); }}
      onCatalogChanged={() => void load(true)}
      onToast={(tone, message) => setToast({ tone, message })}
      onManage={(record, skillId, catalogName) => { setWizardOpen(false); setManageTarget({ record, skillId, catalogName }); }} />
    <InstallSkillDialog client={client} open={installItem !== null} item={installItem} configs={skillConfigs} preselectConfigId={installPreselectId} installer={installer} t={t}
      onClose={() => { setInstallItem(null); setInstallPreselectId(''); }}
      onCatalogChanged={() => void load(true)}
      onToast={(tone, message) => setToast({ tone, message })}
      onManage={(record, skillId, catalogName) => { setInstallItem(null); setManageTarget({ record, skillId, catalogName }); }} />
    <ManageSkillDialog client={client} open={manageTarget !== null} target={manageTarget} t={t}
      onClose={() => setManageTarget(null)}
      onChanged={() => void load(true)}
      onToast={(tone, message) => setToast({ tone, message })} />
    <CatalogFilesDialog client={client} open={filesTarget !== null} target={filesTarget} t={t} onClose={() => setFilesTarget(null)} />
    <Dialog open={pendingDelete !== null} title={t('common.confirmDelete')} onClose={() => setPendingDelete(null)}>
      {pendingDelete ? <>
        <p>{t('settings.skills.deleteCatalogConfirm', { name: pendingDelete.name })}</p>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <Button type="button" onClick={() => setPendingDelete(null)}>{t('common.cancel')}</Button>
          <Button type="button" loading={deletingId !== ''} onClick={() => { const item = pendingDelete; setPendingDelete(null); void removeCatalog(item); }}>{t('common.delete')}</Button>
        </div>
      </> : null}
    </Dialog>
  </section>;
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
  return <label className="grid gap-[.35rem] text-[#27364d] font-semibold">{t('settings.sandbox.skillInstallerModel')}
    <Select className="w-full [font:inherit]" value={installer.modelId} disabled={installer.saving} onChange={(event) => installer.onChange(event.target.value)}>
      <option value="">{t('settings.sandbox.skillInstallerModelRequired')}</option>
      {installer.models.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
    </Select>
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
  if (rows.length === 0) return <p className="wk-muted text-muted">{t('settings.skills.noSandboxToInstall')}</p>;
  // Vue .sandbox-pick-list rows (SkillSettings.vue:1841-1953): 1px #e7e7e7
  // border, radius 10, 10px/12px padding; hover/checked tints are 40%/4-5%
  // brand mixes; busy rows take a 35% warning border.
  const pickRowBase = 'min-w-0 max-w-full box-border flex items-center gap-2.5 px-3 py-[10px] border border-[#e7e7e7] rounded-[10px] bg-white';
  return <div className="grid gap-2 w-full">
    {rows.map((row) => row.selectable ? (
      <label key={row.config.id} className={`cursor-pointer ${pickRowBase} hover:border-[rgb(7_192_95/40%)] hover:bg-[rgb(7_192_95/4%)] has-[:checked]:border-[rgb(7_192_95/40%)] has-[:checked]:bg-[rgb(7_192_95/5%)]`}>
        <Checkbox checked={targetIds.includes(row.config.id)} onChange={(event) => onToggle(row.config.id, event.target.checked)} />
        <span className="flex items-center gap-2.5 flex-1 min-w-0">
          <SandboxBadge type={row.config.sandbox_type} size="sm" />
          <span className="flex flex-col gap-px min-w-0"><span className="text-[13px] font-medium text-[rgba(0_0_0_/.9)] leading-[1.3] overflow-hidden text-ellipsis whitespace-nowrap">{row.config.name}</span><span className="text-xs text-[rgba(0_0_0_/.6)] leading-[1.3] overflow-hidden text-ellipsis whitespace-nowrap">{metaLine(row.config)}</span></span>
        </span>
      </label>
    ) : (
      <div key={row.config.id} className={`${pickRowBase}${row.busy ? ' border-[rgb(237_123_47/35%)]' : ''}`}>
        <span className="flex items-center gap-2.5 flex-1 min-w-0">
          <SandboxBadge type={row.config.sandbox_type} size="sm" />
          <span className="flex flex-col gap-px min-w-0">
            <span className="text-[13px] font-medium text-[rgba(0_0_0_/.9)] leading-[1.3] overflow-hidden text-ellipsis whitespace-nowrap">{row.config.name}</span>
            <span className="text-xs text-[rgba(0_0_0_/.6)] leading-[1.3] overflow-hidden text-ellipsis whitespace-nowrap">{row.install && isInstallBusy(row.install) ? installStatusKeys(row.install.status, row.install.enabled).map((key) => t(key)).join(' ') : row.ready ? t('settings.sandbox.skillStatusReady') : metaLine(row.config)}</span>
          </span>
        </span>
        {row.busy && row.install ? <div className="flex items-center justify-end gap-1.5 shrink-0 text-xs font-medium leading-none text-[#07c05f] [&_.skill-manage__progress]:w-[18px] [&_.skill-manage__progress]:h-[18px] [&_.skill-manage__progress]:m-0">
          <ProgressRing percent={installProgressPercent(progressByConfig[row.config.id], row.install.status)} />
          {progressByConfig[row.config.id] ? <span>{installProgressPercent(progressByConfig[row.config.id], row.install.status)}%</span> : null}
          <Button type="button" variant="text" size="small" className="text-[#07c05f]!" onClick={() => onManage(row.config, row.install!)}>{t('settings.skills.viewInstallProgress')}</Button>
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
      <span className="text-[13px] text-primary [overflow-wrap:anywhere]">{t('settings.sandbox.skillUploading', { percent })}</span>
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

  return <DrawerShell open={open} spec={SKILL_DRAWER_SPECS.add}>
  <Dialog open={open} title={<DrawerTitle icon={<SkillGlyph size={16} />} title={t('settings.skills.addSkill')} subtitle={stepDescription} />} onClose={() => onClose(registeredId || undefined)}>
    <nav className="skill-add-steps" aria-label={t('settings.skills.addProgress')}>
      {steps.map((title, index) => {
        const clickable = canJump(index);
        return <button key={title} type="button" disabled={!clickable} aria-current={step === index ? 'step' : undefined}
          className={`skill-add-step${step === index ? ' is-active' : ''}${step > index ? ' is-done' : ''}${clickable ? ' is-clickable' : ''}`}
          onClick={() => { if (clickable) setStep(index); }}>
          <span className="skill-add-step__marker">{step > index ? '✓' : index + 1}</span>
          <span className="skill-add-step__title">{title}</span>
        </button>;
      })}
    </nav>
    {error ? <Status tone="error">{error}</Status> : null}
    {step > 0 && parsedCard ? <article className="parsed-skill relative flex flex-col p-0 overflow-hidden rounded-[10px] bg-white transition-[border-color,box-shadow] duration-[180ms] min-w-0 h-full border border-line"><div className="flex items-stretch p-3 min-w-0 flex-1"><div className="flex-1 min-w-0 flex flex-col gap-2">
      <div className="flex items-center gap-2.5 min-w-0 min-h-[28px]">
        <span className="shrink-0 w-[26px] h-[26px] rounded-[7px] flex items-center justify-center text-sm bg-[#f4f6fa] text-muted-strong" aria-hidden="true">⚡</span>
        <div className="flex-1 min-w-0 flex items-baseline gap-1.5">
          <h3 className="flex-[0_1_auto] min-w-0 m-0 text-sm font-semibold leading-5 text-ink overflow-hidden text-ellipsis whitespace-nowrap" title={parsedCard.name}>{parsedCard.name}</h3>
          {parsedCard.version ? <span className="shrink-0 text-[11px] font-medium leading-[18px] text-muted">{parsedCard.version}</span> : null}
        </div>
      </div>
      {parsedCard.description ? <p className="line-clamp-2 m-0 overflow-hidden text-xs leading-[1.5] text-muted-strong [overflow-wrap:anywhere]" title={parsedCard.description}>{compactSkillText(parsedCard.description)}</p> : null}
    </div></div></article> : null}
    {step === 0 ? <>
      <section className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]">
        <h4>{t('settings.sandbox.skillSourceSection')}</h4>
        <p className="wk-muted text-muted">{t('settings.sandbox.skillSourceSectionHint', { size: maxSkillBundleMB() })}</p>
        <label>{t('settings.sandbox.skillSourcePlaceholder')}
          <Input value={source} placeholder={t('settings.sandbox.skillSourcePlaceholder')} disabled={addBusy || Boolean(registeredId)} onChange={(event) => setSource(event.target.value)} />
        </label>
      </section>
      <section className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]">
        <h4>{t('settings.sandbox.skillUploadSection')}</h4>
        <p className="wk-muted text-muted">{t('settings.sandbox.skillUploadSectionHint', { size: maxSkillBundleMB() })}</p>
        <input ref={fileInputRef} type="file" accept=".zip,application/zip" className="absolute w-px h-px overflow-hidden [clip:rect(0_0_0_0)] whitespace-nowrap" disabled={addBusy || Boolean(registeredId)} onChange={(event) => acceptFile(event.currentTarget.files?.[0] ?? null)} />
        <div className={`flex items-center justify-center min-h-24 p-3.5 border rounded-[10px] cursor-pointer text-center ${pendingFile ? 'border-solid border-primary bg-[#f4f8ff]' : 'border-dashed border-line-control bg-[#fafbfd]'}`} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); acceptFile(event.dataTransfer.files?.[0] ?? null); }}>
          {uploading ? <SkillUploadProgress percent={uploadPercent} t={t} /> : pendingFile ? <span className="text-[13px] text-primary [overflow-wrap:anywhere]">{t('settings.skills.addFileSelected', { name: pendingFile.name })}</span> : <>
            <span className="block text-[13px] font-medium text-[#27364d]">{t('settings.sandbox.skillUploadClick')}</span>
            <span className="block mt-0.5 text-xs text-muted">{t('settings.sandbox.skillUploadDrag')}</span>
          </>}
        </div>
        {pendingFile && !registeredId ? <Button type="button" disabled={addBusy} onClick={() => { setPendingFile(null); if (fileInputRef.current) fileInputRef.current.value = ''; }}>{t('settings.skills.addClearFile')}</Button> : null}
      </section>
    </> : <>
      {configs.length > 0 ? <section className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]">
        <h4>{t('settings.skills.pickSandboxes')}</h4>
        <p className="wk-muted text-muted">{t('settings.skills.pickSandboxesHint')}</p>
        <SandboxPickList client={client} item={pickItem} configs={configs} mode="all" sessionIds={sessionIds} targetIds={targetIds} onToggle={setPick} t={t}
          metaLine={(record) => { const label = t(backendLabelKey(record.sandbox_type)); const target = sandboxTargetLine(record); return target ? `${label} · ${target}` : label; }}
          onManage={(record, installation) => { if (installation.skillId) onManage(record, installation.skillId, parsedCard?.name ?? ''); }} />
      </section> : <p className="wk-muted text-muted">{t('settings.skills.emptyNoSandboxHint')}</p>}
      {targetIds.length > 0 ? <section className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]">
        <h4>{t('settings.sandbox.skillInstallerModel')}</h4>
        <p className="wk-muted text-muted">{t('settings.sandbox.skillInstallerModelHint')}</p>
        <InstallerModelSelect installer={installer} t={t} />
      </section> : null}
    </>}
    <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
      {step > 0 ? <Button type="button" onClick={() => setStep((current) => Math.max(0, current - 1))}>{t('settings.sandbox.back')}</Button> : null}
      <Button type="button" loading={primaryLoading} disabled={primaryDisabled} onClick={() => void handlePrimary()}>{primaryText}</Button>
    </div>
  </Dialog>
  </DrawerShell>;
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

  return <DrawerShell open={open} spec={SKILL_DRAWER_SPECS.install}>
  <Dialog open={open} title={<DrawerTitle icon={<SkillGlyph size={16} />} title={t('settings.skills.installToSandbox')} subtitle={description} />} onClose={onClose}>
    {error ? <Status tone="error">{error}</Status> : null}
    <SandboxPickList client={client} item={item} configs={configs} mode="remaining" sessionIds={sessionIds} targetIds={targetIds}
      onToggle={(configId, checked) => setTargetIds((current) => (checked ? [...new Set([...current, configId])] : current.filter((id) => id !== configId)))} t={t}
      metaLine={(record) => { const label = t(backendLabelKey(record.sandbox_type)); const target = sandboxTargetLine(record); return target ? `${label} · ${target}` : label; }}
      onManage={(record, installation) => { if (installation.skillId && item) onManage(record, installation.skillId, item.name); }} />
    {targetIds.length > 0 ? <section className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]">
      <h4>{t('settings.sandbox.skillInstallerModel')}</h4>
      <p className="wk-muted text-muted">{t('settings.sandbox.skillInstallerModelHint')}</p>
      <InstallerModelSelect installer={installer} t={t} />
    </section> : null}
    <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
      <Button type="button" onClick={onClose}>{t('common.cancel')}</Button>
      <Button type="button" loading={installing} disabled={confirmDisabled} onClick={() => void confirm()}>{confirmText}</Button>
    </div>
  </Dialog>
  </DrawerShell>;
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

  return <section className="skill-timeline flex flex-col pt-[10px] pr-3 pb-3 pl-3 bg-transparent border-0 rounded-none" aria-busy={loading}>
    <div className="flex-1 min-w-0">
      {loading && timeline.frames === 0 ? <Status>{t('common.loading')}</Status>
        : timeline.frames === 0 ? <p className="my-1 text-[#999] text-xs">{live ? t('settings.sandbox.skillTranscriptWaiting') : t('settings.sandbox.skillTranscriptEmpty')}</p>
        : <>
          {timeline.prompt ? <pre className="skill-timeline__prompt max-h-[72px] m-0 px-2 py-1.5 overflow-y-auto text-muted text-[11px] leading-[1.5] bg-white rounded-control whitespace-pre-wrap break-words">{timeline.prompt}</pre> : null}
          {timeline.thinking ? <div className="my-2 px-[10px] py-2 text-muted text-xs leading-[1.55] whitespace-pre-wrap [overflow-wrap:anywhere] bg-white rounded-control">{timeline.thinking}</div> : null}
          {timeline.toolCalls.map((call) => (
            <div key={call.id} className="flex items-baseline gap-1.5 my-1 text-xs leading-[1.5]">
              <span className={`shrink-0 w-1.5 h-1.5 rounded-full self-center ${call.status === 'completed' ? 'bg-[#067647]' : call.status === 'failed' ? 'bg-danger' : 'bg-[#b45309]'}`} aria-hidden="true" />
              <span className="text-[#27364d] font-medium [overflow-wrap:anywhere]">{call.name ?? call.id}</span>
              {typeof call.result === 'string' && call.result ? <span className="min-w-0 text-muted whitespace-pre-wrap [overflow-wrap:anywhere]">{call.result}</span> : null}
            </div>
          ))}
          {timeline.answer ? <div className="markdown-content min-w-0 text-xs leading-[1.55] text-ink [overflow-wrap:anywhere] [&_p]:m-0 [&_p]:mb-[0.5em] [&_p:last-child]:mb-0 [&_pre]:overflow-x-auto [&_pre]:text-[11px] [&_h1]:m-[0.4em_0_0.3em] [&_h1]:text-xs [&_h2]:m-[0.4em_0_0.3em] [&_h2]:text-xs [&_h3]:m-[0.4em_0_0.3em] [&_h3]:text-xs" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(timeline.answer) }} /> : null}
          {timeline.error ? <p className="mt-2 mb-0 text-danger text-xs" role="alert">{timeline.error}</p> : null}
        </>}
      {guidance.messages.map((item) => (
        <div key={item.id} className="my-2 p-2 bg-white rounded-control [&_span]:text-xs [&_span]:text-muted [&_p]:mt-1 [&_p]:mb-0 [&_p]:whitespace-pre-wrap [&_p]:[overflow-wrap:anywhere]">
          <span>{t(`settings.sandbox.skillGuidance.${item.status}`)}</span>
          <p>{item.content}</p>
        </div>
      ))}
    </div>
    {live || canRetry ? <div className="sticky bottom-0 z-[1] shrink-0 mt-3 pt-3 bg-[var(--wk-dialog-bg,#fff)] border-t border-line-neutral">
      <Textarea value={guidanceText} maxLength={10000} rows={2} disabled={sendingGuidance}
        className="box-border w-full resize-y min-h-[44px] max-h-[132px] border border-line-control rounded-control bg-white text-ink [font:inherit] text-[13px] px-[10px] py-1.5 disabled:bg-canvas disabled:text-muted"
        placeholder={t('settings.sandbox.skillGuidance.placeholder')}
        onChange={(event) => setGuidanceText(event.target.value)} />
      {guidanceError ? <p role="alert" className="mt-1.5 mb-0 text-danger text-xs">{guidanceError}</p> : null}
      <div className="flex items-center justify-end gap-3 mt-2 [&_span]:flex-1 [&_span]:text-xs [&_span]:text-muted">
        <span>{live && !guidance.accepting ? t('settings.sandbox.skillGuidance.unavailable') : ''}</span>
        <Button type="button" loading={sendingGuidance} disabled={!guidanceText.trim() || (live && !guidance.accepting)}
          onClick={() => void sendGuidance()}>
          {t(live ? 'settings.sandbox.skillGuidance.send' : 'settings.sandbox.skillGuidance.retry')}
        </Button>
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
  return <DrawerShell open={open} spec={SKILL_DRAWER_SPECS.manage}>
  <Dialog open={open} title={<DrawerTitle icon={<SkillGlyph size={16} />} title={target?.catalogName ?? ''} subtitle={target ? t('settings.skills.manageDrawerDesc', { name: target.record.name }) : undefined} />} onClose={onClose}>
    {loading ? <Status>{t('common.loading')}</Status> : null}
    {error ? <Status tone="error">{error}</Status> : null}
    {skill ? uninstallDone ? <div className="flex flex-col items-start gap-2.5 pt-2 pb-1 text-[#067647] [&_p]:m-0 [&_p]:text-[13px]">
      <span aria-hidden="true">✓</span>
      <p>{t('settings.sandbox.skillRemoveDone', { name: skill.name })}</p>
    </div> : busy && skill.status === 'removing' ? <section className="flex flex-col items-stretch gap-[10px] pt-3 border-t border-line-soft [&_h4]:m-0 [&_h4]:text-[13px] [&_h4]:font-semibold [&_h4]:text-ink">
      <div className="flex items-center justify-between gap-3">
        <h4>{t('settings.sandbox.skillRemoveInProgress')}</h4>
        <ProgressRing percent={installProgressPercent(progress, skill.status)} />
      </div>
      <p className="m-0 text-[13px] leading-[1.5] text-muted-strong">{removeStageText(progress, skill, t)}</p>
    </section> : <>
      <div className="flex items-start justify-between gap-4 max-[720px]:flex-col">
        <div className="min-w-0 [&_label]:block [&_label]:mb-1 [&_label]:text-sm [&_label]:font-medium [&_label]:text-ink">
          <label>{t('settings.skills.manageEnable')}</label>
          <p className="wk-muted m-0 text-xs leading-[1.5] text-muted-strong!">{t('settings.sandbox.skillDisableHint')}</p>
          <p className="wk-muted m-0 text-xs leading-[1.5] text-muted-strong!">{installStatusKeys(skill.status, skill.enabled).map((key) => t(key)).join(' · ')}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <label className="inline-flex shrink-0 cursor-pointer items-center gap-2"><Switch className="h-[18px]! w-[34px]!" aria-label={t('settings.skills.manageEnable')} checked={skill.enabled} disabled={busy} onCheckedChange={(checked) => void toggleEnabled(checked)} /> {toggling ? '…' : ''}</label>
          {skill.status === 'failed' ? <Button type="button" loading={retrying} disabled={busy} title={t('settings.sandbox.skillRetryHint')} onClick={() => void retry()}>{t('settings.sandbox.skillRetry')}</Button> : null}
          {skill.status === 'installing' ? <Button type="button" loading={stopping} disabled={busy} title={t('settings.sandbox.skillStopHint')} onClick={() => void stop()}>{t('settings.sandbox.skillStop')}</Button> : null}
          {skill.status !== 'installing' ? <Button type="button" loading={uninstalling} disabled={busy} title={t('settings.skills.manageUninstallConfirm', { name: skill.name })} onClick={() => setPendingUninstall(true)}>{t('settings.skills.manageUninstall')}</Button> : null}
        </div>
      </div>
      {pendingUninstall ? <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
        <span className="mr-auto text-[0.85rem] text-muted">{t('settings.skills.manageUninstallConfirm', { name: skill.name })}</span>
        <Button type="button" onClick={() => setPendingUninstall(false)}>{t('common.cancel')}</Button>
        <Button type="button" loading={uninstalling} onClick={() => void uninstall()}>{t('common.delete')}</Button>
      </div> : null}
      {errorLines.length > 0 ? <ul className="mt-1 mb-0 pl-[18px] text-danger text-xs leading-[1.6]">{errorLines.map((line, index) => <li key={index}>{line}</li>)}</ul> : null}
      {(skill.envs ?? []).length > 0 ? <section className="flex flex-col items-stretch gap-[10px] pt-3 border-t border-line-soft [&_h4]:m-0 [&_h4]:text-[13px] [&_h4]:font-semibold [&_h4]:text-ink">
        <h4>{t('settings.sandbox.skillEnv.toggle')}</h4>
        <p className="wk-muted text-muted">{t('settings.sandbox.skillEnv.workspaceHint')}</p>
        <div className="grid gap-2.5">
          {(skill.envs ?? []).map((env) => <div key={env.name} className="grid gap-1.5 py-2 border-b border-[#f1f4f9]">
            <div className="flex items-center flex-wrap gap-1.5">
              <code className="text-xs text-[#27364d]">{env.name}</code>
              {env.required ? <span className="px-1.5 py-px rounded-pill text-[11px] leading-4 bg-[rgb(180_35_24/8%)] text-danger">{t('settings.sandbox.skillEnv.required')}</span> : null}
              <span className={`px-1.5 py-px rounded-pill text-[11px] leading-4 ${env.isSet ? 'bg-[rgb(6_118_71/8%)] text-[#067647]' : 'bg-[#f1f4f9] text-muted'}`}>{env.isSet ? t('settings.sandbox.skillEnv.isSet') : t('settings.sandbox.skillEnv.notSet')}</span>
              {env.description ? <span className="text-xs text-muted">{env.description}</span> : null}
            </div>
            <div className="flex items-center gap-2 [&_input]:flex-1 [&_input]:min-w-0 [&_input]:box-border [&_input]:border [&_input]:border-line-control [&_input]:rounded-control [&_input]:px-[10px] [&_input]:py-1.5 [&_input]:[font:inherit] [&_input]:text-[13px]">
              <Input type="password" autoComplete="new-password" spellCheck={false} aria-label={env.name}
                placeholder={env.isSet ? t('settings.sandbox.skillEnv.placeholderSet') : t('settings.sandbox.skillEnv.placeholderUnset')}
                value={envDrafts[env.name] ?? ''} disabled={busy || envSaving}
                onChange={(event) => setEnvDrafts((current) => ({ ...current, [env.name]: event.target.value }))} />
              {canClearAdminSkillEnv(env) ? <>
                <Button type="button" disabled={envSaving} onClick={() => setPendingClearEnv(env.name)}>{t('settings.sandbox.skillEnv.clear')}</Button>
                {pendingClearEnv === env.name ? <span>
                  {t('settings.sandbox.skillEnv.clearConfirm', { name: env.name })}
                  <Button type="button" onClick={() => setPendingClearEnv('')}>{t('common.cancel')}</Button>
                  <Button type="button" loading={envSaving} onClick={() => { const name = env.name; setPendingClearEnv(''); void submitEnvs(adminSkillEnvClearPayload(name), 'settings.sandbox.skillEnv.clearSuccess'); }}>{t('common.delete')}</Button>
                </span> : null}
              </> : null}
            </div>
          </div>)}
        </div>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <Button type="button" loading={envSaving} disabled={Object.keys(envPayload()).length === 0 || busy} onClick={() => void saveEnvs()}>{t('settings.sandbox.skillEnv.save')}</Button>
        </div>
      </section> : null}
      {hasTranscript(skill) ? <section className="skill-manage__section--transcript flex flex-col items-stretch gap-[10px] pt-3 border-t border-line-soft [&_h4]:m-0 [&_h4]:text-[13px] [&_h4]:font-semibold [&_h4]:text-ink">
        <div className="flex items-center justify-between gap-3">
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
  </Dialog>
  </DrawerShell>;
}

/** Vue focus-drawer progress ring (SandboxSkillsPanel.vue:239-248 t-progress circle + percent). */
function ProgressRing({ percent }: { percent: number }) {
  const circumference = 2 * Math.PI * 7;
  const clamped = Math.max(0, Math.min(100, percent));
  return <div className="skill-manage__progress inline-flex items-center gap-1.5 text-xs font-medium leading-none text-primary [&_svg]:block">
    <svg viewBox="0 0 18 18" width="18" height="18" aria-hidden="true">
      <circle className="stroke-line-soft" cx="9" cy="9" r="7" fill="none" strokeWidth="2" />
      <circle className="stroke-primary" cx="9" cy="9" r="7" fill="none" strokeWidth="2" strokeLinecap="round"
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

  return <Dialog open={open} title={target?.name ?? ''} onClose={onClose}>
    <p className="wk-muted text-muted">{t('settings.sandbox.skillFilesTitle')}</p>
    <div className="grid grid-cols-[minmax(160px,220px)_minmax(0,1fr)] gap-3 min-h-[260px] max-[720px]:grid-cols-1">
      <aside className="min-w-0 max-h-[420px] overflow-y-auto">
        {listError ? <p className="wk-muted text-muted">{listError}</p>
          : listLoading ? <Status>{t('common.loading')}</Status>
          : rows.length === 0 ? <p className="wk-muted text-muted">{t('settings.sandbox.skillFilesEmpty')}</p>
          : <ul className="m-0 p-0 list-none grid gap-0.5">
            {rows.map((row) => (
              <li key={row.path}>
                <button type="button" className={`flex items-center gap-1 w-full px-2 py-[5px] border-0 rounded-control [font:inherit] text-[13px] text-left cursor-pointer ${selectedPath === row.path ? 'bg-[#eef4ff] text-primary-deep' : 'bg-none text-[#27364d] hover:bg-[#f4f6fa]'}${row.isDir ? ' font-medium' : ''}`} title={row.path}
                  onClick={() => { if (row.isDir) setExpanded((current) => { const next = new Set(current); if (next.has(row.path)) next.delete(row.path); else next.add(row.path); return next; }); else void selectFile(row.path); }}>
                  <span className="skill-files-panel__indent" style={{ width: `${row.depth * 12}px` }} />
                  <span aria-hidden="true">{row.isDir ? (expanded.has(row.path) ? '▾' : '▸') : '·'}</span>
                  <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{row.name}</span>
                </button>
              </li>
            ))}
          </ul>}
      </aside>
      <section className="min-w-0 flex flex-col gap-2">
        {selectedPath ? <div className="flex items-center gap-2">
          <span className="flex-1 min-w-0 text-xs text-muted overflow-hidden text-ellipsis whitespace-nowrap" title={selectedPath}>{selectedPath}</span>
          {markdown && file?.encoding === 'utf-8' ? <Button type="button" onClick={() => setMarkdownSource((current) => !current)}>{markdownSource ? t('settings.sandbox.skillFilesPreview') : t('settings.sandbox.skillFilesSource')}</Button> : null}
          {file?.content && file.encoding === 'utf-8' ? <Button type="button" onClick={() => void copyContent()}>{copied ? t('common.copied') : t('common.copy')}</Button> : null}
        </div> : null}
        <div className="min-w-0 flex-1 flex flex-col gap-2">
          {fileLoading ? <Status>{t('common.loading')}</Status>
            : fileError ? <Status tone="error">{fileError}</Status>
            : !selectedPath ? <p className="wk-muted text-muted">{t('settings.sandbox.skillFilesSelectHint')}</p>
            : <>
              {file?.truncated ? <Status tone="warning">{t('settings.sandbox.skillFilesTruncated')}</Status> : null}
              {imageSrc ? <img className="max-w-full rounded-card border border-line-soft" src={imageSrc} alt={selectedPath} />
                : markdown && file?.encoding === 'utf-8' && file.content != null && !markdownSource ? <>
                  {frontmatter && frontmatter.fields.length > 0 ? <dl className="skill-files-panel__meta m-0 mb-2.5 px-3 py-2.5 bg-[#f7f8fa] border border-line-soft rounded-card grid gap-1.5 text-xs">
                    {frontmatter.fields.map((field) => <div key={field.key} className="grid grid-cols-[minmax(72px,max-content)_1fr] gap-2.5 [&_dt]:text-muted [&_dd]:m-0 [&_dd]:text-ink [&_dd]:[overflow-wrap:anywhere]"><dt>{field.key}</dt><dd className={field.code ? 'font-mono whitespace-pre-wrap' : undefined}>{field.value}</dd></div>)}
                  </dl> : null}
                  {/* Vue renders the markdown body as HTML (SkillFilesPanel.vue:87-91, 476-478); the shared renderer escapes raw HTML and allow-lists links. */}
                  <div className="skill-files-panel__markdown markdown-content min-w-0 max-h-[420px] overflow-y-auto px-3.5 py-3 bg-white border border-line-soft rounded-card text-[13px] leading-[1.65] text-ink [overflow-wrap:anywhere] [&_h1]:m-[0.8em_0_0.4em] [&_h1]:leading-[1.3] [&_h1]:text-lg [&_h2]:m-[0.8em_0_0.4em] [&_h2]:leading-[1.3] [&_h2]:text-base [&_h3]:m-[0.8em_0_0.4em] [&_h3]:leading-[1.3] [&_h3]:text-sm [&_p]:my-[0.4em] [&_pre]:overflow-x-auto [&_pre]:px-3 [&_pre]:py-2.5 [&_pre]:bg-[#f7f8fa] [&_pre]:rounded-card [&_pre]:text-xs [&_code]:font-mono [&_code]:text-xs [&_table]:border-collapse [&_th]:border [&_th]:border-line-soft [&_th]:px-2 [&_th]:py-1 [&_td]:border [&_td]:border-line-soft [&_td]:px-2 [&_td]:py-1 [&_img]:max-w-full" dangerouslySetInnerHTML={{ __html: renderChatMarkdown(frontmatter?.body ?? file.content) }} />
                </>
                : file?.encoding === 'utf-8' && file.content != null ? <pre className="m-0 px-3 py-2.5 overflow-auto max-h-[380px] bg-[#f7f8fa] border border-line-soft rounded-card text-xs leading-[1.55]"><code>{file.content}</code></pre>
                : <p className="wk-muted text-muted">{t('settings.sandbox.skillFilesBinary')}</p>}
            </>}
        </div>
      </section>
    </div>
  </Dialog>;
}
