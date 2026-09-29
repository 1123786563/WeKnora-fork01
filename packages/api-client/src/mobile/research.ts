import type { ClientRequest } from '../client.ts';
import { ApiError } from '../errors.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';
import { parseAnnotationListResponse, parseResearchListResponse } from '@weknora/contracts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface ResearchRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 语义行（camelCase）：与 mobile-core research 模块的 Port 行结构逐字一致。 */
export interface ResearchDelegationRow {
  delegationId: string;
  runId: string;
  sessionId: string;
  objective: string;
  sources: string[];
  status: 'assigned' | 'completed';
  summary?: string;
  createdAt: string;
}

export interface ResearchAnnotationRow {
  annotationId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  authorId: string;
  createdAt: string;
}

/** 与 mobile-core ResearchBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface ResearchRemote {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw coded('RESEARCH_BACKEND');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw coded('RESEARCH_BACKEND');
  }
  return envelope.data as Record<string, unknown>;
}

function coded(message: string, cause?: unknown): Error {
  const error = new Error(message, cause === undefined ? undefined : { cause });
  (error as unknown as { code?: string }).code = message;
  return error;
}

/** annotate 的 409 有唯一含义（base_version 钉版冲突）；其余失败统一 RESEARCH_BACKEND。 */
function annotateFailure(error: unknown): Error {
  if (error instanceof ApiError && error.status === 409) return coded('RESEARCH_BASE_VERSION_CONFLICT', error);
  return coded('RESEARCH_BACKEND', error);
}

/** catch 处理器必须「重抛」而不是返回：返回错误对象会把 promise 变为 resolved。 */
function rethrow(map: (error: unknown) => Error): (error: unknown) => never {
  return (error: unknown) => { throw map(error); };
}

/** GET 通路的信封校验 + 失败映射（原始响应必须先过 unwrap 再交给解析器）。 */
async function getEnvelope(request: Request, path: string): Promise<Record<string, unknown>> {
  try {
    return unwrap(await request({ method: 'GET', path }));
  } catch (error) {
    if ((error as { code?: unknown } | null)?.code === 'RESEARCH_BACKEND') throw error;
    throw coded('RESEARCH_BACKEND', error);
  }
}

function delegationFrom(value: unknown): ResearchDelegationRow {
  const wire = parseResearchListResponse({ success: true, data: { items: [value] } }).items[0]!;
  return {
    delegationId: wire.delegation_id,
    runId: wire.run_id,
    sessionId: wire.session_id,
    objective: wire.objective,
    sources: wire.sources,
    status: wire.status,
    summary: wire.summary,
    createdAt: wire.created_at,
  };
}

function annotationFrom(value: unknown): ResearchAnnotationRow {
  const wire = parseAnnotationListResponse({ success: true, data: { items: [value] } }).items[0]!;
  return {
    annotationId: wire.annotation_id,
    runId: wire.run_id,
    materialId: wire.material_id,
    baseVersion: wire.base_version,
    body: wire.body,
    authorId: wire.author_id,
    createdAt: wire.created_at,
  };
}

export function createMobileResearchRemote(options: ResearchRemoteOptions): ResearchRemote {
  requireDeploymentOrigin(options.origin);
  const researchPath = (runId: string): string => `/api/v1/workbench/executions/${encodeURIComponent(runId)}/research`;
  const annotationsPath = (runId: string): string => `/api/v1/workbench/executions/${encodeURIComponent(runId)}/annotations`;
  return {
    async delegate(input) {
      const data = unwrap(await options.request({ method: 'POST', path: researchPath(input.runId), body: { objective: input.objective, sources: input.sources } }).catch(rethrow((error) => coded('RESEARCH_BACKEND', error))));
      return delegationFrom(data.delegation);
    },
    async list(runId) {
      const data = await getEnvelope(options.request, researchPath(runId));
      const wire = parseResearchListResponse({ success: true, data });
      return { runId, delegations: wire.items.map((row) => ({
        delegationId: row.delegation_id, runId: row.run_id, sessionId: row.session_id,
        objective: row.objective, sources: row.sources, status: row.status,
        summary: row.summary, createdAt: row.created_at,
      })) };
    },
    async complete(input) {
      const data = unwrap(await options.request({ method: 'POST', path: `${researchPath(input.runId)}/${encodeURIComponent(input.delegationId)}/summary`, body: { summary: input.summary } }).catch(rethrow((error) => coded('RESEARCH_BACKEND', error))));
      return delegationFrom(data.delegation);
    },
    async annotate(input) {
      const data = unwrap(await options.request({ method: 'POST', path: annotationsPath(input.runId), body: { material_id: input.materialId, base_version: input.baseVersion, body: input.body } }).catch(rethrow(annotateFailure)));
      return annotationFrom(data.annotation);
    },
    async annotations(runId) {
      const data = await getEnvelope(options.request, annotationsPath(runId));
      const wire = parseAnnotationListResponse({ success: true, data });
      return { runId, annotations: wire.items.map((row) => ({
        annotationId: row.annotation_id, runId: row.run_id, materialId: row.material_id,
        baseVersion: row.base_version, body: row.body, authorId: row.author_id, createdAt: row.created_at,
      })) };
    },
  };
}
