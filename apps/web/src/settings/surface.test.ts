import assert from 'node:assert/strict';
import test from 'node:test';

import { chatHistoryEmbeddingLocked, cloudCredentialPatch, envVarRemove, envVarSet, formatUptimeText, memoryEnabledPatch, memoryItemPatch, memoryWorkspacePatch, ollamaModelInput, profilePasswordPatch, settingsCloseMode, settingsConfigPatch, settingsResourceInput, settingsResourceRows, settingsSectionHeading, settingsSectionMeta, settingsValueEntries, systemInfoRows, tenantEditState, tenantModelIds, tenantPatch } from './surface.ts';
import { SETTINGS_SECTIONS, roleAtLeast, settingsSection, settingsSectionsForRole } from '@weknora/views';

// Vue section headers: GeneralSettings.vue lines 3-6, EnvVarSettings.vue lines
// 3-15, ModelSettings.vue lines 3-8, TenantInfo.vue lines 3-6, UserProfile.vue
// lines 3-6 — every section renders an h2 + section-description pair.
test('localizes the Vue section h2 + description pair for the wrapper heading', () => {
  assert.deepEqual(settingsSectionHeading('zh-CN', 'general'), { title: '常规设置', description: '配置语言、外观等基础选项' });
  assert.deepEqual(settingsSectionHeading('zh-CN', 'models'), { title: '模型配置', description: '管理不同类型的 AI 模型，支持 Ollama 本地模型和远程 API' });
  assert.deepEqual(settingsSectionHeading('zh-CN', 'tenant'), { title: '空间信息', description: '查看空间的详细配置信息' });
  assert.deepEqual(settingsSectionHeading('zh-CN', 'userprofile'), { title: '用户信息', description: '查看您的账户基础信息（用户 ID、用户名、邮箱、注册时间），并可修改登录密码' });
  const envvars = settingsSectionHeading('zh-CN', 'envvars');
  assert.equal(envvars.title, '沙箱密钥');
  assert.equal(envvars.description, '给技能和沙箱用的个人密钥，不是 WeKnora 的系统或部署配置。');
  // Vue TenantMembers.vue renders h2 only — no description line. The old
  // fallback leaked the registry apiDomain ("identity.tenants.members").
  assert.deepEqual(settingsSectionHeading('zh-CN', 'members'), { title: '成员管理', description: '' });
  assert.deepEqual(settingsSectionHeading('zh-CN', 'storage'), { title: '存储引擎', description: '配置文档与图片的存储方式。此处设置各引擎参数，知识库中仅选择使用哪个引擎。' });
  assert.equal(settingsSectionHeading('zh-CN', 'vectorstore').title, '向量数据库引擎');
  assert.equal(settingsSectionHeading('zh-CN', 'websearch').title, '网络搜索配置');
  // English locale resolves through the same shared keys.
  assert.equal(settingsSectionHeading('en-US', 'envvars').title, 'Sandbox secrets');
});

test('gives every registered settings section a concrete inventory description', () => {
  assert.equal(settingsSectionMeta('general')?.title, 'General and preferences');
  assert.equal(settingsSectionMeta('weknoracloud')?.scope, 'tenant');
  assert.equal(SETTINGS_SECTIONS.every((section) => settingsSectionMeta(section.key)), true);
});

test('system-admin settings keep Vue-localized section titles on direct role-denied links', () => {
  assert.equal(settingsSectionMeta('system-global')?.title, '系统全局设置');
  assert.equal(settingsSectionMeta('runtime-queues')?.title, '运行时队列');
  assert.equal(settingsSectionMeta('platform-api-keys')?.title, '平台 API Key');
  assert.equal(settingsSectionMeta('system-audit-log')?.title, '审计日志');
});

