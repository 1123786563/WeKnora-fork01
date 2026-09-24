import type { BlobFetchPort, SharePort } from '@weknora/mobile-core';

/** 免凭据抓取超时（B3-F10）：CDN 挂起不得让材料页 loading 永久挂起。 */
const BLOB_FETCH_TIMEOUT_MS = 30_000;

/** 免凭据字节抓取（签名链接兑现通道）：仅 http/https；非 2xx 以 {status, code} 形态拒绝（模块据此映射 GRANT_*）。 */
export function createFetchBlobAdapter(): BlobFetchPort {
  return {
    async fetch(url) {
      let parsed: URL;
      try { parsed = new URL(url); } catch { throw new Error(`blob url must be absolute: ${url}`); }
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('blob url must use http(s)');
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), BLOB_FETCH_TIMEOUT_MS);
      let response: Response;
      try {
        response = await fetch(url, { signal: controller.signal });
      } finally {
        clearTimeout(timer);
      }
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

/** 原生平台探测（device-identity.ts:40-50 同型惰性 require 先例）。 */
function nativeSharePlatformOs(): 'ios' | 'android' {
  try {
    const reactNative = require('react-native') as { Platform?: { OS?: string } };
    return reactNative.Platform?.OS === 'ios' ? 'ios' : 'android';
  } catch {
    return 'android';
  }
}

/** 系统分享 native Adapter（module-seams §7.3 Preview / Share Port）。解析失败返回 undefined（fail closed）。 */
export function createNativeSharePortIfAvailable(): SharePort | undefined {
  try {
    const shareLike = (require('react-native') as { Share?: { share(input: { url?: string; message?: string }): Promise<unknown> } }).Share;
    if (shareLike === undefined) return undefined;
    return {
      async share({ url, name }) {
        // RN 的 Share.share 仅 iOS 消费 url；Android 的 ACTION_SEND 只取 message——
        // 把 url 并入 message，签名链接在 Android 不被静默丢弃（B3-F57）。
        if (nativeSharePlatformOs() === 'android') {
          await shareLike.share({ message: `${name} ${url}` });
          return;
        }
        await shareLike.share({ url, message: name });
      },
    };
  } catch {
    return undefined;
  }
}
