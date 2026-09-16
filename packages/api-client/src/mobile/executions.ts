import {
  ContractError,
  parseExecution,
  parseExecutionSnapshot,
  type ExecutionDTO,
  type ExecutionSnapshot,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

/** The durable admission input. Keep these names aligned with the HTTP wire contract. */
export interface StartExecutionInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number;
}

/** Commands are intentionally a closed union: arbitrary method/body pairs are not exposed. */
export type ExecutionCommandInput =
  | { action: 'cancel'; expected_revision: number }
  | { action: 'steer'; text: string; expected_revision: number };

export type RequestLookupState = 'pending' | 'dispatching' | 'admitted' | 'rejected' | 'unknown';

export interface RequestLookup {
  state: RequestLookupState;
  run_id?: string;
  reason?: string;
}

type Request = (input: ClientRequest) => Promise<unknown>;

function required(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return value;
}

function pathId(value: string, name: string): string {
  return encodeURIComponent(required(value, name));
}

function positiveSafeInteger(value: number, name: string): number {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`${name} must be a non-negative safe integer`);
  return value;
}

function unwrap(value: unknown): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('execution response must be a success envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new Error('execution response.success must be true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('execution response.data is required');
  }
  return envelope.data;
}

function parseExecutionForRun(value: unknown, runID: string): ExecutionDTO {
  const execution = parseExecution(value);
  if (execution.run_id !== runID) {
    throw new ContractError('run_id', 'must match the requested run_id');
  }
  return execution;
}

function parseSnapshotForRun(value: unknown, runID: string): ExecutionSnapshot {
  const snapshot = parseExecutionSnapshot(value);
  if (snapshot.execution.run_id !== runID) {
    throw new ContractError('execution.run_id', 'must match the requested run_id');
  }
  if (snapshot.events.some((event) => event.seq > snapshot.watermark)) {
    throw new ContractError('watermark', 'must be greater than or equal to every event sequence');
  }
  return snapshot;
}

function parseLookup(value: unknown): RequestLookup {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('request lookup.data must be an object');
  }
  const row = value as Record<string, unknown>;
  const state = row.state;
  if (state !== 'pending' && state !== 'dispatching' && state !== 'admitted' && state !== 'rejected' && state !== 'unknown') {
    throw new Error('request lookup.data.state is invalid');
  }
  if (row.run_id !== undefined && typeof row.run_id !== 'string') throw new Error('request lookup.data.run_id must be a string');
  if (row.reason !== undefined && typeof row.reason !== 'string') throw new Error('request lookup.data.reason must be a string');
  if ((state === 'admitted' || state === 'dispatching') && (typeof row.run_id !== 'string' || row.run_id.trim() === '')) {
    throw new Error('request lookup.data.run_id is required for an admitted request');
  }
  return {
    state,
    ...(row.run_id === undefined ? {} : { run_id: row.run_id }),
    ...(row.reason === undefined ? {} : { reason: row.reason }),
  };
}

function validateStart(input: StartExecutionInput): void {
  required(input.request_id, 'request_id');
  required(input.session_id, 'session_id');
  required(input.agent_id, 'agent_id');
  required(input.target_id, 'target_id');
  if (typeof input.workspace_ref !== 'string') throw new Error('workspace_ref must be a string');
  required(input.text, 'text');
  positiveSafeInteger(input.budget_upper, 'budget_upper');
}

function validateCommand(input: ExecutionCommandInput): void {
  if (input.action !== 'cancel' && input.action !== 'steer') throw new Error('command action is invalid');
  positiveSafeInteger(input.expected_revision, 'expected_revision');
  if (input.action === 'cancel') {
    if ('text' in input) throw new Error('cancel command cannot include text');
  } else if (typeof input.text !== 'string' || input.text.trim() === '') {
    throw new Error('steer command text must not be empty');
  }
}

export function executionEventsRequest(runID: string, lastEventID?: string): ClientRequest {
  const id = pathId(runID, 'runID');
  const cursor = lastEventID === undefined ? undefined : required(lastEventID, 'lastEventID');
  return {
    method: 'GET',
    path: `/api/v1/workbench/executions/${id}/events`,
    headers: cursor === undefined ? undefined : { 'Last-Event-ID': cursor },
  };
}

export function createExecutionsApi(request: Request) {
  const get = async (runID: string, signal?: AbortSignal): Promise<ExecutionDTO> => {
    const requestedRunID = required(runID, 'runID');
    const response = await request({ method: 'GET', path: `/api/v1/workbench/executions/${pathId(requestedRunID, 'runID')}`, ...(signal === undefined ? {} : { signal }) });
    return parseExecutionForRun(unwrap(response), requestedRunID);
  };

  const snapshot = async (runID: string, signal?: AbortSignal): Promise<ExecutionSnapshot> => {
    const requestedRunID = required(runID, 'runID');
    const response = await request({ method: 'GET', path: `/api/v1/workbench/executions/${pathId(requestedRunID, 'runID')}/snapshot`, ...(signal === undefined ? {} : { signal }) });
    return parseSnapshotForRun(unwrap(response), requestedRunID);
  };

  const start = async (input: StartExecutionInput, signal?: AbortSignal): Promise<ExecutionDTO> => {
    validateStart(input);
    const response = await request({ method: 'POST', path: '/api/v1/workbench/executions', body: input, ...(signal === undefined ? {} : { signal }) });
    return parseExecution(unwrap(response));
  };

  const lookup = async (requestID: string, signal?: AbortSignal): Promise<RequestLookup> => {
    const id = pathId(requestID, 'requestID');
    const response = await request({ method: 'GET', path: `/api/v1/workbench/executions/requests/${id}`, ...(signal === undefined ? {} : { signal }) });
    return parseLookup(unwrap(response));
  };

  const command = async (runID: string, input: ExecutionCommandInput, signal?: AbortSignal): Promise<ExecutionDTO> => {
    validateCommand(input);
    const requestedRunID = required(runID, 'runID');
    const response = await request({ method: 'POST', path: `/api/v1/workbench/executions/${pathId(requestedRunID, 'runID')}/commands`, body: input, ...(signal === undefined ? {} : { signal }) });
    return parseExecutionForRun(unwrap(response), requestedRunID);
  };

  return { get, snapshot, start, lookup, command };
}

export type ExecutionsApi = ReturnType<typeof createExecutionsApi>;
