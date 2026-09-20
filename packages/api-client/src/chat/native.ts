import { ContractError } from '@weknora/contracts';
import {
  parseLastEventID,
  parseNativeEvent,
  type NativeEvent,
} from '@weknora/contracts/chat/native';
import type { ClientRequest } from '../client.ts';
import { createServerSentEventParser, type ParsedServerSentEvent } from './stream.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

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

export function parseNativeStreamEvent(frame: ParsedServerSentEvent): NativeEvent {
  try {
    const parsed = parseNativeEvent(JSON.parse(frame.data) as unknown);
    if (frame.id !== undefined && frame.id !== parsed.event_id) {
      throw new ContractError('event_id', 'must match the SSE id');
    }
    return parsed;
  } catch (error: unknown) {
    if (error instanceof NativeProtocolError) throw error;
    if (error instanceof ContractError && (error.path === 'protocol' || error.path === 'schema_version')) {
      throw new NativeProtocolError(error);
    }
    throw error;
  }
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

export function createNativeAgentApi(request: Request) {
  return {
    stream(options: NativeEventStreamOptions, onEvent: (event: NativeEvent) => void): Promise<void> {
      return consumeNativeEventStream(request, options, onEvent);
    },
    async cancel(sessionId: string, runId: string, input: NativeCommandInput, signal?: AbortSignal): Promise<NativeCommandResult> {
      validateCommand(input);
      return parseCommandResult(await request({
        method: 'POST', path: `${runPath(sessionId, runId)}/cancel`, body: input,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async steer(sessionId: string, runId: string, input: NativeSteerInput, signal?: AbortSignal): Promise<NativeCommandResult> {
      validateCommand(input);
      required(input.input_id, 'input_id');
      if (input.mode !== 'inject' && input.mode !== 'after') throw new Error('mode must be inject or after');
      required(input.message, 'message');
      return parseCommandResult(await request({
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
      return parseCommandResult(await request({
        method: 'POST', path: `${runPath(sessionId, runId)}/pending/${encoded(pendingId, 'pendingId')}/resolve`, body: input,
        ...(signal === undefined ? {} : { signal }),
      }));
    },
  };
}

export type NativeAgentApi = ReturnType<typeof createNativeAgentApi>;
