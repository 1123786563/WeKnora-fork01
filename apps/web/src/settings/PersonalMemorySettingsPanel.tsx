// Personal memory settings — ported from Vue frontend/src/views/settings/MemorySettings.vue
// (mounted by Settings.vue under the "mymemory" section). Anatomy, states, copy and
// action semantics follow the Vue source; visual tokens mirror the TDesign light theme
// values Vue resolves at runtime (see personal-memory.css header).
import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Status, Switch } from '@weknora/ui';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import './personal-memory.css';

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
    <span className='wk-mem-anchor' ref={ref}>
      <span onClick={onToggle}>{children}</span>
      {open ? (
        <div className='wk-popconfirm' role='alertdialog' aria-label={label ?? confirmLabel}>
          <p>{message}</p>
          <div className='wk-popconfirm__actions'>
            <Button type='button' onClick={onCancel}>{cancelLabel}</Button>
            <Button type='button' className={'wk-popconfirm__confirm' + (danger ? ' danger' : '')} disabled={busy} onClick={onConfirm}>{confirmLabel}</Button>
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
    window.location.assign(url);
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
        <li key={id || index} className='wk-memory-item'>
          <div className='wk-memory-main'>
            {editingId === id ? (
              <div className='wk-memory-edit'>
                <textarea
                  value={editingContent}
                  rows={2}
                  aria-label={t('common.edit')}
                  onChange={(event) => setEditingContent(event.target.value)}
                  onKeyDown={(event) => { if (event.ctrlKey && event.key === 'Enter') void handleSaveEdit(row); }}
                />
                <div className='wk-memory-edit-actions'>
                  <Button type='button' onClick={() => { setEditingId(''); setEditingContent(''); }}>{t('common.cancel')}</Button>
                  <Button type='button' loading={busy} onClick={() => void handleSaveEdit(row)}>{t('common.save')}</Button>
                </div>
              </div>
            ) : (
              <p className={'wk-memory-content' + (isRetired ? ' is-inactive' : '')}>{str(row, 'content')}</p>
            )}
            <div className='wk-memory-meta'>
              <span title={kind ? kindHint(kind) : undefined}>{kind ? kindLabel(kind) : ''}</span>
              {topic && topic !== str(row, 'content') ? <span className='wk-memory-topic' title={topic}>{topic}</span> : null}
              <span>{originLabel(str(row, 'origin'))}</span>
              <span>{formatTime(str(row, 'valid_from'))}</span>
            </div>
          </div>
          <div className='wk-memory-actions'>
            {status === 'pending' ? (
              <>
                <button type='button' className='wk-mem-icon-btn wk-mem-chip-btn' disabled={!canWrite} onClick={() => void handleConfirmGuess(row)}>
                  <Icon name='check' />
                  {t('memorySettings.confirmGuess')}
                </button>
                <button type='button' className='wk-mem-icon-btn wk-mem-chip-btn' onClick={() => void handleRejectGuess(row)}>
                  <Icon name='close' />
                  {t('memorySettings.rejectGuess')}
                </button>
              </>
            ) : null}
            {status === 'active' ? (
              <button type='button' className='wk-mem-icon-btn' aria-label={t('common.edit')} title={t('common.edit')} disabled={!canWrite} onClick={() => startEdit(row)}>
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
                <button type='button' className='wk-mem-icon-btn is-danger' aria-label={t('common.delete')} title={t('common.delete')}>
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
        <li key={id || index} className='wk-memory-item'>
          <div className='wk-memory-main'>
            <p className='wk-memory-content'>{str(row, 'topic')}</p>
            <div className='wk-topic-progress'>
              <div className='wk-topic-progress-track' role='progressbar' aria-valuenow={percentage} aria-valuemin={0} aria-valuemax={100}>
                <div className='wk-topic-progress-fill' style={{ width: percentage + '%' }} />
              </div>
              <span>{hits >= threshold ? t('memorySettings.trackingReady') : t('memorySettings.trackingProgress', { hits, threshold })}</span>
            </div>
            <div className='wk-memory-meta'>
              <span>{t('memorySettings.kinds.interest')}</span>
              {aliases.length > 0 ? (
                <span className='wk-memory-topic' title={aliases.join(', ')}>
                  {t('memorySettings.trackingAliases', { aliases: aliases.join(', ') })}
                </span>
              ) : null}
              <span>{formatTime(str(row, 'last_seen_at'))}</span>
            </div>
          </div>
          <div className='wk-memory-actions'>
            <button type='button' className='wk-mem-icon-btn wk-mem-chip-btn' disabled={!canWrite} onClick={() => void handlePromoteTopic(row)}>
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
              <button type='button' className='wk-mem-icon-btn wk-mem-chip-btn' title={t('memorySettings.dismissTopic')}>
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
        <li key={id || index} className='wk-memory-item'>
          <div className='wk-memory-main'>
            <p className='wk-memory-content'>{str(row, 'title') || t('memorySettings.untitledDocument')}</p>
            <div className='wk-memory-meta'>
              <span>{t('memorySettings.documentsHits', { hits: num(row, 'hits') })}</span>
              <span>{formatTime(str(row, 'last_used_at'))}</span>
            </div>
          </div>
          <div className='wk-memory-actions'>
            <button
              type='button'
              className='wk-mem-icon-btn wk-mem-chip-btn'
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
              <button type='button' className='wk-mem-icon-btn wk-mem-chip-btn' title={t('memorySettings.stopTrackingDocument')}>
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
        <div className='wk-mem-empty'>
          <p className='wk-mem-empty-title'>{emptyTitle}</p>
          <p className='wk-mem-empty-desc'>{emptyDescription}</p>
        </div>
      );
    }
    if (isDocuments) return <ul className='wk-memory-list'>{documentRows}</ul>;
    if (isTracking) return <ul className='wk-memory-list'>{trackingRows}</ul>;
    return <ul className='wk-memory-list'>{itemRows}</ul>;
  })();
  // Vue renders the section bare on the drawer background (no outer card).
  return (
    <div className='wk-memory-settings'>
      <div className='wk-mem-header'>
        <div className='wk-mem-header-wrap'>
          <h2>{t('memorySettings.title')}</h2>
          <span className='wk-mem-usage-anchor'>
            <button
              type='button'
              className='wk-mem-usage-trigger'
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
              <div className='wk-mem-usage-popup' role='dialog' aria-label={t('memorySettings.usage.title')}>
                <div className='wk-mem-usage-title'>{t('memorySettings.usage.title')}</div>
                <p className='wk-mem-usage-intro'>{t('memorySettings.usage.intro')}</p>
                <div className='wk-mem-usage-rows'>
                  {USAGE_ROW_KEYS.map((key) => (
                    <div key={key} className='wk-mem-usage-row'>
                      <span className='wk-mem-usage-label'>{t('memorySettings.usage.rows.' + key + '.label')}</span>
                      <span className='wk-mem-usage-text'>{t('memorySettings.usage.rows.' + key + '.text')}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </span>
        </div>
        <p className='wk-mem-description'>{t('memorySettings.description')}</p>
      </div>

      {settings !== null && !workspaceEnabled ? (
        <div className='wk-mem-notice' role='status'>
          <Icon name='info-circle' />
          <span>{t('memorySettings.workspaceDisabled')}</span>
        </div>
      ) : null}

      <div className='wk-mem-setting-row'>
        <div className='wk-mem-setting-info'>
          <label>{t('memorySettings.enableLabel')}</label>
          <p className='wk-mem-desc'>{t('memorySettings.enableDescription')}</p>
          {enabled && workspaceEnabled ? <p className='wk-mem-desc'>{t('memorySettings.agentDisabledHint')}</p> : null}
        </div>
        <div className='wk-memory-actions'>
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

      <div className='wk-mem-list-section'>
        <div className='wk-mem-toolbar'>
          <div className='wk-mem-list-title'>
            <h3>{t('memorySettings.listTitle')}</h3>
            <span className='wk-mem-list-count'>{t('memorySettings.listCount', { count: totalAll })}</span>
          </div>
          <div className='wk-mem-list-actions'>
            <span className='wk-mem-anchor' ref={addAnchorRef}>
              <Button type='button' className='wk-mem-tool-btn' disabled={!canWrite} onClick={() => { if (!addVisible) setDraftContent(''); setAddVisible(!addVisible); }}>
                <Icon name='add' />
                {t('memorySettings.add')}
              </Button>
              {addVisible ? (
                <div className='wk-mem-pop' role='dialog' aria-label={t('memorySettings.addTitle')}>
                  <div className='wk-mem-add-title'>{t('memorySettings.addTitle')}</div>
                  <div className='wk-mem-add-form'>
                    <label className='wk-mem-add-field'>
                      <span>{t('memorySettings.addKindLabel')}</span>
                      <select value={draftKind} onChange={(event) => setDraftKind(event.target.value as typeof KINDS[number])}>
                        {KINDS.map((kind) => <option key={kind} value={kind}>{kindLabel(kind)}</option>)}
                      </select>
                      <span className='wk-mem-kind-hint'>{kindHint(draftKind)}</span>
                    </label>
                    <label className='wk-mem-add-field'>
                      <span>{t('memorySettings.addContentLabel')}</span>
                      <textarea
                        value={draftContent}
                        rows={3}
                        maxLength={300}
                        placeholder={t('memorySettings.addPlaceholder')}
                        onChange={(event) => setDraftContent(event.target.value)}
                      />
                    </label>
                    <div className='wk-mem-add-footer'>
                      <Button type='button' onClick={() => setAddVisible(false)}>{t('common.cancel')}</Button>
                      <Button type='button' loading={busy} disabled={!draftContent.trim()} onClick={() => void handleCreate()}>{t('memorySettings.add')}</Button>
                    </div>
                  </div>
                </div>
              ) : null}
            </span>
            <Button type='button' className='wk-mem-tool-btn' onClick={() => void handleExport()}>
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
              <Button type='button' className='wk-mem-tool-btn' loading={consolidating} disabled={!canWrite || totalAll === 0}>
                <Icon name='swap' />
                {t('memorySettings.consolidate')}
              </Button>
            </Popconfirm>
            <span className='wk-mem-anchor' ref={clearAnchorRef}>
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
                <Button type='button' className='wk-mem-tool-btn is-danger' disabled={totalAll === 0 && trackingCount === 0 && documentCount === 0}>
                  <Icon name='delete' />
                  {t('memorySettings.clear')}
                </Button>
              </Popconfirm>
            </span>
          </div>
        </div>

        <div className='wk-mem-tabs' role='tablist' aria-label={t('memorySettings.listTitle')}>
          {allTabs.map((value) => (
            <button key={value} type='button' role='tab' aria-selected={value === tab} className='wk-mem-tab' onClick={() => void handleTabChange(value)}>
              <Icon name={TAB_ICONS[value]} size={14} />
              <span>{tabLabel(value)}</span>
            </button>
          ))}
        </div>

        <div className={'wk-mem-loading' + (loading ? ' is-loading' : '')}>
          {listBody}
        </div>
        {statusHint ? <p className='wk-mem-status-hint'>{statusHint}</p> : null}

        {listTotal > PAGE_SIZE ? (
          <div className='wk-memory-pagination' role='navigation' aria-label={t('memorySettings.listTitle')}>
            <button type='button' className='wk-mem-page-btn' disabled={page <= 1} onClick={() => void handlePageChange(page - 1)} aria-label='previous page'>{'<'}</button>
            {Array.from({ length: totalPages }, (_, index) => index + 1).map((pageNumber) => (
              <button key={pageNumber} type='button' className={'wk-mem-page-btn' + (pageNumber === page ? ' is-current' : '')} aria-current={pageNumber === page ? 'page' : undefined} onClick={() => void handlePageChange(pageNumber)}>
                {pageNumber}
              </button>
            ))}
            <button type='button' className='wk-mem-page-btn' disabled={page >= totalPages} onClick={() => void handlePageChange(page + 1)} aria-label='next page'>{'>'}</button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
