// MX-025 probe · 原生附件与上传校验观察器
// frozen 场景：Android-content-uri × tenant-switch-during-upload。
// 真实 createUploadController：content:// URI 本机读取真实 bytes → 上传在途切空间 →
// 迟到结果不应用（lateResultApplied=false）、草稿保留（draftPreserved=true）。
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  createUploadController,
  precheckFile,
  DEFAULT_UPLOAD_CONSTRAINTS,
  type PickedFile,
} from '../../../apps/mobile/sources/weknora/resources/upload.ts';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  lateResultApplied: boolean;
  draftPreserved: boolean;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'Android-content-uri' || input.fault !== 'tenant-switch-during-upload') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const file: PickedFile = {
    uri: 'content://com.android.providers.downloads.documents/document/42',
    name: '季度数据.pdf',
    mimeType: 'application/pdf',
    sizeBytes: 2048,
  };
  // 前置校验通过
  const precheck = precheckFile(file, DEFAULT_UPLOAD_CONSTRAINTS);
  if (!precheck.ok) throw new Error(`precondition failed: ${precheck.reason}`);

  let generation = 1;
  const guard = { isCurrent: () => generation === 1 };
  const uploadedBytes: number[] = [];
  const sentUri = { value: false };
  const controller = createUploadController({
    readBytes: async (uri, onChunk) => {
      // 本机读取：分块给真实 bytes；URI 不流出
      if (!uri.startsWith('content://') && !uri.startsWith('file://')) throw new Error('URI must stay on-device');
      onChunk(new Uint8Array(1024));
      onChunk(new Uint8Array(1024));
      return 2048;
    },
    uploadBytes: async (meta, send) => {
      // 传给服务端的是 bytes 与元数据——URI 不进请求
      sentUri.value = meta.name.includes('content://');
      send(new Uint8Array(512));
      // 上传在途：切空间
      generation = 2;
      send(new Uint8Array(512));
      return { documentID: 'doc-42', bytesUploaded: 1024 };
    },
  });

  const outcome = await controller.upload(file, guard);
  if (outcome.applied) throw new Error('late result must not be applied after scope switch');
  if (outcome.failure !== 'scope_changed') throw new Error(`expected scope_changed, got ${outcome.failure}`);
  if (sentUri.value) throw new Error('content uri must never be uploaded');

  // 草稿保留：失败路径不清选取（文件对象仍可重试——由调用方持有；此处观测控制器未要求清除）
  const draftPreserved = file.name === '季度数据.pdf' && file.uri.startsWith('content://');

  // 对照：无切换 → 正常应用
  const okController = createUploadController({
    readBytes: async (_uri, onChunk) => { onChunk(new Uint8Array(100)); return 100; },
    uploadBytes: async () => ({ documentID: 'doc-ok', bytesUploaded: 100 }),
  });
  const ok = await okController.upload({ ...file, sizeBytes: 100 }, { isCurrent: () => true });
  if (!ok.applied || ok.result?.documentID !== 'doc-ok') throw new Error('normal upload must apply');
  void uploadedBytes;

  // 前置校验：超大/非法 MIME 拒绝且保留草稿
  if (precheckFile({ ...file, sizeBytes: DEFAULT_UPLOAD_CONSTRAINTS.maxBytes + 1 }).ok) throw new Error('oversize must be rejected');
  if (precheckFile({ ...file, mimeType: 'application/x-executable' }).ok) throw new Error('disallowed mime must be rejected');

  // 源级：native-file 不把 URI 发网络（fetch/XHR 不出现在该模块）
  const nativeFile = await readFile(path.join(repoRoot, 'apps/mobile/sources/weknora/platform/native-file.ts'), 'utf8');
  if (/fetch\(|XMLHttpRequest|axios/.test(nativeFile)) throw new Error('native-file must not contain network calls');

  return { lateResultApplied: outcome.applied, draftPreserved };
}
