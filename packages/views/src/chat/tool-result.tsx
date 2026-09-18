import * as React from 'react';

import { normalizeToolResult, type NormalizedToolResult } from '@weknora/domain/chat/tool-results';
import { BrowserToolDetails } from './browser-tool-details.tsx';
import { browserToolTitle } from './browser-tool-display.ts';
import { CHAT_COPY, formatChatCopy, type ChatCopyTable } from './chat-copy.ts';

export interface ToolResultViewInput {
  id: string;
  name?: string;
  result?: unknown;
}

export interface ToolResultPresentation {
  renderer: NormalizedToolResult['renderer'];
  title: string;
  text: string;
  contentMode: NormalizedToolResult['contentMode'];
  /** Secret-redacted structured payload for the typed renderers. */
  data: Record<string, unknown>;
  toolName?: string;
}

const TITLE_KEYS: Readonly<Record<NormalizedToolResult['renderer'], keyof ChatCopyTable>> = Object.freeze({
  'search-results': 'toolTitleSearchResults', 'chunk-detail': 'toolTitleChunkDetail', 'related-chunks': 'toolTitleRelatedChunks',
  'knowledge-base-list': 'toolTitleKnowledgeBaseList', 'document-info': 'toolTitleDocumentInfo', 'graph-query-results': 'toolTitleGraphQueryResults',
  thinking: 'toolTitleThinking', plan: 'toolTitlePlan', 'database-query': 'toolTitleDatabaseQuery', 'web-search-results': 'toolTitleWebSearchResults',
  'web-fetch-results': 'toolTitleWebFetchResults', 'grep-results': 'toolTitleGrepResults', 'knowledge-chunks-list': 'toolTitleKnowledgeChunksList',
  'wiki-edit': 'toolTitleWikiEdit', 'shell-exec': 'toolTitleShellExec', 'sandbox-files': 'toolTitleSandboxFiles',
  'sandbox-file-write': 'toolTitleSandboxFileWrite', 'read-skill': 'toolTitleReadSkill', 'mcp-discovery': 'toolTitleMcpDiscovery',
  'mcp-call': 'toolTitleMcpCall', 'plain-text': 'toolTitlePlainText',
});
const LEGACY_TITLES: Readonly<Record<NormalizedToolResult['renderer'], string>> = Object.freeze({
  'search-results': 'Search results', 'chunk-detail': 'Chunk detail', 'related-chunks': 'Related chunks',
  'knowledge-base-list': 'Knowledge bases', 'document-info': 'Document info', 'graph-query-results': 'Graph results',
  thinking: 'Thinking', plan: 'Plan', 'database-query': 'Database query', 'web-search-results': 'Web search results',
  'web-fetch-results': 'Fetched page', 'grep-results': 'Search in files', 'knowledge-chunks-list': 'Knowledge chunks',
  'wiki-edit': 'Wiki update', 'shell-exec': 'Shell command', 'sandbox-files': 'Sandbox files',
  'sandbox-file-write': 'Sandbox file update', 'read-skill': 'Read skill', 'mcp-discovery': 'MCP tools',
  'mcp-call': 'MCP result', 'plain-text': 'Tool result',
});

// Compatibility fallbacks for pure view-model helpers. Renderer-facing labels
// are supplied by ChatCopyTable; these defaults keep direct helper consumers
// stable while older callers are migrated incrementally.
const LABELS = Object.freeze({
  chunkHits: (count: number) => `${count} chunk hits`,
  keywordHits: (count: number) => `${count} keyword hits`,
  noResults: 'No results',
  noRecords: 'No records returned',
  noMatches: 'No matches found',
  nullValue: '(null)',
  emptyOutput: '(no output)',
  binarySuppressed: 'Binary output suppressed',
  workDir: 'work dir',
  exitCode: 'exit code',
  killed: 'killed',
  truncated: 'truncated',
  stdout: 'stdout',
  stderr: 'stderr',
  untitled: 'Untitled document',
});

const SECRET_KEY = /(?:api[_-]?key|app[_-]?secret|access[_-]?token|refresh[_-]?token|client[_-]?secret|password|secret|token)/i;

function record(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function redact(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redact);
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, SECRET_KEY.test(key) ? '[redacted]' : redact(item)]));
  }
  return value;
}

function str(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value);
}

