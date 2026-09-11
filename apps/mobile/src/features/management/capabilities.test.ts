import assert from 'node:assert/strict';
import test from 'node:test';
import { MOBILE_CAPABILITIES, mobileCapability, projectMobileCapability } from './capabilities.ts';

test('mobile capability matrix is explicit about core, read-only, and unsupported surfaces', () => {
  assert.equal(MOBILE_CAPABILITIES.filter((item) => item.support === 'core').length, 4);
  assert.equal(mobileCapability('sandbox')?.support, 'unsupported');
  assert.match(mobileCapability('identity')?.reason || '', /server-owned/);
});

test('mobile capability projection fails closed when the server disables a surface', () => {
  const projected = projectMobileCapability(mobileCapability('organizations')!, {
    organizations: { supported: false, reason: 'not_supported_in_lite' },
  });
  assert.equal(projected.support, 'unsupported');
  assert.equal(projected.reason, 'not_supported_in_lite');
});
