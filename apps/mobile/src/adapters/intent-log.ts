import type { SubmissionScope } from '@weknora/domain/mobile';
import { TaskOfficeError, type SubmissionIntentLog, type SubmissionIntentRecord } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const INTENTS_KEY = 'weknora.mobile.intents.v1';

/**
 * 单 key 整包存储的字节预算：Android SecureStore 单值约 2048 字节，1536 为
 * 预留 Base64 膨胀与余量的安全线（主控裁决 2026-09-24；有损丢最旧与
 * deployment-registry.ts:7 的 MAX_REGISTRY_ENTRIES 同先例）。多条时丢最旧直至
 * 达标；仅剩 1 条仍超限即显式拒绝（B3-F38：New 表单 maxLength 的 500 字估算
 * 漏算 knowledgeIds/attachments——合法组合越限必须有可行动错误码，而不是
 * setItemAsync 底层抛错或截断 text 破坏 goalKeyOf digest）。save 失败 =
 * 零 Start 派发的语义不变。
 */
const INTENTS_BUDGET_BYTES = 1536;

function sameScope(a: SubmissionScope, b: SubmissionScope): boolean {
  return a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
}

function isRecord(value: unknown): value is SubmissionIntentRecord {
  if (typeof value !== 'object' || value === null) return false;
  const row = value as Partial<SubmissionIntentRecord>;
  return typeof row.requestId === 'string' && row.requestId !== ''
    && typeof row.sessionId === 'string' && row.sessionId !== ''
    && typeof row.persistedAt === 'string'
    && typeof row.goal === 'object' && row.goal !== null
    && typeof row.scope === 'object' && row.scope !== null;
}

function parseRecords(raw: string | null): Map<string, SubmissionIntentRecord> {
  const map = new Map<string, SubmissionIntentRecord>();
  if (!raw) return map;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return map; // 损坏 JSON 读空表（不炸创建流），下一次 save 覆写修复
  }
  if (!Array.isArray(value)) return map;
  for (const row of value) {
    if (isRecord(row)) map.set(row.requestId, row);
  }
  return map;
}

/** 序列化并施加字节预算：超预算即丢最旧（Map 保持插入序）重试，直至达标或仅剩最新 1 条。 */
function serializeWithinBudget(records: Map<string, SubmissionIntentRecord>): string {
  let json = JSON.stringify([...records.values()]);
  while (records.size > 1 && new TextEncoder().encode(json).length > INTENTS_BUDGET_BYTES) {
    const oldest = records.keys().next().value;
    if (oldest === undefined) break;
    records.delete(oldest);
    json = JSON.stringify([...records.values()]);
  }
  return json;
}

/**
 * 耐久意图日志（Task Office 的 SubmissionIntentLog 端口实现）：
 * office 在 Start POST 前 await save——SecureStore 是异步 API，这正是耐久层
 * 必须由 office 显式 await 的原因（进程内 domain SubmissionStore 是同步契约，
 * 只有 in-memory 实现能真正满足它；见 plan-t36 的两层持久化边界声明）。
 *
 * save/remove 经模块内串行队列互斥（B3-F51）：两个并发 start 的 read-modify-
 * write 后写不得整包覆盖前写丢意图记录——该记录正是崩溃后幂等恢复的凭据。
 */
export function createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog {
  let queue: Promise<unknown> = Promise.resolve();
  const serialized = <T>(action: () => Promise<T>): Promise<T> => {
    const run = queue.then(action, action); // 前次失败不阻塞后续
    queue = run.then(() => undefined, () => undefined);
    return run;
  };
  const readAll = async (): Promise<Map<string, SubmissionIntentRecord>> => parseRecords(await store.getItemAsync(INTENTS_KEY));
  return {
    async save(record) {
      return serialized(async () => {
        const records = await readAll();
        records.set(record.requestId, record);
        const json = serializeWithinBudget(records);
        if (new TextEncoder().encode(json).length > INTENTS_BUDGET_BYTES) {
          // 单条即超限：源头显式拒绝（可行动错误码），不截断（截断破坏 goalKeyOf
          // digest），也不把底层 setItemAsync 的越限错误上抛（B3-F38）。
          throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('intent record exceeds the SecureStore budget; reduce goal text or attachments') });
        }
        await store.setItemAsync(INTENTS_KEY, json);
      });
    },
    async load(requestId) {
      return (await readAll()).get(requestId);
    },
    async listScope(scope) {
      return [...(await readAll()).values()].filter((record) => sameScope(record.scope, scope));
    },
    async remove(requestId) {
      return serialized(async () => {
        const records = await readAll();
        if (!records.delete(requestId)) return;
        await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
      });
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureIntentLog(): SubmissionIntentLog {
  return createSecureIntentLog(require('expo-secure-store') as SecureStorePort);
}
