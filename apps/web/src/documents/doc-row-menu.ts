import {
  buildKnowledgeTimeline,
  flattenKnowledgeSpans,
  isKnowledgeProcessingActive,
  knowledgeProcessingStages,
  type KnowledgeProcessingStage,
  type KnowledgeSpansView,
  type KnowledgeTimelineStep,
} from '@weknora/domain/knowledge/processing';

// R483 F4 — Vue KB detail document row menu parity (R482 B1 diff #8).
// Ports the pure decisions behind frontend/src/views/knowledge/components/
// DocumentActionMenu.vue (item set + gates), KnowledgeBase.vue
// (probeTraceAvailable / move sub-flow state) and knowledge-processing-
// timeline.vue (compact hover-popover caption) so the React page and its
// tests share one source of truth.

/** Vue DocumentActionMenu emits, in template order. */
export type DocumentMenuAction =
  | 'download'
  | 'edit'
  | 'view-trace'
  | 'reparse'
  | 'cancel-parse'
  | 'move-folder'
  | 'move-kb'
  | 'batch-manage'
  | 'delete';

export interface DocumentMenuItem {
  action: DocumentMenuAction;
  /** i18n key — every key exists in all five packages/i18n locales. */
  labelKey: string;
}

export interface DocumentMenuInput {
  /** Vue item.type ('file' | 'manual' | 'url' …) — Vue gates download/edit on it. */
  source?: string;
  parseStatus?: string;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  /** Result of the per-document /spans probe; undefined = not probed yet. */
  traceAvailable?: boolean;
}

/**
 * Vue DocumentActionMenu.vue item list: 下载 → 编辑(仅 manual) → 查看 Trace
 * (in-flight or probed) → 重建知识 → 取消解析(in-flight) → 移动到目录 →
 * 移动到... → 批量管理 → 删除文档. The delete entry has no gate in Vue —
 * the menu itself only renders for contributors; batch-manage gates on
 * canMutateKnowledge || canDownload.
 */
export function documentMenuItems(input: DocumentMenuInput): DocumentMenuItem[] {
  const items: DocumentMenuItem[] = [];
  // Vue DocumentActionMenu.vue:48 gates on item.type ('file' | 'manual');
  // an absent type hides download (no fallback).
  const source = input.source ?? '';
  const downloadable = source === 'file' || source === 'manual';
  const inFlight = isKnowledgeProcessingActive(input.parseStatus);

  if (input.canDownload && downloadable) {
    items.push({ action: 'download', labelKey: 'knowledgeBase.downloadDocument' });
  }
  if (source === 'manual') {
    items.push({ action: 'edit', labelKey: 'knowledgeBase.editDocument' });
  }
  if (inFlight || input.traceAvailable === true) {
    items.push({ action: 'view-trace', labelKey: 'knowledgeStages.viewTrace' });
  }
  items.push({ action: 'reparse', labelKey: 'knowledgeBase.rebuildDocument' });
  if (inFlight) {
    items.push({ action: 'cancel-parse', labelKey: 'knowledgeBase.documents.cancelParse' });
  }
  if (input.canMutateKnowledge) {
    items.push({ action: 'move-folder', labelKey: 'knowledgeBase.moveToFolder.action' });
    items.push({ action: 'move-kb', labelKey: 'knowledgeBase.moveDocument' });
  }
  if (input.canMutateKnowledge || input.canDownload) {
    items.push({ action: 'batch-manage', labelKey: 'menu.batchManage' });
  }
  items.push({ action: 'delete', labelKey: 'knowledgeBase.deleteDocument' });
  return items;
}

/**
 * Vue utils/knowledgeTrace.ts knowledgeSpansPayloadHasTrace: GET /spans only
 * counts as a real trace when the root span carries an id or the attempt
 * counter moved past the legacy placeholder.
 */
export function knowledgeSpansViewHasTrace(view: unknown): boolean {
  if (view === null || view === undefined || typeof view !== 'object' || Array.isArray(view)) return false;
  const row = view as { trace?: unknown; current_attempt?: unknown };
  const trace = row.trace;
  if (trace === null || trace === undefined || typeof trace !== 'object' || Array.isArray(trace)) return false;
  const spanId = (trace as { span_id?: unknown }).span_id;
  const spanIdTruthy = (typeof spanId === 'string' && spanId !== '') || (typeof spanId === 'number' && spanId !== 0);
  const attempt = row.current_attempt;
  const attemptPositive = typeof attempt === 'number' && attempt > 0;
  return spanIdTruthy || attemptPositive;
}

/** Vue knowledge-processing-timeline.vue formatDuration. */
export function formatTraceDurationMs(ms: number | undefined | null): string {
  if (ms === undefined || ms === null || Number.isNaN(ms) || ms < 0) return '—';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
  const mins = Math.floor(ms / 60000);
  const rem = ((ms % 60000) / 1000).toFixed(1);
  return `${mins}m${rem}s`;
}

