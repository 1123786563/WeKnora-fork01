import assert from 'node:assert/strict';
import test from 'node:test';
import { parseIntegrationTenantId } from './tenant.ts';

test('accepts only a positive safe integer from the active scope', () => {
  assert.equal(parseIntegrationTenantId('42'), 42);
  assert.equal(parseIntegrationTenantId('0'), null);
  assert.equal(parseIntegrationTenantId('tenant-42'), null);
  assert.equal(parseIntegrationTenantId(null), null);
});