function list(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function records(value: unknown): Record<string, unknown>[] {
  return list(value).map((item) => record(item));
}

function num(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function bool(value: unknown): boolean {
  return value === true || value === 'true';
}

/** Parse tool arguments that may arrive as a JSON string or object. */
function argsRecord(value: unknown): Record<string, unknown> {
  if (typeof value === 'string' && value.trim()) {
    try {
      return record(JSON.parse(value));
    } catch {
      return {};
    }
  }
  return record(value);
}

export function toolResultPresentation(input: ToolResultViewInput, copy?: ChatCopyTable): ToolResultPresentation {
  const safeResult = redact(input.result);
  const result = record(safeResult);
  const normalized = normalizeToolResult({
    toolName: input.name,
    displayType: result.display_type ?? result.displayType,
    data: result.data ?? safeResult,
    output: result.output ?? (typeof safeResult === 'string' ? safeResult : undefined),
    error: result.error,
    success: result.success,
  });
  return {
    renderer: normalized.renderer,
    title: copy ? copy[TITLE_KEYS[normalized.renderer]] : LEGACY_TITLES[normalized.renderer],
    text: normalized.text,
    contentMode: normalized.contentMode,
    data: normalized.data,
    ...(normalized.toolName ? { toolName: normalized.toolName } : {}),
  };
}

/* ------------------------------------------------------------------ */
/* Pure view models (unit-tested without a DOM, consumed by renderers) */
/* ------------------------------------------------------------------ */

export interface SearchResultsRow {
  key: string;
  title: string;
  meta: string;
  snippets: string[];
}

export interface SearchResultsViewModel {
  query: string;
  rows: SearchResultsRow[];
}

/** Group knowledge-search hits per document; FAQ entries stay distinct rows. */
export type SearchResultsCopy = Pick<ChatCopyTable, 'toolChunkHits'>;
const LEGACY_CHUNK_HITS = '{count} chunk hits';
const LEGACY_KEYWORD_HITS = '{count} keyword hits';
function countCopy(template: string, count: number): string { return template.replace('{count}', String(count)); }

export function searchResultsView(data: unknown, copy?: SearchResultsCopy): SearchResultsViewModel {
  const d = record(data);
  const items = records(d.results);
  const grouped = new Map<string, SearchResultsRow>();
  const order: string[] = [];
  for (const item of items) {
    const faqQuestion = str(item.faq_standard_question).trim();
    const key = faqQuestion ? str(item.chunk_id) : str(item.knowledge_id) || str(item.chunk_id);
    if (!grouped.has(key)) {
      grouped.set(key, {
        key,
        title: (faqQuestion || str(item.knowledge_title)) || LABELS.untitled,
        meta: '',
        snippets: [],
      });
      order.push(key);
    }
    grouped.get(key)!.snippets.push(str(item.content));
  }
  for (const key of order) {
    grouped.get(key)!.meta = countCopy(copy?.toolChunkHits ?? LEGACY_CHUNK_HITS, grouped.get(key)!.snippets.length);
  }
  return { query: str(d.query), rows: order.map((k) => grouped.get(k)!) };
}

export interface WebSearchResultRow {
  index: number;
  title: string;
  url: string;
  snippet: string;
  meta: string;
}

export interface WebSearchResultsViewModel {
  query: string;
  rows: WebSearchResultRow[];
}

export function webSearchResultsView(data: unknown): WebSearchResultsViewModel {
  const d = record(data);
  const rows = records(d.results).map((item, i) => ({
    index: num(item.result_index) ?? i + 1,
    title: str(item.title),
    url: str(item.url),
    snippet: str(item.snippet) || str(item.content),
    meta: str(item.published_at) || str(item.age),
  }));
  return { query: str(d.query), rows };
}

export interface DatabaseQueryViewModel {
  columns: string[];
  rows: string[][];
  rowCount: number;
}

function formatCellValue(value: unknown): string {
  if (value === null || value === undefined) return LABELS.nullValue;
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value) ?? '';
    } catch {
      return String(value);
    }
  }
  return String(value);
}

export function databaseQueryView(data: unknown): DatabaseQueryViewModel {
  const d = record(data);
  const columns = list(d.columns).map(str);
  const rows = records(d.rows).map((row) => columns.map((column) => formatCellValue(row[column])));
  return { columns, rows, rowCount: num(d.row_count) ?? rows.length };
}

export interface GrepResultRow {
  key: string;
  title: string;
  meta: string;
  snippet: string;
}

export interface GrepResultsViewModel {
  pattern: string;
  rows: GrepResultRow[];
}

export type GrepResultsCopy = Pick<ChatCopyTable, 'grepTitleMatch' | 'grepFaqEntry' | 'toolChunkHits' | 'toolKeywordHits'>;

function grepKnowledgeMeta(hitCount: number, patternHits: number, titleMatch: boolean, copy?: GrepResultsCopy): string {
  const parts: string[] = [];
  if (hitCount > 0) parts.push(countCopy(copy?.toolChunkHits ?? LEGACY_CHUNK_HITS, hitCount));
  if (patternHits > 0 && patternHits !== hitCount) parts.push(countCopy(copy?.toolKeywordHits ?? LEGACY_KEYWORD_HITS, patternHits));
  if (titleMatch) parts.push(copy?.grepTitleMatch ?? CHAT_COPY.grepTitleMatch);
  return parts.join(' · ');
}

/** Vue agentStream.grepResults.titleMatch is locale-dependent; the optional copy
 * argument keeps the existing pure-function call shape backwards compatible. */
export function grepResultsView(data: unknown, copy?: GrepResultsCopy): GrepResultsViewModel {
  const d = record(data);
  const pattern = str(d.query) || str(list(d.patterns)[0]);
  const chunkRows = records(d.chunk_results);
  if (chunkRows.length) {
    // Vue groups chunk hits by document (FAQ entries remain distinct) and
    // carries title_match across the group before formatting its metadata.
    const grouped = new Map<string, {
      title: string;
      snippet: string;
      hitCount: number;
      titleMatch: boolean;
      isFaq: boolean;
    }>();
    const order: string[] = [];
    for (const item of chunkRows) {
      const isFaq = Boolean(item.faq_id) || str(item.chunk_type) === 'faq';
      const key = isFaq
        ? str(item.faq_id) || str(item.chunk_id)
        : str(item.knowledge_id) || str(item.chunk_id);
      if (!key) continue;
      if (!grouped.has(key)) {
        grouped.set(key, {
          title: str(item.faq_question) || str(item.knowledge_title) || LABELS.untitled,
          snippet: str(item.match_snippet),
          hitCount: 0,
          titleMatch: false,
          isFaq,
        });
        order.push(key);
      }
      const row = grouped.get(key)!;
      row.hitCount += 1;
      row.titleMatch ||= bool(item.title_match);
      if (!row.snippet && str(item.match_snippet)) row.snippet = str(item.match_snippet);
    }
    return {
      pattern,
      rows: order.map((key) => {
        const row = grouped.get(key)!;
        return {
          key,
          title: row.title,
          meta: row.isFaq ? (copy?.grepFaqEntry ?? CHAT_COPY.grepFaqEntry) : grepKnowledgeMeta(row.hitCount, row.hitCount, row.titleMatch, copy),
          snippet: row.snippet,
        };
      }),
    };
  }
  return {
    pattern,
    rows: records(d.knowledge_results).map((item) => ({
      key: str(item.knowledge_id),
      title: str(item.faq_question) || str(item.knowledge_title) || LABELS.untitled,
      meta: grepKnowledgeMeta(num(item.chunk_hit_count) ?? 0, num(item.total_pattern_hits) ?? 0, item.title_match === true, copy),
      snippet: str(item.match_snippet),
    })),
  };
}

