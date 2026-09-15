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

export type { OrderView, CommercialSummary, QuoteView, QuoteInput, CreateOrderInput, RefundInput, RefundView } from './commercial.ts';
export { parseOrderView, parseCommercialSummary, parseQuoteView, parseRefundView } from './commercial.ts';
export type { ConnectionView, InstallationView, SyncBindingView, SyncStatusView, ConnectionState, InstallationState, ConnectionKind, SyncPauseReason, CreateInstallationInput, UpgradeInstallationInput, CreateConnectionInput } from './appconnector.ts';
export { parseConnectionView, parseInstallationView, parseSyncStatusView } from './appconnector.ts';
export type { ActionView, ActionState, ActionDetail, ActionRisk, PrepareActionInput, ApproveActionInput, ExtendTaskBudgetInput, TaskBudgetExtensionResult } from './appconnector.ts';
export { ACTION_STATES, isActionState, parseActionView, parseActionDetail, parseTaskBudgetExtensionResult } from './appconnector.ts';
export type { CraftSessionKind, CraftRunStatus, CraftTerminalRunStatus, CraftCheckStatus, CraftEventKind, CraftSessionCreatedView, CraftSessionSummaryView, CraftSessionPageView, CraftRunView, CraftFileVersionView, CraftVersionCheckView, CraftVersionView, CraftVersionsPageView, CraftWorkspaceRefView, CraftWorkspaceView, CraftInputView, CraftPreviewTicketView, CraftRunEventView, CraftEventPayloadView } from './craft/index.ts';
export { CRAFT_SESSION_KINDS, CRAFT_RUN_STATUSES, CRAFT_TERMINAL_RUN_STATUSES, CRAFT_CHECK_STATUSES, CRAFT_EVENT_KINDS, parseCraftSessionCreated, parseCraftSessionPage, parseCraftRunView, parseCraftVersionView, parseCraftVersionsPage, parseCraftWorkspaceView, parseCraftInputView, parseCraftPreviewTicket, parseCraftRunEvent, parseCraftEventPayload } from './craft/index.ts';

function requireNonEmptyString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(path, 'expected a non-empty string');
  }
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
<<<<<<< HEAD
=======

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
>>>>>>> 7e34e3b03 (feat(workbench): define versioned execution contracts)
