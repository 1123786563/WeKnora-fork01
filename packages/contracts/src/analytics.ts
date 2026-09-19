// SP11 feedback analytics wire contracts.
// Shapes mirror internal/types/message_feedback.go and
// internal/types/interfaces/analytics.go; extra backend fields pass through
// via index signatures / spreads per index.ts conventions.
import { ContractError } from './index.ts';

export type FeedbackRating = 'like' | 'dislike';

export interface MessageFeedbackEntry {
  id: number;
  message_id: string;
  session_id: string;
  rating: FeedbackRating;
  comment?: string;
  [key: string]: unknown;
}

export interface QueryTrendPoint {
  date: string;
  queries: number;
  likes: number;
  dislikes: number;
}

export interface ActiveUsersPoint {
  date: string;
  active_users: number;
}

export interface ChannelSessionsPoint {
  date: string;
  source: string;
  sessions: number;
}

export interface AgentUsagePoint {
  date: string;
  messages: number;
  unique_users: number;
}

function requiredString(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new ContractError(path, 'expected a string');
  return value;
}

function requiredNonNegativeInteger(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ContractError(path, 'expected a non-negative integer');
  }
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

function parsePointList<T>(value: unknown, parseItem: (item: unknown, path: string) => T, path: string): { items: T[] } {
  const result = successEnvelope(value);
  if (!Array.isArray(result.data)) throw new ContractError('data', 'expected an array');
  return { items: result.data.map((item, index) => parseItem(item, `${path}[${index}]`)) };
}

function parseObject(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  return value as Record<string, unknown>;
}

function parseQueryTrendPoint(value: unknown, path: string): QueryTrendPoint {
  const row = parseObject(value, path);
  return {
    date: requiredString(row.date, `${path}.date`),
    queries: requiredNonNegativeInteger(row.queries, `${path}.queries`),
    likes: requiredNonNegativeInteger(row.likes, `${path}.likes`),
    dislikes: requiredNonNegativeInteger(row.dislikes, `${path}.dislikes`),
  };
}

function parseActiveUsersPoint(value: unknown, path: string): ActiveUsersPoint {
  const row = parseObject(value, path);
  return {
    date: requiredString(row.date, `${path}.date`),
    active_users: requiredNonNegativeInteger(row.active_users, `${path}.active_users`),
  };
}

function parseChannelSessionsPoint(value: unknown, path: string): ChannelSessionsPoint {
  const row = parseObject(value, path);
  return {
    date: requiredString(row.date, `${path}.date`),
    source: requiredString(row.source, `${path}.source`),
    sessions: requiredNonNegativeInteger(row.sessions, `${path}.sessions`),
  };
}

function parseAgentUsagePoint(value: unknown, path: string): AgentUsagePoint {
  const row = parseObject(value, path);
  return {
    date: requiredString(row.date, `${path}.date`),
    messages: requiredNonNegativeInteger(row.messages, `${path}.messages`),
    unique_users: requiredNonNegativeInteger(row.unique_users, `${path}.unique_users`),
  };
}

function parseFeedbackRating(value: unknown, path: string): FeedbackRating {
  if (value !== 'like' && value !== 'dislike') {
    throw new ContractError(path, 'expected like or dislike');
  }
  return value;
}

function parseMessageFeedbackEntry(value: unknown, path: string): MessageFeedbackEntry {
  const row = parseObject(value, path);
  const comment = row.comment === undefined || row.comment === null
    ? undefined
    : requiredString(row.comment, `${path}.comment`);
  return {
    ...row,
    id: requiredNonNegativeInteger(row.id, `${path}.id`),
    message_id: requiredString(row.message_id, `${path}.message_id`),
    session_id: requiredString(row.session_id, `${path}.session_id`),
    rating: parseFeedbackRating(row.rating, `${path}.rating`),
    ...(comment === undefined ? {} : { comment }),
  } as MessageFeedbackEntry;
}

export function parseQueryTrendResponse(value: unknown): { items: QueryTrendPoint[] } {
  return parsePointList(value, parseQueryTrendPoint, 'data');
}

export function parseActiveUsersResponse(value: unknown): { items: ActiveUsersPoint[] } {
  return parsePointList(value, parseActiveUsersPoint, 'data');
}

export function parseChannelSessionsResponse(value: unknown): { items: ChannelSessionsPoint[] } {
  return parsePointList(value, parseChannelSessionsPoint, 'data');
}

export function parseAgentUsageResponse(value: unknown): { items: AgentUsagePoint[] } {
  return parsePointList(value, parseAgentUsagePoint, 'data');
}

export function parseMessageFeedbackListResponse(value: unknown): { items: MessageFeedbackEntry[] } {
  return parsePointList(value, parseMessageFeedbackEntry, 'data');
}
