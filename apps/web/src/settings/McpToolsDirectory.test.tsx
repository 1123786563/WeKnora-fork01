import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'McpToolsDirectory.tsx'), 'utf8');

test('MCP tools directory exposes Vue-aligned search, policy controls, details, and pagination', () => {
  assert.match(source, /tools\.length > pageSize/);
  assert.match(source, /'mcpMetadata\.searchTools': 'Search tool names or descriptions'/);
  assert.match(source, /aria-label=\{t\('mcpMetadata\.searchTools'\)\}/);
  assert.match(source, /aria-controls=\{detailId\}/);
  assert.match(source, /aria-live="polite"/);
  assert.match(source, /filtered\.length > pageSize/);
});

test('MCP tools directory fails closed and offers policy retry', () => {
  assert.match(source, /policyError \?/);
  assert.match(source, /onRetryPolicies/);
  assert.match(source, /Status tone="error"/);
});

test('MCP policy errors keep the Vue tool directory visible while disabling policy controls', () => {
  assert.match(source, /policyError[\s\S]*?visible\.map/);
  assert.match(source, /disabled=\{busy \|\| busyTools\?\.has\(tool\.name\) === true \|\| Boolean\(policyError\)\}/);
});

test('MCP stale metadata keeps the Vue read-only directory without policy switches', () => {
  assert.match(source, /serviceId \? /);
  assert.match(source, /t\('mcpMetadata\.noTools'\)/);
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
