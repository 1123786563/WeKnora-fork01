import assert from 'node:assert/strict';
import test from 'node:test';
import { ContractError } from '../index.ts';
import { parseAdoptionResponse, parseAdoptionListResponse, parseVariantResponse, parseAvailableAgentListResponse } from './agent-adoption.ts';

const variant = { id: 'variant-1', adoption_id: 'adoption-1', release_id: 'release-1', name: 'Sales Assistant', state: 'draft', missing_capabilities: ['knowledge', 'model'], created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z' };
const adoption = { id: 'adoption-1', listing_id: 'listing-1', accepted_release_id: 'release-1', state: 'active', created_by: 'admin', created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:00Z', variants: [variant] };
const available = { agent_id: 'agent-1', variant_id: 'variant-1', adoption_id: 'adoption-1', release_id: 'release-1', name: 'Sales Assistant', description: 'A helpful agent', is_builtin: false, capability: { state: 'supported', reason: '' } };

test('parses adoption with nested variants and missing capabilities', () => {
  const parsed = parseAdoptionResponse({ success: true, data: adoption });
  assert.equal(parsed.id, adoption.id);
  assert.equal(parsed.variants[0].missing_capabilities.join(','), 'knowledge,model');
  const published = parseVariantResponse({ success: true, data: { ...variant, state: 'published', local_agent_id: 'agent-1', local_agent_version_id: 'version-1', missing_capabilities: [] } });
  assert.equal(published.local_agent_id, 'agent-1');
  assert.equal(published.state, 'published');
  assert.deepEqual(parseAdoptionListResponse({ success: true, data: [adoption] }), [adoption]);
});

test('available agents carry lineage and a supported verdict', () => {
  const rows = parseAvailableAgentListResponse({ success: true, data: [available] });
  assert.equal(rows[0].agent_id, 'agent-1');
  assert.equal(rows[0].capability.state, 'supported');
  assert.deepEqual(parseAvailableAgentListResponse({ success: true, data: [] }), []);
});

test('rejects malformed ids, states, verdicts and envelopes', () => {
  assert.throws(() => parseAdoptionResponse({ success: true, data: { ...adoption, id: '' } }), ContractError);
  assert.throws(() => parseAdoptionResponse({ success: true, data: { ...adoption, listing_id: '' } }), ContractError);
  assert.throws(() => parseAdoptionResponse({ success: false, data: adoption }), ContractError);
  assert.throws(() => parseAdoptionListResponse({ success: true, data: adoption }), ContractError);
  assert.throws(() => parseVariantResponse({ success: true, data: { ...variant, state: 'unknown-state' } }), ContractError);
  assert.throws(() => parseVariantResponse({ success: true, data: { ...variant, missing_capabilities: 'knowledge' } }), ContractError);
  assert.throws(() => parseAvailableAgentListResponse({ success: true, data: [{ ...available, agent_id: '' }] }), ContractError);
  assert.throws(() => parseAvailableAgentListResponse({ success: true, data: [{ ...available, capability: { state: 'maybe', reason: '' } }] }), ContractError);
});
