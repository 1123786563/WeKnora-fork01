export interface ChatAgentOption {
  id: string;
  name: string;
}

export interface WebChatStreamOptions {
  sessionId: string;
  mode: 'knowledge' | 'agent';
  body: Record<string, unknown>;
  signal?: AbortSignal;
}

export const CHAT_ATTACHMENT_MAX_FILES = 5;
export const CHAT_ATTACHMENT_DEFAULT_MAX_SIZE_MB = 50;
export interface ChatAttachmentLimits {
  maxFiles: number;
  maxSizeBytes: number;
}

function positiveMegabytes(value: unknown, fallback: number): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

export function resolveChatAttachmentLimits(
  runtime?: { MAX_FILE_SIZE_MB?: unknown },
  buildTimeValue: unknown = import.meta.env?.VITE_MAX_FILE_SIZE_MB,
): ChatAttachmentLimits {
  const runtimeValue = runtime?.MAX_FILE_SIZE_MB
    ?? (typeof window !== 'undefined' ? (window as typeof window & { __RUNTIME_CONFIG__?: { MAX_FILE_SIZE_MB?: unknown } }).__RUNTIME_CONFIG__?.MAX_FILE_SIZE_MB : undefined);
  const megabytes = positiveMegabytes(runtimeValue ?? buildTimeValue, CHAT_ATTACHMENT_DEFAULT_MAX_SIZE_MB);
  return { maxFiles: CHAT_ATTACHMENT_MAX_FILES, maxSizeBytes: megabytes * 1024 * 1024 };
}
export const CHAT_ATTACHMENT_DEFAULT_EXTENSIONS = [
  '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.epub', '.mhtml',
  '.txt', '.md', '.csv', '.json', '.xml', '.html', '.markdown', '.yaml', '.yml', '.log',
  '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.tiff', '.webp',
  '.mp3', '.wav', '.m4a', '.flac', '.ogg', '.aac',
];

export function normalizeChatAttachmentExtensions(fileTypes: readonly unknown[]): string[] {
  return [...new Set(fileTypes
    .filter((value): value is string => typeof value === 'string')
    .map((value) => value.trim().toLowerCase())
    .filter((value) => value && value !== 'url')
    .map((value) => value.startsWith('.') ? value : `.${value}`))];
}

export function mergeChatAttachmentExtensions(fileTypes: readonly unknown[] | undefined): string[] {
  return [...new Set([...CHAT_ATTACHMENT_DEFAULT_EXTENSIONS, ...normalizeChatAttachmentExtensions(fileTypes ?? [])])];
}

export function validateChatAttachment(file: { name: string; size: number }, existingCount: number, limits = resolveChatAttachmentLimits(), supportedExtensions: readonly string[] = CHAT_ATTACHMENT_DEFAULT_EXTENSIONS): 'too-many' | 'too-large' | 'unsupported-type' | undefined {
  if (existingCount >= limits.maxFiles) return 'too-many';
  if (file.size > limits.maxSizeBytes) return 'too-large';
  const extension = file.name.lastIndexOf('.') >= 0 ? file.name.slice(file.name.lastIndexOf('.')).toLowerCase() : '';
  return supportedExtensions.includes(extension) ? undefined : 'unsupported-type';
}

export function shouldPollAttachmentStatus(status: 'uploaded' | 'processing' | 'ready' | 'failed'): boolean {
  return status === 'uploaded' || status === 'processing';
}

export function initialAgentSelection(
  search: string,
  agents: readonly ChatAgentOption[],
  disabledIds: readonly string[],
): string {
  const requested = new URLSearchParams(search).get('agentId')?.trim() ?? '';
  return requested && agents.some((agent) => agent.id === requested) && !disabledIds.includes(requested)
    ? requested
    : '';
}

export interface ChatMentionItem {
  id: string;
  name: string;
  type: 'kb' | 'file' | 'tag' | 'mcp' | 'skill';
  kb_type?: 'document' | 'faq';
  kb_id?: string;
  kb_name?: string;
  skill_name?: string;
}

export function buildWebChatStreamOptions(sessionId: string, content: string, agentId: string | undefined, knowledgeBaseId?: string, attachmentIds?: readonly string[], mentionedItems?: readonly ChatMentionItem[]): WebChatStreamOptions {
  const selected = agentId?.trim();
  const knowledgeBaseIds = knowledgeBaseId?.trim() ? { knowledge_base_ids: [knowledgeBaseId.trim()] } : {};
  const attachmentBody = attachmentIds && attachmentIds.length > 0 ? { attachment_ids: [...attachmentIds] } : {};
  const mentionBody = mentionedItems && mentionedItems.length > 0 ? { mentioned_items: [...mentionedItems] } : {};
  if (!selected) return { sessionId, mode: 'knowledge', body: { query: content, channel: 'web', ...knowledgeBaseIds, ...attachmentBody, ...mentionBody } };
  return {
    sessionId,
    mode: 'agent',
    body: { query: content, agent_enabled: true, agent_id: selected, channel: 'web', ...knowledgeBaseIds, ...attachmentBody, ...mentionBody },
  };
}
