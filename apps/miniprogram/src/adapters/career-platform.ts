import Taro from '@tarojs/taro';
import type { NativeFileSource } from '@weknora/api-client';
import { storage } from '../platform/storage.ts';
import { chooseDocument } from '../platform/files.ts';
import { requestId } from '../core/intent.ts';
import type { ValueStore } from '../core/intent.ts';
// T26 导出兑付下载与 T06 files.ts 同源：认证凭据与可信 origin 只来自 runtime 单例。
// runtime 不反向依赖本适配器，无环。
import { auth, apiOrigin } from '../services/runtime.ts';

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

// ---- T26 申请材料导出的本端文件 seam：sha256 校验 + 认证兑付下载 + 打开 ----
// 导出下载是“带 Authorization 的 GET 兑付”（T06 downloadFile 带 header 先例），不是
// 免签公开 URL；下载字节做全量 sha256 与授权 digest 比对（校验层级：全字节摘要，
// 强于“大小+首字节”），一致才复制私有副本并打开（D3：运行时临时文件不可删，可删的
// 只有 USER_DATA_PATH 副本，打开后立即清理）。

const SHA256_K = new Uint32Array([
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
]);
const rotr = (x: number, n: number): number => ((x >>> n) | (x << (32 - n))) >>> 0;
/** 纯 TS sha256（weapp 运行时没有 node:crypto / subtle）：公开向量在测试里钉死。 */
export function sha256Hex(bytes: Uint8Array): string {
  const h = new Uint32Array([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
  const bitLength = bytes.length * 8;
  const total = ((((bytes.length + 8) >>> 6) + 1) << 6) >>> 0;
  const padded = new Uint8Array(total);
  padded.set(bytes);
  padded[bytes.length] = 0x80;
  const view = new DataView(padded.buffer);
  view.setUint32(total - 8, Math.floor(bitLength / 0x100000000), false);
  view.setUint32(total - 4, bitLength >>> 0, false);
  const w = new Uint32Array(64);
  for (let offset = 0; offset < total; offset += 64) {
    for (let i = 0; i < 16; i++) w[i] = view.getUint32(offset + i * 4, false);
    for (let i = 16; i < 64; i++) {
      const s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3);
      const s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10);
      w[i] = (w[i - 16] + s0 + w[i - 7] + s1) >>> 0;
    }
    let a = h[0], b = h[1], c = h[2], d = h[3], e = h[4], f = h[5], g = h[6], hh = h[7];
    for (let i = 0; i < 64; i++) {
      const big1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      const choose = (e & f) ^ (~e & g);
      const t1 = (hh + big1 + choose + SHA256_K[i] + w[i]) >>> 0;
      const big0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      const majority = (a & b) ^ (a & c) ^ (b & c);
      const t2 = (big0 + majority) >>> 0;
      hh = g; g = f; f = e; e = (d + t1) >>> 0; d = c; c = b; b = a; a = (t1 + t2) >>> 0;
    }
    h[0] = (h[0] + a) >>> 0; h[1] = (h[1] + b) >>> 0; h[2] = (h[2] + c) >>> 0; h[3] = (h[3] + d) >>> 0;
    h[4] = (h[4] + e) >>> 0; h[5] = (h[5] + f) >>> 0; h[6] = (h[6] + g) >>> 0; h[7] = (h[7] + hh) >>> 0;
  }
  let hex = '';
  for (let i = 0; i < 8; i++) hex += h[i].toString(16).padStart(8, '0');
  return hex;
}

/** 导出下载授权失效（过期/吊销后兑付被拒）：可重取授权后重试的 typed 恢复码。 */
export const EXPORT_GRANT_EXPIRED = 'export_grant_invalid';
/** 下载字节与授权 digest 不一致：内容不是该确定版本，拒绝打开。 */
export const EXPORT_DIGEST_MISMATCH = 'export_digest_mismatch';
const MAX_EXPORT_BYTES = 20 * 1024 * 1024;
const MAX_DOWNLOAD_ERROR_BYTES = 4096;

/** 一次兑付下载的校验记录（页面如实展示校验层级与结果）。 */
export interface ExportOpenRecord {
  format: 'pdf' | 'docx';
  version: number;
  size: number;
  expectedDigest: string;
  actualDigest: string;
  digestMatched: boolean;
  opened: boolean;
  checkedAt: string;
}
/** 与 api-client MaterialExportDownload 同形的授权（解耦包边界的结构副本）。 */
export interface ExportDownloadGrant {
  exportId: string;
  materialId: string;
  version: number;
  format: 'pdf' | 'docx';
  digest: string;
  size: number;
  expiresAt: number;
  signature: string;
  url: string;
}

