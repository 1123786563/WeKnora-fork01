import { ContractError } from '../index.ts';

export interface AgentVersion {
  id: string;
  agent_id: string;
  version_number: number;
  source_sha256: string;
  frozen_by: string;
  created_at: string;
  agent?: Record<string, unknown>;
  [key: string]: unknown;
}

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function stringValue(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string') throw new ContractError(`${path}.${key}`, 'expected a string');
  return value;
}

function successData(value: unknown): unknown {
  const envelope = object(value, '');
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new ContractError('data', 'is required');
  return envelope.data;
}

function parseVersion(value: unknown, path: string): AgentVersion {
  const row = object(value, path);
  const versionNumber = row.version_number;
  if (typeof versionNumber !== 'number' || !Number.isSafeInteger(versionNumber) || versionNumber < 1) {
    throw new ContractError(`${path}.version_number`, 'expected a positive safe integer');
  }
  const sourceSHA256 = requiredString(row, 'source_sha256', path);
  if (!/^[a-f\d]{64}$/i.test(sourceSHA256)) throw new ContractError(`${path}.source_sha256`, 'expected a SHA-256 hex digest');
  const agent = row.agent;
  if (agent !== undefined && (typeof agent !== 'object' || agent === null || Array.isArray(agent))) {
    throw new ContractError(`${path}.agent`, 'expected an object when present');
  }
  return {
    ...row,
    id: requiredString(row, 'id', path),
    agent_id: requiredString(row, 'agent_id', path),
    version_number: versionNumber,
    source_sha256: sourceSHA256,
    // Machine principals may freeze a version without a user ID; the HTTP
    // handler deliberately serializes that trusted empty actor as "".
    frozen_by: stringValue(row, 'frozen_by', path),
    created_at: requiredString(row, 'created_at', path),
    ...(agent === undefined ? {} : { agent: agent as Record<string, unknown> }),
  } as AgentVersion;
}

export function parseAgentVersion(value: unknown): AgentVersion {
  return parseVersion(successData(value), 'data');
}

export function parseAgentVersionListResponse(value: unknown): AgentVersion[] {
  const data = successData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((item, index) => parseVersion(item, `data[${index}]`));
}
