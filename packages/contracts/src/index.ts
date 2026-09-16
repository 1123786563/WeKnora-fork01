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

export type MessageSuggestionStatus = 'generating' | 'ready' | 'suppressed' | 'failed';

export interface MessageSuggestionItem {
  id: string;
  text: string;
  category?: 'clarify' | 'deepen' | 'action' | string;
  source: string;
  knowledge_base_ids?: string[];
}

export interface MessageSuggestionSet {
  id: string;
  session_id: string;
  assistant_message_id: string;
  status: MessageSuggestionStatus;
  allow_regenerate: boolean;
  suppression_reason?: string;
  questions: MessageSuggestionItem[];
  generated_at?: string;
}

export type TemporaryAttachmentStatus = 'uploaded' | 'processing' | 'ready' | 'failed';

export interface TemporaryAttachment {
  id: string;
  session_id: string;
  file_name: string;
  file_type: string;
  file_size: number;
  mime_type?: string;
  status: TemporaryAttachmentStatus;
  token_count?: number;
  chunk_count?: number;
  image_refs?: unknown[];
  error_message?: string;
  expires_at?: string;
  [key: string]: unknown;
}

export interface TemporaryAttachmentListResponse {
  success: true;
  data: TemporaryAttachment[];
}

export interface ActionSuccessResponse {
  success: true;
}

export type SteerDelivery = 'inject' | 'after';

export interface SteerQueueItem {
  steer_id: string;
  content: string;
  delivery: SteerDelivery;
  mentioned_items?: unknown[];
}

export interface SteerListResponse extends ActionSuccessResponse {
  assistant_message_id?: string;
  items: SteerQueueItem[];
}

export type SteerMutationResponse =
  | (ActionSuccessResponse & { status: 'new_run' })
  | (ActionSuccessResponse & {
    status: 'queued';
    steer_id: string;
    assistant_message_id: string;
    delivery: SteerDelivery;
  })
  | (ActionSuccessResponse & { status: 'already_injected'; steer_id: string });

export type SteerDeleteResponse =
  | (ActionSuccessResponse & { status: 'gone' })
  | (ActionSuccessResponse & { status: 'deleted'; steer_id: string; removed: boolean })
  | (ActionSuccessResponse & { status: 'already_injected'; steer_id: string; removed: false });

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
  type?: string;
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

export type { OrderView, CommercialSummary, QuoteView, QuoteInput, CreateOrderInput, RefundInput, RefundView } from './commercial.ts';
export { parseOrderView, parseCommercialSummary, parseQuoteView, parseRefundView } from './commercial.ts';
export type { ConnectionView, InstallationView, SyncBindingView, SyncStatusView, ConnectionState, InstallationState, ConnectionKind, SyncPauseReason, CreateInstallationInput, UpgradeInstallationInput, CreateConnectionInput } from './appconnector.ts';
export { parseConnectionView, parseInstallationView, parseSyncStatusView } from './appconnector.ts';
export type { ActionView, ActionState, ActionDetail, ActionRisk, PrepareActionInput, ApproveActionInput, ExtendTaskBudgetInput, TaskBudgetExtensionResult } from './appconnector.ts';
export { ACTION_STATES, isActionState, parseActionView, parseActionDetail, parseTaskBudgetExtensionResult } from './appconnector.ts';
export type { CraftSessionKind, CraftRunStatus, CraftTerminalRunStatus, CraftCheckStatus, CraftEventKind, CraftSessionCreatedView, CraftSessionSummaryView, CraftSessionPageView, CraftRunView, CraftFileVersionView, CraftVersionCheckView, CraftVersionView, CraftVersionsPageView, CraftWorkspaceRefView, CraftWorkspaceView, CraftInputView, CraftPreviewTicketView, CraftRunEventView, CraftEventPayloadView } from './craft/index.ts';
export { CRAFT_SESSION_KINDS, CRAFT_RUN_STATUSES, CRAFT_TERMINAL_RUN_STATUSES, CRAFT_CHECK_STATUSES, CRAFT_EVENT_KINDS, parseCraftSessionCreated, parseCraftSessionPage, parseCraftRunView, parseCraftVersionView, parseCraftVersionsPage, parseCraftWorkspaceView, parseCraftInputView, parseCraftPreviewTicket, parseCraftRunEvent, parseCraftEventPayload } from './craft/index.ts';

