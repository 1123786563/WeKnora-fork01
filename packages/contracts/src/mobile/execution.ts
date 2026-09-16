import { ContractError } from '../index.ts';

export type CapabilityState = 'supported' | 'unavailable' | 'forbidden';
export type ExecutionDriver = 'platform' | 'paseo';
export type RunStatus = 'queued' | 'running' | 'waiting_user' | 'reconciling' | 'succeeded' | 'failed' | 'canceled';

export interface Capability {
  state: CapabilityState;
  reason: string;
}

export interface ExecutionDTO {
  schema_version: 1;
  run_id: string;
  session_id: string;
  revision: number;
  driver: ExecutionDriver;
  run_status: RunStatus;
  execution_status: string;
  settlement_status: string;
  seq: number;
  capabilities: Record<string, Capability>;
}

export interface ExecutionEvent {
  schema_version: 1;
  run_id: string;
  attempt_id: string;
  seq: number;
  type: string;
  occurred_at: string;
  payload: Record<string, unknown>;
}

export interface ExecutionSnapshot {
  execution: ExecutionDTO;
  watermark: number;
  events: ExecutionEvent[];
}

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(path, 'expected an object');
  }
  return value as Record<string, unknown>;
}

function nonEmpty(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(path, 'expected a non-empty string');
  return value;
}

function safeInteger(value: unknown, path: string, minimum: number): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < minimum) {
    throw new ContractError(path, 'INVALID_EVENT: expected a safe integer');
  }
  return value;
}

function isoTime(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new ContractError(path, 'expected an ISO-8601 timestamp');
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/.exec(value);
  if (!match) throw new ContractError(path, 'expected an ISO-8601 timestamp');
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6]);
  const offset = match[7];
  const calendar = new Date(0);
  calendar.setUTCHours(0, 0, 0, 0);
  calendar.setUTCFullYear(year, month - 1, day);
  const offsetHour = offset === 'Z' ? 0 : Number(offset.slice(1, 3));
  const offsetMinute = offset === 'Z' ? 0 : Number(offset.slice(4, 6));
  if (calendar.getUTCFullYear() !== year || calendar.getUTCMonth() !== month - 1 || calendar.getUTCDate() !== day
    || month < 1 || month > 12 || hour > 23 || minute > 59 || second > 59
    || offsetHour > 23 || offsetMinute > 59 || Number.isNaN(Date.parse(value))) {
    throw new ContractError(path, 'expected an ISO-8601 timestamp');
  }
  return value;
}

function capability(value: unknown, path: string): Capability {
  const row = object(value, path);
  const state = row.state;
  if (state !== 'supported' && state !== 'unavailable' && state !== 'forbidden') {
    throw new ContractError(`${path}.state`, 'unknown capability state');
  }
  if (typeof row.reason !== 'string' || (state !== 'supported' && row.reason.trim() === '')) {
    throw new ContractError(`${path}.reason`, 'reason is required unless supported');
  }
  return { state, reason: row.reason };
}

function schemaVersion(row: Record<string, unknown>): void {
  if (row.schema_version !== 1) throw new ContractError('schema_version', 'SCHEMA_VERSION: expected 1');
}

export function parseExecution(value: unknown): ExecutionDTO {
  const row = object(value, '');
  schemaVersion(row);
  const runStatus = nonEmpty(row.run_status, 'run_status');
  const allowed: readonly string[] = ['queued', 'running', 'waiting_user', 'reconciling', 'succeeded', 'failed', 'canceled'];
  if (!allowed.includes(runStatus)) throw new ContractError('run_status', 'unknown run status');
  if (row.driver !== 'platform' && row.driver !== 'paseo') throw new ContractError('driver', 'unknown execution driver');
  const capabilities = object(row.capabilities, 'capabilities');
  const parsedCapabilities: Record<string, Capability> = {};
  for (const [name, value] of Object.entries(capabilities)) parsedCapabilities[name] = capability(value, `capabilities.${name}`);
  return {
    schema_version: 1,
    run_id: nonEmpty(row.run_id, 'run_id'),
    session_id: nonEmpty(row.session_id, 'session_id'),
    revision: safeInteger(row.revision, 'revision', 0),
    driver: row.driver,
    run_status: runStatus as RunStatus,
    execution_status: nonEmpty(row.execution_status, 'execution_status'),
    settlement_status: nonEmpty(row.settlement_status, 'settlement_status'),
    seq: safeInteger(row.seq, 'seq', 0),
    capabilities: parsedCapabilities,
  };
}

export function parseExecutionEvent(value: unknown): ExecutionEvent {
  const row = object(value, '');
  schemaVersion(row);
  if (typeof row.payload !== 'object' || row.payload === null || Array.isArray(row.payload)) {
    throw new ContractError('payload', 'expected an object');
  }
  return {
    schema_version: 1,
    run_id: nonEmpty(row.run_id, 'run_id'),
    attempt_id: typeof row.attempt_id === 'string' ? row.attempt_id : (() => { throw new ContractError('attempt_id', 'expected a string'); })(),
    seq: safeInteger(row.seq, 'seq', 1),
    type: nonEmpty(row.type, 'type'),
    occurred_at: isoTime(row.occurred_at, 'occurred_at'),
    payload: row.payload as Record<string, unknown>,
  };
}

export function parseExecutionSnapshot(value: unknown): ExecutionSnapshot {
  const row = object(value, '');
  if (!Array.isArray(row.events)) throw new ContractError('events', 'expected an array');
  const execution = parseExecution(row.execution);
  const events = row.events.map(parseExecutionEvent);
  for (const [index, event] of events.entries()) {
    if (event.run_id !== execution.run_id) throw new ContractError(`events[${index}].run_id`, 'must match execution.run_id');
  }
  return { execution, watermark: safeInteger(row.watermark, 'watermark', 0), events };
}
