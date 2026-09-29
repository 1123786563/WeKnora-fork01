import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';

const hooks = createRequire(import.meta.url)('node:module') as typeof import('node:module') & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
/* tdesign 运行时依赖的 DOM 全局必须在模块加载前就位：_util/listener.js 的
 * 事件绑定函数是模块级 IIFE，导入时若 document.addEventListener 缺席会
 * 固化到 IE 时代的 attachEvent 分支（jsdom 无此 API）——pilot/kb-list 同款
 * 顶层 JSDOM + 全局补齐。 */
const harnessDom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/apps' });
Object.assign(globalThis, {
  React,
  IS_REACT_ACT_ENVIRONMENT: true,
  window: harnessDom.window,
  document: harnessDom.window.document,
  HTMLElement: harnessDom.window.HTMLElement,
  HTMLInputElement: harnessDom.window.HTMLInputElement,
  HTMLTextAreaElement: harnessDom.window.HTMLTextAreaElement,
  HTMLSelectElement: harnessDom.window.HTMLSelectElement,
  Element: harnessDom.window.Element,
  Node: harnessDom.window.Node,
  SVGElement: harnessDom.window.SVGElement,
  DocumentFragment: harnessDom.window.DocumentFragment,
  Event: harnessDom.window.Event,
  KeyboardEvent: harnessDom.window.KeyboardEvent,
  MouseEvent: harnessDom.window.MouseEvent,
  MutationObserver: harnessDom.window.MutationObserver,
  getComputedStyle: harnessDom.window.getComputedStyle?.bind(harnessDom.window),
  requestAnimationFrame: harnessDom.window.requestAnimationFrame?.bind(harnessDom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: harnessDom.window.cancelAnimationFrame?.bind(harnessDom.window) ?? clearTimeout,
});
/* APP-9 — 写操作反馈走 tdesign MessagePlugin 全局消息条（agent-editor.test
 * 同款 renderAdapter 注入：React 19 下命令式 API 需要 createRoot）。 */
{
  const { renderAdapter } = createRequire(import.meta.url)('tdesign-react/lib/_util/react-render.js') as { renderAdapter: (renderer: unknown) => void };
  const { createRoot } = createRequire(import.meta.url)('react-dom/client') as { createRoot: (container: Element) => unknown };
  renderAdapter?.(createRoot);
}
const { AppsPage } = await import('./AppsPages.tsx');

/* APP-1 — 两页字段表为 t-descriptions：label/content 均为 td 单元格。 */
function descriptionRows(container: HTMLElement): Array<{ label: string; content: HTMLElement }> {
  const labels = Array.from(container.querySelectorAll('.t-descriptions__label'));
  return labels.map((label) => {
    const row = label.closest('tr');
    const content = row?.querySelector('.t-descriptions__content') ?? null;
    return { label: (label.textContent ?? '').replace(/[:：]\s*$/, ''), content: content as HTMLElement };
  });
}
function descriptionContent(container: HTMLElement, label: string): HTMLElement | null {
  return descriptionRows(container).find((row) => row.label === label)?.content ?? null;
}

/* Vue ActionView parity: the approval card renders the FROZEN risk from the
   persisted snapshot (apps.risk.* label verbatim); an empty payload degrades
   to an honest em dash with the riskUnknownHint — never a guess. */
function renderActionPage(actionDto: unknown, props: Record<string, unknown> = {}): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  return (async () => {
    const dom = harnessDom;
    const { createRoot } = await import('react-dom/client');
    const container = dom.window.document.createElement('div');
    dom.window.document.body.appendChild(container);
    const root = createRoot(container);
    const client = { request: async () => actionDto } as never;
    await act(async () => {
      root.render(React.createElement(AppsPage, { client, mode: 'action', id: 'action-1', ...props }));
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
    return {
      container,
      cleanup: async () => {
        await act(async () => { root.unmount(); });
        document.body.replaceChildren();
      },
    };
  })();
}

async function renderCatalogPage(): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  const dom = harnessDom;
  const { createRoot } = await import('react-dom/client');
  const container = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(container);
  const root = createRoot(container);
  const client = { request: async () => [] } as never;
  await act(async () => {
    root.render(React.createElement(AppsPage, { client, mode: 'catalog' }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return {
    container,
    cleanup: async () => {
      await act(async () => { root.unmount(); });
      document.body.replaceChildren();
    },
  };
}

test('catalog tables use the Vue unboxed section layout and centered empty states', async () => {
  const { container, cleanup } = await renderCatalogPage();
  try {
    assert.equal(container.querySelectorAll('section.rounded-card').length, 0, 'catalog and installed tables are not wrapped in cards');
    assert.ok(container.querySelector('.apps-view__header button svg'), 'refresh control includes the Vue refresh icon (tdesign sprite use)');
    // tdesign t-table empty：双表各渲染一个 .t-table__empty 占位块。
    const emptyBlocks = Array.from(container.querySelectorAll('.t-table__empty'));
    assert.equal(emptyBlocks.length, 2, 'catalog + installed tables each render the t-table empty block');
    for (const block of emptyBlocks) {
      assert.ok(block.textContent, 'empty block carries its description text');
    }
  } finally {
    await cleanup();
  }
});

test('renders the frozen delete risk verbatim like the Vue ActionView', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'delete', state: 'awaiting_approval', content: '{}' } });
  try {
    const risk = descriptionContent(container, '风险');
    assert.ok(risk, 'renders the risk field (t-descriptions row)');
    assert.equal(risk.textContent, '删除');
  } finally {
    await cleanup();
  }
});

test('renders an unknown action risk as an em dash with the honest hint', async () => {
  const { container, cleanup } = await renderActionPage({ action: { state: 'awaiting_approval', content: '{}' } });
  try {
    const risk = descriptionContent(container, '风险');
    assert.ok(risk, 'renders the risk field (t-descriptions row)');
    // Vue ActionView: t-tooltip(hover 提示) > span.risk-missing「—」，不再用
    // title 属性承载提示文案。
    const missing = risk.querySelector('.action-view__risk-missing');
    assert.ok(missing, 'missing risk renders the .action-view__risk-missing em dash');
    assert.equal(missing.textContent, '—');
  } finally {
    await cleanup();
  }
});

test('APP-1: action page uses the Vue layout — h2 title, bordered descriptions table, outline refresh', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'awaiting_approval', content: '{}' } });
  try {
    const root = container.querySelector('.action-view');
    assert.ok(root, 'renders the .action-view root (Vue ActionView.vue)');
    assert.ok(root.querySelector('h2.action-view__title'), 'h2 title like Vue (not the legacy h1 frame)');
    assert.ok(root.querySelector('.action-view__header button svg'), 'refresh control carries the tdesign refresh icon');
    const refresh = root.querySelector('.action-view__header button') as HTMLElement;
    assert.ok(refresh.className.includes('t-button--variant-outline'), 'refresh button is outline (APP-3)');
    assert.ok(root.querySelector('.t-descriptions'), 'fields render as a t-descriptions table (APP-1)');
    for (const label of ['账号（连接）', '目标', '风险', '状态', '内容指纹', '版本围栏', '参数']) {
      assert.ok(descriptionContent(container, label), `descriptions row ${label} present`);
    }
  } finally {
    await cleanup();
  }
});

