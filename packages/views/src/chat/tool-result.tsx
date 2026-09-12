import * as React from 'react';

import { normalizeToolResult, type NormalizedToolResult } from '@weknora/domain/chat/tool-results';

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

const TITLES: Readonly<Record<NormalizedToolResult['renderer'], string>> = Object.freeze({
  'search-results': 'Search results',
  'chunk-detail': 'Chunk detail',
  'related-chunks': 'Related chunks',
  'knowledge-base-list': 'Knowledge bases',
  'document-info': 'Document info',
  'graph-query-results': 'Graph results',
  thinking: 'Thinking',
  plan: 'Plan',
  'database-query': 'Database query',
  'web-search-results': 'Web search results',
  'web-fetch-results': 'Fetched page',
  'grep-results': 'Search in files',
  'knowledge-chunks-list': 'Knowledge chunks',
  'wiki-edit': 'Wiki update',
  'shell-exec': 'Shell command',
  'sandbox-files': 'Sandbox files',
  'sandbox-file-write': 'Sandbox file update',
  'read-skill': 'Read skill',
  'mcp-discovery': 'MCP tools',
  'mcp-call': 'MCP result',
  'plain-text': 'Tool result',
});

// TODO(migration): local English labels until the strings move to @weknora/i18n
// (packages/i18n is intentionally untouched by the React migration).
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

