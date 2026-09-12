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
  };
}

export function ToolResultView({ toolCall }: { toolCall: ToolResultViewInput }) {
  const presentation = toolResultPresentation(toolCall);
  if (!presentation.text) return <small>{presentation.title}: no output</small>;
  return <details className="wk-chat-tool-result">
    <summary>{presentation.title}</summary>
    <pre>{presentation.text}</pre>
  </details>;
}
