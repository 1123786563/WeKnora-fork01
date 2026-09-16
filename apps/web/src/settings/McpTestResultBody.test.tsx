import assert from 'node:assert/strict';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { McpTestResultBody } from './McpTestResultBody.tsx';

test('MCP test result preserves Vue success details, tools, schemas, resources, and empty state', () => {
  const html = renderToStaticMarkup(React.createElement(McpTestResultBody, { approvals: [{ toolName: 'search_docs', enabled: false, requireApproval: true }], onPolicyChange: () => undefined, result: {
    success: true,
    description: 'Documentation connector',
    tools: [{ name: 'search_docs', description: 'Search documentation', inputSchema: { type: 'object', properties: { query: { type: 'string' } } } }],
    resources: [{ name: 'readme', uri: 'mcp://readme', mimeType: 'text/plain', description: 'Connector overview' }],
  } }));
  assert.match(html, /Connection succeeded/);
  assert.match(html, /Documentation connector/);
  assert.match(html, /search_docs/);
  assert.match(html, /Enabled/);
  assert.match(html, /Approval/);
  assert.match(html, /aria-expanded="false"/);
  assert.match(html, /readme/);
  assert.match(html, /mcp:\/\/readme/);
});

test('MCP test result preserves failure and successful empty states', () => {
  assert.match(renderToStaticMarkup(React.createElement(McpTestResultBody, { result: { success: false, message: 'OAuth required' } })), /OAuth required/);
  assert.match(renderToStaticMarkup(React.createElement(McpTestResultBody, { result: { success: true } })), /No tools or resources returned/);
});
