import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
const source = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'McpToolsDirectory.tsx'), 'utf8');

test('MCP tools directory exposes Vue-aligned search, policy controls, details, and pagination', () => {
  assert.match(source, /tools\.length > pageSize/);
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