export interface ExportOpenDeps {
  bearer?: () => string;
  origin?: () => string;
}
interface ExportPlatformParts { bearer(): string; origin(): string }

function defaultExportParts(): ExportPlatformParts {
  // 运行时认证凭据与可信 origin 与 T06 files.ts 同源（services/runtime 单例）。
  const credential = auth.credential();
  if (credential.kind !== 'bearer') throw new Error('AUTH_REQUIRED');
  return {
    bearer: () => {
      const current = auth.credential();
      if (current.kind !== 'bearer') throw new Error('AUTH_REQUIRED');
      return current.accessToken;
    },
    origin: () => apiOrigin,
  };
}

interface DownloadOutcome { statusCode: number; tempFilePath: string }
function taroDownload(options: { url: string; header: Record<string, string>; timeout: number }): Promise<DownloadOutcome> {
  return new Promise((resolve, reject) => {
    Taro.downloadFile({
      ...options, success: res => {
        if (res.statusCode !== 200) {
          const code = downloadErrorCode(res.tempFilePath);
          reject(Object.assign(new Error(code === EXPORT_GRANT_EXPIRED ? '下载授权已失效，可重新获取' : '下载失败'), { status: res.statusCode, ...(code ? { code } : {}) }));
          return;
        }
        resolve({ statusCode: res.statusCode, tempFilePath: res.tempFilePath });
      }, fail: e => reject(Object.assign(new Error('下载失败'), { cause: e })),
    });
  });
}

