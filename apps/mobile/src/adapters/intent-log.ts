import type { SubmissionScope } from '@weknora/domain/mobile';
import type { SubmissionIntentLog, SubmissionIntentRecord } from '@weknora/mobile-core';
import type { SecureStorePort } from './secure-store.ts';

const INTENTS_KEY = 'weknora.mobile.intents.v1';

/**
 * 单 key 整包存储的字节预算：Android SecureStore 单值约 2048 字节，1536 为
 * 预留 Base64 膨胀与余量的安全线（主控裁决 2026-09-24；有损丢最旧与
 * deployment-registry.ts:7 的 MAX_REGISTRY_ENTRIES 同先例）。仅剩 1 条时不再
 * 裁剪——保住最新意图的耐久落盘（save 失败 = 零 Start 派发）；单条超限由
 * New 表单的 maxLength 约束在源头拦截，不在本层截断（截断 text 会破坏
 * goalKeyOf 意图一致性 digest）。
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
 */
export function createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog {
  const readAll = async (): Promise<Map<string, SubmissionIntentRecord>> => parseRecords(await store.getItemAsync(INTENTS_KEY));
  return {
    async save(record) {
      const records = await readAll();
      records.set(record.requestId, record);
      await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
    },
    async load(requestId) {
      return (await readAll()).get(requestId);
    },
    async listScope(scope) {
      return [...(await readAll()).values()].filter((record) => sameScope(record.scope, scope));
    },
    async remove(requestId) {
      const records = await readAll();
      if (!records.delete(requestId)) return;
      await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
    },
  };
}

/** Loads Expo SecureStore only in the native composition path. */
export function createNativeSecureIntentLog(): SubmissionIntentLog {
  return createSecureIntentLog(require('expo-secure-store') as SecureStorePort);
}
