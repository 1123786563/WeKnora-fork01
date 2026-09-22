// Personal memory settings — ported from Vue frontend/src/views/settings/MemorySettings.vue
// (mounted by Settings.vue under the "mymemory" section). Anatomy, states, copy and
// action semantics follow the Vue source; visual tokens mirror the TDesign light theme
// values Vue resolves at runtime: text rgba(0,0,0,.9/.6/.4), stroke #e7e7e7, brand #07c05f, warning-1 #fef3e6.
import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Status } from '@weknora/ui';
// T12a：可见面直译 MemorySettings.vue 的 t-popup / t-button / t-select /
// t-textarea / t-switch / t-tabs / t-loading / t-popconfirm；记忆行与分页
// 暂保留 React 实现（parity 空态不可见，见 task-12a 报告偏离项）。
import { Icon as TIcon } from 'tdesign-icons-react';
import { Button as TButton, Loading as TLoading, Popconfirm as TPopconfirm, Popup as TPopup, Select as TSelect, Switch as TSwitch, Tabs, Textarea as TTextarea } from 'tdesign-react';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { navigate } from '../platform/navigation.ts';

type MemoryRow = Record<string, unknown>;
type MemoryStatusTab = 'active' | 'pending' | 'tracking' | 'documents' | 'superseded' | 'archived';
type NoticeTone = 'success' | 'info' | 'warning';

const ITEM_STATUSES = ['active', 'pending', 'superseded', 'archived'] as const;
const PAGE_SIZE = 20;
const KINDS = ['profile', 'preference', 'fact', 'task', 'interest'] as const;
const USAGE_ROW_KEYS = ['alwaysOn', 'situational', 'interest', 'tracking', 'documents', 'pending', 'inactive'] as const;

const STATUS_LABEL_KEYS: Record<Exclude<MemoryStatusTab, 'tracking' | 'documents'>, string> = {
  active: 'memorySettings.statusActive',
  pending: 'memorySettings.statusPending',
  superseded: 'memorySettings.statusSuperseded',
  archived: 'memorySettings.statusArchived',
};

const EMPTY_TITLE_KEYS: Record<MemoryStatusTab, string> = {
  active: 'memorySettings.emptyTitle',
  pending: 'memorySettings.pendingEmptyTitle',
  tracking: 'memorySettings.trackingEmptyTitle',
  documents: 'memorySettings.documentsEmptyTitle',
  superseded: 'memorySettings.supersededEmptyTitle',
  archived: 'memorySettings.archivedEmptyTitle',
};

const EMPTY_DESC_KEYS: Record<MemoryStatusTab, string> = {
  active: 'memorySettings.emptyDescription',
  pending: 'memorySettings.pendingEmptyDescription',
  tracking: 'memorySettings.trackingEmptyDescription',
  documents: 'memorySettings.documentsEmptyDescription',
  superseded: 'memorySettings.supersededEmptyDescription',
  archived: 'memorySettings.archivedEmptyDescription',
};

const HINT_KEYS: Partial<Record<MemoryStatusTab, string>> = {
  pending: 'memorySettings.pendingHint',
  tracking: 'memorySettings.trackingHint',
  documents: 'memorySettings.documentsHint',
  superseded: 'memorySettings.supersededHint',
  archived: 'memorySettings.archivedHint',
};

// Vue tabIcons (MemorySettings.vue): active check-circle, pending help-circle,
// tracking chart-bubble, documents file, superseded history, archived folder.
const TAB_ICONS: Record<MemoryStatusTab, IconName> = {
  active: 'check-circle',
  pending: 'help-circle',
  tracking: 'chart-bubble',
  documents: 'file',
  superseded: 'history',
  archived: 'folder',
};

// Vue consolidateSkipMessage map + the model_unavailable branch handled separately.
const CONSOLIDATE_SKIP_KEYS: Record<string, string> = {
  too_few_items: 'memorySettings.consolidateTooFewItems',
  no_candidates: 'memorySettings.consolidateNoCandidates',
  model_declined: 'memorySettings.consolidateModelDeclined',
  too_soon: 'memorySettings.consolidateTooSoon',
};
type IconName =
  | 'info-circle' | 'add' | 'download' | 'swap' | 'delete' | 'check-circle' | 'help-circle'
  | 'chart-bubble' | 'file' | 'history' | 'folder' | 'check' | 'close' | 'edit' | 'star' | 'jump';