test('APP-8: an unknown action state renders the warning tag (结果待核对), not danger', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'unknown', content: '{}' } });
  try {
    const state = descriptionContent(container, '状态');
    assert.ok(state, 'state row present');
    const tag = state.querySelector('.t-tag');
    assert.ok(tag, 'state renders a t-tag');
    assert.ok((tag.className as string).includes('t-tag--warning'), `unknown maps to warning, got ${tag.className}`);
    assert.ok(container.textContent?.includes('结果待核对'));
  } finally {
    await cleanup();
  }
});

test('APP-1/2: authorization page failure state keeps the Vue layout and the 状态： label fallback', async () => {
  const dom = harnessDom;
  const { createRoot } = await import('react-dom/client');
  const mount = document.createElement('div');
  document.body.appendChild(mount);
  const root = createRoot(mount);
  const client = { request: async () => { throw Object.assign(new Error('not found'), { status: 404 }); } } as never;
  await act(async () => {
    root.render(React.createElement(AppsPage, { client, mode: 'authorization', id: 'attempt-404' }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
  try {
    const root2 = mount.querySelector('.authorization-view');
    assert.ok(root2, 'renders the .authorization-view root (Vue AuthorizationView.vue)');
    assert.ok(root2.querySelector('h2.authorization-view__title'), 'h2 title like Vue');
    const alerts = Array.from(root2.querySelectorAll('.t-alert'));
    assert.equal(alerts.length, 2, 'error alert + info guidance alert (t-alert, not wk-status text)');
    // APP-2 — 空状态值回退为「状态：」（Vue stateOther 空插值），非「—」。
    const status = descriptionContent(mount, '状态');
    assert.ok(status, 'status row present');
    assert.equal(status.textContent, '状态：');
    // APP-4 — 返回按钮带 arrow-left 图标。
    const back = Array.from(root2.querySelectorAll('button')).find((element) => element.textContent === '返回连接列表');
    assert.ok(back, 'back button present');
    assert.ok(back.querySelector('svg') ?? back.querySelector('i'), 'back button carries the arrow-left icon');
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }
});

test('approve controls follow actionControls: awaiting_approval + permission shows 批准', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'awaiting_approval', content: '{"a":1}' } }, { role: 'owner' });
  try {
    const approve = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '批准');
    assert.ok(approve, 'approve button present for awaiting_approval');
    const execute = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '执行');
    assert.equal(execute, undefined, 'no execute button before approval');
  } finally {
    await cleanup();
  }
});

