import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  clearDisabledGraphData,
  graphActionPath,
  graphDatabaseEnabled,
  graphExample,
  validateGraphSettings,
  type GraphExtractConfig,
} from './GraphSettings.tsx';

const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), 'GraphSettings.tsx'), 'utf8');

const emptyGraph = (): GraphExtractConfig => ({
  enabled: true,
  text: '',
  tags: [],
  nodes: [],
  relations: [],
  customInstructions: 'keep this',
});

test('matches Vue graph database capability and action endpoint contracts', () => {
  assert.equal(graphDatabaseEnabled('neo4j'), true);
  assert.equal(graphDatabaseEnabled('Not Enabled'), false);
  assert.equal(graphDatabaseEnabled(undefined), false);
  assert.equal(graphActionPath('tags'), '/api/v1/initialization/extract/fabri-tag');
  assert.equal(graphActionPath('text'), '/api/v1/initialization/extract/fabri-text');
  assert.equal(graphActionPath('relations'), '/api/v1/initialization/extract/text-relation');
});

test('clearing a disabled graph preserves custom instructions but removes generated data', () => {
  assert.deepEqual(clearDisabledGraphData({
    ...emptyGraph(),
    text: 'sample',
    tags: ['Person'],
    nodes: [{ name: 'User', attributes: ['email'] }],
    relations: [{ node1: 'User', type: 'owns', node2: 'Account' }],
  }), {
    enabled: false,
    text: '',
    tags: [],
    nodes: [],
    relations: [],
    customInstructions: 'keep this',
  });
});

test('validates the same graph action preconditions as Vue', () => {
  assert.deepEqual(validateGraphSettings(emptyGraph(), '', 'relations'), ['completeModelConfig']);
  assert.deepEqual(validateGraphSettings(emptyGraph(), 'llm-1', 'relations'), ['pleaseInputText']);
  assert.deepEqual(validateGraphSettings({ ...emptyGraph(), text: 'sample' }, 'llm-1', 'relations'), []);
  assert.deepEqual(validateGraphSettings(emptyGraph(), '', 'text'), ['completeModelConfig']);
});

test('provides the Vue default example without sharing mutable graph data', () => {
  const first = graphExample();
  const second = graphExample();
  assert.equal(first.tags.join(','), 'Author,Alias');
  assert.equal(first.nodes.length, 4);
  assert.equal(first.relations.length, 3);
  first.nodes[0].attributes.push('changed');
  assert.equal(second.nodes[0].attributes.includes('changed'), false);
});

test('renders Vue-aligned graph fields, state feedback, and embedded layout hooks', () => {
  for (const marker of ['Custom instructions', 'Relationship types', 'Sample text', 'Entities', 'Relations', 'Loading graph database status', 'tone="error"', 'tone="success"', 'wk-graph-settings-embedded']) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});
