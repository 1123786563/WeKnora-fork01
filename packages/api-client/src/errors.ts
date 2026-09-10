export interface ApiErrorInit {
  status?: number;
  code: string;
  message: string;
  requestId?: string;
  details?: unknown;
  cause?: unknown;
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

export function errorFromResult(status: number, body: unknown, headers: Record<string, string> = {}): ApiError {
  const record = typeof body === 'object' && body !== null ? body as Record<string, unknown> : undefined;
  const nested = typeof record?.error === 'object' && record.error !== null
    ? record.error as Record<string, unknown>
    : undefined;
  const message = typeof nested?.message === 'string'
    ? nested.message
    : typeof record?.message === 'string'
      ? record.message
    : typeof body === 'string' && body.trim() ? body
      : `Request failed with status ${status}`;
  const code = status === 413
    ? 'PAYLOAD_TOO_LARGE'
    : typeof nested?.code === 'string' ? nested.code
      : typeof record?.code === 'string' ? record.code : `HTTP_${status}`;
  const requestId = typeof nested?.requestId === 'string'
    ? nested.requestId
    : typeof nested?.request_id === 'string'
      ? nested.request_id
      : typeof record?.requestId === 'string'
        ? record.requestId
    : typeof headers['x-request-id'] === 'string' ? headers['x-request-id'] : undefined;
  return new ApiError({ status, code, message, requestId, details: nested?.details ?? record?.details });
}
