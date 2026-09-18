// M14 成果下载/分享服务（RW-022）：签名链接 → 真实 bytes → 本地缓存文件。
// IO 注入式设计：状态机可单测（401 过期重授权闭环、501 如实、成功写文件），
// 平台装配（fetch 无凭证 + expo-file-system）在 AppProvider 注入。

export type ArtifactDownloadOutcome =
  | { kind: "downloaded"; localUri: string }
  | { kind: "signing_disabled" } // 部署未配置签名密钥（501）——如实提示，不假装
  | { kind: "grant_expired" } // 重签后仍 401：需要用户重新发起
  | { kind: "forbidden" }
  | { kind: "not_found" }
  | { kind: "network"; message: string };

/** 签名链接签发错误（对齐 ApiError.kind + 后端 501 语义） */
export class SigningDisabledError extends Error {
  readonly kind = "signing_disabled";
}

export interface ArtifactDownloadIO {
  /** 请求新的签名链接；部署未启用时抛 SigningDisabledError */
  requestSignedUrl(): Promise<string>;
  /** 无凭证 fetch 签名 URL；返回 HTTP status 与（200 时的）bytes */
  fetchBytes(url: string): Promise<{ status: number; bytes?: Uint8Array }>;
  /** 写入本地缓存，返回 file URI */
  writeFile(fileName: string, bytes: Uint8Array): Promise<string>;
}

const fileNameSafe = (name: string): string => name.replace(/[^\w.\-]+/g, "_").slice(-80) || "artifact";

/**
 * 下载一个成果：签名 → fetch bytes → 落盘。
 * 签名过期（fetch 401）时自动重签一次——重授权闭环；重签后仍失败按 grant_expired 如实返回。
 * 决不把长期凭证放进下载 URL；下载请求本身不携带 Authorization。
 */
export async function downloadArtifact(
  io: ArtifactDownloadIO,
  fileName: string,
): Promise<ArtifactDownloadOutcome> {
  let url: string;
  try {
    url = await io.requestSignedUrl();
  } catch (e) {
    return classifySignError(e);
  }

  // 首次下载 + 过期重授权各一次，共至多两轮
  for (let attempt = 0; attempt < 2; attempt++) {
    let res: { status: number; bytes?: Uint8Array };
    try {
      res = await io.fetchBytes(url);
    } catch (e) {
      return { kind: "network", message: (e as Error).message };
    }
    if (res.status === 200 && res.bytes) {
      try {
        const localUri = await io.writeFile(fileNameSafe(fileName), res.bytes);
        return { kind: "downloaded", localUri };
      } catch (e) {
        return { kind: "network", message: `写入本地文件失败：${(e as Error).message}` };
      }
    }
    if (res.status === 401) {
      if (attempt === 0) {
        // 签名在签发与下载之间过期：重新授权一次（不换目标、不换 ID 语义）
        try {
          url = await io.requestSignedUrl();
          continue;
        } catch (e) {
          return classifySignError(e);
        }
      }
      return { kind: "grant_expired" };
    }
    if (res.status === 403) return { kind: "forbidden" };
    if (res.status === 404) return { kind: "not_found" };
    return { kind: "network", message: `下载失败（HTTP ${res.status}）` };
  }
  return { kind: "grant_expired" };
}

function classifySignError(e: unknown): ArtifactDownloadOutcome {
  const err = e as { kind?: string; code?: string };
  if (e instanceof SigningDisabledError || err?.code === "artifact_signing_disabled" || err?.kind === "signing_disabled") {
    return { kind: "signing_disabled" };
  }
  if (err?.kind === "not_implemented") return { kind: "signing_disabled" };
  if (err?.kind === "forbidden") return { kind: "forbidden" };
  if (err?.kind === "not_found") return { kind: "not_found" };
  return { kind: "network", message: (e as Error).message || "签名链接获取失败" };
}
