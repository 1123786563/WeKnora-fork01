import type { SubmissionIntentLog, SubmissionIntentRecord } from '@weknora/mobile-core';
import type { ValueStore } from '../core/intent.ts';

const KEY = 'wk:mini:intents.v1';

/**
 * 耐久意图日志（#36 SubmissionIntentLog 的 storage 实现）：TaskOffice 在 Start POST 前
 * await save；重启后 load/listScope 用原 session 与原 goal 重建 digest 一致的 Start 输入。
 * 损坏 JSON 读空表——恢复语义宁可少恢复，不可恢复出脏意图。
 */
export function createTaroIntentLog(store: ValueStore): SubmissionIntentLog {
  const readAll = (): SubmissionIntentRecord[] => {
    const value = store.read(KEY);
    if (!Array.isArray(value)) return [];
    return value.filter((item): item is SubmissionIntentRecord =>
      typeof item === 'object' && item !== null && typeof (item as { requestId?: unknown }).requestId === 'string');
  };
  const writeAll = (records: SubmissionIntentRecord[]): void => { store.write(KEY, records); };
  return {
    async save(record) {
      writeAll([...readAll().filter(item => item.requestId !== record.requestId), record]);
    },
    async load(requestId) { return readAll().find(item => item.requestId === requestId); },
    async listScope(scope) {
      return readAll().filter(item => {
        const s = item.scope;
        return s !== undefined && s.origin === scope.origin && s.tenantID === scope.tenantID && s.userID === scope.userID;
      });
    },
    async remove(requestId) { writeAll(readAll().filter(item => item.requestId !== requestId)); },
  };
}
