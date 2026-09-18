// Personal memory settings — ported from Vue frontend/src/views/settings/MemorySettings.vue
// (mounted by Settings.vue under the "mymemory" section). Anatomy, states, copy and
// action semantics follow the Vue source; visual tokens mirror the TDesign light theme
// values Vue resolves at runtime: text rgba(0,0,0,.9/.6/.4), stroke #e7e7e7, brand #07c05f, warning-1 #fef3e6.
import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Select, Status, Switch, Textarea } from '@weknora/ui';
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
  'check-circle': <><circle cx='8' cy='8' r='6.25' /><polyline points='5.3 8.2, 7.1 10, 10.7 6.2' /></>,
  'help-circle': <><circle cx='8' cy='8' r='6.25' /><path d='M6.3 6.4a1.75 1.75 0 1 1 2.55 1.95c-.6.35-.85.7-.85 1.35' /><line x1='8' y1='11.1' x2='8' y2='11.3' /></>,
  'chart-bubble': <><circle cx='5.5' cy='10' r='2.75' /><circle cx='10.5' cy='5.5' r='3' /></>,
  file: <><path d='M4 2.5h4.75L12 5.75V13.5H4z' /><polyline points='8.75 2.5, 8.75 5.75, 12 5.75' /></>,
  history: <><circle cx='8' cy='8' r='6.25' /><polyline points='8 4.6, 8 8, 10.4 9.4' /></>,
  folder: <path d='M2.5 4h4.25L8.25 5.6H13.5V12.5H2.5z' />,
  check: <polyline points='3.5 8.5, 6.5 11.5, 12.5 4.5' />,
  close: <><line x1='4' y1='4' x2='12' y2='12' /><line x1='12' y1='4' x2='4' y2='12' /></>,
  edit: <path d='M3 13l.8-3.1L10.6 3.1l2.3 2.3L6.1 12.2z' />,
  star: <path d='M8 2.8l1.6 3.3 3.6.5-2.6 2.5.6 3.6L8 11l-3.2 1.7.6-3.6L2.8 6.6l3.6-.5z' />,
  jump: <><path d='M6.5 3.5h6v6' /><line x1='12.5' y1='3.5' x2='5.5' y2='10.5' /></>,
};

