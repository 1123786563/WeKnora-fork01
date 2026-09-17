/**
 * W28 — 知识引用与专业结果注册器（result renderer registry）。
 *
 * `selectRenderer` 是产品会话中「服务端结构化结果 → 客户端渲染器」的唯一
 * 映射点：只有显式列入 switch 的 kind 才能成为可交互 UI；任何未知 kind
 * （`javascript:eval`、`remote-component`、拼写变体……）一律落到安全文本
 * 渲染，永不成为可执行 UI。
 *
 * 渲染器不是授权层：引用的打开、文件的下载都经由注入的产品资源 seam
 * 重新请求授权（服务端逐次校验，撤销即拒绝）；引用契约数据不携带任何可
 * 直接执行的 source URL。所有 parse* 函数沿用 contracts 纪律——字段缺失、
 * 类型不符、超限时抛出带原因的错误，绝不把未校验的服务端数据交给组件。
 */
export type ResultRenderer = 'citation' | 'table' | 'file' | 'text';

export function selectRenderer(kind: string): ResultRenderer {
  switch (kind) {
    case 'knowledge.citation': return 'citation';
    case 'analysis.table': return 'table';
    case 'artifact.file': return 'file';
    default: return 'text';
  }
}

/** 结构化表格渲染上限：超出即降级为文件（下载）回退，不做无限表格。 */
export const ANALYSIS_TABLE_LIMITS = {
  maxColumns: 12,
  maxRows: 200,
  maxCellLength: 2000,
} as const;

function row(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error(`${label}_PAYLOAD_INVALID`);
  }
  return value as Record<string, unknown>;
}