export interface KnowledgeFolderNode {
  path: string;
  name: string;
  document_count: number;
  total_count: number;
  children?: KnowledgeFolderNode[];
}

export interface KnowledgeFolderTree {
  root_document_count: number;
  total_document_count: number;
  folders: KnowledgeFolderNode[];
}

export interface KnowledgeTag {
  id: string;
  seq_id?: number;
  knowledge_base_id?: string;
  name: string;
  color?: string;
  sort_order?: number;
  knowledge_count?: number;
  chunk_count?: number;
  [key: string]: unknown;
}

export interface KnowledgeTagListResponse {
  success: true;
  data: KnowledgeTag[];
  total?: number;
  page?: number;
  page_size?: number;
}

export interface KnowledgeSearchResponse {
  success: true;
  data: KnowledgeDocument[];
  has_more: boolean;
  total: number;
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

function actionEnvelope(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const result = value as Record<string, unknown>;
  if (result.success !== true) throw new ContractError('success', 'expected true');
  return result;
}

function parseSteerDelivery(value: unknown, path: string): SteerDelivery {
  if (value !== 'inject' && value !== 'after') {
    throw new ContractError(path, 'expected inject or after');
  }
  return value;
}

export function parseActionSuccessResponse(value: unknown): ActionSuccessResponse {
  actionEnvelope(value);
  return { success: true };
}

export function parseSteerMutationResponse(value: unknown): SteerMutationResponse {
  const result = actionEnvelope(value);
  if (result.status === 'new_run') return { success: true, status: 'new_run' };
  if (result.status === 'already_injected') {
    return {
      success: true,
      status: 'already_injected',
      steer_id: requireNonEmptyString(result.steer_id, 'steer_id'),
    };
  }
  if (result.status === 'queued') {
    return {
      success: true,
      status: 'queued',
      steer_id: requireNonEmptyString(result.steer_id, 'steer_id'),
      assistant_message_id: requireNonEmptyString(result.assistant_message_id, 'assistant_message_id'),
      delivery: parseSteerDelivery(result.delivery, 'delivery'),
    };
  }
  throw new ContractError('status', 'expected queued, new_run, or already_injected');
}

export function parseSteerListResponse(value: unknown): SteerListResponse {
  const result = actionEnvelope(value);
  if (!Array.isArray(result.items)) throw new ContractError('items', 'expected an array');
  const assistantMessageId = result.assistant_message_id === undefined
    ? undefined
    : requireNonEmptyString(result.assistant_message_id, 'assistant_message_id');
  const items = result.items.map((value, index) => {
    const path = `items[${index}]`;
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
      throw new ContractError(path, 'expected an object');
    }
    const item = value as Record<string, unknown>;
    if (item.mentioned_items !== undefined && !Array.isArray(item.mentioned_items)) {
      throw new ContractError(`${path}.mentioned_items`, 'expected an array');
    }
    return {
      steer_id: requireNonEmptyString(item.steer_id, `${path}.steer_id`),
      content: requireNonEmptyString(item.content, `${path}.content`),
      delivery: parseSteerDelivery(item.delivery, `${path}.delivery`),
      ...(item.mentioned_items === undefined ? {} : { mentioned_items: item.mentioned_items }),
    };
  });
  return {
    success: true,
    ...(assistantMessageId === undefined ? {} : { assistant_message_id: assistantMessageId }),
    items,
  };
}

