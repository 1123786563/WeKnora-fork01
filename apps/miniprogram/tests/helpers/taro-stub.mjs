// 微信/Taro 平台边界测试替身：只实现 src/platform 与 src/services 实际消费的
// 契约（request/uploadFile 的 success/fail 回调 + RequestTask 生命周期、同步存储），
// 以及页面渲染测试所需的窗口/导航/模态/文件选择契约。
// 通过 tests/*.test.mjs 里的 module.registerHooks 把 '@tarojs/taro' 重定向到本文件，
// 使真实源码（transport/storage/runtime/workbench/pages）在 Node 中原样装配。
import { useEffect, useRef } from 'react';

const state = {
  storage: new Map(),
  calls: [],
  handler: null,
  // 页面渲染替身状态：路由栈/路由参数/窗口几何/模态结果/文件选择。
  navigation: [],
  pageStack: [{ route: 'pages/home/index' }],
  routerParams: {},
  window: { statusBarHeight: 20, windowWidth: 390, windowHeight: 844 },
  capsule: { top: 48, left: 281, height: 32 },
  modal: { confirm: true },
  chosenFile: null,
  download: { statusCode: 200, tempFilePath: '/tmp/wk-stub-file' },
  fileInfo: { size: 1024 },
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
  getStorageSync(key) { return state.storage.has(key) ? state.storage.get(key) : ''; },
  setStorageSync(key, value) { state.storage.set(key, value); },
  removeStorageSync(key) { state.storage.delete(key); },
  getStorageInfoSync() { return { keys: [...state.storage.keys()] }; },
  getWindowInfo() { return { ...state.window }; },
  getMenuButtonBoundingClientRect() { return { ...state.capsule }; },
  getCurrentPages() { return state.pageStack.map(page => ({ ...page })); },
  getCurrentInstance() { return { router: { params: { ...state.routerParams } } }; },
  navigateTo(options) { return nav('navigateTo', options); },
  switchTab(options) { return nav('switchTab', options); },
  navigateBack(options) { return nav('navigateBack', options); },
  reLaunch(options) { return nav('reLaunch', options); },
  showModal(options) {
    // confirmAction 只消费 confirm 布尔；测试通过 stub.modal.confirm 驱动两个分支。
    return Promise.resolve({ confirm: state.modal.confirm });
  },
  showToast(options) { state.navigation.push({ kind: 'showToast', title: options?.title }); return Promise.resolve(); },
  chooseMessageFile(options) {
    return new Promise((resolve, reject) => {
      if (!state.chosenFile) { reject(new Error('未选择文件')); return; }
      resolve({ tempFiles: [state.chosenFile] });
    });
  },
  downloadFile(options) {
    state.calls.push({ kind: 'download', options, onAborted: null, task: null });
    const fake = { abort() {}, onProgressUpdate() {}, offProgressUpdate() {} };
    queueMicrotask(() => {
      const outcome = state.download;
      if (typeof outcome === 'function') { outcome(options); return; }
      if (outcome && outcome.errMsg) { options.fail?.(outcome); return; }
      options.success?.({ statusCode: outcome?.statusCode ?? 200, tempFilePath: outcome?.tempFilePath ?? '/tmp/wk-stub-file' });
    });
    return fake;
  },
  getFileInfo() { return Promise.resolve({ ...state.fileInfo }); },
  openDocument() { return Promise.resolve(); },
  getFileSystemManager() { return { unlinkSync() {} }; },
};

// ---- 页面生命周期（useDidShow / useDidHide 契约替身）----
// 记录订阅时的最新回调；测试用 stub.pageShown()/stub.pageHidden() 派发。
// 与真实 Taro 的差异：挂载时不自动触发一次 show（初始加载由 useEffect 承担）。
const lifecycle = { show: new Set(), hide: new Set() };
export function useDidShow(callback) {
  const latest = useRef(callback); latest.current = callback;
  useEffect(() => {
    const handler = () => latest.current();
    lifecycle.show.add(handler);
    return () => lifecycle.show.delete(handler);
  }, []);
}
export function useDidHide(callback) {
  const latest = useRef(callback); latest.current = callback;
  useEffect(() => {
    const handler = () => latest.current();
    lifecycle.hide.add(handler);
    return () => lifecycle.hide.delete(handler);
  }, []);
}

function nav(kind, options) {
  const entry = { kind, url: options?.url };
  state.navigation.push(entry);
  if (kind === 'navigateTo' && options?.url) state.pageStack.push({ route: options.url.split('?')[0].replace(/^\//, '') });
  else if (kind === 'switchTab' && options?.url) state.pageStack = [{ route: options.url.split('?')[0].replace(/^\//, '') }];
  else if (kind === 'navigateBack') state.pageStack.pop();
  else if (kind === 'reLaunch' && options?.url) state.pageStack = [{ route: options.url.split('?')[0].replace(/^\//, '') }];
  return new Promise(resolve => options?.success ? options.success({}) : resolve({}));
}

// ---- 测试驱动面 ----
function reset() {
  state.storage.clear();
  state.calls.length = 0;
  state.handler = null;
  state.navigation.length = 0;
  state.pageStack = [{ route: 'pages/home/index' }];
  state.routerParams = {};
  state.window = { statusBarHeight: 20, windowWidth: 390, windowHeight: 844 };
  state.capsule = { top: 48, left: 281, height: 32 };
  state.modal = { confirm: true };
  state.chosenFile = null;
  state.download = { statusCode: 200, tempFilePath: '/tmp/wk-stub-file' };
  state.fileInfo = { size: 1024 };
  lifecycle.show.clear();
  lifecycle.hide.clear();
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
  use(fn) { state.handler = fn; },
  /** 派发页面 show/hide 生命周期（对应 useDidShow / useDidHide 订阅者）。 */
  pageShown() { for (const handler of [...lifecycle.show]) handler(); },
  pageHidden() { for (const handler of [...lifecycle.hide]) handler(); },
  /** 记录到的导航序列：[{kind:'navigateTo'|'switchTab'|'navigateBack'|'showToast', url?}]。 */
  navigations() { return state.navigation.map(entry => ({ ...entry })); } };