export interface ShellExecViewModel {
  command: string;
  workDir: string;
  exitCode: number | null;
  durationLabel: string;
  killed: boolean;
  truncated: boolean;
  stdoutBinary: boolean;
  stderrBinary: boolean;
  stdout: string;
  stderr: string;
  empty: boolean;
}

function durationLabel(ms: number | null): string {
  if (ms === null) return '';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const seconds = ms / 1000;
  if (seconds < 10) return `${seconds.toFixed(1)}s`;
  return `${Math.round(seconds)}s`;
}

export function shellExecView(data: unknown, args?: unknown, output?: unknown): ShellExecViewModel {
  const d = record(data);
  const argumentsRecord = argsRecord(args);
  const stdoutBinary = bool(d.stdout_binary);
  const stderrBinary = bool(d.stderr_binary);
  const stdout = stdoutBinary ? '' : str(d.stdout) || str(output);
  const stderr = stderrBinary ? '' : str(d.stderr);
  return {
    command: str(d.command) || str(argumentsRecord.command),
    workDir: str(d.work_dir) || str(argumentsRecord.work_dir),
    exitCode: num(d.exit_code),
    durationLabel: durationLabel(num(d.duration_ms)),
    killed: bool(d.killed),
    truncated: bool(d.truncated) || bool(d.stdout_truncated) || bool(d.stderr_truncated),
    stdoutBinary,
    stderrBinary,
    stdout,
    stderr,
    empty: !stdout && !stderr && !stdoutBinary && !stderrBinary,
  };
}


export interface ChunkDetailViewModel {
  chunkId: string;
  knowledgeId: string;
  chunkIndexLabel: string;
  contentLength: number | null;
  content: string;
}

/** Chunk detail (Vue ChunkDetail.vue): id/id/position meta plus full content. */
export function chunkDetailView(data: unknown): ChunkDetailViewModel {
  const d = record(data);
  const index = num(d.chunk_index);
  return {
    chunkId: str(d.chunk_id),
    knowledgeId: str(d.knowledge_id),
    chunkIndexLabel: index !== null ? `#${index}` : '',
    contentLength: num(d.content_length),
    content: str(d.content),
  };
}

export interface RelatedChunkRow {
  key: string;
  indexLabel: string;
  positionLabel: string;
  score: number | null;
  content: string;
}

export interface RelatedChunksViewModel {
  rows: RelatedChunkRow[];
}

/** Related chunks (Vue RelatedChunks.vue): per-chunk index/position links. */
export function relatedChunksView(data: unknown): RelatedChunksViewModel {
  const d = record(data);
  return {
    rows: records(d.chunks).map((item, i) => {
      const index = num(item.index);
      const position = num(item.chunk_index);
      return {
        key: str(item.chunk_id) || `chunk-${i}`,
        indexLabel: `#${index ?? i + 1}`,
        positionLabel: position !== null ? `chunk #${position}` : '',
        score: num(item.score),
        content: str(item.content),
      };
    }),
  };
}

export interface KnowledgeBaseRow {
  key: string;
  indexLabel: string;
  name: string;
  id: string;
  description: string;
}

export interface KnowledgeBaseListViewModel {
  count: number;
  rows: KnowledgeBaseRow[];
}

/** Knowledge-base list (Vue KnowledgeBaseList.vue): name/id/description cards. */
export function knowledgeBaseListView(data: unknown): KnowledgeBaseListViewModel {
  const d = record(data);
  const rows = records(d.knowledge_bases);
  return {
    count: num(d.count) ?? rows.length,
    rows: rows.map((item, i) => ({
      key: str(item.id) || `kb-${i}`,
      indexLabel: `#${num(item.index) ?? i + 1}`,
      name: str(item.name) || LABELS.untitled,
      id: str(item.id),
      description: str(item.description),
    })),
  };
}

export interface DocumentInfoMetadataEntry {
  key: string;
  value: string;
}

export interface DocumentInfoRow {
  key: string;
  indexLabel: string;
  title: string;
  description: string;
  sourceLabel: string;
  knowledgeId: string;
  faqId: string;
  chunkCount: number | null;
  faqAnswers: string[];
  fileLabel: string;
  metadata: DocumentInfoMetadataEntry[];
}

export interface DocumentInfoViewModel {
  rows: DocumentInfoRow[];
}

