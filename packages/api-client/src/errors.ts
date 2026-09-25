export interface ApiErrorInit {
  status?: number;
  code: string;
  message: string;
  requestId?: string;
  details?: unknown;
  cause?: unknown;
}

export function createAbortError(message = 'Request was cancelled'): Error {
  const DOMExceptionConstructor = (globalThis as typeof globalThis & {
    DOMException?: new (message?: string, name?: string) => Error;
  }).DOMException;
  if (DOMExceptionConstructor) return new DOMExceptionConstructor(message, 'AbortError');
  const error = new Error(message);
  error.name = 'AbortError';
  return error;
}

export function isNamedError(error: unknown, name: string): boolean {
  return error instanceof Error && error.name === name;
}

export class ApiError extends Error {
  readonly status?: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details?: unknown;

  constructor(init: ApiErrorInit) {
    super(init.message, { cause: init.cause });
    this.name = 'ApiError';
    this.status = init.status;
    this.code = init.code;
    this.requestId = init.requestId;
    this.details = init.details;
  }
}

function errorCodeValue(value: unknown): string | undefined {
  if (typeof value === 'string' && value !== '') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return undefined;
}

export function errorFromResult(status: number, body: unknown, headers: Record<string, string> = {}): ApiError {
  const record = typeof body === 'object' && body !== null ? body as Record<string, unknown> : undefined;
  const nested = typeof record?.error === 'object' && record.error !== null
    ? record.error as Record<string, unknown>
    : undefined;
  const message = typeof nested?.message === 'string'
    ? nested.message
    : typeof record?.message === 'string'
      ? record.message
    : typeof record?.error === 'string' && record.error.trim() ? record.error
    : typeof body === 'string' && body.trim() ? body
      : `Request failed with status ${status}`;
  const code = status === 413
    ? 'PAYLOAD_TOO_LARGE'
    : errorCodeValue(nested?.code) ?? errorCodeValue(record?.code) ?? `HTTP_${status}`;
  const requestId = typeof nested?.requestId === 'string'
    ? nested.requestId
    : typeof nested?.request_id === 'string'
      ? nested.request_id
      : typeof record?.requestId === 'string'
        ? record.requestId
    : typeof headers['x-request-id'] === 'string' ? headers['x-request-id'] : undefined;
  // (R1-24) A closed-vocabulary top-level reason token (e.g. the purchase
  // 503 envelope's {"error":..., "reason":"unreachable"}) must survive the
  // non-2xx throw: merge it into details so callers can map the closed
  // token instead of showing the raw English message to the user.
  const reason = typeof record?.reason === 'string' && record.reason !== '' ? record.reason : undefined;
  let details: unknown = nested?.details ?? record?.details;
  if (reason !== undefined) {
    const base = typeof details === 'object' && details !== null ? details as Record<string, unknown> : {};
    details = { ...base, reason };
  }
  return new ApiError({ status, code, message, requestId, details });
}
