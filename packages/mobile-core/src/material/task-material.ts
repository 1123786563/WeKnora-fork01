import { leaseActive, leaseScopeOf } from '../runtime/scope-lease.ts';
import { MaterialError } from './material-errors.ts';
import { isInlineImageMime, materialKindOf, previewVerdictOf, PREVIEW_MAX_BYTES } from './material-kinds.ts';
import { parseUnifiedDiff } from './diff.ts';
import { projectCitations } from './evidence.ts';
import type {
  MaterialBackendArtifact, MaterialBackendGrant, MaterialBackendPort, TaskMaterialPorts,
} from './ports.ts';
import type {
  MaterialActResult, MaterialEntry, MaterialEvent, MaterialIndex, MaterialIntent, MaterialRef,
  MaterialView, TaskMaterial, TaskMaterialHandle,
} from './types.ts';

const TERMINAL_PAGE_LIMIT = 200;

/** 字节缓存条目上限（B3-F31）：单条 ≤2MB（PREVIEW_MAX_BYTES）→ 常驻上限 ~12MB。 */
const MAX_BLOB_ENTRIES = 6;

/** 结构化读取（不读 message 字面量）：与 shelf 的 httpStatus 同一纪律。 */
function errorStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

function errorCode(error: unknown): string | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === 'string' && code !== '' ? code : undefined;
}

function decodeUtf8(bytes: Uint8Array): string {
  if (typeof TextDecoder !== 'undefined') return new TextDecoder('utf-8').decode(bytes);
  // 无 TextDecoder 环境的正确 UTF-8 解码回退（B3-F34）：码点循环替代 Latin-1 拼接。
  let out = '';
  for (let index = 0; index < bytes.length; ) {
    const byte0 = bytes[index]!;
    let codePoint: number;
    let width: number;
    if (byte0 < 0x80) { codePoint = byte0; width = 1; }
    else if ((byte0 & 0xe0) === 0xc0) { codePoint = byte0 & 0x1f; width = 2; }
    else if ((byte0 & 0xf0) === 0xe0) { codePoint = byte0 & 0x0f; width = 3; }
    else if ((byte0 & 0xf8) === 0xf0) { codePoint = byte0 & 0x07; width = 4; }
    else { codePoint = 0xfffd; width = 1; } // 无效前导字节以替换符落地，不抛错
    for (let offset = 1; offset < width && index + offset < bytes.length; offset += 1) {
      const continuation = bytes[index + offset]!;
      codePoint = (codePoint << 6) | (continuation & 0x3f);
    }
    out += String.fromCodePoint(codePoint > 0x10ffff ? 0xfffd : codePoint);
    index += width;
  }
  return out;
}

function toEntry(row: MaterialBackendArtifact): MaterialEntry {
  const version = typeof row.version === 'string' && row.version.trim() !== '' ? row.version : row.id;
  return {
    materialId: row.id,
    index: row.index,
    kind: materialKindOf(row.name, row.mime),
    name: row.name,
    mime: row.mime,
    size: row.size,
    version,
    sourceRun: row.sourceRun,
    ...(row.createdAt === undefined || row.createdAt === '' ? {} : { createdAt: row.createdAt }),
    ...(row.digest === undefined || row.digest === '' ? {} : { digest: row.digest }),
  };
}

/**
 * Task Material 深模块（module-seams §7）。MIME/大小判定、签名 grant 铸造纪律
 * （每次新鲜铸造、origin 钉住活动部署、TTL 内不复用 URL）、版本键字节缓存、
 * diff 解析、终端日志分页与系统分享全部藏在句柄后（§7.2）。
 * 批注/基于版本请求修改属 #47，不在本模块意图集内。
 */
