// SP13 query-history wire contracts.
// Shapes mirror internal/types/query_history.go (QueryHistorySnapshot /
// SharedSessionSnapshot / QueryHistoryExportJob), internal/types/session.go
// (Session audit rows — reusing the existing ChatSession contract, only
// narrowing the audit columns), internal/types/message.go (Message rows ride
// ChatMessage; knowledge_references passes through), and
// internal/types/message_feedback.go (snapshot feedback rows). The CSV export
// download is a binary response handled by the api-client's requestBinary
// channel, so it has no JSON contract here. Extra backend fields pass through
// via index signatures / spreads per index.ts conventions.
import { ContractError, type ChatMessage, type ChatSession } from './index.ts';

export type QueryHistoryMode = 'normal' | 'anonymized' | 'disabled';

export type QueryHistoryExportState = 'pending' | 'running' | 'done' | 'failed';

export type QueryHistoryFeedbackRating = 'like' | 'dislike';

/**
 * Audit-listing / snapshot session row: the existing ChatSession contract
 * plus the audit columns the query-history views read. user_id is omitempty
 * on the wire (and "anonymous" under the anonymized policy), engine_type /
 * tenant_id / IM origin fields only narrow when present.
 */
export interface QueryHistorySessionRow extends ChatSession {
  user_id?: string;
  engine_type?: string;
}

/** One like/dislike row of the admin snapshot (types.MessageFeedback). */
export interface MessageFeedbackRow {
  id: number;
  message_id: string;
  session_id: string;
  user_id: string;
  rating: QueryHistoryFeedbackRating;
  comment?: string;
  [key: string]: unknown;
}

/** GET /api/v1/admin/sessions/:session_id/snapshot data (Admin+). */
export interface QueryHistorySnapshot {
  session: QueryHistorySessionRow;
  messages: ChatMessage[];
  feedback: MessageFeedbackRow[];
  truncated: boolean;
}

/** GET /api/v1/shared/sessions/:token data — like the snapshot minus feedback. */
export interface SharedSessionSnapshot {
  session: QueryHistorySessionRow;
  messages: ChatMessage[];
  truncated: boolean;
}

/** GET .../export/:job_id/status data; extra job fields (file_path,
 *  requested_by, timestamps) pass through. */
export interface QueryHistoryExportStatus {
  status: QueryHistoryExportState;
  error_message: string;
  job_id?: number;
  [key: string]: unknown;
}

/** GET /api/v1/sessions?source=all audit listing (same envelope as the
 *  ChatSession list). */
export interface QueryHistorySessionListResult {
  data: QueryHistorySessionRow[];
  total: number;
  page: number;
  page_size: number;
}

/** GET/PUT /api/v1/tenants/kv/query-history-config data. */
export interface QueryHistoryConfig {
  mode: QueryHistoryMode;
}

function requiredString(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new ContractError(path, 'expected a string');
  return value;
}

function requireNonEmptyString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(path, 'expected a non-empty string');
  }
  return value;
}

