import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import type { ChatMessage } from '@weknora/contracts';
import { normalizeArtifactList, type ChatArtifact } from '@weknora/domain/chat/artifacts';
import type { ChatCopyTable } from './chat-copy.ts';
import { SpriteIcon } from './message-face.tsx';

/*
 * px-chat-sandbox 收敛件：SandboxSidePanel 三 tab 侧面板（产物/终端/桌面）。
 * Vue 事实源：
 * - frontend/src/components/chat/SandboxSidePanel.vue（面板 chrome + 拖宽把手）
 * - frontend/src/views/chat/components/ChatArtifactsPanel.vue（产物 tab 内容）
 * - frontend/src/composables/useChatSandboxPanel.ts（宽度持久化 + 收敛边界）
 * - frontend/src/utils/sessionArtifacts.ts（会话产物收集）
 * 面板 position:fixed 贴视口右缘（Vue 同款 in-tree fixed，无 portal 需求）。
 */

export type SandboxPanelTab = 'artifacts' | 'terminal' | 'desktop';

export const SANDBOX_PANEL_MIN_WIDTH = 320;
export const SANDBOX_PANEL_MAX_WIDTH = 1200;
export const SANDBOX_PANEL_DEFAULT_WIDTH = 420;

const SANDBOX_PANEL_WIDTH_STORAGE_KEY = 'sandbox_panel_width';

function clampPanelWidth(width: number): number {
  if (typeof window === 'undefined') return SANDBOX_PANEL_DEFAULT_WIDTH;
  const viewportCap = Math.max(SANDBOX_PANEL_MIN_WIDTH, window.innerWidth - 480);
  return Math.min(
    SANDBOX_PANEL_MAX_WIDTH,
    viewportCap,
    Math.max(SANDBOX_PANEL_MIN_WIDTH, Math.round(width)),
  );
}

export function initialSandboxPanelWidth(): number {
  if (typeof window === 'undefined') return SANDBOX_PANEL_DEFAULT_WIDTH;
  try {
    const raw = Number(window.localStorage.getItem(SANDBOX_PANEL_WIDTH_STORAGE_KEY));
    return Number.isFinite(raw) && raw > 0 ? clampPanelWidth(raw) : SANDBOX_PANEL_DEFAULT_WIDTH;
  } catch {
    return SANDBOX_PANEL_DEFAULT_WIDTH;
  }
}

/** Vue sessionArtifacts.ts collectSessionArtifacts — flatten every assistant
 *  message's artifacts in list order, each row keeping the owning message id. */
export interface SandboxArtifactItem extends ChatArtifact {
  messageId: string;
}

export function collectSessionArtifacts(messages: readonly ChatMessage[]): SandboxArtifactItem[] {
  const items: SandboxArtifactItem[] = [];
  for (const message of messages) {
    const raw = (message as { artifacts?: unknown }).artifacts;
    const list = normalizeArtifactList(Array.isArray(raw) ? raw : undefined);
    if (list.length === 0) continue;
    for (const artifact of list) {
      items.push({ ...artifact, messageId: message.id });
    }
  }
  return items;
}

/** Vue sessionArtifacts.ts formatArtifactSize / formatArtifactDateTime. */
export function formatArtifactSize(size: number | undefined | null): string {
  if (!size || size < 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return unit === 0 ? `${value} ${units[unit]}` : `${value.toFixed(1)} ${units[unit]}`;
}

export function formatArtifactDateTime(raw: string | undefined | null): string {
  if (!raw) return '—';
  const parsed = new Date(raw);
  if (Number.isNaN(parsed.getTime())) return String(raw);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())} ${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
}

export interface SandboxSidePanelProps {
  copy: ChatCopyTable;
  visible: boolean;
  activeTab: SandboxPanelTab;
  width: number;
  sessionId: string;
  /** 参考来源面板同开时整体左移（Vue shifted，≥1400px 视口）。 */
  shifted?: boolean;
  artifacts?: readonly SandboxArtifactItem[];
  artifactsCollecting?: boolean;
  /** 终端 tab 内容（React TerminalPanel 面，由 page.tsx 注入；惰性挂载）。 */
  terminal?: ReactNode;
  onTabChange(tab: SandboxPanelTab): void;
  onClose(): void;
  onWidthChange(width: number): void;
  onArtifactDownload?(item: SandboxArtifactItem): void;
  /** Vue 行内 DocumentPreview 之外的宿主预览面（React artifact-preview 通道）。 */
  onArtifactPreview?(item: SandboxArtifactItem): void;
}

