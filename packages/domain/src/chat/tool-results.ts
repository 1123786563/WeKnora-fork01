export const CHAT_MARKDOWN_POLICY = Object.freeze({
  rawHtml: 'escape' as const,
  sanitizeRenderedHtml: true,
});

export type ToolResultRenderer =
  | 'search-results'
  | 'chunk-detail'
  | 'related-chunks'
  | 'knowledge-base-list'
  | 'document-info'
  | 'graph-query-results'
  | 'thinking'
  | 'plan'
  | 'database-query'
  | 'web-search-results'
  | 'web-fetch-results'
  | 'grep-results'
  | 'knowledge-chunks-list'
  | 'wiki-edit'
  | 'shell-exec'
  | 'sandbox-files'
  | 'sandbox-file-write'
  | 'read-skill'
  | 'mcp-discovery'
  | 'mcp-call'
  | 'plain-text';

export interface ToolResultInput {
  displayType?: unknown;
  toolName?: unknown;
  data?: unknown;
  output?: unknown;
  error?: unknown;
  success?: unknown;
}

export interface NormalizedToolResult {
  renderer: ToolResultRenderer;
  displayType?: string;
  toolName?: string;
  success: boolean | null;
  text: string;
  data: Record<string, unknown>;
  /** Render with textContent/a React text child. Never pass this value to an HTML sink. */
  contentMode: 'structured-data' | 'plain-text';
}

const DISPLAY_RENDERERS: Readonly<Record<string, Exclude<ToolResultRenderer, 'plain-text'>>> = Object.freeze({
  search_results: 'search-results',
  chunk_detail: 'chunk-detail',
  related_chunks: 'related-chunks',
  knowledge_base_list: 'knowledge-base-list',
  document_info: 'document-info',
  graph_query_results: 'graph-query-results',
  thinking: 'thinking',
  plan: 'plan',
  database_query: 'database-query',
  web_search_results: 'web-search-results',
  web_fetch_results: 'web-fetch-results',
  grep_results: 'grep-results',
  knowledge_chunks_list: 'knowledge-chunks-list',
  wiki_write_page: 'wiki-edit',
  wiki_replace_text: 'wiki-edit',
  wiki_rename_page: 'wiki-edit',
  wiki_delete_page: 'wiki-edit',
  shell_exec: 'shell-exec',
  list_sandbox_files: 'sandbox-files',
  write_sandbox_file: 'sandbox-file-write',
  edit_sandbox_file: 'sandbox-file-write',
  read_skill: 'read-skill',
  mcp_discovery: 'mcp-discovery',
  mcp_call: 'mcp-call',
});

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function dataRecord(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function readable(value: unknown): string {
  if (typeof value === 'string') return value;
  if (value === undefined || value === null) return '';
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return String(value);
  }
}

function rendererFor(displayType: string, toolName: string): ToolResultRenderer {
  if (displayType && DISPLAY_RENDERERS[displayType]) return DISPLAY_RENDERERS[displayType]!;
  if (toolName === 'discover_mcp_tools') return 'mcp-discovery';
  if (toolName === 'call_mcp_tool') return 'mcp-call';
  return 'plain-text';
}

/** Normalize old history and live SSE results without allowing raw-output HTML. */
export function normalizeToolResult(input: ToolResultInput): NormalizedToolResult {
  const displayType = text(input.displayType);
  const toolName = text(input.toolName);
  const renderer = rendererFor(displayType, toolName);
  const output = readable(input.output);
  const error = readable(input.error);
  const data = dataRecord(input.data);
  const fallbackData = Object.keys(data).length ? readable(data) : '';
  return {
    renderer,
    ...(displayType ? { displayType } : {}),
    ...(toolName ? { toolName } : {}),
    success: typeof input.success === 'boolean' ? input.success : null,
    text: output || error || fallbackData,
    data,
    contentMode: renderer === 'plain-text' ? 'plain-text' : 'structured-data',
  };
}
