import { enumOr, type Decoder } from "./wire";

// 临时文档（会话附件）wire —— 对齐 internal/types/temporary_document.go:24
// 上传：POST /sessions/:session_id/attachments（multipart 字段 file）→ 202 {success,data:{...}}
// 解析在 document worker 异步进行：上传完成 ≠ 可用，必须轮询到 ready。
export interface TemporaryDocumentWire {
  id: string;
  session_id: string;
  file_name: string;
  file_type: string;
  mime_type: string;
  file_size: number;
  status: TemporaryDocumentStatus;
  error_message: string | null;
}

export type TemporaryDocumentStatus =
  | "pending"
  | "processing"
  | "ready"
  | "failed"
  | "expired"
  | "unknown";

export const decodeTemporaryDocument: Decoder<TemporaryDocumentWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? r.document_id ?? ""),
    session_id: String(r.session_id ?? ""),
    file_name: String(r.file_name ?? r.filename ?? ""),
    file_type: String(r.file_type ?? ""),
    mime_type: String(r.mime_type ?? ""),
    file_size: Number(r.file_size ?? 0),
    status: decodeDocStatus(r.status),
    error_message: r.error_message == null || r.error_message === "" ? null : String(r.error_message),
  };
};

const decodeDocStatus = (v: unknown): TemporaryDocumentStatus => {
  const s = String(v ?? "").toLowerCase();
  // 服务端解析状态枚举（worker 推进）；未知值保留 unknown，不冒充 ready
  if (s === "pending" || s === "processing" || s === "parsing" || s === "uploading") return s === "parsing" || s === "uploading" ? "processing" : s;
  if (s === "ready" || s === "completed" || s === "active") return "ready";
  if (s === "failed" || s === "error") return "failed";
  if (s === "expired") return "expired";
  return "unknown";
};

export const decodeTemporaryDocumentList: Decoder<TemporaryDocumentWire[]> = (v) => {
  const list = Array.isArray(v)
    ? v
    : ((v as Record<string, unknown>)?.documents as unknown[]) ?? ((v as Record<string, unknown>)?.items as unknown[]) ?? [];
  return list.map((d) => decodeTemporaryDocument(d));
};

void enumOr;
