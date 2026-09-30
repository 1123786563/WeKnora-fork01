import { deliveryViewOf, type DeliveryReceiptView, type DeliveryRemoteRecord } from './delivery-view.ts';
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

export interface DeliveryRemote {
  delivery(runId: string): Promise<DeliveryRemoteRecord | null>;
}

export type DeliveryReaderErrorCode = 'DELIVERY_SCOPE_CHANGED' | 'DELIVERY_BACKEND';

export class DeliveryReaderError extends Error {
  readonly code: DeliveryReaderErrorCode;
  constructor(code: DeliveryReaderErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

export interface DeliveryReader {
  read(runId: string): Promise<DeliveryReceiptView | undefined>;
}

/** scope-guard 读器：无有效 lease 拒绝；lease 在途失效时丢弃迟到结果。 */
export function createDeliveryReader(ports: { remote: DeliveryRemote; lease(): ScopeLease | undefined }): DeliveryReader {
  return {
    async read(runId: string): Promise<DeliveryReceiptView | undefined> {
      const lease = ports.lease();
      if (lease === undefined || !leaseActive(lease)) {
        throw new DeliveryReaderError('DELIVERY_SCOPE_CHANGED', 'delivery reads require an active scope lease');
      }
      let record: DeliveryRemoteRecord | null;
      try {
        record = await ports.remote.delivery(runId);
      } catch (error) {
        throw new DeliveryReaderError('DELIVERY_BACKEND', error instanceof Error ? error.message : 'delivery read failed');
      }
      if (!leaseActive(lease)) {
        throw new DeliveryReaderError('DELIVERY_SCOPE_CHANGED', 'scope changed while the delivery record was in flight');
      }
      return record === null ? undefined : deliveryViewOf(record);
    },
  };
}