function formatFileSize(size: number | null): string {
  if (size === null || size <= 0) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${unitIndex === 0 || value >= 10 ? Math.round(value) : value.toFixed(1)} ${units[unitIndex]}`;
}

function formatMetadataValue(value: unknown): string {
  if (value === null || value === undefined) return LABELS.nullValue;
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value) ?? String(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

/** Document info (Vue DocumentInfo.vue): per-document metadata cards. */
export function documentInfoView(data: unknown): DocumentInfoViewModel {
  const d = record(data);
  return {
    rows: records(d.documents).map((item, i) => {
      const type = str(item.type);
      const source = str(item.source);
      const fileParts = [
        str(item.file_name),
        str(item.file_type) ? `(${str(item.file_type)})` : '',
        formatFileSize(num(item.file_size)),
      ].filter(Boolean);
      const metadata = record(item.metadata);
      const faqId = str(item.faq_id);
      return {
        key: faqId || str(item.knowledge_id) || `doc-${i}`,
        indexLabel: `#${i + 1}`,
        title: str(item.title) || str(item.faq_question) || LABELS.untitled,
        description: str(item.description),
        sourceLabel: [type, source].filter(Boolean).join(' · '),
        knowledgeId: str(item.knowledge_id),
        faqId,
        chunkCount: num(item.chunk_count),
        faqAnswers: list(item.faq_answers).map(str),
        fileLabel: fileParts.join(' · '),
        metadata: Object.entries(metadata).map(([key, value]) => ({ key, value: formatMetadataValue(value) })),
      };
    }),
  };
}

function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return '';
  }
}

export interface WebFetchRow {
  key: string;
  indexLabel: string;
  url: string;
  hostname: string;
  status: string;
  statusKind: 'ok' | 'failed' | 'skipped' | '';
  method: string;
  errorCode: string;
  errorMessage: string;
  summary: string;
  summaryFailed: boolean;
  rawContent: string;
  contentLengthLabel: string;
  truncated: boolean;
}

export interface WebFetchViewModel {
  rows: WebFetchRow[];
}

/** Web fetch (Vue WebFetchResults.vue): URL/status/summary/raw content cards. */
export function webFetchView(data: unknown): WebFetchViewModel {
  const d = record(data);
  return {
    rows: records(d.results).map((item, i) => {
      const status = str(item.status);
      const rawStatus = str(item.summary_status);
      const summary = str(item.summary);
      const length = num(item.content_length);
      return {
        key: str(item.url) || `fetch-${i}`,
        indexLabel: `#${i + 1}`,
        url: str(item.url),
        hostname: hostnameOf(str(item.url)),
        status,
        statusKind: status === 'success' ? 'ok' : status === 'failed' ? 'failed' : status === 'skipped' ? 'skipped' : '',
        method: str(item.method).toUpperCase(),
        errorCode: str(item.error_code) || (!summary ? str(item.summary_error_code) : ''),
        errorMessage: str(item.error_message) || str(item.error) || (!summary ? str(item.summary_error_message) : ''),
        summary,
        summaryFailed: rawStatus === 'failed',
        rawContent: str(item.raw_content),
        contentLengthLabel: length !== null ? `${length} chars` : '',
        truncated: bool(item.truncated),
      };
    }),
  };
}

/** Thinking (Vue ThinkingDisplay.vue): the reasoning text itself. */
export function thinkingView(data: unknown, output?: unknown): string {
  const d = record(data);
  return str(d.thought) || str(d.content) || str(output);
}

export interface PlanStepRow {
  id: string;
  description: string;
  status: 'pending' | 'in_progress' | 'completed' | 'skipped';
}

export interface PlanViewModel {
  task: string;
  steps: PlanStepRow[];
}

/** Plan (Vue PlanDisplay.vue): ordered steps with status check boxes. */
export function planView(data: unknown): PlanViewModel {
  const d = record(data);
  return {
    task: str(d.task),
    steps: records(d.steps).map((item, i) => {
      const status = str(item.status);
      return {
        id: str(item.id) || `step-${i + 1}`,
        description: str(item.description),
        status: status === 'in_progress' || status === 'completed' || status === 'skipped' ? status : 'pending',
      };
    }),
  };
}

/* ---------------- */

/*
 * chat.css → utilities (Tailwind migration): the typed tool-result
 * renderers carry their recipes inline; the wk-tool-* classes stay as
 * DOM/test hooks. State classes (is-error / is-ok / is-failed /
 * is-stderr / is-in_progress / is-completed) keep their names and map
 * to conditional utilities below.
 */
const MONO = "[font-family:ui-monospace,SFMono-Regular,Menlo,monospace]";
/** .wk-tool-search/web/grep-results list + li recipe (shared). */
const TOOL_RESULT_LIST = "m-0 flex max-h-[14rem] list-none flex-col gap-[0.5rem] overflow-y-auto p-0";
const TOOL_RESULT_ITEM = "flex flex-col gap-[0.15rem] border-l-2 border-l-[#edf0f5] pl-[0.5rem]";
/** .wk-tool-card (shared info-card recipe). */
const TOOL_CARD = "flex flex-col gap-[0.3rem] rounded-[6px] border border-[#e3e8ef] px-[0.7rem] py-[0.5rem]";
const TOOL_ROW_TITLE = "text-[0.8rem] font-semibold text-[#1f2d3d] break-words";
const TOOL_ROW_META = "text-[#8a94a6] text-[0.7rem]";
const TOOL_SNIPPET = "m-0 text-[#4a5568] text-[0.75rem] leading-[1.45] break-words";

function EmptyState({ label }: { label: string }) {
  return <p className="wk-tool-empty m-0 text-[0.78rem] italic text-[#66758b]">{label}</p>;
}

