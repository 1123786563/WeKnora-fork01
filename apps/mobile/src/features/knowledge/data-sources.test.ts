import assert from 'node:assert/strict';
import test from 'node:test';
import { canManageDataSources, dataSourceStatusLabel, safeDataSourceType } from './data-sources.ts';

test('mobile data source inventory exposes safe status and type labels', () => {
  assert.equal(dataSourceStatusLabel({ status: 'active' }), 'active');
  assert.equal(dataSourceStatusLabel({ status: undefined }), 'unknown');
  assert.equal(safeDataSourceType({ type: 'notion' }), 'notion');
  assert.equal(safeDataSourceType({ type: '' }), 'unknown');
});

test('mobile data-source mutations fail closed for non-admin workspace roles', () => {
  assert.equal(canManageDataSources('owner'), true);
  assert.equal(canManageDataSources('admin'), true);
  assert.equal(canManageDataSources('contributor'), false);
  assert.equal(canManageDataSources('viewer'), false);
  assert.equal(canManageDataSources(undefined), false);
});
