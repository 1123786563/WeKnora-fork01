// Shared write-recovery protocol helpers for the career surfaces.
//
// One implementation of the red line: an ambiguous transport failure leaves
// the durable write outcome unknown and must recover under the original
// request ID, while a typed rejection (a 4xx contract answer) is definite and
// must never route into receipt recovery. Pages layer their endpoint-specific
// error-code whitelists on top of this base contract.

export type CareerErrorDetails = { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string }

/**
 * OCR ocr2-067/082：回执 requestId（或业务键）与本次请求不匹配是「确定性
 * 协议错误」，不得与网络层抛出的无 code/status 的 TypeError 混淆——后者才
 * 是 outcome unknown。此前两态都是裸 TypeError，被 isUncertainWrite 判
 * unknown 路由进回执恢复，违反「确定性协议错误不得路由进回执恢复」红线。
 * 消费方应在 catch 顶部 instanceof 识别并直接置 'error'。
 */
export class ReceiptMismatchError extends Error {
 constructor(message = '回执与本次请求不匹配') {
  super(message)
  this.name = 'ReceiptMismatchError'
 }
}

export const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`

export function errorDetails(cause: unknown): CareerErrorDetails {
 const error = cause as { code?: string; requestId?: string; currentRevision?: string | number; status?: number; message?: string }
 const currentRevision = typeof error?.currentRevision === 'number' ? error.currentRevision : undefined
 return { code: error?.code, requestId: error?.requestId, currentRevision, status: error?.status, message: error?.message || '请求未完成' }
}

export function isUncertainWrite(cause: unknown): boolean {
 if (cause instanceof ReceiptMismatchError) return false
 const error = errorDetails(cause)
 if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
 if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
 if (error.status !== undefined) return error.status >= 500 || error.status < 400
 return true
}