export function SearchResultsRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = searchResultsView(data, labels);
  if (!view.rows.length) return <EmptyState label={labels.toolNoResults} />;
  return (
    <ul className={"wk-tool-search-results " + TOOL_RESULT_LIST}>
      {view.rows.map((row) => (
        <li key={row.key} className={TOOL_RESULT_ITEM}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>{row.title}</div>
          <div className={"wk-tool-row-meta " + TOOL_ROW_META}>{row.meta}</div>
          {row.snippets.map((snippet, i) => <p key={i} className={"wk-tool-snippet " + TOOL_SNIPPET}>{snippet}</p>)}
        </li>
      ))}
    </ul>
  );
}

export function WebSearchResultsRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = webSearchResultsView(data);
  if (!view.rows.length) return <EmptyState label={labels.toolNoResults} />;
  return (
    <ul className={"wk-tool-web-results " + TOOL_RESULT_LIST}>
      {view.rows.map((row) => (
        <li key={`${row.index}-${row.url}`} className={TOOL_RESULT_ITEM}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>
            <span className="wk-tool-row-index mr-[0.35rem] text-[0.7rem] font-semibold text-[#8a94a6] font-mono! text-[0.8rem]! text-muted!">#{row.index}</span>
            {row.url
              ? <a href={row.url} target="_blank" rel="noopener noreferrer" className="text-[#245a9b]">{row.title}</a>
              : row.title}
          </div>
          {row.snippet ? <p className={"wk-tool-snippet " + TOOL_SNIPPET}>{row.snippet}</p> : null}
          {row.meta ? <div className={"wk-tool-row-meta " + TOOL_ROW_META}>{row.meta}</div> : null}
        </li>
      ))}
    </ul>
  );
}

export function DatabaseQueryRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = databaseQueryView(data);
  if (!view.rows.length) return <EmptyState label={labels.toolNoRecords} />;
  return (
    <div className="wk-tool-table-wrap overflow-x-auto rounded-[6px] border border-[#e3e8ef]">
      <table className="wk-tool-table w-full border-collapse text-[0.75rem]">
        <thead><tr>{view.columns.map((c) => <th key={c} className="whitespace-nowrap border-b-2 border-b-[#e3e8ef] bg-[#f6f8fa] px-[0.6rem] py-[0.4rem] text-left font-semibold">{c}</th>)}</tr></thead>
        <tbody>
          {view.rows.map((cells, i) => <tr key={i}>{cells.map((cell, j) => <td key={j} className={i === view.rows.length - 1 ? "max-w-[24rem] overflow-hidden px-[0.6rem] py-[0.4rem] align-top text-ellipsis" : "max-w-[24rem] overflow-hidden border-b border-b-[#edf0f5] px-[0.6rem] py-[0.4rem] align-top text-ellipsis"}>{cell}</td>)}</tr>)}
        </tbody>
      </table>
    </div>
  );
}

export function GrepResultsRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = grepResultsView(data, copy);
  if (!view.rows.length) return <EmptyState label={labels.toolNoMatches} />;
  return (
    <ul className={"wk-tool-grep-results " + TOOL_RESULT_LIST}>
      {view.rows.map((row) => (
        <li key={row.key} className={TOOL_RESULT_ITEM}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>{row.title}</div>
          {row.meta ? <div className={"wk-tool-row-meta " + TOOL_ROW_META}>{row.meta}</div> : null}
          {row.snippet ? <pre className={"wk-tool-snippet-line m-0 rounded-[4px] bg-[#f6f8fa] px-[0.5rem] py-[0.25rem] text-[0.72rem] text-[#24292f] whitespace-pre-wrap break-words " + MONO}>{row.snippet}</pre> : null}
        </li>
      ))}
    </ul>
  );
}