function requiredNonNegativeInteger(value: unknown, path: string): number {
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

// index.ts keeps its envelope helpers private; mirror the local variant here.
function successEnvelope(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const result = value as Record<string, unknown>;
  if (result.success !== true) throw new ContractError('success', 'expected true');
  return result;
}

function parseObject(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  return value as Record<string, unknown>;
}

function parseFeedbackRating(value: unknown, path: string): QueryHistoryFeedbackRating {
  if (value !== 'like' && value !== 'dislike') {
    throw new ContractError(path, 'expected like or dislike');
  }
  return value;
}

function parseQueryHistoryMode(value: unknown, path: string): QueryHistoryMode {
  if (value !== 'normal' && value !== 'anonymized' && value !== 'disabled') {
    throw new ContractError(path, 'expected normal, anonymized, or disabled');
  }
  return value;
}

function parseQueryHistorySessionRow(value: unknown, path: string): QueryHistorySessionRow {
  const row = parseObject(value, path);
  if (typeof row.is_pinned !== 'boolean') throw new ContractError(`${path}.is_pinned`, 'expected a boolean');
  const description = optionalString(row.description, `${path}.description`);
  const createdAt = optionalString(row.created_at, `${path}.created_at`);
  const updatedAt = optionalString(row.updated_at, `${path}.updated_at`);
  const userId = optionalString(row.user_id, `${path}.user_id`);
  const engineType = optionalString(row.engine_type, `${path}.engine_type`);
  return {
    ...row,
    id: requireNonEmptyString(row.id, `${path}.id`),
    title: requiredString(row.title, `${path}.title`),
    is_pinned: row.is_pinned,
    ...(description === undefined ? {} : { description }),
    ...(createdAt === undefined ? {} : { created_at: createdAt }),
    ...(updatedAt === undefined ? {} : { updated_at: updatedAt }),
    ...(userId === undefined ? {} : { user_id: userId }),
    ...(engineType === undefined ? {} : { engine_type: engineType }),
  } as QueryHistorySessionRow;
}

function parseSnapshotMessage(value: unknown, path: string): ChatMessage {
  const row = parseObject(value, path);
  const role = requiredString(row.role, `${path}.role`);
  if (role !== 'user' && role !== 'assistant' && role !== 'system') {
    throw new ContractError(`${path}.role`, 'expected user, assistant, or system');
  }
  if (row.knowledge_references !== undefined && row.knowledge_references !== null && !Array.isArray(row.knowledge_references)) {
    throw new ContractError(`${path}.knowledge_references`, 'expected an array');
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
}

function parseMessageFeedbackRow(value: unknown, path: string): MessageFeedbackRow {
  const row = parseObject(value, path);
  const comment = row.comment === undefined || row.comment === null
    ? undefined
    : requiredString(row.comment, `${path}.comment`);
  return {
    ...row,
    id: requiredNonNegativeInteger(row.id, `${path}.id`),
    message_id: requireNonEmptyString(row.message_id, `${path}.message_id`),
    session_id: requiredString(row.session_id, `${path}.session_id`),
    user_id: requiredString(row.user_id, `${path}.user_id`),
    rating: parseFeedbackRating(row.rating, `${path}.rating`),
    ...(comment === undefined ? {} : { comment }),
  } as MessageFeedbackRow;
}

export function parseQueryHistorySessionListResponse(value: unknown): QueryHistorySessionListResult {
  const result = successEnvelope(value);
  if (!Array.isArray(result.data)) throw new ContractError('data', 'expected an array');
  return {
    data: result.data.map((item, index) => parseQueryHistorySessionRow(item, `data[${index}]`)),
    total: requiredNonNegativeInteger(result.total, 'total'),
    page: requiredNonNegativeInteger(result.page, 'page'),
    page_size: requiredNonNegativeInteger(result.page_size, 'page_size'),
  };
}

export function parseQueryHistorySnapshotResponse(value: unknown): QueryHistorySnapshot {
  const result = successEnvelope(value);
  const data = parseObject(result.data, 'data');
  if (!Array.isArray(data.messages)) throw new ContractError('data.messages', 'expected an array');
  if (!Array.isArray(data.feedback)) throw new ContractError('data.feedback', 'expected an array');
  if (typeof data.truncated !== 'boolean') throw new ContractError('data.truncated', 'expected a boolean');
  return {
    session: parseQueryHistorySessionRow(data.session, 'data.session'),
    messages: data.messages.map((item, index) => parseSnapshotMessage(item, `data.messages[${index}]`)),
    feedback: data.feedback.map((item, index) => parseMessageFeedbackRow(item, `data.feedback[${index}]`)),
    truncated: data.truncated,
  };
}

export function parseSharedSessionResponse(value: unknown): SharedSessionSnapshot {
  const result = successEnvelope(value);
  const data = parseObject(result.data, 'data');
  if (!Array.isArray(data.messages)) throw new ContractError('data.messages', 'expected an array');
  if (typeof data.truncated !== 'boolean') throw new ContractError('data.truncated', 'expected a boolean');
  return {
    session: parseQueryHistorySessionRow(data.session, 'data.session'),
    messages: data.messages.map((item, index) => parseSnapshotMessage(item, `data.messages[${index}]`)),
    truncated: data.truncated,
  };
}

export function parseQueryHistoryExportStartResponse(value: unknown): { job_id: number } {
  const data = parseObject(successEnvelope(value).data, 'data');
  return { job_id: requiredNonNegativeInteger(data.job_id, 'data.job_id') };
}

export function parseQueryHistoryExportStatusResponse(value: unknown): QueryHistoryExportStatus {
  const data = parseObject(successEnvelope(value).data, 'data');
  const status = requiredString(data.status, 'data.status');
  if (status !== 'pending' && status !== 'running' && status !== 'done' && status !== 'failed') {
    throw new ContractError('data.status', 'expected pending, running, done, or failed');
  }
  return {
    ...data,
    status,
    error_message: requiredString(data.error_message, 'data.error_message'),
    ...(data.job_id === undefined ? {} : { job_id: requiredNonNegativeInteger(data.job_id, 'data.job_id') }),
  } as QueryHistoryExportStatus;
}

export function parseQueryHistoryShareTokenResponse(value: unknown): { share_token: string } {
  const data = parseObject(successEnvelope(value).data, 'data');
  return { share_token: requireNonEmptyString(data.share_token, 'data.share_token') };
}

export function parseQueryHistoryConfigResponse(value: unknown): QueryHistoryConfig {
  const data = parseObject(successEnvelope(value).data, 'data');
  return { mode: parseQueryHistoryMode(data.mode, 'data.mode') };
}
