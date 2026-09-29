import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MaterialRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

export interface MaterialRemoteArtifact {
  id: string; index: number; name: string; mime: string; version: string; size: number;
  sourceRun: string; createdAt?: string; digest?: string;
}
export interface MaterialRemoteList { runId: string; artifacts: MaterialRemoteArtifact[]; terminalAvailable: boolean }
export interface MaterialRemoteGrant { url: string; expiresAt: string; artifact: MaterialRemoteArtifact }
export interface MaterialRemoteTerminalLine { seq: number; occurredAt: string; stream: 'stdout' | 'stderr'; text: string }
/** 末页语义（B3-F52/F36）：nextCursor 省略 = 末页（Go 端恒发数字，转译责任在本适配器）。 */
export interface MaterialRemoteTerminalPage { lines: MaterialRemoteTerminalLine[]; nextCursor?: number }
export interface MaterialRemoteEvent { seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }

/** 与 mobile-core MaterialBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MaterialRemote {
  list(runId: string): Promise<MaterialRemoteList>;
  signedUrl(input: { runId: string; index: number }): Promise<MaterialRemoteGrant>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<MaterialRemoteTerminalPage>;
  events(runId: string): Promise<MaterialRemoteEvent[]>;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('material response must be a success envelope');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('material response.success must be true with data');
  }
  const data = envelope.data;
  if (typeof data !== 'object' || data === null || Array.isArray(data)) throw new Error('material response data must be an object');
  return data as Record<string, unknown>;
}

function stringRow(row: unknown, at: string): Record<string, unknown> {
  if (typeof row !== 'object' || row === null) throw new Error(`${at} must be an object`);
  return row as Record<string, unknown>;
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

export function createMobileMaterialRemote(options: MaterialRemoteOptions): MaterialRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;

  const artifactFrom = (row: unknown): MaterialRemoteArtifact => {
    const r = stringRow(row, 'artifact');
    return {
      id: String(r.id ?? ''),
      index: typeof r.index === 'number' ? r.index : Number(r.index ?? 0),
      name: String(r.name ?? ''),
      mime: String(r.mime ?? ''),
      version: String(r.version ?? ''),
      size: typeof r.size === 'number' ? r.size : Number(r.size ?? 0),
      sourceRun: String(r.source_run ?? ''),
      ...(optionalString(r.created_at) === undefined ? {} : { createdAt: optionalString(r.created_at) }),
      ...(optionalString(r.digest) === undefined ? {} : { digest: optionalString(r.digest) }),
    };
  };

  return {
    async list(runId) {
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts` }));
      const items = Array.isArray(data.items) ? data.items : [];
      const terminal = typeof data.terminal === 'object' && data.terminal !== null
        ? (data.terminal as { available?: unknown }).available === true
        : false;
      return { runId, artifacts: items.map(artifactFrom), terminalAvailable: terminal };
    },
    async signedUrl({ runId, index }) {
      const data = unwrap(await request({ method: 'POST', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts/${index}/signed-url` }));
      return {
        url: String(data.url ?? ''),
        expiresAt: String(data.expires_at ?? ''),
        artifact: artifactFrom(data.artifact),
      };
    },
    async terminalLog({ runId, after, limit }) {
      const data = unwrap(await request({
        method: 'GET',
        path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/terminal-log?after=${after}&limit=${limit}`,
      }));
      const lines = Array.isArray(data.lines) ? data.lines : [];
      return {
        lines: lines.map((row) => {
          const r = stringRow(row, 'terminal line');
          const stream = r.stream === 'stderr' ? 'stderr' : 'stdout';
          return { seq: typeof r.seq === 'number' ? r.seq : Number(r.seq ?? 0), occurredAt: String(r.occurred_at ?? ''), stream, text: String(r.text ?? '') };
        }),
        // 末页语义（B3-F52，对齐 Go 端 workbench_terminal_log 的「游标未推进仍发数字」）：
        // next <= after 即末页 → 省略键（undefined），mobile-core 据此判无下一页。
        ...((typeof data.next_cursor === 'number' ? data.next_cursor : Number(data.next_cursor ?? after)) > after
          ? { nextCursor: typeof data.next_cursor === 'number' ? data.next_cursor : Number(data.next_cursor ?? after) }
          : {}),
      };
    },
    async events(runId) {
      // Evidence 引用复用既有 snapshot 端点（ADR-0006：REST 取权威 Snapshot）。
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/snapshot` }));
      const events = Array.isArray(data.events) ? data.events : [];
      return events.map((row) => {
        const r = stringRow(row, 'event');
        return {
          seq: typeof r.seq === 'number' ? r.seq : Number(r.seq ?? 0),
          type: String(r.type ?? ''),
          occurredAt: String(r.occurred_at ?? ''),
          payload: (typeof r.payload === 'object' && r.payload !== null ? r.payload : {}) as Record<string, unknown>,
        };
      });
    },
  };
}
