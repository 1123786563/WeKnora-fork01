import assert from 'node:assert/strict';
import test from 'node:test';
import { createCareerApi } from '@weknora/api-client/career';
import { createCareerDesk } from '../src/index.ts';
import type { CareerCommand, CareerIntentStore, CareerScope } from '@weknora/contracts';

test('CareerApi composes directly into Career Desk with scoped durable intent storage', () => {
  const scope: CareerScope = { deploymentOrigin: 'https://weknora.example', tenantId: 'tenant-1', actorId: 'actor-1' };
  const api = createCareerApi(async () => ({ success: true, data: { revision: 1, value: { opportunities: [], applications: [] } } }));
  const store: CareerIntentStore<CareerCommand> = { save: async () => undefined, list: async () => [], remove: async () => undefined };
  const desk = createCareerDesk({ remote: api, intentStore: store, initialScope: scope });
  assert.equal(typeof desk.open, 'function');
  assert.equal(typeof desk.reconcilePending, 'function');
});
