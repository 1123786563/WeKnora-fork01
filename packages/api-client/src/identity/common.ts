import type { ClientRequest } from '../client.ts';

export type IdentityRequest = (input: ClientRequest) => Promise<unknown>;
export type JsonRecord = Record<string, unknown>;

export function record(value: unknown, path: string): JsonRecord {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as JsonRecord;
}

export function stringValue(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}

export function stringValueAllowEmpty(value: unknown, path: string): string {
  if (typeof value !== 'string') throw new Error(`${path} must be a string`);
  return value;
}

export function numberValue(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new Error(`${path} must be a safe integer`);
  return value;
}

export function booleanValue(value: unknown, path: string): boolean {
  if (typeof value !== 'boolean') throw new Error(`${path} must be a boolean`);
  return value;
}

export function success(value: unknown, path: string): JsonRecord {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  return envelope;
}

export function action(value: unknown, path = 'response'): void {
  if (value === undefined) return;
  success(value, path);
}

export function encoded(value: string | number, path: string): string {
  const raw = String(value);
  if (raw.trim() === '') throw new Error(`${path} must not be empty`);
  return encodeURIComponent(raw);
}

export function query(path: string, values: Array<[string, string | number | boolean | undefined]>): string {
  const params = new URLSearchParams();
  for (const [key, value] of values) if (value !== undefined) params.set(key, String(value));
  const suffix = params.toString();
  return suffix ? `${path}?${suffix}` : path;
}

export function withSignal(input: ClientRequest, signal?: AbortSignal): ClientRequest {
  return signal === undefined ? input : { ...input, signal };
}

export function array(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`${path} must be an array`);
  return value;
}

export function dataRecord(value: unknown, path: string): JsonRecord {
  return record(success(value, path).data, `${path}.data`);
}

export function dataArray(value: unknown, path: string): unknown[] {
  return array(success(value, path).data, `${path}.data`);
}

export function optionalString(value: unknown, path: string): string | undefined {
  if (value === undefined || value === null) return undefined;
  return stringValue(value, path);
}

export function optionalNumber(value: unknown, path: string): number | undefined {
  if (value === undefined || value === null) return undefined;
  return numberValue(value, path);
}
