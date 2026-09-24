// 微信/Taro 平台边界测试替身：只实现 src/platform 与 src/services 实际消费的
// 契约（request/uploadFile 的 success/fail 回调 + RequestTask 生命周期、同步存储）。
// 通过 tests/*.test.mjs 里的 module.registerHooks 把 '@tarojs/taro' 重定向到本文件，
// 使真实源码（transport/storage/runtime/workbench）在 Node 中原样装配。
const state = {
  storage: new Map(),
  calls: [],
  openedDocuments: [],
  removedFiles: [],
  copies: [],
  fileContents: new Map(),
  fileReads: [],
  fileInfoSize: 128,
  fileInfoError: null,
  openDocumentError: null,
  unlinkError: null,
  handler: null,
};

// 微信开发者工具实测（task-6-live-validation D3）：downloadFile 落下的运行时临时文件
// （http://tmp/…、wxfile://tmp…、真机 /tmp/…）unlink/unlinkSync 一律 permission denied，
// 只有 USER_DATA_PATH 下的文件可写可删——替身必须复现该平台契约，防止单测再次掩盖 D3。
function isRuntimeTempPath(p) {
  return typeof p === 'string' && (p.startsWith('http://tmp/') || p.startsWith('wxfile://tmp') || p.startsWith('/tmp/') || p.startsWith('tmp/'));
}

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
  getFileInfo(options) {
    if (state.fileInfoError) return Promise.reject(state.fileInfoError);
    const content = state.fileContents.get(options.filePath);
    return Promise.resolve({ size: content === undefined ? state.fileInfoSize : Buffer.byteLength(content) });
  },
  openDocument(options) { if (state.openDocumentError) return Promise.reject(state.openDocumentError); state.openedDocuments.push(options); return Promise.resolve(); },
  getFileSystemManager() {
    return {
      readFileSync(path, encoding, position = 0, length) {
        state.fileReads.push({ filePath: path, encoding, position, length });
        const content = state.fileContents.get(path);
        if (content === undefined) throw new Error(`stub: missing file ${path}`);
        const bytes = Buffer.from(content, 'utf8').subarray(position, length === undefined ? undefined : position + length);
        return encoding === 'utf8' ? bytes.toString('utf8') : bytes.buffer;
      },
      unlinkSync(path) {
        if (isRuntimeTempPath(path)) {
          throw Object.assign(new Error(`unlinkSync:fail permission denied, open ${path}`), { errMsg: `unlinkSync:fail permission denied, open ${path}` });
        }
        state.removedFiles.push(path); state.fileContents.delete(path);
      },
      unlink({ filePath, success, fail }) {
        queueMicrotask(() => {
          if (isRuntimeTempPath(filePath)) { fail?.({ errMsg: `unlink:fail permission denied, open ${filePath}` }); return; }
          if (state.unlinkError) { fail?.(state.unlinkError); return; }
          state.removedFiles.push(filePath); state.fileContents.delete(filePath); success?.({});
        });
      },
      copyFile({ srcPath, destPath, success, fail }) {
        queueMicrotask(() => {
          state.copies.push({ srcPath, destPath });
          if (!state.fileContents.has(srcPath)) { fail?.({ errMsg: `copyFile:fail no such file or directory, open ${srcPath}` }); return; }
          state.fileContents.set(destPath, state.fileContents.get(srcPath));
          success?.({});
        });
      },
    };
  },
  // 注意：真实运行时的 Taro 对象没有 env 字段（@tarojs/api 不提供、weapp 插件也不复制
  // wx.env——实测 Taro.env.USER_DATA_PATH 抛 TypeError）。替身同样不提供，USER_DATA_PATH
  // 只经 wx 全局暴露，防止单测再次掩盖平台契约差异。
  getStorageSync(key) { return state.storage.has(key) ? state.storage.get(key) : ''; },
  setStorageSync(key, value) { state.storage.set(key, value); },
  removeStorageSync(key) { state.storage.delete(key); },
  getStorageInfoSync() { return { keys: [...state.storage.keys()] }; },
};
// weapp 运行时全局存在 wx 对象；USER_DATA_PATH 只能从这里取。
globalThis.wx = globalThis.wx ?? { env: { USER_DATA_PATH: 'wxfile://usr' } };

// ---- 测试驱动面 ----
function reset() {
  state.storage.clear();
  state.calls.length = 0;
  state.openedDocuments.length = 0;
  state.removedFiles.length = 0;
  state.copies.length = 0;
  state.fileContents.clear();
  state.fileReads.length = 0;
  state.fileInfoSize = 128;
  state.fileInfoError = null;
  state.openDocumentError = null;
  state.unlinkError = null;
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
export const stub = { reset, lastCall, succeed, fail, emitHeaders, emitChunk, paths, state,
  /** 安装网络 handler：handler(call, task) 必须自行调用 succeed/fail 或 emit*。 */
  use(fn) { state.handler = fn; } };
