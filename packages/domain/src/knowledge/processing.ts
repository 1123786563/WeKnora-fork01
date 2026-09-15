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

/** Polling rule (strict mode, Vue gracePoll=false): poll only while the
 *  document parse_status itself is non-terminal. */
export function shouldPollKnowledgeSpans(status: string | undefined | null): boolean {
  return isKnowledgeProcessingActive(status);
}

export interface KnowledgeSpanNode {
  name?: string;
  stage?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
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

export type KnowledgeTimelineStepState = 'pending' | 'running' | 'done' | 'failed';

export interface KnowledgeTimelineStep {
  stage: KnowledgeProcessingStage;
  label: string;
  state: KnowledgeTimelineStepState;
}

function spanStatus(node: KnowledgeSpanNode): KnowledgeTimelineStepState {
  const raw = typeof node.status === 'string' ? node.status.toLowerCase() : '';
  if (/(fail|error|cancel|abort)/.test(raw)) return 'failed';
  if (/(succe|complet|done|finish|ok)/.test(raw)) return 'done';
  if (raw === 'running' || raw === 'in_progress' || raw === 'started' || raw === 'active') return 'running';
  if (typeof node.end_time === 'string' && node.end_time !== '') return 'done';
  if (typeof node.start_time === 'string' && node.start_time !== '') return 'running';
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
    return states.every((state) => state === 'done') ? 'done' : 'running';
  };
  return knowledgeProcessingStages.map((stage) => ({
    stage,
    label: stage,
    state: stateFor(stage),
  }));
}
