import { parseChatMessageListResponse, responseType } from '@weknora/contracts';
import { createServerSentEventParser, parseChatEvent } from '../chat/stream.ts';
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface LegacyTaskRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
  /** 授权 SSE 通道（MobileRuntime.authorizedEventStream 或测试替身）；普通追问走既有聊天 SSE wire。 */
  stream?: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
}

/** 与 mobile-core LegacyBackendTask 逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteLegacyTask { taskId: string; title: string; attention: 'none'; archivedAt?: string; updatedAt: string }
export interface RemoteLegacyPage { items: RemoteLegacyTask[]; nextCursor?: string }
export interface RemoteLegacyMessage { messageId: string; role: 'user' | 'assistant' | 'system'; content: string; createdAt?: string }

function legacyRow(item: unknown, path: string): RemoteLegacyTask {
  if (typeof item !== 'object' || item === null || Array.isArray(item)) throw new Error(`${path} must be an object`);
  const row = item as Record<string, unknown>;
  if (row.kind !== 'legacy') throw new Error(`${path}.kind must be "legacy"`);
  if (row.attention !== undefined && row.attention !== null && row.attention !== 'none') {
    throw new Error(`${path}.attention must be "none" for legacy tasks`);
  }
  if (typeof row.task_id !== 'string' || row.task_id.trim() === '') throw new Error(`${path}.task_id must be a non-empty string`);
  if (typeof row.updated_at !== 'string' || row.updated_at === '') throw new Error(`${path}.updated_at must be a non-empty string`);
  const archivedAt = typeof row.archived_at === 'string' && row.archived_at !== '' ? row.archived_at : undefined;
  return {
    taskId: row.task_id,
    title: typeof row.title === 'string' ? row.title : '',
    attention: 'none',
    ...(archivedAt === undefined ? {} : { archivedAt }),
    updatedAt: row.updated_at,
  };
}

export function createMobileLegacyTaskRemote(options: LegacyTaskRemoteOptions) {
  requireDeploymentOrigin(options.origin);
  const request = options.request;

  return {
    async list(input: { search?: string; archived?: boolean; cursor?: string; limit?: number }): Promise<RemoteLegacyPage> {
      const query = new URLSearchParams();
      if (input.search !== undefined && input.search !== '') query.set('q', input.search);
      if (input.archived === true) query.set('archived', 'true');
      if (input.cursor !== undefined && input.cursor !== '') query.set('cursor', input.cursor);
      if (input.limit !== undefined) query.set('limit', String(input.limit));
      const suffix = query.toString();
      const value = await request({ method: 'GET', path: `/api/v1/workbench/legacy-tasks${suffix ? `?${suffix}` : ''}` });
      const envelope = value as { success?: unknown; data?: unknown } | null;
      if (typeof envelope !== 'object' || envelope === null || envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null) {
        throw new Error('legacy task list response.success must be true with data');
      }
      const page = envelope.data as { items?: unknown; next_cursor?: unknown };
      if (!Array.isArray(page.items)) throw new Error('legacy task list data.items must be an array');
      return {
        items: page.items.map((item, index) => legacyRow(item, `items[${index}]`)),
        ...(typeof page.next_cursor === 'string' && page.next_cursor !== '' ? { nextCursor: page.next_cursor } : {}),
      };
    },
    async history(taskId: string): Promise<RemoteLegacyMessage[]> {
      const trimmed = taskId.trim();
      if (trimmed === '') throw new Error('taskId must not be empty');
      const messages = parseChatMessageListResponse(await request({
        method: 'GET',
        path: `/api/v1/messages/${encodeURIComponent(trimmed)}/load?limit=20`,
      }));
      return messages.map((message) => ({
        messageId: message.id,
        role: message.role,
        content: message.content,
        ...(message.created_at === undefined ? {} : { createdAt: message.created_at }),
      }));
    },
    async followUp(input: { taskId: string; question: string; signal?: AbortSignal }): Promise<void> {
      const transport = options.stream;
      if (!transport) throw new Error('legacy follow-up requires an authorized stream transport');
      const taskId = input.taskId.trim();
      const question = input.question.trim();
      if (taskId === '' || question === '') throw new Error('legacy follow-up requires taskId and question');
      let failed = false;
      let terminated = false;
      let frames = 0;
      const parser = createServerSentEventParser((frame) => {
        frames += 1;
        let event;
        try {
          event = parseChatEvent(frame);
        } catch {
          throw new Error('LEGACY_FOLLOW_UP_MALFORMED_FRAME');
        }
        const type = responseType(event);
        if (type === 'error') {
          failed = true;
          return;
        }
        if (type === 'complete' || type === 'stop') terminated = true;
      });
      await transport({
        method: 'POST',
        path: `/api/v1/knowledge-chat/${encodeURIComponent(taskId)}`,
        headers: { accept: 'text/event-stream', 'content-type': 'application/json' },
        body: { query: question },
        ...(input.signal === undefined ? {} : { signal: input.signal }),
      }, (chunk) => parser.push(chunk));
      parser.finish();
      if (failed) throw new Error('LEGACY_FOLLOW_UP_FAILED');
      // Fail-closed 终止帧判定（T44 Task 6 修复轮主控裁决）：服务端正常完成必发 complete/stop
      // （internal/handler/session/stream.go:143,186,404）；流正常结束但未见终止帧＝结果未知
      // （代理断连/服务端中途崩溃），按 #30 Spec 信任模型必须让上层感知并触发对账，不得静默判成功。
      if (!terminated) throw new Error(`LEGACY_FOLLOW_UP_TRUNCATED: legacy follow-up for task ${taskId} ended without a terminal frame after ${frames} frame(s)`);
    },
  };
}
