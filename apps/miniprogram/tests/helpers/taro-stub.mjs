// 微信/Taro 平台边界测试替身：只实现 src/platform 与 src/services 实际消费的
// 契约（request/uploadFile/downloadFile 的 success/fail 回调 + RequestTask 生命周期、同步存储、
// 文件查看通道）。通过 tests/*.test.mjs 里的 module.registerHooks 把 '@tarojs/taro' 重定向到本文件，
// 使真实源码（transport/storage/runtime/workbench）在 Node 中原样装配。
const state = {
  storage: new Map(),
  calls: [],
  handler: null,
};

function makeTask(call) {
  const listeners = { headers: new Set(), chunk: new Set(), progress: new Set() };
  let aborted = false;
  const task = {
    _listeners: listeners,
    abort() {
      if (aborted) return;
      aborted = true;
      // 真实 RequestTask abort 后以 fail 回调收尾；由测试 handler 决定是否模拟。
      if (typeof call.onAborted === 'function') call.onAborted(task);
    },
    onHeadersReceived(fn) { listeners.headers.add(fn); },
    offHeadersReceived(fn) { listeners.headers.delete(fn); },
    onChunkReceived(fn) { listeners.chunk.add(fn); },
    offChunkReceived(fn) { listeners.chunk.delete(fn); },
    onProgressUpdate(fn) { listeners.progress.add(fn); },
    offProgressUpdate(fn) { listeners.progress.delete(fn); },
  };
  return task;
}

function dispatch(kind, options) {
  const call = { kind, options, onAborted: null, task: null };
  state.calls.push(call);
  const task = makeTask(call);
  call.task = task;
  if (!state.handler) {
    queueMicrotask(() => options.fail({ errMsg: `stub: no handler installed for ${kind} ${options.url}` }));
    return task;
  }
  // 真实网络回调总是异步到达；同步派发会早于 onChunkReceived 注册，烧掉流式监听。
  queueMicrotask(() => state.handler(call, task));
  return task;
}

const Taro = {
  request(options) { return dispatch('request', options); },
  uploadFile(options) { return dispatch('uploadFile', options); },
  downloadFile(options) { return dispatch('downloadFile', options); },
  // wx.login 契约替身：静默登录链路（services/runtime 的 wxCode 薄适配）固定换回 CODE。
  login() { return Promise.resolve({ code: 'CODE' }); },
  getStorageSync(key) { return state.storage.has(key) ? state.storage.get(key) : ''; },
  setStorageSync(key, value) { state.storage.set(key, value); },
  removeStorageSync(key) { state.storage.delete(key); },
  getStorageInfoSync() { return { keys: [...state.storage.keys()] }; },
  // 文件查看通道（platform/files.ts，Promise 风格调用）：小文件、可打开、临时文件可清理。
  getFileInfo(options) {
    return new Promise(resolve => queueMicrotask(() => { const result = { size: 5 }; options.success?.(result); resolve(result); }));
  },
  openDocument(options) {
    return new Promise(resolve => queueMicrotask(() => { options.success?.({}); resolve({}); }));
  },
  getFileSystemManager() { return { unlinkSync() { /* temporary file cleanup is a no-op in the stub */ } }; },
};

// ---- 测试驱动面 ----
function reset() {
  state.storage.clear();
  state.calls.length = 0;
  state.handler = null;
}
function lastCall(kind) {
  const found = [...state.calls].reverse().find(call => call.kind === (kind ?? call.kind));
  if (!found) throw new Error('stub: no recorded native call');
  return found;
}
/** 以 success 回调收尾（微信契约：statusCode/header/data）。 */
function succeed(call, result) {
  call.options.success({ statusCode: 200, header: {}, ...result });
}
function fail(call, errMsg) {
  call.options.fail({ errMsg });
}
function emitHeaders(call, header, statusCode) {
  for (const fn of call.task?._listeners.headers ?? []) fn({ header, ...(statusCode === undefined ? {} : { statusCode }) });
}
function emitChunk(call, bytes) {
  const data = bytes instanceof ArrayBuffer ? bytes : new TextEncoder().encode(bytes).buffer;
  for (const fn of call.task?._listeners.chunk ?? []) fn({ data });
}
function paths() {
  return state.calls.map(call => `${call.options.method ?? ''} ${new URL(call.options.url).pathname}`);
}

export default Taro;
// dispatch 同时挂到 stub 对象与命名导出：测试驱动面经 stub.dispatch 调用（platform-adapters.test.mjs）。
export const stub = { reset, lastCall, succeed, fail, emitHeaders, emitChunk, paths, state, dispatch,
  /** 安装网络 handler：handler(call, task) 必须自行调用 succeed/fail 或 emit*。 */
  use(fn) { state.handler = fn; } };
export { dispatch };
