import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerApi } from '@weknora/api-client/career';
import { CareerDesk } from '../src/desk.ts';

test('CareerApi composes directly into Career Desk', () => {
  const api = createCareerApi(async () => ({ revision: 0, facts: [], proposals: [] }));
  const desk = new CareerDesk(api);
  assert.equal(typeof desk.open, 'function');
  assert.equal(typeof desk.refresh, 'function');
});
