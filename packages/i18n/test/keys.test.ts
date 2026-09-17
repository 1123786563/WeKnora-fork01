import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, isLocale, messages, supportedLocales } from '../src/index.ts';

test('keeps the five locales present in the current source inventory aligned', () => {
  assert.deepEqual([...supportedLocales].sort(), ['en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN'].sort());
  for (const locale of supportedLocales) assert.ok(messages[locale]['auth.login']);
});

test('formats interpolation and falls back to English/key without Vue runtime', () => {
  assert.equal(formatMessage('en-US', 'missing', { name: 'x' }), 'missing');
  // Vue baseline (frontend/src/i18n/locales/en-US.ts auth.login) is authoritative.
  assert.equal(formatMessage('en-US', 'auth.login'), 'Login');
  assert.equal(isLocale('fr-FR'), false);
});

test('keeps the React foundation keys and placeholders aligned across every locale', () => {
  const requiredKeys = ['auth.login', 'auth.email', 'auth.password', 'common.loading', 'common.retry', 'common.cancel', 'common.workspaceRequired', 'common.itemCount'];
  for (const locale of supportedLocales) {
    for (const key of requiredKeys) assert.ok(messages[locale][key], `${locale} is missing ${key}`);
  }
  assert.equal(formatMessage('en-US', 'common.itemCount', { count: 3 }), '3 items');
  assert.equal(formatMessage('zh-CN', 'common.itemCount', { count: 3 }), '3 项');
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const key of requiredKeys) {
    const expected = placeholders(messages['en-US'][key]);
    for (const locale of supportedLocales) assert.deepEqual(placeholders(messages[locale][key]), expected, `${locale} placeholder mismatch for ${key}`);
  }
});

