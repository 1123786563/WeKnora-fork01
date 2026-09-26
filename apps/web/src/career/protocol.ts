// Shared write-recovery protocol helpers for the career surfaces.
//
// One implementation of the red line: an ambiguous transport failure leaves
// the durable write outcome unknown and must recover under the original
// request ID, while a typed rejection (a 4xx contract answer) is definite and
// must never route into receipt recovery. Pages layer their endpoint-specific
// error-code whitelists on top of this base contract.

export type CareerErrorDetails = { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string }

export const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`

export function errorDetails(cause: unknown): CareerErrorDetails {
 const error = cause as { code?: string; requestId?: string; currentRevision?: string | number; status?: number; message?: string }
 const currentRevision = typeof error?.currentRevision === 'number' ? error.currentRevision : undefined
 return { code: error?.code, requestId: error?.requestId, currentRevision, status: error?.status, message: error?.message || '请求未完成' }
}

export function isUncertainWrite(cause: unknown): boolean {
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 if (error.status !== undefined) return error.status >= 500 || error.status < 400
 return true
}