test('a non-admin viewer sees the member hint and no approval controls', async () => {
  const { container, cleanup } = await renderActionPage({ action: { risk: 'read', state: 'awaiting_approval', content: '{}' } });
  try {
    assert.equal(Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '批准'), undefined);
    assert.match(container.textContent ?? '', /当前角色无法审批或执行动作/);
  } finally {
    await cleanup();
  }
});

/* APP-6/7 — ConnectionsView.vue 行级语义：操作列空态渲染空单元格（非「—」）；
 * 发起授权不互斥断开按钮（startAuthorization 不触碰 revokingId）。 */
async function renderConnectionsPage(options: {
  rows: Array<Record<string, unknown>>;
  onPost?: () => Promise<unknown>;
}): Promise<{ container: HTMLElement; cleanup: () => Promise<void> }> {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const { createRoot } = await import('react-dom/client');
  const root = createRoot(container);
  const client = {
    request: async (input: { method: string; path: string }) => {
      if (input.method === 'GET') return { data: options.rows };
      return options.onPost ? options.onPost() : {};
    },
  } as never;
  await act(async () => {
    root.render(React.createElement(AppsPage, { client, mode: 'connections', role: 'owner' }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await act(async () => { await new Promise((resolve) => setImmediate(resolve)); });
  return {
    container,
    cleanup: async () => {
      await act(async () => { root.unmount(); });
      document.body.replaceChildren();
    },
  };
}

test('APP-6: a non-active non-revoked connection row renders an empty ops cell, not a — placeholder', async () => {
  const { container, cleanup } = await renderConnectionsPage({ rows: [{ id: 'pending-1', kind: 'personal', state: 'pending' }] });
  try {
    const ops = container.querySelector('.connections-view__ops');
    assert.ok(ops, 'ops cell renders for the pending row');
    assert.equal(ops.textContent, '', 'ops cell stays empty like the Vue #ops template (no — placeholder)');
  } finally {
    await cleanup();
  }
});

test('APP-7: starting an authorization attempt does not disable the revoke buttons', async () => {
  let releasePost: (() => void) | null = null;
  const postGate = new Promise<unknown>((resolve) => { releasePost = () => resolve({ attempt_id: 'att-new' }); });
  const { container, cleanup } = await renderConnectionsPage({
    rows: [{ id: 'conn-1', kind: 'personal', state: 'active', owner_id: 'user-1', auth_version: 3 }],
    onPost: () => postGate,
  });
  try {
    const start = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '授权');
    assert.ok(start, 'start-authorization button (「授权」) present for the active row (owner)');
    const revoke = Array.from(container.querySelectorAll('button')).find((element) => element.textContent === '断开');
    assert.ok(revoke, 'revoke button present');
    assert.equal((revoke as HTMLButtonElement).disabled, false, 'revoke enabled before the start flight');
    await act(async () => { (start as HTMLButtonElement).click(); await new Promise((resolve) => setTimeout(resolve, 0)); });
    // 发起授权飞行中：断开按钮不被互斥禁用（APP-7，Vue 无此互斥）。
    assert.equal((revoke as HTMLButtonElement).disabled, false, 'revoke stays enabled while the authorization attempt is in flight');
    await act(async () => { releasePost?.(); await new Promise((resolve) => setTimeout(resolve, 10)); });
  } finally {
    await cleanup();
  }
});
