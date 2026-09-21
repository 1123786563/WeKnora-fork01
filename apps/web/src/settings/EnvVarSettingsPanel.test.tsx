import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { EnvVarSettingsPanel } = await import('./EnvVarSettingsPanel.tsx');

test('does not render stored environment-variable values in the page markup', () => {
  const html = renderToStaticMarkup(React.createElement(EnvVarSettingsPanel, {
    client: {} as never,
    initialPayload: [{ skill_id: 'skill-1', name: 'API_TOKEN', value: 'secret-value' }],
  }));

  assert.match(html, /API_TOKEN/);
  assert.doesNotMatch(html, /secret-value/);
  assert.match(html, /type="password"/);
});

// R484 G4 D3 (R482 report-B3.md D3): Vue EnvVarSettings.vue renders the
// section header itself (lines 3-25) — h2 沙箱密钥, a help-circle hint
// trigger (envVarSettings.helpAria) opening the hover popup with the two
// intro blocks, and the section description. The React panel must own the
// same header so the help entry exists with the Vue copy.
test('renders the Vue section header and the sandbox-secrets help popup content', async () => {
  const { createRoot } = await import('react-dom/client');
  const { act } = await import('react');
  const client = {
    request: async () => [{ id: 'sbx-1', name: 'parity-sandbox' }],
  } as never;
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  await act(async () => {
    root.render(React.createElement(EnvVarSettingsPanel, { client, initialPayload: [] }));
  });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 50)); });

  try {
    const text = host.textContent ?? '';
    assert.ok(text.includes('沙箱密钥'), 'the Vue envvars h2 renders inside the panel');
    assert.ok(text.includes('给技能和沙箱用的个人密钥'), 'the Vue section description renders');

    const helpTrigger = Array.from(host.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.getAttribute('aria-label') === '沙箱密钥说明');
    assert.ok(helpTrigger, 'the help-circle hint trigger renders (envVarSettings.helpAria)');

    // The popup content is hidden until hover, like the Vue t-popup trigger="hover".
    assert.equal(host.textContent?.includes('只属于你'), false, 'popup copy stays hidden before hover');

    await act(async () => {
      helpTrigger.dispatchEvent(new dom.window.MouseEvent('mouseover', { bubbles: true }));
    });
    const popover = host.querySelector('[data-testid="envvar-help-popover"]');
    assert.ok(popover, 'hovering the hint trigger opens the popup');
    const popoverText = popover.textContent ?? '';
    assert.ok(popoverText.includes('只属于你'), 'introPersonalTitle renders');
    assert.ok(popoverText.includes('只注入到你自己的对话和执行里，空间里的其他人看不到，也不会改成他们的值。'), 'introPersonalBody renders');
    assert.ok(popoverText.includes('用的时候才带上'), 'introRuntimeTitle renders');
    assert.ok(popoverText.includes('技能运行或在沙箱里执行命令时才会注入；对话里也可以当场提供。保存后不再显示明文。'), 'introRuntimeBody renders');
  } finally {
    await act(async () => root.unmount());
    document.body.replaceChildren();
  }

/*
 * R479 A3 — R022 错误路径交互锚定（Vue EnvVarSettings.vue 契约）。
 *
 * Vue rejectBadValue（EnvVarSettings.vue:485-495）在提交前拦截：
 * - 空值  → warning「保存前请先填入值。」（envVarSettings.valueRequired）+ 不调 API
 * - 超长  → error「单个值不能超过 8192 字节。」（envVarSettings.valueTooLong,
 *           MAX_ENV_VALUE_BYTES=8192, envVarState.ts:17）+ 不调 API
 * 保存失败 → e?.message 优先、envVarSettings.saveFailed 本地化兜底（L517-518）。
 * React 侧错误呈现为面板内 <Status tone="error">（settings 域已接受的 Vue toast
 * 等价形态，R471/R472 三模式口径），消息链与 Vue 一致。
 */

interface EnvVarCalls { skillSet: number; sandboxSet: number }

function inputByType(container: HTMLElement, type: string): HTMLInputElement {
  const input = container.querySelector(`input[type="${type}"]`) as HTMLInputElement | null;
  assert.ok(input, `expected an input[type="${type}"] field`);
  return input;
}