function nonEmpty(value: unknown, label: string, field: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${label}_${field.toUpperCase()}_INVALID`);
  return value;
}

function bounded(value: unknown, label: string, field: string, max: number): string | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value !== 'string') throw new Error(`${label}_${field.toUpperCase()}_INVALID`);
  return value.length > max ? value.slice(0, max) : value;
}

export interface StructuredResultEnvelope {
  type: string;
  data: unknown;
}

/**
 * Detects the structured-result envelope inside one tool block's text. Only a
 * JSON object carrying a non-empty string `type` qualifies; everything else
 * (plain prose, arrays, primitives, broken JSON) stays ordinary text.
 */
export function parseStructuredResultText(text: string): StructuredResultEnvelope | null {
  if (typeof text !== 'string' || !text.trim().startsWith('{')) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return null;
  const envelope = parsed as Record<string, unknown>;
  if (typeof envelope.type !== 'string' || envelope.type.trim() === '') return null;
  // `{type, data}` envelopes keep their payload; flattened payloads become
  // their own data (minus the type key) so both wire shapes reach the same
  // validators.
  if (envelope.data !== undefined) return { type: envelope.type, data: envelope.data };
  const { type: _type, ...data } = envelope;
  return { type: envelope.type, data };
}

export interface KnowledgeCitationData {
  /** 授权的产品知识对象标识：打开动作每次都经产品 API 重新校验。 */
  documentID: string;
  knowledgeID?: string;
  knowledgeBaseID?: string;
  chunkIDs: readonly string[];
  title: string;
  snippet?: string;
}

/**
 * Validates one knowledge citation. A citation is only renderable when it
 * carries authorized knowledge/document/chunk ids the product can re-request;
 * URL-only citations have nothing authorized to open and are rejected. The
 * parsed data deliberately contains no URL field — rendering never executes a
 * source link.
 */
export function parseKnowledgeCitation(value: unknown): KnowledgeCitationData {
  const input = row(value, 'CITATION');
  const documentID = typeof input.document_id === 'string' ? input.document_id.trim() : '';
  const knowledgeID = typeof input.knowledge_id === 'string' && input.knowledge_id.trim() !== '' ? input.knowledge_id : undefined;
  const knowledgeBaseID = typeof input.knowledge_base_id === 'string' && input.knowledge_base_id.trim() !== '' ? input.knowledge_base_id : undefined;
  let chunkIDs: readonly string[] = [];
  if (input.chunk_ids !== undefined) {
    if (!Array.isArray(input.chunk_ids)) throw new Error('CITATION_CHUNK_IDS_INVALID');
    chunkIDs = input.chunk_ids.map((chunk) => nonEmpty(chunk, 'CITATION', 'chunk_ids'));
  }
  if (documentID === '' || (knowledgeID === undefined && chunkIDs.length === 0)) {
    throw new Error('CITATION_REQUIRES_AUTHORIZED_IDS');
  }
  const snippet = bounded(input.snippet ?? input.content, 'CITATION', 'snippet', 500);
  return {
    documentID,
    ...(knowledgeID ? { knowledgeID } : {}),
    ...(knowledgeBaseID ? { knowledgeBaseID } : {}),
    chunkIDs,
    title: bounded(input.title ?? input.knowledge_title ?? input.name, 'CITATION', 'title', 200) ?? '知识引用',
    ...(snippet !== undefined ? { snippet } : {}),
  };
}

export type AnalysisTableCell = string | number | boolean | null;

export interface AnalysisTableData {
  columns: readonly string[];
  rows: readonly AnalysisTableCell[][];
}

export type AnalysisFileFallbackReason = 'TOO_MANY_COLUMNS' | 'TOO_MANY_ROWS' | 'CELL_TOO_LARGE';

export type AnalysisResultResolution =
  | { renderer: 'table'; table: AnalysisTableData }
  | { renderer: 'file'; reason: AnalysisFileFallbackReason; columns: number; rows: number; file?: ArtifactFileData }
  | { renderer: 'text'; reason: 'INVALID_TABLE_PAYLOAD' | 'INVALID_CELL_TYPE' };

function tableCell(value: unknown): AnalysisTableCell | undefined {
  if (value === null || typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value;
  return undefined;
}

/**
 * Validates a structured analysis table. Shape/cell-type failures degrade to
 * safe text; size overruns degrade to the file (download) renderer so a huge
 * table never renders as an unbounded interactive grid.
 */
export function resolveAnalysisResult(value: unknown, limits: typeof ANALYSIS_TABLE_LIMITS = ANALYSIS_TABLE_LIMITS): AnalysisResultResolution {
  // 渲染期安全回退：任何不是对象的 payload（字符串、数组、null……）都不
  // 抛给调用方，而是降级为安全文本，绝不让损坏 payload 打断会话渲染。
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return { renderer: 'text', reason: 'INVALID_TABLE_PAYLOAD' };
  }
  const input = value as Record<string, unknown>;
  // 附件回退：data-analysis 结果可同时携带其来源附件引用，超限时下载走
  // 同一个授权产品对象（W25 附件/知识接口），而不是重新发明下载入口。
  const artifact = input.artifact ?? input.file ?? input.attachment;
  const artifactFile = artifact === undefined ? undefined : (() => { try { return parseArtifactFile(artifact); } catch { return undefined; } })();
  if (!Array.isArray(input.columns) || input.columns.length === 0 || !Array.isArray(input.rows)) {
    return { renderer: 'text', reason: 'INVALID_TABLE_PAYLOAD' };
  }
  if (input.columns.length > limits.maxColumns) {
    return { renderer: 'file', reason: 'TOO_MANY_COLUMNS', columns: input.columns.length, rows: input.rows.length, ...(artifactFile ? { file: artifactFile } : {}) };
  }
  const columns = input.columns.map((column) => (typeof column === 'string' && column.trim() !== '' ? column : undefined));
  if (columns.some((column) => column === undefined)) return { renderer: 'text', reason: 'INVALID_TABLE_PAYLOAD' };
  if (input.rows.length > limits.maxRows) {
    return { renderer: 'file', reason: 'TOO_MANY_ROWS', columns: columns.length, rows: input.rows.length, ...(artifactFile ? { file: artifactFile } : {}) };
  }
  const rows: AnalysisTableCell[][] = [];
  for (const rawRow of input.rows) {
    if (!Array.isArray(rawRow) || rawRow.length !== columns.length) return { renderer: 'text', reason: 'INVALID_TABLE_PAYLOAD' };
    const cells: AnalysisTableCell[] = [];
    for (const rawCell of rawRow) {
      if (typeof rawCell === 'string' && rawCell.length > limits.maxCellLength) {
        return { renderer: 'file', reason: 'CELL_TOO_LARGE', columns: columns.length, rows: input.rows.length, ...(artifactFile ? { file: artifactFile } : {}) };
      }
      const cell = tableCell(rawCell);
      if (cell === undefined) return { renderer: 'text', reason: 'INVALID_CELL_TYPE' };
      cells.push(cell);
    }
    rows.push(cells);
  }
  return { renderer: 'table', table: { columns: columns as readonly string[], rows } };
}

export interface ArtifactFileData {
  name: string;
  mime: string;
  bytes: number;
  /** 产品对象引用（attachment/knowledge document id）——绝不是 URL。 */
  ref: string;
}

export function parseArtifactFile(value: unknown): ArtifactFileData {
  const input = row(value, 'ARTIFACT');
  const name = typeof input.name === 'string' && input.name.trim() !== '' ? input.name
    : typeof input.filename === 'string' && input.filename.trim() !== '' ? input.filename
      : (() => { throw new Error('ARTIFACT_PAYLOAD_INVALID'); })();
  const mime = typeof input.mime === 'string' && input.mime.trim() !== '' ? input.mime : 'application/octet-stream';
  const bytes = input.bytes;
  if (typeof bytes !== 'number' || !Number.isSafeInteger(bytes) || bytes < 0) throw new Error('ARTIFACT_PAYLOAD_INVALID');
  const refSource = [input.ref, input.attachment_id, input.document_id].find((candidate) => typeof candidate === 'string' && candidate.trim() !== '');
  if (refSource === undefined) throw new Error('ARTIFACT_PAYLOAD_INVALID');
  const ref = refSource as string;
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(ref)) throw new Error('ARTIFACT_REF_MUST_BE_PRODUCT_OBJECT_ID');
  return { name, mime, bytes, ref };
}

export type StructuredResultRouting =
  | { renderer: 'citation'; citation: KnowledgeCitationData }
  | { renderer: 'table'; analysis: Extract<AnalysisResultResolution, { renderer: 'table' }> }
  | { renderer: 'file'; analysis: Extract<AnalysisResultResolution, { renderer: 'file' }> }
  | { renderer: 'file'; file: ArtifactFileData }
  | { renderer: 'text'; reason: string };

/**
 * Routes one structured envelope through the registry: `selectRenderer` stays
 * the single kind→renderer mapping, then the payload is validated for that
 * renderer. Corrupt payloads and unknown kinds both land on safe text — the
 * caller never receives a shell card for an unrenderable result.
 */
export function routeStructuredResult(envelope: StructuredResultEnvelope): StructuredResultRouting {
  switch (selectRenderer(envelope.type)) {
    case 'citation':
      try {
        return { renderer: 'citation', citation: parseKnowledgeCitation(envelope.data) };
      } catch {
        return { renderer: 'text', reason: 'INVALID_CITATION_PAYLOAD' };
      }
    case 'table': {
      const analysis = resolveAnalysisResult(envelope.data);
      if (analysis.renderer === 'text') return { renderer: 'text', reason: analysis.reason };
      return { renderer: analysis.renderer, analysis } as StructuredResultRouting;
    }
    case 'file':
      try {
        return { renderer: 'file', file: parseArtifactFile(envelope.data) };
      } catch {
        return { renderer: 'text', reason: 'INVALID_ARTIFACT_PAYLOAD' };
      }
    default:
      return { renderer: 'text', reason: 'UNKNOWN_RESULT_TYPE' };
  }
}
