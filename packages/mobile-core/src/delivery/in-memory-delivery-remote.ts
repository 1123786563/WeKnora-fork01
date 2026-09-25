import type { DeliveryRemote } from './delivery-reader.ts';
import type { DeliveryRemoteRecord } from './delivery-view.ts';

/** 场景 Adapter：按 runId 脚本化返回记录（null=无交付）。 */
export function createScenarioDeliveryRemote(script: { runId: string; record: DeliveryRemoteRecord | null }[]): DeliveryRemote {
  return {
    async delivery(runId: string): Promise<DeliveryRemoteRecord | null> {
      for (const entry of script) {
        if (entry.runId === runId) return entry.record;
      }
      return null;
    },
  };
}
