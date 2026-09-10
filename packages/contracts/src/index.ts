export class ContractError extends Error {
  readonly path: string;

  constructor(path: string, message: string) {
    super(`${path}: ${message}`);
    this.name = 'ContractError';
    this.path = path;
  }
}

export interface ApiErrorPayload {
  code: string;
  message: string;
  requestId?: string;
  details?: unknown;
}

export interface KnowledgeBase {
  id: string;
  name: string;
  type?: string;
  tenant_id?: string | number;
  [key: string]: unknown;
}

export interface KnowledgeBaseListResponse {
  success: true;
  data: KnowledgeBase[];
}

export interface ChatSession {
  id: string;
  title: string;
  description?: string;
  is_pinned: boolean;
  created_at?: string;
  updated_at?: string;
  [key: string]: unknown;
}

export interface ChatSessionListResponse {
  data: ChatSession[];
  total: number;
  page: number;
  page_size: number;
}

export interface ChatMessage {
  id: string;
  session_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  created_at?: string;
  updated_at?: string;
  is_completed?: boolean;
  [key: string]: unknown;
}

export type { ChatResponseType, ChatStreamEvent } from './chat/events.ts';
export { responseType } from './chat/events.ts';

export type KnowledgeProcessingStatus =
  | 'pending'
  | 'processing'
  | 'finalizing'
  | 'completed'
  | 'failed'
  | 'deleting'
  | 'cancelled';

export interface KnowledgeDocument {
  id: string;
  knowledge_base_id?: string;
  title?: string;
  file_name?: string;
  file_type?: string;
  source?: string;
  parse_status?: KnowledgeProcessingStatus;
  summary_status?: string;
  folder_path?: string;
  [key: string]: unknown;
}

export interface KnowledgeDocumentListResponse {
  success: true;
  data: KnowledgeDocument[];
  total: number;
  page: number;
  page_size: number;
}

function requireNonEmptyString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(path, 'expected a non-empty string');
  }
  return value;
}

function requiredString(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new ContractError(path, 'expected a string');
  return value;
}

export function parseApiErrorPayload(value: unknown): ApiErrorPayload {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const code = requireNonEmptyString(row.code, 'code');
  const message = requireNonEmptyString(row.message, 'message');
  const requestId = row.requestId ?? row.request_id;
  if (requestId !== undefined && typeof requestId !== 'string') {
    throw new ContractError('requestId', 'expected a string');
  }
  return {
    code,
    message,
    ...(requestId === undefined ? {} : { requestId }),
    ...(row.details === undefined ? {} : { details: row.details }),
  };
}

function validateTenantId(value: unknown, path: string): string | number {
  if (typeof value === 'string' && value.trim() !== '') return value;
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return value;
  throw new ContractError(path, 'expected a decimal string or safe non-negative integer');
}

export function parseKnowledgeBaseListResponse(value: unknown): KnowledgeBase[] {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Array.isArray(envelope.data)) throw new ContractError('data', 'expected an array');

  return envelope.data.map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) {
      throw new ContractError(path, 'expected an object');
    }
    const row = item as Record<string, unknown>;
    const result: KnowledgeBase = {
      ...row,
      id: requireNonEmptyString(row.id, `${path}.id`),
      name: requireNonEmptyString(row.name, `${path}.name`),
    };
    if ('tenant_id' in row && row.tenant_id !== undefined && row.tenant_id !== null) {
      result.tenant_id = validateTenantId(row.tenant_id, `${path}.tenant_id`);
    }
    if ('type' in row && row.type !== undefined && typeof row.type !== 'string') {
      throw new ContractError(`${path}.type`, 'expected a string');
    }
    return result;
  });
}

export function parseKnowledgeBaseResponse(value: unknown): KnowledgeBase {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (envelope.data === undefined) throw new ContractError('data', 'expected an object');
  return parseKnowledgeBaseListResponse({ success: true, data: [envelope.data] })[0]!;
}