function Icon({ name, size = 16 }: { name: IconName; size?: number }) {
  return (
    <svg width={size} height={size} viewBox='0 0 16 16' fill='none' stroke='currentColor' strokeWidth='1.5' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
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
            <Button type='button' onClick={onCancel}>{cancelLabel}</Button>
            <Button type='button' className={danger ? 'border-[#e34d59] bg-[#e34d59] text-surface enabled:hover:border-[#f36d78] enabled:hover:bg-[#f36d78]' : undefined} disabled={busy} onClick={onConfirm}>{confirmLabel}</Button>
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
  const [usageOpen, setUsageOpen] = useState(false);
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

  const addAnchorRef = useDismiss(addVisible, () => () => setAddVisible(false));
  const clearAnchorRef = useDismiss(confirmKey === 'clear' || confirmKey === 'consolidate', () => () => setConfirmKey(null));
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
                <Textarea
                  className='w-full min-h-14 resize-y rounded-[3px] border border-line-input px-2 py-1.5 text-[14px] leading-[1.6] focus:border-accent focus:outline-2 focus:outline-offset-0 focus:outline-accent/20'
                  value={editingContent}
                  rows={2}
                  aria-label={t('common.edit')}
                  onChange={(event) => setEditingContent(event.target.value)}
                  onKeyDown={(event) => { if (event.ctrlKey && event.key === 'Enter') void handleSaveEdit(row); }}
                />
                <div className='flex justify-end gap-2'>
                  <Button type='button' onClick={() => { setEditingId(''); setEditingContent(''); }}>{t('common.cancel')}</Button>
                  <Button type='button' loading={busy} onClick={() => void handleSaveEdit(row)}>{t('common.save')}</Button>
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
        <div className='py-8 text-center'>
          <p className='mb-1 mt-0 text-[14px] font-medium text-[rgba(0,0,0,0.6)]'>{emptyTitle}</p>
          <p className='m-0 text-[13px] text-[rgba(0,0,0,0.4)]'>{emptyDescription}</p>
        </div>
      );
    }
    if (isDocuments) return <ul className='m-0 list-none p-0'>{documentRows}</ul>;
    if (isTracking) return <ul className='m-0 list-none p-0'>{trackingRows}</ul>;
    return <ul className='m-0 list-none p-0'>{itemRows}</ul>;
  })();
  // Vue renders the section bare on the drawer background (no outer card).
  return (
    <div className='w-full text-[rgba(0,0,0,0.9)]'>
      <div className='mb-6'>
        <div className='inline-flex items-center gap-2'>
          <h2 className='m-0 text-[20px] font-semibold text-[rgba(0,0,0,0.9)]'>{t('memorySettings.title')}</h2>
          <span className='relative inline-flex'>
            <button
              type='button'
              className='m-0 inline-flex h-[22px] w-[22px] shrink-0 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] leading-none transition-[background-color,color] duration-200 ease-[ease] enabled:hover:bg-[#f3f3f3] enabled:hover:text-accent focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent/20'
              aria-label={t('memorySettings.usage.iconHint')}
              title={t('memorySettings.usage.iconHint')}
              aria-expanded={usageOpen}
              onMouseEnter={() => setUsageOpen(true)}
              onMouseLeave={() => setUsageOpen(false)}
              onFocus={() => setUsageOpen(true)}
              onBlur={() => setUsageOpen(false)}
              onClick={() => setUsageOpen(!usageOpen)}
            >
              <Icon name='info-circle' />
            </button>
            {usageOpen ? (
              <div className='absolute left-0 top-[calc(100%_+_6px)] z-30 w-[380px] max-w-[calc(100vw_-_24px)] rounded-[12px] border-[0.5px] border-line-neutral bg-surface px-4 pb-3 pt-[14px] text-left shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)]' role='dialog' aria-label={t('memorySettings.usage.title')}>
                <div className='text-[13px] font-semibold text-[rgba(0,0,0,0.9)]'>{t('memorySettings.usage.title')}</div>
                <p className='mb-3 mt-1 text-[12px] leading-normal text-[rgba(0,0,0,0.4)]'>{t('memorySettings.usage.intro')}</p>
                <div className='flex flex-col gap-2.5'>
                  {USAGE_ROW_KEYS.map((key) => (
                    <div key={key} className='flex items-start gap-3 leading-normal'>
                      <span className='w-[88px] flex-none text-[12px] font-medium text-[rgba(0,0,0,0.9)]'>{t('memorySettings.usage.rows.' + key + '.label')}</span>
                      <span className='min-w-0 flex-1 text-[12px] text-[rgba(0,0,0,0.6)]'>{t('memorySettings.usage.rows.' + key + '.text')}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </span>
        </div>
        <p className='mb-0 mt-2 text-[14px] leading-normal text-[rgba(0,0,0,0.6)]'>{t('memorySettings.description')}</p>
      </div>

      {settings !== null && !workspaceEnabled ? (
        <div className='mb-4 flex items-center gap-2 rounded-card bg-[#fef3e6] px-4 py-3 text-[13px] text-[rgba(0,0,0,0.9)]' role='status'>
          <Icon name='info-circle' />
          <span>{t('memorySettings.workspaceDisabled')}</span>
        </div>
      ) : null}

      <div className='flex items-start justify-between border-b border-line-neutral py-5 max-[720px]:flex-col max-[720px]:gap-2'>
        <div className='max-w-[65%] flex-1 pr-6 max-[720px]:max-w-full max-[720px]:pr-0'>
          <label className='mb-1 block text-[15px] font-medium text-[rgba(0,0,0,0.9)]'>{t('memorySettings.enableLabel')}</label>
          <p className='m-0 text-[13px] leading-normal text-[rgba(0,0,0,0.6)]'>{t('memorySettings.enableDescription')}</p>
          {enabled && workspaceEnabled ? <p className='m-0 text-[13px] leading-normal text-[rgba(0,0,0,0.6)]'>{t('memorySettings.agentDisabledHint')}</p> : null}
        </div>
        <div className='-mt-0.5 flex shrink-0 items-center gap-1 max-[720px]:mt-0'>
          <Switch
            checked={enabled}
            disabled={settings === null || !workspaceEnabled || busy}
            onCheckedChange={(checked) => void handleEnabledChange(checked)}
            aria-label={t('memorySettings.enableLabel')}
          />
        </div>
      </div>

      {error ? <Status tone='error'>{error}</Status> : null}
      {notice ? <Status tone={noticeTone === 'info' ? 'neutral' : noticeTone}>{notice}</Status> : null}

      <div className='mt-7'>
        <div className='mb-2 flex flex-wrap items-center justify-between gap-3'>
          <div className='flex items-baseline gap-2'>
            <h3 className='m-0 text-[16px] font-semibold text-[rgba(0,0,0,0.9)]'>{t('memorySettings.listTitle')}</h3>
            <span className='text-[13px] text-[rgba(0,0,0,0.4)]'>{t('memorySettings.listCount', { count: totalAll })}</span>
          </div>
          <div className='flex flex-wrap items-center gap-1'>
            <span className='relative inline-flex' ref={addAnchorRef}>
              <Button type='button' className='min-h-6 gap-1 rounded-[3px] border-0 bg-transparent px-[7px] py-0 text-[13px] text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] enabled:hover:text-[rgba(0,0,0,0.9)] disabled:cursor-not-allowed disabled:opacity-40' disabled={!canWrite} onClick={() => { if (!addVisible) setDraftContent(''); setAddVisible(!addVisible); }}>
                <Icon name='add' />
                {t('memorySettings.add')}
              </Button>
              {addVisible ? (
                <div className='absolute right-0 top-[calc(100%_+_6px)] z-30 w-[320px] max-w-[calc(100vw_-_24px)] rounded-[12px] border-[0.5px] border-line-neutral bg-surface px-4 py-[14px] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)]' role='dialog' aria-label={t('memorySettings.addTitle')}>
                  <div className='text-[14px] font-semibold text-[rgba(0,0,0,0.9)]'>{t('memorySettings.addTitle')}</div>
                  <div className='mt-3 flex flex-col gap-3'>
                    <label className='flex flex-col gap-1.5 text-[#27364d] font-semibold'>
                      <span className='text-[12px] text-[rgba(0,0,0,0.6)]'>{t('memorySettings.addKindLabel')}</span>
                      <Select className="w-full" value={draftKind} onChange={(event) => setDraftKind(event.target.value as typeof KINDS[number])}>
                        {KINDS.map((kind) => <option key={kind} value={kind}>{kindLabel(kind)}</option>)}
                      </Select>
                      <span className='text-[12px] leading-[18px] text-[rgba(0,0,0,0.4)]'>{kindHint(draftKind)}</span>
                    </label>
                    <label className='flex flex-col gap-1.5 text-[#27364d] font-semibold'>
                      <span className='text-[12px] text-[rgba(0,0,0,0.6)]'>{t('memorySettings.addContentLabel')}</span>
                      <Textarea
                        className="w-full box-border border border-[#cbd5e1] rounded-control bg-white text-ink [font:inherit] px-[.65rem] py-[.55rem]"
                        value={draftContent}
                        rows={3}
                        maxLength={300}
                        placeholder={t('memorySettings.addPlaceholder')}
                        onChange={(event) => setDraftContent(event.target.value)}
                      />
                    </label>
                    <div className='flex justify-end gap-2'>
                      <Button type='button' onClick={() => setAddVisible(false)}>{t('common.cancel')}</Button>
                      <Button type='button' loading={busy} disabled={!draftContent.trim()} onClick={() => void handleCreate()}>{t('memorySettings.add')}</Button>
                    </div>
                  </div>
                </div>
              ) : null}
            </span>
            <Button type='button' className='min-h-6 gap-1 rounded-[3px] border-0 bg-transparent px-[7px] py-0 text-[13px] text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] enabled:hover:text-[rgba(0,0,0,0.9)] disabled:cursor-not-allowed disabled:opacity-40' onClick={() => void handleExport()}>
              <Icon name='download' />
              {t('memorySettings.export')}
            </Button>
            <Popconfirm
              open={confirmKey === 'consolidate'}
              message={t('memorySettings.consolidateConfirm')}
              confirmLabel={t('memorySettings.consolidate')}
              cancelLabel={t('common.cancel')}
              danger={false}
              busy={consolidating}
              label={t('memorySettings.consolidateConfirm')}
              onToggle={() => setConfirmKey(confirmKey === 'consolidate' ? null : 'consolidate')}
              onCancel={() => setConfirmKey(null)}
              onConfirm={() => void handleConsolidate()}
            >
              <Button type='button' className='min-h-6 gap-1 rounded-[3px] border-0 bg-transparent px-[7px] py-0 text-[13px] text-[rgba(0,0,0,0.6)] enabled:hover:bg-[#f3f3f3] enabled:hover:text-[rgba(0,0,0,0.9)] disabled:cursor-not-allowed disabled:opacity-40' loading={consolidating} disabled={!canWrite || totalAll === 0}>
                <Icon name='swap' />
                {t('memorySettings.consolidate')}
              </Button>
            </Popconfirm>
            <span className='relative inline-flex' ref={clearAnchorRef}>
              <Popconfirm
                open={confirmKey === 'clear'}
                message={t('memorySettings.clearConfirm')}
                confirmLabel={t('memorySettings.clear')}
                cancelLabel={t('common.cancel')}
                label={t('memorySettings.clearConfirm')}
                busy={busy}
                onToggle={() => setConfirmKey(confirmKey === 'clear' ? null : 'clear')}
                onCancel={() => setConfirmKey(null)}
                onConfirm={() => void handleClear()}
              >
                <Button type='button' className='min-h-6 gap-1 rounded-[3px] border-0 bg-transparent px-[7px] py-0 text-[13px] text-[#e34d59] enabled:hover:bg-[#f3f3f3] enabled:hover:text-[#c9353f] disabled:cursor-not-allowed disabled:opacity-40' disabled={totalAll === 0 && trackingCount === 0 && documentCount === 0}>
                  <Icon name='delete' />
                  {t('memorySettings.clear')}
                </Button>
              </Popconfirm>
            </span>
          </div>
        </div>

        <div className='flex items-center overflow-x-auto border-b border-line-neutral' role='tablist' aria-label={t('memorySettings.listTitle')}>
          {allTabs.map((value) => (
            <button key={value} type='button' role='tab' aria-selected={value === tab} className='m-0 inline-flex cursor-pointer items-center gap-[5px] whitespace-nowrap rounded-none border-0 border-b-2 border-b-transparent bg-transparent px-3 py-2 text-[13px] text-[rgba(0,0,0,0.6)] transition-[color,border-color] duration-200 ease-[ease] first:pl-0 hover:text-[rgba(0,0,0,0.9)] focus-visible:-outline-offset-2 focus-visible:outline-2 focus-visible:outline-accent/20 aria-selected:border-b-accent aria-selected:font-medium aria-selected:text-accent' onClick={() => void handleTabChange(value)}>
              <Icon name={TAB_ICONS[value]} size={14} />
              <span>{tabLabel(value)}</span>
            </button>
          ))}
        </div>

        <div className={'relative min-h-12' + (loading ? ' pointer-events-none opacity-55' : '')} aria-busy={loading}>
          {loading ? (
            <div className='flex min-h-12 items-center justify-center gap-2 py-4 text-[13px] text-[rgba(0,0,0,0.4)]' role='status' aria-label={t('common.loading')}>
              <span className='h-4 w-4 animate-spin rounded-full border-2 border-line-neutral border-t-accent' aria-hidden='true' />
              <span>{t('common.loading')}</span>
            </div>
          ) : null}
          {listBody}
        </div>
        {statusHint ? <p className='mb-0 mt-3 text-[12px] leading-[18px] text-[rgba(0,0,0,0.6)]'>{statusHint}</p> : null}

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
