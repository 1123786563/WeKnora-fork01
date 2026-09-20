import { ContractError } from '@weknora/contracts';
import {
  parseLastEventID,
  parseNativeEvent,
  type NativeEvent,
} from '@weknora/contracts/chat/native';
import type { ClientRequest } from '../client.ts';
import type { HttpStreamResult } from '../ports.ts';
import { createServerSentEventParser, type ParsedServerSentEvent } from './stream.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface NativeAgentApiDeps {
  request: Request;
  sendStream?: (input: ClientRequest) => Promise<HttpStreamResult>;
}

export interface NativeEventStreamOptions {
  sessionId: string;
  runId: string;
  lastEventId?: string;
  signal?: AbortSignal;
}

export interface NativeCommandInput {
  command_id: string;
  expected_revision: string;
}

export interface NativeSteerInput extends NativeCommandInput {
  input_id: string;
  mode: 'inject' | 'after';
  message: string;
}

export interface NativeResolvePendingInput extends NativeCommandInput {
  pending_revision: string;
  plan_version: number;
  args_hash: string;
  call_id: string;
  action: 'retry' | 'terminate' | 'provide_result';
  reason: string;
  result?: Record<string, unknown>;
}

export interface NativeCommandResult {
  status: 'queued' | 'held' | 'terminated';
}

export class NativeProtocolError extends Error {
  readonly code = 'client_upgrade_required' as const;

  constructor(cause: unknown) {
    super('Native agent protocol is unsupported; upgrade required', { cause });
    this.name = 'NativeProtocolError';
  }
}

export class NativeStreamHttpError extends Error {
  readonly status: number;
  readonly headers: Readonly<Record<string, string>>;

  constructor(result: Pick<HttpStreamResult, 'status' | 'headers'>) {
    super(`Native event stream failed with HTTP ${result.status}`);
    this.name = 'NativeStreamHttpError';
    this.status = result.status;
    this.headers = result.headers;
  }
}

export interface NativeSnapshot {
  last_event_id: string;
}

function required(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return value;
}

function decimal(value: string, name: string): string {
  required(value, name);
  if (!/^(?:0|[1-9]\d*)$/.test(value)) throw new Error(`${name} must be a non-negative decimal`);
  return value;
}

function encoded(value: string, name: string): string {
  return encodeURIComponent(required(value, name));
}

function runPath(sessionId: string, runId: string): string {
  return `/api/v1/native-agent/sessions/${encoded(sessionId, 'sessionId')}/runs/${encoded(runId, 'runId')}`;
}

export function buildNativeEventsRequest(sessionId: string, runId: string, lastEventId?: string, signal?: AbortSignal): ClientRequest {
  const path = `${runPath(sessionId, runId)}/events`;
  if (lastEventId === undefined) {
    return { method: 'GET', path, headers: { accept: 'text/event-stream' }, ...(signal === undefined ? {} : { signal }) };
  }
  const cursor = parseLastEventID(lastEventId);
  if (cursor.run_id !== runId) throw new ContractError('run_id', 'must match the Last-Event-ID run');
  return {
    method: 'GET', path, headers: { accept: 'text/event-stream', 'Last-Event-ID': lastEventId },
    ...(signal === undefined ? {} : { signal }),
  };
}

export function buildNativeSnapshotRequest(sessionId: string, runId: string, signal?: AbortSignal): ClientRequest {
  return {
    method: 'GET', path: runPath(sessionId, runId), headers: { accept: 'application/json' },
    ...(signal === undefined ? {} : { signal }),
  };
}

export function parseNativeStreamEvent(frame: ParsedServerSentEvent): NativeEvent {
  let value: unknown;
  try {
    value = JSON.parse(frame.data) as unknown;
    const parsed = parseNativeEvent(value);
    if (frame.id !== undefined && frame.id !== parsed.event_id) {
      throw new ContractError('event_id', 'must match the SSE id');
    }
    return parsed;
  } catch (error: unknown) {
    if (error instanceof NativeProtocolError) throw error;
    const hasWellFormedVersion = typeof value === 'object' && value !== null && !Array.isArray(value)
      && typeof (value as Record<string, unknown>).protocol === 'string'
      && typeof (value as Record<string, unknown>).schema_version === 'number';
    if (hasWellFormedVersion && error instanceof ContractError && (error.path === 'protocol' || error.path === 'schema_version')) {
      throw new NativeProtocolError(error);
    }
    throw error;
  }
}

export function parseNativeSnapshot(value: unknown, runId: string): NativeSnapshot {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('native snapshot response must be an envelope');
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new Error('native snapshot response must be a success envelope');
  }
  const lastEventID = (envelope.data as Record<string, unknown>).last_event_id;
  const cursor = parseLastEventID(lastEventID);
  if (cursor.run_id !== runId) throw new ContractError('run_id', 'must match the snapshot cursor run');
  return { last_event_id: lastEventID as string };
}

export async function consumeNativeEventStream(
  request: Request,
  options: NativeEventStreamOptions,
  onEvent: (event: NativeEvent) => void,
): Promise<void> {
  const body = await request(buildNativeEventsRequest(options.sessionId, options.runId, options.lastEventId, options.signal));
  if (typeof body !== 'string') throw new Error('Native event stream returned a non-text body');
  const parser = createServerSentEventParser((frame) => onEvent(parseNativeStreamEvent(frame)));
  parser.push(body);
  parser.finish();
}

async function consumeNativeStreamResult(result: HttpStreamResult, onEvent: (event: NativeEvent) => void): Promise<void> {
  if (result.status < 200 || result.status >= 300) throw new NativeStreamHttpError(result);
  const parser = createServerSentEventParser((frame) => onEvent(parseNativeStreamEvent(frame)));
  for await (const chunk of result.chunks) parser.push(chunk);
  parser.finish();
}

