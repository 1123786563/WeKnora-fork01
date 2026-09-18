/**
 * 原生文件选取与上传校验（MX-025 / M05 附件通道）。
 * 冻结规则：
 * - 文件 URI 只用于**本机读取**（content:// 或 file://）；上传的是真实 bytes，
 *   不是 URI 字符串；服务端校验大小/MIME/归属/扫描结果——客户端只做前置校验不越权放行；
 * - 上传全程绑定 scope generation：切空间时在途结果丢弃（lateResultApplied=false）、
 *   任务草稿（含附件引用）原样保留（draftPreserved=true），不做任何自动补发；
 * - 上传前本地校验：大小上限、MIME 白名单；失败保留草稿与选取。
 */

export interface PickedFile {
  /** 本机内容 URI（仅本机读取用；不进入服务端请求体） */
  uri: string;
  name: string;
  mimeType: string;
  sizeBytes: number;
}

export interface UploadConstraints {
  maxBytes: number;
  allowedMimePrefixes: readonly string[];
}

export const DEFAULT_UPLOAD_CONSTRAINTS: UploadConstraints = {
  maxBytes: 50 * 1024 * 1024,
  allowedMimePrefixes: ['image/', 'text/', 'application/pdf', 'application/vnd.openxmlformats-officedocument', 'application/json'],
};

export type PrecheckFailure = 'too_large' | 'mime_not_allowed' | 'empty_file';

/** 本地前置校验（服务端仍会做权威校验——这里不越权放行任何服务端规则）。 */
export function precheckFile(file: PickedFile, constraints: UploadConstraints = DEFAULT_UPLOAD_CONSTRAINTS): { ok: true } | { ok: false; reason: PrecheckFailure } {
  if (file.sizeBytes <= 0) return { ok: false, reason: 'empty_file' };
  if (file.sizeBytes > constraints.maxBytes) return { ok: false, reason: 'too_large' };
  const mimeAllowed = constraints.allowedMimePrefixes.some((prefix) => file.mimeType.startsWith(prefix));
  if (!mimeAllowed) return { ok: false, reason: 'mime_not_allowed' };
  return { ok: true };
}

export interface UploadResult {
  documentID: string;
  bytesUploaded: number;
}

export interface UploadPorts {
  /** 读取本机 URI 的真实字节（分段；不把 URI 发给服务器）。 */
  readBytes(uri: string, onChunk: (chunk: Uint8Array) => void): Promise<number>;
  /** 上传真实 bytes 到知识/附件通道（服务端权威校验）。 */
  uploadBytes(meta: { name: string; mimeType: string; sizeBytes: number }, provideChunks: (send: (chunk: Uint8Array) => void) => void): Promise<UploadResult>;
}

export interface UploadGuard {
  /** scope generation 守卫：切空间后 false（结果丢弃、草稿保留）。 */
  isCurrent(): boolean;
}

export function createUploadController(ports: UploadPorts) {
  return {
    /**
     * 上传：前置校验→本机读 bytes→分块上传。期间 scope 失效：
     * - 已完成的结果**不应用**（lateResultApplied=false）；
     * - 异常/中断不删除本地草稿（draftPreserved=true）。
     */
    async upload(file: PickedFile, guard: UploadGuard, constraints: UploadConstraints = DEFAULT_UPLOAD_CONSTRAINTS): Promise<{ applied: boolean; result?: UploadResult; failure?: PrecheckFailure | 'scope_changed' }> {
      const precheck = precheckFile(file, constraints);
      if (!precheck.ok) return { applied: false, failure: precheck.reason };
      let buffered: Uint8Array[] = [];
      const totalRead = await ports.readBytes(file.uri, (chunk) => {
        buffered.push(chunk);
      });
      if (!guard.isCurrent()) {
        return { applied: false, failure: 'scope_changed' }; // 草稿保留（不抛错、不清理）
      }
      if (totalRead <= 0) return { applied: false, failure: 'empty_file' };
      try {
        const result = await ports.uploadBytes(
          { name: file.name, mimeType: file.mimeType, sizeBytes: totalRead },
          (send) => {
            for (const chunk of buffered) send(chunk);
            buffered = [];
          },
        );
        if (!guard.isCurrent()) {
          return { applied: false, failure: 'scope_changed' }; // 迟到结果不应用（服务端或已清理）
        }
        return { applied: true, result };
      } catch {
        return { applied: false, failure: 'scope_changed' }; // 网络失败：草稿保留（离线不自动补发）
      }
    },
  };
}
