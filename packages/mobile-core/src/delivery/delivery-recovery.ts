import { deliveryViewOf, type DeliveryReceiptView, type DeliveryRemoteRecord } from './delivery-view.ts';
import type { DeliveryRemote } from './delivery-reader.ts';
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

export interface DeliveryRecoveryRemote {
  dispatchDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
  resolveDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
}

export type DeliveryRecoveryErrorCode =
  | 'DELIVERY_SCOPE_CHANGED'
  | 'DELIVERY_STATE_CONFLICT'
  | 'DELIVERY_INVALID_INPUT'
  | 'DELIVERY_BACKEND';

export class DeliveryRecoveryError extends Error {
  readonly code: DeliveryRecoveryErrorCode;

  constructor(code: DeliveryRecoveryErrorCode, message: string) {
    super(message);
    this.name = 'DeliveryRecoveryError';
    this.code = code;
  }
}

export interface DeliveryRecovery {
  recover(input: { runId: string; deliveryId: string }): Promise<DeliveryReceiptView>;
}

function isStateConflict(error: unknown): boolean {
  if (typeof error !== 'object' || error === null) return false;
  const candidate = error as { status?: unknown; code?: unknown; body?: unknown };
  if (candidate.status === 409) return true;
  if (candidate.code === 'code_delivery_state_conflict') return true;
  if (typeof candidate.body === 'object' && candidate.body !== null) {
    return (candidate.body as { code?: unknown }).code === 'code_delivery_state_conflict';
  }
  return false;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function createDeliveryRecovery(ports: {
  remote: DeliveryRecoveryRemote & Pick<DeliveryRemote, 'delivery'>;
  lease(): ScopeLease | undefined;
}): DeliveryRecovery {
  return {
    async recover(input): Promise<DeliveryReceiptView> {
      // Fixed order: lease guard, read current state, route to exactly one recovery
      // write, translate API errors, then revalidate the lease before exposing it.
      const lease = ports.lease();
      if (lease === undefined || !leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'delivery recovery requires an active scope lease');
      }

      let current: DeliveryRemoteRecord | null;
      try {
        current = await ports.remote.delivery(input.runId);
      } catch (error) {
        if (!leaseActive(lease)) {
          throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while the delivery read was in flight');
        }
        throw new DeliveryRecoveryError('DELIVERY_BACKEND', errorMessage(error, 'delivery read failed'));
      }
      if (!leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while the delivery record was in flight');
      }
      if (current === null) {
        throw new DeliveryRecoveryError('DELIVERY_INVALID_INPUT', 'no delivery exists for the requested run');
      }

      if (current.state === 'delivered') return deliveryViewOf(current);
      if (current.state !== 'pushed' && current.state !== 'unknown') {
        throw new DeliveryRecoveryError(
          'DELIVERY_STATE_CONFLICT',
          `delivery in state ${current.state} is not eligible for recovery`,
        );
      }

      let recovered: DeliveryRemoteRecord;
      try {
        recovered = current.state === 'pushed'
          ? await ports.remote.dispatchDelivery(input)
          : await ports.remote.resolveDelivery(input);
      } catch (error) {
        if (!leaseActive(lease)) {
          throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while delivery recovery was in flight');
        }
        if (isStateConflict(error)) {
          throw new DeliveryRecoveryError('DELIVERY_STATE_CONFLICT', errorMessage(error, 'delivery state changed during recovery'));
        }
        throw new DeliveryRecoveryError('DELIVERY_BACKEND', errorMessage(error, 'delivery recovery failed'));
      }

      if (!leaseActive(lease)) {
        throw new DeliveryRecoveryError('DELIVERY_SCOPE_CHANGED', 'scope changed while delivery recovery was in flight');
      }
      return deliveryViewOf(recovered);
    },
  };
}
