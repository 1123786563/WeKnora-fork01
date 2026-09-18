import type { KnowledgeProcessingStatus } from '@weknora/contracts';

export const knowledgeProcessingStatuses: readonly KnowledgeProcessingStatus[] = [
  'pending', 'processing', 'finalizing', 'completed', 'failed', 'deleting', 'cancelled',
];

export function normalizeKnowledgeProcessingStatus(value: unknown): KnowledgeProcessingStatus {
  if (typeof value === 'string' && knowledgeProcessingStatuses.includes(value as KnowledgeProcessingStatus)) {
    return value as KnowledgeProcessingStatus;
  }
  throw new Error(`Unknown knowledge processing status: ${String(value)}`);
}

export function isKnowledgeProcessingTerminal(status: KnowledgeProcessingStatus): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}

export function processingStatusLabel(status: KnowledgeProcessingStatus): string {
  return status === 'finalizing' ? 'Finalizing' : status.charAt(0).toUpperCase() + status.slice(1);
}

// ===== Processing timeline (minimal port of Vue knowledge-processing-timeline.vue) =====
// Vue polls GET /knowledge/:id/spans every 2s while parsing is in flight and
// renders an ordered stage list from the returned trace tree
// (internal/router/routes_knowledge.go:104-105 → handler.GetKnowledgeSpans).

export const knowledgeProcessingStages = ['docreader', 'chunking', 'embedding', 'multimodal', 'postprocess'] as const;
export type KnowledgeProcessingStage = (typeof knowledgeProcessingStages)[number];

/** Statuses in which the timeline keeps polling the spans endpoint. */
export function isKnowledgeProcessingActive(status: string | undefined | null): boolean {
  return status === 'pending' || status === 'processing' || status === 'finalizing';
}

/** Strict polling rule used by background consumers. */
export function shouldPollKnowledgeSpans(status: string | undefined | null): boolean {
  return isKnowledgeProcessingActive(status);
}

export interface KnowledgeSpanNode {
  name?: string;
  stage?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  // Backend spans (internal/handler/knowledge.go GetKnowledgeSpans) serialize
  // started_at/finished_at — the fields Vue's nodeStart/nodeEnd read. The
  // start_time/end_time spellings above stay accepted for older fixtures.
  started_at?: string;
  finished_at?: string;
  duration_ms?: number;
  error?: unknown;
  children?: KnowledgeSpanNode[];
  [key: string]: unknown;
}

export interface KnowledgeSpansView {
  knowledge_id?: unknown;
  parse_status?: unknown;
  current_stage?: unknown;
  trace?: KnowledgeSpanNode | null;
  [key: string]: unknown;
}

/** Keep Vue's visible-timeline quiesce grace for delayed postprocess spans. */
export function shouldGracePollKnowledgeSpans(
  spans: KnowledgeSpansView,
  now = Date.now(),
  graceMs = 2 * 60 * 1000,
): boolean {
  if (shouldPollKnowledgeSpans(typeof spans.parse_status === 'string' ? spans.parse_status : undefined)) return true;

  const latestActivity = (node: KnowledgeSpanNode | null | undefined): number => {
    if (!node || typeof node !== 'object') return 0;
    const timestamps = [node.updated_at, node.finished_at, node.started_at, node.start_time, node.created_at];
    let latest = 0;
    for (const value of timestamps) {
      if (typeof value !== 'string') continue;
      const parsed = Date.parse(value);
      if (!Number.isNaN(parsed)) latest = Math.max(latest, parsed);
    }
    for (const child of node.children ?? []) latest = Math.max(latest, latestActivity(child));
    return latest;
  };

  const activity = latestActivity(spans.trace);
  return activity > 0 && now >= activity && now - activity < graceMs;
}

export type KnowledgeTimelineStepState = 'pending' | 'running' | 'done' | 'failed' | 'skipped';

export interface KnowledgeTimelineStep {
  stage: KnowledgeProcessingStage;
  label: string;
  state: KnowledgeTimelineStepState;
}

