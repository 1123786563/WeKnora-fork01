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

const { McpSettingsPanel, importMcpConfig } = await import('./McpSettingsPanel.tsx');

const client = {} as never;

test('MCP settings keeps the Vue empty state for a viewer', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [],
    role: 'viewer',
  }));
  assert.match(html, /No MCP services configured/);
  assert.doesNotMatch(html, /Add MCP service/);
});

test('MCP settings renders service metadata and admin actions', () => {
  const html = renderToStaticMarkup(React.createElement(McpSettingsPanel, {
    client,
    initialServices: [{ id: 'mcp-1', name: 'Docs', description: 'Search docs', enabled: true, transport_type: 'sse', is_builtin: false }],
    role: 'admin',
  }));
  assert.match(html, /Docs/);
  assert.match(html, /Search docs/);
  assert.match(html, /Edit/);
  assert.match(html, /Delete/);
  assert.match(html, /Add MCP service/);
});

test('MCP JSON import maps transport, auth, and custom headers without saving', () => {
  const base = { name: '', description: '', usageInstructions: '', url: '', transportType: 'sse', enabled: true, authType: '', apiKeyHeader: '', apiKey: '', oauthScopes: '', headers: [], timeout: 30, retryCount: 3, retryDelay: 1, codeImport: '', codeImportError: '', authConfig: {} } as never;
  const draft = importMcpConfig(JSON.stringify({ mcpServers: { docs: { url: 'https://example.com/mcp', headers: { Authorization: 'Bearer secret', 'X-Trace': 'yes' } } } }), base);
  assert.equal(draft.name, 'docs');
  assert.equal(draft.transportType, 'http-streamable');
  assert.equal(draft.authType, 'api_key');
  assert.equal(draft.apiKeyHeader, 'Authorization');
  assert.deepEqual(draft.headers, [{ key: 'X-Trace', value: 'yes' }]);
});
