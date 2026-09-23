import { filterAgents, knowledgeRefForPrompt, toAgentOptions, toConnectionResource, toKnowledgeResource } from '@weknora/domain/mobile';
import { leaseActive, leaseScopeOf } from '../runtime/scope-lease.ts';
import type { ResourceRemote, ResourceShelfPorts } from './ports.ts';
import type {
  ResourceClass,
  ResourceClassVerdict,
  ResourcePage,
  ResourceQuery,
  ResourceShelf,
  ResourceShelfHandle,
  SelectionVerdict,
  ShelfCloseReason,
  ShelfInvalidationEvent,
} from './types.ts';

const SUPPORTED: ResourceClassVerdict = { state: 'supported', reason: '' };
const RESOURCE_CLASSES: readonly ResourceClass[] = ['agent', 'knowledge', 'connection'];

function httpStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

export function createResourceShelf(ports: ResourceShelfPorts): ResourceShelf {
  return {
    open({ lease }) {
      const scope = leaseScopeOf(lease);
      if (!scope || !leaseActive(lease)) throw new Error('SHELF_LEASE');
      const listeners = new Set<(event: ShelfInvalidationEvent) => void>();
      let closed = false;
      let agents = toAgentOptions([]);
      let knowledge: Array<ReturnType<typeof toKnowledgeResource>> = [];
      let connections: Array<ReturnType<typeof toConnectionResource>> = [];
      let verdicts: Record<ResourceClass, ResourceClassVerdict> = { agent: SUPPORTED, knowledge: SUPPORTED, connection: SUPPORTED };

      const notify = (event: ShelfInvalidationEvent): void => {
        for (const listener of [...listeners]) listener(event);
      };
      const guard = (): void => {
        if (closed || !leaseActive(lease)) throw new Error('SHELF_SCOPE_CLOSED');
      };
      const loadClass = async <T>(
        load: (token: string) => Promise<T>,
      ): Promise<{ ok: true; value: T } | { ok: false; verdict: ResourceClassVerdict }> => {
        // 重试判定读结构化 status，不读 verdict.reason 字面量：reason 是展示字符串，
        // 未来任何 verdict 复用 'http_401' 文本都不得误触发一次多余刷新。
        type Attempt = { ok: true; value: T } | { ok: false; verdict: ResourceClassVerdict; status?: number };
        const attempt = async (refresh: boolean): Promise<Attempt> => {
          try {
            const token = await ports.accessTokenFor(scope.deploymentOrigin, refresh ? { refresh: true } : undefined);
            return { ok: true, value: await load(token) };
          } catch (error) {
            const status = httpStatus(error);
            if (status === 403) return { ok: false, verdict: { state: 'forbidden', reason: 'http_403' }, status };
            return { ok: false, verdict: { state: 'unavailable', reason: status === undefined ? 'fetch_failed' : `http_${status}` }, status };
          }
        };
        const first = await attempt(false);
        if (first.ok) return first;
        if (first.status === 401) return attempt(true); // 恰好一次刷新重试，禁止循环
        return first;
      };

      const handle: ResourceShelfHandle = {
        async browse(query: ResourceQuery = {}) {
          guard();
          const [agentResult, knowledgeResult, connectionResult] = await Promise.all([
            loadClass((token) => ports.remote.availableAgents(token)),
            loadClass((token) => ports.remote.knowledgeBases(token)),
            loadClass((token) => ports.remote.connections(token)),
          ]);
          guard(); // 在途期间 scope 已关闭/撤销 → 丢弃迟到结果（spec §5.3 不变量同源）
          const previous = verdicts;
          // 失败类必须清空旧投影（resource-presentation.ts:4 冻结规则：撤权后敏感字段立即
          // 不可见，展示模型从服务端事实重建，不从缓存回填）——forbidden/unavailable 均为空列表。
          agents = agentResult.ok
            ? toAgentOptions([...agentResult.value.rows]).filter((option) => !agentResult.value.disabledOwnAgentIds.has(option.id))
            : [];
          knowledge = knowledgeResult.ok ? knowledgeResult.value.map((row) => toKnowledgeResource(row)) : [];
          connections = connectionResult.ok ? connectionResult.value.map((row) => toConnectionResource(row)) : [];
          verdicts = {
            agent: agentResult.ok ? SUPPORTED : agentResult.verdict,
            knowledge: knowledgeResult.ok ? SUPPORTED : knowledgeResult.verdict,
            connection: connectionResult.ok ? SUPPORTED : connectionResult.verdict,
          };
          for (const resourceClass of RESOURCE_CLASSES) {
            if (verdicts[resourceClass].state === 'forbidden' && previous[resourceClass].state !== 'forbidden') {
              notify({ type: 'authorization-revoked', resourceClass });
            }
          }
          const keyword = query.keyword?.trim().toLowerCase() ?? '';
          const page: ResourcePage = {
            tenantId: scope.tenantId,
            agents: filterAgents({ agents }, { kind: query.kind, keyword: query.keyword }),
            knowledge: keyword === '' ? knowledge : knowledge.filter((resource) => resource.title.toLowerCase().includes(keyword)),
            connections,
            classVerdicts: verdicts,
          };
          return page;
        },
        selection(input): SelectionVerdict {
          guard();
          if (typeof input.agentId === 'string') {
            if (verdicts.agent.state === 'forbidden') return { allowed: false, state: 'forbidden', reason: verdicts.agent.reason };
            const base = agents.find((agent) => agent.id === input.agentId);
            if (!base) return { allowed: false, state: 'unavailable', reason: 'agent_not_found' };
            if (base.capability.state !== 'supported') {
              return { allowed: false, state: base.capability.state === 'forbidden' ? 'forbidden' : 'unavailable', reason: base.capability.reason || base.capability.state };
            }
            return { allowed: true, selection: { kind: 'agent', agentId: base.id } };
          }
          if (typeof input.knowledgeId === 'string') {
            if (verdicts.knowledge.state === 'forbidden') return { allowed: false, state: 'forbidden', reason: verdicts.knowledge.reason };
            const resource = knowledge.find((item) => item.id === input.knowledgeId);
            if (!resource) return { allowed: false, state: 'unavailable', reason: 'knowledge_not_found' };
            const ref = knowledgeRefForPrompt(resource, { revoked: false });
            return ref === null
              ? { allowed: false, state: 'unavailable', reason: 'knowledge_not_selectable' }
              : { allowed: true, selection: ref };
          }
          return { allowed: false, state: 'unavailable', reason: 'selection_required' };
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => listeners.delete(listener);
        },
        close(reason: ShelfCloseReason) {
          if (closed) return;
          closed = true;
          notify({ type: 'scope-closed', reason });
        },
      };
      return handle;
    },
  };
}
