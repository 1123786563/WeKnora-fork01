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
  // Vue McpToolsList.vue copy (zh-CN): searchTools/启用工具/上一步-style pager.
  assert.match(html, /搜索工具名称或描述/);
  assert.match(html, /tool-0/);
  assert.match(html, /启用工具/);
  assert.match(html, /调用需审批/);
  assert.match(html, /详情/);
  assert.match(html, /上一步/);
  assert.match(html, /下一页/);
  assert.match(html, /1 \/ 2/);
});

test('MCP tools directory fails closed and offers policy retry', () => {
  const html = renderToStaticMarkup(React.createElement(McpToolsDirectory, { serviceId: 'svc-1', busy: false, policyError: 'policy unavailable', onRetryPolicies: () => undefined, onPolicyChange: () => undefined, approvals: [], tools: [] }));
  assert.match(html, /policy unavailable/);
  assert.match(html, /重试/);
});

test('MCP policy errors keep the Vue tool directory visible and disable its controls', () => {
  const html = renderToStaticMarkup(React.createElement(McpToolsDirectory, {
    serviceId: 'svc-1',
    busy: false,
    policyError: 'policy unavailable',
    onRetryPolicies: () => undefined,
    onPolicyChange: () => undefined,
    approvals: [],
    tools: [{ name: 'search', description: 'Search docs' }],
  }));
  assert.match(html, /search/);
  assert.match(html, /详情/);
  assert.match(html, /policy unavailable/);
  const switches = html.match(/role="switch"/g) ?? [];
  assert.equal(switches.length, 2);
  assert.equal((html.match(/disabled=""/g) ?? []).length, 2, 'Vue policy error disables both switches');
});

test('MCP stale metadata keeps the Vue read-only directory without policy switches', () => {
  const html = renderToStaticMarkup(React.createElement(McpToolsDirectory, { serviceId: undefined, busy: true, policyError: null, onRetryPolicies: () => undefined, onPolicyChange: () => undefined, approvals: [], tools: [{ name: 'search' }] }));
  assert.match(html, /search/);
  assert.doesNotMatch(html, /启用工具/);
  assert.doesNotMatch(html, /调用需审批/);
});

test('MCP tool details expose a modal tab keyboard contract', () => {
  assert.match(source, /role="dialog" aria-modal="true"/);
  assert.match(source, /role="tabpanel"/);
  assert.match(source, /aria-controls=\{panelId\}/);
  assert.match(source, /tabIndex=\{tab === item \? 0 : -1\}/);
  assert.match(source, /event\.key === 'ArrowRight'/);
  assert.match(source, /event\.key === 'ArrowLeft'/);
  assert.match(source, /event\.key === 'Tab'/);
});

test('MCP tool details restore focus and keep detail ids unique per rendered tool', () => {
  assert.match(source, /tabRefs\.current/);
  assert.match(source, /activeTab\?\.focus\(\)/);
  assert.match(source, /trigger\.focus\(\)/);
  assert.match(source, /mcp-tool-detail-\$\{index\}/);
});
