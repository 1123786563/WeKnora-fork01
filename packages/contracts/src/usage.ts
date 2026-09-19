// SP12 usage aggregation wire contracts.
// Shapes mirror internal/types/interfaces/user_usage.go UsageAggregate
// (internal/handler/usage.go): /usage/me rows omit user_id (omitempty, the
// caller's own buckets), /admin/usage/by-user rows carry it. The CSV export is
// a binary response handled by the api-client's requestBinary channel, so it
// has no JSON contract here. Extra backend fields pass through via index
// signatures / spreads per index.ts conventions.
import { ContractError } from './index.ts';

export interface UsageRow {
  window_start: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  cost_microcredits: number;
  [key: string]: unknown;
}

/** by-user rows are grouped per user; a user_id column is always present. */
export interface UsageByUserRow extends UsageRow {
  user_id: string;
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

function parseUsageRow(value: unknown, path: string): UsageRow {
  const row = parseObject(value, path);
  return {
    ...row,
    window_start: requiredString(row.window_start, `${path}.window_start`),
    model: requiredString(row.model, `${path}.model`),
    input_tokens: requiredNonNegativeInteger(row.input_tokens, `${path}.input_tokens`),
    output_tokens: requiredNonNegativeInteger(row.output_tokens, `${path}.output_tokens`),
    cache_read_tokens: requiredNonNegativeInteger(row.cache_read_tokens, `${path}.cache_read_tokens`),
    cache_write_tokens: requiredNonNegativeInteger(row.cache_write_tokens, `${path}.cache_write_tokens`),
    cost_microcredits: requiredNonNegativeInteger(row.cost_microcredits, `${path}.cost_microcredits`),
  } as UsageRow;
}

function parseUsageByUserRow(value: unknown, path: string): UsageByUserRow {
  const row = parseUsageRow(value, path);
  return {
    ...row,
    user_id: requireNonEmptyString(row.user_id, `${path}.user_id`),
  } as UsageByUserRow;
}

function parseRowList<T>(value: unknown, parseItem: (item: unknown, path: string) => T, path: string): { items: T[] } {
  const result = successEnvelope(value);
  if (!Array.isArray(result.data)) throw new ContractError('data', 'expected an array');
  return { items: result.data.map((item, index) => parseItem(item, `${path}[${index}]`)) };
}

export function parseMyUsageResponse(value: unknown): { items: UsageRow[] } {
  return parseRowList(value, parseUsageRow, 'data');
}

export function parseUsageByUserResponse(value: unknown): { items: UsageByUserRow[] } {
  return parseRowList(value, parseUsageByUserRow, 'data');
}
