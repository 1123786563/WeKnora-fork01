// 附件上传状态机（RW-027，详细设计 §8）：
// selected → uploading（multipart 真实 bytes）→ verifying（worker 解析轮询）→ ready / failed
// 规则：未 ready 不得提交任务；上传/解析失败不丢文字草稿；客户端类型检查只是体验，服务端仍权威。
import { ApiError } from "@/api/http";
import { decodeTemporaryDocument, type TemporaryDocumentWire } from "@/contracts/attachments";

export type AttachmentState = "selected" | "uploading" | "verifying" | "ready" | "failed";

export interface AttachmentItem {
  /** 本地选择标识（M05 chips 用） */
  localId: string;
  fileName: string;
  mimeType: string;
  size: number;
  state: AttachmentState;
  /** 服务端 document id（上传成功后） */
  documentId: string | null;
  error: string | null;
}

export interface PickedFile {
  uri: string;
  name: string;
  mimeType: string;
  size: number;
}

/** 构建 multipart FormData：RN 原生层读 uri 文件内容为真实 bytes（非把 file:// 当 URL 提交） */
export function buildUploadForm(file: PickedFile, agentId?: string): FormData {
  const fd = new FormData();
  fd.append("file", { uri: file.uri, name: file.name, type: file.mimeType } as unknown as Blob);
  if (agentId) fd.append("agent_id", agentId);
  return fd;
}

export interface AttachmentTransport {
  /** POST /sessions/:id/attachments（multipart file）→ 202 document */
  upload(sessionId: string, file: PickedFile, agentId?: string, signal?: AbortSignal): Promise<TemporaryDocumentWire>;
  /** GET /sessions/:id/attachments/:attachment_id */
  get(sessionId: string, attachmentId: string, signal?: AbortSignal): Promise<TemporaryDocumentWire>;
}

export interface AttachmentUploaderDeps {
  transport: AttachmentTransport;
  /** 轮询间隔（ms）；测试注入 0/微小值 */
  pollIntervalMs?: number;
  pollLimit?: number;
  /** 客户端类型白名单（体验层；服务端权威校验） */
  allowedExtensions?: string[];
  maxBytes?: number;
}

const DEFAULT_ALLOWED = ["pdf", "doc", "docx", "txt", "md", "csv", "xls", "xlsx", "ppt", "pptx", "png", "jpg", "jpeg", "webp", "json"];

export class AttachmentUploader {
  private items = new Map<string, AttachmentItem>();

  constructor(private d: AttachmentUploaderDeps) {}

  get list(): AttachmentItem[] {
    return [...this.items.values()];
  }

  item(localId: string): AttachmentItem | undefined {
    return this.items.get(localId);
  }

  /** 全部附件 ready 才允许提交（详细设计：扫描前不能交给 Agent） */
  allReady(): boolean {
    return this.list.every((i) => i.state === "ready");
  }

  hasUploading(): boolean {
    return this.list.some((i) => i.state === "selected" || i.state === "uploading" || i.state === "verifying");
  }

  readyDocumentIds(): string[] {
    return this.list.filter((i) => i.state === "ready" && i.documentId).map((i) => i.documentId!);
  }

  remove(localId: string): void {
    this.items.delete(localId);
  }

  /** 选择文件 → selected；本地校验失败直接 failed（保留在列表供用户看到原因） */
  pick(file: PickedFile): AttachmentItem {
    const item: AttachmentItem = {
      localId: `att_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`,
      fileName: file.name,
      mimeType: file.mimeType || "application/octet-stream",
      size: file.size,
      state: "selected",
      documentId: null,
      error: null,
    };
    const ext = file.name.split(".").pop()?.toLowerCase() ?? "";
    const allowed = this.d.allowedExtensions ?? DEFAULT_ALLOWED;
    const maxBytes = this.d.maxBytes ?? 100 * 1024 * 1024;
    if (ext && !allowed.includes(ext)) {
      item.state = "failed";
      item.error = `不支持的文件类型 .${ext}`;
    } else if (file.size > maxBytes) {
      item.state = "failed";
      item.error = `文件超过大小限制（${Math.floor(maxBytes / 1024 / 1024)}MB）`;
    }
    this.items.set(item.localId, item);
    return item;
  }

  /** 上传 + 等待解析 ready；失败进入 failed 且不清除（用户可移除重选；文字草稿由页面保留） */
  async uploadAndWait(sessionId: string, localId: string, file: PickedFile, agentId?: string, signal?: AbortSignal): Promise<AttachmentItem> {
    const item = this.items.get(localId);
    if (!item) throw new Error("附件不存在");
    if (item.state === "failed") return item;

    item.state = "uploading";
    item.error = null;
    try {
      const doc = await this.d.transport.upload(sessionId, file, agentId, signal);
      item.documentId = doc.id;
      // 上传 200/202 只是受理；解析 worker 异步推进 → verifying
      item.state = "verifying";
      if (doc.status === "ready") {
        item.state = "ready";
        return item;
      }
      if (doc.status === "failed") {
        item.state = "failed";
        item.error = doc.error_message ?? "附件解析失败";
        return item;
      }
      await this.waitUntilSettled(sessionId, item, signal);
      return item;
    } catch (e) {
      item.state = "failed";
      item.error = e instanceof ApiError ? e.message : `上传失败：${(e as Error).message}`;
      return item;
    }
  }

  private async waitUntilSettled(sessionId: string, item: AttachmentItem, signal?: AbortSignal): Promise<void> {
    const interval = this.d.pollIntervalMs ?? 1500;
    const limit = this.d.pollLimit ?? 40;
    for (let i = 0; i < limit; i++) {
      if (signal?.aborted) return;
      if (!item.documentId) return;
      await sleep(interval);
      const doc = await this.d.transport.get(sessionId, item.documentId, signal);
      if (doc.status === "ready") {
        item.state = "ready";
        return;
      }
      if (doc.status === "failed" || doc.status === "expired") {
        item.state = "failed";
        item.error = doc.error_message ?? (doc.status === "expired" ? "附件已过期" : "附件解析失败");
        return;
      }
      // pending/processing/unknown → 继续等待（unknown 不当作失败）
    }
    item.state = "failed";
    item.error = "附件解析超时，请稍后重试";
  }
}

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));
