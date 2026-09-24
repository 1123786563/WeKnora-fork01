import type { BlobFetchPort, SharePort } from '@weknora/mobile-core';

/** 免凭据字节抓取（签名链接兑现通道）：仅 http/https；非 2xx 以 {status, code} 形态拒绝（模块据此映射 GRANT_*）。 */
export function createFetchBlobAdapter(): BlobFetchPort {
  return {
    async fetch(url) {
      let parsed: URL;
      try { parsed = new URL(url); } catch { throw new Error(`blob url must be absolute: ${url}`); }
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('blob url must use http(s)');
      const response = await fetch(url);
      if (!response.ok) {
        let code = '';
        try { code = String(((await response.json()) as { code?: unknown }).code ?? ''); } catch { code = ''; }
        const error = new Error(`blob fetch failed: ${response.status}`) as Error & { status?: number; code?: string };
        error.status = response.status;
        error.code = code;
        throw error;
      }
      const bytes = new Uint8Array(await response.arrayBuffer());
      return { bytes, mime: response.headers.get('content-type') ?? 'application/octet-stream' };
    },
  };
}

/** 系统分享 native Adapter（module-seams §7.3 Preview / Share Port）。解析失败返回 undefined（fail closed）。 */
export function createNativeSharePortIfAvailable(): SharePort | undefined {
  try {
    const shareLike = (require('react-native') as { Share?: { share(input: { url?: string; message?: string }): Promise<unknown> } }).Share;
    if (shareLike === undefined) return undefined;
    return {
      async share({ url, name }) {
        await shareLike.share({ url, message: name });
      },
    };
  } catch {
    return undefined;
  }
}