function spanStatus(node: KnowledgeSpanNode): KnowledgeTimelineStepState {
  const raw = typeof node.status === 'string' ? node.status.toLowerCase() : '';
  if (/(fail|error|cancel|abort)/.test(raw)) return 'failed';
  // A skipped stage never executed (e.g. multimodal disabled for the KB);
  // Vue renders it as knowledgeStages.status.skipped (已跳过), never running.
  if (raw === 'skipped' || raw === 'skip') return 'skipped';
  if (/(succe|complet|done|finish|ok)/.test(raw)) return 'done';
  if (raw === 'running' || raw === 'in_progress' || raw === 'started' || raw === 'active') return 'running';
  // Backend spans carry started_at/finished_at; accept both spellings.
  if (typeof (node.end_time ?? node.finished_at) === 'string' && (node.end_time ?? node.finished_at) !== '') return 'done';
  if (typeof (node.start_time ?? node.started_at) === 'string' && (node.start_time ?? node.started_at) !== '') return 'running';
  return 'pending';
}

function normalizeStageToken(value: unknown): string {
  return typeof value === 'string' ? value.toLowerCase().replace(/[\s_-]+/g, '') : '';
}

function collectSpans(node: KnowledgeSpanNode | null | undefined, into: KnowledgeSpanNode[]): void {
  if (!node || typeof node !== 'object') return;
  into.push(node);
  if (Array.isArray(node.children)) for (const child of node.children) collectSpans(child, into);
}

export interface KnowledgeTimelineNode {
  key: string;
  depth: number;
  node: KnowledgeSpanNode;
  hasChildren: boolean;
}

/** Flatten the backend trace tree without losing parent depth for a waterfall UI. */
export function flattenKnowledgeSpans(root: KnowledgeSpanNode | null | undefined): KnowledgeTimelineNode[] {
  const rows: KnowledgeTimelineNode[] = [];
  const visit = (node: KnowledgeSpanNode, depth: number, key: string) => {
    rows.push({ key, depth, node, hasChildren: Array.isArray(node.children) && node.children.length > 0 });
    if (Array.isArray(node.children)) node.children.forEach((child, index) => visit(child, depth + 1, `${key}.${index}`));
  };
  if (root) visit(root, 0, 'root');
  return rows;
}

/** Ordered step list for the timeline: the fixed pipeline stages, each mapped
 *  to the best-matching span in the trace tree (or pending when absent). */
export function buildKnowledgeTimeline(spans: KnowledgeSpansView): KnowledgeTimelineStep[] {
  const collected: KnowledgeSpanNode[] = [];
  collectSpans(spans.trace ?? null, collected);
  const currentStage = normalizeStageToken(spans.current_stage);
  const stageFor = (node: KnowledgeSpanNode): KnowledgeProcessingStage | undefined => {
    const tokens = [node.name, node.stage].map(normalizeStageToken).filter((token) => token !== '');
    for (const stage of knowledgeProcessingStages) {
      const token = normalizeStageToken(stage);
      if (tokens.some((candidate) => candidate.includes(token) || token.includes(candidate))) return stage;
    }
    return undefined;
  };
  const stateFor = (stage: KnowledgeProcessingStage): KnowledgeTimelineStepState => {
    const matches = collected.filter((node) => stageFor(node) === stage);
    if (matches.length === 0) {
      // No span yet: the backend current_stage marks where parsing is right now.
      return currentStage === normalizeStageToken(stage) ? 'running' : 'pending';
    }
    const states = matches.map(spanStatus);
    if (states.includes('failed')) return 'failed';
    if (states.includes('running')) return 'running';
    // Vue counts done and skipped alike as traversed (currentStageIndex);
    // a stage whose spans all skipped (multimodal disabled) surfaces as
    // skipped so the timeline reads 已跳过 instead of 进行中.
    if (states.every((state) => state === 'done' || state === 'skipped')) {
      return states.includes('done') ? 'done' : 'skipped';
    }
    return 'running';
  };
  return knowledgeProcessingStages.map((stage) => ({
    stage,
    label: stage,
    state: stateFor(stage),
  }));
}
