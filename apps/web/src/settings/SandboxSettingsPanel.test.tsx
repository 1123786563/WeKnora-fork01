import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`, import.meta.url);

(globalThis as typeof globalThis & { React: typeof React }).React = React;

const { SandboxSettingsPanel } = await import('./SandboxSettingsPanel.tsx');
const client = {} as never;

const e2bRecord = {
  id: 'sandbox-1',
  name: 'E2B production',
  description: 'Remote execution',
  sandbox_type: 'e2b',
  config: { sandbox_type: 'e2b', e2b: { api_url: 'https://api.e2b.app', template_id: 'template-1', api_key: '<redacted>' } },
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
};

test('sandbox settings renders a loading state before its list request resolves', () => {
  const html = renderToStaticMarkup(React.createElement(SandboxSettingsPanel, { client, role: 'admin' }));
  assert.match(html, /Loading sandbox configurations/);
});

test('sandbox settings keeps viewer actions read-only and renders the empty state', () => {
  const html = renderToStaticMarkup(React.createElement(SandboxSettingsPanel, {
    client,
    role: 'viewer',
    initialData: { items: [], workspaceScriptsDisabled: false },
  }));
  assert.match(html, /No sandbox configurations configured/);
  assert.doesNotMatch(html, /Add sandbox configuration/);
  assert.doesNotMatch(html, /Disable script execution/);
});

test('sandbox settings renders backend tabs, cards, admin actions, and supported-scope note', () => {
  const html = renderToStaticMarkup(React.createElement(SandboxSettingsPanel, {
    client,
    role: 'admin',
    initialData: {
      items: [e2bRecord],
      workspaceScriptsDisabled: false,
    },
  }));
  assert.match(html, /All \(1\)/);
  assert.match(html, /E2B \(1\)/);
  assert.match(html, /E2B production/);
  assert.match(html, /Edit/);
  assert.match(html, /Inspect inventory/);
  assert.match(html, /Delete/);
  assert.match(html, /Add sandbox configuration/);
  assert.match(html, /Wizard, template catalog, deep checks, and skill installation remain unsupported/);
});

test('sandbox settings exposes a confirmation step for the workspace script kill-switch', () => {
  const html = renderToStaticMarkup(React.createElement(SandboxSettingsPanel, {
    client,
    role: 'owner',
    initialData: { items: [], workspaceScriptsDisabled: false },
  }));
  assert.match(html, /Disable script execution/);
  assert.match(html, /data-confirm="disable-scripts"/);
  assert.match(html, /Confirm disable/);
  assert.match(html, /Cancel/);
});

test('sandbox settings renders the enable state without a destructive confirmation', () => {
  const html = renderToStaticMarkup(React.createElement(SandboxSettingsPanel, {
    client,
    role: 'admin',
    initialData: { items: [], workspaceScriptsDisabled: true },
  }));
  assert.match(html, /Enable script execution/);
  assert.doesNotMatch(html, /data-confirm="disable-scripts"/);
});
