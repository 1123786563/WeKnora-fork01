// T17（Issue #47）：只读研究委派与版本钉定批注的信封契约。键集与
// internal/handler/session/workbench_research.go 的 JSON tag 逐字对应；
// 解析器整体拒绝（不部分渲染）——客户端绝不渲染服务端从未确认的行。

export type ResearchStatusWire = 'assigned' | 'completed';

export interface ResearchDelegationWire {
  delegation_id: string;
  run_id: string;
  session_id: string;
  objective: string;
  sources: string[];
  status: ResearchStatusWire;
  summary?: string;
  created_at: string;
}

export interface ResearchListWire {
  items: ResearchDelegationWire[];
}

export interface AnnotationWire {
  annotation_id: string;
  run_id: string;
  material_id: string;
  base_version: string;
  body: string;
  author_id: string;
  created_at: string;
}

export interface AnnotationListWire {
  items: AnnotationWire[];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function envelope(value: unknown): Record<string, unknown> {
  if (!isObject(value)) throw new Error('research response must be an object');
  if (value.success !== true || !isObject(value.data)) {
    throw new Error('research response must be a success envelope with data');
  }
  return value.data;
}

function rows<T>(value: unknown, parseRow: (row: unknown) => T): T[] {
  const data = envelope(value);
  if (!Array.isArray(data.items)) throw new Error('research response items must be an array');
  return data.items.map(parseRow);
}

function stringField(row: Record<string, unknown>, key: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value === '') throw new Error(`research row.${key} must be a non-empty string`);
  return value;
}

function optionalStringField(row: Record<string, unknown>, key: string): string | undefined {
  const value = row[key];
  if (value === undefined) return undefined;
  if (typeof value !== 'string' || value === '') throw new Error(`research row.${key} must be a non-empty string when present`);
  return value;
}

const RESEARCH_STATUSES: readonly ResearchStatusWire[] = ['assigned', 'completed'];

function delegationRow(row: unknown): ResearchDelegationWire {
  if (!isObject(row)) throw new Error('research delegation row must be an object');
  const status = row.status;
  if (typeof status !== 'string' || !RESEARCH_STATUSES.includes(status as ResearchStatusWire)) {
    throw new Error('research row.status must be assigned or completed');
  }
  if (!Array.isArray(row.sources) || row.sources.some((s) => typeof s !== 'string' || s === '')) {
    throw new Error('research row.sources must be an array of non-empty strings');
  }
  return {
    delegation_id: stringField(row, 'delegation_id'),
    run_id: stringField(row, 'run_id'),
    session_id: stringField(row, 'session_id'),
    objective: stringField(row, 'objective'),
    sources: row.sources as string[],
    status: status as ResearchStatusWire,
    summary: optionalStringField(row, 'summary'),
    created_at: stringField(row, 'created_at'),
  };
}

function annotationRow(row: unknown): AnnotationWire {
  if (!isObject(row)) throw new Error('annotation row must be an object');
  return {
    annotation_id: stringField(row, 'annotation_id'),
    run_id: stringField(row, 'run_id'),
    material_id: stringField(row, 'material_id'),
    base_version: stringField(row, 'base_version'),
    body: stringField(row, 'body'),
    author_id: stringField(row, 'author_id'),
    created_at: stringField(row, 'created_at'),
  };
}

/** 解析委派列表响应；任何字段缺失/类型不符整体抛错。 */
export function parseResearchListResponse(value: unknown): ResearchListWire {
  return { items: rows(value, delegationRow) };
}

/** 解析批注列表响应；任何字段缺失/类型不符整体抛错。 */
export function parseAnnotationListResponse(value: unknown): AnnotationListWire {
  return { items: rows(value, annotationRow) };
}