export function toolResultPresentation(input: ToolResultViewInput): ToolResultPresentation {
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
    title: TITLES[normalized.renderer],
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
export function searchResultsView(data: unknown): SearchResultsViewModel {
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
    grouped.get(key)!.meta = LABELS.chunkHits(grouped.get(key)!.snippets.length);
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

function grepKnowledgeMeta(hitCount: number, patternHits: number, titleMatch: boolean): string {
  const parts: string[] = [];
  if (hitCount > 0) parts.push(LABELS.chunkHits(hitCount));
  if (patternHits > 0 && patternHits !== hitCount) parts.push(LABELS.keywordHits(patternHits));
  if (titleMatch) parts.push('title match');
  return parts.join(' · ');
}

export function grepResultsView(data: unknown): GrepResultsViewModel {
  const d = record(data);
  const pattern = str(d.query) || str(list(d.patterns)[0]);
  const chunkRows = records(d.chunk_results);
  if (chunkRows.length) {
    return {
      pattern,
      rows: chunkRows.map((item) => ({
        key: str(item.chunk_id) || str(item.knowledge_id),
        title: str(item.faq_question) || str(item.knowledge_title) || LABELS.untitled,
        meta: num(item.chunk_index) !== null ? `chunk #${item.chunk_index}` : '',
        snippet: str(item.match_snippet),
      })),
    };
  }
  return {
    pattern,
    rows: records(d.knowledge_results).map((item) => ({
      key: str(item.knowledge_id),
      title: str(item.faq_question) || str(item.knowledge_title) || LABELS.untitled,
      meta: grepKnowledgeMeta(num(item.chunk_hit_count) ?? 0, num(item.total_pattern_hits) ?? 0, item.title_match === true),
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

/* ---------------- */
/* React renderers  */
/* ---------------- */

function EmptyState({ label }: { label: string }) {
  return <p className="wk-tool-empty">{label}</p>;
}

export function SearchResultsRenderer({ data }: { data: unknown }) {
  const view = searchResultsView(data);
  if (!view.rows.length) return <EmptyState label={LABELS.noResults} />;
  return (
    <ul className="wk-tool-search-results">
      {view.rows.map((row) => (
        <li key={row.key}>
          <div className="wk-tool-row-title">{row.title}</div>
          <div className="wk-tool-row-meta">{row.meta}</div>
          {row.snippets.map((snippet, i) => <p key={i} className="wk-tool-snippet">{snippet}</p>)}
        </li>
      ))}
    </ul>
  );
}

export function WebSearchResultsRenderer({ data }: { data: unknown }) {
  const view = webSearchResultsView(data);
  if (!view.rows.length) return <EmptyState label={LABELS.noResults} />;
  return (
    <ul className="wk-tool-web-results">
      {view.rows.map((row) => (
        <li key={`${row.index}-${row.url}`}>
          <div className="wk-tool-row-title">
            <span className="wk-tool-row-index">#{row.index}</span>
            {row.url
              ? <a href={row.url} target="_blank" rel="noopener noreferrer">{row.title}</a>
              : row.title}
          </div>
          {row.snippet ? <p className="wk-tool-snippet">{row.snippet}</p> : null}
          {row.meta ? <div className="wk-tool-row-meta">{row.meta}</div> : null}
        </li>
      ))}
    </ul>
  );
}

export function DatabaseQueryRenderer({ data }: { data: unknown }) {
  const view = databaseQueryView(data);
  if (!view.rows.length) return <EmptyState label={LABELS.noRecords} />;
  return (
    <div className="wk-tool-table-wrap">
      <table className="wk-tool-table">
        <thead><tr>{view.columns.map((c) => <th key={c}>{c}</th>)}</tr></thead>
        <tbody>
          {view.rows.map((cells, i) => <tr key={i}>{cells.map((cell, j) => <td key={j}>{cell}</td>)}</tr>)}
        </tbody>
      </table>
    </div>
  );
}

export function GrepResultsRenderer({ data }: { data: unknown }) {
  const view = grepResultsView(data);
  if (!view.rows.length) return <EmptyState label={LABELS.noMatches} />;
  return (
    <ul className="wk-tool-grep-results">
      {view.rows.map((row) => (
        <li key={row.key}>
          <div className="wk-tool-row-title">{row.title}</div>
          {row.meta ? <div className="wk-tool-row-meta">{row.meta}</div> : null}
          {row.snippet ? <pre className="wk-tool-snippet-line">{row.snippet}</pre> : null}
        </li>
      ))}
    </ul>
  );
}

export function ShellExecRenderer({ data, args, output }: { data: unknown; args?: unknown; output?: unknown }) {
  const view = shellExecView(data, args, output);
  return (
    <div className="wk-tool-shell">
      {view.command ? <pre className="wk-tool-shell-command"><span aria-hidden="true">$ </span>{view.command}</pre> : null}
      {view.workDir || view.exitCode !== null || view.durationLabel || view.killed || view.truncated ? (
        <div className="wk-tool-shell-meta">
          {view.workDir ? <span>{LABELS.workDir}: {view.workDir}</span> : null}
          {view.exitCode !== null ? <span className={view.exitCode !== 0 ? 'is-error' : undefined}>{LABELS.exitCode}: {view.exitCode}</span> : null}
          {view.durationLabel ? <span>{view.durationLabel}</span> : null}
          {view.killed ? <span>{LABELS.killed}</span> : null}
          {view.truncated ? <span>{LABELS.truncated}</span> : null}
        </div>
      ) : null}
      {view.stdoutBinary || view.stderrBinary ? <p className="wk-tool-empty">{LABELS.binarySuppressed}</p> : null}
      {view.stdout ? (
        <div className="wk-tool-shell-stream">
          <div className="wk-tool-shell-stream-label">{LABELS.stdout}</div>
          <pre>{view.stdout}</pre>
        </div>
      ) : null}
      {view.stderr ? (
        <div className="wk-tool-shell-stream is-stderr">
          <div className="wk-tool-shell-stream-label">{LABELS.stderr}</div>
          <pre>{view.stderr}</pre>
        </div>
      ) : null}
      {view.empty ? <EmptyState label={LABELS.emptyOutput} /> : null}
    </div>
  );
}

export function GenericToolResultRenderer({ title, text }: { title: string; text: string }) {
  return <pre>{text}</pre>;
}

const TYPED_RENDERERS: Readonly<Partial<Record<NormalizedToolResult['renderer'], React.ComponentType<{ data: unknown; args?: unknown; output?: unknown }>>>> = Object.freeze({
  'search-results': SearchResultsRenderer,
  'web-search-results': WebSearchResultsRenderer,
  'database-query': DatabaseQueryRenderer,
  'grep-results': GrepResultsRenderer,
  'shell-exec': ShellExecRenderer,
});

export function ToolResultView({ toolCall }: { toolCall: ToolResultViewInput }) {
  const presentation = toolResultPresentation(toolCall);
  if (!presentation.text && presentation.renderer === 'plain-text') {
    return <small>{presentation.title}: no output</small>;
  }
  const Typed = TYPED_RENDERERS[presentation.renderer];
  if (Typed) {
    const result = record(toolCall.result);
    return <details className="wk-chat-tool-result">
      <summary>{presentation.title}</summary>
      <Typed data={presentation.data} args={result.arguments} output={str(result.output)} />
    </details>;
  }
  return <details className="wk-chat-tool-result">
    <summary>{presentation.title}</summary>
    <GenericToolResultRenderer title={presentation.title} text={presentation.text} />
  </details>;
}
