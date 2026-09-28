import { auth } from './runtime.ts';
import { requestId as newRequestId, type ValueStore } from '../core/intent.ts';
import { scopeKey, type ScopeStamp } from '../core/scope.ts';
import { storage } from '../platform/storage.ts';

// OCR med-39（high-7/high-8 的根因）：services/career.ts 与 adapters/career-platform.ts
// 曾各自维护一套几乎相同的恢复链设施并已行为漂移（search 版缺作用域守卫、不持久化
// expectedRevision）。本模块是两域共用的唯一实现：
//   · 歧义判据一份：typed 拒绝/4xx=确定失败；超时/网络/5xx/无状态=进入恢复链；
//   · intent 键一律按“发送时刻”的 stamp 预铸——catch 中绝不按当前作用域补铸，作用域
//     在途切换时旧作用域的失败不得把恢复意图落到新作用域键下（空间失效红线）；
//   · expectedRevision 在发送前捕获并随 intent 持久化：服务端幂等指纹是全量请求体
//     （含 expectedRevision），安全重发必须逐字节重放原始值——修订前进后用当前值重发
//     必然 ErrIdempotencyConflict，恢复死路（幂等红线）。

/** 受控存储唯一前缀：登出 clearPrivateCache 只清 wk: 非 wk:auth: 键，本前缀随之清空。 */
export const CAREER_STORE_PREFIX = 'wk:career:';
export interface ControlledCareerStore { read(key: string): unknown; write(key: string, value: unknown): void; remove(key: string): void }
/** 受控存储：只接受 wk:career: 前缀内的键，登出时随 clearPrivateCache 一并清除。 */
export function createControlledStore(store: ValueStore = storage): ControlledCareerStore {
  const full = (key: string) => {
    if (!key.startsWith(CAREER_STORE_PREFIX)) throw new Error(`career store key must be prefixed with ${CAREER_STORE_PREFIX}`);
    return key;
  };
  return { read: key => store.read(full(key)), write: (key, value) => store.write(full(key), value), remove: key => store.remove(full(key)) };
}

export interface StoredIntent<T> { requestId: string; input: T; expectedRevision?: number }
const activeRecoveries = new Map<string, number>();
function recoveryKey(kind: string, stamp: ScopeStamp, requestId: string): string {
  return `${kind}:${scopeKey(stamp)}:${requestId}`;
}
/** Mark recovery active for its full asynchronous lifetime; abandonment uses the same key. */
export function withActiveRecovery<T>(kind: string, stamp: ScopeStamp, requestId: string, work: () => Promise<T>): Promise<T> {
  const key = recoveryKey(kind, stamp, requestId);
  activeRecoveries.set(key, (activeRecoveries.get(key) ?? 0) + 1);
  const release = () => {
    const count = activeRecoveries.get(key) ?? 0;
    if (count <= 1) activeRecoveries.delete(key);
    else activeRecoveries.set(key, count - 1);
  };
  try { return work().finally(release); }
  catch (error) { release(); return Promise.reject(error); }
}
export function isRecoveryActive(kind: string, stamp: ScopeStamp, requestId: string): boolean {
  return (activeRecoveries.get(recoveryKey(kind, stamp, requestId)) ?? 0) > 0;
}
/** intent 键 = 前缀 + kind + 作用域。写入侧传发送时刻 stamp（预铸）；读取侧传当前 capture()。 */
export function intentKeyFor(kind: string, stamp: ScopeStamp): string { return `${CAREER_STORE_PREFIX}${kind}:${scopeKey(stamp)}`; }

/** 歧义判据（两域冻结合同口径）：typed 拒绝/4xx 是确定失败，不进恢复链；无 code 且无
 *  status（如传输层 SCOPE_CHANGED）按歧义处理——由调用方的 stamp 守卫兜底定性。 */
export function ambiguousOutcome(error: unknown): boolean {
  const code = (error as { code?: unknown })?.code;
  if (typeof code === 'string') {
    if (['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED', 'outcome_unknown'].includes(code)) return true;
    if (['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved', 'search_quota_refused', 'contract_violation'].includes(code)) return false;
  }
  const status = (error as { status?: unknown })?.status;
  return typeof status !== 'number' || status >= 500;
}
/** 空间切换/AUTH_REQUIRED 是确定失败：旧作用域的响应不能为新作用域留恢复意图。 */
export function definiteLocalFailure(error: unknown): boolean { return /SCOPE_CHANGED|AUTH_REQUIRED/i.test(`${(error as Error)?.message ?? ''} ${(error as { code?: unknown })?.code ?? ''}`); }

export function readStoredIntent<T>(store: ControlledCareerStore, key: string): StoredIntent<T> | null {
  const value = store.read(key);
  if (!value || typeof value !== 'object') return null;
  const parsed = value as { requestId?: unknown; input?: unknown; expectedRevision?: unknown };
  if (typeof parsed.requestId !== 'string' || !parsed.requestId || typeof parsed.input !== 'object' || !parsed.input) return null;
  const expectedRevision = typeof parsed.expectedRevision === 'number' && Number.isSafeInteger(parsed.expectedRevision) && parsed.expectedRevision >= 0 ? parsed.expectedRevision : undefined;
  return { requestId: parsed.requestId, input: parsed.input as T, ...(expectedRevision !== undefined ? { expectedRevision } : {}) };
}

/** “结果未知”包装：同 requestId 对账/重发可恢复，绝不静默丢弃。 */
export function unknownOutcomeError(describe: string, requestId: string, cause: unknown): Error {
  return Object.assign(new Error(`${describe}结果未知：请用原请求对账后再试`, { cause }), { code: 'outcome_unknown', requestId });
}