// R455 A1: the knowledge-settings overview tiles (activity/datasource/share/
// graph) are a React-side addition with no Vue nav copy to port, so the
// kbSettings.summary.* keys are fresh translations carried by all five
// locales. The count guard pins the inventory size; the byte checks keep the
// en-US fallback copy identical to the pre-R455 hardcoded strings.
test('keeps the kbSettings.summary tile keys aligned across every locale', () => {
  const summaryKeys = [
    'kbSettings.summary.activity.countOne', 'kbSettings.summary.activity.countOther',
    'kbSettings.summary.activity.inspect', 'kbSettings.summary.activity.emptyLabel',
    'kbSettings.summary.activity.emptyDetail', 'kbSettings.summary.activity.sectionEmpty',
    'kbSettings.summary.datasource.countOne', 'kbSettings.summary.datasource.countOther',
    'kbSettings.summary.datasource.inspect', 'kbSettings.summary.datasource.emptyLabel',
    'kbSettings.summary.datasource.emptyDetail', 'kbSettings.summary.datasource.sectionEmpty',
    'kbSettings.summary.share.countOne', 'kbSettings.summary.share.countOther',
    'kbSettings.summary.share.managed', 'kbSettings.summary.share.emptyLabel',
    'kbSettings.summary.share.emptyDetail', 'kbSettings.summary.share.sectionEmpty',
    'kbSettings.summary.graph.enabledLabel', 'kbSettings.summary.graph.enabledDetail',
    'kbSettings.summary.graph.disabledLabel', 'kbSettings.summary.graph.disabledDetail',
    // R456 A1: the parser/vectorStore/storage tiles keep their data-driven
    // label/detail literals (dynamic engine and store names) but the hardcoded
    // English fallback copy gains summary keys for the non-data branches.
    'kbSettings.summary.parser.customRulesLabel', 'kbSettings.summary.parser.overridesDetail',
    'kbSettings.summary.parser.defaultLabel', 'kbSettings.summary.parser.noOverridesDetail',
    'kbSettings.summary.vectorStore.unavailableLabel', 'kbSettings.summary.vectorStore.unavailableDetail',
    'kbSettings.summary.vectorStore.boundLabel', 'kbSettings.summary.vectorStore.explicitDetail',
    'kbSettings.summary.vectorStore.defaultLabel', 'kbSettings.summary.vectorStore.noBindingDetail',
    'kbSettings.summary.storage.instanceLabel', 'kbSettings.summary.storage.providerDetail',
    'kbSettings.summary.storage.defaultLabel', 'kbSettings.summary.storage.noInstanceDetail',
  ];
  assert.equal(summaryKeys.length, 36);
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
  for (const locale of supportedLocales) {
    const localeSummaryKeys = Object.keys(messages[locale]).filter((key) => key.startsWith('kbSettings.summary.'));
    assert.equal(localeSummaryKeys.length, 36, `${locale} kbSettings.summary key count`);
    for (const key of summaryKeys) {
      assert.ok(messages[locale][key], `${locale} is missing ${key}`);
      assert.notEqual(messages[locale][key], key, `${locale} ${key} resolved to the raw key`);
    }
    for (const key of summaryKeys.filter((candidate) => candidate.endsWith('countOne') || candidate.endsWith('countOther'))) {
      assert.deepEqual(placeholders(messages[locale][key]), ['count'], `${locale} ${key} must interpolate {count}`);
    }
    for (const key of summaryKeys.filter((candidate) => !candidate.includes('count'))) {
      assert.deepEqual(placeholders(messages[locale][key]), [], `${locale} ${key} must not carry placeholders`);
    }
  }
  // en-US reuses the pre-R455 hardcoded English byte-for-byte.
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.emptyLabel'), 'No data sources');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.emptyDetail'), 'Add an external connector');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.share.emptyLabel'), 'Not shared');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.graph.enabledLabel'), 'Knowledge graph enabled');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.activity.sectionEmpty'), 'No recorded changes for this knowledge base.');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.datasource.countOther', { count: 2 }), '2 data sources');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.datasource.countOther', { count: 2 }), '2 个数据源');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.share.emptyLabel'), '未共享');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.graph.enabledLabel'), '知识图谱已启用');
  // R456: en-US keeps the hardcoded parser/vectorStore/storage tile copy
  // byte-for-byte; zh-CN renders idiomatic Chinese with no English residue.
  assert.equal(formatMessage('en-US', 'kbSettings.summary.parser.customRulesLabel'), 'Custom rules');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.parser.overridesDetail'), 'File-type overrides');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.parser.defaultLabel'), 'Default parser');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.parser.noOverridesDetail'), 'No file-type overrides');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.unavailableLabel'), 'Vector store unavailable');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.unavailableDetail'), 'Check the global vector-store settings');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.boundLabel'), 'Bound vector store');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.explicitDetail'), 'Explicit binding');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.defaultLabel'), 'System default');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.vectorStore.noBindingDetail'), 'No explicit binding');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.storage.instanceLabel'), 'Storage instance');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.storage.providerDetail'), 'Provider configured');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.storage.defaultLabel'), 'System default');
  assert.equal(formatMessage('en-US', 'kbSettings.summary.storage.noInstanceDetail'), 'No explicit instance');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.parser.customRulesLabel'), '自定义规则');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.parser.overridesDetail'), '按文件类型覆盖');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.parser.defaultLabel'), '默认解析引擎');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.parser.noOverridesDetail'), '未按文件类型覆盖');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.unavailableLabel'), '向量存储不可用');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.unavailableDetail'), '请检查全局向量存储设置');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.boundLabel'), '已绑定的向量存储');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.explicitDetail'), '显式绑定');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.defaultLabel'), '系统默认');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.vectorStore.noBindingDetail'), '未显式绑定');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.storage.instanceLabel'), '存储实例');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.storage.providerDetail'), '已配置存储提供方');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.storage.defaultLabel'), '系统默认');
  assert.equal(formatMessage('zh-CN', 'kbSettings.summary.storage.noInstanceDetail'), '未绑定实例');
});

