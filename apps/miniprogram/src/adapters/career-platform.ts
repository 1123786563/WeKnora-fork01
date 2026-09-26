import Taro from '@tarojs/taro';
import type { NativeFileSource } from '@weknora/api-client';
import type { MaterialBody, RuleStatus, SetRuleReceipt, RuleView, UsageEstimateView, ReminderReceipt, ReminderList, ReminderSourceKind } from '../../../../packages/api-client/src/career.ts';
import { decodeSetRuleReceipt, decodeRuleView, decodeUsageEstimateView, decodeReminderReceipt, decodeReminderList } from '../../../../packages/api-client/src/career.ts';
import type { CareerReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { decodeCareerReceipt } from '../../../../packages/career-core/src/contracts.ts';
import { storage } from '../platform/storage.ts';
import { chooseDocument } from '../platform/files.ts';
import { requestId } from '../core/intent.ts';
import type { ValueStore } from '../core/intent.ts';
// T26 导出兑付下载与 T06 files.ts 同源：认证凭据与可信 origin 只来自 runtime 单例。
// runtime 不反向依赖本适配器，无环。
import { auth, apiOrigin, client } from '../services/runtime.ts';
import { scopeKey } from '../core/scope.ts';

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

// ---- T28 面试准备本地草稿 seam：断网可再编辑、按 scope 隔离、登出即清 ----
// 断网语义（简报第 5 节）：网络失败只保留本地草稿（可继续编辑），绝不静默改申请状态，
// 重联网也不自动提交——同步永远是用户显式动作（页面上的保存/对账/重试）。草稿键 =
// 前缀 + scopeKey（origin+userId+tenantId）+ 申请 + 焦点：账号切换后读不到前一用户的
// 草稿；登出 clearPrivateCache 清 wk:career:* 时整段一并清除。
// 主张保全（R1-F1/F2）：本地草稿携带各节 claims 快照；恢复编辑与提交前都从材料域正文
// 取回主张——本端不编辑主张，也绝不静默丢弃 Web/生成链路建立的主张。

/** 材料小节主张（api-client MaterialBody 同形，解耦包边界的结构引用）。 */
export type PreparationSectionClaim = MaterialBody['sections'][number]['claims'][number];
/** 面试准备本地草稿的可编辑小节（claims 不在本端编辑，随草稿往返保留）。 */
export interface PreparationDraftSection { heading: string; content: string; claims?: PreparationSectionClaim[] }
/** 一次保留在本机的准备草稿：断网期间的用户输入，联网后由用户显式提交。 */
export interface PreparationDraftRecord {
  applicationId: string;
  focus: string;
  preparationId?: string;
  materialId?: string;
  sections: PreparationDraftSection[];
  savedAt: string;
}
export const PREPARATION_DRAFT_PREFIX = `${CAREER_STORE_PREFIX}prep-draft:`;
const draftStore = createControlledStore();
function preparationDraftKey(applicationId: string, focus: string): string {
  return `${PREPARATION_DRAFT_PREFIX}${scopeKey(auth.scope.capture())}:${applicationId.trim()}:${focus}`;
}
/** 保留（或覆盖）当前作用域的一份准备草稿。返回实际写入的受控键。 */
export function savePreparationDraft(record: PreparationDraftRecord): string {
  if (!record.applicationId.trim() || !record.focus.trim()) throw new Error('准备草稿缺少申请或焦点');
  const key = preparationDraftKey(record.applicationId, record.focus);
  draftStore.write(key, { ...record, savedAt: new Date().toISOString() });
  return key;
}
/** 读取当前作用域的准备草稿；无草稿或结构不符返回 undefined（绝不用演示数据顶替）。 */
export function readPreparationDraft(applicationId: string, focus: string): PreparationDraftRecord | undefined {
  const value = draftStore.read(preparationDraftKey(applicationId, focus));
  if (!value || typeof value !== 'object') return undefined;
  const parsed = value as Record<string, unknown>;
  if (typeof parsed.applicationId !== 'string' || typeof parsed.focus !== 'string' || typeof parsed.savedAt !== 'string') return undefined;
  const sections = Array.isArray(parsed.sections) ? parsed.sections : undefined;
  if (!sections?.every(section => {
    if (!section || typeof section !== 'object') return false;
    const s = section as Record<string, unknown>;
    return typeof s.heading === 'string' && typeof s.content === 'string' && (s.claims === undefined || Array.isArray(s.claims));
  })) return undefined;
  return {
    applicationId: parsed.applicationId, focus: parsed.focus,
    ...(typeof parsed.preparationId === 'string' ? { preparationId: parsed.preparationId } : {}),
    ...(typeof parsed.materialId === 'string' ? { materialId: parsed.materialId } : {}),
    sections: (sections as PreparationDraftSection[]).map(section => ({ heading: section.heading, content: section.content, claims: claimsOf(section) })),
    savedAt: parsed.savedAt,
  };
}
/** 修订成功提交后清除本地草稿（草稿只是断网过渡态，不是第二份事实源）。 */
export function clearPreparationDraft(applicationId: string, focus: string): void {
  draftStore.remove(preparationDraftKey(applicationId, focus));
}

/** 把小节的 claims 规整成数组（undefined/null → []；其余原样）。 */
function claimsOf(section: PreparationDraftSection): PreparationSectionClaim[] {
  return Array.isArray(section.claims) ? section.claims : [];
}
/** 合并/取回结果：claims 恒为数组（可直接进入材料正文）。 */
export type ClaimedDraftSection = PreparationDraftSection & { claims: PreparationSectionClaim[] };

/**
 * 恢复编辑时的本地优先合并（R1-F2）：以本地草稿为基数（它是用户最新编辑态，可能新增
 * 或删除小节——服务端为基数会把本地新增节静默丢掉），主张一律从服务端正文按同名小节
 * 取回（服务端是主张的事实源）；本地已删除的服务端小节不复活；真正的本地新增小节从
 * 无主张开始。
 */
export function attachClaimsFromServer(local: PreparationDraftSection[], server: PreparationDraftSection[]): ClaimedDraftSection[] {
  const unconsumed = server.map(section => ({ heading: section.heading, claims: claimsOf(section) }));
  return local.map(section => {
    const at = unconsumed.findIndex(candidate => candidate.heading.trim() === section.heading.trim() && candidate.claims.length > 0);
    if (at >= 0) { const [matched] = unconsumed.splice(at, 1); return { heading: section.heading, content: section.content, claims: matched.claims }; }
    return { heading: section.heading, content: section.content, claims: claimsOf(section) };
  });
}

/**
 * 提交前主张保全（R1-F1）：claims 为空的小节，若材料域当前正文有同名小节的主张则取回
 * 合并；已有主张的小节绝不改写；无同名服务端小节的（真正新增节）保持无主张。服务端
 * 材料编辑是整体替换草稿正文（material.go DraftBody）——缺了这一步，断网独立入口保存
 * 会把 claims 为空的正文整体提交，静默丢弃已建立的主张。
 */
export function recoverEmptyClaims(sections: PreparationDraftSection[], server: PreparationDraftSection[]): ClaimedDraftSection[] {
  const unconsumed = server.map(section => ({ heading: section.heading, claims: claimsOf(section) }));
  return sections.map(section => {
    const claims = claimsOf(section);
    if (claims.length > 0) return { heading: section.heading, content: section.content, claims };
    const at = unconsumed.findIndex(candidate => candidate.heading.trim() === section.heading.trim() && candidate.claims.length > 0);
    return { heading: section.heading, content: section.content, claims: at >= 0 ? unconsumed.splice(at, 1)[0].claims : [] };
  });
}

// ---- T30 持续规则、额度与提醒：与 Web 同版本的服务层 + wx.requestSubscribeMessage 原生例外 ----
// 规则（T13 冻结合同）：POST /rules、GET /rules/receipt、GET /rules/:id；写入一律
// requestId + expectedRevision（档案头修订——与 Web RulePage 同一 CAS 域，同一版本
// 读写）；修改后下次运行计划（nextDueAt）由服务端按新条件重新排程，本端只呈现。
// 额度（T21）：GET /usage/estimate 是只读预估，前端如实展示（含触发条件原文）绝不
// 重算；超额只阻新收费动作（typed 429 search_quota_refused），档案/申请/时间线等
// 历史读取永不受影响。提醒（T20）：POST/GET /reminders 正文为服务端冻结隐私模板
// （progress_updated / discovery_found 两个字面量，零公司/岗位/面试细节），退订
// （notifications.push=unsubscribed）只停推送，站内待办永读。
// 订阅消息 = wx.requestSubscribeMessage（平台原生 API——已记录的原生能力例外，不在
// TDesign 组件域）：原生载荷只携带模板 id；拒绝/不可用如实呈现并回落站内待办；
// 本端绝不声称“已送达”——送达是服务端后续推送的事实，回执里的 push 报告也只是
// response-only 的尽力通知结果。

/** T30 写入 intent 的受控存储（按 scope 隔离；登出 clearPrivateCache 一并清除）。 */
const t30Store = createControlledStore();
function t30Key(kind: string): string {
  return `${CAREER_STORE_PREFIX}${kind}:${scopeKey(auth.scope.capture())}`;
}
/** 与 services/career.ts 同一判据的本地副本（该文件不在 T30 所有权内）：确定失败
 * （typed 拒绝/4xx）不进恢复链；歧义失败（超时/网络/5xx）才保留 intent 供对账。 */
function t30Ambiguous(error: unknown): boolean {
  const code = (error as { code?: unknown })?.code;
  if (typeof code === 'string') {
    if (['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED', 'outcome_unknown'].includes(code)) return true;
    if (['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved', 'search_quota_refused'].includes(code)) return false;
  }
  const status = (error as { status?: unknown })?.status;
  return typeof status !== 'number' || status >= 500;
}
async function t30Recoverable<T>(kind: string, describe: string, input: unknown, send: (id: string) => Promise<T>, reuseId?: string): Promise<T> {
  const id = reuseId ?? requestId();
  const stamp = auth.scope.capture();
  try {
    return await send(id);
  } catch (error) {
    if (t30Ambiguous(error) && !auth.scope.isCurrent(stamp)) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
    if (t30Ambiguous(error)) {
      t30Store.write(t30Key(kind), { requestId: id, input });
      throw Object.assign(new Error(`${describe}结果未知：请用原请求对账后再试`, { cause: error }), { code: 'outcome_unknown', requestId: id });
    }
    throw error;
  }
}
function t30Intent<T>(kind: string): { requestId: string; input: T } | null {
  const value = t30Store.read(t30Key(kind));
  if (!value || typeof value !== 'object') return null;
  const parsed = value as { requestId?: unknown; input?: unknown };
  if (typeof parsed.requestId !== 'string' || !parsed.requestId || typeof parsed.input !== 'object' || !parsed.input) return null;
  return { requestId: parsed.requestId, input: parsed.input as T };
}
function t30Clear(kind: string): void { t30Store.remove(t30Key(kind)); }

// —— 规则（与 Web 同版本读写）——
export interface RuleWriteInput { ruleId?: string; query: string; intervalMinutes: number; status: RuleStatus; expectedRevision: number }
function ruleRequestBody(id: string, input: RuleWriteInput): Record<string, unknown> {
  const query = input.query.trim();
  if (!query) throw Object.assign(new Error('请先填写找岗条件'), { code: 'invalid_request', recoverable: true });
  if (!Number.isSafeInteger(input.intervalMinutes) || input.intervalMinutes < 1 || input.intervalMinutes > 43200) throw Object.assign(new Error('触发间隔需为 1–43200 的整数分钟'), { code: 'invalid_request', recoverable: true });
  if (!['enabled', 'paused', 'disabled'].includes(input.status)) throw Object.assign(new Error('无效的规则状态'), { code: 'invalid_request', recoverable: true });
  if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw Object.assign(new Error('缺少档案修订'), { code: 'invalid_request', recoverable: true });
  return { requestId: id, query, intervalMinutes: input.intervalMinutes, status: input.status, expectedRevision: input.expectedRevision, ...(input.ruleId?.trim() ? { ruleId: input.ruleId.trim() } : {}) };
}
function acceptRuleReceipt(receipt: SetRuleReceipt): void {
  // 已保存规则（尤其启用中）会改变下一次收费运行的消耗口径——规则引用落受控存储，
  // 下次进入按同一 ruleId 读回同一版本（与 Web localStorage 引用同语义）。
  t30Store.write(t30Key('rule-id'), receipt.ruleId);
}
/** 保存（创建或更新）规则：POST /rules 冻结合同；结果未知保留 intent 供原 requestId 对账。 */
export async function saveRule(input: RuleWriteInput): Promise<SetRuleReceipt> {
  ruleRequestBody(requestId(), input); // 先做参数校验，再进入可恢复写入
  return t30Recoverable<SetRuleReceipt>('rule-write', '规则保存', input, async id => {
    const receipt = decodeSetRuleReceipt(await client.request({ method: 'POST', path: '/api/v1/career/rules', body: ruleRequestBody(id, input) }));
    acceptRuleReceipt(receipt);
    return receipt;
  });
}
export function pendingRuleWrite(): { requestId: string; input: RuleWriteInput } | null { return t30Intent<RuleWriteInput>('rule-write'); }
/** 用原 requestId 读取服务端持久回执（只读，不二次写入）。 */
export async function reconcilePendingRule(): Promise<SetRuleReceipt> {
  const pending = pendingRuleWrite();
  if (!pending) throw new Error('没有待对账的规则保存');
  const receipt = decodeSetRuleReceipt(await client.request({ method: 'GET', path: `/api/v1/career/rules/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
  t30Clear('rule-write'); acceptRuleReceipt(receipt);
  return receipt;
}
/** 对账确认服务端无记录（404）后的安全重发：同 requestId + 原 expectedRevision 逐字节重放。 */
export async function retryPendingRule(): Promise<SetRuleReceipt> {
  const pending = pendingRuleWrite();
  if (!pending) throw new Error('没有待恢复的规则保存');
  return t30Recoverable<SetRuleReceipt>('rule-write', '规则保存', pending.input, async id => {
    const receipt = decodeSetRuleReceipt(await client.request({ method: 'POST', path: '/api/v1/career/rules', body: ruleRequestBody(id, pending.input) }));
    t30Clear('rule-write'); acceptRuleReceipt(receipt);
    return receipt;
  }, pending.requestId);
}
/** 当前作用域保存过的规则引用（未保存过返回 undefined——绝不用演示规则顶替）。 */
export function readStoredRuleId(): string | undefined {
  const value = t30Store.read(t30Key('rule-id'));
  return typeof value === 'string' && value.trim() ? value : undefined;
}
/** 读取规则当前版本（含下次运行计划、执行历史、发现待办）：GET /rules/:id。 */
export async function readRule(ruleId: string): Promise<RuleView> {
  if (!ruleId.trim()) throw Object.assign(new Error('缺少规则编号'), { code: 'invalid_request', recoverable: true });
  return decodeRuleView(await client.request({ method: 'GET', path: `/api/v1/career/rules/${encodeURIComponent(ruleId.trim())}` }));
}

// —— 额度（执行前只读预估）——
/** GET /usage/estimate?operation=search_once：后端冻结数字与触发条件原文，本端不重算。 */
export async function fetchUsageEstimate(): Promise<UsageEstimateView> {
  return decodeUsageEstimateView(await client.request({ method: 'GET', path: '/api/v1/career/usage/estimate?operation=search_once' }));
}

// —— 提醒（站内待办收件箱 + 登记 + 推送退订）——
export interface ReminderWriteInput { sourceKind: ReminderSourceKind; sourceId: string; expectedRevision: number }
/** 站内待办列表：正文是服务端冻结隐私模板字面量，原样呈现。 */
export async function fetchReminders(): Promise<ReminderList> {
  return decodeReminderList(await client.request({ method: 'GET', path: '/api/v1/career/reminders' }));
}
/** 为一个来源事件登记待办（POST /reminders 冻结合同）；同一来源只保留一条（deduplicated）。 */
export async function createReminder(input: ReminderWriteInput): Promise<ReminderReceipt> {
  if (!input.sourceId.trim()) throw Object.assign(new Error('请先填写来源事件编号'), { code: 'invalid_request', recoverable: true });
  return t30Recoverable<ReminderReceipt>('reminder-write', '待办登记', input, async id =>
    decodeReminderReceipt(await client.request({ method: 'POST', path: '/api/v1/career/reminders', body: { requestId: id, sourceKind: input.sourceKind, sourceId: input.sourceId.trim(), expectedRevision: input.expectedRevision } })));
}
export function pendingReminderWrite(): { requestId: string; input: ReminderWriteInput } | null { return t30Intent<ReminderWriteInput>('reminder-write'); }
export async function reconcilePendingReminder(): Promise<ReminderReceipt> {
  const pending = pendingReminderWrite();
  if (!pending) throw new Error('没有待对账的待办登记');
  const receipt = decodeReminderReceipt(await client.request({ method: 'GET', path: `/api/v1/career/reminders/receipt?requestId=${encodeURIComponent(pending.requestId)}` }));
  t30Clear('reminder-write');
  return receipt;
}
export async function retryPendingReminder(): Promise<ReminderReceipt> {
  const pending = pendingReminderWrite();
  if (!pending) throw new Error('没有待恢复的待办登记');
  return t30Recoverable<ReminderReceipt>('reminder-write', '待办登记', pending.input, async id => {
    const receipt = decodeReminderReceipt(await client.request({ method: 'POST', path: '/api/v1/career/reminders', body: { requestId: id, sourceKind: pending.input.sourceKind, sourceId: pending.input.sourceId.trim(), expectedRevision: pending.input.expectedRevision } }));
    t30Clear('reminder-write');
    return receipt;
  }, pending.requestId);
}
/** 推送订阅事实键（服务端 reminder.go 冻结常量的本端镜像）。 */
export const REMINDER_PUSH_FACT_KEY = 'notifications.push';
/** 退订/重新订阅推送（档案 confirm 事实写）：退订只停推送，站内待办仍可读。 */
export async function setPushSubscription(value: 'subscribed' | 'unsubscribed', expectedRevision: number): Promise<CareerReceipt> {
  return decodeCareerReceipt(await client.request({
    method: 'POST', path: '/api/v1/career/act',
    body: { action: 'confirm', key: REMINDER_PUSH_FACT_KEY, value, source: { kind: 'user', label: '微信小程序' }, requestId: requestId(), expectedRevision },
  }));
}

// —— 订阅消息（wx.requestSubscribeMessage 原生例外 seam）——
/** 本构建已配置的订阅消息模板 id（微信公众平台申请，与 appid 绑定）。空数组 = 未
 * 配置：如实按“不可用”呈现并回落站内待办，绝不把未配置伪装成已订阅。 */
export const REMINDER_SUBSCRIBE_TEMPLATE_IDS: string[] = [];
/** 订阅请求注入 seam（测试与真实运行时共用）：默认调 wx.requestSubscribeMessage。 */
export type SubscribeInvoke = (options: { tmplIds: string[]; success: (res: Record<string, unknown>) => void; fail: (error: { errMsg: string }) => void }) => void;
function defaultSubscribeInvoke(): SubscribeInvoke | undefined {
  const wxApi = (globalThis as { wx?: { requestSubscribeMessage?: SubscribeInvoke } }).wx;
  return wxApi?.requestSubscribeMessage;
}
/** 一次订阅请求的如实结果：accepted 只代表本次授权，绝不代表已送达。 */
export interface SubscriptionRequestOutcome {
  status: 'accepted' | 'rejected' | 'unavailable';
  reason?: 'no_templates' | 'api_unavailable' | 'request_failed' | 'main_switch_off';
  templateIds: string[];
  /** 每个模板的授权结果（accept/reject/ban/filter，微信原样词汇）。 */
  results?: Record<string, string>;
  /** 不可用/失败时的原生 errMsg（如实呈现环境限制）。 */
  errMsg?: string;
  /** 本端永不声称送达：送达是服务端后续推送的事实。 */
  delivered: false;
}
export interface ReminderSubscriptionDeps { templateIds?: string[]; invoke?: SubscribeInvoke }
/**
 * 请求订阅消息授权：原生载荷只有模板 id（零敏感细节——正文口径在后端冻结隐私模板，
 * 本端不拼接任何公司/岗位/面试内容）。用户拒绝、主开关关闭、API 不可用（模拟器/
 * 未配置模板）都如实返回，由 UI 回落站内待办并声明限制——不伪造任何“已送达”。
 */
export function requestReminderSubscription(deps: ReminderSubscriptionDeps = {}): Promise<SubscriptionRequestOutcome> {
  const templateIds = (deps.templateIds ?? REMINDER_SUBSCRIBE_TEMPLATE_IDS).filter(id => typeof id === 'string' && id.trim());
  const base = { templateIds, delivered: false as const };
  if (templateIds.length === 0) return Promise.resolve({ ...base, status: 'unavailable', reason: 'no_templates' });
  const invoke = deps.invoke ?? defaultSubscribeInvoke();
  if (!invoke) return Promise.resolve({ ...base, status: 'unavailable', reason: 'api_unavailable' });
  return new Promise<SubscriptionRequestOutcome>(resolve => {
    invoke({
      tmplIds: templateIds,
      success: res => {
        const results: Record<string, string> = {};
        for (const id of templateIds) { const value = (res as Record<string, unknown>)[id]; if (typeof value === 'string') results[id] = value; }
        const values = Object.values(results);
        if (values.length > 0 && values.every(value => value === 'accept')) resolve({ ...base, status: 'accepted', results });
        else resolve({ ...base, status: 'rejected', results });
      },
      fail: error => {
        const message = error?.errMsg ?? '';
        // 主开关关闭（用户在设置里整体拒绝订阅）按“拒绝”呈现；其余失败（模板无效、
        // 模拟器不支持等）按“不可用”如实呈现并保留 errMsg。
        if (/main\s*switch|20004/i.test(message)) resolve({ ...base, status: 'rejected', reason: 'main_switch_off', errMsg: message });
        else resolve({ ...base, status: 'unavailable', reason: 'request_failed', errMsg: message });
      },
    });
  });
}
/** 订阅消息口径说明（页面隐私文案）：与后端冻结隐私模板一致的提醒语义。 */
export const REMINDER_PUSH_PRIVACY_NOTE = '订阅消息只提醒「有更新」，正文为平台固定的隐私文案，不含公司、岗位或面试细节；完整事实始终在站内待办里。';