export function ShellExecRenderer({ data, args, output, copy }: { data: unknown; args?: unknown; output?: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = shellExecView(data, args, output);
  return (
    <div className="wk-tool-shell flex flex-col gap-[0.4rem]">
      {view.command ? <pre className={"wk-tool-shell-command m-0 rounded-[6px] bg-[#1f2430] px-[0.7rem] py-[0.5rem] text-[0.75rem] text-[#9ecbff] whitespace-pre-wrap break-words " + MONO}><span aria-hidden="true" className="font-mono! text-[0.8rem]! text-muted!">$ </span>{view.command}</pre> : null}
      {view.workDir || view.exitCode !== null || view.durationLabel || view.killed || view.truncated ? (
        <div className="wk-tool-shell-meta flex flex-wrap gap-x-[0.75rem] gap-y-[0.15rem] text-[0.72rem] text-[#8a94a6]">
          {view.workDir ? <span className="font-mono! text-[0.8rem]! text-muted!">{labels.toolWorkDir}: {view.workDir}</span> : null}
          {view.exitCode !== null ? <span className={`font-mono! text-[0.8rem]! text-muted! ${view.exitCode !== 0 ? 'is-error text-[#c0392b] font-semibold' : ''}`}>{labels.toolExitCode}: {view.exitCode}</span> : null}
          {view.durationLabel ? <span className="font-mono! text-[0.8rem]! text-muted!">{view.durationLabel}</span> : null}
          {view.killed ? <span className="font-mono! text-[0.8rem]! text-muted!">{labels.toolKilled}</span> : null}
          {view.truncated ? <span className="font-mono! text-[0.8rem]! text-muted!">{labels.toolTruncated}</span> : null}
        </div>
      ) : null}
      {view.stdoutBinary || view.stderrBinary ? <EmptyState label={labels.toolBinarySuppressed} /> : null}
      {view.stdout ? (
        <div className="wk-tool-shell-stream overflow-hidden rounded-[6px] border border-[#e3e8ef]">
          <div className="wk-tool-shell-stream-label border-b border-b-[#e3e8ef] bg-[#f6f8fa] px-[0.6rem] py-[0.25rem] text-[0.7rem] font-semibold text-[#52606d]">{labels.toolStdout}</div>
          <pre className={"m-0 max-h-[16rem] overflow-auto bg-white px-[0.7rem] py-[0.5rem] text-[0.72rem] leading-[1.5] text-[#24292f] whitespace-pre-wrap break-words " + MONO}>{view.stdout}</pre>
        </div>
      ) : null}
      {view.stderr ? (
        <div className="wk-tool-shell-stream is-stderr overflow-hidden rounded-[6px] border border-[#e3e8ef]">
          <div className="wk-tool-shell-stream-label border-b border-b-[#e3e8ef] bg-[#f6f8fa] px-[0.6rem] py-[0.25rem] text-[0.7rem] font-semibold text-[#c0392b]">{labels.toolStderr}</div>
          <pre className={"m-0 max-h-[16rem] overflow-auto bg-white px-[0.7rem] py-[0.5rem] text-[0.72rem] leading-[1.5] text-[#24292f] whitespace-pre-wrap break-words " + MONO}>{view.stderr}</pre>
        </div>
      ) : null}
      {view.empty ? <EmptyState label={labels.toolEmptyOutput} /> : null}
    </div>
  );
}

function InfoField({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className="wk-tool-info-field flex gap-[0.6rem] text-[0.75rem] leading-[1.5]"><span className="wk-tool-info-label min-w-[6rem] shrink-0 text-[#52606d] font-medium font-mono! text-[0.8rem]! text-muted!">{label}</span><span className="wk-tool-info-value min-w-0 flex-1 text-[#24292f] break-words font-mono! text-[0.8rem]! text-muted!">{children}</span></div>;
}
/** .wk-tool-info-value code */
function InfoCode({ children }: { children: React.ReactNode }) {
  return <code className={"rounded-[3px] bg-[#f1f5f9] px-[0.3rem] py-[0.1rem] text-[0.7rem] " + MONO}>{children}</code>;
}

export function ChunkDetailRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const view = chunkDetailView(data);
  const labels = copy ?? CHAT_COPY;
  if (!view.chunkId && !view.knowledgeId && !view.content) return <EmptyState label={labels.toolNoRecords} />;
  return (
    <div className="wk-tool-chunk-detail flex flex-col gap-[0.3rem]">
      {view.chunkId ? <InfoField label={labels.chunkIdLabel}><InfoCode>{view.chunkId}</InfoCode></InfoField> : null}
      {view.knowledgeId ? <InfoField label={labels.documentIdLabel}><InfoCode>{view.knowledgeId}</InfoCode></InfoField> : null}
      {view.chunkIndexLabel ? <InfoField label={labels.positionLabel}>{view.chunkIndexLabel}</InfoField> : null}
      {view.contentLength !== null ? <InfoField label={labels.contentLengthLabelSimple}>{formatChatCopy(labels, 'lengthChars', { value: view.contentLength })}</InfoField> : null}
      {view.content ? (
        <div className="wk-tool-section mt-[0.4rem]">
          <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{labels.fullContentLabel}</div>
          <div className="wk-tool-full-content whitespace-pre-wrap break-words text-[0.75rem] leading-[1.55] text-[#24292f]">{view.content}</div>
        </div>
      ) : null}
    </div>
  );
}

export function RelatedChunksRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const view = relatedChunksView(data);
  const labels = copy ?? CHAT_COPY;
  if (!view.rows.length) return <EmptyState label={labels.noRelatedChunks} />;
  return (
    <ul className="wk-tool-related-chunks m-0 flex list-none flex-col gap-[0.4rem] p-0">
      {view.rows.map((row) => (
        <li key={row.key} className={TOOL_RESULT_ITEM}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>
            <span className="wk-tool-row-index mr-[0.35rem] text-[0.7rem] font-semibold text-[#8a94a6] font-mono! text-[0.8rem]! text-muted!">{row.indexLabel}</span>
            {row.positionLabel ? <span className={"wk-tool-row-meta font-mono! text-[0.8rem]! text-muted! " + TOOL_ROW_META}>{row.positionLabel}</span> : null}
            {row.score !== null ? <span className={"wk-tool-row-meta font-mono! text-[0.8rem]! text-muted! " + TOOL_ROW_META}>{labels.toolScore} {row.score.toFixed(3)}</span> : null}
          </div>
          {row.content ? <p className={"wk-tool-snippet " + TOOL_SNIPPET}>{row.content}</p> : null}
        </li>
      ))}
    </ul>
  );
}

export function KnowledgeBaseListRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const view = knowledgeBaseListView(data);
  const labels = copy ?? CHAT_COPY;
  if (!view.rows.length) return <EmptyState label={labels.toolNoResults} />;
  return (
    <div className="wk-tool-kb-list">
      <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{formatChatCopy(labels, 'knowledgeBaseCount', { count: view.count })}</div>
      <ul className="wk-tool-kb-cards m-0 mt-[0.3rem] flex list-none flex-col gap-[0.4rem] p-0">
        {view.rows.map((row) => (
          <li key={row.key} className={"wk-tool-card " + TOOL_CARD}>
            <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>
              <span className="wk-tool-row-index mr-[0.35rem] text-[0.7rem] font-semibold text-[#8a94a6] font-mono! text-[0.8rem]! text-muted!">{row.indexLabel}</span>
              {row.name}
            </div>
            {row.id ? <div className="wk-tool-info-field flex gap-[0.6rem] text-[0.75rem] leading-[1.5]"><span className="wk-tool-info-label min-w-[6rem] shrink-0 text-[#52606d] font-medium font-mono! text-[0.8rem]! text-muted!">{labels.idLabel}</span><span className="wk-tool-info-value min-w-0 flex-1 text-[#24292f] break-words font-mono! text-[0.8rem]! text-muted!"><InfoCode>{row.id}</InfoCode></span></div> : null}
            {row.description ? <p className={"wk-tool-snippet " + TOOL_SNIPPET}>{row.description}</p> : null}
          </li>
        ))}
      </ul>
    </div>
  );
}