function exportUserCopyPath(name: string): string {
  // D3（T06 实测）：运行时临时文件不可删；副本必须落在 USER_DATA_PATH 且保留原始
  // 扩展名（openDocument 靠扩展名识别类型），打开后立即删除我们创建的副本。
  const env = (globalThis as { wx?: { env?: { USER_DATA_PATH?: string } } }).wx?.env;
  const dir = env?.USER_DATA_PATH;
  if (!dir) throw new Error('本机存储目录不可用');
  const dot = name.lastIndexOf('.');
  const ext = dot > 0 ? name.slice(dot + 1).toLowerCase() : '';
  const stem = (dot > 0 ? name.slice(0, dot) : name).replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^[-.]+/, '') || 'material';
  const suffix = `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
  return `${dir}/wk-export-${stem}-${suffix}${ext ? `.${ext}` : ''}`;
}

function downloadErrorCode(filePath: string): string | undefined {
  try {
    const fs = Taro.getFileSystemManager();
    const body = fs.readFileSync(filePath, 'utf8', 0, MAX_DOWNLOAD_ERROR_BYTES) as unknown;
    if (typeof body !== 'string') return undefined;
    const parsed = JSON.parse(body) as { code?: unknown; error?: { code?: unknown } };
    const code = parsed.code ?? parsed.error?.code;
    return typeof code === 'string' ? code : undefined;
  } catch { return undefined }
}

/**
 * 兑付一份导出下载授权：带认证下载 → 全字节 sha256 与授权 digest 比对 → 私有副本
 * 打开 → 立即清理。授权失效抛 EXPORT_GRANT_EXPIRED（上层重取授权后重试）。
 */
export async function openExportedDocument(grant: ExportDownloadGrant, deps: ExportOpenDeps = {}): Promise<ExportOpenRecord> {
  if (!grant.url.startsWith('/api/v1/career/materials/') || grant.url.includes('://') || grant.url.includes('..')) throw new Error('不可信下载路径');
  // 与 Web 客户端一致：不直接取回执里的 url，用解码后的授权字段重建兑付路径。
  const path = `/api/v1/career/materials/${encodeURIComponent(grant.materialId)}/exports/${encodeURIComponent(grant.exportId)}/download?format=${grant.format}&expires=${grant.expiresAt}&signature=${encodeURIComponent(grant.signature)}`;
  const parts: ExportPlatformParts = deps.bearer && deps.origin ? { bearer: deps.bearer, origin: deps.origin } : defaultExportParts();
  const fs = Taro.getFileSystemManager();
  const outcome = await taroDownload({ url: `${parts.origin()}${path}`, header: { Authorization: `Bearer ${parts.bearer()}` }, timeout: 60000 });
  const info = await Taro.getFileInfo({ filePath: outcome.tempFilePath }) as { size: number };
  if (info.size > MAX_EXPORT_BYTES) throw new Error('文件超出本端查看上限');
  const buffer = fs.readFileSync(outcome.tempFilePath) as unknown as ArrayBuffer;
  const bytes = new Uint8Array(buffer);
  const actualDigest = sha256Hex(bytes);
  const record: ExportOpenRecord = {
    format: grant.format, version: grant.version, size: bytes.length,
    expectedDigest: grant.digest, actualDigest, digestMatched: actualDigest === grant.digest,
    opened: false, checkedAt: new Date().toISOString(),
  };
  if (!record.digestMatched) throw Object.assign(new Error('文件校验失败：下载内容与授权版本不一致，已拒绝打开'), { code: EXPORT_DIGEST_MISMATCH });
  const userCopy = exportUserCopyPath(`career-material-v${grant.version}.${grant.format}`);
  let failure: { failed: boolean; error: unknown } = { failed: false, error: undefined };
  try {
    await new Promise<void>((resolve, reject) => fs.copyFile({ srcPath: outcome.tempFilePath, destPath: userCopy, success: () => resolve(), fail: e => reject(Object.assign(new Error('本机文件准备失败'), { cause: e })) }));
    await Taro.openDocument({ filePath: userCopy, showMenu: true });
    record.opened = true;
  } catch (error) { failure = { failed: true, error }; }
  finally {
    if (userCopy) {
      // 清理失败不静默（D3/T06）：UI 承诺打开后立即清理，违背承诺必须可见。
      try { await new Promise<void>((resolve, reject) => fs.unlink({ filePath: userCopy, success: () => resolve(), fail: e => reject(Object.assign(new Error(`临时副本清理失败：${userCopy}`), { cause: e })) })) }
      catch (cleanupError) { if (!failure.failed) failure = { failed: true, error: cleanupError } }
    }
  }
  if (failure.failed) throw failure.error;
  return record;
}

// ---- T32 全空间导出的本端留存 seam：等效可取得流程，不静默截断 ----
// 微信小程序没有浏览器式下载（无 Blob/URL.createObjectURL/anchor 保存）；平台限制下的
// 等效可取得流程 = ①写入 USER_DATA_PATH 真实文件（本机持久目录，跨会话留存，路径如实
// 呈现）+ ②完整内容复制到剪贴板（可转移到任何长期留存位置）。写入字节 = 完整 JSON
// 载荷；本地副本摘要由落盘字节独立复算，用户可用文件字节复核。

/** 序列化整份导出回执（含内联归档）为可留存的 JSON 载荷——内容不截断。 */
export function spaceExportPayload(receipt: unknown): string {
  return JSON.stringify(receipt, null, 2);
}

/** 一次本机留存的如实记录：路径、字节长度与由落盘字节复算的摘要。 */
export interface SpaceExportSaveRecord { filePath: string; bytes: number; digest: string; savedAt: string }

/**
 * 把完整导出载荷写入 USER_DATA_PATH（weapp 唯一可写的本机持久目录）。写入后回读
 * 落盘字节计算 sha256——记录的是磁盘上真实内容的摘要，不是内存载荷的摘要。
 */
export async function saveSpaceExportPackage(payload: string, exportId: string): Promise<SpaceExportSaveRecord> {
  const env = (globalThis as { wx?: { env?: { USER_DATA_PATH?: string } } }).wx?.env;
  const dir = env?.USER_DATA_PATH;
  if (!dir) throw Object.assign(new Error('本机存储目录不可用，无法保存导出包'), { code: 'export_save_unavailable' });
  const stem = exportId.replace(/[^A-Za-z0-9._-]+/g, '-').replace(/^[-.]+/, '') || 'export';
  const filePath = `${dir}/career-export-${stem}.json`;
  const fs = Taro.getFileSystemManager();
  await new Promise<void>((resolve, reject) => fs.writeFile({
    filePath, data: payload, encoding: 'utf8',
    success: () => resolve(),
    fail: e => reject(Object.assign(new Error('导出包写入本机失败'), { cause: e })),
  }));
  // 无 encoding 的 readFileSync 返回精确文件字节的 ArrayBuffer（T26 保真契约）。
  const written = fs.readFileSync(filePath) as unknown as ArrayBuffer;
  const bytes = new Uint8Array(written).length;
  const digest = sha256Hex(new Uint8Array(written));
  return { filePath, bytes, digest, savedAt: new Date().toISOString() };
}

/** 等效可取得流程之二：完整载荷（不截断）复制到系统剪贴板，供转移到长期留存位置。 */
export async function copySpaceExportToClipboard(payload: string): Promise<void> {
  try {
    await Taro.setClipboardData({ data: payload });
  } catch (cause) {
    throw Object.assign(new Error('复制完整导出内容失败，可重试或先保存本机文件'), { cause });
  }
}