function normalizeDeps(input: Request | NativeAgentApiDeps): NativeAgentApiDeps {
  return typeof input === 'function' ? { request: input } : input;
}

function joinedSignal(first: AbortSignal | undefined, second: AbortSignal): { signal: AbortSignal; dispose: () => void } {
  if (first === undefined) return { signal: second, dispose: () => {} };
  const controller = new AbortController();
  const abort = () => controller.abort();
  if (first.aborted || second.aborted) controller.abort();
  else {
    first.addEventListener('abort', abort, { once: true });
    second.addEventListener('abort', abort, { once: true });
  }
  return { signal: controller.signal, dispose: () => { first.removeEventListener('abort', abort); second.removeEventListener('abort', abort); } };
}

function isAborted(signal: AbortSignal | undefined): boolean {
  return signal?.aborted === true;
}

function parseCommandResult(value: unknown): NativeCommandResult {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('native command response must be an envelope');
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new Error('native command response must be a success envelope');
  }
  const status = (envelope.data as Record<string, unknown>).status;
  if (status !== 'queued' && status !== 'held' && status !== 'terminated') throw new Error('native command response.status is invalid');
  return { status };
}

function validateCommand(input: NativeCommandInput): void {
  required(input.command_id, 'command_id');
  decimal(input.expected_revision, 'expected_revision');
}

export function createNativeAgentApi(input: Request | NativeAgentApiDeps) {
  const deps = normalizeDeps(input);
  async function stream(options: NativeEventStreamOptions, onEvent: (event: NativeEvent) => void): Promise<void> {
    const request = buildNativeEventsRequest(options.sessionId, options.runId, options.lastEventId, options.signal);
    if (deps.sendStream) return consumeNativeStreamResult(await deps.sendStream(request), onEvent);
    return consumeNativeEventStream(deps.request, options, onEvent);
  }

  async function snapshot(sessionId: string, runId: string, signal?: AbortSignal): Promise<NativeSnapshot> {
    return parseNativeSnapshot(await deps.request(buildNativeSnapshotRequest(sessionId, runId, signal)), runId);
  }

  async function follow(options: NativeEventStreamOptions, onEvent: (event: NativeEvent) => void): Promise<void> {
    let lastEventId = options.lastEventId;
    while (!isAborted(options.signal)) {
      try {
        await stream({ ...options, ...(lastEventId === undefined ? {} : { lastEventId }) }, onEvent);
        return;
      } catch (error: unknown) {
        if (isAborted(options.signal)) return;
        if (!(error instanceof NativeStreamHttpError) || error.status !== 409) throw error;
        const authoritative = await snapshot(options.sessionId, options.runId, options.signal);
        if (isAborted(options.signal)) return;
        lastEventId = authoritative.last_event_id;
      }
    }
  }

  function createLifecycle() {
    let scopeKey: string | undefined;
    let controller = new AbortController();
    return {
      advanceScope(nextScopeKey: string): void {
        required(nextScopeKey, 'scopeKey');
        controller.abort();
        controller = new AbortController();
        scopeKey = nextScopeKey;
      },
      async follow(nextScopeKey: string, options: NativeEventStreamOptions, onEvent: (event: NativeEvent) => void): Promise<void> {
        required(nextScopeKey, 'scopeKey');
        if (scopeKey !== nextScopeKey) this.advanceScope(nextScopeKey);
        const active = controller;
        const joined = joinedSignal(options.signal, active.signal);
        try {
          await follow({ ...options, signal: joined.signal }, (event) => {
            if (controller === active && !joined.signal.aborted) onEvent(event);
          });
        } finally {
          joined.dispose();
        }
      },
      dispose(): void { controller.abort(); },
    };
  }

  return {
    stream,
    snapshot,
    follow,
    createLifecycle,
    async cancel(sessionId: string, runId: string, input: NativeCommandInput, signal?: AbortSignal): Promise<NativeCommandResult> {
      validateCommand(input);
      return parseCommandResult(await deps.request({
        method: 'POST', path: `${runPath(sessionId, runId)}/cancel`, body: input,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async steer(sessionId: string, runId: string, input: NativeSteerInput, signal?: AbortSignal): Promise<NativeCommandResult> {
      validateCommand(input);
      required(input.input_id, 'input_id');
      if (input.mode !== 'inject' && input.mode !== 'after') throw new Error('mode must be inject or after');
      required(input.message, 'message');
      return parseCommandResult(await deps.request({
        method: 'POST', path: `${runPath(sessionId, runId)}/steer`, body: input,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async resolvePending(sessionId: string, runId: string, pendingId: string, input: NativeResolvePendingInput, signal?: AbortSignal): Promise<NativeCommandResult> {
      validateCommand(input);
      decimal(input.pending_revision, 'pending_revision');
      if (!Number.isSafeInteger(input.plan_version) || input.plan_version < 1) throw new Error('plan_version must be a positive safe integer');
      required(input.args_hash, 'args_hash');
      required(input.call_id, 'call_id');
      if (input.action !== 'retry' && input.action !== 'terminate' && input.action !== 'provide_result') throw new Error('action is invalid');
      required(input.reason, 'reason');
      if (input.action === 'provide_result' && (typeof input.result !== 'object' || input.result === null || Array.isArray(input.result))) {
        throw new Error('provide_result requires a result object');
      }
      if (input.action !== 'provide_result' && input.result !== undefined) throw new Error('result is only valid for provide_result');
      return parseCommandResult(await deps.request({
        method: 'POST', path: `${runPath(sessionId, runId)}/pending/${encoded(pendingId, 'pendingId')}/resolve`, body: input,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

export type NativeAgentApi = ReturnType<typeof createNativeAgentApi>;
