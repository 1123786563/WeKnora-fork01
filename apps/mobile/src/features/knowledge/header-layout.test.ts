import assert from 'node:assert/strict';
import test from 'node:test';
import { knowledgeHeaderLayout } from './header-layout.ts';

test('knowledge header exposes the native chat route', () => {
  assert.equal(knowledgeHeaderLayout.chatRoute, '/chat');
});

test('knowledge header keeps all actions in a reachable wrapping row', () => {
  assert.equal(knowledgeHeaderLayout.container.flexDirection, 'column');
  assert.equal(knowledgeHeaderLayout.actions.flexDirection, 'row');
  assert.equal(knowledgeHeaderLayout.actions.flexWrap, 'wrap');
  assert.equal(knowledgeHeaderLayout.actions.width, '100%');
});