const ICON_PATHS: Record<IconName, ReactNode> = {
  'info-circle': <><circle cx='8' cy='8' r='6.25' /><line x1='8' y1='7.4' x2='8' y2='11.2' /><line x1='8' y1='4.9' x2='8' y2='5.1' /></>,
  add: <><line x1='8' y1='3' x2='8' y2='13' /><line x1='3' y1='8' x2='13' y2='8' /></>,
  download: <><polyline points='5 7.5, 8 10.5, 11 7.5' /><line x1='8' y1='3' x2='8' y2='10.5' /><line x1='3.5' y1='12.75' x2='12.5' y2='12.75' /></>,
  swap: <><polyline points='9.5 2.5, 11.5 4.5, 9.5 6.5' /><line x1='11.5' y1='4.5' x2='3' y2='4.5' /><polyline points='6.5 13.5, 4.5 11.5, 6.5 9.5' /><line x1='4.5' y1='11.5' x2='13' y2='11.5' /></>,
  delete: <><line x1='3' y1='4.5' x2='13' y2='4.5' /><polyline points='6.5 4.5, 6.5 3, 9.5 3, 9.5 4.5' /><path d='M4.5 4.5l.7 8.5h5.6l.7-8.5' /><line x1='6.7' y1='6.8' x2='6.7' y2='10.6' /><line x1='9.3' y1='6.8' x2='9.3' y2='10.6' /></>,
  // Tab icons use exact tdesign-icon path data (24px grid) so the rail glyphs
  // match the Vue t-icon raster; Icon renders them with viewBox 24 at 14px.
  'check-circle': <TDFallbackPaths name='check-circle' />,
  'help-circle': <TDFallbackPaths name='help-circle' />,
  'chart-bubble': <TDFallbackPaths name='chart-bubble' />,
  file: <TDFallbackPaths name='file' />,
  history: <TDFallbackPaths name='history' />,
  folder: <TDFallbackPaths name='folder' />,
  check: <polyline points='3.5 8.5, 6.5 11.5, 12.5 4.5' />,
  close: <><line x1='4' y1='4' x2='12' y2='12' /><line x1='12' y1='4' x2='4' y2='12' /></>,
  edit: <path d='M3 13l.8-3.1L10.6 3.1l2.3 2.3L6.1 12.2z' />,
  star: <path d='M8 2.8l1.6 3.3 3.6.5-2.6 2.5.6 3.6L8 11l-3.2 1.7.6-3.6L2.8 6.6l3.6-.5z' />,
  jump: <><path d='M6.5 3.5h6v6' /><line x1='12.5' y1='3.5' x2='5.5' y2='10.5' /></>,
};

// tdesign-icons-vue-next path data (transparent fills + 1px square strokes).
const TD_TAB_PATHS: Record<string, { fills: string[]; strokes: string[] }> = {
  'check-circle': {
    fills: ['M2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12Z'],
    strokes: ['M2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12Z', 'M16.5 9L10.5 15L7.5 12'],
  },
  'help-circle': {
    fills: ['M2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12Z'],
    strokes: ['M2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12Z', 'M12.0002 14.25V14C12.0002 12.8954 12.9877 12.0414 13.8553 11.3578C14.5525 10.8085 15.0002 9.95652 15.0002 9C15.0002 7.34315 13.657 6 12.0002 6C10.694 6 9.58273 6.83481 9.1709 8M12.0002 17.25H12.0041V17.2539H12.0002V17.25Z'],
  },
  'chart-bubble': {
    fills: [],
    strokes: ['M13 14C13 15.6569 11.6569 17 10 17C8.34315 17 7 15.6569 7 14C7 12.3431 8.34315 11 10 11C11.6569 11 13 12.3431 13 14Z', 'M18 6C18 7.10457 17.1046 8 16 8C14.8954 8 14 7.10457 14 6C14 4.89543 14.8954 4 16 4C17.1046 4 18 4.89543 18 6Z', 'M19 15C18.4477 15 18 14.5523 18 14C18 13.4477 18.4477 13 19 13C19.5523 13 20 13.4477 20 14C20 14.5523 19.5523 15 19 15Z', 'M21 21H3V3'],
  },
  file: {
    fills: ['M4 22H20V8H14V2H4V22Z'],
    strokes: ['M14 2V8H20M14 2H15L20 7V8M14 2H4V22H20V8'],
  },
  history: {
    fills: [],
    strokes: ['M2.552 13C3.0517 17.7767 7.09104 21.5 12 21.5C17.2467 21.5 21.5 17.2467 21.5 12C21.5 6.7533 17.2467 2.5 12 2.5C10.3719 2.5 8.8394 2.90957 7.5 3.63131C5.69871 4.60193 4.24661 6.13714 3.38065 8M12 7V12L14.5 14.5M2.5 3.5V8.5H7.5'],
  },
  folder: {
    fills: ['M2 3.5L9 3.5L11 6L22 6L22 20L2 20L2 3.5Z'],
    strokes: ['M2 3.5H9L11 6H22V20H2V3.5Z'],
  },
};

function TDFallbackPaths({ name }: { name: string }) {
  const def = TD_TAB_PATHS[name];
  if (!def) return null;
  return (
    <>
      {def.fills.map((d) => <path key={'f' + d.slice(0, 24)} fill='transparent' d={d} />)}
      {def.strokes.map((d) => <path key={'s' + d.slice(0, 24)} stroke='currentColor' strokeWidth='1.3' strokeLinecap='square' d={d} />)}
    </>
  );
}

function Icon({ name, size = 16 }: { name: IconName; size?: number }) {
  const isTd = name in TD_TAB_PATHS;
  return (
    <svg width={size} height={size} viewBox={isTd ? '0 0 24 24' : '0 0 16 16'} fill='none' stroke='currentColor' strokeWidth={isTd ? 1 : 1.5} strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      {ICON_PATHS[name]}
    </svg>
  );
}

function str(row: MemoryRow, key: string): string {
  return typeof row[key] === 'string' ? row[key] as string : '';
}
function num(row: MemoryRow, key: string): number {
  return typeof row[key] === 'number' ? row[key] as number : 0;
}
function rowId(row: MemoryRow): string {
  return typeof row.id === 'string' || typeof row.id === 'number' ? String(row.id) : '';
}

// Vue handleOpenDocument: router push knowledgeBaseDetail with query.knowledge_id.
export function documentOpenUrl(row: MemoryRow): string | null {
  const knowledgeBaseId = typeof row.knowledge_base_id === 'string' ? row.knowledge_base_id : '';
  if (!knowledgeBaseId) return null;
  const knowledgeId = typeof row.knowledge_id === 'string' ? row.knowledge_id : '';
  return '/knowledgeBase/' + encodeURIComponent(knowledgeBaseId) + (knowledgeId ? '?knowledge_id=' + encodeURIComponent(knowledgeId) : '');
}