/** Vue utils/files.ts getFileIcon —— 产物文件图标 kind（色相族见 chat.td.css §16）。 */
function artifactFileIconKind(fileName: string): string {
  const ext = (fileName.split('.').pop() ?? '').toLowerCase();
  if (!ext) return 'file';
  if (ext === 'pdf') return 'file-pdf';
  if (ext === 'doc' || ext === 'docx') return 'file-word';
  if (ext === 'xls' || ext === 'xlsx' || ext === 'csv') return 'file-excel';
  if (ext === 'ppt' || ext === 'pptx') return 'file-powerpoint';
  if (['txt', 'md', 'markdown', 'json', 'log', 'yaml', 'yml', 'xml'].includes(ext)) return 'file';
  if (['py', 'pyc', 'pyo', 'js', 'mjs', 'cjs', 'ts', 'tsx', 'jsx', 'go', 'rs', 'java', 'c', 'cc', 'cpp', 'h', 'hpp', 'sh', 'bash', 'rb', 'php', 'sql', 'html', 'htm'].includes(ext)) return 'code';
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg'].includes(ext)) return 'image';
  if (['mp3', 'wav', 'm4a', 'flac', 'ogg', 'aac'].includes(ext)) return 'sound';
  if (['mp4', 'mov', 'webm', 'mkv', 'avi'].includes(ext)) return 'video';
  return 'file';
}

/** Vue ArtifactFileIcon.vue + utils/artifactFileIcon.ts renderArtifactFileIcon
 *  （同构 svg；kind 色相由 CSS 类承载）。 */
function ArtifactFileIcon({ fileName }: { fileName: string }) {
  const ext = (fileName.split('.').pop() ?? '').toLowerCase();
  const label = /^[a-z0-9]{1,4}$/i.test(ext) ? ext.toUpperCase() : 'FILE';
  return (
    <span className="artifact-file-icon" aria-hidden="true">
      <svg className={`artifact-file-icon-svg kind-${artifactFileIconKind(fileName)}`} viewBox="0 0 32 38" fill="none" aria-hidden="true">
        <path className="file-sheet" d="M6 1.5h13L28.5 11v22A3.5 3.5 0 0 1 25 36.5H6A3.5 3.5 0 0 1 2.5 33V5A3.5 3.5 0 0 1 6 1.5Z" />
        <path className="file-fold" d="M19 1.5V8a3 3 0 0 0 3 3h6.5" />
        <path className="file-lines" d="M8 14h9M8 18h14" />
        <rect x="5" y="23" width="24" height="11" rx="3" fill="currentColor" />
        <text x="17" y="30.8" text-anchor="middle">{label}</text>
      </svg>
    </span>
  );
}

