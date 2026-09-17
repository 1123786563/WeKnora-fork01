import type { SubmissionEntry, SubmissionScope, SubmissionStore } from '@weknora/domain/mobile';

/**
 * 原生持久提交存储适配器（MX-006）：以 MMKV（同步、进程重启存活）承载
 * domain SubmissionStore 端口。KV 接口注入以便 node 环境单测；真实 MMKV
 * 实例由 react-native-mmkv 提供（apps 内已有使用先例）。
 * 键按规范化 origin/user/tenant 隔离：store 与单一 scope 绑定，
 * 拒绝跨空间写入、跨空间读取返回空——切空间后旧 store 不可见新空间数据。
 */

export interface KVLike {
  getString(key: string): string | undefined;
  set(key: string, value: string): void;
  delete(key: string): void;
  getAllKeys(): string[];
}

const KEY_PREFIX = 'weknora.submission.v1';

function entryKey(scope: SubmissionScope, requestId: string): string {
  const scopePart = [scope.origin, scope.userID, scope.tenantID].map((part) => encodeURIComponent(part)).join('|');
  return `${KEY_PREFIX}#${scopePart}#${encodeURIComponent(requestId)}`;
}

function sameScope(a: SubmissionScope, b: SubmissionScope): boolean {
  return a.origin === b.origin && a.tenantID === b.tenantID && a.userID === b.userID;
}

function parse(raw: string | undefined): SubmissionEntry | null {
  if (raw === undefined) return null;
  try {
    const entry = JSON.parse(raw) as SubmissionEntry;
    if (typeof entry?.request_id !== 'string' || typeof entry?.input_digest !== 'string') return null;
    return entry;
  } catch {
    return null;
  }
}

/** scope 绑定存储：协调器每个空间构造一个；写入即时同步落盘（发送前置条件）。 */
export function createScopedKVSubmissionStore(kv: KVLike, scope: SubmissionScope): SubmissionStore {
  const prefix = () => entryKey(scope, '');
  return {
    load(requestId) {
      return parse(kv.getString(entryKey(scope, requestId))) ?? undefined;
    },
    save(entry) {
      if (!sameScope(entry.scope, scope)) {
        throw new Error('submission store is scope-bound; refusing cross-scope write');
      }
      kv.set(entryKey(entry.scope, entry.request_id), JSON.stringify(entry));
    },
    listScope(target) {
      if (!sameScope(target, scope)) return [];
      const head = prefix();
      return kv.getAllKeys()
        .filter((key) => key.startsWith(head))
        .map((key) => parse(kv.getString(key)))
        .filter((entry): entry is SubmissionEntry => entry !== null);
    },
  };
}
