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

export interface NativePendingReference { pending_id: string; detail_path: string; revision: string; }
export interface NativePublicArtifact { id: string; media_type: string; sha256: string; size_bytes: string; }
export interface NativeOutcome { attempt_id: string; call_id: string; result_hash: string; effect: NativeFailure['effect']; is_error: boolean; truncated: boolean; content: null; failure?: NativeFailure; }
export type NativeEventPayload =
  | { status: string; wait_kind?: string; pending_id?: string }
  | { replaces_attempt_id: string }
  | { text: string; offset: number }
  | { call_id: string; plan_version: number; tool_name: string }
  | { call_id: string; outcome: NativeOutcome }
  | { pending: NativePendingReference; call_id: string; plan_version: number; args_hash: string; expires_at: string; wait_kind: string }
  | { usage: NativePublicUsage }
  | { artifact: NativePublicArtifact }
  | { failure: NativeFailure }
  | Record<string, never>;

export interface NativeEventBase {
  protocol: typeof NATIVE_AGENT_PROTOCOL;
  schema_version: typeof NATIVE_AGENT_SCHEMA_VERSION;
  event_id: string;
  tenant_id: string;
  session_id: string;
  run_id: string;
  parent_run_id?: string;
  attempt_id?: string;
  seq: string;
}
export type NativeEvent = NativeEventBase
  & ({ kind: 'run.status'; payload: { status: string; wait_kind?: string; pending_id?: string } }
    | { kind: 'attempt.started' | 'attempt.finished'; payload: Record<string, never> }
    | { kind: 'attempt.replaced'; payload: { replaces_attempt_id: string } }
    | { kind: 'text.delta' | 'reasoning.delta'; attempt_id: string; payload: { text: string; offset: number } }
    | { kind: 'tool.planned'; payload: { call_id: string; plan_version: number; tool_name: string } }
    | { kind: 'tool.result'; attempt_id: string; payload: { call_id: string; outcome: NativeOutcome } }
    | { kind: 'decision.required'; payload: { pending: NativePendingReference; call_id: string; plan_version: number; args_hash: string; expires_at: string; wait_kind: string } }
    | { kind: 'usage.observed'; attempt_id: string; payload: { usage: NativePublicUsage } }
    | { kind: 'artifact.available'; payload: { artifact: NativePublicArtifact } }
    | { kind: 'error'; payload: { failure: NativeFailure } });

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
const ERROR_CODES = new Set(['unauthorized', 'forbidden', 'invalid_request', 'conflict', 'not_found', 'lease_lost', 'cancelled', 'deadline_exceeded', 'budget_exhausted', 'provider_error', 'incomplete_stream', 'unknown_effect', 'durable_store_unavailable', 'checkpoint_incompatible', 'cursor_expired', 'archive_read_only', 'client_upgrade_required', 'admission_closed', 'execution_gate_closed']);
const SENSITIVE_FIELD = /(?:token|secret|password|credential|authorization|receipt|anchor)/i;

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function exact(row: Record<string, unknown>, allowed: readonly string[], path: string): void {
  for (const key of Object.keys(row)) {
    if (!allowed.includes(key)) throw new ContractError(`${path}${path === '' ? '' : '.'}${key}`, 'unknown or private public-wire field');
  }
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
  exact(row, ['code', 'message', 'retryable', 'effect', 'attempt_id'], path);
  const effect = nonEmpty(row.effect, `${path}.effect`);
  if (!EFFECTS.has(effect)) throw new ContractError(`${path}.effect`, 'unknown effect');
  const code = nonEmpty(row.code, `${path}.code`);
  if (!ERROR_CODES.has(code)) throw new ContractError(`${path}.code`, 'unknown error code');
  const parsed: NativeFailure = {
    code,
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

function pendingReference(value: unknown, path: string): NativePendingReference {
  const row = object(value, path);
  exact(row, ['pending_id', 'detail_path', 'revision'], path);
  return { pending_id: nonEmpty(row.pending_id, `${path}.pending_id`), detail_path: nonEmpty(row.detail_path, `${path}.detail_path`), revision: decimal(row.revision, `${path}.revision`) };
}

function publicArtifact(value: unknown, path: string): NativePublicArtifact {
  const row = object(value, path);
  exact(row, ['id', 'media_type', 'sha256', 'size_bytes'], path);
  return { id: nonEmpty(row.id, `${path}.id`), media_type: nonEmpty(row.media_type, `${path}.media_type`), sha256: nonEmpty(row.sha256, `${path}.sha256`), size_bytes: decimal(row.size_bytes, `${path}.size_bytes`) };
}

function publicOutcome(value: unknown, path: string): NativeOutcome {
  const row = object(value, path);
  exact(row, ['attempt_id', 'call_id', 'result_hash', 'effect', 'is_error', 'truncated', 'content', 'failure'], path);
  const effect = nonEmpty(row.effect, `${path}.effect`);
  if (!EFFECTS.has(effect)) throw new ContractError(`${path}.effect`, 'unknown effect');
  if (typeof row.is_error !== 'boolean' || typeof row.truncated !== 'boolean' || row.content !== null) throw new ContractError(path, 'invalid public outcome');
  const parsed: NativeOutcome = { attempt_id: nonEmpty(row.attempt_id, `${path}.attempt_id`), call_id: nonEmpty(row.call_id, `${path}.call_id`), result_hash: nonEmpty(row.result_hash, `${path}.result_hash`), effect: effect as NativeFailure['effect'], is_error: row.is_error, truncated: row.truncated, content: null };
  if (row.failure !== undefined) parsed.failure = failure(row.failure, `${path}.failure`);
  return parsed;
}

function payload(value: unknown, event: Pick<NativeEvent, 'kind' | 'attempt_id'>, path: string): NativeEventPayload {
  const row = object(value, path);
  switch (event.kind) {
    case 'run.status': {
      exact(row, ['status', 'wait_kind', 'pending_id'], path);
      const status = nonEmpty(row.status, `${path}.status`);
      if (!RUN_STATUSES.has(status)) throw new ContractError(`${path}.status`, 'unknown run status');
      const wait = optionalString(row.wait_kind, `${path}.wait_kind`);
      const pendingID = optionalString(row.pending_id, `${path}.pending_id`);
      if (wait !== undefined && !WAIT_KINDS.has(wait)) throw new ContractError(`${path}.wait_kind`, 'unknown wait kind');
      if (status === 'waiting_user' && (wait === undefined || pendingID === undefined)) throw new ContractError(path, 'waiting status requires wait_kind and pending_id');
      if (status !== 'waiting_user' && (wait !== undefined || pendingID !== undefined)) throw new ContractError(path, 'only waiting status may carry pending state');
      return { status, ...(wait === undefined ? {} : { wait_kind: wait }), ...(pendingID === undefined ? {} : { pending_id: pendingID }) };
    }
    case 'attempt.started':
    case 'attempt.finished':
      exact(row, [], path);
      return {};
    case 'attempt.replaced':
      exact(row, ['replaces_attempt_id'], path);
      return { replaces_attempt_id: nonEmpty(row.replaces_attempt_id, `${path}.replaces_attempt_id`) };
    case 'text.delta':
    case 'reasoning.delta':
      if (event.attempt_id === undefined) throw new ContractError('attempt_id', 'stream delta requires an attempt');
      exact(row, ['text', 'offset'], path);
      return { text: nonEmpty(row.text, `${path}.text`), offset: nonNegativeSafeInteger(row.offset, `${path}.offset`) };
    case 'tool.planned':
      exact(row, ['call_id', 'plan_version', 'tool_name'], path);
      return { call_id: nonEmpty(row.call_id, `${path}.call_id`), plan_version: nonNegativeSafeInteger(row.plan_version, `${path}.plan_version`), tool_name: nonEmpty(row.tool_name, `${path}.tool_name`) };
    case 'tool.result':
      if (event.attempt_id === undefined) throw new ContractError('attempt_id', 'tool result requires an attempt');
      exact(row, ['call_id', 'outcome'], path);
      return { call_id: nonEmpty(row.call_id, `${path}.call_id`), outcome: publicOutcome(row.outcome, `${path}.outcome`) };
    case 'decision.required': {
      exact(row, ['pending', 'call_id', 'plan_version', 'args_hash', 'expires_at', 'wait_kind'], path);
      const wait = nonEmpty(row.wait_kind, `${path}.wait_kind`);
      if (!WAIT_KINDS.has(wait)) throw new ContractError(`${path}.wait_kind`, 'unknown wait kind');
      return { pending: pendingReference(row.pending, `${path}.pending`), call_id: nonEmpty(row.call_id, `${path}.call_id`), plan_version: nonNegativeSafeInteger(row.plan_version, `${path}.plan_version`), args_hash: nonEmpty(row.args_hash, `${path}.args_hash`), expires_at: nonEmpty(row.expires_at, `${path}.expires_at`), wait_kind: wait };
    }
    case 'usage.observed':
      if (event.attempt_id === undefined) throw new ContractError('attempt_id', 'usage requires an attempt');
      exact(row, ['usage'], path);
      return { usage: publicUsage(row.usage, `${path}.usage`) };
    case 'artifact.available':
      exact(row, ['artifact'], path);
      return { artifact: publicArtifact(row.artifact, `${path}.artifact`) };
    case 'error':
      exact(row, ['failure'], path);
      return { failure: failure(row.failure, `${path}.failure`) };
  }
}

export function parseSequence(value: string): bigint {
  decimal(value, 'seq', false);
  return BigInt(value);
}

export function parseNativeEvent(value: unknown): NativeEvent {
  const row = object(value, '');
  exact(row, ['protocol', 'schema_version', 'event_id', 'tenant_id', 'session_id', 'run_id', 'parent_run_id', 'attempt_id', 'seq', 'kind', 'payload'], '');
  version(row);
  const kind = nonEmpty(row.kind, 'kind');
  if (!EVENT_KINDS.has(kind as NativeEventKind)) throw new ContractError('kind', 'unknown native event kind');
  const event: NativeEventBase & { kind: NativeEventKind } = {
    protocol: NATIVE_AGENT_PROTOCOL,
    schema_version: NATIVE_AGENT_SCHEMA_VERSION,
    event_id: nonEmpty(row.event_id, 'event_id'),
    tenant_id: decimal(row.tenant_id, 'tenant_id', false),
    session_id: nonEmpty(row.session_id, 'session_id'),
    run_id: nonEmpty(row.run_id, 'run_id'),
    seq: decimal(row.seq, 'seq', false),
    kind: kind as NativeEventKind,
  };
  const parentRunID = optionalString(row.parent_run_id, 'parent_run_id');
  const attemptID = optionalString(row.attempt_id, 'attempt_id');
  if (parentRunID !== undefined) event.parent_run_id = parentRunID;
  if (attemptID !== undefined) event.attempt_id = attemptID;
  return { ...event, payload: payload(row.payload, event, 'payload') } as NativeEvent;
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