// R457 A1: the editor-section headings/descriptions (the eyebrow, h3 title and
// muted sentence above every knowledge-settings section) were the last
// hardcoded English on the surface ("Basics", "Parser", "Storage", …). Vue's
// navGroups/sidebar copy is already i18n through knowledgeEditor.navGroups.* /
// knowledgeEditor.sidebar.*; the section headings gain fresh
// kbSettings.sections.* keys carried by all five locales. en-US keeps the
// pre-R457 hardcoded strings byte-for-byte.
test('keeps the kbSettings.sections heading keys aligned across every locale', () => {
  const sectionKeys: Array<[string, string, string]> = [
    ['basic', 'Basics', 'Name, description and knowledge-base type'],
    ['models', 'Models', 'Language and embedding models'],
    ['faq', 'FAQ', 'FAQ indexing modes'],
    ['multimodal', 'Multimodal', 'Image description processing'],
    ['asr', 'Speech recognition', 'Audio transcription model'],
    ['vectorStore', 'Vector store', 'Bound retrieval engine and health'],
    ['parser', 'Parser', 'File-type parser rules'],
    ['chunking', 'Chunking', 'Chunk size and splitting behavior'],
    ['advanced', 'Advanced', 'Question generation and extra options'],
    ['storage', 'Storage', 'Files and document instance'],
    ['datasource', 'Data sources', 'External connectors and sync status'],
    ['share', 'Share', 'Spaces with access to this knowledge base'],
    ['activity', 'Activity', 'Recent configuration changes'],
    ['graph', 'Knowledge graph', 'Entity and relationship extraction'],
  ];
  const expected = sectionKeys.flatMap(([key]) => [
    `kbSettings.sections.${key}.label`,
    `kbSettings.sections.${key}.description`,
  ]);
  assert.equal(expected.length, 28);
  const placeholders = (value: string) => [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]);
  for (const locale of supportedLocales) {
    const localeSectionKeys = Object.keys(messages[locale]).filter((key) => key.startsWith('kbSettings.sections.'));
    assert.equal(localeSectionKeys.length, 28, `${locale} kbSettings.sections key count`);
    for (const key of expected) {
      assert.ok(messages[locale][key], `${locale} is missing ${key}`);
      assert.notEqual(messages[locale][key], key, `${locale} ${key} resolved to the raw key`);
      assert.deepEqual(placeholders(messages[locale][key]), [], `${locale} ${key} must not carry placeholders`);
    }
    // The summary baseline (36 keys) stays untouched by this round.
    assert.equal(Object.keys(messages[locale]).filter((key) => key.startsWith('kbSettings.summary.')).length, 36, `${locale} kbSettings.summary key count`);
  }
  // en-US reuses the pre-R457 hardcoded English byte-for-byte.
  for (const [key, label, description] of sectionKeys) {
    assert.equal(formatMessage('en-US', `kbSettings.sections.${key}.label`), label);
    assert.equal(formatMessage('en-US', `kbSettings.sections.${key}.description`), description);
  }
  // zh-CN renders idiomatic Chinese aligned with the Vue sidebar vocabulary.
  const zhLabels: Record<string, string> = {
    basic: '基本信息', models: '模型配置', faq: 'FAQ 设置', multimodal: '图像处理',
    asr: '音频处理', vectorStore: '向量存储', parser: '解析引擎', chunking: '分块设置',
    advanced: '高级设置', storage: '存储引擎', datasource: '数据源', share: '共享管理',
    activity: '活动记录', graph: '知识图谱',
  };
  for (const [key, label] of Object.entries(zhLabels)) {
    assert.equal(formatMessage('zh-CN', `kbSettings.sections.${key}.label`), label);
    assert.ok(formatMessage('zh-CN', `kbSettings.sections.${key}.description`).length > 0, `zh-CN sections.${key}.description is non-empty`);
  }
});

test('keeps zh-CN knowledge-settings headings and navigation free of hardcoded English', () => {
  const guarded = (key: string) =>
    key.startsWith('kbSettings.sections.')
    || key.startsWith('knowledgeEditor.sidebar.')
    || key.startsWith('knowledgeEditor.navGroups.');
  for (const [key, value] of Object.entries(messages['zh-CN'])) {
    if (!guarded(key)) continue;
    // Uppercase proper nouns (FAQ, Embedding) are allowed; lowercase residue
    // from hardcoded English sentences is not.
    assert.doesNotMatch(value, /[a-z]/, `zh-CN ${key} must not embed lowercase English residue`);
  }
});
