import { parseWorkbenchOverview, type WorkbenchOverview } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

/**
 * 工作台聚合读模型 SDK（MX-013）：GET /api/v1/workbench/overview。
 * 一次调用返回计数/进行中/待处理/成果/as_of——页面不得逐会话补读（N+1 禁止）。
 * 错误语义同 executions：401 单飞刷新、403 隐藏敏感内容、5xx 错误态（StateView）。
 */
type Request = (input: ClientRequest) => Promise<unknown>;

export interface OverviewLoader {
  overview(): Promise<WorkbenchOverview>;
}

export function createOverviewApi(request: Request): OverviewLoader {
  return {
    async overview(): Promise<WorkbenchOverview> {
      const response = await request({ method: 'GET', path: '/api/v1/workbench/overview' });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('overview response must be a success envelope');
      }
      const envelope = response as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
        throw new Error('overview response.success must be true');
      }
      return parseWorkbenchOverview(envelope.data);
    },
  };
}
