import { ContractError } from '../index.ts';

export const NATIVE_AGENT_PROTOCOL = 'weknora.agent.v1' as const;
export const NATIVE_AGENT_SCHEMA_VERSION = 1 as const;

export type NativeEventKind =
  | 'run.status'
  | 'attempt.started'
  | 'attempt.replaced'
  | 'attempt.finished'
  | 'text.delta'
  | 'reasoning.delta'
  | 'tool.planned'
  | 'tool.result'
  | 'decision.required'
  | 'usage.observed'
  | 'artifact.available'
  | 'error';

export interface NativeFailure {
  code: string;
  message: string;
  retryable: boolean;
  effect: 'not_dispatched' | 'confirmed' | 'unknown';
  attempt_id?: string;
}

export interface NativePublicUsage {
  observation_id: string;
  revision: string;
  prompt_tokens: string;
  completion_tokens: string;
  total_tokens: string;
  cached_tokens: string;
  cache_read_tokens: string;
  cache_create_tokens: string;
  accounting_status: string;
}

export interface NativeEvent {
  protocol: typeof NATIVE_AGENT_PROTOCOL;
  schema_version: typeof NATIVE_AGENT_SCHEMA_VERSION;
  event_id: string;
  tenant_id: string;
  session_id: string;
  run_id: string;
  parent_run_id?: string;
  attempt_id?: string;
  seq: string;
  kind: NativeEventKind;
  payload: Record<string, unknown> & { usage?: NativePublicUsage };
}

export interface NativeLastEventID {
  run_id: string;
  sequence: bigint;
}

export interface NativeAgentFixture {
  protocol: typeof NATIVE_AGENT_PROTOCOL;
  schema_version: typeof NATIVE_AGENT_SCHEMA_VERSION;
  last_event_id: NativeLastEventID;
  events: NativeEvent[];
  pending: Record<string, unknown>;
  command_errors: NativeFailure[];
  archive: Record<string, unknown>;
}

const EVENT_KINDS = new Set<NativeEventKind>([
  'run.status', 'attempt.started', 'attempt.replaced', 'attempt.finished', 'text.delta', 'reasoning.delta',
  'tool.planned', 'tool.result', 'decision.required', 'usage.observed', 'artifact.available', 'error',
]);
const EFFECTS = new Set(['not_dispatched', 'confirmed', 'unknown']);
const WAIT_KINDS = new Set(['tool_approval', 'mcp_oauth', 'connector_approval', 'unknown_result']);
const RUN_STATUSES = new Set(['queued', 'running', 'waiting_user', 'cancelling', 'succeeded', 'failed', 'cancelled']);
const SENSITIVE_FIELD = /(?:token|secret|password|credential|authorization|receipt|anchor)/i;

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function nonEmpty(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected a non-empty string');
  return value;
}

function optionalString(value: unknown, path: string): string | undefined {
  if (value === undefined) return undefined;
  return nonEmpty(value, path);
}

function decimal(value: unknown, path: string, allowZero = true): string {
  if (typeof value !== 'string' || !/^(?:0|[1-9][0-9]*)$/.test(value) || (!allowZero && value === '0')) {
    throw new ContractError(path, 'expected an unsigned decimal string');
  }
  try {
    if (BigInt(value) > 9223372036854775807n) throw new ContractError(path, 'integer exceeds int64');
  } catch (error) {
    if (error instanceof ContractError) throw error;
    throw new ContractError(path, 'expected an unsigned decimal string');
  }
  return value;
}

function nonNegativeSafeInteger(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < 0) {
    throw new ContractError(path, 'expected a non-negative safe integer');
  }
  return value;
}

function version(row: Record<string, unknown>, path = ''): void {
  const prefix = path === '' ? '' : `${path}.`;
  if (row.protocol !== NATIVE_AGENT_PROTOCOL) throw new ContractError(`${prefix}protocol`, `expected ${NATIVE_AGENT_PROTOCOL}`);
  if (row.schema_version !== NATIVE_AGENT_SCHEMA_VERSION) throw new ContractError(`${prefix}schema_version`, 'expected 1');
}

function failure(value: unknown, path: string): NativeFailure {
  const row = object(value, path);
  const effect = nonEmpty(row.effect, `${path}.effect`);
  if (!EFFECTS.has(effect)) throw new ContractError(`${path}.effect`, 'unknown effect');
  const parsed: NativeFailure = {
    code: nonEmpty(row.code, `${path}.code`),
    message: nonEmpty(row.message, `${path}.message`),
    retryable: typeof row.retryable === 'boolean' ? row.retryable : (() => { throw new ContractError(`${path}.retryable`, 'expected a boolean'); })(),
    effect: effect as NativeFailure['effect'],
  };
  const attemptID = optionalString(row.attempt_id, `${path}.attempt_id`);
  if (attemptID !== undefined) parsed.attempt_id = attemptID;
  return parsed;
}

