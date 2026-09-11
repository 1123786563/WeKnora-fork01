import {
  parseCommercialSummary,
  parseOrderView,
  parseQuoteView,
  parseRefundView,
  type CommercialSummary,
  type CreateOrderInput,
  type OrderView,
  type QuoteInput,
  type QuoteView,
  type RefundInput,
  type RefundView,
} from '@weknora/contracts';
import type { ClientRequest } from './client.ts';
import { ApiError } from './errors.ts';

function unwrap(value: unknown): unknown {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected a commercial response envelope' });
  }
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) {
    const error = typeof envelope.error === 'object' && envelope.error !== null
      ? envelope.error as Record<string, unknown>
      : {};
    throw new ApiError({
      code: typeof error.code === 'string' && error.code !== '' ? error.code : 'COMMERCIAL_ERROR',
      message: typeof error.message === 'string' && error.message !== '' ? error.message : 'Commercial request failed',
      requestId: typeof error.requestId === 'string' ? error.requestId : undefined,
    });
  }
  if (envelope.data === undefined) {
    throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Commercial response envelope is missing data' });
  }
  return envelope.data;
}

export function createCommercialApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async getOrder(id: string, signal?: AbortSignal): Promise<OrderView> {
      return parseOrderView(unwrap(await request({
        method: 'GET',
        path: `/api/v1/commercial/orders/${encodeURIComponent(id)}`,
        signal,
      })));
    },
    async summary(signal?: AbortSignal): Promise<CommercialSummary> {
      return parseCommercialSummary(unwrap(await request({ method: 'GET', path: '/api/v1/commercial/summary', signal })));
    },
    async quote(input: QuoteInput, signal?: AbortSignal): Promise<QuoteView> {
      return parseQuoteView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/quotes', body: input, signal })));
    },
    async createOrder(input: CreateOrderInput, signal?: AbortSignal): Promise<OrderView> {
      return parseOrderView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/orders', body: input, signal })));
    },
    async requestRefund(input: RefundInput, signal?: AbortSignal): Promise<{ id: string; state: string }> {
      const data = unwrap(await request({ method: 'POST', path: '/api/v1/commercial/refunds', body: input, signal }));
      if (typeof data !== 'object' || data === null || Array.isArray(data)) {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Expected a refund response object' });
      }
      const row = data as Record<string, unknown>;
      const id = row.id;
      const state = row.state;
      if (typeof id !== 'string' || id === '' || typeof state !== 'string' || state === '') {
        throw new ApiError({ code: 'INVALID_RESPONSE', message: 'Invalid refund response fields' });
      }
      return { id, state };
    },
    async getRefund(id: string, signal?: AbortSignal): Promise<RefundView> {
      return parseRefundView(unwrap(await request({
        method: 'GET',
        path: `/api/v1/commercial/refunds/${encodeURIComponent(id)}`,
        signal,
      })));
    },
    async reviewRefund(id: string, decision: 'approve' | 'reject', expectedVersion: number, signal?: AbortSignal): Promise<RefundView> {
      return parseRefundView(unwrap(await request({
        method: 'POST',
        path: `/api/v1/commercial/refunds/${encodeURIComponent(id)}/review`,
        body: { decision, expected_version: expectedVersion },
        signal,
      })));
    },
  };
}