/** 解码守卫（OCR r3 ocr3-029）：api-client 解码器对畸形 200 正文抛裸 TypeError/Error（无
 *  code 无 status），会被歧义判据误判“结果未知”落 intent——但服务端已 200，同 requestId
 *  重放必然再次解码失败，用户会被困在恢复循环。此处给解码失败附加确定性标记
 *  contract_violation（歧义判据判 false，不进恢复链）；已携带 code 的错误原样放行。 */
export function decodeAs<T>(decode: (value: unknown) => T, value: unknown): T {
  try {
    return decode(value);
  } catch (error) {
    if ((error as { code?: unknown } | null | undefined)?.code !== undefined) throw error;
    throw Object.assign(new Error('服务端响应不符合冻结合同（解码失败）：已按确定失败处理，不进入恢复链'), { code: 'contract_violation', cause: error });
  }
}

export interface RecoverableWriteInput<T> {
  kind: string; describe: string; input: unknown;
  /** 发送前捕获的 CAS 期望值（幂等指纹的一部分，随 intent 持久化）。 */
  expected: number;
  /** 重试复用原请求编号（新写入不传）。 */
  reuseId?: string;
  send: (id: string, expected: number) => Promise<T>;
}
/** 写入 + 未知结果恢复（两域共用）：stamp 守卫先于 intent 落盘——作用域在途切换或
 *  确定性本地失败（SCOPE_CHANGED/AUTH_REQUIRED，见 definiteLocalFailure 冻结口径）抛
 *  SCOPE_CHANGED 且绝不落 intent（键按发送时刻 stamp 预铸）；其余歧义失败落 intent 并抛
 *  outcome_unknown；确定失败原样上抛，不进恢复链。与 retryRecoverable 的守卫保持同一判据。
 *  发送前先按同 kind 检查未对账 intent（unresolved_action 阻断，OCR r3 ocr3-030）。 */
export async function recoverableWrite<T>(store: ControlledCareerStore, options: RecoverableWriteInput<T>): Promise<T> {
  const id = options.reuseId ?? newRequestId();
  const stamp = auth.scope.capture();
  const key = intentKeyFor(options.kind, stamp);
  // OCR r3 ocr3-030（对齐 desk.mutate 的 unresolved_action 硬性守卫）：同 kind 已有未对账
  // intent 时绝不发新写入——catch 中的单键覆盖会静默丢掉旧 requestId（唯一对账凭据），
  // 第一次写入可能已落地，覆盖即永久失去对账入口。先对账/安全重发/显式放弃。
  const unresolved = readStoredIntent(store, key);
  if (unresolved) throw Object.assign(new Error(`有一次结果未知的${options.describe}（${unresolved.requestId.slice(0, 10)}…）：请先用原请求对账、安全重发，或显式放弃本次恢复`), { code: 'unresolved_action', requestId: unresolved.requestId });
  try {
    return await options.send(id, options.expected);
  } catch (error) {
    // OCR2-037：AUTH_REQUIRED 等确定性本地失败（裸 message 无 code/status，会被
    // ambiguousOutcome 误判歧义）绝不落 intent——与 retryRecoverable 同一守卫口径。
    if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) {
      throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    }
    if (ambiguousOutcome(error)) {
      store.write(key, { requestId: id, input: options.input, expectedRevision: options.expected });
      throw unknownOutcomeError(options.describe, id, error);
    }
    throw error;
  }
}

/** intent 恢复重放：requestId + expectedRevision 逐字节重放（优先 intent 持久化值；其次
 *  input 内携带值——兼容历史 T30 intent 形状；再退回调用方提供的当前值）。成功即清 intent。 */
export async function retryRecoverable<T>(store: ControlledCareerStore, kind: string, describe: string, resend: (id: string, expected: number) => Promise<T>, fallbackExpected?: () => number): Promise<T> {
  const key = intentKeyFor(kind, auth.scope.capture());
  const pending = readStoredIntent<Record<string, unknown>>(store, key);
  if (!pending) throw new Error(`没有待恢复的${describe}`);
  const inputExpected = typeof pending.input?.expectedRevision === 'number' ? pending.input.expectedRevision : undefined;
  const expected = pending.expectedRevision ?? inputExpected ?? fallbackExpected?.();
  if (expected === undefined) throw Object.assign(new Error('恢复记录缺少原档案修订：请放弃本次恢复后按当前修订重新保存'), { code: 'intent_revision_missing' });
  const stamp = auth.scope.capture();
  try {
    const receipt = await resend(pending.requestId, expected);
    store.remove(key);
    return receipt;
  } catch (error) {
    if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (ambiguousOutcome(error)) throw unknownOutcomeError(describe, pending.requestId, error);
    throw error;
  }
}

/** 显式放弃恢复（OCR r3 ocr3-029 统一出口，rule/reminder 先例同语义）：只清本端 intent，
 *  不动任何服务端事实——若原写入实际已落地，以服务端记录为准（回执/列表可对账找回）。
 *  未对账封锁（recoverableWrite 的 unresolved_action）把用户挡在门外时，这是唯一解除出口。 */
export function abandonRecoverable(store: ControlledCareerStore, kind: string, expectedRequestId?: string): boolean {
  const stamp = auth.scope.capture();
  const key = intentKeyFor(kind, stamp);
  const current = readStoredIntent(store, key);
  const requestId = expectedRequestId ?? current?.requestId;
  if (requestId && isRecoveryActive(kind, stamp, requestId)) return false;
  if (expectedRequestId !== undefined && current?.requestId !== expectedRequestId) return false;
  store.remove(key);
  return true;
}