function publicUsage(value: unknown, path: string): NativePublicUsage {
  const row = object(value, path);
  return {
    observation_id: nonEmpty(row.observation_id, `${path}.observation_id`),
    revision: decimal(row.revision, `${path}.revision`),
    prompt_tokens: decimal(row.prompt_tokens, `${path}.prompt_tokens`),
    completion_tokens: decimal(row.completion_tokens, `${path}.completion_tokens`),
    total_tokens: decimal(row.total_tokens, `${path}.total_tokens`),
    cached_tokens: decimal(row.cached_tokens, `${path}.cached_tokens`),
    cache_read_tokens: decimal(row.cache_read_tokens, `${path}.cache_read_tokens`),
    cache_create_tokens: decimal(row.cache_create_tokens, `${path}.cache_create_tokens`),
    accounting_status: nonEmpty(row.accounting_status, `${path}.accounting_status`),
  };
}

function payload(value: unknown, event: Pick<NativeEvent, 'kind' | 'attempt_id'>, path: string): NativeEvent['payload'] {
  const row = object(value, path);
  if (event.kind === 'text.delta' || event.kind === 'reasoning.delta') {
    if (event.attempt_id === undefined) throw new ContractError(`${path}.attempt_id`, 'stream delta requires an attempt');
    nonEmpty(row.text, `${path}.text`);
    nonNegativeSafeInteger(row.offset, `${path}.offset`);
  }
  if (event.kind === 'attempt.replaced') nonEmpty(row.replaces_attempt_id, `${path}.replaces_attempt_id`);
  if (event.kind === 'tool.result') {
    nonEmpty(row.call_id, `${path}.call_id`);
    const outcome = object(row.outcome, `${path}.outcome`);
    for (const field of ['provider_receipt', 'query_anchor'] as const) {
      if (field in outcome) throw new ContractError(`${path}.outcome.${field}`, 'is not public wire data');
    }
    nonEmpty(outcome.attempt_id, `${path}.outcome.attempt_id`);
    nonEmpty(outcome.call_id, `${path}.outcome.call_id`);
    nonEmpty(outcome.result_hash, `${path}.outcome.result_hash`);
    if (!EFFECTS.has(nonEmpty(outcome.effect, `${path}.outcome.effect`))) throw new ContractError(`${path}.outcome.effect`, 'unknown effect');
    if (typeof outcome.is_error !== 'boolean' || typeof outcome.truncated !== 'boolean' || outcome.content !== null) throw new ContractError(`${path}.outcome`, 'must contain public outcome fields only');
    if (outcome.failure !== undefined) failure(outcome.failure, `${path}.outcome.failure`);
  }
  if (event.kind === 'usage.observed') row.usage = publicUsage(row.usage, `${path}.usage`);
  if (event.kind === 'error') row.failure = failure(row.failure, `${path}.failure`);
  return row as NativeEvent['payload'];
}

export function parseSequence(value: string): bigint {
  decimal(value, 'seq', false);
  return BigInt(value);
}

export function parseNativeEvent(value: unknown): NativeEvent {
  const row = object(value, '');
  version(row);
  const kind = nonEmpty(row.kind, 'kind');
  if (!EVENT_KINDS.has(kind as NativeEventKind)) throw new ContractError('kind', 'unknown native event kind');
  const event: NativeEvent = {
    protocol: NATIVE_AGENT_PROTOCOL,
    schema_version: NATIVE_AGENT_SCHEMA_VERSION,
    event_id: nonEmpty(row.event_id, 'event_id'),
    tenant_id: decimal(row.tenant_id, 'tenant_id', false),
    session_id: nonEmpty(row.session_id, 'session_id'),
    run_id: nonEmpty(row.run_id, 'run_id'),
    seq: decimal(row.seq, 'seq', false),
    kind: kind as NativeEventKind,
    payload: {},
  };
  const parentRunID = optionalString(row.parent_run_id, 'parent_run_id');
  const attemptID = optionalString(row.attempt_id, 'attempt_id');
  if (parentRunID !== undefined) event.parent_run_id = parentRunID;
  if (attemptID !== undefined) event.attempt_id = attemptID;
  event.payload = payload(row.payload, event, 'payload');
  return event;
}