function validatePageNumber(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ContractError(path, 'expected a non-negative integer');
  }
  return value;
}

function optionalString(value: unknown, path: string): string | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value !== 'string') throw new ContractError(path, 'expected a string');
  return value;
}

function envelope(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const result = value as Record<string, unknown>;
  if (result.success !== true) throw new ContractError('success', 'expected true');
  if (!Array.isArray(result.data)) throw new ContractError('data', 'expected an array');
  return result;
}

function parseChatSession(value: unknown, path: string): ChatSession {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  const row = value as Record<string, unknown>;
  if (typeof row.is_pinned !== 'boolean') throw new ContractError(`${path}.is_pinned`, 'expected a boolean');
  const description = optionalString(row.description, `${path}.description`);
  const createdAt = optionalString(row.created_at, `${path}.created_at`);
  const updatedAt = optionalString(row.updated_at, `${path}.updated_at`);
  return {
    ...row,
    id: requireNonEmptyString(row.id, `${path}.id`),
    title: requiredString(row.title, `${path}.title`),
    is_pinned: row.is_pinned,
    ...(description === undefined ? {} : { description }),
    ...(createdAt === undefined ? {} : { created_at: createdAt }),
    ...(updatedAt === undefined ? {} : { updated_at: updatedAt }),
  } as ChatSession;
}

export function parseChatSessionListResponse(value: unknown): ChatSessionListResponse {
  const result = envelope(value);
  const data = (result.data as unknown[]).map((item, index) => parseChatSession(item, `data[${index}]`));
  return {
    data,
    total: validatePageNumber(result.total, 'total'),
    page: validatePageNumber(result.page, 'page'),
    page_size: validatePageNumber(result.page_size, 'page_size'),
  };
}

export function parseChatSessionResponse(value: unknown): ChatSession {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const result = value as Record<string, unknown>;
  if (result.success !== true) throw new ContractError('success', 'expected true');
  return parseChatSession(result.data, 'data');
}

export function parseChatMessageListResponse(value: unknown): ChatMessage[] {
  const result = envelope(value);
  return (result.data as unknown[]).map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) {
      throw new ContractError(path, 'expected an object');
    }
    const row = item as Record<string, unknown>;
    const role = requiredString(row.role, `${path}.role`);
    if (role !== 'user' && role !== 'assistant' && role !== 'system') {
      throw new ContractError(`${path}.role`, 'expected user, assistant, or system');
    }
    if (row.is_completed !== undefined && typeof row.is_completed !== 'boolean') {
      throw new ContractError(`${path}.is_completed`, 'expected a boolean');
    }
    const createdAt = optionalString(row.created_at, `${path}.created_at`);
    const updatedAt = optionalString(row.updated_at, `${path}.updated_at`);
    return {
      ...row,
      id: requireNonEmptyString(row.id, `${path}.id`),
      session_id: requireNonEmptyString(row.session_id, `${path}.session_id`),
      role,
      content: requiredString(row.content, `${path}.content`),
      ...(createdAt === undefined ? {} : { created_at: createdAt }),
      ...(updatedAt === undefined ? {} : { updated_at: updatedAt }),
    } as ChatMessage;
  });
}

export function parseKnowledgeDocumentListResponse(value: unknown): KnowledgeDocumentListResponse {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Array.isArray(envelope.data)) throw new ContractError('data', 'expected an array');
  const data = envelope.data.map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) {
      throw new ContractError(path, 'expected an object');
    }
    const row = item as Record<string, unknown>;
    const id = requireNonEmptyString(row.id, `${path}.id`);
    if (row.parse_status !== undefined && typeof row.parse_status !== 'string') {
      throw new ContractError(`${path}.parse_status`, 'expected a string');
    }
    return { ...row, id } as KnowledgeDocument;
  });
  return {
    success: true,
    data,
    total: validatePageNumber(envelope.total, 'total'),
    page: validatePageNumber(envelope.page, 'page'),
    page_size: validatePageNumber(envelope.page_size, 'page_size'),
  };
}