export function parseSteerDeleteResponse(value: unknown): SteerDeleteResponse {
  const result = actionEnvelope(value);
  if (result.status === 'gone') return { success: true, status: 'gone' };
  if (result.status === 'already_injected') {
    if (result.removed !== false) throw new ContractError('removed', 'expected false');
    return {
      success: true,
      status: 'already_injected',
      steer_id: requireNonEmptyString(result.steer_id, 'steer_id'),
      removed: false,
    };
  }
  if (result.status === 'deleted') {
    if (typeof result.removed !== 'boolean') throw new ContractError('removed', 'expected a boolean');
    return {
      success: true,
      status: 'deleted',
      steer_id: requireNonEmptyString(result.steer_id, 'steer_id'),
      removed: result.removed,
    };
  }
  throw new ContractError('status', 'expected deleted, gone, or already_injected');
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

function parseMessageSuggestionItem(value: unknown, path: string): MessageSuggestionItem {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  const row = value as Record<string, unknown>;
  const category = optionalString(row.category, `${path}.category`);
  const knowledgeBaseIds = row.knowledge_base_ids === undefined || row.knowledge_base_ids === null
    ? undefined
    : (() => {
      if (!Array.isArray(row.knowledge_base_ids)) throw new ContractError(`${path}.knowledge_base_ids`, 'expected an array');
      return row.knowledge_base_ids.map((id, index) => requireNonEmptyString(id, `${path}.knowledge_base_ids[${index}]`));
    })();
  return {
    ...row,
    id: requireNonEmptyString(row.id, `${path}.id`),
    text: requiredString(row.text, `${path}.text`),
    source: requiredString(row.source, `${path}.source`),
    ...(category === undefined ? {} : { category }),
    ...(knowledgeBaseIds === undefined ? {} : { knowledge_base_ids: knowledgeBaseIds }),
  } as MessageSuggestionItem;
}

function parseMessageSuggestionSet(value: unknown, path: string): MessageSuggestionSet {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  const row = value as Record<string, unknown>;
  const status = requiredString(row.status, `${path}.status`);
  if (!['generating', 'ready', 'suppressed', 'failed'].includes(status)) throw new ContractError(`${path}.status`, 'unknown suggestion status');
  if (typeof row.allow_regenerate !== 'boolean') throw new ContractError(`${path}.allow_regenerate`, 'expected a boolean');
  if (!Array.isArray(row.questions)) throw new ContractError(`${path}.questions`, 'expected an array');
  const suppressionReason = optionalString(row.suppression_reason, `${path}.suppression_reason`);
  const generatedAt = optionalString(row.generated_at, `${path}.generated_at`);
  return {
    ...row,
    id: requireNonEmptyString(row.id, `${path}.id`),
    session_id: requireNonEmptyString(row.session_id, `${path}.session_id`),
    assistant_message_id: requireNonEmptyString(row.assistant_message_id, `${path}.assistant_message_id`),
    status: status as MessageSuggestionStatus,
    allow_regenerate: row.allow_regenerate,
    questions: row.questions.map((item, index) => parseMessageSuggestionItem(item, `${path}.questions[${index}]`)),
    ...(suppressionReason === undefined ? {} : { suppression_reason: suppressionReason }),
    ...(generatedAt === undefined ? {} : { generated_at: generatedAt }),
  };
}

export function parseMessageSuggestionResponse(value: unknown): MessageSuggestionSet {
  const result = actionEnvelope(value);
  return parseMessageSuggestionSet(result.data, 'data');
}

function parseTemporaryAttachment(value: unknown, path: string): TemporaryAttachment {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  const row = value as Record<string, unknown>;
  const status = requiredString(row.status, `${path}.status`);
  if (!['uploaded', 'processing', 'ready', 'failed'].includes(status)) throw new ContractError(`${path}.status`, 'unknown attachment status');
  if (typeof row.file_size !== 'number' || !Number.isFinite(row.file_size) || row.file_size < 0) throw new ContractError(`${path}.file_size`, 'expected a non-negative number');
  return {
    ...row,
    id: requireNonEmptyString(row.id, `${path}.id`),
    session_id: requireNonEmptyString(row.session_id, `${path}.session_id`),
    file_name: requireNonEmptyString(row.file_name, `${path}.file_name`),
    file_type: requiredString(row.file_type, `${path}.file_type`),
    file_size: row.file_size,
    status: status as TemporaryAttachmentStatus,
  } as TemporaryAttachment;
}

export function parseTemporaryAttachmentResponse(value: unknown): TemporaryAttachment {
  const envelope = actionEnvelope(value);
  return parseTemporaryAttachment(envelope.data, 'data');
}

export function parseTemporaryAttachmentListResponse(value: unknown): TemporaryAttachmentListResponse {
  const envelope = actionEnvelope(value);
  if (!Array.isArray(envelope.data)) throw new ContractError('data', 'expected an array');
  return { success: true, data: envelope.data.map((item, index) => parseTemporaryAttachment(item, `data[${index}]`)) };
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

export function parseKnowledgeDocumentResponse(value: unknown): KnowledgeDocument {
  const envelope = actionEnvelope(value);
  const row = envelope.data;
  if (typeof row !== 'object' || row === null || Array.isArray(row)) {
    throw new ContractError('data', 'expected an object');
  }
  const record = row as Record<string, unknown>;
  return { ...record, id: requireNonEmptyString(record.id, 'data.id') } as KnowledgeDocument;
}

function parseFolderNode(value: unknown, path: string): KnowledgeFolderNode {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const children = row.children === undefined
    ? undefined
    : (() => {
      if (!Array.isArray(row.children)) throw new ContractError(`${path}.children`, 'expected an array');
      return row.children.map((child, index) => parseFolderNode(child, `${path}.children[${index}]`));
    })();
  return {
    ...row,
    path: requiredString(row.path, `${path}.path`),
    name: requiredString(row.name, `${path}.name`),
    document_count: validatePageNumber(row.document_count, `${path}.document_count`),
    total_count: validatePageNumber(row.total_count, `${path}.total_count`),
    ...(children === undefined ? {} : { children }),
  } as KnowledgeFolderNode;
}

export function parseKnowledgeFolderTreeResponse(value: unknown): KnowledgeFolderTree {
  const envelope = actionEnvelope(value);
  if (typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new ContractError('data', 'expected an object');
  }
  const data = envelope.data as Record<string, unknown>;
  if (!Array.isArray(data.folders)) throw new ContractError('data.folders', 'expected an array');
  return {
    root_document_count: validatePageNumber(data.root_document_count, 'data.root_document_count'),
    total_document_count: validatePageNumber(data.total_document_count, 'data.total_document_count'),
    folders: data.folders.map((folder, index) => parseFolderNode(folder, `data.folders[${index}]`)),
  };
}

export type {
  Capability,
  CapabilityState,
  ExecutionDriver,
  ExecutionDTO,
  ExecutionEvent,
  ExecutionSnapshot,
  RunStatus,
} from './mobile/execution.ts';
export { parseExecution, parseExecutionEvent, parseExecutionSnapshot } from './mobile/execution.ts';

export function parseKnowledgeTagListResponse(value: unknown): KnowledgeTagListResponse {
  const envelope = actionEnvelope(value);
  const payload = envelope.data;
  let rows: unknown[];
  let pagination: { total?: number; page?: number; page_size?: number } = {};
  if (Array.isArray(payload)) {
    rows = payload;
  } else if (typeof payload === 'object' && payload !== null) {
    const paged = payload as Record<string, unknown>;
    if (!Array.isArray(paged.data)) throw new ContractError('data.data', 'expected an array');
    rows = paged.data;
    for (const key of ['total', 'page', 'page_size'] as const) {
      if (paged[key] !== undefined) pagination[key] = validatePageNumber(paged[key], `data.${key}`);
    }
  } else {
    throw new ContractError('data', 'expected an array or paginated object');
  }
  const data = rows.map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) throw new ContractError(path, 'expected an object');
    const row = item as Record<string, unknown>;
    if (row.seq_id !== undefined && typeof row.seq_id !== 'number') throw new ContractError(`${path}.seq_id`, 'expected a number');
    if (row.sort_order !== undefined && typeof row.sort_order !== 'number') throw new ContractError(`${path}.sort_order`, 'expected a number');
    return {
      ...row,
      id: requireNonEmptyString(row.id, `${path}.id`),
      name: requireNonEmptyString(row.name, `${path}.name`),
    } as KnowledgeTag;
  });
  return { success: true, data, ...pagination };
}

export function parseKnowledgeSearchResponse(value: unknown): KnowledgeSearchResponse {
  const envelope = actionEnvelope(value);
  if (!Array.isArray(envelope.data)) throw new ContractError('data', 'expected an array');
  const data = envelope.data.map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) throw new ContractError(path, 'expected an object');
    const row = item as Record<string, unknown>;
    return { ...row, id: requireNonEmptyString(row.id, `${path}.id`) } as KnowledgeDocument;
  });
  if (typeof envelope.has_more !== 'boolean') throw new ContractError('has_more', 'expected a boolean');
  return { success: true, data, has_more: envelope.has_more, total: validatePageNumber(envelope.total, 'total') };
}
