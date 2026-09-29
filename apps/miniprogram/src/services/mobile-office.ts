import { createTaskOffice, createTaskMaterial } from '@weknora/mobile-core';
import type { ResourceShelfHandle, ScopeLease, TaskMaterial, TaskMaterialHandle, TaskOffice } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createTaroBlobFetch } from '../platform/authorized-channels.ts';
import { createTaroIntentLog } from '../platform/intent-log.ts';
import { storage } from '../platform/storage.ts';
import { requestId } from '../core/intent.ts';
import { runtime, executions, network } from './runtime.ts';

/**
 * 深模块组合根（issue #68 What-to-build）：小程序的 Task/Resource/Material 关键 scenario
 * 全部经由 @weknora/mobile-core 的深模块 Interface——与 apps/mobile/src/composition.ts 同一
 * 装配、不同平台 Adapter。lease/授权通道/token 纪律全部由 MobileRuntime 铸造。
 */

/** 记忆化键 = origin::tenant（与 apps/mobile deploymentScopeKey 同语义）：切租户/换部署不复用含旧 scope 状态的实例。 */
const MAX_CACHE_ENTRIES = 8;
const cachePut = <T>(cache: Map<string, T>, key: string, make: () => T): T => {
  const existing = cache.get(key);
  if (existing !== undefined) return existing;
  if (cache.size >= MAX_CACHE_ENTRIES) cache.delete(cache.keys().next().value!);
  const created = make();
  cache.set(key, created);
  return created;
};

function deploymentScope(): { origin: string; tenantId: string } | undefined {
  const snapshot = runtime.snapshot();
  if (snapshot.surface !== 'authorized') return undefined;
  const origin = snapshot.deployment?.origin;
  const tenantId = snapshot.identity?.activeTenantId;
  if (!origin || !tenantId) return undefined;
  return { origin, tenantId };
}

const intentLog = createTaroIntentLog(storage);

/** 缓存条目绑定创建时的 lease 世代：Runtime 重签 lease（重新登录等）即废弃重建，
 *  避免「同 scope 缓存命中携带已撤销 lease → 一切读 TASK_OFFICE_SCOPE_CHANGED」的陈旧实例。 */
const offices = new Map<string, { office: TaskOffice; lease: ScopeLease }>();

export function activeTaskOffice(): TaskOffice | undefined {
  const scope = deploymentScope();
  const lease = runtime.scopeLease();
  if (!scope || !lease) return undefined;
  // office 持有创建时刻的 lease 引用（与 mobile-core task-office.test.ts:47-55 的语义一致）：
  // 切租户/换部署/登出撤销旧 lease 后，旧实例的一切读按 TASK_OFFICE_SCOPE_CHANGED fail closed；
  // 同 scope 且 lease 未轮换时复用实例（分页累积/提交状态延续）。
  const key = `${scope.origin}::${scope.tenantId}`;
  const cached = offices.get(key);
  if (cached !== undefined) {
    if (cached.lease === lease) return cached.office;
    offices.delete(key); // lease 已轮换：旧实例随旧 lease 废弃，此后 fail closed
  }
  if (offices.size >= MAX_CACHE_ENTRIES) offices.delete(offices.keys().next().value!);
  const remote = createTaskOfficeRemote({
    origin: scope.origin,
    request: input => runtime.authorizedRequest(input),
    stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
  });
  const office = createTaskOffice({
    backend: remote,
    detail: remote,
    interactions: remote,
    lease: () => lease,
    // 耐久意图日志（storage）：Start POST 前落盘，重启后 reconcilePending 恢复。
    intentLog,
    // 平台无 crypto.randomUUID 时必须显式注入（#36 契约）。
    newRequestId: requestId,
    // TaskProjectionStore 持久化选型（SQLite/SecureStore）是 B2-F23 未决项——此处显式
    // 采用 in-memory（缺省），使「未注入持久化」成为组合根的显式决策而非静默回退。
  });
  offices.set(key, { office, lease });
  return office;
}

export function requireTaskOffice(): TaskOffice {
  const office = activeTaskOffice();
  if (!office) throw new Error('任务面板尚未就绪（未授权或缺少活跃空间）');
  return office;
}

const materials = new Map<string, TaskMaterial>();

export function openActiveMaterial(): TaskMaterialHandle | undefined {
  const scope = deploymentScope();
  const lease = runtime.scopeLease();
  if (!scope || !lease) return undefined;
  const material = cachePut(materials, scope.origin, () => createTaskMaterial({
    remote: createMobileMaterialRemote({ origin: scope.origin, request: input => runtime.authorizedRequest(input) }),
    blob: createTaroBlobFetch(network),
    // share 缺省 fail closed（MATERIAL_SHARE_UNAVAILABLE）：微信分享面板属真机验收（spec Testing Decisions）。
  }));
  return material.open({ lease });
}

export function activeResourceShelf(): ResourceShelfHandle | undefined {
  return runtime.resourceShelf();
}

/** 按 runId 恢复任务详情的权威途径：run 行的 session_id 就是 taskId（ADR-0004）。 */
export async function resolveTaskForRun(runId: string): Promise<{ taskId: string; runId: string }> {
  const execution = await executions.get(runId);
  const sessionId = (execution as { session_id?: unknown }).session_id;
  if (sessionId === undefined || sessionId === null) throw new Error('该执行缺少会话身份，无法打开任务详情');
  return { taskId: String(sessionId), runId };
}