function setInputValue(input: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function submitForm(container: HTMLElement): void {
  const form = container.querySelector('form');
  assert.ok(form, 'expected the env-var form');
  form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }));
}

let root: Root | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

test('rejects an empty value with the localized valueRequired message and skips the API (Vue rejectBadValue)', async () => {
  const calls: EnvVarCalls = { skillSet: 0, sandboxSet: 0 };
  const client = {
    settings: {
      envVars: {
        skill: { set: async () => { calls.skillSet += 1; } },
        sandbox: { set: async () => { calls.sandboxSet += 1; } },
      },
    },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(React.createElement(EnvVarSettingsPanel, { client, initialPayload: [] })));

  const inputs = [...container.querySelectorAll('input')];
  setInputValue(inputs[0] as HTMLInputElement, 'skill-1');
  setInputValue(inputs[1] as HTMLInputElement, 'API_TOKEN');
  // Leave the password value empty — the guard must reject before any API call.
  await act(async () => { submitForm(container); });

  assert.equal(calls.skillSet, 0, 'an empty value must not reach the skill env-var API');
  assert.equal(calls.sandboxSet, 0, 'an empty value must not reach the sandbox env-var API');
  assert.match(container.textContent ?? '', /保存前请先填入值。/);
});

test('rejects a value over MAX_ENV_VALUE_BYTES with the localized valueTooLong message and skips the API', async () => {
  const calls: EnvVarCalls = { skillSet: 0, sandboxSet: 0 };
  const client = {
    settings: {
      envVars: {
        skill: { set: async () => { calls.skillSet += 1; } },
        sandbox: { set: async () => { calls.sandboxSet += 1; } },
      },
    },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(React.createElement(EnvVarSettingsPanel, { client, initialPayload: [] })));

  const inputs = [...container.querySelectorAll('input')];
  setInputValue(inputs[0] as HTMLInputElement, 'skill-1');
  setInputValue(inputs[1] as HTMLInputElement, 'API_TOKEN');
  setInputValue(inputByType(container, 'password'), 'x'.repeat(8193));
  await act(async () => { submitForm(container); });

  assert.equal(calls.skillSet, 0, 'an oversized value must not reach the skill env-var API');
  assert.match(container.textContent ?? '', /单个值不能超过 8192 字节。/);
});

test('surfaces the backend error message first when saving fails and keeps the variable list rendered', async () => {
  const client = {
    settings: {
      envVars: {
        skill: { set: async () => { throw new Error('quota exceeded'); } },
        sandbox: { set: async () => undefined },
      },
    },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(React.createElement(EnvVarSettingsPanel, {
    client,
    initialPayload: [{ skill_id: 'skill-1', name: 'API_TOKEN', value: 'stored' }],
  })));

  const inputs = [...container.querySelectorAll('input')];
  setInputValue(inputs[0] as HTMLInputElement, 'skill-1');
  setInputValue(inputs[1] as HTMLInputElement, 'API_TOKEN');
  setInputValue(inputByType(container, 'password'), 'fresh-value');
  await act(async () => { submitForm(container); });
  await act(async () => { await Promise.resolve(); });

  assert.match(container.textContent ?? '', /quota exceeded/);
  assert.match(container.textContent ?? '', /API_TOKEN/, 'the stored variable row must stay rendered after a failed save');
});

test('falls back to the localized saveFailed message when the backend rejects without an Error payload', async () => {
  const client = {
    settings: {
      envVars: {
        skill: { set: async () => { throw 'network blip'; } },
        sandbox: { set: async () => undefined },
      },
    },
  } as unknown as WeKnoraClient;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(React.createElement(EnvVarSettingsPanel, { client, initialPayload: [] })));

  const inputs = [...container.querySelectorAll('input')];
  setInputValue(inputs[0] as HTMLInputElement, 'skill-1');
  setInputValue(inputs[1] as HTMLInputElement, 'API_TOKEN');
  setInputValue(inputByType(container, 'password'), 'fresh-value');
  await act(async () => { submitForm(container); });
  await act(async () => { await Promise.resolve(); });

  assert.match(container.textContent ?? '', /保存失败。/);
});
