import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, { React, window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, Event: dom.window.Event, MouseEvent: dom.window.MouseEvent, IS_REACT_ACT_ENVIRONMENT: true });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

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
});
