import assert from 'node:assert/strict';
import test from 'node:test';

import { cloudCredentialPatch, memoryEnabledPatch, memoryItemPatch, memoryWorkspacePatch, ollamaModelInput, profilePasswordPatch, settingsConfigPatch, settingsResourceInput, settingsResourceRows, settingsSectionMeta, settingsValueEntries, tenantEditState, tenantPatch } from './surface.ts';
import { SETTINGS_SECTIONS } from '@weknora/views';

test('gives every registered settings section a concrete inventory description', () => {
  assert.equal(settingsSectionMeta('general')?.title, 'General and preferences');
  assert.equal(settingsSectionMeta('weknoracloud')?.scope, 'tenant');
  assert.equal(SETTINGS_SECTIONS.every((section) => settingsSectionMeta(section.key)), true);
});

test('keeps settings operations explicit and does not display secret-shaped fields', () => {
  const section = settingsSectionMeta('storage');
  assert.deepEqual(section?.operations, ['read', 'save', 'test', 'delete', 'unavailable']);
  assert.deepEqual(settingsValueEntries({ name: 's3', api_key: 'hidden', configured: true, nested: { region: 'cn' } }), [
    ['name', 's3'],
    ['configured', 'true'],
    ['nested', '{"region":"cn"}'],
  ]);
});

test('validates a password change before issuing a credential mutation', () => {
  assert.deepEqual(profilePasswordPatch('old-pass', 'new-pass-123', 'new-pass-123'), {
    old_password: 'old-pass',
    new_password: 'new-pass-123',
  });
  assert.throws(() => profilePasswordPatch('same', 'same', 'same'), /different/);
  assert.throws(() => profilePasswordPatch('old', 'new', 'mismatch'), /match/);
});

test('requires meaningful personal memory content before saving', () => {
  assert.deepEqual(memoryItemPatch('  remember this  '), { content: 'remember this' });
  assert.throws(() => memoryItemPatch('   '), /content is required/);
});

test('serializes the personal memory enable toggle as an explicit write', () => {
  assert.deepEqual(memoryEnabledPatch(true), { enabled: true });
  assert.deepEqual(memoryEnabledPatch(false), { enabled: false });
});

test('limits workspace memory writes to supported typed controls', () => {
  assert.deepEqual(memoryWorkspacePatch(true, 'auto', 400, true, false), {
    enabled: true,
    write_mode: 'auto',
    max_items: 400,
    vector_recall: true,
    retrieval_conditioning: false,
  });
  assert.throws(() => memoryWorkspacePatch(true, 'unknown', 400, true, true), /write mode/);
  assert.throws(() => memoryWorkspacePatch(true, 'auto', 1, true, true), /max items/);
});

test('limits tenant editing to the server-owned name and description fields', () => {
  assert.deepEqual(tenantEditState({ id: 7, name: 'Acme', description: 'Docs', owner_id: 'u-1' }), { name: 'Acme', description: 'Docs' });
  assert.deepEqual(tenantPatch(' Acme ', ' Docs '), { name: 'Acme', description: 'Docs' });
  assert.throws(() => tenantPatch('  ', 'Docs'), /name/);
});

test('normalizes resource settings rows and validates a resource mutation payload', () => {
  assert.deepEqual(settingsResourceRows({ backends: [{ id: 'storage-1', name: 'Local' }], legacy: {} }, 'storage'), [{ id: 'storage-1', name: 'Local' }]);
  assert.deepEqual(settingsResourceRows([{ id: 'vector-1' }], 'vectorstore'), [{ id: 'vector-1' }]);
  assert.deepEqual(settingsResourceInput('  Primary ', ' qdrant ', '{"url":"http://qdrant"}'), {
    name: 'Primary',
    type: 'qdrant',
    config: { url: 'http://qdrant' },
  });
  assert.throws(() => settingsResourceInput('', 'qdrant', '{}'), /name/);
  assert.throws(() => settingsResourceInput('Primary', '', '{}'), /type/);
  assert.throws(() => settingsResourceInput('Primary', 'qdrant', '[]'), /object/);
});

test('builds field-level patches for retrieval, chat history, and parser settings', () => {
  assert.deepEqual(settingsConfigPatch('retrieval', { embedding_top_k: '20', vector_threshold: '0.25', keyword_threshold: '0.3', rerank_top_k: '10', rerank_threshold: '-0.2', rerank_model_id: 'rerank-1' }), {
    embedding_top_k: 20, vector_threshold: 0.25, keyword_threshold: 0.3, rerank_top_k: 10, rerank_threshold: -0.2, rerank_model_id: 'rerank-1',
  });
  assert.deepEqual(settingsConfigPatch('chathistory', { enabled: true, embedding_model_id: 'embed-1' }), { enabled: true, embedding_model_id: 'embed-1' });
  assert.deepEqual(settingsConfigPatch('parser', { mineru_endpoint: ' https://mineru.example ', mineru_api_key: '  ' }), { mineru_endpoint: 'https://mineru.example' });
  assert.throws(() => settingsConfigPatch('retrieval', { embedding_top_k: '0', vector_threshold: '0', keyword_threshold: '0', rerank_top_k: '1', rerank_threshold: '0', rerank_model_id: '' }), /embedding_top_k/);
  assert.throws(() => settingsConfigPatch('parser', { mineru_endpoint: 'not-a-url' }), /endpoint/);
});

test('requires a concrete Ollama model name before starting a download', () => {
  assert.equal(ollamaModelInput('  llama3.2:latest  '), 'llama3.2:latest');
  assert.throws(() => ollamaModelInput('  '), /model name/);
});

test('requires both WeKnora Cloud credential fields without accepting a blank secret', () => {
  assert.deepEqual(cloudCredentialPatch(' app-id ', ' app-secret '), { app_id: 'app-id', app_secret: 'app-secret' });
  assert.throws(() => cloudCredentialPatch('', 'secret'), /app ID/);
  assert.throws(() => cloudCredentialPatch('id', '  '), /app secret/);
});
