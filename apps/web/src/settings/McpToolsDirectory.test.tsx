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
const { McpToolsDirectory } = await import('./McpToolsDirectory.tsx');

test('MCP tools directory preserves Vue detail tabs, policy controls, and pagination entry', () => {
  const html = renderToStaticMarkup(React.createElement(McpToolsDirectory, {
    serviceId: 'svc-1', busy: false, policyError: null, onRetryPolicies: () => undefined, onPolicyChange: () => undefined,
    approvals: [], tools: Array.from({ length: 21 }, (_, index) => ({ name: `tool-${index}`, description: index === 0 ? 'Search docs' : undefined, inputSchema: index === 0 ? { properties: { query: { type: 'string' } }, required: ['query'] } : undefined })),
  }));
  assert.match(html, /Search tools/);
  assert.match(html, /tool-0/);
  assert.match(html, /Enabled/);
  assert.match(html, /Previous/);
  assert.match(html, /Next/);
  assert.match(html, /1 \/ 2/);
});

test('MCP tools directory fails closed and offers policy retry', () => {
  const html = renderToStaticMarkup(React.createElement(McpToolsDirectory, { serviceId: 'svc-1', busy: false, policyError: 'policy unavailable', onRetryPolicies: () => undefined, onPolicyChange: () => undefined, approvals: [], tools: [] }));
  assert.match(html, /policy unavailable/);
  assert.match(html, /Retry/);
});
