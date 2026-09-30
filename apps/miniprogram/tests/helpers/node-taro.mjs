// 真实网络版 Taro 契约替身：与 taro-stub.mjs 相同的模块契约（request/uploadFile/storage），
// 但 request 底座是 Node fetch——组合根以上（Runtime/深模块/remote Adapter/传输层）100% 真实源码，
// 只有最外层平台调用被替换为等价的 fetch 语义。SSE（enableChunked）不支持：真实集成证据不覆盖流式面。
const storage = new Map();
async function request(options) {
  const controller = new AbortController();
  const stop = () => controller.abort();
  options.signal?.addEventListener?.('abort', stop, { once: true });
  try {
    const hasBody = options.data !== undefined && options.method !== 'GET';
    const response = await fetch(options.url, {
      method: options.method,
      headers: options.header,
      ...(hasBody ? { body: typeof options.data === 'string' ? options.data : JSON.stringify(options.data) } : {}),
      ...(options.signal ? { signal: controller.signal } : {}),
    });
    const text = await response.text();
    options.success({ statusCode: response.status, header: Object.fromEntries(response.headers), data: text });
  } catch (error) {
    options.fail({ errMsg: `node-taro: ${error?.message ?? 'network error'}` });
  } finally {
    options.signal?.removeEventListener?.('abort', stop);
  }
  return { abort() { controller.abort(); }, onHeadersReceived() {}, offHeadersReceived() {}, onChunkReceived() {}, offChunkReceived() {} };
}
export const Taro = {
  request,
  uploadFile(options) { options.fail({ errMsg: 'node-taro: uploadFile not supported in integration mode' }); return { abort() {} }; },
  getStorageSync(key) { return storage.has(key) ? storage.get(key) : ''; },
  setStorageSync(key, value) { storage.set(key, value); },
  removeStorageSync(key) { storage.delete(key); },
  getStorageInfoSync() { return { keys: [...storage.keys()] }; },
};
// 与 taro-stub.mjs 相同的模块契约：runtime.ts 以 default import 消费 '@tarojs/taro'。
export default Taro;
export const nodeStorage = storage;