export function createTaskMaterial(ports: TaskMaterialPorts): TaskMaterial {
  return {
    open({ lease }) {
      const scope = leaseScopeOf(lease);
      if (!scope || !leaseActive(lease)) throw new MaterialError('MATERIAL_SCOPE_CHANGED');
      const listeners = new Set<(event: MaterialEvent) => void>();
      const blobs = new Map<string, Uint8Array>(); // key: `${materialId}@${version}`
      let closed = false;
      let lastIndex: MaterialIndex | undefined;

      const notify = (event: MaterialEvent): void => {
        for (const listener of [...listeners]) listener(event);
      };
      const guard = (): void => {
        if (closed) throw new MaterialError('MATERIAL_CLOSED');
        if (!leaseActive(lease)) throw new MaterialError('MATERIAL_SCOPE_CHANGED');
      };
      const mapError = (error: unknown): MaterialError => {
        if (error instanceof MaterialError) return error;
        const status = errorStatus(error);
        const code = errorCode(error);
        if (status === 501 && code === 'artifact_signing_disabled') return new MaterialError('MATERIAL_SIGNING_DISABLED', { cause: error });
        if (status === 401 && code === 'artifact_grant_expired') return new MaterialError('MATERIAL_GRANT_EXPIRED', { cause: error });
        if (status === 401) return new MaterialError('MATERIAL_GRANT_INVALID', { cause: error });
        return new MaterialError('MATERIAL_BACKEND', { cause: error });
      };
      const callRemote = async <T>(action: () => Promise<T>): Promise<T> => {
        try {
          return await action();
        } catch (error) {
          throw mapError(error);
        }
      };
      /** grant 铸造 + 身份回验 + origin 钉住：URL 只在当次调用内存活，永不缓存（AC1）。 */
      const mintGrant = async (runId: string, entry: MaterialEntry): Promise<MaterialBackendGrant> => {
        const grant = await callRemote(() => ports.remote.signedUrl({ runId, index: entry.index }));
        // signedUrl 按易变 index 铸造——回验响应自带的 artifact 身份，错位即拒绝
        // （fail closed，B3-F58）：校验数据已在手，零额外往返。
        if (grant.artifact.id !== entry.materialId || grant.artifact.version !== entry.version) {
          throw new MaterialError('MATERIAL_GRANT_MISMATCH', { cause: new Error(`grant artifact ${grant.artifact.id}@${grant.artifact.version} does not match requested ${entry.materialId}@${entry.version}`) });
        }
        let origin: string;
        try {
          origin = new URL(grant.url).origin;
        } catch {
          throw new MaterialError('MATERIAL_GRANT_ORIGIN', { cause: new Error(`unparsable grant url: ${grant.url}`) });
        }
        if (origin !== scope.deploymentOrigin) {
          throw new MaterialError('MATERIAL_GRANT_ORIGIN', { cause: new Error(`grant origin ${origin} is not the active deployment`) });
        }
        return grant;
      };
      const fetchBytes = async (runId: string, entry: MaterialEntry): Promise<Uint8Array> => {
        const cacheKey = `${entry.materialId}@${entry.version}`;
        const cached = blobs.get(cacheKey);
        if (cached !== undefined) {
          blobs.delete(cacheKey); // LRU 命中重排（B3-F31）
          blobs.set(cacheKey, cached);
          return cached;
        }
        const grant = await mintGrant(runId, entry);
        let fetched: { bytes: Uint8Array; mime: string };
        try {
          fetched = await ports.blob.fetch(grant.url);
        } catch (error) {
          const mapped = mapError(error);
          if (mapped.code === 'MATERIAL_GRANT_EXPIRED') notify({ type: 'grant-expired', materialId: entry.materialId });
          throw mapped;
        }
        // 实际字节数复检预览上限（B3-F32）：后端声明的 size 之外，真实超限不入缓存。
        if (fetched.bytes.length > PREVIEW_MAX_BYTES) {
          throw new MaterialError('MATERIAL_INVALID_INPUT', { cause: new Error(`fetched ${fetched.bytes.length} bytes exceeds the preview budget`) });
        }
        blobs.delete(cacheKey);
        blobs.set(cacheKey, fetched.bytes);
        while (blobs.size > MAX_BLOB_ENTRIES) blobs.delete(blobs.keys().next().value!); // LRU 逐出最旧（B3-F31）
        return fetched.bytes;
      };
      const entryFor = async (runId: string, materialId: string): Promise<MaterialEntry> => {
        let index = lastIndex;
        if (index === undefined || index.runId !== runId) index = await loadIndex(runId);
        const entry = index.materials.find((candidate) => candidate.materialId === materialId);
        if (entry === undefined) throw new MaterialError('MATERIAL_NOT_FOUND');
        return entry;
      };
      const loadIndex = async (runId: string): Promise<MaterialIndex> => {
        const list = await callRemote(() => ports.remote.list(runId));
        guard();
        const next: MaterialIndex = { runId, materials: list.artifacts.map(toEntry), terminal: { available: list.terminalAvailable } };
        lastIndex = next;
        return next;
      };

      const handle: TaskMaterialHandle = {
        async index(input): Promise<MaterialIndex> {
          guard();
          const runId = input.runId.trim();
          if (runId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          try {
            return await loadIndex(runId);
          } catch (error) {
            // 失败清空旧投影：撤权/故障后不回填旧材料（AC 与 shelf 冻结规则同源）。
            lastIndex = undefined;
            throw error;
          }
        },
        async open(ref): Promise<MaterialView> {
          guard();
          const runId = ref.runId.trim();
          if (runId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          if (ref.kind === 'evidence') {
            const events = await callRemote(() => ports.remote.events(runId));
            guard();
            return { kind: 'evidence', citations: projectCitations(events) };
          }
          if (ref.kind === 'terminal') {
            const after = typeof ref.cursor === 'number' && Number.isSafeInteger(ref.cursor) && ref.cursor >= 0 ? ref.cursor : 0;
            const page = await callRemote(() => ports.remote.terminalLog({ runId, after, limit: TERMINAL_PAGE_LIMIT }));
            guard();
            return {
              kind: 'terminal',
              lines: page.lines.map((line) => ({ seq: line.seq, occurredAt: line.occurredAt, stream: line.stream, text: line.text })),
              ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
              readOnly: true,
            };
          }
          const materialId = ref.materialId.trim();
          if (materialId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          const entry = await entryFor(runId, materialId);
          const verdict = previewVerdictOf(entry.mime, entry.size);
          if (verdict.state === 'unsupported') {
            // 判定先于一切网络调用（Review Focus 5）。
            return entry.kind === 'diff'
              ? { kind: 'diff', entry, preview: verdict, hunks: [], malformed: false }
              : { kind: entry.kind, entry, preview: verdict };
          }
          const bytes = await fetchBytes(runId, entry);
          guard();
          if (entry.kind === 'diff') {
            const raw = decodeUtf8(bytes);
            const parsed = parseUnifiedDiff(raw);
            return { kind: 'diff', entry, preview: verdict, hunks: parsed.hunks, malformed: parsed.malformed, raw };
          }
          if (isInlineImageMime(entry.mime)) return { kind: entry.kind, entry, preview: verdict, bytes };
          return { kind: entry.kind, entry, preview: verdict, text: decodeUtf8(bytes) };
        },
        async act(intent): Promise<MaterialActResult> {
          guard();
          if (intent.kind === 'terminal-input') {
            // 只读终端：任何输入尝试一律拒绝（AC2）；移动面不存在 PTY 写通道。
            throw new MaterialError('MATERIAL_TERMINAL_READ_ONLY');
          }
          const runId = intent.runId.trim();
          const materialId = intent.materialId.trim();
          if (runId === '' || materialId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          const entry = await entryFor(runId, materialId);
          const share = ports.share;
          if (intent.kind === 'share' && share === undefined) {
            // 先检查端口再铸造（B3-F33）：无系统分享通道时不白铸短时效签名 URL。
            throw new MaterialError('MATERIAL_SHARE_UNAVAILABLE');
          }
          const grant = await mintGrant(runId, entry); // 每次 act 新鲜铸造，不复用任何旧 URL（AC1）
          guard();
          if (intent.kind === 'download') {
            return { kind: 'grant', materialId: entry.materialId, url: grant.url, expiresAt: grant.expiresAt };
          }
          if (share === undefined) throw new MaterialError('MATERIAL_SHARE_UNAVAILABLE'); // 运行时双保险（download 已返回）
          try {
            await share.share({ url: grant.url, name: entry.name });
          } catch (error) {
            throw mapError(error);
          }
          guard();
          return { kind: 'shared', materialId: entry.materialId };
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => { listeners.delete(listener); };
        },
        close(reason) {
          if (closed) return;
          closed = true;
          blobs.clear();
          lastIndex = undefined;
          notify({ type: 'scope-closed', reason });
        },
      };
      return handle;
    },
  };
}
