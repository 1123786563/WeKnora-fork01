import assert from 'node:assert/strict';
import test from 'node:test';

import { configurationSections, configurationStatus } from './surface.ts';

test('keeps the four T15 configuration surfaces explicit and capability-labelled', () => {
  assert.deepEqual(configurationSections.map((section) => section.key), ['agents', 'models', 'mcp', 'skills']);
  assert.equal(configurationSections.find((section) => section.key === 'skills')?.writeSupport, 'read-only');
  assert.equal(configurationStatus({ source: 'env', enabled: false }), 'environment-managed');
  assert.equal(configurationStatus({ enabled: true }), 'enabled');
});

test('does not infer health from the mere presence of a configuration row', () => {
  assert.equal(configurationStatus({ id: 'model-1', name: 'Model' }), 'configured');
  assert.equal(configurationStatus({ id: 'mcp-1', name: 'MCP', enabled: false }), 'disabled');
});