export function DocumentInfoRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = documentInfoView(data);
  if (!view.rows.length) return <EmptyState label={labels.toolNoResults} />;
  return (
    <ul className="wk-tool-doc-list m-0 flex list-none flex-col gap-[0.4rem] p-0">
      {view.rows.map((row) => (
        <li key={row.key} className={"wk-tool-card " + TOOL_CARD}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>
            <span className="wk-tool-row-index mr-[0.35rem] text-[0.7rem] font-semibold text-[#8a94a6] font-mono! text-[0.8rem]! text-muted!">{row.indexLabel}</span>
            {row.title}
            {row.chunkCount !== null ? <span className={"wk-tool-row-meta font-mono! text-[0.8rem]! text-muted! " + TOOL_ROW_META}>{row.chunkCount} {labels.toolChunks}</span> : null}
          </div>
          {row.faqId ? <InfoField label={labels.toolFaqId}><InfoCode>{row.faqId}</InfoCode></InfoField> : null}
          {row.knowledgeId ? <InfoField label={labels.toolDocumentId}><InfoCode>{row.knowledgeId}</InfoCode></InfoField> : null}
          {row.faqAnswers.length ? (
            <InfoField label={labels.toolAnswers}>
              <ul className="wk-tool-faq-answers m-0 flex list-none flex-col gap-[0.15rem] p-0">{row.faqAnswers.map((answer, i) => <li key={i}>{answer}</li>)}</ul>
            </InfoField>
          ) : null}
          {row.description ? <InfoField label={labels.toolDescription}>{row.description}</InfoField> : null}
          {row.sourceLabel ? <InfoField label={labels.toolSource}>{row.sourceLabel}</InfoField> : null}
          {row.fileLabel ? <InfoField label={labels.toolFile}>{row.fileLabel}</InfoField> : null}
          {row.metadata.length ? (
            <div className="wk-tool-section mt-[0.4rem]">
              <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{labels.metadataLabel}</div>
              <ul className="wk-tool-metadata-list m-0 flex list-none flex-col gap-[0.1rem] p-0 text-[0.7rem] text-[#52606d]">
                {row.metadata.map((entry) => <li key={entry.key}><span className="wk-tool-metadata-key font-semibold text-[0.8rem]! text-muted! font-mono!">{entry.key}:</span> {entry.value}</li>)}
              </ul>
            </div>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

export function WebFetchRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const view = webFetchView(data);
  const labels = copy ?? CHAT_COPY;
  if (!view.rows.length) return <EmptyState label={labels.toolNoResults} />;
  return (
    <ul className="wk-tool-web-fetch m-0 flex list-none flex-col gap-[0.4rem] p-0">
      {view.rows.map((row) => (
        <li key={row.key} className={"wk-tool-card " + TOOL_CARD}>
          <div className={"wk-tool-row-title " + TOOL_ROW_TITLE}>
            <span className="wk-tool-row-index mr-[0.35rem] text-[0.7rem] font-semibold text-[#8a94a6] font-mono! text-[0.8rem]! text-muted!">{row.indexLabel}</span>
            {row.url
              ? <a href={row.url} target="_blank" rel="noopener noreferrer" className="text-[#245a9b]">{row.hostname || row.url}</a>
              : <span className="font-mono! text-[0.8rem]! text-muted!">{labels.unknownLink}</span>}
            {row.status ? <span className={`wk-tool-status-pill is-${row.statusKind} whitespace-nowrap rounded-[10px] border px-[0.45rem] py-0 text-[0.68rem] leading-[1.5] ${row.statusKind === 'ok' ? 'border-[rgba(7,192,95,0.35)] text-[#0a7d33]' : row.statusKind === 'failed' ? 'border-[rgba(192,57,43,0.35)] text-[#c0392b]' : 'border-[#e3e8ef] text-[#52606d]'} font-mono! text-[0.8rem]! text-muted!`}>{row.status}</span> : null}
            {row.method ? <span className="wk-tool-status-pill whitespace-nowrap rounded-[10px] border border-[#e3e8ef] px-[0.45rem] py-0 text-[0.68rem] leading-[1.5] text-[#52606d] font-mono! text-[0.8rem]! text-muted!">{row.method}</span> : null}
            {row.contentLengthLabel ? <span className={"wk-tool-row-meta font-mono! text-[0.8rem]! text-muted! " + TOOL_ROW_META}>{row.contentLengthLabel}</span> : null}
            {row.truncated ? <span className={"wk-tool-row-meta font-mono! text-[0.8rem]! text-muted! " + TOOL_ROW_META}>{labels.webFetchPartialContent}</span> : null}
          </div>
          {row.url ? <InfoField label={labels.toolUrl}><a href={row.url} target="_blank" rel="noopener noreferrer" className="text-[#245a9b]">{row.url}</a></InfoField> : null}
          {row.errorCode ? <InfoField label={labels.toolErrorCode}>{row.errorCode}</InfoField> : null}
          {row.errorMessage ? <p className={"wk-tool-snippet is-error text-[#c0392b]"}>{row.errorMessage}</p> : null}
          {row.summary ? (
            <div className="wk-tool-section mt-[0.4rem]">
              <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{labels.summaryLabel}</div>
              <div className="wk-tool-full-content whitespace-pre-wrap break-words text-[0.75rem] leading-[1.55] text-[#24292f]">{row.summary}</div>
            </div>
          ) : null}
          {!row.summary && row.summaryFailed ? <div className="wk-tool-section-title is-error mb-[0.2rem] text-[0.72rem] font-semibold text-[#c0392b]">{labels.webFetchSummaryFailed}</div> : null}
          {row.rawContent ? (
            <div className="wk-tool-section mt-[0.4rem]">
              <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{labels.rawTextLabel}{row.contentLengthLabel ? ` (${row.contentLengthLabel})` : ''}</div>
              <pre className={"wk-tool-raw-content m-0 mt-[0.2rem] max-h-[12rem] overflow-auto rounded-[6px] border border-[#e3e8ef] bg-[#f6f8fa] px-[0.6rem] py-[0.4rem] text-[0.7rem] leading-[1.5] whitespace-pre-wrap break-words " + MONO}>{row.rawContent}</pre>
            </div>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

export function ThinkingRenderer({ data, output, copy }: { data: unknown; output?: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const thought = thinkingView(data, output);
  if (!thought) return <EmptyState label={labels.toolEmptyOutput} />;
  return <div className="wk-tool-thinking wk-tool-full-content whitespace-pre-wrap break-words text-[0.75rem] leading-[1.55] text-[#52606d]">{thought}</div>;
}

const PLAN_STATUS_ICONS: Readonly<Record<PlanStepRow['status'], string>> = Object.freeze({
  pending: '○',
  in_progress: '●',
  completed: '✓',
  skipped: '—',
});

export function PlanRenderer({ data, copy }: { data: unknown; copy?: ChatCopyTable }) {
  const labels = copy ?? CHAT_COPY;
  const view = planView(data);
  if (!view.steps.length) return <EmptyState label={labels.toolNoRecords} />;
  return (
    <div className="wk-tool-plan">
      {view.task ? <div className="wk-tool-section-title mb-[0.2rem] text-[0.72rem] font-semibold text-[#52606d]">{view.task}</div> : null}
      <ul className="wk-tool-plan-steps m-0 mt-[0.3rem] flex list-none flex-col gap-[0.15rem] p-0">
        {view.steps.map((step) => (
          <li key={step.id} className={`wk-tool-plan-step is-${step.status} flex items-start gap-[0.4rem] text-[0.75rem] leading-[1.5] text-[#52606d] ${step.status === 'in_progress' ? 'font-medium text-[#24292f]' : ''}`}>
            <span className={`wk-tool-plan-icon w-[1rem] shrink-0 text-center ${step.status === 'completed' || step.status === 'in_progress' ? 'text-[#2563eb]' : ''} font-mono! text-[0.8rem]! text-muted!`} aria-hidden="true">{PLAN_STATUS_ICONS[step.status]}</span>
            <span className={`wk-tool-plan-description ${step.status === 'completed' ? 'text-[#9aa5b1] line-through' : ''} font-mono! text-[0.8rem]! text-muted!`}>{step.description}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function GenericToolResultRenderer({ title, text }: { title: string; text: string }) {
  return <pre>{text}</pre>;
}

const TYPED_RENDERERS: Readonly<Partial<Record<NormalizedToolResult['renderer'], React.ComponentType<{ data: unknown; args?: unknown; output?: unknown; copy?: ChatCopyTable }>>>> = Object.freeze({
  'search-results': SearchResultsRenderer,
  'web-search-results': WebSearchResultsRenderer,
  'database-query': DatabaseQueryRenderer,
  'grep-results': GrepResultsRenderer,
  'shell-exec': ShellExecRenderer,
  'chunk-detail': ChunkDetailRenderer,
  'related-chunks': RelatedChunksRenderer,
  'knowledge-base-list': KnowledgeBaseListRenderer,
  'document-info': DocumentInfoRenderer,
  'web-fetch-results': WebFetchRenderer,
  thinking: ThinkingRenderer,
  plan: PlanRenderer,
});

export function ToolResultView({ toolCall, copy }: { toolCall: ToolResultViewInput; copy?: ChatCopyTable }) {
  const table = copy ?? CHAT_COPY;
  // local_browser renders its own detail surface (upstream BrowserToolDetails).
  if (toolCall.name === 'local_browser') {
    const result = record(toolCall.result);
    const event = {
      arguments: result.arguments,
      output: result.output,
      error: result.error,
      success: result.success === true ? true : result.success === false ? false : undefined,
      tool_data: result.tool_data,
    };
    return <details className="wk-chat-tool-result">
      <summary>{browserToolTitle(table, event)}</summary>
      <BrowserToolDetails event={event} copy={table} />
    </details>;
  }
  const presentation = toolResultPresentation(toolCall, copy ?? CHAT_COPY);
  if (!presentation.text && presentation.renderer === 'plain-text') {
    return <small>{presentation.title}: {(copy ?? CHAT_COPY).toolEmptyOutput}</small>;
  }
  const Typed = TYPED_RENDERERS[presentation.renderer];
  if (Typed) {
    const result = record(toolCall.result);
    return <details className="wk-chat-tool-result">
      <summary>{presentation.title}</summary>
      <Typed data={presentation.data} args={result.arguments} output={str(result.output)} copy={copy} />
    </details>;
  }
  return <details className="wk-chat-tool-result">
    <summary>{presentation.title}</summary>
    <GenericToolResultRenderer title={presentation.title} text={presentation.text} />
  </details>;
}
