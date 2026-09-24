import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { resolveChatModelOptions } from './model-chip.ts';

/*
 * R483 D13 (R482 report-B2): the live tenant's only KnowledgeQA model
 * (builtin-llm-mock / mock-stream-model) carries `display_name: ""` since the
 * 2026-09-19 23:44 row update. The old inline ChatRoutePage mapping collapsed
 * `display_name ?? name` onto the empty string and the .filter(name) guard
 * then dropped the model entirely, so the composer chip rendered the disabled
 * variant with no dropdown. Vue (Input-field.vue / chatResources) resolves
 * display_name || name — an empty display name falls back to name — so the
 * chip stays enabled with the model listed.
 */

/*
 * B1 model-selector convergence: every option also carries the dropdown-row
 * extras rawName (Vue v-if="model.display_name" suffix) and
 * contextLabel/contextIsDefault (Vue formatContextWindow on
 * parameters.context_window; the 200K backend default when absent).
 */
test('a chat model with an empty display_name still yields a dropdown option (Vue display_name || name)', () => {
  const options = resolveChatModelOptions([
    { id: 'builtin-llm-mock', name: 'mock-stream-model', display_name: '', type: 'KnowledgeQA' },
  ]);
  assert.deepEqual(options, [
    { id: 'builtin-llm-mock', name: 'mock-stream-model', contextLabel: '200K', contextIsDefault: true },
  ]);
});

test('a missing display_name keeps falling back to name, and a real display_name wins', () => {
  const options = resolveChatModelOptions([
    { id: 'm-absent', name: 'name-only-model' },
    { id: 'm-set', name: 'internal-id', display_name: '显示名' },
  ]);
  assert.deepEqual(options, [
    { id: 'm-absent', name: 'name-only-model', contextLabel: '200K', contextIsDefault: true },
    { id: 'm-set', name: '显示名', rawName: 'internal-id', contextLabel: '200K', contextIsDefault: true },
  ]);
});

test('models without an id stay dropped; the id stays the final label fallback', () => {
  const options = resolveChatModelOptions([
    // No display_name and a blank name fall back to the raw id (the previous
    // `?? model.id` tail kept this row too).
    { id: 'm-blank', name: '  ', display_name: '' },
    { id: '', name: 'no-id' },
    { id: 'm-ok', name: 'kept' },
  ]);
  assert.deepEqual(options, [
    { id: 'm-blank', name: 'm-blank', contextLabel: '200K', contextIsDefault: true },
    { id: 'm-ok', name: 'kept', contextLabel: '200K', contextIsDefault: true },
  ]);
});

test('chat route builds the composer model options through the Vue-parity resolver', () => {
  const routeSource = readFileSync(new URL('./ChatRoutePage.tsx', import.meta.url), 'utf8');
  assert.match(routeSource, /resolveChatModelOptions/);
  // The nullish mapping is the D13 regression: display_name "" is not nullish,
  // so `??` keeps it and the empty-name filter drops the model.
  assert.doesNotMatch(routeSource, /display_name \?\? model\.name/);
});
