import { parseCodeDeliveryRecord, type CodeDeliveryRecord } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface CodeDeliveryRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 与 mobile-core DeliveryRemote 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileCodeDeliveryRemote {
  delivery(runId: string): Promise<CodeDeliveryRecord | null>;
  dispatchDelivery(input: { runId: string; deliveryId: string }): Promise<CodeDeliveryRecord>;
  resolveDelivery(input: { runId: string; deliveryId: string }): Promise<CodeDeliveryRecord>;
}

const DELIVERY_NOT_FOUND = 'code_delivery_not_found';

/**
 * 404 code_delivery_not_found → null（Run 可读但尚未准备交付的正常态）。
 * 真实通道抛 ApiError（顶层 .status/.code，errors.ts errorFromResult）；
 * 测试替身与部分适配器把原始 body 挂在 .body 上——两种形状都识别，
 * 其余 404 code 与全部非 404 失败照常 reject。
 */
function isDeliveryNotFound(error: unknown): boolean {
  const shaped = error as { status?: unknown; code?: unknown; body?: { code?: unknown } } | null;
  if (shaped === null || typeof shaped !== 'object') return false;
  if (shaped.status !== 404) return false;
  return shaped.code === DELIVERY_NOT_FOUND || shaped.body?.code === DELIVERY_NOT_FOUND;
}

function deliveryRecordOf(response: unknown): CodeDeliveryRecord {
  const envelope = response as { success?: unknown; data?: unknown };
  if (envelope?.success !== true || typeof envelope.data !== 'object' || envelope.data === null) {
    throw new Error('code delivery response must be a success envelope');
  }
  const delivery = (envelope.data as { delivery?: unknown }).delivery;
  if (delivery === undefined) throw new Error('code delivery response must carry a delivery row');
  return parseCodeDeliveryRecord(delivery);
}

/** GET /workbench/executions/:run_id/delivery —— 交付追溯读面（授权通道）。 */
export function createMobileCodeDeliveryRemote(options: CodeDeliveryRemoteOptions): MobileCodeDeliveryRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async delivery(runId: string): Promise<CodeDeliveryRecord | null> {
      try {
        const response = await request({
          method: 'GET',
          path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/delivery`,
        });
        return deliveryRecordOf(response);
      } catch (error) {
        if (isDeliveryNotFound(error)) return null;
        throw error;
      }
    },
    async dispatchDelivery({ runId, deliveryId }): Promise<CodeDeliveryRecord> {
      const response = await request({
        method: 'POST',
        path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/delivery/${encodeURIComponent(deliveryId)}/dispatch`,
        body: {},
      });
      return deliveryRecordOf(response);
    },
    async resolveDelivery({ runId, deliveryId }): Promise<CodeDeliveryRecord> {
      const response = await request({
        method: 'POST',
        path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/delivery/${encodeURIComponent(deliveryId)}/resolve`,
        body: {},
      });
      return deliveryRecordOf(response);
    },
  };
}
