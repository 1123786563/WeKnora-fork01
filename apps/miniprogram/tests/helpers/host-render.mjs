// 渲染级测试的自定义 React 渲染器：用 react-reconciler 把真实页面组件挂载为
// 纯对象树（无 DOM 依赖，纯 Node 运行），并提供查找/事件模拟驱动面。
// react-reconciler 与 react 以 pnpm 依赖路径解析（@tarojs/react 的 peer 依赖，
// 与 app 的 react 18.3.1 同一 store 实例），无需新增 package.json 依赖。
import { createRequire } from 'node:module';
import { HOST } from './components-host.mjs';

const require = createRequire(import.meta.url);
function resolveThrough(anchor, specifier) {
  const anchorDir = require.resolve(`${anchor}/package.json`).replace(/\/package\.json$/, '');
  return require(require.resolve(specifier, { paths: [anchorDir] }));
}
const Reconciler = resolveThrough('@tarojs/react', 'react-reconciler');
const DefaultEventPriority = Reconciler.DefaultEventPriority ?? 32;

function createNode(type, props) {
  const tag = type === HOST ? String(props.host ?? 'host') : String(typeof type === 'object' ? type.toString() : type);
  const { host, ...rest } = props;
  void host;
  return { tag, props: rest, children: [], parent: null };
}function createTextNode(text) {
  return { text, parent: null };
}
function attach(parent, child) {
  if (child.parent) detach(child.parent, child);
  child.parent = parent;
  parent.children.push(child);
}
function detach(parent, child) {
  const index = parent.children.indexOf(child);
  if (index >= 0) parent.children.splice(index, 1);
  child.parent = null;
}

const reconciler = Reconciler({
  getRootHostContext: () => null,
  getChildHostContext: () => null,
  prepareForCommit: () => null,
  resetAfterCommit: () => {},
  createInstance: (type, props) => createNode(type, props),
  createTextInstance: text => createTextNode(text),
  appendInitialChild: (parent, child) => attach(parent, child),
  appendChild: (parent, child) => attach(parent, child),
  appendChildToContainer: (container, child) => attach(container.root, child),
  insertBefore: (parent, child, before) => {
    detach(parent, child);
    const index = parent.children.indexOf(before);
    parent.children.splice(index < 0 ? parent.children.length : index, 0, child);
    child.parent = parent;
  },
  removeChild: (parent, child) => detach(parent, child),
  removeChildFromContainer: (container, child) => detach(container.root, child),
  finalizeInitialChildren: () => false,
  prepareUpdate: () => true,
  commitUpdate: (instance, payload, type, prevProps, nextProps) => {
    const { host, ...rest } = nextProps;
    void host;
    instance.props = rest;
  },
  commitTextUpdate: (instance, _oldText, newText) => { instance.text = newText; },
  clearContainer: container => { container.root.children.length = 0; },
  shouldSetTextContent: () => false,
  getPublicInstance: instance => instance,
  preparePortalMount: () => {},
  detachDeletedInstance: () => {},
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  noTimeout: -1,
  supportsMutation: true,
  supportsPersistence: false,
  supportsHydration: false,
  isPrimaryRenderer: true,
  getCurrentEventPriority: () => DefaultEventPriority,
  beforeActiveInstanceBlur: () => {},
  afterActiveInstanceBlur: () => {},
  prepareScopeUpdate: () => {},
  getInstanceFromScope: () => null,
  getInstanceFromNode: () => null,
});

/** 等待 reconciler 调度、网络微任务链与 passive effects 全部落定。 */
async function settle(renderApi) {
  for (let round = 0; round < 4; round++) {
    await new Promise(resolve => setTimeout(resolve, 0));
    reconciler.flushSync(() => {});
  }
  reconciler.flushPassiveEffects?.();
  void renderApi;
}

function isHost(node) { return node && node.tag !== undefined; }
function* walk(node) {
  if (!node) return;
  yield node;
  if (node.children) for (const child of node.children) yield* walk(child);
}
/** 深度优先拼接后代文本（跳过被样式隐藏等无关概念——纯对象树不存在）。 */
function textOf(node) {
  let out = '';
  for (const current of walk(node)) if (typeof current.text === 'string') out += current.text;
  return out;
}
function findAll(node, predicate) {
  return [...walk(node)].filter(item => isHost(item) && predicate(item));
}
function textNodes(node, expected) {
  return [...walk(node)].filter(item => typeof item.text === 'string' && (expected === undefined || item.text === expected));
}

/** 向上找最近满足条件的祖先节点。 */
function closest(node, predicate) {
  let current = node?.parent;
  while (current) { if (predicate(current)) return current; current = current.parent; }
  return null;
}
/** 以 label 文本定位 Field 内的输入控件（Field = Text.wk-label + Input/Textarea）。 */
function fieldInput(root, label) {
  const labelNode = textNodes(root, label)[0];
  const field = labelNode && closest(labelNode, node => typeof node.props.className === 'string' && node.props.className.includes('wk-field'));
  if (!field) throw new Error(`field not found: ${label}`);
  const inputs = findAll(field, node => ['input', 'textarea'].includes(node.tag));
  if (!inputs.length) throw new Error(`no input inside field: ${label}`);
  return inputs[0];
}

function weappEvent(detail = {}) {
  return { detail, timeStamp: Date.now(), stopPropagation() {}, preventDefault() {} };
}

/** 挂载真实页面组件并返回查询/模拟驱动面；务必在用例结束时调用 unmount。 */
export async function render(element) {
  const containerRoot = { tag: 'root', props: {}, children: [], parent: null };
  const container = { root: containerRoot };
  const fiberRoot = reconciler.createContainer(
    container, 0 /* ConcurrentRoot */, false, null, false, null, 'render-test',
    () => {}, null,
  );
  const api = {
    root: containerRoot,
    unmount() { reconciler.updateContainer(null, fiberRoot, null, null); },
    async rerender(next) { reconciler.updateContainer(next, fiberRoot, null, null); await settle(); },
    text: () => textOf(containerRoot),
    textOf,
    findAll: predicate => findAll(containerRoot, predicate),
    byTag: tag => findAll(containerRoot, node => node.tag === tag),
    byClassName: className => findAll(containerRoot, node => typeof node.props.className === 'string' && node.props.className.split(/\s+/).includes(className)),
    /** 按“可点击元素文本”定位：遍历所有同名文本节点，取最近的带 onClick 祖先。 */
    clickable(expected) {
      for (const node of textNodes(containerRoot, expected)) {
        const target = closest(node, candidate => typeof candidate.props.onClick === 'function');
        if (target) return target;
      }
      throw new Error(`clickable not found: ${expected}`);
    },
    click(expected) { api.clickable(expected).props.onClick(weappEvent()); },
    /** 在 label 对应的输入框中输入文本（驱动 onInput → e.detail.value 契约）。 */
    async type(label, value) {
      const input = fieldInput(containerRoot, label);
      input.props.onInput(weappEvent({ value }));
      await settle();
    },
    async pressCheckboxGroup(value) {
      const group = api.byTag('checkbox-group')[0];
      if (!group) throw new Error('checkbox-group not found');
      group.props.onChange(weappEvent({ value }));
      await settle();
    },
    /** 含指定文本（子串匹配）的节点是否存在。 */
    hasText: (expected, scope) => textOf(scope ?? containerRoot).includes(expected),
    textNodes,
    settle: () => settle(),
  };
  reconciler.updateContainer(element, fiberRoot, null, null);
  await settle();
  return api;
}