export interface TraceSummary {
  /** Vue totalMs computed: max(root duration_ms, observed span window). */
  totalMs: number;
  duration: string;
  /** Vue currentStageIndex (1-based). */
  stageIndex: number;
  stageTotal: number;
  /** Stage the Vue currentStageLabel points at (failed → running → pending → last). */
  activeStage: KnowledgeProcessingStage | undefined;
  steps: KnowledgeTimelineStep[];
}

function spanTime(value: unknown): number | null {
  if (typeof value !== 'string' || value === '') return null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
}

/** Vue nodeEnd: finished_at wins; otherwise started_at + duration_ms. */
function spanEnd(node: { started_at?: unknown; finished_at?: unknown; duration_ms?: unknown }): number | null {
  const finished = spanTime(node.finished_at);
  if (finished !== null) return finished;
  const started = spanTime(node.started_at);
  const duration = typeof node.duration_ms === 'number' && node.duration_ms >= 0 ? node.duration_ms : null;
  if (started !== null && duration !== null) return started + duration;
  return null;
}

/**
 * Compact hover-popover summary (Vue KnowledgeProcessingTimeline compact
 * mode): the fixed five stages plus the total-duration caption the failed /
 * in-flight document card shows.
 */
export function buildTraceSummary(view: KnowledgeSpansView | null | undefined): TraceSummary {
  const steps = buildKnowledgeTimeline(view ?? {});
  const root = view?.trace ?? null;

  let start = Infinity;
  let end = -Infinity;
  for (const row of flattenKnowledgeSpans(root)) {
    const nodeStart = spanTime(row.node.started_at);
    if (nodeStart !== null) start = Math.min(start, nodeStart);
    const nodeEnd = spanEnd(row.node);
    if (nodeEnd !== null) end = Math.max(end, nodeEnd);
  }
  const observed = start === Infinity || end === -Infinity || end <= start ? 0 : end - start;
  const rootDuration = typeof root?.duration_ms === 'number' ? root.duration_ms : undefined;
  const totalMs =
    rootDuration !== undefined && rootDuration > 0
      ? Math.max(rootDuration, observed)
      : observed;

  // Vue currentStageIndex: the first running/failed stage, else traversed+1.
  const activeIndex = steps.findIndex((step) => step.state === 'running' || step.state === 'failed');
  const traversed = steps.filter((step) => step.state === 'done' || step.state === 'skipped').length;
  const stageIndex = activeIndex >= 0 ? activeIndex + 1 : Math.min(traversed + 1, steps.length);

  const labelStep =
    steps.find((step) => step.state === 'failed')
    ?? steps.find((step) => step.state === 'running')
    ?? steps.find((step) => step.state === 'pending')
    ?? steps[steps.length - 1];

  return {
    totalMs,
    // Vue compact caption only renders the duration when totalMs > 0; the
    // progress fallback takes over otherwise.
    duration: totalMs > 0 ? formatTraceDurationMs(totalMs) : '—',
    stageIndex,
    stageTotal: knowledgeProcessingStages.length,
    activeStage: labelStep?.stage,
    steps,
  };
}

// --- move-to-KB sub-flow (Vue KnowledgeBase.vue L1517-1600) -------------------

export type MoveMenuView = 'normal' | 'targets' | 'confirm';
export type KnowledgeMoveMode = 'reuse_vectors' | 'reparse';

/** Vue handleMoveSelectTarget always re-selects 复用向量 first. */
export const DEFAULT_MOVE_MODE: KnowledgeMoveMode = 'reuse_vectors';

export interface MoveTargetKb {
  id: string;
  name: string;
  knowledge_count?: number;
  [key: string]: unknown;
}

/** Vue handleMoveBack: confirm → targets, targets → normal. */
export function moveMenuViewAfterBack(view: MoveMenuView): MoveMenuView {
  if (view === 'confirm') return 'targets';
  if (view === 'targets') return 'normal';
  return 'normal';
}

/**
 * Page-owned state for the in-menu move sub-flow (Vue keeps moveMenuMode /
 * moveTargetKbs / moveSubmitting on KnowledgeBase.vue and pipes them into
 * every row menu). view 'normal' = the row renders the normal item list, but
 * onStart stays callable so 移动到... can open the picker.
 */
export interface DocumentMoveKbController {
  view: MoveMenuView;
  targets: MoveTargetKb[];
  loading: boolean;
  selectedTargetName: string;
  mode: KnowledgeMoveMode;
  submitting: boolean;
  onStart: () => void;
  onSelectTarget: (kb: MoveTargetKb) => void;
  onBack: () => void;
  onModeChange: (mode: KnowledgeMoveMode) => void;
  onConfirm: () => void;
}
