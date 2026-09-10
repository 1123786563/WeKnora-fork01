import assert from 'node:assert/strict';
import test from 'node:test';

import { scopedKey } from './query-key.ts';

test('scopedKey includes origin, user and tenant before resource parameters', () => {
  assert.deepEqual(
    scopedKey(
      { origin: 'https://example.test/base/', userId: 'user-1', tenantId: 'tenant-1', generation: 4 },
      'knowledge-bases',
      { creator: 'mine' },
    ),
    ['weknora', 'https://example.test/base', 'user-1', 'tenant-1', 'knowledge-bases', { creator: 'mine' }],
  );
});