export function parseLastEventID(value: unknown): NativeLastEventID {
  if (typeof value !== 'string') throw new ContractError('last_event_id', 'expected v1 cursor');
  const parts = value.split(':');
  if (parts.length !== 3 || parts[0] !== 'v1' || !/^[A-Za-z0-9_-]+$/.test(parts[1]!)) throw new ContractError('last_event_id', 'expected v1 cursor');
  let runID: string;
  try {
    const encoded = parts[1]!.replace(/-/g, '+').replace(/_/g, '/');
    const padded = encoded.padEnd(encoded.length + ((4 - encoded.length % 4) % 4), '=');
    const bytes = Uint8Array.from(atob(padded), (character) => character.charCodeAt(0));
    runID = new TextDecoder().decode(bytes);
  } catch {
    throw new ContractError('last_event_id', 'invalid run encoding');
  }
  const canonical = btoa(String.fromCharCode(...new TextEncoder().encode(runID))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  if (runID === '' || canonical !== parts[1]) throw new ContractError('last_event_id', 'invalid run encoding');
  try {
    return { run_id: runID, sequence: parseSequence(parts[2]!) };
  } catch {
    throw new ContractError('last_event_id', 'invalid sequence');
  }
}

function containsSensitive(value: unknown): boolean {
  if (Array.isArray(value)) return value.some(containsSensitive);
  if (typeof value !== 'object' || value === null) return false;
  return Object.entries(value as Record<string, unknown>).some(([key, item]) => SENSITIVE_FIELD.test(key) || containsSensitive(item));
}

function pending(value: unknown): Record<string, unknown> {
  const row = object(value, 'pending');
  if (row.version !== 1) throw new ContractError('pending.version', 'expected 1');
  const ref = object(row.ref, 'pending.ref');
  nonEmpty(ref.pending_id, 'pending.ref.pending_id');
  nonEmpty(ref.detail_path, 'pending.ref.detail_path');
  decimal(ref.revision, 'pending.ref.revision');
  for (const field of ['session_id', 'run_id', 'call_id', 'args_hash', 'operation_description', 'redaction_version', 'expires_at', 'resolve_path'] as const) nonEmpty(row[field], `pending.${field}`);
  if (!WAIT_KINDS.has(nonEmpty(row.wait_kind, 'pending.wait_kind'))) throw new ContractError('pending.wait_kind', 'unknown wait kind');
  if (!RUN_STATUSES.has(nonEmpty(row.run_status, 'pending.run_status'))) throw new ContractError('pending.run_status', 'unknown run status');
  decimal(row.run_revision, 'pending.run_revision');
  if (!Number.isInteger(row.plan_version) || (row.plan_version as number) < 1) throw new ContractError('pending.plan_version', 'expected a positive integer');
  const redactedPaths = row.redacted_paths;
  if (!Array.isArray(redactedPaths) || !redactedPaths.every((item) => typeof item === 'string' && item.startsWith('/'))) throw new ContractError('pending.redacted_paths', 'expected JSON pointer paths');
  if (containsSensitive(row.redacted_args) && redactedPaths.length === 0) throw new ContractError('pending.redacted_args', 'sensitive values must be redacted');
  const oauth = row.oauth;
  if (oauth !== undefined) {
    const details = object(oauth, 'pending.oauth');
    for (const key of Object.keys(details)) if (SENSITIVE_FIELD.test(key) || key === 'authorization_url') throw new ContractError(`pending.oauth.${key}`, 'is not public wire data');
    for (const field of ['service_id', 'state', 'begin_path'] as const) nonEmpty(details[field], `pending.oauth.${field}`);
  }
  return row;
}

function archive(value: unknown): Record<string, unknown> {
  const row = object(value, 'archive');
  if (!Array.isArray(row.records)) throw new ContractError('archive.records', 'expected an array');
  for (const [index, record] of row.records.entries()) {
    const item = object(record, `archive.records[${index}]`);
    for (const field of ['id', 'session_id', 'kind'] as const) nonEmpty(item[field], `archive.records[${index}].${field}`);
    if (!Array.isArray(item.artifacts)) throw new ContractError(`archive.records[${index}].artifacts`, 'expected an array');
    for (const [artifactIndex, artifact] of item.artifacts.entries()) {
      const ref = object(artifact, `archive.records[${index}].artifacts[${artifactIndex}]`);
      for (const field of ['id', 'media_type', 'sha256'] as const) nonEmpty(ref[field], `archive.records[${index}].artifacts[${artifactIndex}].${field}`);
      decimal(ref.size_bytes, `archive.records[${index}].artifacts[${artifactIndex}].size_bytes`);
    }
  }
  optionalString(row.next_cursor, 'archive.next_cursor');
  return row;
}

export function parseNativeAgentFixture(value: unknown): NativeAgentFixture {
  const row = object(value, '');
  version(row);
  if (!Array.isArray(row.events)) throw new ContractError('events', 'expected an array');
  const events = row.events.map(parseNativeEvent);
  const lastEventID = parseLastEventID(row.last_event_id);
  if (events.some((event) => event.run_id !== lastEventID.run_id)) throw new ContractError('last_event_id', 'must be scoped to the fixture run');
  if (!Array.isArray(row.command_errors)) throw new ContractError('command_errors', 'expected an array');
  return {
    protocol: NATIVE_AGENT_PROTOCOL,
    schema_version: NATIVE_AGENT_SCHEMA_VERSION,
    last_event_id: lastEventID,
    events,
    pending: pending(row.pending),
    command_errors: row.command_errors.map((item, index) => failure(item, `command_errors[${index}]`)),
    archive: archive(row.archive),
  };
}