/** Vue ChatArtifactsPanel.vue（空态/筛选/列表；预览走宿主通道）。 */
function ChatArtifactsPanel(props: {
  className?: string;
  style?: React.CSSProperties;
  copy: ChatCopyTable;
  sessionId: string;
  items: readonly SandboxArtifactItem[];
  collecting?: boolean;
  active?: boolean;
  onDownload?(item: SandboxArtifactItem): void;
  onPreview?(item: SandboxArtifactItem): void;
}) {
  const { copy } = props;
  const [scope, setScope] = useState<'current' | 'all'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [focusMessageId, setFocusMessageId] = useState<string | null>(null);

  useEffect(() => {
    setScope('all');
    setSearchQuery('');
    setFocusMessageId(null);
  }, [props.sessionId]);

  const currentItems = useMemo(
    () => props.items.filter((item) => item.messageId === focusMessageId),
    [props.items, focusMessageId],
  );
  const visibleItems = useMemo(() => {
    const items = scope === 'current' ? currentItems : props.items;
    const query = searchQuery.trim().toLocaleLowerCase();
    return query ? items.filter((item) => item.fileName.toLocaleLowerCase().includes(query)) : items;
  }, [scope, currentItems, props.items, searchQuery]);

  if (props.items.length === 0) {
    return (
      <div className={('chat-artifacts-panel ' + (props.className ?? '')).trim()} style={props.style}>
        {props.collecting ? (
          <div className="artifact-panel-empty">
            <SpriteIcon name="loading" size="20px" className="artifact-panel-banner-spin" />
            <span>{copy.artifactDrawerCollecting}</span>
          </div>
        ) : (
          <div className="artifact-panel-empty">
            <SpriteIcon name="folder-open" size="32px" />
            <span>{copy.sandboxArtifactsEmpty}</span>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className={('chat-artifacts-panel ' + (props.className ?? '')).trim()} style={props.style}>
      <div className="artifact-filters">
        <div className="artifact-scope" role="group" aria-label={copy.sandboxArtifactScope}>
          {focusMessageId ? (
            <button type="button" aria-pressed={scope === 'current'} onClick={() => setScope('current')}>
              {copy.sandboxArtifactsCurrent}
              <span>{currentItems.length}</span>
            </button>
          ) : null}
          <button type="button" aria-pressed={scope === 'all'} onClick={() => setScope('all')}>
            {copy.sandboxArtifactsAll}
            <span>{props.items.length}</span>
          </button>
        </div>
        <div className="artifact-search">
          <input
            value={searchQuery}
            placeholder={copy.sandboxArtifactsSearch}
            aria-label={copy.sandboxArtifactsSearch}
            onChange={(event) => setSearchQuery(event.target.value)}
          />
        </div>
      </div>
      {props.collecting ? (
        <div className="artifact-panel-banner">
          <SpriteIcon name="loading" className="artifact-panel-banner-spin" />
          <span>{copy.artifactDrawerCollecting}</span>
        </div>
      ) : null}
      {visibleItems.length === 0 ? (
        <div className="artifact-panel-empty">
          <SpriteIcon name="search" size="24px" />
          <span>{copy.sandboxArtifactsNoMatches}</span>
        </div>
      ) : (
        <ul className="artifact-list" style={{ display: visibleItems.length ? undefined : 'none' }}>
          {visibleItems.map((item) => (
            <li
              key={`${item.messageId}:${item.index}-${item.fileName}`}
              className="artifact-item is-previewable"
              data-message-id={item.messageId}
              data-artifact-index={item.index}
              onClick={() => props.onPreview?.(item)}
            >
              <button type="button" className="artifact-open" title={copy.artifactDrawerPreview}>
                <ArtifactFileIcon fileName={item.fileName} />
                <span className="artifact-body">
                  <span className="artifact-name" title={item.fileName}>{item.fileName}</span>
                  <span className="artifact-meta">
                    <span>{formatArtifactSize(item.fileSize)}</span>
                    <span className="artifact-meta-sep">·</span>
                    <span>{formatArtifactDateTime(item.createdAt ?? item.modTime)}</span>
                  </span>
                </span>
              </button>
              <button
                type="button"
                className="artifact-download t-button t-button--theme-default t-button--variant-text t-size-s"
                title={copy.artifactDrawerDownload}
                aria-label={copy.artifactDrawerDownload}
                onClick={(event) => { event.stopPropagation(); props.onDownload?.(item); }}
              >
                <span className="t-button__text"><SpriteIcon name="download" size="16px" /></span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function SandboxSidePanel(props: SandboxSidePanelProps) {
  const { copy, visible, activeTab, width, artifacts = [], artifactsCollecting } = props;
  // Vue SandboxSidePanel terminalMounted watch：首切终端 tab 惰性挂载，面板
  // 关闭即销毁（组件卸载天然达成）；切 tab 用 display:none 保留实例。
  const [terminalMounted, setTerminalMounted] = useState(false);
  const [resizing, setResizing] = useState(false);
  const resizingRef = useRef(false);

  useEffect(() => {
    if (!visible) {
      setTerminalMounted(false);
      return;
    }
    if (activeTab === 'terminal') setTerminalMounted(true);
  }, [visible, activeTab]);

  if (!visible) return null;

  const tabs: Array<{ id: SandboxPanelTab; icon: string; label: string }> = [
    { id: 'artifacts', icon: 'folder', label: copy.sandboxTabArtifacts },
    { id: 'terminal', icon: 'terminal', label: copy.sandboxTabTerminal },
    { id: 'desktop', icon: 'desktop', label: copy.sandboxTabDesktop },
  ];

  /** Vue startResize：左缘把手拖拽调宽，期间禁选区/游标，松手交 page 持久化。 */
  function startResize(event: React.MouseEvent) {
    event.preventDefault();
    if (resizingRef.current) return;
    resizingRef.current = true;
    setResizing(true);
    document.body.style.userSelect = 'none';
    document.body.style.cursor = 'col-resize';
    const baseOffset = props.shifted && typeof window !== 'undefined' && window.innerWidth >= 1400 ? 420 : 0;
    const onMove = (moveEvent: MouseEvent) => {
      const next = window.innerWidth - moveEvent.clientX - baseOffset;
      props.onWidthChange(Math.min(SANDBOX_PANEL_MAX_WIDTH, Math.max(SANDBOX_PANEL_MIN_WIDTH, next)));
    };
    const cleanup = () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', cleanup);
      document.body.style.userSelect = '';
      document.body.style.cursor = '';
      resizingRef.current = false;
      setResizing(false);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', cleanup);
  }

  return (
    <aside
      className={'chat-sandbox-panel'
        + (props.shifted ? ' is-shifted' : '')
        + (resizing ? ' is-resizing' : '')}
      style={{ width: `${width}px` }}
      role="complementary"
      aria-label={copy.sandboxPanelTitle}
    >
      <div
        className="chat-sandbox-panel__resize-handle"
        aria-hidden="true"
        onMouseDown={(event) => startResize(event)}
      />
      <div className="chat-sandbox-panel__tabs">
        <div className="chat-sandbox-panel__tablist" role="tablist">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              className={'chat-sandbox-panel__tab' + (activeTab === tab.id ? ' is-active' : '')}
              role="tab"
              aria-selected={activeTab === tab.id}
              onClick={() => props.onTabChange(tab.id)}
            >
              <SpriteIcon name={tab.icon} size="16px" />
              <span>{tab.label}</span>
              {tab.id === 'artifacts' && artifacts.length > 0 ? (
                <span className="chat-sandbox-panel__tab-count" aria-hidden="true">{artifacts.length}</span>
              ) : null}
            </button>
          ))}
        </div>
        <button
          type="button"
          className="chat-sandbox-panel__close"
          aria-label={copy.close}
          onClick={props.onClose}
        >
          <SpriteIcon name="close" size="20px" />
        </button>
      </div>

      <div className={'chat-sandbox-panel__body' + (activeTab === 'artifacts' ? ' is-flush' : '')}>
        <ChatArtifactsPanel
          className="chat-sandbox-panel__artifacts"
          style={{ display: activeTab === 'artifacts' ? undefined : 'none' }}
          copy={copy}
          sessionId={props.sessionId}
          items={artifacts}
          collecting={artifactsCollecting}
          active={activeTab === 'artifacts'}
          onDownload={props.onArtifactDownload}
          onPreview={props.onArtifactPreview}
        />

        {terminalMounted ? (
          <div className="chat-sandbox-panel__terminal" style={{ display: activeTab === 'terminal' ? undefined : 'none' }}>
            {props.terminal}
          </div>
        ) : activeTab === 'terminal' ? (
          <div className="chat-sandbox-panel__placeholder" />
        ) : null}

        {activeTab === 'desktop' ? (
          <div className="chat-sandbox-panel__placeholder">
            <SpriteIcon name="desktop" size="28px" />
            <p>{copy.sandboxDesktopPlaceholder}</p>
          </div>
        ) : null}
      </div>
    </aside>
  );
}