test('matches Vue close routing: ordinary sections go back, system admin sections go home', () => {
  assert.equal(settingsCloseMode('?section=general'), 'history');
  assert.equal(settingsCloseMode('?section=members'), 'history');
  assert.equal(settingsCloseMode('?section=system-global'), 'knowledge-bases');
  assert.equal(settingsCloseMode('?section=runtime-queues'), 'knowledge-bases');
  assert.equal(settingsCloseMode('?section=platform-api-keys'), 'knowledge-bases');
  assert.equal(settingsCloseMode('?section=system-audit-log'), 'knowledge-bases');
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
  assert.deepEqual(memoryWorkspacePatch(true, 'auto', 400, true, true, {
    extractModelId: 'chat-1', extractDelaySeconds: 90, extractMinIntervalSeconds: 300,
    extractInstructions: 'keep concise', interestThreshold: 3, embeddingModelId: 'embed-1',
  }), {
    enabled: true, write_mode: 'auto', max_items: 400, vector_recall: true, retrieval_conditioning: true,
    extract_model_id: 'chat-1', extract_delay_seconds: 90, extract_min_interval_seconds: 300,
    extract_instructions: 'keep concise', interest_threshold: 3, embedding_model_id: 'embed-1',
  });
  assert.throws(() => memoryWorkspacePatch(true, 'auto', 400, true, true, { extractDelaySeconds: 4 }), /extract delay/);
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
  assert.throws(() => settingsResourceInput('   ', 'qdrant', '{}'), /name/);
  assert.throws(() => settingsResourceInput('Primary', '', '{}'), /type/);
  assert.throws(() => settingsResourceInput('Primary', 'qdrant', '{bad json}'), /JSON/);
  assert.throws(() => settingsResourceInput('Primary', 'qdrant', '[]'), /object/);
});

test('builds field-level patches for retrieval, chat history, and parser settings', () => {
  assert.deepEqual(settingsConfigPatch('retrieval', { embedding_top_k: '20', vector_threshold: '0.25', keyword_threshold: '0.3', rerank_top_k: '10', rerank_threshold: '-0.2', rerank_model_id: 'rerank-1' }), {
    embedding_top_k: 20, vector_threshold: 0.25, keyword_threshold: 0.3, rerank_top_k: 10, rerank_threshold: -0.2, rerank_model_id: 'rerank-1',
  });
  assert.deepEqual(settingsConfigPatch('chathistory', { enabled: true, embedding_model_id: 'embed-1' }), { enabled: true, embedding_model_id: 'embed-1' });
  const parser = settingsConfigPatch('parser', { mineru_endpoint: ' https://mineru.example ', mineru_api_key: '  ' });
  assert.equal(parser.mineru_endpoint, 'https://mineru.example');
  assert.equal('mineru_api_key' in parser, false);
  assert.throws(() => settingsConfigPatch('retrieval', { embedding_top_k: '0', vector_threshold: '0', keyword_threshold: '0', rerank_top_k: '1', rerank_threshold: '0', rerank_model_id: '' }), /embedding_top_k/);
  assert.throws(() => settingsConfigPatch('parser', { mineru_endpoint: 'not-a-url' }), /endpoint/);
});

