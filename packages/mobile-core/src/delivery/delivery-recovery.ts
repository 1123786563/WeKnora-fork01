import { deliveryViewOf, type DeliveryReceiptView, type DeliveryRemoteRecord } from './delivery-view.ts';
import type { DeliveryRemote } from './delivery-reader.ts';
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

export interface DeliveryRecoveryRemote {
  dispatchDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
  resolveDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
}

export type DeliveryRecoveryErrorCode = 'DELIVERY_SCOPE_CHANGED' | 'DELIVERY_STATE_CONFLICT' | 'DELIVERY_INVALID_INPUT' | 'DELIVERY_BACKEND';

export class DeliveryRecoveryError extends Error {
  readonly code: DeliveryRecoveryErrorCode;
  constructor(code: DeliveryRecoveryErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

export interface DeliveryRecovery {
  recover(input: { runId: string; deliveryId: string }): Promise<DeliveryReceiptView>;
}

function isDeliveryStateConflict(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false;
  const value = error as { status?: unknown; code?: unknown; body?: { code?: unknown } };
  return value.status === 409 || value.code === 'code_delivery_state_conflict' || value.body?.code === 'code_delivery_state_conflict';
}

/** 固定恢复顺序：lease 前置守卫、读取现状、按状态路由、写失败翻译、lease 复验、投影返回。 */
export function createDeliveryRecovery(ports: {
  remote: DeliveryRecoveryRemote & Pick<DeliveryRemote, 'delivery'>;
  lease(): ScopeLease | undefined;
}): DeliveryRecovery {
  return {
    async recover(input): Promise<DeliveryReceiptView> {
      const lease = ports.lease();
      if (!leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'delivery recovery requires an active scope lease');
      }

      let current: DeliveryRemoteRecord | null;
      try {
        current = await ports.remote.delivery(input.runId);
      } catch (error) {
        throw new DeliveryRecoveryError('DELIVERY_BACKEND', error instanceof Error ? error.message : 'delivery read failed');
      }
      if (!leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while the delivery record was in flight');
      }
      if (current === null) {
        throw new DeliveryRecoveryError('DELIVERY_INVALID_INPUT', 'no delivery exists to recover');
      }
      if (current.id !== input.deliveryId) {
        throw new DeliveryRecoveryError('DELIVERY_INVALID_INPUT', 'the requested delivery does not match the run delivery');
      }

      if (current.state === 'delivered') return deliveryViewOf(current);
      let operation: 'dispatchDelivery' | 'resolveDelivery';
      if (current.state === 'pushed') operation = 'dispatchDelivery';
      else if (current.state === 'unknown') operation = 'resolveDelivery';
      else {
        throw new DeliveryRecoveryError('DELIVERY_STATE_CONFLICT', `delivery state ${current.state} is not recoverable`);
      }

      let updated: DeliveryRemoteRecord;
      try {
        updated = await ports.remote[operation](input);
      } catch (error) {
        if (isDeliveryStateConflict(error)) {
          throw new DeliveryRecoveryError('DELIVERY_STATE_CONFLICT', 'delivery state changed before recovery completed');
        }
        throw new DeliveryRecoveryError('DELIVERY_BACKEND', error instanceof Error ? error.message : 'delivery recovery failed');
      }
      if (!leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while delivery recovery was in flight');
      }
      return deliveryViewOf(updated);
    },
  };
}
