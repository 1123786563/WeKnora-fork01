import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

type ResolveHook = (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown;
const resolveCSS: ResolveHook = (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context);
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: ResolveHook }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else nodeModule.register(`data:text/javascript,${encodeURIComponent(`export async function resolve(specifier, context, nextResolve) { if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' }; return nextResolve(specifier, context); }`)}`, import.meta.url);

const { clearDisabledGraphData, graphActionPath, graphDatabaseEnabled, graphExample, shouldRenderGraphActions, validateGraphSettings } = await import('./GraphSettings.tsx');
type GraphExtractConfig = import('./GraphSettings.tsx').GraphExtractConfig;

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

test('hides empty entity and relation collections like Vue', () => {
  assert.match(source, /local\.nodes\.length > 0/);
  assert.match(source, /local\.relations\.length > 0/);
});

test('hides protected graph actions unless the caller explicitly grants admin capability', () => {
  assert.equal(shouldRenderGraphActions(undefined), false);
  assert.equal(shouldRenderGraphActions(false), false);
  assert.equal(shouldRenderGraphActions(true), true);
});

test('keeps the existing Chinese graph settings copy instead of replacing i18n with English literals', () => {
  for (const marker of ['知识图谱配置', '启用实体关系提取', '关系类型', '示例文本', '实体列表', '关系列表', '提取操作', '知识图谱数据库未启用，实体关系提取功能将无法使用']) {
    assert.match(source, new RegExp(marker));
  }
  assert.doesNotMatch(source, /Knowledge Graph Configuration/);
});

test('does not leave graph system status stuck in loading when no client is available', () => {
  assert.match(source, /!client[\s\S]*setSystem\(\{ status: 'error'/);
});

test('resets graph system state when the supplied database engine changes', () => {
  assert.match(source, /graphDatabaseEngine !== undefined[\s\S]*setSystem\(\{ status: 'ready', engine: graphDatabaseEngine \}\)/);
});
