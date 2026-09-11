import assert from 'node:assert/strict';
import test from 'node:test';
import { dataSourceStatusLabel, safeDataSourceType } from './data-sources.ts';

test('mobile data source inventory exposes safe status and type labels', () => {
  assert.equal(dataSourceStatusLabel({ status: 'active' }), 'active');
  assert.equal(dataSourceStatusLabel({ status: undefined }), 'unknown');
  assert.equal(safeDataSourceType({ type: 'notion' }), 'notion');
  assert.equal(safeDataSourceType({ type: '' }), 'unknown');
});
