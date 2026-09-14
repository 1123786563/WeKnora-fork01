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
export const CHAT_ATTACHMENT_MAX_SIZE_BYTES = 50 * 1024 * 1024;
const CHAT_ATTACHMENT_EXTENSIONS = new Set([
  '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.epub', '.mhtml',
  '.txt', '.md', '.csv', '.json', '.xml', '.html', '.markdown', '.yaml', '.yml', '.log',
  '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.tiff', '.webp',
  '.mp3', '.wav', '.m4a', '.flac', '.ogg', '.aac',
]);

export function validateChatAttachment(file: { name: string; size: number }, existingCount: number): 'too-many' | 'too-large' | 'unsupported-type' | undefined {
  if (existingCount >= CHAT_ATTACHMENT_MAX_FILES) return 'too-many';
  if (file.size > CHAT_ATTACHMENT_MAX_SIZE_BYTES) return 'too-large';
  const extension = file.name.lastIndexOf('.') >= 0 ? file.name.slice(file.name.lastIndexOf('.')).toLowerCase() : '';
  return CHAT_ATTACHMENT_EXTENSIONS.has(extension) ? undefined : 'unsupported-type';
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

export function buildWebChatStreamOptions(sessionId: string, content: string, agentId: string | undefined, knowledgeBaseId?: string, attachmentIds?: readonly string[]): WebChatStreamOptions {
  const selected = agentId?.trim();
  const knowledgeBaseIds = knowledgeBaseId?.trim() ? { knowledge_base_ids: [knowledgeBaseId.trim()] } : {};
  const attachmentBody = attachmentIds && attachmentIds.length > 0 ? { attachment_ids: [...attachmentIds] } : {};
  if (!selected) return { sessionId, mode: 'knowledge', body: { query: content, channel: 'web', ...knowledgeBaseIds, ...attachmentBody } };
  return {
    sessionId,
    mode: 'agent',
    body: { query: content, agent_enabled: true, agent_id: selected, channel: 'web', ...knowledgeBaseIds, ...attachmentBody },
  };
}