// Close an open overlay on outside pointerdown or Escape (Vue popups close on
// outside click; popconfirms and dialogs close on Escape).
function useDismiss(open: boolean, onClose: () => () => void) {
  const ref = useRef<HTMLSpanElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const close = onClose();
    const onPointer = (event: Event) => {
      if (ref.current && event.target instanceof Node && !ref.current.contains(event.target)) close();
    };
    const onKey = (event: Event) => {
      if ((event as KeyboardEvent).key === 'Escape') close();
    };
    document.addEventListener('pointerdown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open, onClose]);
  return ref;
}

function formatTime(value: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const now = new Date();
  const time = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (date.toDateString() === now.toDateString()) return time;
  if (date.getFullYear() === now.getFullYear()) return (date.getMonth() + 1) + '/' + date.getDate() + ' ' + time;
  return date.getFullYear() + '/' + (date.getMonth() + 1) + '/' + date.getDate() + ' ' + time;
}

interface PopconfirmProps {
  open: boolean;
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  danger?: boolean;
  busy?: boolean;
  onToggle: () => void;
  onCancel: () => void;
  onConfirm: () => void;
  children: ReactNode;
  label?: string;
}

function Popconfirm({ open, message, confirmLabel, cancelLabel, danger = true, busy = false, onToggle, onCancel, onConfirm, children, label }: PopconfirmProps) {
  const ref = useDismiss(open, () => onCancel);
  return (
    <span className='relative inline-flex' ref={ref}>
      <span onClick={onToggle}>{children}</span>
      {open ? (
        <div className='absolute right-0 top-[calc(100%_+_6px)] z-40 w-max max-w-[260px] rounded-control border border-line-neutral bg-surface px-3 py-[10px] shadow-[0_3px_14px_2px_rgba(0,0,0,0.05),0_8px_10px_1px_rgba(0,0,0,0.06),0_5px_5px_-3px_rgba(0,0,0,0.1)]' role='alertdialog' aria-label={label ?? confirmLabel}>
          <p className='m-0 mb-2 text-[13px] text-[rgba(0,0,0,0.9)]'>{message}</p>
          <div className='flex justify-end gap-2'>
            <TButton onClick={onCancel}>{cancelLabel}</TButton>
            <TButton theme={danger ? 'danger' : 'default'} disabled={busy} onClick={onConfirm}>{confirmLabel}</TButton>
          </div>
        </div>
      ) : null}
    </span>
  );
}
export function PersonalMemorySettingsPanel({ client, initialSettings }: { client: WeKnoraClient; initialSettings: unknown }) {
  const t = settingsT(readInitialLocale());
  const [settings, setSettings] = useState<MemoryRow | null>(
    initialSettings !== null && typeof initialSettings === 'object' && !Array.isArray(initialSettings) ? initialSettings as MemoryRow : null,
  );
  const [enabled, setEnabled] = useState(settings?.user_enabled === true);
  const [tab, setTab] = useState<MemoryStatusTab>('active');
  const [items, setItems] = useState<MemoryRow[]>([]);
  const [topics, setTopics] = useState<MemoryRow[]>([]);
  const [documents, setDocuments] = useState<MemoryRow[]>([]);
  const [total, setTotal] = useState(0);
  const [counts, setCounts] = useState<Record<string, number>>({ active: 0, pending: 0, superseded: 0, archived: 0 });
  const [trackingCount, setTrackingCount] = useState(0);
  const [documentCount, setDocumentCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [consolidating, setConsolidating] = useState(false);
  const [page, setPage] = useState(1);
  const [addVisible, setAddVisible] = useState(false);
  const [draftKind, setDraftKind] = useState<typeof KINDS[number]>('fact');
  const [draftContent, setDraftContent] = useState('');
  const [editingId, setEditingId] = useState('');
  const [editingContent, setEditingContent] = useState('');
  const [editingImportance, setEditingImportance] = useState(3);
  const [confirmKey, setConfirmKey] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [noticeTone, setNoticeTone] = useState<NoticeTone>('success');

  const clientRef = useRef(client);
  clientRef.current = client;

  // Derived state mirrors the Vue computed properties.
  const workspaceEnabled = settings?.workspace_enabled === true;
  const canWrite = settings?.effective === true;
  const isTracking = tab === 'tracking';
  const isDocuments = tab === 'documents';
  const listTotal = isTracking ? trackingCount : isDocuments ? documentCount : total;
  const visibleRows = isTracking ? topics : isDocuments ? documents : items;
  const listIsEmpty = visibleRows.length === 0;
  const totalAll = ITEM_STATUSES.reduce((sum, value) => sum + (counts[value] ?? 0), 0);
  const emptyTitle = t(EMPTY_TITLE_KEYS[tab]);
  const emptyDescription = t(EMPTY_DESC_KEYS[tab]);
  const totalPages = Math.max(1, Math.ceil(listTotal / PAGE_SIZE));

  const kindLabel = (kind: string) => t('memorySettings.kinds.' + kind);
  const kindHint = (kind: string) => t('memorySettings.kindHints.' + kind);
  const originLabel = (origin: string) => t('memorySettings.origins.' + origin);

  const tabLabel = (value: MemoryStatusTab) => {
    if (value === 'tracking') return t('memorySettings.statusTracking') + '(' + trackingCount + ')';
    if (value === 'documents') return t('memorySettings.statusDocuments') + '(' + documentCount + ')';
    return t(STATUS_LABEL_KEYS[value]) + '(' + (counts[value] ?? 0) + ')';
  };

  const loadSettings = async () => {
    try {
      const loaded = await clientRef.current.settings.memory.personal.settings();
      setSettings(loaded);
      // Vue loadSettings keeps the toggle position from user_enabled.
      setEnabled(loaded.user_enabled === true);
    } catch (cause) {
      console.error('Failed to load memory settings:', cause);
    }
  };

  const fetchPage = async (which: MemoryStatusTab, offset: number) => {
    const current = clientRef.current.settings.memory.personal;
    if (which === 'tracking') {
      const result = await current.topics.list({ limit: PAGE_SIZE, offset });
      setTopics(result.rows); setTrackingCount(result.total);
      return;
    }
    if (which === 'documents') {
      const result = await current.documents.list({ limit: PAGE_SIZE, offset });
      setDocuments(result.rows); setDocumentCount(result.total);
      return;
    }
    const result = await current.items.list({ status: which, limit: PAGE_SIZE, offset });
    setItems(result.rows); setTotal(result.total);
  };

  // Per-status counts, so every tab carries its own size (Vue loadCounts).
  const loadCounts = async () => {
    const current = clientRef.current.settings.memory.personal;
    const statusTotals = await Promise.all(ITEM_STATUSES.map(async (value) => {
      try {
        return (await current.items.list({ status: value, limit: 1 })).total;
      } catch {
        return 0;
      }
    }));
    const topicTotal = await current.topics.list({ limit: 1 }).then((result) => result.total).catch(() => 0);
    const docTotal = await current.documents.list({ limit: 1 }).then((result) => result.total).catch(() => 0);
    const nextCounts: Record<string, number> = {};
    ITEM_STATUSES.forEach((value, index) => { nextCounts[value] = statusTotals[index] ?? 0; });
    setCounts(nextCounts);
    setTrackingCount(topicTotal);
    setDocumentCount(docTotal);
  };

  const runList = async () => {
    setLoading(true);
    try {
      await fetchPage(tab, (page - 1) * PAGE_SIZE);
    } catch (cause) {
      console.error('Failed to load memories:', cause);
      setItems([]); setTopics([]); setDocuments([]); setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const reload = async () => {
    setPage(1);
    await Promise.all([runList(), loadCounts().catch(() => undefined)]);
  };

  useEffect(() => {
    void (async () => {
      await loadSettings();
      await reload();
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const showNotice = (message: string, tone: NoticeTone = 'success') => {
    setNotice(message); setNoticeTone(tone);
  };

  const handleEnabledChange = async (value: boolean) => {
    setBusy(true); setError(null); setNotice(null);
    const previous = enabled;
    setEnabled(value);
    try {
      const updated = await clientRef.current.settings.memory.personal.updateEnabled(value);
      setSettings(updated);
      setEnabled(updated.user_enabled === true);
      showNotice(t(value ? 'memorySettings.toasts.enabled' : 'memorySettings.toasts.disabled'));
    } catch (cause) {
      setEnabled(previous);
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    } finally {
      setBusy(false);
    }
  };

  const handleTabChange = async (value: MemoryStatusTab) => {
    if (value === tab) return;
    setTab(value);
    setPage(1);
    setConfirmKey(null);
    setLoading(true);
    try {
      await fetchPage(value, 0);
    } catch (cause) {
      console.error('Failed to load memories:', cause);
      setItems([]); setTopics([]); setDocuments([]); setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const handlePageChange = async (next: number) => {
    if (next < 1 || next > totalPages || next === page) return;
    setPage(next);
    setLoading(true);
    try {
      await fetchPage(tab, (next - 1) * PAGE_SIZE);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    const content = draftContent.trim();
    if (!content || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.items.create({ kind: draftKind, content });
      setDraftContent('');
      setAddVisible(false);
      setTab('active');
      await reload();
      await loadSettings();
      showNotice(t('memorySettings.toasts.added'));
    } catch (cause) {
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (row: MemoryRow) => {
    setEditingId(rowId(row));
    setEditingContent(str(row, 'content'));
    setEditingImportance(typeof row.importance === 'number' ? row.importance as number : 3);
  };

  const handleSaveEdit = async (row: MemoryRow) => {
    const content = editingContent.trim();
    if (!content) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.items.update(rowId(row), { content, importance: editingImportance });
      setEditingId(''); setEditingContent('');
      await Promise.all([runList(), loadCounts().catch(() => undefined)]);
      showNotice(t('memorySettings.toasts.updated'));
    } catch (cause) {
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    } finally {
      setBusy(false);
    }
  };

  const handleConfirmGuess = async (row: MemoryRow) => {
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.items.confirm(rowId(row));
      await reload();
      showNotice(t('memorySettings.confirmSuccess'));
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.confirmFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleRejectGuess = async (row: MemoryRow) => {
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.items.reject(rowId(row));
      await reload();
      showNotice(t('memorySettings.rejectSuccess'));
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.rejectFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async (row: MemoryRow) => {
    setConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.items.remove(rowId(row));
      await Promise.all([runList(), loadCounts().catch(() => undefined), loadSettings()]);
      showNotice(t('memorySettings.toasts.deleted'));
    } catch (cause) {
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    } finally {
      setBusy(false);
    }
  };

  const handleClear = async () => {
    setConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      const removed = await clientRef.current.settings.memory.personal.clear();
      await Promise.all([reload(), loadSettings()]);
      showNotice(t('memorySettings.toasts.cleared', { count: typeof removed.removed === 'number' ? removed.removed as number : 0 }));
    } catch (cause) {
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    } finally {
      setBusy(false);
    }
  };

  // A review that merges nothing is the normal outcome, so say which kind of
  // "nothing" it was (Vue handleConsolidate).
  const handleConsolidate = async () => {
    if (consolidating) return;
    setConfirmKey(null);
    setConsolidating(true); setError(null); setNotice(null);
    try {
      const result = await clientRef.current.settings.memory.personal.consolidate();
      const merged = num(result, 'merged');
      const demoted = num(result, 'demoted');
      const expired = num(result, 'expired');
      const skipped = str(result, 'skipped');
      if (merged || demoted || expired) {
        showNotice(t('memorySettings.consolidateSuccess', { merged, demoted, expired }));
      } else if (skipped === 'model_unavailable') {
        showNotice(t('memorySettings.consolidateModelUnavailable'), 'warning');
      } else {
        showNotice(t(CONSOLIDATE_SKIP_KEYS[skipped] ?? 'memorySettings.consolidateNothing'), 'info');
      }
      await Promise.all([runList(), loadCounts().catch(() => undefined)]);
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.consolidateFailed'));
    } finally {
      setConsolidating(false);
    }
  };

  const handleExport = async () => {
    setError(null); setNotice(null);
    try {
      const rows = await clientRef.current.settings.memory.personal.export();
      const blob = new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'weknora-memories.json';
      link.click();
      URL.revokeObjectURL(url);
    } catch (cause) {
      setError(t('memorySettings.toasts.saveFailed', { message: cause instanceof Error ? cause.message : '' }));
    }
  };

  const handlePromoteTopic = async (row: MemoryRow) => {
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.topics.promote(rowId(row));
      showNotice(t('memorySettings.promoteSuccess'));
      setTab('active');
      await reload();
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.promoteFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleDismissTopic = async (row: MemoryRow) => {
    setConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.topics.remove(rowId(row));
      await Promise.all([runList(), loadCounts().catch(() => undefined)]);
      showNotice(t('memorySettings.dismissSuccess'));
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.dismissFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleOpenDocument = (row: MemoryRow) => {
    const url = documentOpenUrl(row);
    if (url === null) {
      showNotice(t('memorySettings.openDocumentUnavailable'), 'warning');
      return;
    }
    navigate(url);
  };

  const handleStopTrackingDocument = async (row: MemoryRow) => {
    setConfirmKey(null);
    setBusy(true); setError(null); setNotice(null);
    try {
      await clientRef.current.settings.memory.personal.documents.remove(rowId(row));
      await Promise.all([runList(), loadCounts().catch(() => undefined)]);
      showNotice(t('memorySettings.stopTrackingDocumentSuccess'));
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('memorySettings.stopTrackingDocumentFailed'));
    } finally {
      setBusy(false);
    }
  };

  const allTabs: MemoryStatusTab[] = ['active', 'pending', 'tracking', 'documents', 'superseded', 'archived'];

  // Vue statusHint: tracking/documents hints need rows; status hints need items.
  const statusHint = (() => {
    if (isTracking) return topics.length === 0 ? '' : t('memorySettings.trackingHint');
    if (isDocuments) return documents.length === 0 ? '' : t('memorySettings.documentsHint');
    if (items.length === 0) return '';
    const key = HINT_KEYS[tab];
    return key ? t(key) : '';
  })();

  const itemRows: ReactNode = (() => {
    if (items.length === 0) return null;
    return items.map((row, index) => {
      const id = rowId(row);
      const status = str(row, 'status') || 'active';
      const kind = str(row, 'kind');
      const topic = str(row, 'topic');
      const isRetired = status === 'superseded' || status === 'archived';
      return (
        <li key={id || index} className='flex items-start justify-between gap-4 border-b border-line-neutral py-4 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2'>
          <div className='min-w-0 flex-1'>
            {editingId === id ? (
              <div className='mb-2 flex flex-col gap-2'>
                <TTextarea
                  className='inline-edit-textarea'
                  value={editingContent}
                  autosize={{ minRows: 2, maxRows: 6 }}
                  aria-label={t('common.edit')}
                  onChange={(value) => setEditingContent(String(value ?? ''))}
                  onKeydown={(_value, { e }) => { if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') void handleSaveEdit(row); }}
                />
                <div className='memory-edit-actions'>
                  <TButton size='small' variant='outline' onClick={() => { setEditingId(''); setEditingContent(''); }}>{t('common.cancel')}</TButton>
                  <TButton size='small' theme='primary' loading={busy} onClick={() => void handleSaveEdit(row)}>{t('common.save')}</TButton>
                </div>
              </div>
            ) : (
              <p className={'mb-1 mt-0 break-words text-[14px] leading-[1.6] ' + (isRetired ? 'text-[rgba(0,0,0,0.4)] line-through' : 'text-[rgba(0,0,0,0.9)]')}>{str(row, 'content')}</p>
            )}
            <div className="flex flex-wrap items-center text-[12px] leading-[18px] text-[rgba(0,0,0,0.4)] [&>span:not(:last-child)]:after:mx-1.5 [&>span:not(:last-child)]:after:content-['·'] [&>span:not(:last-child)]:after:text-[rgba(0,0,0,0.4)]">
              <span title={kind ? kindHint(kind) : undefined}>{kind ? kindLabel(kind) : ''}</span>
              {topic && topic !== str(row, 'content') ? <span className='max-w-[160px] overflow-hidden text-ellipsis whitespace-nowrap' title={topic}>{topic}</span> : null}
              <span>{originLabel(str(row, 'origin'))}</span>
              <span>{formatTime(str(row, 'valid_from'))}</span>
            </div>
          </div>
          <div className='-mt-0.5 flex shrink-0 items-center gap-1 max-[720px]:mt-0'>
            {status === 'pending' ? (
              <>
                <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' disabled={!canWrite} onClick={() => void handleConfirmGuess(row)}>
                  <Icon name='check' />
                  {t('memorySettings.confirmGuess')}
                </button>
                <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' onClick={() => void handleRejectGuess(row)}>
                  <Icon name='close' />
                  {t('memorySettings.rejectGuess')}
                </button>
              </>
            ) : null}
            {status === 'active' ? (
              <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' aria-label={t('common.edit')} title={t('common.edit')} disabled={!canWrite} onClick={() => startEdit(row)}>
                <Icon name='edit' />
              </button>
            ) : null}
            {status !== 'pending' ? (
              <Popconfirm
                open={confirmKey === 'item:' + id}
                message={t('memorySettings.deleteConfirm')}
                confirmLabel={t('common.delete')}
                cancelLabel={t('common.cancel')}
                label={t('memorySettings.deleteConfirm')}
                busy={busy}
                onToggle={() => setConfirmKey(confirmKey === 'item:' + id ? null : 'item:' + id)}
                onCancel={() => setConfirmKey(null)}
                onConfirm={() => void handleDelete(row)}
              >
                <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[#e34d59] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' aria-label={t('common.delete')} title={t('common.delete')}>
                  <Icon name='delete' />
                </button>
              </Popconfirm>
            ) : null}
          </div>
        </li>
      );
    });
  })();

  const trackingRows: ReactNode = (() => {
    if (topics.length === 0) return null;
    return topics.map((row, index) => {
      const id = rowId(row);
      const threshold = Math.max(num(row, 'threshold'), 1);
      const hits = num(row, 'hits');
      const percentage = Math.min(100, Math.round((hits / threshold) * 100));
      const aliases = Array.isArray(row.aliases) ? (row.aliases as unknown[]).filter((value): value is string => typeof value === 'string') : [];
      return (
        <li key={id || index} className='flex items-start justify-between gap-4 border-b border-line-neutral py-4 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2'>
          <div className='min-w-0 flex-1'>
            <p className='mb-1 mt-0 break-words text-[14px] leading-[1.6] text-[rgba(0,0,0,0.9)]'>{str(row, 'topic')}</p>
            <div className='mb-1 mt-1.5 flex max-w-[360px] items-center gap-2.5'>
              <div className='h-1 min-w-20 flex-1 overflow-hidden rounded-full bg-line-neutral' role='progressbar' aria-valuenow={percentage} aria-valuemin={0} aria-valuemax={100}>
                <div className='h-full rounded-full bg-accent' style={{ width: percentage + '%' }} />
              </div>
              <span className='shrink-0 text-[12px] leading-[18px] text-[rgba(0,0,0,0.4)]'>{hits >= threshold ? t('memorySettings.trackingReady') : t('memorySettings.trackingProgress', { hits, threshold })}</span>
            </div>
            <div className="flex flex-wrap items-center text-[12px] leading-[18px] text-[rgba(0,0,0,0.4)] [&>span:not(:last-child)]:after:mx-1.5 [&>span:not(:last-child)]:after:content-['·'] [&>span:not(:last-child)]:after:text-[rgba(0,0,0,0.4)]">
              <span>{t('memorySettings.kinds.interest')}</span>
              {aliases.length > 0 ? (
                <span className='max-w-[160px] overflow-hidden text-ellipsis whitespace-nowrap' title={aliases.join(', ')}>
                  {t('memorySettings.trackingAliases', { aliases: aliases.join(', ') })}
                </span>
              ) : null}
              <span>{formatTime(str(row, 'last_seen_at'))}</span>
            </div>
          </div>
          <div className='-mt-0.5 flex shrink-0 items-center gap-1 max-[720px]:mt-0'>
            <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' disabled={!canWrite} onClick={() => void handlePromoteTopic(row)}>
              <Icon name='star' />
              {t('memorySettings.promoteTopic')}
            </button>
            <Popconfirm
              open={confirmKey === 'topic:' + id}
              message={t('memorySettings.dismissTopicConfirm')}
              confirmLabel={t('memorySettings.dismissTopic')}
              cancelLabel={t('common.cancel')}
              label={t('memorySettings.dismissTopicConfirm')}
              busy={busy}
              onToggle={() => setConfirmKey(confirmKey === 'topic:' + id ? null : 'topic:' + id)}
              onCancel={() => setConfirmKey(null)}
              onConfirm={() => void handleDismissTopic(row)}
            >
              <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' title={t('memorySettings.dismissTopic')}>
                {t('memorySettings.dismissTopic')}
              </button>
            </Popconfirm>
          </div>
        </li>
      );
    });
  })();

  const documentRows: ReactNode = (() => {
    if (documents.length === 0) return null;
    return documents.map((row, index) => {
      const id = rowId(row);
      const knowledgeBaseId = str(row, 'knowledge_base_id');
      return (
        <li key={id || index} className='flex items-start justify-between gap-4 border-b border-line-neutral py-4 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2'>
          <div className='min-w-0 flex-1'>
            <p className='mb-1 mt-0 break-words text-[14px] leading-[1.6] text-[rgba(0,0,0,0.9)]'>{str(row, 'title') || t('memorySettings.untitledDocument')}</p>
            <div className="flex flex-wrap items-center text-[12px] leading-[18px] text-[rgba(0,0,0,0.4)] [&>span:not(:last-child)]:after:mx-1.5 [&>span:not(:last-child)]:after:content-['·'] [&>span:not(:last-child)]:after:text-[rgba(0,0,0,0.4)]">
              <span>{t('memorySettings.documentsHits', { hits: num(row, 'hits') })}</span>
              <span>{formatTime(str(row, 'last_used_at'))}</span>
            </div>
          </div>
          <div className='-mt-0.5 flex shrink-0 items-center gap-1 max-[720px]:mt-0'>
            <button
              type='button'
              className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40'
              disabled={!knowledgeBaseId}
              title={knowledgeBaseId ? t('memorySettings.openDocument') : t('memorySettings.openDocumentUnavailable')}
              onClick={() => handleOpenDocument(row)}
            >
              <Icon name='jump' />
              {t('memorySettings.openDocument')}
            </button>
            <Popconfirm
              open={confirmKey === 'doc:' + id}
              message={t('memorySettings.stopTrackingDocumentConfirm')}
              confirmLabel={t('memorySettings.stopTrackingDocument')}
              cancelLabel={t('common.cancel')}
              label={t('memorySettings.stopTrackingDocumentConfirm')}
              busy={busy}
              onToggle={() => setConfirmKey(confirmKey === 'doc:' + id ? null : 'doc:' + id)}
              onCancel={() => setConfirmKey(null)}
              onConfirm={() => void handleStopTrackingDocument(row)}
            >
              <button type='button' className='inline-flex min-h-6 w-6 min-w-6 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] disabled:cursor-not-allowed disabled:opacity-40' title={t('memorySettings.stopTrackingDocument')}>
                {t('memorySettings.stopTrackingDocument')}
              </button>
            </Popconfirm>
          </div>
        </li>
      );
    });
  })();

  const listBody: ReactNode = (() => {
    if (listIsEmpty) {
      return (
        <div className='empty'>
          <p className='empty-title'>{emptyTitle}</p>
          <p className='empty-desc'>{emptyDescription}</p>
        </div>
      );
    }
    if (isDocuments) return <ul className='memory-list'>{documentRows}</ul>;
    if (isTracking) return <ul className='memory-list'>{trackingRows}</ul>;
    return <ul className='memory-list'>{itemRows}</ul>;
  })();
  // Vue renders the section bare on the drawer background (no outer card).
  // T12a：MemorySettings.vue 逐节点复刻（section-header-titlewrap + usage
  // popup + notice + settings-group 开关行 + list-section 工具条 + status
  // tabs + t-loading 列表区）。样式走 settings.td.css §6。
  return (
    <div className='memory-settings'>
      <div className='section-header'>
        <div className='section-header-titlewrap'>
          <h2>{t('memorySettings.title')}</h2>
          <TPopup
            // Vue placement="bottom-start"；react 1.18.3 PopupPlacement 无
            // -start 粒度（库间差异，hover 弹层几何，稳态扫描不可见）。
            placement='bottom-left'
            trigger='hover'
            overlayClassName='memory-usage-popup-overlay'
            content={(
              <div className='usage-popup'>
                <div className='usage-popup-title'>{t('memorySettings.usage.title')}</div>
                <p className='usage-popup-intro'>{t('memorySettings.usage.intro')}</p>
                <div className='usage-popup-rows'>
                  {USAGE_ROW_KEYS.map((key) => (
                    <div key={key} className='usage-popup-row'>
                      <span className='usage-popup-label'>{t('memorySettings.usage.rows.' + key + '.label')}</span>
                      <span className='usage-popup-text'>{t('memorySettings.usage.rows.' + key + '.text')}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          >
            <button
              type='button'
              className='usage-trigger-btn'
              aria-label={t('memorySettings.usage.iconHint')}
              title={t('memorySettings.usage.iconHint')}
            >
              <TIcon name='info-circle' size='16px' />
            </button>
          </TPopup>
        </div>
        <p className='section-description'>{t('memorySettings.description')}</p>
      </div>

      {settings !== null && !workspaceEnabled ? (
        <div className='notice' role='status'>
          <TIcon name='info-circle' />
          <span>{t('memorySettings.workspaceDisabled')}</span>
        </div>
      ) : null}

      <div className='settings-group'>
        <div className='setting-row'>
          <div className='setting-info'>
            <label>{t('memorySettings.enableLabel')}</label>
            <p className='desc'>{t('memorySettings.enableDescription')}</p>
            {enabled && workspaceEnabled ? <p className='desc'>{t('memorySettings.agentDisabledHint')}</p> : null}
          </div>
          <div className='setting-control'>
            <TSwitch
              value={enabled}
              disabled={settings === null || !workspaceEnabled || busy}
              onChange={(checked) => void handleEnabledChange(Boolean(checked))}
            />
          </div>
        </div>
      </div>

      {error ? <Status tone='error'>{error}</Status> : null}
      {notice ? <Status tone={noticeTone === 'info' ? 'neutral' : noticeTone}>{notice}</Status> : null}

      <div className='list-section'>
        <div className='list-toolbar'>
          <div className='list-title'>
            <h3>{t('memorySettings.listTitle')}</h3>
            <span className='list-count'>{t('memorySettings.listCount', { count: totalAll })}</span>
          </div>
          <div className='list-actions'>
            <TPopup
              trigger='click'
              // Vue bottom-end → react 1.18.3 无 -end 粒度（同上库间差异）。
              placement='bottom-right'
              destroyOnClose
              visible={addVisible}
              onVisibleChange={(visible) => { if (visible) setDraftContent(''); setAddVisible(visible); }}
              overlayClassName='memory-add-popup-overlay'
              content={(
                <div className='add-popup' onClick={(event) => event.stopPropagation()}>
                  <div className='add-popup-title'>{t('memorySettings.addTitle')}</div>
                  <label className='add-field'>
                    <span className='add-label'>{t('memorySettings.addKindLabel')}</span>
                    <TSelect
                      size='small'
                      value={draftKind}
                      onChange={(value) => setDraftKind(String(value) as typeof KINDS[number])}
                      popupProps={{ overlayClassName: 'memory-add-kind-popup' }}
                    >
                      {KINDS.map((kind) => <TSelect.Option key={kind} value={kind} label={kindLabel(kind)} />)}
                    </TSelect>
                    <span className='add-kind-hint'>{kindHint(draftKind)}</span>
                  </label>
                  <label className='add-field'>
                    <span className='add-label'>{t('memorySettings.addContentLabel')}</span>
                    <TTextarea
                      placeholder={t('memorySettings.addPlaceholder')}
                      maxlength={300}
                      autosize={{ minRows: 3, maxRows: 6 }}
                      value={draftContent}
                      onChange={(value) => setDraftContent(String(value ?? ''))}
                      count={({ count, maxLength }: { count: number; maxLength?: number }) => <span className='t-textarea__limit'>{`${count}/${maxLength}`}</span>}
                    />
                  </label>
                  <div className='add-popup-footer'>
                    <TButton size='small' variant='outline' onClick={() => setAddVisible(false)}>
                      {t('common.cancel')}
                    </TButton>
                    <TButton size='small' theme='primary' disabled={!draftContent.trim()} onClick={() => void handleCreate()}>
                      {t('memorySettings.add')}
                    </TButton>
                  </div>
                </div>
              )}
            >
              <TButton size='small' variant='text' disabled={!canWrite} icon={<TIcon name='add' />}>
                {t('memorySettings.add')}
              </TButton>
            </TPopup>
            <TButton size='small' variant='text' icon={<TIcon name='download' />} onClick={() => void handleExport()}>
              {t('memorySettings.export')}
            </TButton>
            <TPopconfirm
              content={t('memorySettings.consolidateConfirm')}
              confirmBtn={{ content: t('memorySettings.consolidate') }}
              cancelBtn={{ content: t('common.cancel') }}
              placement='bottom'
              onConfirm={() => void handleConsolidate()}
            >
              <TButton size='small' variant='text' loading={consolidating} disabled={!canWrite || totalAll === 0} icon={<TIcon name='swap' />}>
                {t('memorySettings.consolidate')}
              </TButton>
            </TPopconfirm>
            <TPopconfirm
              theme='danger'
              content={t('memorySettings.clearConfirm')}
              confirmBtn={{ content: t('memorySettings.clear'), theme: 'danger' }}
              cancelBtn={{ content: t('common.cancel') }}
              placement='left'
              onConfirm={() => void handleClear()}
            >
              <TButton size='small' theme='danger' variant='text' disabled={totalAll === 0 && trackingCount === 0 && documentCount === 0} icon={<TIcon name='delete' />}>
                {t('memorySettings.clear')}
              </TButton>
            </TPopconfirm>
          </div>
        </div>

        <Tabs value={tab} className='status-tabs' onChange={(value) => void handleTabChange(value as MemoryStatusTab)}>
          {allTabs.map((value) => (
            <Tabs.TabPanel
              key={value}
              value={value}
              label={<span className='status-tab-label'><TIcon name={TAB_ICONS[value]} size='14px' /><span>{tabLabel(value)}</span></span>}
            />
          ))}
        </Tabs>

        <TLoading loading={loading}>
          {listBody}
        </TLoading>
        {statusHint ? <p className='status-hint'>{statusHint}</p> : null}

        {listTotal > PAGE_SIZE ? (
          <div className='mt-4 flex items-center gap-1' role='navigation' aria-label={t('memorySettings.listTitle')}>
            <button type='button' className='h-6 min-w-6 cursor-pointer rounded-[3px] border border-transparent bg-transparent px-1.5 py-0 text-[13px] text-[rgba(0,0,0,0.6)] disabled:cursor-not-allowed disabled:opacity-40' disabled={page <= 1} onClick={() => void handlePageChange(page - 1)} aria-label='previous page'>{'<'}</button>
            {Array.from({ length: totalPages }, (_, index) => index + 1).map((pageNumber) => (
              <button key={pageNumber} type='button' className={'h-6 min-w-6 cursor-pointer rounded-[3px] border bg-transparent px-1.5 py-0 text-[13px] disabled:cursor-not-allowed disabled:opacity-40 ' + (pageNumber === page ? 'border-accent bg-accent text-surface' : 'border-transparent text-[rgba(0,0,0,0.6)] enabled:hover:border-accent enabled:hover:text-accent')} aria-current={pageNumber === page ? 'page' : undefined} onClick={() => void handlePageChange(pageNumber)}>
                {pageNumber}
              </button>
            ))}
            <button type='button' className='h-6 min-w-6 cursor-pointer rounded-[3px] border border-transparent bg-transparent px-1.5 py-0 text-[13px] text-[rgba(0,0,0,0.6)] disabled:cursor-not-allowed disabled:opacity-40' disabled={page >= totalPages} onClick={() => void handlePageChange(page + 1)} aria-label='next page'>{'>'}</button>
          </div>
        ) : null}
      </div>
    </div>
  );
}