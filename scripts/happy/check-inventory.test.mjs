import test from 'node:test';
import assert from 'node:assert/strict';
import { checkInventory, validateCommittedInventory } from './check-inventory.mjs';

test('inventory cannot silently omit a route', () => {
  assert.throws(() => checkInventory([], ['sources/app/(app)/index.tsx']), /unmapped/);
});

test('duplicate IDs are rejected', () => {
  const row = {id:'voice.start', route:'voice', source:'apiVoice.ts', interaction:'start', destination:'voice adapter', service:'voice', task:'H21', status:'pending'};
  assert.throws(() => checkInventory([row,row], ['voice']), /duplicate/);
});

test('rows require destinations and valid status', () => {
  const row = {id:'home.open', route:'home', source:'index.tsx', interaction:'open', destination:'', service:'navigation', task:'H02', status:'pending'};
  assert.throws(() => checkInventory([row], ['home']), /missing destination/);
});

test('a complete inventory passes', () => {
  checkInventory([{id:'home.open', route:'home', source:'index.tsx', interaction:'open', destination:'home', service:'navigation', task:'H02', status:'pending'}], ['home']);
});

test('invalid status is rejected', () => {
  assert.throws(() => checkInventory([{id:'x',route:'r',source:'s',interaction:'i',destination:'d',service:'s',task:'H01',status:'done'}], ['r']), /invalid status/);
});

test('committed matrix covers every fixed production route', () => {
  const count = validateCommittedInventory({upstream:'/Users/wuyongjun/trea/happy', commit:'ac64b9b4677870f7b7a9eacfd0780959229717f1', matrixPath:'docs/migrations/happy/interaction-matrix.json'});
  assert.ok(count >= 35);
});
