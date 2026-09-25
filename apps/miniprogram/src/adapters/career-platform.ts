import Taro from '@tarojs/taro';
import type { NativeFileSource } from '@weknora/api-client';
import { storage } from '../platform/storage.ts';
import { chooseDocument } from '../platform/files.ts';
import { requestId } from '../core/intent.ts';
import type { ValueStore } from '../core/intent.ts';

// T24 小程序 Career 平台适配器：分享、文件、受控存储三个本端能力 seam。
// 全部依赖可注入（测试与真实运行时共用同一实现）；网络一律经 services/career.ts
// 的认证客户端，本适配器永不直接发请求。

/** 分享入口 query 参数名（分享卡片 path 携带的 JD 原文）。 */
export const SHARED_ENTRY_PARAM = 'jd';
export const SHARED_ENTRY_SOURCE = '微信分享';
/** 受控存储唯一前缀：登出 clearPrivateCache 只清 wk: 非 wk:auth: 键，本前缀随之清空。 */
export const CAREER_STORE_PREFIX = 'wk:career:';
const PREVIEW_LIMIT = 160;

export interface SharedEntry { text: string; sourceLabel: string }
export interface CareerPlatformDeps {
  entryParams?: () => Record<string, string | undefined>;
  chooseFile?: () => Promise<NativeFileSource>;
}
export interface CareerPlatform {
  readSharedEntry(): SharedEntry | undefined;
  chooseResume(): Promise<NativeFileSource>;
}

function defaultEntryParams(): Record<string, string | undefined> {
  const params = Taro.getCurrentInstance().router?.params;
  return (params ?? {}) as Record<string, string | undefined>;
}
/** 分享卡片 path 的 query 在部分基础库/入口下拿到的是未解码的 percent-encoding
 * （DevTools automator 实测 71 字 JD 变 400 字）；仅在解码成功且更短时采用，
 * 避免把用户原文里的字面 '%' 误解码。 */
function decodeEntryPayload(raw: string): string {
  if (!/%[0-9A-Fa-f]{2}/.test(raw)) return raw;
  try {
    const decoded = decodeURIComponent(raw);
    return decoded.length < raw.length ? decoded : raw;
  } catch { return raw; }
}

export function createCareerPlatform(deps: CareerPlatformDeps = {}): CareerPlatform {
  return {
    // 负载缺失返回 undefined：由 UI 展示恢复入口（粘贴原文），绝不用演示数据顶替。
    readSharedEntry() {
      const raw = (deps.entryParams ?? defaultEntryParams)()[SHARED_ENTRY_PARAM];
      const textValue = typeof raw === 'string' ? decodeEntryPayload(raw).trim() : '';
      if (!textValue) return undefined;
      return { text: textValue, sourceLabel: SHARED_ENTRY_SOURCE };
    },
    async chooseResume() { return await (deps.chooseFile ?? chooseDocument)(); },
  };
}
/** 真实 weapp 装配：分享参数来自页面启动 query，简历从聊天文件选择。 */
export const careerPlatform = createCareerPlatform();

export interface SharedImportDraft { rawText: string; requestId: string; sourceLabel: string; preview: { excerpt: string; fullLength: number } }
/** 分享导入第一阶段：只构造可核对预览（截断摘录 + 原文长度），不发起任何网络写。 */
export function prepareSharedImport(rawText: string, sourceLabel = SHARED_ENTRY_SOURCE, makeId: () => string = requestId): SharedImportDraft {
  const textValue = rawText.trim();
  if (!textValue) throw Object.assign(new Error('分享内容为空：请重新分享，或在下方粘贴职位原文后再导入'), { code: 'share_payload_missing', recoverable: true });
  return {
    rawText: textValue, requestId: makeId(), sourceLabel,
    preview: { excerpt: textValue.length > PREVIEW_LIMIT ? `${textValue.slice(0, PREVIEW_LIMIT)}…` : textValue, fullLength: textValue.length },
  };
}

export interface ControlledCareerStore { read(key: string): unknown; write(key: string, value: unknown): void; remove(key: string): void }
/** 受控存储：只接受 wk:career: 前缀内的键，登出时随 clearPrivateCache 一并清除。 */
export function createControlledStore(store: ValueStore = storage): ControlledCareerStore {
  const full = (key: string) => {
    if (!key.startsWith(CAREER_STORE_PREFIX)) throw new Error(`career store key must be prefixed with ${CAREER_STORE_PREFIX}`);
    return key;
  };
  return { read: key => store.read(full(key)), write: (key, value) => store.write(full(key), value), remove: key => store.remove(full(key)) };
}
