export class ContractError extends Error {
  readonly path: string;

  constructor(path: string, message: string) {
    super(`${path}: ${message}`);
    this.name = 'ContractError';
    this.path = path;
  }
}

export interface ApiErrorPayload {
  code: string;
  message: string;
  requestId?: string;
  details?: unknown;
}

export interface KnowledgeBase {
  id: string;
  name: string;
  type?: string;
  tenant_id?: string | number;
  [key: string]: unknown;
}

export interface KnowledgeBaseListResponse {
  success: true;
  data: KnowledgeBase[];
}

function requireNonEmptyString(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new ContractError(path, 'expected a non-empty string');
  }
  return value;
}

export function parseApiErrorPayload(value: unknown): ApiErrorPayload {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object');
  }
  const row = value as Record<string, unknown>;
  const code = requireNonEmptyString(row.code, 'code');
  const message = requireNonEmptyString(row.message, 'message');
  const requestId = row.requestId ?? row.request_id;
  if (requestId !== undefined && typeof requestId !== 'string') {
    throw new ContractError('requestId', 'expected a string');
  }
  return {
    code,
    message,
    ...(requestId === undefined ? {} : { requestId }),
    ...(row.details === undefined ? {} : { details: row.details }),
  };
}

function validateTenantId(value: unknown, path: string): string | number {
  if (typeof value === 'string' && value.trim() !== '') return value;
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return value;
  throw new ContractError(path, 'expected a decimal string or safe non-negative integer');
}

export function parseKnowledgeBaseListResponse(value: unknown): KnowledgeBase[] {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError('', 'expected an object envelope');
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Array.isArray(envelope.data)) throw new ContractError('data', 'expected an array');

  return envelope.data.map((item, index) => {
    const path = `data[${index}]`;
    if (typeof item !== 'object' || item === null || Array.isArray(item)) {
      throw new ContractError(path, 'expected an object');
    }
    const row = item as Record<string, unknown>;
    const result: KnowledgeBase = {
      ...row,
      id: requireNonEmptyString(row.id, `${path}.id`),
      name: requireNonEmptyString(row.name, `${path}.name`),
    };
    if ('tenant_id' in row && row.tenant_id !== undefined && row.tenant_id !== null) {
      result.tenant_id = validateTenantId(row.tenant_id, `${path}.tenant_id`);
    }
    if ('type' in row && row.type !== undefined && typeof row.type !== 'string') {
      throw new ContractError(`${path}.type`, 'expected a string');
    }
    return result;
  });
}