test('preserves the Vue parser-engine configuration fields when saving', () => {
  assert.deepEqual(settingsConfigPatch('parser', {
    mineru_endpoint: ' https://mineru.example ', mineru_api_key: ' mineru-key ', mineru_model: 'vlm-auto-engine',
    mineru_vlm_server_url: ' https://vllm.example ', mineru_enable_formula: false, mineru_enable_table: true,
    mineru_parse_method: 'txt', mineru_language: ' en ', mineru_cloud_model: 'vlm',
    mineru_cloud_enable_formula: false, mineru_cloud_enable_table: true, mineru_cloud_enable_ocr: false,
    mineru_cloud_language: ' ja ', paddleocr_vl_endpoint: ' https://paddle.example ',
    paddleocr_vl_use_seal_recognition: false, paddleocr_vl_use_chart_recognition: true,
    paddleocr_vl_cloud_token: ' paddle-token ', paddleocr_vl_cloud_model: ' PaddleOCR-VL-1.6 ',
    paddleocr_vl_cloud_use_seal_recognition: false, paddleocr_vl_cloud_use_chart_recognition: true,
  }), {
    mineru_endpoint: 'https://mineru.example', mineru_api_key: 'mineru-key', mineru_model: 'vlm-auto-engine',
    mineru_vlm_server_url: 'https://vllm.example', mineru_enable_formula: false, mineru_enable_table: true,
    mineru_parse_method: 'txt', mineru_enable_ocr: false, mineru_language: 'en', mineru_cloud_model: 'vlm',
    mineru_cloud_enable_formula: false, mineru_cloud_enable_table: true, mineru_cloud_enable_ocr: false,
    mineru_cloud_language: 'ja', paddleocr_vl_endpoint: 'https://paddle.example',
    paddleocr_vl_use_seal_recognition: false, paddleocr_vl_use_chart_recognition: true,
    paddleocr_vl_cloud_token: 'paddle-token', paddleocr_vl_cloud_model: 'PaddleOCR-VL-1.6',
    paddleocr_vl_cloud_use_seal_recognition: false, paddleocr_vl_cloud_use_chart_recognition: true,
  });
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

test('builds skill and sandbox env-var mutation payloads with their scope ids', () => {
  assert.deepEqual(envVarSet('skill', ' skill-1 ', ' API_KEY ', 'v1'), {
    scope: 'skill', name: 'API_KEY', value: 'v1', body: { skill_id: 'skill-1', name: 'API_KEY', value: 'v1' },
  });
  assert.deepEqual(envVarSet('sandbox', 'cfg-1', 'TOKEN', 'v2').body, { sandbox_config_id: 'cfg-1', name: 'TOKEN', value: 'v2' });
  assert.throws(() => envVarSet('skill', '  ', 'NAME', 'v'), /Skill ID/);
  assert.throws(() => envVarSet('sandbox', 'cfg-1', '  ', 'v'), /name is required/);
});

test('builds skill and sandbox env-var removal payloads without a value', () => {
  assert.deepEqual(envVarRemove('skill', 'skill-1', 'API_KEY'), { skill_id: 'skill-1', name: 'API_KEY' });
  assert.deepEqual(envVarRemove('sandbox', 'cfg-1', 'TOKEN'), { sandbox_config_id: 'cfg-1', name: 'TOKEN' });
  assert.throws(() => envVarRemove('sandbox', '', 'TOKEN'), /Sandbox config ID/);
});

test('extracts tenant model ids for selector option lists', () => {
  assert.deepEqual(tenantModelIds([{ id: 'm-1', name: 'A' }, { id: 'm-2' }, null, { name: 'x' }, 'junk']), ['m-1', 'm-2']);
  assert.deepEqual(tenantModelIds(undefined), []);
});

test('rejects saving model ids that are not in the tenant model list', () => {
  const base = { embedding_top_k: 5, vector_threshold: 0.1, keyword_threshold: 0.2, rerank_top_k: 5, rerank_threshold: 0 };
  const allowed = ['m-1', 'm-2'];
  assert.throws(() => settingsConfigPatch('retrieval', { ...base, rerank_model_id: 'not-in-list' }, { allowedModelIds: allowed }), /rerank_model_id/);
  assert.equal(settingsConfigPatch('retrieval', { ...base, rerank_model_id: ' m-2 ' }, { allowedModelIds: allowed }).rerank_model_id, 'm-2');
  assert.equal(settingsConfigPatch('retrieval', { ...base, rerank_model_id: 'anything' }).rerank_model_id, 'anything');
  assert.throws(() => settingsConfigPatch('chathistory', { enabled: true, embedding_model_id: 'ghost' }, { allowedModelIds: allowed }), /embedding_model_id/);
  assert.equal(settingsConfigPatch('chathistory', { enabled: true, embedding_model_id: 'm-1' }, { allowedModelIds: allowed }).embedding_model_id, 'm-1');
});

test('locks the chat-history embedding model once messages are indexed', () => {
  assert.equal(chatHistoryEmbeddingLocked({ has_indexed_messages: true }), true);
  assert.equal(chatHistoryEmbeddingLocked({ has_indexed_messages: false }), false);
  assert.equal(chatHistoryEmbeddingLocked(null), false);
  assert.equal(chatHistoryEmbeddingLocked(undefined), false);
});

test('role helpers deny a viewer the admin-only registry sections', () => {
  assert.equal(roleAtLeast('viewer', 'admin'), false);
  const deniedButRegistered = settingsSection('platform-api-keys');
  assert.ok(deniedButRegistered);
  assert.equal(settingsSectionsForRole('viewer').some((section) => section.key === 'platform-api-keys'), false);
});
test('rejects weak new passwords through the ported password policy', () => {
  assert.throws(() => profilePasswordPatch('old', 'abc123', 'abc123'), /password policy/);
  assert.throws(() => profilePasswordPatch('old', 'abc123', 'abc123', { complexPasswordEnabled: true }), /password policy/);
  assert.deepEqual(profilePasswordPatch('old', 'Str0ng!Pass', 'Str0ng!Pass', { complexPasswordEnabled: true }), { old_password: 'old', new_password: 'Str0ng!Pass' });
  assert.deepEqual(profilePasswordPatch('old', 'abcdefgh1', 'abcdefgh1'), { old_password: 'old', new_password: 'abcdefgh1' });
});
